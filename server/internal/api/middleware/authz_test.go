// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Old Authz middleware tests removed - all authorization uses CasbinAuthz exclusively.
// Tests for the removed Authz, AuthzConfig, RoutePermission, DefaultRoutePermissions,
// Permission, and PermissionChecker have been deleted.

// mockCasbinPolicyEnforcer implements CasbinPolicyEnforcer for testing
type mockCasbinPolicyEnforcer struct {
	canAccessFunc           func(ctx context.Context, user, object, action string) (bool, error)
	canAccessWithGroupsFunc func(ctx context.Context, user string, groups []string, object, action string) (bool, error)
	hasRoleFunc             func(ctx context.Context, user, role string) (bool, error)
}

func (m *mockCasbinPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessFunc != nil {
		return m.canAccessFunc(ctx, user, object, action)
	}
	// Default: allow all for testing
	return true, nil
}

// CanAccessWithGroups implements CasbinPolicyEnforcer
// Delegates to canAccessWithGroupsFunc if set, otherwise falls back to CanAccess
func (m *mockCasbinPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	if m.canAccessWithGroupsFunc != nil {
		return m.canAccessWithGroupsFunc(ctx, user, groups, object, action)
	}
	return m.CanAccess(ctx, user, object, action)
}

// HasRole implements CasbinPolicyEnforcer
// For testing, returns false by default (user is not global admin)
func (m *mockCasbinPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	if m.hasRoleFunc != nil {
		return m.hasRoleFunc(ctx, user, role)
	}
	// Default: user does not have role (not global admin)
	return false, nil
}

// Use RequireGlobalAdminWithEnforcer which tests via Casbin permissions.
// All authorization should flow through Casbin policies - no special role checks.

// mockReadyChecker implements RBACReadyChecker for testing
type mockReadyChecker struct {
	synced bool
}

func (m *mockReadyChecker) IsPolicySynced() bool {
	return m.synced
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestGetUserProjects(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	userCtx := &UserContext{
		UserID:   "user-123",
		Projects: []string{"project-1", "project-2", "project-3"},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	orgs := GetUserProjects(req)
	if len(orgs) != 3 {
		t.Errorf("expected 3 projects, got %d", len(orgs))
	}
	if orgs[0] != "project-1" || orgs[1] != "project-2" || orgs[2] != "project-3" {
		t.Errorf("unexpected project list: %v", orgs)
	}
}

func TestGetUserProjects_NoUserContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)

	orgs := GetUserProjects(req)
	if len(orgs) != 0 {
		t.Errorf("expected empty projects, got %v", orgs)
	}
}

// HasGlobalAdminRole check removed - all authorization flows through Casbin Enforce() calls.
// This aligns with ArgoCD's Casbin-only authorization pattern (single source of truth).

func TestGetUserID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	userCtx := &UserContext{
		UserID: "user-123",
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	userID, err := GetUserID(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("expected userID 'user-123', got '%s'", userID)
	}
}

func TestGetUserID_NoUserContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/api/v1/test", nil)

	userID, err := GetUserID(req)
	if err == nil {
		t.Error("expected error when no user context")
	}
	if userID != "" {
		t.Errorf("expected empty userID, got '%s'", userID)
	}
}

// ============================================================================
// CasbinAuthz Middleware Tests
// ============================================================================

func TestCasbinAuthz_Allowed(t *testing.T) {
	t.Parallel()

	callCount := 0
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			callCount++
			if user != "user-123" {
				t.Errorf("expected user 'user-123', got '%s'", user)
			}

			if callCount == 1 {
				if object != "*" || action != "*" {
					t.Errorf("expected first call to be admin check (*, *), got (%s, %s)", object, action)
				}
				return false, nil // User is not admin
			}
			// Second call is the actual permission check
			if object != "projects/engineering" {
				t.Errorf("expected object 'projects/engineering', got '%s'", object)
			}
			if action != "get" {
				t.Errorf("expected action 'get', got '%s'", action)
			}
			return true, nil
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/projects/engineering", nil)
	userCtx := &UserContext{
		UserID:         "user-123",
		Email:          "user@example.com",
		DisplayName:    "Regular User",
		Projects:       []string{"engineering"},
		DefaultProject: "engineering",
		CasbinRoles:    []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCasbinAuthz_Denied(t *testing.T) {
	t.Parallel()

	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return false, nil
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when access denied")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/engineering", nil)
	userCtx := &UserContext{
		UserID:         "user-123",
		Email:          "user@example.com",
		DisplayName:    "Regular User",
		Projects:       []string{"engineering"},
		DefaultProject: "engineering",
		CasbinRoles:    []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	// Verify flat response format with details
	var errResp struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Details map[string]string `json:"details"`
	}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %q", errResp.Code)
	}
	if errResp.Message != "insufficient permissions" {
		t.Errorf("expected message 'insufficient permissions', got %q", errResp.Message)
	}
	if errResp.Details["object"] != "projects/engineering" {
		t.Errorf("expected details.object 'projects/engineering', got %q", errResp.Details["object"])
	}
	if errResp.Details["action"] != "delete" {
		t.Errorf("expected details.action 'delete', got %q", errResp.Details["action"])
	}
}

