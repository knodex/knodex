// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

// NOTE: Tests in this file are NOT safe for t.Parallel() due to shared K8s fake client
// and mockIntegrationPolicyEnforcer state (assignedRoles/accessibleProjects maps mutated across test steps).
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

// Integration tests updated to work without UserService/User CRD.
// User-project access is now determined by:
// 1. OIDC users: Group mappings in Project spec.roles.groups
// 2. Local users: Casbin policies (via AccountStore)
// All user project visibility now comes from PolicyEnforcer.GetAccessibleProjects()

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// Helper function to create an ArgoCD-aligned project spec for integration tests
func createIntegrationTestProjectSpec(projectID, description string, userGroups map[string]string) ProjectSpec {
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

	// Add user groups to their respective roles
	for userID, role := range userGroups {
		userGroup := fmt.Sprintf("user:%s", userID)
		for i := range roles {
			if roles[i].Name == role {
				roles[i].Groups = append(roles[i].Groups, userGroup)
				break
			}
		}
	}

	return ProjectSpec{
		Description: description,
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

// Helper function to get the first destination namespace from a project
func getProjectNamespace(project *Project) string {
	if len(project.Spec.Destinations) > 0 {
		return project.Spec.Destinations[0].Namespace
	}
	return ""
}

// TestIntegration_UserProjectVisibility tests that users can only see their projects
// Updated to use PolicyEnforcer for user project access
func TestIntegration_UserProjectVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Define user IDs (no User CRD creation needed)
	globalAdminID := "admin-user-123"
	user1ID := "user1-456"
	user2ID := "user2-789"

	// Create three projects
	project1Spec := createIntegrationTestProjectSpec("project-one", "Project One", map[string]string{
		user1ID: "developer",
	})
	project1, err := services.projectService.CreateProject(ctx, "project-one", project1Spec, globalAdminID)
	if err != nil {
		t.Fatalf("Failed to create project1: %v", err)
	}

	project2Spec := createIntegrationTestProjectSpec("project-two", "Project Two", map[string]string{
		user1ID: "developer",
	})
	project2, err := services.projectService.CreateProject(ctx, "project-two", project2Spec, globalAdminID)
	if err != nil {
		t.Fatalf("Failed to create project2: %v", err)
	}

	project3Spec := createIntegrationTestProjectSpec("project-three", "Project Three", map[string]string{
		user2ID: "developer",
	})
	project3, err := services.projectService.CreateProject(ctx, "project-three", project3Spec, globalAdminID)
	if err != nil {
		t.Fatalf("Failed to create project3: %v", err)
	}

	// Configure mock policy enforcer to define user project access
	services.policyEnforcer.setUserProjects(user1ID, []string{project1.Name, project2.Name})
	services.policyEnforcer.setUserProjects(user2ID, []string{project3.Name})
	// Global admin has access to all projects
	services.policyEnforcer.AssignRole(ctx, "user:"+globalAdminID, CasbinRoleServerAdmin)
	services.policyEnforcer.setAllProjects([]string{project1.Name, project2.Name, project3.Name})

	// Test: User1 can see project1 and project2
	user1Projects, err := services.permService.GetUserProjects(ctx, user1ID)
	if err != nil {
		t.Fatalf("GetUserProjects(user1) failed: %v", err)
	}

	if len(user1Projects) != 2 {
		t.Errorf("User1 should see 2 projects, got %d", len(user1Projects))
	}

	// Verify project1 and project2 are in the list
	hasProject1 := false
	hasProject2 := false
	for _, p := range user1Projects {
		if p.Name == project1.Name {
			hasProject1 = true
		}
		if p.Name == project2.Name {
			hasProject2 = true
		}
	}
	if !hasProject1 {
		t.Error("User1 should see project1")
	}
	if !hasProject2 {
		t.Error("User1 should see project2")
	}

	// Test: User2 can only see project3
	user2Projects, err := services.permService.GetUserProjects(ctx, user2ID)
	if err != nil {
		t.Fatalf("GetUserProjects(user2) failed: %v", err)
	}

	if len(user2Projects) != 1 {
		t.Errorf("User2 should see 1 project, got %d", len(user2Projects))
	}
	if len(user2Projects) > 0 && user2Projects[0].Name != "project-three" {
		t.Errorf("User2 should see project-three, got %s", user2Projects[0].Name)
	}
}

// TestIntegration_NamespaceFiltering tests that users can only access their project namespaces
// Updated to use PolicyEnforcer for namespace access
func TestIntegration_NamespaceFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Define user IDs
	globalAdminID := "admin-user-ns"
	user1ID := "user1-ns"

	// Create projects with namespaces
	project1Spec := createIntegrationTestProjectSpec("instance-alpha", "Instance Test Alpha", map[string]string{
		user1ID: "developer",
	})
	project1, err := services.projectService.CreateProject(ctx, "instance-alpha", project1Spec, globalAdminID)
	if err != nil {
		t.Fatalf("Failed to create project1: %v", err)
	}

	project2Spec := createIntegrationTestProjectSpec("instance-beta", "Instance Test Beta", map[string]string{})
	project2, err := services.projectService.CreateProject(ctx, "instance-beta", project2Spec, globalAdminID)
	if err != nil {
		t.Fatalf("Failed to create project2: %v", err)
	}

	// Configure mock policy enforcer
	services.policyEnforcer.setUserProjects(user1ID, []string{project1.Name})

	// Test: User can only access their project namespace
	namespaces, err := services.permService.GetUserNamespaces(ctx, user1ID)
	if err != nil {
		t.Fatalf("GetUserNamespaces(user1) failed: %v", err)
	}

	// User1 should have access to instance-alpha namespace
	expectedNS := getProjectNamespace(project1)
	forbiddenNS := getProjectNamespace(project2)

	hasExpected := false
	hasForbidden := false
	for _, ns := range namespaces {
		if ns == expectedNS {
			hasExpected = true
		}
		if ns == forbiddenNS {
			hasForbidden = true
		}
	}

	if !hasExpected {
		t.Errorf("User1 should have access to namespace %s", expectedNS)
	}
	if hasForbidden {
		t.Errorf("User1 should NOT have access to namespace %s", forbiddenNS)
	}
}

