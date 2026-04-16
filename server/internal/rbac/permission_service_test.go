// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// Updated tests to work without UserService.
// PermissionService now uses PolicyEnforcer for user project access.
// User CRD is no longer used - OIDC users are ephemeral, local users use ConfigMap/Secret.

// mockPermissionPolicyEnforcer implements PolicyEnforcer for permission service tests
type mockPermissionPolicyEnforcer struct {
	accessibleProjects map[string][]string // userID -> projectIDs
	assignedRoles      map[string][]string // userID -> Casbin roles
}

func newMockPermissionPolicyEnforcer() *mockPermissionPolicyEnforcer {
	return &mockPermissionPolicyEnforcer{
		accessibleProjects: make(map[string][]string),
		assignedRoles:      make(map[string][]string),
	}
}

func (m *mockPermissionPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == CasbinRoleServerAdmin {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockPermissionPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.CanAccess(ctx, user, object, action)
}

func (m *mockPermissionPolicyEnforcer) AssignRole(ctx context.Context, user, role string) error {
	m.assignedRoles[user] = append(m.assignedRoles[user], role)
	return nil
}

func (m *mockPermissionPolicyEnforcer) RemoveRole(ctx context.Context, user, role string) error {
	roles := m.assignedRoles[user]
	for i, r := range roles {
		if r == role {
			m.assignedRoles[user] = append(roles[:i], roles[i+1:]...)
			break
		}
	}
	return nil
}

// AssignUserRoles assigns multiple roles to a user, replacing existing roles
func (m *mockPermissionPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	m.assignedRoles[user] = roles
	return nil
}

func (m *mockPermissionPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return m.assignedRoles[user], nil
}

func (m *mockPermissionPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	// Check if admin role (ArgoCD-aligned: use CasbinRoleServerAdmin, not deprecated alias)
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == CasbinRoleServerAdmin {
			// Return all projects for admin
			return m.accessibleProjects["*"], nil
		}
	}
	return m.accessibleProjects[user], nil
}

func (m *mockPermissionPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	return nil
}

func (m *mockPermissionPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	allowed, err := m.CanAccess(ctx, user, "projects/"+projectName, action)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrAccessDenied
	}
	return nil
}

func (m *mockPermissionPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *Project) error {
	return nil
}

func (m *mockPermissionPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockPermissionPolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	delete(m.assignedRoles, user)
	return nil
}

func (m *mockPermissionPolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	return m.RemoveRole(ctx, user, role)
}

func (m *mockPermissionPolicyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	return nil
}

func (m *mockPermissionPolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return nil
}

func (m *mockPermissionPolicyEnforcer) InvalidateCache() {
	// No-op for mock
}

func (m *mockPermissionPolicyEnforcer) InvalidateCacheForUser(user string) int {
	return 0 // No-op for mock
}

func (m *mockPermissionPolicyEnforcer) InvalidateCacheForProject(project string) int {
	return 0 // No-op for mock
}

func (m *mockPermissionPolicyEnforcer) CacheStats() CacheStats {
	return CacheStats{}
}

func (m *mockPermissionPolicyEnforcer) Metrics() PolicyMetrics {
	return PolicyMetrics{}
}

func (m *mockPermissionPolicyEnforcer) IncrementPolicyReloads() {
	// No-op for mock
}

func (m *mockPermissionPolicyEnforcer) IncrementBackgroundSyncs() {
	// No-op for mock
}

func (m *mockPermissionPolicyEnforcer) IncrementWatcherRestarts() {
	// No-op for mock
}

// setUserProjects sets accessible projects for a user
func (m *mockPermissionPolicyEnforcer) setUserProjects(userID string, projects []string) {
	m.accessibleProjects["user:"+userID] = projects
}

// setAllProjects sets the list of all projects (for global admin access)
func (m *mockPermissionPolicyEnforcer) setAllProjects(projects []string) {
	m.accessibleProjects["*"] = projects
}

// setupTestServices creates test services with proper CRD registration
// No longer creates UserService - PermissionService uses PolicyEnforcer
func setupTestServices() (*ProjectService, *PermissionService, *mockPermissionPolicyEnforcer) {
	// Register Project CRD in scheme (no User CRD needed)
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(
		schema.GroupVersion{Group: ProjectGroup, Version: ProjectVersion},
		&Project{},
		&ProjectList{},
	)
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Group: ProjectGroup, Version: ProjectVersion})

	// Create fake clients
	k8sClient := k8sfake.NewSimpleClientset()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create mock policy enforcer
	policyEnforcer := newMockPermissionPolicyEnforcer()

	// Create services
	projectService := NewProjectService(k8sClient, dynamicClient, "knodex-system")
	permService := NewPermissionService(PermissionServiceConfig{
		ProjectService: projectService,
		PolicyEnforcer: policyEnforcer,
	})

	return projectService, permService, policyEnforcer
}