func TestCasbinAuthz_MissingUserContext(t *testing.T) {
	t.Parallel()

	mockEnforcer := &mockCasbinPolicyEnforcer{}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no user context")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestCasbinAuthz_GlobalAdminBypass(t *testing.T) {
	t.Parallel()

	enforcerCalled := false
	mockEnforcer := &mockCasbinPolicyEnforcer{

		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// Wildcard permission check for admin - this should allow bypass
			if user == "admin-123" && object == "*" && action == "*" {
				return true, nil
			}
			// Other permission checks should not be called (this tracks unexpected calls)
			enforcerCalled = true
			return false, nil
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/critical", nil)
	userCtx := &UserContext{
		UserID:         "admin-123",
		Email:          "admin@example.com",
		DisplayName:    "Global Admin",
		Projects:       []string{},
		DefaultProject: "",
		CasbinRoles:    []string{"role:serveradmin"},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if enforcerCalled {
		t.Error("enforcer should not be called for global admin")
	}
}

func TestCasbinAuthz_GlobalAdminBypassViaGroups(t *testing.T) {
	t.Parallel()

	// Verify that a user who is NOT a direct admin but whose OIDC group
	// has role:serveradmin gets admin bypass via CanAccessWithGroups.
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			// Admin wildcard check: grant access only when groups include "platform-admins"
			if object == "*" && action == "*" {
				for _, g := range groups {
					if g == "platform-admins" {
						return true, nil
					}
				}
				return false, nil
			}
			return false, nil
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/critical", nil)
	userCtx := &UserContext{
		UserID:      "group-admin-user",
		Email:       "groupadmin@example.com",
		Groups:      []string{"platform-admins", "engineering"},
		CasbinRoles: []string{}, // NOT a direct admin — admin comes via group
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 (admin via group), got %d", w.Code)
	}
}

func TestCasbinAuthz_EnforcerNotConfigured(t *testing.T) {
	t.Parallel()

	// SECURITY: Fail-safe when enforcer not configured
	// Use a path NOT covered by hybrid model bypasses (projects list, RGD catalog, instances list)
	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: nil,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when enforcer not configured")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Use a specific project resource (not the list endpoint which uses hybrid model)
	req := httptest.NewRequest("GET", "/api/v1/projects/engineering", nil)
	userCtx := &UserContext{
		UserID:         "user-123",
		Email:          "user@example.com",
		DisplayName:    "Regular User",
		Projects:       []string{"project-1"},
		DefaultProject: "project-1",
		CasbinRoles:    []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestCasbinAuthz_EnforcerError(t *testing.T) {
	t.Parallel()

	// Use a path NOT covered by hybrid model bypasses (projects list, RGD catalog, instances list)
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return false, errors.New("enforcer database error")
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when enforcer errors")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Use a specific project resource (not the list endpoint which uses hybrid model)
	req := httptest.NewRequest("GET", "/api/v1/projects/engineering", nil)
	userCtx := &UserContext{
		UserID:         "user-123",
		Email:          "user@example.com",
		DisplayName:    "Regular User",
		Projects:       []string{"project-1"},
		DefaultProject: "project-1",
		CasbinRoles:    []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestCasbinAuthz_PathTraversalBlocked(t *testing.T) {
	t.Parallel()

	mockEnforcer := &mockCasbinPolicyEnforcer{}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for path traversal attempt")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Test path traversal attempt
	req := httptest.NewRequest("GET", "/api/v1/projects/../secrets", nil)
	userCtx := &UserContext{
		UserID:      "user-123",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCasbinAuthz_NullByteBlocked(t *testing.T) {
	t.Parallel()

	mockEnforcer := &mockCasbinPolicyEnforcer{}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for null byte attempt")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Test null byte in path
	req := httptest.NewRequest("GET", "/api/v1/projects/test", nil)
	req.URL.Path = "/api/v1/projects/test\x00admin"
	userCtx := &UserContext{
		UserID:      "user-123",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	// Verify response body contains correct error structure (flat format)
	var errResp struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "BAD_REQUEST" {
		t.Errorf("expected error code BAD_REQUEST, got %q", errResp.Code)
	}
	if errResp.Message != "invalid path parameter: contains null byte or control character" {
		t.Errorf("unexpected error message: %q", errResp.Message)
	}
}

func TestCasbinAuthz_ControlCharsBlocked(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{"control_char_0x01", "/api/v1/projects/test\x01"},
		{"control_char_0x1F", "/api/v1/projects/test\x1F"},
		{"del_char_0x7F", "/api/v1/projects/test\x7F"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockEnforcer := &mockCasbinPolicyEnforcer{}
			middleware := CasbinAuthz(CasbinAuthzConfig{
				Enforcer: mockEnforcer,
				Logger:   slog.Default(),
			})

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("handler should not be called for control char attempt")
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(testHandler)

			req := httptest.NewRequest("GET", "/api/v1/projects/test", nil)
			req.URL.Path = tt.path
			userCtx := &UserContext{
				UserID:      "user-123",
				CasbinRoles: []string{},
			}
			ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestCasbinAuthz_ValidPathsNotBlockedByControlCharCheck(t *testing.T) {
	t.Parallel()

	validPaths := []struct {
		name string
		path string
	}{
		{"simple_project", "/api/v1/projects/my-project"},
		{"dots_and_underscores", "/api/v1/instances/default/MyKind/my_instance.v1"},
		{"rgds", "/api/v1/rgds"},
		{"hyphen_project", "/api/v1/projects/alpha-beta"},
	}

	for _, tt := range validPaths {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handlerCalled := false
			mockEnforcer := &mockCasbinPolicyEnforcer{
				canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
					return true, nil
				},
			}
			middleware := CasbinAuthz(CasbinAuthzConfig{
				Enforcer: mockEnforcer,
				Logger:   slog.Default(),
			})

			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(testHandler)

			req := httptest.NewRequest("GET", tt.path, nil)
			userCtx := &UserContext{
				UserID:      "user-123",
				CasbinRoles: []string{},
			}
			ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest {
				t.Errorf("valid path %q should not be blocked, got status %d", tt.path, w.Code)
			}
			if !handlerCalled {
				t.Errorf("handler should have been called for valid path %q", tt.path)
			}
		})
	}
}

func TestCasbinAuthz_RawPathControlCharBlocked(t *testing.T) {
	t.Parallel()

	mockEnforcer := &mockCasbinPolicyEnforcer{}
	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for RawPath control char attempt")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("GET", "/api/v1/projects/test", nil)
	req.URL.Path = "/api/v1/projects/test"
	req.URL.RawPath = "/api/v1/projects/test\x01inject"
	userCtx := &UserContext{
		UserID:      "user-123",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for control char in RawPath, got %d", w.Code)
	}
}

// ============================================================================
// RGD Catalog Access Tests
// ============================================================================

func TestCasbinAuthz_RGDCatalogAccess(t *testing.T) {
	t.Parallel()

	// All authenticated users can access RGD catalog
	// The handler filters what they can see based on visibility labels

	nonWildcardCallMade := false
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {

			if object == "*" && action == "*" {
				return false, nil // User is not admin, falls through to RGD bypass
			}
			// Any non-wildcard call means the RGD bypass didn't work
			nonWildcardCallMade = true
			return false, nil // Would deny, but should be bypassed for RGDs
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{"list rgds", "GET", "/api/v1/rgds"},
		{"get specific rgd", "GET", "/api/v1/rgds/my-rgd"},
		{"get rgd with namespace", "GET", "/api/v1/rgds/default/my-rgd"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nonWildcardCallMade = false

			req := httptest.NewRequest(tc.method, tc.path, nil)
			userCtx := &UserContext{
				UserID:         "user-123",
				Email:          "user@example.com",
				DisplayName:    "Regular User",
				Projects:       []string{},
				DefaultProject: "",
				CasbinRoles:    []string{},
			}
			ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for RGD catalog access, got %d", w.Code)
			}

			if nonWildcardCallMade {
				t.Error("non-wildcard enforcer calls should not be made for RGD catalog access")
			}
		})
	}
}

func TestCasbinAuthz_RGDPostNotAllowed(t *testing.T) {
	t.Parallel()

	// POST to /rgds should NOT use the RGD catalog bypass
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return false, nil // Deny
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when access denied")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("POST", "/api/v1/rgds", nil)
	userCtx := &UserContext{
		UserID:      "user-123",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for POST to RGDs, got %d", w.Code)
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestGetUserFromJWTContext_Success(t *testing.T) {
	t.Parallel()

	claims := map[string]interface{}{
		"sub":   "user-123",
		"email": "user@example.com",
	}
	ctx := WithJWTClaims(context.Background(), claims)

	userID, err := GetUserFromJWTContext(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("expected userID 'user-123', got '%s'", userID)
	}
}

func TestGetUserFromJWTContext_MissingClaims(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := GetUserFromJWTContext(ctx)
	if err == nil {
		t.Error("expected error when claims missing")
	}
}

func TestGetUserFromJWTContext_MissingSub(t *testing.T) {
	t.Parallel()

	claims := map[string]interface{}{
		"email": "user@example.com",
	}
	ctx := WithJWTClaims(context.Background(), claims)

	_, err := GetUserFromJWTContext(ctx)
	if err == nil {
		t.Error("expected error when sub claim missing")
	}
}

func TestGetUserGroupsFromContext_Success(t *testing.T) {
	t.Parallel()

	claims := map[string]interface{}{
		"sub":    "user-123",
		"groups": []interface{}{"admin", "developers", "testers"},
	}
	ctx := WithJWTClaims(context.Background(), claims)

	groups, err := GetUserGroupsFromContext(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}
	if groups[0] != "admin" || groups[1] != "developers" || groups[2] != "testers" {
		t.Errorf("unexpected groups: %v", groups)
	}
}

func TestGetUserGroupsFromContext_StringSlice(t *testing.T) {
	t.Parallel()

	claims := map[string]interface{}{
		"sub":    "user-123",
		"groups": []string{"admin", "developers"},
	}
	ctx := WithJWTClaims(context.Background(), claims)

	groups, err := GetUserGroupsFromContext(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestGetUserGroupsFromContext_NoGroups(t *testing.T) {
	t.Parallel()

	claims := map[string]interface{}{
		"sub": "user-123",
	}
	ctx := WithJWTClaims(context.Background(), claims)

	groups, err := GetUserGroupsFromContext(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected empty groups, got %v", groups)
	}
}

func TestGetUserGroupsFromContext_MissingClaims(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	_, err := GetUserGroupsFromContext(ctx)
	if err == nil {
		t.Error("expected error when claims missing")
	}
}

func TestInferCasbinObjectAndAction(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		method         string
		path           string
		expectedObject string
		expectedAction string
	}{
		{
			name:           "list projects",
			method:         "GET",
			path:           "/api/v1/projects",
			expectedObject: "projects/*",
			expectedAction: "list",
		},
		{
			name:           "get specific project",
			method:         "GET",
			path:           "/api/v1/projects/engineering",
			expectedObject: "projects/engineering",
			expectedAction: "get",
		},
		{
			name:           "create project",
			method:         "POST",
			path:           "/api/v1/projects",
			expectedObject: "projects/*",
			expectedAction: "create",
		},
		{
			name:           "update project",
			method:         "PATCH",
			path:           "/api/v1/projects/engineering",
			expectedObject: "projects/engineering",
			expectedAction: "update",
		},
		{
			name:           "delete project",
			method:         "DELETE",
			path:           "/api/v1/projects/engineering",
			expectedObject: "projects/engineering",
			expectedAction: "delete",
		},
		{
			name:           "list instances",
			method:         "GET",
			path:           "/api/v1/instances",
			expectedObject: "instances/*",
			expectedAction: "list",
		},
		{
			name:           "get namespaced instance (K8s-aligned)",
			method:         "GET",
			path:           "/api/v1/namespaces/default/instances/WebApp/my-app",
			expectedObject: "instances/default/WebApp/my-app",
			expectedAction: "get",
		},
		{
			name:           "get cluster-scoped instance (K8s-aligned)",
			method:         "GET",
			path:           "/api/v1/instances/WebApp/my-app",
			expectedObject: "instances/WebApp/my-app",
			expectedAction: "get",
		},
		{
			name:           "delete namespaced instance (K8s-aligned)",
			method:         "DELETE",
			path:           "/api/v1/namespaces/staging/instances/Database/my-db",
			expectedObject: "instances/staging/Database/my-db",
			expectedAction: "delete",
		},
		{
			name:           "update namespaced instance (K8s-aligned)",
			method:         "PATCH",
			path:           "/api/v1/namespaces/prod/instances/Cache/redis-1",
			expectedObject: "instances/prod/Cache/redis-1",
			expectedAction: "update",
		},
		{
			name:           "create namespaced instance (K8s-aligned)",
			method:         "POST",
			path:           "/api/v1/namespaces/default/instances/WebApp",
			expectedObject: "instances/default/WebApp",
			expectedAction: "create",
		},
		{
			name:           "create cluster-scoped instance (K8s-aligned)",
			method:         "POST",
			path:           "/api/v1/instances/ClusterConfig",
			expectedObject: "instances/ClusterConfig",
			expectedAction: "create",
		},
		{
			name:           "get namespaced instance history sub-resource (K8s-aligned)",
			method:         "GET",
			path:           "/api/v1/namespaces/default/instances/WebApp/my-app/history",
			expectedObject: "instances/default/WebApp/my-app/history",
			expectedAction: "get",
		},
		{
			name:           "list rgds",
			method:         "GET",
			path:           "/api/v1/rgds",
			expectedObject: "rgds/*",
			expectedAction: "list",
		},
		{
			name:           "put update",
			method:         "PUT",
			path:           "/api/v1/projects/engineering",
			expectedObject: "projects/engineering",
			expectedAction: "update",
		},
		{
			name:           "list sso providers",
			method:         "GET",
			path:           "/api/v1/settings/sso/providers",
			expectedObject: "settings/sso/providers",
			expectedAction: "get",
		},
		{
			name:           "get specific sso provider",
			method:         "GET",
			path:           "/api/v1/settings/sso/providers/google",
			expectedObject: "settings/sso/providers/google",
			expectedAction: "get",
		},
		{
			name:           "create sso provider",
			method:         "POST",
			path:           "/api/v1/settings/sso/providers",
			expectedObject: "settings/sso/providers",
			expectedAction: "create",
		},
		{
			name:           "update sso provider",
			method:         "PUT",
			path:           "/api/v1/settings/sso/providers/google",
			expectedObject: "settings/sso/providers/google",
			expectedAction: "update",
		},
		{
			name:           "delete sso provider",
			method:         "DELETE",
			path:           "/api/v1/settings/sso/providers/google",
			expectedObject: "settings/sso/providers/google",
			expectedAction: "delete",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.path, nil)
			object, action := inferCasbinObjectAndAction(req, tc.path)

			if object != tc.expectedObject {
				t.Errorf("expected object '%s', got '%s'", tc.expectedObject, object)
			}
			if action != tc.expectedAction {
				t.Errorf("expected action '%s', got '%s'", tc.expectedAction, action)
			}
		})
	}
}

func TestWithJWTClaims(t *testing.T) {
	t.Parallel()

	claims := map[string]interface{}{
		"sub":   "user-123",
		"email": "user@example.com",
		"name":  "Test User",
	}

	ctx := WithJWTClaims(context.Background(), claims)

	// Verify claims can be retrieved
	retrievedClaims, ok := ctx.Value(jwtClaimsContextKey).(map[string]interface{})
	if !ok {
		t.Error("claims not retrievable from context")
	}
	if retrievedClaims["sub"] != "user-123" {
		t.Errorf("expected sub 'user-123', got '%s'", retrievedClaims["sub"])
	}
}

// ============================================================================
// Security Tests for matchesPathPrefix
// ============================================================================

func TestMatchesPathPrefix_Security(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		requestPath    string
		pathPrefix     string
		expectedResult bool
		description    string
	}{
		// SECURITY: Empty prefix must NOT match anything (HIGH-1 fix)
		{
			name:           "empty prefix rejected",
			requestPath:    "/api/v1/projects",
			pathPrefix:     "",
			expectedResult: false,
			description:    "Empty prefix must not match any path to prevent authorization bypass",
		},
		{
			name:           "empty prefix with root path rejected",
			requestPath:    "/",
			pathPrefix:     "",
			expectedResult: false,
			description:    "Empty prefix must not match even root path",
		},
		{
			name:           "empty prefix with admin path rejected",
			requestPath:    "/api/v1/admin/secrets",
			pathPrefix:     "",
			expectedResult: false,
			description:    "Empty prefix must not match sensitive admin endpoints",
		},
		// SECURITY: Root prefix must NOT match anything (HIGH-2 fix)
		{
			name:           "root prefix rejected",
			requestPath:    "/api/v1/projects",
			pathPrefix:     "/",
			expectedResult: false,
			description:    "Root prefix must not match any path to prevent catch-all authorization",
		},
		{
			name:           "root prefix with admin path rejected",
			requestPath:    "/api/v1/admin/delete-all-users",
			pathPrefix:     "/",
			expectedResult: false,
			description:    "Root prefix must not match sensitive admin endpoints",
		},
		{
			name:           "root prefix with sensitive data rejected",
			requestPath:    "/api/v1/secrets/database-password",
			pathPrefix:     "/",
			expectedResult: false,
			description:    "Root prefix must not match secret endpoints",
		},
		// Normal prefix matching must still work
		{
			name:           "valid prefix exact match",
			requestPath:    "/api/v1/projects",
			pathPrefix:     "/api/v1/projects",
			expectedResult: true,
			description:    "Exact prefix match should work",
		},
		{
			name:           "valid prefix with subpath",
			requestPath:    "/api/v1/projects/engineering",
			pathPrefix:     "/api/v1/projects",
			expectedResult: true,
			description:    "Prefix with subpath should match",
		},
		{
			name:           "valid prefix no partial string match",
			requestPath:    "/api/v1/projectsmalicious",
			pathPrefix:     "/api/v1/projects",
			expectedResult: false,
			description:    "Must not match partial strings without path separator",
		},
		{
			name:           "valid prefix instances",
			requestPath:    "/api/v1/instances/default/my-app",
			pathPrefix:     "/api/v1/instances/",
			expectedResult: true,
			description:    "Instances path should match",
		},
		{
			name:           "valid prefix rgds",
			requestPath:    "/api/v1/rgds",
			pathPrefix:     "/api/v1/rgds",
			expectedResult: true,
			description:    "RGDs path exact match",
		},
		{
			name:           "different path no match",
			requestPath:    "/api/v1/users",
			pathPrefix:     "/api/v1/projects",
			expectedResult: false,
			description:    "Unrelated paths should not match",
		},
		// Path traversal prevention (existing functionality)
		{
			name:           "path traversal in request blocked by path.Clean elsewhere",
			requestPath:    "/api/v1/projects/engineering",
			pathPrefix:     "/api/v1/projects",
			expectedResult: true,
			description:    "Clean paths should match normally",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := matchesPathPrefix(tc.requestPath, tc.pathPrefix)
			if result != tc.expectedResult {
				t.Errorf("matchesPathPrefix(%q, %q) = %v, want %v\nDescription: %s",
					tc.requestPath, tc.pathPrefix, result, tc.expectedResult, tc.description)
			}
		})
	}
}

func TestMatchesPathPrefix_EmptyPrefixDoesNotMatchAll(t *testing.T) {
	t.Parallel()

	// This test specifically validates the HIGH-1 security fix
	// An empty prefix must never match any request path
	testPaths := []string{
		"/",
		"/api",
		"/api/v1",
		"/api/v1/projects",
		"/api/v1/projects/secret-project",
		"/api/v1/instances",
		"/api/v1/admin",
		"/api/v1/admin/users/delete",
		"/healthz",
		"/readyz",
	}

	for _, path := range testPaths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if matchesPathPrefix(path, "") {
				t.Errorf("SECURITY VIOLATION: Empty prefix matched path %q - this would bypass authorization", path)
			}
		})
	}
}

func TestMatchesPathPrefix_RootPrefixDoesNotMatchAll(t *testing.T) {
	t.Parallel()

	// This test specifically validates the HIGH-2 security fix
	// A root "/" prefix must never match any request path
	testPaths := []string{
		"/",
		"/api",
		"/api/v1",
		"/api/v1/projects",
		"/api/v1/projects/secret-project",
		"/api/v1/instances",
		"/api/v1/admin",
		"/api/v1/admin/users/delete",
		"/healthz",
		"/readyz",
	}

	for _, path := range testPaths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if matchesPathPrefix(path, "/") {
				t.Errorf("SECURITY VIOLATION: Root prefix '/' matched path %q - this would bypass authorization", path)
			}
		})
	}
}

