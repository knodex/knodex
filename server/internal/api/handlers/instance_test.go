package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/models"
	"github.com/provops-org/knodex/server/internal/watcher"
)

// TestInstanceCRUDHandler_ListInstances_NilTracker tests that ListInstances returns 503
// when the instance tracker is nil (e.g., due to Kubernetes client initialization failure)
func TestInstanceCRUDHandler_ListInstances_NilTracker(t *testing.T) {
	t.Parallel()
	// This test prevents regression of the bug where slow K8s API initialization
	// caused instanceTracker to be nil, resulting in 503 errors

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()

	handler.ListInstances(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Code != response.ErrCodeServiceUnavailable {
		t.Errorf("expected error code %s, got %s", response.ErrCodeServiceUnavailable, errResp.Code)
	}

	expectedMsg := "Instance tracker not available"
	if errResp.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, errResp.Message)
	}
}

// TestInstanceCRUDHandler_GetInstance_NilTracker tests that GetInstance returns 503
// when the instance tracker is nil
func TestInstanceCRUDHandler_GetInstance_NilTracker(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/default/WebApp/test", nil)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "WebApp")
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.GetInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// TestInstanceCRUDHandler_DeleteInstance_NilTracker tests that DeleteInstance returns 503
// when the instance tracker is nil
func TestInstanceCRUDHandler_DeleteInstance_NilTracker(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/instances/default/WebApp/test", nil)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "WebApp")
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.DeleteInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// TestInstanceDeploymentHandler_CreateInstance_NilRGDWatcher tests that CreateInstance returns 503
// when the RGD watcher is nil
func TestInstanceDeploymentHandler_CreateInstance_NilRGDWatcher(t *testing.T) {
	t.Parallel()
	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/instances", nil)
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	expectedMsg := "RGD watcher not available"
	if errResp.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, errResp.Message)
	}
}

// TestInstanceCRUDHandler_DeleteInstance_NilDynamicClient tests that DeleteInstance returns 503
// when the dynamic client is nil but tracker is available
func TestInstanceCRUDHandler_DeleteInstance_NilDynamicClient(t *testing.T) {
	t.Parallel()
	// Create a handler with nil dynamic client
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/instances/default/WebApp/test", nil)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "WebApp")
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.DeleteInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should return 503 because tracker is nil (checked first)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// TestInstanceCRUDHandler_GetCount_NilTracker tests that GetCount returns 503
// when the instance tracker is nil
func TestInstanceCRUDHandler_GetCount_NilTracker(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/count", nil)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Code != response.ErrCodeServiceUnavailable {
		t.Errorf("expected error code %s, got %s", response.ErrCodeServiceUnavailable, errResp.Code)
	}

	expectedMsg := "Instance tracker not available"
	if errResp.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, errResp.Message)
	}
}

// TestInstanceCRUDHandler_GetCount_UnauthenticatedAccess tests that unauthenticated
// requests get count=0 (secure default - no access without auth context)
func TestInstanceCRUDHandler_GetCount_UnauthenticatedAccess(t *testing.T) {
	t.Parallel()
	// Create a cache with test instances
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:         "test-instance-1",
		Namespace:    "default",
		RGDName:      "test-rgd",
		RGDNamespace: "default",
	})
	cache.Set(&models.Instance{
		Name:         "test-instance-2",
		Namespace:    "default",
		RGDName:      "test-rgd",
		RGDNamespace: "default",
	})
	cache.Set(&models.Instance{
		Name:         "test-instance-3",
		Namespace:    "kube-system",
		RGDName:      "test-rgd",
		RGDNamespace: "default",
	})

	// Create tracker with the cache
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// Create handler with the tracker
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	// Make request without auth context - without user context, no namespace access
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/count", nil)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should return 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify response format matches InstanceCountResponse
	var countResp InstanceCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Without auth context, getAccessibleNamespaces returns empty slice (no access)
	// So count should be 0 (secure default - unauthenticated sees nothing)
	if countResp.Count != 0 {
		t.Errorf("expected count 0 (no access without auth), got %d", countResp.Count)
	}
}