// createPermissionTestProjectSpec creates an ArgoCD-aligned ProjectSpec for permission tests
// The roleAssignments map userID to their role name (platform-admin, developer, viewer)
func createPermissionTestProjectSpec(projectID string, roleAssignments map[string]string) ProjectSpec {
	roles := []ProjectRole{
		{
			Name:        "platform-admin",
			Description: "Full access to project resources",
			Policies: []string{
				fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", projectID, projectID),
			},
			Groups: []string{},
		},
		{
			Name:        "developer",
			Description: "Deploy and manage instances within the project",
			Policies: []string{
				fmt.Sprintf("p, proj:%s:developer, applications, *, %s/*, allow", projectID, projectID),
				fmt.Sprintf("p, proj:%s:developer, repositories, get, %s/*, allow", projectID, projectID),
			},
			Groups: []string{},
		},
		{
			Name:        "viewer",
			Description: "Read-only access to project resources",
			Policies: []string{
				fmt.Sprintf("p, proj:%s:viewer, *, get, %s/*, allow", projectID, projectID),
			},
			Groups: []string{},
		},
	}

	// Assign users to roles via OIDC groups
	for userID, roleName := range roleAssignments {
		userGroup := fmt.Sprintf("user:%s", userID)
		for i := range roles {
			if roles[i].Name == roleName {
				roles[i].Groups = append(roles[i].Groups, userGroup)
				break
			}
		}
	}

	return ProjectSpec{
		Description: "Test Project",
		Destinations: []Destination{
			{
				Namespace: projectID,
			},
		},
		NamespaceResourceWhitelist: []ResourceSpec{
			{Group: "*", Kind: "*"},
		},
		Roles: roles,
	}
}

func TestGetUserProjects(t *testing.T) {
	ctx := context.Background()

	// Setup
	projectService, permService, policyEnforcer := setupTestServices()

	// Create multiple projects
	projectID1 := "project-1"
	spec1 := createPermissionTestProjectSpec(projectID1, map[string]string{})
	project1, err := projectService.CreateProject(ctx, projectID1, spec1, "system")
	if err != nil {
		t.Fatalf("Failed to create project1: %v", err)
	}

	projectID2 := "project-2"
	spec2 := createPermissionTestProjectSpec(projectID2, map[string]string{})
	project2, err := projectService.CreateProject(ctx, projectID2, spec2, "system")
	if err != nil {
		t.Fatalf("Failed to create project2: %v", err)
	}

	// Configure mock policy enforcer to grant user access to both projects
	userID := "test-user-123"
	policyEnforcer.setUserProjects(userID, []string{project1.Name, project2.Name})

	// Test GetUserProjects
	projects, err := permService.GetUserProjects(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserProjects failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(projects))
	}

	// Verify project IDs
	projectIDs := make(map[string]bool)
	for _, project := range projects {
		projectIDs[project.Name] = true
	}

	if !projectIDs[project1.Name] {
		t.Errorf("Missing project1 in user projects")
	}
	if !projectIDs[project2.Name] {
		t.Errorf("Missing project2 in user projects")
	}
}

func TestGetUserRole(t *testing.T) {
	ctx := context.Background()

	// Setup
	projectService, permService, _ := setupTestServices()

	// Define user for role assignment
	userID := "test-user-456"

	// Create project with user as platform-admin using ArgoCD-aligned spec
	projectID := "test-project"
	spec := createPermissionTestProjectSpec(projectID, map[string]string{
		userID: "platform-admin",
	})
	project, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test GetUserRole
	role, err := permService.GetUserRole(ctx, userID, project.Name)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}

	if role != RolePlatformAdmin {
		t.Errorf("Expected role %s, got %s", RolePlatformAdmin, role)
	}

	// Test non-member
	outsiderID := "outsider-user"
	role, err = permService.GetUserRole(ctx, outsiderID, project.Name)
	if err != nil {
		t.Fatalf("GetUserRole failed: %v", err)
	}

	if role != "" {
		t.Errorf("Expected empty role for non-member, got %s", role)
	}
}

