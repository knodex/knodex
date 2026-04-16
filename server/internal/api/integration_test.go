// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api"
	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/health"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
)

// setupTestServer creates a test server with mock data
func setupTestServer(t *testing.T) (*httptest.Server, *watcher.RGDCache) {
	cache := watcher.NewRGDCache()

	// Add test RGDs - all must have catalog annotation to be visible
	testRGDs := []models.CatalogRGD{
		{
			Name:        "postgres-cluster",
			Namespace:   "default",
			Description: "PostgreSQL cluster with high availability",
			Tags:        []string{"database", "production"},
			Category:    "database",
			Labels:      map[string]string{"tier": "backend"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-24 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        "redis-cache",
			Namespace:   "default",
			Description: "Redis cache for session storage",
			Tags:        []string{"cache", "production"},
			Category:    "cache",
			Labels:      map[string]string{"tier": "backend"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-12 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        "mongodb",
			Namespace:   "staging",
			Description: "MongoDB document database",
			Tags:        []string{"database", "staging"},
			Category:    "database",
			Labels:      map[string]string{},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-6 * time.Hour),
			UpdatedAt:   time.Now(),
		},
	}

	for i := range testRGDs {
		cache.Set(&testRGDs[i])
	}

	// Create watcher with test cache
	w := watcher.NewRGDWatcherWithCache(cache)

	// Create health checker with watcher (nil for redis and k8s in tests)
	checker := health.NewChecker(nil, nil, w)

	// Create router (nil for instanceTracker, schema extractor, and wsHub in tests)
	routerResult := api.NewRouterWithConfig(checker, w, nil, nil, api.RouterConfig{})
	t.Cleanup(func() {
		for _, rl := range routerResult.UserRateLimiters {
			rl.Stop()
		}
	})

	// Create test server
	server := httptest.NewServer(routerResult.Handler)

	return server, cache
}

func TestIntegration_ListRGDs(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "list all RGDs",
			path:           "/api/v1/rgds",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "filter by namespace",
			path:           "/api/v1/rgds?namespace=default",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "filter by category",
			path:           "/api/v1/rgds?category=database",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "search by name",
			path:           "/api/v1/rgds?search=postgres",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "combined filters",
			path:           "/api/v1/rgds?namespace=default&category=database",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "pagination",
			path:           "/api/v1/rgds?page=1&pageSize=2",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			var listResp handlers.ListRGDsResponse
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(listResp.Items) != tt.expectedCount {
				t.Errorf("expected %d items, got %d", tt.expectedCount, len(listResp.Items))
			}
		})
	}
}

func TestIntegration_ListRGDs_ValidationErrors(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	tests := []struct {
		name          string
		path          string
		expectedField string
	}{
		{
			name:          "invalid page",
			path:          "/api/v1/rgds?page=invalid",
			expectedField: "page",
		},
		{
			name:          "negative page",
			path:          "/api/v1/rgds?page=0",
			expectedField: "page",
		},
		{
			name:          "pageSize too large",
			path:          "/api/v1/rgds?pageSize=500",
			expectedField: "pageSize",
		},
		{
			name:          "invalid sortBy",
			path:          "/api/v1/rgds?sortBy=invalid",
			expectedField: "sortBy",
		},
		{
			name:          "invalid sortOrder",
			path:          "/api/v1/rgds?sortOrder=random",
			expectedField: "sortOrder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResp response.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Code != response.ErrCodeBadRequest {
				t.Errorf("expected error code %s, got %s", response.ErrCodeBadRequest, errResp.Code)
			}

			if _, ok := errResp.Details[tt.expectedField]; !ok {
				t.Errorf("expected error details to contain field %s", tt.expectedField)
			}
		})
	}
}

func TestIntegration_GetRGD(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedName   string
	}{
		{
			name:           "get existing RGD",
			path:           "/api/v1/rgds/postgres-cluster",
			expectedStatus: http.StatusOK,
			expectedName:   "postgres-cluster",
		},
		{
			name:           "get RGD with namespace",
			path:           "/api/v1/rgds/mongodb?namespace=staging",
			expectedStatus: http.StatusOK,
			expectedName:   "mongodb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			var rgdResp services.RGDResponse
			if err := json.NewDecoder(resp.Body).Decode(&rgdResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if rgdResp.Name != tt.expectedName {
				t.Errorf("expected name %s, got %s", tt.expectedName, rgdResp.Name)
			}
		})
	}
}

