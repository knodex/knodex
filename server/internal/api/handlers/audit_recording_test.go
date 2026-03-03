package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/audit"
	"github.com/provops-org/knodex/server/internal/rbac"
	"github.com/provops-org/knodex/server/internal/sso"
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
	if e.Details["description"] != "new description" {
		t.Errorf("expected description in details, got %v", e.Details)
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
