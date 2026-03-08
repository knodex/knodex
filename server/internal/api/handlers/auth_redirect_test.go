// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fmt"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/auth"
)

func TestIsAllowedRedirectURL(t *testing.T) {
	t.Parallel()
	allowedOrigins := []string{
		"http://localhost:3000",
		"https://knodex.example.com",
	}

	tests := []struct {
		name    string
		url     string
		allowed bool
	}{
		// Allowed: empty redirect
		{"empty redirect", "", true},

		// Allowed: relative paths
		{"relative path", "/auth/callback", true},
		{"relative path with query", "/auth/callback?foo=bar", true},
		{"relative root path", "/", true},

		// Allowed: configured origins
		{"allowed origin http", "http://localhost:3000/auth/callback", true},
		{"allowed origin https", "https://knodex.example.com/auth/callback", true},
		{"allowed origin root", "https://knodex.example.com/", true},

		// Rejected: external/unknown origins
		{"external origin", "https://attacker.example.com/steal", false},
		{"external origin similar", "https://knodex.example.com.evil.com/steal", false},
		{"different port", "http://localhost:8080/callback", false},

		// Rejected: protocol-relative URLs
		{"protocol-relative", "//evil.com/steal", false},

		// Rejected: dangerous schemes
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,<script>alert(1)</script>", false},

		// Rejected: malformed URLs
		{"no scheme external", "evil.com/steal", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isAllowedRedirectURL(tt.url, allowedOrigins)
			if result != tt.allowed {
				t.Errorf("isAllowedRedirectURL(%q) = %v, want %v", tt.url, result, tt.allowed)
			}
		})
	}
}

func TestIsAllowedRedirectURL_NoOrigins(t *testing.T) {
	t.Parallel()
	// With no configured origins, only relative paths should be allowed
	tests := []struct {
		name    string
		url     string
		allowed bool
	}{
		{"relative path", "/auth/callback", true},
		{"absolute http", "http://localhost:3000/callback", false},
		{"empty", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isAllowedRedirectURL(tt.url, nil)
			if result != tt.allowed {
				t.Errorf("isAllowedRedirectURL(%q, nil) = %v, want %v", tt.url, result, tt.allowed)
			}
		})
	}
}

