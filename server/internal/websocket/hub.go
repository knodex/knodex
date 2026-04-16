// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/knodex/knodex/server/internal/util/env"
)

const (
	// MaxConnections is the maximum number of concurrent WebSocket connections
	MaxConnections = 100

	// BroadcastBufferSize is the size of the broadcast channel buffer
	BroadcastBufferSize = 256

	// DebounceInterval is the minimum time between updates for the same resource
	DebounceInterval = 100 * time.Millisecond

	// CleanupInterval is how often the debounce map is swept for stale entries
	CleanupInterval = 5 * time.Second
)

// CountFunc computes RBAC-filtered RGD and instance counts for a specific user.
// Returns (rgdCount, instanceCount). Called per-client with their context.
type CountFunc func(ctx context.Context, userID string, projects []string, groups []string) (int, int)

// Hub manages all WebSocket client connections and message broadcasting
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan *Message

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// countRequest receives clients that need an initial counts push.
	// Processed within the Run() event loop to avoid unmanaged goroutines.
	countRequest chan *Client

	// ctxMu protects ctx from concurrent read/write between Run() and SendCountsToClients().
	ctxMu sync.RWMutex
	// ctx is the hub's lifecycle context, set when Run() is called.
	// Used to derive contexts for RBAC checks and count computations.
	ctx context.Context

	// logger is the hub's structured logger. Set via NewHub; nil falls back to slog.Default().
	logger *slog.Logger

	// Mutex for thread-safe client access
	mu sync.RWMutex

	// Debounce tracking for updates
	lastUpdate     map[string]time.Time
	lastUpdateLock sync.Mutex

	// Count function for initial-connect push
	countFn CountFunc

	// maxConnections is the maximum number of concurrent WebSocket connections.
	// Configurable via WEBSOCKET_MAX_CONNECTIONS env var; defaults to MaxConnections (100).
	maxConnections int

	// Metrics
	totalConnections  int64
	totalMessages     int64
	activeConnections int
	connectionsMu     sync.RWMutex
}

// NewHub creates a new Hub instance with the given logger.
// Pass nil to use slog.Default(). The hub ctx defaults to context.Background()
// and is replaced by Run's argument.
func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan *Message, BroadcastBufferSize),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		countRequest:   make(chan *Client, 16),
		ctx:            context.Background(),
		logger:         logger,
		lastUpdate:     make(map[string]time.Time),
		maxConnections: validatedMaxConnections(logger, env.GetInt("WEBSOCKET_MAX_CONNECTIONS", MaxConnections)),
	}
}

// validatedMaxConnections ensures maxConnections is at least 1.
// Returns the default MaxConnections if the value is invalid.
func validatedMaxConnections(logger *slog.Logger, n int) int {
	if n < 1 {
		logger.Warn("WEBSOCKET_MAX_CONNECTIONS must be >= 1, using default",
			"configured", n,
			"default", MaxConnections,
		)
		return MaxConnections
	}
	return n
}

// Run starts the hub's main event loop. The provided context controls the hub's
// lifecycle: when ctx is canceled, the hub closes all clients and returns.
// Callers use the context's cancel function instead of a separate Stop method.
func (h *Hub) Run(ctx context.Context) {
	h.ctxMu.Lock()
	h.ctx = ctx
	h.ctxMu.Unlock()
	h.logger.Info("WebSocket hub started")
	cleanupTicker := time.NewTicker(CleanupInterval)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.closeAllClients()
			h.logger.Info("WebSocket hub stopped")
			return

		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)

		case client := <-h.countRequest:
			h.handleCountRequest(client)

		case <-cleanupTicker.C:
			h.cleanLastUpdate()
		}
	}
}

// clientAddr returns the remote address of the client for logging.
// Returns "unknown" when conn is nil (e.g., in test mocks).
func clientAddr(client *Client) string {
	if client.conn != nil {
		return client.conn.RemoteAddr().String()
	}
	return "unknown"
}

// closeAllClients closes all connected clients during shutdown
func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	clientCount := len(h.clients)
	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
	}

	h.connectionsMu.Lock()
	h.activeConnections = 0
	h.connectionsMu.Unlock()

	h.logger.Info("WebSocket hub closed all clients", "count", clientCount)
}

