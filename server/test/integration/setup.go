// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/knodex/knodex/server/internal/api/handlers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/testutil"
)

const (
	// JWTSecretTest is a test JWT secret for integration tests
	JWTSecretTest = "test-jwt-secret-for-integration-tests-32bytes"
)

// TestServer wraps an HTTP test server and its dependencies for integration testing
type TestServer struct {
	Server   *httptest.Server
	Client   *http.Client
	Enforcer *MockPolicyEnforcer
	Auth     *MockAuthService
	// ProjectServiceReal is the actual project service connected to fake k8s client
	ProjectService rbac.ProjectServiceInterface
	// Keep reference to k8s client for direct manipulation if needed
	K8sClient *dynamicfake.FakeDynamicClient
	t         *testing.T
}

// NewTestServer creates a new test server with all dependencies configured
func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	// Create fake k8s clients
	k8sClient := testutil.NewFakeClientset(t)

	// Register custom list kinds for the fake dynamic client
	// This is required for List operations on CRD resources
	gvrToListKind := map[schema.GroupVersionResource]string{
		ProjectGVR: "ProjectList",
	}
	dynamicClient := testutil.NewFakeDynamicClientWithListKinds(t, gvrToListKind)

	// Create mock services
	enforcer := NewMockPolicyEnforcer()
	authService := NewMockAuthService(JWTSecretTest)

	// Create real project service with fake k8s clients
	projectService := rbac.NewProjectService(k8sClient, dynamicClient)

	// Create handlers
	projectHandler := handlers.NewProjectHandler(projectService, enforcer, nil)
	validationHandler := handlers.NewValidationHandler(projectService, enforcer)
	roleBindingHandler := handlers.NewRoleBindingHandler(projectService, enforcer, nil)

	// Create router
	mux := http.NewServeMux()

	// Register project routes
	mux.HandleFunc("GET /api/v1/projects", projectHandler.ListProjects)
	mux.HandleFunc("GET /api/v1/projects/{name}", projectHandler.GetProject)
	mux.HandleFunc("POST /api/v1/projects", projectHandler.CreateProject)
	mux.HandleFunc("PUT /api/v1/projects/{name}", projectHandler.UpdateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{name}", projectHandler.DeleteProject)

	// Register validation routes
	mux.HandleFunc("POST /api/v1/projects/validate", validationHandler.ValidateProjectCreation)
	mux.HandleFunc("POST /api/v1/projects/{name}/validate", validationHandler.ValidateProjectUpdate)

	// Register role binding routes
	mux.HandleFunc("POST /api/v1/projects/{name}/roles/{role}/users/{user}", roleBindingHandler.AssignUserRole)
	mux.HandleFunc("POST /api/v1/projects/{name}/roles/{role}/groups/{group}", roleBindingHandler.AssignGroupRole)
	mux.HandleFunc("GET /api/v1/projects/{name}/role-bindings", roleBindingHandler.ListRoleBindings)
	mux.HandleFunc("DELETE /api/v1/projects/{name}/roles/{role}/users/{user}", roleBindingHandler.RemoveUserRole)
	mux.HandleFunc("DELETE /api/v1/projects/{name}/roles/{role}/groups/{group}", roleBindingHandler.RemoveGroupRole)

	// Wrap with middleware
	var handler http.Handler = mux

	// Apply auth middleware
	handler = middleware.Auth(middleware.AuthConfig{
		AuthService: authService,
	})(handler)

	// Apply request ID middleware
	handler = middleware.RequestID(handler)

	// Create test server
	server := httptest.NewServer(handler)

	return &TestServer{
		Server:         server,
		Client:         server.Client(),
		Enforcer:       enforcer,
		Auth:           authService,
		ProjectService: projectService,
		K8sClient:      dynamicClient,
		t:              t,
	}
}

// Close shuts down the test server
func (ts *TestServer) Close() {
	ts.Server.Close()
}

// Reset clears all mock state for a fresh test
func (ts *TestServer) Reset() {
	ts.Enforcer.Reset()
	ts.Auth.Reset()
}

// URL returns the test server's base URL
func (ts *TestServer) URL() string {
	return ts.Server.URL
}

// AddGlobalAdmin registers a global admin user and returns their token

