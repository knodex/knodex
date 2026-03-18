// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
)

// mockRGDWatcher creates a test watcher with sample data
func mockRGDWatcher(t *testing.T) *watcher.RGDWatcher {
	cache := watcher.NewRGDCache()

	// Add test RGDs - all must have catalog annotation to be visible
	testRGDs := []models.CatalogRGD{
		{
			Name:        "postgres-cluster",
			Namespace:   "default",
			Description: "PostgreSQL cluster with high availability",
			Version:     "1.0.0",
			Tags:        []string{"database", "production"},
			Category:    "database",
			Labels:      map[string]string{"tier": "backend"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-24 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			Name:         "redis-cache",
			Namespace:    "default",
			Description:  "Redis cache for session storage",
			Version:      "2.0.0",
			Tags:         []string{"cache", "production"},
			Category:     "cache",
			Labels:       map[string]string{"tier": "backend"},
			Annotations:  map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			ExtendsKinds: []string{"PostgreSQLCluster"},                       // Extends postgres kind for filter tests
			CreatedAt:    time.Now().Add(-12 * time.Hour),
			UpdatedAt:    time.Now(),
		},
		{
			Name:        "mongodb",
			Namespace:   "staging",
			Description: "MongoDB document database",
			Version:     "1.5.0",
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

	// Create a mock watcher with the cache
	return watcher.NewRGDWatcherWithCache(cache)
}

// createTestHandler creates an RGDHandler with a CatalogService backed by the given watcher
func createTestHandler(w *watcher.RGDWatcher) *RGDHandler {
	catalogService := services.NewCatalogService(services.CatalogServiceConfig{
		RGDProvider: w,
	})
	return NewRGDHandler(RGDHandlerConfig{
		CatalogService: catalogService,
	})
}

func TestRGDHandler_ListRGDs(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		expectedCount  int
		checkResponse  func(*testing.T, *ListRGDsResponse)
	}{
		{
			name:           "list all RGDs",
			query:          "",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "filter by namespace",
			query:          "?namespace=default",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				for _, item := range resp.Items {
					if item.Namespace != "default" {
						t.Errorf("expected namespace 'default', got %s", item.Namespace)
					}
				}
			},
		},
		{
			name:           "filter by category",
			query:          "?category=database",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "filter by tags",
			query:          "?tags=production",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:           "search by name",
			query:          "?search=postgres",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				if len(resp.Items) > 0 && resp.Items[0].Name != "postgres-cluster" {
					t.Errorf("expected postgres-cluster, got %s", resp.Items[0].Name)
				}
			},
		},
		{
			name:           "pagination with page and pageSize",
			query:          "?page=1&pageSize=2",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				if resp.TotalCount != 3 {
					t.Errorf("expected totalCount 3, got %d", resp.TotalCount)
				}
				if resp.PageSize != 2 {
					t.Errorf("expected pageSize 2, got %d", resp.PageSize)
				}
			},
		},
		{
			name:           "pagination with limit alias",
			query:          "?limit=1",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name:           "sort by name descending",
			query:          "?sortBy=name&sortOrder=desc",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				if len(resp.Items) > 0 && resp.Items[0].Name != "redis-cache" {
					t.Errorf("expected first item to be redis-cache (desc sort), got %s", resp.Items[0].Name)
				}
			},
		},
		{
			name:           "sort alias works",
			query:          "?sort=name&order=asc",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				if len(resp.Items) > 0 && resp.Items[0].Name != "mongodb" {
					t.Errorf("expected first item to be mongodb (asc sort), got %s", resp.Items[0].Name)
				}
			},
		},
		{
			name:           "combined filters",
			query:          "?namespace=default&category=database",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				if len(resp.Items) > 0 && resp.Items[0].Name != "postgres-cluster" {
					t.Errorf("expected postgres-cluster, got %s", resp.Items[0].Name)
				}
			},
		},
		{
			name:           "filter by extendsKind",
			query:          "?extendsKind=PostgreSQLCluster",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			checkResponse: func(t *testing.T, resp *ListRGDsResponse) {
				if len(resp.Items) > 0 && resp.Items[0].Name != "redis-cache" {
					t.Errorf("expected redis-cache (extends PostgreSQLCluster), got %s", resp.Items[0].Name)
				}
			},
		},
		{
			name:           "filter by extendsKind with no matches",
			query:          "?extendsKind=NonExistentKind",
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds"+tt.query, nil)
			rec := httptest.NewRecorder()

			handler.ListRGDs(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			var listResp ListRGDsResponse
			if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(listResp.Items) != tt.expectedCount {
				t.Errorf("expected %d items, got %d", tt.expectedCount, len(listResp.Items))
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, &listResp)
			}
		})
	}
}