// TestIntegration_UserRoleRetrieval tests that user roles are correctly retrieved
// Updated to use Project role assignments directly
func TestIntegration_UserRoleRetrieval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Define user IDs
	adminUserID := "project-admin-user"
	developerUserID := "developer-user"
	viewerUserID := "viewer-user"

	// Create project with different users in different roles
	projectSpec := createIntegrationTestProjectSpec("role-test-project", "Role Test Project", map[string]string{
		adminUserID:     "platform-admin",
		developerUserID: "developer",
		viewerUserID:    "viewer",
	})
	project, err := services.projectService.CreateProject(ctx, "role-test-project", projectSpec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test: Retrieve admin role
	adminRole, err := services.permService.GetUserRole(ctx, adminUserID, project.Name)
	if err != nil {
		t.Fatalf("GetUserRole(admin) failed: %v", err)
	}
	if adminRole != RolePlatformAdmin {
		t.Errorf("Expected admin role %s, got %s", RolePlatformAdmin, adminRole)
	}

	// Test: Retrieve developer role
	devRole, err := services.permService.GetUserRole(ctx, developerUserID, project.Name)
	if err != nil {
		t.Fatalf("GetUserRole(developer) failed: %v", err)
	}
	if devRole != RoleDeveloper {
		t.Errorf("Expected developer role %s, got %s", RoleDeveloper, devRole)
	}

	// Test: Retrieve viewer role
	viewerRole, err := services.permService.GetUserRole(ctx, viewerUserID, project.Name)
	if err != nil {
		t.Fatalf("GetUserRole(viewer) failed: %v", err)
	}
	if viewerRole != RoleViewer {
		t.Errorf("Expected viewer role %s, got %s", RoleViewer, viewerRole)
	}

	// Test: Non-member returns empty role
	nonMemberRole, err := services.permService.GetUserRole(ctx, "non-member-user", project.Name)
	if err != nil {
		t.Fatalf("GetUserRole(non-member) failed: %v", err)
	}
	if nonMemberRole != "" {
		t.Errorf("Expected empty role for non-member, got %s", nonMemberRole)
	}
}

