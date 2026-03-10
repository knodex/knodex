// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPolicyEnforcer_PolicyParsing tests various policy string formats
func TestPolicyEnforcer_PolicyParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policies   []string
		testObject string
		testAction string
		expected   bool
	}{
		{
			name:       "3-part format",
			policies:   []string{"projects/test, get, allow"},
			testObject: "projects/test",
			testAction: "get",
			expected:   true,
		},
		{
			name:       "wildcard action",
			policies:   []string{"projects/test, *, allow"},
			testObject: "projects/test",
			testAction: "delete",
			expected:   true,
		},
		{
			// SECURITY: "projects/*" in a project role is scoped to only that project
			// A project admin with "projects/*, get, allow" can only access their OWN project
			// This enforces project admin isolation
			name:       "wildcard object scoped to own project",
			policies:   []string{"projects/*, get, allow"},
			testObject: "projects/test-project", // Must be the project where role is defined
			testAction: "get",
			expected:   true,
		},
		{
			// SECURITY: Verify that "projects/*" does NOT grant access to other projects
			// This tests the project isolation security fix
			name:       "wildcard object denied for other project",
			policies:   []string{"projects/*, get, allow"},
			testObject: "projects/other-project", // NOT the project where role is defined
			testAction: "get",
			expected:   false, // Should be denied - cross-project access
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create fresh enforcer for each test
			pe := newTestEnforcer(t)

			project := &Project{
				ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
				Spec: ProjectSpec{
					Roles: []ProjectRole{
						{
							Name:     "testrole",
							Policies: tt.policies,
						},
					},
				},
			}

			err := pe.LoadProjectPolicies(context.Background(), project)
			require.NoError(t, err)

			err = pe.AssignUserRoles(context.Background(), "user:tester", []string{"proj:test-project:testrole"})
			require.NoError(t, err)

			allowed, err := pe.CanAccess(context.Background(), "user:tester", tt.testObject, tt.testAction)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

// TestPolicyEnforcer_PolicyParsing_RejectedFormats tests that 4-part, 5-part,
// 2-part, and 1-part policy formats are rejected for security reasons
// (to prevent subject injection and ensure valid policies)
// Valid formats are: 3-part (object, action, effect) or 6-part ArgoCD format
func TestPolicyEnforcer_PolicyParsing_RejectedFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		policies      []string
		expectedError string
	}{
		{
			name:          "4-part format rejected",
			policies:      []string{"ignored-subject, projects/test, get, allow"},
			expectedError: "must be 3 parts 'object, action, effect' or 6 parts ArgoCD format (got 4 parts)",
		},
		{
			name:          "5-part format with p prefix rejected",
			policies:      []string{"p, ignored-subject, projects/test, get, allow"},
			expectedError: "must be 3 parts 'object, action, effect' or 6 parts ArgoCD format (got 5 parts)",
		},
		{
			name:          "2-part format rejected",
			policies:      []string{"projects/test, get"},
			expectedError: "must be 3 parts 'object, action, effect' or 6 parts ArgoCD format (got 2 parts)",
		},
		{
			name:          "1-part format rejected",
			policies:      []string{"projects/test"},
			expectedError: "must be 3 parts 'object, action, effect' or 6 parts ArgoCD format (got 1 parts)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pe := newTestEnforcer(t)

			project := &Project{
				ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
				Spec: ProjectSpec{
					Roles: []ProjectRole{
						{
							Name:     "testrole",
							Policies: tt.policies,
						},
					},
				},
			}

			err := pe.LoadProjectPolicies(context.Background(), project)
			require.Error(t, err, "Expected error for invalid policy format")
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// TestPolicyEnforcer_DenyPolicies tests deny effect
func TestPolicyEnforcer_DenyPolicies(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "restricted-project"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name: "limited-admin",
					Policies: []string{
						"projects/restricted-project, *, allow",
						"projects/restricted-project, delete, deny", // Explicitly deny delete
					},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	err = pe.AssignUserRoles(context.Background(), "user:limited", []string{"proj:restricted-project:limited-admin"})
	require.NoError(t, err)

	// Should have general access
	allowed, err := pe.CanAccess(context.Background(), "user:limited", "projects/restricted-project", "update")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Delete should be denied
	allowed, err = pe.CanAccess(context.Background(), "user:limited", "projects/restricted-project", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "Delete should be explicitly denied")
}

// TestPolicyEnforcer_ArgoCD_AlignedPolicyFormat tests ArgoCD-style policy format
func TestPolicyEnforcer_ArgoCD_AlignedPolicyFormat(t *testing.T) {
	t.Parallel()

	pe, _ := newTestEnforcerWithMock(t)
	ctx := context.Background()

	// Azure AD group IDs (matching azuread-staging-project.yaml)
	azureAdminGroupID := "7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"
	azureReaderGroupID := "a9563419-38a1-4f85-92f2-5133f99d0ebd"

	// Create project with ArgoCD-aligned policy format
	// Format: "p, proj:{project}:{role}, {resource}, {action}, {object}, {effect}"
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-azuread-staging"},
		Spec: ProjectSpec{
			Description: "Azure EntraID staging project - OIDC group-based access control",
			Roles: []ProjectRole{
				{
					Name:        "admin",
					Description: "Full administrative access (Azure AD group)",
					Policies: []string{
						// Project self-management
						"p, proj:proj-azuread-staging:admin, projects, get, proj-azuread-staging, allow",
						"p, proj:proj-azuread-staging:admin, projects, update, proj-azuread-staging, allow",
						// Instance management
						"p, proj:proj-azuread-staging:admin, instances, *, proj-azuread-staging/*, allow",
						// RGD catalog access
						"p, proj:proj-azuread-staging:admin, rgds, *, *, allow",
						// Settings access
						"p, proj:proj-azuread-staging:admin, settings, get, *, allow",
					},
					Groups: []string{azureAdminGroupID},
				},
				{
					Name:        "reader",
					Description: "Read-only access (Azure AD group)",
					Policies: []string{
						// Project read access
						"p, proj:proj-azuread-staging:reader, projects, get, proj-azuread-staging, allow",
						// Instance read access
						"p, proj:proj-azuread-staging:reader, instances, get, proj-azuread-staging/*, allow",
						"p, proj:proj-azuread-staging:reader, instances, list, proj-azuread-staging/*, allow",
						// RGD catalog read access
						"p, proj:proj-azuread-staging:reader, rgds, get, *, allow",
						"p, proj:proj-azuread-staging:reader, rgds, list, *, allow",
					},
					Groups: []string{azureReaderGroupID},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Test admin role permissions
	t.Run("Admin permissions", func(t *testing.T) {
		t.Parallel()

		// Project get/update
		allowed, err := pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "projects/proj-azuread-staging", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have project get")

		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "projects/proj-azuread-staging", "update")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have project update")

		// Instance CRUD
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "instances/proj-azuread-staging/my-instance", "create")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have instance create")

		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "instances/proj-azuread-staging/my-instance", "delete")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have instance delete")

		// RGD access
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "rgds/any-rgd", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have rgd get")

		// Settings access
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "settings/general", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have settings get")
	})

	// Test reader role permissions
	t.Run("Reader permissions", func(t *testing.T) {
		t.Parallel()

		// Project get (allowed)
		allowed, err := pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "projects/proj-azuread-staging", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Reader should have project get")

		// Project update (denied)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "projects/proj-azuread-staging", "update")
		require.NoError(t, err)
		assert.False(t, allowed, "Reader should NOT have project update")

		// Instance get/list (allowed)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "instances/proj-azuread-staging/my-instance", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Reader should have instance get")

		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "instances/proj-azuread-staging/my-instance", "list")
		require.NoError(t, err)
		assert.True(t, allowed, "Reader should have instance list")

		// Instance create/delete (denied)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "instances/proj-azuread-staging/my-instance", "create")
		require.NoError(t, err)
		assert.False(t, allowed, "Reader should NOT have instance create")

		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "instances/proj-azuread-staging/my-instance", "delete")
		require.NoError(t, err)
		assert.False(t, allowed, "Reader should NOT have instance delete")

		// RGD get/list (allowed)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "rgds/any-rgd", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Reader should have rgd get")

		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "rgds/any-rgd", "list")
		require.NoError(t, err)
		assert.True(t, allowed, "Reader should have rgd list")

		// Settings access (denied - not in reader policy)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "settings/general", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "Reader should NOT have settings access")
	})

	// Test cross-project access denial
	t.Run("Cross-project access denial", func(t *testing.T) {
		t.Parallel()

		// Admin should NOT have access to different project
		allowed, err := pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "projects/other-project", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "Admin should NOT access different project")

		// Admin should NOT have access to instances in different project
		allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "instances/other-project/my-instance", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "Admin should NOT access instances in different project")
	})
}

