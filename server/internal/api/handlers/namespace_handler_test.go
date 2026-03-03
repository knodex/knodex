package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/rbac"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// mockNamespaceProjectService is a mock implementation of rbac.ProjectServiceInterface for namespace tests
type mockNamespaceProjectService struct {
	projects map[string]*rbac.Project
	getErr   error
}

func newMockNamespaceProjectService() *mockNamespaceProjectService {
	return &mockNamespaceProjectService{
		projects: make(map[string]*rbac.Project),
	}
}

func (m *mockNamespaceProjectService) CreateProject(ctx context.Context, name string, spec rbac.ProjectSpec, createdBy string) (*rbac.Project, error) {
	return nil, nil
}

func (m *mockNamespaceProjectService) GetProject(ctx context.Context, name string) (*rbac.Project, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	project, exists := m.projects[name]
	if !exists {
		return nil, rbac.ErrProjectNotFound
	}
	return project, nil
}

func (m *mockNamespaceProjectService) ListProjects(ctx context.Context) (*rbac.ProjectList, error) {
	return nil, nil
}

func (m *mockNamespaceProjectService) UpdateProject(ctx context.Context, project *rbac.Project, updatedBy string) (*rbac.Project, error) {
	return nil, nil
}

func (m *mockNamespaceProjectService) DeleteProject(ctx context.Context, name string) error {
	return nil
}

func (m *mockNamespaceProjectService) Exists(ctx context.Context, name string) (bool, error) {
	_, exists := m.projects[name]
	return exists, nil
}

func (m *mockNamespaceProjectService) addProject(name string, spec rbac.ProjectSpec) *rbac.Project {
	project := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	m.projects[name] = project
	return project
}

// mockNamespacePolicyEnforcer is a mock implementation of rbac.PolicyEnforcer for namespace tests
type mockNamespacePolicyEnforcer struct {
	canAccessResult      bool
	canAccessErr         error
	accessibleProjects   []string
	accessibleProjectErr error
}

func (m *mockNamespacePolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessErr != nil {
		return false, m.canAccessErr
	}
	return m.canAccessResult, nil
}

// CanAccessWithGroups implements rbac.PolicyEnforcer
func (m *mockNamespacePolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.CanAccess(ctx, user, object, action)
}

func (m *mockNamespacePolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) SyncPolicies(ctx context.Context) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return nil, nil
}

func (m *mockNamespacePolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	return false, nil
}

func (m *mockNamespacePolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return nil
}

func (m *mockNamespacePolicyEnforcer) InvalidateCache() {
}

func (m *mockNamespacePolicyEnforcer) CacheStats() rbac.CacheStats {
	return rbac.CacheStats{}
}

func (m *mockNamespacePolicyEnforcer) Metrics() rbac.PolicyMetrics {
	return rbac.PolicyMetrics{}
}

func (m *mockNamespacePolicyEnforcer) IncrementPolicyReloads() {
}

func (m *mockNamespacePolicyEnforcer) IncrementBackgroundSyncs() {
}

func (m *mockNamespacePolicyEnforcer) IncrementWatcherRestarts() {
}

func (m *mockNamespacePolicyEnforcer) InvalidateCacheForUser(user string) int {
	return 0
}

func (m *mockNamespacePolicyEnforcer) InvalidateCacheForProject(projectName string) int {
	return 0
}

// GetAccessibleProjects implements rbac.PolicyEnforcer
func (m *mockNamespacePolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	if m.accessibleProjectErr != nil {
		return nil, m.accessibleProjectErr
	}
	return m.accessibleProjects, nil
}

// createTestNamespaceService creates a NamespaceService with fake k8s client and mock project service
func createTestNamespaceService(projectService rbac.ProjectServiceInterface) *rbac.NamespaceService {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-node-lease"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team1"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev-team2"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "staging"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "production"}},
	)
	return rbac.NewNamespaceService(fakeClient, projectService)
}

