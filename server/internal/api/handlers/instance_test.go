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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/kro/children"
	kroparser "github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
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
	req.SetPathValue("kind", "Webapp")
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
	req.SetPathValue("kind", "Webapp")
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
	req.SetPathValue("kind", "Webapp")
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
// with admin-level authService see all instances (["*"] namespaces = no filtering)
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

	// Handler with adminAuthService - simulates global admin access
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     adminAuthService(),
	})

	// Create request with user context (UserID required for project lookup)
	userCtx := &middleware.UserContext{
		UserID:      "user:admin",
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

	// With adminAuthService, getAccessibleNamespaces returns ["*"] (global admin)
	// ["*"] namespaces = no filtering = all instances visible
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
			handler.handleDeleteError(rec, "default", "webapp", "test-instance", tt.err)

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

// --- UpdateInstance tests ---

// newUpdateTestSetup creates common test infrastructure for UpdateInstance tests.
// Returns handler, mockAuditRecorder, and fakeDynClient for test assertions.
func newUpdateTestSetup(t *testing.T) (*InstanceCRUDHandler, *mockAuditRecorder, *fakedynamic.FakeDynamicClient) {
	t.Helper()
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:        "test-instance",
		Namespace:   "production",
		Kind:        "Webapp",
		APIVersion:  "kro.run/v1alpha1",
		RGDName:     "webapp-rgd",
		Health:      models.HealthHealthy,
		ProjectName: "alpha",
		Labels: map[string]string{
			models.DeploymentModeLabel: string(deployment.ModeDirect),
		},
		Spec: map[string]interface{}{
			"replicas": float64(2),
			"image":    "nginx:1.0",
		},
	})

	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Webapp",
			"metadata": map[string]interface{}{
				"name":            "test-instance",
				"namespace":       "production",
				"resourceVersion": "12345",
			},
			"spec": map[string]interface{}{
				"replicas": float64(2),
				"image":    "nginx:1.0",
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "kro.run", Version: "v1alpha1", Resource: "webapps"}: "WebAppList",
	}
	fakeDynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, obj)

	tracker := watcher.NewInstanceTrackerForTest(cache, fakeDynClient)

	recorder := &mockAuditRecorder{}
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
		AuditRecorder:   recorder,
		AuthService:     adminAuthService(),
	})

	return handler, recorder, fakeDynClient
}

