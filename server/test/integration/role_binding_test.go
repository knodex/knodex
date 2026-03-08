// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package integration

import (
	"net/http"
	"testing"

	"github.com/knodex/knodex/server/internal/rbac"
)

// TestRoleBindingAPI_AssignUserRole_GlobalAdmin tests assigning a user to a project role as global admin
func TestRoleBindingAPI_AssignUserRole_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project with a role
	_, err := ts.CreateProjectDirectly("role-binding-project", "Project for role binding tests", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:role-binding-project:developer, role-binding-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Assign user to role
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/role-binding-project/roles/developer/users/test-user-id", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusCreated)

	// Decode response
	binding, err := DecodeJSON[RoleBindingAssignmentResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if binding.Project != "role-binding-project" {
		t.Errorf("expected project 'role-binding-project', got '%s'", binding.Project)
	}
	if binding.Role != "developer" {
		t.Errorf("expected role 'developer', got '%s'", binding.Role)
	}
	if binding.Subject != "test-user-id" {
		t.Errorf("expected subject 'test-user-id', got '%s'", binding.Subject)
	}
	if binding.Type != "user" {
		t.Errorf("expected type 'user', got '%s'", binding.Type)
	}
}

// TestRoleBindingAPI_AssignUserRole_ProjectAdmin tests assigning a user to a role as project admin
func TestRoleBindingAPI_AssignUserRole_ProjectAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	projectAdminToken := ts.AddUser(ProjectAdminID, ProjectAdminEmail, []string{"admin-project"}, "admin-project")

	// Create project with roles
	_, err := ts.CreateProjectDirectly("admin-project", "Project with admin", []rbac.ProjectRole{
		{Name: "admin", Description: "Admin role", Policies: []string{"p, proj:admin-project:admin, admin-project, *"}},
		{Name: "viewer", Description: "Viewer role", Policies: []string{"p, proj:admin-project:viewer, admin-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Grant project admin member-add access (required for role binding assignment)
	ts.AllowUserAccess(ProjectAdminID, "admin-project", "member-add")

	// Assign user to viewer role
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/admin-project/roles/viewer/users/new-viewer-id", nil, projectAdminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusCreated)
}

// TestRoleBindingAPI_AssignUserRole_NonAdmin_Forbidden tests that non-admin cannot assign roles
func TestRoleBindingAPI_AssignUserRole_NonAdmin_Forbidden(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{"viewer-project"}, "viewer-project")

	// Create project
	_, err := ts.CreateProjectDirectly("viewer-project", "Project for viewer test", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:viewer-project:developer, viewer-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// User has no update permission - deny all by default
	ts.Enforcer.SetDenyAll(false)

	// Try to assign role
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/viewer-project/roles/developer/users/another-user", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestRoleBindingAPI_AssignUserRole_InvalidRole tests assigning user to non-existent role
func TestRoleBindingAPI_AssignUserRole_InvalidRole(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project without the target role
	_, err := ts.CreateProjectDirectly("no-role-project", "Project without target role", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:no-role-project:developer, no-role-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Try to assign to non-existent role
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/no-role-project/roles/nonexistent-role/users/test-user", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusBadRequest)
}

// TestRoleBindingAPI_AssignUserRole_ProjectNotFound tests assigning role to non-existent project
func TestRoleBindingAPI_AssignUserRole_ProjectNotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/nonexistent-project/roles/developer/users/test-user", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNotFound)
}

// TestRoleBindingAPI_AssignGroupRole_GlobalAdmin tests assigning a group to a project role
func TestRoleBindingAPI_AssignGroupRole_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project with a role
	_, err := ts.CreateProjectDirectly("group-role-project", "Project for group role tests", []rbac.ProjectRole{
		{Name: "viewer", Description: "Viewer role", Policies: []string{"p, proj:group-role-project:viewer, group-role-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Assign group to role
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/group-role-project/roles/viewer/groups/engineering-team", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusCreated)

	// Decode response
	binding, err := DecodeJSON[RoleBindingAssignmentResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if binding.Project != "group-role-project" {
		t.Errorf("expected project 'group-role-project', got '%s'", binding.Project)
	}
	if binding.Role != "viewer" {
		t.Errorf("expected role 'viewer', got '%s'", binding.Role)
	}
	if binding.Subject != "engineering-team" {
		t.Errorf("expected subject 'engineering-team', got '%s'", binding.Subject)
	}
	if binding.Type != "group" {
		t.Errorf("expected type 'group', got '%s'", binding.Type)
	}
}

// TestRoleBindingAPI_AssignGroupRole_NonAdmin_Forbidden tests that non-admin cannot assign group roles
func TestRoleBindingAPI_AssignGroupRole_NonAdmin_Forbidden(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")

	// Create project
	_, err := ts.CreateProjectDirectly("forbidden-group-project", "Project for forbidden group test", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:forbidden-group-project:developer, forbidden-group-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// User has no update permission
	ts.Enforcer.SetDenyAll(false)

	// Try to assign group role
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/forbidden-group-project/roles/developer/groups/some-group", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestRoleBindingAPI_ListRoleBindings_GlobalAdmin tests listing role bindings as global admin
func TestRoleBindingAPI_ListRoleBindings_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project with roles
	_, err := ts.CreateProjectDirectly("list-bindings-project", "Project for listing bindings", []rbac.ProjectRole{
		{Name: "admin", Description: "Admin role", Policies: []string{"p, proj:list-bindings-project:admin, list-bindings-project, *"}},
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:list-bindings-project:developer, list-bindings-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Assign some users and groups to roles
	_, err = ts.Request(http.MethodPost, "/api/v1/projects/list-bindings-project/roles/admin/users/admin-user-1", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to assign admin user: %v", err)
	}
	_, err = ts.Request(http.MethodPost, "/api/v1/projects/list-bindings-project/roles/developer/users/dev-user-1", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to assign developer user: %v", err)
	}
	_, err = ts.Request(http.MethodPost, "/api/v1/projects/list-bindings-project/roles/developer/groups/dev-team", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to assign developer group: %v", err)
	}

	// List role bindings
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/list-bindings-project/role-bindings", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	// Decode response
	list, err := DecodeJSON[ListRoleBindingsResponseIntegration](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if list.Project != "list-bindings-project" {
		t.Errorf("expected project 'list-bindings-project', got '%s'", list.Project)
	}
	if len(list.Bindings) < 3 {
		t.Errorf("expected at least 3 bindings, got %d", len(list.Bindings))
	}
}

// TestRoleBindingAPI_ListRoleBindings_WithAccess tests listing role bindings with get permission
func TestRoleBindingAPI_ListRoleBindings_WithAccess(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{"accessible-bindings-project"}, "accessible-bindings-project")

	// Create project
	_, err := ts.CreateProjectDirectly("accessible-bindings-project", "Accessible project", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:accessible-bindings-project:developer, accessible-bindings-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Grant user get access
	ts.AllowUserAccess(RegularUserID, "accessible-bindings-project", "get")

	// List role bindings
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/accessible-bindings-project/role-bindings", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)
}

// TestRoleBindingAPI_ListRoleBindings_WithoutAccess tests listing role bindings without permission
func TestRoleBindingAPI_ListRoleBindings_WithoutAccess(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")

	// Create project
	_, err := ts.CreateProjectDirectly("restricted-bindings-project", "Restricted project", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:restricted-bindings-project:developer, restricted-bindings-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// No access granted
	ts.Enforcer.SetDenyAll(false)

	// List role bindings - should be forbidden
	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/restricted-bindings-project/role-bindings", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestRoleBindingAPI_ListRoleBindings_ProjectNotFound tests listing bindings for non-existent project
func TestRoleBindingAPI_ListRoleBindings_ProjectNotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	resp, err := ts.Request(http.MethodGet, "/api/v1/projects/nonexistent-project/role-bindings", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNotFound)
}

// TestRoleBindingAPI_RemoveUserRole_GlobalAdmin tests removing a user from a role as global admin
func TestRoleBindingAPI_RemoveUserRole_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project with role
	_, err := ts.CreateProjectDirectly("remove-user-project", "Project for remove user test", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:remove-user-project:developer, remove-user-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Assign user first
	_, err = ts.Request(http.MethodPost, "/api/v1/projects/remove-user-project/roles/developer/users/user-to-remove", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to assign user: %v", err)
	}

	// Remove user from role
	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/remove-user-project/roles/developer/users/user-to-remove", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNoContent)
}

// TestRoleBindingAPI_RemoveUserRole_NonAdmin_Forbidden tests that non-admin cannot remove user roles
func TestRoleBindingAPI_RemoveUserRole_NonAdmin_Forbidden(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")

	// Create project
	_, err := ts.CreateProjectDirectly("remove-forbidden-project", "Project for forbidden removal", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:remove-forbidden-project:developer, remove-forbidden-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// No update permission
	ts.Enforcer.SetDenyAll(false)

	// Try to remove user role
	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/remove-forbidden-project/roles/developer/users/some-user", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestRoleBindingAPI_RemoveUserRole_ProjectNotFound tests removing role from non-existent project
func TestRoleBindingAPI_RemoveUserRole_ProjectNotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/nonexistent-project/roles/developer/users/test-user", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNotFound)
}

// TestRoleBindingAPI_RemoveGroupRole_GlobalAdmin tests removing a group from a role as global admin
func TestRoleBindingAPI_RemoveGroupRole_GlobalAdmin(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project with role
	_, err := ts.CreateProjectDirectly("remove-group-project", "Project for remove group test", []rbac.ProjectRole{
		{Name: "viewer", Description: "Viewer role", Policies: []string{"p, proj:remove-group-project:viewer, remove-group-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Assign group first
	_, err = ts.Request(http.MethodPost, "/api/v1/projects/remove-group-project/roles/viewer/groups/group-to-remove", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to assign group: %v", err)
	}

	// Remove group from role
	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/remove-group-project/roles/viewer/groups/group-to-remove", nil, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNoContent)
}

// TestRoleBindingAPI_RemoveGroupRole_NonAdmin_Forbidden tests that non-admin cannot remove group roles
func TestRoleBindingAPI_RemoveGroupRole_NonAdmin_Forbidden(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")

	// Create project
	_, err := ts.CreateProjectDirectly("remove-group-forbidden-project", "Project for forbidden group removal", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:remove-group-forbidden-project:developer, remove-group-forbidden-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// No update permission
	ts.Enforcer.SetDenyAll(false)

	// Try to remove group role
	resp, err := ts.Request(http.MethodDelete, "/api/v1/projects/remove-group-forbidden-project/roles/developer/groups/some-group", nil, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusForbidden)
}

// TestRoleBindingAPI_Unauthorized tests that unauthenticated requests are rejected
func TestRoleBindingAPI_Unauthorized(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Create project
	_, err := ts.CreateProjectDirectly("unauth-binding-project", "Project for unauth test", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:unauth-binding-project:developer, unauth-binding-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{"assign user role", http.MethodPost, "/api/v1/projects/unauth-binding-project/roles/developer/users/test-user"},
		{"assign group role", http.MethodPost, "/api/v1/projects/unauth-binding-project/roles/developer/groups/test-group"},
		{"list role bindings", http.MethodGet, "/api/v1/projects/unauth-binding-project/role-bindings"},
		{"remove user role", http.MethodDelete, "/api/v1/projects/unauth-binding-project/roles/developer/users/test-user"},
		{"remove group role", http.MethodDelete, "/api/v1/projects/unauth-binding-project/roles/developer/groups/test-group"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ts.Request(tc.method, tc.path, nil, "")
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}

			ts.AssertStatus(resp, http.StatusUnauthorized)
		})
	}
}
