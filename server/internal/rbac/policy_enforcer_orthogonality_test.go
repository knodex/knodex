// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Story 11.3 — Org-Role Orthogonality Guard (FR-T10, DT-6, NFR-T1).
//
// These tests PIN the management/data-plane separation at the enforcement
// boundary: a user carrying only org-membership-style group claims (e.g. a
// non-admin team like kx-team-<org>-platform-eng or arbitrary org groups), with
// NO project-granting group and NO configured serveradmin globalAdmin binding in
// this enforcer, must be DENIED every project/data-plane action. Project access
// derives ONLY from project-group bindings — org/team membership never
// auto-generates project policies (loadProjectPoliciesLocked never sees an org role).
//
// Story 12.1 (ADR adr-cloud-team-membership-keycloak-groups): cloud tokens no
// longer carry kx-proj-* (the Knodex-side minting was retired). Cloud project
// access is now realized through roles[].teams[] → effectiveRoleGroups →
// kx-team-* groups bound to a project role. The orthogonality invariant is
// unchanged and unweakened: an org-only user belongs to no team bound to any
// project, so it carries no project-granting group and reaches nothing. The
// positive controls below use a literal groups[] binding (the raw-group path);
// the kx-team-* path is exercised by the team_resolver tests, not duplicated
// here.
//
// Each test pairs the denial assertions with a positive control (a real
// project-group binding DOES grant its role) so a silently-broken enforcer
// cannot pass by denying everything. This is a regression pin, NOT a new
// authorization layer: nothing here filters groups or adds a second Enforce
// path (NFR-T1 / the STORY-433 anti-pattern is explicitly avoided).

// orgOnlyGroups represents the group claims a cloud org member carries when they
// have org membership but NO project membership and belong to NO project-bound
// team. kx-team-acme-platform-eng is a real (non-admin) team that is NOT bound to
// any project; "some-org-group" stands in for any other non-project group.
// Neither is a project-role binding nor a kx-team-* group bound to a project, so
// neither can reach a project through Casbin in an enforcer without an explicit
// globalAdmin mapping. (Story 12.2 retired kx-org-*-serveradmin; the reserved
// kx-team-<org>-admins group is the serveradmin source and is deliberately NOT
// here — this set models an ordinary org member, not an admin.)
var orgOnlyGroups = []string{"kx-team-acme-platform-eng", "some-org-group"}

// TestPolicyEnforcer_OrgOnlyContext_DeniedAcrossDataPlane pins AC #2: org-only
// group context is denied for representative project/data-plane resources and
// actions, while a real project group still grants access (positive control).
func TestPolicyEnforcer_OrgOnlyContext_DeniedAcrossDataPlane(t *testing.T) {
	t.Parallel()

	pe, reader := newTestEnforcerWithMockAndTeams(t, identityTeamResolver{})
	ctx := context.Background()

	// Project "alpha" grants its developer role to the project group
	// "alpha-developers" via a literal groups[] binding. "*, *, allow" is
	// project-scoped by loadProjectPoliciesLocked into the per-resource object
	// set (projects/alpha, instances/alpha/*, repositories/alpha/*). The role
	// declares destinations[ns-apps] so secrets are emitted as namespace-keyed
	// policies (secrets/ns-apps/*) — secrets are not project-scoped under the
	// namespace-keyed model.
	project := &Project{
		ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
		Spec: ProjectSpec{
			Destinations: []Destination{{Namespace: "ns-apps"}},
			Roles: []ProjectRole{
				{
					Name:         "developer",
					Policies:     []string{"*, *, allow"},
					Teams:        []string{"alpha-developers"},
					Destinations: []string{"ns-apps"},
				},
			},
		},
	}
	reader.AddProject(project)
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	// Representative project/data-plane checks (AC #2 enumerates these).
	// secrets/ns-apps/... is the namespace-keyed shape — projects/alpha is the
	// project-scoped resource. Same enforcer, two shapes, one layer.
	checks := []struct {
		object string
		action string
	}{
		{"instances/alpha/ns-apps/apps", "update"},
		{"secrets/ns-apps/apps", "get"},
		{"projects/alpha", "get"},
		{"projects/alpha", "delete"},
		{"repositories/alpha/my-repo", "create"},
	}

	// DENIAL: org-only group context reaches none of these.
	for _, c := range checks {
		allowed, err := pe.CanAccessWithGroups(ctx, "user:org-member", orgOnlyGroups, c.object, c.action)
		require.NoError(t, err)
		assert.Falsef(t, allowed,
			"org-only groups %v must NOT grant %s on %s (org role must never auto-bear project access)",
			orgOnlyGroups, c.action, c.object)
	}

	// POSITIVE CONTROL: the real project group DOES grant its role — proves the
	// enforcer is correctly wired and the denial above is meaningful.
	for _, c := range checks {
		allowed, err := pe.CanAccessWithGroups(ctx, "user:dev", []string{"alpha-developers"}, c.object, c.action)
		require.NoError(t, err)
		assert.Truef(t, allowed,
			"project group alpha-developers SHOULD grant %s on %s (positive control)",
			c.action, c.object)
	}
}

