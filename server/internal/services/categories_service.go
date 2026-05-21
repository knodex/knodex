// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"sort"
	"strings"
)

// CategoryService defines the interface for category operations.
// Categories are auto-discovered from knodex.io/category annotations on live RGDs.
// This is an OSS feature — no license required, no ConfigMap needed.
type CategoryService interface {
	// ListCategories returns all categories auto-discovered from RGD annotations.
	// Each category includes the count of RGDs in that category.
	ListCategories(ctx context.Context) CategoryList

	// GetCategory returns a specific category by slug.
	// Returns nil if not found.
	GetCategory(ctx context.Context, slug string) *Category
}

// Category represents an auto-discovered category from RGD annotations.
type Category struct {
	// Name is the display name (the annotation value as found on RGDs)
	Name string `json:"name"`

	// Slug is the URL-safe identifier (lowercase category annotation value)
	Slug string `json:"slug"`

	// Icon is the icon identifier. When IconType is "lucide", this is a Lucide icon name
	// (e.g., "server", "activity"). When IconType is "svg", this is the slug for
	// GET /api/v1/icons/{slug} (e.g., "cat:infrastructure").
	Icon string `json:"icon"`

	// IconType indicates how to render the icon: "lucide" for a Lucide React icon,
	// "svg" for a custom SVG served from the icons API.
	IconType string `json:"iconType"`

	// Count is the number of RGDs with this category annotation
	Count int `json:"count"`
}

// CategoryList represents the response from the categories API endpoint.
type CategoryList struct {
	// Categories is the list of discovered categories with counts
	Categories []Category `json:"categories"`
}

// CategoryEntry represents a single entry in the knodex-category-config ConfigMap.
// It defines which categories appear in the sidebar sub-nav, in what order,
// and optionally overrides the display icon.
// Both yaml and json tags are required: sigs.k8s.io/yaml unmarshals via encoding/json internally.
type CategoryEntry struct {
	Name   string `yaml:"name"   json:"name"`
	Weight int    `yaml:"weight" json:"weight"`
	Icon   string `yaml:"icon,omitempty" json:"icon,omitempty"`
}

// MergeCategoryConfigs merges multiple ConfigMap payloads into a deduplicated CategoryEntry slice.
// configsByName maps ConfigMap name → parsed []CategoryEntry.
//
// Merge rules:
//   - Weight: minimum across all active configs for that category name.
//   - Icon: from the alphabetically-first ConfigMap name with a non-empty icon for that category.
//   - Display name: preserved from the entry with the lowest weight (alphabetically-first CM on tie).
//
// Output is sorted by weight ascending, then display name alphabetically as tiebreaker.
func MergeCategoryConfigs(configsByName map[string][]CategoryEntry) []CategoryEntry {
	if len(configsByName) == 0 {
		return nil
	}

	type mergeState struct {
		weight      int
		displayName string
		icon        string
		iconSource  string // CM name that provided the icon (for alphabetical tiebreak)
	}

	cmNames := make([]string, 0, len(configsByName))
	for name := range configsByName {
		cmNames = append(cmNames, name)
	}
	sort.Strings(cmNames)

	byKey := make(map[string]*mergeState)

	for _, cmName := range cmNames {
		for _, entry := range configsByName[cmName] {
			key := strings.ToLower(entry.Name)
			if existing, ok := byKey[key]; !ok {
				s := &mergeState{weight: entry.Weight, displayName: entry.Name}
				if entry.Icon != "" {
					s.icon = entry.Icon
					s.iconSource = cmName
				}
				byKey[key] = s
			} else {
				if entry.Weight < existing.weight {
					existing.weight = entry.Weight
					existing.displayName = entry.Name
				}
				if entry.Icon != "" && (existing.icon == "" || cmName < existing.iconSource) {
					existing.icon = entry.Icon
					existing.iconSource = cmName
				}
			}
		}
	}

	result := make([]CategoryEntry, 0, len(byKey))
	for _, s := range byKey {
		result = append(result, CategoryEntry{Name: s.displayName, Weight: s.weight, Icon: s.icon})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Weight != result[j].Weight {
			return result[i].Weight < result[j].Weight
		}
		return result[i].Name < result[j].Name
	})
	return result
}
