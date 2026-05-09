// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/auth"
)

// MockAuthService is a mock implementation of auth.ServiceInterface for testing
type MockAuthService struct {
	authenticateLocalFunc func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error)
	validateTokenFunc     func(token string) (*auth.JWTClaims, error)
	revokeTokenFunc       func(ctx context.Context, jti string, remainingTTL time.Duration) error
	localLoginEnabled     bool
}

func (m *MockAuthService) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
	if m.authenticateLocalFunc != nil {
		return m.authenticateLocalFunc(ctx, username, password, sourceIP)
	}
	return nil, errors.New("not implemented")
}

func (m *MockAuthService) GenerateTokenForAccount(account *auth.Account, userID string) (string, time.Time, error) {
	return "", time.Time{}, errors.New("not implemented")
}

func (m *MockAuthService) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	return "", time.Time{}, errors.New("not implemented")
}

func (m *MockAuthService) ValidateToken(_ context.Context, tokenString string) (*auth.JWTClaims, error) {
	if m.validateTokenFunc != nil {
		return m.validateTokenFunc(tokenString)
	}
	return nil, errors.New("not implemented")
}

func (m *MockAuthService) RevokeToken(ctx context.Context, jti string, remainingTTL time.Duration) error {
	if m.revokeTokenFunc != nil {
		return m.revokeTokenFunc(ctx, jti, remainingTTL)
	}
	return nil
}

func (m *MockAuthService) IsLocalLoginEnabled() bool { return m.localLoginEnabled }

// assertNoCacheHeaders verifies all three no-cache headers are set on auth responses.
func assertNoCacheHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store, no-cache, must-revalidate")
	}
	if got := rec.Header().Get("Pragma"); got != "no-cache" {
		t.Errorf("Pragma = %q, want %q", got, "no-cache")
	}
	if got := rec.Header().Get("Expires"); got != "0" {
		t.Errorf("Expires = %q, want %q", got, "0")
	}
}

func TestLocalLogin(t *testing.T) {
	tests := []struct {
		name               string
		requestBody        interface{}
		mockAuthFunc       func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error)
		expectedStatusCode int
		checkResponse      func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful login",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "password123",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				return &auth.LoginResponse{
					Token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
					ExpiresAt: time.Now().Add(1 * time.Hour),
					User: auth.UserInfo{
						ID:          "user-local-admin",
						Email:       "admin@local",
						DisplayName: "Local Administrator",
						CasbinRoles: []string{"role:serveradmin"},
					},
				}, nil
			},
			expectedStatusCode: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				// Verify Set-Cookie header with knodex_session
				cookies := rec.Result().Cookies()
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
				if !sessionCookie.HttpOnly {
					t.Error("expected HttpOnly flag on session cookie")
				}
				if sessionCookie.SameSite != http.SameSiteStrictMode {
					t.Errorf("expected SameSite=Strict, got %v", sessionCookie.SameSite)
				}

				// Verify response body contains user info but NOT raw token
				var resp map[string]interface{}
				if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if _, hasToken := resp["token"]; hasToken {
					t.Error("response body should NOT contain 'token' field")
				}
				user, ok := resp["user"].(map[string]interface{})
				if !ok {
					t.Fatal("expected 'user' field in response")
				}
				if user["id"] != "user-local-admin" {
					t.Errorf("user ID = %v, want user-local-admin", user["id"])
				}
			},
		},
		{
			name: "invalid credentials",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "wrongpassword",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				return nil, errors.New("invalid credentials")
			},
			expectedStatusCode: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				if !bytes.Contains(rec.Body.Bytes(), []byte("invalid credentials")) {
					t.Error("expected 'invalid credentials' error message in response")
				}
			},
		},
		{
			name:        "malformed JSON",
			requestBody: `{"username": "admin", "password":`,
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with malformed JSON")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				if !bytes.Contains(rec.Body.Bytes(), []byte("invalid request body")) {
					t.Error("expected 'invalid request body' error message")
				}
			},
		},
		{
			name: "missing username",
			requestBody: auth.LocalLoginRequest{
				Username: "",
				Password: "password123",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with missing username")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				if !bytes.Contains(rec.Body.Bytes(), []byte("username and password are required")) {
					t.Error("expected 'username and password are required' error message")
				}
			},
		},
		{
			name: "missing password",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with missing password")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				if !bytes.Contains(rec.Body.Bytes(), []byte("username and password are required")) {
					t.Error("expected 'username and password are required' error message")
				}
			},
		},
		{
			name: "empty request body",
			requestBody: auth.LocalLoginRequest{
				Username: "",
				Password: "",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				t.Error("AuthenticateLocal should not be called with empty body")
				return nil, nil
			},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				if !bytes.Contains(rec.Body.Bytes(), []byte("username and password are required")) {
					t.Error("expected 'username and password are required' error message")
				}
			},
		},
		{
			name: "rate limited",
			requestBody: auth.LocalLoginRequest{
				Username: "admin",
				Password: "password123",
			},
			mockAuthFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
				return nil, &auth.ErrRateLimited{RetryAfter: 180 * time.Second}
			},
			expectedStatusCode: http.StatusTooManyRequests,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assertNoCacheHeaders(t, rec)
				if !bytes.Contains(rec.Body.Bytes(), []byte("RATE_LIMIT_EXCEEDED")) {
					t.Error("expected 'RATE_LIMIT_EXCEEDED' code in response")
				}
				if !bytes.Contains(rec.Body.Bytes(), []byte(`"retry_after":"180"`)) {
					t.Errorf("expected retry_after=180 in response details, got: %s", rec.Body.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock auth service
			mockAuthSvc := &MockAuthService{
				authenticateLocalFunc: tt.mockAuthFunc,
				localLoginEnabled:     true,
			}

			// Create handler with mock
			testHandler := NewAuthHandler(mockAuthSvc, nil)

			// Marshal request body
			var bodyBytes []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("failed to marshal request body: %v", err)
				}
			}

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rec := httptest.NewRecorder()

			// Call handler
			testHandler.LocalLogin(rec, req)

			// Check status code
			if rec.Code != tt.expectedStatusCode {
				t.Errorf("status code = %d, want %d", rec.Code, tt.expectedStatusCode)
			}

			// Run additional response checks
			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}
		})
	}
}

