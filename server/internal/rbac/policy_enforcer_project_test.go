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

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})

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
					Teams: []string{"engineering-devs"},
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

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})

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

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})

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

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})

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
					Teams: []string{"admin-group"},
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
					Teams: []string{"admin-group"},
				},
				{
					Name:        "viewer",
					Description: "Read-only viewer",
					Policies: []string{
						"p, proj:proj-policy-test:viewer, projects, get, proj-policy-test, allow",
						"p, proj:proj-policy-test:viewer, instances, get, proj-policy-test/*, allow",
					},
					Teams: []string{"viewer-group"},
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

// TestPolicyEnforcer_SecretsAccess_AdminRole tests that a project admin with
// destinations gets full CRUD on secrets in the listed namespaces (and only
// there). Object shape is namespace-keyed (secrets/{ns}/{name}) — no project
// segment — mirroring the URL middleware's Casbin normalization.
func TestPolicyEnforcer_SecretsAccess_AdminRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})
	ctx := context.Background()

	// Admin role with one destination namespace. Per TD-7, an admin role with
	// no destinations gets no secret policies — destinations are the access
	// boundary for namespace-keyed resources.
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testproject",
		},
		Spec: ProjectSpec{
			Description: "Test project for secrets access",
			Destinations: []Destination{
				{Namespace: "test-ns"},
			},
			Roles: []ProjectRole{
				{
					Name:         "admin",
					Description:  "Project admin",
					Policies:     []string{},
					Teams:        []string{"admin-group"},
					Destinations: []string{"test-ns"},
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
		{"admin can get secret", "secrets/test-ns/mysecret", "get", true},
		{"admin can create secret", "secrets/test-ns/mysecret", "create", true},
		{"admin can update secret", "secrets/test-ns/mysecret", "update", true},
		{"admin can delete secret", "secrets/test-ns/mysecret", "delete", true},
		{"admin can list secrets", "secrets/test-ns/mysecret", "list", true},
		// Cross-namespace isolation (was "cross-project" under the old model)
		{"admin cannot access secrets in other namespace", "secrets/other-ns/mysecret", "get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_AdminRole_NoDestinations verifies TD-7:
// an admin role WITHOUT destinations gets NO secret policies. The same role
// retains all its non-secret built-ins (instances, repositories, etc.).
func TestPolicyEnforcer_SecretsAccess_AdminRole_NoDestinations(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})
	ctx := context.Background()

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "noscope",
		},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "admin",
					Policies: []string{},
					Teams:    []string{"admin-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Non-secrets built-ins still work (regression guard for the change).
	allowed, err := pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, "instances/noscope/Deployment/web", "create")
	require.NoError(t, err)
	assert.True(t, allowed, "admin without destinations should still create instances project-wide")

	// AC8: no secret policy lines emitted for a role without destinations.
	for _, action := range []string{"get", "create", "update", "delete", "list"} {
		allowed, err := pe.CanAccessWithGroups(ctx, "user:admin", []string{"admin-group"}, "secrets/any-ns/mysecret", action)
		require.NoError(t, err)
		assert.Falsef(t, allowed, "admin without destinations should NOT be able to %s secrets", action)
	}
}

// TestPolicyEnforcer_SecretsAccess_ReadonlyRole tests readonly with a single
// destination — get/list only on secrets in that namespace.
func TestPolicyEnforcer_SecretsAccess_ReadonlyRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})
	ctx := context.Background()

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testproject",
		},
		Spec: ProjectSpec{
			Description: "Test project for secrets access",
			Destinations: []Destination{
				{Namespace: "test-ns"},
			},
			Roles: []ProjectRole{
				{
					Name:         "readonly",
					Description:  "Read-only role",
					Policies:     []string{},
					Teams:        []string{"readonly-group"},
					Destinations: []string{"test-ns"},
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
		{"readonly can get secret", "secrets/test-ns/mysecret", "get", true},
		{"readonly can list secrets", "secrets/test-ns/mysecret", "list", true},
		{"readonly cannot create secret", "secrets/test-ns/mysecret", "create", false},
		{"readonly cannot update secret", "secrets/test-ns/mysecret", "update", false},
		{"readonly cannot delete secret", "secrets/test-ns/mysecret", "delete", false},
		// Cross-namespace isolation
		{"readonly cannot access secrets in other namespace", "secrets/other-ns/mysecret", "get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:viewer", []string{"readonly-group"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_SharedNamespace covers AC2 — two roles
// from different projects that both list the same namespace in destinations
// both gain access to the same Secret. This is the cross-project sharing
// behavior the namespace-keyed model unlocks.
func TestPolicyEnforcer_SecretsAccess_SharedNamespace(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})
	ctx := context.Background()

	projectAlpha := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
		Spec: ProjectSpec{
			Destinations: []Destination{{Namespace: "xxx-shared"}},
			Roles: []ProjectRole{
				{
					Name:         "developer",
					Policies:     []string{"secrets/*, get, allow"},
					Teams:        []string{"alpha-devs"},
					Destinations: []string{"xxx-shared"},
				},
			},
		},
	}
	projectBeta := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "beta"},
		Spec: ProjectSpec{
			Destinations: []Destination{{Namespace: "xxx-shared"}},
			Roles: []ProjectRole{
				{
					Name:         "developer",
					Policies:     []string{"secrets/*, get, allow"},
					Teams:        []string{"beta-devs"},
					Destinations: []string{"xxx-shared"},
				},
			},
		},
	}

	require.NoError(t, pe.LoadProjectPolicies(ctx, projectAlpha))
	require.NoError(t, pe.LoadProjectPolicies(ctx, projectBeta))

	allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-devs"}, "secrets/xxx-shared/api-key", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "alpha developer should read shared-namespace secret")

	allowed, err = pe.CanAccessWithGroups(ctx, "user:bob", []string{"beta-devs"}, "secrets/xxx-shared/api-key", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "beta developer should read the same shared-namespace secret")

	// Sanity: cross-namespace read still denied
	allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-devs"}, "secrets/xxx-private/api-key", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "alpha developer should NOT read secret outside their destinations")
}

