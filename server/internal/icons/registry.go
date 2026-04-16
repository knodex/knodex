// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package icons

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"strings"
)

//go:embed icons.json
var builtinJSON []byte

// Registry holds the icon slug → SVG string mapping.
type Registry struct {
	icons map[string]string
}

// NewRegistry creates a Registry pre-loaded with the built-in icon set.
func NewRegistry() (*Registry, error) {
	var data map[string]string
	if err := json.Unmarshal(builtinJSON, &data); err != nil {
		return nil, err
	}
	return &Registry{icons: data}, nil
}

// NewEmptyRegistry creates a Registry with no icons (used as a safe fallback).
func NewEmptyRegistry() *Registry {
	return &Registry{icons: make(map[string]string)}
}

// Get returns the SVG string for a slug, or ("", false) if not found.
func (r *Registry) Get(slug string) (string, bool) {
	svg, ok := r.icons[slug]
	return svg, ok
}

// CategoryIconPrefix is the ConfigMap key prefix for category icons.
// Example: "cat:infrastructure" in the knodex-custom-icons ConfigMap.
const CategoryIconPrefix = "cat:"

// defaultCategoryIcons maps well-known category slugs to Lucide icon names.
// These are used when no custom SVG is defined in the ConfigMap.
var defaultCategoryIcons = map[string]string{
	"infrastructure": "server",
	"observability":  "activity",
	"applications":   "app-window",
	"examples":       "book-open",
	"networking":     "network",
	"security":       "shield",
	"storage":        "hard-drive",
	"databases":      "database",
	"controller":     "repeat",
	"controllers":    "repeat",
	"provider":       "cloud",
	"providers":      "cloud",
	"cloud":          "cloud",
	"database":       "database",
	"vault":          "lock",
}

// GetCategoryIcon returns the icon info for a category slug.
// Resolution chain:
//  1. ConfigMap custom icon: "cat:{slug}" key → returns ("svg", svgContent)
//  2. Default Lucide mapping → returns ("lucide", lucideIconName)
//  3. Fallback → returns ("lucide", "layout-grid")
func (r *Registry) GetCategoryIcon(slug string) (iconType, iconValue string) {
	// 1. Check ConfigMap for custom SVG: "cat:{slug}"
	if svg, ok := r.icons[CategoryIconPrefix+slug]; ok {
		return "svg", svg
	}

	// 2. Check default Lucide mapping
	if lucide, ok := defaultCategoryIcons[slug]; ok {
		return "lucide", lucide
	}

	// 3. Fallback
	return "lucide", "layout-grid"
}

// ConfigMapEntry holds the name and data of a single ConfigMap for icon loading.
type ConfigMapEntry struct {
	Name string
	Data map[string]string
}

// isSafeSVG returns false if the SVG string contains obvious XSS vectors.
// SVG served as image/svg+xml from a same-origin endpoint can execute scripts,
// so we reject content with <script elements or javascript: URIs.
func isSafeSVG(svg string) bool {
	lower := strings.ToLower(svg)
	return !strings.Contains(lower, "<script") && !strings.Contains(lower, "javascript:")
}

// LoadFromConfigMaps merges icons from multiple ConfigMaps into the registry.
// Entries must be pre-sorted alphabetically by Name before calling — first entry wins on collision.
// Custom entries take precedence over the built-in set; within custom entries, the first
// ConfigMap (alphabetically by name) wins and a warning is logged for each collision.
// SVG content is checked for obvious XSS vectors and rejected with a warning if unsafe.
func (r *Registry) LoadFromConfigMaps(cms []ConfigMapEntry, logger *slog.Logger) {
	customSlugs := make(map[string]string) // slug → first ConfigMap name that defined it
	for _, cm := range cms {
		for slug, svg := range cm.Data {
			if !isSafeSVG(svg) {
				logger.Warn("icon rejected — SVG contains unsafe content",
					"slug", slug, "configmap", cm.Name)
				continue
			}
			if prevCM, isCustomCollision := customSlugs[slug]; isCustomCollision {
				// ConfigMap-vs-ConfigMap collision: first alphabetically wins
				logger.Warn("icon slug collision between ConfigMaps — first alphabetically wins",
					"slug", slug, "kept", prevCM, "skipped", cm.Name)
				continue
			}
			// Custom entries always override built-in set
			r.icons[slug] = svg
			customSlugs[slug] = cm.Name
		}
	}
}