func TestOIDCLogin_RejectsOpenRedirect(t *testing.T) {
	t.Parallel()
	authService := &mockAuthService{}
	oidcService := &MockOIDCService{
		listProvidersFunc: func() []string {
			return []string{"azuread"}
		},
	}

	handler := NewAuthHandler(authService, oidcService)
	handler.SetAllowedRedirectOrigins([]string{"http://localhost:3000"})

	// Request with attacker redirect URL
	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/login?provider=azuread&redirect=https://attacker.example.com/steal", nil)
	w := httptest.NewRecorder()

	handler.OIDCLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for open redirect, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOIDCLogin_AllowsRelativeRedirect(t *testing.T) {
	t.Parallel()
	authService := &mockAuthService{}
	oidcService := &MockOIDCService{
		listProvidersFunc: func() []string {
			return []string{"azuread"}
		},
		generateStateTokenFunc: func(ctx context.Context, providerName, redirectURL string) (string, string, error) {
			return "mock-state", "mock-nonce", nil
		},
		getAuthCodeURLFunc: func(providerName, state, nonce string) (string, error) {
			return "https://provider.example.com/authorize?state=" + state + "&nonce=" + nonce, nil
		},
	}

	handler := NewAuthHandler(authService, oidcService)
	handler.SetAllowedRedirectOrigins(nil) // No explicit origins

	// Request with relative redirect URL (always allowed)
	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/login?provider=azuread&redirect=/auth/callback", nil)
	w := httptest.NewRecorder()

	handler.OIDCLogin(w, req)

	// Should redirect to provider (302), not 400
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 for relative redirect, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOIDCLogin_AllowsConfiguredOriginRedirect(t *testing.T) {
	t.Parallel()
	authService := &mockAuthService{}
	oidcService := &MockOIDCService{
		listProvidersFunc: func() []string {
			return []string{"azuread"}
		},
		generateStateTokenFunc: func(ctx context.Context, providerName, redirectURL string) (string, string, error) {
			return "mock-state", "mock-nonce", nil
		},
		getAuthCodeURLFunc: func(providerName, state, nonce string) (string, error) {
			return "https://provider.example.com/authorize?state=" + state + "&nonce=" + nonce, nil
		},
	}

	handler := NewAuthHandler(authService, oidcService)
	handler.SetAllowedRedirectOrigins([]string{"http://localhost:3000"})

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/login?provider=azuread&redirect=http://localhost:3000/auth/callback", nil)
	w := httptest.NewRecorder()

	handler.OIDCLogin(w, req)

	// Should redirect to provider (302)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 for configured origin redirect, got %d: %s", w.Code, w.Body.String())
	}
}

// TestOIDCCallback_OpaqueCodeRedirect verifies AC-4: OIDC callback stores JWT in Redis
// with an opaque code and redirects with ?code=<opaque> instead of ?token=<jwt>.
func TestOIDCCallback_OpaqueCodeRedirect(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock-jwt-token"

	oidcService := &MockOIDCService{
		validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
			return "azuread", "http://localhost:3000/auth/callback", nil
		},
		exchangeCodeForTokenFunc: func(ctx context.Context, providerName, code, nonce string) (*auth.LoginResponse, error) {
			return &auth.LoginResponse{
				Token:     jwtToken,
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:          "user-123",
					Email:       "test@example.com",
					DisplayName: "Test User",
					CasbinRoles: []string{"role:serveradmin"},
				},
			}, nil
		},
	}

	handler := NewAuthHandler(nil, oidcService)
	handler.SetRedisClient(redisClient)

	// Pre-populate nonce in Redis (simulating GenerateStateToken)
	redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.NoncePrefix, "valid-state"), "test-nonce", auth.NonceTTL)

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback?code=oidc-auth-code&state=valid-state", nil)
	w := httptest.NewRecorder()

	handler.OIDCCallback(w, req)

	// Should be a 302 redirect
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", w.Code, w.Body.String())
	}

	// Redirect URL must contain ?code= (NOT ?token=)
	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header in redirect")
	}
	if !strings.HasPrefix(location, "http://localhost:3000/auth/callback?code=") {
		t.Fatalf("redirect should use ?code= parameter, got: %s", location)
	}
	if strings.Contains(location, "token=") {
		t.Fatalf("redirect must NOT contain token= parameter (JWT in URL), got: %s", location)
	}
	if strings.Contains(location, jwtToken) {
		t.Fatalf("redirect must NOT contain the actual JWT, got: %s", location)
	}

	// Extract the opaque code from the redirect URL
	parts := strings.SplitN(location, "?code=", 2)
	if len(parts) != 2 || parts[1] == "" {
		t.Fatalf("could not extract code from redirect URL: %s", location)
	}
	opaqueCode := parts[1]

	// Verify the opaque code can be exchanged for the JWT via Redis
	key := authCodePrefix + opaqueCode
	storedValue, err := redisClient.Get(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("opaque code not found in Redis: %v", err)
	}
	if storedValue != jwtToken {
		t.Fatalf("expected JWT %q stored in Redis, got %q", jwtToken, storedValue)
	}
}

