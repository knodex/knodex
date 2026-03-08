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

	"github.com/golang-jwt/jwt/v5"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/auth"
)

// MockAuthService is a mock implementation of auth.ServiceInterface for testing
type MockAuthService struct {
	authenticateLocalFunc func(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error)
	validateTokenFunc     func(token string) (*auth.JWTClaims, error)
	revokeTokenFunc       func(ctx context.Context, jti string, remainingTTL time.Duration) error
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

// createTestJWT creates a minimal JWT string with the given jti and exp for logout testing
func createTestJWT(jti string, exp time.Time) string {
	claims := jwt.MapClaims{
		"sub":      "user-test",
		"email":    "test@example.com",
		"name":     "Test User",
		"projects": []string{},
		"iss":      "knodex",
		"aud":      "knodex-api",
		"exp":      exp.Unix(),
		"iat":      time.Now().Unix(),
	}
	if jti != "" {
		claims["jti"] = jti
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret"))
	return tokenString
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

		// Create a JWT with a jti claim
		tokenExp := time.Now().Add(30 * time.Minute)
		tokenString := createTestJWT("test-jti-for-logout", tokenExp)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		// Set user context (normally done by middleware)
		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: tokenExp.Unix(),
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
		tokenString := createTestJWT("jti-for-error-test", tokenExp)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: tokenExp.Unix(),
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

	t.Run("revokes token from cookie when no Authorization header", func(t *testing.T) {
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
		tokenString := createTestJWT("cookie-jti-test", tokenExp)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		// Set token via cookie instead of Authorization header
		req.AddCookie(&http.Cookie{
			Name:  "knodex_session",
			Value: tokenString,
		})

		userCtx := &middleware.UserContext{
			UserID:         "user-test",
			Email:          "test@example.com",
			TokenExpiresAt: tokenExp.Unix(),
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

		// Create a JWT without jti
		tokenString := createTestJWT("", time.Now().Add(30*time.Minute))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		req.Header.Set("Authorization", "Bearer "+tokenString)

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
