// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"testing"
)

func TestMergeCategoryConfigs_Empty(t *testing.T) {
	result := MergeCategoryConfigs(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
	result = MergeCategoryConfigs(map[string][]CategoryEntry{})
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestMergeCategoryConfigs_SingleConfig(t *testing.T) {
	input := map[string][]CategoryEntry{
		"knodex-category-config": {
			{Name: "Infrastructure", Weight: 10, Icon: "server"},
			{Name: "Applications", Weight: 20},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Name != "Infrastructure" || result[0].Weight != 10 || result[0].Icon != "server" {
		t.Errorf("unexpected first entry: %+v", result[0])
	}
	if result[1].Name != "Applications" || result[1].Weight != 20 {
		t.Errorf("unexpected second entry: %+v", result[1])
	}
}

func TestMergeCategoryConfigs_NoOverlap(t *testing.T) {
	input := map[string][]CategoryEntry{
		"config-a": {
			{Name: "Infrastructure", Weight: 10},
		},
		"config-b": {
			{Name: "Networking", Weight: 30},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (union), got %d", len(result))
	}
	names := map[string]bool{result[0].Name: true, result[1].Name: true}
	if !names["Infrastructure"] || !names["Networking"] {
		t.Errorf("expected both categories in result, got %+v", result)
	}
}

func TestMergeCategoryConfigs_WeightMinimum(t *testing.T) {
	// Same category (case-insensitive) in two configs with different weights →
	// lower weight wins, and display name comes from the lower-weight entry.
	input := map[string][]CategoryEntry{
		"config-global": {
			{Name: "NETWORKING", Weight: 30, Icon: "network"},
		},
		"config-networking": {
			{Name: "networking", Weight: 5},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged entry, got %d", len(result))
	}
	if result[0].Weight != 5 {
		t.Errorf("expected weight 5 (minimum), got %d", result[0].Weight)
	}
	if result[0].Name != "networking" {
		t.Errorf("expected display name from lower-weight entry (networking), got %q", result[0].Name)
	}
}

func TestMergeCategoryConfigs_WeightSameTiebreak(t *testing.T) {
	// Same category name (case-insensitive), same weight → display name from
	// alphabetically-first ConfigMap is preserved (config-aaa processed first).
	input := map[string][]CategoryEntry{
		"config-zzz": {
			{Name: "NETWORKING", Weight: 10},
		},
		"config-aaa": {
			{Name: "networking", Weight: 10},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged entry, got %d: %+v", len(result), result)
	}
	// "config-aaa" is processed first (sorted), so its displayName is kept.
	if result[0].Name != "networking" {
		t.Errorf("expected displayName from alphabetically-first CM (networking), got %q", result[0].Name)
	}
}

func TestMergeCategoryConfigs_IconFromEarliestCM(t *testing.T) {
	// Icon from the alphabetically-first ConfigMap with a non-empty icon wins.
	input := map[string][]CategoryEntry{
		"config-aaa": {
			{Name: "Security", Weight: 15}, // no icon
		},
		"config-bbb": {
			{Name: "Security", Weight: 20, Icon: "shield"},
		},
		"config-ccc": {
			{Name: "Security", Weight: 25, Icon: "lock"},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged entry, got %d", len(result))
	}
	// "config-bbb" is alphabetically first with a non-empty icon.
	if result[0].Icon != "shield" {
		t.Errorf("expected icon from config-bbb (shield), got %q", result[0].Icon)
	}
	// Weight: minimum is 15 (from config-aaa).
	if result[0].Weight != 15 {
		t.Errorf("expected weight 15 (minimum), got %d", result[0].Weight)
	}
}

func TestMergeCategoryConfigs_SortOrder(t *testing.T) {
	input := map[string][]CategoryEntry{
		"config-a": {
			{Name: "Zebra", Weight: 100},
			{Name: "Alpha", Weight: 10},
		},
		"config-b": {
			{Name: "Middle", Weight: 50},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	expected := []struct {
		name   string
		weight int
	}{
		{"Alpha", 10},
		{"Middle", 50},
		{"Zebra", 100},
	}
	for i, exp := range expected {
		if result[i].Name != exp.name || result[i].Weight != exp.weight {
			t.Errorf("index %d: expected {%s %d}, got {%s %d}", i, exp.name, exp.weight, result[i].Name, result[i].Weight)
		}
	}
}

func TestMergeCategoryConfigs_GlobalAndPackageUnion(t *testing.T) {
	// Verifies that when all configs are included (no filter active, AC #9e),
	// global and package entries merge correctly into a unified list.
	input := map[string][]CategoryEntry{
		"knodex-category-config": {
			{Name: "Infrastructure", Weight: 10},
			{Name: "Networking", Weight: 30},
		},
		"knodex-category-config-networking": {
			{Name: "Networking", Weight: 5, Icon: "network"},
			{Name: "Security", Weight: 15},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 merged entries (Infrastructure, Networking, Security), got %d: %+v", len(result), result)
	}
	// Networking: min weight = 5 from knodex-category-config-networking.
	var networking CategoryEntry
	for _, e := range result {
		if e.Name == "Networking" {
			networking = e
		}
	}
	if networking.Weight != 5 {
		t.Errorf("expected Networking weight=5, got %d", networking.Weight)
	}
	if networking.Icon != "network" {
		t.Errorf("expected Networking icon=network, got %q", networking.Icon)
	}
}

func TestMergeCategoryConfigs_DisplayNameFromLowestWeight(t *testing.T) {
	// config-aaa is processed first (alphabetically) but carries high weight.
	// config-bbb is processed second but carries low weight.
	// The lower-weight entry's display name must override the first-processed entry's name.
	input := map[string][]CategoryEntry{
		"config-aaa": {
			{Name: "Infra", Weight: 100},
		},
		"config-bbb": {
			{Name: "INFRA", Weight: 1},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged entry, got %d", len(result))
	}
	if result[0].Weight != 1 {
		t.Errorf("expected weight 1 (minimum), got %d", result[0].Weight)
	}
	if result[0].Name != "INFRA" {
		t.Errorf("expected display name from lower-weight entry (INFRA), got %q", result[0].Name)
	}
}

func TestMergeCategoryConfigs_SortOrderEqualWeightTiebreak(t *testing.T) {
	// When multiple categories share the same weight, they must be sorted alphabetically by name.
	input := map[string][]CategoryEntry{
		"config-a": {
			{Name: "Zebra", Weight: 10},
			{Name: "Apple", Weight: 10},
		},
		"config-b": {
			{Name: "Mango", Weight: 10},
		},
	}
	result := MergeCategoryConfigs(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(result), result)
	}
	expected := []string{"Apple", "Mango", "Zebra"}
	for i, exp := range expected {
		if result[i].Name != exp {
			t.Errorf("index %d: expected %q (alphabetical weight tiebreak), got %q", i, exp, result[i].Name)
		}
	}
}