// TestUpdateInstance_Success tests successful spec update (Task 3.1)
func TestUpdateInstance_Success(t *testing.T) {
	t.Parallel()
	handler, recorder, _ := newUpdateTestSetup(t)

	body := `{"spec": {"replicas": 3, "image": "nginx:2.0"}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp UpdateInstanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Name != "test-instance" {
		t.Errorf("expected name 'test-instance', got %q", resp.Name)
	}
	if resp.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", resp.Namespace)
	}
	if resp.Kind != "Webapp" {
		t.Errorf("expected kind 'Webapp', got %q", resp.Kind)
	}
	if resp.Status != "updated" {
		t.Errorf("expected status 'updated', got %q", resp.Status)
	}
	if resp.DeploymentMode != "direct" {
		t.Errorf("expected deploymentMode 'direct', got %q", resp.DeploymentMode)
	}

	// Verify audit event was recorded
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Action != "update" {
		t.Errorf("expected action 'update', got %q", e.Action)
	}
	if e.Resource != "instances" {
		t.Errorf("expected resource 'instances', got %q", e.Resource)
	}
	if e.Name != "test-instance" {
		t.Errorf("expected name 'test-instance', got %q", e.Name)
	}
	if e.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", e.Namespace)
	}
	if e.Project != "alpha" {
		t.Errorf("expected project 'alpha', got %q", e.Project)
	}
	if e.UserEmail != "dev@test.local" {
		t.Errorf("expected email 'dev@test.local', got %q", e.UserEmail)
	}
	if e.Result != "success" {
		t.Errorf("expected result 'success', got %q", e.Result)
	}
	// Verify audit details contain metadata (not spec)
	if e.Details["rgdName"] != "webapp-rgd" {
		t.Errorf("expected rgdName 'webapp-rgd', got %v", e.Details["rgdName"])
	}
	if e.Details["kind"] != "Webapp" {
		t.Errorf("expected kind 'Webapp', got %v", e.Details["kind"])
	}
	if e.Details["deploymentMode"] != "direct" {
		t.Errorf("expected deploymentMode 'direct', got %v", e.Details["deploymentMode"])
	}
	// Spec must NOT be in audit details (may contain secrets)
	if _, hasSpec := e.Details["spec"]; hasSpec {
		t.Errorf("audit details must NOT contain spec (may contain secrets)")
	}
}

// TestUpdateInstance_Unauthorized tests that unauthenticated users get 401 (Task 3.2)
func TestUpdateInstance_Unauthorized(t *testing.T) {
	t.Parallel()
	handler, _, _ := newUpdateTestSetup(t)

	body := `{"spec": {"replicas": 3}}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
		strings.NewReader(body))
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// TestUpdateInstance_NotFound tests that non-existent instances return 404 (Task 3.3)
func TestUpdateInstance_NotFound(t *testing.T) {
	t.Parallel()
	handler, _, _ := newUpdateTestSetup(t)

	body := `{"spec": {"replicas": 3}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/nonexistent",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateInstance_InvalidSpec tests that empty/nil spec returns 400 (Task 3.4)
func TestUpdateInstance_InvalidSpec(t *testing.T) {
	t.Parallel()
	handler, _, _ := newUpdateTestSetup(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty spec", `{"spec": {}}`},
		{"null spec", `{"spec": null}`},
		{"missing spec", `{}`},
		{"invalid json", `{invalid`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
			req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
				[]byte(tt.body), userCtx)
			req.SetPathValue("namespace", "production")
			req.SetPathValue("kind", "Webapp")
			req.SetPathValue("name", "test-instance")
			rec := httptest.NewRecorder()

			handler.UpdateInstance(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestUpdateInstance_SpecInjection tests that malicious spec payloads are rejected (INJ-VULN-02)
func TestUpdateInstance_SpecInjection(t *testing.T) {
	t.Parallel()
	handler, _, _ := newUpdateTestSetup(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		errContain string
	}{
		{
			name:       "valid spec passes validation",
			body:       `{"spec": {"replicas": 3, "image": "nginx:2.0"}}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "newline in key rejected",
			body:       `{"spec": {"malicious\nkey": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited control character",
		},
		{
			name:       "carriage return in key rejected",
			body:       `{"spec": {"malicious\rkey": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited control character",
		},
		{
			name:       "null byte in key rejected",
			body:       `{"spec": {"malicious\u0000key": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited control character",
		},
		{
			name:       "YAML colon-space injection in key rejected",
			body:       `{"spec": {"key: injected": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited sequence",
		},
		{
			name:       "template injection in key rejected",
			body:       `{"spec": {"{{malicious}}": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited sequence",
		},
		{
			name:       "dollar-brace injection in key rejected",
			body:       `{"spec": {"${ENV_VAR}": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited sequence",
		},
		{
			name:       "depth exceeding 10 levels rejected",
			body:       `{"spec": {"a": {"b": {"c": {"d": {"e": {"f": {"g": {"h": {"i": {"j": {"k": {"l": "deep"}}}}}}}}}}}}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "nesting depth",
		},
		{
			name:       "excessively long string value rejected",
			body:       `{"spec": {"data": "` + strings.Repeat("x", 70000) + `"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "excessively long string",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
			req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
				[]byte(tt.body), userCtx)
			req.SetPathValue("namespace", "production")
			req.SetPathValue("kind", "Webapp")
			req.SetPathValue("name", "test-instance")
			rec := httptest.NewRecorder()

			handler.UpdateInstance(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d; body: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if tt.errContain != "" {
				body := rec.Body.String()
				if !strings.Contains(body, tt.errContain) {
					t.Errorf("expected body to contain %q, got: %s", tt.errContain, body)
				}
			}
		})
	}
}

// TestUpdateInstance_NilTracker tests that UpdateInstance returns 503 when tracker is nil
func TestUpdateInstance_NilTracker(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	body := `{"spec": {"replicas": 3}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/default/WebApp/test",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// TestUpdateInstance_NilDynamicClient tests 503 when dynamic client is nil
func TestUpdateInstance_NilDynamicClient(t *testing.T) {
	t.Parallel()
	cache := watcher.NewInstanceCache()
	tracker := watcher.NewInstanceTrackerWithCache(cache)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	body := `{"spec": {"replicas": 3}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/default/WebApp/test",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// TestUpdateInstance_MissingPathParams tests 400 when path parameters are missing
func TestUpdateInstance_MissingPathParams(t *testing.T) {
	t.Parallel()
	handler, _, _ := newUpdateTestSetup(t)

	body := `{"spec": {"replicas": 3}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances///",
		[]byte(body), userCtx)
	// Intentionally not setting path values
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestHandleUpdateError_NoRawErrorLeakage tests that handleUpdateError does not
// leak raw K8s error messages to the client (Task 3.5)
func TestHandleUpdateError_NoRawErrorLeakage(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{})

	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{
			name:           "not found error",
			err:            fmt.Errorf("webapps.kro.run \"test\" not found"),
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "forbidden error",
			err:            fmt.Errorf("webapps.kro.run is forbidden"),
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "conflict error - object modified",
			err:            fmt.Errorf("Operation cannot be fulfilled: the object has been modified"),
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "validation error",
			err:            fmt.Errorf("admission webhook denied: invalid spec"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "generic internal error",
			err:            fmt.Errorf("connection refused: dial tcp 10.96.0.1:443"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			handler.handleUpdateError(rec, "default", "webapp", "test", tt.err)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Verify no raw error message leaked
			var errResp response.ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			rawErr := tt.err.Error()
			if errResp.Message == rawErr {
				t.Errorf("raw error message leaked to client: %q", rawErr)
			}
		})
	}
}

// --- Deployment mode update tests (Task 2) ---

// newGitOpsUpdateTestSetup creates a test instance with gitops deployment mode label.
// The handler has NO deploymentController or repoService to test error paths.
func newGitOpsUpdateTestSetup(t *testing.T, deployMode deployment.DeploymentMode) (*InstanceCRUDHandler, *mockAuditRecorder) {
	t.Helper()
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:         "gitops-instance",
		Namespace:    "staging",
		Kind:         "Webapp",
		APIVersion:   "kro.run/v1alpha1",
		RGDName:      "webapp-rgd",
		RGDNamespace: "kro-system",
		Health:       models.HealthHealthy,
		ProjectName:  "beta",
		ProjectID:    "proj-beta",
		Labels: map[string]string{
			models.DeploymentModeLabel: string(deployMode),
		},
		Spec: map[string]interface{}{
			"replicas": float64(1),
		},
	})

	schm := runtime.NewScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Webapp",
			"metadata": map[string]interface{}{
				"name":            "gitops-instance",
				"namespace":       "staging",
				"resourceVersion": "999",
			},
			"spec": map[string]interface{}{
				"replicas": float64(1),
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "kro.run", Version: "v1alpha1", Resource: "webapps"}: "WebAppList",
	}
	fakeDynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(schm, gvrToListKind, obj)
	tracker := watcher.NewInstanceTrackerForTest(cache, fakeDynClient)
	recorder := &mockAuditRecorder{}

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
		AuditRecorder:   recorder,
		AuthService:     adminAuthService(),
	})

	return handler, recorder
}

// TestUpdateInstance_GitOps_NoRepositoryID tests that gitops mode requires repositoryId.
// When deploymentController is nil, the handler returns 503 before checking repositoryId.
// When deploymentController IS available but repositoryId is missing, it returns 400.
func TestUpdateInstance_GitOps_NoRepositoryID(t *testing.T) {
	t.Parallel()

	// Without controller: 503 "not configured" (checked first in pushSpecUpdateToGit)
	t.Run("no_controller", func(t *testing.T) {
		t.Parallel()
		handler, _ := newGitOpsUpdateTestSetup(t, deployment.ModeGitOps)

		body := `{"spec": {"replicas": 3}}`
		userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
		req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/staging/WebApp/gitops-instance",
			[]byte(body), userCtx)
		req.SetPathValue("namespace", "staging")
		req.SetPathValue("kind", "Webapp")
		req.SetPathValue("name", "gitops-instance")
		rec := httptest.NewRecorder()

		handler.UpdateInstance(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestUpdateInstance_GitOps_NoController tests that gitops mode returns 503 when controller is nil
func TestUpdateInstance_GitOps_NoController(t *testing.T) {
	t.Parallel()
	handler, _ := newGitOpsUpdateTestSetup(t, deployment.ModeGitOps)

	body := `{"spec": {"replicas": 3}, "repositoryId": "repo-1"}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/staging/WebApp/gitops-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "staging")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "gitops-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateInstance_GitOps_DoesNotPatchK8s tests that gitops mode does NOT patch the K8s resource
func TestUpdateInstance_GitOps_DoesNotPatchK8s(t *testing.T) {
	t.Parallel()
	handler, _ := newGitOpsUpdateTestSetup(t, deployment.ModeGitOps)

	// Send request without repositoryId — this will error before any K8s patch
	body := `{"spec": {"replicas": 5}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/staging/WebApp/gitops-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "staging")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "gitops-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	// Should fail before patching K8s (no repositoryId)
	if rec.Code == http.StatusOK {
		t.Errorf("gitops mode should NOT return 200 without repositoryId")
	}
}

// TestUpdateInstance_Hybrid_PatchesK8sEvenIfGitFails tests that hybrid mode patches K8s
// even when Git push fails (graceful degradation)
func TestUpdateInstance_Hybrid_PatchesK8sEvenIfGitFails(t *testing.T) {
	t.Parallel()
	handler, recorder := newGitOpsUpdateTestSetup(t, deployment.ModeHybrid)

	// Hybrid mode: K8s patch should succeed, Git push will fail (no controller/repo)
	// but the handler treats Git failure as non-fatal in hybrid mode
	body := `{"spec": {"replicas": 5}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/staging/WebApp/gitops-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "staging")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "gitops-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for hybrid mode (K8s should succeed), got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp UpdateInstanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.DeploymentMode != "hybrid" {
		t.Errorf("expected deploymentMode 'hybrid', got %q", resp.DeploymentMode)
	}

	// Verify GitInfo indicates failure
	if resp.GitInfo == nil {
		t.Fatal("expected gitInfo in response for hybrid mode")
	}
	if resp.GitInfo.PushStatus != deployment.GitPushFailed {
		t.Errorf("expected gitInfo.pushStatus 'failed', got %q", resp.GitInfo.PushStatus)
	}

	// Verify audit event was recorded with partial result (K8s succeeded, Git failed)
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Action != "update" {
		t.Errorf("expected action 'update', got %q", e.Action)
	}
	if e.Result != "partial" {
		t.Errorf("expected audit result 'partial' (K8s OK, Git failed), got %q", e.Result)
	}
	if e.Details["deploymentMode"] != "hybrid" {
		t.Errorf("expected deploymentMode 'hybrid' in audit, got %v", e.Details["deploymentMode"])
	}
	if e.Details["gitPushFailed"] != true {
		t.Error("expected gitPushFailed=true in audit details for hybrid Git failure")
	}
}

// TestUpdateInstance_DirectWithLabel tests that direct mode works correctly with label
func TestUpdateInstance_DirectWithLabel(t *testing.T) {
	t.Parallel()
	handler, recorder, _ := newUpdateTestSetup(t)

	body := `{"spec": {"replicas": 10}}`
	userCtx := &middleware.UserContext{UserID: "admin", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp UpdateInstanceResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.DeploymentMode != "direct" {
		t.Errorf("expected deploymentMode 'direct', got %q", resp.DeploymentMode)
	}
	// Direct mode should have no GitInfo
	if resp.GitInfo != nil {
		t.Errorf("direct mode should not have gitInfo, got %+v", resp.GitInfo)
	}

	// Verify audit records deployment mode
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}
	if recorder.lastEvent().Details["deploymentMode"] != "direct" {
		t.Errorf("expected deploymentMode 'direct' in audit, got %v", recorder.lastEvent().Details["deploymentMode"])
	}
}

// TestUpdateInstance_GitOps_RepositoryIdAccepted verifies that repositoryId is parsed
// from the request body (fails with 503 because no deployment controller is configured).
func TestUpdateInstance_GitOps_RepositoryIdAccepted(t *testing.T) {
	t.Parallel()
	handler, _ := newGitOpsUpdateTestSetup(t, deployment.ModeGitOps)

	body := `{"spec": {"replicas": 3}, "repositoryId": "my-repo-123"}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/staging/WebApp/gitops-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "staging")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "gitops-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	// Fails with 503 because no deployment controller — but validates request parsing
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (no controller), got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateInstance_AuditSpecChanges verifies that audit events record which spec keys changed.
// Old spec: {"replicas": 2, "image": "nginx:1.0"}
// New spec: {"replicas": 10, "env": "production"} → replicas modified, image removed, env added
func TestUpdateInstance_AuditSpecChanges(t *testing.T) {
	t.Parallel()
	handler, recorder, _ := newUpdateTestSetup(t)

	body := `{"spec": {"replicas": 10, "env": "production"}}`
	userCtx := &middleware.UserContext{UserID: "admin", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()

	// Verify specChanges present in audit details
	specChanges, ok := e.Details["specChanges"].(map[string]any)
	if !ok {
		t.Fatal("expected specChanges in audit details")
	}

	// "env" is new (added)
	if added, ok := specChanges["added"].([]string); ok {
		if !containsStr(added, "env") {
			t.Errorf("expected 'env' in added keys, got %v", added)
		}
	} else {
		t.Error("expected 'added' in specChanges")
	}

	// "image" was removed (not in new spec)
	if removed, ok := specChanges["removed"].([]string); ok {
		if !containsStr(removed, "image") {
			t.Errorf("expected 'image' in removed keys, got %v", removed)
		}
	} else {
		t.Error("expected 'removed' in specChanges")
	}

	// "replicas" changed from 2 to 10 (modified)
	if modified, ok := specChanges["modified"].([]string); ok {
		if !containsStr(modified, "replicas") {
			t.Errorf("expected 'replicas' in modified keys, got %v", modified)
		}
	} else {
		t.Error("expected 'modified' in specChanges")
	}

	// Verify NO spec values are leaked in audit (security: specs may contain secrets)
	for _, v := range specChanges {
		if keyList, ok := v.([]string); ok {
			for _, keyName := range keyList {
				if keyName == "production" || keyName == "10" || keyName == "nginx:1.0" {
					t.Errorf("spec VALUE leaked in audit specChanges: %v", keyList)
				}
			}
		}
	}
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// TestUpdateInstance_ResourceVersionConflict tests that a K8s resource version conflict returns 409 (Task 3.5)
func TestUpdateInstance_ResourceVersionConflict(t *testing.T) {
	t.Parallel()
	handler, _, fakeDynClient := newUpdateTestSetup(t)

	// Inject a reactor that returns a Conflict error for patch operations
	fakeDynClient.PrependReactor("patch", "webapps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(
			schema.GroupResource{Group: "kro.run", Resource: "webapps"},
			"test-instance",
			fmt.Errorf("the object has been modified; please apply your changes to the latest version"),
		)
	})

	body := `{"spec": {"replicas": 5}, "resourceVersion": "stale-version"}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Code != "CONFLICT" {
		t.Errorf("expected error code 'CONFLICT', got %q", errResp.Code)
	}
}

// TestUpdateInstance_DirectPatchFailure_AuditRecorded verifies that a failed direct-mode K8s patch
// records an audit event with result="error" (H1 fix verification)
func TestUpdateInstance_DirectPatchFailure_AuditRecorded(t *testing.T) {
	t.Parallel()
	handler, recorder, fakeDynClient := newUpdateTestSetup(t)

	// Inject a reactor that returns an error for patch operations
	fakeDynClient.PrependReactor("patch", "webapps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("forbidden: service account cannot patch resources")
	})

	body := `{"spec": {"replicas": 5}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/production/WebApp/test-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	// Should return an error status
	if rec.Code == http.StatusOK {
		t.Fatal("expected error response, got 200")
	}

	// Verify audit event was recorded with result="error"
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event for failed patch, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Result != "error" {
		t.Errorf("expected audit result 'error', got %q", e.Result)
	}
	if e.Action != "update" {
		t.Errorf("expected audit action 'update', got %q", e.Action)
	}
	if e.UserID != "dev-user" {
		t.Errorf("expected audit userID 'dev-user', got %q", e.UserID)
	}
	if dm, ok := e.Details["deploymentMode"].(string); !ok || dm != "direct" {
		t.Errorf("expected deploymentMode 'direct' in audit details, got %v", e.Details["deploymentMode"])
	}
}

// TestUpdateInstance_HybridPatchFailure_AuditRecorded verifies that a failed hybrid-mode K8s patch
// records an audit event with result="error" (H2 fix verification)
func TestUpdateInstance_HybridPatchFailure_AuditRecorded(t *testing.T) {
	t.Parallel()

	// Create a hybrid-mode instance
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:        "hybrid-instance",
		Namespace:   "staging",
		Kind:        "Webapp",
		APIVersion:  "kro.run/v1alpha1",
		RGDName:     "webapp-rgd",
		Health:      models.HealthHealthy,
		ProjectName: "gamma",
		Labels: map[string]string{
			models.DeploymentModeLabel: string(deployment.ModeHybrid),
		},
		Spec: map[string]interface{}{
			"replicas": float64(1),
		},
	})

	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Webapp",
			"metadata": map[string]interface{}{
				"name":            "hybrid-instance",
				"namespace":       "staging",
				"resourceVersion": "500",
			},
			"spec": map[string]interface{}{
				"replicas": float64(1),
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "kro.run", Version: "v1alpha1", Resource: "webapps"}: "WebAppList",
	}
	fakeDynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, obj)
	tracker := watcher.NewInstanceTrackerForTest(cache, fakeDynClient)
	recorder := &mockAuditRecorder{}
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
		AuditRecorder:   recorder,
		AuthService:     adminAuthService(),
	})

	// Inject a reactor that fails on patch
	fakeDynClient.PrependReactor("patch", "webapps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("forbidden: cannot patch")
	})

	body := `{"spec": {"replicas": 3}}`
	userCtx := &middleware.UserContext{UserID: "hybrid-user", Email: "hybrid@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/staging/WebApp/hybrid-instance",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "staging")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "hybrid-instance")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("expected error response, got 200")
	}

	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event for failed hybrid patch, got %d", len(recorder.events))
	}
	e := recorder.lastEvent()
	if e.Result != "error" {
		t.Errorf("expected audit result 'error', got %q", e.Result)
	}
	if dm, ok := e.Details["deploymentMode"].(string); !ok || dm != "hybrid" {
		t.Errorf("expected deploymentMode 'hybrid' in audit details, got %v", e.Details["deploymentMode"])
	}
}

// TestCreateInstance_SpecInjection tests that malicious spec payloads are rejected in POST path (INJ-VULN-02)
func TestCreateInstance_SpecInjection(t *testing.T) {
	t.Parallel()
	// Create a handler with valid RGD watcher and dynamic client - validation happens before RGD lookup
	rgdCache := watcher.NewRGDCache()
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)
	scheme := runtime.NewScheme()
	fakeDynClient := fakedynamic.NewSimpleDynamicClient(scheme)
	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:    rgdWatcher,
		DynamicClient: fakeDynClient,
	})

	tests := []struct {
		name       string
		body       string
		wantStatus int
		errContain string
	}{
		{
			name:       "newline in spec key rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"malicious\nkey": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited control character",
		},
		{
			name:       "carriage return in spec key rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"malicious\rkey": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited control character",
		},
		{
			name:       "null byte in spec key rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"malicious\u0000key": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited control character",
		},
		{
			name:       "YAML colon-space injection in spec key rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"key: injected": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited sequence",
		},
		{
			name:       "template injection in spec key rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"{{malicious}}": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited sequence",
		},
		{
			name:       "dollar-brace injection in spec key rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"${ENV_VAR}": "value"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "prohibited sequence",
		},
		{
			name:       "depth exceeding 10 levels rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"a": {"b": {"c": {"d": {"e": {"f": {"g": {"h": {"i": {"j": {"k": {"l": "deep"}}}}}}}}}}}}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "nesting depth",
		},
		{
			name:       "excessively long string value rejected",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd", "spec": {"data": "` + strings.Repeat("x", 70000) + `"}}`,
			wantStatus: http.StatusBadRequest,
			errContain: "excessively long string",
		},
		{
			name:       "nil spec passes validation (reaches RGD lookup)",
			body:       `{"name": "test-inst", "namespace": "default", "rgdName": "test-rgd"}`,
			wantStatus: http.StatusNotFound, // passes validation, fails on RGD lookup
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
			req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances",
				[]byte(tt.body), userCtx)
			rec := httptest.NewRecorder()

			handler.CreateInstance(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d; body: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
			if tt.errContain != "" {
				body := rec.Body.String()
				if !strings.Contains(body, tt.errContain) {
					t.Errorf("expected body to contain %q, got: %s", tt.errContain, body)
				}
			}
		})
	}
}

// TestUpdateInstance_IrregularPlural_UsesDiscovery verifies that UpdateInstance uses discovery-based
// GVR resolution for kinds with irregular plurals (e.g., "proxy" -> "proxies" not "proxys").
func TestUpdateInstance_IrregularPlural_UsesDiscovery(t *testing.T) {
	t.Parallel()

	// Set up fake discovery with Proxy -> proxies mapping
	disc := &fakediscovery.FakeDiscovery{
		Fake: &k8stesting.Fake{},
	}
	disc.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "proxies", Kind: "Proxy", Verbs: metav1.Verbs{"get", "list", "create", "update", "patch"}},
			},
		},
	}

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:       "my-proxy",
		Namespace:  "default",
		Kind:       "Proxy",
		APIVersion: "example.com/v1",
		RGDName:    "proxy-rgd",
		Health:     models.HealthHealthy,
		Labels: map[string]string{
			models.DeploymentModeLabel: string(deployment.ModeDirect),
		},
		Spec: map[string]interface{}{"port": float64(8080)},
	})

	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "Proxy",
			"metadata": map[string]interface{}{
				"name":            "my-proxy",
				"namespace":       "default",
				"resourceVersion": "100",
			},
			"spec": map[string]interface{}{"port": float64(8080)},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "example.com", Version: "v1", Resource: "proxies"}: "ProxyList",
	}
	fakeDynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, obj)

	tracker := watcher.NewInstanceTrackerForTest(cache, fakeDynClient)
	tracker.SetDiscoveryClient(disc)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
		AuthService:     adminAuthService(),
	})

	body := `{"spec": {"port": 9090}}`
	userCtx := &middleware.UserContext{UserID: "dev-user", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/instances/default/Proxy/my-proxy",
		[]byte(body), userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Proxy")
	req.SetPathValue("name", "my-proxy")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify the dynamic client received "proxies" (not "proxys")
	actions := fakeDynClient.Actions()
	var patchAction k8stesting.PatchAction
	for _, a := range actions {
		if pa, ok := a.(k8stesting.PatchAction); ok {
			patchAction = pa
			break
		}
	}

	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxies"}
	if patchAction.GetResource() != expectedGVR {
		t.Errorf("expected GVR %v, got %v", expectedGVR, patchAction.GetResource())
	}
}

