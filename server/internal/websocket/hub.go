package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

const (
	// MaxConnections is the maximum number of concurrent WebSocket connections
	MaxConnections = 100

	// BroadcastBufferSize is the size of the broadcast channel buffer
	BroadcastBufferSize = 256

	// DebounceInterval is the minimum time between updates for the same resource
	DebounceInterval = 100 * time.Millisecond
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

	// Stop signal for graceful shutdown
	stop chan struct{}

	// Mutex for thread-safe client access
	mu sync.RWMutex

	// Debounce tracking for updates
	lastUpdate     map[string]time.Time
	lastUpdateLock sync.Mutex

	// Count function for initial-connect push
	countFn CountFunc

	// Metrics
	totalConnections  int64
	totalMessages     int64
	activeConnections int
	connectionsMu     sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *Message, BroadcastBufferSize),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
		lastUpdate: make(map[string]time.Time),
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	slog.Info("WebSocket hub started")

	for {
		select {
		case <-h.stop:
			h.closeAllClients()
			slog.Info("WebSocket hub stopped")
			return

		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)
		}
	}
}

// Stop signals the hub to shut down gracefully.
// It closes all client connections and stops the event loop.
func (h *Hub) Stop() {
	close(h.stop)
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

	slog.Info("WebSocket hub closed all clients", "count", clientCount)
}

