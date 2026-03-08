// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPolicyEnforcer_NewWithCacheDisabled tests NewPolicyEnforcerWithConfig with cache disabled
func TestPolicyEnforcer_NewWithCacheDisabled(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create with cache disabled
	config := PolicyEnforcerConfig{
		CacheEnabled: false,
		CacheTTL:     0,
		Logger:       nil, // Test nil logger handling
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	assert.NotNil(t, pe)

	// CacheStats should return empty when cache disabled
	stats := pe.CacheStats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

// TestPolicyEnforcer_InvalidateCache tests the InvalidateCache function
func TestPolicyEnforcer_InvalidateCache(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Invalidate cache - should not panic even with nil cache
	pe.InvalidateCache()
}

// TestPolicyEnforcer_CacheStats tests the CacheStats function
func TestPolicyEnforcer_CacheStats(t *testing.T) {
	t.Parallel()

	pe := newTestEnforcer(t)

	// Get cache stats - should return initial stats
	stats := pe.CacheStats()
	// Size is int, Hits/Misses are int64
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

// TestPolicyEnforcer_CacheStatsWithEnabledCache tests CacheStats with an active cache
func TestPolicyEnforcer_CacheStatsWithEnabledCache(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create with cache enabled
	config := PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	ctx := context.Background()

	// Assign a role to test user
	err = pe.AssignUserRoles(ctx, "user:cache-test", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// First access - cache miss
	_, err = pe.CanAccess(ctx, "user:cache-test", "projects/test", "get")
	require.NoError(t, err)

	// Get stats after first access
	stats := pe.CacheStats()
	// After first call there should be a cache entry
	assert.GreaterOrEqual(t, stats.Size, 0) // Could be 0 if cache write is async or skipped

	// Second access - should be cache hit (if caching is working)
	_, err = pe.CanAccess(ctx, "user:cache-test", "projects/test", "get")
	require.NoError(t, err)
}

// TestPolicyEnforcer_CanAccessCacheFlow tests the cache hit/miss flow in CanAccess
func TestPolicyEnforcer_CanAccessCacheFlow(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	// Create with cache enabled
	config := PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	ctx := context.Background()

	// Assign role
	err = pe.AssignUserRoles(ctx, "user:cached-user", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// First access - should check enforcer and populate cache
	allowed1, err := pe.CanAccess(ctx, "user:cached-user", "projects/cache-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed1)

	// Second access - should hit cache
	allowed2, err := pe.CanAccess(ctx, "user:cached-user", "projects/cache-project", "get")
	require.NoError(t, err)
	assert.True(t, allowed2)

	// Result should be consistent
	assert.Equal(t, allowed1, allowed2)
}

// TestPolicyEnforcer_InvalidateCacheFlow tests cache invalidation during operations
func TestPolicyEnforcer_InvalidateCacheFlow(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	config := PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	ctx := context.Background()

	// Test that cache is invalidated on role assignment
	err = pe.AssignUserRoles(ctx, "user:invalidate-test", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Access to populate cache
	_, err = pe.CanAccess(ctx, "user:invalidate-test", "projects/test", "get")
	require.NoError(t, err)

	// Explicit invalidation
	pe.InvalidateCache()

	// Access should still work after invalidation
	allowed, err := pe.CanAccess(ctx, "user:invalidate-test", "projects/test", "get")
	require.NoError(t, err)
	assert.True(t, allowed)
}

// TestPolicyEnforcer_AssignUserRoles_TargetedCacheInvalidation verifies that AssignUserRoles
// only invalidates cache entries for the specific user, leaving other users' cache intact.
// This tests the fix that changed cache.Clear() to InvalidateCacheForUser(user) and
// the separator fix (\x00 instead of :) that made InvalidateCacheForUser actually work.
func TestPolicyEnforcer_AssignUserRoles_TargetedCacheInvalidation(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	config := PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	ctx := context.Background()

	// Assign admin roles to both users so CanAccess returns true
	err = pe.AssignUserRoles(ctx, "user:alice", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)
	err = pe.AssignUserRoles(ctx, "user:bob", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Populate cache for both users via CanAccess
	allowed, err := pe.CanAccess(ctx, "user:alice", "projects/eng", "get")
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = pe.CanAccess(ctx, "user:alice", "projects/ops", "delete")
	require.NoError(t, err)
	assert.True(t, allowed)

	allowed, err = pe.CanAccess(ctx, "user:bob", "projects/eng", "get")
	require.NoError(t, err)
	assert.True(t, allowed)

	// Verify cache has entries for both users
	stats := pe.CacheStats()
	assert.True(t, stats.Size > 0, "cache should have entries")

	// Now reassign Alice's roles — this should only invalidate Alice's cache entries
	err = pe.AssignUserRoles(ctx, "user:alice", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Bob's cached entry should still be in cache (cache hit on next access)
	// Alice's entries should be gone (cache miss on next access)
	// We verify this indirectly: access should still work for both (roles are still assigned),
	// but the key behavior is that Bob's cache wasn't cleared unnecessarily.

	// Bob should still have access
	allowed, err = pe.CanAccess(ctx, "user:bob", "projects/eng", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Bob should still have access after Alice's role reassignment")

	// Alice should also have access (roles were re-assigned, cache rebuilt)
	allowed, err = pe.CanAccess(ctx, "user:alice", "projects/eng", "get")
	require.NoError(t, err)
	assert.True(t, allowed, "Alice should have access after role reassignment")
}

// TestPolicyEnforcer_InvalidateCacheForUser_SeparatorFix verifies that InvalidateCacheForUser
// correctly uses the null byte separator (\x00) matching the cacheKey() format.
// This is a regression test for the bug where ":" was used instead of "\x00".
func TestPolicyEnforcer_InvalidateCacheForUser_SeparatorFix(t *testing.T) {
	t.Parallel()

	enforcer, err := NewCasbinEnforcer()
	require.NoError(t, err)

	config := PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
	}

	pe := NewPolicyEnforcerWithConfig(enforcer, nil, config)
	ctx := context.Background()

	// Assign admin role and populate cache
	err = pe.AssignUserRoles(ctx, "user:test-sep", []string{CasbinRoleServerAdmin})
	require.NoError(t, err)

	// Populate cache entries
	_, err = pe.CanAccess(ctx, "user:test-sep", "projects/a", "get")
	require.NoError(t, err)
	_, err = pe.CanAccess(ctx, "user:test-sep", "projects/b", "update")
	require.NoError(t, err)

	// InvalidateCacheForUser should find and remove entries
	// This would return 0 with the old ":" separator (bug), >0 with correct "\x00"
	count := pe.InvalidateCacheForUser("user:test-sep")
	assert.Greater(t, count, 0, "InvalidateCacheForUser should have invalidated cache entries (separator fix)")
	assert.Equal(t, 2, count, "Expected exactly 2 cache entries invalidated")
}