// ============================================================================
// Instance Create Request Tests (K8s-aligned routes, STORY-327)
// ============================================================================

func TestIsInstanceCreateRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		method   string
		expected bool
	}{
		{
			name:     "namespaced create (K8s-aligned)",
			path:     "/api/v1/namespaces/default/instances/WebApp",
			method:   "POST",
			expected: true,
		},
		{
			name:     "cluster-scoped create (K8s-aligned)",
			path:     "/api/v1/instances/ClusterConfig",
			method:   "POST",
			expected: true,
		},
		{
			name:     "namespaced create with trailing slash",
			path:     "/api/v1/namespaces/default/instances/WebApp/",
			method:   "POST",
			expected: true,
		},
		{
			name:     "GET not matched",
			path:     "/api/v1/namespaces/default/instances/WebApp",
			method:   "GET",
			expected: false,
		},
		{
			name:     "list instances not matched",
			path:     "/api/v1/instances",
			method:   "POST",
			expected: false,
		},
		{
			name:     "too many segments not matched",
			path:     "/api/v1/namespaces/default/instances/WebApp/my-app",
			method:   "POST",
			expected: false,
		},
		{
			name:     "wrong resource not matched",
			path:     "/api/v1/namespaces/default/projects/WebApp",
			method:   "POST",
			expected: false,
		},
		{
			name:     "POST instances/count not matched (collection guard)",
			path:     "/api/v1/instances/count",
			method:   "POST",
			expected: false,
		},
		{
			name:     "POST instances/pending not matched (collection guard)",
			path:     "/api/v1/instances/pending",
			method:   "POST",
			expected: false,
		},
		{
			name:     "POST instances/stuck not matched (collection guard)",
			path:     "/api/v1/instances/stuck",
			method:   "POST",
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isInstanceCreateRequest(tc.path, tc.method)
			if result != tc.expected {
				t.Errorf("isInstanceCreateRequest(%q, %q) = %v, want %v", tc.path, tc.method, result, tc.expected)
			}
		})
	}
}

