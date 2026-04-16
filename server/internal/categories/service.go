// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package categories provides OSS category auto-discovery for the sidebar.
// Categories are derived from the knodex.io/category annotation on live RGDs
// in the cluster — no ConfigMap or enterprise license required.
package categories

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// allRGDsPageSize is a large page size used to fetch all RGDs from the in-memory
// watcher cache in a single call for category discovery.
const allRGDsPageSize = 10000

// RGDProvider defines the interface for retrieving RGD data.
// This matches the watcher's public API for dependency injection.
type RGDProvider interface {
	// ListRGDs returns a paginated list of RGDs matching the options.
	ListRGDs(opts models.ListOptions) models.CatalogRGDList
}

// Service provides category auto-discovery for the OSS sidebar.
// It discovers categories by examining knodex.io/category annotations on
// all live RGDs in the cluster's in-memory cache.
type Service struct {
	rgdProvider RGDProvider
	logger      *slog.Logger
}

// NewService creates a new categories Service.
func NewService(rgdProvider RGDProvider, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		rgdProvider: rgdProvider,
		logger:      logger.With("component", "categories-service"),
	}
}

// ListCategories returns all categories auto-discovered from live RGD annotations,
// sorted by name, with a count of RGDs per category.
// Only counts catalog-visible RGDs (IncludePublic filters out project-scoped RGDs
// that lack the knodex.io/catalog label). This ensures sidebar category counts
// match the catalog page results.
func (s *Service) ListCategories(_ context.Context) services.CategoryList {
	// Fetch catalog-visible RGDs from the in-memory watcher cache.
	// IncludePublic: true excludes project-scoped RGDs (those without knodex.io/catalog
	// label) so category counts match what the catalog page displays.
	result := s.rgdProvider.ListRGDs(models.ListOptions{
		Page:          1,
		PageSize:      allRGDsPageSize,
		IncludePublic: true,
		Status:        "Active",
	})

	// Collect unique category names and count RGDs per category
	counts := make(map[string]int)
	for _, rgd := range result.Items {
		cat := rgd.Category
		if cat == "" {
			cat = "uncategorized"
		}
		counts[cat]++
	}

	// Build the category list (icon resolution happens in the handler via icon registry)
	categories := make([]services.Category, 0, len(counts))
	for name, count := range counts {
		categories = append(categories, services.Category{
			Name:  name,
			Slug:  sanitize.GlobCharacters(strings.ToLower(name)),
			Icon:  "layout-grid",
			Count: count,
		})
	}

	// Sort by name for stable ordering
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	return services.CategoryList{Categories: categories}
}

// GetCategory returns a specific category by slug.
// Returns nil if not found.
func (s *Service) GetCategory(ctx context.Context, slug string) *services.Category {
	// Re-discover categories from current watcher state (cache is cheap)
	result := s.ListCategories(ctx)
	for i := range result.Categories {
		if result.Categories[i].Slug == slug {
			return &result.Categories[i]
		}
	}
	return nil
}

// Ensure Service implements services.CategoryService.
var _ services.CategoryService = (*Service)(nil)
