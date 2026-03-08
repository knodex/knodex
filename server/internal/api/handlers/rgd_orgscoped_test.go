package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRGDWatcherWithNamespaces creates a test watcher with RGDs in different namespaces
func mockRGDWatcherWithNamespaces(t *testing.T) *watcher.RGDWatcher {
	cache := watcher.NewRGDCache()

	// Add RGDs to different project namespaces - all must have catalog annotation to be visible
	testRGDs := []models.CatalogRGD{
		{
			Name:        "postgres-cluster",
			Namespace:   "project-1-ns",
			Description: "PostgreSQL cluster in project 1",
			Version:     "1.0.0",
			Tags:        []string{"database", "production"},
			Category:    "database",
			Labels:      map[string]string{"project": "project-1"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-24 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        "redis-cache",
			Namespace:   "project-1-ns",
			Description: "Redis cache in project 1",
			Version:     "2.0.0",
			Tags:        []string{"cache", "production"},
			Category:    "cache",
			Labels:      map[string]string{"project": "project-1"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-12 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        "mongodb",
			Namespace:   "project-2-ns",
			Description: "MongoDB in project 2",
			Version:     "1.5.0",
			Tags:        []string{"database", "staging"},
			Category:    "database",
			Labels:      map[string]string{"project": "project-2"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-6 * time.Hour),
			UpdatedAt:   time.Now(),
		},
		{
			Name:        "mysql-cluster",
			Namespace:   "project-3-ns",
			Description: "MySQL in project 3",
			Version:     "1.0.0",
			Tags:        []string{"database"},
			Category:    "database",
			Labels:      map[string]string{"project": "project-3"},
			Annotations: map[string]string{models.CatalogAnnotation: "true"}, // Required for catalog visibility
			CreatedAt:   time.Now().Add(-1 * time.Hour),
			UpdatedAt:   time.Now(),
		},
	}

	for i := range testRGDs {
		cache.Set(&testRGDs[i])
	}

	return watcher.NewRGDWatcherWithCache(cache)
}

// createTestHandlerWithWatcher creates an RGDHandler with a CatalogService backed by the given watcher
func createTestHandlerWithWatcher(w *watcher.RGDWatcher, redisClient *redis.Client) *RGDHandler {
	catalogService := services.NewCatalogService(services.CatalogServiceConfig{
		RGDProvider: w,
		RedisClient: redisClient,
	})
	return NewRGDHandler(RGDHandlerConfig{
		CatalogService: catalogService,
	})
}

// TestRGDHandler_ListRGDs_WithoutAuthService tests behavior when authService is nil
// When auth is not configured, no project-scoped filtering is applied
func TestRGDHandler_ListRGDs_WithoutAuthService(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcherWithNamespaces(t)
	handler := createTestHandlerWithWatcher(w, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds", nil)
	rec := httptest.NewRecorder()

	handler.ListRGDs(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK")

	var listResp ListRGDsResponse
	err := json.NewDecoder(resp.Body).Decode(&listResp)
	require.NoError(t, err, "failed to decode response")

	// Without authService, all 4 RGDs are visible (no filtering)
	assert.Equal(t, 4, listResp.TotalCount, "expected all 4 RGDs without project filtering")
	assert.Len(t, listResp.Items, 4, "expected 4 items in response")
}

// TestRGDHandler_ListRGDs_NamespaceFilter tests explicit namespace filtering
func TestRGDHandler_ListRGDs_NamespaceFilter(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcherWithNamespaces(t)
	handler := createTestHandlerWithWatcher(w, nil)

	tests := []struct {
		name          string
		namespace     string
		expectedCount int
	}{
		{
			name:          "filter by project-1-ns",
			namespace:     "project-1-ns",
			expectedCount: 2,
		},
		{
			name:          "filter by project-2-ns",
			namespace:     "project-2-ns",
			expectedCount: 1,
		},
		{
			name:          "filter by project-3-ns",
			namespace:     "project-3-ns",
			expectedCount: 1,
		},
		{
			name:          "filter by nonexistent namespace",
			namespace:     "nonexistent",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds?namespace="+tt.namespace, nil)
			rec := httptest.NewRecorder()

			handler.ListRGDs(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			var listResp ListRGDsResponse
			err := json.NewDecoder(resp.Body).Decode(&listResp)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedCount, listResp.TotalCount, "unexpected RGD count for namespace %s", tt.namespace)

			// Verify all returned RGDs are from the requested namespace
			for _, item := range listResp.Items {
				assert.Equal(t, tt.namespace, item.Namespace)
			}
		})
	}
}

// TestRGDHandler_ListRGDs_RedisCacheBehavior tests Redis caching with explicit namespaces
func TestRGDHandler_ListRGDs_RedisCacheBehavior(t *testing.T) {
	t.Parallel()

	w := mockRGDWatcherWithNamespaces(t)

	// Create a real Redis client for testing
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for tests to avoid conflicts
	})

	// Check if Redis is available
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping cache test")
	}

	// Clean up test data before and after
	defer func() {
		redisClient.FlushDB(ctx)
		redisClient.Close()
	}()
	redisClient.FlushDB(ctx)

	handler := createTestHandlerWithWatcher(w, redisClient)

	// First request - cache miss, should populate cache
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/rgds?namespace=project-1-ns", nil)
	rec1 := httptest.NewRecorder()

	handler.ListRGDs(rec1, req1)

	resp1 := rec1.Result()
	defer resp1.Body.Close()

	require.Equal(t, http.StatusOK, resp1.StatusCode)

	var listResp1 ListRGDsResponse
	err := json.NewDecoder(resp1.Body).Decode(&listResp1)
	require.NoError(t, err)
	assert.Equal(t, 2, listResp1.TotalCount, "expected 2 RGDs on first request")

	// Verify cache key exists in Redis
	// Cache key format from catalog_service.listCacheKey():
	// rgd:list:org=%s:ns=%s:cat=%s:tags=%s:search=%s:projects=%s:public=%t:page=%d:size=%d:sort=%s:%s
	cacheKey := "rgd:list:org=:ns=project-1-ns:cat=:tags=:search=:projects=:public=false:page=1:size=20:sort=name:asc"
	exists, err := redisClient.Exists(ctx, cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "cache key should exist in Redis after first request")

	// Second request - should hit cache
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/rgds?namespace=project-1-ns", nil)
	rec2 := httptest.NewRecorder()

	handler.ListRGDs(rec2, req2)

	resp2 := rec2.Result()
	defer resp2.Body.Close()

	require.Equal(t, http.StatusOK, resp2.StatusCode)

	var listResp2 ListRGDsResponse
	err = json.NewDecoder(resp2.Body).Decode(&listResp2)
	require.NoError(t, err)
	assert.Equal(t, 2, listResp2.TotalCount, "expected 2 RGDs on cached request")

	// Responses should be identical
	assert.Equal(t, listResp1.TotalCount, listResp2.TotalCount)
	assert.Equal(t, len(listResp1.Items), len(listResp2.Items))
}

// NOTE: TestRGDHandler_SortRGDs_Performance test was removed as sorting logic
// has been moved to CatalogService. See catalog_service_test.go for sorting tests.

// NOTE: Full project-scoped filtering tests with real ProjectService
// require integration testing with Kubernetes clients. These tests verify:
// 1. Regular users see only their projects' namespaces
// 2. Global admins see all namespaces
// 3. Users with multiple projects see merged results
// 4. Users with no projects see no RGDs
//
// These scenarios are covered by integration tests and QA verification.
