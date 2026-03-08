// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

// Integration tests updated to use CasbinAuthz exclusively.
// The old Authz middleware, mockPermissionChecker, Permission type, and DefaultRoutePermissions
// have been removed. All authorization now uses CasbinAuthz with PolicyEnforcer.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/auth"
)

// mockCasbinEnforcer implements CasbinPolicyEnforcer for testing
type mockCasbinEnforcer struct {
	canAccessFunc           func(ctx context.Context, user, object, action string) (bool, error)
	canAccessWithGroupsFunc func(ctx context.Context, user string, groups []string, object, action string) (bool, error)
	hasRoleFunc             func(ctx context.Context, user, role string) (bool, error) //
}

func (m *mockCasbinEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessFunc != nil {
		return m.canAccessFunc(ctx, user, object, action)
	}
	return false, nil
}

func (m *mockCasbinEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	if m.canAccessWithGroupsFunc != nil {
		return m.canAccessWithGroupsFunc(ctx, user, groups, object, action)
	}
	// Fallback to CanAccess if WithGroups not implemented
	if m.canAccessFunc != nil {
		return m.canAccessFunc(ctx, user, object, action)
	}
	return false, nil
}

// HasRole implements CasbinPolicyEnforcer
// For testing, returns false by default (user is not global admin)
func (m *mockCasbinEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	if m.hasRoleFunc != nil {
		return m.hasRoleFunc(ctx, user, role)
	}
	// Default: user does not have role (not global admin)
	return false, nil
}

// TestMiddlewareChain_FullStack tests the complete middleware chain integration
func TestMiddlewareChain_FullStack(t *testing.T) {
	// Create mock auth service
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			if token == "valid-token" {
				return &auth.JWTClaims{
					UserID:         "user-123",
					Email:          "user@example.com",
					DisplayName:    "Test User",
					Projects:       []string{"project-1"},
					DefaultProject: "project-1",
					CasbinRoles:    []string{},
				}, nil
			}
			return nil, errors.New("invalid token")
		},
	}

	// Create mock Casbin enforcer that allows access
	mockEnforcer := &mockCasbinEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			// Allow all access for this test
			return true, nil
		},
	}

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	// Build middleware chain (RequestID -> SecurityHeaders -> Logging -> Auth -> CasbinAuthz -> UserRateLimit)
	var handler http.Handler = testHandler

	// User rate limiting
	handler = UserRateLimit(UserRateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         10,
		FallbackToIP:      false,
	})(handler)

	// Authorization using CasbinAuthz
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)

	// Authentication
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	// Logging
	handler = Logging(handler)

	// Security Headers
	handler = SecurityHeaders(handler)

	// Request ID
	handler = RequestID(handler)

	// Test successful request with valid token
	req := httptest.NewRequest("GET", "/api/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify security headers were set
	if w.Header().Get("X-Frame-Options") == "" {
		t.Error("expected X-Frame-Options header to be set")
	}

	// Verify Request ID header was set
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestMiddlewareChain_AuthenticationFailure(t *testing.T) {
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return nil, errors.New("invalid token")
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when authentication fails")
		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: nil, // Enforcer not needed since auth will fail first
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)
	handler = Logging(handler)
	handler = SecurityHeaders(handler)
	handler = RequestID(handler)

	req := httptest.NewRequest("GET", "/api/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestMiddlewareChain_AuthorizationFailure(t *testing.T) {
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return &auth.JWTClaims{
				UserID:         "user-123",
				Email:          "user@example.com",
				DisplayName:    "Regular User",
				Projects:       []string{"project-1"},
				DefaultProject: "project-1",
				CasbinRoles:    []string{},
			}, nil
		},
	}

	// Create mock enforcer that denies access
	mockEnforcer := &mockCasbinEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			// Deny access to project creation
			return false, nil
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when authorization fails")
		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	// Try to access route that requires authorization
	req := httptest.NewRequest("POST", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestMiddlewareChain_RateLimitFailure(t *testing.T) {
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

	// Create mock enforcer that allows access
	mockEnforcer := &mockCasbinEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			return true, nil
		},
	}

	requestCount := 0
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = UserRateLimit(UserRateLimitConfig{
		RequestsPerMinute: 60,
		BurstSize:         2,
		FallbackToIP:      false,
	})(handler)
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/v1/instances", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	if requestCount != 2 {
		t.Errorf("expected 2 successful requests, got %d", requestCount)
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("GET", "/api/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}

	if requestCount != 2 {
		t.Errorf("handler should not be called on rate limited request, got %d calls", requestCount)
	}
}

