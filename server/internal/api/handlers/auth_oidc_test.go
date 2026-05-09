// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/auth"
	"github.com/knodex/knodex/server/internal/testutil"
)

// MockOIDCService is a mock OIDC service for testing handlers
type MockOIDCService struct {
	generateStateTokenFunc   func(ctx context.Context, providerName, redirectURL string) (string, string, string, error)
	validateStateTokenFunc   func(ctx context.Context, state string) (providerName, redirectURL string, err error)
	getAuthCodeURLFunc       func(providerName, state, nonce, verifier string) (string, error)
	exchangeCodeForTokenFunc func(ctx context.Context, providerName, code, nonce, verifier string) (*auth.LoginResponse, error)
	listProvidersFunc        func() []string
}

func (m *MockOIDCService) GenerateStateToken(ctx context.Context, providerName, redirectURL string) (string, string, string, error) {
	if m.generateStateTokenFunc != nil {
		return m.generateStateTokenFunc(ctx, providerName, redirectURL)
	}
	return "mock-state-token", "mock-nonce", "mock-verifier", nil
}

func (m *MockOIDCService) ValidateStateToken(ctx context.Context, state string) (providerName, redirectURL string, err error) {
	if m.validateStateTokenFunc != nil {
		return m.validateStateTokenFunc(ctx, state)
	}
	return "azuread", "", nil
}

func (m *MockOIDCService) GetAuthCodeURL(providerName, state, nonce, verifier string) (string, error) {
	if m.getAuthCodeURLFunc != nil {
		return m.getAuthCodeURLFunc(providerName, state, nonce, verifier)
	}
	return "https://provider.example.com/authorize?state=" + state + "&nonce=" + nonce, nil
}

func (m *MockOIDCService) ExchangeCodeForToken(ctx context.Context, providerName, code, nonce, verifier string) (*auth.LoginResponse, error) {
	if m.exchangeCodeForTokenFunc != nil {
		return m.exchangeCodeForTokenFunc(ctx, providerName, code, nonce, verifier)
	}
	return &auth.LoginResponse{
		Token:     "mock-jwt-token",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		User: auth.UserInfo{
			ID:          "user-123",
			Email:       "test@example.com",
			DisplayName: "Test User",
			CasbinRoles: []string{},
		},
	}, nil
}

func (m *MockOIDCService) ListProviders() []string {
	if m.listProvidersFunc != nil {
		return m.listProvidersFunc()
	}
	return []string{"google", "azure", "okta"}
}

func (m *MockOIDCService) ReloadProviders(_ context.Context, _ []auth.OIDCProviderConfig) error {
	return nil
}

// TestOIDCLogin tests the OIDC login endpoint
func TestOIDCLogin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		queryParams    string
		mockOIDC       *MockOIDCService
		wantStatusCode int
		wantRedirect   bool
		wantErrMsg     string
	}{
		{
			name:        "missing provider parameter",
			queryParams: "",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{"google", "azure"}
				},
			},
			wantStatusCode: http.StatusBadRequest,
			wantErrMsg:     "provider parameter is required",
		},
		{
			name:        "unknown provider",
			queryParams: "?provider=unknown",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{"google", "azure"}
				},
			},
			wantStatusCode: http.StatusBadRequest,
			wantErrMsg:     "unknown OIDC provider",
		},
		{
			name:        "state token generation fails",
			queryParams: "?provider=google",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{"google", "azure"}
				},
				generateStateTokenFunc: func(ctx context.Context, providerName, redirectURL string) (string, string, string, error) {
					return "", "", "", context.DeadlineExceeded
				},
			},
			wantStatusCode: http.StatusInternalServerError,
			wantErrMsg:     "failed to initiate OIDC login",
		},
		{
			name:        "get auth URL fails",
			queryParams: "?provider=google",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{"google", "azure"}
				},
				generateStateTokenFunc: func(ctx context.Context, providerName, redirectURL string) (string, string, string, error) {
					return "state-token", "test-nonce", "test-verifier", nil
				},
				getAuthCodeURLFunc: func(providerName, state, nonce, verifier string) (string, error) {
					return "", context.DeadlineExceeded
				},
			},
			wantStatusCode: http.StatusInternalServerError,
			wantErrMsg:     "failed to initiate OIDC login",
		},
		{
			name:        "successful login initiation",
			queryParams: "?provider=google",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{"google", "azure"}
				},
				generateStateTokenFunc: func(ctx context.Context, providerName, redirectURL string) (string, string, string, error) {
					return "state-token", "test-nonce", "test-verifier", nil
				},
				getAuthCodeURLFunc: func(providerName, state, nonce, verifier string) (string, error) {
					return "https://accounts.google.com/authorize?state=" + state + "&nonce=" + nonce, nil
				},
			},
			wantStatusCode: http.StatusFound,
			wantRedirect:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NewAuthHandler(nil, tt.mockOIDC)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/login"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			handler.OIDCLogin(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("Status code = %d, want %d", w.Code, tt.wantStatusCode)
			}

			if tt.wantRedirect {
				location := w.Header().Get("Location")
				if location == "" {
					t.Error("Expected redirect, but Location header is empty")
				}
				if !strings.Contains(location, "state=") {
					t.Error("Redirect URL should contain state parameter")
				}
			}

			if tt.wantErrMsg != "" {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if msg, ok := resp["message"].(string); !ok || !strings.Contains(msg, tt.wantErrMsg) {
					t.Errorf("Error message = %v, want to contain %s", msg, tt.wantErrMsg)
				}
			}
		})
	}
}

