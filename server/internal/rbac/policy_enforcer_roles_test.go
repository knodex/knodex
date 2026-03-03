package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPolicyEnforcer_AssignUserRoles tests role assignment
func TestPolicyEnforcer_AssignUserRoles(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	tests := []struct {
		name    string
		user    string
		roles   []string
		wantErr bool
	}{
		{"assign global admin", "user:admin", []string{CasbinRoleServerAdmin}, false},
		{"assign custom role", "user:viewer", []string{"role:test-viewer"}, false},
		{"assign project role", "user:dev", []string{"proj:myproject:developer"}, false},
		{"assign multiple roles", "user:multi", []string{"role:test-viewer", "proj:myproject:developer"}, false},
		{"empty roles", "user:empty", []string{}, false},
		{"empty user", "", []string{CasbinRoleServerAdmin}, true},
		{"invalid role format", "user:bad", []string{"invalid-role"}, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := pe.AssignUserRoles(context.Background(), tt.user, tt.roles)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify roles were assigned
			if len(tt.roles) > 0 {
				roles, err := pe.GetUserRoles(context.Background(), tt.user)
				require.NoError(t, err)
				for _, role := range tt.roles {
					if role == "" {
						continue
					}
					assert.Contains(t, roles, role)
				}
			}
		})
	}
}

