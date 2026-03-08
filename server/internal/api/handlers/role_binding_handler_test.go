// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockRoleBindingEnforcer extends mockPolicyEnforcer with RemoveUserRole for role binding tests
type mockRoleBindingEnforcer struct {
	canAccessResult   bool
	canAccessErr      error
	assignErr         error
	removeErr         error
	removeUserRoleErr error
	assignedRoles     map[string][]string // user -> roles
	removedRoles      map[string][]string // user -> removed roles

	// AC-7: Track policy reload calls for testing immediate policy effect
	loadProjectPoliciesCalls    []*rbac.Project
	loadProjectPoliciesErr      error
	invalidateCacheForProjCalls []string
}

func newMockRoleBindingEnforcer() *mockRoleBindingEnforcer {
	return &mockRoleBindingEnforcer{
		assignedRoles: make(map[string][]string),
		removedRoles:  make(map[string][]string),
	}
}

// newMockRoleBindingEnforcerWithGlobalAdmin creates a mock enforcer where the specified user has global admin role

func newMockRoleBindingEnforcerWithGlobalAdmin(adminUserID string) *mockRoleBindingEnforcer {
	m := newMockRoleBindingEnforcer()
	m.assignedRoles[adminUserID] = []string{"role:serveradmin"}

	m.canAccessResult = true
	return m
}

func (m *mockRoleBindingEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessErr != nil {
		return false, m.canAccessErr
	}
	return m.canAccessResult, nil
}

func (m *mockRoleBindingEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	if m.canAccessErr != nil {
		return m.canAccessErr
	}
	if !m.canAccessResult {
		return rbac.ErrAccessDenied
	}
	return nil
}

func (m *mockRoleBindingEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	m.loadProjectPoliciesCalls = append(m.loadProjectPoliciesCalls, project)
	return m.loadProjectPoliciesErr
}

func (m *mockRoleBindingEnforcer) SyncPolicies(ctx context.Context) error {
	return nil
}

func (m *mockRoleBindingEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	if m.assignErr != nil {
		return m.assignErr
	}
	m.assignedRoles[user] = append(m.assignedRoles[user], roles...)
	return nil
}

func (m *mockRoleBindingEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return m.assignedRoles[user], nil
}

func (m *mockRoleBindingEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRoleBindingEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	delete(m.assignedRoles, user)
	return nil
}

func (m *mockRoleBindingEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	if m.removeUserRoleErr != nil {
		return m.removeUserRoleErr
	}
	m.removedRoles[user] = append(m.removedRoles[user], role)
	// Remove from assignedRoles if present
	roles := m.assignedRoles[user]
	filtered := []string{}
	for _, r := range roles {
		if r != role {
			filtered = append(filtered, r)
		}
	}
	m.assignedRoles[user] = filtered
	return nil
}

func (m *mockRoleBindingEnforcer) RestorePersistedRoles(ctx context.Context) error {
	return nil
}

func (m *mockRoleBindingEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return m.removeErr
}

func (m *mockRoleBindingEnforcer) InvalidateCache() {
}

func (m *mockRoleBindingEnforcer) CacheStats() rbac.CacheStats {
	return rbac.CacheStats{}
}

func (m *mockRoleBindingEnforcer) Metrics() rbac.PolicyMetrics {
	return rbac.PolicyMetrics{}
}

func (m *mockRoleBindingEnforcer) IncrementPolicyReloads() {
}

func (m *mockRoleBindingEnforcer) IncrementBackgroundSyncs() {
}

func (m *mockRoleBindingEnforcer) IncrementWatcherRestarts() {
}

// CanAccessWithGroups implements rbac.PolicyEnforcer
// For testing, delegates to CanAccess with the user - groups/roles are not used in these tests
func (m *mockRoleBindingEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.CanAccess(ctx, user, object, action)
}

// InvalidateCacheForUser implements rbac.PolicyEnforcer
func (m *mockRoleBindingEnforcer) InvalidateCacheForUser(user string) int {
	return 0
}

// InvalidateCacheForProject implements rbac.PolicyEnforcer
func (m *mockRoleBindingEnforcer) InvalidateCacheForProject(projectName string) int {
	m.invalidateCacheForProjCalls = append(m.invalidateCacheForProjCalls, projectName)
	return 1
}

