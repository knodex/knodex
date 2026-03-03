package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/auth"
	ws "github.com/provops-org/knodex/server/internal/websocket"
)

const (
	// wsRateLimitCleanupInterval is how often to remove stale IP connection trackers.
	// 1min balances memory cleanup vs CPU overhead.
	wsRateLimitCleanupInterval = 1 * time.Minute

	// wsTrackerStaleThreshold is how long an idle tracker lives before cleanup.
	// 5min allows for brief disconnects without losing rate limit state.
	wsTrackerStaleThreshold = 5 * time.Minute

	// DefaultWSMaxConnectionsPerIP is the default max concurrent WebSocket upgrade
	// attempts allowed from a single IP address. Override with SetMaxConnectionsPerIP.
	DefaultWSMaxConnectionsPerIP = 10
)

// WebSocketPolicyEnforcer defines the interface for checking permissions in WebSocket handler
// This interface follows the ArgoCD-aligned pattern where permissions are always
// checked via Casbin, never by direct role string comparison.
type WebSocketPolicyEnforcer interface {
	// CanAccessWithGroups checks if user/groups/server-side roles can perform action on object.
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

// WebSocketHandler handles WebSocket connections using a Hub
type WebSocketHandler struct {
	hub            *ws.Hub
	authService    auth.ServiceInterface // Readiness sentinel: nil means server not ready; auth via redisClient tickets
	policyEnforcer WebSocketPolicyEnforcer
	upgrader       websocket.Upgrader
	rateLimiter    *connectionRateLimiter
	redisClient    *redis.Client // For ticket-based authentication
}

// connectionRateLimiter implements per-IP rate limiting for WebSocket connections
type connectionRateLimiter struct {
	mu          sync.RWMutex
	connections map[string]*connectionTracker
	maxPerIP    int
	cleanupDone chan struct{}
	stopOnce    sync.Once
}

// connectionTracker tracks connection attempts for rate limiting
type connectionTracker struct {
	count      int
	lastAccess time.Time
	mu         sync.Mutex
}

// newConnectionRateLimiter creates a new rate limiter
func newConnectionRateLimiter(maxPerIP int) *connectionRateLimiter {
	limiter := &connectionRateLimiter{
		connections: make(map[string]*connectionTracker),
		maxPerIP:    maxPerIP,
		cleanupDone: make(chan struct{}),
	}
	go limiter.cleanup()
	return limiter
}

// stop terminates the cleanup goroutine (safe to call multiple times)
func (r *connectionRateLimiter) stop() {
	r.stopOnce.Do(func() {
		close(r.cleanupDone)
	})
}

// cleanup removes stale connection trackers
func (r *connectionRateLimiter) cleanup() {
	ticker := time.NewTicker(wsRateLimitCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for ip, tracker := range r.connections {
				tracker.mu.Lock()
				if now.Sub(tracker.lastAccess) > wsTrackerStaleThreshold && tracker.count == 0 {
					delete(r.connections, ip)
				}
				tracker.mu.Unlock()
			}
			r.mu.Unlock()
		case <-r.cleanupDone:
			return
		}
	}
}

// checkAndIncrement checks if connection is allowed and increments counter
func (r *connectionRateLimiter) checkAndIncrement(ip string) bool {
	r.mu.Lock()
	tracker, exists := r.connections[ip]
	if !exists {
		tracker = &connectionTracker{
			count:      0,
			lastAccess: time.Now(),
		}
		r.connections[ip] = tracker
	}
	maxPerIP := r.maxPerIP // capture while holding lock to avoid data race with SetMaxConnectionsPerIP
	r.mu.Unlock()

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	if tracker.count >= maxPerIP {
		slog.Warn("Connection rate limit exceeded",
			"ip", ip,
			"count", tracker.count,
			"maxPerIP", maxPerIP)
		return false
	}

	tracker.count++
	tracker.lastAccess = time.Now()
	return true
}

// decrement decrements the connection counter for an IP
func (r *connectionRateLimiter) decrement(ip string) {
	r.mu.RLock()
	tracker, exists := r.connections[ip]
	r.mu.RUnlock()

	if !exists {
		return
	}

	tracker.mu.Lock()
	if tracker.count > 0 {
		tracker.count--
	}
	tracker.lastAccess = time.Now()
	tracker.mu.Unlock()
}

