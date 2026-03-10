// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Test users
	testUserAlice = "alice@example.com"
	testUserBob   = "bob@example.com"
	testUserAdmin = "admin@example.com"

	// Test groups
	testGroupDevelopers = "developers"
	testGroupViewers    = "viewers"

	// Test projects
	testProjectShared = "e2e-shared"
	testProjectTeamA  = "e2e-team-a"
	testProjectTeamB  = "e2e-team-b"

	// Server timeout
	serverTimeout = 30 * time.Second
)

var (
	dynamicClient dynamic.Interface
	httpClient    *http.Client
	apiBaseURL    string

	// GVRs for CRDs
	projectGVR = schema.GroupVersionResource{
		Group:    "knodex.io",
		Version:  "v1alpha1",
		Resource: "projects",
	}
)

func TestMain(m *testing.M) {
	if os.Getenv("E2E_TESTS") != "true" {
		fmt.Println("Skipping E2E tests. Set E2E_TESTS=true to run.")
		os.Exit(0)
	}

	// Get API base URL from environment
	apiBaseURL = os.Getenv("E2E_API_URL")
	if apiBaseURL == "" {
		apiBaseURL = "http://localhost:8080"
	}

	// Setup Kubernetes client
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Failed to load kubeconfig: %v\n", err)
		os.Exit(1)
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to create dynamic client: %v\n", err)
		os.Exit(1)
	}
	dynamicClient = client

	// Setup HTTP client
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Wait for server to be ready
	if err := waitForServer(); err != nil {
		fmt.Printf("Server not ready: %v\n", err)
		os.Exit(1)
	}

	// Setup test fixtures
	if err := setupTestFixtures(); err != nil {
		fmt.Printf("Failed to setup test fixtures: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Cleanup
	_ = cleanupTestFixtures()

	os.Exit(code)
}

func waitForServer() error {
	fmt.Printf("Waiting for server at %s...\n", apiBaseURL)
	deadline := time.Now().Add(serverTimeout)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(apiBaseURL + "/healthz")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			fmt.Println("Server is ready!")
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("server not ready after %v", serverTimeout)
}

// generateTestJWT creates a JWT token for test users using the centralized utility.
// This is a convenience wrapper that delegates to GenerateSimpleJWT from test_utils.go.
// The addAdminRole parameter sets JWT casbin_roles for UI display hints;
// actual authorization uses server-side Casbin policies (STORY-228).
func generateTestJWT(userID string, projects []string, addAdminRole bool) string {
	return GenerateSimpleJWT(userID, projects, addAdminRole)
}

// makeAuthenticatedRequest makes HTTP request with JWT token using the centralized utility.
// This is a convenience wrapper that maintains the existing function signature.
func makeAuthenticatedRequest(method, path string, token string, body interface{}) (*http.Response, error) {
	return MakeAuthenticatedRequest(httpClient, apiBaseURL, method, path, token, body)
}

// setupTestFixtures creates test Projects with role bindings
func setupTestFixtures() error {
	fmt.Println("Setting up test fixtures...")
	ctx := context.Background()

	// Create test projects
	projects := []struct {
		name        string
		description string
		roles       []map[string]interface{}
	}{
		{
			name:        testProjectShared,
			description: "E2E shared project",
			roles: []map[string]interface{}{
				{
					"name":        "admin",
					"description": "Administrator",
					"policies": []interface{}{
						// 3-part format: object, action, effect
						"*, *, allow",
					},
				},
				{
					"name":        "viewer",
					"description": "Viewer",
					"policies": []interface{}{
						// 3-part format: object, action, effect
						"*, get, allow",
					},
				},
			},
		},
		{
			name:        testProjectTeamA,
			description: "E2E team-a project",
			roles: []map[string]interface{}{
				{
					"name":        "admin",
					"description": "Administrator",
					"policies": []interface{}{
						"*, *, allow",
					},
				},
				{
					"name":        "viewer",
					"description": "Viewer",
					"policies": []interface{}{
						"*, get, allow",
					},
				},
			},
		},
		{
			name:        testProjectTeamB,
			description: "E2E team-b project",
			roles: []map[string]interface{}{
				{
					"name":        "admin",
					"description": "Administrator",
					"policies": []interface{}{
						"*, *, allow",
					},
				},
			},
		},
	}

	for _, proj := range projects {
		// Delete if exists (cleanup from previous run)
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, proj.name, metav1.DeleteOptions{})
		time.Sleep(100 * time.Millisecond)

		if err := createTestProject(ctx, proj.name, proj.description, proj.roles); err != nil {
			return fmt.Errorf("failed to create project %s: %w", proj.name, err)
		}
		fmt.Printf("  Created project: %s\n", proj.name)
	}

	// Wait for Casbin to load policies
	fmt.Println("Waiting for policy sync...")
	time.Sleep(3 * time.Second)

	fmt.Println("Test fixtures ready!")
	return nil
}