func TestMiddlewareChain_GlobalAdminBypass(t *testing.T) {
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return &auth.JWTClaims{
				UserID:         "admin-123",
				Email:          "admin@example.com",
				DisplayName:    "Global Admin",
				Projects:       []string{"project-1"},
				DefaultProject: "project-1",
				CasbinRoles:    []string{"role:serveradmin"},
			}, nil
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create mock enforcer - Non-wildcard CanAccess calls should not happen for global admin (bypass)

	mockEnforcer := &mockCasbinEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// Wildcard permission check for admin - this enables bypass
			if user == "admin-123" && object == "*" && action == "*" {
				return true, nil
			}
			// Non-wildcard calls should not happen for global admin
			t.Errorf("CasbinAuthz should bypass non-wildcard permission check for global admin: object=%s, action=%s", object, action)
			return false, nil
		},
	}

	var handler http.Handler = testHandler
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	// Global admin should be able to access all routes
	routes := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/projects"},
		{"DELETE", "/api/v1/projects/project-1"},
		{"POST", "/api/v1/instances"},
		{"GET", "/api/v1/rgds"},
	}

	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s %s: expected status 200, got %d", route.method, route.path, w.Code)
		}
	}
}

func TestMiddlewareChain_UserContextPropagation(t *testing.T) {
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
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

	// Create mock enforcer that allows access
	mockEnforcer := &mockCasbinEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			return true, nil
		},
	}

	// Test handler verifies user context is propagated
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
		if len(userCtx.Projects) != 2 {
			t.Errorf("expected 2 projects, got %d", len(userCtx.Projects))
		}
		if userCtx.DefaultProject != "project-1" {
			t.Errorf("expected DefaultProject 'project-1', got '%s'", userCtx.DefaultProject)
		}

		for _, role := range userCtx.CasbinRoles {
			if role == "role:serveradmin" {
				t.Error("expected user to NOT have role:serveradmin in CasbinRoles")
				break
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	req := httptest.NewRequest("GET", "/api/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMiddlewareChain_CasbinEnforcer(t *testing.T) {
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

	// Create mock enforcer that allows some operations and denies others
	mockEnforcer := &mockCasbinEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			// Allow instance:create for user-123
			if user == "user-123" && action == "create" {
				return true, nil
			}
			// Deny instance:delete
			if action == "delete" {
				return false, nil
			}
			// Allow GET requests (list, get)
			if action == "list" || action == "get" {
				return true, nil
			}
			return false, nil
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	// Should succeed (user has permission)
	req := httptest.NewRequest("POST", "/api/v1/instances", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Should fail (user doesn't have delete permission)
	req2 := httptest.NewRequest("DELETE", "/api/v1/instances/default/SimpleApp/test", nil)
	req2.Header.Set("Authorization", "Bearer valid-token")
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w2.Code)
	}
}

func TestMiddlewareChain_RGDCatalogAccess(t *testing.T) {
	// Test RGD catalog hybrid authorization model
	// All authenticated users should access RGD catalog API regardless of other permissions
	mockAuth := &mockAuthService{
		validateTokenFunc: func(token string) (*auth.JWTClaims, error) {
			return &auth.JWTClaims{
				UserID:         "user-123",
				Email:          "user@example.com",
				DisplayName:    "Regular User",
				Projects:       []string{}, // No projects - but should still access RGD catalog
				DefaultProject: "",
				CasbinRoles:    []string{},
			}, nil
		},
	}

	// Create mock enforcer that would deny everything - but RGD catalog bypasses this
	mockEnforcer := &mockCasbinEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			// Deny everything - RGD catalog should bypass this
			return false, nil
		},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var handler http.Handler = testHandler
	handler = CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
	})(handler)
	handler = Auth(AuthConfig{
		AuthService: mockAuth,
	})(handler)

	// Test RGD catalog access
	rgdRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/rgds"},
		{"GET", "/api/v1/rgds/my-rgd"},
		{"GET", "/api/v1/rgds/my-rgd/resources"},
		{"GET", "/api/v1/rgds/my-rgd/schema"},
	}

	for _, route := range rgdRoutes {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s %s: expected status 200 (RGD catalog bypass), got %d", route.method, route.path, w.Code)
		}
	}

	// Non-RGD routes should still be denied
	// Note: /api/v1/instances (list) now uses hybrid model bypass,
	// so we test with a specific instance path which requires direct authorization
	nonRgdRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/instances/default/WebApp/my-app"}, // Specific instance, not covered by hybrid model
		{"POST", "/api/v1/projects"},
	}

	for _, route := range nonRgdRoutes {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected status 403 (no permission), got %d", route.method, route.path, w.Code)
		}
	}
}
