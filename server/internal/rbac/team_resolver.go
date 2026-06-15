// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

// TeamResolver resolves a team name to its configured OIDC groups.
// Backed by *TeamStore in production; trivially fakeable in tests.
type TeamResolver interface {
	GetGroups(name string) ([]string, bool)
}

// resolveRoleTeamGroups returns the dedup'd union of the oidcGroups of every
// team in role.Teams. A missing team contributes nothing and is reported via
// onMissing; a team that exists but has zero groups is reported via onEmpty.
// Either callback may be nil.
//
// When resolver is nil (e.g. dynamicClient unavailable / unit tests without a
// store), each team in role.Teams is reported via onMissing, and nil is returned.
//
// This helper is the SINGLE place team→group expansion lives — every consumer
// (enforcement path and login/claims path) calls it, so there is no second
// authorization path (NFR-T1). Teams are only ever expanded into group
// strings, never enforced separately.
//
// Ordering is deterministic: teams resolved in role.Teams order, first
// occurrence of each group kept (dedup).
func resolveRoleTeamGroups(role ProjectRole, resolver TeamResolver, onMissing func(team string)) []string {
	return resolveRoleTeamGroupsWithCallbacks(role, resolver, onMissing, nil)
}

// resolveRoleTeamGroupsWithCallbacks is the full version of resolveRoleTeamGroups
// that additionally fires onEmpty for teams that exist but have no configured groups.
func resolveRoleTeamGroupsWithCallbacks(role ProjectRole, resolver TeamResolver, onMissing, onEmpty func(team string)) []string {
	if len(role.Teams) == 0 {
		return nil
	}

	// Resolver unavailable: report each team as unresolvable, return nil.
	if resolver == nil {
		if onMissing != nil {
			for _, team := range role.Teams {
				onMissing(team)
			}
		}
		return nil
	}

	seen := make(map[string]struct{}, len(role.Teams))
	out := make([]string, 0, len(role.Teams))

	appendGroup := func(g string) {
		if _, ok := seen[g]; ok {
			return
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}

	for _, team := range role.Teams {
		groups, ok := resolver.GetGroups(team)
		if !ok {
			if onMissing != nil {
				onMissing(team)
			}
			continue
		}
		if len(groups) == 0 {
			if onEmpty != nil {
				onEmpty(team)
			}
			continue
		}
		for _, g := range groups {
			appendGroup(g)
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}