func (ts *TestServer) AddGlobalAdmin(userID, email string) string {
	token := "token-admin-" + userID
	claims := CreateTestClaims(userID, email, "Global Admin", []string{}, "", true)
	ts.Auth.AddValidToken(token, claims)

	// This is required because handlers now use policyEnforcer.CanAccess("*", "*") for admin checks
	ts.Enforcer.Allow(userID, "*", "*")

	// Also assign the role for any code that still uses HasRole checks
	ts.Enforcer.AssignUserRoles(context.Background(), userID, []string{"role:serveradmin"})

	return token
}

// AddUser registers a regular user and returns their token
func (ts *TestServer) AddUser(userID, email string, projects []string, defaultProject string) string {
	token := "token-user-" + userID
	claims := CreateTestClaims(userID, email, "Test User", projects, defaultProject, false)
	ts.Auth.AddValidToken(token, claims)
	return token
}

// AllowUserAccess grants a user access to a project for a specific action
// The object format is "projects/{projectName}" to match Casbin policy format used by handlers
func (ts *TestServer) AllowUserAccess(userID, projectName, action string) {
	// Use "projects/{projectName}" format to match handler's authorization checks
	// e.g., handler checks: CanAccessWithGroups(ctx, userID, groups, "projects/my-project", "get")
	projectObject := "projects/" + projectName
	ts.Enforcer.Allow(userID, projectObject, action)
}

// Request makes an HTTP request to the test server
func (ts *TestServer) Request(method, path string, body interface{}, token string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, ts.URL()+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return ts.Client.Do(req)
}

// RequestJSON makes an HTTP request and decodes the JSON response
func (ts *TestServer) RequestJSON(method, path string, body interface{}, token string, result interface{}) (*http.Response, error) {
	resp, err := ts.Request(method, path, body, token)
	if err != nil {
		return nil, err
	}

	if result != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return resp, err
		}
	}

	return resp, nil
}

// CreateProjectDirectly creates a project in the fake k8s client directly
// Useful for setting up test fixtures without going through the API.
// Note: fake dynamic client does not auto-generate resourceVersion, so
// we manually set "1" as the initial resourceVersion for testing purposes.
func (ts *TestServer) CreateProjectDirectly(name, description string, roles []rbac.ProjectRole) (*rbac.Project, error) {
	ctx := context.Background()

	// Create unstructured project with resourceVersion preset
	// This is needed because fake dynamic client doesn't auto-generate resourceVersion
	// Note: All slices must use []interface{} for k8s deep copy to work
	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            name,
				"resourceVersion": "1", // Preset for testing
				"labels": map[string]interface{}{
					"knodex.io/created-by": "test-setup",
				},
			},
			"spec": map[string]interface{}{
				"description":  description,
				"destinations": []interface{}{map[string]interface{}{"namespace": name}},
			},
		},
	}

	// Add roles if provided (convert all slices to []interface{})
	if len(roles) > 0 {
		rolesSlice := make([]interface{}, 0, len(roles))
		for _, role := range roles {
			// Convert policies []string to []interface{}
			policiesSlice := make([]interface{}, 0, len(role.Policies))
			for _, p := range role.Policies {
				policiesSlice = append(policiesSlice, p)
			}

			roleMap := map[string]interface{}{
				"name":        role.Name,
				"description": role.Description,
				"policies":    policiesSlice,
			}

			// Convert groups []string to []interface{}
			if len(role.Groups) > 0 {
				groupsSlice := make([]interface{}, 0, len(role.Groups))
				for _, g := range role.Groups {
					groupsSlice = append(groupsSlice, g)
				}
				roleMap["groups"] = groupsSlice
			}
			rolesSlice = append(rolesSlice, roleMap)
		}
		spec := project.Object["spec"].(map[string]interface{})
		spec["roles"] = rolesSlice
	}

	// Create directly in fake k8s client
	result, err := ts.K8sClient.Resource(ProjectGVR).Create(ctx, project, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	ts.Enforcer.RegisterProject(name)

	// Convert to Project struct
	return ts.unstructuredToProject(result)
}

