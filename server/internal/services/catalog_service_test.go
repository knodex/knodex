// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// --- GetRGD tier filtering tests (STORY-416 review fix H1) ---

func TestCatalogService_GetRGD_TierFiltering_AppUserBlockedFromInfraRGD(t *testing.T) {
	t.Parallel()

	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCatalogTier("infrastructure"))
	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return &infraRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		ProjectTypeResolver: &mockProjectTypeResolver{
			types: map[string]string{"my-app": "app"},
		},
	})

	authCtx := &UserAuthContext{
		UserID:               "dev",
		AccessibleProjects:   []string{"my-app"},
		AccessibleNamespaces: []string{"ns-1"},
	}

	result, err := svc.GetRGD(context.Background(), authCtx, "aks-cluster", "default")
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err, "app user should not see infrastructure-tier RGD")
	assert.Nil(t, result)
}

func TestCatalogService_GetRGD_TierFiltering_AppUserAllowedBothTier(t *testing.T) {
	t.Parallel()

	bothRGD := testutil.NewCatalogRGD("shared-db", "default", testutil.WithCatalogTier("both"))
	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return &bothRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		ProjectTypeResolver: &mockProjectTypeResolver{
			types: map[string]string{"my-app": "app"},
		},
	})

	authCtx := &UserAuthContext{
		UserID:               "dev",
		AccessibleProjects:   []string{"my-app"},
		AccessibleNamespaces: []string{"ns-1"},
	}

	result, err := svc.GetRGD(context.Background(), authCtx, "shared-db", "default")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "shared-db", result.Name)
}

func TestCatalogService_GetRGD_TierFiltering_GlobalAdminSeesAllTiers(t *testing.T) {
	t.Parallel()

	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCatalogTier("infrastructure"))
	provider := &mockRGDProvider{
		getFn: func(namespace, name string) (*models.CatalogRGD, bool) {
			return &infraRGD, true
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		ProjectTypeResolver: &mockProjectTypeResolver{
			types: map[string]string{"my-app": "app"},
		},
	})

	authCtx := &UserAuthContext{
		UserID:               "admin",
		AccessibleProjects:   []string{"my-app"},
		AccessibleNamespaces: []string{"*"}, // global admin
	}

	result, err := svc.GetRGD(context.Background(), authCtx, "aks-cluster", "default")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "aks-cluster", result.Name, "global admin should see infrastructure-tier RGD")
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
			got := svc.canAccessRGD(context.Background(), rgd, tt.authCtx)
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
	key1 := svc.filtersCacheKey([]string{"b", "a"}, true, nil)
	key2 := svc.filtersCacheKey([]string{"a", "b"}, true, nil)
	assert.Equal(t, key1, key2)

	// Keys should differ with different includePublic
	key3 := svc.filtersCacheKey([]string{"a", "b"}, false, nil)
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
	cacheKey := svc.filtersCacheKey(nil, true, nil)
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
	assert.True(t, svc.canAccessRGD(context.Background(), rgd, authCtx), "matching org should be accessible")
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
	assert.True(t, svc.canAccessRGD(context.Background(), rgd, authCtx), "canAccessRGD should not check org (handled by GetRGD)")
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
	assert.True(t, svc.canAccessRGD(context.Background(), rgd, authCtx), "shared RGD should be accessible")
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
	assert.True(t, svc.canAccessRGD(context.Background(), rgd, authCtx), "all RGDs accessible when no org filter")
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

	cacheKey := svc.filtersCacheKey(nil, true, nil)
	err = redisClient.Set(context.Background(), cacheKey, data, time.Hour).Err()
	require.NoError(t, err)

	result, err := svc.GetFilters(context.Background(), nil)

	require.NoError(t, err)
	assert.Equal(t, cachedOpts.Projects, result.Projects)
	assert.Equal(t, cachedOpts.Tags, result.Tags)
	assert.Equal(t, cachedOpts.Categories, result.Categories)
}

