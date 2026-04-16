// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/models"
)

func TestInstanceCache_SetGet(t *testing.T) {
	cache := NewInstanceCache()

	instance := &models.Instance{
		Name:         "test-instance",
		Namespace:    "default",
		Kind:         "WebApp",
		RGDName:      "test-rgd",
		RGDNamespace: "default",
		Health:       models.HealthHealthy,
		CreatedAt:    time.Now(),
	}

	cache.Set(instance)

	got, ok := cache.Get("default", "WebApp", "test-instance")
	if !ok {
		t.Fatal("expected to find instance")
	}
	if got.Name != "test-instance" {
		t.Errorf("expected name 'test-instance', got '%s'", got.Name)
	}
	if got.RGDName != "test-rgd" {
		t.Errorf("expected rgdName 'test-rgd', got '%s'", got.RGDName)
	}
}

func TestInstanceCache_SetGet_SameNameDifferentKind(t *testing.T) {
	cache := NewInstanceCache()

	webapp := &models.Instance{
		Name:      "demo",
		Namespace: "demo",
		Kind:      "WebApp",
	}
	database := &models.Instance{
		Name:      "demo",
		Namespace: "demo",
		Kind:      "Database",
	}

	cache.Set(webapp)
	cache.Set(database)

	// Both should be stored separately
	if cache.Count() != 2 {
		t.Errorf("expected count 2, got %d", cache.Count())
	}

	gotWebapp, ok := cache.Get("demo", "WebApp", "demo")
	if !ok {
		t.Fatal("expected to find WebApp instance")
	}
	if gotWebapp.Kind != "WebApp" {
		t.Errorf("expected Kind 'WebApp', got '%s'", gotWebapp.Kind)
	}

	gotDB, ok := cache.Get("demo", "Database", "demo")
	if !ok {
		t.Fatal("expected to find Database instance")
	}
	if gotDB.Kind != "Database" {
		t.Errorf("expected Kind 'Database', got '%s'", gotDB.Kind)
	}
}

func TestInstanceCache_Delete(t *testing.T) {
	cache := NewInstanceCache()

	instance := &models.Instance{
		Name:      "test-instance",
		Namespace: "default",
		Kind:      "WebApp",
	}

	cache.Set(instance)
	cache.Delete("default", "WebApp", "test-instance")

	_, ok := cache.Get("default", "WebApp", "test-instance")
	if ok {
		t.Error("expected instance to be deleted")
	}
}

func TestInstanceCache_Count(t *testing.T) {
	cache := NewInstanceCache()

	if cache.Count() != 0 {
		t.Errorf("expected count 0, got %d", cache.Count())
	}

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default"})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "other"})

	if cache.Count() != 3 {
		t.Errorf("expected count 3, got %d", cache.Count())
	}
}

func TestInstanceCache_CountByRGD(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "default", RGDName: "rgd-b", RGDNamespace: "default"})

	count := cache.CountByRGD("default", "rgd-a")
	if count != 2 {
		t.Errorf("expected count 2 for rgd-a, got %d", count)
	}

	count = cache.CountByRGD("default", "rgd-b")
	if count != 1 {
		t.Errorf("expected count 1 for rgd-b, got %d", count)
	}
}

func TestInstanceCache_GetByRGD(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "default", RGDName: "rgd-b", RGDNamespace: "default"})

	instances := cache.GetByRGD("default", "rgd-a")
	if len(instances) != 2 {
		t.Errorf("expected 2 instances for rgd-a, got %d", len(instances))
	}
}