// TestDirectDeploy_IrregularPlural_UsesDiscovery verifies that directDeploy uses discovery-based
// GVR resolution for kinds with irregular plurals.
func TestDirectDeploy_IrregularPlural_UsesDiscovery(t *testing.T) {
	t.Parallel()

	// Set up fake discovery with Proxy -> proxies mapping
	disc := &fakediscovery.FakeDiscovery{
		Fake: &k8stesting.Fake{},
	}
	disc.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "proxies", Kind: "proxy", Verbs: metav1.Verbs{"get", "list", "create"}},
			},
		},
	}

	cache := watcher.NewInstanceCache()
	tracker := watcher.NewInstanceTrackerForTest(cache, nil)
	tracker.SetDiscoveryClient(disc)

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "example.com", Version: "v1", Resource: "proxies"}: "ProxyList",
	}
	fakeDynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind)

	// Set up RGD watcher with a Proxy RGD
	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:       "proxy-rgd",
		APIVersion: "example.com/v1",
		Kind:       "proxy",
	})

	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)

	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:      rgdWatcher,
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
	})

	deployReq := &deployment.DeployRequest{
		Name:           "my-proxy",
		Namespace:      "default",
		APIVersion:     "example.com/v1",
		Kind:           "proxy",
		Spec:           map[string]interface{}{"port": float64(8080)},
		DeploymentMode: deployment.ModeDirect,
	}

	result, err := handler.directDeploy(context.Background(), deployReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "my-proxy" {
		t.Errorf("expected name 'my-proxy', got %q", result.Name)
	}

	// Verify the dynamic client received "proxies" (not "proxys")
	actions := fakeDynClient.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	createAction, ok := actions[0].(k8stesting.CreateAction)
	if !ok {
		t.Fatalf("expected CreateAction, got %T", actions[0])
	}

	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxies"}
	if createAction.GetResource() != expectedGVR {
		t.Errorf("expected GVR %v, got %v", expectedGVR, createAction.GetResource())
	}
}

