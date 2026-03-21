// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

func TestNewCasbinEnforcer(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err, "Failed to create enforcer")
	require.NotNil(t, enforcer, "Enforcer is nil")
	require.NotNil(t, enforcer.enforcer, "Underlying enforcer is nil")
}

func TestNewCasbinEnforcer_InvalidModel(t *testing.T) {
	// Test with invalid model string
	invalidModel := `
[request_definition]
r = invalid

[policy_definition]
p = invalid
`
	enforcer, err := NewCasbinEnforcerFromString(invalidModel)
	// The model might still parse but policies might fail
	// This tests that error handling works
	if err != nil {
		assert.Contains(t, err.Error(), "failed", "Error should indicate failure")
		assert.Nil(t, enforcer)
	}
}

func TestBuiltinPolicies_GlobalAdmin(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign global-admin role to user
	added, err := enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)
	assert.True(t, added, "Should add user role")

	tests := []struct {
		name     string
		sub      string
		obj      string
		act      string
		expected bool
	}{
		// Global admin can access everything
		{"admin create project", "user:alice", "projects/test-project", "create", true},
		{"admin get project", "user:alice", "projects/test-project", "get", true},
		{"admin delete project", "user:alice", "projects/test-project", "delete", true},
		{"admin update project", "user:alice", "projects/test-project", "update", true},
		{"admin get rgd", "user:alice", "rgds/test-rgd", "get", true},
		{"admin create instance", "user:alice", "instances/test-instance", "create", true},
		{"admin delete user", "user:alice", "users/some-user", "delete", true},
		{"admin list applications", "user:alice", "applications/my-app", "list", true},

		// Non-admin user has no access
		{"non-admin get project", "user:bob", "projects/test-project", "get", false},
		{"non-admin create instance", "user:bob", "instances/test", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce(tt.sub, tt.obj, tt.act)
			require.NoError(t, err, "Enforce error for %s", tt.name)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				tt.sub, tt.obj, tt.act, allowed, tt.expected)
		})
	}
}

func TestBuiltinPolicies_GlobalReadonlyRemoved(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign deprecated global readonly role - should have NO built-in policies
	added, err := enforcer.AddUserRole("user:bob", "role:readonly")
	require.NoError(t, err)
	assert.True(t, added)

	tests := []struct {
		name     string
		sub      string
		obj      string
		act      string
		expected bool
	}{
		// Global readonly has NO policies - all access denied
		{"readonly get project denied", "user:bob", "projects/test-project", "get", false},
		{"readonly list projects denied", "user:bob", "projects/any", "list", false},
		{"readonly get rgd denied", "user:bob", "rgds/test-rgd", "get", false},
		{"readonly list rgds denied", "user:bob", "rgds/any", "list", false},
		{"readonly get instance denied", "user:bob", "instances/test-instance", "get", false},
		{"readonly list applications denied", "user:bob", "applications/app", "list", false},
		{"readonly create project denied", "user:bob", "projects/test-project", "create", false},
		{"readonly update project denied", "user:bob", "projects/test-project", "update", false},
		{"readonly delete project denied", "user:bob", "projects/test-project", "delete", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce(tt.sub, tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				tt.sub, tt.obj, tt.act, allowed, tt.expected)
		})
	}
}

func TestServerAdminFullAccess(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign serveradmin role
	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Serveradmin should have read permissions
	allowed, err := enforcer.Enforce("user:alice", "projects/test", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Server admin should have get permissions")

	// Serveradmin should have write permissions
	allowed, err = enforcer.Enforce("user:alice", "projects/test", "delete")
	require.NoError(t, err)
	assert.True(t, allowed, "Server admin should have delete permissions")

	// Serveradmin should have settings access
	allowed, err = enforcer.Enforce("user:alice", "settings/sso", "update")
	require.NoError(t, err)
	assert.True(t, allowed, "Server admin should have settings access")
}

func TestGlobPatternMatching(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add policy with glob pattern for a project admin
	added, err := enforcer.AddPolicy("role:project-admin", "projects/engineering-*", "*", "allow")
	require.NoError(t, err)
	assert.True(t, added)

	// Assign role to user
	_, err = enforcer.AddUserRole("user:alice", "role:project-admin")
	require.NoError(t, err)

	tests := []struct {
		name     string
		obj      string
		expected bool
	}{
		{"engineering-team1", "projects/engineering-team1", true},
		{"engineering-team2", "projects/engineering-team2", true},
		{"engineering-prod", "projects/engineering-prod", true},
		{"engineering empty suffix", "projects/engineering-", true},
		{"marketing-team", "projects/marketing-team", false},
		{"other project", "projects/other", false},
		{"engineering without dash", "projects/engineeringteam", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:alice", tt.obj, "get")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed, "Enforce(user:alice, %s, get) = %v, want %v",
				tt.obj, allowed, tt.expected)
		})
	}
}

