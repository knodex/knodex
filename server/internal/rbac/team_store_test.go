// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"fmt"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func teamWithGroups(name string, groups ...string) *Team {
	return &Team{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       TeamSpec{OIDCGroups: groups},
	}
}

func TestTeamStore_UpsertGetRemoveList(t *testing.T) {
	t.Parallel()

	s := NewTeamStore()

	// Empty store
	if groups, ok := s.GetGroups("absent"); ok || groups != nil {
		t.Errorf("expected absent team to return (nil, false), got (%v, %v)", groups, ok)
	}
	if len(s.List()) != 0 {
		t.Errorf("expected empty list, got %v", s.List())
	}

	// Upsert
	s.Upsert(teamWithGroups("alpha", "alpha-devs", "alpha-ops"))
	groups, ok := s.GetGroups("alpha")
	if !ok {
		t.Fatal("expected alpha to exist after upsert")
	}
	if len(groups) != 2 || groups[0] != "alpha-devs" || groups[1] != "alpha-ops" {
		t.Errorf("unexpected groups: %v", groups)
	}

	// Returned slice is a copy — mutating it must not affect the store.
	groups[0] = "MUTATED"
	again, _ := s.GetGroups("alpha")
	if again[0] != "alpha-devs" {
		t.Errorf("store mutated via returned slice: %v", again)
	}

	// Upsert replaces
	s.Upsert(teamWithGroups("alpha", "new-group"))
	replaced, _ := s.GetGroups("alpha")
	if len(replaced) != 1 || replaced[0] != "new-group" {
		t.Errorf("expected replaced groups [new-group], got %v", replaced)
	}

	// List
	s.Upsert(teamWithGroups("beta", "beta-devs"))
	names := s.List()
	if len(names) != 2 {
		t.Errorf("expected 2 teams, got %v", names)
	}

	// Remove
	s.Remove("alpha")
	if _, ok := s.GetGroups("alpha"); ok {
		t.Error("expected alpha to be removed")
	}
	if len(s.List()) != 1 {
		t.Errorf("expected 1 team after remove, got %v", s.List())
	}

	// Remove absent is a no-op (no panic)
	s.Remove("does-not-exist")
}

func TestTeamStore_UpsertNil(t *testing.T) {
	t.Parallel()
	s := NewTeamStore()
	s.Upsert(nil) // must not panic
	if len(s.List()) != 0 {
		t.Errorf("expected empty store after nil upsert, got %v", s.List())
	}
}

func TestTeamStore_SourceSliceIsolation(t *testing.T) {
	t.Parallel()
	s := NewTeamStore()

	src := []string{"g1", "g2"}
	s.Upsert(teamWithGroups("alpha", src...))
	src[0] = "MUTATED"

	stored, _ := s.GetGroups("alpha")
	if stored[0] != "g1" {
		t.Errorf("store affected by source slice mutation: %v", stored)
	}
}

func TestTeamStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	s := NewTeamStore()

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("team-%d", n)
			for j := 0; j < 100; j++ {
				s.Upsert(teamWithGroups(name, fmt.Sprintf("g-%d", j)))
				_, _ = s.GetGroups(name)
				_ = s.List()
				if j%2 == 0 {
					s.Remove(name)
				}
			}
		}(i)
	}

	wg.Wait()
	// Passes if -race detects no data races.
}
