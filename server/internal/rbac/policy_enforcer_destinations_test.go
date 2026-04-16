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

// TestScopeObjectToProjectWithDestinations tests the destination-aware policy scoping.
func TestScopeObjectToProjectWithDestinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		projectName  string
		object       string
		destinations []string
		expected     []string
	}{
		{
			name:         "wildcard without destinations - project-wide",
			projectName:  "alpha",
			object:       "*",
			destinations: nil,
			expected: []string{
				"projects/alpha",
				"instances/alpha/*",
				"secrets/alpha/*",
				"repositories/alpha/*",
				"compliance/alpha/*",
				"rgds/*",
			},
		},
		{
			name:         "wildcard with destinations - namespace-scoped + project-wide",
			projectName:  "alpha",
			object:       "*",
			destinations: []string{"xxx-applications", "xxx-shared"},
			expected: []string{
				"projects/alpha",
				// Namespace-bearing resources: per-destination
				"instances/alpha/xxx-applications/*",
				"instances/alpha/xxx-shared/*",
				"secrets/alpha/xxx-applications/*",
				"secrets/alpha/xxx-shared/*",
				// Project-level resources: always project-wide
				"repositories/alpha/*",
				"compliance/alpha/*",
				"rgds/*",
			},
		},
		{
			name:         "instances/* without destinations",
			projectName:  "alpha",
			object:       "instances/*",
			destinations: nil,
			expected:     []string{"instances/alpha/*"},
		},
		{
			name:         "instances/* with destinations",
			projectName:  "alpha",
			object:       "instances/*",
			destinations: []string{"ns-app", "ns-infra"},
			expected:     []string{"instances/alpha/ns-app/*", "instances/alpha/ns-infra/*"},
		},
		{
			name:         "secrets/* with single destination",
			projectName:  "alpha",
			object:       "secrets/*",
			destinations: []string{"xxx-shared"},
			expected:     []string{"secrets/alpha/xxx-shared/*"},
		},
		{
			name:         "rgds/* unchanged regardless of destinations",
			projectName:  "alpha",
			object:       "rgds/*",
			destinations: []string{"ns-app"},
			expected:     []string{"rgds/*"},
		},
		{
			name:         "projects/* unchanged regardless of destinations",
			projectName:  "alpha",
			object:       "projects/*",
			destinations: []string{"ns-app"},
			expected:     []string{"projects/alpha"},
		},
		{
			name:         "explicit path unchanged",
			projectName:  "alpha",
			object:       "instances/alpha/specific-path",
			destinations: []string{"ns-app"},
			expected:     []string{"instances/alpha/specific-path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := scopeObjectToProjectWithDestinations(tt.projectName, tt.object, tt.destinations)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPolicyEnforcer_NamespaceScopedPolicies tests that roles with destinations
// generate namespace-scoped Casbin policies.
func TestPolicyEnforcer_NamespaceScopedPolicies(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alpha",
		},
		Spec: ProjectSpec{
			Description: "Alpha project",
			Destinations: []Destination{
				{Namespace: "xxx-infra"},
				{Namespace: "xxx-shared"},
				{Namespace: "xxx-applications"},
			},
			Roles: []ProjectRole{
				{
					Name:         "developer",
					Description:  "App developer",
					Policies:     []string{"instances/*, *, allow", "secrets/*, *, allow"},
					Groups:       []string{"alpha-devs"},
					Destinations: []string{"xxx-applications"},
				},
				{
					Name:         "shared-reader",
					Description:  "Read secrets in shared",
					Policies:     []string{"secrets/*, get, allow"},
					Groups:       []string{"alpha-devs"},
					Destinations: []string{"xxx-shared"},
				},
				{
					Name:         "operator",
					Description:  "Platform operator",
					Policies:     []string{"instances/*, *, allow", "secrets/*, *, allow"},
					Groups:       []string{"platform-engineers"},
					Destinations: []string{"xxx-infra", "xxx-shared"},
				},
				{
					Name:        "project-wide",
					Description: "Role without destinations - backward compat",
					Policies:    []string{"instances/*, get, allow"},
					Groups:      []string{"viewers"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Test: developer can access instances in xxx-applications
	canAccess, err := pe.CanAccessWithGroups(context.Background(), "dev-user", []string{"alpha-devs"}, "instances/alpha/xxx-applications/WebApp/my-app", "create")
	require.NoError(t, err)
	assert.True(t, canAccess, "developer should access instances in xxx-applications")

	// Test: developer CANNOT access instances in xxx-infra (not in destinations)
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "dev-user", []string{"alpha-devs"}, "instances/alpha/xxx-infra/WebApp/my-app", "create")
	require.NoError(t, err)
	assert.False(t, canAccess, "developer should NOT access instances in xxx-infra")

	// Test: shared-reader can GET secrets in xxx-shared
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "reader-user", []string{"alpha-devs"}, "secrets/alpha/xxx-shared/my-secret", "get")
	require.NoError(t, err)
	assert.True(t, canAccess, "shared-reader should GET secrets in xxx-shared")

	// Test: shared-reader CANNOT create secrets in xxx-shared
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "reader-user2", []string{"alpha-devs"}, "secrets/alpha/xxx-shared/my-secret", "create")
	require.NoError(t, err)
	assert.False(t, canAccess, "shared-reader should NOT create secrets in xxx-shared")

	// Test: operator has full access in xxx-infra
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "ops-user", []string{"platform-engineers"}, "instances/alpha/xxx-infra/Deployment/nginx", "create")
	require.NoError(t, err)
	assert.True(t, canAccess, "operator should have full access in xxx-infra")

	// Test: operator has full access in xxx-shared
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "ops-user", []string{"platform-engineers"}, "instances/alpha/xxx-shared/Deployment/redis", "delete")
	require.NoError(t, err)
	assert.True(t, canAccess, "operator should have full access in xxx-shared")

	// Test: operator CANNOT access xxx-applications (not in destinations)
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "ops-user", []string{"platform-engineers"}, "instances/alpha/xxx-applications/WebApp/app", "create")
	require.NoError(t, err)
	assert.False(t, canAccess, "operator should NOT access xxx-applications")

	// Test: project-wide role (no destinations) has access everywhere
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "viewer-user", []string{"viewers"}, "instances/alpha/xxx-applications/WebApp/app", "get")
	require.NoError(t, err)
	assert.True(t, canAccess, "project-wide role should GET instances in any namespace")

	canAccess, err = pe.CanAccessWithGroups(context.Background(), "viewer-user", []string{"viewers"}, "instances/alpha/xxx-infra/Deployment/nginx", "get")
	require.NoError(t, err)
	assert.True(t, canAccess, "project-wide role should GET instances in any namespace")
}

