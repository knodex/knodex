// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"sort"
	"strings"
	"sync"

	"github.com/knodex/knodex/server/internal/models"
)

// InstanceCache provides thread-safe storage for discovered instances
type InstanceCache struct {
	mu        sync.RWMutex
	instances map[string]*models.Instance // key: namespace/kind/name
}

// NewInstanceCache creates a new instance cache
func NewInstanceCache() *InstanceCache {
	return &InstanceCache{
		instances: make(map[string]*models.Instance),
	}
}

// instanceCacheKey generates a unique key for an instance
func instanceCacheKey(namespace, kind, name string) string {
	return namespace + "/" + kind + "/" + name
}

// Set adds or updates an instance in the cache
func (c *InstanceCache) Set(instance *models.Instance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := instanceCacheKey(instance.Namespace, instance.Kind, instance.Name)
	c.instances[key] = instance
}

// Get retrieves an instance from the cache
func (c *InstanceCache) Get(namespace, kind, name string) (*models.Instance, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := instanceCacheKey(namespace, kind, name)
	instance, ok := c.instances[key]
	return instance, ok
}

// Delete removes an instance from the cache
func (c *InstanceCache) Delete(namespace, kind, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := instanceCacheKey(namespace, kind, name)
	delete(c.instances, key)
}

// List returns all instances matching the given options
func (c *InstanceCache) List(opts models.InstanceListOptions) models.InstanceList {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Collect all matching instances
	var matches []models.Instance
	for _, instance := range c.instances {
		if c.matchesFilter(instance, opts) {
			matches = append(matches, *instance)
		}
	}

	// Sort results
	c.sortInstances(matches, opts.SortBy, opts.SortOrder)

	// Apply pagination
	totalCount := len(matches)
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize

	if start >= totalCount {
		return models.InstanceList{
			Items:      []models.Instance{},
			TotalCount: totalCount,
			Page:       opts.Page,
			PageSize:   opts.PageSize,
		}
	}

	if end > totalCount {
		end = totalCount
	}

	return models.InstanceList{
		Items:      matches[start:end],
		TotalCount: totalCount,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
	}
}

// Count returns the total number of instances in the cache
func (c *InstanceCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.instances)
}

// CountByRGD returns the count of instances for a specific RGD
func (c *InstanceCache) CountByRGD(rgdNamespace, rgdName string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, instance := range c.instances {
		if instance.RGDName == rgdName && instance.RGDNamespace == rgdNamespace {
			count++
		}
	}
	return count
}

// CountByNamespaces returns the count of instances in the given namespaces.
// Uses namespace pattern matching (supports wildcards like "team-*").
// If namespaces is nil, returns total count (all instances).
// If namespaces is empty slice, returns 0.
func (c *InstanceCache) CountByNamespaces(namespaces []string, matchFunc func(namespace string, patterns []string) bool) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// nil means no filtering - return all
	if namespaces == nil {
		return len(c.instances)
	}

	// Empty slice means no access
	if len(namespaces) == 0 {
		return 0
	}

	count := 0
	for _, instance := range c.instances {
		if matchFunc(instance.Namespace, namespaces) {
			count++
		}
	}
	return count
}

// CountByRGDAndNamespaces returns the count of instances for a specific RGD
// filtered by the user's accessible namespaces.
// Uses namespace pattern matching (supports wildcards like "team-*").
// If namespaces is nil, returns total count for the RGD (global admin).
// If namespaces is empty slice, returns 0 (no access).
func (c *InstanceCache) CountByRGDAndNamespaces(rgdNamespace, rgdName string, namespaces []string, matchFunc func(namespace string, patterns []string) bool) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Empty slice means no access
	if namespaces != nil && len(namespaces) == 0 {
		return 0
	}

	count := 0
	for _, instance := range c.instances {
		// Must match the RGD
		if instance.RGDName != rgdName || instance.RGDNamespace != rgdNamespace {
			continue
		}
		// nil namespaces means no filtering (global admin) - count all for this RGD
		if namespaces == nil {
			count++
			continue
		}
		// Check namespace access
		if matchFunc(instance.Namespace, namespaces) {
			count++
		}
	}
	return count
}