// TestPolicyEnforcer_BuiltInAdminPolicies tests built-in policies for admin roles
// Admin roles automatically receive repository and member management permissions for their project
func TestPolicyEnforcer_BuiltInAdminPolicies(t *testing.T) {
	t.Parallel()

	pe, _ := newTestEnforcerWithMock(t)
	ctx := context.Background()

	// Create project with an admin role that has NO explicit repository/member policies
	// The built-in policies should grant these automatically
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
		Spec: ProjectSpec{
			Description: "Test project for built-in admin policies",
			Roles: []ProjectRole{
				{
					Name:        "admin", // Built-in policies will be added for roles named "admin"
					Description: "Project admin - should get built-in policies",
					Policies:    []string{}, // Empty - relying on built-in policies
					Groups:      []string{"alpha-admin-group"},
				},
				{
					Name:        "developer", // Non-admin role should NOT get built-in policies
					Description: "Developer role",
					Policies: []string{
						"p, proj:alpha:developer, instances, get, alpha/*, allow",
					},
					Groups: []string{"alpha-dev-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Test built-in repository permissions for admin
	t.Run("Admin has repository permissions", func(t *testing.T) {
		t.Parallel()

		// Repository create
		allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "repositories/alpha/my-repo", "create")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have repository create permission via built-in policy")

		// Repository get
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "repositories/alpha/my-repo", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have repository get permission via built-in policy")

		// Repository update
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "repositories/alpha/my-repo", "update")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have repository update permission via built-in policy")

		// Repository delete
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "repositories/alpha/my-repo", "delete")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have repository delete permission via built-in policy")

		// Repository wildcard
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "repositories/alpha/*", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have wildcard repository access via built-in policy")
	})

	// Test built-in member management permissions for admin
	t.Run("Admin has member management permissions", func(t *testing.T) {
		t.Parallel()

		// Project update
		allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "projects/alpha", "update")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have project update permission via built-in policy")

		// Member add
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "projects/alpha", "member-add")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have member-add permission via built-in policy")

		// Member remove
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "projects/alpha", "member-remove")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have member-remove permission via built-in policy")
	})

	// Test built-in instance permissions for admin
	t.Run("Admin has instance permissions", func(t *testing.T) {
		t.Parallel()

		// Instance create
		allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "instances/alpha/my-instance", "create")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have instance create permission via built-in policy")

		// Instance delete
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "instances/alpha/my-instance", "delete")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have instance delete permission via built-in policy")
	})

	// Test built-in RGD read permissions for admin
	t.Run("Admin has RGD read permissions", func(t *testing.T) {
		t.Parallel()

		// RGD get (global access for catalog browsing)
		allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "rgds/any-rgd", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have rgd get permission via built-in policy")

		// RGD list
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "rgds/any-rgd", "list")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have rgd list permission via built-in policy")
	})

	// Test built-in compliance permissions for admin (project-scoped)
	t.Run("Admin has project-scoped compliance permissions", func(t *testing.T) {
		t.Parallel()

		// Compliance get within own project
		allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "compliance/alpha/any-policy", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have compliance get permission for own project")

		// Compliance list within own project
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "compliance/alpha/any-policy", "list")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have compliance list permission for own project")

		// Compliance get in OTHER project - should be denied
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "compliance/beta/any-policy", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "Admin should NOT have compliance access in other project")
	})

	// Test developer does NOT get admin permissions
	t.Run("Developer does NOT get admin permissions", func(t *testing.T) {
		t.Parallel()

		// Repository access denied
		allowed, err := pe.CanAccessWithGroups(ctx, "user:bob", []string{"alpha-dev-group"}, "repositories/alpha/my-repo", "create")
		require.NoError(t, err)
		assert.False(t, allowed, "Developer should NOT have repository create permission")

		// Member management denied
		allowed, err = pe.CanAccessWithGroups(ctx, "user:bob", []string{"alpha-dev-group"}, "projects/alpha", "member-add")
		require.NoError(t, err)
		assert.False(t, allowed, "Developer should NOT have member-add permission")

		// Project update denied
		allowed, err = pe.CanAccessWithGroups(ctx, "user:bob", []string{"alpha-dev-group"}, "projects/alpha", "update")
		require.NoError(t, err)
		assert.False(t, allowed, "Developer should NOT have project update permission")

		// Instance get allowed (from explicit policy)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:bob", []string{"alpha-dev-group"}, "instances/alpha/my-instance", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Developer should have instance get permission from explicit policy")

		// Instance delete denied (not in explicit policy)
		allowed, err = pe.CanAccessWithGroups(ctx, "user:bob", []string{"alpha-dev-group"}, "instances/alpha/my-instance", "delete")
		require.NoError(t, err)
		assert.False(t, allowed, "Developer should NOT have instance delete permission")
	})

	// Test cross-project access denial for admin
	t.Run("Admin cannot access other projects", func(t *testing.T) {
		t.Parallel()

		// Repository in other project
		allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "repositories/beta/my-repo", "create")
		require.NoError(t, err)
		assert.False(t, allowed, "Admin should NOT have repository access in other project")

		// Member management in other project
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "projects/beta", "member-add")
		require.NoError(t, err)
		assert.False(t, allowed, "Admin should NOT have member-add permission in other project")

		// Instances in other project
		allowed, err = pe.CanAccessWithGroups(ctx, "user:alice", []string{"alpha-admin-group"}, "instances/beta/my-instance", "create")
		require.NoError(t, err)
		assert.False(t, allowed, "Admin should NOT have instance create permission in other project")
	})
}

