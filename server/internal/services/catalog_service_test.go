// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/testutil"
)

// mockRGDProvider implements RGDProvider for testing
type mockRGDProvider struct {
	listFn      func(opts models.ListOptions) models.CatalogRGDList
	getFn       func(namespace, name string) (*models.CatalogRGD, bool)
	getByNameFn func(name string) (*models.CatalogRGD, bool)
}

func (m *mockRGDProvider) ListRGDs(opts models.ListOptions) models.CatalogRGDList {
	if m.listFn != nil {
		return m.listFn(opts)
	}
	return models.CatalogRGDList{}
}

func (m *mockRGDProvider) GetRGD(namespace, name string) (*models.CatalogRGD, bool) {
	if m.getFn != nil {
		return m.getFn(namespace, name)
	}
	return nil, false
}

func (m *mockRGDProvider) GetRGDByName(name string) (*models.CatalogRGD, bool) {
	if m.getByNameFn != nil {
		return m.getByNameFn(name)
	}
	return nil, false
}

// mockInstanceCounter implements InstanceCounter for testing
type mockInstanceCounter struct {
	countFn func(rgdNamespace, rgdName string, namespaces []string, matchFn func(string, []string) bool) int
}

func (m *mockInstanceCounter) CountInstancesByRGDAndNamespaces(rgdNamespace, rgdName string, namespaces []string, matchFn func(string, []string) bool) int {
	if m.countFn != nil {
		return m.countFn(rgdNamespace, rgdName, namespaces, matchFn)
	}
	return 0
}

// createTestRGD delegates to testutil.NewCatalogRGD.
func createTestRGD(name, namespace string, labels map[string]string) models.CatalogRGD {
	var opts []testutil.CatalogRGDOption
	if labels != nil {
		opts = append(opts, testutil.WithCatalogLabels(labels))
	}
	return testutil.NewCatalogRGD(name, namespace, opts...)
}

func TestNewCatalogService(t *testing.T) {
	provider := &mockRGDProvider{}
	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	assert.NotNil(t, svc)
	assert.NotNil(t, svc.logger)
	assert.Equal(t, provider, svc.rgdProvider)
}

func TestCatalogService_ListRGDs_NilProvider(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	result, err := svc.ListRGDs(context.Background(), nil, RGDFilters{})

	assert.Error(t, err)
	assert.Equal(t, ErrServiceUnavailable, err)
	assert.Nil(t, result)
}

func TestCatalogService_ListRGDs_Success(t *testing.T) {
	testRGD := createTestRGD("test-rgd", "default", nil)

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{testRGD},
				TotalCount: 1,
				Page:       1,
				PageSize:   20,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.ListRGDs(context.Background(), nil, RGDFilters{})

	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "test-rgd", result.Items[0].Name)
	assert.Equal(t, 1, result.TotalCount)
}

func TestCatalogService_ListRGDs_WithAuthContext(t *testing.T) {
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			// Verify that projects and includePublic are set from authCtx
			assert.Equal(t, []string{"project-a", "project-b"}, opts.Projects)
			assert.True(t, opts.IncludePublic)
			return models.CatalogRGDList{}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	authCtx := &UserAuthContext{
		UserID:             "user-1",
		AccessibleProjects: []string{"project-a", "project-b"},
	}

	_, err := svc.ListRGDs(context.Background(), authCtx, RGDFilters{})
	require.NoError(t, err)
}

func TestCatalogService_ListRGDs_WithFilters(t *testing.T) {
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			assert.Equal(t, "production", opts.Namespace)
			assert.Equal(t, "Databases", opts.Category)
			assert.Equal(t, []string{"postgres", "ha"}, opts.Tags)
			assert.Equal(t, "my-search", opts.Search)
			assert.Equal(t, 2, opts.Page)
			assert.Equal(t, 50, opts.PageSize)
			assert.Equal(t, "createdAt", opts.SortBy)
			assert.Equal(t, "desc", opts.SortOrder)
			return models.CatalogRGDList{}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	filters := RGDFilters{
		Namespace: "production",
		Category:  "Databases",
		Tags:      []string{"postgres", "ha"},
		Search:    "my-search",
		Page:      2,
		PageSize:  50,
		SortBy:    "createdAt",
		SortOrder: "desc",
	}

	_, err := svc.ListRGDs(context.Background(), nil, filters)
	require.NoError(t, err)
}