// teamBoundProjectList builds an unstructured Project list with a single project
// whose role is bound via a teams[] entry. The team name is derived from the group
// name; a corresponding fakeTeamResolver that maps teamName→[group] must be set on
// the ProjectService before calling GetUserProjectRolesByGroup / GetUserProjectsByGroup.
func teamBoundProjectList(projectName, roleName, group string) (*unstructured.UnstructuredList, fakeTeamResolver) {
	teamName := roleName + "-team"
	resolver := fakeTeamResolver{teamName: {group}}
	list := &unstructured.UnstructuredList{
		Object: map[string]interface{}{
			"apiVersion": ProjectGroup + "/" + ProjectVersion,
			"kind":       ProjectKind + "List",
			"metadata":   map[string]interface{}{},
		},
		Items: []unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"apiVersion": ProjectGroup + "/" + ProjectVersion,
					"kind":       ProjectKind,
					"metadata": map[string]interface{}{
						"name":      projectName,
						"namespace": "knodex-system",
					},
					"spec": map[string]interface{}{
						"description": "team-bound project",
						"roles": []interface{}{
							map[string]interface{}{
								"name":     roleName,
								"policies": []interface{}{"projects/" + projectName + ", get, allow"},
								"teams":    []interface{}{teamName},
							},
						},
					},
				},
			},
		},
	}
	return list, resolver
}

// TestProjectService_OrgOnlyContext_NoProjectClaims pins AC #2 on the JWT-claims
// path: org-only group context resolves to NO project roles and NO accessible
// projects, while the real project group resolves to its role (positive control).
// This proves org roles produce no project policies even in the login/claims
// derivation, not just in the live enforcer.
func TestProjectService_OrgOnlyContext_NoProjectClaims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockClient, mockResource := setupMockDynamicClient()
	projectList, resolver := teamBoundProjectList("alpha", "developer", "alpha-developers")
	mockResource.On("List", mock.Anything, mock.Anything).
		Return(projectList, nil)

	svc := NewProjectService(nil, mockClient, "knodex-system")
	svc.SetTeamResolver(resolver)

	// DENIAL: org-only groups match no project role binding.
	roles, err := svc.GetUserProjectRolesByGroup(ctx, orgOnlyGroups)
	require.NoError(t, err)
	assert.Empty(t, roles, "org-only groups must yield no project roles in the JWT claims path")

	projects, err := svc.GetUserProjectsByGroup(ctx, orgOnlyGroups)
	require.NoError(t, err)
	assert.Empty(t, projects, "org-only groups must yield no accessible projects")

	// POSITIVE CONTROL: the real project group resolves to its role/project.
	roles, err = svc.GetUserProjectRolesByGroup(ctx, []string{"alpha-developers"})
	require.NoError(t, err)
	assert.Equal(t, "developer", roles["alpha"], "project group must resolve to its role (positive control)")

	projects, err = svc.GetUserProjectsByGroup(ctx, []string{"alpha-developers"})
	require.NoError(t, err)
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, "alpha", "project group must surface the project (positive control)")
}