// All returns all instances in the cache
// Note: Primarily used for testing purposes
func (c *InstanceCache) All() []*models.Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*models.Instance, 0, len(c.instances))
	for _, instance := range c.instances {
		result = append(result, instance)
	}
	return result
}

// Clear removes all instances from the cache
// Note: Primarily used for testing purposes
func (c *InstanceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instances = make(map[string]*models.Instance)
}

// matchesFilter checks if an instance matches the filter options
func (c *InstanceCache) matchesFilter(instance *models.Instance, opts models.InstanceListOptions) bool {
	// Namespace filter
	if opts.Namespace != "" && instance.Namespace != opts.Namespace {
		return false
	}

	// RGD name filter
	if opts.RGDName != "" && instance.RGDName != opts.RGDName {
		return false
	}

	// RGD namespace filter
	if opts.RGDNamespace != "" && instance.RGDNamespace != opts.RGDNamespace {
		return false
	}

	// Health filter
	if opts.Health != "" && instance.Health != opts.Health {
		return false
	}

	// Search filter (case-insensitive contains on name)
	if opts.Search != "" {
		search := strings.ToLower(opts.Search)
		nameMatch := strings.Contains(strings.ToLower(instance.Name), search)
		if !nameMatch {
			return false
		}
	}

	return true
}

// sortInstances sorts the instance slice in place
func (c *InstanceCache) sortInstances(instances []models.Instance, sortBy, sortOrder string) {
	if len(instances) == 0 {
		return
	}

	ascending := sortOrder != "desc"

	sort.SliceStable(instances, func(i, j int) bool {
		var less, equal bool
		switch sortBy {
		case "createdAt":
			less = instances[i].CreatedAt.Before(instances[j].CreatedAt)
			equal = instances[i].CreatedAt.Equal(instances[j].CreatedAt)
		case "updatedAt":
			less = instances[i].UpdatedAt.Before(instances[j].UpdatedAt)
			equal = instances[i].UpdatedAt.Equal(instances[j].UpdatedAt)
		case "health":
			hi := string(instances[i].Health)
			hj := string(instances[j].Health)
			less = hi < hj
			equal = hi == hj
		case "rgdName":
			ri := strings.ToLower(instances[i].RGDName)
			rj := strings.ToLower(instances[j].RGDName)
			less = ri < rj
			equal = ri == rj
		default: // "name" and any unrecognized sort field
			ni := strings.ToLower(instances[i].Name)
			nj := strings.ToLower(instances[j].Name)
			less = ni < nj
			equal = ni == nj
		}

		// Tie-break by namespace/kind/name for deterministic order
		if equal {
			ki := strings.ToLower(instances[i].Namespace + "/" + instances[i].Kind + "/" + instances[i].Name)
			kj := strings.ToLower(instances[j].Namespace + "/" + instances[j].Kind + "/" + instances[j].Name)
			if ascending {
				return ki < kj
			}
			return ki > kj
		}

		if ascending {
			return less
		}
		return !less
	})
}

// DeleteByRGD removes all instances belonging to a specific RGD and returns the deleted instances
func (c *InstanceCache) DeleteByRGD(rgdNamespace, rgdName string) []*models.Instance {
	c.mu.Lock()
	defer c.mu.Unlock()

	var removed []*models.Instance
	for key, instance := range c.instances {
		if instance.RGDName == rgdName && instance.RGDNamespace == rgdNamespace {
			removed = append(removed, instance)
			delete(c.instances, key)
		}
	}
	return removed
}

// GetByRGD returns all instances for a specific RGD
func (c *InstanceCache) GetByRGD(rgdNamespace, rgdName string) []*models.Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*models.Instance
	for _, instance := range c.instances {
		if instance.RGDName == rgdName && instance.RGDNamespace == rgdNamespace {
			result = append(result, instance)
		}
	}
	return result
}