// TestIntegration_GlobalAdminNamespaceAccess tests that global admins can access all namespaces
// Updated to use PolicyEnforcer for global admin access
func TestIntegration_GlobalAdminNamespaceAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Define user IDs
	globalAdminID := "global-admin-user"

	// Create multiple projects
	project1Spec := createIntegrationTestProjectSpec("global-ns-project-1", "Global Admin NS Project 1", nil)
	project1, err := services.projectService.CreateProject(ctx, "global-ns-project-1", project1Spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project1: %v", err)
	}

	project2Spec := createIntegrationTestProjectSpec("global-ns-project-2", "Global Admin NS Project 2", nil)
	project2, err := services.projectService.CreateProject(ctx, "global-ns-project-2", project2Spec, "system")
	if err != nil {
		t.Fatalf("Failed to create project2: %v", err)
	}

	// Configure mock policy enforcer - global admin has access to all projects
	services.policyEnforcer.AssignRole(ctx, "user:"+globalAdminID, CasbinRoleServerAdmin)
	services.policyEnforcer.setAllProjects([]string{project1.Name, project2.Name})

	// Test: Global admin can see all projects
	adminProjects, err := services.permService.GetUserProjects(ctx, globalAdminID)
	if err != nil {
		t.Fatalf("GetUserProjects(globalAdmin) failed: %v", err)
	}

	if len(adminProjects) != 2 {
		t.Errorf("Global admin should see all 2 projects, got %d", len(adminProjects))
	}

	// Global admin should have access to all namespaces
	ns1 := getProjectNamespace(project1)
	ns2 := getProjectNamespace(project2)

	namespaces, err := services.permService.GetUserNamespacesWithGroups(ctx, globalAdminID, nil)
	if err != nil {
		t.Fatalf("GetUserNamespacesWithGroups(globalAdmin) failed: %v", err)
	}

	hasNS1 := false
	hasNS2 := false
	for _, ns := range namespaces {
		if ns == ns1 {
			hasNS1 = true
		}
		if ns == ns2 {
			hasNS2 = true
		}
	}

	if !hasNS1 {
		t.Errorf("Global admin should have access to namespace %s", ns1)
	}
	if !hasNS2 {
		t.Errorf("Global admin should have access to namespace %s", ns2)
	}
}

// TestIntegration_UserPermissions tests that user permissions are correctly computed
// Updated to use Project role-based permissions
func TestIntegration_UserPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Define user IDs
	adminUserID := "perm-admin-user"
	developerUserID := "perm-developer-user"
	viewerUserID := "perm-viewer-user"

	// Create project with different roles
	projectSpec := createIntegrationTestProjectSpec("perm-test-project", "Permission Test Project", map[string]string{
		adminUserID:     "platform-admin",
		developerUserID: "developer",
		viewerUserID:    "viewer",
	})
	project, err := services.projectService.CreateProject(ctx, "perm-test-project", projectSpec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Test: Admin has full permissions
	adminPerms, err := services.permService.GetUserPermissions(ctx, adminUserID, project.Name)
	if err != nil {
		t.Fatalf("GetUserPermissions(admin) failed: %v", err)
	}

	adminPermMap := make(map[Permission]bool)
	for _, p := range adminPerms {
		adminPermMap[p] = true
	}

	if !adminPermMap[PermissionProjectUpdate] {
		t.Error("Admin should have project update permission")
	}
	if !adminPermMap[PermissionProjectMemberAdd] {
		t.Error("Admin should have project member add permission")
	}

	// Test: Developer has deploy permissions but not admin permissions
	devPerms, err := services.permService.GetUserPermissions(ctx, developerUserID, project.Name)
	if err != nil {
		t.Fatalf("GetUserPermissions(developer) failed: %v", err)
	}

	devPermMap := make(map[Permission]bool)
	for _, p := range devPerms {
		devPermMap[p] = true
	}

	if !devPermMap[PermissionInstanceDeploy] {
		t.Error("Developer should have instance deploy permission")
	}
	if devPermMap[PermissionProjectMemberAdd] {
		t.Error("Developer should NOT have project member add permission")
	}

	// Test: Viewer has read-only permissions
	viewerPerms, err := services.permService.GetUserPermissions(ctx, viewerUserID, project.Name)
	if err != nil {
		t.Fatalf("GetUserPermissions(viewer) failed: %v", err)
	}

	viewerPermMap := make(map[Permission]bool)
	for _, p := range viewerPerms {
		viewerPermMap[p] = true
	}

	if !viewerPermMap[PermissionProjectRead] {
		t.Error("Viewer should have project read permission")
	}
	if viewerPermMap[PermissionInstanceDeploy] {
		t.Error("Viewer should NOT have instance deploy permission")
	}
}