// TestPolicyEnforcer_BuiltInAdminPolicies_CaseInsensitive tests that admin role name matching is case-insensitive
func TestPolicyEnforcer_BuiltInAdminPolicies_CaseInsensitive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		name     string
		roleName string
	}{
		{"lowercase admin", "admin"},
		{"uppercase ADMIN", "ADMIN"},
		{"mixed case Admin", "Admin"},
		{"mixed case ADMin", "ADMin"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create fresh enforcer for clean state
			pe, _ := newTestEnforcerWithMock(t)

			project := &Project{
				ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
				Spec: ProjectSpec{
					Roles: []ProjectRole{
						{
							Name:     tc.roleName,
							Policies: []string{},
							Groups:   []string{"test-group"},
						},
					},
				},
			}

			err := pe.LoadProjectPolicies(ctx, project)
			require.NoError(t, err)

			// Should have repository permission regardless of case
			allowed, err := pe.CanAccessWithGroups(ctx, "user:test", []string{"test-group"}, "repositories/test-project/my-repo", "create")
			require.NoError(t, err)
			assert.True(t, allowed, "Admin role with name '%s' should have repository permission", tc.roleName)

			// Should have member-add permission regardless of case
			allowed, err = pe.CanAccessWithGroups(ctx, "user:test", []string{"test-group"}, "projects/test-project", "member-add")
			require.NoError(t, err)
			assert.True(t, allowed, "Admin role with name '%s' should have member-add permission", tc.roleName)
		})
	}
}