func TestGlobPatternMatching_Actions(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add policy allowing all read actions
	added, err := enforcer.AddPolicy("role:reader", "projects/*", "get*", "allow")
	require.NoError(t, err)
	assert.True(t, added)

	_, err = enforcer.AddUserRole("user:reader", "role:reader")
	require.NoError(t, err)

	tests := []struct {
		name     string
		action   string
		expected bool
	}{
		{"get action", "get", true},
		{"getAll action", "getAll", true},
		{"getDetails action", "getDetails", true},
		{"list action", "list", false},
		{"create action", "create", false},
		{"delete action", "delete", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:reader", "projects/any", tt.action)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestGetRolesForUser(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add multiple roles
	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)
	_, err = enforcer.AddUserRole("user:alice", "role:project-developer")
	require.NoError(t, err)

	roles, err := enforcer.GetRolesForUser("user:alice")
	require.NoError(t, err)
	assert.Len(t, roles, 2, "Expected 2 roles")

	// Check specific roles
	assert.Contains(t, roles, CasbinRoleServerAdmin)
	assert.Contains(t, roles, "role:project-developer")
}

func TestGetUsersForRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add multiple users to a role
	_, err = enforcer.AddUserRole("user:alice", "role:test-viewer")
	require.NoError(t, err)
	_, err = enforcer.AddUserRole("user:bob", "role:test-viewer")
	require.NoError(t, err)

	users, err := enforcer.GetUsersForRole("role:test-viewer")
	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Contains(t, users, "user:alice")
	assert.Contains(t, users, "user:bob")
}

func TestRemoveUserRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add and then remove role
	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Verify role is assigned
	roles, err := enforcer.GetRolesForUser("user:alice")
	require.NoError(t, err)
	assert.Contains(t, roles, CasbinRoleServerAdmin)

	// Remove role
	removed, err := enforcer.RemoveUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)
	assert.True(t, removed)

	// Verify role is removed
	roles, err = enforcer.GetRolesForUser("user:alice")
	require.NoError(t, err)
	assert.NotContains(t, roles, CasbinRoleServerAdmin)

	// Verify permissions are revoked
	allowed, err := enforcer.Enforce("user:alice", "projects/test", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "User should no longer have admin permissions")
}

func TestHasPolicy(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Check built-in policy exists
	exists, err := enforcer.HasPolicy(CasbinRoleServerAdmin, "projects/*", "*", "allow")
	require.NoError(t, err)
	assert.True(t, exists, "Built-in global-admin policy should exist")

	// Check non-existent policy
	exists, err = enforcer.HasPolicy("role:fake", "fake/*", "*", "allow")
	require.NoError(t, err)
	assert.False(t, exists, "Non-existent policy should not exist")
}

func TestHasUserRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)

	hasRole, err := enforcer.HasUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)
	assert.True(t, hasRole)

	hasRole, err = enforcer.HasUserRole("user:alice", "role:test-viewer")
	require.NoError(t, err)
	assert.False(t, hasRole, "User should not have an unassigned role")

	hasRole, err = enforcer.HasUserRole("user:bob", CasbinRoleServerAdmin)
	require.NoError(t, err)
	assert.False(t, hasRole, "Bob should not have admin role")
}

func TestAddRemovePolicy(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add custom policy
	added, err := enforcer.AddPolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)
	assert.True(t, added)

	// Verify policy exists
	exists, err := enforcer.HasPolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)
	assert.True(t, exists)

	// Remove policy
	removed, err := enforcer.RemovePolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)
	assert.True(t, removed)

	// Verify policy removed
	exists, err = enforcer.HasPolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestClearPolicies(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add custom policy
	_, err = enforcer.AddPolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)

	// Clear all policies
	err = enforcer.ClearPolicies()
	require.NoError(t, err)

	// Custom policy should be gone
	exists, err := enforcer.HasPolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)
	assert.False(t, exists, "Custom policy should be cleared")

	// Built-in policies should be reloaded
	exists, err = enforcer.HasPolicy(CasbinRoleServerAdmin, "projects/*", "*", "allow")
	require.NoError(t, err)
	assert.True(t, exists, "Built-in policies should be reloaded")
}

func TestGetAllPolicies(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	policies, err := enforcer.GetAllPolicies()
	require.NoError(t, err)
	assert.NotEmpty(t, policies, "Should have built-in policies")

	// 1-global-role model: role:serveradmin has 11 built-in policies
	// (wildcard + explicit for projects, rgds, instances, users, applications,
	// repositories, namespaces, settings, compliance, secrets)
	assert.GreaterOrEqual(t, len(policies), 11, "Should have at least 11 built-in policies")
}

func TestGetPoliciesForRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Get global-admin policies
	adminPolicies, err := enforcer.GetPoliciesForRole(CasbinRoleServerAdmin)
	require.NoError(t, err)
	assert.NotEmpty(t, adminPolicies, "Global admin should have policies")
	assert.GreaterOrEqual(t, len(adminPolicies), 5, "Global admin should have at least 5 policies")

	// Global readonly no longer has built-in policies (deprecated)
	viewerPolicies, err := enforcer.GetPoliciesForRole("role:readonly")
	require.NoError(t, err)
	assert.Empty(t, viewerPolicies, "Deprecated global readonly should have no built-in policies")
}

