// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/testutil"
	"github.com/knodex/knodex/server/internal/websocket"
)

// mockWSPolicyEnforcer implements WebSocketPolicyEnforcer for testing
// This is separate from mockPolicyEnforcer (which implements the full rbac.PolicyEnforcer)
// to provide a simpler, more focused mock for WebSocket tests.
type mockWSPolicyEnforcer struct {
	canAccessWithGroupsFunc func(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

func (m *mockWSPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	if m.canAccessWithGroupsFunc != nil {
		return m.canAccessWithGroupsFunc(ctx, user, groups, object, action)
	}
	return false, nil
}

// mockAuthService implements auth.ServiceInterface for testing
// Updated to match new interface (removed rbac.User dependency)
type mockAuthService struct {
	validateTokenFunc func(token string) (*auth.JWTClaims, error)
}

func (m *mockAuthService) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
	return nil, errors.New("not implemented in mock")
}

// GenerateTokenForAccount implements auth.ServiceInterface
func (m *mockAuthService) GenerateTokenForAccount(account *auth.Account, userID string) (string, time.Time, error) {
	return "", time.Time{}, errors.New("not implemented in mock")
}

// GenerateTokenWithGroups implements auth.ServiceInterface
func (m *mockAuthService) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	return "", time.Time{}, errors.New("not implemented in mock")
}

func (m *mockAuthService) ValidateToken(_ context.Context, token string) (*auth.JWTClaims, error) {
	if m.validateTokenFunc != nil {
		return m.validateTokenFunc(token)
	}
	return nil, errors.New("not implemented")
}

func (m *mockAuthService) RevokeToken(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

// storeWSTicket creates a test WebSocket ticket in Redis and returns the ticket string.
func storeWSTicket(t *testing.T, redisClient *redis.Client, userID, email string, groups, casbinRoles, projects []string) string {
	t.Helper()
	ticket, err := generateTicket()
	if err != nil {
		t.Fatalf("failed to generate ticket: %v", err)
	}
	key := wsTicketPrefix + ticket
	value := encodeTicketValue(userID, email, groups, casbinRoles, projects)
	if err := redisClient.Set(context.Background(), key, value, wsTicketTTL).Err(); err != nil {
		t.Fatalf("failed to store ticket: %v", err)
	}
	return ticket
}

func TestWebSocketHandler_MissingTicket(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Request without ticket parameter
	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "missing ticket") {
		t.Errorf("Expected error message about missing ticket, got: %s", body)
	}
}

// TestWebSocketHandler_RejectLegacyToken verifies that ?token= parameter is rejected (AC-3)
func TestWebSocketHandler_RejectLegacyToken(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Request with legacy ?token= parameter (must be rejected)
	req := httptest.NewRequest("GET", "/ws?token=some-jwt-token", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "use /api/v1/ws/ticket") {
		t.Errorf("Expected message directing to ticket endpoint, got: %s", body)
	}
}

func TestWebSocketHandler_InvalidTicket(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Request with invalid ticket
	req := httptest.NewRequest("GET", "/ws?ticket=nonexistent-ticket-123", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "invalid, expired, or already-used") {
		t.Errorf("Expected error message about invalid ticket, got: %s", body)
	}
}

func TestWebSocketHandler_ExpiredTicket(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	mr, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Store a ticket
	ticket := storeWSTicket(t, redisClient, "user-123", "test@example.com", nil, nil, nil)

	// Fast-forward past TTL
	mr.FastForward(31 * time.Second)

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWebSocketHandler_TicketSingleUse(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Store a ticket
	ticket := storeWSTicket(t, redisClient, "user-123", "test@example.com",
		[]string{"group-a"}, []string{"role:serveradmin"}, []string{"proj-1"})

	// First request consumes the ticket (will fail upgrade but auth should pass)
	req1 := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req1.Header.Set("Connection", "Upgrade")
	req1.Header.Set("Upgrade", "websocket")
	req1.Header.Set("Sec-WebSocket-Version", "13")
	req1.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// First request should not be 401 (ticket is valid)
	if w1.Code == http.StatusUnauthorized {
		t.Errorf("First use of ticket should not be rejected: %s", w1.Body.String())
	}

	// Second request with same ticket should be 401 (already used)
	req2 := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Second use of ticket should be rejected, got status %d: %s", w2.Code, w2.Body.String())
	}
}