// unstructuredToProject converts an unstructured object to a Project
func (ts *TestServer) unstructuredToProject(obj *unstructured.Unstructured) (*rbac.Project, error) {
	project := &rbac.Project{}
	project.ObjectMeta.Name = obj.GetName()
	project.ObjectMeta.Namespace = obj.GetNamespace()
	project.ObjectMeta.ResourceVersion = obj.GetResourceVersion()

	if spec, ok := obj.Object["spec"].(map[string]interface{}); ok {
		if desc, ok := spec["description"].(string); ok {
			project.Spec.Description = desc
		}
		if dests, ok := spec["destinations"].([]interface{}); ok {
			for _, d := range dests {
				if dm, ok := d.(map[string]interface{}); ok {
					dest := rbac.Destination{}
					if ns, ok := dm["namespace"].(string); ok {
						dest.Namespace = ns
					}
					if n, ok := dm["name"].(string); ok {
						dest.Name = n
					}
					project.Spec.Destinations = append(project.Spec.Destinations, dest)
				}
			}
		}
		if roles, ok := spec["roles"].([]interface{}); ok {
			for _, r := range roles {
				if rm, ok := r.(map[string]interface{}); ok {
					role := rbac.ProjectRole{}
					if n, ok := rm["name"].(string); ok {
						role.Name = n
					}
					if d, ok := rm["description"].(string); ok {
						role.Description = d
					}
					if p, ok := rm["policies"].([]interface{}); ok {
						for _, pol := range p {
							if ps, ok := pol.(string); ok {
								role.Policies = append(role.Policies, ps)
							}
						}
					}
					if g, ok := rm["groups"].([]interface{}); ok {
						for _, grp := range g {
							if gs, ok := grp.(string); ok {
								role.Groups = append(role.Groups, gs)
							}
						}
					}
					project.Spec.Roles = append(project.Spec.Roles, role)
				}
			}
		}
	}

	return project, nil
}

// GetProjectDirectly retrieves a project from the fake k8s client directly
func (ts *TestServer) GetProjectDirectly(name string) (*rbac.Project, error) {
	ctx := context.Background()
	return ts.ProjectService.GetProject(ctx, name)
}

// DeleteProjectDirectly deletes a project from the fake k8s client directly
func (ts *TestServer) DeleteProjectDirectly(name string) error {
	ctx := context.Background()
	return ts.ProjectService.DeleteProject(ctx, name)
}

// AssertStatus checks that the response has the expected status code
func (ts *TestServer) AssertStatus(resp *http.Response, expected int) {
	ts.t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		ts.t.Errorf("expected status %d, got %d: %s", expected, resp.StatusCode, string(body))
	}
}

// ReadBody reads and returns the response body as bytes
func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// DecodeJSON decodes a response body into the given type
func DecodeJSON[T any](resp *http.Response) (T, error) {
	var result T
	defer resp.Body.Close()
	err := json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

// ProjectGVR is the GroupVersionResource for Project CRDs
var ProjectGVR = schema.GroupVersionResource{
	Group:    "knodex.io",
	Version:  "v1alpha1",
	Resource: "projects",
}

// CreateUnstructuredProject creates an unstructured Project object for k8s client
func CreateUnstructuredProject(name, description string, roles []map[string]interface{}) *unstructured.Unstructured {
	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"description": description,
				"roles":       roles,
			},
		},
	}
	return project
}

// ToUnstructured converts a Project to unstructured format
func ToUnstructured(project *rbac.Project) (*unstructured.Unstructured, error) {
	// Create roles as map slice
	roles := make([]interface{}, 0, len(project.Spec.Roles))
	for _, role := range project.Spec.Roles {
		roleMap := map[string]interface{}{
			"name":        role.Name,
			"description": role.Description,
			"policies":    role.Policies,
		}
		if len(role.Groups) > 0 {
			roleMap["groups"] = role.Groups
		}
		roles = append(roles, roleMap)
	}

	// Create destinations as map slice
	destinations := make([]interface{}, 0, len(project.Spec.Destinations))
	for _, dest := range project.Spec.Destinations {
		destMap := map[string]interface{}{}
		if dest.Namespace != "" {
			destMap["namespace"] = dest.Namespace
		}
		if dest.Name != "" {
			destMap["name"] = dest.Name
		}
		destinations = append(destinations, destMap)
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":      project.Name,
				"namespace": project.Namespace,
			},
			"spec": map[string]interface{}{
				"description":  project.Spec.Description,
				"roles":        roles,
				"destinations": destinations,
			},
		},
	}

	// Set resourceVersion if present
	if project.ResourceVersion != "" {
		metadata := obj.Object["metadata"].(map[string]interface{})
		metadata["resourceVersion"] = project.ResourceVersion
	}

	// Set createdAt if present
	if !project.CreationTimestamp.IsZero() {
		metadata := obj.Object["metadata"].(map[string]interface{})
		metadata["creationTimestamp"] = project.CreationTimestamp.Format(metav1.RFC3339Micro)
	}

	return obj, nil
}
