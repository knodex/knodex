// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knodex/knodex/server/internal/rbac"
)

// fakeTeamService is an in-memory rbac.TeamServiceInterface for handler tests.
type fakeTeamService struct {
	teams      map[string]*rbac.Team
	createErr  error
	listErr    error
	failExists bool // make Create return AlreadyExists
}

func newFakeTeamService() *fakeTeamService {
	return &fakeTeamService{teams: map[string]*rbac.Team{}}
}

func (f *fakeTeamService) CreateTeam(_ context.Context, name string, spec rbac.TeamSpec, _ string) (*rbac.Team, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	// Mirror the real service's error wrapping so the handler's "invalid team"
	// → 400 mapping is exercised faithfully.
	if err := rbac.ValidateTeamName(name); err != nil {
		return nil, fmt.Errorf("invalid team name: %w", err)
	}
	if err := rbac.ValidateTeamSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid team spec: %w", err)
	}
	if f.failExists {
		return nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "teams"}, name)
	}
	t := &rbac.Team{Spec: spec}
	t.Name = name
	f.teams[name] = t
	return t, nil
}

func (f *fakeTeamService) GetTeam(_ context.Context, name string) (*rbac.Team, error) {
	t, ok := f.teams[name]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "teams"}, name)
	}
	return t, nil
}

func (f *fakeTeamService) ListTeams(_ context.Context) (*rbac.TeamList, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	list := &rbac.TeamList{}
	for _, t := range f.teams {
		list.Items = append(list.Items, *t)
	}
	return list, nil
}

func (f *fakeTeamService) UpdateTeam(_ context.Context, team *rbac.Team, _ string) (*rbac.Team, error) {
	if err := rbac.ValidateTeamSpec(team.Spec); err != nil {
		return nil, fmt.Errorf("invalid team spec: %w", err)
	}
	if _, ok := f.teams[team.Name]; !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "teams"}, team.Name)
	}
	f.teams[team.Name] = team
	return team, nil
}

func (f *fakeTeamService) DeleteTeam(_ context.Context, name string) error {
	if _, ok := f.teams[name]; !ok {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "teams"}, name)
	}
	delete(f.teams, name)
	return nil
}

// actionEnforcer is a rbac.Authorizer that grants only the configured actions
// on settings/*, so a test can grant "get" but deny "update" (the non-operator
// read-only case in AC #1).
//
// serveradminSubjects / roleErrSubjects drive the per-user HasRole join the
// Users API uses for the derived application role (Story 17.3). Both default to
// nil, so existing callers (which never set them) see HasRole → (false, nil)
// for every subject, preserving all prior behaviour.
type actionEnforcer struct {
	allow map[string]bool // action → allowed

	// serveradminSubjects: Casbin subjects that effectively hold role:serveradmin.
	serveradminSubjects map[string]bool
	// roleErrSubjects: Casbin subjects whose HasRole lookup returns an error
	// (exercises the AC#3 degrade-to-member-on-error path per user).
	roleErrSubjects map[string]bool
}

func (e *actionEnforcer) CanAccess(_ context.Context, _, _, action string) (bool, error) {
	return e.allow[action], nil
}

func (e *actionEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, action string) (bool, error) {
	return e.allow[action], nil
}

func (e *actionEnforcer) EnforceProjectAccess(_ context.Context, _, _, _ string) error { return nil }

func (e *actionEnforcer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}

func (e *actionEnforcer) HasRole(_ context.Context, user, role string) (bool, error) {
	if e.roleErrSubjects[user] {
		return false, errors.New("transient casbin lookup error")
	}
	return role == rbac.CasbinRoleServerAdmin && e.serveradminSubjects[user], nil
}

func operatorEnforcer() *actionEnforcer {
	return &actionEnforcer{allow: map[string]bool{"get": true, "update": true}}
}
func readOnlyEnforcer() *actionEnforcer { return &actionEnforcer{allow: map[string]bool{"get": true}} }
func deniedEnforcer() *actionEnforcer   { return &actionEnforcer{allow: map[string]bool{}} }

func teamReq(method, target string, body interface{}) *http.Request {
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, target, bytes.NewReader(b))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	return setAdminContext(r)
}

func TestTeamHandler_List_Operator200(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "alpha-devs")
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/teams", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body TeamListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalCount != 1 || body.Items[0].Name != "alpha" {
		t.Errorf("unexpected list: %+v", body)
	}
}