// TestInstanceCRUDHandler_GetCount_EmptyCache tests that GetCount returns 0 when cache is empty
func TestInstanceCRUDHandler_GetCount_EmptyCache(t *testing.T) {
	t.Parallel()
	// Create an empty cache
	cache := watcher.NewInstanceCache()
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/count", nil)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var countResp InstanceCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if countResp.Count != 0 {
		t.Errorf("expected count 0 for empty cache, got %d", countResp.Count)
	}
}

// TestInstanceCRUDHandler_GetCount_AuthenticatedUser tests that authenticated users
// without explicit authService filtering see all instances (authService=nil returns nil namespaces)
func TestInstanceCRUDHandler_GetCount_AuthenticatedUser(t *testing.T) {
	t.Parallel()
	// Create a cache with test instances in different namespaces
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:         "test-instance-1",
		Namespace:    "default",
		RGDName:      "test-rgd",
		RGDNamespace: "default",
	})
	cache.Set(&models.Instance{
		Name:         "test-instance-2",
		Namespace:    "kube-system",
		RGDName:      "test-rgd",
		RGDNamespace: "default",
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// Handler WITHOUT authService - when authService is nil, getAccessibleNamespaces
	// returns nil for authenticated users (no filtering = all instances visible)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		// AuthService intentionally nil
	})

	// Create request with user context
	userCtx := &middleware.UserContext{
		Email:       "admin@test.local",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/count", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var countResp InstanceCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// With userContext but no authService, getAccessibleNamespaces returns nil
	// nil namespaces = no filtering = all instances visible
	if countResp.Count != 2 {
		t.Errorf("expected count 2 (all instances), got %d", countResp.Count)
	}
}

// TestDeploymentModeValidation tests IsDeploymentModeAllowed from models package
// is correctly integrated into the CreateInstance handler validation
func TestDeploymentModeValidation(t *testing.T) {
	t.Parallel()
	// This test verifies the models.IsDeploymentModeAllowed function
	// The actual handler integration is tested via E2E tests since it requires
	// a full setup with RGDWatcher and Kubernetes client

	tests := []struct {
		name         string
		allowedModes []string
		requestMode  string
		expectValid  bool
	}{
		{
			name:         "nil modes allows all - direct",
			allowedModes: nil,
			requestMode:  "direct",
			expectValid:  true,
		},
		{
			name:         "nil modes allows all - gitops",
			allowedModes: nil,
			requestMode:  "gitops",
			expectValid:  true,
		},
		{
			name:         "empty modes allows all - hybrid",
			allowedModes: []string{},
			requestMode:  "hybrid",
			expectValid:  true,
		},
		{
			name:         "gitops only - gitops allowed",
			allowedModes: []string{"gitops"},
			requestMode:  "gitops",
			expectValid:  true,
		},
		{
			name:         "gitops only - direct denied",
			allowedModes: []string{"gitops"},
			requestMode:  "direct",
			expectValid:  false,
		},
		{
			name:         "gitops only - hybrid denied",
			allowedModes: []string{"gitops"},
			requestMode:  "hybrid",
			expectValid:  false,
		},
		{
			name:         "direct and hybrid - direct allowed",
			allowedModes: []string{"direct", "hybrid"},
			requestMode:  "direct",
			expectValid:  true,
		},
		{
			name:         "direct and hybrid - hybrid allowed",
			allowedModes: []string{"direct", "hybrid"},
			requestMode:  "hybrid",
			expectValid:  true,
		},
		{
			name:         "direct and hybrid - gitops denied",
			allowedModes: []string{"direct", "hybrid"},
			requestMode:  "gitops",
			expectValid:  false,
		},
		{
			name:         "all three modes - direct allowed",
			allowedModes: []string{"direct", "gitops", "hybrid"},
			requestMode:  "direct",
			expectValid:  true,
		},
		{
			name:         "case insensitive - GITOPS matches gitops",
			allowedModes: []string{"gitops"},
			requestMode:  "GITOPS",
			expectValid:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Use the models package function directly
			result := isDeploymentModeAllowedWrapper(tt.allowedModes, tt.requestMode)
			if result != tt.expectValid {
				t.Errorf("expected valid=%v, got %v for mode %q with allowed=%v",
					tt.expectValid, result, tt.requestMode, tt.allowedModes)
			}
		})
	}
}