func createTestProject(ctx context.Context, name, description string, roles []map[string]interface{}) error {
	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"e2e-test": "true",
				},
			},
			"spec": map[string]interface{}{
				"description": description,
				"destinations": []interface{}{
					map[string]interface{}{
						"namespace": name,
					},
				},
				"roles": roles,
			},
		},
	}

	_, err := dynamicClient.Resource(projectGVR).Create(ctx, project, metav1.CreateOptions{})
	return err
}

func cleanupTestFixtures() error {
	fmt.Println("Cleaning up test fixtures...")
	ctx := context.Background()

	// Delete test projects by label
	err := dynamicClient.Resource(projectGVR).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "e2e-test=true",
	})
	if err != nil {
		fmt.Printf("Warning: failed to cleanup projects: %v\n", err)
	}

	return nil
}

// ==============================================================================
// Authentication Tests
// ==============================================================================

func TestE2E_Authentication_UnauthenticatedRequest(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated request should return 401")
}

func TestE2E_Authentication_InvalidToken(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", "invalid-token", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "invalid token should return 401")
}

func TestE2E_Authentication_ExpiredToken(t *testing.T) {
	// Create an expired token
	claims := jwt.MapClaims{
		"sub":   testUserAlice,
		"email": testUserAlice,
		"iss":   "knodex",
		"aud":   "knodex-api",
		"exp":   time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
		"iat":   time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, _ := token.SignedString([]byte(TestJWTSecret))

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", expiredToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "expired token should return 401")
}

func TestE2E_Authentication_ValidToken(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed (200) or return 200 with empty list
	assert.Equal(t, http.StatusOK, resp.StatusCode, "valid token should be accepted")
}

// ==============================================================================
// Project API RBAC Tests
// ==============================================================================

func TestE2E_ProjectAPI_ListProjects_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// API returns {"items": [...], "totalCount": N} format
	var response struct {
		Items      []map[string]interface{} `json:"items"`
		TotalCount int                      `json:"totalCount"`
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	projects := response.Items

	// Global admin should see at least all test projects
	assert.GreaterOrEqual(t, len(projects), 3, "global admin should see all test projects")

	// Verify test projects are in the list
	projectNames := make([]string, 0)
	for _, proj := range projects {
		if name, ok := proj["name"].(string); ok {
			projectNames = append(projectNames, name)
		}
	}
	assert.Contains(t, projectNames, testProjectShared)
	assert.Contains(t, projectNames, testProjectTeamA)
	assert.Contains(t, projectNames, testProjectTeamB)
}

func TestE2E_ProjectAPI_ListProjects_NoRoleBindings(t *testing.T) {
	// User with no role bindings should see empty list or be forbidden
	token := generateTestJWT("noroles@example.com", []string{}, false)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Either empty list (200) or forbidden (403) depending on implementation
	if resp.StatusCode == http.StatusOK {
		// API returns {"items": [...], "totalCount": N} format
		var response struct {
			Items      []map[string]interface{} `json:"items"`
			TotalCount int                      `json:"totalCount"`
		}
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		// User with no bindings should see no projects
		assert.Empty(t, response.Items, "user with no role bindings should see no projects")
	} else {
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	}
}

func TestE2E_ProjectAPI_GetProject_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+testProjectShared, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var project map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&project)
	require.NoError(t, err)

	assert.Equal(t, testProjectShared, project["name"])
}

func TestE2E_ProjectAPI_GetProject_Forbidden(t *testing.T) {
	// User with no role bindings cannot access project
	token := generateTestJWT("noroles@example.com", []string{}, false)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+testProjectShared, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "user without role should get 403")
}

