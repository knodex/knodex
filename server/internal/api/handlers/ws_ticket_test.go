// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/testutil"
)

// addUserContext adds a mock user context to the request.
func addUserContext(r *http.Request, userCtx *middleware.UserContext) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, userCtx)
	return r.WithContext(ctx)
}

func TestWSTicketHandler_CreateTicket(t *testing.T) {
	t.Parallel()
	_, redisClient := testutil.NewRedis(t)
	handler := NewWSTicketHandler(redisClient)

	t.Run("success - generates ticket with valid response", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ws/ticket", nil)
		req = addUserContext(req, &middleware.UserContext{
			UserID:      "user-123",
			Email:       "test@example.com",
			Groups:      []string{"group-a", "group-b"},
			CasbinRoles: []string{"role:serveradmin"},
			Projects:    []string{"proj-1", "proj-2"},
		})
		w := httptest.NewRecorder()

		handler.CreateTicket(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp wsTicketResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Ticket == "" {
			t.Fatal("ticket should not be empty")
		}
		if len(resp.Ticket) != 64 { // 32 bytes = 64 hex chars
			t.Fatalf("expected 64 char ticket, got %d", len(resp.Ticket))
		}
		if resp.ExpiresAt == "" {
			t.Fatal("expiresAt should not be empty")
		}

		// Verify expiresAt is valid ISO8601
		expiresAt, err := time.Parse(time.RFC3339, resp.ExpiresAt)
		if err != nil {
			t.Fatalf("expiresAt is not valid RFC3339: %v", err)
		}

		// Should be ~30 seconds in the future (allow 5s tolerance)
		diff := time.Until(expiresAt)
		if diff < 25*time.Second || diff > 35*time.Second {
			t.Fatalf("expiresAt should be ~30s in the future, got %v", diff)
		}

		// Verify ticket is stored in Redis
		key := wsTicketPrefix + resp.Ticket
		val, err := redisClient.Get(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("ticket not found in Redis: %v", err)
		}
		if val == "" {
			t.Fatal("ticket value in Redis should not be empty")
		}

		// Verify stored value contains user context
		userID, email, groups, casbinRoles, projects, decErr := decodeTicketValue(val)
		if decErr != nil {
			t.Fatalf("failed to decode ticket value: %v", decErr)
		}
		if userID != "user-123" {
			t.Fatalf("expected userID 'user-123', got %q", userID)
		}
		if email != "test@example.com" {
			t.Fatalf("expected email 'test@example.com', got %q", email)
		}
		if len(groups) != 2 || groups[0] != "group-a" || groups[1] != "group-b" {
			t.Fatalf("unexpected groups: %v", groups)
		}
		if len(casbinRoles) != 1 || casbinRoles[0] != "role:serveradmin" {
			t.Fatalf("unexpected casbinRoles: %v", casbinRoles)
		}
		if len(projects) != 2 || projects[0] != "proj-1" || projects[1] != "proj-2" {
			t.Fatalf("unexpected projects: %v", projects)
		}
	})

	t.Run("401 when no user context", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ws/ticket", nil)
		w := httptest.NewRecorder()

		handler.CreateTicket(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestExchangeTicket(t *testing.T) {
	t.Parallel()
	mr, redisClient := testutil.NewRedis(t)

	// NOTE: subtests not safe for t.Parallel — mr.FastForward in "expired ticket" subtest
	// affects all keys in the shared miniredis instance.
	t.Run("success - exchanges valid ticket", func(t *testing.T) {
		// Store a ticket manually
		ticket := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
		key := wsTicketPrefix + ticket
		value := encodeTicketValue("user-123", "test@example.com", []string{"group-a"}, []string{"role:serveradmin"}, []string{"proj-1"})
		redisClient.Set(context.Background(), key, value, wsTicketTTL)

		userID, email, groups, casbinRoles, projects, err := ExchangeTicket(context.Background(), redisClient, ticket)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if userID != "user-123" {
			t.Fatalf("expected userID 'user-123', got %q", userID)
		}
		if email != "test@example.com" {
			t.Fatalf("expected email 'test@example.com', got %q", email)
		}
		if len(groups) != 1 || groups[0] != "group-a" {
			t.Fatalf("unexpected groups: %v", groups)
		}
		if len(casbinRoles) != 1 || casbinRoles[0] != "role:serveradmin" {
			t.Fatalf("unexpected casbinRoles: %v", casbinRoles)
		}
		if len(projects) != 1 || projects[0] != "proj-1" {
			t.Fatalf("unexpected projects: %v", projects)
		}

		// Verify ticket is deleted (single-use)
		_, err = redisClient.Get(context.Background(), key).Result()
		if err != redis.Nil {
			t.Fatalf("ticket should have been deleted after exchange, err: %v", err)
		}
	})

	t.Run("error - ticket already used (single-use)", func(t *testing.T) {
		// First exchange succeeds (ticket created above was already consumed)
		ticket := "already_used_ticket_00000000000000000000000000000000000000000000"
		key := wsTicketPrefix + ticket
		value := encodeTicketValue("user-123", "test@example.com", nil, nil, nil)
		redisClient.Set(context.Background(), key, value, wsTicketTTL)

		// First exchange
		_, _, _, _, _, err := ExchangeTicket(context.Background(), redisClient, ticket)
		if err != nil {
			t.Fatalf("first exchange should succeed: %v", err)
		}

		// Second exchange should fail (single-use)
		_, _, _, _, _, err = ExchangeTicket(context.Background(), redisClient, ticket)
		if err == nil {
			t.Fatal("second exchange should fail (single-use)")
		}
		if !strings.Contains(err.Error(), "invalid, expired, or already-used") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error - expired ticket", func(t *testing.T) {
		ticket := "expired_ticket_000000000000000000000000000000000000000000000000"
		key := wsTicketPrefix + ticket
		value := encodeTicketValue("user-123", "test@example.com", nil, nil, nil)
		redisClient.Set(context.Background(), key, value, wsTicketTTL)

		// Fast-forward time past TTL
		mr.FastForward(31 * time.Second)

		_, _, _, _, _, err := ExchangeTicket(context.Background(), redisClient, ticket)
		if err == nil {
			t.Fatal("exchange of expired ticket should fail")
		}
		if !strings.Contains(err.Error(), "invalid, expired, or already-used") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error - nonexistent ticket", func(t *testing.T) {
		_, _, _, _, _, err := ExchangeTicket(context.Background(), redisClient, "nonexistent_ticket")
		if err == nil {
			t.Fatal("exchange of nonexistent ticket should fail")
		}
	})
}

func TestEncodeDecodeTicketValue(t *testing.T) {
	t.Parallel()
	t.Run("round-trip with all fields", func(t *testing.T) {
		t.Parallel()
		encoded := encodeTicketValue("uid", "email@test.com", []string{"g1", "g2"}, []string{"role:serveradmin"}, []string{"p1", "p2"})
		uid, email, groups, roles, projects, err := decodeTicketValue(encoded)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uid != "uid" || email != "email@test.com" {
			t.Fatalf("unexpected uid/email: %q/%q", uid, email)
		}
		if len(groups) != 2 || groups[0] != "g1" || groups[1] != "g2" {
			t.Fatalf("unexpected groups: %v", groups)
		}
		if len(roles) != 1 || roles[0] != "role:serveradmin" {
			t.Fatalf("unexpected roles: %v", roles)
		}
		if len(projects) != 2 || projects[0] != "p1" || projects[1] != "p2" {
			t.Fatalf("unexpected projects: %v", projects)
		}
	})

	t.Run("round-trip with empty slices", func(t *testing.T) {
		t.Parallel()
		encoded := encodeTicketValue("uid", "email@test.com", nil, nil, nil)
		uid, email, groups, roles, projects, err := decodeTicketValue(encoded)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uid != "uid" || email != "email@test.com" {
			t.Fatalf("unexpected uid/email: %q/%q", uid, email)
		}
		if groups != nil {
			t.Fatalf("expected nil groups, got: %v", groups)
		}
		if roles != nil {
			t.Fatalf("expected nil roles, got: %v", roles)
		}
		if projects != nil {
			t.Fatalf("expected nil projects, got: %v", projects)
		}
	})

	t.Run("decode malformed value returns error", func(t *testing.T) {
		t.Parallel()
		_, _, _, _, _, err := decodeTicketValue("invalid")
		if err == nil {
			t.Fatal("malformed value should return error")
		}
		if !strings.Contains(err.Error(), "malformed ticket value") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("decode empty userID returns error", func(t *testing.T) {
		t.Parallel()
		// 5 fields but empty userID
		malformed := "\x1femail\x1f\x1f\x1f"
		_, _, _, _, _, err := decodeTicketValue(malformed)
		if err == nil {
			t.Fatal("empty userID should return error")
		}
		if !strings.Contains(err.Error(), "empty userID") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestAuthCodeHandler_TokenExchange(t *testing.T) {
	t.Parallel()
	_, redisClient := testutil.NewRedis(t)

	// Mock auth service that validates "jwt-token-value" and returns test claims
	mockAuthSvc := &MockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			if token == "jwt-token-value" {
				return &auth.JWTClaims{
					UserID:      "test-user-id",
					Email:       "test@example.com",
					DisplayName: "Test User",
					ExpiresAt:   time.Now().Add(1 * time.Hour).Unix(),
					IssuedAt:    time.Now().Unix(),
				}, nil
			}
			return nil, fmt.Errorf("invalid token")
		},
	}
	handler := NewAuthCodeHandler(redisClient, mockAuthSvc, cookie.Config{Secure: false})

	t.Run("success - exchanges valid code and sets cookie", func(t *testing.T) {
		t.Parallel()
		// Store an auth code manually
		code, err := StoreAuthCode(context.Background(), redisClient, "jwt-token-value")
		if err != nil {
			t.Fatalf("failed to store auth code: %v", err)
		}

		body := `{"code":"` + code + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.TokenExchange(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify no-cache headers on auth response
		assertNoCacheHeaders(t, w)

		// Verify Set-Cookie header is present with knodex_session
		cookies := w.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "knodex_session" {
				sessionCookie = c
				break
			}
		}
		if sessionCookie == nil {
			t.Fatal("expected knodex_session cookie to be set")
		}
		if sessionCookie.Value != "jwt-token-value" {
			t.Fatalf("expected cookie value 'jwt-token-value', got %q", sessionCookie.Value)
		}
		if !sessionCookie.HttpOnly {
			t.Fatal("expected HttpOnly flag on session cookie")
		}

		// Verify response body contains user info (not raw token)
		var resp authCodeResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.User.Email != "test@example.com" {
			t.Fatalf("expected user email 'test@example.com', got %q", resp.User.Email)
		}
	})

	t.Run("401 - code already used (single-use)", func(t *testing.T) {
		t.Parallel()
		code, _ := StoreAuthCode(context.Background(), redisClient, "jwt-token-value")

		// First exchange
		body := `{"code":"` + code + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.TokenExchange(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("first exchange should return 200, got %d", w.Code)
		}

		// Second exchange should fail
		req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(body))
		w = httptest.NewRecorder()
		handler.TokenExchange(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("second exchange should return 401, got %d: %s", w.Code, w.Body.String())
		}
		assertNoCacheHeaders(t, w)
	})

	t.Run("400 - missing code", func(t *testing.T) {
		t.Parallel()
		body := `{}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.TokenExchange(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		assertNoCacheHeaders(t, w)
	})

	t.Run("400 - invalid body", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader("not json"))
		w := httptest.NewRecorder()
		handler.TokenExchange(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		assertNoCacheHeaders(t, w)
	})

	t.Run("401 - nonexistent code", func(t *testing.T) {
		t.Parallel()
		body := `{"code":"nonexistent-code-abc123"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(body))
		w := httptest.NewRecorder()
		handler.TokenExchange(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
		assertNoCacheHeaders(t, w)
	})

	t.Run("400 - oversized request body", func(t *testing.T) {
		t.Parallel()
		// MaxBytesReader limits body to 1024 bytes; send >1KB to trigger rejection
		oversized := `{"code":"` + strings.Repeat("x", 2000) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(oversized))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.TokenExchange(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for oversized body, got %d: %s", w.Code, w.Body.String())
		}
		assertNoCacheHeaders(t, w)
	})

	t.Run("500 - valid code but invalid JWT stored", func(t *testing.T) {
		t.Parallel()
		// Store a code whose value is NOT a valid JWT (validateTokenFunc will reject it)
		code, err := StoreAuthCode(context.Background(), redisClient, "corrupt-jwt-value")
		if err != nil {
			t.Fatalf("failed to store auth code: %v", err)
		}

		body := `{"code":"` + code + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-exchange", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.TokenExchange(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for invalid JWT, got %d: %s", w.Code, w.Body.String())
		}
		assertNoCacheHeaders(t, w)

		// Verify error message doesn't leak internal details
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["message"] != "failed to process authentication token" {
			t.Errorf("message = %q, want generic error", resp["message"])
		}
	})
}
