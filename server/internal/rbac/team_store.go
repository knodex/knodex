// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import "sync"

// TeamStore is an in-memory, mutex-guarded lookup table mapping a team name to
// its resolved set of OIDC groups. The TeamWatcher populates it; Story 10.2's
// policy generator is the only intended consumer (team name → groups).
//
// This is a passive lookup table. It performs NO authorization and is NOT
// wired into Casbin — see CLAUDE.md "Unified Casbin Authorization Model".
type TeamStore struct {
	mu     sync.RWMutex
	groups map[string][]string
}

// NewTeamStore creates an empty TeamStore.
func NewTeamStore() *TeamStore {
	return &TeamStore{
		groups: make(map[string][]string),
	}
}

// Upsert stores (or replaces) the OIDC groups for the given team. The groups
// slice is copied so later mutations by the caller do not affect the store.
func (s *TeamStore) Upsert(team *Team) {
	if team == nil {
		return
	}
	groupsCopy := make([]string, len(team.Spec.OIDCGroups))
	copy(groupsCopy, team.Spec.OIDCGroups)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.groups[team.Name] = groupsCopy
}

// Remove deletes the team from the store. Safe to call for an absent team.
func (s *TeamStore) Remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.groups, name)
}

// GetGroups returns a copy of the OIDC groups for the named team and whether it
// exists. The returned slice is a copy, safe for the caller to mutate.
func (s *TeamStore) GetGroups(name string) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	groups, ok := s.groups[name]
	if !ok {
		return nil, false
	}
	out := make([]string, len(groups))
	copy(out, groups)
	return out, true
}

// List returns the names of all teams currently in the store.
func (s *TeamStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.groups))
	for name := range s.groups {
		names = append(names, name)
	}
	return names
}