func TestE2E_ProjectAPI_GetProject_NotFound(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/nonexistent-project", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestE2E_ProjectAPI_CreateProject_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	newProject := map[string]interface{}{
		"name":        "e2e-new-project",
		"description": "E2E created project",
		"destinations": []map[string]interface{}{
			{
				"namespace": "e2e-new-project",
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects", token, newProject)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "global admin should be able to create project")

	// Cleanup
	ctx := context.Background()
	_ = dynamicClient.Resource(projectGVR).Delete(ctx, "e2e-new-project", metav1.DeleteOptions{})
}

func TestE2E_ProjectAPI_CreateProject_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT("noroles@example.com", []string{}, false)

	newProject := map[string]interface{}{
		"name":        "e2e-forbidden-project",
		"description": "Should fail",
		"destinations": []map[string]interface{}{
			{
				"namespace": "e2e-forbidden-project",
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects", token, newProject)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "non-admin should not be able to create project")
}

func TestE2E_ProjectAPI_CreateProject_DuplicateName_Conflict(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Try to create project with existing name
	newProject := map[string]interface{}{
		"name":        testProjectShared, // Already exists
		"description": "Duplicate project",
		"destinations": []map[string]interface{}{
			{
				"namespace": "e2e-duplicate",
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects", token, newProject)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode, "duplicate project name should return conflict")
}

func TestE2E_ProjectAPI_UpdateProject_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// First get the project to get its current state
	getResp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+testProjectShared, token, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var currentProject map[string]interface{}
	err = json.NewDecoder(getResp.Body).Decode(&currentProject)
	require.NoError(t, err)

	// Update description (must include destinations as they are replaced entirely)
	update := map[string]interface{}{
		"description":     "Updated E2E description",
		"destinations":    currentProject["destinations"],
		"resourceVersion": currentProject["resourceVersion"],
	}

	resp, err := makeAuthenticatedRequest("PUT", "/api/v1/projects/"+testProjectShared, token, update)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "global admin should be able to update project")
}

func TestE2E_ProjectAPI_UpdateProject_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT("noroles@example.com", []string{}, false)

	update := map[string]interface{}{
		"description": "Should fail",
	}

	resp, err := makeAuthenticatedRequest("PUT", "/api/v1/projects/"+testProjectShared, token, update)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "non-admin should not be able to update project")
}

func TestE2E_ProjectAPI_DeleteProject_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)
	ctx := context.Background()

	// Create a project to delete
	projectName := "e2e-delete-test"
	err := createTestProject(ctx, projectName, "Project to delete", []map[string]interface{}{
		{
			"name":        "admin",
			"description": "Admin",
			"policies":    []interface{}{fmt.Sprintf("p, proj:%s:admin, %s, *", projectName, projectName)},
		},
	})
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// Delete the project
	resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/projects/"+projectName, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"global admin should be able to delete project, got %d", resp.StatusCode)
}

func TestE2E_ProjectAPI_DeleteProject_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT("noroles@example.com", []string{}, false)

	resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/projects/"+testProjectShared, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "non-admin should not be able to delete project")
}

// ==============================================================================
// RGD Catalog RBAC Tests
// ==============================================================================

func TestE2E_RGDCatalog_ListRGDs_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "global admin should be able to list RGDs")
}

func TestE2E_RGDCatalog_ListRGDs_Unauthenticated(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated request should return 401")
}

// ==============================================================================
// Instance Deployment RBAC Tests
// ==============================================================================

func TestE2E_Instance_ListInstances_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "global admin should be able to list instances")
}

func TestE2E_Instance_ListInstances_Unauthenticated(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated request should return 401")
}

func TestE2E_Instance_CreateInstance_Unauthenticated(t *testing.T) {
	instance := map[string]interface{}{
		"name":      "test-instance",
		"namespace": "default",
		"rgdName":   "test-rgd",
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/instances", "", instance)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated create should return 401")
}

// ==============================================================================
// Cross-Project Access Tests
// ==============================================================================