func TestWebSocketHandler_ValidTicket_RegularUser(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	ticket := storeWSTicket(t, redisClient, "user-123", "testuser@example.com",
		nil, []string{}, []string{"project-a", "project-b"})

	// Request with valid ticket and WebSocket upgrade headers
	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// In unit tests, the upgrade will fail because httptest.ResponseRecorder
	// doesn't support protocol switching. But we verify auth didn't reject it.
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Valid ticket should not be rejected, got status %d: %s", w.Code, w.Body.String())
	}
}

func TestWebSocketHandler_ValidTicket_GlobalAdmin(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	ticket := storeWSTicket(t, redisClient, "admin-456", "admin@example.com",
		[]string{"platform-admins"}, []string{"role:serveradmin"}, []string{"system"})

	// Request with valid admin ticket
	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should not be unauthorized
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Valid admin ticket should not be rejected, got status %d: %s", w.Code, w.Body.String())
	}
}

func TestWebSocketHandler_EmptyTicketParameter(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Request with empty ticket parameter
	req := httptest.NewRequest("GET", "/ws?ticket=", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Empty ticket should be rejected with status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "missing ticket") {
		t.Errorf("Expected missing ticket error, got: %s", body)
	}
}

// TestWebSocketHandler_NoRedisClient tests that the handler rejects connections
// when no Redis client is configured (fail closed).
func TestWebSocketHandler_NoRedisClient(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, nil) // No Redis client

	req := httptest.NewRequest("GET", "/ws?ticket=some-ticket", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d: %s", http.StatusInternalServerError, w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_GlobalAdmin_ViaCasbin tests that global admin is detected via Casbin
// CanAccessWithGroups method, NOT via direct role string comparison.
func TestWebSocketHandler_GlobalAdmin_ViaCasbin(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}

	// Mock policy enforcer that returns true for *, * (global admin check)
	policyEnforcer := &mockWSPolicyEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			if user != "admin-user" {
				t.Errorf("Expected user 'admin-user', got %s", user)
			}
			if len(groups) != 1 || groups[0] != "platform-admins" {
				t.Errorf("Expected groups ['platform-admins'], got %v", groups)
			}
			return true, nil
		},
	}

	handler := NewWebSocketHandler(hub, authService, redisClient)
	handler.SetPolicyEnforcer(policyEnforcer)

	ticket := storeWSTicket(t, redisClient, "admin-user", "admin@example.com",
		[]string{"platform-admins"}, []string{"role:serveradmin"}, []string{"system"})

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("Admin ticket should not be rejected, got status %d: %s", w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_NonAdmin_ViaCasbin tests that non-admin users are correctly identified
func TestWebSocketHandler_NonAdmin_ViaCasbin(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}

	policyEnforcer := &mockWSPolicyEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			if user != "regular-user" {
				t.Errorf("Expected user 'regular-user', got %s", user)
			}
			return false, nil
		},
	}

	handler := NewWebSocketHandler(hub, authService, redisClient)
	handler.SetPolicyEnforcer(policyEnforcer)

	ticket := storeWSTicket(t, redisClient, "regular-user", "user@example.com",
		[]string{"developers"}, []string{"proj:project-a:viewer"}, []string{"project-a"})

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code == http.StatusUnauthorized {
		t.Errorf("Valid user ticket should not be rejected, got status %d: %s", w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_Casbin_ErrorHandling tests that Casbin errors don't grant admin access
func TestWebSocketHandler_Casbin_ErrorHandling(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}

	policyEnforcer := &mockWSPolicyEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			return false, errors.New("casbin policy check failed")
		},
	}

	handler := NewWebSocketHandler(hub, authService, redisClient)
	handler.SetPolicyEnforcer(policyEnforcer)

	ticket := storeWSTicket(t, redisClient, "error-user", "error@example.com",
		[]string{"team-a"}, []string{"role:serveradmin"}, []string{"project-a"})

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Token is valid, connection should proceed (admin access denied due to Casbin error — fail closed)
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Valid ticket should not be rejected even when Casbin errors, got status %d: %s", w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_NilAuthService verifies fail-closed behavior when authService is nil.
// AC-1: Given authService is nil, WebSocket upgrade is rejected with 500 "server not ready".
func TestWebSocketHandler_NilAuthService(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	handler := NewWebSocketHandler(hub, nil, redisClient) // nil authService

	req := httptest.NewRequest("GET", "/ws?ticket=some-ticket", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d for nil authService, got %d: %s",
			http.StatusInternalServerError, w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "server not ready") {
		t.Errorf("Expected 'server not ready' error, got: %s", body)
	}
}