// handleRegister handles client registration
func (h *Hub) handleRegister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check connection limit
	if len(h.clients) >= MaxConnections {
		slog.Warn("WebSocket connection limit reached",
			"limit", MaxConnections,
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

	slog.Info("WebSocket client registered",
		"clientAddr", clientAddr(client),
		"activeConnections", len(h.clients))

	// Send initial counts to newly connected client (AC: #1, #6)
	if h.countFn != nil {
		countFn := h.countFn
		go func() {
			// Recover from panic if client disconnects and send channel is closed
			// between goroutine spawn and the send operation.
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("Recovered panic in initial count push", "error", r)
				}
			}()

			ctx := context.Background()
			userID, projects, groups := client.GetUserContext()

			if client.HasGlobalAccess(ctx) {
				projects = nil
			}

			rgdCount, instanceCount := countFn(ctx, userID, projects, groups)

			msg, err := NewCountsUpdateMessage(rgdCount, instanceCount)
			if err != nil {
				slog.Error("Failed to create initial counts message", "error", err, "userID", userID)
				return
			}

			select {
			case client.send <- msg:
			default:
				slog.Warn("Client send buffer full, skipping initial counts", "userID", userID)
			}
		}()
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

		slog.Info("WebSocket client unregistered",
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

	for client := range h.clients {
		if h.shouldSendToClient(client, message) {
			select {
			case client.send <- message:
			default:
				// Client buffer full, skip this message
				slog.Warn("Client send buffer full, skipping message",
					"clientAddr", clientAddr(client))
			}
		}
	}
}

// shouldSendToClient checks if a message should be sent to a specific client
func (h *Hub) shouldSendToClient(client *Client, message *Message) bool {
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
	case MessageTypeInstanceUpdate, MessageTypeRGDUpdate:
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
	ctx := context.Background()
	if client.HasGlobalAccess(ctx) {
		return true
	}

	// Compliance messages (violations, templates, constraints) are always sent to subscribed admins only
	// Non-admin users don't receive compliance updates as this data is admin-only
	if message.Type == MessageTypeViolationUpdate || message.Type == MessageTypeTemplateUpdate || message.Type == MessageTypeConstraintUpdate {
		// Only global admins receive compliance updates (handled above)
		// Non-admin users should not see compliance data
		slog.Debug("Compliance message not sent to non-admin user",
			"userID", userID,
			"messageType", message.Type)
		return false
	}

	// Extract project ID from message data for instance/RGD messages
	var projectID string
	switch message.Type {
	case MessageTypeInstanceUpdate:
		var data InstanceUpdateData
		if err := json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}

	case MessageTypeRGDUpdate:
		var data RGDUpdateData
		if err := json.Unmarshal(message.Data, &data); err == nil {
			projectID = data.ProjectID
		}
	}

	// If no project ID in message, don't send (safety default)
	if projectID == "" {
		slog.Debug("Message has no project ID, not sending to client",
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

// BroadcastInstanceUpdate sends an instance update to all subscribed clients
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering
func (h *Hub) BroadcastInstanceUpdate(action Action, namespace, name string, instance interface{}, projectNamespace string) {
	// Apply debouncing
	key := "instance:" + namespace + "/" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewInstanceUpdateMessage(action, namespace, name, instance, projectNamespace)
	if err != nil {
		slog.Error("Failed to create instance update message", "error", err)
		return
	}

	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("Broadcast buffer full, dropping instance update",
			"namespace", namespace, "name", name, "projectNamespace", projectNamespace)
	}
}

// BroadcastRGDUpdate sends an RGD update to all subscribed clients
// projectNamespace is the project's namespace name (e.g., "acme"), used for RBAC filtering
func (h *Hub) BroadcastRGDUpdate(action Action, name string, rgd interface{}, projectNamespace string) {
	// Apply debouncing
	key := "rgd:" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewRGDUpdateMessage(action, name, rgd, projectNamespace)
	if err != nil {
		slog.Error("Failed to create RGD update message", "error", err)
		return
	}

	select {
	case h.broadcast <- msg:
	default:
		slog.Warn("Broadcast buffer full, dropping RGD update", "name", name, "projectNamespace", projectNamespace)
	}
}

// BroadcastViolationUpdate sends a violation update to all subscribed clients (admin-only).
// action should be ActionAdd for newly detected violations, or ActionDelete for resolved violations.
// This is an enterprise-only feature for OPA Gatekeeper compliance monitoring.
func (h *Hub) BroadcastViolationUpdate(action Action, constraintKind, constraintName string, resource ViolationResourceData, message, enforcementAction string) {
	// Apply debouncing using a unique key for the violation
	key := "violation:" + constraintKind + "/" + constraintName + "/" + resource.Kind + "/" + resource.Namespace + "/" + resource.Name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewViolationUpdateMessage(action, constraintKind, constraintName, resource, message, enforcementAction)
	if err != nil {
		slog.Error("Failed to create violation update message", "error", err)
		return
	}

	select {
	case h.broadcast <- msg:
		slog.Debug("Broadcast violation update",
			"action", action,
			"constraintKind", constraintKind,
			"constraintName", constraintName,
			"resourceKind", resource.Kind,
			"resourceNamespace", resource.Namespace,
			"resourceName", resource.Name)
	default:
		slog.Warn("Broadcast buffer full, dropping violation update",
			"constraintKind", constraintKind,
			"constraintName", constraintName)
	}
}

// BroadcastTemplateUpdate sends a constraint template update to all subscribed clients (admin-only).
// action should be ActionAdd, ActionUpdate, or ActionDelete.
// This is an enterprise-only feature for OPA Gatekeeper compliance monitoring.
func (h *Hub) BroadcastTemplateUpdate(action Action, name, kind, description string) {
	// Apply debouncing using a unique key for the template
	key := "template:" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewTemplateUpdateMessage(action, name, kind, description)
	if err != nil {
		slog.Error("Failed to create template update message", "error", err)
		return
	}

	select {
	case h.broadcast <- msg:
		slog.Debug("Broadcast template update",
			"action", action,
			"name", name,
			"kind", kind)
	default:
		slog.Warn("Broadcast buffer full, dropping template update",
			"name", name)
	}
}

// BroadcastConstraintUpdate sends a constraint update to all subscribed clients (admin-only).
// action should be ActionAdd, ActionUpdate, or ActionDelete.
// This is an enterprise-only feature for OPA Gatekeeper compliance monitoring.
func (h *Hub) BroadcastConstraintUpdate(action Action, kind, name, enforcementAction string, violationCount int) {
	// Apply debouncing using a unique key for the constraint
	key := "constraint:" + kind + "/" + name
	if !h.shouldBroadcast(key) {
		return
	}

	msg, err := NewConstraintUpdateMessage(action, kind, name, enforcementAction, violationCount)
	if err != nil {
		slog.Error("Failed to create constraint update message", "error", err)
		return
	}

	select {
	case h.broadcast <- msg:
		slog.Debug("Broadcast constraint update",
			"action", action,
			"kind", kind,
			"name", name,
			"enforcementAction", enforcementAction,
			"violationCount", violationCount)
	default:
		slog.Warn("Broadcast buffer full, dropping constraint update",
			"kind", kind,
			"name", name)
	}
}

// SetCountFunc sets the function used to compute per-user counts on initial connect.
func (h *Hub) SetCountFunc(fn CountFunc) {
	h.countFn = fn
}

// SendCountsToClients computes RBAC-filtered counts per-client and sends personalized
// counts_update messages. Unlike standard broadcasts that send the same message to all,
// each client receives different counts based on their project access.
// Called from watcher goroutines (outside Hub event loop) - holds RLock for the entire
// iteration to prevent handleUnregister from closing client.send channels mid-send.
// This is safe because countFn uses in-memory caches only (no I/O).
func (h *Hub) SendCountsToClients(countFn CountFunc) {
	// Debounce rapid changes (e.g., 10 instances created in 500ms)
	if !h.shouldBroadcast("counts") {
		return
	}

	// Hold read lock for entire iteration to prevent concurrent handleUnregister
	// from closing client.send channels (which would panic on send).
	// This mirrors the handleBroadcast pattern.
	h.mu.RLock()
	defer h.mu.RUnlock()

	ctx := context.Background()
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

		// Get user context for RBAC-filtered counts
		userID, projects, groups := client.GetUserContext()

		// Global admins see unfiltered counts (nil projects)
		if client.HasGlobalAccess(ctx) {
			projects = nil
		}

		rgdCount, instanceCount := countFn(ctx, userID, projects, groups)

		msg, err := NewCountsUpdateMessage(rgdCount, instanceCount)
		if err != nil {
			slog.Error("Failed to create counts update message", "error", err, "userID", userID)
			continue
		}

		// Non-blocking send
		select {
		case client.send <- msg:
		default:
			slog.Warn("Client send buffer full, skipping counts update",
				"userID", userID)
		}
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
		"maxConnections":     MaxConnections,
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
