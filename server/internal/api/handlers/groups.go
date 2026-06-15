// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/groups"
	"github.com/knodex/knodex/server/internal/rbac"
)

// ObservedGroupsLister is the tiny, List-only dependency the GroupsHandler
// needs. Declared in this package (not the concrete *groups.RedisStore) so the
// handler stays unit-testable with a fake and the router can inject it via
// RouterConfig.
type ObservedGroupsLister interface {
	List(ctx context.Context) ([]groups.ObservedGroup, error)
}

// ObservedGroupsResponse is the typeahead-friendly response shape for
// GET /api/v1/groups/observed.
type ObservedGroupsResponse struct {
	Groups []groups.ObservedGroup `json:"groups"`
}

// GroupsHandler serves the operator-gated observed-groups discovery endpoint
// (Story 10.3). It records nothing; recording happens passively at the login
// choke point in auth.Service.GenerateTokenWithGroups.
type GroupsHandler struct {
	enforcer rbac.PolicyEnforcer
	store    ObservedGroupsLister
}

// NewGroupsHandler creates a new observed-groups handler. The store may be nil
// (e.g. Redis unavailable), in which case the endpoint returns an empty list.
func NewGroupsHandler(enforcer rbac.PolicyEnforcer, store ObservedGroupsLister) *GroupsHandler {
	return &GroupsHandler{enforcer: enforcer, store: store}
}

// ListObserved handles GET /api/v1/groups/observed.
// Returns the distinct OIDC group strings observed at login, most-recently-seen
// first, for use as a typeahead in the team/role editor.
// Requires operator access — the same settings/* get gate as GET /api/v1/rbac/metrics.
func (h *GroupsHandler) ListObserved(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "settings/*", "get", r.Header.Get("X-Request-ID")) {
		return
	}

	// Graceful degradation: with no store (Redis unavailable) the typeahead just
	// gets no suggestions — return 200 + empty list, not a 500.
	if h.store == nil {
		response.WriteJSON(w, http.StatusOK, ObservedGroupsResponse{Groups: []groups.ObservedGroup{}})
		return
	}

	items, err := h.store.List(r.Context())
	if err != nil {
		response.InternalError(w, "failed to list observed groups")
		return
	}
	if items == nil {
		items = []groups.ObservedGroup{}
	}

	response.WriteJSON(w, http.StatusOK, ObservedGroupsResponse{Groups: items})
}