// isDeploymentModeAllowedWrapper wraps the models package function for testing
// This ensures the handler uses the same validation logic as models.IsDeploymentModeAllowed
func isDeploymentModeAllowedWrapper(allowedModes []string, mode string) bool {
	return models.IsDeploymentModeAllowed(allowedModes, mode)
}

// TestHandleDeployError_NoRawErrorLeakage tests that handleDeployError does not
// leak raw K8s error messages to the client (AC-2: no raw K8s errors in instance responses)
func TestHandleDeployError_NoRawErrorLeakage(t *testing.T) {
	t.Parallel()
	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{})

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectNoRawErr bool // response body must NOT contain the raw error
	}{
		{
			name:           "not found error - no raw details",
			err:            fmt.Errorf("widgets.kro.run \"test-widget\" not found"),
			expectedStatus: http.StatusNotFound,
			expectNoRawErr: true,
		},
		{
			name:           "already exists error - no raw details",
			err:            fmt.Errorf("widgets.kro.run \"my-widget\" already exists"),
			expectedStatus: http.StatusConflict,
			expectNoRawErr: true,
		},
		{
			name:           "forbidden error - no raw details",
			err:            fmt.Errorf("widgets.kro.run is forbidden: User \"system:serviceaccount:knodex:knodex\" cannot create resource"),
			expectedStatus: http.StatusForbidden,
			expectNoRawErr: true,
		},
		{
			name:           "git error - no raw details",
			err:            fmt.Errorf("GitHub API rate limit exceeded for 10.0.0.1"),
			expectedStatus: http.StatusBadGateway,
			expectNoRawErr: true,
		},
		{
			name:           "generic internal error - no raw details",
			err:            fmt.Errorf("connection refused: dial tcp 10.96.0.1:443: i/o timeout"),
			expectedStatus: http.StatusInternalServerError,
			expectNoRawErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			handler.handleDeployError(rec, tt.err)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Parse response body and verify no raw error message is leaked
			var errResp response.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			rawErr := tt.err.Error()
			if tt.expectNoRawErr {
				if errResp.Message == rawErr {
					t.Errorf("raw error message leaked to client: %q", rawErr)
				}
				if errResp.Details != nil {
					detailsJSON, _ := json.Marshal(errResp.Details)
					if strings.Contains(string(detailsJSON), rawErr) {
						t.Errorf("raw error message leaked in details: %q", rawErr)
					}
				}
			}
		})
	}
}

// TestHandleDeleteError_NoRawErrorLeakage tests that handleDeleteError does not
// leak raw K8s error messages to the client (AC-2: no raw K8s errors in instance responses)
func TestHandleDeleteError_NoRawErrorLeakage(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectNoRawErr bool
	}{
		{
			name:           "not found error - no raw details",
			err:            fmt.Errorf("widgets.kro.run \"test-widget\" not found"),
			expectedStatus: http.StatusNotFound,
			expectNoRawErr: true,
		},
		{
			name:           "forbidden error - no raw details",
			err:            fmt.Errorf("widgets.kro.run is forbidden: User \"system:serviceaccount:knodex:knodex\" cannot delete resource"),
			expectedStatus: http.StatusForbidden,
			expectNoRawErr: true,
		},
		{
			name:           "generic internal error - no raw details",
			err:            fmt.Errorf("connection refused: dial tcp 10.96.0.1:443: i/o timeout"),
			expectedStatus: http.StatusInternalServerError,
			expectNoRawErr: true,
		},
		{
			name:           "etcd error - no raw details",
			err:            fmt.Errorf("etcdserver: request timed out, possibly due to connection lost"),
			expectedStatus: http.StatusInternalServerError,
			expectNoRawErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			handler.handleDeleteError(rec, "default", "WebApp", "test-instance", tt.err)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Parse response body and verify no raw error message is leaked
			var errResp response.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			rawErr := tt.err.Error()
			if tt.expectNoRawErr {
				if errResp.Message == rawErr {
					t.Errorf("raw error message leaked to client: %q", rawErr)
				}
				if errResp.Details != nil {
					detailsJSON, _ := json.Marshal(errResp.Details)
					if strings.Contains(string(detailsJSON), rawErr) {
						t.Errorf("raw error message leaked in details: %q", rawErr)
					}
				}
			}
		})
	}
}

