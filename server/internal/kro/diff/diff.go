// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package diff provides a diff engine for comparing RGD revision snapshots.
// It is a pure domain package with no dependency on HTTP or WebSocket.
package diff

import (
	"fmt"
	"log/slog"

	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
)

// GraphRevisionProvider is the subset of services.GraphRevisionProvider needed by DiffService.
// Using the full interface from services keeps the diff package free of watcher imports.
type GraphRevisionProvider = services.GraphRevisionProvider

// DiffService computes and caches diffs between RGD revision snapshots.
type DiffService struct {
	cache  *diffCache
	logger *slog.Logger
}

// NewDiffService creates a new DiffService with a default-size LRU cache.
func NewDiffService() (*DiffService, error) {
	return NewDiffServiceWithSize(defaultCacheSize)
}

// NewDiffServiceWithSize creates a new DiffService with a custom LRU cache size.
func NewDiffServiceWithSize(size int) (*DiffService, error) {
	c, err := newDiffCache(size)
	if err != nil {
		return nil, fmt.Errorf("diff: failed to create cache: %w", err)
	}
	return &DiffService{
		cache:  c,
		logger: slog.Default().With("component", "diff-service"),
	}, nil
}

// GetDiff returns the diff between rev1 and rev2 for the given RGD.
// Results are cached; cache hits avoid re-fetching revision snapshots.
// If rev1 > rev2, the arguments are swapped to canonical order (smaller first).
func (s *DiffService) GetDiff(provider GraphRevisionProvider, rgdName string, rev1, rev2 int) (*models.RevisionDiff, error) {
	// Canonical ordering: rev1 is always the smaller (older) revision.
	if rev1 > rev2 {
		rev1, rev2 = rev2, rev1
	}

	if cached, ok := s.cache.get(rgdName, rev1, rev2); ok {
		return cached, nil
	}

	oldRev, found := provider.GetRevision(rgdName, rev1)
	if !found {
		return nil, fmt.Errorf("diff: revision %d not found for RGD %q", rev1, rgdName)
	}

	newRev, found := provider.GetRevision(rgdName, rev2)
	if !found {
		return nil, fmt.Errorf("diff: revision %d not found for RGD %q", rev2, rgdName)
	}

	d, err := ComputeDiff(oldRev.Snapshot, newRev.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("diff: compute failed: %w", err)
	}
	d.RGDName = rgdName
	d.Rev1 = rev1
	d.Rev2 = rev2

	s.cache.set(rgdName, rev1, rev2, d)
	return d, nil
}

// PreComputeConsecutiveDiff pre-computes and caches the diff between `revision` and `revision-1`.
// Intended as a watcher callback: called after a new revision is added to warm the cache.
func (s *DiffService) PreComputeConsecutiveDiff(provider GraphRevisionProvider, rgdName string, revision int) {
	if revision <= 1 {
		return
	}
	prev := revision - 1

	// Skip if already cached.
	if _, ok := s.cache.get(rgdName, prev, revision); ok {
		return
	}

	_, err := s.GetDiff(provider, rgdName, prev, revision)
	if err != nil {
		s.logger.Warn("pre-compute consecutive diff failed",
			"rgdName", rgdName,
			"rev1", prev,
			"rev2", revision,
			"error", err)
	} else {
		s.logger.Debug("pre-computed consecutive diff",
			"rgdName", rgdName,
			"rev1", prev,
			"rev2", revision)
	}
}

// ComputeDiff performs a recursive map comparison between two RGD snapshots.
// Returns a RevisionDiff with added, removed, and modified fields.
// Uses reflect-style recursive comparison — no OpenAPI schema required.
func ComputeDiff(oldSpec, newSpec map[string]interface{}) (*models.RevisionDiff, error) {
	d := &models.RevisionDiff{
		Added:    []models.DiffField{},
		Removed:  []models.DiffField{},
		Modified: []models.DiffField{},
	}

	diffMaps("", oldSpec, newSpec, d)

	d.Identical = len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0
	return d, nil
}

// diffMaps recursively compares two maps and appends changes to d.
func diffMaps(prefix string, old, new map[string]interface{}, d *models.RevisionDiff) {
	// Find removed and modified fields (keys in old).
	for key, oldVal := range old {
		path := joinPath(prefix, key)
		newVal, exists := new[key]
		if !exists {
			d.Removed = append(d.Removed, models.DiffField{
				Path:     path,
				OldValue: oldVal,
			})
			continue
		}

		oldMap, oldIsMap := oldVal.(map[string]interface{})
		newMap, newIsMap := newVal.(map[string]interface{})

		if oldIsMap && newIsMap {
			diffMaps(path, oldMap, newMap, d)
			continue
		}

		if !valuesEqual(oldVal, newVal) {
			d.Modified = append(d.Modified, models.DiffField{
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
	}

	// Find added fields (keys in new but not in old).
	for key, newVal := range new {
		if _, exists := old[key]; !exists {
			d.Added = append(d.Added, models.DiffField{
				Path:     joinPath(prefix, key),
				NewValue: newVal,
			})
		}
	}
}

// joinPath builds a dot-separated field path.
func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// valuesEqual performs a deep equality check for two interface{} values.
func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	aMap, aIsMap := a.(map[string]interface{})
	bMap, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		return mapsEqual(aMap, bMap)
	}
	if aIsMap != bIsMap {
		return false
	}

	aSlice, aIsSlice := toSlice(a)
	bSlice, bIsSlice := toSlice(b)
	if aIsSlice && bIsSlice {
		if len(aSlice) != len(bSlice) {
			return false
		}
		for i := range aSlice {
			if !valuesEqual(aSlice[i], bSlice[i]) {
				return false
			}
		}
		return true
	}
	if aIsSlice != bIsSlice {
		return false
	}

	// Scalar comparison via fmt.Sprintf for consistent cross-type equality
	// (handles JSON number types like float64 vs int).
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// mapsEqual returns true when two maps have identical keys and values.
func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !valuesEqual(av, bv) {
			return false
		}
	}
	return true
}

// toSlice attempts to convert an interface{} to []interface{}.
func toSlice(v interface{}) ([]interface{}, bool) {
	switch s := v.(type) {
	case []interface{}:
		return s, true
	default:
		return nil, false
	}
}