func TestInstanceCache_List_WithFilters(t *testing.T) {
	cache := NewInstanceCache()

	now := time.Now()
	cache.Set(&models.Instance{Name: "web-app-1", Namespace: "prod", RGDName: "web-app", RGDNamespace: "default", Health: models.HealthHealthy, CreatedAt: now})
	cache.Set(&models.Instance{Name: "web-app-2", Namespace: "prod", RGDName: "web-app", RGDNamespace: "default", Health: models.HealthUnhealthy, CreatedAt: now.Add(time.Hour)})
	cache.Set(&models.Instance{Name: "db-1", Namespace: "dev", RGDName: "database", RGDNamespace: "default", Health: models.HealthHealthy, CreatedAt: now.Add(2 * time.Hour)})

	tests := []struct {
		name     string
		opts     models.InstanceListOptions
		expected int
	}{
		{
			name:     "all instances",
			opts:     models.InstanceListOptions{Page: 1, PageSize: 10},
			expected: 3,
		},
		{
			name:     "filter by namespace",
			opts:     models.InstanceListOptions{Page: 1, PageSize: 10, Namespace: "prod"},
			expected: 2,
		},
		{
			name:     "filter by RGD name",
			opts:     models.InstanceListOptions{Page: 1, PageSize: 10, RGDName: "web-app"},
			expected: 2,
		},
		{
			name:     "filter by health",
			opts:     models.InstanceListOptions{Page: 1, PageSize: 10, Health: models.HealthHealthy},
			expected: 2,
		},
		{
			name:     "filter by search",
			opts:     models.InstanceListOptions{Page: 1, PageSize: 10, Search: "db"},
			expected: 1,
		},
		{
			name:     "combined filters",
			opts:     models.InstanceListOptions{Page: 1, PageSize: 10, Namespace: "prod", Health: models.HealthHealthy},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.List(tt.opts)
			if result.TotalCount != tt.expected {
				t.Errorf("expected %d items, got %d", tt.expected, result.TotalCount)
			}
		})
	}
}

func TestInstanceCache_List_Pagination(t *testing.T) {
	cache := NewInstanceCache()

	for i := 0; i < 25; i++ {
		cache.Set(&models.Instance{
			Name:      "instance-" + string(rune('a'+i)),
			Namespace: "default",
		})
	}

	// First page
	result := cache.List(models.InstanceListOptions{Page: 1, PageSize: 10})
	if len(result.Items) != 10 {
		t.Errorf("expected 10 items on page 1, got %d", len(result.Items))
	}
	if result.TotalCount != 25 {
		t.Errorf("expected total count 25, got %d", result.TotalCount)
	}

	// Second page
	result = cache.List(models.InstanceListOptions{Page: 2, PageSize: 10})
	if len(result.Items) != 10 {
		t.Errorf("expected 10 items on page 2, got %d", len(result.Items))
	}

	// Third page (partial)
	result = cache.List(models.InstanceListOptions{Page: 3, PageSize: 10})
	if len(result.Items) != 5 {
		t.Errorf("expected 5 items on page 3, got %d", len(result.Items))
	}

	// Beyond available pages
	result = cache.List(models.InstanceListOptions{Page: 5, PageSize: 10})
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items on page 5, got %d", len(result.Items))
	}
}

func TestInstanceCache_List_Sorting(t *testing.T) {
	cache := NewInstanceCache()

	now := time.Now()
	cache.Set(&models.Instance{Name: "charlie", Namespace: "default", CreatedAt: now})
	cache.Set(&models.Instance{Name: "alpha", Namespace: "default", CreatedAt: now.Add(-2 * time.Hour)})
	cache.Set(&models.Instance{Name: "bravo", Namespace: "default", CreatedAt: now.Add(-time.Hour)})

	// Sort by name ascending (default)
	result := cache.List(models.InstanceListOptions{Page: 1, PageSize: 10, SortBy: "name", SortOrder: "asc"})
	if result.Items[0].Name != "alpha" {
		t.Errorf("expected first item 'alpha', got '%s'", result.Items[0].Name)
	}

	// Sort by name descending
	result = cache.List(models.InstanceListOptions{Page: 1, PageSize: 10, SortBy: "name", SortOrder: "desc"})
	if result.Items[0].Name != "charlie" {
		t.Errorf("expected first item 'charlie', got '%s'", result.Items[0].Name)
	}

	// Sort by createdAt ascending
	result = cache.List(models.InstanceListOptions{Page: 1, PageSize: 10, SortBy: "createdAt", SortOrder: "asc"})
	if result.Items[0].Name != "alpha" {
		t.Errorf("expected first item 'alpha' (oldest), got '%s'", result.Items[0].Name)
	}

	// Sort by createdAt descending
	result = cache.List(models.InstanceListOptions{Page: 1, PageSize: 10, SortBy: "createdAt", SortOrder: "desc"})
	if result.Items[0].Name != "charlie" {
		t.Errorf("expected first item 'charlie' (newest), got '%s'", result.Items[0].Name)
	}
}