// TestOIDCCallback tests the OIDC callback endpoint
func TestOIDCCallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		queryParams    string
		mockOIDC       *MockOIDCService
		setupRedis     func(t *testing.T, handler *AuthHandler) // pre-populate nonce in Redis
		wantStatusCode int
		wantUserInfo   bool
		wantErrMsg     string
	}{
		{
			name:           "missing code parameter",
			queryParams:    "?state=test-state&provider=google",
			mockOIDC:       &MockOIDCService{},
			wantStatusCode: http.StatusBadRequest,
			wantErrMsg:     "code parameter is required",
		},
		{
			name:           "missing state parameter",
			queryParams:    "?code=test-code",
			mockOIDC:       &MockOIDCService{},
			wantStatusCode: http.StatusBadRequest,
			wantErrMsg:     "state parameter is required",
		},
		{
			name:        "invalid state token",
			queryParams: "?code=test-code&state=invalid-state",
			mockOIDC: &MockOIDCService{
				validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
					return "", "", context.DeadlineExceeded
				},
			},
			wantStatusCode: http.StatusUnauthorized,
			wantErrMsg:     "authentication failed",
		},
		{
			name:        "nonce not found in Redis",
			queryParams: "?code=test-code&state=valid-state",
			mockOIDC: &MockOIDCService{
				validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
					return "google", "", nil
				},
			},
			// No setupRedis — nonce key doesn't exist
			wantStatusCode: http.StatusUnauthorized,
			wantErrMsg:     "authentication failed",
		},
		{
			name:        "token exchange fails",
			queryParams: "?code=test-code&state=valid-state",
			mockOIDC: &MockOIDCService{
				validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
					return "google", "", nil
				},
				exchangeCodeForTokenFunc: func(ctx context.Context, providerName, code, nonce, verifier string) (*auth.LoginResponse, error) {
					return nil, context.DeadlineExceeded
				},
			},
			setupRedis: func(t *testing.T, handler *AuthHandler) {
				handler.redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.NoncePrefix, "valid-state"), "test-nonce", auth.NonceTTL)
				handler.redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.PKCEVerifierPrefix, "valid-state"), "test-verifier", auth.PKCEVerifierTTL)
			},
			wantStatusCode: http.StatusUnauthorized,
			wantErrMsg:     "authentication failed",
		},
		{
			name:        "successful callback",
			queryParams: "?code=test-code&state=valid-state",
			mockOIDC: &MockOIDCService{
				validateStateTokenFunc: func(ctx context.Context, state string) (providerName, redirectURL string, err error) {
					return "google", "", nil
				},
				exchangeCodeForTokenFunc: func(ctx context.Context, providerName, code, nonce, verifier string) (*auth.LoginResponse, error) {
					return &auth.LoginResponse{
						Token:     "jwt-token-123",
						ExpiresAt: time.Now().Add(1 * time.Hour),
						User: auth.UserInfo{
							ID:          "user-123",
							Email:       "test@example.com",
							DisplayName: "Test User",
							CasbinRoles: []string{},
						},
					}, nil
				},
			},
			setupRedis: func(t *testing.T, handler *AuthHandler) {
				handler.redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.NoncePrefix, "valid-state"), "test-nonce", auth.NonceTTL)
				handler.redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.PKCEVerifierPrefix, "valid-state"), "test-verifier", auth.PKCEVerifierTTL)
			},
			wantStatusCode: http.StatusOK,
			wantUserInfo:   true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, redisClient := testutil.NewRedis(t)
			handler := NewAuthHandler(nil, tt.mockOIDC)
			handler.SetRedisClient(redisClient)

			if tt.setupRedis != nil {
				tt.setupRedis(t, handler)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			handler.OIDCCallback(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("Status code = %d, want %d, body = %s", w.Code, tt.wantStatusCode, w.Body.String())
			}

			if tt.wantUserInfo {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				// Token should NOT be in response body (delivered via cookie)
				if _, hasToken := resp["token"]; hasToken {
					t.Error("Response body should NOT contain 'token' field")
				}
				user, ok := resp["user"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected 'user' field in response")
				}
				if user["email"] != "test@example.com" {
					t.Errorf("User email = %v, want test@example.com", user["email"])
				}
			}

			if tt.wantErrMsg != "" {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if msg, ok := resp["message"].(string); !ok || !strings.Contains(msg, tt.wantErrMsg) {
					t.Errorf("Error message = %v, want to contain %s", msg, tt.wantErrMsg)
				}
			}
		})
	}
}

