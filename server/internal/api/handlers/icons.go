// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"
	"regexp"

	"github.com/knodex/knodex/server/internal/icons"
)

// slugPattern allows lowercase alphanumeric, hyphens, and colon for category prefix (cat:slug).
var slugPattern = regexp.MustCompile(`^[a-z0-9:-]+$`)

// IconsHandler serves SVG icons from the icon registry.
type IconsHandler struct {
	registry *icons.Registry
}

// NewIconsHandler creates a new IconsHandler backed by the given registry.
func NewIconsHandler(registry *icons.Registry) *IconsHandler {
	return &IconsHandler{registry: registry}
}

// GetIcon serves the SVG for a given icon slug.
// Returns 200 with image/svg+xml on success, 404 for unknown slugs, 400 for invalid slugs.
func (h *IconsHandler) GetIcon(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !slugPattern.MatchString(slug) {
		http.Error(w, "invalid slug", http.StatusBadRequest)
		return
	}
	svg, ok := h.registry.Get(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(svg))
}
