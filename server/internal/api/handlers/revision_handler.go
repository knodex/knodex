// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kro/diff"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// RevisionHandlerConfig holds configuration for the revision handler.
type RevisionHandlerConfig struct {
	Provider       services.GraphRevisionProvider
	PolicyEnforcer rbac.Authorizer
	DiffService    *diff.DiffService
	Logger         *slog.Logger
}

// RevisionHandler handles GraphRevision HTTP requests.
type RevisionHandler struct {
	provider       services.GraphRevisionProvider
	policyEnforcer rbac.Authorizer
	diffService    *diff.DiffService
	logger         *slog.Logger
}

// NewRevisionHandler creates a new RevisionHandler.
func NewRevisionHandler(config RevisionHandlerConfig) *RevisionHandler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &RevisionHandler{
		provider:       config.Provider,
		policyEnforcer: config.PolicyEnforcer,
		diffService:    config.DiffService,
		logger:         logger.With("handler", "revision"),
	}
}

// ListRevisions handles GET /api/v1/rgds/{name}/revisions
func (h *RevisionHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil {
		response.ServiceUnavailable(w, "GraphRevision watcher not available")
		return
	}

	rgdName := r.PathValue("name")
	if rgdName == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	requestID := r.Header.Get("X-Request-ID")
	if !helpers.RequireAccess(w, r.Context(), h.policyEnforcer, userCtx, "rgds/"+rgdName, "get", requestID) {
		return
	}

	list := h.provider.ListRevisions(rgdName)
	response.WriteJSON(w, http.StatusOK, list)
}

// DiffRevisions handles GET /api/v1/rgds/{name}/revisions/{rev1}/diff/{rev2}
func (h *RevisionHandler) DiffRevisions(w http.ResponseWriter, r *http.Request) {
	if h.diffService == nil {
		response.ServiceUnavailable(w, "revision diff not available")
		return
	}
	if h.provider == nil {
		response.ServiceUnavailable(w, "GraphRevision watcher not available")
		return
	}

	rgdName := r.PathValue("name")
	if rgdName == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}
	if !sanitize.IsValidDNS1123Subdomain(rgdName) {
		response.BadRequest(w, "invalid RGD name", map[string]string{"name": "must be a valid DNS-1123 subdomain"})
		return
	}

	rev1Str := r.PathValue("rev1")
	rev1, err := strconv.Atoi(rev1Str)
	if err != nil {
		response.BadRequest(w, "invalid revision number", map[string]string{"rev1": rev1Str})
		return
	}

	rev2Str := r.PathValue("rev2")
	rev2, err := strconv.Atoi(rev2Str)
	if err != nil {
		response.BadRequest(w, "invalid revision number", map[string]string{"rev2": rev2Str})
		return
	}

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	requestID := r.Header.Get("X-Request-ID")
	if !helpers.RequireAccess(w, r.Context(), h.policyEnforcer, userCtx, "rgds/"+rgdName, "get", requestID) {
		return
	}

	d, err := h.diffService.GetDiff(h.provider, rgdName, rev1, rev2)
	if err != nil {
		response.NotFound(w, "revision diff", rgdName)
		return
	}

	response.WriteJSON(w, http.StatusOK, d)
}

// GetRevision handles GET /api/v1/rgds/{name}/revisions/{revision}
func (h *RevisionHandler) GetRevision(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil {
		response.ServiceUnavailable(w, "GraphRevision watcher not available")
		return
	}

	rgdName := r.PathValue("name")
	if rgdName == "" {
		response.BadRequest(w, "name is required", map[string]string{"name": "path parameter is required"})
		return
	}

	revisionStr := r.PathValue("revision")
	revisionNum, err := strconv.Atoi(revisionStr)
	if err != nil {
		response.BadRequest(w, "invalid revision number", map[string]string{"revision": "must be an integer"})
		return
	}

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	requestID := r.Header.Get("X-Request-ID")
	if !helpers.RequireAccess(w, r.Context(), h.policyEnforcer, userCtx, "rgds/"+rgdName, "get", requestID) {
		return
	}

	rev, found := h.provider.GetRevision(rgdName, revisionNum)
	if !found {
		response.NotFound(w, "GraphRevision", rgdName+"/"+revisionStr)
		return
	}

	response.WriteJSON(w, http.StatusOK, rev)
}