// Test ListNamespaces

func TestNamespaceHandler_ListNamespaces_Unauthorized(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	rec := httptest.NewRecorder()

	handler.ListNamespaces(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestNamespaceHandler_ListNamespaces_Success(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "test-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListNamespaces(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should exclude system namespaces by default
	for _, ns := range listResp.Namespaces {
		if ns == "kube-system" || ns == "kube-public" || ns == "kube-node-lease" {
			t.Errorf("system namespace %s should be excluded by default", ns)
		}
	}

	// Should include regular namespaces
	found := map[string]bool{}
	for _, ns := range listResp.Namespaces {
		found[ns] = true
	}

	if !found["default"] {
		t.Error("expected 'default' namespace in response")
	}
	if !found["dev-team1"] {
		t.Error("expected 'dev-team1' namespace in response")
	}
	if !found["staging"] {
		t.Error("expected 'staging' namespace in response")
	}
}

func TestNamespaceHandler_ListNamespaces_IncludeSystem(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "test-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces?exclude_system=false", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListNamespaces(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should include system namespaces when exclude_system=false
	found := map[string]bool{}
	for _, ns := range listResp.Namespaces {
		found[ns] = true
	}

	if !found["kube-system"] {
		t.Error("expected 'kube-system' namespace in response when exclude_system=false")
	}
}

// Test ListProjectNamespaces

func TestNamespaceHandler_ListProjectNamespaces_Unauthorized(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	// Use Go 1.22+ request with path value
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/alpha/namespaces", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestNamespaceHandler_ListProjectNamespaces_GlobalAdmin(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	projectSvc.addProject("alpha", rbac.ProjectSpec{
		Description: "Alpha Project",
		Destinations: []rbac.Destination{
			{Namespace: "dev-*"},
			{Namespace: "staging"},
		},
	})

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}

	// Use Go 1.22+ request with path value
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/alpha/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should match dev-* and staging patterns
	found := map[string]bool{}
	for _, ns := range listResp.Namespaces {
		found[ns] = true
	}

	if !found["dev-team1"] {
		t.Error("expected 'dev-team1' namespace in response")
	}
	if !found["dev-team2"] {
		t.Error("expected 'dev-team2' namespace in response")
	}
	if !found["staging"] {
		t.Error("expected 'staging' namespace in response")
	}

	// Should not include production (not in patterns)
	if found["production"] {
		t.Error("should not include 'production' namespace (not in patterns)")
	}
}

func TestNamespaceHandler_ListProjectNamespaces_NonAdmin_HasAccess(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	projectSvc.addProject("alpha", rbac.ProjectSpec{
		Description: "Alpha Project",
		Destinations: []rbac.Destination{
			{Namespace: "staging"},
		},
	})

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true} // User has access
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
		Groups:      []string{"alpha-viewers"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/alpha/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(listResp.Namespaces) != 1 || listResp.Namespaces[0] != "staging" {
		t.Errorf("expected only 'staging', got %v", listResp.Namespaces)
	}
}

