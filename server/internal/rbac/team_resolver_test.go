// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"reflect"
	"testing"
)

// fakeTeamResolver is a trivial in-memory TeamResolver for tests.
type fakeTeamResolver map[string][]string

func (f fakeTeamResolver) GetGroups(name string) ([]string, bool) {
	g, ok := f[name]
	return g, ok
}

func TestResolveRoleTeamGroups(t *testing.T) {
	t.Parallel()

	resolver := fakeTeamResolver{
		"platform-team": {"platform-admins", "sre"},
		"app-team":      {"app-devs", "sre"}, // "sre" overlaps platform-team
		"empty-team":    {},
	}

	tests := []struct {
		name        string
		role        ProjectRole
		resolver    TeamResolver
		want        []string
		wantMissing []string // teams reported via onMissing, in order
		nilResolver bool
	}{
		{
			name:     "only teams, resolves to groups",
			role:     ProjectRole{Teams: []string{"platform-team"}},
			resolver: resolver,
			want:     []string{"platform-admins", "sre"},
		},
		{
			name:     "multiple teams, dedup of overlapping group",
			role:     ProjectRole{Teams: []string{"platform-team", "app-team"}},
			resolver: resolver,
			// platform-team -> platform-admins, sre ; app-team -> app-devs, sre(dup)
			want: []string{"platform-admins", "sre", "app-devs"},
		},
		{
			name:        "missing team contributes nothing and triggers onMissing",
			role:        ProjectRole{Teams: []string{"ghost-team", "platform-team"}},
			resolver:    resolver,
			want:        []string{"platform-admins", "sre"},
			wantMissing: []string{"ghost-team"},
		},
		{
			name:     "team that exists but has empty groups contributes nothing, not reported missing",
			role:     ProjectRole{Teams: []string{"empty-team"}},
			resolver: resolver,
			want:     nil,
		},
		{
			name:        "nil resolver reports each team via onMissing, returns nil",
			role:        ProjectRole{Teams: []string{"platform-team"}},
			nilResolver: true,
			want:        nil,
			wantMissing: []string{"platform-team"},
		},
		{
			name:     "no teams returns nil",
			role:     ProjectRole{},
			resolver: resolver,
			want:     nil,
		},
		{
			name:     "deterministic order: teams resolved in role.Teams order",
			role:     ProjectRole{Teams: []string{"app-team", "platform-team"}},
			resolver: resolver,
			// app-team -> app-devs, sre ; platform-team -> platform-admins, sre(dup)
			want: []string{"app-devs", "sre", "platform-admins"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var missing []string
			onMissing := func(team string) { missing = append(missing, team) }

			var r TeamResolver = tt.resolver
			if tt.nilResolver {
				r = nil
			}

			got := resolveRoleTeamGroups(tt.role, r, onMissing)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveRoleTeamGroups() = %v, want %v", got, tt.want)
			}
			if !equalStringSlices(missing, tt.wantMissing) {
				t.Errorf("onMissing teams = %v, want %v", missing, tt.wantMissing)
			}
		})
	}
}

// TestResolveRoleTeamGroups_NilOnMissing ensures a nil onMissing callback is safe.
func TestResolveRoleTeamGroups_NilOnMissing(t *testing.T) {
	t.Parallel()
	resolver := fakeTeamResolver{"t": {"g"}}
	role := ProjectRole{Teams: []string{"t", "ghost"}}

	got := resolveRoleTeamGroups(role, resolver, nil)
	want := []string{"g"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveRoleTeamGroups() = %v, want %v", got, want)
	}
}

// TestResolveRoleTeamGroupsWithCallbacks_EmptyTeam ensures empty teams are reported via onEmpty.
func TestResolveRoleTeamGroupsWithCallbacks_EmptyTeam(t *testing.T) {
	t.Parallel()
	resolver := fakeTeamResolver{
		"real-team":  {"group-a"},
		"empty-team": {},
	}
	var missing, empty []string
	got := resolveRoleTeamGroupsWithCallbacks(
		ProjectRole{Teams: []string{"empty-team", "real-team"}},
		resolver,
		func(team string) { missing = append(missing, team) },
		func(team string) { empty = append(empty, team) },
	)
	if !reflect.DeepEqual(got, []string{"group-a"}) {
		t.Errorf("got %v, want [group-a]", got)
	}
	if len(missing) != 0 {
		t.Errorf("unexpected missing teams: %v", missing)
	}
	if !reflect.DeepEqual(empty, []string{"empty-team"}) {
		t.Errorf("empty teams = %v, want [empty-team]", empty)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
