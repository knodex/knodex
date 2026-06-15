// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

// teamListGVR maps the Team CRD GVR to its list kind so fake dynamic clients
// can handle LIST operations without panicking.
var teamListGVR = map[schema.GroupVersionResource]string{
	{Group: TeamGroup, Version: TeamVersion, Resource: TeamResource}: "TeamList",
}

// testNamespace is the install namespace used by unit-test fixtures. Watcher
// and service tests pin their namespace here so the watcher's informer filter
// and store match.
const testNamespace = "knodex-a"

func newFakeTeamWatcher(t *testing.T) (*teamWatcher, *TeamStore) {
	t.Helper()
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, teamListGVR)
	store := NewTeamStore()
	w := NewTeamWatcher(client, store, testNamespace, TeamWatcherConfig{}).(*teamWatcher)
	return w, store
}

func teamUnstructured(name, resourceVersion string, groups ...interface{}) *unstructured.Unstructured {
	return teamUnstructuredInNamespace(name, testNamespace, resourceVersion, groups...)
}

func teamUnstructuredInNamespace(name, namespace, resourceVersion string, groups ...interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": TeamGroup + "/" + TeamVersion,
			"kind":       TeamKind,
			"metadata": map[string]interface{}{
				"name":            name,
				"namespace":       namespace,
				"resourceVersion": resourceVersion,
			},
			"spec": map[string]interface{}{
				"description": "desc",
				"oidcGroups":  groups,
			},
		},
	}
}

func TestNewTeamWatcher_Defaults(t *testing.T) {
	t.Parallel()
	w, _ := newFakeTeamWatcher(t)

	if w.IsRunning() {
		t.Error("expected watcher not running initially")
	}
	if !w.LastSyncTime().IsZero() {
		t.Error("expected zero last sync time initially")
	}
	if w.config.ResyncPeriod != DefaultTeamWatcherResyncPeriod {
		t.Errorf("expected default resync period, got %v", w.config.ResyncPeriod)
	}
	if w.logger == nil {
		t.Error("expected default logger")
	}
}

func TestTeamWatcher_GVRConstants(t *testing.T) {
	t.Parallel()
	if TeamGroup != "knodex.io" || TeamVersion != "v1alpha1" || TeamResource != "teams" || TeamKind != "Team" {
		t.Errorf("unexpected Team CRD constants: %s/%s %s %s", TeamGroup, TeamVersion, TeamResource, TeamKind)
	}
}

func TestTeamWatcher_StopWhenNotRunning(t *testing.T) {
	t.Parallel()
	w, _ := newFakeTeamWatcher(t)
	w.Stop() // must not panic
	if w.IsRunning() {
		t.Error("expected still not running")
	}
}

func TestTeamWatcher_OnAdd(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)

	w.onAdd(teamUnstructured("alpha", "1", "alpha-devs", "alpha-ops"))

	groups, ok := store.GetGroups("alpha")
	if !ok {
		t.Fatal("expected alpha in store after add")
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %v", groups)
	}
	if w.LastSyncTime().IsZero() {
		t.Error("expected sync time updated after add")
	}
}

func TestTeamWatcher_OnUpdate_ChangedVersion(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)

	old := teamUnstructured("alpha", "1", "g1")
	updated := teamUnstructured("alpha", "2", "g1", "g2")
	w.onUpdate(old, updated)

	groups, ok := store.GetGroups("alpha")
	if !ok || len(groups) != 2 {
		t.Errorf("expected updated groups of length 2, got %v ok=%v", groups, ok)
	}
}

func TestTeamWatcher_OnUpdate_SameVersionSkips(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)

	old := teamUnstructured("alpha", "1", "g1")
	same := teamUnstructured("alpha", "1", "g1", "g2")
	w.onUpdate(old, same)

	if _, ok := store.GetGroups("alpha"); ok {
		t.Error("expected no store mutation when resourceVersion unchanged")
	}
}

func TestTeamWatcher_OnDelete(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)

	store.Upsert(teamWithGroups("alpha", "g1"))
	w.onDelete(teamUnstructured("alpha", "1", "g1"))

	if _, ok := store.GetGroups("alpha"); ok {
		t.Error("expected alpha removed after delete")
	}
}

func TestTeamWatcher_OnDelete_Tombstone(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)

	store.Upsert(teamWithGroups("alpha", "g1"))
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "alpha",
		Obj: teamUnstructured("alpha", "1", "g1"),
	}
	w.onDelete(tombstone)

	if _, ok := store.GetGroups("alpha"); ok {
		t.Error("expected alpha removed after tombstone delete")
	}
}