func TestCatalogService_ListRGDs_WithInstanceCounter(t *testing.T) {
	testRGD := createTestRGD("test-rgd", "default", nil)

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{testRGD},
				TotalCount: 1,
				Page:       1,
				PageSize:   20,
			}
		},
	}

	counter := &mockInstanceCounter{
		countFn: func(rgdNamespace, rgdName string, namespaces []string, matchFn func(string, []string) bool) int {
			return 5 // Return filtered count
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:     provider,
		InstanceCounter: counter,
	})

	authCtx := &UserAuthContext{
		UserID:               "user-1",
		AccessibleNamespaces: []string{"ns-a", "ns-b"},
	}

	result, err := svc.ListRGDs(context.Background(), authCtx, RGDFilters{})

	require.NoError(t, err)
	assert.Equal(t, 5, result.Items[0].Instances)
}

func TestCatalogService_GetRGD_NilProvider(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	result, err := svc.GetRGD(context.Background(), nil, "test", "default")

	assert.Error(t, err)
	assert.Equal(t, ErrServiceUnavailable, err)
	assert.Nil(t, result)
}

func TestCatalogService_GetRGD_NotFound(t *testing.T) {
	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return nil, false
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.GetRGD(context.Background(), nil, "nonexistent", "default")

	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
	assert.Nil(t, result)
}

func TestCatalogService_GetRGD_WithNamespace(t *testing.T) {
	testRGD := createTestRGD("test-rgd", "production", nil)

	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			assert.Equal(t, "production", namespace)
			assert.Equal(t, "test-rgd", name)
			return &testRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.GetRGD(context.Background(), nil, "test-rgd", "production")

	require.NoError(t, err)
	assert.Equal(t, "test-rgd", result.Name)
	assert.Equal(t, "production", result.Namespace)
}