// categoryPolicyEnforcer implements PolicyEnforcer with object-aware authorization.
// It allows access based on a set of allowed object patterns using Casbin's keyMatch logic.
type categoryPolicyEnforcer struct {
	allowedObjects     []string // Objects/patterns that are allowed (e.g., "rgds/infrastructure/*")
	accessibleProjects []string
}

func (m *categoryPolicyEnforcer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return m.accessibleProjects, nil
}

func (m *categoryPolicyEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, object, _ string) (bool, error) {
	for _, allowed := range m.allowedObjects {
		if casbinKeyMatch(object, allowed) {
			return true, nil
		}
	}
	return false, nil
}

// casbinKeyMatch replicates Casbin v2 KeyMatch: checks the prefix of object up to (but not
// including) the FIRST '*' in pattern. A '*' matches any sequence of characters including '/'.
// This matches the exact implementation in github.com/casbin/casbin/v2/util.KeyMatch.
func casbinKeyMatch(object, pattern string) bool {
	if pattern == "*" {
		return true
	}
	i := strings.Index(pattern, "*")
	if i == -1 {
		return object == pattern
	}
	if len(object) > i {
		return object[:i] == pattern[:i]
	}
	return object == pattern[:i]
}

func TestListRGDs_CategoryScopedAuthorization(t *testing.T) {
	infraRGD := testutil.NewCatalogRGD("simple-aks-cluster", "default", testutil.WithCategory("infrastructure"))
	appRGD := testutil.NewCatalogRGD("web-app", "default", testutil.WithCategory("applications"))
	uncatRGD := testutil.NewCatalogRGD("misc-tool", "default", testutil.WithCategory(""))

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{infraRGD, appRGD, uncatRGD},
				TotalCount: 3,
				Page:       1,
				PageSize:   50,
			}
		},
	}

	t.Run("user with rgds/infrastructure/* sees only infrastructure RGDs", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			PolicyEnforcer: &categoryPolicyEnforcer{
				allowedObjects: []string{"rgds/infrastructure/*"},
			},
		})

		authCtx := &UserAuthContext{UserID: "user:infra-dev", Groups: []string{}}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)

		assert.Equal(t, 1, result.TotalCount)
		assert.Equal(t, "simple-aks-cluster", result.Items[0].Name)
	})

	t.Run("user with rgds/* sees all RGDs", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			PolicyEnforcer: &categoryPolicyEnforcer{
				allowedObjects: []string{"rgds/*"},
			},
		})

		authCtx := &UserAuthContext{UserID: "user:admin", Groups: []string{}}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)

		assert.Equal(t, 3, result.TotalCount)
	})

	t.Run("user with no RGD policy sees no RGDs", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			PolicyEnforcer: &categoryPolicyEnforcer{
				allowedObjects: []string{}, // No RGD access
			},
		})

		authCtx := &UserAuthContext{UserID: "user:no-access", Groups: []string{}}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)

		assert.Equal(t, 0, result.TotalCount)
		assert.Empty(t, result.Items)
	})

	t.Run("user with multiple category policies sees only those categories", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			PolicyEnforcer: &categoryPolicyEnforcer{
				allowedObjects: []string{"rgds/applications/*", "rgds/uncategorized/*"},
			},
		})

		authCtx := &UserAuthContext{UserID: "user:multi-cat", Groups: []string{}}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)

		assert.Equal(t, 2, result.TotalCount)
		names := make([]string, len(result.Items))
		for i, item := range result.Items {
			names[i] = item.Name
		}
		assert.Contains(t, names, "web-app")
		assert.Contains(t, names, "misc-tool")
	})
}

