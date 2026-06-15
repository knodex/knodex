// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeStatusUpdater captures UpdateProjectStatus calls for assertion.
type fakeStatusUpdater struct {
	mu    sync.Mutex
	calls []*Project
}

func (f *fakeStatusUpdater) UpdateProjectStatus(_ context.Context, p *Project) (*Project, error) {
	f.mu.Lock()
	f.calls = append(f.calls, p.DeepCopyObject().(*Project))
	f.mu.Unlock()
	return p, nil
}

// lastCondition returns the most-recently written RolesResolved condition, or nil.
func (f *fakeStatusUpdater) lastRolesResolved() *ProjectCondition {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		for _, c := range f.calls[i].Status.Conditions {
			if c.Type == ProjectConditionRolesResolved {
				cp := c
				return &cp
			}
		}
	}
	return nil
}

// newTestEnforcerWithTeams builds a PolicyEnforcer wired with the given team
// resolver (nil ProjectReader — tests load projects directly).
func newTestEnforcerWithTeams(t *testing.T, resolver TeamResolver) PolicyEnforcer {
	t.Helper()
	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)
	return NewPolicyEnforcerWithConfig(enforcer, nil, DefaultPolicyEnforcerConfig(), WithTeamResolver(resolver))
}

// teamBoundProject returns a project whose single role grants project "get" and
// is bound via teams[] only (Teams-only binding, Story 10.6).
// The groups parameter is ignored and retained only for call-site compatibility.
func teamBoundProject(name string, _ []string, teams []string) *Project {
	return &Project{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ProjectSpec{
			Roles: []ProjectRole{
				{
					Name:     "developer",
					Policies: []string{"projects/" + name + ", get, allow"},
					Teams:    teams,
				},
			},
		},
	}
}

// TestPolicyEnforcer_TeamBinding_GrantsViaResolvedGroup proves a user whose OIDC
// group is one of a bound team's groups is granted access through the single
// Casbin enforcer — the grouping policy is identical to the raw-group path
// (AC #1, #4).
func TestPolicyEnforcer_TeamBinding_GrantsViaResolvedGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	resolver := fakeTeamResolver{"platform-team": {"platform-admins"}}
	pe := newTestEnforcerWithTeams(t, resolver)

	project := teamBoundProject("team-only", nil, []string{"platform-team"})
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	// User in the team's resolved group is granted.
	allowed, err := pe.CanAccessWithGroups(ctx, "user:alice", []string{"platform-admins"}, "projects/team-only", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "user in team-resolved group should be granted")

	// User in an unrelated group is denied.
	denied, err := pe.CanAccessWithGroups(ctx, "user:bob", []string{"random-group"}, "projects/team-only", "get")
	require.NoError(t, err)
	assert.False(t, denied, "user not in any resolved group should be denied")
}

// TestPolicyEnforcer_TeamBinding_MultipleTeams covers binding multiple teams to one role.
func TestPolicyEnforcer_TeamBinding_MultipleTeams(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	resolver := fakeTeamResolver{
		"platform-team": {"platform-admins", "shared"},
		"ops-team":      {"ops-staff"},
	}
	pe := newTestEnforcerWithTeams(t, resolver)

	project := teamBoundProject("multi-team-proj", nil, []string{"platform-team", "ops-team"})
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	for _, group := range []string{"platform-admins", "shared", "ops-staff"} {
		allowed, err := pe.CanAccessWithGroups(ctx, "user:u", []string{group}, "projects/multi-team-proj", "get")
		require.NoError(t, err)
		assert.Truef(t, allowed, "group %q (resolved from team) should grant access", group)
	}
}

// TestPolicyEnforcer_TeamBinding_MissingTeam covers AC #3: a missing team
// contributes no policies, does not panic, and the role grants no access.
func TestPolicyEnforcer_TeamBinding_MissingTeam(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Resolver knows nothing about "ghost-team".
	resolver := fakeTeamResolver{}
	pe := newTestEnforcerWithTeams(t, resolver)

	project := teamBoundProject("missing-team-proj", nil, []string{"ghost-team"})
	require.NotPanics(t, func() {
		require.NoError(t, pe.LoadProjectPolicies(ctx, project))
	})

	// The (missing) team contributes nothing — any user is denied.
	denied, err := pe.CanAccessWithGroups(ctx, "user:u", []string{"unrelated"}, "projects/missing-team-proj", "get")
	require.NoError(t, err)
	assert.False(t, denied, "missing team must contribute no access")
}

// TestPolicyEnforcer_TeamBinding_NilResolver proves nil-safety: with no resolver
// configured, teams contribute no groups and all access is denied.
func TestPolicyEnforcer_TeamBinding_NilResolver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pe := newTestEnforcerWithTeams(t, nil)
	project := teamBoundProject("nil-resolver-proj", nil, []string{"some-team"})
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	// Without a resolver, the team cannot be resolved — no access granted.
	denied, err := pe.CanAccessWithGroups(ctx, "user:u", []string{"some-group"}, "projects/nil-resolver-proj", "get")
	require.NoError(t, err)
	assert.False(t, denied, "without resolver, teams contribute no access")
}