func TestCatalogService_GetRGD_WithoutNamespace(t *testing.T) {
	testRGD := createTestRGD("test-rgd", "default", nil)

	provider := &mockRGDProvider{
		getByNameFn: func(name string) (*models.CatalogRGD, bool) {
			assert.Equal(t, "test-rgd", name)
			return &testRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.GetRGD(context.Background(), nil, "test-rgd", "")

	require.NoError(t, err)
	assert.Equal(t, "test-rgd", result.Name)
}

func TestCatalogService_GetRGD_PublicRGDAccessible(t *testing.T) {
	// Public RGD (no project label)
	testRGD := createTestRGD("public-rgd", "default", nil)

	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return &testRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	authCtx := &UserAuthContext{
		UserID:             "user-1",
		AccessibleProjects: []string{"project-a"},
	}

	result, err := svc.GetRGD(context.Background(), authCtx, "public-rgd", "default")

	require.NoError(t, err)
	assert.Equal(t, "public-rgd", result.Name)
}

func TestCatalogService_GetRGD_ProjectRGDAccessible(t *testing.T) {
	// Project-scoped RGD that user has access to
	testRGD := createTestRGD("project-rgd", "default", map[string]string{
		models.RGDProjectLabel: "project-a",
	})

	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return &testRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	authCtx := &UserAuthContext{
		UserID:             "user-1",
		AccessibleProjects: []string{"project-a", "project-b"},
	}

	result, err := svc.GetRGD(context.Background(), authCtx, "project-rgd", "default")

	require.NoError(t, err)
	assert.Equal(t, "project-rgd", result.Name)
}

func TestCatalogService_GetRGD_ProjectRGDForbidden(t *testing.T) {
	// Project-scoped RGD that user does NOT have access to
	testRGD := createTestRGD("project-rgd", "default", map[string]string{
		models.RGDProjectLabel: "other-project",
	})

	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return &testRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	authCtx := &UserAuthContext{
		UserID:             "user-1",
		AccessibleProjects: []string{"project-a", "project-b"},
	}

	result, err := svc.GetRGD(context.Background(), authCtx, "project-rgd", "default")

	assert.Error(t, err)
	assert.Equal(t, ErrForbidden, err)
	assert.Nil(t, result)
}

func TestCatalogService_GetCount_NilProvider(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	count, err := svc.GetCount(context.Background(), nil)

	assert.Error(t, err)
	assert.Equal(t, ErrServiceUnavailable, err)
	assert.Equal(t, 0, count)
}

func TestCatalogService_GetCount_Success(t *testing.T) {
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			assert.Equal(t, 1, opts.Page)
			assert.Equal(t, 1, opts.PageSize)
			return models.CatalogRGDList{
				TotalCount: 42,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	count, err := svc.GetCount(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestCatalogService_GetCount_WithAuthContext(t *testing.T) {
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			assert.Equal(t, []string{"project-a"}, opts.Projects)
			assert.True(t, opts.IncludePublic)
			return models.CatalogRGDList{TotalCount: 10}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	authCtx := &UserAuthContext{
		UserID:             "user-1",
		AccessibleProjects: []string{"project-a"},
	}

	count, err := svc.GetCount(context.Background(), authCtx)

	require.NoError(t, err)
	assert.Equal(t, 10, count)
}

func TestCatalogService_GetFilters_NilProvider(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	result, err := svc.GetFilters(context.Background(), nil)

	assert.Error(t, err)
	assert.Equal(t, ErrServiceUnavailable, err)
	assert.Nil(t, result)
}

func TestCatalogService_GetFilters_Success(t *testing.T) {
	rgds := []models.CatalogRGD{
		{
			Name:     "rgd-1",
			Labels:   map[string]string{models.RGDProjectLabel: "project-a"},
			Tags:     []string{"database", "postgres"},
			Category: "Databases",
		},
		{
			Name:     "rgd-2",
			Labels:   map[string]string{models.RGDProjectLabel: "project-b"},
			Tags:     []string{"cache", "redis"},
			Category: "Caching",
		},
		{
			Name:     "rgd-3",
			Labels:   nil, // Public RGD
			Tags:     []string{"database"},
			Category: "Databases",
		},
	}

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      rgds,
				TotalCount: 3,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.GetFilters(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"project-a", "project-b"}, result.Projects)
	assert.Equal(t, []string{"cache", "database", "postgres", "redis"}, result.Tags)
	assert.Equal(t, []string{"Caching", "Databases"}, result.Categories)
}

func TestCatalogService_GetFilters_WithCaching(t *testing.T) {
	// Start miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	callCount := 0
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			callCount++
			return models.CatalogRGDList{
				Items: []models.CatalogRGD{
					{Name: "rgd-1", Category: "Testing"},
				},
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		RedisClient: redisClient,
	})

	// First call - should hit provider
	result1, err := svc.GetFilters(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, []string{"Testing"}, result1.Categories)

	// Second call - should hit cache
	result2, err := svc.GetFilters(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount) // Still 1 - cache hit
	assert.Equal(t, result1.Categories, result2.Categories)
}

func TestCatalogService_ListRGDs_WithCaching(t *testing.T) {
	// Start miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	callCount := 0
	testRGD := createTestRGD("test-rgd", "default", nil)

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			callCount++
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{testRGD},
				TotalCount: 1,
				Page:       1,
				PageSize:   20,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		RedisClient: redisClient,
	})

	filters := RGDFilters{Page: 1, PageSize: 20}

	// First call - should hit provider
	result1, err := svc.ListRGDs(context.Background(), nil, filters)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Len(t, result1.Items, 1)

	// Second call with same filters - should hit cache
	result2, err := svc.ListRGDs(context.Background(), nil, filters)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount) // Still 1 - cache hit
	assert.Len(t, result2.Items, 1)
}

