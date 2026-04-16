// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/kro"
	"github.com/knodex/knodex/server/internal/models"
)

// catalogRGD creates a CatalogRGD with the catalog annotation set to "true"
// This is a helper for tests that need visible RGDs in the catalog.
// catalog annotation is the gateway to visibility
func catalogRGD(name, namespace string) *models.CatalogRGD {
	return &models.CatalogRGD{
		Name:      name,
		Title:     name,
		Namespace: namespace,
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	}
}

func TestRGDCache_SetAndGet(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd := &models.CatalogRGD{
		Name:        "test-rgd",
		Namespace:   "default",
		Description: "Test RGD",
		Category:    "database",
		Tags:        []string{"postgres", "database"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Set
	cache.Set(rgd)

	// Get
	got, found := cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected to find RGD in cache")
	}

	if got.Name != rgd.Name {
		t.Errorf("expected name %q, got %q", rgd.Name, got.Name)
	}
	if got.Namespace != rgd.Namespace {
		t.Errorf("expected namespace %q, got %q", rgd.Namespace, got.Namespace)
	}
	if got.Description != rgd.Description {
		t.Errorf("expected description %q, got %q", rgd.Description, got.Description)
	}
}

func TestRGDCache_GetNotFound(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	_, found := cache.Get("default", "nonexistent")
	if found {
		t.Error("expected not to find nonexistent RGD")
	}
}

func TestRGDCache_Delete(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd := &models.CatalogRGD{
		Name:      "test-rgd",
		Namespace: "default",
	}

	cache.Set(rgd)

	// Verify it exists
	if _, found := cache.Get("default", "test-rgd"); !found {
		t.Fatal("expected to find RGD before delete")
	}

	// Delete
	cache.Delete("default", "test-rgd")

	// Verify it's gone
	if _, found := cache.Get("default", "test-rgd"); found {
		t.Error("expected RGD to be deleted")
	}
}

func TestRGDCache_Count(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	if cache.Count() != 0 {
		t.Errorf("expected empty cache, got count %d", cache.Count())
	}

	cache.Set(&models.CatalogRGD{Name: "rgd1", Namespace: "ns1"})
	cache.Set(&models.CatalogRGD{Name: "rgd2", Namespace: "ns1"})
	cache.Set(&models.CatalogRGD{Name: "rgd1", Namespace: "ns2"})

	if cache.Count() != 3 {
		t.Errorf("expected count 3, got %d", cache.Count())
	}
}

func TestRGDCache_Clear(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	cache.Set(&models.CatalogRGD{Name: "rgd1", Namespace: "ns1"})
	cache.Set(&models.CatalogRGD{Name: "rgd2", Namespace: "ns1"})

	cache.Clear()

	if cache.Count() != 0 {
		t.Errorf("expected empty cache after clear, got count %d", cache.Count())
	}
}

func TestRGDCache_All(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	cache.Set(&models.CatalogRGD{Name: "rgd1", Namespace: "ns1"})
	cache.Set(&models.CatalogRGD{Name: "rgd2", Namespace: "ns2"})

	all := cache.All()

	if len(all) != 2 {
		t.Errorf("expected 2 RGDs, got %d", len(all))
	}
}

func TestRGDCache_ListFilterByNamespace(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	cache.Set(catalogRGD("rgd1", "ns1"))
	cache.Set(catalogRGD("rgd2", "ns1"))
	cache.Set(catalogRGD("rgd3", "ns2"))

	opts := models.DefaultListOptions()
	opts.Namespace = "ns1"

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs in ns1, got %d", result.TotalCount)
	}

	for _, rgd := range result.Items {
		if rgd.Namespace != "ns1" {
			t.Errorf("expected namespace ns1, got %s", rgd.Namespace)
		}
	}
}

func TestRGDCache_ListFilterByCategory(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("rgd1", "default")
	rgd1.Category = "database"
	rgd2 := catalogRGD("rgd2", "default")
	rgd2.Category = "cache"
	rgd3 := catalogRGD("rgd3", "default")
	rgd3.Category = "database"

	cache.Set(rgd1)
	cache.Set(rgd2)
	cache.Set(rgd3)

	opts := models.DefaultListOptions()
	opts.Category = "database"

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 database RGDs, got %d", result.TotalCount)
	}
}

func TestRGDCache_ListFilterByTags(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("rgd1", "default")
	rgd1.Tags = []string{"aws", "database"}
	rgd2 := catalogRGD("rgd2", "default")
	rgd2.Tags = []string{"gcp", "cache"}
	rgd3 := catalogRGD("rgd3", "default")
	rgd3.Tags = []string{"aws", "cache"}

	cache.Set(rgd1)
	cache.Set(rgd2)
	cache.Set(rgd3)

	opts := models.DefaultListOptions()
	opts.Tags = []string{"aws"}

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 AWS RGDs, got %d", result.TotalCount)
	}

	// Test with multiple tags (AND logic)
	opts.Tags = []string{"aws", "database"}
	result = cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD with both aws and database tags, got %d", result.TotalCount)
	}
}

func TestRGDCache_ListFilterBySearch(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("postgres-cluster", "default")
	rgd1.Description = "PostgreSQL cluster"
	rgd2 := catalogRGD("redis-cache", "default")
	rgd2.Description = "Redis caching"
	rgd3 := catalogRGD("mysql-db", "default")
	rgd3.Description = "MySQL database"

	cache.Set(rgd1)
	cache.Set(rgd2)
	cache.Set(rgd3)

	opts := models.DefaultListOptions()
	opts.Search = "postgres"

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD matching 'postgres', got %d", result.TotalCount)
	}

	// Search in description
	opts.Search = "caching"
	result = cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD with 'caching' in description, got %d", result.TotalCount)
	}
}

func TestRGDCache_ListFilterBySearchTitle(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("prometheus-stack", "default")
	rgd1.Title = "Prometheus Monitoring Stack"
	rgd1.Description = "Cluster monitoring"
	rgd2 := catalogRGD("redis-cache", "default")
	rgd2.Title = "Redis Cache System"
	rgd2.Description = "Caching solution"
	rgd3 := catalogRGD("simple-app", "default")
	rgd3.Title = "simple-app" // Title same as name
	rgd3.Description = "A simple app"

	cache.Set(rgd1)
	cache.Set(rgd2)
	cache.Set(rgd3)

	// Search by title (not in name or description)
	opts := models.DefaultListOptions()
	opts.Search = "Monitoring"

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD matching 'Monitoring' in title, got %d", result.TotalCount)
	}
	if result.TotalCount > 0 && result.Items[0].Name != "prometheus-stack" {
		t.Errorf("expected prometheus-stack, got %s", result.Items[0].Name)
	}

	// Search case-insensitive on title
	opts.Search = "cache system"
	result = cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD matching 'cache system' in title, got %d", result.TotalCount)
	}
}

