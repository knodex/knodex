// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
)

// fakeRosterEnricher is an in-package RosterEnricher for handler tests,
// exercising the optional member-enrichment wiring (AC #3).
type fakeRosterEnricher struct {
	members []IntegrationMember
	err     error
	calls   int
}

func (f *fakeRosterEnricher) MembersForGroups(_ context.Context, _ []string) ([]IntegrationMember, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.members, nil
}

// reqWithGroups builds an authenticated request carrying the given token groups,
// to pin that the integration responses never depend on the caller's claims.
func reqWithGroups(method, target string, groups []string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	userCtx := &middleware.UserContext{UserID: "user-x", Email: "x@local", Groups: groups}
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, userCtx)
	return r.WithContext(ctx)
}

func projectWithRoleTeams(name, ptype, desc string, namespaces []string, role string, teams []string) *rbac.Project {
	dests := make([]rbac.Destination, 0, len(namespaces))
	for _, ns := range namespaces {
		dests = append(dests, rbac.Destination{Namespace: ns})
	}
	p := &rbac.Project{
		Spec: rbac.ProjectSpec{
			Type:         rbac.ProjectType(ptype),
			Description:  desc,
			Destinations: dests,
			Roles: []rbac.ProjectRole{
				{Name: role, Teams: teams},
			},
		},
	}
	p.ObjectMeta = metav1.ObjectMeta{Name: name}
	return p
}

func newIntegrationHandler(teams *fakeTeamService, projects *mockProjectService, enf *actionEnforcer) *IntegrationHandler {
	return NewIntegrationHandler(teams, projects, enf)
}

// --- Operator gate (AC #4) ---