// TestPolicyEnforcer_AssignUserRoles_ReplaceExisting tests role replacement
func TestPolicyEnforcer_AssignUserRoles_ReplaceExisting(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Initially assign admin role
	err := pe.AssignUserRoles(ctx, "user:alice", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Verify admin access
	allowed, err := pe.CanAccess(ctx, "user:alice", "projects/test", "delete")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Replace with a project-scoped role (global readonly no longer has policies)
	err = pe.AssignUserRoles(ctx, "user:alice", []string{"proj:test:developer"})
	require.NoError(t, err)

	// Verify admin access is revoked
	allowed, err = pe.CanAccess(ctx, "user:alice", "projects/test", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "Admin access should be revoked after role replacement")

	// Verify serveradmin wildcard is gone
	allowed, err = pe.CanAccess(ctx, "user:alice", "*", "*")
	require.NoError(t, err)
	assert.False(t, allowed, "Serveradmin wildcard should be revoked after role replacement")
}

// TestPolicyEnforcer_AssignUserRolesMultiple tests assigning multiple roles at once
func TestPolicyEnforcer_AssignUserRolesMultiple(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign multiple roles at once
	roles := []string{
		"proj:project1:developer",
		"proj:project2:admin",
		"proj:project3:viewer",
	}

	err := pe.AssignUserRoles(ctx, "user:multi-role", roles)
	require.NoError(t, err)

	// Verify all roles are assigned
	userRoles, err := pe.GetUserRoles(ctx, "user:multi-role")
	require.NoError(t, err)

	for _, role := range roles {
		assert.Contains(t, userRoles, role, "User should have role: %s", role)
	}
}

// TestPolicyEnforcer_GetUserRoles tests retrieving user roles
func TestPolicyEnforcer_GetUserRoles(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign roles
	err := pe.AssignUserRoles(ctx, "user:multi", []string{CasbinRoleServerAdmin, "proj:test:developer"})
	require.NoError(t, err)

	roles, err := pe.GetUserRoles(ctx, "user:multi")
	require.NoError(t, err)
	assert.Contains(t, roles, CasbinRoleServerAdmin)
	assert.Contains(t, roles, "proj:test:developer")

	// Note: g2 (role-to-role) inheritance is used in policy enforcement matchers,
	// not in role queries. GetImplicitRolesForUser only traverses g (user-to-role).
	// However, the user still has viewer PERMISSIONS via enforcement inheritance.

	// Verify that admin inherits viewer permissions (enforcement check)
	allowed, err := pe.CanAccess(ctx, "user:multi", "projects/test", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Admin should have viewer permissions via g2 inheritance")
}

// TestPolicyEnforcer_GetUserRoles_Errors tests error handling
func TestPolicyEnforcer_GetUserRoles_Errors(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	roles, err := pe.GetUserRoles(context.Background(), "")
	assert.Error(t, err)
	assert.Nil(t, roles)
}

// TestPolicyEnforcer_HasRole tests role checking
func TestPolicyEnforcer_HasRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign admin role (which inherits viewer)
	err := pe.AssignUserRoles(ctx, "user:admin", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	tests := []struct {
		name     string
		user     string
		role     string
		expected bool
	}{
		{"has assigned role", "user:admin", CasbinRoleServerAdmin, true},
		// User should not have roles they were not explicitly assigned.
		{"unassigned role not in role query", "user:admin", "role:test-viewer", false},
		{"does not have role", "user:admin", "proj:test:developer", false},
		{"unknown user", "user:unknown", CasbinRoleServerAdmin, false},
		{"empty user", "", CasbinRoleServerAdmin, false},
		{"empty role", "user:admin", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hasRole, err := pe.HasRole(ctx, tt.user, tt.role)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, hasRole)
		})
	}

	// Verify that despite g2-inherited roles not showing in HasRole,
	// the user still has the PERMISSIONS from the inherited role
	t.Run("g2 permission inheritance works via enforcement", func(t *testing.T) {
		t.Parallel()

		// Admin should have viewer permissions via g2 inheritance
		allowed, err := pe.CanAccess(ctx, "user:admin", "projects/test", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have viewer 'get' permission via g2")

		allowed, err = pe.CanAccess(ctx, "user:admin", "projects/test", "list")
		require.NoError(t, err)
		assert.True(t, allowed, "Admin should have viewer 'list' permission via g2")
	})
}

// TestPolicyEnforcer_HasRoleWithGroups tests HasRole with group membership
func TestPolicyEnforcer_HasRoleWithGroups(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcer(enforcer, nil)
	ctx := context.Background()

	// Assign role via group (simulate OIDC group mapping)
	// First, add user to a group
	_, err = enforcer.AddUserRole("user:group-user", "group:admin-group")
	require.NoError(t, err)

	// Add group to role
	_, err = enforcer.AddUserRole("group:admin-group", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// User should have the role via group inheritance
	hasRole, err := pe.HasRole(ctx, "user:group-user", CasbinRoleServerAdmin)
	require.NoError(t, err)
	assert.True(t, hasRole, "User should have role via group membership")

	// User should not have an unassigned role
	hasViewer, err := pe.HasRole(ctx, "user:group-user", "role:test-viewer")
	require.NoError(t, err)
	assert.False(t, hasViewer, "User should not have an unassigned role")

}

// TestPolicyEnforcer_RemoveUserRoles tests role removal
func TestPolicyEnforcer_RemoveUserRoles(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign roles
	err := pe.AssignUserRoles(ctx, "user:alice", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Verify access
	allowed, err := pe.CanAccess(ctx, "user:alice", "projects/test", "delete")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Remove roles
	err = pe.RemoveUserRoles(ctx, "user:alice")
	require.NoError(t, err)

	// Verify access revoked
	allowed, err = pe.CanAccess(ctx, "user:alice", "projects/test", "delete")
	require.NoError(t, err)
	assert.False(t, allowed)

	// Verify no roles remain
	roles, err := pe.GetUserRoles(ctx, "user:alice")
	require.NoError(t, err)
	assert.Empty(t, roles)
}

// TestPolicyEnforcer_RemoveUserRoles_Errors tests error handling
func TestPolicyEnforcer_RemoveUserRoles_Errors(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	err := pe.RemoveUserRoles(context.Background(), "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user cannot be empty")
}

// TestPolicyEnforcer_RemoveUserRole tests the RemoveUserRole function on PolicyEnforcer
func TestPolicyEnforcer_RemoveUserRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pe := newTestEnforcer(t)

	// First add a user role using AssignUserRoles (plural)
	err := pe.AssignUserRoles(ctx, "user:test-remove-user", []string{"proj:test-project:developer"})
	require.NoError(t, err, "failed to assign user role")

	// Verify role was assigned
	roles, err := pe.GetUserRoles(ctx, "user:test-remove-user")
	require.NoError(t, err)
	assert.Contains(t, roles, "proj:test-project:developer")

	// Test removing the role
	err = pe.RemoveUserRole(ctx, "user:test-remove-user", "proj:test-project:developer")
	assert.NoError(t, err, "failed to remove user role")

	// Verify role was removed
	roles, err = pe.GetUserRoles(ctx, "user:test-remove-user")
	require.NoError(t, err)
	assert.NotContains(t, roles, "proj:test-project:developer")
}

// TestPolicyEnforcer_RemoveUserRole_EmptyUser tests RemoveUserRole with empty user
func TestPolicyEnforcer_RemoveUserRole_EmptyUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pe := newTestEnforcer(t)

	err := pe.RemoveUserRole(ctx, "", "proj:test-project:developer")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user cannot be empty")
}

// TestPolicyEnforcer_RemoveUserRole_EmptyRole tests RemoveUserRole with empty role
func TestPolicyEnforcer_RemoveUserRole_EmptyRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pe := newTestEnforcer(t)

	err := pe.RemoveUserRole(ctx, "user:test-user", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role cannot be empty")
}

// TestPolicyEnforcer_RemoveProjectPolicies tests the RemoveProjectPolicies function
func TestPolicyEnforcer_RemoveProjectPolicies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pe := newTestEnforcer(t)

	// Create a test project with correct policy format: object, action, effect
	// NOTE: "applications/*" is scoped to "applications/{project}/*" for security
	// (project admin isolation requirement)
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "remove-project"},
		Spec: ProjectSpec{
			Description: "Test project for removal",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer role",
					Policies: []string{
						"applications/*, get, allow",
					},
				},
			},
		},
	}

	// Load project with policies
	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err, "failed to load project policies")

	// Assign user to project role and verify access is granted
	err = pe.AssignUserRoles(ctx, "user:dev-user", []string{"proj:remove-project:developer"})
	require.NoError(t, err)

	// Verify access is granted before removal
	// SECURITY: "applications/*" is scoped to "applications/remove-project/*"
	// So we must check an application within the project namespace
	allowed, err := pe.CanAccess(ctx, "user:dev-user", "applications/remove-project/my-app", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "user should have access before project policies are removed")

	// Remove project policies
	err = pe.RemoveProjectPolicies(ctx, "remove-project")
	assert.NoError(t, err, "failed to remove project policies")

	// Verify access is denied after removal (policies are gone)
	allowed, err = pe.CanAccess(ctx, "user:dev-user", "applications/remove-project/my-app", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "user should not have access after project policies are removed")
}

// TestPolicyEnforcer_RemoveProjectPolicies_EmptyProjectName tests RemoveProjectPolicies with empty name
func TestPolicyEnforcer_RemoveProjectPolicies_EmptyProjectName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pe := newTestEnforcer(t)

	err := pe.RemoveProjectPolicies(ctx, "")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrProjectNotFound)
}

// TestIsValidRoleFormat tests role format validation
func TestIsValidRoleFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role     string
		expected bool
	}{
		// Valid global roles
		{CasbinRoleServerAdmin, true},
		{"role:readonly", true},
		{"role:custom-role", true},

		// Valid project roles
		{"proj:myproject:developer", true},
		{"proj:my-project:admin", true},
		{"proj:project123:viewer", true},

		// Invalid formats
		{"invalid-role", false},
		{"role:", false},
		{"proj:", false},
		{"proj:project", false}, // Missing role name
		{"proj::developer", false},
		{"", false},
		{"admin", false},
		{"some:other:format:here", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.role, func(t *testing.T) {
			t.Parallel()

			result := isValidRoleFormat(tt.role)
			assert.Equal(t, tt.expected, result, "isValidRoleFormat(%q) = %v, want %v",
				tt.role, result, tt.expected)
		})
	}
}