func TestRGDCache_ListPagination(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	for i := 0; i < 25; i++ {
		cache.Set(catalogRGD("rgd-"+string(rune('a'+i)), "default"))
	}

	// First page
	opts := models.DefaultListOptions()
	opts.Page = 1
	opts.PageSize = 10

	result := cache.List(opts)

	if result.TotalCount != 25 {
		t.Errorf("expected total count 25, got %d", result.TotalCount)
	}
	if len(result.Items) != 10 {
		t.Errorf("expected 10 items on page 1, got %d", len(result.Items))
	}
	if result.Page != 1 {
		t.Errorf("expected page 1, got %d", result.Page)
	}
	if result.PageSize != 10 {
		t.Errorf("expected page size 10, got %d", result.PageSize)
	}

	// Second page
	opts.Page = 2
	result = cache.List(opts)

	if len(result.Items) != 10 {
		t.Errorf("expected 10 items on page 2, got %d", len(result.Items))
	}

	// Third page (partial)
	opts.Page = 3
	result = cache.List(opts)

	if len(result.Items) != 5 {
		t.Errorf("expected 5 items on page 3, got %d", len(result.Items))
	}

	// Beyond last page
	opts.Page = 10
	result = cache.List(opts)

	if len(result.Items) != 0 {
		t.Errorf("expected 0 items beyond last page, got %d", len(result.Items))
	}
}

func TestRGDCache_ListSortByName(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	cache.Set(catalogRGD("charlie", "default"))
	cache.Set(catalogRGD("alpha", "default"))
	cache.Set(catalogRGD("bravo", "default"))

	opts := models.DefaultListOptions()
	opts.SortBy = "name"
	opts.SortOrder = "asc"

	result := cache.List(opts)

	if result.Items[0].Name != "alpha" {
		t.Errorf("expected first item to be 'alpha', got %q", result.Items[0].Name)
	}
	if result.Items[1].Name != "bravo" {
		t.Errorf("expected second item to be 'bravo', got %q", result.Items[1].Name)
	}
	if result.Items[2].Name != "charlie" {
		t.Errorf("expected third item to be 'charlie', got %q", result.Items[2].Name)
	}

	// Descending
	opts.SortOrder = "desc"
	result = cache.List(opts)

	if result.Items[0].Name != "charlie" {
		t.Errorf("expected first item (desc) to be 'charlie', got %q", result.Items[0].Name)
	}
}

func TestRGDCache_ListSortByCreatedAt(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	now := time.Now()
	rgd1 := catalogRGD("rgd1", "default")
	rgd1.CreatedAt = now.Add(-2 * time.Hour)
	rgd2 := catalogRGD("rgd2", "default")
	rgd2.CreatedAt = now
	rgd3 := catalogRGD("rgd3", "default")
	rgd3.CreatedAt = now.Add(-1 * time.Hour)

	cache.Set(rgd1)
	cache.Set(rgd2)
	cache.Set(rgd3)

	opts := models.DefaultListOptions()
	opts.SortBy = "createdAt"
	opts.SortOrder = "asc"

	result := cache.List(opts)

	if result.Items[0].Name != "rgd1" {
		t.Errorf("expected oldest first, got %q", result.Items[0].Name)
	}
	if result.Items[2].Name != "rgd2" {
		t.Errorf("expected newest last, got %q", result.Items[2].Name)
	}
}

func TestRGDCache_SortStability_IdenticalCreatedAt(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	rgd1 := catalogRGD("delta", "ns-b")
	rgd1.CreatedAt = sameTime
	cache.Set(rgd1)
	rgd2 := catalogRGD("alpha", "ns-a")
	rgd2.CreatedAt = sameTime
	cache.Set(rgd2)
	rgd3 := catalogRGD("charlie", "ns-a")
	rgd3.CreatedAt = sameTime
	cache.Set(rgd3)
	rgd4 := catalogRGD("bravo", "ns-b")
	rgd4.CreatedAt = sameTime
	cache.Set(rgd4)

	var firstOrder []string
	for attempt := 0; attempt < 10; attempt++ {
		opts := models.DefaultListOptions()
		opts.SortBy = "createdAt"
		opts.SortOrder = "asc"
		result := cache.List(opts)
		var names []string
		for _, rgd := range result.Items {
			names = append(names, rgd.Namespace+"/"+rgd.Name)
		}
		if attempt == 0 {
			firstOrder = names
			expected := []string{"ns-a/alpha", "ns-a/charlie", "ns-b/bravo", "ns-b/delta"}
			for i, exp := range expected {
				if names[i] != exp {
					t.Errorf("attempt %d: expected index %d = %q, got %q", attempt, i, exp, names[i])
				}
			}
		} else {
			for i, name := range names {
				if name != firstOrder[i] {
					t.Errorf("attempt %d: order changed at index %d: %q != %q", attempt, i, name, firstOrder[i])
				}
			}
		}
	}
}

func TestRGDCache_SortStability_IdenticalName(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// All RGDs have the same name but different namespaces
	cache.Set(catalogRGD("web-app", "prod"))
	cache.Set(catalogRGD("web-app", "dev"))
	cache.Set(catalogRGD("web-app", "staging"))

	var firstOrder []string
	for attempt := 0; attempt < 10; attempt++ {
		opts := models.DefaultListOptions()
		opts.SortBy = "name"
		opts.SortOrder = "asc"
		result := cache.List(opts)
		var names []string
		for _, rgd := range result.Items {
			names = append(names, rgd.Namespace+"/"+rgd.Name)
		}
		if attempt == 0 {
			firstOrder = names
			// Tie-break by namespace/name ascending
			expected := []string{"dev/web-app", "prod/web-app", "staging/web-app"}
			for i, exp := range expected {
				if names[i] != exp {
					t.Errorf("attempt %d: expected index %d = %q, got %q", attempt, i, exp, names[i])
				}
			}
		} else {
			for i, name := range names {
				if name != firstOrder[i] {
					t.Errorf("attempt %d: order changed at index %d: %q != %q", attempt, i, name, firstOrder[i])
				}
			}
		}
	}
}

func TestRGDCache_SortStability_DescendingTieBreak(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	rgd1 := catalogRGD("alpha", "ns-a")
	rgd1.CreatedAt = sameTime
	cache.Set(rgd1)
	rgd2 := catalogRGD("bravo", "ns-a")
	rgd2.CreatedAt = sameTime
	cache.Set(rgd2)
	rgd3 := catalogRGD("charlie", "ns-b")
	rgd3.CreatedAt = sameTime
	cache.Set(rgd3)

	opts := models.DefaultListOptions()
	opts.SortBy = "createdAt"
	opts.SortOrder = "desc"
	result := cache.List(opts)

	expected := []string{"ns-b/charlie", "ns-a/bravo", "ns-a/alpha"}
	for i, exp := range expected {
		got := result.Items[i].Namespace + "/" + result.Items[i].Name
		if got != exp {
			t.Errorf("desc tie-break: expected index %d = %q, got %q", i, exp, got)
		}
	}
}