// Instance List Request Tests (unchanged by STORY-327)
// ============================================================================

func TestIsInstanceListRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		method   string
		expected bool
	}{
		{
			name:     "list instances",
			path:     "/api/v1/instances",
			method:   "GET",
			expected: true,
		},
		{
			name:     "count instances",
			path:     "/api/v1/instances/count",
			method:   "GET",
			expected: true,
		},
		{
			name:     "pending instances",
			path:     "/api/v1/instances/pending",
			method:   "GET",
			expected: true,
		},
		{
			name:     "stuck instances",
			path:     "/api/v1/instances/stuck",
			method:   "GET",
			expected: true,
		},
		{
			name:     "POST not matched",
			path:     "/api/v1/instances",
			method:   "POST",
			expected: false,
		},
		{
			name:     "specific instance not matched",
			path:     "/api/v1/instances/WebApp/my-app",
			method:   "GET",
			expected: false,
		},
		{
			name:     "namespaced instance not matched",
			path:     "/api/v1/namespaces/default/instances/WebApp/my-app",
			method:   "GET",
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isInstanceListRequest(tc.path, tc.method)
			if result != tc.expected {
				t.Errorf("isInstanceListRequest(%q, %q) = %v, want %v", tc.path, tc.method, result, tc.expected)
			}
		})
	}
}