func TestGetRGD_CategoryScopedAuthorization(t *testing.T) {
	infraRGD := testutil.NewCatalogRGD("simple-aks-cluster", "default", testutil.WithCategory("infrastructure"))

	provider := &mockRGDProvider{
		getByNameFn: func(name string) (*models.CatalogRGD, bool) {
			if name == "simple-aks-cluster" {
				return &infraRGD, true
			}
			return nil, false
		},
	}

	t.Run("returns 403 when user lacks category access", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			PolicyEnforcer: &categoryPolicyEnforcer{
				allowedObjects: []string{"rgds/applications/*"}, // Has app access, not infra
			},
		})

		authCtx := &UserAuthContext{UserID: "user:app-dev", Groups: []string{}}
		_, err := svc.GetRGD(context.Background(), authCtx, "simple-aks-cluster", "")

		assert.ErrorIs(t, err, ErrForbidden)
	})

	t.Run("returns RGD when user has category access", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			PolicyEnforcer: &categoryPolicyEnforcer{
				allowedObjects: []string{"rgds/infrastructure/*"},
			},
		})

		authCtx := &UserAuthContext{UserID: "user:infra-dev", Groups: []string{}}
		result, err := svc.GetRGD(context.Background(), authCtx, "simple-aks-cluster", "")

		require.NoError(t, err)
		assert.Equal(t, "simple-aks-cluster", result.Name)
	})
}

func TestGetCount_CategoryScopedAuthorization(t *testing.T) {
	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCategory("infrastructure"))
	appRGD := testutil.NewCatalogRGD("web-app", "default", testutil.WithCategory("applications"))

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{infraRGD, appRGD},
				TotalCount: 2,
				Page:       1,
				PageSize:   opts.PageSize,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		PolicyEnforcer: &categoryPolicyEnforcer{
			allowedObjects: []string{"rgds/infrastructure/*"},
		},
	})

	authCtx := &UserAuthContext{UserID: "user:infra-dev", Groups: []string{}}
	count, err := svc.GetCount(context.Background(), authCtx)

	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetFilters_CategoryScopedAuthorization(t *testing.T) {
	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCategory("infrastructure"))
	appRGD := testutil.NewCatalogRGD("web-app", "default", testutil.WithCategory("applications"))

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{infraRGD, appRGD},
				TotalCount: 2,
				Page:       1,
				PageSize:   opts.PageSize,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		PolicyEnforcer: &categoryPolicyEnforcer{
			allowedObjects: []string{"rgds/infrastructure/*"},
		},
	})

	authCtx := &UserAuthContext{UserID: "user:infra-dev", Groups: []string{}}
	result, err := svc.GetFilters(context.Background(), authCtx)

	require.NoError(t, err)
	// Only infrastructure category should be in filters
	assert.Equal(t, []string{"infrastructure"}, result.Categories)
}

func TestGetFilters_CacheBypassedWhenPolicyEnforcerActive(t *testing.T) {
	// When policyEnforcer is configured, filter results are per-user and must not be
	// shared via Redis cache. Verify that two users with different Casbin scopes receive
	// independent filter results even under the same project membership.
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCategory("infrastructure"))
	appRGD := testutil.NewCatalogRGD("web-app", "default", testutil.WithCategory("applications"))

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{Items: []models.CatalogRGD{infraRGD, appRGD}, TotalCount: 2}
		},
	}

	// User A: restricted to infrastructure only
	infraOnly := NewCatalogService(CatalogServiceConfig{
		RGDProvider:    provider,
		RedisClient:    rc,
		PolicyEnforcer: &categoryPolicyEnforcer{allowedObjects: []string{"rgds/infrastructure/*"}},
	})
	// User B: access to all RGDs
	allAccess := NewCatalogService(CatalogServiceConfig{
		RGDProvider:    provider,
		RedisClient:    rc,
		PolicyEnforcer: &categoryPolicyEnforcer{allowedObjects: []string{"rgds/*"}},
	})

	ctxA := &UserAuthContext{UserID: "user:infra-dev", Groups: []string{}}
	ctxB := &UserAuthContext{UserID: "user:admin", Groups: []string{}}

	// User A requests filters first — must NOT poison cache for user B
	resultA, err := infraOnly.GetFilters(context.Background(), ctxA)
	require.NoError(t, err)
	assert.Equal(t, []string{"infrastructure"}, resultA.Categories)

	// User B must get their own full set, not the cached subset from user A
	resultB, err := allAccess.GetFilters(context.Background(), ctxB)
	require.NoError(t, err)
	assert.Equal(t, []string{"applications", "infrastructure"}, resultB.Categories)
}