func TestRGDCache_Update(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd := &models.CatalogRGD{
		Name:        "test-rgd",
		Namespace:   "default",
		Description: "Original",
	}

	cache.Set(rgd)

	// Update
	updated := &models.CatalogRGD{
		Name:        "test-rgd",
		Namespace:   "default",
		Description: "Updated",
	}
	cache.Set(updated)

	// Should still be 1 item
	if cache.Count() != 1 {
		t.Errorf("expected count 1 after update, got %d", cache.Count())
	}

	// Should have new values
	got, _ := cache.Get("default", "test-rgd")
	if got.Description != "Updated" {
		t.Errorf("expected updated description, got %q", got.Description)
	}
}

func TestRGDCache_CaseInsensitiveTagMatch(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("rgd1", "default")
	rgd1.Tags = []string{"AWS", "Database"}
	cache.Set(rgd1)

	opts := models.DefaultListOptions()
	opts.Tags = []string{"aws"}

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected case-insensitive tag match, got %d results", result.TotalCount)
	}
}

func TestRGDCache_CaseInsensitiveSearch(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("PostgreSQL-Cluster", "default")
	rgd1.Description = "A DATABASE cluster"
	cache.Set(rgd1)

	opts := models.DefaultListOptions()
	opts.Search = "postgresql"

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected case-insensitive name search, got %d results", result.TotalCount)
	}

	opts.Search = "database"
	result = cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected case-insensitive description search, got %d results", result.TotalCount)
	}
}

