// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// This file contains test-support helpers that must be accessible from external
// test packages (e.g. server/internal/api/handlers). They are not part of the
// public Watcher contract and must not be called from production code.
//
// If Go's export_test.go mechanism were sufficient (it is only visible to the
// wrapper package's own tests), these would live there. Because they are needed
// cross-package they are exported here with names that signal their purpose.
package wrapper

// SetWatcherEntriesForTest replaces the Watcher's in-memory cache without
// going through the ConfigMap informer. Use only in tests to prime the cache
// to a known state before exercising handler logic.
func SetWatcherEntriesForTest(w *Watcher, entries []Entry) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if entries == nil {
		w.lastValidEntries = []Entry{}
	} else {
		cp := make([]Entry, len(entries))
		copy(cp, entries)
		w.lastValidEntries = cp
	}
	w.lastEntriesHash = entriesHash(w.lastValidEntries)
}
