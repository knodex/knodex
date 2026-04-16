// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/sso"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// mockAuditRecorder captures audit events for test assertions.
type mockAuditRecorder struct {
	events []audit.Event
}

func (m *mockAuditRecorder) Record(_ context.Context, event audit.Event) {
	m.events = append(m.events, event)
}

func (m *mockAuditRecorder) lastEvent() audit.Event {
	if len(m.events) == 0 {
		return audit.Event{}
	}
	return m.events[len(m.events)-1]
}

// --- ProjectHandler audit tests ---

func TestProjectHandler_CreateProject_AuditEvent(t *testing.T) {
	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	reqBody := CreateProjectRequest{
		Name:        "audit-project",
		Description: "Testing audit",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		Email:  "admin@test.local",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "create" {
		t.Errorf("expected action 'create', got %q", e.Action)
	}
	if e.Resource != "projects" {
		t.Errorf("expected resource 'projects', got %q", e.Resource)
	}
	if e.Name != "audit-project" {
		t.Errorf("expected name 'audit-project', got %q", e.Name)
	}
	if e.UserID != "admin-user" {
		t.Errorf("expected userID 'admin-user', got %q", e.UserID)
	}
	if e.UserEmail != "admin@test.local" {
		t.Errorf("expected email 'admin@test.local', got %q", e.UserEmail)
	}
	if e.Project != "audit-project" {
		t.Errorf("expected project 'audit-project', got %q", e.Project)
	}
	if e.Result != "success" {
		t.Errorf("expected result 'success', got %q", e.Result)
	}
	if e.Details["description"] != "Testing audit" {
		t.Errorf("expected description in details, got %v", e.Details)
	}
}

func TestProjectHandler_CreateProject_NoAuditOnFailure(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("existing-project", rbac.ProjectSpec{})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	reqBody := CreateProjectRequest{Name: "existing-project"}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if len(recorder.events) != 0 {
		t.Errorf("expected no audit events on failure, got %d", len(recorder.events))
	}
}

func TestProjectHandler_UpdateProject_AuditEvent(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{Description: "old"})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	reqBody := UpdateProjectRequest{Description: "new description", ResourceVersion: "1"}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "update" {
		t.Errorf("expected action 'update', got %q", e.Action)
	}
	if e.Resource != "projects" {
		t.Errorf("expected resource 'projects', got %q", e.Resource)
	}
	if e.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", e.Name)
	}
	if e.Project != "my-project" {
		t.Errorf("expected project 'my-project', got %q", e.Project)
	}
	// Verify change tracking: description changed from "old" → "new description"
	changes, ok := e.Details["changes"].(map[string]any)
	if !ok {
		t.Fatalf("expected changes map in details, got %v", e.Details)
	}
	descChange, ok := changes["description"].(map[string]any)
	if !ok {
		t.Fatalf("expected description change in changes, got %v", changes)
	}
	if descChange["old"] != "old" {
		t.Errorf("expected old description 'old', got %v", descChange["old"])
	}
	if descChange["new"] != "new description" {
		t.Errorf("expected new description 'new description', got %v", descChange["new"])
	}
}

func TestProjectHandler_UpdateProject_NoAuditOnNotFound(t *testing.T) {
	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	reqBody := UpdateProjectRequest{Description: "new", ResourceVersion: "1"}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/nonexistent", body, userCtx)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if len(recorder.events) != 0 {
		t.Errorf("expected no audit events on failure, got %d", len(recorder.events))
	}
}

func TestProjectHandler_DeleteProject_AuditEvent(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("doomed-project", rbac.ProjectSpec{})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/doomed-project", nil, userCtx)
	req.SetPathValue("name", "doomed-project")
	rec := httptest.NewRecorder()

	handler.DeleteProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "delete" {
		t.Errorf("expected action 'delete', got %q", e.Action)
	}
	if e.Name != "doomed-project" {
		t.Errorf("expected name 'doomed-project', got %q", e.Name)
	}
	if e.Project != "doomed-project" {
		t.Errorf("expected project 'doomed-project', got %q", e.Project)
	}
}