func TestGetUserPermissions(t *testing.T) {
	ctx := context.Background()

	// Setup
	projectService, permService, _ := setupTestServices()

	// Define developer user
	userID := "developer-user"

	// Create project with user as developer using ArgoCD-aligned spec
	projectID := "test-project"
	spec := createPermissionTestProjectSpec(projectID, map[string]string{
		userID: "developer",
	})
	project, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test GetUserPermissions
	permissions, err := permService.GetUserPermissions(ctx, userID, project.Name)
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}

	// Developer should have these permissions
	expectedPerms := map[Permission]bool{
		PermissionProjectRead:    true,
		PermissionInstanceDeploy: true,
		PermissionInstanceDelete: true,
		PermissionInstanceView:   true,
		PermissionRGDView:        true,
	}

	// Developer should NOT have these permissions
	forbiddenPerms := map[Permission]bool{
		PermissionProjectCreate:       true,
		PermissionProjectDelete:       true,
		PermissionProjectUpdate:       true,
		PermissionProjectMemberAdd:    true,
		PermissionProjectMemberRemove: true,
	}

	// Build permission map
	permMap := make(map[Permission]bool)
	for _, perm := range permissions {
		permMap[perm] = true
	}

	// Verify expected permissions
	for perm := range expectedPerms {
		if !permMap[perm] {
			t.Errorf("Developer should have permission: %s", perm)
		}
	}

	// Verify forbidden permissions are NOT present
	for perm := range forbiddenPerms {
		if permMap[perm] {
			t.Errorf("Developer should NOT have permission: %s", perm)
		}
	}
}

// TestCacheInvalidation_ViaPermissionService tests that cache invalidation delegates to PolicyEnforcer
func TestCacheInvalidation_ViaPermissionService(t *testing.T) {
	ctx := context.Background()

	// Setup services
	projectService, permService, policyEnforcer := setupTestServices()

	// Create test user reference and project
	userID := "cache-test-user"
	projectID := "cache-test-project"
	spec := createPermissionTestProjectSpec(projectID, map[string]string{
		userID: "developer",
	})
	project, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Configure mock policy enforcer
	policyEnforcer.setUserProjects(userID, []string{project.Name})

	// Test InvalidateUserCache - should not error when PolicyEnforcer is set
	err = permService.InvalidateUserCache(ctx, userID)
	if err != nil {
		t.Errorf("InvalidateUserCache returned error: %v", err)
	}

	// Test InvalidateProjectCache - should not error when PolicyEnforcer is set
	err = permService.InvalidateProjectCache(ctx, project.Name)
	if err != nil {
		t.Errorf("InvalidateProjectCache returned error: %v", err)
	}
}

// TestGetUserPermissions_ExtendedCases tests additional GetUserPermissions scenarios
func TestGetUserPermissions_ExtendedCases(t *testing.T) {
	ctx := context.Background()
	projectService, permService, _ := setupTestServices()

	// Create test project with admin user
	userID := "admin-user-ext"
	projectID := "getuserpermissions-project"
	spec := createPermissionTestProjectSpec(projectID, map[string]string{
		userID: "platform-admin",
	})
	project, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	tests := []struct {
		name          string
		userID        string
		resourceID    string
		wantErr       bool
		minPermCount  int
		checkContains []Permission
	}{
		{
			name:          "Admin has multiple permissions",
			userID:        userID,
			resourceID:    project.Name,
			wantErr:       false,
			minPermCount:  2,
			checkContains: []Permission{PermissionProjectRead, PermissionProjectUpdate},
		},
		{
			// This prevents information disclosure about user existence
			// and is consistent with Casbin-only role checks
			name:         "Non-existent user returns empty permissions",
			userID:       "nonexistent-user",
			resourceID:   project.Name,
			wantErr:      false,
			minPermCount: 0,
		},
		{
			name:         "Non-existent project returns error",
			userID:       userID,
			resourceID:   "nonexistent-project",
			wantErr:      true,
			minPermCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms, err := permService.GetUserPermissions(ctx, tt.userID, tt.resourceID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetUserPermissions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(perms) < tt.minPermCount {
					t.Errorf("GetUserPermissions() returned %d permissions, want at least %d", len(perms), tt.minPermCount)
				}
				for _, expected := range tt.checkContains {
					found := false
					for _, perm := range perms {
						if perm == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("GetUserPermissions() missing expected permission %s", expected)
					}
				}
			}
		})
	}
}

// TestGetUserProjects_NoProjects tests GetUserProjects when user has no projects
func TestGetUserProjects_NoProjects(t *testing.T) {
	_, permService, _ := setupTestServices()
	ctx := context.Background()

	// User with no projects configured in policy enforcer
	userID := "no-projects-user"

	// Get projects - should return empty list, not error
	projects, err := permService.GetUserProjects(ctx, userID)
	if err != nil {
		t.Errorf("GetUserProjects() unexpected error = %v", err)
	}
	if projects == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
}

