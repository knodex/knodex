package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/rbac"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockProjectService implements rbac.ProjectServiceInterface for testing
type mockProjectService struct {
	getProjectFunc    func(ctx context.Context, name string) (*rbac.Project, error)
	listProjectsFunc  func(ctx context.Context) (*rbac.ProjectList, error)
	createProjectFunc func(ctx context.Context, name string, spec rbac.ProjectSpec, createdBy string) (*rbac.Project, error)
	updateProjectFunc func(ctx context.Context, project *rbac.Project, updatedBy string) (*rbac.Project, error)
	deleteProjectFunc func(ctx context.Context, name string) error
	existsFunc        func(ctx context.Context, name string) (bool, error)
}

func (m *mockProjectService) GetProject(ctx context.Context, name string) (*rbac.Project, error) {
	if m.getProjectFunc != nil {
		return m.getProjectFunc(ctx, name)
	}
	return nil, errors.New("project not found")
}

func (m *mockProjectService) ListProjects(ctx context.Context) (*rbac.ProjectList, error) {
	if m.listProjectsFunc != nil {
		return m.listProjectsFunc(ctx)
	}
	return &rbac.ProjectList{}, nil
}

func (m *mockProjectService) CreateProject(ctx context.Context, name string, spec rbac.ProjectSpec, createdBy string) (*rbac.Project, error) {
	if m.createProjectFunc != nil {
		return m.createProjectFunc(ctx, name, spec, createdBy)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProjectService) UpdateProject(ctx context.Context, project *rbac.Project, updatedBy string) (*rbac.Project, error) {
	if m.updateProjectFunc != nil {
		return m.updateProjectFunc(ctx, project, updatedBy)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProjectService) DeleteProject(ctx context.Context, name string) error {
	if m.deleteProjectFunc != nil {
		return m.deleteProjectFunc(ctx, name)
	}
	return errors.New("not implemented")
}

func (m *mockProjectService) Exists(ctx context.Context, name string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, name)
	}
	if m.getProjectFunc != nil {
		_, err := m.getProjectFunc(ctx, name)
		return err == nil, nil
	}
	return false, nil
}

// mockPolicyEnforcer implements rbac.PolicyEnforcer for testing
type mockPolicyEnforcer struct {
	canAccessFunc            func(ctx context.Context, user, object, action string) (bool, error)
	canAccessWithGroupsFunc  func(ctx context.Context, user string, groups []string, object, action string) (bool, error)
	enforceProjectAccessFunc func(ctx context.Context, user, projectName, action string) error
	loadProjectPoliciesFunc  func(ctx context.Context, project *rbac.Project) error
	syncPoliciesFunc         func(ctx context.Context) error
	hasRoleFunc              func(ctx context.Context, user, role string) (bool, error) //
}

func (m *mockPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	if m.canAccessFunc != nil {
		return m.canAccessFunc(ctx, user, object, action)
	}

	// Tests that need admin bypass must explicitly set canAccessFunc
	return false, nil
}

func (m *mockPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	if m.enforceProjectAccessFunc != nil {
		return m.enforceProjectAccessFunc(ctx, user, projectName, action)
	}
	return nil
}

func (m *mockPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *rbac.Project) error {
	if m.loadProjectPoliciesFunc != nil {
		return m.loadProjectPoliciesFunc(ctx, project)
	}
	return nil
}

func (m *mockPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	if m.syncPoliciesFunc != nil {
		return m.syncPoliciesFunc(ctx)
	}
	return nil
}

func (m *mockPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	return nil
}

func (m *mockPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return nil, nil
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
	return nil
}

func (m *mockPolicyEnforcer) InvalidateCache() {
}

func (m *mockPolicyEnforcer) CacheStats() rbac.CacheStats {
	return rbac.CacheStats{}
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

// CanAccessWithGroups implements rbac.PolicyEnforcer
// For testing, delegates to canAccessWithGroupsFunc if set, otherwise CanAccess
func (m *mockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	if m.canAccessWithGroupsFunc != nil {
		return m.canAccessWithGroupsFunc(ctx, user, groups, object, action)
	}
	return m.CanAccess(ctx, user, object, action)
}

// InvalidateCacheForUser implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcer) InvalidateCacheForUser(user string) int {
	return 0
}

// InvalidateCacheForProject implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcer) InvalidateCacheForProject(projectName string) int {
	return 0
}

