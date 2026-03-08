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
	"time"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockProjectService is a mock implementation of rbac.ProjectServiceInterface
type mockProjectService struct {
	projects       map[string]*rbac.Project
	createErr      error
	getErr         error
	listErr        error
	updateErr      error
	deleteErr      error
	existsOverride *bool
}

func newMockProjectService() *mockProjectService {
	return &mockProjectService{
		projects: make(map[string]*rbac.Project),
	}
}

func (m *mockProjectService) CreateProject(ctx context.Context, name string, spec rbac.ProjectSpec, createdBy string) (*rbac.Project, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if _, exists := m.projects[name]; exists {
		return nil, rbac.ErrAlreadyExists
	}
	project := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			ResourceVersion:   "1",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				"knodex.io/created-by": createdBy,
			},
		},
		Spec: spec,
	}
	m.projects[name] = project
	return project, nil
}

func (m *mockProjectService) GetProject(ctx context.Context, name string) (*rbac.Project, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	project, exists := m.projects[name]
	if !exists {
		return nil, rbac.ErrProjectNotFound
	}
	return project, nil
}

func (m *mockProjectService) ListProjects(ctx context.Context) (*rbac.ProjectList, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	list := &rbac.ProjectList{
		Items: make([]rbac.Project, 0, len(m.projects)),
	}
	for _, p := range m.projects {
		list.Items = append(list.Items, *p)
	}
	return list, nil
}

func (m *mockProjectService) UpdateProject(ctx context.Context, project *rbac.Project, updatedBy string) (*rbac.Project, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if _, exists := m.projects[project.Name]; !exists {
		return nil, rbac.ErrProjectNotFound
	}
	// Increment resource version
	project.ResourceVersion = "2"
	if project.Annotations == nil {
		project.Annotations = make(map[string]string)
	}
	project.Annotations["knodex.io/updated-by"] = updatedBy
	project.Annotations["knodex.io/updated-at"] = time.Now().Format(time.RFC3339)
	m.projects[project.Name] = project
	return project, nil
}

func (m *mockProjectService) DeleteProject(ctx context.Context, name string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, exists := m.projects[name]; !exists {
		return rbac.ErrProjectNotFound
	}
	delete(m.projects, name)
	return nil
}

func (m *mockProjectService) Exists(ctx context.Context, name string) (bool, error) {
	if m.existsOverride != nil {
		return *m.existsOverride, nil
	}
	_, exists := m.projects[name]
	return exists, nil
}

func (m *mockProjectService) addProject(name string, spec rbac.ProjectSpec) *rbac.Project {
	project := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			ResourceVersion:   "1",
			CreationTimestamp: metav1.Time{Time: time.Now()},
			Labels: map[string]string{
				"knodex.io/created-by": "test-user",
			},
		},
		Spec: spec,
	}
	m.projects[name] = project
	return project
}

// mockPolicyEnforcer is a mock implementation of rbac.PolicyEnforcer
type mockPolicyEnforcer struct {
	canAccessResult    bool
	canAccessErr       error
	loadErr            error
	removeErr          error
	hasRoleFunc        func(ctx context.Context, user, role string) (bool, error) //
	accessibleProjects []string
	// canAccessMap provides object+action specific overrides (format: "object:action" -> result)
	// If an object+action is in this map, its result is used instead of canAccessResult
	canAccessMap map[string]bool

	// getUserRolesResult returns these roles for any GetUserRoles call
	getUserRolesResult []string

	// AC-7: Track policy reload calls for testing immediate policy effect
	loadProjectPoliciesCalls    []*rbac.Project
	invalidateCacheForProjCalls []string
}

func (m *mockPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessErr != nil {
		return false, m.canAccessErr
	}
	// Check for object+action specific override
	if m.canAccessMap != nil {
		key := object + ":" + action
		if result, ok := m.canAccessMap[key]; ok {
			return result, nil
		}
	}
	return m.canAccessResult, nil
}

func (m *mockPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	if m.canAccessErr != nil {
		return m.canAccessErr
	}
	if !m.canAccessResult {
		return rbac.ErrAccessDenied
	}
	return nil
}

func (m *mockPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	m.loadProjectPoliciesCalls = append(m.loadProjectPoliciesCalls, project)
	return m.loadErr
}