func TestListRGDs_CategoryScopedAuthorization_UncategorizedAccess(t *testing.T) {
	uncatRGD := testutil.NewCatalogRGD("misc-tool", "default", testutil.WithCategory(""))
	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCategory("infrastructure"))

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      []models.CatalogRGD{uncatRGD, infraRGD},
				TotalCount: 2,
				Page:       1,
				PageSize:   50,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		PolicyEnforcer: &categoryPolicyEnforcer{
			allowedObjects: []string{"rgds/uncategorized/*"},
		},
	})

	authCtx := &UserAuthContext{UserID: "user:misc", Groups: []string{}}
	result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
	require.NoError(t, err)

	assert.Equal(t, 1, result.TotalCount)
	assert.Equal(t, "misc-tool", result.Items[0].Name)
}

func TestListRGDs_CategoryScopedAuthorization_Pagination(t *testing.T) {
	// Create 5 RGDs: 3 infrastructure, 2 applications
	rgds := []models.CatalogRGD{
		testutil.NewCatalogRGD("infra-1", "default", testutil.WithCategory("infrastructure")),
		testutil.NewCatalogRGD("app-1", "default", testutil.WithCategory("applications")),
		testutil.NewCatalogRGD("infra-2", "default", testutil.WithCategory("infrastructure")),
		testutil.NewCatalogRGD("app-2", "default", testutil.WithCategory("applications")),
		testutil.NewCatalogRGD("infra-3", "default", testutil.WithCategory("infrastructure")),
	}

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			return models.CatalogRGDList{
				Items:      rgds,
				TotalCount: len(rgds),
				Page:       1,
				PageSize:   opts.PageSize,
			}
		},
	}

	svc := NewCatalogService(CatalogServiceConfig{
		RGDProvider: provider,
		PolicyEnforcer: &categoryPolicyEnforcer{
			allowedObjects: []string{"rgds/infrastructure/*"},
		},
	})

	authCtx := &UserAuthContext{UserID: "user:infra-dev", Groups: []string{}}

	// Page 1, size 2: should get first 2 infrastructure RGDs
	result, err := svc.ListRGDs(context.Background(), authCtx, RGDFilters{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalCount, "TotalCount should reflect all authorized RGDs")
	assert.Len(t, result.Items, 2, "Page 1 should have 2 items")
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.PageSize)

	// Page 2, size 2: should get remaining 1 infrastructure RGD
	result2, err := svc.ListRGDs(context.Background(), authCtx, RGDFilters{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, result2.TotalCount, "TotalCount consistent across pages")
	assert.Len(t, result2.Items, 1, "Page 2 should have 1 item")

	// Page 3, size 2: should be empty
	result3, err := svc.ListRGDs(context.Background(), authCtx, RGDFilters{Page: 3, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 3, result3.TotalCount)
	assert.Empty(t, result3.Items, "Page 3 should be empty")
}

// --- CatalogTier filtering tests (STORY-416) ---

// mockProjectTypeResolver implements ProjectTypeResolver for testing.
type mockProjectTypeResolver struct {
	types map[string]string
	err   error
}

func (m *mockProjectTypeResolver) GetProjectTypes(_ context.Context, projectNames []string) (map[string]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make(map[string]string)
	for _, name := range projectNames {
		if t, ok := m.types[name]; ok {
			result[name] = t
		} else {
			result[name] = "app" // default
		}
	}
	return result, nil
}

func TestComputeVisibleCatalogTiers_NilResolver(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: nil,
	})
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), &UserAuthContext{UserID: "user"})
	require.NoError(t, err)
	assert.Nil(t, tiers, "nil resolver should return nil (all tiers)")
}

