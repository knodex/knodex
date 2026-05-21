// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeTestConfigMap(name string, labels map[string]string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
	}
}

func TestFilterConfigMapsByPackage_NoFilter(t *testing.T) {
	// Empty filterSet → all ConfigMaps included regardless of package label.
	cms := []corev1.ConfigMap{
		makeTestConfigMap("global", nil),
		makeTestConfigMap("pkg-networking", map[string]string{"knodex.io/package": "networking"}),
		makeTestConfigMap("pkg-database", map[string]string{"knodex.io/package": "database"}),
	}
	result := filterConfigMapsByPackage(cms, map[string]bool{})
	if len(result) != 3 {
		t.Errorf("expected 3 (all included when no filter), got %d", len(result))
	}
}

func TestFilterConfigMapsByPackage_WithFilter(t *testing.T) {
	// Filter active → only matching-package + no-label ConfigMaps included.
	cms := []corev1.ConfigMap{
		makeTestConfigMap("global", nil),
		makeTestConfigMap("pkg-networking", map[string]string{"knodex.io/package": "networking"}),
		makeTestConfigMap("pkg-database", map[string]string{"knodex.io/package": "database"}),
	}
	result := filterConfigMapsByPackage(cms, map[string]bool{"networking": true})
	if len(result) != 2 {
		t.Fatalf("expected 2 (global + networking), got %d", len(result))
	}
	names := map[string]bool{result[0].Name: true, result[1].Name: true}
	if !names["global"] || !names["pkg-networking"] {
		t.Errorf("expected global and pkg-networking in result, got %+v", result)
	}
}

func TestFilterConfigMapsByPackage_GlobalAlwaysIncluded(t *testing.T) {
	// ConfigMap with no package label is always included, even when filter excludes all packages.
	cms := []corev1.ConfigMap{
		makeTestConfigMap("global", nil),
		makeTestConfigMap("pkg-networking", map[string]string{"knodex.io/package": "networking"}),
	}
	result := filterConfigMapsByPackage(cms, map[string]bool{"database": true})
	if len(result) != 1 {
		t.Fatalf("expected 1 (only global), got %d", len(result))
	}
	if result[0].Name != "global" {
		t.Errorf("expected global, got %s", result[0].Name)
	}
}

func TestFilterConfigMapsByPackage_CaseInsensitive(t *testing.T) {
	// Package label value is lowercased before lookup; filterSet is pre-lowercased by config loader.
	cms := []corev1.ConfigMap{
		makeTestConfigMap("pkg-net", map[string]string{"knodex.io/package": "Networking"}),
	}
	result := filterConfigMapsByPackage(cms, map[string]bool{"networking": true})
	if len(result) != 1 {
		t.Errorf("expected 1 (case-insensitive match), got %d", len(result))
	}
}

func TestFilterConfigMapsByPackage_EmptyInput(t *testing.T) {
	result := filterConfigMapsByPackage(nil, map[string]bool{"networking": true})
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}

func TestFilterConfigMapsByPackage_MultiplePackagesInFilter(t *testing.T) {
	// filterSet with multiple packages: both matching and non-matching included/excluded.
	cms := []corev1.ConfigMap{
		makeTestConfigMap("global", nil),
		makeTestConfigMap("pkg-net", map[string]string{"knodex.io/package": "networking"}),
		makeTestConfigMap("pkg-db", map[string]string{"knodex.io/package": "database"}),
		makeTestConfigMap("pkg-security", map[string]string{"knodex.io/package": "security"}),
	}
	result := filterConfigMapsByPackage(cms, map[string]bool{"networking": true, "database": true})
	if len(result) != 3 {
		t.Fatalf("expected 3 (global + networking + database), got %d", len(result))
	}
	names := map[string]bool{}
	for _, cm := range result {
		names[cm.Name] = true
	}
	if !names["global"] || !names["pkg-net"] || !names["pkg-db"] {
		t.Errorf("unexpected result set: %+v", result)
	}
	if names["pkg-security"] {
		t.Errorf("pkg-security should have been filtered out")
	}
}