// NewWebSocketHandler creates a new WebSocketHandler with same-origin validation and rate limiting.
// Since the frontend is embedded in the same binary, WebSocket connections must come from the same origin.
// The redisClient parameter enables ticket-based authentication (recommended). Pass nil to fall back
// to direct JWT token validation (legacy, not recommended — tokens leak into URL logs).
func NewWebSocketHandler(hub *ws.Hub, authService auth.ServiceInterface, redisClient *redis.Client) *WebSocketHandler {
	handler := &WebSocketHandler{
		hub:         hub,
		authService: authService,
		redisClient: redisClient,
		rateLimiter: newConnectionRateLimiter(DefaultWSMaxConnectionsPerIP),
	}

	// Configure upgrader with same-origin validation
	handler.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")

			// No origin header — allow (same-origin requests from non-browser clients)
			if origin == "" {
				return true
			}

			// Same-origin check: the Origin header host must match the Host header
			host := r.Host
			if host == "" {
				host = r.URL.Host
			}

			// Extract host from origin URL (e.g., "http://localhost:8080" -> "localhost:8080")
			originHost := origin
			if idx := strings.Index(origin, "://"); idx != -1 {
				originHost = origin[idx+3:]
			}
			// Remove trailing slash if present
			originHost = strings.TrimRight(originHost, "/")

			if originHost == host {
				return true
			}

			slog.Warn("WebSocket: rejecting cross-origin request",
				"origin", origin,
				"host", host)
			return false
		},
	}

	return handler
}

// Shutdown stops the WebSocket handler's background goroutines.
// This must be called during graceful shutdown to prevent goroutine leaks.
func (h *WebSocketHandler) Shutdown() {
	if h.rateLimiter != nil {
		h.rateLimiter.stop()
	}
}

// SetPolicyEnforcer sets the policy enforcer for role checks
func (h *WebSocketHandler) SetPolicyEnforcer(enforcer WebSocketPolicyEnforcer) {
	h.policyEnforcer = enforcer
}

// SetMaxConnectionsPerIP overrides the default per-IP rate limit for WebSocket upgrades.
// Must be called before serving requests. Default is DefaultWSMaxConnectionsPerIP (10).
// Values <= 0 are ignored (default is retained).
func (h *WebSocketHandler) SetMaxConnectionsPerIP(max int) {
	if max <= 0 {
		return
	}
	h.rateLimiter.mu.Lock()
	defer h.rateLimiter.mu.Unlock()
	h.rateLimiter.maxPerIP = max
}

// getClientIP extracts the client IP from r.RemoteAddr using net.SplitHostPort.
// X-Forwarded-For and X-Real-IP headers are intentionally NOT trusted because
// they can be spoofed by any client to bypass per-IP rate limiting.
// In Kubernetes deployments, r.RemoteAddr reflects the actual source IP as seen
// by the pod, which is the correct value for rate limiting.
//
// NOTE: For proxy-aware IP extraction with trusted proxy validation, see
// middleware.getClientIP in internal/api/middleware/ratelimit.go instead.
func getClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// SplitHostPort failed (empty, no port, or malformed — unexpected from Go HTTP server)
		return r.RemoteAddr
	}
	return host
}

// policyEnforcerAdapter adapts WebSocketPolicyEnforcer to ws.ClientPolicyEnforcer
// This allows the handler's policy enforcer to be used by websocket clients
type policyEnforcerAdapter struct {
	enforcer WebSocketPolicyEnforcer
}

func (a *policyEnforcerAdapter) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return a.enforcer.CanAccessWithGroups(ctx, user, groups, object, action)
}