// TestGetUserProjects_MissingProject tests GetUserProjects when policy grants access to non-existent project
func TestGetUserProjects_MissingProject(t *testing.T) {
	projectService, permService, policyEnforcer := setupTestServices()
	ctx := context.Background()

	// Create one real project
	projectID := "existing-project"
	spec := createPermissionTestProjectSpec(projectID, map[string]string{})
	project, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Configure user to have access to both existing and non-existing project
	userID := "orphan-project-user"
	policyEnforcer.setUserProjects(userID, []string{project.Name, "non-existent-project"})

	// GetUserProjects should still return existing project, skipping non-existent one
	projects, err := permService.GetUserProjects(ctx, userID)
	if err != nil {
		t.Errorf("GetUserProjects() unexpected error = %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("Expected 1 project (skipping non-existent), got %d", len(projects))
	}
	if len(projects) == 1 && projects[0].Name != project.Name {
		t.Errorf("Expected project %s, got %s", project.Name, projects[0].Name)
	}
}

// TestGetUserRole_NonExistentProject tests GetUserRole with a non-existent project
func TestGetUserRole_NonExistentProject(t *testing.T) {
	_, permService, _ := setupTestServices()
	ctx := context.Background()

	userID := "test-user"

	// Get role for non-existent project - should return error
	role, err := permService.GetUserRole(ctx, userID, "non-existent-project")
	if err == nil {
		t.Error("Expected error for non-existent project, got nil")
	}
	if role != "" {
		t.Errorf("Expected empty role for non-existent project, got %q", role)
	}
}

// TestGetUserNamespaces_NonExistentUser tests GetUserNamespaces with a non-existent user
// This prevents information disclosure about user existence and aligns with unified approach
func TestGetUserNamespaces_NonExistentUser(t *testing.T) {
	_, permService, _ := setupTestServices()
	ctx := context.Background()

	// Get namespaces for non-existent user
	// This is a secure default that doesn't reveal user existence
	namespaces, err := permService.GetUserNamespaces(ctx, "non-existent-user")
	if err != nil {
		t.Errorf("Expected no error for non-existent user, got %v", err)
	}
	if len(namespaces) != 0 {
		t.Errorf("Expected empty namespaces for non-existent user, got %v", namespaces)
	}
}

// TestGetUserPermissions_NonExistentUser tests GetUserPermissions with non-existent user
// This prevents information disclosure about user existence and aligns with Casbin-only role checks
func TestGetUserPermissions_NonExistentUser(t *testing.T) {
	projectService, permService, _ := setupTestServices()
	ctx := context.Background()

	// Create an existing project to test against
	projectID := "non-existent-user-test-project"
	spec := ProjectSpec{
		Description: "Test Project",
		Destinations: []Destination{
			{Namespace: projectID},
		},
		NamespaceResourceWhitelist: []ResourceSpec{{Group: "*", Kind: "*"}},
		Roles: []ProjectRole{
			{
				Name:        "platform-admin",
				Description: "Admin role",
				Policies:    []string{fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", projectID, projectID)},
				Groups:      []string{"admin:system"},
			},
		},
	}
	_, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Get permissions for non-existent user on existing project
	// Should return empty permissions (not error) to prevent user existence disclosure
	perms, err := permService.GetUserPermissions(ctx, "non-existent-user", projectID)
	if err != nil {
		t.Errorf("Expected no error for non-existent user, got %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("Expected empty permissions for non-existent user, got %v", perms)
	}
}

// TestGetUserPermissions_NonMember tests GetUserPermissions for user not in project
func TestGetUserPermissions_NonMember(t *testing.T) {
	projectService, permService, _ := setupTestServices()
	ctx := context.Background()

	// Non-member user ID
	userID := "nonmember-user"

	// Create a project without adding the user
	projectID := "non-member-project"
	spec := ProjectSpec{
		Description: "Test Project",
		Destinations: []Destination{
			{Namespace: projectID},
		},
		NamespaceResourceWhitelist: []ResourceSpec{{Group: "*", Kind: "*"}},
		Roles: []ProjectRole{
			{
				Name:        "platform-admin",
				Description: "Admin role",
				Policies:    []string{fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", projectID, projectID)},
				Groups:      []string{"admin:system"},
			},
		},
	}
	project, err := projectService.CreateProject(ctx, projectID, spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Get permissions - user is not a member, should return empty list
	perms, err := permService.GetUserPermissions(ctx, userID, project.Name)
	if err != nil {
		t.Errorf("GetUserPermissions() unexpected error = %v", err)
	}
	if len(perms) != 0 {
		t.Errorf("Expected 0 permissions for non-member, got %d: %v", len(perms), perms)
	}
}

// TestGetUserPermissions_NonExistentProject tests GetUserPermissions with non-existent project
func TestGetUserPermissions_NonExistentProject(t *testing.T) {
	_, permService, _ := setupTestServices()
	ctx := context.Background()

	userID := "test-user"

	// Get permissions for non-existent project - should return error
	perms, err := permService.GetUserPermissions(ctx, userID, "non-existent-project")
	if err == nil {
		t.Error("Expected error for non-existent project, got nil")
	}
	if perms != nil {
		t.Errorf("Expected nil permissions for non-existent project, got %v", perms)
	}
}