// TestOIDCCallback_RedirectError_NoRawErrorLeakage verifies AC-6: when OIDC callback
// fails with a redirect URL, the error redirect uses a generic error code
// ("authentication_failed") and does NOT leak internal error details in the URL.
func TestOIDCCallback_RedirectError_NoRawErrorLeakage(t *testing.T) {
	t.Parallel()
	internalErrors := []struct {
		name          string
		failPoint     string // "state" or "exchange"
		internalError string
	}{
		{"state validation - OIDC provider error", "state", "OIDC provider connection refused: dial tcp 10.0.0.1:443: i/o timeout"},
		{"state validation - Redis error", "state", "redis: connection pool exhausted"},
		{"code exchange - token endpoint error", "exchange", "Post https://idp.example.com/token: context deadline exceeded"},
		{"code exchange - invalid grant", "exchange", "oauth2: cannot fetch token: 400 Bad Request\nResponse: {\"error\":\"invalid_grant\"}"},
		{"code exchange - JWKS fetch failure", "exchange", "failed to verify ID token: fetching keys: Get https://idp.example.com/.well-known/jwks.json: dial tcp: lookup idp.example.com: no such host"},
	}

	for _, tc := range internalErrors {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, redisClient := testutil.NewRedis(t)
			mockOIDC := &MockOIDCService{}
			redirectURL := "https://app.example.com/auth/callback"

			if tc.failPoint == "state" {
				mockOIDC.validateStateTokenFunc = func(ctx context.Context, state string) (string, string, error) {
					return "", redirectURL, errors.New(tc.internalError)
				}
			} else {
				mockOIDC.validateStateTokenFunc = func(ctx context.Context, state string) (string, string, error) {
					return "azuread", redirectURL, nil
				}
				mockOIDC.exchangeCodeForTokenFunc = func(ctx context.Context, providerName, code, nonce, verifier string) (*auth.LoginResponse, error) {
					return nil, errors.New(tc.internalError)
				}
				// Pre-populate nonce + PKCE verifier in Redis for exchange test cases
				redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.NoncePrefix, "test-state"), "test-nonce", auth.NonceTTL)
				redisClient.Set(context.Background(), fmt.Sprintf("%s%s", auth.PKCEVerifierPrefix, "test-state"), "test-verifier", auth.PKCEVerifierTTL)
			}

			handler := NewAuthHandler(nil, mockOIDC)
			handler.SetRedisClient(redisClient)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/callback?code=test-code&state=test-state", nil)
			w := httptest.NewRecorder()

			handler.OIDCCallback(w, req)

			// Should redirect (302)
			if w.Code != http.StatusFound {
				t.Fatalf("expected status %d, got %d", http.StatusFound, w.Code)
			}

			location := w.Header().Get("Location")
			if location == "" {
				t.Fatal("expected Location header in redirect")
			}

			// Verify the redirect URL uses generic error code
			if !strings.Contains(location, "error=authentication_failed") {
				t.Errorf("redirect should contain generic error code 'authentication_failed', got: %s", location)
			}

			// Verify NO internal error details are leaked in the URL
			if strings.Contains(location, "tcp") ||
				strings.Contains(location, "redis") ||
				strings.Contains(location, "oauth2") ||
				strings.Contains(location, "timeout") ||
				strings.Contains(location, "token") ||
				strings.Contains(location, "jwks") ||
				strings.Contains(location, "10.0.0.1") {
				t.Errorf("redirect URL leaks internal error details: %s", location)
			}
		})
	}
}

// TestListOIDCProviders tests the list providers endpoint
func TestListOIDCProviders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		mockOIDC       *MockOIDCService
		wantStatusCode int
		wantProviders  []string
	}{
		{
			name: "no providers",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{}
				},
			},
			wantStatusCode: http.StatusOK,
			wantProviders:  []string{},
		},
		{
			name: "multiple providers",
			mockOIDC: &MockOIDCService{
				listProvidersFunc: func() []string {
					return []string{"google", "azure", "okta"}
				},
			},
			wantStatusCode: http.StatusOK,
			wantProviders:  []string{"google", "azure", "okta"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NewAuthHandler(nil, tt.mockOIDC)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/providers", nil)
			w := httptest.NewRecorder()

			handler.ListOIDCProviders(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("Status code = %d, want %d", w.Code, tt.wantStatusCode)
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			providers, ok := resp["providers"].([]interface{})
			if !ok {
				t.Fatalf("Response does not contain providers array")
			}

			if len(providers) != len(tt.wantProviders) {
				t.Errorf("Providers count = %d, want %d", len(providers), len(tt.wantProviders))
			}

			providerMap := make(map[string]bool)
			for _, p := range providers {
				if pStr, ok := p.(string); ok {
					providerMap[pStr] = true
				}
			}

			for _, want := range tt.wantProviders {
				if !providerMap[want] {
					t.Errorf("Missing provider %s in response", want)
				}
			}
		})
	}
}