func TestTeamHandler_List_Unauthorized(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), operatorEnforcer())
	w := httptest.NewRecorder()
	// No user context → 401.
	h.ListTeams(w, httptest.NewRequest("GET", "/api/v1/teams", nil))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTeamHandler_Create_Operator201(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.CreateTeam(w, teamReq("POST", "/api/v1/teams", CreateTeamRequest{
		Name: "beta", Description: "b", OIDCGroups: []string{"beta-devs"},
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
	}
	if _, ok := svc.teams["beta"]; !ok {
		t.Error("team not persisted")
	}
}

func TestTeamHandler_Create_ReadOnlyForbidden(t *testing.T) {
	t.Parallel()
	// Granting only get → mutation denied (403) — the AC #1 read-only operator.
	h := NewTeamHandler(newFakeTeamService(), readOnlyEnforcer())
	w := httptest.NewRecorder()
	h.CreateTeam(w, teamReq("POST", "/api/v1/teams", CreateTeamRequest{
		Name: "beta", OIDCGroups: []string{"g"},
	}))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-operator mutation, got %d", w.Code)
	}
}

func TestTeamHandler_List_DeniedForbidden(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), deniedEnforcer())
	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/teams", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestTeamHandler_Create_InvalidName400(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), operatorEnforcer())
	w := httptest.NewRecorder()
	h.CreateTeam(w, teamReq("POST", "/api/v1/teams", CreateTeamRequest{
		Name: "Invalid_Name", OIDCGroups: []string{"g"},
	}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTeamHandler_Create_NoGroups400(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), operatorEnforcer())
	w := httptest.NewRecorder()
	h.CreateTeam(w, teamReq("POST", "/api/v1/teams", CreateTeamRequest{
		Name: "nogroups", OIDCGroups: nil,
	}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty groups, got %d", w.Code)
	}
}

func TestTeamHandler_Update_NotFound404(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), operatorEnforcer())
	w := httptest.NewRecorder()
	req := teamReq("PUT", "/api/v1/teams/ghost", UpdateTeamRequest{OIDCGroups: []string{"g"}})
	req.SetPathValue("name", "ghost")
	h.UpdateTeam(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTeamHandler_Update_Operator200(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "alpha-devs")
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	req := teamReq("PUT", "/api/v1/teams/alpha", UpdateTeamRequest{
		Description: "updated", OIDCGroups: []string{"alpha-devs", "alpha-ops"},
	})
	req.SetPathValue("name", "alpha")
	h.UpdateTeam(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if len(svc.teams["alpha"].Spec.OIDCGroups) != 2 {
		t.Errorf("update not applied: %+v", svc.teams["alpha"].Spec)
	}
}

func TestTeamHandler_Delete_Operator204(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["alpha"] = teamObj("alpha", "alpha-devs")
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	req := teamReq("DELETE", "/api/v1/teams/alpha", nil)
	req.SetPathValue("name", "alpha")
	h.DeleteTeam(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if _, ok := svc.teams["alpha"]; ok {
		t.Error("team not deleted")
	}
}

func TestTeamHandler_Delete_NotFound404(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), operatorEnforcer())
	w := httptest.NewRecorder()
	req := teamReq("DELETE", "/api/v1/teams/ghost", nil)
	req.SetPathValue("name", "ghost")
	h.DeleteTeam(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTeamHandler_Get_NotFound404(t *testing.T) {
	t.Parallel()
	h := NewTeamHandler(newFakeTeamService(), operatorEnforcer())
	w := httptest.NewRecorder()
	req := teamReq("GET", "/api/v1/teams/ghost", nil)
	req.SetPathValue("name", "ghost")
	h.GetTeam(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTeamHandler_Create_Conflict409(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.failExists = true
	h := NewTeamHandler(svc, operatorEnforcer())
	w := httptest.NewRecorder()
	h.CreateTeam(w, teamReq("POST", "/api/v1/teams", CreateTeamRequest{
		Name: "dup", OIDCGroups: []string{"g"},
	}))
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func teamObj(name string, groups ...string) *rbac.Team {
	t := &rbac.Team{Spec: rbac.TeamSpec{OIDCGroups: groups}}
	t.ObjectMeta = metav1.ObjectMeta{Name: name}
	return t
}

// managedTeamObj creates a team carrying the control-plane provenance annotation.
func managedTeamObj(name string, groups ...string) *rbac.Team {
	t := teamObj(name, groups...)
	t.Annotations = map[string]string{
		rbac.TeamAnnotationCreatedBy: rbac.TeamAnnotationCreatedByControlPlane,
	}
	return t
}

func TestTeamHandler_ManagedFlag_TrueWhenControlPlaneAnnotation(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["cp-alpha"] = managedTeamObj("cp-alpha", "kx-team-alpha")
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	req := teamReq("GET", "/api/v1/teams/cp-alpha", nil)
	req.SetPathValue("name", "cp-alpha")
	h.GetTeam(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body TeamResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Managed {
		t.Errorf("expected managed=true for control-plane team, got false")
	}
}

func TestTeamHandler_ManagedFlag_FalseWhenNoAnnotation(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["op-beta"] = teamObj("op-beta", "beta-devs")
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	req := teamReq("GET", "/api/v1/teams/op-beta", nil)
	req.SetPathValue("name", "op-beta")
	h.GetTeam(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body TeamResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Managed {
		t.Errorf("expected managed=false for operator team, got true")
	}
}

func TestTeamHandler_ManagedFlag_InList(t *testing.T) {
	t.Parallel()
	svc := newFakeTeamService()
	svc.teams["cp-team"] = managedTeamObj("cp-team", "kx-team-foo")
	svc.teams["op-team"] = teamObj("op-team", "op-grp")
	h := NewTeamHandler(svc, operatorEnforcer())

	w := httptest.NewRecorder()
	h.ListTeams(w, teamReq("GET", "/api/v1/teams", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body TeamListResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalCount != 2 {
		t.Fatalf("expected 2 items, got %d", body.TotalCount)
	}
	managed := map[string]bool{}
	for _, item := range body.Items {
		managed[item.Name] = item.Managed
	}
	if !managed["cp-team"] {
		t.Errorf("expected cp-team.managed=true in list")
	}
	if managed["op-team"] {
		t.Errorf("expected op-team.managed=false in list")
	}
}
