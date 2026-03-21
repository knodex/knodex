// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPolicyEnforcer_LoadProjectPolicies tests loading policies from Project CRD
func TestPolicyEnforcer_LoadProjectPolicies(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Create a project with roles
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "engineering",
		},
		Spec: ProjectSpec{
			Description: "Engineering project",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer role",
					Policies: []string{
						"projects/engineering, get, allow",
						"rgds/*, get, allow",
						"instances/engineering-*, create, allow",
					},
					Groups: []string{"engineering-devs"},
				},
				{
					Name:        "maintainer",
					Description: "Maintainer role",
					Policies: []string{
						"projects/engineering, *, allow",
						"rgds/*, *, allow",
						"instances/engineering-*, *, allow",
					},
				},
			},
		},
	}

	// Load project policies
	err := pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Assign developer role to a user
	err = pe.AssignUserRoles(context.Background(), "user:dev1", []string{"proj:engineering:developer"})
	require.NoError(t, err)

	// Assign maintainer role to another user
	err = pe.AssignUserRoles(context.Background(), "user:maintainer1", []string{"proj:engineering:maintainer"})
	require.NoError(t, err)

	// Test developer permissions
	t.Run("developer can get project", func(t *testing.T) {
		t.Parallel()

		allowed, err := pe.CanAccess(context.Background(), "user:dev1", "projects/engineering", "get")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("developer can create instances", func(t *testing.T) {
		t.Parallel()

		allowed, err := pe.CanAccess(context.Background(), "user:dev1", "instances/engineering-app1", "create")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("developer cannot delete project", func(t *testing.T) {
		t.Parallel()

		allowed, err := pe.CanAccess(context.Background(), "user:dev1", "projects/engineering", "delete")
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	// Test maintainer permissions
	t.Run("maintainer can delete project", func(t *testing.T) {
		t.Parallel()

		allowed, err := pe.CanAccess(context.Background(), "user:maintainer1", "projects/engineering", "delete")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("maintainer can delete instances", func(t *testing.T) {
		t.Parallel()

		allowed, err := pe.CanAccess(context.Background(), "user:maintainer1", "instances/engineering-app1", "delete")
		require.NoError(t, err)
		assert.True(t, allowed)
	})
}

// TestPolicyEnforcer_LoadProjectPolicies_Errors tests error handling
func TestPolicyEnforcer_LoadProjectPolicies_Errors(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	tests := []struct {
		name        string
		project     *Project
		wantErr     bool
		errContains string
	}{
		{
			name:        "nil project",
			project:     nil,
			wantErr:     true,
			errContains: "project cannot be nil",
		},
		{
			name: "empty project name",
			project: &Project{
				ObjectMeta: metav1.ObjectMeta{Name: ""},
			},
			wantErr:     true,
			errContains: "project name cannot be empty",
		},
		{
			name: "invalid role name with colon",
			project: &Project{
				ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
				Spec: ProjectSpec{
					Roles: []ProjectRole{
						{
							Name:     "role:invalid",
							Policies: []string{"projects/test, get, allow"},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "invalid role",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := pe.LoadProjectPolicies(context.Background(), tt.project)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestPolicyEnforcer_LoadProjectPolicies_Idempotent tests idempotency
func TestPolicyEnforcer_LoadProjectPolicies_Idempotent(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "viewer",
					Policies: []string{"projects/test-project, get, allow"},
				},
			},
		},
	}

	// Load twice - should not cause errors or duplicate policies
	err := pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	err = pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Assign role and verify access works
	err = pe.AssignUserRoles(context.Background(), "user:test", []string{"proj:test-project:viewer"})
	require.NoError(t, err)

	allowed, err := pe.CanAccess(context.Background(), "user:test", "projects/test-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestPolicyEnforcer_SyncPolicies tests policy synchronization
func TestPolicyEnforcer_SyncPolicies(t *testing.T) {
	t.Parallel()

	pe, mockReader := newTestEnforcerWithMock(t)

	// Add projects to mock service
	mockReader.AddProject(&Project{
		ObjectMeta: metav1.ObjectMeta{Name: "project-a"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "reader",
					Policies: []string{"projects/project-a, get, allow"},
				},
			},
		},
	})
	mockReader.AddProject(&Project{
		ObjectMeta: metav1.ObjectMeta{Name: "project-b"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "admin",
					Policies: []string{"projects/project-b, *, allow"},
				},
			},
		},
	})

	// Sync policies
	err := pe.SyncPolicies(context.Background())
	require.NoError(t, err)

	// Verify roles can be assigned from synced projects
	err = pe.AssignUserRoles(context.Background(), "user:reader", []string{"proj:project-a:reader"})
	require.NoError(t, err)

	allowed, err := pe.CanAccess(context.Background(), "user:reader", "projects/project-a", "get")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Remove a project and re-sync
	mockReader.RemoveProject("project-a")

	err = pe.SyncPolicies(context.Background())
	require.NoError(t, err)

	// Project-a role should no longer grant access (policies removed)
	// Note: The user still has the role assigned but the role's policies are gone
}

// TestPolicyEnforcer_SyncPolicies_NoService tests error when no service configured
func TestPolicyEnforcer_SyncPolicies_NoService(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	err := pe.SyncPolicies(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project service not configured")
}

// TestPolicyEnforcer_SyncPolicies_ListError tests error handling during sync
func TestPolicyEnforcer_SyncPolicies_ListError(t *testing.T) {
	t.Parallel()

	pe, mockReader := newTestEnforcerWithMock(t)
	mockReader.listErr = errors.New("kubernetes unavailable")

	err := pe.SyncPolicies(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list projects")
}

// TestPolicyEnforcer_SyncPoliciesEmptyProjects tests SyncPolicies when no projects exist
func TestPolicyEnforcer_SyncPoliciesEmptyProjects(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create a mock project reader that returns empty list
	mockReader := &mockEmptyProjectReader{}

	pe := NewPolicyEnforcerWithConfig(enforcer, mockReader, DefaultPolicyEnforcerConfig())
	ctx := context.Background()

	// SyncPolicies should succeed with empty project list
	err = pe.SyncPolicies(ctx)
	require.NoError(t, err)
}

// TestPolicyEnforcer_ProjectRole_PolicyUpdate tests project admin can update role policies
func TestPolicyEnforcer_ProjectRole_PolicyUpdate(t *testing.T) {
	t.Parallel()

	pe, _ := newTestEnforcerWithMock(t)
	ctx := context.Background()

	// Create initial project with admin role
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-policy-test"},
		Spec: ProjectSpec{
			Description: "Project for policy update testing",
			Roles: []ProjectRole{
				{
					Name:        "admin",
					Description: "Project admin",
					Policies: []string{
						"p, proj:proj-policy-test:admin, projects, get, proj-policy-test, allow",
						"p, proj:proj-policy-test:admin, projects, update, proj-policy-test, allow",
					},
					Groups: []string{"admin-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Verify admin has project update permission (required for role management)
	allowed, err := pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, "projects/proj-policy-test", "update")
	require.NoError(t, err)
	assert.True(t, allowed, "Project admin should have project update permission")

	// Update project with new role policies (simulating API update)
	updatedProject := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-policy-test"},
		Spec: ProjectSpec{
			Description: "Project for policy update testing",
			Roles: []ProjectRole{
				{
					Name:        "admin",
					Description: "Project admin with expanded permissions",
					Policies: []string{
						"p, proj:proj-policy-test:admin, projects, get, proj-policy-test, allow",
						"p, proj:proj-policy-test:admin, projects, update, proj-policy-test, allow",
						"p, proj:proj-policy-test:admin, instances, *, proj-policy-test/*, allow",
						"p, proj:proj-policy-test:admin, settings, get, *, allow",
					},
					Groups: []string{"admin-group"},
				},
				{
					Name:        "viewer",
					Description: "Read-only viewer",
					Policies: []string{
						"p, proj:proj-policy-test:viewer, projects, get, proj-policy-test, allow",
						"p, proj:proj-policy-test:viewer, instances, get, proj-policy-test/*, allow",
					},
					Groups: []string{"viewer-group"},
				},
			},
		},
	}

	// Reload policies with updated project
	err = pe.LoadProjectPolicies(ctx, updatedProject)
	require.NoError(t, err)

	// Verify admin now has instance and settings access
	allowed, err = pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, "instances/proj-policy-test/my-instance", "create")
	require.NoError(t, err)
	assert.True(t, allowed, "Project admin should have instance create permission after update")

	allowed, err = pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, "settings/general", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Project admin should have settings get permission after update")

	// Verify new viewer role works
	allowed, err = pe.CanAccessWithGroups(ctx, "user:viewer", []string{"viewer-group"}, "projects/proj-policy-test", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Viewer should have project get permission")

	allowed, err = pe.CanAccessWithGroups(ctx, "user:viewer", []string{"viewer-group"}, "instances/proj-policy-test/my-instance", "create")
	require.NoError(t, err)
	assert.False(t, allowed, "Viewer should NOT have instance create permission")
}

// TestPolicyEnforcer_SecretsAccess_AdminRole tests that project admin has full CRUD on secrets
func TestPolicyEnforcer_SecretsAccess_AdminRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create a project with admin role
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testproject",
		},
		Spec: ProjectSpec{
			Description: "Test project for secrets access",
			Roles: []ProjectRole{
				{
					Name:        "admin",
					Description: "Project admin",
					Policies:    []string{},
					Groups:      []string{"admin-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	tests := []struct {
		name     string
		object   string
		action   string
		expected bool
	}{
		{"admin can get secret", "secrets/testproject/mysecret", "get", true},
		{"admin can create secret", "secrets/testproject/mysecret", "create", true},
		{"admin can update secret", "secrets/testproject/mysecret", "update", true},
		{"admin can delete secret", "secrets/testproject/mysecret", "delete", true},
		{"admin can list secrets", "secrets/testproject/mysecret", "list", true},
		// Cross-project isolation
		{"admin cannot access other project secrets", "secrets/otherproject/mysecret", "get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_ReadonlyRole tests that readonly role can only get/list secrets
func TestPolicyEnforcer_SecretsAccess_ReadonlyRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create a project with readonly role
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testproject",
		},
		Spec: ProjectSpec{
			Description: "Test project for secrets access",
			Roles: []ProjectRole{
				{
					Name:        "readonly",
					Description: "Read-only role",
					Policies:    []string{},
					Groups:      []string{"readonly-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	tests := []struct {
		name     string
		object   string
		action   string
		expected bool
	}{
		// Readonly can get and list
		{"readonly can get secret", "secrets/testproject/mysecret", "get", true},
		{"readonly can list secrets", "secrets/testproject/mysecret", "list", true},
		// Readonly cannot create/update/delete
		{"readonly cannot create secret", "secrets/testproject/mysecret", "create", false},
		{"readonly cannot update secret", "secrets/testproject/mysecret", "update", false},
		{"readonly cannot delete secret", "secrets/testproject/mysecret", "delete", false},
		// Cross-project isolation
		{"readonly cannot access other project secrets", "secrets/otherproject/mysecret", "get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:viewer", []string{"readonly-group"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_WildcardScoping tests that secrets/* in a custom project
// policy is correctly scoped to the project (not passed through as global secrets/*)
func TestPolicyEnforcer_SecretsAccess_WildcardScoping(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create project "alpha" with a custom role that uses the unqualified "secrets/*" wildcard
	projectAlpha := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alpha",
		},
		Spec: ProjectSpec{
			Description: "Alpha project",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer with unscoped secrets wildcard",
					Policies: []string{
						"secrets/*, create, allow",
					},
					Groups: []string{"alpha-devs"},
				},
			},
		},
	}

	// Create project "beta" (separate tenant)
	projectBeta := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "beta",
		},
		Spec: ProjectSpec{
			Description: "Beta project",
			Roles:       []ProjectRole{},
		},
	}

	err := pe.LoadProjectPolicies(ctx, projectAlpha)
	require.NoError(t, err)
	err = pe.LoadProjectPolicies(ctx, projectBeta)
	require.NoError(t, err)

	tests := []struct {
		name     string
		object   string
		action   string
		expected bool
	}{
		// Alpha developer can create secrets within their own project (scoped correctly)
		{"alpha developer can create in alpha", "secrets/alpha/mysecret", "create", true},
		// CRITICAL: alpha developer must NOT be able to create in beta (cross-project isolation)
		{"alpha developer cannot create in beta", "secrets/beta/mysecret", "create", false},
		// Cannot create globally
		{"alpha developer cannot create globally", "secrets/other/mysecret", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:alphadev", []string{"alpha-devs"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_CustomProjectRole tests custom project role with secrets access
func TestPolicyEnforcer_SecretsAccess_CustomProjectRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create project with a custom developer role that has secrets create access
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo",
		},
		Spec: ProjectSpec{
			Description: "Demo project",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer role with secrets create",
					Policies: []string{
						"secrets/demo/*, create, allow",
					},
					Groups: []string{"dev-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	tests := []struct {
		name     string
		object   string
		action   string
		expected bool
	}{
		{"developer can create secret", "secrets/demo/mysecret", "create", true},
		{"developer cannot delete secret", "secrets/demo/mysecret", "delete", false},
		{"developer cannot get secret", "secrets/demo/mysecret", "get", false},
		// Cross-project isolation
		{"developer cannot create secret in other project", "secrets/other/mysecret", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:dev", []string{"dev-group"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_ArgoFormatPolicy tests AC #3 using the 6-part ArgoCD policy
// format: "p, proj:demo:developer, secrets, create, demo, allow"
func TestPolicyEnforcer_SecretsAccess_ArgoFormatPolicy(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Simulate AC #3: project with a developer role using 6-part ArgoCD policy format
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo",
		},
		Spec: ProjectSpec{
			Description: "Demo project",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer role using ArgoCD 6-part format",
					Policies: []string{
						// 6-part ArgoCD format: p, subject, resource_type, action, scope, effect
						"p, proj:demo:developer, secrets, create, demo/*, allow",
					},
					Groups: []string{"demo-devs"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	tests := []struct {
		name     string
		object   string
		action   string
		expected bool
	}{
		{"argo format: developer can create secret in demo", "secrets/demo/mysecret", "create", true},
		{"argo format: developer cannot delete secret", "secrets/demo/mysecret", "delete", false},
		{"argo format: developer cannot access other project", "secrets/other/mysecret", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:demodev", []string{"demo-devs"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}