// handleRegister handles client registration
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check connection limit
	if len(h.clients) >= h.maxConnections {
		h.logger.Warn("WebSocket connection limit reached",
			"limit", h.maxConnections,
			"clientAddr", clientAddr(client))

		// Send error message and close
		errMsg, _ := NewErrorMessage("CONNECTION_LIMIT", "Maximum connection limit reached")
		client.send <- errMsg
		close(client.send)
		return
	}

	h.clients[client] = true

	h.connectionsMu.Lock()
	h.activeConnections = len(h.clients)
	h.totalConnections++
	h.connectionsMu.Unlock()

	h.logger.Info("WebSocket client registered",
		"clientAddr", clientAddr(client),
		"activeConnections", len(h.clients))

	// Queue initial counts push to be processed within the event loop.
	// Non-blocking: if the channel is full, skip (client will get counts on next broadcast).
	if h.countFn != nil {
		select {
		case h.countRequest <- client:
		default:
			h.logger.Warn("Count request buffer full, skipping initial counts",
				"clientAddr", clientAddr(client))
		}
	}
}

// handleCountRequest computes and sends initial RBAC-filtered counts to a newly
// registered client. Runs within the event loop — no goroutine or panic recovery needed.
func (h *Hub) handleCountRequest(client *Client) {
	// Verify client is still registered (may have disconnected between register and now)
	h.mu.RLock()
	registered := h.clients[client]
	h.mu.RUnlock()
	if !registered {
		return
	}

	ctx := h.ctx
	userID, projects, groups := client.GetUserContext()

	if client.CachedHasGlobalAccess(ctx) {
		projects = nil
	}

	rgdCount, instanceCount := h.countFn(ctx, userID, projects, groups)

	msg, err := NewCountsUpdateMessage(rgdCount, instanceCount)
	if err != nil {
		h.logger.Error("Failed to create initial counts message", "error", err, "userID", userID)
		return
	}

	select {
	case client.send <- msg:
	default:
		h.logger.Warn("Client send buffer full, skipping initial counts", "userID", userID)
	}
}

// handleUnregister handles client unregistration
func (h *Hub) handleUnregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)

		h.connectionsMu.Lock()
		h.activeConnections = len(h.clients)
		h.connectionsMu.Unlock()

		h.logger.Info("WebSocket client unregistered",
			"clientAddr", clientAddr(client),
			"activeConnections", len(h.clients))
	}
}

// handleBroadcast sends a message to all subscribed clients
func (h *Hub) handleBroadcast(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.connectionsMu.Lock()
	h.totalMessages++
	h.connectionsMu.Unlock()

	// Extract projectID once before iterating clients to avoid N×JSON deserialization
	projectID := h.extractProjectID(message)

	for client := range h.clients {
		if h.shouldSendToClient(client, message, projectID) {
			select {
			case client.send <- message:
			default:
				// Client buffer full, skip this message
				h.logger.Warn("Client send buffer full, skipping message",
					"clientAddr", clientAddr(client))
			}
		}
	}
}

// extractProjectID extracts the project ID from a message's data payload.
// Called once per broadcast to avoid repeated JSON deserialization per client.
func (h *Hub) extractProjectID(message *Message) string {
	var projectID string
	var err error

	switch message.Type {
	case MessageTypeInstanceUpdate:
		var data InstanceUpdateData
		if err = json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}
	case MessageTypeRGDUpdate:
		var data RGDUpdateData
		if err = json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}
	case MessageTypeDriftUpdate:
		var data DriftUpdateData
		if err = json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}
	case MessageTypeRevisionUpdate:
		var data RevisionUpdateData
		if err = json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}
	case MessageTypeResourceEvent:
		var data ResourceEventData
		if err = json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}
	}

	if err != nil {
		h.logger.Warn("failed to unmarshal message data for project ID extraction",
			"messageType", message.Type,
			"error", err)
	}

	return projectID
}