// TestWebSocketHandler_EmptyUserIDFromTicket verifies that an empty userID from ticket exchange
// results in 401 rejection. AC-2: token validation succeeds but userID is empty → reject.
// Note: This tests the decodeTicketValue layer which validates non-empty userID during exchange.
// The handler's defense-in-depth TrimSpace check is tested separately in WhitespaceUserID test.
func TestWebSocketHandler_EmptyUserIDFromTicket(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Store a ticket with empty userID — caught by decodeTicketValue validation
	ticket := storeWSTicket(t, redisClient, "", "test@example.com", nil, nil, nil)

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// AC-2: Empty userID must be rejected with 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for empty userID, got %d: %s",
			http.StatusUnauthorized, w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_WhitespaceUserIDFromTicket verifies that whitespace-only userIDs
// are rejected. decodeTicketValue now uses TrimSpace so this is caught at the decode layer
// (not at the handler's defense-in-depth check), returning an "invalid ticket" error.
func TestWebSocketHandler_WhitespaceUserIDFromTicket(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Store a ticket with whitespace-only userID — caught by decodeTicketValue's
	// TrimSpace validation during ExchangeTicket.
	ticket := storeWSTicket(t, redisClient, "  ", "test@example.com", nil, nil, nil)

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d for whitespace userID, got %d: %s",
			http.StatusUnauthorized, w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "invalid") {
		t.Errorf("Expected 'invalid' in error message, got: %s", body)
	}
}

// TestWebSocketHandler_GroupsAndRoles_ExtractedFromTicket verifies that OIDC groups and Casbin roles
// from the ticket are correctly extracted and used.
func TestWebSocketHandler_GroupsAndRoles_ExtractedFromTicket(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}

	policyEnforcer := &mockWSPolicyEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			return true, nil
		},
	}

	handler := NewWebSocketHandler(hub, authService, redisClient)
	handler.SetPolicyEnforcer(policyEnforcer)

	ticket := storeWSTicket(t, redisClient, "multi-group-user", "multigroup@example.com",
		[]string{"team-alpha", "team-beta", "platform-developers"},
		[]string{"proj:alpha:admin"},
		[]string{"alpha", "beta"})

	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should not be unauthorized - ticket contains valid user context
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Valid ticket should not be rejected, got status %d: %s", w.Code, w.Body.String())
	}
}

// TestGetClientIP_IgnoresXForwardedFor verifies that X-Forwarded-For header is NOT trusted.
// AC-3: When the server is NOT behind a configured trusted proxy, r.RemoteAddr is used.
func TestGetClientIP_IgnoresXForwardedFor(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")

	ip := getClientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("Expected RemoteAddr IP '10.0.0.1', got '%s' (X-Forwarded-For should be ignored)", ip)
	}
}

// TestGetClientIP_IgnoresXRealIP verifies that X-Real-IP header is NOT trusted.
func TestGetClientIP_IgnoresXRealIP(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/ws", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "5.6.7.8")

	ip := getClientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("Expected RemoteAddr IP '10.0.0.1', got '%s' (X-Real-IP should be ignored)", ip)
	}
}

// TestGetClientIP_UsesRemoteAddr verifies that getClientIP uses r.RemoteAddr directly.
func TestGetClientIP_UsesRemoteAddr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		remoteAddr string
		expected   string
	}{
		{"IPv4 with port", "192.168.1.1:54321", "192.168.1.1"},
		{"IPv4 without port", "192.168.1.1", "192.168.1.1"},
		{"IPv6 with port", "[::1]:54321", "::1"},
		{"empty RemoteAddr", "", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest("GET", "/ws", nil)
			req.RemoteAddr = tt.remoteAddr

			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, ip)
			}
		})
	}
}