func TestNamespaceHandler_ListProjectNamespaces_NonAdmin_NoAccess(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	projectSvc.addProject("alpha", rbac.ProjectSpec{
		Description: "Alpha Project",
		Destinations: []rbac.Destination{
			{Namespace: "staging"},
		},
	})

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: false} // User does NOT have access
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "unauthorized-user",
		CasbinRoles: []string{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/alpha/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestNamespaceHandler_ListProjectNamespaces_ProjectNotFound(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	// Don't add project - simulate not found

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/nonexistent/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestNamespaceHandler_ListProjectNamespaces_FullWildcard(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	projectSvc.addProject("global", rbac.ProjectSpec{
		Description: "Global Project",
		Destinations: []rbac.Destination{
			{Namespace: "*"},
		},
	})

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/global/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should return all non-system namespaces
	found := map[string]bool{}
	for _, ns := range listResp.Namespaces {
		found[ns] = true
	}

	expectedNamespaces := []string{"default", "dev-team1", "dev-team2", "staging", "production"}
	for _, expected := range expectedNamespaces {
		if !found[expected] {
			t.Errorf("expected '%s' namespace in response with wildcard", expected)
		}
	}

	// Should not include system namespaces
	if found["kube-system"] {
		t.Error("should not include 'kube-system' (system namespace)")
	}
}

func TestNamespaceHandler_ListProjectNamespaces_EmptyDestinations(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	projectSvc.addProject("empty", rbac.ProjectSpec{
		Description:  "Empty Project",
		Destinations: []rbac.Destination{}, // No destinations
	})

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/empty/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(listResp.Namespaces) != 0 {
		t.Errorf("expected empty list, got %v", listResp.Namespaces)
	}
}

// Tests for non-admin ListNamespaces path (STORY-235 AC-2: project-scoped namespace filtering)

func TestNamespaceHandler_ListNamespaces_NonAdmin_FilteredByProject(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	projectSvc.addProject("alpha", rbac.ProjectSpec{
		Destinations: []rbac.Destination{
			{Namespace: "dev-*"},
			{Namespace: "staging"},
		},
	})

	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{
		canAccessResult:    false, // Not admin (settings/*:get denied)
		accessibleProjects: []string{"alpha"},
	}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "regular-user",
		CasbinRoles: []string{},
		Groups:      []string{"alpha-viewers"},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListNamespaces(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	found := map[string]bool{}
	for _, ns := range listResp.Namespaces {
		found[ns] = true
	}

	// Should see namespaces matching alpha's dev-* and staging patterns
	if !found["dev-team1"] {
		t.Error("expected 'dev-team1' namespace for project alpha member")
	}
	if !found["dev-team2"] {
		t.Error("expected 'dev-team2' namespace for project alpha member")
	}
	if !found["staging"] {
		t.Error("expected 'staging' namespace for project alpha member")
	}

	// Should NOT see namespaces outside project patterns
	if found["production"] {
		t.Error("should not see 'production' (not in alpha destinations)")
	}
	if found["default"] {
		t.Error("should not see 'default' (not in alpha destinations)")
	}
}

func TestNamespaceHandler_ListNamespaces_NonAdmin_NoProjects(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{
		canAccessResult:    false, // Not admin
		accessibleProjects: []string{},
	}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "no-project-user",
		CasbinRoles: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/namespaces", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListNamespaces(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp NamespaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(listResp.Namespaces) != 0 {
		t.Errorf("expected empty namespace list for user with no projects, got %v", listResp.Namespaces)
	}
}

// SECURITY (H-2): Test input validation for project name parameter
func TestNamespaceHandler_ListProjectNamespaces_InvalidProjectName(t *testing.T) {
	t.Parallel()
	projectSvc := newMockNamespaceProjectService()
	namespaceService := createTestNamespaceService(projectSvc)
	enforcer := &mockNamespacePolicyEnforcer{canAccessResult: true}
	handler := NewNamespaceHandler(namespaceService, enforcer)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/projects/{name}/namespaces", handler.ListProjectNamespaces)

	// Test cases with invalid project names (DNS-1123 violations)
	// Note: Path traversal, special characters, and spaces are handled by the HTTP router
	// before reaching the handler, so we only test cases that pass routing but fail validation
	testCases := []struct {
		name        string
		projectName string
	}{
		{"uppercase letters", "Alpha-Project"},
		{"starts with hyphen", "-alpha"},
		{"ends with hyphen", "alpha-"},
		{"contains underscore", "alpha_project"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := newRequestWithUserContext(http.MethodGet, "/api/v1/projects/"+tc.projectName+"/namespaces", nil, userCtx)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected status %d for invalid project name %q, got %d", http.StatusBadRequest, tc.projectName, resp.StatusCode)
			}
		})
	}
}