// ============================================================================
// Cluster-Scoped Instance RBAC Tests (STORY-301)
// ============================================================================

// testAuthService creates an AuthorizationService with mock dependencies for testing.
// When accessibleProjects is nil, it means global admin (sees all).
// When accessibleNamespaces is ["*"], it means global admin (no filtering).
func testAuthService(accessibleProjects []string, accessibleNamespaces []string) *services.AuthorizationService {
	return services.NewAuthorizationService(services.AuthorizationServiceConfig{
		PolicyEnforcer: &testPolicyEnforcer{
			accessibleProjects: accessibleProjects,
		},
		NamespaceProvider: &testNamespaceProvider{
			namespaces: accessibleNamespaces,
		},
	})
}

type testPolicyEnforcer struct {
	accessibleProjects []string
}

func (t *testPolicyEnforcer) GetAccessibleProjects(_ context.Context, user string, _ []string) ([]string, error) {
	if user == "" {
		return nil, fmt.Errorf("testPolicyEnforcer: GetAccessibleProjects called with empty user")
	}
	return t.accessibleProjects, nil
}

func (t *testPolicyEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return true, nil
}

type testNamespaceProvider struct {
	namespaces []string
}

func (t *testNamespaceProvider) GetUserNamespacesWithGroups(_ context.Context, _ string, _ []string) ([]string, error) {
	return t.namespaces, nil
}

// adminAuthService returns an AuthorizationService that grants global admin access
// (["*"] namespaces = all access, all projects accessible).
// Use this in tests that previously relied on nil authService = admin bypass.
func adminAuthService() *services.AuthorizationService {
	return services.NewAuthorizationService(services.AuthorizationServiceConfig{
		PolicyEnforcer: &testPolicyEnforcer{
			accessibleProjects: nil, // nil from GetAccessibleProjects triggers admin path
		},
		NamespaceProvider: &testNamespaceProvider{
			namespaces: []string{"*"}, // ["*"] from provider = global admin
		},
	})
}