func TestGetAllRoles(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add a user to create implicit roles
	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)

	roles, err := enforcer.GetAllRoles()
	require.NoError(t, err)
	assert.Contains(t, roles, CasbinRoleServerAdmin)
}

func TestDenyPolicy(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign global admin
	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Add deny policy for specific project
	_, err = enforcer.AddPolicy(CasbinRoleServerAdmin, "projects/secret-project", "*", "deny")
	require.NoError(t, err)

	// Alice should be denied access to secret project
	allowed, err := enforcer.Enforce("user:alice", "projects/secret-project", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "Deny policy should block access")

	// Alice should still have access to other projects
	allowed, err = enforcer.Enforce("user:alice", "projects/normal-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Other projects should still be accessible")
}

func TestProjectRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create project-specific role
	projectRole, err := FormatProjectRole("engineering", "developer")
	require.NoError(t, err)

	// Add policies for this project role
	_, err = enforcer.AddPolicy(projectRole, "projects/engineering", "*", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "instances/engineering/*", "*", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "rgds/*", "get", "allow")
	require.NoError(t, err)

	// Assign role to user
	_, err = enforcer.AddUserRole("user:dev1", projectRole)
	require.NoError(t, err)

	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		{"access own project", "projects/engineering", "get", true},
		{"create in own project", "projects/engineering", "create", true},
		{"access own instances", "instances/engineering/app1", "get", true},
		{"create own instance", "instances/engineering/app2", "create", true},
		{"read any rgd", "rgds/some-rgd", "get", true},
		{"no access other project", "projects/marketing", "get", false},
		{"no access other instances", "instances/marketing/app", "get", false},
		{"no create rgd", "rgds/new-rgd", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:dev1", tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	// Test FormatObject - now returns (string, error)
	obj, err := FormatObject(ResourceProjects, "my-project")
	require.NoError(t, err)
	assert.Equal(t, "projects/my-project", obj)

	obj, err = FormatObject(ResourceProjects, "")
	require.NoError(t, err)
	assert.Equal(t, "projects/*", obj)

	obj, err = FormatObject(ResourceProjects, "*")
	require.NoError(t, err)
	assert.Equal(t, "projects/*", obj)

	obj, err = FormatObject(ResourceRGDs, "my-rgd")
	require.NoError(t, err)
	assert.Equal(t, "rgds/my-rgd", obj)

	obj, err = FormatObject(ResourceInstances, "my-instance")
	require.NoError(t, err)
	assert.Equal(t, "instances/my-instance", obj)

	// Test FormatObjectUnsafe - for trusted input, no error
	assert.Equal(t, "projects/my-project", FormatObjectUnsafe(ResourceProjects, "my-project"))
	assert.Equal(t, "projects/*", FormatObjectUnsafe(ResourceProjects, ""))
	assert.Equal(t, "projects/*", FormatObjectUnsafe(ResourceProjects, "*"))

	// Test FormatProjectRole - now returns (string, error)
	role, err := FormatProjectRole("engineering", "developer")
	require.NoError(t, err)
	assert.Equal(t, "proj:engineering:developer", role)

	role, err = FormatProjectRole("marketing", "viewer")
	require.NoError(t, err)
	assert.Equal(t, "proj:marketing:viewer", role)

	// Test FormatUserSubject - now returns (string, error)
	sub, err := FormatUserSubject("alice")
	require.NoError(t, err)
	assert.Equal(t, "user:alice", sub)

	sub, err = FormatUserSubject("user-123")
	require.NoError(t, err)
	assert.Equal(t, "user:user-123", sub)

	// Test FormatGroupSubject - now returns (string, error)
	grp, err := FormatGroupSubject("engineering")
	require.NoError(t, err)
	assert.Equal(t, "group:engineering", grp)

	grp, err = FormatGroupSubject("platform-admins")
	require.NoError(t, err)
	assert.Equal(t, "group:platform-admins", grp)
}