// TestPolicyEnforcer_NamespaceScopedBuiltInPolicies tests that built-in admin/readonly
// roles respect destinations when set.
func TestPolicyEnforcer_NamespaceScopedBuiltInPolicies(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "beta",
		},
		Spec: ProjectSpec{
			Description: "Beta project",
			Destinations: []Destination{
				{Namespace: "ns-staging"},
				{Namespace: "ns-prod"},
			},
			Roles: []ProjectRole{
				{
					Name:         "admin",
					Description:  "Admin scoped to staging only",
					Policies:     []string{"*, *, allow"},
					Groups:       []string{"staging-admins"},
					Destinations: []string{"ns-staging"},
				},
				{
					Name:        "readonly",
					Description: "Global readonly within project",
					Policies:    []string{"*, get, allow"},
					Groups:      []string{"readers"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Test: scoped admin can manage instances in ns-staging
	canAccess, err := pe.CanAccessWithGroups(context.Background(), "staging-admin", []string{"staging-admins"}, "instances/beta/ns-staging/WebApp/app", "create")
	require.NoError(t, err)
	assert.True(t, canAccess, "scoped admin should manage ns-staging instances")

	// Test: scoped admin CANNOT manage instances in ns-prod
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "staging-admin", []string{"staging-admins"}, "instances/beta/ns-prod/WebApp/app", "create")
	require.NoError(t, err)
	assert.False(t, canAccess, "scoped admin should NOT manage ns-prod instances")

	// Test: unscoped readonly can read everywhere in the project
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "reader", []string{"readers"}, "instances/beta/ns-prod/WebApp/app", "get")
	require.NoError(t, err)
	assert.True(t, canAccess, "unscoped readonly should read ns-prod instances")

	canAccess, err = pe.CanAccessWithGroups(context.Background(), "reader", []string{"readers"}, "instances/beta/ns-staging/WebApp/app", "get")
	require.NoError(t, err)
	assert.True(t, canAccess, "unscoped readonly should read ns-staging instances")
}

// TestPolicyEnforcer_AdminWildcardStillWorks tests that the global admin
// (role:serveradmin with instances/*) still matches namespace-scoped objects.
func TestPolicyEnforcer_AdminWildcardStillWorks(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Set up server admin
	err := pe.AssignUserRoles(context.Background(), "admin-user", []string{"role:serveradmin"})
	require.NoError(t, err)

	// Load a project with namespace-scoped roles
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gamma",
		},
		Spec: ProjectSpec{
			Description: "Gamma project",
			Destinations: []Destination{
				{Namespace: "ns-app"},
			},
			Roles: []ProjectRole{
				{
					Name:         "developer",
					Policies:     []string{"instances/*, *, allow"},
					Destinations: []string{"ns-app"},
				},
			},
		},
	}

	err = pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Admin with "instances/*" should match anything including namespace-scoped paths
	canAccess, err := pe.CanAccess(context.Background(), "admin-user", "instances/gamma/ns-app/WebApp/app", "create")
	require.NoError(t, err)
	assert.True(t, canAccess, "server admin should access any namespace-scoped path")
}