func TestIntegrationHandler_Teams_Unauthorized(t *testing.T) {
	t.Parallel()
	h := newIntegrationHandler(newFakeTeamService(), newMockProjectService(), operatorEnforcer())
	w := httptest.NewRecorder()
	h.ListTeams(w, httptest.NewRequest("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegrationHandler_Teams_Forbidden(t *testing.T) {
	t.Parallel()
	h := newIntegrationHandler(newFakeTeamService(), newMockProjectService(), deniedEnforcer())
	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestIntegrationHandler_Projects_Unauthorized(t *testing.T) {
	t.Parallel()
	h := newIntegrationHandler(newFakeTeamService(), newMockProjectService(), operatorEnforcer())
	w := httptest.NewRecorder()
	h.ListProjects(w, httptest.NewRequest("GET", "/api/v1/integration/projects", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestIntegrationHandler_Projects_Forbidden(t *testing.T) {
	t.Parallel()
	h := newIntegrationHandler(newFakeTeamService(), newMockProjectService(), deniedEnforcer())
	w := httptest.NewRecorder()
	h.ListProjects(w, teamReq("GET", "/api/v1/integration/projects", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// Read-only operators (settings/* get but not update) can still read the
// contract — every endpoint is GET-only and gated on "get".
func TestIntegrationHandler_Teams_ReadOnlyOperatorAllowed(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "kx-team-acme-alpha")
	h := newIntegrationHandler(svc, newMockProjectService(), readOnlyEnforcer())
	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for read-only operator, got %d", w.Code)
	}
}

// --- Teams response shape (AC #1) ---

func TestIntegrationHandler_Teams_ShapeAndGroupIdentifiers(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "kx-team-acme-alpha", "extra-group")
	h := newIntegrationHandler(svc, newMockProjectService(), operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body IntegrationTeamListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalCount != 1 || body.Items[0].Name != "alpha" {
		t.Fatalf("unexpected list: %+v", body)
	}
	got := body.Items[0].GroupIdentifiers
	if len(got) != 2 || got[0] != "kx-team-acme-alpha" {
		t.Errorf("expected group identifiers [kx-team-acme-alpha extra-group], got %v", got)
	}
	if body.Items[0].Members != nil {
		t.Errorf("OSS (no enricher) must omit members, got %v", body.Items[0].Members)
	}
}

// A Team with no groups must emit "groupIdentifiers": [] (never null).
func TestIntegrationHandler_Teams_NilSafeSlices(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["empty"] = teamObj("empty") // variadic with no args → nil OIDCGroups
	h := newIntegrationHandler(svc, newMockProjectService(), operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"groupIdentifiers":[]`) {
		t.Errorf("expected empty (non-null) groupIdentifiers, got %s", w.Body.String())
	}
}

// --- Projects + team→role bindings (AC #1) ---

func TestIntegrationHandler_Projects_DerivesBindingsAndGroupIdentifiers(t *testing.T) {
	t.Parallel()
	teams := newFakeTeamService()
	teams.teams["alpha"] = teamObj("alpha", "kx-team-acme-alpha")
	teams.teams["beta"] = teamObj("beta", "kx-team-acme-beta")

	projects := newMockProjectService()
	projects.projects["proj1"] = projectWithRoleTeams(
		"proj1", "app", "first project",
		[]string{"acme-apps", "acme-shared"},
		"developer", []string{"alpha", "beta"},
	)

	h := newIntegrationHandler(teams, projects, operatorEnforcer())
	w := httptest.NewRecorder()
	h.ListProjects(w, teamReq("GET", "/api/v1/integration/projects", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body IntegrationProjectListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalCount != 1 {
		t.Fatalf("expected 1 project, got %d", body.TotalCount)
	}
	p := body.Items[0]
	if p.Name != "proj1" || p.Type != "app" || p.Description != "first project" {
		t.Errorf("unexpected project meta: %+v", p)
	}
	if len(p.Destinations) != 2 || p.Destinations[0] != "acme-apps" {
		t.Errorf("unexpected destinations: %v", p.Destinations)
	}
	if len(p.TeamBindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(p.TeamBindings))
	}
	b := p.TeamBindings[0]
	if b.Role != "developer" {
		t.Errorf("expected role developer, got %s", b.Role)
	}
	if len(b.Teams) != 2 || b.Teams[0] != "alpha" {
		t.Errorf("unexpected teams: %v", b.Teams)
	}
	if len(b.GroupIdentifiers) != 2 ||
		b.GroupIdentifiers[0] != "kx-team-acme-alpha" ||
		b.GroupIdentifiers[1] != "kx-team-acme-beta" {
		t.Errorf("unexpected resolved group identifiers: %v", b.GroupIdentifiers)
	}
}

// A team named in a binding but with no matching Team CRD must be non-fatal:
// the binding is returned with empty group identifiers (AC #1 / Task 3).
func TestIntegrationHandler_Projects_MissingTeamBindingNonFatal(t *testing.T) {
	t.Parallel()
	teams := newFakeTeamService() // no teams registered
	projects := newMockProjectService()
	projects.projects["proj1"] = projectWithRoleTeams(
		"proj1", "app", "", nil, "viewer", []string{"ghost-team"},
	)

	h := newIntegrationHandler(teams, projects, operatorEnforcer())
	w := httptest.NewRecorder()
	h.ListProjects(w, teamReq("GET", "/api/v1/integration/projects", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body IntegrationProjectListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	b := body.Items[0].TeamBindings[0]
	if b.Role != "viewer" || len(b.Teams) != 1 || b.Teams[0] != "ghost-team" {
		t.Errorf("unexpected binding: %+v", b)
	}
	if b.GroupIdentifiers == nil {
		t.Error("expected non-null groupIdentifiers for unresolved team")
	}
	if len(b.GroupIdentifiers) != 0 {
		t.Errorf("expected empty group identifiers for unresolved team, got %v", b.GroupIdentifiers)
	}
}

// --- Token independence (AC #2) ---

// The endpoints must produce byte-identical output regardless of the caller's
// token groups. The token authorizes; it is NEVER a data source.
func TestIntegrationHandler_TokenIndependence(t *testing.T) {
	t.Parallel()
	teams := newFakeTeamService()
	teams.teams["alpha"] = teamObj("alpha", "kx-team-acme-alpha")
	projects := newMockProjectService()
	projects.projects["proj1"] = projectWithRoleTeams(
		"proj1", "app", "", []string{"acme-apps"}, "developer", []string{"alpha"},
	)

	cases := [][]string{
		nil,
		{},
		{"kx-team-acme-alpha"},
		{"some-unrelated-group", "another"},
	}

	var teamBodies, projectBodies []string
	for _, groups := range cases {
		h := newIntegrationHandler(teams, projects, operatorEnforcer())

		wt := httptest.NewRecorder()
		h.ListTeams(wt, reqWithGroups("GET", "/api/v1/integration/teams", groups))
		if wt.Code != http.StatusOK {
			t.Fatalf("teams: expected 200 for groups %v, got %d", groups, wt.Code)
		}
		teamBodies = append(teamBodies, wt.Body.String())

		wp := httptest.NewRecorder()
		h.ListProjects(wp, reqWithGroups("GET", "/api/v1/integration/projects", groups))
		if wp.Code != http.StatusOK {
			t.Fatalf("projects: expected 200 for groups %v, got %d", groups, wp.Code)
		}
		projectBodies = append(projectBodies, wp.Body.String())
	}

	for i := 1; i < len(teamBodies); i++ {
		if teamBodies[i] != teamBodies[0] {
			t.Errorf("teams output differs by token groups:\n %q\nvs\n %q", teamBodies[0], teamBodies[i])
		}
		if projectBodies[i] != projectBodies[0] {
			t.Errorf("projects output differs by token groups:\n %q\nvs\n %q", projectBodies[0], projectBodies[i])
		}
	}
}

// --- Roster enrichment wiring (AC #3) ---

func TestIntegrationHandler_Teams_RosterEnrichment(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "kx-team-acme-alpha")
	enricher := &fakeRosterEnricher{members: []IntegrationMember{
		{UserID: "u1", Email: "a@acme.io", Name: "Ada"},
	}}
	h := newIntegrationHandler(svc, newMockProjectService(), operatorEnforcer())
	h.SetRosterEnricher(enricher)

	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body IntegrationTeamListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if enricher.calls != 1 {
		t.Errorf("expected enricher called once, got %d", enricher.calls)
	}
	if len(body.Items[0].Members) != 1 || body.Items[0].Members[0].Email != "a@acme.io" {
		t.Errorf("expected enriched members, got %+v", body.Items[0].Members)
	}
}

// A roster fetch failure is non-fatal: the team is still returned with its
// group identifier; members is simply absent (AC #3).
func TestIntegrationHandler_Teams_RosterEnrichmentFailureNonFatal(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "kx-team-acme-alpha")
	enricher := &fakeRosterEnricher{err: errors.New("control plane unavailable")}
	h := newIntegrationHandler(svc, newMockProjectService(), operatorEnforcer())
	h.SetRosterEnricher(enricher)

	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/integration/teams", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 despite roster failure, got %d", w.Code)
	}
	var body IntegrationTeamListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items[0].GroupIdentifiers) != 1 || body.Items[0].GroupIdentifiers[0] != "kx-team-acme-alpha" {
		t.Errorf("group identifier must survive roster failure, got %v", body.Items[0].GroupIdentifiers)
	}
	if body.Items[0].Members != nil {
		t.Errorf("expected no members on roster failure, got %v", body.Items[0].Members)
	}
}