func TestProjectHandler_DeleteProject_NoAuditOnNotFound(t *testing.T) {
	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/nonexistent", nil, userCtx)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.DeleteProject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if len(recorder.events) != 0 {
		t.Errorf("expected no audit events on failure, got %d", len(recorder.events))
	}
}

// --- RoleBindingHandler audit tests ---

func TestRoleBindingHandler_AssignUserRole_AuditMemberAdd(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("team-alpha", rbac.ProjectSpec{
		Roles: []rbac.ProjectRole{{Name: "developer"}},
	})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewRoleBindingHandler(svc, enforcer, recorder)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/team-alpha/roles/developer/users/dev-user", nil, userCtx)
	req.SetPathValue("name", "team-alpha")
	req.SetPathValue("role", "developer")
	req.SetPathValue("user", "dev-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "member_add" {
		t.Errorf("expected action 'member_add', got %q", e.Action)
	}
	if e.Resource != "projects" {
		t.Errorf("expected resource 'projects', got %q", e.Resource)
	}
	if e.Project != "team-alpha" {
		t.Errorf("expected project 'team-alpha', got %q", e.Project)
	}
	if e.Details["targetUser"] != "dev-user" {
		t.Errorf("expected targetUser 'dev-user' in details, got %v", e.Details["targetUser"])
	}
	if e.Details["role"] != "developer" {
		t.Errorf("expected role 'developer' in details, got %v", e.Details["role"])
	}
}

func TestRoleBindingHandler_AssignUserRole_AuditRoleChange(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("team-alpha", rbac.ProjectSpec{
		Roles: []rbac.ProjectRole{
			{Name: "viewer"},
			{Name: "developer"},
		},
	})
	// Enforcer returns existing role "viewer" for the user
	enforcer := &mockPolicyEnforcer{
		canAccessResult:    true,
		getUserRolesResult: []string{"proj:team-alpha:viewer"},
	}
	recorder := &mockAuditRecorder{}
	handler := NewRoleBindingHandler(svc, enforcer, recorder)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/team-alpha/roles/developer/users/dev-user", nil, userCtx)
	req.SetPathValue("name", "team-alpha")
	req.SetPathValue("role", "developer")
	req.SetPathValue("user", "dev-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "role_change" {
		t.Errorf("expected action 'role_change', got %q", e.Action)
	}
	if e.Details["previousRole"] != "viewer" {
		t.Errorf("expected previousRole 'viewer' in details, got %v", e.Details["previousRole"])
	}
	if e.Details["role"] != "developer" {
		t.Errorf("expected role 'developer' in details, got %v", e.Details["role"])
	}
}

func TestRoleBindingHandler_AssignUserRole_NoAuditOnInvalidRole(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("team-alpha", rbac.ProjectSpec{
		Roles: []rbac.ProjectRole{{Name: "developer"}},
	})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewRoleBindingHandler(svc, enforcer, recorder)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/team-alpha/roles/nonexistent-role/users/dev-user", nil, userCtx)
	req.SetPathValue("name", "team-alpha")
	req.SetPathValue("role", "nonexistent-role")
	req.SetPathValue("user", "dev-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if len(recorder.events) != 0 {
		t.Errorf("expected no audit events on failure, got %d", len(recorder.events))
	}
}

// --- SSOSettingsHandler audit tests ---