// Project Namespace Access Tests (Hybrid Authorization Model)
// ============================================================================

func TestIsProjectNamespaceRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		method   string
		expected bool
	}{
		{
			name:     "valid project namespace request",
			path:     "/api/v1/projects/engineering/namespaces",
			method:   "GET",
			expected: true,
		},
		{
			name:     "valid project namespace request with trailing slash",
			path:     "/api/v1/projects/engineering/namespaces/",
			method:   "GET",
			expected: true,
		},
		{
			name:     "valid project namespace with hyphenated name",
			path:     "/api/v1/projects/proj-azuread-staging/namespaces",
			method:   "GET",
			expected: true,
		},
		{
			name:     "POST to project namespace not allowed",
			path:     "/api/v1/projects/engineering/namespaces",
			method:   "POST",
			expected: false,
		},
		{
			name:     "DELETE to project namespace not allowed",
			path:     "/api/v1/projects/engineering/namespaces",
			method:   "DELETE",
			expected: false,
		},
		{
			name:     "project list not matched",
			path:     "/api/v1/projects",
			method:   "GET",
			expected: false,
		},
		{
			name:     "specific project not matched",
			path:     "/api/v1/projects/engineering",
			method:   "GET",
			expected: false,
		},
		{
			name:     "project roles not matched",
			path:     "/api/v1/projects/engineering/roles",
			method:   "GET",
			expected: false,
		},
		{
			name:     "deep subpath not matched",
			path:     "/api/v1/projects/engineering/namespaces/staging",
			method:   "GET",
			expected: false,
		},
		{
			name:     "empty project name not matched",
			path:     "/api/v1/projects//namespaces",
			method:   "GET",
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := isProjectNamespaceRequest(tc.path, tc.method)
			if result != tc.expected {
				t.Errorf("isProjectNamespaceRequest(%q, %q) = %v, want %v",
					tc.path, tc.method, result, tc.expected)
			}
		})
	}
}

func TestIsWSTicketRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		path     string
		method   string
		expected bool
	}{
		{
			name:     "valid POST ws ticket request",
			path:     "/api/v1/ws/ticket",
			method:   "POST",
			expected: true,
		},
		{
			name:     "valid POST ws ticket request with trailing slash",
			path:     "/api/v1/ws/ticket/",
			method:   "POST",
			expected: true,
		},
		{
			name:     "GET ws ticket not allowed",
			path:     "/api/v1/ws/ticket",
			method:   "GET",
			expected: false,
		},
		{
			name:     "DELETE ws ticket not allowed",
			path:     "/api/v1/ws/ticket",
			method:   "DELETE",
			expected: false,
		},
		{
			name:     "ws metrics not matched",
			path:     "/api/v1/ws/metrics",
			method:   "POST",
			expected: false,
		},
		{
			name:     "different path not matched",
			path:     "/api/v1/settings",
			method:   "POST",
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := isWSTicketRequest(tc.path, tc.method)
			if result != tc.expected {
				t.Errorf("isWSTicketRequest(%q, %q) = %v, want %v",
					tc.path, tc.method, result, tc.expected)
			}
		})
	}
}

func TestCasbinAuthz_WSTicketBypass(t *testing.T) {
	t.Parallel()

	// WebSocket ticket requests should bypass CasbinAuthz for all authenticated users
	enforcerCalled := false
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			if object == "*" && action == "*" {
				return false, nil // User is NOT admin
			}
			enforcerCalled = true
			return false, nil // Would deny
		},
	}

	handler := CasbinAuthz(CasbinAuthzConfig{
		Enforcer:     mockEnforcer,
		ReadyChecker: &mockReadyChecker{synced: true},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/ws/ticket", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, &UserContext{
		UserID: "user-oidc-abc123",
		Email:  "test@example.com",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for WS ticket bypass, got %d", rr.Code)
	}
	if enforcerCalled {
		t.Error("enforcer should NOT be called for WS ticket requests (bypassed)")
	}
}