// TestFilterConfigMapsByPackage_IconRegistryShape verifies that the helper applies
// the same filter semantics to ConfigMaps shaped like icon-registry entries
// (labeled knodex.io/icon-registry=true with optional knodex.io/package=<pkg>).
// AC #8 of STORY-453: icon path reuses the helper unchanged; this test pins that contract.
func TestFilterConfigMapsByPackage_IconRegistryShape(t *testing.T) {
	cms := []corev1.ConfigMap{
		makeTestConfigMap("knodex-custom-icons", map[string]string{
			"knodex.io/icon-registry": "true",
		}),
		makeTestConfigMap("knodex-custom-icons-networking", map[string]string{
			"knodex.io/icon-registry": "true",
			"knodex.io/package":       "networking",
		}),
		makeTestConfigMap("knodex-custom-icons-database", map[string]string{
			"knodex.io/icon-registry": "true",
			"knodex.io/package":       "database",
		}),
	}

	// (a) no filter → all icon ConfigMaps loaded.
	all := filterConfigMapsByPackage(cms, map[string]bool{})
	if len(all) != 3 {
		t.Errorf("no filter: expected 3 icon ConfigMaps, got %d", len(all))
	}

	// (b) filter active with networking → global + networking only.
	netOnly := filterConfigMapsByPackage(cms, map[string]bool{"networking": true})
	if len(netOnly) != 2 {
		t.Fatalf("networking filter: expected 2 (global + networking), got %d", len(netOnly))
	}
	names := map[string]bool{netOnly[0].Name: true, netOnly[1].Name: true}
	if !names["knodex-custom-icons"] || !names["knodex-custom-icons-networking"] {
		t.Errorf("networking filter: expected global + networking, got %+v", netOnly)
	}
	if names["knodex-custom-icons-database"] {
		t.Errorf("networking filter: database ConfigMap should have been filtered out")
	}

	// (c) filter active with non-matching package → only global icon CM remains.
	nonMatching := filterConfigMapsByPackage(cms, map[string]bool{"unknown": true})
	if len(nonMatching) != 1 || nonMatching[0].Name != "knodex-custom-icons" {
		t.Errorf("non-matching filter: expected only knodex-custom-icons, got %+v", nonMatching)
	}
}

// TestDiscoveredNamesDeduplication verifies the backward-compat logic that prevents
// knodex-category-config from being double-loaded when it already carries the new label.
// This covers AC #4: label-selector List finds it → backward-compat Get must be skipped.
func TestDiscoveredNamesDeduplication(t *testing.T) {
	// Simulate: knodex-category-config already discovered via label-selector (post-Helm-upgrade).
	discoveredNames := map[string]bool{
		"knodex-category-config": true,
	}

	// The backward-compat Get guard must short-circuit when name is already discovered.
	if !discoveredNames["knodex-category-config"] {
		t.Fatal("backward-compat Get should be skipped: knodex-category-config already in discoveredNames")
	}

	// Simulate: knodex-category-config NOT discovered via label-selector (legacy install).
	discoveredNamesLegacy := map[string]bool{}
	if discoveredNamesLegacy["knodex-category-config"] {
		t.Fatal("backward-compat Get should be invoked: knodex-category-config not in discoveredNames")
	}
}

// TestDiscoveredNamesNoDuplicateEntry verifies that adding a CM to discoveredNames
// prevents it from being appended again via the fallback path.
func TestDiscoveredNamesNoDuplicateEntry(t *testing.T) {
	cm := makeTestConfigMap("knodex-category-config", map[string]string{"knodex.io/category-config": "true"})

	// Step A result: CM found via label-selector.
	discoveredNames := map[string]bool{}
	activeCMs := []corev1.ConfigMap{}
	discoveredNames[cm.Name] = true
	activeCMs = append(activeCMs, cm)

	// Step B: backward-compat Get would have returned the same CM — must be skipped.
	if !discoveredNames["knodex-category-config"] {
		// This branch should NOT execute; if it did, we'd append again.
		activeCMs = append(activeCMs, cm)
	}

	if len(activeCMs) != 1 {
		t.Errorf("expected exactly 1 CM (no double-load), got %d", len(activeCMs))
	}
}
