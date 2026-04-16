// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/models"
)

// newTestGraphRevisionWatcher creates a watcher with pre-populated cache for testing.
func newTestGraphRevisionWatcher() *GraphRevisionWatcher {
	w := &GraphRevisionWatcher{
		cache:  make(map[string][]models.GraphRevision),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
		logger: slog.Default().With("component", "graph-revision-watcher-test"),
	}
	w.running.Store(true)
	w.synced.Store(true)
	return w
}

func TestGraphRevisionWatcher_AddToCache(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1, CreatedAt: time.Now()})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 3, CreatedAt: time.Now()})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 2, CreatedAt: time.Now()})

	list := w.ListRevisions("my-rgd")
	if list.TotalCount != 3 {
		t.Fatalf("TotalCount = %d, want 3", list.TotalCount)
	}
	// Should be sorted descending
	if list.Items[0].RevisionNumber != 3 {
		t.Errorf("first item revision = %d, want 3", list.Items[0].RevisionNumber)
	}
	if list.Items[1].RevisionNumber != 2 {
		t.Errorf("second item revision = %d, want 2", list.Items[1].RevisionNumber)
	}
	if list.Items[2].RevisionNumber != 1 {
		t.Errorf("third item revision = %d, want 1", list.Items[2].RevisionNumber)
	}
}

func TestGraphRevisionWatcher_AddToCache_UpdateExisting(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1, ContentHash: "old"})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1, ContentHash: "new"})

	list := w.ListRevisions("my-rgd")
	if list.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1 (should deduplicate)", list.TotalCount)
	}
	if list.Items[0].ContentHash != "new" {
		t.Errorf("ContentHash = %q, want %q", list.Items[0].ContentHash, "new")
	}
}

func TestGraphRevisionWatcher_RemoveFromCache(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 2})
	w.removeFromCache("my-rgd", 1)

	list := w.ListRevisions("my-rgd")
	if list.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", list.TotalCount)
	}
	if list.Items[0].RevisionNumber != 2 {
		t.Errorf("remaining revision = %d, want 2", list.Items[0].RevisionNumber)
	}
}

func TestGraphRevisionWatcher_RemoveFromCache_LastEntry(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1})
	w.removeFromCache("my-rgd", 1)

	list := w.ListRevisions("my-rgd")
	if list.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", list.TotalCount)
	}
}

func TestGraphRevisionWatcher_GetRevision(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 2})

	rev, found := w.GetRevision("my-rgd", 2)
	if !found {
		t.Fatal("revision 2 not found")
	}
	if rev.RevisionNumber != 2 {
		t.Errorf("RevisionNumber = %d, want 2", rev.RevisionNumber)
	}

	_, found = w.GetRevision("my-rgd", 99)
	if found {
		t.Error("revision 99 should not be found")
	}

	_, found = w.GetRevision("nonexistent", 1)
	if found {
		t.Error("nonexistent RGD should not be found")
	}
}

func TestGraphRevisionWatcher_GetLatestRevision(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 5})
	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 3})

	rev, found := w.GetLatestRevision("my-rgd")
	if !found {
		t.Fatal("latest revision not found")
	}
	if rev.RevisionNumber != 5 {
		t.Errorf("latest RevisionNumber = %d, want 5", rev.RevisionNumber)
	}

	_, found = w.GetLatestRevision("nonexistent")
	if found {
		t.Error("nonexistent RGD should not have latest revision")
	}
}

func TestGraphRevisionWatcher_ListRevisions_ReturnsCopy(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "my-rgd", RevisionNumber: 1})

	list1 := w.ListRevisions("my-rgd")
	list1.Items[0].RevisionNumber = 999

	list2 := w.ListRevisions("my-rgd")
	if list2.Items[0].RevisionNumber == 999 {
		t.Error("ListRevisions should return a copy, not a reference to cache data")
	}
}

func TestGraphRevisionWatcher_ListRevisions_EmptyRGD(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	list := w.ListRevisions("nonexistent")
	if list.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", list.TotalCount)
	}
	if list.Items == nil {
		t.Error("Items should be empty slice, not nil")
	}
}

func TestGraphRevisionWatcher_MultipleRGDs(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.addToCache(models.GraphRevision{RGDName: "rgd-a", RevisionNumber: 1})
	w.addToCache(models.GraphRevision{RGDName: "rgd-b", RevisionNumber: 1})
	w.addToCache(models.GraphRevision{RGDName: "rgd-b", RevisionNumber: 2})

	listA := w.ListRevisions("rgd-a")
	if listA.TotalCount != 1 {
		t.Errorf("rgd-a TotalCount = %d, want 1", listA.TotalCount)
	}

	listB := w.ListRevisions("rgd-b")
	if listB.TotalCount != 2 {
		t.Errorf("rgd-b TotalCount = %d, want 2", listB.TotalCount)
	}
}

func TestGraphRevisionWatcher_CallbackInvocation(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	var changeCalled atomic.Int32
	var updateAction string
	var updateRGD string
	var updateRev int

	w.SetOnChangeCallback(func() {
		changeCalled.Add(1)
	})
	w.SetOnUpdateCallback(func(action string, rgdName string, revision int) {
		updateAction = action
		updateRGD = rgdName
		updateRev = revision
	})

	w.notifyChange()
	w.notifyUpdate("add", "my-rgd", 3)

	if changeCalled.Load() != 1 {
		t.Errorf("change callback called %d times, want 1", changeCalled.Load())
	}
	if updateAction != "add" {
		t.Errorf("update action = %q, want %q", updateAction, "add")
	}
	if updateRGD != "my-rgd" {
		t.Errorf("update rgdName = %q, want %q", updateRGD, "my-rgd")
	}
	if updateRev != 3 {
		t.Errorf("update revision = %d, want 3", updateRev)
	}
}

func TestGraphRevisionWatcher_CallbackPanicRecovery(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	w.SetOnChangeCallback(func() {
		panic("test panic in change callback")
	})
	w.SetOnUpdateCallback(func(action string, rgdName string, revision int) {
		panic("test panic in update callback")
	})

	// These should not panic the test — recover() in notifyChange/notifyUpdate catches it
	w.notifyChange()
	w.notifyUpdate("add", "test", 1)
}

func TestGraphRevisionWatcher_ImplementsProvider(t *testing.T) {
	w := newTestGraphRevisionWatcher()

	// Compile-time check: *GraphRevisionWatcher satisfies services.GraphRevisionProvider
	var _ interface {
		ListRevisions(string) models.GraphRevisionList
		GetRevision(string, int) (*models.GraphRevision, bool)
		GetLatestRevision(string) (*models.GraphRevision, bool)
	} = w
}