func TestCatalogService_canAccessRGD(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	tests := []struct {
		name       string
		rgdLabels  map[string]string
		authCtx    *UserAuthContext
		wantAccess bool
	}{
		{
			name:       "nil auth context",
			rgdLabels:  nil,
			authCtx:    nil,
			wantAccess: false,
		},
		{
			name:      "public RGD - no labels",
			rgdLabels: nil,
			authCtx: &UserAuthContext{
				UserID:             "user-1",
				AccessibleProjects: []string{"project-a"},
			},
			wantAccess: true,
		},
		{
			name:      "public RGD - empty labels",
			rgdLabels: map[string]string{},
			authCtx: &UserAuthContext{
				UserID:             "user-1",
				AccessibleProjects: []string{"project-a"},
			},
			wantAccess: true,
		},
		{
			name:      "project RGD - user has access",
			rgdLabels: map[string]string{models.RGDProjectLabel: "project-a"},
			authCtx: &UserAuthContext{
				UserID:             "user-1",
				AccessibleProjects: []string{"project-a", "project-b"},
			},
			wantAccess: true,
		},
		{
			name:      "project RGD - user does not have access",
			rgdLabels: map[string]string{models.RGDProjectLabel: "other-project"},
			authCtx: &UserAuthContext{
				UserID:             "user-1",
				AccessibleProjects: []string{"project-a", "project-b"},
			},
			wantAccess: false,
		},
		{
			name:      "project RGD - user has no projects",
			rgdLabels: map[string]string{models.RGDProjectLabel: "project-a"},
			authCtx: &UserAuthContext{
				UserID:             "user-1",
				AccessibleProjects: []string{},
			},
			wantAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgd := &models.CatalogRGD{
				Name:   "test-rgd",
				Labels: tt.rgdLabels,
			}
			got := svc.canAccessRGD(rgd, tt.authCtx)
			assert.Equal(t, tt.wantAccess, got)
		})
	}
}

func TestCatalogService_getFilteredInstanceCount(t *testing.T) {
	testRGD := &models.CatalogRGD{
		Name:          "test-rgd",
		Namespace:     "default",
		InstanceCount: 10, // Default count
	}

	t.Run("without instance counter - uses RGD count", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{})
		count := svc.getFilteredInstanceCount(testRGD, nil)
		assert.Equal(t, 10, count)
	})

	t.Run("with instance counter - uses filtered count", func(t *testing.T) {
		counter := &mockInstanceCounter{
			countFn: func(rgdNamespace, rgdName string, namespaces []string, matchFn func(string, []string) bool) int {
				assert.Equal(t, "default", rgdNamespace)
				assert.Equal(t, "test-rgd", rgdName)
				assert.Equal(t, []string{"ns-a"}, namespaces)
				return 5
			},
		}

		svc := NewCatalogService(CatalogServiceConfig{
			InstanceCounter: counter,
		})

		authCtx := &UserAuthContext{
			AccessibleNamespaces: []string{"ns-a"},
		}

		count := svc.getFilteredInstanceCount(testRGD, authCtx)
		assert.Equal(t, 5, count)
	})
}

func TestCatalogService_filtersToListOptions(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	t.Run("empty filters - uses defaults", func(t *testing.T) {
		opts := svc.filtersToListOptions(RGDFilters{})
		assert.Equal(t, 1, opts.Page)
		assert.Equal(t, 20, opts.PageSize)
		assert.Equal(t, "name", opts.SortBy)
		assert.Equal(t, "asc", opts.SortOrder)
	})

	t.Run("full filters - all fields set", func(t *testing.T) {
		filters := RGDFilters{
			Namespace: "prod",
			Category:  "DB",
			Tags:      []string{"a", "b"},
			Search:    "search",
			Page:      3,
			PageSize:  50,
			SortBy:    "createdAt",
			SortOrder: "desc",
		}

		opts := svc.filtersToListOptions(filters)

		assert.Equal(t, "prod", opts.Namespace)
		assert.Equal(t, "DB", opts.Category)
		assert.Equal(t, []string{"a", "b"}, opts.Tags)
		assert.Equal(t, "search", opts.Search)
		assert.Equal(t, 3, opts.Page)
		assert.Equal(t, 50, opts.PageSize)
		assert.Equal(t, "createdAt", opts.SortBy)
		assert.Equal(t, "desc", opts.SortOrder)
	})
}

