// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// globalAdminCacheTTL is how long a cached global admin check remains valid.
	// WebSocket sessions are token-scoped and short-lived; admin status changing
	// mid-session is rare. A 60s lag is acceptable for the performance gain of
	// avoiding per-message Casbin calls.
	globalAdminCacheTTL = 60 * time.Second

	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512

	// Send buffer size
	sendBufferSize = 256
)

// ClientPolicyEnforcer defines the interface for checking permissions in WebSocket client
// This interface follows the ArgoCD-aligned pattern where permissions are always
// checked via Casbin, never by direct role string comparison.
type ClientPolicyEnforcer interface {
	// CanAccessWithGroups checks if user/groups/server-side roles can perform action on object.
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

// Client represents a WebSocket client connection
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan *Message

	// Subscriptions this client is interested in
	subscriptions map[string]bool
	subMu         sync.RWMutex

	// User context for project-scoped filtering (ArgoCD-aligned: Casbin for all permission checks)
	userID   string
	projects []string
	groups   []string // OIDC groups for Casbin policy evaluation
	userMu   sync.RWMutex

	// Cached global admin status to avoid per-message Casbin calls.
	// Guarded by userMu. Lazily populated on first CachedHasGlobalAccess call.
	isGlobalAdmin       bool
	globalAdminCachedAt time.Time

	// Policy enforcer for permission checks (ArgoCD-aligned: use Casbin, not booleans)
	policyEnforcer ClientPolicyEnforcer

	// Connection lifecycle management - prevents double-close race
	closeOnce sync.Once
	done      chan struct{}
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan *Message, sendBufferSize),
		subscriptions: make(map[string]bool),
		projects:      make([]string, 0),
		groups:        make([]string, 0),
		done:          make(chan struct{}),
	}
}

// close safely closes the client connection exactly once.
// This prevents the race condition where both ReadPump and WritePump
// attempt to close the connection simultaneously.
func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.hub.unregister <- c
		c.conn.Close()
	})
}

// SetUserContext sets the user context for project-scoped filtering with defensive copying.
// ArgoCD-aligned: stores groups for Casbin policy evaluation via OIDC group-to-role mapping.
func (c *Client) SetUserContext(userID string, projects []string, groups []string, policyEnforcer ClientPolicyEnforcer) {
	c.userMu.Lock()
	defer c.userMu.Unlock()
	c.userID = userID

	// Defensive copy: create new slices to prevent external mutation
	c.projects = make([]string, len(projects))
	copy(c.projects, projects)

	c.groups = make([]string, len(groups))
	copy(c.groups, groups)

	c.policyEnforcer = policyEnforcer

	// Invalidate cached admin status — the new enforcer may return a different result.
	c.globalAdminCachedAt = time.Time{}
}

// GetUserContext returns the user context with defensive copying to prevent races
// ArgoCD-aligned: returns groups for Casbin policy evaluation via OIDC group-to-role mapping
func (c *Client) GetUserContext() (userID string, projects []string, groups []string) {
	c.userMu.RLock()
	defer c.userMu.RUnlock()

	// Defensive copy: return copies of slices to prevent external mutation
	projectsCopy := make([]string, len(c.projects))
	copy(projectsCopy, c.projects)

	groupsCopy := make([]string, len(c.groups))
	copy(groupsCopy, c.groups)

	return c.userID, projectsCopy, groupsCopy
}

// HasGlobalAccess checks if the client has global admin access via Casbin
// ArgoCD-aligned: uses Casbin CanAccessWithGroups instead of a boolean flag.
// This is the correct pattern - permission checks should always go through Casbin.
func (c *Client) HasGlobalAccess(ctx context.Context) bool {
	c.userMu.RLock()
	policyEnforcer := c.policyEnforcer
	userID := c.userID
	groups := make([]string, len(c.groups))
	copy(groups, c.groups)
	c.userMu.RUnlock()

	// Fail closed: if no policy enforcer, deny access
	if policyEnforcer == nil {
		slog.Debug("HasGlobalAccess: no policy enforcer configured, denying access",
			"userID", userID)
		return false
	}

	// Check for wildcard access via Casbin (ArgoCD-aligned pattern)
	// Users with "*, *" permission have global admin access
	hasAccess, err := policyEnforcer.CanAccessWithGroups(ctx, userID, groups, "*", "*")
	if err != nil {
		slog.Warn("HasGlobalAccess: permission check failed, denying access",
			"userID", userID,
			"error", err)
		return false
	}

	return hasAccess
}

