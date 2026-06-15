// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// IntegrationHandler serves the read-only "future Grafana replicator" contract
// (Story 12.7). It exposes projects, teams (with their Keycloak group
// identifiers), and team→project-role bindings derived purely from cluster CRDs
// — never from the caller's token/claims.
//
// Why a separate handler/DTO set instead of reusing TeamResponse / the project
// DTOs: this is an EXTERNAL integration contract. A future, separate-repo
// replicator pins to this shape and maps `project → Grafana org` and
// `team group → Grafana team`, resolving members from Keycloak directly. Keeping
// the DTOs distinct decouples the contract from UI churn (the UI-facing
// /api/v1/teams + /api/v1/projects may change freely without breaking it).
//
// Authorization reuses the SAME settings/* get Casbin check as /api/v1/teams and
// /api/v1/groups/observed — there is NO new can-i resource (NFR-T1, the single
// enforcement layer). The token is used ONLY for that authorization decision,
// never as a data source: responses are identical regardless of the caller's
// groups/claims (pinned by a token-independence test).
type IntegrationHandler struct {
	teamService    rbac.TeamServiceInterface
	projectService rbac.ProjectServiceInterface
	enforcer       rbac.Authorizer

	// rosterEnricher is an OPTIONAL, best-effort member-roster source injected via
	// SetRosterEnricher. Nil by default — the group identifiers are the canonical
	// join key, so members are strictly enrichment, never required. A fetch
	// failure is non-fatal: the team is still returned with its group identifier
	// intact (AC #3).
	rosterEnricher RosterEnricher
}

// RosterEnricher resolves a team's member roster without any token dependency.
// Optional: left nil unless an enricher is injected. The group identifiers are
// the canonical join key, so members are strictly enrichment — never required.
type RosterEnricher interface {
	// MembersForGroups returns the roster for the team named by groupIdentifiers
	// (the team's spec.oidcGroups, e.g. ["kx-team-<org>-<slug>"]). Returns an
	// error on failure; the caller logs it and continues (non-fatal).
	MembersForGroups(ctx context.Context, groupIdentifiers []string) ([]IntegrationMember, error)
}

// IntegrationTeam is the replicator-facing view of a Team. groupIdentifiers is
// the cross-tool join key (each consumer maps the same group to its own
// resources). members is populated only when a roster enricher is injected.
type IntegrationTeam struct {
	Name             string              `json:"name"`
	Description      string              `json:"description,omitempty"`
	GroupIdentifiers []string            `json:"groupIdentifiers"`
	Members          []IntegrationMember `json:"members,omitempty"`
}

// IntegrationMember is a roster entry (present only when an enricher is injected).
type IntegrationMember struct {
	UserID string `json:"userId,omitempty"`
	Email  string `json:"email,omitempty"`
	Name   string `json:"name,omitempty"`
}

// IntegrationTeamBinding maps a project role to the teams bound to it and the
// group identifiers those teams resolve to — so the replicator gets the join
// key directly, without re-reading the Team list.
type IntegrationTeamBinding struct {
	Role             string   `json:"role"`
	Teams            []string `json:"teams"`
	GroupIdentifiers []string `json:"groupIdentifiers"`
}

// IntegrationProject is the replicator-facing view of a Project. The replicator
// maps each project to a Grafana org; destinations are the project's namespaces.
type IntegrationProject struct {
	Name         string                   `json:"name"`
	Type         string                   `json:"type,omitempty"`
	Description  string                   `json:"description,omitempty"`
	Destinations []string                 `json:"destinations"`
	TeamBindings []IntegrationTeamBinding `json:"teamBindings"`
}

// IntegrationTeamListResponse wraps GET /api/v1/integration/teams.
type IntegrationTeamListResponse struct {
	Items      []IntegrationTeam `json:"items"`
	TotalCount int               `json:"totalCount"`
}

// IntegrationProjectListResponse wraps GET /api/v1/integration/projects.
type IntegrationProjectListResponse struct {
	Items      []IntegrationProject `json:"items"`
	TotalCount int                  `json:"totalCount"`
}

// NewIntegrationHandler builds the read-only replicator-contract handler.
func NewIntegrationHandler(
	teamSvc rbac.TeamServiceInterface,
	projectSvc rbac.ProjectServiceInterface,
	enforcer rbac.Authorizer,
) *IntegrationHandler {
	return &IntegrationHandler{
		teamService:    teamSvc,
		projectService: projectSvc,
		enforcer:       enforcer,
	}
}

// SetRosterEnricher injects the optional member-roster source. Left unset by
// default, so rosterEnricher stays nil and members are omitted.
func (h *IntegrationHandler) SetRosterEnricher(e RosterEnricher) {
	h.rosterEnricher = e
}

