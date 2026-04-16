// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/icons"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// writeJSON marshals v and writes it as JSON. Returns false (and writes an error response) on failure.
// Marshaling before writing prevents partial-write corruption: if Marshal fails, the response body
// is untouched so an error status can still be set.
func writeJSON(w http.ResponseWriter, v any) bool {
	b, err := json.Marshal(v)
	if err != nil {
		response.InternalError(w, "failed to encode response")
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
	return true
}

// iconNameRe validates that an icon name from the ConfigMap is a safe Lucide icon identifier.
var iconNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// resolveIcon sets Icon and IconType on cat using the icon registry chain.
// When registry is nil, falls back to "layout-grid" / "lucide".
func resolveIcon(cat *services.Category, registry *icons.Registry) {
	if registry != nil {
		iconType, iconValue := registry.GetCategoryIcon(cat.Slug)
		cat.IconType = iconType
		if iconType == "svg" {
			cat.Icon = icons.CategoryIconPrefix + cat.Slug
		} else {
			cat.Icon = iconValue
		}
	} else {
		cat.IconType = "lucide"
		if cat.Icon == "" {
			cat.Icon = "layout-grid"
		}
	}
}

// CategoriesHandler provides HTTP handlers for the categories API endpoints.
// Categories are auto-discovered from knodex.io/category annotations on live RGDs.
// Each category is filtered per-item using the caller's Casbin rgds/{category}/* policies.
// Icons are resolved from the icon registry (ConfigMap custom → default Lucide → fallback).
type CategoriesHandler struct {
	service        services.CategoryService
	enforcer       rbac.Authorizer
	iconRegistry   *icons.Registry
	categoryConfig []services.CategoryEntry // nil = no sub-nav; loaded from ConfigMap at startup
	logger         *slog.Logger
}

// NewCategoriesHandler creates a new categories HTTP handler.
func NewCategoriesHandler(service services.CategoryService, enforcer rbac.Authorizer, iconRegistry *icons.Registry, categoryConfig []services.CategoryEntry, logger *slog.Logger) *CategoriesHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CategoriesHandler{
		service:        service,
		enforcer:       enforcer,
		iconRegistry:   iconRegistry,
		categoryConfig: categoryConfig,
		logger:         logger.With("component", "categories-handler"),
	}
}

// ListCategories handles GET /api/v1/categories
// Returns the categories visible to the authenticated user based on their
// Casbin rgds/{category}/*, get policies. Each category is individually
// checked — no top-level gate.
func (h *CategoriesHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	if h.enforcer == nil {
		response.Forbidden(w, "authorization not configured")
		return
	}
	if h.service == nil {
		response.InternalError(w, "category service not configured")
		return
	}

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// No category config → no sub-nav; return empty list
	if len(h.categoryConfig) == 0 {
		writeJSON(w, services.CategoryList{Categories: []services.Category{}})
		return
	}

	// Build case-insensitive name → entry lookup
	configByName := make(map[string]services.CategoryEntry, len(h.categoryConfig))
	for _, entry := range h.categoryConfig {
		configByName[strings.ToLower(entry.Name)] = entry
	}

	// Get all discovered categories from service
	all := h.service.ListCategories(r.Context())

	// Filter to ConfigMap-defined categories
	type candidate struct {
		cat    services.Category
		entry  services.CategoryEntry
		weight int
	}
	candidates := make([]candidate, 0, len(h.categoryConfig))
	for _, cat := range all.Categories {
		entry, ok := configByName[strings.ToLower(cat.Name)]
		if !ok {
			continue // not in ConfigMap — hidden from sidebar
		}
		candidates = append(candidates, candidate{cat: cat, entry: entry, weight: entry.Weight})
	}

	// Sort by weight ascending; alphabetical name tiebreak
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].weight != candidates[j].weight {
			return candidates[i].weight < candidates[j].weight
		}
		return candidates[i].cat.Name < candidates[j].cat.Name
	})

	// Per-category Casbin filter, then resolve icons only for visible categories
	visible := make([]services.Category, 0, len(candidates))
	for _, c := range candidates {
		allowed, err := h.enforcer.CanAccessWithGroups(
			r.Context(),
			userCtx.UserID,
			userCtx.Groups,
			"rgds/"+c.cat.Slug+"/*",
			rbac.ActionGet,
		)
		if err != nil {
			h.logger.Warn("category authorization check failed",
				"category", c.cat.Slug,
				"user", userCtx.UserID,
				"error", err,
			)
			continue
		}
		if allowed {
			// Resolve icon after auth check to avoid wasted work and misleading logs
			if c.entry.Icon != "" {
				if iconNameRe.MatchString(c.entry.Icon) {
					c.cat.Icon = c.entry.Icon
					c.cat.IconType = "lucide"
				} else {
					h.logger.Warn("category config icon has invalid format, ignoring",
						"category", c.cat.Name, "icon", c.entry.Icon)
					resolveIcon(&c.cat, h.iconRegistry)
				}
			} else {
				resolveIcon(&c.cat, h.iconRegistry)
			}
			visible = append(visible, c.cat)
		}
	}

	writeJSON(w, services.CategoryList{Categories: visible})
}

// GetCategory handles GET /api/v1/categories/{slug}
// Returns a specific category if it exists and the caller has
// rgds/{slug}/*, get Casbin access.
func (h *CategoriesHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	if h.enforcer == nil {
		response.Forbidden(w, "authorization not configured")
		return
	}
	if h.service == nil {
		response.InternalError(w, "category service not configured")
		return
	}

	slug := r.PathValue("slug")
	if slug == "" {
		response.BadRequest(w, "category slug required", nil)
		return
	}

	// Escape glob characters to prevent wildcard injection in Casbin object paths.
	// A request to /categories/* would otherwise construct "rgds/*/*" matching any policy.
	safeSlug := sanitize.GlobCharacters(slug)

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Check Casbin access for this category
	allowed, err := h.enforcer.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"rgds/"+safeSlug+"/*",
		rbac.ActionGet,
	)
	if err != nil {
		h.logger.Warn("category authorization check failed",
			"category", slug,
			"user", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "authorization check failed")
		return
	}
	if !allowed {
		response.Forbidden(w, "access denied")
		return
	}

	cat := h.service.GetCategory(r.Context(), safeSlug)
	if cat == nil {
		response.NotFound(w, "category", slug)
		return
	}

	// ConfigMap gate: build slug-keyed lookup from config entries using same slug derivation as service
	if len(h.categoryConfig) == 0 {
		response.NotFound(w, "category", slug)
		return
	}
	configBySlug := make(map[string]services.CategoryEntry, len(h.categoryConfig))
	for _, entry := range h.categoryConfig {
		entrySlug := sanitize.GlobCharacters(strings.ToLower(entry.Name))
		configBySlug[entrySlug] = entry
	}
	configEntry, inConfig := configBySlug[safeSlug]
	if !inConfig {
		h.logger.Warn("category exists but not in category config, returning 404",
			"category", slug)
		response.NotFound(w, "category", slug)
		return
	}

	// Resolve icon: ConfigMap override takes precedence over registry
	if configEntry.Icon != "" {
		if iconNameRe.MatchString(configEntry.Icon) {
			cat.Icon = configEntry.Icon
			cat.IconType = "lucide"
		} else {
			h.logger.Warn("category config icon has invalid format, ignoring",
				"category", cat.Name, "icon", configEntry.Icon)
			resolveIcon(cat, h.iconRegistry)
		}
	} else {
		resolveIcon(cat, h.iconRegistry)
	}

	writeJSON(w, cat)
}