// TestIntegration_NamespacesWithGroups tests GetUserNamespacesWithGroups
// Tests OIDC group-based namespace access
func TestIntegration_NamespacesWithGroups(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Create a project with OIDC group mapping
	projectSpec := ProjectSpec{
		Description: "OIDC Group Test Project",
		Destinations: []Destination{
			{
				Namespace: "oidc-ns-project",
			},
		},
		NamespaceResourceWhitelist: []ResourceSpec{
			{Group: "*", Kind: "*"},
		},
		Roles: []ProjectRole{
			{
				Name:        "developer",
				Description: "Developer role",
				Policies:    []string{"p, proj:oidc-ns-project:developer, applications, *, oidc-ns-project/*, allow"},
				Groups:      []string{"engineering-team"}, // OIDC group mapping
			},
		},
	}
	project, err := services.projectService.CreateProject(ctx, "oidc-ns-project", projectSpec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Define a user with OIDC groups
	userID := "oidc-user-123"
	oidcGroups := []string{"engineering-team", "other-team"}

	// Configure mock policy enforcer to grant access based on groups
	services.policyEnforcer.setUserProjectsForGroups(userID, oidcGroups, []string{project.Name})

	// Test: User with matching OIDC group can access the namespace
	namespaces, err := services.permService.GetUserNamespacesWithGroups(ctx, userID, oidcGroups)
	if err != nil {
		t.Fatalf("GetUserNamespacesWithGroups failed: %v", err)
	}

	expectedNS := getProjectNamespace(project)
	hasExpected := false
	for _, ns := range namespaces {
		if ns == expectedNS {
			hasExpected = true
		}
	}

	if !hasExpected {
		t.Errorf("User with matching OIDC group should have access to namespace %s, got %v", expectedNS, namespaces)
	}
}

// TestIntegration_CacheInvalidation tests cache invalidation via PermissionService
// Cache invalidation now delegates to PolicyEnforcer
func TestIntegration_CacheInvalidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	services := setupIntegrationTestServices(t)

	// Define user ID
	userID := "cache-test-user"

	// Create a project
	projectSpec := createIntegrationTestProjectSpec("cache-project", "Cache Test Project", map[string]string{
		userID: "developer",
	})
	project, err := services.projectService.CreateProject(ctx, "cache-project", projectSpec, "system")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Configure mock policy enforcer
	services.policyEnforcer.setUserProjects(userID, []string{project.Name})

	// Test: InvalidateUserCache should not error
	err = services.permService.InvalidateUserCache(ctx, userID)
	if err != nil {
		t.Errorf("InvalidateUserCache should not error: %v", err)
	}

	// Test: InvalidateProjectCache should not error
	err = services.permService.InvalidateProjectCache(ctx, project.Name)
	if err != nil {
		t.Errorf("InvalidateProjectCache should not error: %v", err)
	}

	// Verify user can still access the project after cache invalidation
	projects, err := services.permService.GetUserProjects(ctx, userID)
	if err != nil {
		t.Fatalf("GetUserProjects after cache invalidation failed: %v", err)
	}

	if len(projects) != 1 || projects[0].Name != project.Name {
		t.Errorf("User should still have access to project after cache invalidation")
	}
}

// Helper Functions

type integrationTestServices struct {
	k8sClient      *k8sfake.Clientset
	dynamicClient  *dynamicfake.FakeDynamicClient
	projectService *ProjectService
	permService    *PermissionService
	policyEnforcer *mockIntegrationPolicyEnforcer
	logger         *slog.Logger
}

// projectLister is a minimal interface for listing projects in integration tests
type projectLister interface {
	ListProjects(ctx context.Context) (*ProjectList, error)
}

// mockIntegrationPolicyEnforcer implements PolicyEnforcer for integration tests
// Updated to support user-project access without User CRD
type mockIntegrationPolicyEnforcer struct {
	assignedRoles      map[string][]string // user -> roles
	accessibleProjects map[string][]string // user -> projectIDs
	projectLister      projectLister
}

func newMockIntegrationPolicyEnforcer() *mockIntegrationPolicyEnforcer {
	return &mockIntegrationPolicyEnforcer{
		assignedRoles:      make(map[string][]string),
		accessibleProjects: make(map[string][]string),
	}
}

// SetProjectLister sets the project lister for GetAccessibleProjects
func (m *mockIntegrationPolicyEnforcer) SetProjectLister(pl projectLister) {
	m.projectLister = pl
}

func (m *mockIntegrationPolicyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	// Only users with role:serveradmin have wildcard access
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == CasbinRoleServerAdmin {
			return true, nil
		}
	}
	// Regular users don't have wildcard permission
	return false, nil
}

func (m *mockIntegrationPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	return m.CanAccess(ctx, user, object, action)
}

func (m *mockIntegrationPolicyEnforcer) AssignRole(ctx context.Context, user, role string) error {
	m.assignedRoles[user] = append(m.assignedRoles[user], role)
	return nil
}