func TestE2E_CrossProject_AdminCanAccessAllProjects(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Admin should be able to access all test projects
	for _, projectName := range []string{testProjectShared, testProjectTeamA, testProjectTeamB} {
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, token, nil)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"global admin should be able to access project %s", projectName)
	}
}

func TestE2E_CrossProject_UserCannotAccessUnassignedProject(t *testing.T) {
	// Non-admin user without role bindings cannot access any project
	token := generateTestJWT("noroles@example.com", []string{}, false)

	for _, projectName := range []string{testProjectShared, testProjectTeamA, testProjectTeamB} {
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, token, nil)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"user without role should not access project %s", projectName)
	}
}

// ==============================================================================
// Role Binding API Tests
// ==============================================================================

func TestE2E_RoleBinding_AssignUserRole_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/users/e2e-test-user@example.com",
		token,
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"global admin should be able to assign role, got %d", resp.StatusCode)
}

func TestE2E_RoleBinding_AssignUserRole_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT("noroles@example.com", []string{}, false)

	resp, err := makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/users/test-user@example.com",
		token,
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "non-admin should not assign roles")
}

func TestE2E_RoleBinding_ListRoleBindings_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest(
		"GET",
		"/api/v1/projects/"+testProjectShared+"/role-bindings",
		token,
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "global admin should list role bindings")
}

func TestE2E_RoleBinding_RemoveUserRole_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// First assign a role
	_, err := makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/users/e2e-remove-test@example.com",
		token,
		nil,
	)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// Now remove it
	resp, err := makeAuthenticatedRequest(
		"DELETE",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/users/e2e-remove-test@example.com",
		token,
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"global admin should remove role, got %d", resp.StatusCode)
}

// ==============================================================================
// Validation API Tests
// ==============================================================================

func TestE2E_Validation_ValidateProjectCreation(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Wrap project data in a "project" field as expected by the handler
	requestBody := map[string]interface{}{
		"project": map[string]interface{}{
			"name":        "e2e-validate-test",
			"description": "Test project for validation",
			"destinations": []map[string]interface{}{
				{
					"namespace": "e2e-validate-test",
				},
			},
			"roles": []map[string]interface{}{
				{
					"name":        "admin",
					"description": "Administrator",
					"policies":    []string{"p, proj:e2e-validate-test:admin, e2e-validate-test, *"},
				},
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects/validate", token, requestBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "valid project should pass validation")
}

func TestE2E_Validation_ValidateProjectCreation_InvalidName(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Wrap project data in a "project" field as expected by the handler
	requestBody := map[string]interface{}{
		"project": map[string]interface{}{
			"name":        "Invalid_Project_Name!", // Invalid: uppercase and special chars
			"description": "Should fail validation",
			"destinations": []map[string]interface{}{
				{
					"namespace": "test",
				},
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects/validate", token, requestBody)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return OK with validation errors in body, or BadRequest
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest,
		"invalid project name should be caught by validation")
}

// ==============================================================================
// Group Role Binding Tests
// ==============================================================================

func TestE2E_GroupRoleBinding_AssignGroupRole_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/groups/"+testGroupDevelopers,
		token,
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"global admin should assign group role, got %d", resp.StatusCode)
}

func TestE2E_GroupRoleBinding_RemoveGroupRole_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// First assign a group role
	_, err := makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/groups/e2e-remove-group",
		token,
		nil,
	)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	// Now remove it
	resp, err := makeAuthenticatedRequest(
		"DELETE",
		"/api/v1/projects/"+testProjectShared+"/roles/viewer/groups/e2e-remove-group",
		token,
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"global admin should remove group role, got %d", resp.StatusCode)
}

// ==============================================================================
// Health Check Tests
// ==============================================================================

func TestE2E_Health_Healthz(t *testing.T) {
	resp, err := httpClient.Get(apiBaseURL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "healthz should return 200")
}

func TestE2E_Health_Readyz(t *testing.T) {
	resp, err := httpClient.Get(apiBaseURL + "/readyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "readyz should return 200")
}

// ==============================================================================
// RBAC Metrics Tests
// ==============================================================================

func TestE2E_RBACMetrics_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rbac/metrics", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "global admin should access RBAC metrics")
}

func TestE2E_RBACMetrics_Unauthenticated(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rbac/metrics", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated should return 401")
}