// TestPolicyEnforcer_TeamBinding_ReResolution simulates the team-change re-sync
// (AC #6): a team's groups change, the project is reloaded, and access reflects
// the new group set (added grants, removed revokes). Reloading the project is
// exactly what the watcher's OnChange → SyncPolicies path does.
func TestPolicyEnforcer_TeamBinding_ReResolution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	resolver := fakeTeamResolver{"team-x": {"old-group"}}
	pe := newTestEnforcerWithTeams(t, resolver)

	project := teamBoundProject("rr-proj", nil, []string{"team-x"})
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	// Initially, old-group grants.
	allowed, err := pe.CanAccessWithGroups(ctx, "user:u", []string{"old-group"}, "projects/rr-proj", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "old-group should grant before the team change")

	// Team groups change: old-group removed, new-group added.
	resolver["team-x"] = []string{"new-group"}
	// Re-sync: reload the project's policies (what OnChange → SyncPolicies triggers).
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	// new-group now grants.
	allowed, err = pe.CanAccessWithGroups(ctx, "user:u", []string{"new-group"}, "projects/rr-proj", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "new-group should grant after the team change")

	// old-group is revoked (no stale grouping policy lingers).
	revoked, err := pe.CanAccessWithGroups(ctx, "user:u", []string{"old-group"}, "projects/rr-proj", "get")
	require.NoError(t, err)
	assert.False(t, revoked, "old-group should be revoked after re-resolution")
}

// TestPolicyEnforcer_RolesResolved_LoadProjectPolicies verifies that
// LoadProjectPolicies sets RolesResolved: False for a missing team and
// RolesResolved: True when all teams are resolved (AC #7).
func TestPolicyEnforcer_RolesResolved_LoadProjectPolicies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	resolver := fakeTeamResolver{} // empty: "ghost-team" is missing
	su := &fakeStatusUpdater{}

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)
	pe := NewPolicyEnforcerWithConfig(enforcer, nil, DefaultPolicyEnforcerConfig(),
		WithTeamResolver(resolver),
		WithProjectStatusUpdater(su),
	)

	project := teamBoundProject("roles-resolved-lp", nil, []string{"ghost-team"})
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	cond := su.lastRolesResolved()
	require.NotNil(t, cond, "RolesResolved condition must be written")
	assert.Equal(t, ConditionStatusFalse, cond.Status, "missing team → RolesResolved: False")
	assert.Equal(t, "TeamUnresolved", cond.Reason)

	// Now resolve the team and reload — condition must flip to True.
	resolver["ghost-team"] = []string{"ghost-group"}
	require.NoError(t, pe.LoadProjectPolicies(ctx, project))

	cond = su.lastRolesResolved()
	require.NotNil(t, cond)
	assert.Equal(t, ConditionStatusTrue, cond.Status, "resolved team → RolesResolved: True")
	assert.Equal(t, "AllTeamsResolved", cond.Reason)
}

// TestPolicyEnforcer_RolesResolved_SyncPolicies verifies that SyncPolicies clears
// RolesResolved: False → True once a previously-missing team is resolved.
// This specifically covers the bug fixed in story review: the previous code only
// updated the condition for projects with issues, so resolved projects were stuck.
func TestPolicyEnforcer_RolesResolved_SyncPolicies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	resolver := fakeTeamResolver{} // "ghost-team" missing initially
	su := &fakeStatusUpdater{}

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)
	reader := newMockProjectReader()
	project := teamBoundProject("sync-roles-resolved", nil, []string{"ghost-team"})
	reader.AddProject(project)

	pe := NewPolicyEnforcerWithConfig(enforcer, reader, DefaultPolicyEnforcerConfig(),
		WithTeamResolver(resolver),
		WithProjectStatusUpdater(su),
	)

	// First SyncPolicies: ghost-team is missing → RolesResolved: False.
	require.NoError(t, pe.SyncPolicies(ctx))
	cond := su.lastRolesResolved()
	require.NotNil(t, cond, "condition must be written on first sync")
	assert.Equal(t, ConditionStatusFalse, cond.Status, "missing team → RolesResolved: False")

	// Resolve the team.
	resolver["ghost-team"] = []string{"ghost-group"}

	// Second SyncPolicies: all teams resolved → RolesResolved must flip to True.
	require.NoError(t, pe.SyncPolicies(ctx))
	cond = su.lastRolesResolved()
	require.NotNil(t, cond)
	assert.Equal(t, ConditionStatusTrue, cond.Status, "resolved team → RolesResolved must flip to True in SyncPolicies")
}
