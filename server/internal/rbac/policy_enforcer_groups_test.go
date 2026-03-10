// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPolicyEnforcer_GroupMembership tests OIDC group to role mapping
func TestPolicyEnforcer_GroupMembership(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcer(enforcer, nil)

	// Create project with group-based role
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team-project"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "developer",
					Policies: []string{"projects/team-project, get, allow"},
					Groups:   []string{"engineering-team"},
				},
			},
		},
	}

	err = pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Assign group membership (via Casbin grouping policy)
	_, err = enforcer.AddUserRole("user:groupmember", "group:engineering-team")
	require.NoError(t, err)

	// User in the group should have access via project role
	// Note: This requires the group -> role -> policy chain to work
	// The LoadProjectPolicies adds: group:engineering-team -> proj:team-project:developer
	allowed, err := pe.CanAccess(context.Background(), "user:groupmember", "projects/team-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "User in group should have access via group membership")
}

// TestPolicyEnforcer_CanAccessWithGroups_UserPermission tests that direct user permission is checked first
func TestPolicyEnforcer_CanAccessWithGroups_UserPermission(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign global admin role to user
	err := pe.AssignUserRoles(ctx, "user:admin", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// User with direct permission should have access regardless of groups

	allowed, err := pe.CanAccessWithGroups(ctx, "user:admin", []string{}, "projects/test", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "User with direct permission should have access")

	// User with direct permission and empty groups should still have access
	allowed, err = pe.CanAccessWithGroups(ctx, "user:admin", nil, "projects/test", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "User with direct permission and nil groups should have access")
}

// TestPolicyEnforcer_CanAccessWithGroups_GroupPermission tests OIDC group-based access
func TestPolicyEnforcer_CanAccessWithGroups_GroupPermission(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create a project with group-based role mapping
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team-alpha"},
		Spec: ProjectSpec{
			Description: "Team Alpha project",
			Roles: []ProjectRole{
				{
					Name:        "developer",
					Description: "Developer role for team alpha",
					Policies: []string{
						"projects/team-alpha, get, allow",
						"projects/team-alpha, update, allow",
						"instances/team-alpha-*, create, allow",
						"rgds/*, get, allow",
					},
					Groups: []string{"alpha-developers"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// User without direct permission but with matching OIDC group should have access
	// The group "alpha-developers" is mapped to role "proj:team-alpha:developer"

	allowed, err := pe.CanAccessWithGroups(ctx, "user:new-employee", []string{"alpha-developers"}, "projects/team-alpha", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "User with matching OIDC group should have access")

	// Same user should be able to create instances
	allowed, err = pe.CanAccessWithGroups(ctx, "user:new-employee", []string{"alpha-developers"}, "instances/team-alpha-app1", "create")
	require.NoError(t, err)
	assert.True(t, allowed, "User with matching OIDC group should be able to create instances")

	// User should not have delete permission (not in policy)
	allowed, err = pe.CanAccessWithGroups(ctx, "user:new-employee", []string{"alpha-developers"}, "projects/team-alpha", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User should not have delete permission")
}

// TestPolicyEnforcer_CanAccessWithGroups_NoMatchingGroup tests access denied when no group matches
func TestPolicyEnforcer_CanAccessWithGroups_NoMatchingGroup(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create a project with group-based role
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "team-beta"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "admin",
					Policies: []string{"projects/team-beta, *, allow"},
					Groups:   []string{"beta-admins"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// User with unrelated groups should not have access

	allowed, err := pe.CanAccessWithGroups(ctx, "user:outsider", []string{"gamma-team", "delta-team"}, "projects/team-beta", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "User with non-matching groups should not have access")
}

// TestPolicyEnforcer_CanAccessWithGroups_MultipleGroups tests access with multiple OIDC groups
func TestPolicyEnforcer_CanAccessWithGroups_MultipleGroups(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create two projects with different group mappings
	projectA := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "project-a"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "viewer",
					Policies: []string{"projects/project-a, get, allow"},
					Groups:   []string{"team-a-viewers"},
				},
			},
		},
	}
	projectB := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "project-b"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "admin",
					Policies: []string{"projects/project-b, *, allow"},
					Groups:   []string{"team-b-admins"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, projectA)
	require.NoError(t, err)
	err = pe.LoadProjectPolicies(ctx, projectB)
	require.NoError(t, err)

	// User with multiple groups - should have access to both projects
	groups := []string{"team-a-viewers", "team-b-admins", "other-group"}

	// Can view project A

	allowed, err := pe.CanAccessWithGroups(ctx, "user:multi-team", groups, "projects/project-a", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "User with team-a-viewers group should access project-a")

	// Can admin project B
	allowed, err = pe.CanAccessWithGroups(ctx, "user:multi-team", groups, "projects/project-b", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "User with team-b-admins group should admin project-b")

	// Cannot admin project A (only viewer)
	allowed, err = pe.CanAccessWithGroups(ctx, "user:multi-team", groups, "projects/project-a", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User with only viewer group should not delete project-a")
}

// TestPolicyEnforcer_CanAccessWithGroups_UserOverrideGroup tests direct user permission overrides lack of group
func TestPolicyEnforcer_CanAccessWithGroups_UserOverrideGroup(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create project with group-based access
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-project"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "developer",
					Policies: []string{"projects/shared-project, *, allow"},
					Groups:   []string{"dev-team"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Assign user directly to the role (without being in the group)
	err = pe.AssignUserRoles(ctx, "user:direct-assign", []string{"proj:shared-project:developer"})
	require.NoError(t, err)

	// User should have access via direct role assignment, even with different groups

	allowed, err := pe.CanAccessWithGroups(ctx, "user:direct-assign", []string{"unrelated-group"}, "projects/shared-project", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "Direct role assignment should grant access regardless of groups")
}

// TestPolicyEnforcer_CanAccessWithGroups_EmptyGroups tests access with empty groups list
func TestPolicyEnforcer_CanAccessWithGroups_EmptyGroups(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// User without any roles or groups should not have access

	allowed, err := pe.CanAccessWithGroups(ctx, "user:nobody", []string{}, "projects/test", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "User with no roles or groups should not have access")

	// With nil groups
	allowed, err = pe.CanAccessWithGroups(ctx, "user:nobody", nil, "projects/test", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "User with no roles and nil groups should not have access")
}

// TestPolicyEnforcer_CanAccessWithGroups_InvalidInputs tests error handling
func TestPolicyEnforcer_CanAccessWithGroups_InvalidInputs(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		user    string
		groups  []string
		object  string
		action  string
		wantErr bool
	}{
		{"empty user", "", []string{"group1"}, "projects/test", "get", true},
		{"empty object", "user:test", []string{}, "", "get", true},
		{"empty action", "user:test", []string{}, "projects/test", "", true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowed, err := pe.CanAccessWithGroups(ctx, tt.user, tt.groups, tt.object, tt.action)
			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, allowed)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPolicyEnforcer_CanAccessWithGroups_InvalidGroupName tests handling of invalid group names
func TestPolicyEnforcer_CanAccessWithGroups_InvalidGroupName(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Create project with valid group
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "test-project"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "viewer",
					Policies: []string{"projects/test-project, get, allow"},
					Groups:   []string{"valid-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Empty strings in groups should be skipped

	allowed, err := pe.CanAccessWithGroups(ctx, "user:test", []string{"", "valid-group", ""}, "projects/test-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Should skip empty groups and find valid-group")

	// Only empty strings should result in no access (if user has no direct permission)
	allowed, err = pe.CanAccessWithGroups(ctx, "user:test", []string{"", ""}, "projects/test-project", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "Only empty groups should not grant access")
}

// TestPolicyEnforcer_CanAccessWithGroups_CacheHitMiss tests caching behavior
func TestPolicyEnforcer_CanAccessWithGroups_CacheHitMiss(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create with cache enabled
	config := PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	ctx := context.Background()

	// Create project with group
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "cached-project"},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "developer",
					Policies: []string{"projects/cached-project, get, allow"},
					Groups:   []string{"cached-group"},
				},
			},
		},
	}

	err = pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// First call - cache miss

	allowed1, err := pe.CanAccessWithGroups(ctx, "user:cache-test", []string{"cached-group"}, "projects/cached-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed1)

	// Second call - should hit cache
	allowed2, err := pe.CanAccessWithGroups(ctx, "user:cache-test", []string{"cached-group"}, "projects/cached-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed2)

	// Results should be consistent
	assert.Equal(t, allowed1, allowed2)
}

// TestPolicyEnforcer_CanAccessWithGroups_AzureADGroups tests Azure AD (EntraID) group IDs
func TestPolicyEnforcer_CanAccessWithGroups_AzureADGroups(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Azure AD groups use UUIDs as group IDs
	azureAdminGroupID := "7e24cb11-e404-4b4d-9e2c-96d6e7b4733c"
	azureReaderGroupID := "a9563419-38a1-4f85-92f2-5133f99d0ebd"

	// Create project mimicking the azuread-staging-project.yaml example
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-azuread-staging"},
		Spec: ProjectSpec{
			Description: "Azure EntraID staging project - OIDC group-based access control",
			Roles: []ProjectRole{
				{
					Name:        "admin",
					Description: "Full administrative access (Azure AD group)",
					Policies: []string{
						"projects/proj-azuread-staging, *, allow",
						"instances/proj-azuread-staging-*, *, allow",
						"rgds/*, *, allow",
					},
					Groups: []string{azureAdminGroupID},
				},
				{
					Name:        "reader",
					Description: "Read-only access (Azure AD group)",
					Policies: []string{
						"projects/proj-azuread-staging, get, allow",
						"instances/proj-azuread-staging-*, get, allow",
						"rgds/*, get, allow",
					},
					Groups: []string{azureReaderGroupID},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// User with Azure AD admin group should have full access

	allowed, err := pe.CanAccessWithGroups(ctx, "user:azure-admin", []string{azureAdminGroupID}, "projects/proj-azuread-staging", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "User in Azure AD admin group should have delete permission")

	// User with Azure AD reader group should only have read access
	allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "projects/proj-azuread-staging", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "User in Azure AD reader group should have get permission")

	// Reader should not have delete permission
	allowed, err = pe.CanAccessWithGroups(ctx, "user:azure-reader", []string{azureReaderGroupID}, "projects/proj-azuread-staging", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User in Azure AD reader group should not have delete permission")
}

// TestPolicyEnforcer_CanAccessWithGroups_GlobalAdmin tests global admin bypass
func TestPolicyEnforcer_CanAccessWithGroups_GlobalAdmin(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign global admin role
	err := pe.AssignUserRoles(ctx, "user:super-admin", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Global admin should have access to anything, regardless of groups

	allowed, err := pe.CanAccessWithGroups(ctx, "user:super-admin", []string{}, "projects/any-project", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "Global admin should have access without groups")

	// Even with unrelated groups
	allowed, err = pe.CanAccessWithGroups(ctx, "user:super-admin", []string{"random-group"}, "instances/any-instance", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "Global admin should have access with unrelated groups")
}

// TestPolicyEnforcer_CanAccessWithGroups_RoleRevocationImmediate is a SECURITY regression test.
// It verifies that when a user's role is revoked in Casbin, the revocation takes effect
// immediately on the next API request. This prevents privilege escalation via stale JWT claims.
// (STORY-228: C1 finding - JWT casbin_roles trusted for server-side authorization)
func TestPolicyEnforcer_CanAccessWithGroups_RoleRevocationImmediate(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Disable caching to isolate the server-side role lookup behavior
	pe := NewPolicyEnforcerWithConfig(enforcer, nil, PolicyEnforcerConfig{CacheEnabled: false})
	ctx := context.Background()

	// Step 1: Assign admin role to user
	err = pe.AssignUserRoles(ctx, "user:revoke-test", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Step 2: Verify user has access via server-side role
	allowed, err := pe.CanAccessWithGroups(ctx, "user:revoke-test", nil, "projects/any-project", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "Admin user should have access before revocation")

	// Step 3: Revoke the role in Casbin (simulates admin removing the role)
	err = pe.RemoveUserRoles(ctx, "user:revoke-test")
	require.NoError(t, err)

	// Step 4: CRITICAL SECURITY CHECK - User should be DENIED immediately
	// Before STORY-228 fix, if JWT roles were trusted, the old JWT containing
	// "role:serveradmin" would still grant access even after revocation.
	// After the fix, roles are sourced from Casbin's authoritative state,
	// so revocation takes effect immediately.
	allowed, err = pe.CanAccessWithGroups(ctx, "user:revoke-test", nil, "projects/any-project", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "SECURITY: Revoked admin should be denied immediately - roles must come from server state, not JWT")

	// Also verify the user cannot perform any privileged actions
	allowed, err = pe.CanAccessWithGroups(ctx, "user:revoke-test", nil, "settings/*", "update")
	require.NoError(t, err)
	assert.False(t, allowed, "SECURITY: Revoked admin should not access settings")
}

// TestPolicyEnforcer_CanAccessWithGroups_ServerSideRolesUsed verifies that
// CanAccessWithGroups uses server-side Casbin roles (GetImplicitRolesForUser),
// not externally supplied roles. This is the core STORY-228 security fix.
func TestPolicyEnforcer_CanAccessWithGroups_ServerSideRolesUsed(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, PolicyEnforcerConfig{CacheEnabled: false})
	ctx := context.Background()

	// User has NO roles assigned in Casbin
	// The method should deny access because server-side role lookup returns empty

	// Even though role:admin policy exists (built-in), the user is not assigned to it
	allowed, err := pe.CanAccessWithGroups(ctx, "user:no-roles", nil, "projects/test", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User with no server-side roles should be denied")

	// Now assign the role and verify access is granted
	err = pe.AssignUserRoles(ctx, "user:no-roles", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	allowed, err = pe.CanAccessWithGroups(ctx, "user:no-roles", nil, "projects/test", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "User with server-side admin role should have access")
}