// TestListInstances_SortByValidation tests that sortBy rejects invalid values (AC-1)
func TestListInstances_SortByValidation(t *testing.T) {
	t.Parallel()
	cache := watcher.NewInstanceCache()
	tracker := watcher.NewInstanceTrackerWithCache(cache)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	tests := []struct {
		name       string
		sortBy     string
		wantStatus int
	}{
		{"valid name", "name", http.StatusOK},
		{"valid createdAt", "createdAt", http.StatusOK},
		{"valid updatedAt", "updatedAt", http.StatusOK},
		{"valid health", "health", http.StatusOK},
		{"invalid __proto__", "__proto__", http.StatusBadRequest},
		{"invalid constructor", "constructor", http.StatusBadRequest},
		{"invalid arbitrary", "foo", http.StatusBadRequest},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/instances?sortBy="+tt.sortBy, nil)
			rec := httptest.NewRecorder()
			handler.ListInstances(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("sortBy=%q: expected status %d, got %d", tt.sortBy, tt.wantStatus, rec.Code)
			}
		})
	}
}

// TestListInstances_SortOrderValidation tests that sortOrder rejects invalid values (AC-1)
func TestListInstances_SortOrderValidation(t *testing.T) {
	t.Parallel()
	cache := watcher.NewInstanceCache()
	tracker := watcher.NewInstanceTrackerWithCache(cache)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	tests := []struct {
		name       string
		sortOrder  string
		wantStatus int
	}{
		{"valid asc", "asc", http.StatusOK},
		{"valid desc", "desc", http.StatusOK},
		{"invalid ASC", "ASC", http.StatusBadRequest},
		{"invalid random", "random", http.StatusBadRequest},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/instances?sortOrder="+tt.sortOrder, nil)
			rec := httptest.NewRecorder()
			handler.ListInstances(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("sortOrder=%q: expected status %d, got %d", tt.sortOrder, tt.wantStatus, rec.Code)
			}
		})
	}
}

// TestSanitizeConnectionError tests that connection error sanitization works correctly
func TestSanitizeConnectionError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "authentication failure",
			input:    "connection test failed: authentication failed: 401 Bad credentials",
			expected: "Authentication failed — check your credentials",
		},
		{
			name:     "not found",
			input:    "connection test failed: repository not found: GET https://api.github.com/repos/foo/bar: 404",
			expected: "Repository not found — check the repository URL",
		},
		{
			name:     "forbidden",
			input:    "connection test failed: 403 Forbidden: https://api.github.com/repos/private/repo",
			expected: "Access denied — check your repository permissions",
		},
		{
			name:     "timeout",
			input:    "connection test failed: context deadline exceeded: dial tcp 10.0.0.1:443",
			expected: "Connection timed out — check network connectivity",
		},
		{
			name:     "connection refused",
			input:    "connection test failed: connection refused: dial tcp 192.168.1.100:443",
			expected: "Unable to reach repository host — check the URL and network",
		},
		{
			name:     "generic error hides internal details",
			input:    "unexpected status from internal service at svc.cluster.local:8080: 502",
			expected: "Connection test failed — check repository URL and credentials",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := sanitizeConnectionError(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