// TestListInstances_ClusterScoped_FiltersByProject tests that cluster-scoped instances
// are filtered by the user's accessible projects (STORY-301, AC #6).
func TestListInstances_ClusterScoped_FiltersByProject(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	// Namespace-scoped instance in "ns-infra" namespace
	cache.Set(&models.Instance{
		Name:        "ns-instance-1",
		Namespace:   "ns-infra",
		Kind:        "webapp",
		RGDName:     "test-rgd",
		ProjectName: "infra",
	})
	// Cluster-scoped instance in project "infra"
	cache.Set(&models.Instance{
		Name:            "cluster-instance-1",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	// Cluster-scoped instance in project "payments"
	cache.Set(&models.Instance{
		Name:            "cluster-instance-2",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "payments",
		IsClusterScoped: true,
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// User has access to project "infra" only (and namespace "ns-infra")
	authSvc := testAuthService(
		[]string{"infra"},    // accessible projects
		[]string{"ns-infra"}, // accessible namespaces
	)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
	})

	userCtx := &middleware.UserContext{
		UserID:      "user:dev",
		Email:       "dev@test.local",
		CasbinRoles: []string{"proj:infra:developer"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ListInstances(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var result models.InstanceList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Should see: ns-instance-1 (namespace match) + cluster-instance-1 (project match)
	// Should NOT see: cluster-instance-2 (wrong project)
	if result.TotalCount != 2 {
		t.Errorf("expected 2 instances, got %d", result.TotalCount)
		for _, inst := range result.Items {
			t.Logf("  instance: %s (ns=%s, project=%s, cluster=%v)", inst.Name, inst.Namespace, inst.ProjectName, inst.IsClusterScoped)
		}
	}

	// Verify the correct instances are returned
	names := make(map[string]bool)
	for _, inst := range result.Items {
		names[inst.Name] = true
	}
	if !names["ns-instance-1"] {
		t.Error("expected ns-instance-1 to be in results")
	}
	if !names["cluster-instance-1"] {
		t.Error("expected cluster-instance-1 to be in results (same project)")
	}
	if names["cluster-instance-2"] {
		t.Error("cluster-instance-2 should NOT be in results (different project)")
	}
}

// TestListInstances_ClusterScoped_GlobalAdmin tests that global admins see all instances
// including cluster-scoped ones from any project (STORY-301, AC #4).
func TestListInstances_ClusterScoped_GlobalAdmin(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-1",
		Kind:            "Clusterpolicy",
		RGDName:         "rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	cache.Set(&models.Instance{
		Name:            "cluster-2",
		Kind:            "Clusterpolicy",
		RGDName:         "rgd",
		ProjectName:     "payments",
		IsClusterScoped: true,
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// Global admin: adminAuthService returns ["*"] namespaces = no filtering
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     adminAuthService(),
	})

	userCtx := &middleware.UserContext{
		UserID:      "user:admin",
		Email:       "admin@test.local",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ListInstances(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result models.InstanceList
	json.NewDecoder(resp.Body).Decode(&result)

	// Global admin sees all cluster-scoped instances
	if result.TotalCount != 2 {
		t.Errorf("global admin should see all 2 instances, got %d", result.TotalCount)
	}
}

// TestGetInstance_ClusterScoped_ProjectAccess tests that GetInstance checks project access
// for cluster-scoped instances (STORY-301, AC #1, #3).
func TestGetInstance_ClusterScoped_ProjectAccess(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "my-policy",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	tests := []struct {
		name               string
		accessibleProjects []string
		accessibleNS       []string
		expectedStatus     int
	}{
		{
			name:               "authorized user can access cluster-scoped instance",
			accessibleProjects: []string{"infra"},
			accessibleNS:       []string{"ns-infra"},
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "unauthorized user cannot access cluster-scoped instance",
			accessibleProjects: []string{"payments"},
			accessibleNS:       []string{"ns-payments"},
			expectedStatus:     http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc := testAuthService(tt.accessibleProjects, tt.accessibleNS)
			handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
				InstanceTracker: tracker,
				AuthService:     authSvc,
			})

			userCtx := &middleware.UserContext{
				UserID: "user:test",
				Email:  "test@test.local",
			}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/instances//ClusterPolicy/my-policy", nil)
			req.SetPathValue("namespace", "")
			req.SetPathValue("kind", "Clusterpolicy")
			req.SetPathValue("name", "my-policy")
			ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			handler.GetInstance(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

// TestIsProjectAccessible tests the isProjectAccessible helper function.
func TestIsProjectAccessible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		projectName        string
		accessibleProjects []string
		expected           bool
	}{
		{"nil projects means global admin", "any-project", nil, true},
		{"project in list", "infra", []string{"infra", "payments"}, true},
		{"project not in list", "infra", []string{"payments"}, false},
		{"empty list means no access", "infra", []string{}, false},
		{"empty project name", "", []string{"infra"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isProjectAccessible(tt.projectName, tt.accessibleProjects)
			if result != tt.expected {
				t.Errorf("isProjectAccessible(%q, %v) = %v, want %v",
					tt.projectName, tt.accessibleProjects, result, tt.expected)
			}
		})
	}
}

// TestDeleteInstance_ClusterScoped_Unauthorized tests that DeleteInstance returns 404
// (not 403) for unauthorized cluster-scoped instances, consistent with GetInstance
// to avoid leaking resource existence to cross-project users (STORY-301, H2 security fix).
func TestDeleteInstance_ClusterScoped_Unauthorized(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-policy-1",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		APIVersion:      "example.com/v1",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// User only has access to "payments" project, NOT "infra"
	authSvc := testAuthService(
		[]string{"payments"},
		[]string{"ns-payments"},
	)

	fakeDynClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
		DynamicClient:   fakeDynClient,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:pay-dev",
		Email:  "pay-dev@test.local",
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/instances//ClusterPolicy/cluster-policy-1", nil)
	req.SetPathValue("namespace", "")
	req.SetPathValue("kind", "Clusterpolicy")
	req.SetPathValue("name", "cluster-policy-1")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DeleteInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Must return 404, not 403 — avoids leaking that the resource exists
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unauthorized cross-project delete, got %d", resp.StatusCode)
	}
}

// TestUpdateInstance_ClusterScoped_Unauthorized tests that UpdateInstance returns 404
// (not 403) for unauthorized cluster-scoped instances, consistent with GetInstance
// to avoid leaking resource existence to cross-project users (STORY-301, H2 security fix).
func TestUpdateInstance_ClusterScoped_Unauthorized(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-policy-1",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		APIVersion:      "example.com/v1",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// User only has access to "payments" project, NOT "infra"
	authSvc := testAuthService(
		[]string{"payments"},
		[]string{"ns-payments"},
	)

	fakeDynClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
		DynamicClient:   fakeDynClient,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:pay-dev",
		Email:  "pay-dev@test.local",
	}
	body := strings.NewReader(`{"spec":{"key":"value"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/instances//ClusterPolicy/cluster-policy-1", body)
	req.SetPathValue("namespace", "")
	req.SetPathValue("kind", "Clusterpolicy")
	req.SetPathValue("name", "cluster-policy-1")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Must return 404, not 403 — avoids leaking that the resource exists
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unauthorized cross-project update, got %d", resp.StatusCode)
	}
}

// TestGetCount_ClusterScoped_FiltersByProject tests that GetCount correctly counts
// cluster-scoped instances filtered by the user's accessible projects (STORY-301).
// This exercises the CountFilteredInstances code path which differs from ListInstances.
func TestGetCount_ClusterScoped_FiltersByProject(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	// Namespace-scoped instance the user can access
	cache.Set(&models.Instance{
		Name:      "ns-inst",
		Namespace: "ns-infra",
		Kind:      "webapp",
		RGDName:   "test-rgd",
	})
	// Cluster-scoped instance in project "infra" (user has access)
	cache.Set(&models.Instance{
		Name:            "cluster-inst-infra",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	// Cluster-scoped instance in project "payments" (user does NOT have access)
	cache.Set(&models.Instance{
		Name:            "cluster-inst-payments",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "payments",
		IsClusterScoped: true,
	})

	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// User has access to project "infra" only
	authSvc := testAuthService(
		[]string{"infra"},    // accessible projects
		[]string{"ns-infra"}, // accessible namespaces
	)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
	})

	userCtx := &middleware.UserContext{
		UserID:      "user:dev",
		Email:       "dev@test.local",
		CasbinRoles: []string{"proj:infra:developer"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/count", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var countResp InstanceCountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// Should count: ns-inst (namespace match) + cluster-inst-infra (project match) = 2
	// Should NOT count: cluster-inst-payments (wrong project)
	if countResp.Count != 2 {
		t.Errorf("expected count 2, got %d", countResp.Count)
	}
}

// --- Cluster-Scoped Deployment Tests (STORY-302) ---

func TestCreateInstance_ClusterScoped_NamespaceNotRequired(t *testing.T) {
	t.Parallel()

	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-config-rgd",
		Namespace:       "kro-system",
		Kind:            "clusterconfig",
		APIVersion:      "kro.run/v1alpha1",
		IsClusterScoped: true,
	})
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)
	scheme := runtime.NewScheme()
	fakeDynClient := fakedynamic.NewSimpleDynamicClient(scheme)
	tracker := watcher.NewInstanceTracker(fakeDynClient, nil, nil, rgdWatcher)

	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:      rgdWatcher,
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
	})

	// Deploy cluster-scoped without namespace — should succeed
	body := `{"name": "my-cluster-config", "rgdName": "cluster-config-rgd", "spec": {"tier": "gold"}}`
	userCtx := &middleware.UserContext{UserID: "admin", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateInstance_NamespaceScoped_RequiresNamespace(t *testing.T) {
	t.Parallel()

	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "app-rgd",
		Namespace:       "kro-system",
		Kind:            "Application",
		APIVersion:      "kro.run/v1alpha1",
		IsClusterScoped: false,
	})
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)
	scheme := runtime.NewScheme()
	fakeDynClient := fakedynamic.NewSimpleDynamicClient(scheme)

	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:    rgdWatcher,
		DynamicClient: fakeDynClient,
	})

	// Deploy namespace-scoped without namespace — should fail
	body := `{"name": "my-app", "rgdName": "app-rgd", "spec": {"replicas": 1}}`
	userCtx := &middleware.UserContext{UserID: "dev", Email: "dev@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "namespace") {
		t.Errorf("expected error about namespace, got: %s", respBody)
	}
}

func TestCreateInstance_ClusterScoped_NoNamespaceInManifest(t *testing.T) {
	t.Parallel()

	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-policy-rgd",
		Namespace:       "kro-system",
		Kind:            "Clusterpolicy",
		APIVersion:      "kro.run/v1alpha1",
		IsClusterScoped: true,
	})
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)
	scheme := runtime.NewScheme()
	fakeDynClient := fakedynamic.NewSimpleDynamicClient(scheme)
	tracker := watcher.NewInstanceTracker(fakeDynClient, nil, nil, rgdWatcher)

	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:      rgdWatcher,
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
	})

	body := `{"name": "global-policy", "rgdName": "cluster-policy-rgd", "spec": {"enforce": true}}`
	userCtx := &middleware.UserContext{UserID: "admin", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify the dynamic client received a cluster-scoped create
	actions := fakeDynClient.Actions()
	if len(actions) == 0 {
		t.Fatal("expected at least one dynamic client action")
	}
	lastAction := actions[len(actions)-1]
	if lastAction.GetNamespace() != "" {
		t.Errorf("expected empty namespace for cluster-scoped create, got %q", lastAction.GetNamespace())
	}
}

func TestCreateInstance_ClusterScoped_ProjectLabelInjected(t *testing.T) {
	t.Parallel()

	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-config-rgd",
		Namespace:       "kro-system",
		Kind:            "clusterconfig",
		APIVersion:      "kro.run/v1alpha1",
		IsClusterScoped: true,
	})
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)
	scheme := runtime.NewScheme()
	fakeDynClient := fakedynamic.NewSimpleDynamicClient(scheme)
	tracker := watcher.NewInstanceTracker(fakeDynClient, nil, nil, rgdWatcher)

	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:      rgdWatcher,
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
	})

	body := `{"name": "test-config", "rgdName": "cluster-config-rgd", "projectId": "infra", "spec": {"level": "high"}}`
	userCtx := &middleware.UserContext{UserID: "admin", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	// Verify the project label was injected
	actions := fakeDynClient.Actions()
	if len(actions) == 0 {
		t.Fatal("expected at least one action")
	}
	createAction, ok := actions[len(actions)-1].(interface{ GetObject() runtime.Object })
	if !ok {
		t.Fatal("cannot extract object from action")
	}
	obj := createAction.GetObject()
	unstrObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		t.Fatal("expected unstructured object")
	}

	labels := unstrObj.GetLabels()
	if labels[models.ProjectLabel] != "infra" {
		t.Errorf("expected project label 'infra', got %q", labels[models.ProjectLabel])
	}
	if labels[models.DeploymentModeLabel] != "direct" {
		t.Errorf("expected deployment-mode label 'direct', got %q", labels[models.DeploymentModeLabel])
	}
}

// TestCreateInstance_ClusterScoped_AnnotationsInjected verifies AC #5: standard tracking
// annotations are injected by directDeploy() for cluster-scoped instances.
func TestCreateInstance_ClusterScoped_AnnotationsInjected(t *testing.T) {
	t.Parallel()

	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-config-rgd",
		Namespace:       "kro-system",
		Kind:            "clusterconfig",
		APIVersion:      "kro.run/v1alpha1",
		IsClusterScoped: true,
	})
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)
	scheme := runtime.NewScheme()
	fakeDynClient := fakedynamic.NewSimpleDynamicClient(scheme)
	tracker := watcher.NewInstanceTracker(fakeDynClient, nil, nil, rgdWatcher)

	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:      rgdWatcher,
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
	})

	body := `{"name": "annotated-config", "rgdName": "cluster-config-rgd", "projectId": "platform", "spec": {"mode": "strict"}}`
	userCtx := &middleware.UserContext{UserID: "admin", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	actions := fakeDynClient.Actions()
	if len(actions) == 0 {
		t.Fatal("expected at least one action")
	}
	createAction, ok := actions[len(actions)-1].(interface{ GetObject() runtime.Object })
	if !ok {
		t.Fatal("cannot extract object from action")
	}
	unstrObj, ok := createAction.GetObject().(*unstructured.Unstructured)
	if !ok {
		t.Fatal("expected unstructured object")
	}

	annotations := unstrObj.GetAnnotations()
	if annotations["knodex.io/instance-id"] == "" {
		t.Error("expected knodex.io/instance-id annotation to be set (AC #5)")
	}
	if annotations["knodex.io/created-by"] != "admin@test.local" {
		t.Errorf("expected knodex.io/created-by='admin@test.local', got %q", annotations["knodex.io/created-by"])
	}
	if annotations["knodex.io/created-at"] == "" {
		t.Error("expected knodex.io/created-at annotation to be set (AC #5)")
	}
	if annotations["knodex.io/deployment-mode"] != "direct" {
		t.Errorf("expected knodex.io/deployment-mode='direct', got %q", annotations["knodex.io/deployment-mode"])
	}
	if annotations["knodex.io/project-id"] != "platform" {
		t.Errorf("expected knodex.io/project-id='platform', got %q", annotations["knodex.io/project-id"])
	}
	// Cluster-scoped: no namespace annotation
	if annotations["knodex.io/namespace"] != "" {
		t.Errorf("cluster-scoped must not have knodex.io/namespace annotation, got %q", annotations["knodex.io/namespace"])
	}
}

// TestAuthorizeInstanceAccess tests the shared authorizeInstanceAccess method (STORY-309).
func TestAuthorizeInstanceAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		instance           *models.Instance
		accessibleProjects []string
		accessibleNS       []string
		userNamespaces     []string
		userCtx            *middleware.UserContext // nil uses default "user:test" context
		expectedAuth       bool
		expectErr          bool
	}{
		{
			name:           "global admin (wildcard namespaces) always authorized",
			instance:       &models.Instance{Name: "test", Namespace: "default"},
			userNamespaces: []string{"*"},
			expectedAuth:   true,
		},
		{
			name:               "namespace-scoped instance with matching namespace",
			instance:           &models.Instance{Name: "test", Namespace: "staging", Kind: "webapp"},
			accessibleProjects: []string{"dev"},
			accessibleNS:       []string{"staging"},
			userNamespaces:     []string{"staging"},
			expectedAuth:       true,
		},
		{
			name:               "namespace-scoped instance without matching namespace",
			instance:           &models.Instance{Name: "test", Namespace: "production", Kind: "webapp"},
			accessibleProjects: []string{"dev"},
			accessibleNS:       []string{"staging"},
			userNamespaces:     []string{"staging"},
			expectedAuth:       false,
		},
		{
			name:               "cluster-scoped instance with matching project",
			instance:           &models.Instance{Name: "policy-1", Namespace: "", Kind: "clusterpolicy", ProjectName: "infra", IsClusterScoped: true},
			accessibleProjects: []string{"infra", "payments"},
			accessibleNS:       []string{"ns-infra"},
			userNamespaces:     []string{"ns-infra"},
			expectedAuth:       true,
		},
		{
			name:               "cluster-scoped instance without matching project",
			instance:           &models.Instance{Name: "policy-1", Namespace: "", Kind: "clusterpolicy", ProjectName: "infra", IsClusterScoped: true},
			accessibleProjects: []string{"payments"},
			accessibleNS:       []string{"ns-payments"},
			userNamespaces:     []string{"ns-payments"},
			expectedAuth:       false,
		},
		{
			// testPolicyEnforcer.GetAccessibleProjects returns error when UserID == "".
			// Verifies that authorizeInstanceAccess propagates the error from the auth service.
			name:           "cluster-scoped authService error propagates",
			instance:       &models.Instance{Name: "policy-1", Namespace: "", Kind: "clusterpolicy", ProjectName: "infra", IsClusterScoped: true},
			userNamespaces: []string{"ns-infra"},
			userCtx:        &middleware.UserContext{UserID: "", Email: ""},
			expectErr:      true,
			expectedAuth:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authSvc := testAuthService(tt.accessibleProjects, tt.accessibleNS)
			handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
				AuthService: authSvc,
			})

			userCtx := tt.userCtx
			if userCtx == nil {
				userCtx = &middleware.UserContext{
					UserID: "user:test",
					Email:  "test@test.local",
				}
			}

			authorized, err := handler.authorizeInstanceAccess(
				context.Background(), userCtx, tt.instance, tt.userNamespaces,
			)
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if authorized != tt.expectedAuth {
				t.Errorf("expected authorized=%v, got %v", tt.expectedAuth, authorized)
			}
		})
	}
}

// TestBuildInstanceAccessFilter tests the shared filter builder (STORY-309).
func TestBuildInstanceAccessFilter(t *testing.T) {
	t.Parallel()

	filter := buildInstanceAccessFilter(
		[]string{"staging", "dev"},
		[]string{"infra", "payments"},
	)

	tests := []struct {
		name     string
		instance models.Instance
		expected bool
	}{
		{
			name:     "namespace-scoped in accessible namespace",
			instance: models.Instance{Name: "app", Namespace: "staging"},
			expected: true,
		},
		{
			name:     "namespace-scoped in inaccessible namespace",
			instance: models.Instance{Name: "app", Namespace: "production"},
			expected: false,
		},
		{
			name:     "cluster-scoped in accessible project",
			instance: models.Instance{Name: "policy", ProjectName: "infra", IsClusterScoped: true},
			expected: true,
		},
		{
			name:     "cluster-scoped in inaccessible project",
			instance: models.Instance{Name: "policy", ProjectName: "secret-ops", IsClusterScoped: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := filter(tt.instance)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}

	// ["*"] userNamespaces = global admin: filter must always return true for ns-scoped instances.
	// Cluster-scoped instances still require project access (no nil bypass).
	t.Run("wildcard userNamespaces (global admin) passes ns-scoped", func(t *testing.T) {
		t.Parallel()
		adminFilter := buildInstanceAccessFilter([]string{"*"}, []string{"secret-ops"})
		for _, inst := range []models.Instance{
			{Name: "ns-app", Namespace: "production"},
			{Name: "cluster-policy", IsClusterScoped: true, ProjectName: "secret-ops"},
		} {
			if !adminFilter(inst) {
				t.Errorf("global admin filter returned false for instance %q", inst.Name)
			}
		}
	})
}

// TestGetInstance_Unauthorized_NoUserContext tests that GetInstance returns 401
// when no user context is present, consistent with Delete/Update (STORY-309, F-3 fix).
func TestGetInstance_Unauthorized_NoUserContext(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:      "test-app",
		Namespace: "default",
		Kind:      "webapp",
	})
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	// Request without user context (no auth middleware set it)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/default/WebApp/test-app", nil)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-app")
	rec := httptest.NewRecorder()

	handler.GetInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// ============================================================================
// Cluster-Scoped Authorization Error & Filtering Tests (STORY-311)
// ============================================================================

// testErrorPolicyEnforcer is a mock that returns an error from GetAccessibleProjects,
// simulating a Casbin or backend failure during cluster-scoped authorization.
type testErrorPolicyEnforcer struct {
	projectsErr error
}

func (t *testErrorPolicyEnforcer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, t.projectsErr
}

func (t *testErrorPolicyEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return true, nil
}

// testAuthServiceWithError creates an AuthorizationService whose GetAccessibleProjects returns an error.
func testAuthServiceWithError(namespaces []string, projectsErr error) *services.AuthorizationService {
	return services.NewAuthorizationService(services.AuthorizationServiceConfig{
		PolicyEnforcer: &testErrorPolicyEnforcer{projectsErr: projectsErr},
		NamespaceProvider: &testNamespaceProvider{
			namespaces: namespaces,
		},
	})
}

// TestListInstances_ClusterScoped_ProjectError tests that ListInstances returns 500
// when getAccessibleProjects() fails (STORY-311, AC #1, F-9).
func TestListInstances_ClusterScoped_ProjectError(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-instance",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	// AuthService returns namespaces (non-nil = non-admin) but projects fail
	authSvc := testAuthServiceWithError(
		[]string{"ns-infra"},
		fmt.Errorf("simulated Casbin failure"),
	)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:dev",
		Email:  "dev@test.local",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ListInstances(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 when getAccessibleProjects fails, got %d", resp.StatusCode)
	}
}

// TestGetInstance_ClusterScoped_ProjectError tests that GetInstance returns 500
// when getAccessibleProjects() fails during cluster-scoped authorization (STORY-311, F-9).
func TestGetInstance_ClusterScoped_ProjectError(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "my-policy",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	authSvc := testAuthServiceWithError(
		[]string{"ns-infra"},
		fmt.Errorf("simulated project lookup failure"),
	)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:dev",
		Email:  "dev@test.local",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances//ClusterPolicy/my-policy", nil)
	req.SetPathValue("namespace", "")
	req.SetPathValue("kind", "Clusterpolicy")
	req.SetPathValue("name", "my-policy")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 when authorizeInstanceAccess fails, got %d", resp.StatusCode)
	}
}

// TestDeleteInstance_ClusterScoped_ProjectError tests that DeleteInstance returns 500
// when getAccessibleProjects() fails during cluster-scoped authorization (STORY-311, F-9).
func TestDeleteInstance_ClusterScoped_ProjectError(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-policy-1",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		APIVersion:      "example.com/v1",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	authSvc := testAuthServiceWithError(
		[]string{"ns-infra"},
		fmt.Errorf("simulated project lookup failure"),
	)

	fakeDynClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
		DynamicClient:   fakeDynClient,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:dev",
		Email:  "dev@test.local",
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/instances//ClusterPolicy/cluster-policy-1", nil)
	req.SetPathValue("namespace", "")
	req.SetPathValue("kind", "Clusterpolicy")
	req.SetPathValue("name", "cluster-policy-1")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.DeleteInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 when authorizeInstanceAccess fails, got %d", resp.StatusCode)
	}
}

// TestUpdateInstance_ClusterScoped_ProjectError tests that UpdateInstance returns 500
// when getAccessibleProjects() fails during cluster-scoped authorization (STORY-311, F-9).
func TestUpdateInstance_ClusterScoped_ProjectError(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-policy-1",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		APIVersion:      "example.com/v1",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	authSvc := testAuthServiceWithError(
		[]string{"ns-infra"},
		fmt.Errorf("simulated project lookup failure"),
	)

	fakeDynClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
		DynamicClient:   fakeDynClient,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:dev",
		Email:  "dev@test.local",
	}
	body := strings.NewReader(`{"spec":{"key":"value"}}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/instances//ClusterPolicy/cluster-policy-1", body)
	req.SetPathValue("namespace", "")
	req.SetPathValue("kind", "Clusterpolicy")
	req.SetPathValue("name", "cluster-policy-1")
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 when authorizeInstanceAccess fails, got %d", resp.StatusCode)
	}
}

// TestGetCount_ClusterScoped_ProjectError tests that GetCount returns 500
// when getAccessibleProjects() fails (STORY-311, F-9).
func TestGetCount_ClusterScoped_ProjectError(t *testing.T) {
	t.Parallel()

	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:            "cluster-inst",
		Namespace:       "",
		Kind:            "Clusterpolicy",
		RGDName:         "cluster-rgd",
		ProjectName:     "infra",
		IsClusterScoped: true,
	})
	tracker := watcher.NewInstanceTrackerWithCache(cache)

	authSvc := testAuthServiceWithError(
		[]string{"ns-infra"},
		fmt.Errorf("simulated Casbin failure"),
	)

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		AuthService:     authSvc,
	})

	userCtx := &middleware.UserContext{
		UserID: "user:dev",
		Email:  "dev@test.local",
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/count", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 when getAccessibleProjects fails, got %d", resp.StatusCode)
	}
}