func TestCatalogService_listCacheKey(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	opts1 := models.ListOptions{
		Namespace:     "ns1",
		Projects:      []string{"b", "a"}, // Out of order
		Tags:          []string{"z", "a"}, // Out of order
		Page:          1,
		PageSize:      20,
		IncludePublic: true,
		SortBy:        "name",
		SortOrder:     "asc",
	}

	opts2 := models.ListOptions{
		Namespace:     "ns1",
		Projects:      []string{"a", "b"}, // In order
		Tags:          []string{"a", "z"}, // In order
		Page:          1,
		PageSize:      20,
		IncludePublic: true,
		SortBy:        "name",
		SortOrder:     "asc",
	}

	// Keys should be identical regardless of project/tag order
	key1 := svc.listCacheKey(opts1)
	key2 := svc.listCacheKey(opts2)
	assert.Equal(t, key1, key2)
}

func TestCatalogService_filtersCacheKey(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	// Keys should be identical regardless of project order
	key1 := svc.filtersCacheKey([]string{"b", "a"}, true)
	key2 := svc.filtersCacheKey([]string{"a", "b"}, true)
	assert.Equal(t, key1, key2)

	// Keys should differ with different includePublic
	key3 := svc.filtersCacheKey([]string{"a", "b"}, false)
	assert.NotEqual(t, key2, key3)
}

func TestCatalogService_GetFilters_EmptyRGDs(t *testing.T) {
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{},
				TotalCount: 0,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.GetFilters(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, result.Projects)
	assert.Empty(t, result.Tags)
	assert.Empty(t, result.Categories)
}

func TestCatalogService_CacheInvalidation(t *testing.T) {
	// Start miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: &mockRGDProvider{
			listFn: func(opts models.ListOptions) models.CatalogRGDList {
				return models.CatalogRGDList{
					Items: []models.CatalogRGD{{Name: "rgd-1"}},
				}
			},
		},
		RedisClient: redisClient,
	})

	// Populate cache
	_, err = svc.ListRGDs(context.Background(), nil, RGDFilters{})
	require.NoError(t, err)

	// Manually expire cache (simulate TTL expiry via miniredis)
	mr.FastForward(31 * time.Second)

	// Verify key expired
	exists, err := redisClient.Exists(context.Background(), svc.listCacheKey(models.DefaultListOptions())).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

func TestCatalogService_CacheCorruptedData(t *testing.T) {
	// Start miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	callCount := 0
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			callCount++
			return models.CatalogRGDList{
				Items: []models.CatalogRGD{{Name: "rgd-1"}},
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		RedisClient: redisClient,
	})

	// Inject corrupted data into cache
	cacheKey := svc.listCacheKey(models.DefaultListOptions())
	err = redisClient.Set(context.Background(), cacheKey, "not valid json", time.Hour).Err()
	require.NoError(t, err)

	// Service should gracefully handle corrupted cache and fetch from provider
	result, err := svc.ListRGDs(context.Background(), nil, RGDFilters{})
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, 1, callCount)
}

func TestCatalogService_GetFilters_CacheCorruptedData(t *testing.T) {
	// Start miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	callCount := 0
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			callCount++
			return models.CatalogRGDList{
				Items: []models.CatalogRGD{{Category: "Testing"}},
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		RedisClient: redisClient,
	})

	// Inject corrupted data into cache
	cacheKey := svc.filtersCacheKey(nil, true)
	err = redisClient.Set(context.Background(), cacheKey, "corrupted", time.Hour).Err()
	require.NoError(t, err)

	// Service should gracefully handle corrupted cache
	result, err := svc.GetFilters(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"Testing"}, result.Categories)
	assert.Equal(t, 1, callCount)
}

