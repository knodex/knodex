// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// newFakeTeamWatcherWithOnChange builds a teamWatcher whose config carries the
// given OnChange callback.
func newFakeTeamWatcherWithOnChange(t *testing.T, onChange func(string)) (*teamWatcher, *TeamStore) {
	t.Helper()
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, teamListGVR)
	store := NewTeamStore()
	w := NewTeamWatcher(client, store, testNamespace, TeamWatcherConfig{OnChange: onChange}).(*teamWatcher)
	return w, store
}

// TestTeamWatcher_OnChange_FiresAfterUpsert asserts the OnChange callback is
// invoked after an upsert, AND that the store already reflects the new groups
// when the callback runs (AC #6: a triggered re-sync must read fresh data).
func TestTeamWatcher_OnChange_FiresAfterUpsert(t *testing.T) {
	t.Parallel()

	var firedTeam string
	var groupsAtCallback []string
	var storeReadyAtCallback bool

	var store *TeamStore
	var w *teamWatcher
	w, store = newFakeTeamWatcherWithOnChange(t, func(team string) {
		firedTeam = team
		g, ok := store.GetGroups(team)
		storeReadyAtCallback = ok
		groupsAtCallback = g
	})

	w.onAdd(teamUnstructured("alpha", "1", "g1", "g2"))

	if firedTeam != "alpha" {
		t.Errorf("expected OnChange fired for 'alpha', got %q", firedTeam)
	}
	if !storeReadyAtCallback {
		t.Error("expected store to already contain the team when OnChange fires")
	}
	if len(groupsAtCallback) != 2 {
		t.Errorf("expected store to hold 2 groups at callback time, got %v", groupsAtCallback)
	}
}

// TestTeamWatcher_OnChange_FiresAfterDelete asserts OnChange fires after a delete
// and that the team is already absent from the store when it runs.
func TestTeamWatcher_OnChange_FiresAfterDelete(t *testing.T) {
	t.Parallel()

	var firedTeam string
	var presentAtCallback bool

	var store *TeamStore
	var w *teamWatcher
	w, store = newFakeTeamWatcherWithOnChange(t, func(team string) {
		firedTeam = team
		_, presentAtCallback = store.GetGroups(team)
	})

	store.Upsert(teamWithGroups("alpha", "g1"))
	w.onDelete(teamUnstructured("alpha", "1", "g1"))

	if firedTeam != "alpha" {
		t.Errorf("expected OnChange fired for 'alpha' on delete, got %q", firedTeam)
	}
	if presentAtCallback {
		t.Error("expected team already removed from store when OnChange fires on delete")
	}
}

// TestTeamWatcher_OnChange_NilSafe ensures a nil OnChange callback never panics.
func TestTeamWatcher_OnChange_NilSafe(t *testing.T) {
	t.Parallel()
	w, _ := newFakeTeamWatcher(t) // config has no OnChange

	// Neither of these should panic with a nil OnChange.
	w.onAdd(teamUnstructured("alpha", "1", "g1"))
	w.onDelete(teamUnstructured("alpha", "1", "g1"))
}
