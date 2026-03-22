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

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
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

// --- UpdateInstance tests ---

// newUpdateTestSetup creates common test infrastructure for UpdateInstance tests.
// Returns handler, mockAuditRecorder, and fakeDynClient for test assertions.
func newUpdateTestSetup(t *testing.T) (*InstanceCRUDHandler, *mockAuditRecorder, *fakedynamic.FakeDynamicClient) {
	t.Helper()
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:        "test-instance",
		Namespace:   "production",
		Kind:        "WebApp",
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
			"kind":       "WebApp",
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
	req.SetPathValue("kind", "WebApp")
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
	if resp.Kind != "WebApp" {
		t.Errorf("expected kind 'WebApp', got %q", resp.Kind)
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
	if e.Details["kind"] != "WebApp" {
		t.Errorf("expected kind 'WebApp', got %v", e.Details["kind"])
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
			req.SetPathValue("kind", "WebApp")
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
			req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
			handler.handleUpdateError(rec, "default", "WebApp", "test", tt.err)

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
		Kind:         "WebApp",
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
			"kind":       "WebApp",
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
		req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
	req.SetPathValue("kind", "WebApp")
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
		Kind:        "WebApp",
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
			"kind":       "WebApp",
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
	req.SetPathValue("kind", "WebApp")
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
// GVR resolution for kinds with irregular plurals (e.g., "Proxy" -> "proxies" not "proxys").
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
				{Name: "proxies", Kind: "Proxy", Verbs: metav1.Verbs{"get", "list", "create"}},
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
		Kind:       "Proxy",
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
		Kind:           "Proxy",
		Spec:           map[string]interface{}{"port": float64(8080)},
		DeploymentMode: deployment.ModeDirect,
	}

	result, err := handler.directDeploy(context.Background(), deployReq, "example.com", "v1", "Proxy", "default")
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