// Tests for this feature/Note: Visibility-based filtering
// New model: catalog annotation is the gateway, project label restricts access
func TestRGDCache_VisibilityFiltering_PublicRGDs(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Setup: Public RGD (catalog: true, no project label = visible to all)
	cache.Set(&models.CatalogRGD{
		Name:      "public-rgd",
		Namespace: "",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Setup: Project-restricted RGD (catalog: true + project label)
	cache.Set(&models.CatalogRGD{
		Name:      "private-alpha-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Setup: Project-restricted RGD with different project
	cache.Set(&models.CatalogRGD{
		Name:      "private-beta-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "beta-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Setup: RGD without catalog annotation - should never appear
	cache.Set(&models.CatalogRGD{
		Name:      "no-catalog-rgd",
		Namespace: "",
	})

	// Test: User with IncludePublic=true and no projects should see only public RGDs
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{} // User has no projects

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 public RGD, got %d", result.TotalCount)
	}
	if result.Items[0].Name != "public-rgd" {
		t.Errorf("expected public-rgd, got %s", result.Items[0].Name)
	}
}

func TestRGDCache_VisibilityFiltering_ProjectRGDs(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Setup RGDs with different project labels (all have catalog annotation)
	cache.Set(&models.CatalogRGD{
		Name:      "alpha-rgd-1",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "alpha-rgd-2",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "beta-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "beta-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Test: User in alpha-team should see only alpha project RGDs
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{"alpha-team"}

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 alpha RGDs, got %d", result.TotalCount)
	}

	for _, rgd := range result.Items {
		if rgd.Labels[kro.RGDProjectLabel] != "alpha-team" {
			t.Errorf("expected alpha-team project, got %s", rgd.Labels[kro.RGDProjectLabel])
		}
	}
}

func TestRGDCache_VisibilityFiltering_PublicPlusProject(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Setup: Mix of public and project-specific RGDs
	// Public RGD = catalog: true with NO project label
	cache.Set(&models.CatalogRGD{
		Name:      "public-db",
		Namespace: "",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "public-cache",
		Namespace: "",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	// Project-restricted RGD = catalog: true WITH project label
	cache.Set(&models.CatalogRGD{
		Name:      "alpha-private",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "beta-private",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "beta-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Test: User in alpha-team should see 2 public + 1 alpha = 3 RGDs
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{"alpha-team"}

	result := cache.List(opts)

	if result.TotalCount != 3 {
		t.Errorf("expected 3 RGDs (2 public + 1 alpha), got %d", result.TotalCount)
	}

	// Verify correct RGDs
	names := make(map[string]bool)
	for _, rgd := range result.Items {
		names[rgd.Name] = true
	}
	if !names["public-db"] || !names["public-cache"] || !names["alpha-private"] {
		t.Error("expected public-db, public-cache, and alpha-private")
	}
	if names["beta-private"] {
		t.Error("beta-private should not be visible to alpha-team user")
	}
}

func TestRGDCache_VisibilityFiltering_MultipleProjects(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Setup RGDs - all have catalog annotation
	cache.Set(&models.CatalogRGD{
		Name:      "public-rgd",
		Namespace: "",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "alpha-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "beta-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "beta-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "gamma-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "gamma-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Test: User in alpha-team AND beta-team should see public + alpha + beta
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{"alpha-team", "beta-team"}

	result := cache.List(opts)

	if result.TotalCount != 3 {
		t.Errorf("expected 3 RGDs (1 public + 1 alpha + 1 beta), got %d", result.TotalCount)
	}

	// Verify gamma is not visible
	for _, rgd := range result.Items {
		if rgd.Name == "gamma-rgd" {
			t.Error("gamma-rgd should not be visible")
		}
	}
}

func TestRGDCache_VisibilityFiltering_GlobalAdmin(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Setup: Various RGDs - all with catalog annotation (admin sees all catalog RGDs)
	cache.Set(&models.CatalogRGD{
		Name:      "public-rgd",
		Namespace: "",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "alpha-private",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "beta-private",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "beta-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	// RGD without catalog annotation - admin should NOT see it (not in catalog)
	cache.Set(&models.CatalogRGD{
		Name:      "non-catalog-rgd",
		Namespace: "",
	})

	// Test: Global admin (no IncludePublic, no Projects) should see all catalog RGDs
	opts := models.DefaultListOptions()
	// IncludePublic = false (default)
	// Projects = nil (default)

	result := cache.List(opts)

	// Should see 3 (all catalog RGDs), NOT 4 (non-catalog-rgd is excluded)
	if result.TotalCount != 3 {
		t.Errorf("expected 3 catalog RGDs for global admin, got %d", result.TotalCount)
	}
}

func TestRGDCache_VisibilityFiltering_CatalogAnnotation(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Catalog annotation IS the gateway to visibility
	// catalog: true with no project label = PUBLIC (visible to all)
	cache.Set(&models.CatalogRGD{
		Name:      "catalog-public-rgd",
		Namespace: "",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Setup: RGD without catalog annotation - NOT in catalog
	cache.Set(&models.CatalogRGD{
		Name:      "private-unlabeled-rgd",
		Namespace: "",
	})

	// Test: User with IncludePublic should see the catalog RGD (it's public now)
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{}

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 public RGD (catalog: true = public), got %d", result.TotalCount)
	}
	if result.TotalCount > 0 && result.Items[0].Name != "catalog-public-rgd" {
		t.Errorf("expected catalog-public-rgd, got %s", result.Items[0].Name)
	}

	// Test: Admin view should see only catalog RGDs (non-catalog not visible to anyone)
	adminOpts := models.DefaultListOptions()
	adminOpts.IncludePublic = false
	adminOpts.Projects = nil

	adminResult := cache.List(adminOpts)

	// Admin sees only 1 (only catalog RGDs, non-catalog is invisible to everyone)
	if adminResult.TotalCount != 1 {
		t.Errorf("expected admin to see 1 catalog RGD, got %d", adminResult.TotalCount)
	}
}

func TestRGDCache_VisibilityFiltering_DefaultPrivate(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// RGD without catalog annotation = NOT in catalog (invisible to everyone)
	cache.Set(&models.CatalogRGD{
		Name:      "no-catalog-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		// No catalog annotation - not in catalog
	})

	// RGD with catalog annotation + project label = restricted to project members
	cache.Set(&models.CatalogRGD{
		Name:      "restricted-rgd",
		Namespace: "",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Test: User NOT in alpha-team should not see any (non-catalog excluded, restricted not accessible)
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{"beta-team"}

	result := cache.List(opts)

	if result.TotalCount != 0 {
		t.Errorf("expected 0 RGDs (no access to alpha-team restricted), got %d", result.TotalCount)
	}

	// Test: User IN alpha-team should see the restricted RGD (but not the non-catalog one)
	opts.Projects = []string{"alpha-team"}
	result = cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD for alpha-team user, got %d", result.TotalCount)
	}
	if result.TotalCount > 0 && result.Items[0].Name != "restricted-rgd" {
		t.Errorf("expected restricted-rgd, got %s", result.Items[0].Name)
	}
}

// Organization filter tests (Story 1.3)
func TestRGDCache_OrgFilter_MatchingOrgVisible(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// RGD with org=orgA
	rgd := catalogRGD("rgd-orgA", "")
	rgd.Organization = "orgA"
	cache.Set(rgd)

	opts := models.DefaultListOptions()
	opts.Organization = "orgA"

	result := cache.List(opts)
	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD (matching org), got %d", result.TotalCount)
	}
}

func TestRGDCache_OrgFilter_MismatchingOrgHidden(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// RGD with org=orgB
	rgd := catalogRGD("rgd-orgB", "")
	rgd.Organization = "orgB"
	cache.Set(rgd)

	opts := models.DefaultListOptions()
	opts.Organization = "orgA"

	result := cache.List(opts)
	if result.TotalCount != 0 {
		t.Errorf("expected 0 RGDs (org mismatch), got %d", result.TotalCount)
	}
}

func TestRGDCache_OrgFilter_SharedRGDAlwaysVisible(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Shared RGD (no org annotation)
	shared := catalogRGD("shared-rgd", "")
	cache.Set(shared)

	opts := models.DefaultListOptions()
	opts.Organization = "orgA"

	result := cache.List(opts)
	if result.TotalCount != 1 {
		t.Errorf("expected 1 RGD (shared, no org annotation), got %d", result.TotalCount)
	}
}

func TestRGDCache_OrgFilter_NoFilterShowsAll(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Mix of org-scoped and shared RGDs
	rgdA := catalogRGD("rgd-orgA", "")
	rgdA.Organization = "orgA"
	cache.Set(rgdA)

	rgdB := catalogRGD("rgd-orgB", "")
	rgdB.Organization = "orgB"
	cache.Set(rgdB)

	shared := catalogRGD("shared-rgd", "")
	cache.Set(shared)

	// No org filter = show all (OSS behavior)
	opts := models.DefaultListOptions()
	opts.Organization = ""

	result := cache.List(opts)
	if result.TotalCount != 3 {
		t.Errorf("expected 3 RGDs (no org filter), got %d", result.TotalCount)
	}
}

func TestRGDCache_OrgFilter_MixedVisibility(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// org=orgA RGD (visible)
	rgdA := catalogRGD("rgd-orgA", "")
	rgdA.Organization = "orgA"
	cache.Set(rgdA)

	// org=orgB RGD (hidden)
	rgdB := catalogRGD("rgd-orgB", "")
	rgdB.Organization = "orgB"
	cache.Set(rgdB)

	// Shared RGD (visible)
	shared := catalogRGD("shared-rgd", "")
	cache.Set(shared)

	opts := models.DefaultListOptions()
	opts.Organization = "orgA"

	result := cache.List(opts)
	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs (orgA + shared), got %d", result.TotalCount)
	}

	names := make(map[string]bool)
	for _, rgd := range result.Items {
		names[rgd.Name] = true
	}
	if !names["rgd-orgA"] || !names["shared-rgd"] {
		t.Error("expected rgd-orgA and shared-rgd to be visible")
	}
	if names["rgd-orgB"] {
		t.Error("rgd-orgB should be hidden (org mismatch)")
	}
}

func TestRGDCache_OrgFilter_BeforeProjectFilter(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// RGD with org=orgB and project=alpha (should be filtered by org before project check)
	rgd := catalogRGD("rgd-orgB-alpha", "")
	rgd.Organization = "orgB"
	rgd.Labels = map[string]string{kro.RGDProjectLabel: "alpha-team"}
	cache.Set(rgd)

	opts := models.DefaultListOptions()
	opts.Organization = "orgA"
	opts.IncludePublic = true
	opts.Projects = []string{"alpha-team"}

	result := cache.List(opts)
	if result.TotalCount != 0 {
		t.Errorf("expected 0 RGDs (org mismatch should filter before project), got %d", result.TotalCount)
	}
}

func TestRGDCache_OrgFilter_DefaultOrgValue(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// RGD with org=default
	rgd := catalogRGD("rgd-default-org", "")
	rgd.Organization = "default"
	cache.Set(rgd)

	// Shared RGD
	shared := catalogRGD("shared-rgd", "")
	cache.Set(shared)

	// RGD with org=orgB (should be hidden)
	rgdB := catalogRGD("rgd-orgB", "")
	rgdB.Organization = "orgB"
	cache.Set(rgdB)

	opts := models.DefaultListOptions()
	opts.Organization = "default"

	result := cache.List(opts)
	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs (default org + shared), got %d", result.TotalCount)
	}
}

func TestRGDCache_VisibilityFiltering_CombinedWithOtherFilters(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Setup RGDs with catalog annotation (the gateway to visibility)
	// Public RGD = catalog: true, no project label
	cache.Set(&models.CatalogRGD{
		Name:      "public-db",
		Namespace: "",
		Category:  "database",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	cache.Set(&models.CatalogRGD{
		Name:      "public-cache",
		Namespace: "",
		Category:  "cache",
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})
	// Project-restricted RGD = catalog: true + project label
	cache.Set(&models.CatalogRGD{
		Name:      "alpha-db",
		Namespace: "",
		Category:  "database",
		Labels: map[string]string{
			kro.RGDProjectLabel: "alpha-team",
		},
		Annotations: map[string]string{
			kro.CatalogAnnotation: "true",
		},
	})

	// Test: Filter by visibility + category
	opts := models.DefaultListOptions()
	opts.IncludePublic = true
	opts.Projects = []string{"alpha-team"}
	opts.Category = "database"

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 database RGDs (1 public + 1 alpha), got %d", result.TotalCount)
	}

	for _, rgd := range result.Items {
		if rgd.Category != "database" {
			t.Errorf("expected database category, got %s", rgd.Category)
		}
	}
}

func TestRGDCache_ExtendsKindIndex_SetAndQuery(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Create parent RGD
	parent := catalogRGD("simple-aks", "")
	parent.Kind = "SimpleAKSCluster"
	cache.Set(parent)

	// Create child RGDs that extend SimpleAKSCluster
	child1 := catalogRGD("monitoring-addon", "")
	child1.Kind = "MonitoringAddon"
	child1.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(child1)

	child2 := catalogRGD("logging-addon", "")
	child2.Kind = "LoggingAddon"
	child2.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(child2)

	// Child that extends a different kind
	child3 := catalogRGD("eks-addon", "")
	child3.Kind = "EKSAddon"
	child3.ExtendsKinds = []string{"SimpleEKSCluster"}
	cache.Set(child3)

	// Query RGDs extending SimpleAKSCluster
	results := cache.GetByExtendsKind("SimpleAKSCluster")
	if len(results) != 2 {
		t.Fatalf("expected 2 RGDs extending SimpleAKSCluster, got %d", len(results))
	}

	// Query RGDs extending SimpleEKSCluster
	results = cache.GetByExtendsKind("SimpleEKSCluster")
	if len(results) != 1 {
		t.Fatalf("expected 1 RGD extending SimpleEKSCluster, got %d", len(results))
	}
	if results[0].Name != "eks-addon" {
		t.Errorf("expected eks-addon, got %s", results[0].Name)
	}

	// Query non-existent kind
	results = cache.GetByExtendsKind("NonExistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-existent kind, got %d", len(results))
	}
}

func TestRGDCache_ExtendsKindIndex_Update(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Create child RGD extending kind A
	child := catalogRGD("addon", "")
	child.ExtendsKinds = []string{"KindA"}
	cache.Set(child)

	// Verify it's indexed under KindA
	results := cache.GetByExtendsKind("KindA")
	if len(results) != 1 {
		t.Fatalf("expected 1 RGD extending KindA, got %d", len(results))
	}

	// Update to extend KindB instead
	child2 := catalogRGD("addon", "")
	child2.ExtendsKinds = []string{"KindB"}
	cache.Set(child2)

	// KindA should have no results
	results = cache.GetByExtendsKind("KindA")
	if len(results) != 0 {
		t.Errorf("expected 0 RGDs extending KindA after update, got %d", len(results))
	}

	// KindB should have the result
	results = cache.GetByExtendsKind("KindB")
	if len(results) != 1 {
		t.Fatalf("expected 1 RGD extending KindB after update, got %d", len(results))
	}
}

func TestRGDCache_ExtendsKindIndex_Delete(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	child := catalogRGD("addon", "default")
	child.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(child)

	// Verify indexed
	results := cache.GetByExtendsKind("SimpleAKSCluster")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Delete
	cache.Delete("default", "addon")

	// Should be empty
	results = cache.GetByExtendsKind("SimpleAKSCluster")
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestRGDCache_ExtendsKindIndex_RespectRBAC(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Parent kind - public RGD (no project restriction)
	parent := catalogRGD("simple-aks", "")
	parent.Kind = "SimpleAKSCluster"
	cache.Set(parent)

	// Public add-on extending SimpleAKSCluster (visible to all authenticated users)
	publicAddon := catalogRGD("public-addon", "")
	publicAddon.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(publicAddon)

	// Project-restricted add-on extending SimpleAKSCluster (only for project-a)
	privateAddon := catalogRGD("private-addon", "")
	privateAddon.ExtendsKinds = []string{"SimpleAKSCluster"}
	privateAddon.Labels = map[string]string{kro.RGDProjectLabel: "project-a"}
	cache.Set(privateAddon)

	// User in project-b: should see only public add-on (not project-a's)
	opts := models.DefaultListOptions()
	opts.ExtendsKind = "SimpleAKSCluster"
	opts.IncludePublic = true
	opts.Projects = []string{"project-b"}

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("project-b user: expected 1 add-on (public only), got %d", result.TotalCount)
	}
	if result.TotalCount > 0 && result.Items[0].Name != "public-addon" {
		t.Errorf("project-b user: expected public-addon, got %s", result.Items[0].Name)
	}

	// User in project-a: should see both public and project-a's add-on
	opts.Projects = []string{"project-a"}
	result = cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("project-a user: expected 2 add-ons (public + project-a), got %d", result.TotalCount)
	}

	names := make(map[string]bool)
	for _, rgd := range result.Items {
		names[rgd.Name] = true
	}
	if !names["public-addon"] {
		t.Error("project-a user: expected public-addon to be visible")
	}
	if !names["private-addon"] {
		t.Error("project-a user: expected private-addon to be visible")
	}

	// Verify non-extending RGD (parent itself) is NOT returned
	for _, rgd := range result.Items {
		if rgd.Name == "simple-aks" {
			t.Error("parent RGD simple-aks should not appear in extendsKind results")
		}
	}
}

func TestRGDCache_ExtendsKindIndex_RespectOrgFilter(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Public add-on from orgA
	orgAAddon := catalogRGD("orga-addon", "")
	orgAAddon.ExtendsKinds = []string{"SimpleAKSCluster"}
	orgAAddon.Organization = "orgA"
	cache.Set(orgAAddon)

	// Public add-on from orgB (should be hidden for orgA user)
	orgBAddon := catalogRGD("orgb-addon", "")
	orgBAddon.ExtendsKinds = []string{"SimpleAKSCluster"}
	orgBAddon.Organization = "orgB"
	cache.Set(orgBAddon)

	// Shared add-on (no org restriction)
	sharedAddon := catalogRGD("shared-addon", "")
	sharedAddon.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(sharedAddon)

	// OrgA user: should see orgA addon + shared addon, not orgB addon
	opts := models.DefaultListOptions()
	opts.ExtendsKind = "SimpleAKSCluster"
	opts.Organization = "orgA"

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("orgA user: expected 2 add-ons (orgA + shared), got %d", result.TotalCount)
	}

	for _, rgd := range result.Items {
		if rgd.Name == "orgb-addon" {
			t.Error("orgB add-on should not be visible to orgA user")
		}
	}
}

func TestRGDCache_ExtendsKindIndex_ListWithFilter(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Create several RGDs
	parent := catalogRGD("simple-aks", "")
	parent.Kind = "SimpleAKSCluster"
	cache.Set(parent)

	child1 := catalogRGD("monitoring", "")
	child1.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(child1)

	child2 := catalogRGD("logging", "")
	child2.ExtendsKinds = []string{"SimpleAKSCluster"}
	cache.Set(child2)

	unrelated := catalogRGD("standalone", "")
	cache.Set(unrelated)

	// List with extendsKind filter
	opts := models.DefaultListOptions()
	opts.ExtendsKind = "SimpleAKSCluster"
	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs extending SimpleAKSCluster, got %d", result.TotalCount)
	}

	// Verify only child RGDs are returned (not parent or unrelated)
	for _, rgd := range result.Items {
		if rgd.Name != "monitoring" && rgd.Name != "logging" {
			t.Errorf("unexpected RGD in results: %s", rgd.Name)
		}
	}
}

func TestRGDCache_GetByDependsOnKind(t *testing.T) {
	t.Parallel()
	cache := NewRGDCache()

	rgd1 := catalogRGD("app-with-cluster", "default")
	rgd1.DependsOnKinds = []string{"AKSCluster", "KeyVault"}
	cache.Set(rgd1)

	rgd2 := catalogRGD("standalone-app", "default")
	cache.Set(rgd2)

	rgd3 := catalogRGD("app-with-vault", "default")
	rgd3.DependsOnKinds = []string{"KeyVault"}
	cache.Set(rgd3)

	// Find by AKSCluster - should return only rgd1
	results := cache.GetByDependsOnKind("AKSCluster")
	if len(results) != 1 {
		t.Fatalf("expected 1 RGD depending on AKSCluster, got %d", len(results))
	}
	if results[0].Name != "app-with-cluster" {
		t.Errorf("expected app-with-cluster, got %s", results[0].Name)
	}

	// Find by KeyVault - should return rgd1 and rgd3
	results = cache.GetByDependsOnKind("KeyVault")
	if len(results) != 2 {
		t.Fatalf("expected 2 RGDs depending on KeyVault, got %d", len(results))
	}

	// Find by nonexistent kind
	results = cache.GetByDependsOnKind("NonExistent")
	if len(results) != 0 {
		t.Errorf("expected 0 RGDs, got %d", len(results))
	}
}

func TestRGDCache_ListFilterByDependsOnKind(t *testing.T) {
	t.Parallel()
	cache := NewRGDCache()

	rgd1 := catalogRGD("app-a", "default")
	rgd1.DependsOnKinds = []string{"AKSCluster"}
	cache.Set(rgd1)

	rgd2 := catalogRGD("app-b", "default")
	cache.Set(rgd2)

	opts := models.ListOptions{
		DependsOnKind: "AKSCluster",
		Page:          1,
		PageSize:      20,
		SortBy:        "name",
		SortOrder:     "asc",
	}

	result := cache.List(opts)
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 RGD, got %d", result.TotalCount)
	}
	if result.Items[0].Name != "app-a" {
		t.Errorf("expected app-a, got %s", result.Items[0].Name)
	}
}

func TestRGDCache_DependsOnKindIndex_SetAndQuery(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	dep1 := catalogRGD("logging-addon", "default")
	dep1.DependsOnKinds = []string{"SimpleAKSCluster"}
	cache.Set(dep1)

	dep2 := catalogRGD("monitoring-addon", "default")
	dep2.DependsOnKinds = []string{"SimpleAKSCluster"}
	cache.Set(dep2)

	dep3 := catalogRGD("eks-addon", "default")
	dep3.DependsOnKinds = []string{"SimpleEKSCluster"}
	cache.Set(dep3)

	// Query for AKSCluster dependents — should return 2
	results := cache.GetByDependsOnKind("SimpleAKSCluster")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for SimpleAKSCluster, got %d", len(results))
	}
	// Verify correct RGDs returned (order is non-deterministic from map iteration)
	names := map[string]bool{}
	for _, r := range results {
		names[r.Name] = true
	}
	if !names["logging-addon"] || !names["monitoring-addon"] {
		t.Errorf("expected logging-addon and monitoring-addon, got %v", names)
	}

	// Query for EKSCluster dependents — should return 1
	results = cache.GetByDependsOnKind("SimpleEKSCluster")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for SimpleEKSCluster, got %d", len(results))
	}
	if results[0].Name != "eks-addon" {
		t.Errorf("expected eks-addon, got %s", results[0].Name)
	}

	// Query for non-existent kind — should return nil
	results = cache.GetByDependsOnKind("NonExistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results for NonExistent, got %d", len(results))
	}
}

func TestRGDCache_DependsOnKindIndex_Update(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	dep := catalogRGD("app", "default")
	dep.DependsOnKinds = []string{"KindA"}
	cache.Set(dep)

	results := cache.GetByDependsOnKind("KindA")
	if len(results) != 1 {
		t.Fatalf("expected 1 result before update, got %d", len(results))
	}

	// Update: change DependsOnKinds from KindA to KindB
	dep2 := catalogRGD("app", "default")
	dep2.DependsOnKinds = []string{"KindB"}
	cache.Set(dep2)

	results = cache.GetByDependsOnKind("KindA")
	if len(results) != 0 {
		t.Errorf("expected 0 results for KindA after update, got %d", len(results))
	}

	results = cache.GetByDependsOnKind("KindB")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for KindB after update, got %d", len(results))
	}
}

func TestRGDCache_DependsOnKindIndex_Delete(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	dep := catalogRGD("app", "default")
	dep.DependsOnKinds = []string{"SimpleAKSCluster"}
	cache.Set(dep)

	results := cache.GetByDependsOnKind("SimpleAKSCluster")
	if len(results) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(results))
	}

	cache.Delete("default", "app")

	results = cache.GetByDependsOnKind("SimpleAKSCluster")
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestRGDCache_DependsOnKindIndex_Clear(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	dep1 := catalogRGD("app-a", "default")
	dep1.DependsOnKinds = []string{"AKSCluster"}
	cache.Set(dep1)

	dep2 := catalogRGD("app-b", "default")
	dep2.DependsOnKinds = []string{"KeyVault"}
	cache.Set(dep2)

	cache.Clear()

	results := cache.GetByDependsOnKind("AKSCluster")
	if len(results) != 0 {
		t.Errorf("expected 0 results for AKSCluster after Clear, got %d", len(results))
	}

	results = cache.GetByDependsOnKind("KeyVault")
	if len(results) != 0 {
		t.Errorf("expected 0 results for KeyVault after Clear, got %d", len(results))
	}
}

func TestRGDCache_DependsOnKindIndex_MultipleDependencies(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// RGD that depends on two kinds
	dep := catalogRGD("complex-app", "default")
	dep.DependsOnKinds = []string{"AKSCluster", "KeyVault"}
	cache.Set(dep)

	// Should appear in both indexes
	results := cache.GetByDependsOnKind("AKSCluster")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for AKSCluster, got %d", len(results))
	}

	results = cache.GetByDependsOnKind("KeyVault")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for KeyVault, got %d", len(results))
	}

	// Delete cleans up both index entries
	cache.Delete("default", "complex-app")

	results = cache.GetByDependsOnKind("AKSCluster")
	if len(results) != 0 {
		t.Errorf("expected 0 results for AKSCluster after delete, got %d", len(results))
	}

	results = cache.GetByDependsOnKind("KeyVault")
	if len(results) != 0 {
		t.Errorf("expected 0 results for KeyVault after delete, got %d", len(results))
	}
}

func TestRGDCache_DependsOnKindIndex_CleanupEmptyMaps(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	dep := catalogRGD("app", "default")
	dep.DependsOnKinds = []string{"AKSCluster"}
	cache.Set(dep)

	// Verify index entry exists
	cache.mu.RLock()
	if _, ok := cache.dependsOnKindIndex["AKSCluster"]; !ok {
		t.Fatal("expected AKSCluster key in dependsOnKindIndex after Set")
	}
	cache.mu.RUnlock()

	// Delete the RGD — should clean up the empty inner map
	cache.Delete("default", "app")

	cache.mu.RLock()
	if _, ok := cache.dependsOnKindIndex["AKSCluster"]; ok {
		t.Error("expected AKSCluster key to be removed from dependsOnKindIndex after Delete (empty map cleanup)")
	}
	cache.mu.RUnlock()

	// Also verify cleanup on update (remove old kind, add new)
	dep2 := catalogRGD("app2", "default")
	dep2.DependsOnKinds = []string{"KindA"}
	cache.Set(dep2)

	dep2Updated := catalogRGD("app2", "default")
	dep2Updated.DependsOnKinds = []string{"KindB"}
	cache.Set(dep2Updated)

	cache.mu.RLock()
	if _, ok := cache.dependsOnKindIndex["KindA"]; ok {
		t.Error("expected KindA key to be removed from dependsOnKindIndex after update (empty map cleanup)")
	}
	if _, ok := cache.dependsOnKindIndex["KindB"]; !ok {
		t.Error("expected KindB key to exist in dependsOnKindIndex after update")
	}
	cache.mu.RUnlock()
}

func TestRGDCache_DependsOnKindIndex_RespectRBAC(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// Public addon that depends on SimpleAKSCluster (visible to all authenticated users)
	publicAddon := catalogRGD("public-logging", "")
	publicAddon.DependsOnKinds = []string{"SimpleAKSCluster"}
	cache.Set(publicAddon)

	// Project-restricted addon that depends on SimpleAKSCluster (only for project-a)
	privateAddon := catalogRGD("private-logging", "")
	privateAddon.DependsOnKinds = []string{"SimpleAKSCluster"}
	privateAddon.Labels = map[string]string{kro.RGDProjectLabel: "project-a"}
	cache.Set(privateAddon)

	// User in project-b: should see only public addon (not project-a's)
	opts := models.DefaultListOptions()
	opts.DependsOnKind = "SimpleAKSCluster"
	opts.IncludePublic = true
	opts.Projects = []string{"project-b"}

	result := cache.List(opts)

	if result.TotalCount != 1 {
		t.Errorf("project-b user: expected 1 addon (public only), got %d", result.TotalCount)
	}
	if result.TotalCount > 0 && result.Items[0].Name != "public-logging" {
		t.Errorf("project-b user: expected public-logging, got %s", result.Items[0].Name)
	}

	// User in project-a: should see both public and project-a's addon
	opts.Projects = []string{"project-a"}
	result = cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("project-a user: expected 2 addons (public + project-a), got %d", result.TotalCount)
	}

	names := make(map[string]bool)
	for _, rgd := range result.Items {
		names[rgd.Name] = true
	}
	if !names["public-logging"] {
		t.Error("project-a user: expected public-logging to be visible")
	}
	if !names["private-logging"] {
		t.Error("project-a user: expected private-logging to be visible")
	}
}

func TestRGDCache_DependsOnKindIndex_RespectOrgFilter(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	// OrgA addon depending on AKSCluster
	orgAAddon := catalogRGD("orga-logging", "")
	orgAAddon.DependsOnKinds = []string{"AKSCluster"}
	orgAAddon.Organization = "orgA"
	cache.Set(orgAAddon)

	// OrgB addon depending on AKSCluster (should be hidden for orgA user)
	orgBAddon := catalogRGD("orgb-logging", "")
	orgBAddon.DependsOnKinds = []string{"AKSCluster"}
	orgBAddon.Organization = "orgB"
	cache.Set(orgBAddon)

	// Shared addon depending on AKSCluster (no org restriction)
	sharedAddon := catalogRGD("shared-logging", "")
	sharedAddon.DependsOnKinds = []string{"AKSCluster"}
	cache.Set(sharedAddon)

	// OrgA user: should see orgA addon + shared addon, not orgB addon
	opts := models.DefaultListOptions()
	opts.DependsOnKind = "AKSCluster"
	opts.Organization = "orgA"

	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("orgA user: expected 2 addons (orgA + shared), got %d", result.TotalCount)
	}

	for _, rgd := range result.Items {
		if rgd.Name == "orgb-logging" {
			t.Error("orgB addon should not be visible to orgA user")
		}
	}
}