func TestIntegration_GetRGD_NotFound(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "non-existent RGD",
			path: "/api/v1/rgds/non-existent",
		},
		{
			name: "RGD in wrong namespace",
			path: "/api/v1/rgds/postgres-cluster?namespace=staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
			}

			var errResp response.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Code != response.ErrCodeNotFound {
				t.Errorf("expected error code %s, got %s", response.ErrCodeNotFound, errResp.Code)
			}
		})
	}
}

func TestIntegration_HealthEndpoints(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "liveness probe",
			path:           "/healthz",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "readiness probe",
			path:           "/readyz",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected content-type application/json, got %s", contentType)
			}
		})
	}
}

func TestIntegration_ContentType(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	// All JSON responses should have proper content-type
	paths := []string{
		"/api/v1/rgds",
		"/api/v1/rgds/postgres-cluster",
		"/healthz",
		"/readyz",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected content-type application/json, got %s", contentType)
			}
		})
	}
}

func TestIntegration_Sorting(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	tests := []struct {
		name          string
		path          string
		expectedFirst string
	}{
		{
			name:          "sort by name ascending",
			path:          "/api/v1/rgds?sortBy=name&sortOrder=asc",
			expectedFirst: "mongodb",
		},
		{
			name:          "sort by name descending",
			path:          "/api/v1/rgds?sortBy=name&sortOrder=desc",
			expectedFirst: "redis-cache",
		},
		{
			name:          "sort alias works",
			path:          "/api/v1/rgds?sort=name&order=desc",
			expectedFirst: "redis-cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
			}

			var listResp handlers.ListRGDsResponse
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(listResp.Items) > 0 && listResp.Items[0].Name != tt.expectedFirst {
				t.Errorf("expected first item to be %s, got %s", tt.expectedFirst, listResp.Items[0].Name)
			}
		})
	}
}

func TestIntegration_ResponseFields(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	// Test that response contains all required fields
	resp, err := http.Get(server.URL + "/api/v1/rgds/postgres-cluster")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	var rgdResp services.RGDResponse
	if err := json.NewDecoder(resp.Body).Decode(&rgdResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check required fields
	if rgdResp.Name == "" {
		t.Error("name should not be empty")
	}
	if rgdResp.Namespace == "" {
		t.Error("namespace should not be empty")
	}
	if rgdResp.CreatedAt == "" {
		t.Error("createdAt should not be empty")
	}
	if rgdResp.UpdatedAt == "" {
		t.Error("updatedAt should not be empty")
	}
	if rgdResp.Tags == nil {
		t.Error("tags should never be nil")
	}
	if rgdResp.Labels == nil {
		t.Error("labels should never be nil")
	}
}

func TestIntegration_SecurityHeaders(t *testing.T) {
	server, _ := setupTestServer(t)
	defer server.Close()

	// Test all API endpoints to ensure security headers are applied
	endpoints := []string{
		"/api/v1/rgds",
		"/api/v1/rgds/postgres-cluster",
		"/healthz",
		"/readyz",
	}

	expectedHeaders := map[string]string{
		"Strict-Transport-Security":         "max-age=31536000; includeSubDomains; preload",
		"X-Frame-Options":                   "DENY",
		"X-Content-Type-Options":            "nosniff",
		"X-XSS-Protection":                  "1; mode=block",
		"Referrer-Policy":                   "strict-origin-when-cross-origin",
		"X-Permitted-Cross-Domain-Policies": "none",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Get(server.URL + endpoint)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			for header, expectedValue := range expectedHeaders {
				actualValue := resp.Header.Get(header)
				if actualValue != expectedValue {
					t.Errorf("endpoint %s: expected %s header to be %q, got %q",
						endpoint, header, expectedValue, actualValue)
				}
			}

			// CSP header is more complex, so test it separately
			csp := resp.Header.Get("Content-Security-Policy")
			expectedCSPFragments := []string{
				"default-src 'self'",
				"script-src 'self'",
				"frame-ancestors 'none'",
			}
			for _, fragment := range expectedCSPFragments {
				if !strings.Contains(csp, fragment) {
					t.Errorf("endpoint %s: CSP header should contain %q, got %q",
						endpoint, fragment, csp)
				}
			}
		})
	}
}