// GetAccessibleProjects implements rbac.PolicyEnforcer
func (m *mockPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	return nil, nil
}

// ============================================================================
// DeploymentValidator Middleware Tests
// ============================================================================

func TestDeploymentValidator_NonPostRequest_Passthrough(t *testing.T) {
	t.Parallel()

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		Logger: slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Test GET request - should pass through
	req := httptest.NewRequest("GET", "/api/v1/instances", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for GET request, got %d", w.Code)
	}
}

func TestDeploymentValidator_NoProjectID_Passthrough(t *testing.T) {
	t.Parallel()

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		Logger: slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Test POST request with no projectId - should pass through
	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for request without projectId, got %d", w.Code)
	}
}

func TestDeploymentValidator_InvalidJSON_Error(t *testing.T) {
	t.Parallel()

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		Logger: slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid JSON")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Invalid JSON body
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeploymentValidator_NoUserContext_Unauthorized(t *testing.T) {
	t.Parallel()

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		Logger: slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no user context")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// POST request with projectId but no user context
	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestDeploymentValidator_GlobalAdmin_Bypass(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify project is stored in context
		project, ok := GetValidatedProject(r)
		if !ok {
			t.Error("expected validated project in context")
		}
		if project.Name != "engineering" {
			t.Errorf("expected project name 'engineering', got '%s'", project.Name)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	// Set global admin user context
	userCtx := &UserContext{
		UserID:      "admin-123",
		Email:       "admin@example.com",
		CasbinRoles: []string{"role:serveradmin"},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for global admin, got %d", w.Code)
	}
}

func TestDeploymentValidator_ProjectNotFound(t *testing.T) {
	t.Parallel()

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			return nil, errors.New("project not found")
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when project not found")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"nonexistent"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	// Verify error response
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != ErrCodeProjectNotFound {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeProjectNotFound, errObj["code"])
	}
}

func TestDeploymentValidator_PermissionDenied(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// Deny deploy permission
			return false, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when permission denied")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != ErrCodePermissionDenied {
		t.Errorf("expected error code '%s', got '%s'", ErrCodePermissionDenied, errObj["code"])
	}
}

func TestDeploymentValidator_DestinationNotAllowed(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
			Destinations: []rbac.Destination{
				{Namespace: "production"},
				{Namespace: "staging"},
			},
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// Return false for admin check - user is NOT admin
			if object == "*" && action == "*" {
				return false, nil
			}
			// Allow deploy permission for specific project
			if object == "instances/engineering/*" && action == "create" {
				return true, nil
			}
			return false, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when destination not allowed")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Request with disallowed namespace
	body := `{"name":"test-instance","namespace":"kube-system","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != ErrCodeDestinationNotAllowed {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeDestinationNotAllowed, errObj["code"])
	}
}

func TestDeploymentValidator_DestinationAllowed(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
			Destinations: []rbac.Destination{
				{Namespace: "production"},
				{Namespace: "staging"},
			},
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return true, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Request with allowed namespace
	body := `{"name":"test-instance","namespace":"production","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for allowed destination, got %d", w.Code)
	}
}