func TestCatalogService_GetFilters_ExcludesEmptyValues(t *testing.T) {
	rgds := []models.CatalogRGD{
		{
			Name:     "rgd-1",
			Labels:   map[string]string{models.RGDProjectLabel: ""}, // Empty project
			Tags:     []string{"", "valid-tag", ""},                 // Empty tags
			Category: "",                                            // Empty category
		},
		{
			Name:     "rgd-2",
			Tags:     []string{"another-tag"},
			Category: "ValidCategory",
		},
	}

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{Items: rgds}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.GetFilters(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, result.Projects)                                   // Empty project excluded
	assert.Equal(t, []string{"another-tag", "valid-tag"}, result.Tags) // Empty tags excluded
	assert.Equal(t, []string{"ValidCategory"}, result.Categories)      // Empty category excluded
}

func TestCatalogService_ListRGDs_ProviderReturnsEmpty(t *testing.T) {
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{},
				TotalCount: 0,
				Page:       1,
				PageSize:   20,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
	})

	result, err := svc.ListRGDs(context.Background(), nil, RGDFilters{})

	require.NoError(t, err)
	assert.Empty(t, result.Items)
	assert.Equal(t, 0, result.TotalCount)
}

// Organization filter tests (Story 1.3)

func TestCatalogService_OrgFilter_ListRGDsSetsOrganization(t *testing.T) {
	var capturedOpts models.ListOptions
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			capturedOpts = opts
			return models.CatalogRGDList{Items: []models.CatalogRGD{}, TotalCount: 0, Page: 1, PageSize: 20}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	_, err := svc.ListRGDs(context.Background(), nil, RGDFilters{})
	require.NoError(t, err)
	assert.Equal(t, "orgA", capturedOpts.Organization, "ListRGDs should set Organization on list options")
}

func TestCatalogService_OrgFilter_GetCountSetsOrganization(t *testing.T) {
	var capturedOpts models.ListOptions
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			capturedOpts = opts
			return models.CatalogRGDList{TotalCount: 5}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	count, err := svc.GetCount(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.Equal(t, "orgA", capturedOpts.Organization, "GetCount should set Organization on list options")
}

func TestCatalogService_OrgFilter_GetFiltersSetsOrganization(t *testing.T) {
	var capturedOpts models.ListOptions
	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			capturedOpts = opts
			return models.CatalogRGDList{Items: []models.CatalogRGD{}, TotalCount: 0}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	_, err := svc.GetFilters(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "orgA", capturedOpts.Organization, "GetFilters should set Organization on list options")
}

func TestCatalogService_OrgFilter_CanAccessRGDMatchingOrg(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        &mockRGDProvider{},
		OrganizationFilter: "orgA",
	})

	authCtx := &UserAuthContext{UserID: "user1"}

	// RGD matching org — accessible
	rgd := &models.CatalogRGD{
		Name:         "test-rgd",
		Organization: "orgA",
		Annotations:  map[string]string{models.CatalogAnnotation: "true"},
	}
	assert.True(t, svc.canAccessRGD(rgd, authCtx), "matching org should be accessible")
}

func TestCatalogService_OrgFilter_CanAccessRGDIgnoresOrg(t *testing.T) {
	// canAccessRGD no longer checks org — that's handled in GetRGD (returns 404).
	// canAccessRGD only checks project-level access.
	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        &mockRGDProvider{},
		OrganizationFilter: "orgA",
	})

	authCtx := &UserAuthContext{UserID: "user1"}

	// RGD with different org but no project label = public → accessible via canAccessRGD
	// (GetRGD handles org filtering separately with 404)
	rgd := &models.CatalogRGD{
		Name:         "test-rgd",
		Organization: "orgB",
		Annotations:  map[string]string{models.CatalogAnnotation: "true"},
	}
	assert.True(t, svc.canAccessRGD(rgd, authCtx), "canAccessRGD should not check org (handled by GetRGD)")
}