func TestRGDCache_DependsOnKindIndex_ListWithFilter(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	addon1 := catalogRGD("logging", "")
	addon1.DependsOnKinds = []string{"SimpleAKSCluster"}
	cache.Set(addon1)

	addon2 := catalogRGD("monitoring", "")
	addon2.DependsOnKinds = []string{"SimpleAKSCluster"}
	cache.Set(addon2)

	unrelated := catalogRGD("standalone", "")
	cache.Set(unrelated)

	// List with dependsOnKind filter — should exclude unrelated RGDs
	opts := models.DefaultListOptions()
	opts.DependsOnKind = "SimpleAKSCluster"
	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs depending on SimpleAKSCluster, got %d", result.TotalCount)
	}

	for _, rgd := range result.Items {
		if rgd.Name != "logging" && rgd.Name != "monitoring" {
			t.Errorf("unexpected RGD in results: %s", rgd.Name)
		}
	}
}

// --- CatalogTier filter tests (STORY-416) ---

func TestRGDCache_CatalogTierFilter_NilTiersShowsAll(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	appRGD := catalogRGD("web-app", "default")
	appRGD.CatalogTier = "app"
	cache.Set(appRGD)

	infraRGD := catalogRGD("aks-cluster", "default")
	infraRGD.CatalogTier = "infrastructure"
	cache.Set(infraRGD)

	bothRGD := catalogRGD("shared-db", "default")
	bothRGD.CatalogTier = "both"
	cache.Set(bothRGD)

	// nil CatalogTiers = no filtering, all visible
	opts := models.DefaultListOptions()
	opts.CatalogTiers = nil
	result := cache.List(opts)

	if result.TotalCount != 3 {
		t.Errorf("expected 3 RGDs with nil tier filter, got %d", result.TotalCount)
	}
}