func TestCasbinAuthz_WSTicket_DeniedDuringStartup(t *testing.T) {
	t.Parallel()

	// During startup (RBAC not synced), WS ticket requests should get 503,
	// not bypass authorization. The ReadyChecker guard takes precedence.
	handler := CasbinAuthz(CasbinAuthzConfig{
		Enforcer:     &mockCasbinPolicyEnforcer{},
		ReadyChecker: &mockReadyChecker{synced: false}, // RBAC not yet synced
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should NOT be called when RBAC is not synced")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/ws/ticket", nil)
	ctx := context.WithValue(req.Context(), UserContextKey, &UserContext{
		UserID: "user-oidc-abc123",
		Email:  "test@example.com",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 during startup for WS ticket request, got %d", rr.Code)
	}
}

func TestCasbinAuthz_ProjectNamespaceAccess(t *testing.T) {
	t.Parallel()

	// Project namespace requests bypass middleware and are authorized by handler

	// but for non-admin users, the project namespace bypass should still work
	nonWildcardEnforcerCalled := false
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// Admin check with wildcards is expected
			if object == "*" && action == "*" {
				return false, nil // User is not admin
			}
			// Any non-wildcard permission check should not happen (bypass expected)
			nonWildcardEnforcerCalled = true
			return false, nil // Would deny, but should be bypassed
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{"list project namespaces", "GET", "/api/v1/projects/engineering/namespaces"},
		{"list project namespaces with hyphen", "GET", "/api/v1/projects/proj-azuread-staging/namespaces"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nonWildcardEnforcerCalled = false

			req := httptest.NewRequest(tc.method, tc.path, nil)
			userCtx := &UserContext{
				UserID:         "user-123",
				Email:          "user@example.com",
				DisplayName:    "Regular User",
				Projects:       []string{},
				DefaultProject: "",
				CasbinRoles:    []string{},
			}
			ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200 for project namespace access, got %d", w.Code)
			}

			if nonWildcardEnforcerCalled {
				t.Error("non-wildcard enforcer should not be called for project namespace access")
			}
		})
	}
}