// shouldSendToClient checks if a message should be sent to a specific client.
// projectID is pre-extracted from the message to avoid per-client JSON deserialization.
func (h *Hub) shouldSendToClient(client *Client, message *Message, projectID string) bool {
	// Always send error messages and pong
	if message.Type == MessageTypeError || message.Type == MessageTypePong {
		return true
	}

	client.subMu.RLock()
	hasResourceSubscription := client.subscriptions["all"] ||
		client.subscriptions["instance"] || client.subscriptions["instances"] ||
		client.subscriptions["rgd"] || client.subscriptions["rgds"]
	hasViolationSubscription := client.subscriptions["all"] || client.subscriptions["violations"]
	client.subMu.RUnlock()

	// Check if client has appropriate subscription for message type
	switch message.Type {
	case MessageTypeViolationUpdate, MessageTypeTemplateUpdate, MessageTypeConstraintUpdate:
		// All compliance events require violations subscription (admin-only)
		if !hasViolationSubscription {
			return false
		}
	case MessageTypeInstanceUpdate, MessageTypeRGDUpdate, MessageTypeDriftUpdate, MessageTypeRevisionUpdate, MessageTypeResourceEvent:
		if !hasResourceSubscription {
			return false
		}
	default:
		// Unknown message type - don't send
		return false
	}

	// Get user context for project-scoped filtering
	userID, projects, _ := client.GetUserContext()

	// ArgoCD-aligned: Check global admin access via Casbin, not a boolean flag
	// Users with "*:*" permission have global admin access to all resources
	if client.CachedHasGlobalAccess(h.ctx) {
		return true
	}

	// Compliance messages (violations, templates, constraints) are always sent to subscribed admins only
	// Non-admin users don't receive compliance updates as this data is admin-only
	if message.Type == MessageTypeViolationUpdate || message.Type == MessageTypeTemplateUpdate || message.Type == MessageTypeConstraintUpdate {
		// Only global admins receive compliance updates (handled above)
		// Non-admin users should not see compliance data
		h.logger.Debug("Compliance message not sent to non-admin user",
			"userID", userID,
			"messageType", message.Type)
		return false
	}

	// If no project ID in message, don't send (safety default)
	if projectID == "" {
		h.logger.Debug("Message has no project ID, not sending to client",
			"userID", userID,
			"messageType", message.Type)
		return false
	}

	// Check if user belongs to the project
	for _, userProject := range projects {
		if userProject == projectID {
			return true
		}
	}

	// User doesn't belong to this project - don't send
	return false
}

// enqueue sends a message to the broadcast channel, logging a warning if the buffer is full.
func (h *Hub) enqueue(msg *Message, dropLogAttrs ...any) {
	select {
	case h.broadcast <- msg:
	default:
		h.logger.Warn("Broadcast buffer full, dropping message", dropLogAttrs...)
	}
}