// TestPolicyEnforcer_SecretsAccess_CustomNamespacePolicy tests that a custom
// project role with a namespace-literal secret policy (secrets/<ns>/*) grants
// access in that namespace only. Under the namespace-keyed model the literal
// "demo" is interpreted as a namespace name; cross-namespace remains denied.
func TestPolicyEnforcer_SecretsAccess_CustomNamespacePolicy(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})
	ctx := context.Background()

	// "demo" here is the namespace name in the policy object; the project name
	// matches purely as a convenience for the fixture.
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo",
		},
		Spec: ProjectSpec{
			Description: "Demo project",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer role with secrets create on namespace demo",
					Policies: []string{
						"secrets/demo/*, create, allow",
					},
					Teams: []string{"dev-group"},
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
		{"developer can create secret in demo namespace", "secrets/demo/mysecret", "create", true},
		{"developer cannot delete secret in demo namespace", "secrets/demo/mysecret", "delete", false},
		{"developer cannot get secret in demo namespace", "secrets/demo/mysecret", "get", false},
		// Cross-namespace isolation
		{"developer cannot create secret in other namespace", "secrets/other/mysecret", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:dev", []string{"dev-group"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_SecretsAccess_ArgoFormatPolicy exercises the 6-part
// ArgoCD policy format ("p, proj:demo:developer, secrets, create, demo/*, allow").
// Under the namespace-keyed model the scope segment "demo" is a namespace,
// not a project — but the constructed object "secrets/demo/*" matches the
// same URL-derived shape, so the test outcomes are unchanged.
func TestPolicyEnforcer_SecretsAccess_ArgoFormatPolicy(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcerWithTeams(t, identityTeamResolver{})
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
					Teams: []string{"demo-devs"},
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
		{"argo format: developer can create secret in demo namespace", "secrets/demo/mysecret", "create", true},
		{"argo format: developer cannot delete secret", "secrets/demo/mysecret", "delete", false},
		{"argo format: developer cannot access other namespace", "secrets/other/mysecret", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := pe.CanAccessWithGroups(ctx, "user:demodev", []string{"demo-devs"}, tt.object, tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}