// TestPolicyEnforcer_ClusterScopedWithDestinations tests cluster-scoped instance access.
//
// KNOWN LIMITATION (TODO: STORY-437 follow-up task):
// Roles with destinations CANNOT access cluster-scoped instances. When a role has
// destinations set, policies are generated as "instances/{project}/{dest}/*". The
// cluster-scoped Casbin check uses "instances/{project}/*" (no namespace dimension),
// which does not match namespace-scoped policies via keyMatch. Task 7.2 ("Roles with
// destinations should still generate project-wide policies for cluster-scoped resources")
// is deferred — see action item in story tasks.
//
// Roles WITHOUT destinations generate "instances/{project}/*" and correctly access
// cluster-scoped instances.
func TestPolicyEnforcer_ClusterScopedWithDestinations(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	project := &Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "delta",
		},
		Spec: ProjectSpec{
			Description: "Delta project",
			Destinations: []Destination{
				{Namespace: "ns-app"},
			},
			Roles: []ProjectRole{
				{
					Name:         "destination-scoped-dev",
					Policies:     []string{"instances/*, *, allow"},
					Groups:       []string{"scoped-devs"},
					Destinations: []string{"ns-app"},
				},
				{
					Name:     "project-wide-dev",
					Policies: []string{"instances/*, *, allow"},
					Groups:   []string{"all-devs"},
				},
			},
		},
	}

	err := pe.LoadProjectPolicies(context.Background(), project)
	require.NoError(t, err)

	// Role WITHOUT destinations generates "instances/delta/*" — cluster-scoped access works.
	canAccess, err := pe.CanAccessWithGroups(context.Background(), "dev-user", []string{"all-devs"}, "instances/delta/ClusterWidget/my-widget", "create")
	require.NoError(t, err)
	assert.True(t, canAccess, "project-wide role should access cluster-scoped instances")

	// Role WITH destinations generates "instances/delta/ns-app/*" only.
	// "instances/delta/*" (cluster-scoped check) does NOT match "instances/delta/ns-app/*".
	// TODO: implement cluster-scoped support for destination-scoped roles (STORY-437 task 7.2).
	canAccess, err = pe.CanAccessWithGroups(context.Background(), "scoped-user", []string{"scoped-devs"}, "instances/delta/ClusterWidget/my-widget", "create")
	require.NoError(t, err)
	assert.False(t, canAccess, "destination-scoped role cannot access cluster-scoped instances (known limitation — see TODO above)")
}
