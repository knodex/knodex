// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// teamOnlyProjectList builds an unstructured Project list with a single project
// whose role is bound ONLY via teams[] (no literal groups[]).
func teamOnlyProjectList(projectName, roleName, teamName string) *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{
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
						"description": "team-only project",
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
}

// TestProjectService_TeamOnlyBinding_AppearsInClaims covers AC #5: a project
// reachable ONLY via a team binding must appear in both GetUserProjectRolesByGroup
// (JWT roles claim) and GetUserProjectsByGroup (accessible-project list) for a
// user whose OIDC group is one of the team's resolved groups.
func TestProjectService_TeamOnlyBinding_AppearsInClaims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockClient, mockResource := setupMockDynamicClient()
	mockResource.On("List", mock.Anything, mock.Anything).
		Return(teamOnlyProjectList("alpha", "developer", "platform-team"), nil)

	svc := NewProjectService(nil, mockClient, "knodex-system")
	svc.SetTeamResolver(fakeTeamResolver{"platform-team": {"platform-admins"}})

	// User is in "platform-admins" — only reachable via the team binding.
	userGroups := []string{"platform-admins"}

	roles, err := svc.GetUserProjectRolesByGroup(ctx, userGroups)
	require.NoError(t, err)
	assert.Equal(t, "developer", roles["alpha"], "team-only project/role must appear in the roles claim")

	projects, err := svc.GetUserProjectsByGroup(ctx, userGroups)
	require.NoError(t, err)
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	assert.Contains(t, names, "alpha", "team-only project must appear in the accessible-project list")
}

// TestProjectService_TeamOnlyBinding_NoResolver proves that without a resolver,
// a team-only binding is NOT honored (only literal groups match) — confirming the
// resolver is the single expansion point and behavior is nil-safe.
func TestProjectService_TeamOnlyBinding_NoResolver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	mockClient, mockResource := setupMockDynamicClient()
	mockResource.On("List", mock.Anything, mock.Anything).
		Return(teamOnlyProjectList("alpha", "developer", "platform-team"), nil)

	svc := NewProjectService(nil, mockClient, "knodex-system")
	// No SetTeamResolver call → teamResolver is nil.

	roles, err := svc.GetUserProjectRolesByGroup(ctx, []string{"platform-admins"})
	require.NoError(t, err)
	assert.Empty(t, roles, "without a resolver, a team-only binding must not match")
}