// GetAccessibleProjects implements rbac.PolicyEnforcer
func (m *mockRoleBindingEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	return nil, nil
}

// Helper to create project with roles for role binding tests
func createProjectWithRoles(name string, roleNames ...string) *rbac.Project {
	roles := make([]rbac.ProjectRole, len(roleNames))
	for i, rn := range roleNames {
		roles[i] = rbac.ProjectRole{
			Name:        rn,
			Description: rn + " role",
		}
	}
	return &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: "1",
		},
		Spec: rbac.ProjectSpec{
			Description: "Test project",
			Roles:       roles,
		},
	}
}

// Helper to create mock project service with a project
func createMockProjectServiceWithProject(project *rbac.Project) *mockProjectService {
	svc := newMockProjectService()
	svc.projects[project.Name] = project
	return svc
}

// roleBindingTestSetup bundles common dependencies for role binding handler tests.
type roleBindingTestSetup struct {
	handler  *RoleBindingHandler
	service  *mockProjectService
	enforcer *mockRoleBindingEnforcer
	project  *rbac.Project
	recorder *httptest.ResponseRecorder
}

// newRoleBindingTestSetup creates a setup with a global admin enforcer and a project that has the given roles.
func newRoleBindingTestSetup(t *testing.T, projectName string, roles ...string) *roleBindingTestSetup {
	t.Helper()
	project := createProjectWithRoles(projectName, roles...)
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")
	handler := NewRoleBindingHandler(svc, enforcer, nil)
	return &roleBindingTestSetup{
		handler:  handler,
		service:  svc,
		enforcer: enforcer,
		project:  project,
		recorder: httptest.NewRecorder(),
	}
}

// makeRequest builds an *http.Request with user context and sets path values from the pathValues map.
func (s *roleBindingTestSetup) makeRequest(t *testing.T, method, path string, body []byte, userID string, pathValues map[string]string) *http.Request {
	t.Helper()
	userCtx := &middleware.UserContext{UserID: userID}
	req := newRequestWithUserContext(method, path, body, userCtx)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

// ============== AssignUserRole Tests ==============

func TestRoleBindingHandler_AssignUserRole_Success(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin", "viewer")
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")
	enforcer.canAccessResult = true

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var binding RoleBindingResponse
	if err := json.NewDecoder(resp.Body).Decode(&binding); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if binding.Project != "my-project" {
		t.Errorf("expected project 'my-project', got '%s'", binding.Project)
	}
	if binding.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", binding.Role)
	}
	if binding.Subject != "test-user" {
		t.Errorf("expected subject 'test-user', got '%s'", binding.Subject)
	}
	if binding.Type != "user" {
		t.Errorf("expected type 'user', got '%s'", binding.Type)
	}

	// Verify enforcer received the assignment
	roles := enforcer.assignedRoles["test-user"]
	if len(roles) != 1 || roles[0] != "proj:my-project:admin" {
		t.Errorf("expected role 'proj:my-project:admin' assigned, got %v", roles)
	}
}