func TestTeamWatcher_OnDelete_InvalidType(t *testing.T) {
	t.Parallel()
	w, _ := newFakeTeamWatcher(t)
	w.onDelete(12345) // must not panic
}

func TestTeamWatcher_OnAdd_InvalidType(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)
	w.onAdd("invalid") // must not panic
	if len(store.List()) != 0 {
		t.Error("expected no store mutation for invalid type")
	}
}

func TestTeamWatcher_OnAdd_MalformedRejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  *unstructured.Unstructured
	}{
		{
			name: "no oidcGroups field",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "alpha"},
				"spec":     map[string]interface{}{"description": "d"},
			}},
		},
		{
			name: "empty oidcGroups",
			obj:  teamUnstructured("alpha", "1"),
		},
		{
			name: "duplicate groups",
			obj:  teamUnstructured("alpha", "1", "dup", "dup"),
		},
		{
			name: "invalid team name",
			obj:  teamUnstructured("Alpha_Team", "1", "g1"),
		},
		{
			name: "group element with empty string",
			obj:  teamUnstructured("alpha", "1", ""),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, store := newFakeTeamWatcher(t)
			w.onAdd(tt.obj) // must not panic
			if len(store.List()) != 0 {
				t.Errorf("expected malformed team rejected, store has %v", store.List())
			}
		})
	}
}

func TestTeamWatcher_OnAdd_NonStringGroupSkipped(t *testing.T) {
	t.Parallel()
	w, store := newFakeTeamWatcher(t)

	// One valid string and one non-string element; the non-string is skipped,
	// leaving a valid single-group team. Use a JSON-valid type (float64) since
	// that is what an informer's unstructured object would actually contain.
	w.onAdd(teamUnstructured("alpha", "1", "g1", float64(42)))

	groups, ok := store.GetGroups("alpha")
	if !ok {
		t.Fatal("expected alpha stored with valid group remaining")
	}
	if len(groups) != 1 || groups[0] != "g1" {
		t.Errorf("expected only valid group [g1], got %v", groups)
	}
}

// TestTeamWatcher_NamespaceIsolation drives the real informer with a fake
// dynamic client seeded across two namespaces and asserts the watcher only
// observes Teams in its own namespace (AC #2, AC #5). This is the safety net
// for the "two Knodex installs in the same cluster" topology.
func TestTeamWatcher_NamespaceIsolation(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, teamListGVR,
		teamUnstructuredInNamespace("x", "knodex-a", "1", "x-grp"),
		teamUnstructuredInNamespace("y", "knodex-b", "1", "y-grp"),
	)
	store := NewTeamStore()
	w := NewTeamWatcher(client, store, "knodex-a", TeamWatcherConfig{}).(*teamWatcher)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan struct{})
	go func() {
		_ = w.Start(ctx)
		close(done)
	}()

	deadline := time.After(5 * time.Second)
	for w.LastSyncTime().IsZero() {
		select {
		case <-deadline:
			t.Fatal("watcher did not sync in time")
		case <-time.After(10 * time.Millisecond):
		}
	}

	// The "knodex-a" watcher must see Team "x" only — Team "y" lives in another
	// install's namespace and must be invisible.
	if _, ok := store.GetGroups("x"); !ok {
		t.Error("expected own-namespace Team 'x' to be observed")
	}
	if _, ok := store.GetGroups("y"); ok {
		t.Error("expected other-namespace Team 'y' to be INVISIBLE (cross-tenant leak)")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop after context cancel")
	}
}

func TestTeamWatcher_LifecycleStartStop(t *testing.T) {
	t.Parallel()
	w, _ := newFakeTeamWatcher(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = w.Start(ctx)
		close(done)
	}()

	// Wait for the watcher to come up.
	deadline := time.After(5 * time.Second)
	for !w.IsRunning() {
		select {
		case <-deadline:
			t.Fatal("watcher did not start in time")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop after context cancel")
	}

	if w.IsRunning() {
		t.Error("expected watcher stopped after context cancel")
	}

	// The informer factory was started with w.stopCh; on context cancellation
	// Start must close it so the factory's goroutines tear down (no leak). The
	// app wires this watcher to runCtx only and never calls Stop(), so closing
	// on ctx cancellation is the sole teardown path.
	select {
	case <-w.stopCh:
		// closed as expected
	default:
		t.Error("expected stopCh closed after context cancel (informer would leak otherwise)")
	}
}