// requireOperator gates a read with the shared settings/* get Casbin check —
// the SAME gate used by TeamHandler (NFR-T1, no new can-i resource). The token
// is consulted ONLY here, for authorization; it never shapes the response body.
func (h *IntegrationHandler) requireOperator(w http.ResponseWriter, r *http.Request) *middleware.UserContext {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return nil
	}
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "settings/*", "get", r.Header.Get("X-Request-ID")) {
		return nil
	}
	return userCtx
}

// ListTeams handles GET /api/v1/integration/teams.
func (h *IntegrationHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r) == nil {
		return
	}

	list, err := h.teamService.ListTeams(r.Context())
	if err != nil {
		response.InternalError(w, "failed to list teams")
		return
	}

	items := make([]IntegrationTeam, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, h.toIntegrationTeam(r.Context(), &list.Items[i]))
	}
	response.WriteJSON(w, http.StatusOK, IntegrationTeamListResponse{Items: items, TotalCount: len(items)})
}

// toIntegrationTeam maps a Team CRD to the replicator DTO, optionally enriching
// members when an enricher is injected. Slices are nil-safe (never emit null).
func (h *IntegrationHandler) toIntegrationTeam(ctx context.Context, t *rbac.Team) IntegrationTeam {
	groups := t.Spec.OIDCGroups
	if groups == nil {
		groups = []string{}
	}
	team := IntegrationTeam{
		Name:             t.Name,
		Description:      t.Spec.Description,
		GroupIdentifiers: groups,
	}
	if h.rosterEnricher != nil && len(groups) > 0 {
		members, err := h.rosterEnricher.MembersForGroups(ctx, groups)
		if err != nil {
			// Non-fatal: the group identifier is the contract; members are extra.
			slog.Debug("integration: roster enrichment failed; returning team without members",
				"team", t.Name, "error", err)
		} else if len(members) > 0 {
			team.Members = members
		}
	}
	return team
}

// ListProjects handles GET /api/v1/integration/projects.
func (h *IntegrationHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r) == nil {
		return
	}

	list, err := h.projectService.ListProjects(r.Context())
	if err != nil {
		response.InternalError(w, "failed to list projects")
		return
	}

	// Build a team-name → group-identifiers lookup once so each binding resolves
	// its join key directly. A team listing failure is non-fatal: bindings then
	// carry empty group identifiers (still names the team).
	groupsByTeam := h.teamGroupIndex(r.Context())

	items := make([]IntegrationProject, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, toIntegrationProject(&list.Items[i], groupsByTeam))
	}
	response.WriteJSON(w, http.StatusOK, IntegrationProjectListResponse{Items: items, TotalCount: len(items)})
}

// teamGroupIndex returns a map of team name → spec.oidcGroups, used to resolve
// binding join keys. Returns an empty (non-nil) map if the team list cannot be
// read, so binding resolution degrades to empty group identifiers, not a 500.
func (h *IntegrationHandler) teamGroupIndex(ctx context.Context) map[string][]string {
	index := map[string][]string{}
	list, err := h.teamService.ListTeams(ctx)
	if err != nil {
		slog.Debug("integration: team index unavailable; bindings will omit group identifiers", "error", err)
		return index
	}
	for i := range list.Items {
		index[list.Items[i].Name] = list.Items[i].Spec.OIDCGroups
	}
	return index
}

// toIntegrationProject maps a Project CRD to the replicator DTO, deriving team
// bindings from spec.roles[].teams[] and resolving each bound team's group
// identifiers via groupsByTeam. All slices are nil-safe.
func toIntegrationProject(p *rbac.Project, groupsByTeam map[string][]string) IntegrationProject {
	destinations := make([]string, 0, len(p.Spec.Destinations))
	for _, d := range p.Spec.Destinations {
		if d.Namespace != "" {
			destinations = append(destinations, d.Namespace)
		}
	}

	bindings := make([]IntegrationTeamBinding, 0)
	for _, role := range p.Spec.Roles {
		if len(role.Teams) == 0 {
			continue
		}
		teams := make([]string, 0, len(role.Teams))
		groupIDs := make([]string, 0)
		seen := map[string]bool{}
		for _, teamName := range role.Teams {
			teams = append(teams, teamName)
			for _, g := range groupsByTeam[teamName] {
				if g != "" && !seen[g] {
					seen[g] = true
					groupIDs = append(groupIDs, g)
				}
			}
		}
		bindings = append(bindings, IntegrationTeamBinding{
			Role:             role.Name,
			Teams:            teams,
			GroupIdentifiers: groupIDs,
		})
	}

	return IntegrationProject{
		Name:         p.Name,
		Type:         string(p.Spec.Type),
		Description:  p.Spec.Description,
		Destinations: destinations,
		TeamBindings: bindings,
	}
}