// CachedHasGlobalAccess returns the cached global admin status, re-evaluating
// via Casbin only when the cache is empty or older than globalAdminCacheTTL.
// This avoids a full Casbin call on every broadcast message for every client.
// Uses double-checked locking to prevent thundering-herd re-evaluation when
// multiple goroutines observe an expired cache simultaneously.
func (c *Client) CachedHasGlobalAccess(ctx context.Context) bool {
	c.userMu.RLock()
	cached := c.globalAdminCachedAt
	result := c.isGlobalAdmin
	c.userMu.RUnlock()

	if !cached.IsZero() && time.Since(cached) < globalAdminCacheTTL {
		return result
	}

	// Cache miss or expired — acquire write lock and re-check before calling Casbin.
	// Another goroutine may have already refreshed while we waited for the lock.
	c.userMu.Lock()
	if !c.globalAdminCachedAt.IsZero() && time.Since(c.globalAdminCachedAt) < globalAdminCacheTTL {
		result = c.isGlobalAdmin
		c.userMu.Unlock()
		return result
	}
	c.userMu.Unlock()

	// Re-evaluate via Casbin (outside lock — may be slow)
	fresh := c.HasGlobalAccess(ctx)

	c.userMu.Lock()
	c.isGlobalAdmin = fresh
	c.globalAdminCachedAt = time.Now()
	c.userMu.Unlock()

	return fresh
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer c.close()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket read error",
					"error", err,
					"clientAddr", c.conn.RemoteAddr().String())
			}
			return
		}

		c.handleMessage(messageBytes)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.close()
	}()

	for {
		select {
		case <-c.done:
			// ReadPump triggered shutdown, exit gracefully
			return

		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			bytes, err := message.Bytes()
			if err != nil {
				slog.Error("Failed to serialize message",
					"error", err,
					"clientAddr", c.conn.RemoteAddr().String())
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
				slog.Warn("WebSocket write error",
					"error", err,
					"clientAddr", c.conn.RemoteAddr().String())
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes an incoming message from the client
func (c *Client) handleMessage(messageBytes []byte) {
	var msg Message
	if err := json.Unmarshal(messageBytes, &msg); err != nil {
		slog.Warn("Invalid WebSocket message format",
			"error", err,
			"clientAddr", c.conn.RemoteAddr().String())
		c.sendError("INVALID_MESSAGE", "Invalid message format")
		return
	}

	switch msg.Type {
	case MessageTypeSubscribe:
		c.handleSubscribe(msg.Data)

	case MessageTypeUnsubscribe:
		c.handleUnsubscribe(msg.Data)

	case MessageTypePing:
		c.handlePing()

	default:
		slog.Debug("Unknown message type",
			"type", msg.Type,
			"clientAddr", c.conn.RemoteAddr().String())
	}
}

// handleSubscribe processes a subscription request
func (c *Client) handleSubscribe(data json.RawMessage) {
	var subData SubscribeData
	if err := json.Unmarshal(data, &subData); err != nil {
		c.sendError("INVALID_SUBSCRIBE", "Invalid subscription data")
		return
	}

	// Validate resource type
	validTypes := map[string]bool{
		"all":        true,
		"instance":   true,
		"instances":  true,
		"rgd":        true,
		"rgds":       true,
		"violations": true, // OPA Gatekeeper compliance violations (enterprise)
	}

	if !validTypes[subData.ResourceType] {
		c.sendError("INVALID_RESOURCE_TYPE", "Invalid resource type: "+subData.ResourceType)
		return
	}

	// Add subscription
	c.subMu.Lock()
	c.subscriptions[subData.ResourceType] = true
	c.subMu.Unlock()

	slog.Debug("Client subscribed",
		"resourceType", subData.ResourceType,
		"clientAddr", c.conn.RemoteAddr().String())

	// Send confirmation
	confirmData := SubscriptionConfirmData{
		ResourceType: subData.ResourceType,
		Namespace:    subData.Namespace,
		Name:         subData.Name,
		Success:      true,
	}
	msg, _ := NewMessage(MessageTypeSubscribed, confirmData)
	c.send <- msg
}

// handleUnsubscribe processes an unsubscription request
func (c *Client) handleUnsubscribe(data json.RawMessage) {
	var subData SubscribeData
	if err := json.Unmarshal(data, &subData); err != nil {
		c.sendError("INVALID_UNSUBSCRIBE", "Invalid unsubscription data")
		return
	}

	// Remove subscription
	c.subMu.Lock()
	delete(c.subscriptions, subData.ResourceType)
	c.subMu.Unlock()

	slog.Debug("Client unsubscribed",
		"resourceType", subData.ResourceType,
		"clientAddr", c.conn.RemoteAddr().String())

	// Send confirmation
	confirmData := SubscriptionConfirmData{
		ResourceType: subData.ResourceType,
		Success:      true,
	}
	msg, _ := NewMessage(MessageTypeUnsubscribed, confirmData)
	c.send <- msg
}

// handlePing responds to a ping message
func (c *Client) handlePing() {
	msg, _ := NewMessage(MessageTypePong, nil)
	c.send <- msg
}

// sendError sends an error message to the client
func (c *Client) sendError(code, message string) {
	msg, _ := NewErrorMessage(code, message)
	select {
	case c.send <- msg:
	default:
		slog.Warn("Failed to send error message, buffer full",
			"code", code,
			"clientAddr", c.conn.RemoteAddr().String())
	}
}