func TestSSOSettingsHandler_CreateProvider_AuditEvent(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	recorder := &mockAuditRecorder{}
	handler := NewSSOSettingsHandler(store, recorder, &mockSSOAccessChecker{allowed: true})

	provider := sso.SSOProvider{
		Name:         "test-oidc",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       []string{"openid", "profile"},
	}

	req := ssoRequest(http.MethodPost, "/api/v1/settings/sso/providers", provider)
	rec := httptest.NewRecorder()

	handler.CreateProvider(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "create" {
		t.Errorf("expected action 'create', got %q", e.Action)
	}
	if e.Resource != "settings" {
		t.Errorf("expected resource 'settings', got %q", e.Resource)
	}
	if e.Name != "test-oidc" {
		t.Errorf("expected name 'test-oidc', got %q", e.Name)
	}
	if e.Details["settingsType"] != "sso_provider" {
		t.Errorf("expected settingsType 'sso_provider' in details, got %v", e.Details["settingsType"])
	}
}

func TestSSOSettingsHandler_DeleteProvider_AuditEvent(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	recorder := &mockAuditRecorder{}
	handler := NewSSOSettingsHandler(store, recorder, &mockSSOAccessChecker{allowed: true})

	// Create a provider first
	ctx := context.Background()
	if err := store.Create(ctx, sso.SSOProvider{
		Name: "to-delete", IssuerURL: "https://example.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest(http.MethodDelete, "/api/v1/settings/sso/providers/to-delete", nil)
	req.SetPathValue("name", "to-delete")
	rec := httptest.NewRecorder()

	handler.DeleteProvider(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "delete" {
		t.Errorf("expected action 'delete', got %q", e.Action)
	}
	if e.Name != "to-delete" {
		t.Errorf("expected name 'to-delete', got %q", e.Name)
	}
}

func TestSSOSettingsHandler_DeleteProvider_NoAuditOnNotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	recorder := &mockAuditRecorder{}
	handler := NewSSOSettingsHandler(store, recorder, &mockSSOAccessChecker{allowed: true})

	// Initialize the ConfigMap by creating and keeping a provider so the store exists
	ctx := context.Background()
	if err := store.Create(ctx, sso.SSOProvider{
		Name: "keeper", IssuerURL: "https://accounts.google.com",
		ClientID: "id", ClientSecret: "secret",
		RedirectURL: "https://app.example.com/cb", Scopes: []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	req := ssoRequest(http.MethodDelete, "/api/v1/settings/sso/providers/nonexistent", nil)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.DeleteProvider(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 0 {
		t.Errorf("expected no audit events on failure, got %d", len(recorder.events))
	}
}

// --- ProjectHandler audit detail tests (Task 4.4) ---

func TestProjectHandler_CreateProject_AuditDetailsContent(t *testing.T) {
	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	reqBody := CreateProjectRequest{
		Name:        "detail-project",
		Description: "Test descriptions in audit",
		Destinations: []DestinationRequest{
			{Namespace: "staging"},
			{Namespace: "production"},
		},
		Roles: []RoleRequest{
			{Name: "developer", Policies: []string{"p, proj:detail-project:developer, instances, create, detail-project/*, allow"}},
			{Name: "viewer", Policies: []string{"p, proj:detail-project:viewer, instances, get, detail-project/*, allow"}},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	// AC #6: description in Details
	if e.Details["description"] != "Test descriptions in audit" {
		t.Errorf("expected description in details, got %v", e.Details["description"])
	}
	// AC #6: destinations array
	dests, ok := e.Details["destinations"].([]string)
	if !ok {
		t.Fatalf("expected destinations slice, got %T", e.Details["destinations"])
	}
	if len(dests) != 2 || dests[0] != "staging" || dests[1] != "production" {
		t.Errorf("expected [staging, production], got %v", dests)
	}
	// AC #6: roles array
	roles, ok := e.Details["roles"].([]string)
	if !ok {
		t.Fatalf("expected roles slice, got %T", e.Details["roles"])
	}
	if len(roles) != 2 || roles[0] != "developer" || roles[1] != "viewer" {
		t.Errorf("expected [developer, viewer], got %v", roles)
	}
}

func TestProjectHandler_UpdateProject_AuditModifiedRoles(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("role-project", rbac.ProjectSpec{
		Description: "original",
		Roles: []rbac.ProjectRole{
			{Name: "developer", Policies: []string{"p, old-policy"}},
			{Name: "viewer", Policies: []string{"p, viewer-policy"}},
		},
	})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	// Update developer policy, keep viewer, add admin
	reqBody := UpdateProjectRequest{
		ResourceVersion: "1",
		Roles: []RoleRequest{
			{Name: "developer", Policies: []string{"p, new-policy"}},
			{Name: "viewer", Policies: []string{"p, viewer-policy"}},
			{Name: "admin", Policies: []string{"p, admin-policy"}},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/role-project", body, userCtx)
	req.SetPathValue("name", "role-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	changes, ok := e.Details["changes"].(map[string]any)
	if !ok {
		t.Fatalf("expected changes map, got %v", e.Details)
	}

	// "admin" was added
	addedRoles, ok := changes["addedRoles"].([]string)
	if !ok || len(addedRoles) != 1 || addedRoles[0] != "admin" {
		t.Errorf("expected addedRoles=[admin], got %v", changes["addedRoles"])
	}

	// "developer" was modified (policy changed)
	modifiedRoles, ok := changes["modifiedRoles"].([]string)
	if !ok || len(modifiedRoles) != 1 || modifiedRoles[0] != "developer" {
		t.Errorf("expected modifiedRoles=[developer], got %v", changes["modifiedRoles"])
	}

	// "viewer" should NOT appear in any change list (unchanged)
	if _, exists := changes["removedRoles"]; exists {
		t.Errorf("expected no removedRoles, got %v", changes["removedRoles"])
	}
}

func TestProjectHandler_DeleteProject_AuditDetailsSnapshot(t *testing.T) {
	svc := newMockProjectService()
	svc.addProject("snapshot-project", rbac.ProjectSpec{
		Description: "will be deleted",
		Destinations: []rbac.Destination{
			{Namespace: "ns-a"},
			{Namespace: "ns-b"},
		},
		Roles: []rbac.ProjectRole{
			{Name: "dev"},
			{Name: "viewer"},
			{Name: "admin"},
		},
	})
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewProjectHandler(svc, enforcer, recorder)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/snapshot-project", nil, userCtx)
	req.SetPathValue("name", "snapshot-project")
	rec := httptest.NewRecorder()

	handler.DeleteProject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	// AC #8: description, destinationsCount, rolesCount
	if e.Details["description"] != "will be deleted" {
		t.Errorf("expected description 'will be deleted', got %v", e.Details["description"])
	}
	if e.Details["destinationsCount"] != 2 {
		t.Errorf("expected destinationsCount 2, got %v", e.Details["destinationsCount"])
	}
	if e.Details["rolesCount"] != 3 {
		t.Errorf("expected rolesCount 3, got %v", e.Details["rolesCount"])
	}
}

// --- SSOSettingsHandler audit detail tests (Task 6) ---

func TestSSOSettingsHandler_UpdateProvider_AuditChangeTracking(t *testing.T) {
	cs := fake.NewSimpleClientset()
	store := sso.NewProviderStore(cs, ssoTestNamespace)
	recorder := &mockAuditRecorder{}
	handler := NewSSOSettingsHandler(store, recorder, &mockSSOAccessChecker{allowed: true})

	// Create initial provider
	ctx := context.Background()
	if err := store.Create(ctx, sso.SSOProvider{
		Name:         "update-track",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "old-client-id",
		ClientSecret: "old-secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid"},
	}); err != nil {
		t.Fatal(err)
	}

	// Update with new values
	updateReq := SSOProviderRequest{
		IssuerURL:    "https://login.microsoftonline.com/tenant/v2.0",
		ClientID:     "new-client-id",
		ClientSecret: "new-secret",
		RedirectURL:  "https://app.example.com/cb",
		Scopes:       []string{"openid", "profile"},
	}
	body, _ := json.Marshal(updateReq)
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/settings/sso/providers/update-track", body,
		&middleware.UserContext{UserID: "admin", Email: "admin@test.local"})
	req.SetPathValue("name", "update-track")
	rec := httptest.NewRecorder()

	handler.UpdateProvider(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "update" {
		t.Errorf("expected action 'update', got %q", e.Action)
	}
	if e.Details["settingsType"] != "sso_provider" {
		t.Errorf("expected settingsType, got %v", e.Details["settingsType"])
	}

	// Verify before/after for issuerURL
	issuerChange, ok := e.Details["issuerURL"].(map[string]any)
	if !ok {
		t.Fatalf("expected issuerURL change map, got %T", e.Details["issuerURL"])
	}
	if issuerChange["old"] != "https://accounts.google.com" {
		t.Errorf("expected old issuerURL, got %v", issuerChange["old"])
	}
	if issuerChange["new"] != "https://login.microsoftonline.com/tenant/v2.0" {
		t.Errorf("expected new issuerURL, got %v", issuerChange["new"])
	}

	// Verify credentialsUpdated flag (secret changed)
	if e.Details["credentialsUpdated"] != true {
		t.Errorf("expected credentialsUpdated=true, got %v", e.Details["credentialsUpdated"])
	}

	// Verify no secret values stored
	if _, exists := e.Details["clientSecret"]; exists {
		t.Error("clientSecret should never appear in audit details")
	}

	// M1: Verify scope change tracking
	scopeChange, ok := e.Details["scopes"].(map[string]any)
	if !ok {
		t.Fatalf("expected scopes change map, got %T", e.Details["scopes"])
	}
	oldScopes, ok := scopeChange["old"].([]string)
	if !ok {
		t.Fatalf("expected old scopes slice, got %T", scopeChange["old"])
	}
	if len(oldScopes) != 1 || oldScopes[0] != "openid" {
		t.Errorf("expected old scopes [openid], got %v", oldScopes)
	}
	newScopes, ok := scopeChange["new"].([]string)
	if !ok {
		t.Fatalf("expected new scopes slice, got %T", scopeChange["new"])
	}
	if len(newScopes) != 2 || newScopes[0] != "openid" || newScopes[1] != "profile" {
		t.Errorf("expected new scopes [openid, profile], got %v", newScopes)
	}
}

// --- ComplianceHandler audit detail tests ---

func TestComplianceHandler_EnforcementChange_AuditDetails(t *testing.T) {
	svc := newMockComplianceService(true)
	svc.addConstraint("require-labels", "K8sRequiredLabels", "k8srequiredlabels", "deny", 3)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetAuditRecorder(recorder)

	reqBody := `{"enforcementAction":"warn"}`
	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPatch,
		"/api/v1/compliance/constraints/K8sRequiredLabels/require-labels/enforcement",
		[]byte(reqBody), userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-labels")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "enforcement_change" {
		t.Errorf("expected action 'enforcement_change', got %q", e.Action)
	}
	if e.Resource != "compliance" {
		t.Errorf("expected resource 'compliance', got %q", e.Resource)
	}
	if e.Details["constraintName"] != "require-labels" {
		t.Errorf("expected constraintName, got %v", e.Details["constraintName"])
	}
	if e.Details["kind"] != "K8sRequiredLabels" {
		t.Errorf("expected kind, got %v", e.Details["kind"])
	}
	if e.Details["templateName"] != "k8srequiredlabels" {
		t.Errorf("expected templateName, got %v", e.Details["templateName"])
	}
	if e.Details["enforcementAction"] != "warn" {
		t.Errorf("expected enforcementAction 'warn', got %v", e.Details["enforcementAction"])
	}
	if e.Details["previousEnforcementAction"] != "deny" {
		t.Errorf("expected previousEnforcementAction 'deny', got %v", e.Details["previousEnforcementAction"])
	}
}

func TestComplianceHandler_ConstraintCreate_AuditDetails(t *testing.T) {
	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Requires labels on resources")
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	recorder := &mockAuditRecorder{}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetAuditRecorder(recorder)

	reqBody, _ := json.Marshal(CreateConstraintRequest{
		Name:              "must-have-owner",
		TemplateName:      "k8srequiredlabels",
		EnforcementAction: "warn",
	})
	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost,
		"/api/v1/compliance/constraints", reqBody, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "constraint_create" {
		t.Errorf("expected action 'constraint_create', got %q", e.Action)
	}
	if e.Resource != "compliance" {
		t.Errorf("expected resource 'compliance', got %q", e.Resource)
	}
	if e.Details["constraintName"] != "must-have-owner" {
		t.Errorf("expected constraintName, got %v", e.Details["constraintName"])
	}
	if e.Details["kind"] != "K8sRequiredLabels" {
		t.Errorf("expected kind, got %v", e.Details["kind"])
	}
	if e.Details["templateName"] != "k8srequiredlabels" {
		t.Errorf("expected templateName, got %v", e.Details["templateName"])
	}
	if e.Details["enforcementAction"] != "warn" {
		t.Errorf("expected enforcementAction 'warn', got %v", e.Details["enforcementAction"])
	}
}

// --- RepositoryHandler audit detail tests (C1) ---

func newTestRepoService(t *testing.T) *repository.Service {
	t.Helper()
	svc, err := repository.NewService(fake.NewSimpleClientset(), nil, nil, "test-ns")
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestRepositoryHandler_CreateRepo_AuditDetails(t *testing.T) {
	svc := newTestRepoService(t)
	recorder := &mockAuditRecorder{}
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewRepositoryHandler(svc, nil, enforcer, recorder)

	reqBody := CreateRepositoryConfigRequest{
		Name:          "test-repo",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/repo.git",
		AuthType:      repository.AuthTypeHTTPS,
		DefaultBranch: "main",
		HTTPSAuth:     &repository.HTTPSAuthConfig{BearerToken: "ghp_test12345"},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/repositories", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateRepositoryConfig(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "create" {
		t.Errorf("expected action 'create', got %q", e.Action)
	}
	if e.Resource != "repositories" {
		t.Errorf("expected resource 'repositories', got %q", e.Resource)
	}
	if e.Project != "alpha" {
		t.Errorf("expected project 'alpha', got %q", e.Project)
	}
	if e.Details["repoURL"] != "https://github.com/example/repo.git" {
		t.Errorf("expected repoURL in details, got %v", e.Details["repoURL"])
	}
	if e.Details["authType"] != "https" {
		t.Errorf("expected authType 'https' in details, got %v", e.Details["authType"])
	}
	if e.Details["defaultBranch"] != "main" {
		t.Errorf("expected defaultBranch 'main' in details, got %v", e.Details["defaultBranch"])
	}
	// Verify no secret data leaked
	if _, exists := e.Details["bearerToken"]; exists {
		t.Error("bearerToken should not appear in audit details")
	}
}

func TestRepositoryHandler_UpdateRepo_AuditDetails(t *testing.T) {
	svc := newTestRepoService(t)
	recorder := &mockAuditRecorder{}
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewRepositoryHandler(svc, nil, enforcer, recorder)

	// Create a repo first
	createReq := repository.CreateRepositoryRequest{
		Name:          "original-name",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/repo.git",
		AuthType:      repository.AuthTypeHTTPS,
		DefaultBranch: "main",
		HTTPSAuth:     &repository.HTTPSAuthConfig{BearerToken: "ghp_test"},
	}
	repoConfig, err := svc.CreateRepositoryConfigWithCredentials(context.Background(), createReq, "admin-user")
	if err != nil {
		t.Fatal(err)
	}
	repoID := repoConfig.Name

	// Update the defaultBranch
	updateBody := UpdateRepositoryConfigRequest{
		Name:          "updated-name",
		DefaultBranch: "develop",
	}
	body, _ := json.Marshal(updateBody)

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/repositories/"+repoID, body, userCtx)
	req.SetPathValue("repoId", repoID)
	rec := httptest.NewRecorder()

	handler.UpdateRepositoryConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "update" {
		t.Errorf("expected action 'update', got %q", e.Action)
	}
	if e.Resource != "repositories" {
		t.Errorf("expected resource 'repositories', got %q", e.Resource)
	}

	// Verify change tracking: defaultBranch changed
	branchChange, ok := e.Details["defaultBranch"].(map[string]any)
	if !ok {
		t.Fatalf("expected defaultBranch change map, got %T (%v)", e.Details["defaultBranch"], e.Details)
	}
	if branchChange["old"] != "main" {
		t.Errorf("expected old defaultBranch 'main', got %v", branchChange["old"])
	}
	if branchChange["new"] != "develop" {
		t.Errorf("expected new defaultBranch 'develop', got %v", branchChange["new"])
	}

	// Verify name change tracked
	nameChange, ok := e.Details["name"].(map[string]any)
	if !ok {
		t.Fatalf("expected name change map, got %T (%v)", e.Details["name"], e.Details)
	}
	if nameChange["old"] != "original-name" {
		t.Errorf("expected old name 'original-name', got %v", nameChange["old"])
	}
	if nameChange["new"] != "updated-name" {
		t.Errorf("expected new name 'updated-name', got %v", nameChange["new"])
	}
}

func TestRepositoryHandler_DeleteRepo_AuditDetails(t *testing.T) {
	svc := newTestRepoService(t)
	recorder := &mockAuditRecorder{}
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewRepositoryHandler(svc, nil, enforcer, recorder)

	// Create a repo first (using HTTPS; authType is captured in audit details)
	createReq := repository.CreateRepositoryRequest{
		Name:          "doomed-repo",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/doomed.git",
		AuthType:      repository.AuthTypeHTTPS,
		DefaultBranch: "main",
		HTTPSAuth:     &repository.HTTPSAuthConfig{BearerToken: "ghp_test"},
	}
	repoConfig, err := svc.CreateRepositoryConfigWithCredentials(context.Background(), createReq, "admin-user")
	if err != nil {
		t.Fatal(err)
	}
	repoID := repoConfig.Name

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/repositories/"+repoID, nil, userCtx)
	req.SetPathValue("repoId", repoID)
	rec := httptest.NewRecorder()

	handler.DeleteRepositoryConfig(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "delete" {
		t.Errorf("expected action 'delete', got %q", e.Action)
	}
	if e.Resource != "repositories" {
		t.Errorf("expected resource 'repositories', got %q", e.Resource)
	}
	if e.Project != "alpha" {
		t.Errorf("expected project 'alpha', got %q", e.Project)
	}
	// Delete snapshot should include repoURL and authType
	if e.Details["repoURL"] != "https://github.com/example/doomed.git" {
		t.Errorf("expected repoURL snapshot, got %v", e.Details["repoURL"])
	}
	if e.Details["authType"] != "https" {
		t.Errorf("expected authType snapshot, got %v", e.Details["authType"])
	}
}

// --- InstanceCRUDHandler audit detail tests (C2) ---

func TestInstanceCRUDHandler_DeleteInstance_AuditDetails(t *testing.T) {
	// Set up instance in cache
	cache := watcher.NewInstanceCache()
	cache.Set(&models.Instance{
		Name:        "test-instance",
		Namespace:   "production",
		Kind:        "Webapp",
		APIVersion:  "kro.run/v1alpha1",
		RGDName:     "webapp-rgd",
		Health:      models.HealthHealthy,
		ProjectName: "alpha",
	})

	// Create fake dynamic client with the instance object registered
	scheme := runtime.NewScheme()
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "Webapp",
			"metadata": map[string]interface{}{
				"name":      "test-instance",
				"namespace": "production",
			},
		},
	}
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "kro.run", Version: "v1alpha1", Resource: "webapps"}: "WebAppList",
	}
	fakeDynClient := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, obj)

	// Create tracker with cache and dynamic client
	tracker := watcher.NewInstanceTrackerForTest(cache, fakeDynClient)

	// Create handler
	recorder := &mockAuditRecorder{}
	handler := NewInstanceCRUDHandler(InstanceCRUDHandlerConfig{
		InstanceTracker: tracker,
		DynamicClient:   fakeDynClient,
		AuditRecorder:   recorder,
		AuthService:     adminAuthService(),
	})

	userCtx := &middleware.UserContext{UserID: "admin-user", Email: "admin@test.local"}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/instances/production/WebApp/test-instance", nil, userCtx)
	req.SetPathValue("namespace", "production")
	req.SetPathValue("kind", "Webapp")
	req.SetPathValue("name", "test-instance")
	rec := httptest.NewRecorder()

	handler.DeleteInstance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(recorder.events))
	}

	e := recorder.lastEvent()
	if e.Action != "delete" {
		t.Errorf("expected action 'delete', got %q", e.Action)
	}
	if e.Resource != "instances" {
		t.Errorf("expected resource 'instances', got %q", e.Resource)
	}
	if e.Name != "test-instance" {
		t.Errorf("expected name 'test-instance', got %q", e.Name)
	}
	if e.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", e.Namespace)
	}
	if e.Project != "alpha" {
		t.Errorf("expected project 'alpha', got %q", e.Project)
	}
	// Verify audit detail fields
	if e.Details["rgdName"] != "webapp-rgd" {
		t.Errorf("expected rgdName 'webapp-rgd', got %v", e.Details["rgdName"])
	}
	if e.Details["kind"] != "Webapp" {
		t.Errorf("expected kind 'Webapp', got %v", e.Details["kind"])
	}
	if e.Details["health"] != "Healthy" {
		t.Errorf("expected health 'Healthy', got %v", e.Details["health"])
	}
}
