// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package userprefs

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
)

// Handler handles user preference HTTP endpoints.
type Handler struct {
	store  Store
	logger *slog.Logger
}

// NewHandler creates a new preferences handler.
func NewHandler(store Store, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		store:  store,
		logger: logger.With("component", "userprefs-handler"),
	}
}

// GetPreferences handles GET /api/v1/users/preferences.
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	prefs, err := h.store.Get(r.Context(), userCtx.UserID)
	if err != nil {
		h.logger.Error("failed to get user preferences", "userId", userCtx.UserID, "error", err)
		response.InternalError(w, "Failed to get preferences")
		return
	}

	response.WriteJSON(w, http.StatusOK, prefs)
}

// PutPreferences handles PUT /api/v1/users/preferences.
func (h *Handler) PutPreferences(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	var prefs UserPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		response.BadRequest(w, "invalid request body", nil)
		return
	}

	// Ensure non-nil slices
	if prefs.FavoriteRgds == nil {
		prefs.FavoriteRgds = []string{}
	}
	if prefs.RecentRgds == nil {
		prefs.RecentRgds = []string{}
	}

	if err := h.store.Put(r.Context(), userCtx.UserID, &prefs); err != nil {
		h.logger.Error("failed to put user preferences", "userId", userCtx.UserID, "error", err)
		response.InternalError(w, "Failed to save preferences")
		return
	}

	// Return the stored (potentially truncated) preferences
	stored, err := h.store.Get(r.Context(), userCtx.UserID)
	if err != nil {
		h.logger.Error("failed to read back preferences", "userId", userCtx.UserID, "error", err)
		response.InternalError(w, "Failed to read back preferences")
		return
	}

	response.WriteJSON(w, http.StatusOK, stored)
}