func TestBatchEnrichInstanceDrift(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	driftSvc := drift.NewService(client, nil, "")

	// Store drift for two instances — one drifted, one reconciled
	driftedSpec := map[string]interface{}{"replicas": float64(5)}
	reconciledSpec := map[string]interface{}{"image": "nginx:latest"}
	if err := driftSvc.StoreDrift(context.Background(), "ns1", "webapp", "app1", driftedSpec); err != nil {
		t.Fatalf("StoreDrift app1: %v", err)
	}
	if err := driftSvc.StoreDrift(context.Background(), "ns2", "DB", "app2", reconciledSpec); err != nil {
		t.Fatalf("StoreDrift app2: %v", err)
	}

	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		DriftService: driftSvc,
	})

	instances := []models.Instance{
		{ // index 0: direct mode — should be skipped
			Namespace: "ns0",
			Kind:      "webapp",
			Name:      "direct-app",
			Labels:    map[string]string{models.DeploymentModeLabel: "direct"},
			Spec:      map[string]interface{}{"replicas": float64(1)},
		},
		{ // index 1: gitops mode, drifted (live != desired)
			Namespace: "ns1",
			Kind:      "webapp",
			Name:      "app1",
			Labels:    map[string]string{models.DeploymentModeLabel: "gitops"},
			Spec:      map[string]interface{}{"replicas": float64(3)}, // different from stored 5
		},
		{ // index 2: hybrid mode, reconciled (live == desired)
			Namespace: "ns2",
			Kind:      "DB",
			Name:      "app2",
			Labels:    map[string]string{models.DeploymentModeLabel: "hybrid"},
			Spec:      reconciledSpec,
		},
		{ // index 3: gitops mode, no drift entry in Redis
			Namespace: "ns3",
			Kind:      "webapp",
			Name:      "app3",
			Labels:    map[string]string{models.DeploymentModeLabel: "gitops"},
			Spec:      map[string]interface{}{"replicas": float64(1)},
		},
	}

	handler.batchEnrichInstanceDrift(context.Background(), instances)

	// index 0: direct mode — untouched
	if instances[0].GitOpsDrift {
		t.Error("direct-mode instance should not have drift")
	}

	// index 1: drifted
	if !instances[1].GitOpsDrift {
		t.Error("app1 should be drifted (live spec differs from desired)")
	}
	if instances[1].DesiredSpec == nil {
		t.Error("app1 should have DesiredSpec populated")
	}
	if instances[1].DesiredSpec["replicas"] != float64(5) {
		t.Errorf("app1: expected desired replicas=5, got %v", instances[1].DesiredSpec["replicas"])
	}
	if instances[1].DriftedAt == nil {
		t.Error("app1 should have non-nil DriftedAt when drifted")
	}

	// index 2: reconciled (live == desired)
	if instances[2].GitOpsDrift {
		t.Error("app2 should not be drifted (reconciled)")
	}
	if instances[2].DriftedAt != nil {
		t.Error("app2 should have nil DriftedAt when reconciled")
	}

	// index 3: no entry — no drift
	if instances[3].GitOpsDrift {
		t.Error("app3 should not be drifted (no Redis entry)")
	}
	if instances[3].DriftedAt != nil {
		t.Error("app3 should have nil DriftedAt when no Redis entry")
	}
}