// TestLocalLogin_Disabled verifies that when localLoginEnabled is false the
// handler returns 403 Forbidden with the standardized auth-error body, emits
// no-cache headers, and never calls AuthenticateLocal.
func TestLocalLogin_Disabled(t *testing.T) {
	authCalls := 0
	mockAuthSvc := &MockAuthService{
		authenticateLocalFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
			authCalls++
			return nil, errors.New("should not be called")
		},
		localLoginEnabled: false,
	}

	testHandler := NewAuthHandler(mockAuthSvc, nil)

	reqBody := auth.LocalLoginRequest{Username: "admin", Password: "validpassword"}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testHandler.LocalLogin(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if authCalls != 0 {
		t.Errorf("AuthenticateLocal was called %d times, expected 0 (short-circuit)", authCalls)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("FORBIDDEN")) {
		t.Errorf("expected error code FORBIDDEN in body, got: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("local login is disabled")) {
		t.Errorf("expected 'local login is disabled' message, got: %s", rec.Body.String())
	}

	// Regression guard: no-cache headers MUST be present (WriteAuthError, not Forbidden).
	assertNoCacheHeaders(t, rec)
}

func TestLocalLogin_ContentType(t *testing.T) {
	mockAuthSvc := &MockAuthService{
		authenticateLocalFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
			return &auth.LoginResponse{
				Token:     "test-token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:          "user-test",
					Email:       "test@example.com",
					CasbinRoles: []string{},
				},
			}, nil
		},
		localLoginEnabled: true,
	}

	testHandler := NewAuthHandler(mockAuthSvc, nil)

	reqBody := auth.LocalLoginRequest{
		Username: "admin",
		Password: "password123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	testHandler.LocalLogin(rec, req)

	// Check Content-Type header
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %v, want application/json", contentType)
	}

	// Verify no-cache headers on auth response
	assertNoCacheHeaders(t, rec)
}

func TestLocalLogin_SourceIPPassedThrough(t *testing.T) {
	var capturedIP string
	mockAuthSvc := &MockAuthService{
		authenticateLocalFunc: func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
			capturedIP = sourceIP
			return &auth.LoginResponse{
				Token:     "test-token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				User: auth.UserInfo{
					ID:          "user-test",
					Email:       "test@example.com",
					CasbinRoles: []string{},
				},
			}, nil
		},
		localLoginEnabled: true,
	}

	testHandler := NewAuthHandler(mockAuthSvc, nil)

	reqBody := auth.LocalLoginRequest{
		Username: "admin",
		Password: "password123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/local/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.0.0.42")

	rec := httptest.NewRecorder()
	testHandler.LocalLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedIP != "10.0.0.42" {
		t.Errorf("sourceIP = %q, want %q", capturedIP, "10.0.0.42")
	}

	// Verify no-cache headers on auth response
	assertNoCacheHeaders(t, rec)
}

