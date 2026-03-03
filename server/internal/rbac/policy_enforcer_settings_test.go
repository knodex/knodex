package rbac

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPolicyEnforcer_Settings_GlobalAdmin tests global admin has full settings access
func TestPolicyEnforcer_Settings_GlobalAdmin(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign global admin role
	err := pe.AssignUserRoles(ctx, "user:admin", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Global admin should have full settings access
	testCases := []struct {
		resource string
		action   string
		expected bool
		desc     string
	}{
		{"settings/general", "get", true, "Global admin should read general settings"},
		{"settings/oidc", "get", true, "Global admin should read OIDC settings"},
		{"settings/oidc", "update", true, "Global admin should update OIDC settings"},
		{"settings/*", "get", true, "Global admin should read all settings (wildcard)"},
		{"settings/*", "*", true, "Global admin should have full settings access (wildcard)"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			allowed, err := pe.CanAccess(ctx, "user:admin", tc.resource, tc.action)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, allowed, tc.desc)
		})
	}
}

// TestPolicyEnforcer_Settings_GlobalViewer tests global viewer does NOT have settings access
func TestPolicyEnforcer_Settings_GlobalViewer(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// Assign global viewer role
	err := pe.AssignUserRoles(ctx, "user:viewer", []string{"role:readonly"})
	require.NoError(t, err)

	// Global viewer should NOT have settings access (settings is admin-only)
	testCases := []struct {
		resource string
		action   string
		expected bool
		desc     string
	}{
		{"settings/general", "get", false, "Global viewer should NOT read general settings"},
		{"settings/oidc", "get", false, "Global viewer should NOT read OIDC settings"},
		{"settings/oidc", "update", false, "Global viewer should NOT update OIDC settings"},
		{"settings/*", "get", false, "Global viewer should NOT read all settings"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			allowed, err := pe.CanAccess(ctx, "user:viewer", tc.resource, tc.action)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, allowed, tc.desc)
		})
	}
}