func TestRGDCache_CatalogTierFilter_AppAndBoth(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	appRGD := catalogRGD("web-app", "default")
	appRGD.CatalogTier = "app"
	cache.Set(appRGD)

	infraRGD := catalogRGD("aks-cluster", "default")
	infraRGD.CatalogTier = "infrastructure"
	cache.Set(infraRGD)

	bothRGD := catalogRGD("shared-db", "default")
	bothRGD.CatalogTier = "both"
	cache.Set(bothRGD)

	// Filter to app + both (exclude infrastructure)
	opts := models.DefaultListOptions()
	opts.CatalogTiers = []string{"app", "both"}
	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs with app+both tier filter, got %d", result.TotalCount)
	}
	for _, rgd := range result.Items {
		if rgd.Name == "aks-cluster" {
			t.Error("infrastructure RGD should be excluded by app+both tier filter")
		}
	}
}

func TestRGDCache_CatalogTierFilter_InfraAndBoth(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	appRGD := catalogRGD("web-app", "default")
	appRGD.CatalogTier = "app"
	cache.Set(appRGD)

	infraRGD := catalogRGD("aks-cluster", "default")
	infraRGD.CatalogTier = "infrastructure"
	cache.Set(infraRGD)

	bothRGD := catalogRGD("shared-db", "default")
	bothRGD.CatalogTier = "both"
	cache.Set(bothRGD)

	// Filter to infrastructure + both (exclude app)
	opts := models.DefaultListOptions()
	opts.CatalogTiers = []string{"infrastructure", "both"}
	result := cache.List(opts)

	if result.TotalCount != 2 {
		t.Errorf("expected 2 RGDs with infrastructure+both tier filter, got %d", result.TotalCount)
	}
	for _, rgd := range result.Items {
		if rgd.Name == "web-app" {
			t.Error("app RGD should be excluded by infrastructure+both tier filter")
		}
	}
}