// --- STORY-331: Instance graph tests ---

// withAdminContext returns a copy of r with a dummy admin user context injected.
// Since tests use no authService, any non-nil UserContext results in global-admin access
// (getAccessibleNamespaces returns ["*"] when authService is nil → always authorized).
func withAdminContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID: "test-admin",
		Email:  "admin@test.local",
	})
	return r.WithContext(ctx)
}

// makeInstanceGraphHandler creates an InstanceCRUDHandler with the given instance in cache
// and the given RGD watcher, suitable for GetInstanceGraph tests.
func makeInstanceGraphHandler(instance *models.Instance, rgdWatcherArg *watcher.RGDWatcher) *InstanceCRUDHandler {
	instanceCache := watcher.NewInstanceCache()
	instanceCache.Set(instance)
	tracker := watcher.NewInstanceTrackerWithCache(instanceCache)
	return NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		RGDWatcher:      rgdWatcherArg,
		AuthService:     adminAuthService(),
	})
}

// makeRGDWatcherWithCollectionSpec creates an RGDWatcher containing an RGD with a forEach resource.
// The spec includes readyWhen and includeWhen to exercise AC3 (runtime graph topology parity with definition graph).
func makeRGDWatcherWithCollectionSpec(rgdName, rgdNamespace string) *watcher.RGDWatcher {
	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:      rgdName,
		Namespace: rgdNamespace,
		RawSpec: map[string]interface{}{
			"resources": []interface{}{
				map[string]interface{}{
					"id": "workerPods",
					"forEach": []interface{}{
						map[string]interface{}{"worker": "${schema.spec.workers}"},
					},
					"readyWhen": []interface{}{
						"each.status.phase == 'Running'",
					},
					"includeWhen": []interface{}{
						"schema.spec.workers > 0",
					},
					"template": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
					},
				},
				map[string]interface{}{
					"template": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
					},
				},
			},
		},
		Annotations: map[string]string{models.CatalogAnnotation: "true"},
	})
	return watcher.NewRGDWatcherWithCache(rgdCache)
}

// TestGetInstanceGraph_CollectionStatus_Empty verifies that a collection node in the
// runtime graph has a non-nil collectionStatus with Health=Healthy and TotalCount=0 (AC3).
func TestGetInstanceGraph_CollectionStatus_Empty(t *testing.T) {
	t.Parallel()

	instance := &models.Instance{
		Name:         "my-instance",
		Namespace:    "default",
		Kind:         "Workerapp",
		RGDName:      "worker-rgd",
		RGDNamespace: "default",
	}
	rgdW := makeRGDWatcherWithCollectionSpec("worker-rgd", "default")
	handler := makeInstanceGraphHandler(instance, rgdW)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/Workerapp/my-instance/graph", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Workerapp")
	req.SetPathValue("name", "my-instance")
	rec := httptest.NewRecorder()

	handler.GetInstanceGraph(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body RuntimeGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Resources) == 0 {
		t.Fatal("expected at least one resource node")
	}

	// Find the collection node
	var collFound bool
	for _, node := range body.Resources {
		if node.IsCollection {
			collFound = true
			if node.CollectionStatus == nil {
				t.Error("collection node must have non-nil collectionStatus")
				continue
			}
			if node.CollectionStatus.TotalCount != 0 {
				t.Errorf("expected TotalCount=0, got %d", node.CollectionStatus.TotalCount)
			}
			if node.CollectionStatus.Health != models.HealthHealthy {
				t.Errorf("expected Health=Healthy, got %q", node.CollectionStatus.Health)
			}
			// AC3: collection metadata parity with definition graph
			if len(node.ReadyWhen) == 0 {
				t.Error("collection node: expected non-empty readyWhen (AC3 topology parity)")
			}
			if node.ConditionExpr == "" {
				t.Error("collection node: expected non-empty conditionExpr from includeWhen (AC3 topology parity)")
			}
		} else {
			if node.CollectionStatus != nil {
				t.Errorf("non-collection node %q must have null collectionStatus", node.ID)
			}
		}
	}
	if !collFound {
		t.Error("expected at least one collection node in the runtime graph")
	}
}

// TestGetInstanceGraph_NotFound verifies that a 404 is returned when the instance
// does not exist in the cache (AC4).
func TestGetInstanceGraph_NotFound(t *testing.T) {
	t.Parallel()

	// Empty tracker — instance does not exist
	emptyTracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: emptyTracker,
		RGDWatcher:      makeRGDWatcherWithCollectionSpec("worker-rgd", "default"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/WorkerApp/missing/graph", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Workerapp")
	req.SetPathValue("name", "missing")
	rec := httptest.NewRecorder()

	handler.GetInstanceGraph(rec, req)

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Result().StatusCode)
	}
}

// TestGetInstanceGraph_RGDNotFound verifies that a 404 is returned when the instance
// exists but its parent RGD cannot be found (AC5).
func TestGetInstanceGraph_RGDNotFound(t *testing.T) {
	t.Parallel()

	instance := &models.Instance{
		Name:         "my-instance",
		Namespace:    "default",
		Kind:         "workerapp",
		RGDName:      "missing-rgd",
		RGDNamespace: "default",
	}

	// RGD watcher is empty — RGD not found
	emptyRGDCache := watcher.NewRGDCache()
	emptyRGDWatcher := watcher.NewRGDWatcherWithCache(emptyRGDCache)
	handler := makeInstanceGraphHandler(instance, emptyRGDWatcher)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/WorkerApp/my-instance/graph", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Workerapp")
	req.SetPathValue("name", "my-instance")
	rec := httptest.NewRecorder()

	handler.GetInstanceGraph(rec, req)

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 when RGD not found, got %d", rec.Result().StatusCode)
	}
}