// TestPolicyEnforcer_Settings_ProjectAdmin tests project admin settings access via groups
func TestPolicyEnforcer_Settings_ProjectAdmin(t *testing.T) {
	t.Parallel()

	pe, _ := newTestEnforcerWithMock(t)
	ctx := context.Background()

	// Create project with admin role that has settings get permission
	// ArgoCD-aligned policy format
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "proj-test"},
		Spec: ProjectSpec{
			Description: "Test project with settings access",
			Roles: []ProjectRole{
				{
					Name:        "admin",
					Description: "Project admin with settings access",
					Policies: []string{
						"p, proj:proj-test:admin, projects, get, proj-test, allow",
						"p, proj:proj-test:admin, projects, update, proj-test, allow",
						"p, proj:proj-test:admin, settings, get, *, allow",
					},
					Groups: []string{"project-admin-group"},
				},
				{
					Name:        "developer",
					Description: "Developer without settings access",
					Policies: []string{
						"p, proj:proj-test:developer, projects, get, proj-test, allow",
						"p, proj:proj-test:developer, instances, *, proj-test/*, allow",
					},
					Groups: []string{"developer-group"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Project admin (via group) should have settings get access
	allowed, err := pe.CanAccessWithGroups(ctx, "user:proj-admin", []string{"project-admin-group"}, "settings/general", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Project admin should have settings get access via group")

	// Project admin should NOT have settings update access (not in their policy)
	allowed, err = pe.CanAccessWithGroups(ctx, "user:proj-admin", []string{"project-admin-group"}, "settings/oidc", "update")
	require.NoError(t, err)
	assert.False(t, allowed, "Project admin should NOT have settings update access")

	// Developer (via group) should NOT have settings access
	allowed, err = pe.CanAccessWithGroups(ctx, "user:developer", []string{"developer-group"}, "settings/general", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "Developer should NOT have settings access")
}

// TestPolicyEnforcer_Settings_NoRole tests unauthenticated/no-role user has no settings access
func TestPolicyEnforcer_Settings_NoRole(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)
	ctx := context.Background()

	// User without any role assignment
	allowed, err := pe.CanAccess(ctx, "user:anonymous", "settings/general", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "User without role should NOT have settings access")

	allowed, err = pe.CanAccessWithGroups(ctx, "user:anonymous", []string{}, "settings/*", "get")
	require.NoError(t, err)
	assert.False(t, allowed, "User without role or groups should NOT have settings access")
}

// TestScopeObjectToProject directly tests the scopeObjectToProject function
// to verify wildcard expansion and project scoping logic.
func TestScopeObjectToProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		projectName string
		object      string
		expected    []string
	}{
		// --- Wildcard expansion ---
		{
			name:        "bare wildcard expands to all resource types",
			projectName: "my-project",
			object:      "*",
			expected: []string{
				"projects/my-project",
				"instances/my-project/*",
				"repositories/my-project/*",
				"applications/my-project/*",
				"rgds/*",
			},
		},
		{
			name:        "project wildcard expands same as bare wildcard",
			projectName: "my-project",
			object:      "my-project/*",
			expected: []string{
				"projects/my-project",
				"instances/my-project/*",
				"repositories/my-project/*",
				"applications/my-project/*",
				"rgds/*",
			},
		},

		// --- Resource-specific scoping ---
		{
			name:        "instances wildcard scoped to project",
			projectName: "my-project",
			object:      "instances/*",
			expected:    []string{"instances/my-project/*"},
		},
		{
			name:        "repositories wildcard scoped to project",
			projectName: "my-project",
			object:      "repositories/*",
			expected:    []string{"repositories/my-project/*"},
		},
		{
			name:        "applications wildcard scoped to project",
			projectName: "my-project",
			object:      "applications/*",
			expected:    []string{"applications/my-project/*"},
		},
		{
			name:        "projects wildcard scoped to own project",
			projectName: "my-project",
			object:      "projects/*",
			expected:    []string{"projects/my-project"},
		},

		// --- Pass-through cases ---
		{
			name:        "other project wildcard passes through unchanged",
			projectName: "my-project",
			object:      "other-project/*",
			expected:    []string{"other-project/*"},
		},
		{
			name:        "rgds pass through as global",
			projectName: "my-project",
			object:      "rgds/*",
			expected:    []string{"rgds/*"},
		},
		{
			name:        "explicit path passes through",
			projectName: "my-project",
			object:      "projects/other-project",
			expected:    []string{"projects/other-project"},
		},

		// --- Edge cases ---
		{
			name:        "empty project name with wildcard",
			projectName: "",
			object:      "*",
			expected: []string{
				"projects/",
				"instances//*",
				"repositories//*",
				"applications//*",
				"rgds/*",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := scopeObjectToProject(tt.projectName, tt.object)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPolicyEnforcer_WildcardResourceType_RepositoryAccess tests that 6-part ArgoCD format
// policies with resourceType="*" (like bootstrap platform-admin) correctly grant access
// to resource-prefixed objects like "repositories/{project}/*".
//
// This is a regression test for the bug where "p, sub, *, *, project/*, allow" produced
// Casbin object "project/*" that didn't match "repositories/project/*" checks.
func TestPolicyEnforcer_WildcardResourceType_RepositoryAccess(t *testing.T) {
	t.Parallel()

	pe, _ := newTestEnforcerWithMock(t)
	ctx := context.Background()

	projectName := "default-project"

	// Create project with bootstrap-style platform-admin role
	// This uses the 6-part ArgoCD format with resourceType="*"
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: projectName},
		Spec: ProjectSpec{
			Description: "Test project for wildcard resource type",
			Roles: []ProjectRole{
				{
					Name:        "platform-admin",
					Description: "Full access via wildcard resource type",
					Policies: []string{
						fmt.Sprintf("p, proj:%s:platform-admin, *, *, %s/*, allow", projectName, projectName),
					},
				},
				{
					Name:        "viewer",
					Description: "Read-only via wildcard resource type",
					Policies: []string{
						fmt.Sprintf("p, proj:%s:viewer, *, get, %s/*, allow", projectName, projectName),
					},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(ctx, project)
	require.NoError(t, err)

	// Assign platform-admin role directly (bypasses group validation)
	platformAdminRole := fmt.Sprintf("proj:%s:platform-admin", projectName)
	err = pe.AssignUserRoles(ctx, "user:test-admin", []string{platformAdminRole})
	require.NoError(t, err)

	t.Run("platform-admin repository access", func(t *testing.T) {
		// This was the original bug: platform-admin couldn't access repositories
		// because "p, sub, *, *, project/*, allow" produced Casbin object "project/*"
		// which didn't match "repositories/project/*" checks
		allowed, err := pe.CanAccess(ctx, "user:test-admin",
			fmt.Sprintf("repositories/%s/*", projectName), "get")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have repository get access")

		allowed, err = pe.CanAccess(ctx, "user:test-admin",
			fmt.Sprintf("repositories/%s/*", projectName), "create")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have repository create access")

		allowed, err = pe.CanAccess(ctx, "user:test-admin",
			fmt.Sprintf("repositories/%s/*", projectName), "delete")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have repository delete access")
	})

	t.Run("platform-admin instance access", func(t *testing.T) {
		allowed, err := pe.CanAccess(ctx, "user:test-admin",
			fmt.Sprintf("instances/%s/my-instance", projectName), "create")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have instance create access")
	})

	t.Run("platform-admin project access", func(t *testing.T) {
		allowed, err := pe.CanAccess(ctx, "user:test-admin",
			fmt.Sprintf("projects/%s", projectName), "update")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have project update access")
	})

	t.Run("platform-admin application access", func(t *testing.T) {
		allowed, err := pe.CanAccess(ctx, "user:test-admin",
			fmt.Sprintf("applications/%s/my-app", projectName), "create")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have application create access")
	})

	t.Run("platform-admin rgd access", func(t *testing.T) {
		allowed, err := pe.CanAccess(ctx, "user:test-admin",
			"rgds/any-rgd", "get")
		require.NoError(t, err)
		assert.True(t, allowed, "platform-admin should have RGD get access (global)")
	})

	t.Run("platform-admin cross-project denied", func(t *testing.T) {
		// Security: wildcard should NOT grant cross-project access
		allowed, err := pe.CanAccess(ctx, "user:test-admin",
			"repositories/other-project/*", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "platform-admin should NOT access other project's repositories")

		allowed, err = pe.CanAccess(ctx, "user:test-admin",
			"instances/other-project/foo", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "platform-admin should NOT access other project's instances")

		allowed, err = pe.CanAccess(ctx, "user:test-admin",
			"projects/other-project", "get")
		require.NoError(t, err)
		assert.False(t, allowed, "platform-admin should NOT access other project")
	})

	t.Run("viewer repository read access", func(t *testing.T) {
		viewerRole := fmt.Sprintf("proj:%s:viewer", projectName)
		err := pe.AssignUserRoles(ctx, "user:viewer", []string{viewerRole})
		require.NoError(t, err)

		allowed, err := pe.CanAccess(ctx, "user:viewer",
			fmt.Sprintf("repositories/%s/*", projectName), "get")
		require.NoError(t, err)
		assert.True(t, allowed, "viewer should have repository get access")

		// Viewer should NOT have write access
		allowed, err = pe.CanAccess(ctx, "user:viewer",
			fmt.Sprintf("repositories/%s/*", projectName), "create")
		require.NoError(t, err)
		assert.False(t, allowed, "viewer should NOT have repository create access")
	})
}