func TestLogout_RevokesToken(t *testing.T) {
	t.Parallel()

	t.Run("calls RevokeToken with correct jti and TTL", func(t *testing.T) {
		t.Parallel()
		var revokedJTI string
		var revokedTTL time.Duration

		mockSvc := &MockAuthService{
			revokeTokenFunc: func(ctx context.Context, jti string, remainingTTL time.Duration) error {
				revokedJTI = jti
				revokedTTL = remainingTTL
				return nil
			},
		}

		handler := NewAuthHandler(mockSvc, nil)

		tokenExp := time.Now().Add(30 * time.Minute)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

		// Set user context with JTI (normally done by middleware from validated token)
		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: tokenExp.Unix(),
			JTI:            "test-jti-for-logout",
		}
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		handler.Logout(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if revokedJTI != "test-jti-for-logout" {
			t.Errorf("RevokeToken jti = %q, want %q", revokedJTI, "test-jti-for-logout")
		}

		// TTL should be approximately 30 minutes
		if revokedTTL < 29*time.Minute || revokedTTL > 31*time.Minute {
			t.Errorf("RevokeToken TTL = %v, want ~30m", revokedTTL)
		}
	})

	t.Run("gracefully handles RevokeToken error", func(t *testing.T) {
		t.Parallel()
		mockSvc := &MockAuthService{
			revokeTokenFunc: func(ctx context.Context, jti string, remainingTTL time.Duration) error {
				return errors.New("redis connection refused")
			},
		}

		handler := NewAuthHandler(mockSvc, nil)

		tokenExp := time.Now().Add(30 * time.Minute)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: tokenExp.Unix(),
			JTI:            "jti-for-error-test",
		}
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		handler.Logout(rec, req)

		// Should still return 200 even if revocation fails
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 even when revocation fails, got %d", rec.Code)
		}
	})

	t.Run("revokes token using JTI from UserContext", func(t *testing.T) {
		t.Parallel()
		var revokedJTI string

		mockSvc := &MockAuthService{
			revokeTokenFunc: func(ctx context.Context, jti string, remainingTTL time.Duration) error {
				revokedJTI = jti
				return nil
			},
		}

		handler := NewAuthHandler(mockSvc, nil)

		tokenExp := time.Now().Add(30 * time.Minute)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: tokenExp.Unix(),
			JTI:            "cookie-jti-test",
		}
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		handler.Logout(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if revokedJTI != "cookie-jti-test" {
			t.Errorf("RevokeToken jti = %q, want %q", revokedJTI, "cookie-jti-test")
		}
	})

	t.Run("skips revocation for token without jti", func(t *testing.T) {
		t.Parallel()
		revokeCalled := false
		mockSvc := &MockAuthService{
			revokeTokenFunc: func(ctx context.Context, jti string, remainingTTL time.Duration) error {
				revokeCalled = true
				return nil
			},
		}

		handler := NewAuthHandler(mockSvc, nil)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

		// UserContext without JTI
		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: time.Now().Add(30 * time.Minute).Unix(),
		}
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		handler.Logout(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if revokeCalled {
			t.Error("RevokeToken should not be called for token without jti")
		}
	})
}

func TestListOIDCProviders_LocalLoginField(t *testing.T) {
	withProviders := &MockOIDCService{
		listProvidersFunc: func() []string { return []string{"knodex-cloud", "google"} },
	}

	tests := []struct {
		name                 string
		oidcService          OIDCServiceInterface
		localLoginEnabled    bool
		wantLocalLoginInResp bool
		wantProviderCount    int
	}{
		{
			name:                 "oidc nil, login disabled",
			oidcService:          nil,
			localLoginEnabled:    false,
			wantLocalLoginInResp: false,
			wantProviderCount:    0,
		},
		{
			name:                 "oidc nil, login enabled",
			oidcService:          nil,
			localLoginEnabled:    true,
			wantLocalLoginInResp: true,
			wantProviderCount:    0,
		},
		{
			name:                 "oidc with providers, login enabled",
			oidcService:          withProviders,
			localLoginEnabled:    true,
			wantLocalLoginInResp: true,
			wantProviderCount:    2,
		},
		{
			name:                 "oidc with providers, login disabled",
			oidcService:          withProviders,
			localLoginEnabled:    false,
			wantLocalLoginInResp: false,
			wantProviderCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockAuthService{localLoginEnabled: tt.localLoginEnabled}
			handler := NewAuthHandler(mockSvc, tt.oidcService)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/providers", nil)
			rec := httptest.NewRecorder()
			handler.ListOIDCProviders(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			got, ok := resp["localLoginEnabled"].(bool)
			if !ok {
				t.Fatalf("localLoginEnabled field missing or not bool in response: %v", resp)
			}
			if got != tt.wantLocalLoginInResp {
				t.Errorf("localLoginEnabled = %v, want %v", got, tt.wantLocalLoginInResp)
			}

			providersField, ok := resp["providers"].([]interface{})
			if !ok {
				t.Fatalf("providers field missing or wrong type in response: %v", resp)
			}
			if len(providersField) != tt.wantProviderCount {
				t.Errorf("providers count = %d, want %d", len(providersField), tt.wantProviderCount)
			}
		})
	}
}