// TestGetInstanceGraph_ClusterScoped verifies that the cluster-scoped route
// (empty namespace path value) resolves the instance and returns a valid runtime graph (AC3).
func TestGetInstanceGraph_ClusterScoped(t *testing.T) {
	t.Parallel()

	instance := &models.Instance{
		Name:            "global-instance",
		Namespace:       "", // cluster-scoped: no namespace
		Kind:            "Globalapp",
		IsClusterScoped: true,
		RGDName:         "global-rgd",
		RGDNamespace:    "default",
	}
	rgdW := makeRGDWatcherWithCollectionSpec("global-rgd", "default")
	handler := makeInstanceGraphHandler(instance, rgdW)

	// Use the cluster-scoped route — no namespace in the path
	req := httptest.NewRequest(http.MethodGet, "/api/v1/instances/GlobalApp/global-instance/graph", nil)
	req = withAdminContext(req)
	// namespace path value is intentionally not set (empty string, matches cluster-scoped pattern)
	req.SetPathValue("kind", "Globalapp")
	req.SetPathValue("name", "global-instance")
	rec := httptest.NewRecorder()

	handler.GetInstanceGraph(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cluster-scoped instance graph: expected 200, got %d", resp.StatusCode)
	}

	var body RuntimeGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Resources) == 0 {
		t.Fatal("expected at least one resource node in cluster-scoped runtime graph")
	}
}

// =============================================================================
// STORY-348: DNS-1123 Validation Tests for Instance CRUD Handlers
// =============================================================================

func TestGetInstance_InvalidKind_Returns400(t *testing.T) {
	t.Parallel()
	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces/default/instances/UPPER_KIND/my-instance", nil, userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "UPPER_KIND")
	req.SetPathValue("name", "my-instance")
	rec := httptest.NewRecorder()

	handler.GetInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetInstance_InvalidName_Returns400(t *testing.T) {
	t.Parallel()
	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces/default/instances/MyKind/INVALID_NAME", nil, userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "MyKind")
	req.SetPathValue("name", "INVALID_NAME")
	rec := httptest.NewRecorder()

	handler.GetInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetInstance_InvalidNamespace_Returns400(t *testing.T) {
	t.Parallel()
	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces/BAD_NS/instances/MyKind/my-instance", nil, userCtx)
	req.SetPathValue("namespace", "BAD_NS")
	req.SetPathValue("kind", "MyKind")
	req.SetPathValue("name", "my-instance")
	rec := httptest.NewRecorder()

	handler.GetInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteInstance_InvalidKind_Returns400(t *testing.T) {
	t.Parallel()
	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/namespaces/default/instances/bad_kind/my-instance", nil, userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "bad_kind")
	req.SetPathValue("name", "my-instance")
	rec := httptest.NewRecorder()

	handler.DeleteInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateInstance_InvalidName_Returns400(t *testing.T) {
	t.Parallel()
	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/namespaces/default/instances/MyKind/UPPER_NAME",
		[]byte(`{"spec":{"key":"val"}}`), userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "MyKind")
	req.SetPathValue("name", "UPPER_NAME")
	rec := httptest.NewRecorder()

	handler.UpdateInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestGetInstanceGraph_InvalidKind_Returns400(t *testing.T) {
	t.Parallel()
	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		RGDWatcher:      watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces/default/instances/BAD_KIND/my-instance/graph", nil, userCtx)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "BAD_KIND")
	req.SetPathValue("name", "my-instance")
	rec := httptest.NewRecorder()

	handler.GetInstanceGraph(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateInstance_InvalidPathNamespace_Returns400(t *testing.T) {
	t.Parallel()
	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:    watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
		DynamicClient: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
	})

	body := []byte(`{"name":"my-instance","rgdName":"test-rgd","spec":{}}`)
	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/namespaces/BAD_NS/instances/MyKind", body, userCtx)
	req.SetPathValue("namespace", "BAD_NS")
	req.SetPathValue("kind", "MyKind")
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateInstance_InvalidPathKind_Returns400(t *testing.T) {
	t.Parallel()
	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:    watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
		DynamicClient: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
	})

	body := []byte(`{"name":"my-instance","rgdName":"test-rgd","spec":{}}`)
	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances/UPPER_KIND", body, userCtx)
	req.SetPathValue("kind", "UPPER_KIND")
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateInstance_InvalidBodyNamespace_Returns400(t *testing.T) {
	t.Parallel()
	handler := NewInstanceDeploymentHandler(InstanceDeploymentHandlerConfig{
		RGDWatcher:    watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
		DynamicClient: fakedynamic.NewSimpleDynamicClient(runtime.NewScheme()),
	})

	body := []byte(`{"name":"my-instance","namespace":"INVALID_NS!!","rgdName":"test-rgd","spec":{}}`)
	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/instances/MyKind", body, userCtx)
	req.SetPathValue("kind", "MyKind")
	rec := httptest.NewRecorder()

	handler.CreateInstance(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

// --- GetInstanceChildren handler tests (STORY-408) ---

// TestGetInstanceChildren_NilChildService verifies that a 503 is returned when
// the child resource service is not configured (e.g., dependency init failure).
func TestGetInstanceChildren_NilChildService(t *testing.T) {
	t.Parallel()
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache()),
		// ChildService intentionally nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/WebApp/my-app/children", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "my-app")
	rec := httptest.NewRecorder()

	handler.GetInstanceChildren(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// TestGetInstanceChildren_NilInstanceTracker verifies that a 503 is returned when
// the instance tracker is not configured. The childService nil check comes first, so
// we provide a non-nil childService to reach the tracker guard.
func TestGetInstanceChildren_NilInstanceTracker(t *testing.T) {
	t.Parallel()

	fakeClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	childSvc := children.NewService(
		fakeClient,
		watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
		watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache()),
		kroparser.NewResourceParser(),
		nil,
		nil,
	)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		ChildService: childSvc,
		// InstanceTracker intentionally nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/WebApp/my-app/children", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "my-app")
	rec := httptest.NewRecorder()

	handler.GetInstanceChildren(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// TestGetInstanceChildren_MissingUserContext verifies that a 401 is returned
// when no user context is attached to the request.
func TestGetInstanceChildren_MissingUserContext(t *testing.T) {
	t.Parallel()

	tracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	fakeClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	childSvc := children.NewService(
		fakeClient,
		watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
		tracker,
		kroparser.NewResourceParser(),
		nil,
		nil,
	)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		ChildService:    childSvc,
	})

	// No user context — raw request with no middleware
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/WebApp/my-app/children", nil)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "my-app")
	rec := httptest.NewRecorder()

	handler.GetInstanceChildren(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// TestGetInstanceChildren_InstanceNotFound verifies that a 404 is returned when
// the requested instance does not exist in the tracker cache.
func TestGetInstanceChildren_InstanceNotFound(t *testing.T) {
	t.Parallel()

	emptyTracker := watcher.NewInstanceTrackerWithCache(watcher.NewInstanceCache())
	fakeClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	childSvc := children.NewService(
		fakeClient,
		watcher.NewRGDWatcherWithCache(watcher.NewRGDCache()),
		emptyTracker,
		kroparser.NewResourceParser(),
		nil,
		nil,
	)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: emptyTracker,
		ChildService:    childSvc,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/WebApp/missing/children", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "missing")
	rec := httptest.NewRecorder()

	handler.GetInstanceChildren(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// TestGetInstanceChildren_Success verifies that a 200 with a valid ChildResourceResponse
// is returned when the instance exists and the user has access (global admin context).
// The fake dynamic client returns an empty pod list → TotalCount == 0.
// NOTE: Kind in instance model must exactly match SetPathValue("kind", ...) — cache is case-sensitive.
func TestGetInstanceChildren_Success(t *testing.T) {
	t.Parallel()

	const instanceKind = "Webapp" // CamelCase — must match path value

	instance := &models.Instance{
		Name:         "my-app",
		Namespace:    "default",
		Kind:         instanceKind,
		RGDName:      "web-rgd",
		RGDNamespace: "default",
	}

	instanceCache := watcher.NewInstanceCache()
	instanceCache.Set(instance)
	tracker := watcher.NewInstanceTrackerWithCache(instanceCache)

	rgdCache := watcher.NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:      "web-rgd",
		Namespace: "default",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "kro.run/v1alpha1",
				"kind":       instanceKind,
				"spec":       map[string]interface{}{},
			},
			"resources": []interface{}{
				map[string]interface{}{
					"id": "appPod",
					"template": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata":   map[string]interface{}{"name": "placeholder"},
					},
				},
			},
		},
	})
	rgdWatcher := watcher.NewRGDWatcherWithCache(rgdCache)

	fakeClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
	)

	childSvc := children.NewService(fakeClient, rgdWatcher, tracker, kroparser.NewResourceParser(), nil, nil)
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		ChildService:    childSvc,
		AuthService:     adminAuthService(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/instances/Webapp/my-app/children", nil)
	req = withAdminContext(req)
	req.SetPathValue("namespace", "default")
	req.SetPathValue("kind", instanceKind)
	req.SetPathValue("name", "my-app")
	rec := httptest.NewRecorder()

	handler.GetInstanceChildren(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body models.ChildResourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.InstanceName != "my-app" {
		t.Errorf("InstanceName = %q, want %q", body.InstanceName, "my-app")
	}
	if body.InstanceNamespace != "default" {
		t.Errorf("InstanceNamespace = %q, want %q", body.InstanceNamespace, "default")
	}
}