func TestFormatHelpers_ValidationErrors(t *testing.T) {
	// Test FormatObject with resource name exceeding max length
	longName := string(make([]byte, MaxResourceNameLength+1))
	_, err := FormatObject(ResourceProjects, longName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")

	// Test FormatObject sanitizes glob characters
	obj, err := FormatObject(ResourceProjects, "my*project")
	require.NoError(t, err)
	assert.Equal(t, "projects/my\\*project", obj, "Should escape glob characters")

	obj, err = FormatObject(ResourceProjects, "my?project")
	require.NoError(t, err)
	assert.Equal(t, "projects/my\\?project", obj, "Should escape glob characters")

	// Test FormatProjectRole with invalid project name
	_, err = FormatProjectRole("project:name", "developer")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project name")

	// Test FormatProjectRole with invalid role name
	_, err = FormatProjectRole("engineering", "role:admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role name")

	// Test FormatProjectRole with names exceeding max length
	longProjectName := string(make([]byte, MaxProjectNameLength+1))
	_, err = FormatProjectRole(longProjectName, "developer")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name exceeds")

	longRoleName := string(make([]byte, MaxRoleNameLength+1))
	_, err = FormatProjectRole("engineering", longRoleName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "role name exceeds")

	// Test FormatUserSubject with empty ID
	_, err = FormatUserSubject("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Test FormatUserSubject with invalid characters
	_, err = FormatUserSubject("user:alice")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")

	// Test FormatUserSubject with reserved prefix (colon caught as invalid character)
	_, err = FormatUserSubject("role:admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")

	// Test FormatGroupSubject with empty name
	_, err = FormatGroupSubject("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")

	// Test FormatGroupSubject with spaces
	_, err = FormatGroupSubject("group name")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestValidateSubjectIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
		errMsg  string
	}{
		{"valid simple", "alice", false, ""},
		{"valid with hyphen", "alice-bob", false, ""},
		{"valid with number", "user123", false, ""},
		{"valid complex", "alice.bob-123", false, ""},
		{"empty", "", true, "cannot be empty"},
		{"too long", string(make([]byte, MaxSubjectIDLength+1)), true, "exceeds maximum length"},
		{"contains colon", "user:alice", true, "invalid characters"},
		{"contains space", "user alice", true, "invalid characters"},
		{"contains newline", "user\nalice", true, "invalid characters"},
		{"contains tab", "user\talice", true, "invalid characters"},
		{"starts with role:", "role:admin", true, "invalid characters"}, // colon caught first
		{"starts with user:", "user:alice", true, "invalid characters"}, // colon caught first
		{"starts with group:", "group:admins", true, "invalid characters"},
		{"starts with proj:", "proj:myproject", true, "invalid characters"},
		{"case insensitive prefix", "ROLE:admin", true, "invalid characters"}, // colon caught first
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubjectIdentifier(tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGlobPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
		errMsg  string
	}{
		{"empty pattern", "", false, ""},
		{"simple pattern", "projects/*", false, ""},
		{"pattern with wildcards", "proj*ects/*", false, ""},
		{"pattern at max length", string(make([]byte, MaxPatternLength)), false, ""},
		{"pattern too long", string(make([]byte, MaxPatternLength+1)), true, "pattern too long"},
		{"too many wildcards", "a*b*c*d*e*f*g*h*i*j*k*l*", true, "too many wildcards"},
		{"nested wildcards **", "projects/**", true, "nested or consecutive wildcards"},
		{"consecutive wildcards ??", "projects/??name", true, "nested or consecutive wildcards"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGlobPattern(tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEffect(t *testing.T) {
	// Valid effects
	assert.NoError(t, ValidateEffect(EffectAllow))
	assert.NoError(t, ValidateEffect(EffectDeny))

	// Invalid effects
	err := ValidateEffect("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid effect")

	err = ValidateEffect("permit")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid effect")

	err = ValidateEffect("ALLOW")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid effect")
}

func TestConstants(t *testing.T) {
	// Verify role constants (server-level role model)
	assert.Equal(t, "role:serveradmin", CasbinRoleServerAdmin)

	// Verify resource constants
	assert.Equal(t, "projects", ResourceProjects)
	assert.Equal(t, "rgds", ResourceRGDs)
	assert.Equal(t, "instances", ResourceInstances)
	assert.Equal(t, "users", ResourceUsers)
	assert.Equal(t, "applications", ResourceApplications)
	assert.Equal(t, "repositories", ResourceRepositories)
	assert.Equal(t, "settings", ResourceSettings)
	assert.Equal(t, "compliance", ResourceCompliance)
	assert.Equal(t, "secrets", ResourceSecrets)

	// Verify action constants
	assert.Equal(t, "get", ActionGet)
	assert.Equal(t, "list", ActionList)
	assert.Equal(t, "create", ActionCreate)
	assert.Equal(t, "update", ActionUpdate)
	assert.Equal(t, "delete", ActionDelete)

	// Verify effect constants
	assert.Equal(t, "allow", EffectAllow)
	assert.Equal(t, "deny", EffectDeny)
}

// ArgoCD-aligned 2-role model: Tests for project-scoped roles defined in Project CRD
// Custom roles (admin, developer, readonly) are now defined in Project CRD, not as built-in roles

func TestProjectScopedRoles_CustomRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create a custom project-scoped admin role (as would be defined in Project CRD)
	projectRole, err := FormatProjectRole("engineering", "admin")
	require.NoError(t, err)

	// Add policies for this project role (simulating Project CRD policy loading)
	_, err = enforcer.AddPolicy(projectRole, "projects/engineering", "*", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "instances/engineering/*", "*", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "rgds/*", "get", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "rgds/*", "list", "allow")
	require.NoError(t, err)

	// Assign role to user
	added, err := enforcer.AddUserRole("user:alice", projectRole)
	require.NoError(t, err)
	assert.True(t, added, "Should add user role")

	tests := []struct {
		name     string
		sub      string
		obj      string
		act      string
		expected bool
	}{
		// Project permissions - full access to own project
		{"project-admin get project", "user:alice", "projects/engineering", "get", true},
		{"project-admin update project", "user:alice", "projects/engineering", "update", true},
		{"project-admin cannot access other project", "user:alice", "projects/marketing", "get", false},

		// Instance permissions - full access in project namespace
		{"project-admin list instances", "user:alice", "instances/engineering/app1", "list", true},
		{"project-admin create instance", "user:alice", "instances/engineering/app2", "create", true},
		{"project-admin delete instance", "user:alice", "instances/engineering/app1", "delete", true},
		{"project-admin cannot access other namespace", "user:alice", "instances/marketing/app", "get", false},

		// RGD permissions - read-only
		{"project-admin list rgds", "user:alice", "rgds/test-rgd", "list", true},
		{"project-admin get rgd", "user:alice", "rgds/test-rgd", "get", true},
		{"project-admin cannot create rgd", "user:alice", "rgds/new-rgd", "create", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce(tt.sub, tt.obj, tt.act)
			require.NoError(t, err, "Enforce error for %s", tt.name)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				tt.sub, tt.obj, tt.act, allowed, tt.expected)
		})
	}
}

func TestBuiltinPolicies_AllRolesCount(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	policies, err := enforcer.GetAllPolicies()
	require.NoError(t, err)
	assert.NotEmpty(t, policies, "Should have built-in policies")

	// 1-global-role model: only role:serveradmin has built-in policies (11 total)
	// No global readonly role — readonly is project-scoped only
	assert.GreaterOrEqual(t, len(policies), 11, "Should have at least 11 built-in serveradmin policies")
}

func TestConcurrentAccess(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Set up a user with admin role
	_, err = enforcer.AddUserRole("user:concurrent", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Run concurrent enforce calls
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			allowed, err := enforcer.Enforce("user:concurrent", "projects/test", "get")
			assert.NoError(t, err)
			assert.True(t, allowed)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestMultipleRolesForUser(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create limited role
	_, err = enforcer.AddPolicy("role:limited", "projects/limited-project", "get", "allow")
	require.NoError(t, err)

	// Add a custom reader role with explicit read policies
	_, err = enforcer.AddPolicy("role:reader", "rgds/*", "get", "allow")
	require.NoError(t, err)

	// Assign multiple roles to user
	_, err = enforcer.AddUserRole("user:multi", "role:limited")
	require.NoError(t, err)
	_, err = enforcer.AddUserRole("user:multi", "role:reader")
	require.NoError(t, err)

	// User should have combined permissions
	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		// From limited role
		{"get limited project", "projects/limited-project", "get", true},
		// From reader role
		{"get any rgd", "rgds/any", "get", true},
		// Not allowed
		{"create project", "projects/any", "create", false},
		{"delete instance", "instances/any", "delete", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:multi", tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestNewCasbinEnforcerFromString(t *testing.T) {
	// Valid model string from embedded file should work
	modelText := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft

[role_definition]
g = _, _
g2 = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = (g(r.sub, p.sub) || g2(r.sub, p.sub)) && keyMatch(r.obj, p.obj) && keyMatch(r.act, p.act)
`
	enforcer, err := NewCasbinEnforcerFromString(modelText)
	require.NoError(t, err)
	require.NotNil(t, enforcer)

	// Verify built-in policies are loaded
	exists, err := enforcer.HasPolicy(CasbinRoleServerAdmin, "projects/*", "*", "allow")
	require.NoError(t, err)
	assert.True(t, exists, "Built-in global-admin policy should be loaded")

	// Verify enforcer works correctly
	_, err = enforcer.AddUserRole("user:test", CasbinRoleServerAdmin)
	require.NoError(t, err)

	allowed, err := enforcer.Enforce("user:test", "projects/any", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Admin should have access")
}

func TestNewCasbinEnforcerFromString_InvalidModelSyntax(t *testing.T) {
	// Completely invalid model syntax
	invalidModel := `this is not a valid model`
	enforcer, err := NewCasbinEnforcerFromString(invalidModel)
	assert.Error(t, err)
	assert.Nil(t, enforcer)
	assert.Contains(t, err.Error(), "failed")
}

func TestAddRoleInheritance(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Test that AddRoleInheritance correctly adds g2 (role-to-role) relationships
	// This is the same mechanism used for global-admin inheriting from global-viewer

	// Add role inheritance: custom-super-admin inherits from custom-admin
	added, err := enforcer.AddRoleInheritance("role:custom-super-admin", "role:custom-admin")
	require.NoError(t, err)
	assert.True(t, added, "Should add role inheritance")

	// Verify the inheritance was added by checking if it exists
	// The built-in global-admin inherits from global-viewer via g2 - verify that exists
	policies, err := enforcer.enforcer.GetNamedGroupingPolicy("g2")
	require.NoError(t, err)
	assert.NotEmpty(t, policies, "Should have g2 policies")

	// Verify our custom inheritance exists
	var foundCustomInheritance bool
	for _, policy := range policies {
		if len(policy) >= 2 && policy[0] == "role:custom-super-admin" && policy[1] == "role:custom-admin" {
			foundCustomInheritance = true
			break
		}
	}
	assert.True(t, foundCustomInheritance, "Custom role inheritance should exist in g2 policies")

	// Adding the same inheritance again should return false (already exists)
	added2, err := enforcer.AddRoleInheritance("role:custom-super-admin", "role:custom-admin")
	require.NoError(t, err)
	assert.False(t, added2, "Should return false when inheritance already exists")
}

func TestRemoveRoleInheritance(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Add role inheritance first
	added, err := enforcer.AddRoleInheritance("role:derived-role", "role:base-role")
	require.NoError(t, err)
	assert.True(t, added, "Should add role inheritance")

	// Verify inheritance exists
	policies, err := enforcer.enforcer.GetNamedGroupingPolicy("g2")
	require.NoError(t, err)
	var foundBefore bool
	for _, policy := range policies {
		if len(policy) >= 2 && policy[0] == "role:derived-role" && policy[1] == "role:base-role" {
			foundBefore = true
			break
		}
	}
	assert.True(t, foundBefore, "Inheritance should exist before removal")

	// Remove role inheritance
	removed, err := enforcer.RemoveRoleInheritance("role:derived-role", "role:base-role")
	require.NoError(t, err)
	assert.True(t, removed, "Should remove role inheritance")

	// Verify inheritance is removed
	policies, err = enforcer.enforcer.GetNamedGroupingPolicy("g2")
	require.NoError(t, err)
	var foundAfter bool
	for _, policy := range policies {
		if len(policy) >= 2 && policy[0] == "role:derived-role" && policy[1] == "role:base-role" {
			foundAfter = true
			break
		}
	}
	assert.False(t, foundAfter, "Inheritance should not exist after removal")
}

func TestRemoveRoleInheritance_NonExistent(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Try to remove non-existent inheritance
	removed, err := enforcer.RemoveRoleInheritance("role:fake-child", "role:fake-parent")
	require.NoError(t, err)
	assert.False(t, removed, "Should return false for non-existent inheritance")
}

func TestGetImplicitPermissionsForUser(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign global-admin role to user
	_, err = enforcer.AddUserRole("user:implicit-test", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Get implicit permissions (should include all global-admin policies)
	permissions, err := enforcer.GetImplicitPermissionsForUser("user:implicit-test")
	require.NoError(t, err)
	assert.NotEmpty(t, permissions, "Admin should have implicit permissions")

	// Should have at least the admin policies (projects, rgds, instances, users, applications)
	assert.GreaterOrEqual(t, len(permissions), 5, "Should have at least 5 policies from global-admin")

	// Check specific permissions are included
	var hasProjectsPolicy bool
	for _, perm := range permissions {
		if len(perm) >= 2 && perm[1] == "projects/*" {
			hasProjectsPolicy = true
			break
		}
	}
	assert.True(t, hasProjectsPolicy, "Should have projects/* permission")
}

func TestGetImplicitPermissionsForUser_NoRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Get permissions for user with no role
	permissions, err := enforcer.GetImplicitPermissionsForUser("user:norole")
	require.NoError(t, err)
	assert.Empty(t, permissions, "User with no role should have no permissions")
}

func TestGetImplicitPermissionsForUser_MultipleRoles(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create custom role
	_, err = enforcer.AddPolicy("role:custom", "custom/*", "read", "allow")
	require.NoError(t, err)

	// Assign serveradmin + custom role
	_, err = enforcer.AddUserRole("user:multi-implicit", CasbinRoleServerAdmin)
	require.NoError(t, err)
	_, err = enforcer.AddUserRole("user:multi-implicit", "role:custom")
	require.NoError(t, err)

	// Get implicit permissions
	permissions, err := enforcer.GetImplicitPermissionsForUser("user:multi-implicit")
	require.NoError(t, err)

	// Should have permissions from both roles (serveradmin has 11 policies + custom 1)
	assert.GreaterOrEqual(t, len(permissions), 12, "Should have serveradmin (11) + custom (1) policies")
}

func TestGetImplicitRolesForUser(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign global-admin role (which inherits global-viewer via g2)
	_, err = enforcer.AddUserRole("user:roles-test", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Get implicit roles
	roles, err := enforcer.GetImplicitRolesForUser("user:roles-test")
	require.NoError(t, err)

	// Should have the directly assigned role
	assert.Contains(t, roles, CasbinRoleServerAdmin)
}

func TestBuiltinRoleInheritance_AdminInheritsViewer(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign only global-admin role
	_, err = enforcer.AddUserRole("user:inheritance-test", CasbinRoleServerAdmin)
	require.NoError(t, err)

	// Admin should be able to do viewer actions (get, list) due to inheritance
	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		// Viewer actions (inherited)
		{"admin get project", "projects/test", "get", true},
		{"admin list projects", "projects/any", "list", true},
		{"admin get rgd", "rgds/test", "get", true},
		{"admin list rgds", "rgds/any", "list", true},
		// Admin-only actions
		{"admin create project", "projects/new", "create", true},
		{"admin delete project", "projects/old", "delete", true},
		{"admin update user", "users/someone", "update", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:inheritance-test", tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestConcurrentPolicyModification(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	done := make(chan bool, 200)

	// Concurrent adds
	for i := 0; i < 100; i++ {
		go func(idx int) {
			roleName := "role:concurrent-" + string(rune('A'+idx%26))
			_, err := enforcer.AddPolicy(roleName, "projects/*", "get", "allow")
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Concurrent enforces during modifications
	for i := 0; i < 100; i++ {
		go func() {
			_, err := enforcer.Enforce("user:test", "projects/test", "get")
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 200; i++ {
		<-done
	}
}

func TestConcurrentRoleAssignment(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	done := make(chan bool, 100)

	// Concurrent role assignments
	for i := 0; i < 50; i++ {
		go func(idx int) {
			user := "user:concurrent-" + string(rune('0'+idx%10))
			_, err := enforcer.AddUserRole(user, "role:test-viewer")
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Concurrent role checks
	for i := 0; i < 50; i++ {
		go func(idx int) {
			user := "user:concurrent-" + string(rune('0'+idx%10))
			_, err := enforcer.GetRolesForUser(user)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestEdgeCases_EmptyInputs(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Enforce with empty subject
	allowed, err := enforcer.Enforce("", "projects/test", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "Empty subject should not have access")

	// Enforce with empty object
	allowed, err = enforcer.Enforce("user:test", "", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "Empty object should not match")

	// Enforce with empty action
	allowed, err = enforcer.Enforce("user:test", "projects/test", "")
	require.NoError(t, err)
	assert.False(t, allowed, "Empty action should not match")
}

func TestEdgeCases_SpecialCharacters(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Test with special characters in identifiers (should be handled safely)
	tests := []struct {
		name   string
		sub    string
		obj    string
		act    string
		expect bool
	}{
		{"unicode subject", "user:日本語", "projects/test", "get", false},
		{"dash in name", "user:test-user", "projects/test", "get", false},
		{"underscore in name", "user:test_user", "projects/test", "get", false},
		{"dot in name", "user:test.user", "projects/test", "get", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce(tt.sub, tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, allowed)
		})
	}
}

func TestProjectRoleWithDenyPolicy(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create project role
	projectRole, err := FormatProjectRole("test-project", "developer")
	require.NoError(t, err)

	// Add allow policy for the project
	_, err = enforcer.AddPolicy(projectRole, "projects/test-project/*", "*", "allow")
	require.NoError(t, err)

	// Add deny policy for specific action
	_, err = enforcer.AddPolicy(projectRole, "projects/test-project/secrets", "delete", "deny")
	require.NoError(t, err)

	// Assign role to user
	_, err = enforcer.AddUserRole("user:dev", projectRole)
	require.NoError(t, err)

	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		{"can get project resource", "projects/test-project/deployments", "get", true},
		{"can create in project", "projects/test-project/apps", "create", true},
		{"cannot delete secrets", "projects/test-project/secrets", "delete", false},
		{"can get secrets", "projects/test-project/secrets", "get", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:dev", tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s)", "user:dev", tt.obj, tt.act)
		})
	}
}

func TestSanitizeGlobCharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal-name", "normal-name"},
		{"name*with*asterisks", "name\\*with\\*asterisks"},
		{"name?with?questions", "name\\?with\\?questions"},
		{"name[with]brackets", "name\\[with\\]brackets"},
		{"name{with}braces", "name\\{with\\}braces"},
		{"*?[]{}combined", "\\*\\?\\[\\]\\{\\}combined"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitize.GlobCharacters(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Test: Compliance Permissions (Enterprise: OPA Gatekeeper compliance view) ---

func TestBuiltinPolicies_ComplianceAccess_ServerAdmin(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign serveradmin role to user
	_, err = enforcer.AddUserRole("user:alice", CasbinRoleServerAdmin)
	require.NoError(t, err)

	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		// Admin has full access to compliance resources
		{"admin get compliance summary", "compliance/summary", "get", true},
		{"admin list compliance templates", "compliance/templates", "list", true},
		{"admin get compliance template", "compliance/templates/k8srequiredlabels", "get", true},
		{"admin list compliance constraints", "compliance/constraints", "list", true},
		{"admin get compliance constraint", "compliance/constraints/require-team-label", "get", true},
		{"admin list compliance violations", "compliance/violations", "list", true},
		{"admin wildcard access", "compliance/*", "*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:alice", tt.obj, tt.act)
			require.NoError(t, err, "Enforce error for %s", tt.name)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				"user:alice", tt.obj, tt.act, allowed, tt.expected)
		})
	}
}

func TestBuiltinPolicies_ComplianceAccess_GlobalReadonlyRemoved(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Assign deprecated global readonly role - should have NO compliance access
	_, err = enforcer.AddUserRole("user:bob", "role:readonly")
	require.NoError(t, err)

	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		// Global readonly has NO policies - all compliance access denied
		{"readonly get compliance summary denied", "compliance/summary", "get", false},
		{"readonly list compliance templates denied", "compliance/templates", "list", false},
		{"readonly get compliance template denied", "compliance/templates/k8srequiredlabels", "get", false},
		{"readonly list compliance constraints denied", "compliance/constraints", "list", false},
		{"readonly get compliance constraint denied", "compliance/constraints/require-team-label", "get", false},
		{"readonly list compliance violations denied", "compliance/violations", "list", false},
		{"readonly cannot create compliance template", "compliance/templates", "create", false},
		{"readonly cannot update compliance constraint", "compliance/constraints/test", "update", false},
		{"readonly cannot delete compliance violation", "compliance/violations/v1", "delete", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:bob", tt.obj, tt.act)
			require.NoError(t, err, "Enforce error for %s", tt.name)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				"user:bob", tt.obj, tt.act, allowed, tt.expected)
		})
	}
}

func TestBuiltinPolicies_ComplianceAccess_NoRole(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// User with no role should have no compliance access
	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		{"no role cannot get compliance summary", "compliance/summary", "get", false},
		{"no role cannot list compliance templates", "compliance/templates", "list", false},
		{"no role cannot get compliance template", "compliance/templates/k8srequiredlabels", "get", false},
		{"no role cannot list compliance constraints", "compliance/constraints", "list", false},
		{"no role cannot list compliance violations", "compliance/violations", "list", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:norole", tt.obj, tt.act)
			require.NoError(t, err, "Enforce error for %s", tt.name)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				"user:norole", tt.obj, tt.act, allowed, tt.expected)
		})
	}
}

func TestResourceComplianceConstant(t *testing.T) {
	// Verify the compliance resource constant exists and has the correct value
	assert.Equal(t, "compliance", ResourceCompliance)
}

func TestResourceSecretsConstant(t *testing.T) {
	// Verify the secrets resource constant exists and has the correct value
	assert.Equal(t, "secrets", ResourceSecrets)
}

func TestBuiltinPolicies_SecretsAccess_ServerAdmin(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Verify serveradmin has secrets/* in built-in policies
	exists, err := enforcer.HasPolicy(CasbinRoleServerAdmin, "secrets/*", "*", "allow")
	require.NoError(t, err)
	assert.True(t, exists, "Built-in serveradmin should have secrets/* policy")

	// Assign serveradmin role and test enforcement
	_, err = enforcer.AddUserRole("user:admin", CasbinRoleServerAdmin)
	require.NoError(t, err)

	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		{"admin get secret", "secrets/demo/my-secret", "get", true},
		{"admin create secret", "secrets/demo/my-secret", "create", true},
		{"admin update secret", "secrets/demo/my-secret", "update", true},
		{"admin delete secret", "secrets/demo/my-secret", "delete", true},
		{"admin list secrets", "secrets/demo/my-secret", "list", true},
		{"admin wildcard secrets", "secrets/*", "*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:admin", tt.obj, tt.act)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, allowed)
		})
	}
}

func TestFormatObject_Compliance(t *testing.T) {
	// Test FormatObject with compliance resource
	obj, err := FormatObject(ResourceCompliance, "summary")
	require.NoError(t, err)
	assert.Equal(t, "compliance/summary", obj)

	obj, err = FormatObject(ResourceCompliance, "templates")
	require.NoError(t, err)
	assert.Equal(t, "compliance/templates", obj)

	obj, err = FormatObject(ResourceCompliance, "*")
	require.NoError(t, err)
	assert.Equal(t, "compliance/*", obj)

	obj, err = FormatObject(ResourceCompliance, "")
	require.NoError(t, err)
	assert.Equal(t, "compliance/*", obj)
}

func TestBuiltinPolicies_ComplianceAccess_CustomRoleWithoutPermission(t *testing.T) {
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create a custom role with project access but no compliance access
	projectRole, err := FormatProjectRole("engineering", "developer")
	require.NoError(t, err)

	// Add policies for this project role (only project and instance access)
	_, err = enforcer.AddPolicy(projectRole, "projects/engineering", "*", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "instances/engineering/*", "*", "allow")
	require.NoError(t, err)
	_, err = enforcer.AddPolicy(projectRole, "rgds/*", "get", "allow")
	require.NoError(t, err)

	// Assign role to user
	_, err = enforcer.AddUserRole("user:dev", projectRole)
	require.NoError(t, err)

	// Custom role without compliance permission should be denied
	tests := []struct {
		name     string
		obj      string
		act      string
		expected bool
	}{
		// Should NOT have compliance access
		{"custom role cannot get compliance summary", "compliance/summary", "get", false},
		{"custom role cannot list compliance templates", "compliance/templates", "list", false},
		{"custom role cannot get compliance template", "compliance/templates/k8srequiredlabels", "get", false},
		{"custom role cannot list compliance constraints", "compliance/constraints", "list", false},
		{"custom role cannot list compliance violations", "compliance/violations", "list", false},
		{"custom role cannot access compliance wildcard", "compliance/*", "get", false},

		// Should still have their project permissions
		{"custom role can access own project", "projects/engineering", "get", true},
		{"custom role can access own instances", "instances/engineering/app1", "get", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce("user:dev", tt.obj, tt.act)
			require.NoError(t, err, "Enforce error for %s", tt.name)
			assert.Equal(t, tt.expected, allowed, "Enforce(%s, %s, %s) = %v, want %v",
				"user:dev", tt.obj, tt.act, allowed, tt.expected)
		})
	}
}