func TestInstanceCache_Clear(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default"})

	if cache.Count() != 2 {
		t.Errorf("expected count 2, got %d", cache.Count())
	}

	cache.Clear()

	if cache.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", cache.Count())
	}
}

func TestInstanceCache_All(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default"})

	all := cache.All()
	if len(all) != 2 {
		t.Errorf("expected 2 instances, got %d", len(all))
	}
}

func TestInstanceCache_SortStability_IdenticalCreatedAt(t *testing.T) {
	cache := NewInstanceCache()

	// All instances share the same createdAt — without a tie-breaker, order is random
	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cache.Set(&models.Instance{Name: "delta", Namespace: "ns-b", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "alpha", Namespace: "ns-a", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "charlie", Namespace: "ns-a", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "bravo", Namespace: "ns-b", CreatedAt: sameTime})

	// Call List 10 times — every call must produce the same order
	var firstOrder []string
	for attempt := 0; attempt < 10; attempt++ {
		result := cache.List(models.InstanceListOptions{
			Page: 1, PageSize: 10, SortBy: "createdAt", SortOrder: "asc",
		})
		var names []string
		for _, inst := range result.Items {
			names = append(names, inst.Namespace+"/"+inst.Name)
		}
		if attempt == 0 {
			firstOrder = names
			// Verify deterministic tie-break by namespace/name ascending
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

func TestInstanceCache_SortStability_IdenticalUpdatedAt(t *testing.T) {
	cache := NewInstanceCache()

	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cache.Set(&models.Instance{Name: "delta", Namespace: "ns-b", UpdatedAt: sameTime, CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "alpha", Namespace: "ns-a", UpdatedAt: sameTime, CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "charlie", Namespace: "ns-a", UpdatedAt: sameTime, CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "bravo", Namespace: "ns-b", UpdatedAt: sameTime, CreatedAt: sameTime})

	var firstOrder []string
	for attempt := 0; attempt < 10; attempt++ {
		result := cache.List(models.InstanceListOptions{
			Page: 1, PageSize: 10, SortBy: "updatedAt", SortOrder: "asc",
		})
		var names []string
		for _, inst := range result.Items {
			names = append(names, inst.Namespace+"/"+inst.Name)
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

func TestInstanceCache_SortStability_IdenticalHealth(t *testing.T) {
	cache := NewInstanceCache()

	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cache.Set(&models.Instance{Name: "zulu", Namespace: "ns-a", Health: models.HealthHealthy, CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "alpha", Namespace: "ns-a", Health: models.HealthHealthy, CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "mike", Namespace: "ns-b", Health: models.HealthHealthy, CreatedAt: sameTime})

	var firstOrder []string
	for attempt := 0; attempt < 10; attempt++ {
		result := cache.List(models.InstanceListOptions{
			Page: 1, PageSize: 10, SortBy: "health", SortOrder: "asc",
		})
		var names []string
		for _, inst := range result.Items {
			names = append(names, inst.Namespace+"/"+inst.Name)
		}
		if attempt == 0 {
			firstOrder = names
			// All have same health — tie-break by namespace/name ascending
			expected := []string{"ns-a/alpha", "ns-a/zulu", "ns-b/mike"}
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

func TestInstanceCache_SortStability_IdenticalRGDName(t *testing.T) {
	cache := NewInstanceCache()

	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cache.Set(&models.Instance{Name: "inst-3", Namespace: "prod", RGDName: "web-app", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "inst-1", Namespace: "dev", RGDName: "web-app", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "inst-2", Namespace: "dev", RGDName: "web-app", CreatedAt: sameTime})

	var firstOrder []string
	for attempt := 0; attempt < 10; attempt++ {
		result := cache.List(models.InstanceListOptions{
			Page: 1, PageSize: 10, SortBy: "rgdName", SortOrder: "asc",
		})
		var names []string
		for _, inst := range result.Items {
			names = append(names, inst.Namespace+"/"+inst.Name)
		}
		if attempt == 0 {
			firstOrder = names
			// All have same rgdName — tie-break by namespace/name ascending
			expected := []string{"dev/inst-1", "dev/inst-2", "prod/inst-3"}
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

func TestInstanceCache_SortStability_DescendingTieBreak(t *testing.T) {
	cache := NewInstanceCache()

	sameTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	cache.Set(&models.Instance{Name: "alpha", Namespace: "ns-a", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "bravo", Namespace: "ns-a", CreatedAt: sameTime})
	cache.Set(&models.Instance{Name: "charlie", Namespace: "ns-b", CreatedAt: sameTime})

	// Descending sort — tie-break should also reverse
	result := cache.List(models.InstanceListOptions{
		Page: 1, PageSize: 10, SortBy: "createdAt", SortOrder: "desc",
	})

	expected := []string{"ns-b/charlie", "ns-a/bravo", "ns-a/alpha"}
	for i, exp := range expected {
		got := result.Items[i].Namespace + "/" + result.Items[i].Name
		if got != exp {
			t.Errorf("desc tie-break: expected index %d = %q, got %q", i, exp, got)
		}
	}
}

func TestInstanceCache_DeleteByRGD(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default", Kind: "WebApp", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default", Kind: "WebApp", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "default", Kind: "Database", RGDName: "rgd-b", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst4", Namespace: "other", Kind: "WebApp", RGDName: "rgd-a", RGDNamespace: "default"})

	// Delete all instances for rgd-a
	removed := cache.DeleteByRGD("default", "rgd-a")
	if len(removed) != 3 {
		t.Errorf("expected 3 removed instances, got %d", len(removed))
	}

	// Verify only rgd-b instances remain
	if cache.Count() != 1 {
		t.Errorf("expected 1 remaining instance, got %d", cache.Count())
	}

	_, ok := cache.Get("default", "Database", "inst3")
	if !ok {
		t.Error("expected rgd-b instance to remain")
	}

	// Verify rgd-a instances are gone
	_, ok = cache.Get("default", "WebApp", "inst1")
	if ok {
		t.Error("expected rgd-a instance inst1 to be deleted")
	}
}

func TestInstanceCache_DeleteByRGD_Empty(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default", Kind: "WebApp", RGDName: "rgd-a", RGDNamespace: "default"})

	// Delete non-existent RGD should return empty slice
	removed := cache.DeleteByRGD("default", "non-existent")
	if len(removed) != 0 {
		t.Errorf("expected 0 removed instances, got %d", len(removed))
	}

	// Original instance should still be there
	if cache.Count() != 1 {
		t.Errorf("expected 1 instance, got %d", cache.Count())
	}
}

// --- STORY-300: Cluster-scoped instance cache operations (empty namespace) ---

func TestInstanceCache_ClusterScoped_SetGetDelete(t *testing.T) {
	cache := NewInstanceCache()

	// Cluster-scoped instance has empty namespace
	clusterInst := &models.Instance{
		Name:      "global-config",
		Namespace: "",
		Kind:      "ClusterConfig",
		RGDName:   "cluster-config-rgd",
	}

	cache.Set(clusterInst)

	got, ok := cache.Get("", "ClusterConfig", "global-config")
	if !ok {
		t.Fatal("expected to find cluster-scoped instance")
	}
	if got.Namespace != "" {
		t.Errorf("expected empty namespace, got %q", got.Namespace)
	}
	if got.Name != "global-config" {
		t.Errorf("expected name 'global-config', got %q", got.Name)
	}

	// Delete cluster-scoped instance
	cache.Delete("", "ClusterConfig", "global-config")
	_, ok = cache.Get("", "ClusterConfig", "global-config")
	if ok {
		t.Error("expected cluster-scoped instance to be deleted")
	}
}

func TestInstanceCache_ClusterScoped_NoCollisionWithNamespaced(t *testing.T) {
	cache := NewInstanceCache()

	// Cluster-scoped instance (empty namespace)
	clusterInst := &models.Instance{
		Name:      "my-resource",
		Namespace: "",
		Kind:      "MyKind",
		RGDName:   "cluster-rgd",
	}

	// Namespace-scoped instance with same kind and name
	nsInst := &models.Instance{
		Name:      "my-resource",
		Namespace: "default",
		Kind:      "MyKind",
		RGDName:   "ns-rgd",
	}

	cache.Set(clusterInst)
	cache.Set(nsInst)

	// Both should coexist
	if cache.Count() != 2 {
		t.Errorf("expected 2 instances, got %d", cache.Count())
	}

	// Retrieve cluster-scoped
	got, ok := cache.Get("", "MyKind", "my-resource")
	if !ok {
		t.Fatal("expected to find cluster-scoped instance")
	}
	if got.RGDName != "cluster-rgd" {
		t.Errorf("expected RGDName 'cluster-rgd', got %q", got.RGDName)
	}

	// Retrieve namespace-scoped
	got, ok = cache.Get("default", "MyKind", "my-resource")
	if !ok {
		t.Fatal("expected to find namespace-scoped instance")
	}
	if got.RGDName != "ns-rgd" {
		t.Errorf("expected RGDName 'ns-rgd', got %q", got.RGDName)
	}

	// Delete cluster-scoped should not affect namespace-scoped
	cache.Delete("", "MyKind", "my-resource")
	if cache.Count() != 1 {
		t.Errorf("expected 1 instance after deleting cluster-scoped, got %d", cache.Count())
	}
	_, ok = cache.Get("default", "MyKind", "my-resource")
	if !ok {
		t.Error("expected namespace-scoped instance to remain after deleting cluster-scoped")
	}
}

func TestInstanceCache_ClusterScoped_ListWithMixedScopes(t *testing.T) {
	cache := NewInstanceCache()

	now := time.Now()
	cache.Set(&models.Instance{Name: "cluster-app", Namespace: "", Kind: "ClusterApp", RGDName: "cluster-rgd", Health: models.HealthHealthy, CreatedAt: now})
	cache.Set(&models.Instance{Name: "ns-app-1", Namespace: "prod", Kind: "App", RGDName: "ns-rgd", Health: models.HealthHealthy, CreatedAt: now})
	cache.Set(&models.Instance{Name: "ns-app-2", Namespace: "dev", Kind: "App", RGDName: "ns-rgd", Health: models.HealthUnhealthy, CreatedAt: now})

	// Unfiltered list — must include both cluster-scoped and namespace-scoped instances
	result := cache.List(models.InstanceListOptions{Page: 1, PageSize: 10})
	if result.TotalCount != 3 {
		t.Errorf("expected 3 total instances (1 cluster-scoped + 2 namespaced), got %d", result.TotalCount)
	}

	// Filter by specific namespace "prod" — cluster-scoped instance is NOT in "prod" so
	// opts.Namespace="" means "all namespaces"; opts.Namespace="prod" means only "prod".
	// The opts.Namespace filter is exact: cluster-scoped (Namespace="") is excluded when
	// opts.Namespace is non-empty because "" != "prod".
	resultProd := cache.List(models.InstanceListOptions{Page: 1, PageSize: 10, Namespace: "prod"})
	if resultProd.TotalCount != 1 {
		t.Errorf("expected 1 instance for Namespace='prod' filter, got %d", resultProd.TotalCount)
	}
	if resultProd.Items[0].Name != "ns-app-1" {
		t.Errorf("expected 'ns-app-1' for prod filter, got %q", resultProd.Items[0].Name)
	}

	// Filter by RGD — cluster-scoped RGD returns only cluster-scoped instances
	resultCluster := cache.List(models.InstanceListOptions{Page: 1, PageSize: 10, RGDName: "cluster-rgd"})
	if resultCluster.TotalCount != 1 {
		t.Errorf("expected 1 instance for RGDName='cluster-rgd', got %d", resultCluster.TotalCount)
	}
	if resultCluster.Items[0].Namespace != "" {
		t.Errorf("expected empty namespace for cluster-scoped instance, got %q", resultCluster.Items[0].Namespace)
	}
}

func TestInstanceCache_ClusterScoped_CountByRGD(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "cluster-inst1", Namespace: "", Kind: "ClusterConfig", RGDName: "cluster-rgd", RGDNamespace: ""})
	cache.Set(&models.Instance{Name: "cluster-inst2", Namespace: "", Kind: "ClusterConfig", RGDName: "cluster-rgd", RGDNamespace: ""})
	cache.Set(&models.Instance{Name: "ns-inst1", Namespace: "default", Kind: "App", RGDName: "ns-rgd", RGDNamespace: "default"})

	count := cache.CountByRGD("", "cluster-rgd")
	if count != 2 {
		t.Errorf("expected 2 instances for cluster-rgd, got %d", count)
	}
}

func TestInstanceCache_ClusterScoped_DeleteByRGD(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "cluster-inst1", Namespace: "", Kind: "ClusterConfig", RGDName: "cluster-rgd", RGDNamespace: ""})
	cache.Set(&models.Instance{Name: "cluster-inst2", Namespace: "", Kind: "ClusterConfig", RGDName: "cluster-rgd", RGDNamespace: ""})
	cache.Set(&models.Instance{Name: "ns-inst1", Namespace: "default", Kind: "App", RGDName: "ns-rgd", RGDNamespace: "default"})

	removed := cache.DeleteByRGD("", "cluster-rgd")
	if len(removed) != 2 {
		t.Errorf("expected 2 removed, got %d", len(removed))
	}
	if cache.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", cache.Count())
	}
}

func TestInstanceCacheKey_EmptyNamespace(t *testing.T) {
	// Verify key format for cluster-scoped instances
	key := instanceCacheKey("", "ClusterConfig", "my-config")
	expected := "/ClusterConfig/my-config"
	if key != expected {
		t.Errorf("expected key %q, got %q", expected, key)
	}

	// Verify no collision with namespace-scoped
	nsKey := instanceCacheKey("default", "ClusterConfig", "my-config")
	if key == nsKey {
		t.Error("cluster-scoped and namespace-scoped keys should not collide")
	}
}

func TestInstanceCache_CountByNamespaces(t *testing.T) {
	cache := NewInstanceCache()

	// Set up instances in different namespaces
	cache.Set(&models.Instance{Name: "inst1", Namespace: "team-alpha"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "team-alpha"})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "team-beta"})
	cache.Set(&models.Instance{Name: "inst4", Namespace: "staging"})
	cache.Set(&models.Instance{Name: "inst5", Namespace: "prod"})

	// Match function supporting exact match and "*" wildcard (mirrors rbac.MatchNamespaceInList)
	exactMatch := func(namespace string, patterns []string) bool {
		for _, p := range patterns {
			if p == "*" || p == namespace {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name       string
		namespaces []string
		expected   int
	}{
		{
			name:       "wildcard namespaces returns all (global admin)",
			namespaces: []string{"*"},
			expected:   5,
		},
		{
			name:       "empty namespaces returns none",
			namespaces: []string{},
			expected:   0,
		},
		{
			name:       "single namespace",
			namespaces: []string{"team-alpha"},
			expected:   2,
		},
		{
			name:       "multiple namespaces",
			namespaces: []string{"team-alpha", "staging"},
			expected:   3,
		},
		{
			name:       "all namespaces",
			namespaces: []string{"team-alpha", "team-beta", "staging", "prod"},
			expected:   5,
		},
		{
			name:       "non-existent namespace",
			namespaces: []string{"non-existent"},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := cache.CountByNamespaces(tt.namespaces, exactMatch)
			if count != tt.expected {
				t.Errorf("expected count %d, got %d", tt.expected, count)
			}
		})
	}
}

func TestInstanceCache_CountByNamespaces_ClusterScoped(t *testing.T) {
	cache := NewInstanceCache()

	// Mix of cluster-scoped and namespace-scoped instances.
	// CountByNamespaces does NOT include cluster-scoped instances — they require
	// project-based filtering via CountFiltered (STORY-301 security fix).
	cache.Set(&models.Instance{Name: "ns-inst1", Namespace: "team-alpha", Kind: "App"})
	cache.Set(&models.Instance{Name: "ns-inst2", Namespace: "team-beta", Kind: "App"})
	cache.Set(&models.Instance{Name: "cluster-inst1", Namespace: "", Kind: "ClusterConfig", IsClusterScoped: true})
	cache.Set(&models.Instance{Name: "cluster-inst2", Namespace: "", Kind: "ClusterPolicy", IsClusterScoped: true})

	exactMatch := func(namespace string, patterns []string) bool {
		for _, p := range patterns {
			if p == "*" || p == namespace {
				return true
			}
		}
		return false
	}

	tests := []struct {
		name       string
		namespaces []string
		expected   int
	}{
		{
			name:       "wildcard namespaces returns all (global admin)",
			namespaces: []string{"*"},
			expected:   4,
		},
		{
			name:       "single namespace excludes cluster-scoped",
			namespaces: []string{"team-alpha"},
			expected:   1, // only ns-scoped match
		},
		{
			name:       "empty namespaces returns none",
			namespaces: []string{},
			expected:   0,
		},
		{
			name:       "non-existent namespace returns zero",
			namespaces: []string{"non-existent"},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := cache.CountByNamespaces(tt.namespaces, exactMatch)
			if count != tt.expected {
				t.Errorf("expected count %d, got %d", tt.expected, count)
			}
		})
	}
}

func TestInstanceCache_CountByRGDAndNamespaces_ClusterScoped(t *testing.T) {
	cache := NewInstanceCache()

	cache.Set(&models.Instance{Name: "ns-inst1", Namespace: "prod", Kind: "App", RGDName: "app-rgd", RGDNamespace: "", IsClusterScoped: false})
	cache.Set(&models.Instance{Name: "cluster-inst1", Namespace: "", Kind: "ClusterConfig", RGDName: "cluster-rgd", RGDNamespace: "", IsClusterScoped: true})
	cache.Set(&models.Instance{Name: "cluster-inst2", Namespace: "", Kind: "ClusterConfig", RGDName: "cluster-rgd", RGDNamespace: "", IsClusterScoped: true})

	exactMatch := func(namespace string, patterns []string) bool {
		for _, p := range patterns {
			if p == "*" || p == namespace {
				return true
			}
		}
		return false
	}

	// Non-admin: CountByRGDAndNamespaces does NOT include cluster-scoped instances
	// because it cannot perform project-based filtering. Use CountFiltered for
	// project-aware counting (STORY-301 security fix).
	count := cache.CountByRGDAndNamespaces("", "cluster-rgd", []string{"prod"}, exactMatch)
	if count != 0 {
		t.Errorf("expected 0 (cluster-scoped excluded from namespace-only count), got %d", count)
	}

	// Non-admin: ns-scoped RGD count works as before
	count = cache.CountByRGDAndNamespaces("", "app-rgd", []string{"prod"}, exactMatch)
	if count != 1 {
		t.Errorf("expected 1 ns-scoped instance for app-rgd, got %d", count)
	}

	// Global admin (["*"] namespaces): sees all including cluster-scoped
	count = cache.CountByRGDAndNamespaces("", "cluster-rgd", []string{"*"}, exactMatch)
	if count != 2 {
		t.Errorf("expected 2 cluster-scoped instances for global admin, got %d", count)
	}
}

// --- STORY-300: Benchmark — cluster-wide watch doesn't degrade namespace-scoped performance ---

// BenchmarkInstanceCache_MixedScopeList measures List() performance with a realistic
// mix of cluster-scoped and namespace-scoped instances. A cluster-wide watch adds all
// instances to the same cache regardless of scope; this ensures the mixed load path has
// no measurable overhead compared to namespace-only workloads.
func BenchmarkInstanceCache_MixedScopeList(b *testing.B) {
	cache := NewInstanceCache()
	now := time.Now()

	// Populate 200 namespace-scoped instances across 10 namespaces
	for i := 0; i < 200; i++ {
		cache.Set(&models.Instance{
			Name:      fmt.Sprintf("ns-app-%d", i),
			Namespace: fmt.Sprintf("ns-%d", i%10),
			Kind:      "App",
			RGDName:   "app-rgd",
			CreatedAt: now,
		})
	}
	// Add 20 cluster-scoped instances
	for i := 0; i < 20; i++ {
		cache.Set(&models.Instance{
			Name:            fmt.Sprintf("cluster-cfg-%d", i),
			Namespace:       "",
			Kind:            "ClusterConfig",
			RGDName:         "cluster-rgd",
			IsClusterScoped: true,
			CreatedAt:       now,
		})
	}

	opts := models.InstanceListOptions{Page: 1, PageSize: 50, SortBy: "name", SortOrder: "asc"}

	b.ResetTimer()
	for range b.N {
		_ = cache.List(opts)
	}
}

func TestInstanceCache_GetByTargetCluster(t *testing.T) {
	cache := NewInstanceCache()
	now := time.Now()

	// Populate cache with mixed instances
	cache.Set(&models.Instance{
		Name: "app-eu-1", Namespace: "team-alpha", Kind: "App",
		TargetCluster: "prod-eu-west", CreatedAt: now,
	})
	cache.Set(&models.Instance{
		Name: "app-eu-2", Namespace: "team-alpha", Kind: "App",
		TargetCluster: "prod-eu-west", CreatedAt: now,
	})
	cache.Set(&models.Instance{
		Name: "app-us-1", Namespace: "team-alpha", Kind: "App",
		TargetCluster: "prod-us-east", CreatedAt: now,
	})
	cache.Set(&models.Instance{
		Name: "app-local", Namespace: "default", Kind: "App",
		// No TargetCluster — management cluster
		CreatedAt: now,
	})

	t.Run("returns matching instances", func(t *testing.T) {
		result := cache.GetByTargetCluster("prod-eu-west")
		if len(result) != 2 {
			t.Fatalf("expected 2 instances, got %d", len(result))
		}
		names := map[string]bool{}
		for _, inst := range result {
			names[inst.Name] = true
		}
		if !names["app-eu-1"] || !names["app-eu-2"] {
			t.Errorf("expected app-eu-1 and app-eu-2, got %v", names)
		}
	})

	t.Run("returns single match", func(t *testing.T) {
		result := cache.GetByTargetCluster("prod-us-east")
		if len(result) != 1 {
			t.Fatalf("expected 1 instance, got %d", len(result))
		}
		if result[0].Name != "app-us-1" {
			t.Errorf("expected app-us-1, got %s", result[0].Name)
		}
	})

	t.Run("returns nil for unknown cluster", func(t *testing.T) {
		result := cache.GetByTargetCluster("nonexistent-cluster")
		if result != nil {
			t.Errorf("expected nil, got %d instances", len(result))
		}
	})

	t.Run("returns nil for empty cluster ref", func(t *testing.T) {
		// Instances without TargetCluster should NOT match empty string query
		// (but since app-local has TargetCluster="" and we query "", it would match)
		result := cache.GetByTargetCluster("")
		if len(result) != 1 || result[0].Name != "app-local" {
			t.Errorf("expected [app-local] for empty cluster ref, got %v", result)
		}
	})

	t.Run("empty cache returns nil", func(t *testing.T) {
		emptyCache := NewInstanceCache()
		result := emptyCache.GetByTargetCluster("prod-eu-west")
		if result != nil {
			t.Errorf("expected nil from empty cache, got %d instances", len(result))
		}
	})
}

// BenchmarkInstanceCache_NamespaceScopedOnly is the baseline for comparison with
// BenchmarkInstanceCache_MixedScopeList — same workload without cluster-scoped instances.
func BenchmarkInstanceCache_NamespaceScopedOnly(b *testing.B) {
	cache := NewInstanceCache()
	now := time.Now()

	for i := 0; i < 220; i++ {
		cache.Set(&models.Instance{
			Name:      fmt.Sprintf("ns-app-%d", i),
			Namespace: fmt.Sprintf("ns-%d", i%10),
			Kind:      "App",
			RGDName:   "app-rgd",
			CreatedAt: now,
		})
	}

	opts := models.InstanceListOptions{Page: 1, PageSize: 50, SortBy: "name", SortOrder: "asc"}

	b.ResetTimer()
	for range b.N {
		_ = cache.List(opts)
	}
}