func (m *mockIntegrationPolicyEnforcer) RemoveRole(ctx context.Context, user, role string) error {
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
func (m *mockIntegrationPolicyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	m.assignedRoles[user] = roles
	return nil
}

func (m *mockIntegrationPolicyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	return m.assignedRoles[user], nil
}

func (m *mockIntegrationPolicyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	// Check if global admin
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == CasbinRoleServerAdmin {
			// Return all projects for global admin
			return m.accessibleProjects["*"], nil
		}
	}

	// Check for group-based access
	key := fmt.Sprintf("groups:%s:%v", user, groups)
	if projects, ok := m.accessibleProjects[key]; ok {
		return projects, nil
	}

	return m.accessibleProjects[user], nil
}

func (m *mockIntegrationPolicyEnforcer) SyncPolicies(ctx context.Context) error {
	return nil
}

func (m *mockIntegrationPolicyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	allowed, err := m.CanAccess(ctx, user, "projects/"+projectName, action)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrAccessDenied
	}
	return nil
}

func (m *mockIntegrationPolicyEnforcer) LoadProjectPolicies(ctx context.Context, project *Project) error {
	return nil
}

func (m *mockIntegrationPolicyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	roles := m.assignedRoles[user]
	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockIntegrationPolicyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	delete(m.assignedRoles, user)
	return nil
}

func (m *mockIntegrationPolicyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	return m.RemoveRole(ctx, user, role)
}

func (m *mockIntegrationPolicyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	return nil
}

func (m *mockIntegrationPolicyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	return nil
}

func (m *mockIntegrationPolicyEnforcer) InvalidateCache() {
	// No-op for mock
}

func (m *mockIntegrationPolicyEnforcer) InvalidateCacheForUser(user string) int {
	return 0 // No-op for mock
}

func (m *mockIntegrationPolicyEnforcer) InvalidateCacheForProject(project string) int {
	return 0 // No-op for mock
}

func (m *mockIntegrationPolicyEnforcer) CacheStats() CacheStats {
	return CacheStats{}
}

func (m *mockIntegrationPolicyEnforcer) Metrics() PolicyMetrics {
	return PolicyMetrics{}
}

func (m *mockIntegrationPolicyEnforcer) IncrementPolicyReloads() {
	// No-op for mock
}

func (m *mockIntegrationPolicyEnforcer) IncrementBackgroundSyncs() {
	// No-op for mock
}

func (m *mockIntegrationPolicyEnforcer) IncrementWatcherRestarts() {
	// No-op for mock
}

// setUserProjects sets accessible projects for a user
func (m *mockIntegrationPolicyEnforcer) setUserProjects(userID string, projects []string) {
	m.accessibleProjects["user:"+userID] = projects
}

// setAllProjects sets the list of all projects (for global admin access)
func (m *mockIntegrationPolicyEnforcer) setAllProjects(projects []string) {
	m.accessibleProjects["*"] = projects
}

// setUserProjectsForGroups sets accessible projects for a user based on OIDC groups
func (m *mockIntegrationPolicyEnforcer) setUserProjectsForGroups(userID string, groups []string, projects []string) {
	key := fmt.Sprintf("groups:user:%s:%v", userID, groups)
	m.accessibleProjects[key] = projects
	// Also set for user: prefix for GetUserNamespacesWithGroups
	m.accessibleProjects["user:"+userID] = projects
}

// setupIntegrationTestServices creates test services for integration tests
// No longer creates UserService - uses PolicyEnforcer for user access
func setupIntegrationTestServices(t *testing.T) *integrationTestServices {
	t.Helper()

	k8sClient := k8sfake.NewSimpleClientset()

	// Register Project CRD in scheme (no User CRD needed)
	scheme := runtime.NewScheme()
	projectGV := schema.GroupVersion{Group: ProjectGroup, Version: ProjectVersion}
	scheme.AddKnownTypeWithName(projectGV.WithKind("Project"), &Project{})
	scheme.AddKnownTypeWithName(projectGV.WithKind("ProjectList"), &ProjectList{})
	metav1.AddToGroupVersion(scheme, projectGV)

	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))

	// Create services (no UserService)
	projectService := NewProjectService(k8sClient, dynamicClient)

	policyEnforcer := newMockIntegrationPolicyEnforcer()
	policyEnforcer.SetProjectLister(projectService)

	// PermissionService no longer uses UserService
	permService := NewPermissionService(PermissionServiceConfig{
		ProjectService: projectService,
		PolicyEnforcer: policyEnforcer,
		Logger:         logger,
	})

	return &integrationTestServices{
		k8sClient:      k8sClient,
		dynamicClient:  dynamicClient,
		projectService: projectService,
		permService:    permService,
		policyEnforcer: policyEnforcer,
		logger:         logger,
	}
}