func TestRoleBindingHandler_AssignUserRole_Unauthorized(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	// No user context
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestRoleBindingHandler_AssignUserRole_Forbidden(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = false // Access denied

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestRoleBindingHandler_AssignUserRole_ProjectNotFound(t *testing.T) {
	t.Parallel()

	s := newRoleBindingTestSetup(t, "other-project", "admin")
	// Remove all projects so "nonexistent" is not found
	delete(s.service.projects, "other-project")

	req := s.makeRequest(t, http.MethodPost, "/api/v1/projects/nonexistent/roles/admin/users/test-user", nil, "admin-user",
		map[string]string{"name": "nonexistent", "role": "admin", "user": "test-user"})

	s.handler.AssignUserRole(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestRoleBindingHandler_AssignUserRole_InvalidRole(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin", "viewer") // No 'owner' role
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/owner/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "owner") // Role doesn't exist in project
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestRoleBindingHandler_AssignUserRole_MissingParams(t *testing.T) {
	t.Parallel()

	handler := NewRoleBindingHandler(newMockProjectService(), newMockRoleBindingEnforcerWithGlobalAdmin("admin-user"), nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects//roles//users/", nil, userCtx)
	req.SetPathValue("name", "")
	req.SetPathValue("role", "")
	req.SetPathValue("user", "")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestRoleBindingHandler_AssignUserRole_EnforcerError(t *testing.T) {
	t.Parallel()

	s := newRoleBindingTestSetup(t, "my-project", "admin")
	s.enforcer.assignErr = errors.New("casbin error")

	req := s.makeRequest(t, http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, "admin-user",
		map[string]string{"name": "my-project", "role": "admin", "user": "test-user"})

	s.handler.AssignUserRole(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// ============== AssignGroupRole Tests ==============

func TestRoleBindingHandler_AssignGroupRole_Success(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "deployer")
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/deployer/groups/dev-team", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "deployer")
	req.SetPathValue("group", "dev-team")
	rec := httptest.NewRecorder()

	handler.AssignGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var binding RoleBindingResponse
	if err := json.NewDecoder(resp.Body).Decode(&binding); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if binding.Type != "group" {
		t.Errorf("expected type 'group', got '%s'", binding.Type)
	}
	if binding.Subject != "dev-team" {
		t.Errorf("expected subject 'dev-team', got '%s'", binding.Subject)
	}

	// Verify enforcer received group assignment with correct format
	roles := enforcer.assignedRoles["group:dev-team"]
	if len(roles) != 1 || roles[0] != "proj:my-project:deployer" {
		t.Errorf("expected role 'proj:my-project:deployer' assigned to 'group:dev-team', got %v", roles)
	}
}

func TestRoleBindingHandler_AssignGroupRole_ProjectNotFound(t *testing.T) {
	t.Parallel()

	s := newRoleBindingTestSetup(t, "other-project", "admin")
	delete(s.service.projects, "other-project")

	req := s.makeRequest(t, http.MethodPost, "/api/v1/projects/nonexistent/roles/admin/groups/test-group", nil, "admin-user",
		map[string]string{"name": "nonexistent", "role": "admin", "group": "test-group"})

	s.handler.AssignGroupRole(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestRoleBindingHandler_AssignGroupRole_Forbidden(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = false

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/admin/groups/test-group", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("group", "test-group")
	rec := httptest.NewRecorder()

	handler.AssignGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

// ============== ListRoleBindings Tests ==============

func TestRoleBindingHandler_ListRoleBindings_Success(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin", "viewer")
	// Add role bindings annotation
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"admin","subject":"user1","type":"user"},{"role":"viewer","subject":"group1","type":"group"}]`,
	}
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/my-project/role-bindings", nil, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.ListRoleBindings(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListRoleBindingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Project != "my-project" {
		t.Errorf("expected project 'my-project', got '%s'", listResp.Project)
	}
	if len(listResp.Bindings) != 2 {
		t.Errorf("expected 2 bindings, got %d", len(listResp.Bindings))
	}
}

func TestRoleBindingHandler_ListRoleBindings_EmptyBindings(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	// No role bindings annotation
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/my-project/role-bindings", nil, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.ListRoleBindings(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListRoleBindingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(listResp.Bindings) != 0 {
		t.Errorf("expected 0 bindings, got %d", len(listResp.Bindings))
	}
}

func TestRoleBindingHandler_ListRoleBindings_ProjectNotFound(t *testing.T) {
	t.Parallel()

	s := newRoleBindingTestSetup(t, "other-project", "admin")
	delete(s.service.projects, "other-project")

	req := s.makeRequest(t, http.MethodGet, "/api/v1/projects/nonexistent/role-bindings", nil, "admin-user",
		map[string]string{"name": "nonexistent"})

	s.handler.ListRoleBindings(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestRoleBindingHandler_ListRoleBindings_Forbidden(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = false

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/my-project/role-bindings", nil, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.ListRoleBindings(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestRoleBindingHandler_ListRoleBindings_NonAdminWithAccess(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "viewer")
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"viewer","subject":"user1","type":"user"}]`,
	}
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = true // Has "get" permission

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/my-project/role-bindings", nil, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.ListRoleBindings(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

// ============== RemoveUserRole Tests ==============

func TestRoleBindingHandler_RemoveUserRole_Success(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"admin","subject":"test-user","type":"user"}]`,
	}
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.RemoveUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify enforcer received removal
	removedRoles := enforcer.removedRoles["test-user"]
	if len(removedRoles) != 1 || removedRoles[0] != "proj:my-project:admin" {
		t.Errorf("expected role 'proj:my-project:admin' removed, got %v", removedRoles)
	}
}

func TestRoleBindingHandler_RemoveUserRole_ProjectNotFound(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/nonexistent/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "nonexistent")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.RemoveUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestRoleBindingHandler_RemoveUserRole_Forbidden(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = false

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.RemoveUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestRoleBindingHandler_RemoveUserRole_EnforcerError(t *testing.T) {
	t.Parallel()

	s := newRoleBindingTestSetup(t, "my-project", "admin")
	s.enforcer.removeUserRoleErr = errors.New("casbin error")

	req := s.makeRequest(t, http.MethodDelete, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, "admin-user",
		map[string]string{"name": "my-project", "role": "admin", "user": "test-user"})

	s.handler.RemoveUserRole(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// ============== RemoveGroupRole Tests ==============

func TestRoleBindingHandler_RemoveGroupRole_Success(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "deployer")
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"deployer","subject":"dev-team","type":"group"}]`,
	}
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/deployer/groups/dev-team", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "deployer")
	req.SetPathValue("group", "dev-team")
	rec := httptest.NewRecorder()

	handler.RemoveGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify enforcer received removal with correct group format
	removedRoles := enforcer.removedRoles["group:dev-team"]
	if len(removedRoles) != 1 || removedRoles[0] != "proj:my-project:deployer" {
		t.Errorf("expected role 'proj:my-project:deployer' removed from 'group:dev-team', got %v", removedRoles)
	}
}

func TestRoleBindingHandler_RemoveGroupRole_ProjectNotFound(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/nonexistent/roles/admin/groups/test-group", nil, userCtx)
	req.SetPathValue("name", "nonexistent")
	req.SetPathValue("role", "admin")
	req.SetPathValue("group", "test-group")
	rec := httptest.NewRecorder()

	handler.RemoveGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestRoleBindingHandler_RemoveGroupRole_Forbidden(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = false

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/admin/groups/test-group", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("group", "test-group")
	rec := httptest.NewRecorder()

	handler.RemoveGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

// ============== Helper Function Tests ==============

func TestRoleExistsInProject(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("test", "admin", "viewer", "deployer")

	tests := []struct {
		roleName string
		expected bool
	}{
		{"admin", true},
		{"viewer", true},
		{"deployer", true},
		{"owner", false},
		{"", false},
		{"ADMIN", false}, // Case sensitive
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.roleName, func(t *testing.T) {
			t.Parallel()

			result := roleExistsInProject(project, tt.roleName)
			if result != tt.expected {
				t.Errorf("roleExistsInProject(%q) = %v, want %v", tt.roleName, result, tt.expected)
			}
		})
	}
}

func TestExtractRoleBindingsFromProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
		expected    int
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    0,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    0,
		},
		{
			name: "no role bindings key",
			annotations: map[string]string{
				"other-key": "value",
			},
			expected: 0,
		},
		{
			name: "empty JSON array",
			annotations: map[string]string{
				"knodex.io/role-bindings": "[]",
			},
			expected: 0,
		},
		{
			name: "valid bindings",
			annotations: map[string]string{
				"knodex.io/role-bindings": `[{"role":"admin","subject":"user1","type":"user"},{"role":"viewer","subject":"group1","type":"group"}]`,
			},
			expected: 2,
		},
		{
			name: "invalid JSON",
			annotations: map[string]string{
				"knodex.io/role-bindings": "invalid json",
			},
			expected: 0, // Should return empty slice on error
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project := &rbac.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: tt.annotations,
				},
			}
			bindings := extractRoleBindingsFromProject(project)
			if len(bindings) != tt.expected {
				t.Errorf("extractRoleBindingsFromProject() returned %d bindings, want %d", len(bindings), tt.expected)
			}
		})
	}
}