func TestCatalogService_OrgFilter_CanAccessRGDShared(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        &mockRGDProvider{},
		OrganizationFilter: "orgA",
	})

	authCtx := &UserAuthContext{UserID: "user1"}

	// Shared RGD (no org annotation) — accessible
	rgd := &models.CatalogRGD{
		Name:        "shared-rgd",
		Annotations: map[string]string{models.CatalogAnnotation: "true"},
	}
	assert.True(t, svc.canAccessRGD(rgd, authCtx), "shared RGD should be accessible")
}

func TestCatalogService_OrgFilter_CanAccessRGDNoFilter(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        &mockRGDProvider{},
		OrganizationFilter: "", // OSS - no filter
	})

	authCtx := &UserAuthContext{UserID: "user1"}

	// RGD with org annotation — accessible when no filter set
	rgd := &models.CatalogRGD{
		Name:         "test-rgd",
		Organization: "orgB",
		Annotations:  map[string]string{models.CatalogAnnotation: "true"},
	}
	assert.True(t, svc.canAccessRGD(rgd, authCtx), "all RGDs accessible when no org filter")
}

func TestCatalogService_OrgFilter_GetFiltersReturnsOrgScopedResults(t *testing.T) {
	// Verifies that GetFilters returns only filter values from org-visible RGDs
	rgds := []models.CatalogRGD{
		{
			Name:         "orgA-db",
			Organization: "orgA",
			Tags:         []string{"database"},
			Category:     "Databases",
		},
		{
			Name:         "orgB-cache",
			Organization: "orgB",
			Tags:         []string{"cache"},
			Category:     "Caching",
		},
		{
			Name:     "shared-queue",
			Tags:     []string{"messaging"},
			Category: "Messaging",
		},
	}

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			// Simulate org filtering (as the real cache would do)
			var filtered []models.CatalogRGD
			for _, rgd := range rgds {
				if opts.Organization != "" && rgd.Organization != "" && rgd.Organization != opts.Organization {
					continue
				}
				filtered = append(filtered, rgd)
			}
			return models.CatalogRGDList{Items: filtered, TotalCount: len(filtered)}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	result, err := svc.GetFilters(context.Background(), nil)
	require.NoError(t, err)

	// Should only see tags/categories from orgA + shared, NOT orgB
	assert.Equal(t, []string{"Databases", "Messaging"}, result.Categories)
	assert.Equal(t, []string{"database", "messaging"}, result.Tags)
	// orgB's "cache" tag and "Caching" category should NOT appear
	for _, tag := range result.Tags {
		assert.NotEqual(t, "cache", tag, "orgB tags should not appear in orgA-filtered results")
	}
}

func TestCatalogService_OrgFilter_CacheKeyIncludesOrg(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        &mockRGDProvider{},
		OrganizationFilter: "orgA",
	})

	opts1 := models.ListOptions{Organization: "orgA", Page: 1, PageSize: 20, SortBy: "name", SortOrder: "asc"}
	opts2 := models.ListOptions{Organization: "orgB", Page: 1, PageSize: 20, SortBy: "name", SortOrder: "asc"}
	opts3 := models.ListOptions{Organization: "", Page: 1, PageSize: 20, SortBy: "name", SortOrder: "asc"}

	key1 := svc.listCacheKey(opts1)
	key2 := svc.listCacheKey(opts2)
	key3 := svc.listCacheKey(opts3)

	assert.NotEqual(t, key1, key2, "different orgs should produce different cache keys")
	assert.NotEqual(t, key1, key3, "org vs no-org should produce different cache keys")
	assert.Contains(t, key1, "org=orgA", "cache key should contain org value")
}

