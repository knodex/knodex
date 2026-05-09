// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/auth"
)

// mockAuthService implements auth.ServiceInterface for testing
// Updated to match new interface (removed rbac.User dependency)
type mockAuthService struct {
	validateTokenFunc func(token string) (*auth.JWTClaims, error)
	localLoginEnabled bool // configurable; default false suits middleware tests
}

func (m *mockAuthService) ValidateToken(_ context.Context, token string) (*auth.JWTClaims, error) {
	return m.validateTokenFunc(token)
}

// GenerateTokenForAccount implements auth.ServiceInterface
func (m *mockAuthService) GenerateTokenForAccount(account *auth.Account, userID string) (string, time.Time, error) {
	return "", time.Time{}, nil
}

// GenerateTokenWithGroups implements auth.ServiceInterface
func (m *mockAuthService) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	return "", time.Time{}, nil
}

func (m *mockAuthService) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*auth.LoginResponse, error) {
	return nil, nil
}

func (m *mockAuthService) RevokeToken(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockAuthService) IsLocalLoginEnabled() bool { return m.localLoginEnabled }

// mockPermissionChecker removed - Permission and PermissionChecker types no longer exist.
// All authorization now uses CasbinAuthz middleware with PolicyEnforcer.CanAccessWithGroups().

func TestAuth_Success(t *testing.T) {
	t.Parallel()

	// Create mock auth service that returns valid claims
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			if token != "valid-token" {
				t.Errorf("expected token 'valid-token', got '%s'", token)
			}
			return &auth.JWTClaims{
				UserID:         "user-123",
				Email:          "user@example.com",
				DisplayName:    "Test User",
				Projects:       []string{"project-1", "project-2"},
				DefaultProject: "project-1",
				CasbinRoles:    []string{},
			}, nil
		},
	}

	// Create middleware
	middleware := Auth(AuthConfig{
		AuthService: mockAuth,
	})

	// Create test handler that checks user context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx, ok := GetUserContext(r)
		if !ok {
			t.Error("user context not found in request")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if userCtx.UserID != "user-123" {
			t.Errorf("expected UserID 'user-123', got '%s'", userCtx.UserID)
		}
		if userCtx.Email != "user@example.com" {
			t.Errorf("expected Email 'user@example.com', got '%s'", userCtx.Email)
		}
		if userCtx.DisplayName != "Test User" {
			t.Errorf("expected DisplayName 'Test User', got '%s'", userCtx.DisplayName)
		}
		if len(userCtx.Projects) != 2 {
			t.Errorf("expected 2 projects, got %d", len(userCtx.Projects))
		}
		if userCtx.DefaultProject != "project-1" {
			t.Errorf("expected DefaultProject 'project-1', got '%s'", userCtx.DefaultProject)
		}

		if len(userCtx.CasbinRoles) > 0 {
			t.Error("expected CasbinRoles to be empty for non-admin user")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Wrap handler with middleware
	handler := middleware(testHandler)

	// Create test request with valid Bearer token
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	// Execute request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "success" {
		t.Errorf("expected body 'success', got '%s'", w.Body.String())
	}
}

func TestAuth_MissingAuthorizationHeader(t *testing.T) {
	t.Parallel()

	mockAuth := &mockAuthService{}

	middleware := Auth(AuthConfig{
		AuthService: mockAuth,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when Authorization header is missing")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	// No Authorization header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuth_InvalidAuthorizationHeaderFormat(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header string
	}{
		{"missing Bearer prefix", "valid-token"},
		{"wrong prefix", "Basic valid-token"},
		{"no token", "Bearer"},
		{"extra parts", "Bearer token extra"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mockAuth := &mockAuthService{}

			middleware := Auth(AuthConfig{
				AuthService: mockAuth,
			})

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("handler should not be called with invalid Authorization header: %s", tc.header)
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(testHandler)

			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Code)
			}
		})
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	t.Parallel()

	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return nil, errors.New("invalid token")
		},
	}

	middleware := Auth(AuthConfig{
		AuthService: mockAuth,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when token validation fails")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGetUserContext_Success(t *testing.T) {
	t.Parallel()

	expectedCtx := &UserContext{
		UserID:         "user-123",
		Email:          "user@example.com",
		DisplayName:    "Test User",
		Projects:       []string{"project-1"},
		DefaultProject: "project-1",
		CasbinRoles:    []string{"role:serveradmin"},
	}

	ctx := context.WithValue(context.Background(), UserContextKey, expectedCtx)
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req = req.WithContext(ctx)

	userCtx, ok := GetUserContext(req)
	if !ok {
		t.Fatal("expected GetUserContext to return true")
	}

	if userCtx.UserID != expectedCtx.UserID {
		t.Errorf("expected UserID '%s', got '%s'", expectedCtx.UserID, userCtx.UserID)
	}
	if userCtx.Email != expectedCtx.Email {
		t.Errorf("expected Email '%s', got '%s'", expectedCtx.Email, userCtx.Email)
	}

	if len(userCtx.CasbinRoles) != len(expectedCtx.CasbinRoles) {
		t.Errorf("expected CasbinRoles %v, got %v", expectedCtx.CasbinRoles, userCtx.CasbinRoles)
	}
}

func TestGetUserContext_NotFound(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)

	userCtx, ok := GetUserContext(req)
	if ok {
		t.Error("expected GetUserContext to return false when no context set")
	}
	if userCtx != nil {
		t.Error("expected nil userCtx when not found")
	}
}

func TestOptionalAuth_WithValidToken(t *testing.T) {
	t.Parallel()

	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return &auth.JWTClaims{
				UserID:         "user-123",
				Email:          "user@example.com",
				DisplayName:    "Test User",
				Projects:       []string{"project-1"},
				DefaultProject: "project-1",
				CasbinRoles:    []string{},
			}, nil
		},
	}

	middleware := OptionalAuth(AuthConfig{
		AuthService: mockAuth,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx, ok := GetUserContext(r)
		if !ok {
			t.Error("expected user context to be set with valid token")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if userCtx.UserID != "user-123" {
			t.Errorf("expected UserID 'user-123', got '%s'", userCtx.UserID)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestOptionalAuth_WithoutToken(t *testing.T) {
	t.Parallel()

	mockAuth := &mockAuthService{}

	middleware := OptionalAuth(AuthConfig{
		AuthService: mockAuth,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx, ok := GetUserContext(r)
		if ok {
			t.Error("expected no user context when no token provided")
		}
		if userCtx != nil {
			t.Error("expected nil userCtx when no token provided")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	// No Authorization header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestOptionalAuth_WithInvalidToken(t *testing.T) {
	t.Parallel()

	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return nil, errors.New("invalid token")
		},
	}

	middleware := OptionalAuth(AuthConfig{
		AuthService: mockAuth,
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userCtx, ok := GetUserContext(r)
		if ok {
			t.Error("expected no user context when token is invalid")
		}
		if userCtx != nil {
			t.Error("expected nil userCtx when token is invalid")
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// OptionalAuth should allow request to proceed even with invalid token
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
