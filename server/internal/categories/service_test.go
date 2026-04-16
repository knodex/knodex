// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package categories_test

import (
	"context"
	"testing"

	"github.com/knodex/knodex/server/internal/categories"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRGDProvider implements RGDProvider for testing.
type mockRGDProvider struct {
	items []models.CatalogRGD
}

func (m *mockRGDProvider) ListRGDs(opts models.ListOptions) models.CatalogRGDList {
	// Apply category filter if set (for GetCategory tests)
	if opts.Category != "" {
		var filtered []models.CatalogRGD
		for _, rgd := range m.items {
			if rgd.Category == opts.Category {
				filtered = append(filtered, rgd)
			}
		}
		return models.CatalogRGDList{
			Items:      filtered,
			TotalCount: len(filtered),
		}
	}
	// Apply pagination
	start := 0
	end := len(m.items)
	if opts.PageSize > 0 && opts.PageSize < end {
		end = opts.PageSize
	}
	items := m.items[start:end]
	return models.CatalogRGDList{
		Items:      items,
		TotalCount: len(m.items),
	}
}

func TestService_ListCategories_DerivesFromWatcher(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-1", Category: "infrastructure"},
			{Name: "rgd-2", Category: "infrastructure"},
			{Name: "rgd-3", Category: "networking"},
			{Name: "rgd-4", Category: "storage"},
		},
	}

	svc := categories.NewService(provider, nil)
	result := svc.ListCategories(context.Background())

	require.Len(t, result.Categories, 3)

	// Should be sorted alphabetically
	assert.Equal(t, "infrastructure", result.Categories[0].Slug)
	assert.Equal(t, "networking", result.Categories[1].Slug)
	assert.Equal(t, "storage", result.Categories[2].Slug)
}

func TestService_ListCategories_CountsMatchRGDs(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-1", Category: "infrastructure"},
			{Name: "rgd-2", Category: "infrastructure"},
			{Name: "rgd-3", Category: "infrastructure"},
			{Name: "rgd-4", Category: "networking"},
		},
	}

	svc := categories.NewService(provider, nil)
	result := svc.ListCategories(context.Background())

	require.Len(t, result.Categories, 2)

	catMap := make(map[string]int)
	for _, c := range result.Categories {
		catMap[c.Slug] = c.Count
	}

	assert.Equal(t, 3, catMap["infrastructure"])
	assert.Equal(t, 1, catMap["networking"])
}

func TestService_ListCategories_EmptyRGDs(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{items: []models.CatalogRGD{}}
	svc := categories.NewService(provider, nil)
	result := svc.ListCategories(context.Background())

	assert.Empty(t, result.Categories)
}

func TestService_ListCategories_UncategorizedRGDs(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-no-category", Category: ""},
		},
	}

	svc := categories.NewService(provider, nil)
	result := svc.ListCategories(context.Background())

	require.Len(t, result.Categories, 1)
	assert.Equal(t, "uncategorized", result.Categories[0].Slug)
	assert.Equal(t, 1, result.Categories[0].Count)
}

func TestService_ListCategories_DefaultIcon(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-1", Category: "infrastructure"},
		},
	}

	svc := categories.NewService(provider, nil)
	result := svc.ListCategories(context.Background())

	require.Len(t, result.Categories, 1)
	assert.Equal(t, "layout-grid", result.Categories[0].Icon)
}

func TestService_GetCategory_Found(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-1", Category: "infrastructure"},
			{Name: "rgd-2", Category: "networking"},
		},
	}

	svc := categories.NewService(provider, nil)
	cat := svc.GetCategory(context.Background(), "infrastructure")

	require.NotNil(t, cat)
	assert.Equal(t, "infrastructure", cat.Slug)
	assert.Equal(t, "infrastructure", cat.Name)
}

func TestService_GetCategory_NotFound(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-1", Category: "infrastructure"},
		},
	}

	svc := categories.NewService(provider, nil)
	cat := svc.GetCategory(context.Background(), "nonexistent")

	assert.Nil(t, cat)
}

func TestService_ListCategories_SortedAlphabetically(t *testing.T) {
	t.Parallel()

	provider := &mockRGDProvider{
		items: []models.CatalogRGD{
			{Name: "rgd-1", Category: "storage"},
			{Name: "rgd-2", Category: "applications"},
			{Name: "rgd-3", Category: "infrastructure"},
			{Name: "rgd-4", Category: "networking"},
		},
	}

	svc := categories.NewService(provider, nil)
	result := svc.ListCategories(context.Background())

	require.Len(t, result.Categories, 4)
	assert.Equal(t, "applications", result.Categories[0].Slug)
	assert.Equal(t, "infrastructure", result.Categories[1].Slug)
	assert.Equal(t, "networking", result.Categories[2].Slug)
	assert.Equal(t, "storage", result.Categories[3].Slug)
}