func TestRGDHandler_ListRGDs_ValidationErrors(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	tests := []struct {
		name          string
		query         string
		expectedField string
	}{
		{
			name:          "invalid page",
			query:         "?page=abc",
			expectedField: "page",
		},
		{
			name:          "page less than 1",
			query:         "?page=0",
			expectedField: "page",
		},
		{
			name:          "invalid pageSize",
			query:         "?pageSize=abc",
			expectedField: "pageSize",
		},
		{
			name:          "pageSize too large",
			query:         "?pageSize=101",
			expectedField: "pageSize",
		},
		{
			name:          "invalid sortBy",
			query:         "?sortBy=invalid",
			expectedField: "sortBy",
		},
		{
			name:          "invalid sortOrder",
			query:         "?sortOrder=invalid",
			expectedField: "sortOrder",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds"+tt.query, nil)
			rec := httptest.NewRecorder()

			handler.ListRGDs(rec, req)

			resp := rec.Result()
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

func TestRGDHandler_ListRGDs_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	// Handler with nil CatalogService
	handler := NewRGDHandler(RGDHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds", nil)
	rec := httptest.NewRecorder()

	handler.ListRGDs(rec, req)

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
}

func TestRGDHandler_GetRGD(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	tests := []struct {
		name           string
		rgdName        string
		query          string
		expectedStatus int
		checkResponse  func(*testing.T, *services.RGDResponse)
	}{
		{
			name:           "get existing RGD",
			rgdName:        "postgres-cluster",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *services.RGDResponse) {
				if resp.Name != "postgres-cluster" {
					t.Errorf("expected name 'postgres-cluster', got %s", resp.Name)
				}
				if resp.Description != "PostgreSQL cluster with high availability" {
					t.Errorf("expected correct description, got %s", resp.Description)
				}
			},
		},
		{
			name:           "get RGD with namespace filter",
			rgdName:        "postgres-cluster",
			query:          "?namespace=default",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *services.RGDResponse) {
				if resp.Namespace != "default" {
					t.Errorf("expected namespace 'default', got %s", resp.Namespace)
				}
			},
		},
		{
			name:           "get RGD from different namespace",
			rgdName:        "mongodb",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp *services.RGDResponse) {
				if resp.Namespace != "staging" {
					t.Errorf("expected namespace 'staging', got %s", resp.Namespace)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/"+tt.rgdName+tt.query, nil)
			req.SetPathValue("name", tt.rgdName)
			rec := httptest.NewRecorder()

			handler.GetRGD(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			var rgdResp services.RGDResponse
			if err := json.NewDecoder(resp.Body).Decode(&rgdResp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, &rgdResp)
			}
		})
	}
}

func TestRGDHandler_GetRGD_NotFound(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	tests := []struct {
		name    string
		rgdName string
		query   string
	}{
		{
			name:    "non-existent RGD",
			rgdName: "non-existent",
			query:   "",
		},
		{
			name:    "RGD in wrong namespace",
			rgdName: "postgres-cluster",
			query:   "?namespace=staging",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/"+tt.rgdName+tt.query, nil)
			req.SetPathValue("name", tt.rgdName)
			rec := httptest.NewRecorder()

			handler.GetRGD(rec, req)

			resp := rec.Result()
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

func TestRGDHandler_GetRGD_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	// Handler with nil CatalogService
	handler := NewRGDHandler(RGDHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/test", nil)
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.GetRGD(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestRGDHandler_ResponseFormat(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds", nil)
	rec := httptest.NewRecorder()

	handler.ListRGDs(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content-type application/json, got %s", contentType)
	}

	// Check response structure
	var listResp ListRGDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check that tags and labels are never nil (always arrays/objects)
	for _, item := range listResp.Items {
		if item.Tags == nil {
			t.Error("tags should never be nil, should be empty array")
		}
		if item.Labels == nil {
			t.Error("labels should never be nil, should be empty object")
		}
	}
}

func TestRGDHandler_EmptyTagsHandling(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	// Test with empty tags filter
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds?tags=,,,", nil)
	rec := httptest.NewRecorder()

	handler.ListRGDs(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListRGDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return all items since empty tags filter is ignored
	if listResp.TotalCount != 3 {
		t.Errorf("expected 3 items, got %d", listResp.TotalCount)
	}
}

func TestRGDHandler_GetCount(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/count", nil)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var countResp CountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// mockRGDWatcher adds 3 test RGDs
	if countResp.Count != 3 {
		t.Errorf("expected count 3, got %d", countResp.Count)
	}
}

func TestRGDHandler_GetCount_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	// Handler with nil CatalogService
	handler := NewRGDHandler(RGDHandlerConfig{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/count", nil)
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
}

func TestRGDHandler_GetCount_EmptyCache(t *testing.T) {
	t.Parallel()

	// Create an empty cache
	cache := watcher.NewRGDCache()
	w := watcher.NewRGDWatcherWithCache(cache)
	handler := createTestHandler(w)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/count", nil)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var countResp CountResponse
	if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty cache should return count 0
	if countResp.Count != 0 {
		t.Errorf("expected count 0, got %d", countResp.Count)
	}
}

func TestRGDHandler_GetCount_ResponseFormat(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcher(t)
	handler := createTestHandler(w)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/count", nil)
	rec := httptest.NewRecorder()

	handler.GetCount(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected content-type application/json, got %s", contentType)
	}

	// Check response structure - only count field should be present
	var rawResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(rawResp) != 1 {
		t.Errorf("expected exactly 1 field in response, got %d", len(rawResp))
	}

	if _, ok := rawResp["count"]; !ok {
		t.Error("response should contain 'count' field")
	}
}