func TestRGDCache_ProducesKindIndex_SetAndQuery(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd1 := catalogRGD("aks-cilium", "default")
	rgd1.ProducesKinds = []models.GVKRef{
		{Group: "containerservice.azure.com", Version: "v1api20231001", Kind: "ManagedCluster"},
		{Group: "containerservice.azure.com", Version: "v1api20231001", Kind: "ManagedClustersAgentPool"},
	}
	cache.Set(rgd1)

	rgd2 := catalogRGD("eks-cluster", "default")
	rgd2.ProducesKinds = []models.GVKRef{
		{Group: "eks.aws.com", Version: "v1beta1", Kind: "ManagedCluster"},
	}
	cache.Set(rgd2)

	// Query by Kind "ManagedCluster" — both RGDs produce it
	result := cache.List(models.ListOptions{
		ProducesKind: "ManagedCluster",
		Page:         1,
		PageSize:     20,
	})
	if result.TotalCount != 2 {
		t.Fatalf("expected 2 results for ManagedCluster, got %d", result.TotalCount)
	}

	// Query by Kind "ManagedClustersAgentPool" — only aks-cilium
	result = cache.List(models.ListOptions{
		ProducesKind: "ManagedClustersAgentPool",
		Page:         1,
		PageSize:     20,
	})
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 result for ManagedClustersAgentPool, got %d", result.TotalCount)
	}
	if result.Items[0].Name != "aks-cilium" {
		t.Errorf("expected aks-cilium, got %s", result.Items[0].Name)
	}

	// Query for non-existent kind — should return 0
	result = cache.List(models.ListOptions{
		ProducesKind: "NonExistent",
		Page:         1,
		PageSize:     20,
	})
	if result.TotalCount != 0 {
		t.Errorf("expected 0 results for NonExistent, got %d", result.TotalCount)
	}
}