func TestComputeVisibleCatalogTiers_NilAuthCtx(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{},
	})
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, tiers, "nil authCtx should return nil (all tiers)")
}

func TestComputeVisibleCatalogTiers_GlobalAdmin(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{},
	})
	authCtx := &UserAuthContext{
		UserID:               "admin",
		AccessibleNamespaces: []string{"*"},
		AccessibleProjects:   []string{"proj-a"},
	}
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), authCtx)
	require.NoError(t, err)
	assert.Nil(t, tiers, "global admin should see all tiers")
}

func TestComputeVisibleCatalogTiers_NoProjects(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{},
	})
	authCtx := &UserAuthContext{
		UserID:               "user",
		AccessibleProjects:   []string{},
		AccessibleNamespaces: []string{},
	}
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), authCtx)
	require.NoError(t, err)
	assert.Equal(t, []string{"both"}, tiers, "no projects = only universal RGDs")
}

func TestComputeVisibleCatalogTiers_AppOnlyProjects(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{
			types: map[string]string{"app-1": "app", "app-2": "app"},
		},
	})
	authCtx := &UserAuthContext{
		UserID:               "dev",
		AccessibleProjects:   []string{"app-1", "app-2"},
		AccessibleNamespaces: []string{"ns-1"},
	}
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), authCtx)
	require.NoError(t, err)
	assert.Equal(t, []string{"app", "both"}, tiers)
}

func TestComputeVisibleCatalogTiers_PlatformOnlyProjects(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{
			types: map[string]string{"infra-1": "platform"},
		},
	})
	authCtx := &UserAuthContext{
		UserID:               "platform-admin",
		AccessibleProjects:   []string{"infra-1"},
		AccessibleNamespaces: []string{"ns-1"},
	}
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), authCtx)
	require.NoError(t, err)
	assert.Equal(t, []string{"infrastructure", "both"}, tiers)
}

func TestComputeVisibleCatalogTiers_MixedProjects(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{
			types: map[string]string{"app-1": "app", "infra-1": "platform"},
		},
	})
	authCtx := &UserAuthContext{
		UserID:               "multi-user",
		AccessibleProjects:   []string{"app-1", "infra-1"},
		AccessibleNamespaces: []string{"ns-1", "ns-2"},
	}
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), authCtx)
	require.NoError(t, err)
	assert.Nil(t, tiers, "mixed projects = all tiers visible")
}

func TestComputeVisibleCatalogTiers_ResolverError(t *testing.T) {
	t.Parallel()
	svc := NewCatalogService(CatalogServiceConfig{
		ProjectTypeResolver: &mockProjectTypeResolver{
			err: errors.New("resolver error"),
		},
	})
	authCtx := &UserAuthContext{
		UserID:               "user",
		AccessibleProjects:   []string{"proj-a"},
		AccessibleNamespaces: []string{"ns-1"},
	}
	tiers, err := svc.computeVisibleCatalogTiers(context.Background(), authCtx)
	assert.Error(t, err)
	assert.Nil(t, tiers)
}