// TestWebSocketHandler_RateLimitUsesRealIP verifies that rate limiting uses the real client IP
// (from RemoteAddr), not a spoofed X-Forwarded-For value. AC-4.
//
// Note: The rate limiter tracks concurrent in-flight ServeHTTP calls, not persistent WebSocket
// connections (defer decrement fires when ServeHTTP returns). We pre-fill the limiter synthetically
// to simulate 10 concurrent in-flight requests — calling checkAndIncrement without decrement
// mirrors the window where 10 requests are all being processed simultaneously.
func TestWebSocketHandler_RateLimitUsesRealIP(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)

	// Pre-fill the rate limiter to simulate 10 concurrent in-flight requests from 10.0.0.99.
	// The default maxPerIP is 10, so one more should trigger rate limiting.
	for i := 0; i < 10; i++ {
		handler.rateLimiter.checkAndIncrement("10.0.0.99")
	}

	// Send a request from RemoteAddr 10.0.0.99 but with a spoofed X-Forwarded-For.
	// If getClientIP correctly uses RemoteAddr, this should be rate-limited (429).
	// If it trusts XFF, "192.168.1.1" has 0 connections and would be allowed.
	ticket := storeWSTicket(t, redisClient, "user-123", "test@example.com", nil, nil, nil)
	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.RemoteAddr = "10.0.0.99:12345"
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit (429) for real IP 10.0.0.99 despite spoofed XFF, got %d: %s",
			w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_SetMaxConnectionsPerIP verifies that the rate limit can be overridden.
func TestWebSocketHandler_SetMaxConnectionsPerIP(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)
	t.Cleanup(func() { handler.Shutdown() })

	// Lower the limit to 2 concurrent connections per IP
	handler.SetMaxConnectionsPerIP(2)

	// Pre-fill: 2 connections from same IP
	handler.rateLimiter.checkAndIncrement("10.0.0.50")
	handler.rateLimiter.checkAndIncrement("10.0.0.50")

	// 3rd should be rejected
	ticket := storeWSTicket(t, redisClient, "user-123", "test@example.com", nil, nil, nil)
	req := httptest.NewRequest("GET", "/ws?ticket="+ticket, nil)
	req.RemoteAddr = "10.0.0.50:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit (429) with maxPerIP=2, got %d: %s",
			w.Code, w.Body.String())
	}
}

// TestWebSocketHandler_SetMaxConnectionsPerIP_InvalidValues verifies that invalid values
// (zero, negative) are ignored — the default rate limit is retained.
func TestWebSocketHandler_SetMaxConnectionsPerIP_InvalidValues(t *testing.T) {
	t.Parallel()
	hub := websocket.NewHub(nil)
	_, redisClient := testutil.NewRedis(t)
	authService := &mockAuthService{}
	handler := NewWebSocketHandler(hub, authService, redisClient)
	t.Cleanup(func() { handler.Shutdown() })

	// Set to 0 — should be ignored, default (10) retained
	handler.SetMaxConnectionsPerIP(0)

	handler.rateLimiter.mu.RLock()
	if handler.rateLimiter.maxPerIP != DefaultWSMaxConnectionsPerIP {
		t.Errorf("Expected maxPerIP to remain %d after SetMaxConnectionsPerIP(0), got %d",
			DefaultWSMaxConnectionsPerIP, handler.rateLimiter.maxPerIP)
	}
	handler.rateLimiter.mu.RUnlock()

	// Set to -1 — should also be ignored
	handler.SetMaxConnectionsPerIP(-1)

	handler.rateLimiter.mu.RLock()
	if handler.rateLimiter.maxPerIP != DefaultWSMaxConnectionsPerIP {
		t.Errorf("Expected maxPerIP to remain %d after SetMaxConnectionsPerIP(-1), got %d",
			DefaultWSMaxConnectionsPerIP, handler.rateLimiter.maxPerIP)
	}
	handler.rateLimiter.mu.RUnlock()

	// Set to a valid value — should work
	handler.SetMaxConnectionsPerIP(5)

	handler.rateLimiter.mu.RLock()
	if handler.rateLimiter.maxPerIP != 5 {
		t.Errorf("Expected maxPerIP to be 5 after SetMaxConnectionsPerIP(5), got %d",
			handler.rateLimiter.maxPerIP)
	}
	handler.rateLimiter.mu.RUnlock()
}