// ServeHTTP handles WebSocket upgrade requests with rate limiting
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)

	// Fail-closed: reject if auth service is not initialized (startup race, misconfiguration).
	// Note: this check runs before rate limiting so not-ready responses bypass per-IP limits.
	if h.authService == nil {
		slog.Error("WebSocket connection rejected: auth service not initialized",
			"clientIP", clientIP,
			"remoteAddr", r.RemoteAddr)
		response.InternalError(w, "server not ready")
		return
	}

	// Check rate limiting
	if !h.rateLimiter.checkAndIncrement(clientIP) {
		slog.Warn("WebSocket connection rejected: rate limit exceeded",
			"clientIP", clientIP,
			"remoteAddr", r.RemoteAddr)
		response.WriteError(w, http.StatusTooManyRequests, response.ErrCodeRateLimit, "too many connections", nil)
		return
	}

	// Decrement when ServeHTTP returns (after goroutine launch, NOT when the
	// WebSocket connection closes). This means the rate limiter only caps concurrent
	// upgrade handshakes, not active connections. To track active connections,
	// decrement would need to move into ReadPump's cleanup path.
	defer h.rateLimiter.decrement(clientIP)

	// Reject legacy ?token= parameter (JWT tokens must not appear in URLs)
	if r.URL.Query().Get("token") != "" {
		slog.Warn("WebSocket connection rejected: legacy ?token= parameter used",
			"clientIP", clientIP,
			"remoteAddr", r.RemoteAddr)
		response.Unauthorized(w, "use /api/v1/ws/ticket to obtain a WebSocket ticket")
		return
	}

	// Extract ticket from query parameter
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		slog.Warn("WebSocket connection rejected: missing ticket",
			"clientIP", clientIP,
			"remoteAddr", r.RemoteAddr)
		response.Unauthorized(w, "missing ticket parameter")
		return
	}

	// Exchange ticket for user context via Redis (single-use, atomic)
	var userID string
	var projects []string
	var groups []string

	if h.redisClient != nil {
		uid, _, grps, _, projs, err := ExchangeTicket(r.Context(), h.redisClient, ticket)
		if err != nil {
			slog.Warn("WebSocket connection rejected: invalid ticket",
				"error", err,
				"clientIP", clientIP,
				"remoteAddr", r.RemoteAddr)
			response.Unauthorized(w, "invalid, expired, or already-used ticket")
			return
		}
		userID = uid
		projects = projs
		groups = grps

		// Defense-in-depth: reject if userID is empty or whitespace-only.
		// decodeTicketValue validates non-empty userID, but this handler check
		// catches whitespace-only values and guards against future decoder changes.
		if strings.TrimSpace(userID) == "" {
			slog.Warn("WebSocket connection rejected: empty userID from ticket",
				"clientIP", clientIP,
				"remoteAddr", r.RemoteAddr)
			response.Unauthorized(w, "invalid ticket: missing user identity")
			return
		}
	} else {
		// Fallback: no Redis client configured — reject connection
		slog.Error("WebSocket connection rejected: no Redis client for ticket exchange",
			"clientIP", clientIP)
		response.InternalError(w, "WebSocket ticket exchange not configured")
		return
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection",
			"error", err,
			"clientIP", clientIP,
			"remoteAddr", r.RemoteAddr)
		return
	}

	// Create client with user context (ArgoCD-aligned: pass groups/roles for Casbin checks)
	// NOTE: We pass the policy enforcer to the client so it can check permissions via Casbin
	// This follows the ArgoCD pattern: "Only use Casbin Enforce()", never boolean flags
	client := ws.NewClient(h.hub, conn)

	// Wrap policy enforcer for client use (adapts interface)
	var clientPolicyEnforcer ws.ClientPolicyEnforcer
	if h.policyEnforcer != nil {
		clientPolicyEnforcer = &policyEnforcerAdapter{enforcer: h.policyEnforcer}
	}

	client.SetUserContext(userID, projects, groups, clientPolicyEnforcer)

	// Register client with hub
	h.hub.Register() <- client

	slog.Info("WebSocket connection established",
		"clientIP", clientIP,
		"remoteAddr", r.RemoteAddr,
		"userID", userID,
		"projectCount", len(projects),
		"groupCount", len(groups))

	// Start client pumps in goroutines
	go client.WritePump()
	go client.ReadPump()
}

// GetMetrics returns WebSocket hub metrics
func (h *WebSocketHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.hub.GetMetrics()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		slog.Error("failed to encode WebSocket metrics", "error", err)
	}
}