func TestCasbinAuthz_ProjectNamespacePostNotAllowed(t *testing.T) {
	t.Parallel()

	// POST to project namespaces should NOT use the bypass
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return false, nil // Deny
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when access denied")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("POST", "/api/v1/projects/engineering/namespaces", nil)
	userCtx := &UserContext{
		UserID:      "user-123",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for POST to project namespaces, got %d", w.Code)
	}
}

func TestCasbinAuthz_AccountInfoAccessAllUsers(t *testing.T) {
	t.Parallel()

	// Account info endpoint must be accessible to ALL authenticated users
	// without any Casbin resource-level permission check (AC-5 of STORY-202).
	// The enforcer IS called once for the global admin check (*, *) which is expected.
	// But it should NOT be called for resource-specific authorization.
	callCount := 0
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			callCount++
			if callCount == 1 {
				// First call is the admin check (*, *)
				if object != "*" || action != "*" {
					t.Errorf("expected first call to be admin check (*, *), got (%s, %s)", object, action)
				}
				return false, nil // User is not admin
			}
			// Any subsequent call means the bypass didn't work
			t.Errorf("enforcer called for resource-level check on /account/info: (%s, %s)", object, action)
			return false, nil
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Simulate a non-admin, project-scoped user (no wildcard policy)
	req := httptest.NewRequest("GET", "/api/v1/account/info", nil)
	userCtx := &UserContext{
		UserID:         "developer-user",
		Email:          "dev@example.com",
		DisplayName:    "Developer",
		Projects:       []string{"alpha"},
		DefaultProject: "alpha",
		CasbinRoles:    []string{"proj:alpha:developer"},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for account info (all authenticated users), got %d", w.Code)
	}
}

func TestCasbinAuthz_AccountInfoPostNotAllowed(t *testing.T) {
	t.Parallel()

	// POST to /api/v1/account/info should NOT be exempted from Casbin enforcement
	mockEnforcer := &mockCasbinPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return false, nil // Deny everything (including admin check)
		},
	}

	middleware := CasbinAuthz(CasbinAuthzConfig{
		Enforcer: mockEnforcer,
		Logger:   slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for POST to account info")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	req := httptest.NewRequest("POST", "/api/v1/account/info", nil)
	userCtx := &UserContext{
		UserID:         "user-123",
		Email:          "user@example.com",
		DisplayName:    "Regular User",
		Projects:       []string{"engineering"},
		DefaultProject: "engineering",
		CasbinRoles:    []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for POST to account info, got %d", w.Code)
	}
}
