// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package diff

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/knodex/knodex/server/internal/models"
)

const defaultCacheSize = 256

// diffCache wraps an LRU cache for revision diffs.
// Keys are canonical "{rgdName}:{rev1}:{rev2}" with rev1 < rev2.
// Revisions are immutable so cached diffs never go stale.
type diffCache struct {
	cache *lru.Cache[string, *models.RevisionDiff]
}

// newDiffCache creates a new LRU diff cache with the given capacity.
func newDiffCache(size int) (*diffCache, error) {
	c, err := lru.New[string, *models.RevisionDiff](size)
	if err != nil {
		return nil, err
	}
	return &diffCache{cache: c}, nil
}

// cacheKey returns the canonical cache key for a pair of revisions.
// rev1 is always the smaller revision number.
func cacheKey(rgdName string, rev1, rev2 int) string {
	if rev1 > rev2 {
		rev1, rev2 = rev2, rev1
	}
	return fmt.Sprintf("%s:%d:%d", rgdName, rev1, rev2)
}

// get retrieves a cached diff. Returns nil, false on miss.
func (c *diffCache) get(rgdName string, rev1, rev2 int) (*models.RevisionDiff, bool) {
	return c.cache.Get(cacheKey(rgdName, rev1, rev2))
}

// set stores a diff result in the cache.
func (c *diffCache) set(rgdName string, rev1, rev2 int, d *models.RevisionDiff) {
	c.cache.Add(cacheKey(rgdName, rev1, rev2), d)
}