func TestCatalogService_OrgFilter_GetRGDHidesNonMatchingOrg(t *testing.T) {
	provider := &mockRGDProvider{
		getByNameFn: func(name string) (*models.CatalogRGD, bool) {
			return &models.CatalogRGD{
				Name:         "rgd-orgB",
				Organization: "orgB",
				Annotations:  map[string]string{models.CatalogAnnotation: "true"},
			}, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	authCtx := &UserAuthContext{UserID: "user1"}
	_, err := svc.GetRGD(context.Background(), authCtx, "rgd-orgB", "")
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err, "GetRGD should return 404 to hide non-matching org RGD existence (AC #7)")
}

func TestCatalogService_OrgFilter_GetRGDAllowsMatchingOrg(t *testing.T) {
	provider := &mockRGDProvider{
		getByNameFn: func(name string) (*models.CatalogRGD, bool) {
			return &models.CatalogRGD{
				Name:         "rgd-orgA",
				Organization: "orgA",
				Annotations:  map[string]string{models.CatalogAnnotation: "true"},
			}, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	authCtx := &UserAuthContext{UserID: "user1"}
	result, err := svc.GetRGD(context.Background(), authCtx, "rgd-orgA", "")
	require.NoError(t, err)
	assert.Equal(t, "rgd-orgA", result.Name)
}

func TestCatalogService_OrgFilter_GetRGDAllowsSharedRGD(t *testing.T) {
	provider := &mockRGDProvider{
		getByNameFn: func(name string) (*models.CatalogRGD, bool) {
			return &models.CatalogRGD{
				Name:        "shared-rgd",
				Annotations: map[string]string{models.CatalogAnnotation: "true"},
			}, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "orgA",
	})

	authCtx := &UserAuthContext{UserID: "user1"}
	result, err := svc.GetRGD(context.Background(), authCtx, "shared-rgd", "")
	require.NoError(t, err)
	assert.Equal(t, "shared-rgd", result.Name)
}

func TestCatalogService_OrgFilter_GetRGDNoFilterShowsAll(t *testing.T) {
	provider := &mockRGDProvider{
		getByNameFn: func(name string) (*models.CatalogRGD, bool) {
			return &models.CatalogRGD{
				Name:         "rgd-anyorg",
				Organization: "orgB",
				Annotations:  map[string]string{models.CatalogAnnotation: "true"},
			}, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider:        provider,
		OrganizationFilter: "", // OSS — no filter
	})

	authCtx := &UserAuthContext{UserID: "user1"}
	result, err := svc.GetRGD(context.Background(), authCtx, "rgd-anyorg", "")
	require.NoError(t, err)
	assert.Equal(t, "rgd-anyorg", result.Name)
}

func TestCatalogService_CacheMarshalError(t *testing.T) {
	// This test verifies the service handles marshal errors gracefully
	// We can't easily force a marshal error with valid types, so we verify the code path exists
	// by checking that the service functions correctly even if caching fails

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	// Close redis to simulate connection error
	mr.Close()

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items: []models.CatalogRGD{{Name: "rgd-1"}},
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		RedisClient: redisClient,
	})

	// Should still work even with Redis errors
	result, err := svc.ListRGDs(context.Background(), nil, RGDFilters{})
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
}

func TestCatalogService_GetFilters_CacheHitWithValidData(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer redisClient.Close()

	// Pre-populate cache with valid data
	cachedOpts := RGDFilterOptions{
		Projects:   []string{"cached-project"},
		Tags:       []string{"cached-tag"},
		Categories: []string{"CachedCategory"},
	}
	data, _ := json.Marshal(cachedOpts)

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: &mockRGDProvider{},
		RedisClient: redisClient,
	})

	cacheKey := svc.filtersCacheKey(nil, true)
	err = redisClient.Set(context.Background(), cacheKey, data, time.Hour).Err()
	require.NoError(t, err)

	result, err := svc.GetFilters(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, cachedOpts.Projects, result.Projects)
	assert.Equal(t, cachedOpts.Tags, result.Tags)
	assert.Equal(t, cachedOpts.Categories, result.Categories)
}