// Note: IsNotFoundError tests have been moved to internal/api/helpers/errors_test.go
// as part of the handler helpers refactoring.

// ============== Role Binding Persistence Tests ==============

func TestRoleBindingHandler_AssignUserRole_PersistsBinding(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Verify binding was persisted in project annotations
	updatedProject := svc.projects["my-project"]
	bindings := extractRoleBindingsFromProject(updatedProject)
	if len(bindings) != 1 {
		t.Errorf("expected 1 binding persisted, got %d", len(bindings))
	}
	if len(bindings) > 0 {
		if bindings[0].Role != "admin" || bindings[0].Subject != "test-user" || bindings[0].Type != "user" {
			t.Errorf("unexpected binding: %+v", bindings[0])
		}
	}
}

func TestRoleBindingHandler_RemoveUserRole_RemovesPersistedBinding(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"admin","subject":"test-user","type":"user"},{"role":"admin","subject":"other-user","type":"user"}]`,
	}
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.RemoveUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify binding was removed from project annotations
	updatedProject := svc.projects["my-project"]
	bindings := extractRoleBindingsFromProject(updatedProject)
	if len(bindings) != 1 {
		t.Errorf("expected 1 remaining binding, got %d", len(bindings))
	}
	// The remaining binding should be for other-user
	if len(bindings) > 0 && bindings[0].Subject != "other-user" {
		t.Errorf("expected remaining binding for 'other-user', got %+v", bindings[0])
	}
}

// ============== Authorization with Project-Level Access Tests ==============

func TestRoleBindingHandler_AssignUserRole_NonAdminWithUpdateAccess(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "viewer")
	svc := createMockProjectServiceWithProject(project)
	enforcer := newMockRoleBindingEnforcer()
	enforcer.canAccessResult = true // Has "update" permission

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "project-admin",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/viewer/users/new-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "viewer")
	req.SetPathValue("user", "new-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}
}

// ============== AC-7: Cache Invalidation Tests ==============
// These tests verify that cache invalidation is triggered after successful role binding changes
// to ensure permission changes take effect immediately. Note: Role binding changes do NOT reload
// policies because the in-memory Casbin binding already provides immediate effect.

func TestRoleBindingHandler_AssignUserRole_InvalidatesCache(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Verify LoadProjectPolicies was NOT called (in-memory binding provides immediate effect)
	if len(enforcer.loadProjectPoliciesCalls) != 0 {
		t.Errorf("expected LoadProjectPolicies to NOT be called, got %d calls", len(enforcer.loadProjectPoliciesCalls))
	}

	// AC-4: Verify InvalidateCacheForProject was called for immediate cache refresh
	if len(enforcer.invalidateCacheForProjCalls) != 1 {
		t.Errorf("expected InvalidateCacheForProject to be called 1 time, got %d", len(enforcer.invalidateCacheForProjCalls))
	}
	if len(enforcer.invalidateCacheForProjCalls) > 0 && enforcer.invalidateCacheForProjCalls[0] != "my-project" {
		t.Errorf("expected InvalidateCacheForProject to be called with 'my-project', got '%s'", enforcer.invalidateCacheForProjCalls[0])
	}
}

func TestRoleBindingHandler_AssignGroupRole_InvalidatesCache(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "deployer")
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/deployer/groups/dev-team", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "deployer")
	req.SetPathValue("group", "dev-team")
	rec := httptest.NewRecorder()

	handler.AssignGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Verify LoadProjectPolicies was NOT called (in-memory binding provides immediate effect)
	if len(enforcer.loadProjectPoliciesCalls) != 0 {
		t.Errorf("expected LoadProjectPolicies to NOT be called, got %d calls", len(enforcer.loadProjectPoliciesCalls))
	}

	// AC-4: Verify InvalidateCacheForProject was called for immediate cache refresh
	if len(enforcer.invalidateCacheForProjCalls) != 1 {
		t.Errorf("expected InvalidateCacheForProject to be called 1 time, got %d", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestRoleBindingHandler_RemoveUserRole_InvalidatesCache(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "admin")
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"admin","subject":"test-user","type":"user"}]`,
	}
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.RemoveUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify LoadProjectPolicies was NOT called (in-memory binding removal provides immediate effect)
	if len(enforcer.loadProjectPoliciesCalls) != 0 {
		t.Errorf("expected LoadProjectPolicies to NOT be called, got %d calls", len(enforcer.loadProjectPoliciesCalls))
	}

	// AC-4: Verify InvalidateCacheForProject was called for immediate cache refresh
	if len(enforcer.invalidateCacheForProjCalls) != 1 {
		t.Errorf("expected InvalidateCacheForProject to be called 1 time, got %d", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestRoleBindingHandler_RemoveGroupRole_InvalidatesCache(t *testing.T) {
	t.Parallel()

	project := createProjectWithRoles("my-project", "deployer")
	project.Annotations = map[string]string{
		"knodex.io/role-bindings": `[{"role":"deployer","subject":"dev-team","type":"group"}]`,
	}
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/my-project/roles/deployer/groups/dev-team", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "deployer")
	req.SetPathValue("group", "dev-team")
	rec := httptest.NewRecorder()

	handler.RemoveGroupRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}

	// Verify LoadProjectPolicies was NOT called (in-memory binding removal provides immediate effect)
	if len(enforcer.loadProjectPoliciesCalls) != 0 {
		t.Errorf("expected LoadProjectPolicies to NOT be called, got %d calls", len(enforcer.loadProjectPoliciesCalls))
	}

	// AC-4: Verify InvalidateCacheForProject was called for immediate cache refresh
	if len(enforcer.invalidateCacheForProjCalls) != 1 {
		t.Errorf("expected InvalidateCacheForProject to be called 1 time, got %d", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestRoleBindingHandler_RoleAssignment_SucceedsWithCacheInvalidation(t *testing.T) {
	t.Parallel()

	// Verify role assignment succeeds and cache is invalidated for immediate effect
	project := createProjectWithRoles("my-project", "admin")
	svc := createMockProjectServiceWithProject(project)

	enforcer := newMockRoleBindingEnforcerWithGlobalAdmin("admin-user")

	handler := NewRoleBindingHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/my-project/roles/admin/users/test-user", nil, userCtx)
	req.SetPathValue("name", "my-project")
	req.SetPathValue("role", "admin")
	req.SetPathValue("user", "test-user")
	rec := httptest.NewRecorder()

	handler.AssignUserRole(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Request should succeed
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// Verify the role was assigned via Casbin (in-memory)
	roles := enforcer.assignedRoles["test-user"]
	if len(roles) != 1 || roles[0] != "proj:my-project:admin" {
		t.Errorf("expected role 'proj:my-project:admin' assigned, got %v", roles)
	}

	// Verify cache was invalidated for immediate effect
	if len(enforcer.invalidateCacheForProjCalls) != 1 {
		t.Errorf("expected InvalidateCacheForProject to be called 1 time, got %d", len(enforcer.invalidateCacheForProjCalls))
	}
}

// TestRoleBindingHandler_AssignUserRole_RejectsInvalidSubject tests AC-3:
// user=role:serveradmin is rejected with 400 Bad Request before hitting Casbin.
func TestRoleBindingHandler_AssignUserRole_RejectsInvalidSubject(t *testing.T) {
	t.Parallel()

	handler := NewRoleBindingHandler(nil, nil, nil)

	tests := []struct {
		name   string
		userID string
	}{
		{"colon in user (role:admin)", "role:serveradmin"},
		{"space in user", "user name"},
		{"reserved prefix proj:", "proj:alpha:admin"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/alpha/roles/admin/users/placeholder", nil)
			req.SetPathValue("name", "alpha")
			req.SetPathValue("role", "admin")
			req.SetPathValue("user", tt.userID)
			userCtx := &middleware.UserContext{UserID: "caller@test.local", Email: "caller@test.local"}
			ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.AssignUserRole(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("user=%q: expected status %d, got %d", tt.userID, http.StatusBadRequest, rec.Code)
			}
		})
	}
}

// TestRoleBindingHandler_AssignGroupRole_RejectsInvalidSubject tests AC-3:
// group names with colons or reserved prefixes are rejected.
func TestRoleBindingHandler_AssignGroupRole_RejectsInvalidSubject(t *testing.T) {
	t.Parallel()

	handler := NewRoleBindingHandler(nil, nil, nil)

	tests := []struct {
		name      string
		groupName string
	}{
		{"colon in group", "bad:group"},
		{"space in group", "bad group"},
		{"reserved prefix role:", "role:serveradmin"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/alpha/roles/admin/groups/placeholder", nil)
			req.SetPathValue("name", "alpha")
			req.SetPathValue("role", "admin")
			req.SetPathValue("group", tt.groupName)
			userCtx := &middleware.UserContext{UserID: "caller@test.local", Email: "caller@test.local"}
			ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			handler.AssignGroupRole(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("group=%q: expected status %d, got %d", tt.groupName, http.StatusBadRequest, rec.Code)
			}
		})
	}
}

// Unused variable to test compilation
var _ = bytes.Buffer{}
