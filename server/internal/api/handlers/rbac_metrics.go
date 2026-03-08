// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// RBACMetricsResponse contains RBAC policy and cache metrics
type RBACMetricsResponse struct {
	// Cache performance statistics
	Cache rbac.CacheStats `json:"cache"`

	// Policy enforcement metrics
	Policy rbac.PolicyMetrics `json:"policy"`

	// Cache manager status (if available)
	CacheManager *rbac.PolicyCacheStatus `json:"cache_manager,omitempty"`
}

// RBACMetricsHandler provides metrics for RBAC policy enforcement and caching
type RBACMetricsHandler struct {
	enforcer     rbac.PolicyEnforcer
	cacheManager *rbac.PolicyCacheManager
}

// NewRBACMetricsHandler creates a new RBAC metrics handler
func NewRBACMetricsHandler(enforcer rbac.PolicyEnforcer, cacheManager *rbac.PolicyCacheManager) *RBACMetricsHandler {
	return &RBACMetricsHandler{
		enforcer:     enforcer,
		cacheManager: cacheManager,
	}
}

// GetMetrics handles GET /api/v1/rbac/metrics
// Returns RBAC policy cache and enforcement metrics.
// Requires admin access (settings:get permission).
func (h *RBACMetricsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	// Require authenticated user
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Require admin-level access (settings:get)
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "settings/*", "get", r.Header.Get("X-Request-ID")) {
		return
	}

	resp := RBACMetricsResponse{}

	// Get cache stats from enforcer
	if h.enforcer != nil {
		resp.Cache = h.enforcer.CacheStats()
		resp.Policy = h.enforcer.Metrics()
	}

	// Get cache manager status if available
	if h.cacheManager != nil {
		status := h.cacheManager.Status()
		resp.CacheManager = &status
	}

	response.WriteJSON(w, http.StatusOK, resp)
}