// BroadcastInstanceUpdate sends an instance update to all subscribed clients.
// kind is the resource kind (e.g., "WebApp") used for client-side cache key matching.
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering.
func (h *Hub) BroadcastInstanceUpdate(action Action, namespace, kind, name string, instance interface{}, projectNamespace string) {
	key := "instance:" + namespace + "/" + kind + "/" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewInstanceUpdateMessage(action, namespace, kind, name, instance, projectNamespace)
	if err != nil {
		h.logger.Error("Failed to create instance update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "instance", "namespace", namespace, "kind", kind, "name", name)
}

// BroadcastInstanceEventUpdate sends a per-resource deploy event to all subscribed clients.
// This is used for K8s Event notifications via the EventAdapter.
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering.
func (h *Hub) BroadcastInstanceEventUpdate(instanceID, resourceKind, resourceName, status, message, projectNamespace string) {
	key := "event:" + instanceID
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewResourceEventMessage(instanceID, resourceKind, resourceName, status, message, projectNamespace)
	if err != nil {
		h.logger.Error("Failed to create resource event message", "error", err)
		return
	}

	h.enqueue(msg, "type", "resource_event", "instanceID", instanceID, "resourceKind", resourceKind, "resourceName", resourceName)
}

// BroadcastDriftUpdate sends a drift state change to all subscribed clients.
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering.
func (h *Hub) BroadcastDriftUpdate(namespace, kind, name string, drifted bool, projectNamespace string) {
	key := "drift:" + namespace + "/" + kind + "/" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewDriftUpdateMessage(namespace, kind, name, drifted, projectNamespace)
	if err != nil {
		h.logger.Error("Failed to create drift update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "drift", "namespace", namespace, "kind", kind, "name", name)
}

// BroadcastRGDUpdate sends an RGD update to all subscribed clients.
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering.
func (h *Hub) BroadcastRGDUpdate(action Action, name string, rgd interface{}, projectNamespace string) {
	key := "rgd:" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewRGDUpdateMessage(action, name, rgd, projectNamespace)
	if err != nil {
		h.logger.Error("Failed to create RGD update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "rgd", "name", name, "projectNamespace", projectNamespace)
}

// BroadcastViolationUpdate sends a violation update to all subscribed clients (admin-only).
// action should be ActionAdd for newly detected violations, or ActionDelete for resolved violations.
// This is an enterprise-only feature for OPA Gatekeeper compliance monitoring.
func (h *Hub) BroadcastViolationUpdate(action Action, constraintKind, constraintName string, resource ViolationResourceData, message, enforcementAction string) {
	key := "violation:" + constraintKind + "/" + constraintName + "/" + resource.Kind + "/" + resource.Namespace + "/" + resource.Name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewViolationUpdateMessage(action, constraintKind, constraintName, resource, message, enforcementAction)
	if err != nil {
		h.logger.Error("Failed to create violation update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "violation", "constraintKind", constraintKind, "constraintName", constraintName)
}

// BroadcastTemplateUpdate sends a constraint template update to all subscribed clients (admin-only).
// action should be ActionAdd, ActionUpdate, or ActionDelete.
// This is an enterprise-only feature for OPA Gatekeeper compliance monitoring.
func (h *Hub) BroadcastTemplateUpdate(action Action, name, kind, description string) {
	key := "template:" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewTemplateUpdateMessage(action, name, kind, description)
	if err != nil {
		h.logger.Error("Failed to create template update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "template", "name", name, "kind", kind)
}

// BroadcastConstraintUpdate sends a constraint update to all subscribed clients (admin-only).
// action should be ActionAdd, ActionUpdate, or ActionDelete.
// This is an enterprise-only feature for OPA Gatekeeper compliance monitoring.
func (h *Hub) BroadcastConstraintUpdate(action Action, kind, name, enforcementAction string, violationCount int) {
	key := "constraint:" + kind + "/" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewConstraintUpdateMessage(action, kind, name, enforcementAction, violationCount)
	if err != nil {
		h.logger.Error("Failed to create constraint update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "constraint", "kind", kind, "name", name)
}

// BroadcastRevisionUpdate sends a GraphRevision update to all subscribed clients.
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering.
func (h *Hub) BroadcastRevisionUpdate(action Action, rgdName string, revision int, projectNamespace string) {
	key := "revision:" + rgdName
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewRevisionUpdateMessage(action, rgdName, revision, projectNamespace)
	if err != nil {
		h.logger.Error("Failed to create revision update message", "error", err)
		return
	}

	h.enqueue(msg, "type", "revision", "rgdName", rgdName, "revision", revision)
}

// SetCountFunc sets the function used to compute per-user counts on initial connect.
func (h *Hub) SetCountFunc(fn CountFunc) {
	h.countFn = fn
}

// clientSnapshot holds the data needed to compute and send counts to a single client.
// Collected under h.mu.RLock() so the lock can be released before count computation.
type clientSnapshot struct {
	client        *Client
	userID        string
	projects      []string
	groups        []string
	isGlobalAdmin bool
}

// SendCountsToClients computes RBAC-filtered counts per-client and sends personalized
// counts_update messages. Unlike standard broadcasts that send the same message to all,
// each client receives different counts based on their project access.
// Called from watcher goroutines (outside Hub event loop). Client references and their
// cached admin status are collected under RLock, then the lock is released before count
// computation begins.
func (h *Hub) SendCountsToClients(countFn CountFunc) {
	// Debounce rapid changes (e.g., 10 instances created in 500ms)
	if !h.shouldBroadcast("counts") {
		return
	}

	h.ctxMu.RLock()
	ctx := h.ctx
	h.ctxMu.RUnlock()

	// Collect client snapshots under read lock, then release before computation.
	h.mu.RLock()
	snapshots := make([]clientSnapshot, 0, len(h.clients))
	for client := range h.clients {
		// Check subscription - must have resource subscription
		client.subMu.RLock()
		hasSubscription := client.subscriptions["all"] ||
			client.subscriptions["instance"] || client.subscriptions["instances"] ||
			client.subscriptions["rgd"] || client.subscriptions["rgds"]
		client.subMu.RUnlock()

		if !hasSubscription {
			continue
		}

		userID, projects, groups := client.GetUserContext()
		isAdmin := client.CachedHasGlobalAccess(ctx)

		snapshots = append(snapshots, clientSnapshot{
			client:        client,
			userID:        userID,
			projects:      projects,
			groups:        groups,
			isGlobalAdmin: isAdmin,
		})
	}
	h.mu.RUnlock()

	// Compute counts and send outside the lock.
	for _, snap := range snapshots {
		projects := snap.projects
		if snap.isGlobalAdmin {
			projects = nil
		}

		rgdCount, instanceCount := countFn(ctx, snap.userID, projects, snap.groups)

		msg, err := NewCountsUpdateMessage(rgdCount, instanceCount)
		if err != nil {
			h.logger.Error("Failed to create counts update message", "error", err, "userID", snap.userID)
			continue
		}

		// Safe non-blocking send: client may have been unregistered after snapshot,
		// which closes client.send. trySend recovers from the closed-channel panic.
		if !trySend(snap.client, msg) {
			h.logger.Warn("Client send buffer full or disconnected, skipping counts update",
				"userID", snap.userID)
		}
	}
}

// trySend attempts a non-blocking send on the client's send channel.
// Returns true if sent, false if the buffer was full or channel was closed.
// Safe to call after releasing h.mu.RLock() — handles the race where
// handleUnregister closes client.send between snapshot and send.
func trySend(client *Client, msg *Message) (sent bool) {
	defer func() {
		if r := recover(); r != nil {
			sent = false
		}
	}()
	select {
	case client.send <- msg:
		return true
	default:
		return false
	}
}

// cleanLastUpdate removes stale entries from the lastUpdate debounce map.
// Entries older than 10*DebounceInterval (1s) have served their debounce purpose
// and can be safely evicted to prevent unbounded map growth.
func (h *Hub) cleanLastUpdate() {
	cutoff := time.Now().Add(-10 * DebounceInterval)
	h.lastUpdateLock.Lock()
	defer h.lastUpdateLock.Unlock()
	evicted := 0
	for key, t := range h.lastUpdate {
		if t.Before(cutoff) {
			delete(h.lastUpdate, key)
			evicted++
		}
	}
	if evicted > 0 {
		h.logger.Debug("Cleaned stale debounce entries", "evicted", evicted, "remaining", len(h.lastUpdate))
	}
}

// shouldBroadcast checks if a broadcast should be sent based on debouncing
func (h *Hub) shouldBroadcast(key string) bool {
	h.lastUpdateLock.Lock()
	defer h.lastUpdateLock.Unlock()

	now := time.Now()
	if last, ok := h.lastUpdate[key]; ok {
		if now.Sub(last) < DebounceInterval {
			return false
		}
	}
	h.lastUpdate[key] = now
	return true
}

// GetMetrics returns current hub metrics
func (h *Hub) GetMetrics() map[string]interface{} {
	h.connectionsMu.RLock()
	defer h.connectionsMu.RUnlock()

	return map[string]interface{}{
		"activeConnections":  h.activeConnections,
		"totalConnections":   h.totalConnections,
		"totalMessages":      h.totalMessages,
		"maxConnections":     h.maxConnections,
		"broadcastQueueSize": len(h.broadcast),
	}
}

// ActiveConnections returns the number of active connections
func (h *Hub) ActiveConnections() int {
	h.connectionsMu.RLock()
	defer h.connectionsMu.RUnlock()
	return h.activeConnections
}

// Register returns the register channel for adding new clients
func (h *Hub) Register() chan<- *Client {
	return h.register
}

// Unregister returns the unregister channel for removing clients
func (h *Hub) Unregister() chan<- *Client {
	return h.unregister
}