// TestOIDCCallback_SetsAuditIdentity verifies that the OIDC callback sets the
// audit login identity context signal with the authenticated user's ID and email.
// This is the handler-level integration test that ensures SetLoginIdentity is
// actually called during the OIDC flow (not just tested via mock handlers in middleware tests).
func TestOIDCCallback_SetsAuditIdentity(t *testing.T) {
	t.Parallel()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	oidcService := &MockOIDCService{
		validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
			return "azuread", "http://localhost:3000/auth/callback", nil
		},
		exchangeCodeForTokenFunc: func(ctx context.Context, providerName, code, nonce string) (*auth.LoginResponse, error) {
			return &auth.LoginResponse{
				Token:     "jwt-token-for-audit-test",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:    "user-oidc-audit",
					Email: "audit-user@corp.com",
				},
			}, nil
		},
	}

	handler := NewAuthHandler(nil, oidcService)
	handler.SetRedisClient(redisClient)

	// Pre-populate nonce in Redis (simulating GenerateStateToken)
	redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.NoncePrefix, "valid-state"), "test-nonce", auth.NonceTTL)

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback?code=test-code&state=valid-state", nil)

	// Inject audit identity signal (same as what the EE middleware does)
	req, readIdentity := audit.PrepareLoginIdentity(req)
	req, readResult := audit.PrepareLoginResult(req)

	w := httptest.NewRecorder()
	handler.OIDCCallback(w, req)

	// Verify the handler set the identity signal
	userID, email := readIdentity()
	if userID != "user-oidc-audit" {
		t.Errorf("audit identity userID = %q, want %q", userID, "user-oidc-audit")
	}
	if email != "audit-user@corp.com" {
		t.Errorf("audit identity email = %q, want %q", email, "audit-user@corp.com")
	}

	// Verify the handler set the login result signal
	if result := readResult(); result != "success" {
		t.Errorf("audit login result = %q, want %q", result, "success")
	}
}

// TestLocalLogin_SetsAuditIdentity verifies that the local login handler sets
// the audit login identity context signal with the authenticated user's ID and email.
func TestLocalLogin_SetsAuditIdentity(t *testing.T) {
	t.Parallel()
	mockAuth := &MockAuthService{
		authenticateLocalFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
			return &auth.LoginResponse{
				Token:     "jwt-token-local",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:    "user-local-admin",
					Email: "admin@local",
				},
			}, nil
		},
	}

	handler := NewAuthHandler(mockAuth, nil)

	body, _ := json.Marshal(auth.LocalLoginRequest{Username: "admin", Password: "secret"})
	req := httptest.NewRequest("POST", "/api/v1/auth/local/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Inject audit identity signal
	req, readIdentity := audit.PrepareLoginIdentity(req)

	w := httptest.NewRecorder()
	handler.LocalLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the handler set the identity signal
	userID, email := readIdentity()
	if userID != "user-local-admin" {
		t.Errorf("audit identity userID = %q, want %q", userID, "user-local-admin")
	}
	if email != "admin@local" {
		t.Errorf("audit identity email = %q, want %q", email, "admin@local")
	}
}

// TestOIDCCallback_NoRedis_FailsClosed verifies M-2: callback fails safely when Redis is nil.
// Without Redis, the handler cannot retrieve the OIDC nonce needed for ID token validation,
// so it must fail-closed rather than skip nonce validation or expose the JWT in the URL.
func TestOIDCCallback_NoRedis_FailsClosed(t *testing.T) {
	t.Parallel()
	oidcService := &MockOIDCService{
		validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
			return "azuread", "http://localhost:3000/auth/callback", nil
		},
	}

	// Use miniredis that's already stopped to simulate Redis unavailability.
	// MaxRetries: 0 and short DialTimeout prevent slow TCP retry loops.
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	addr := mr.Addr()
	mr.Close() // Close Redis before creating client to simulate unavailability
	redisClient := redis.NewClient(&redis.Options{
		Addr:        addr,
		MaxRetries:  0,
		DialTimeout: 50 * time.Millisecond,
	})
	defer redisClient.Close()

	handler := NewAuthHandler(nil, oidcService)
	handler.SetRedisClient(redisClient)

	req := httptest.NewRequest("GET", "/api/v1/auth/oidc/callback?code=test-code&state=valid-state", nil)
	w := httptest.NewRecorder()

	handler.OIDCCallback(w, req)

	// Should redirect with error (fail-closed) since nonce cannot be retrieved
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect with error when Redis unavailable, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "error=authentication_failed") {
		t.Fatalf("expected redirect with error=authentication_failed, got: %s", location)
	}
}