func (m *mockPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	return nil
}

func (m *mockPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	return nil
}

func (m *mockPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return m.getUserRolesResult, nil
}

func (m *mockPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {

	if m.hasRoleFunc != nil {
		return m.hasRoleFunc(ctx, user, role)
	}
	return false, nil
}

func (m *mockPolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	return nil
}

func (m *mockPolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	return nil
}

func (m *mockPolicyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	return nil
}

func (m *mockPolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return m.removeErr
}

func (m *mockPolicyEnforcer) InvalidateCache() {
}

func (m *mockPolicyEnforcer) CacheStats() rbac.CacheStats {
	return rbac.CacheStats{}
}

// CanAccessWithGroups implements rbac.PolicyEnforcer
// For testing, delegates to CanAccess with the user - groups/roles are not used in these tests
func (m *mockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.CanAccess(ctx, user, object, action)
}

func (m *mockPolicyEnforcer) Metrics() rbac.PolicyMetrics {
	return rbac.PolicyMetrics{}
}

func (m *mockPolicyEnforcer) IncrementPolicyReloads() {
}

func (m *mockPolicyEnforcer) IncrementBackgroundSyncs() {
}

func (m *mockPolicyEnforcer) IncrementWatcherRestarts() {
}

// InvalidateCacheForUser implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcer) InvalidateCacheForUser(user string) int {
	return 0
}

// InvalidateCacheForProject implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcer) InvalidateCacheForProject(projectName string) int {
	m.invalidateCacheForProjCalls = append(m.invalidateCacheForProjCalls, projectName)
	return 1
}

// GetAccessibleProjects implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	return m.accessibleProjects, nil
}

// Helper to create request with user context
func newRequestWithUserContext(method, path string, body []byte, userCtx *middleware.UserContext) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("X-Request-ID", "test-request-id")
	if userCtx != nil {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
		req = req.WithContext(ctx)
	}
	return req
}

// projectTestSetup bundles the common dependencies for project handler tests.
type projectTestSetup struct {
	handler  *ProjectHandler
	service  *mockProjectService
	enforcer *mockPolicyEnforcer
	recorder *httptest.ResponseRecorder
}

// newProjectTestSetup creates a projectTestSetup with default admin access.
func newProjectTestSetup(t *testing.T) *projectTestSetup {
	t.Helper()
	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)
	return &projectTestSetup{
		handler:  handler,
		service:  svc,
		enforcer: enforcer,
		recorder: httptest.NewRecorder(),
	}
}

// makeRequest builds an *http.Request with the given user context.
// If no roles are provided, defaults to role:serveradmin.
func (s *projectTestSetup) makeRequest(t *testing.T, method, path string, body []byte, userID string, roles ...string) *http.Request {
	t.Helper()
	if len(roles) == 0 {
		roles = []string{"role:serveradmin"}
	}
	userCtx := &middleware.UserContext{UserID: userID, CasbinRoles: roles}
	return newRequestWithUserContext(method, path, body, userCtx)
}

// Test ListProjects