func TestDeploymentValidator_DestinationWildcard_Namespace(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
			Destinations: []rbac.Destination{
				{Namespace: "*"},
			},
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return true, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	// Request with any namespace (should be allowed by wildcard)
	body := `{"name":"test-instance","namespace":"any-namespace","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for wildcard namespace, got %d", w.Code)
	}
}

func TestDeploymentValidator_RequestBodyRestored(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return true, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify body is still readable
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body in handler: %v", err)
		}

		var req DeploymentRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			t.Errorf("failed to unmarshal body in handler: %v", err)
		}

		if req.Name != "test-instance" {
			t.Errorf("expected name 'test-instance', got '%s'", req.Name)
		}
		if req.Namespace != "default" {
			t.Errorf("expected namespace 'default', got '%s'", req.Namespace)
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDeploymentValidator_PermissionCheckError(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			return false, errors.New("database connection error")
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when permission check errors")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestDeploymentValidator_NoProjectServiceConfigured(t *testing.T) {
	t.Parallel()

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: nil, // No project service configured
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when project service not configured")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// ============================================================================
// Context Helper Tests
// ============================================================================

func TestGetValidatedDeploymentRequest_Success(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/instances", nil)

	deployReq := &DeploymentRequest{
		Name:      "test-instance",
		Namespace: "default",
		ProjectID: "engineering",
		RGDName:   "test-rgd",
	}
	ctx := context.WithValue(req.Context(), deploymentRequestContextKey{}, deployReq)
	req = req.WithContext(ctx)

	result, ok := GetValidatedDeploymentRequest(req)
	if !ok {
		t.Error("expected to get validated deployment request from context")
	}
	if result.Name != "test-instance" {
		t.Errorf("expected name 'test-instance', got '%s'", result.Name)
	}
}

func TestGetValidatedDeploymentRequest_NotFound(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/instances", nil)

	_, ok := GetValidatedDeploymentRequest(req)
	if ok {
		t.Error("expected not to find validated deployment request in context")
	}
}

func TestGetValidatedProject_Success(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/instances", nil)

	project := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
		},
	}
	ctx := context.WithValue(req.Context(), validatedProjectContextKey{}, project)
	req = req.WithContext(ctx)

	result, ok := GetValidatedProject(req)
	if !ok {
		t.Error("expected to get validated project from context")
	}
	if result.Name != "engineering" {
		t.Errorf("expected name 'engineering', got '%s'", result.Name)
	}
}

func TestGetValidatedProject_NotFound(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/api/v1/instances", nil)

	_, ok := GetValidatedProject(req)
	if ok {
		t.Error("expected not to find validated project in context")
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestDeploymentValidator_FullValidation_Success(t *testing.T) {
	t.Parallel()

	mockProject := &rbac.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "engineering"},
		Spec: rbac.ProjectSpec{
			Description: "Engineering Project",
			Destinations: []rbac.Destination{
				{Namespace: "production"},
				{Namespace: "staging"},
			},
		},
	}

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			if name == "engineering" {
				return mockProject, nil
			}
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{

		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// User is NOT admin - admin check should fail
			if object == "*" && action == "*" {
				return false, nil
			}
			return false, nil
		},

		canAccessWithGroupsFunc: func(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
			// Verify correct parameters
			// Fixed to check instances/{projectId}/* with create action (matching Casbin policy)
			if user != "user-123" {
				t.Errorf("expected user 'user-123', got '%s'", user)
			}
			if object != "instances/engineering/*" {
				t.Errorf("expected object 'instances/engineering/*', got '%s'", object)
			}
			if action != "create" {
				t.Errorf("expected action 'create', got '%s'", action)
			}
			return true, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request and project are in context
		deployReq, ok := GetValidatedDeploymentRequest(r)
		if !ok {
			t.Error("expected validated deployment request in context")
		}
		if deployReq.ProjectID != "engineering" {
			t.Errorf("expected projectId 'engineering', got '%s'", deployReq.ProjectID)
		}

		project, ok := GetValidatedProject(r)
		if !ok {
			t.Error("expected validated project in context")
		}
		if project.Name != "engineering" {
			t.Errorf("expected project name 'engineering', got '%s'", project.Name)
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"production","rgdName":"test-rgd","projectId":"engineering"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "user-123",
		Email:       "user@example.com",
		CasbinRoles: []string{},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDeploymentValidator_GlobalAdmin_ProjectNotFound(t *testing.T) {
	t.Parallel()

	mockService := &mockProjectService{
		getProjectFunc: func(ctx context.Context, name string) (*rbac.Project, error) {
			return nil, errors.New("project not found")
		},
	}

	mockEnforcer := &mockPolicyEnforcer{
		canAccessFunc: func(ctx context.Context, user, object, action string) (bool, error) {
			// Global admin has wildcard permission
			if user == "admin-123" && object == "*" && action == "*" {
				return true, nil
			}
			return false, nil
		},
	}

	middleware := DeploymentValidator(DeploymentValidatorConfig{
		ProjectService: mockService,
		PolicyEnforcer: mockEnforcer,
		Logger:         slog.Default(),
	})

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when project not found even for global admin")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(testHandler)

	body := `{"name":"test-instance","namespace":"default","rgdName":"test-rgd","projectId":"nonexistent"}`
	req := httptest.NewRequest("POST", "/api/v1/instances", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &UserContext{
		UserID:      "admin-123",
		Email:       "admin@example.com",
		CasbinRoles: []string{"role:serveradmin"},
	}
	ctx := context.WithValue(req.Context(), UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Even global admin should get 404 for non-existent project
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for non-existent project, got %d", w.Code)
	}
}
