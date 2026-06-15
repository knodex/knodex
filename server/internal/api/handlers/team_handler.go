// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// TeamResponse is the API representation of a Team CRD.
type TeamResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	OIDCGroups  []string `json:"oidcGroups"`
	// Managed is true iff the team was materialized by the control-plane
	// reconciler (annotation knodex.io/created-by == "control-plane").
	// Always false for operator-authored teams; always false in OSS/EE.
	Managed bool `json:"managed"`
}

// TeamListResponse wraps a list of teams for GET /api/v1/teams.
type TeamListResponse struct {
	Items      []TeamResponse `json:"items"`
	TotalCount int            `json:"totalCount"`
}

// CreateTeamRequest is the body for POST /api/v1/teams.
type CreateTeamRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	OIDCGroups  []string `json:"oidcGroups"`
}

// UpdateTeamRequest is the body for PUT /api/v1/teams/{name}.
// Name is immutable (taken from the path); only description and groups change.
type UpdateTeamRequest struct {
	Description string   `json:"description,omitempty"`
	OIDCGroups  []string `json:"oidcGroups"`
}

func toTeamResponse(t *rbac.Team) TeamResponse {
	groups := t.Spec.OIDCGroups
	if groups == nil {
		groups = []string{}
	}
	return TeamResponse{
		Name:        t.Name,
		Description: t.Spec.Description,
		OIDCGroups:  groups,
		Managed:     t.Annotations[rbac.TeamAnnotationCreatedBy] == rbac.TeamAnnotationCreatedByControlPlane,
	}
}

// TeamHandler serves the operator-gated Teams CRUD API (Story 10.4).
//
// Teams are operator configuration CRUD, NOT an authorization surface: every
// handler is gated with the SAME settings/* get|update Casbin check used by the
// observed-groups (10.3) and rbac-metrics endpoints — no `teams` can-i resource,
// no per-team permission check (NFR-T1, the single enforcement layer). Access
// *granted by* a team is resolved to OIDC groups by the 10.2 effectiveRoleGroups
// path through the one enforcer; this handler never touches that.
//
// Re-resolution after a write is automatic and NOT triggered here: a write
// mutates the cluster Team object, the 10.2 TeamWatcher fires OnChange, and the
// debounced SyncPolicies re-resolves affected projects. The handler adds no
// second re-sync path.
type TeamHandler struct {
	teamService rbac.TeamServiceInterface
	enforcer    rbac.Authorizer
}

// NewTeamHandler creates a new Teams CRUD handler.
func NewTeamHandler(svc rbac.TeamServiceInterface, enforcer rbac.Authorizer) *TeamHandler {
	return &TeamHandler{teamService: svc, enforcer: enforcer}
}

// requireOperator gates a handler with the shared settings/* Casbin check.
// action is "get" for reads, "update" for mutations. Returns the user context
// when access is granted, or nil after writing the 401/403/500 response.
func (h *TeamHandler) requireOperator(w http.ResponseWriter, r *http.Request, action string) *middleware.UserContext {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return nil
	}
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "settings/*", action, r.Header.Get("X-Request-ID")) {
		return nil
	}
	return userCtx
}

// ListTeams handles GET /api/v1/teams.
func (h *TeamHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "get") == nil {
		return
	}

	list, err := h.teamService.ListTeams(r.Context())
	if err != nil {
		response.InternalError(w, "failed to list teams")
		return
	}

	items := make([]TeamResponse, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, toTeamResponse(&list.Items[i]))
	}
	response.WriteJSON(w, http.StatusOK, TeamListResponse{Items: items, TotalCount: len(items)})
}

// GetTeam handles GET /api/v1/teams/{name}.
func (h *TeamHandler) GetTeam(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "get") == nil {
		return
	}

	name := r.PathValue("name")
	team, err := h.teamService.GetTeam(r.Context(), name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			response.NotFound(w, "team", name)
			return
		}
		response.InternalError(w, "failed to get team")
		return
	}
	response.WriteJSON(w, http.StatusOK, toTeamResponse(team))
}

// CreateTeam handles POST /api/v1/teams.
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	userCtx := h.requireOperator(w, r, "update")
	if userCtx == nil {
		return
	}

	req, err := helpers.DecodeJSON[CreateTeamRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	spec := rbac.TeamSpec{Description: req.Description, OIDCGroups: req.OIDCGroups}
	team, err := h.teamService.CreateTeam(r.Context(), req.Name, spec, userCtx.UserID)
	if err != nil {
		h.writeServiceError(w, err, req.Name)
		return
	}
	response.WriteJSON(w, http.StatusCreated, toTeamResponse(team))
}

// UpdateTeam handles PUT /api/v1/teams/{name}.
func (h *TeamHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	userCtx := h.requireOperator(w, r, "update")
	if userCtx == nil {
		return
	}

	name := r.PathValue("name")
	req, err := helpers.DecodeJSON[UpdateTeamRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Read-modify-write so we preserve resourceVersion and other metadata.
	team, err := h.teamService.GetTeam(r.Context(), name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			response.NotFound(w, "team", name)
			return
		}
		response.InternalError(w, "failed to get team")
		return
	}

	team.Spec.Description = req.Description
	team.Spec.OIDCGroups = req.OIDCGroups

	updated, err := h.teamService.UpdateTeam(r.Context(), team, userCtx.UserID)
	if err != nil {
		h.writeServiceError(w, err, name)
		return
	}
	response.WriteJSON(w, http.StatusOK, toTeamResponse(updated))
}

// DeleteTeam handles DELETE /api/v1/teams/{name}.
func (h *TeamHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "update") == nil {
		return
	}

	name := r.PathValue("name")
	if err := h.teamService.DeleteTeam(r.Context(), name); err != nil {
		if apierrors.IsNotFound(err) {
			response.NotFound(w, "team", name)
			return
		}
		response.InternalError(w, "failed to delete team")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeServiceError maps a TeamService error to the standardized response.
// Validation errors (from the 10.1 validators) → 400 with a field map; k8s
// AlreadyExists → 409; everything else → 500.
func (h *TeamHandler) writeServiceError(w http.ResponseWriter, err error, name string) {
	if apierrors.IsAlreadyExists(err) {
		response.Conflict(w, "team", name)
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid team") {
		response.BadRequest(w, "Validation failed", map[string]string{"team": msg})
		return
	}
	response.InternalError(w, "failed to persist team")
}