func TestRGDCache_ProducesKindIndex_Update(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd := catalogRGD("app", "default")
	rgd.ProducesKinds = []models.GVKRef{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
	}
	cache.Set(rgd)

	result := cache.List(models.ListOptions{ProducesKind: "Deployment", Page: 1, PageSize: 20})
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 result before update, got %d", result.TotalCount)
	}

	// Update: change ProducesKinds from Deployment to StatefulSet
	rgd2 := catalogRGD("app", "default")
	rgd2.ProducesKinds = []models.GVKRef{
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	}
	cache.Set(rgd2)

	result = cache.List(models.ListOptions{ProducesKind: "Deployment", Page: 1, PageSize: 20})
	if result.TotalCount != 0 {
		t.Errorf("expected 0 results for Deployment after update, got %d", result.TotalCount)
	}

	result = cache.List(models.ListOptions{ProducesKind: "StatefulSet", Page: 1, PageSize: 20})
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 result for StatefulSet after update, got %d", result.TotalCount)
	}
}

func TestRGDCache_ProducesKindIndex_Delete(t *testing.T) {
	t.Parallel()

	cache := NewRGDCache()

	rgd := catalogRGD("app", "default")
	rgd.ProducesKinds = []models.GVKRef{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
	}
	cache.Set(rgd)

	result := cache.List(models.ListOptions{ProducesKind: "Deployment", Page: 1, PageSize: 20})
	if result.TotalCount != 1 {
		t.Fatalf("expected 1 result before delete, got %d", result.TotalCount)
	}

	cache.Delete("default", "app")

	result = cache.List(models.ListOptions{ProducesKind: "Deployment", Page: 1, PageSize: 20})
	if result.TotalCount != 0 {
		t.Errorf("expected 0 results after delete, got %d", result.TotalCount)
	}
}