func TestListRGDs_CatalogTierFiltering(t *testing.T) {
	appRGD := testutil.NewCatalogRGD("web-app", "default", testutil.WithCatalogTier("app"))
	infraRGD := testutil.NewCatalogRGD("aks-cluster", "default", testutil.WithCatalogTier("infrastructure"))
	bothRGD := testutil.NewCatalogRGD("shared-db", "default", testutil.WithCatalogTier("both"))

	provider := &mockRGDProvider{
		listFn: func(opts models.ListOptions) models.CatalogRGDList {
			// Simulate cache matchesFilter behavior
			all := []models.CatalogRGD{appRGD, infraRGD, bothRGD}
			if opts.CatalogTiers == nil {
				return models.CatalogRGDList{Items: all, TotalCount: len(all), Page: 1, PageSize: 50}
			}
			var filtered []models.CatalogRGD
			for _, rgd := range all {
				for _, tier := range opts.CatalogTiers {
					if rgd.CatalogTier == tier {
						filtered = append(filtered, rgd)
						break
					}
				}
			}
			return models.CatalogRGDList{Items: filtered, TotalCount: len(filtered), Page: 1, PageSize: 50}
		},
	}

	t.Run("app project user sees app + both RGDs", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			ProjectTypeResolver: &mockProjectTypeResolver{
				types: map[string]string{"my-app": "app"},
			},
		})
		authCtx := &UserAuthContext{
			UserID:               "dev",
			AccessibleProjects:   []string{"my-app"},
			AccessibleNamespaces: []string{"ns-1"},
		}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
		names := []string{result.Items[0].Name, result.Items[1].Name}
		assert.Contains(t, names, "web-app")
		assert.Contains(t, names, "shared-db")
	})

	t.Run("platform project user sees infrastructure + both RGDs", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			ProjectTypeResolver: &mockProjectTypeResolver{
				types: map[string]string{"infra": "platform"},
			},
		})
		authCtx := &UserAuthContext{
			UserID:               "platform-admin",
			AccessibleProjects:   []string{"infra"},
			AccessibleNamespaces: []string{"ns-1"},
		}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)
		assert.Equal(t, 2, result.TotalCount)
		names := []string{result.Items[0].Name, result.Items[1].Name}
		assert.Contains(t, names, "aks-cluster")
		assert.Contains(t, names, "shared-db")
	})

	t.Run("global admin sees all RGDs", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			ProjectTypeResolver: &mockProjectTypeResolver{
				types: map[string]string{"my-app": "app"},
			},
		})
		authCtx := &UserAuthContext{
			UserID:               "admin",
			AccessibleProjects:   []string{"my-app"},
			AccessibleNamespaces: []string{"*"},
		}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount, "global admin sees all tiers")
	})

	t.Run("resolver error fails open (all tiers)", func(t *testing.T) {
		svc := NewCatalogService(CatalogServiceConfig{
			RGDProvider: provider,
			ProjectTypeResolver: &mockProjectTypeResolver{
				err: errors.New("resolver error"),
			},
		})
		authCtx := &UserAuthContext{
			UserID:               "user",
			AccessibleProjects:   []string{"proj-a"},
			AccessibleNamespaces: []string{"ns-1"},
		}
		result, err := svc.ListRGDs(context.Background(), authCtx, DefaultRGDFilters())
		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalCount, "resolver error should show all tiers")
	})
}

func TestCatalogService_CacheKeyIncludesTiers(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	// Cache keys with different tiers should differ
	opts1 := models.ListOptions{CatalogTiers: []string{"app", "both"}}
	opts2 := models.ListOptions{CatalogTiers: []string{"infrastructure", "both"}}
	opts3 := models.ListOptions{CatalogTiers: nil}

	key1 := svc.listCacheKey(opts1)
	key2 := svc.listCacheKey(opts2)
	key3 := svc.listCacheKey(opts3)

	assert.NotEqual(t, key1, key2, "different tiers should produce different keys")
	assert.NotEqual(t, key1, key3, "tiers vs nil should produce different keys")
	assert.Contains(t, key1, "tiers=app,both")
	assert.Contains(t, key3, "tiers=")
}

func TestCatalogService_FiltersCacheKeyIncludesTiers(t *testing.T) {
	svc := NewCatalogService(CatalogServiceConfig{})

	key1 := svc.filtersCacheKey(nil, true, []string{"app", "both"})
	key2 := svc.filtersCacheKey(nil, true, []string{"infrastructure", "both"})
	key3 := svc.filtersCacheKey(nil, true, nil)

	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.Contains(t, key1, "tiers=app,both")
}