func TestProjectHandler_ListProjects_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := NewProjectHandler(newMockProjectService(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestProjectHandler_ListProjects_GlobalAdmin(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("project-a", rbac.ProjectSpec{Description: "Project A"})
	svc.addProject("project-b", rbac.ProjectSpec{Description: "Project B"})

	// accessibleProjects lists the projects the admin can see (via Casbin policies)
	enforcer := &mockPolicyEnforcer{
		canAccessResult:    true,
		accessibleProjects: []string{"project-a", "project-b"}, // Admin has access to all projects
	}
	handler := NewProjectHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
		// Authorization is handled by Casbin enforcer (single source of truth)
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ProjectListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.TotalCount != 2 {
		t.Errorf("expected 2 projects, got %d", listResp.TotalCount)
	}
}

func TestProjectHandler_ListProjects_NonAdmin_WithPolicyEnforcer(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("project-a", rbac.ProjectSpec{Description: "Project A"})
	svc.addProject("project-b", rbac.ProjectSpec{Description: "Project B"})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestProjectHandler_ListProjects_NonAdmin_NilEnforcerFailsClosed(t *testing.T) {
	t.Parallel()

	// Security test: Non-admin users should get zero projects when policy enforcer is nil
	// This tests the "fail closed" security behavior
	svc := newMockProjectService()
	svc.addProject("project-a", rbac.ProjectSpec{Description: "Project A"})
	svc.addProject("project-b", rbac.ProjectSpec{Description: "Project B"})

	// Nil policy enforcer - security should fail closed
	handler := NewProjectHandler(svc, nil, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListProjects(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ProjectListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Security: Non-admin without policy enforcer should see ZERO projects (fail closed)
	if listResp.TotalCount != 0 {
		t.Errorf("expected 0 projects (fail closed), got %d", listResp.TotalCount)
	}
}

func TestProjectHandler_ListProjects_ServiceError(t *testing.T) {
	t.Parallel()

	s := newProjectTestSetup(t)
	s.service.listErr = errors.New("database error")
	req := s.makeRequest(t, http.MethodGet, "/api/v1/projects", nil, "admin-user")

	s.handler.ListProjects(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// Test GetProject

func TestProjectHandler_GetProject_Success(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{
		Description: "Test project",
	})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/my-project", nil, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.GetProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if projectResp.Name != "my-project" {
		t.Errorf("expected project name 'my-project', got '%s'", projectResp.Name)
	}
	if projectResp.Description != "Test project" {
		t.Errorf("expected description 'Test project', got '%s'", projectResp.Description)
	}
}

func TestProjectHandler_GetProject_NotFound(t *testing.T) {
	t.Parallel()

	s := newProjectTestSetup(t)
	req := s.makeRequest(t, http.MethodGet, "/api/v1/projects/nonexistent", nil, "admin-user")
	req.SetPathValue("name", "nonexistent")

	s.handler.GetProject(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestProjectHandler_GetProject_MissingName(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(newMockProjectService(), enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/", nil, userCtx)
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()

	handler.GetProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestProjectHandler_GetProject_AccessDenied tests that access denied returns 403.
// Project admin attempting GET on other project returns 403 Forbidden.
func TestProjectHandler_GetProject_AccessDenied(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("secret-project", rbac.ProjectSpec{Description: "Secret"})

	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewProjectHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/secret-project", nil, userCtx)
	req.SetPathValue("name", "secret-project")
	rec := httptest.NewRecorder()

	handler.GetProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should return 403 Forbidden for authorization failures
	// (Changed from 404 to 403 for clearer authorization error feedback)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

// Test CreateProject

func TestProjectHandler_CreateProject_Success(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := CreateProjectRequest{
		Name:        "new-project",
		Description: "A new project",
		Destinations: []DestinationRequest{
			{Namespace: "default"},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if projectResp.Name != "new-project" {
		t.Errorf("expected project name 'new-project', got '%s'", projectResp.Name)
	}
}

func TestProjectHandler_CreateProject_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewProjectHandler(newMockProjectService(), enforcer, nil)

	reqBody := CreateProjectRequest{
		Name:        "new-project",
		Description: "A new project",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestProjectHandler_CreateProject_NilEnforcerReturns500(t *testing.T) {
	t.Parallel()

	handler := NewProjectHandler(newMockProjectService(), nil, nil)

	reqBody := CreateProjectRequest{
		Name:        "new-project",
		Description: "A new project",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "regular-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

func TestProjectHandler_CreateProject_InvalidName(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(newMockProjectService(), enforcer, nil)

	reqBody := CreateProjectRequest{
		Name:        "Invalid_Name!", // Invalid characters
		Description: "A new project",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Details["name"] == "" {
		t.Error("expected validation error for 'name' field")
	}
}

func TestProjectHandler_CreateProject_AlreadyExists(t *testing.T) {
	t.Parallel()

	s := newProjectTestSetup(t)
	s.service.addProject("existing-project", rbac.ProjectSpec{})

	reqBody := CreateProjectRequest{
		Name: "existing-project",
	}
	body, _ := json.Marshal(reqBody)
	req := s.makeRequest(t, http.MethodPost, "/api/v1/projects", body, "admin-user")

	s.handler.CreateProject(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestProjectHandler_CreateProject_MissingName(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(newMockProjectService(), enforcer, nil)

	reqBody := CreateProjectRequest{
		Description: "No name provided",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestProjectHandler_CreateProject_InvalidJSON(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(newMockProjectService(), enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", []byte("invalid json"), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test UpdateProject

func TestProjectHandler_UpdateProject_Success(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{Description: "Original"})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description:     "Updated description",
		ResourceVersion: "1",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if projectResp.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got '%s'", projectResp.Description)
	}
}

func TestProjectHandler_UpdateProject_NotFound(t *testing.T) {
	t.Parallel()

	s := newProjectTestSetup(t)
	reqBody := UpdateProjectRequest{
		Description:     "Updated",
		ResourceVersion: "1",
	}
	body, _ := json.Marshal(reqBody)
	req := s.makeRequest(t, http.MethodPut, "/api/v1/projects/nonexistent", body, "admin-user")
	req.SetPathValue("name", "nonexistent")

	s.handler.UpdateProject(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestProjectHandler_UpdateProject_VersionConflict(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{Description: "Original"})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description:     "Updated",
		ResourceVersion: "999", // Wrong version
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected status %d, got %d", http.StatusConflict, resp.StatusCode)
	}
}

func TestProjectHandler_UpdateProject_MissingResourceVersion(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description: "Updated",
		// Missing ResourceVersion
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestProjectHandler_UpdateProject_NonAdminWithAccess(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{Description: "Original"})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description:     "Updated by non-admin",
		ResourceVersion: "1",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "project-admin",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestProjectHandler_UpdateProject_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{})

	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description:     "Updated",
		ResourceVersion: "1",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

// SECURITY TEST: Verify that non-admins cannot update roles (privilege escalation prevention)
func TestProjectHandler_UpdateProject_NonAdminCannotUpdateRoles(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{
		Description: "Original",
		Roles: []rbac.ProjectRole{
			{Name: "admin", Policies: []string{"p, proj:my-project:admin, instances, *, my-project/*, allow"}},
		},
	})

	// Non-admin has project update access but is not a global admin
	// The mock allows updating the project itself, but denies modifying roles (projects/roles)
	enforcer := &mockPolicyEnforcer{
		canAccessResult: true, // Default: allow (for project update check)
		canAccessMap: map[string]bool{
			"projects/roles:update": false, // Deny role management (only global-admin has this)
		},
	}
	handler := NewProjectHandler(svc, enforcer, nil)

	// Attempt to update roles (should be forbidden for non-global-admins)
	reqBody := UpdateProjectRequest{
		Description:     "Updated",
		ResourceVersion: "1",
		Roles: []RoleRequest{
			{
				Name:     "admin",
				Policies: []string{"p, proj:my-project:admin, settings, *, *, allow"}, // Trying to escalate to settings access
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "project-admin",
		CasbinRoles: []string{"proj:my-project:admin"}, // Project admin, not global admin
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should be forbidden because only global admins can update roles
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d for role update by non-admin, got %d", http.StatusForbidden, resp.StatusCode)
	}

	// Verify error message
	var errResp response.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Message != "You do not have permission to modify project roles and policies" {
		t.Errorf("unexpected error message: %s", errResp.Message)
	}
}

// SECURITY TEST: Verify that global admins CAN update roles
func TestProjectHandler_UpdateProject_GlobalAdminCanUpdateRoles(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{
		Description: "Original",
		Roles: []rbac.ProjectRole{
			{Name: "admin", Policies: []string{"p, proj:my-project:admin, instances, *, my-project/*, allow"}},
		},
	})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	// Global admin updating roles (should succeed)
	reqBody := UpdateProjectRequest{
		Description:     "Updated",
		ResourceVersion: "1",
		Roles: []RoleRequest{
			{
				Name:     "admin",
				Policies: []string{"p, proj:my-project:admin, instances, *, my-project/*, allow"},
				Groups:   []string{"engineering-admins"},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "global-admin",
		CasbinRoles: []string{rbac.CasbinRoleServerAdmin}, // Global admin
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should succeed for global admin
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d for role update by global admin, got %d", http.StatusOK, resp.StatusCode)
	}
}

// Test DeleteProject

func TestProjectHandler_DeleteProject_Success(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("to-delete", rbac.ProjectSpec{})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/to-delete", nil, userCtx)
	req.SetPathValue("name", "to-delete")
	rec := httptest.NewRecorder()

	handler.DeleteProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify project was deleted
	if _, exists := svc.projects["to-delete"]; exists {
		t.Error("expected project to be deleted")
	}
}

func TestProjectHandler_DeleteProject_NotFound(t *testing.T) {
	t.Parallel()

	s := newProjectTestSetup(t)
	req := s.makeRequest(t, http.MethodDelete, "/api/v1/projects/nonexistent", nil, "admin-user")
	req.SetPathValue("name", "nonexistent")

	s.handler.DeleteProject(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestProjectHandler_DeleteProject_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("protected", rbac.ProjectSpec{})

	handler := NewProjectHandler(svc, nil, nil)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/protected", nil, userCtx)
	req.SetPathValue("name", "protected")
	rec := httptest.NewRecorder()

	handler.DeleteProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestProjectHandler_DeleteProject_MissingName(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(newMockProjectService(), enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodDelete, "/api/v1/projects/", nil, userCtx)
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()

	handler.DeleteProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// Test validation functions

func TestIsValidProjectName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid lowercase", "my-project", true},
		{"valid with numbers", "project-123", true},
		{"valid all numbers", "123", true},
		{"valid single char", "a", true},
		{"invalid uppercase", "My-Project", false},
		{"invalid underscore", "my_project", false},
		{"invalid special char", "my-project!", false},
		{"invalid starts with hyphen", "-my-project", false},
		{"invalid ends with hyphen", "my-project-", false},
		{"invalid empty", "", false},
		{"invalid too long", "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123", false}, // 64 chars
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isValidProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("isValidProjectName(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// Test response transformation

func TestToProjectResponse(t *testing.T) {
	t.Parallel()

	project := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-project",
			ResourceVersion:   "42",
			CreationTimestamp: metav1.Time{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			Annotations: map[string]string{
				"knodex.io/created-by": "creator",
				"knodex.io/updated-by": "updater",
				"knodex.io/updated-at": "2025-01-02T00:00:00Z",
			},
		},
		Spec: rbac.ProjectSpec{
			Description: "Test description",
			Destinations: []rbac.Destination{
				{Namespace: "default"},
			},
			Roles: []rbac.ProjectRole{
				{Name: "admin", Description: "Admin role", Policies: []string{"p,role:serveradmin,*,*,allow"}},
			},
		},
	}

	resp := toProjectResponse(project)

	if resp.Name != "test-project" {
		t.Errorf("expected name 'test-project', got '%s'", resp.Name)
	}
	if resp.Description != "Test description" {
		t.Errorf("expected description 'Test description', got '%s'", resp.Description)
	}
	if resp.ResourceVersion != "42" {
		t.Errorf("expected resourceVersion '42', got '%s'", resp.ResourceVersion)
	}
	if resp.CreatedBy != "creator" {
		t.Errorf("expected createdBy 'creator', got '%s'", resp.CreatedBy)
	}
	if resp.UpdatedBy != "updater" {
		t.Errorf("expected updatedBy 'updater', got '%s'", resp.UpdatedBy)
	}
	if len(resp.Destinations) != 1 {
		t.Errorf("expected 1 destination, got %d", len(resp.Destinations))
	}
	if len(resp.Roles) != 1 {
		t.Errorf("expected 1 role, got %d", len(resp.Roles))
	}
}

// Note: SanitizeJSONError tests have been moved to internal/api/helpers/request_test.go
// as part of the handler helpers refactoring.

// ============== AC-7: Policy Reload Tests ==============
// These tests verify that policy reload is triggered after successful project operations
// to ensure permission changes take effect immediately.

func TestProjectHandler_CreateProject_TriggersPolicyReload(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := CreateProjectRequest{
		Name:        "new-project",
		Description: "A new project",
		Destinations: []DestinationRequest{
			{Namespace: "default"},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	// AC-7: Verify LoadProjectPolicies was called
	if len(enforcer.loadProjectPoliciesCalls) != 1 {
		t.Errorf("expected LoadProjectPolicies to be called 1 time, got %d", len(enforcer.loadProjectPoliciesCalls))
	}
	if len(enforcer.loadProjectPoliciesCalls) > 0 && enforcer.loadProjectPoliciesCalls[0].Name != "new-project" {
		t.Errorf("expected LoadProjectPolicies to be called with project 'new-project', got '%s'", enforcer.loadProjectPoliciesCalls[0].Name)
	}

	// AC-4: InvalidateCacheForProject is NOT called separately — LoadProjectPolicies
	// already clears the cache internally (pe.cache.Clear() at line 818).
	if len(enforcer.invalidateCacheForProjCalls) != 0 {
		t.Errorf("expected InvalidateCacheForProject to NOT be called (LoadProjectPolicies handles cache), got %d calls", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestProjectHandler_UpdateProject_TriggersPolicyReload(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{Description: "Original"})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description:     "Updated description",
		ResourceVersion: "1",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// AC-7: Verify LoadProjectPolicies was called
	if len(enforcer.loadProjectPoliciesCalls) != 1 {
		t.Errorf("expected LoadProjectPolicies to be called 1 time, got %d", len(enforcer.loadProjectPoliciesCalls))
	}
	if len(enforcer.loadProjectPoliciesCalls) > 0 && enforcer.loadProjectPoliciesCalls[0].Name != "my-project" {
		t.Errorf("expected LoadProjectPolicies to be called with project 'my-project', got '%s'", enforcer.loadProjectPoliciesCalls[0].Name)
	}

	// AC-4: InvalidateCacheForProject is NOT called separately — LoadProjectPolicies
	// already clears the cache internally.
	if len(enforcer.invalidateCacheForProjCalls) != 0 {
		t.Errorf("expected InvalidateCacheForProject to NOT be called (LoadProjectPolicies handles cache), got %d calls", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestProjectHandler_UpdateProject_WithRoles_TriggersPolicyReload(t *testing.T) {
	t.Parallel()

	// AC-2: PUT /api/v1/projects/{name}/roles modifies roles - verify immediate reload
	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{
		Description: "Original",
		Roles: []rbac.ProjectRole{
			{Name: "admin", Description: "Admin role"},
		},
	})

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	// Update project with new role definitions
	reqBody := UpdateProjectRequest{
		Description:     "Updated with new roles",
		ResourceVersion: "1",
		Roles: []RoleRequest{
			{Name: "admin", Description: "Admin role"},
			{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:my-project:developer, instances, create, my-project/*, allow"}},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{rbac.CasbinRoleServerAdmin},
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// AC-7: Verify LoadProjectPolicies was called for role update
	if len(enforcer.loadProjectPoliciesCalls) != 1 {
		t.Errorf("expected LoadProjectPolicies to be called 1 time after role update, got %d", len(enforcer.loadProjectPoliciesCalls))
	}

	// AC-4: InvalidateCacheForProject is NOT called separately — LoadProjectPolicies
	// already clears the cache internally.
	if len(enforcer.invalidateCacheForProjCalls) != 0 {
		t.Errorf("expected InvalidateCacheForProject to NOT be called (LoadProjectPolicies handles cache), got %d calls", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestProjectHandler_PolicyReloadError_DoesNotFailRequest(t *testing.T) {
	t.Parallel()

	// AC-6: If policy reload fails, the error is logged but does not fail the API request
	svc := newMockProjectService()
	svc.addProject("my-project", rbac.ProjectSpec{Description: "Original"})

	enforcer := &mockPolicyEnforcer{
		canAccessResult: true,
		loadErr:         errors.New("policy reload failed"),
	}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := UpdateProjectRequest{
		Description:     "Updated description",
		ResourceVersion: "1",
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodPut, "/api/v1/projects/my-project", body, userCtx)
	req.SetPathValue("name", "my-project")
	rec := httptest.NewRecorder()

	handler.UpdateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// AC-6: Request should still succeed even though policy reload failed
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d (policy reload error should not fail request), got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the project was still updated
	var projectResp ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if projectResp.Description != "Updated description" {
		t.Errorf("expected project description to be updated, got '%s'", projectResp.Description)
	}

	// Verify LoadProjectPolicies was attempted
	if len(enforcer.loadProjectPoliciesCalls) != 1 {
		t.Errorf("expected LoadProjectPolicies to be called 1 time, got %d", len(enforcer.loadProjectPoliciesCalls))
	}

	// Verify InvalidateCacheForProject was NOT called since LoadProjectPolicies failed
	if len(enforcer.invalidateCacheForProjCalls) != 0 {
		t.Errorf("expected InvalidateCacheForProject to NOT be called when LoadProjectPolicies fails, got %d calls", len(enforcer.invalidateCacheForProjCalls))
	}
}

func TestProjectHandler_CreateProjectWithRoles(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := CreateProjectRequest{
		Name:        "my-project",
		Description: "Project with roles",
		Destinations: []DestinationRequest{
			{Namespace: "default"},
		},
		Roles: []RoleRequest{
			{
				Name:        "admin",
				Description: "Full project management access",
				Policies:    []string{"p, proj:my-project:admin, projects, *, my-project, allow"},
				Groups:      []string{"team-admins"},
			},
			{
				Name:        "developer",
				Description: "Deploy and manage instances",
				Policies:    []string{"p, proj:my-project:developer, instances, *, my-project/*, allow"},
				Groups:      []string{"team-devs"},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&projectResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if projectResp.Name != "my-project" {
		t.Errorf("expected project name 'my-project', got '%s'", projectResp.Name)
	}
	if len(projectResp.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(projectResp.Roles))
	}
	if projectResp.Roles[0].Name != "admin" {
		t.Errorf("expected first role name 'admin', got '%s'", projectResp.Roles[0].Name)
	}
	if projectResp.Roles[1].Name != "developer" {
		t.Errorf("expected second role name 'developer', got '%s'", projectResp.Roles[1].Name)
	}
	if len(projectResp.Roles[0].Groups) != 1 || projectResp.Roles[0].Groups[0] != "team-admins" {
		t.Errorf("expected admin role groups ['team-admins'], got %v", projectResp.Roles[0].Groups)
	}
}

func TestProjectHandler_CreateProjectWithInvalidRoles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		roles    []RoleRequest
		errField string
		errMsg   string
	}{
		{
			name: "empty role name",
			roles: []RoleRequest{
				{Name: "", Policies: []string{"p, proj:test:role, *, get, test/*, allow"}},
			},
			errField: "roles[0].name",
			errMsg:   "role name is required",
		},
		{
			name: "invalid role name format",
			roles: []RoleRequest{
				{Name: "INVALID_NAME!", Policies: []string{"p, proj:test:role, *, get, test/*, allow"}},
			},
			errField: "roles[0].name",
			errMsg:   "role name must be a valid DNS-1123 subdomain",
		},
		{
			name: "no policies",
			roles: []RoleRequest{
				{Name: "viewer", Policies: []string{}},
			},
			errField: "roles[0].policies",
			errMsg:   "at least one policy is required",
		},
		{
			name: "nil policies",
			roles: []RoleRequest{
				{Name: "viewer"},
			},
			errField: "roles[0].policies",
			errMsg:   "at least one policy is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockProjectService()
			enforcer := &mockPolicyEnforcer{canAccessResult: true}
			handler := NewProjectHandler(svc, enforcer, nil)

			reqBody := CreateProjectRequest{
				Name:  "test-project",
				Roles: tc.roles,
			}
			body, _ := json.Marshal(reqBody)

			userCtx := &middleware.UserContext{UserID: "admin-user"}
			req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
			rec := httptest.NewRecorder()

			handler.CreateProject(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
			}

			var errResp response.ErrorResponse
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			if errResp.Details[tc.errField] != tc.errMsg {
				t.Errorf("expected error '%s' for field '%s', got '%v'", tc.errMsg, tc.errField, errResp.Details[tc.errField])
			}
		})
	}
}

func TestProjectHandler_CreateProjectWithDuplicateRoleNames(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewProjectHandler(svc, enforcer, nil)

	reqBody := CreateProjectRequest{
		Name: "test-project",
		Roles: []RoleRequest{
			{Name: "admin", Policies: []string{"p, proj:test:admin, *, *, test/*, allow"}},
			{Name: "admin", Policies: []string{"p, proj:test:admin, projects, get, test, allow"}},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{UserID: "admin-user"}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects", body, userCtx)
	rec := httptest.NewRecorder()

	handler.CreateProject(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Details["roles[0].name"] != "duplicate role name: admin" {
		t.Errorf("expected duplicate role name error, got %v", errResp.Details["roles[0].name"])
	}
}
