// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RemoteResourceCache stores watched resources from remote clusters.
// Resources are keyed by clusterRef/namespace/kind/name.
// Thread-safe via sync.RWMutex.
type RemoteResourceCache struct {
	mu sync.RWMutex
	// resources maps "clusterRef/namespace/kind/name" → resource
	resources map[string]*unstructured.Unstructured
	// clusterStatus maps clusterRef → RemoteWatchStatus
	clusterStatus map[string]RemoteWatchStatus
}

// NewRemoteResourceCache creates a new empty cache.
func NewRemoteResourceCache() *RemoteResourceCache {
	return &RemoteResourceCache{
		resources:     make(map[string]*unstructured.Unstructured),
		clusterStatus: make(map[string]RemoteWatchStatus),
	}
}

// resourceKey builds the composite key for a cached resource.
func resourceKey(clusterRef, namespace, kind, name string) string {
	return clusterRef + "/" + namespace + "/" + kind + "/" + name
}

// Add inserts or replaces a resource in the cache.
func (c *RemoteResourceCache) Add(clusterRef, namespace, kind, name string, obj *unstructured.Unstructured) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resources[resourceKey(clusterRef, namespace, kind, name)] = obj
}

// Update is an alias for Add (upsert semantics).
func (c *RemoteResourceCache) Update(clusterRef, namespace, kind, name string, obj *unstructured.Unstructured) {
	c.Add(clusterRef, namespace, kind, name, obj)
}

// Delete removes a resource from the cache.
func (c *RemoteResourceCache) Delete(clusterRef, namespace, kind, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.resources, resourceKey(clusterRef, namespace, kind, name))
}

// List returns all cached resources for a given cluster and kind.
func (c *RemoteResourceCache) List(clusterRef, kind string) []*unstructured.Unstructured {
	c.mu.RLock()
	defer c.mu.RUnlock()

	prefix := clusterRef + "/"
	var result []*unstructured.Unstructured
	for key, obj := range c.resources {
		if strings.HasPrefix(key, prefix) {
			// Extract kind from key: clusterRef/namespace/kind/name
			parts := strings.Split(key, "/")
			if len(parts) == 4 && parts[2] == kind {
				result = append(result, obj)
			}
		}
	}
	return result
}

// ListAll returns all cached resources across all clusters.
func (c *RemoteResourceCache) ListAll() []*unstructured.Unstructured {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*unstructured.Unstructured, 0, len(c.resources))
	for _, obj := range c.resources {
		result = append(result, obj)
	}
	return result
}

// DeleteCluster removes all resources for a given cluster.
func (c *RemoteResourceCache) DeleteCluster(clusterRef string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := clusterRef + "/"
	for key := range c.resources {
		if strings.HasPrefix(key, prefix) {
			delete(c.resources, key)
		}
	}
	delete(c.clusterStatus, clusterRef)
}

// SetClusterStatus sets the watch status for a cluster.
func (c *RemoteResourceCache) SetClusterStatus(clusterRef string, status RemoteWatchStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clusterStatus[clusterRef] = status
}

// GetClusterStatus returns the watch status for a cluster.
func (c *RemoteResourceCache) GetClusterStatus(clusterRef string) RemoteWatchStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clusterStatus[clusterRef]
}

// Count returns the total number of cached resources.
func (c *RemoteResourceCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.resources)
}
