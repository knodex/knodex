// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/services"
)

const relatedRGDsAnnotation = "knodex.io/related-rgds"

// RelatedRGD represents a related RGD in the response.
type RelatedRGD struct {
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	Description  string `json:"description"`
	Relationship string `json:"relationship"`
}

// RelatedRGDsResponse is the response for GET /api/v1/rgds/{name}/related.
type RelatedRGDsResponse struct {
	Related []RelatedRGD `json:"related"`
}

// RelatedRGDsHandler handles GET /api/v1/rgds/{name}/related.
type RelatedRGDsHandler struct {
	authService    *services.AuthorizationService
	catalogService *services.CatalogService
	logger         *slog.Logger
}

// NewRelatedRGDsHandler creates a new RelatedRGDsHandler.
func NewRelatedRGDsHandler(authService *services.AuthorizationService, catalogService *services.CatalogService, logger *slog.Logger) *RelatedRGDsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &RelatedRGDsHandler{
		authService:    authService,
		catalogService: catalogService,
		logger:         logger.With("component", "related-rgds-handler"),
	}
}

// GetRelated handles GET /api/v1/rgds/{name}/related.
func (h *RelatedRGDsHandler) GetRelated(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "RGD name is required", nil)
		return
	}

	if h.catalogService == nil {
		response.WriteJSON(w, http.StatusOK, RelatedRGDsResponse{Related: []RelatedRGD{}})
		return
	}

	// Get auth context for project-scoped filtering
	var authCtx *services.UserAuthContext
	if h.authService != nil {
		var err error
		authCtx, err = h.authService.GetUserAuthContext(r.Context(), userCtx)
		if err != nil {
			h.logger.Error("failed to get auth context", "error", err)
			response.InternalError(w, "Failed to get authorization context")
			return
		}
	}

	// Fetch the source RGD
	rgd, err := h.catalogService.GetRGD(r.Context(), authCtx, name, "")
	if err != nil {
		if err == services.ErrNotFound {
			response.NotFound(w, "rgd", name)
			return
		}
		h.logger.Error("failed to get RGD", "name", name, "error", err)
		response.InternalError(w, "Failed to get RGD")
		return
	}

	// Read related-rgds annotation
	annotation := rgd.Labels[relatedRGDsAnnotation]
	if annotation == "" {
		response.WriteJSON(w, http.StatusOK, RelatedRGDsResponse{Related: []RelatedRGD{}})
		return
	}

	// Parse comma-separated names and fetch each
	relatedNames := strings.Split(annotation, ",")
	related := make([]RelatedRGD, 0, len(relatedNames))
	for _, rn := range relatedNames {
		rn = strings.TrimSpace(rn)
		if rn == "" || rn == name {
			continue
		}

		relRGD, err := h.catalogService.GetRGD(r.Context(), authCtx, rn, "")
		if err != nil {
			continue // Skip inaccessible or missing RGDs
		}

		related = append(related, RelatedRGD{
			Name:         relRGD.Name,
			DisplayName:  relRGD.Title,
			Description:  relRGD.Description,
			Relationship: "commonly-deployed-together",
		})
	}

	response.WriteJSON(w, http.StatusOK, RelatedRGDsResponse{Related: related})
}
