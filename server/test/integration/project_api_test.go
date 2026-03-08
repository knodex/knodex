package integration

import (
	"net/http"
	"testing"

	"github.com/knodex/knodex/server/internal/rbac"
)

// TestProjectAPI_CreateProject_GlobalAdmin tests project creation by global admin
func TestProjectAPI_CreateProject_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Add global admin user
	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project request
	reqBody := CreateProjectRequestBody{
		Name:        "my-new-project",
		Description: "A test project created by admin",
		Destinations: []DestinationRequest{
			{Namespace: "my-new-project"},
		},
	}

	// Make request
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	// Assert status
	ts.AssertStatus(resp, http.StatusCreated)

	// Decode response
	project, err := DecodeJSON[ProjectResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify project was created
	if project.Name != "my-new-project" {
		t.Errorf("expected project name 'my-new-project', got '%s'", project.Name)
	}
	if project.Description != "A test project created by admin" {
		t.Errorf("expected description to match, got '%s'", project.Description)
	}
	// Note: fake dynamic client does not auto-generate resourceVersion
	// In real k8s, resourceVersion would be set by the API server
}

// TestProjectAPI_CreateProject_NonAdmin_Forbidden tests that non-admin cannot create projects
func TestProjectAPI_CreateProject_NonAdmin_Forbidden(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Add regular user
	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")

	// Create project request (include valid fields - auth is tested, not validation)
	reqBody := CreateProjectRequestBody{
		Name:        "unauthorized-project",
		Description: "This should fail",
		Destinations: []DestinationRequest{
			{Namespace: "unauthorized-project"},
		},
	}

	// Make request
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects", reqBody, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	// Assert forbidden status
	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestProjectAPI_CreateProject_Duplicate_Conflict tests duplicate project creation returns conflict
func TestProjectAPI_CreateProject_Duplicate_Conflict(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Add global admin
	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create first project directly
	_, err := ts.CreateProjectDirectly("existing-project", "Existing project", nil)
	if err != nil {
		t.Fatalf("failed to create initial project: %v", err)
	}

	// Try to create duplicate project
	reqBody := CreateProjectRequestBody{
		Name:        "existing-project",
		Description: "Duplicate project",
		Destinations: []DestinationRequest{
			{Namespace: "existing-project"},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	// Assert conflict status
	ts.AssertStatus(resp, http.StatusConflict)
}

// TestProjectAPI_CreateProject_InvalidName tests validation of project name
func TestProjectAPI_CreateProject_InvalidName(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	testCases := []struct {
		name        string
		projectName string
		description string
	}{
		{"empty name", "", "should fail empty"},
		{"uppercase", "MyProject", "should fail uppercase"},
		{"starts with hyphen", "-invalid", "should fail starts with hyphen"},
		{"ends with hyphen", "invalid-", "should fail ends with hyphen"},
		{"special chars", "my_project!", "should fail special chars"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := CreateProjectRequestBody{
				Name:        tc.projectName,
				Description: tc.description,
				// Include valid destinations so only name validation is tested
				Destinations: []DestinationRequest{
					{Namespace: "test-ns"},
				},
			}

			resp, err := ts.Request(http.MethodPost, "/api/v1/projects", reqBody, adminToken)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}

			ts.AssertStatus(resp, http.StatusBadRequest)
		})
	}
}

// TestProjectAPI_CreateProject_Unauthorized tests that unauthenticated requests are rejected
func TestProjectAPI_CreateProject_Unauthorized(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := CreateProjectRequestBody{
		Name:        "no-auth-project",
		Description: "Should fail without auth",
		Destinations: []DestinationRequest{
			{Namespace: "no-auth-project"},
		},
	}

	// Make request without token
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects", reqBody, "")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusUnauthorized)
}

// TestProjectAPI_GetProject_GlobalAdmin tests getting project as global admin
func TestProjectAPI_GetProject_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project directly
	_, err := ts.CreateProjectDirectly("get-test-project", "Project for get test", []rbac.ProjectRole{
		{Name: "admin", Description: "Admin role", Policies: []string{"p, proj:get-test-project:admin, get-test-project, *"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Get project
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/get-test-project", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	project, err := DecodeJSON[ProjectResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if project.Name != "get-test-project" {
		t.Errorf("expected project name 'get-test-project', got '%s'", project.Name)
	}
}

// TestProjectAPI_GetProject_RegularUser_WithAccess tests getting project as authorized user
func TestProjectAPI_GetProject_RegularUser_WithAccess(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{"accessible-project"}, "accessible-project")

	// Create project directly
	_, err := ts.CreateProjectDirectly("accessible-project", "Accessible project", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Grant user access
	ts.AllowUserAccess(RegularUserID, "accessible-project", "get")

	// Get project
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/accessible-project", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)
}

// TestProjectAPI_GetProject_RegularUser_WithoutAccess tests getting project without authorization.
// Project admin attempting GET on other project returns 403 Forbidden.
func TestProjectAPI_GetProject_RegularUser_WithoutAccess(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")

	// Create project directly
	_, err := ts.CreateProjectDirectly("restricted-project", "Restricted project", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Don't grant user access (explicitly test denial)
	ts.Enforcer.SetDenyAll(false) // Use default behavior (no access unless granted)

	// Get project - should return 403 Forbidden for authorization failures
	// (Changed from 404 to 403 for clearer authorization error feedback)
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/restricted-project", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestProjectAPI_GetProject_NotFound tests getting a non-existent project
func TestProjectAPI_GetProject_NotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/nonexistent-project", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNotFound)
}

// TestProjectAPI_ListProjects_GlobalAdmin tests listing all projects as global admin
func TestProjectAPI_ListProjects_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create multiple projects
	_, _ = ts.CreateProjectDirectly("list-project-1", "First project", nil)
	_, _ = ts.CreateProjectDirectly("list-project-2", "Second project", nil)
	_, _ = ts.CreateProjectDirectly("list-project-3", "Third project", nil)

	// List projects
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	list, err := DecodeJSON[ProjectListResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if list.TotalCount < 3 {
		t.Errorf("expected at least 3 projects, got %d", list.TotalCount)
	}
}

// TestProjectAPI_ListProjects_RegularUser_FilteredByAccess tests that regular users only see accessible projects
func TestProjectAPI_ListProjects_RegularUser_FilteredByAccess(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{"user-project-1"}, "user-project-1")

	// Create projects
	_, _ = ts.CreateProjectDirectly("user-project-1", "User's project", nil)
	_, _ = ts.CreateProjectDirectly("other-project", "Other project", nil)

	// Grant user access to only one project
	ts.AllowUserAccess(RegularUserID, "user-project-1", "get")

	// List projects
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	list, err := DecodeJSON[ProjectListResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// User should only see the project they have access to
	if list.TotalCount != 1 {
		t.Errorf("expected 1 project, got %d", list.TotalCount)
	}
	if len(list.Items) > 0 && list.Items[0].Name != "user-project-1" {
		t.Errorf("expected 'user-project-1', got '%s'", list.Items[0].Name)
	}
}

// TestProjectAPI_UpdateProject_GlobalAdmin tests updating project as global admin
func TestProjectAPI_UpdateProject_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project (fake k8s client sets resourceVersion to "1")
	project, err := ts.CreateProjectDirectly("update-test-project", "Original description", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Update project using the resourceVersion from create
	updateBody := UpdateProjectRequestBody{
		Description: "Updated description",
		Destinations: []DestinationRequest{
			{Namespace: "update-test-project"},
		},
		ResourceVersion: project.ResourceVersion, // Should be "1" from fake client
	}

	resp, err := ts.Request(http.MethodPut, "/api/v1/projects/update-test-project", updateBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	updated, err := DecodeJSON[ProjectResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if updated.Description != "Updated description" {
		t.Errorf("expected updated description, got '%s'", updated.Description)
	}
}

// TestProjectAPI_UpdateProject_OptimisticLocking tests resource version conflict detection
func TestProjectAPI_UpdateProject_OptimisticLocking(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project
	_, err := ts.CreateProjectDirectly("locking-test-project", "Original", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Update with wrong resource version
	updateBody := UpdateProjectRequestBody{
		Description:     "Updated description",
		ResourceVersion: "wrong-version",
	}

	resp, err := ts.Request(http.MethodPut, "/api/v1/projects/locking-test-project", updateBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusConflict)
}

// TestProjectAPI_UpdateProject_MissingResourceVersion tests that resourceVersion is required
func TestProjectAPI_UpdateProject_MissingResourceVersion(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project
	_, err := ts.CreateProjectDirectly("missing-rv-project", "Original", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Update without resource version
	updateBody := UpdateProjectRequestBody{
		Description: "Updated description",
		// ResourceVersion intentionally omitted
	}

	resp, err := ts.Request(http.MethodPut, "/api/v1/projects/missing-rv-project", updateBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusBadRequest)
}

// TestProjectAPI_DeleteProject_GlobalAdmin tests deleting project as global admin
func TestProjectAPI_DeleteProject_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project
	_, err := ts.CreateProjectDirectly("delete-test-project", "To be deleted", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Delete project
	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/delete-test-project", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	// Verify project is deleted
	_, err = ts.GetProjectDirectly("delete-test-project")
	if err == nil {
		t.Error("expected project to be deleted, but it still exists")
	}
}

// TestProjectAPI_DeleteProject_NonAdmin_Forbidden tests that non-admin cannot delete projects
func TestProjectAPI_DeleteProject_NonAdmin_Forbidden(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{"user-delete-project"}, "user-delete-project")

	// Create project
	_, err := ts.CreateProjectDirectly("user-delete-project", "User's project", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Grant update access but not delete (only global admins can delete)
	ts.AllowUserAccess(RegularUserID, "user-delete-project", "update")

	// Attempt delete
	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/user-delete-project", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestProjectAPI_DeleteProject_NotFound tests deleting non-existent project
func TestProjectAPI_DeleteProject_NotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/nonexistent-project", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNotFound)
}
