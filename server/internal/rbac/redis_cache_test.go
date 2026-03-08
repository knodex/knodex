// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"log/slog"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/testutil"
)

func newTestRedisCache(t *testing.T) (*RedisAuthorizationCache, *miniredis.Miniredis) {
	t.Helper()
	mr, client := testutil.NewRedis(t)
	cache := NewRedisAuthorizationCache(client, 5*time.Minute, slog.Default())
	t.Cleanup(func() { cache.Stop() })
	return cache, mr
}

func TestRedisCache_GetSet(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	// Set value
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)

	// Get value
	allowed, found := cache.Get("user1", "projects/eng", "get")
	assert.True(t, found, "expected cache hit")
	assert.True(t, allowed, "expected allowed=true")
}

func TestRedisCache_GetSet_Denied(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	// Set denied value
	cache.Set("user1", "projects/secret", "delete", false, 5*time.Second)

	// Get value
	allowed, found := cache.Get("user1", "projects/secret", "delete")
	assert.True(t, found, "expected cache hit")
	assert.False(t, allowed, "expected allowed=false")
}

func TestRedisCache_Miss(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	_, found := cache.Get("user1", "projects/eng", "get")
	assert.False(t, found, "expected cache miss")
}

func TestRedisCache_Delete(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)

	// Verify it's there
	_, found := cache.Get("user1", "projects/eng", "get")
	require.True(t, found)

	// Delete it
	cache.Delete("user1", "projects/eng", "get")

	// Should be gone
	_, found = cache.Get("user1", "projects/eng", "get")
	assert.False(t, found)
}

func TestRedisCache_Clear(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Set("user2", "projects/ops", "delete", false, 5*time.Second)

	cache.Clear()

	_, found1 := cache.Get("user1", "projects/eng", "get")
	_, found2 := cache.Get("user2", "projects/ops", "delete")
	assert.False(t, found1, "expected miss after clear")
	assert.False(t, found2, "expected miss after clear")
}

func TestRedisCache_InvalidateByPrefix(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	// Set entries for different users
	cache.Set("user:alice", "projects/eng", "get", true, 5*time.Second)
	cache.Set("user:alice", "projects/ops", "get", true, 5*time.Second)
	cache.Set("user:bob", "projects/eng", "get", true, 5*time.Second)

	// Invalidate alice's entries
	count := cache.InvalidateByPrefix("user:alice\x00")
	assert.Equal(t, 2, count)

	// Alice's entries should be gone
	_, found := cache.Get("user:alice", "projects/eng", "get")
	assert.False(t, found)

	// Bob's entry should still exist
	_, found = cache.Get("user:bob", "projects/eng", "get")
	assert.True(t, found)
}

func TestRedisCache_Stats(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)

	// Hit
	cache.Get("user1", "projects/eng", "get")

	// Miss
	cache.Get("user2", "projects/eng", "get")

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(300), stats.TTLSeconds)
	assert.InDelta(t, 50.0, stats.HitRate, 0.1)
}

func TestRedisCache_ZeroTTL_NotCached(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, 0)

	_, found := cache.Get("user1", "projects/eng", "get")
	assert.False(t, found, "expected no cache with zero TTL")
}

func TestRedisCache_NegativeTTL_NotCached(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, -1*time.Second)

	_, found := cache.Get("user1", "projects/eng", "get")
	assert.False(t, found, "expected no cache with negative TTL")
}

func TestRedisCache_Expiry(t *testing.T) {
	t.Parallel()

	cache, mr := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, 1*time.Second)

	// Should be found initially
	_, found := cache.Get("user1", "projects/eng", "get")
	require.True(t, found)

	// Fast-forward miniredis time
	mr.FastForward(2 * time.Second)

	// Should be expired
	_, found = cache.Get("user1", "projects/eng", "get")
	assert.False(t, found, "expected cache miss after TTL expiry")
}

func TestRedisCache_NilClient_FallsBackToInMemory(t *testing.T) {
	t.Parallel()

	cache := NewRedisAuthorizationCache(nil, 5*time.Minute, slog.Default())
	defer cache.Stop()

	assert.True(t, cache.IsDegraded(), "expected degraded mode with nil client")

	// Should still work via fallback
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)

	allowed, found := cache.Get("user1", "projects/eng", "get")
	assert.True(t, found, "expected cache hit via fallback")
	assert.True(t, allowed)
}

func TestRedisCache_DegradedMode_OnRedisFailure(t *testing.T) {
	t.Parallel()

	cache, mr := newTestRedisCache(t)

	// Initially not degraded
	assert.False(t, cache.IsDegraded())

	// Set a value while Redis is up
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	_, found := cache.Get("user1", "projects/eng", "get")
	require.True(t, found)

	// Close Redis to simulate failure
	mr.Close()

	// Get should fail and enter degraded mode
	_, found = cache.Get("user1", "projects/eng", "get")
	// The value won't be in fallback since it was stored in Redis
	// But the cache should enter degraded mode
	assert.True(t, cache.IsDegraded(), "expected degraded mode after Redis failure")

	// Writes should now go to fallback
	cache.Set("user2", "projects/ops", "get", true, 5*time.Second)
	allowed, found := cache.Get("user2", "projects/ops", "get")
	assert.True(t, found, "expected cache hit via fallback after degradation")
	assert.True(t, allowed)
}

func TestRedisCache_ResetStats(t *testing.T) {
	t.Parallel()

	cache, _ := newTestRedisCache(t)

	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Get("user1", "projects/eng", "get")
	cache.Get("missing", "x", "y")

	stats := cache.Stats()
	assert.Equal(t, int64(1), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)

	cache.ResetStats()

	stats = cache.Stats()
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestRedisCache_ClearInDegradedMode(t *testing.T) {
	t.Parallel()

	cache := NewRedisAuthorizationCache(nil, 5*time.Minute, slog.Default())
	defer cache.Stop()

	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Clear()

	_, found := cache.Get("user1", "projects/eng", "get")
	assert.False(t, found, "expected miss after clear in degraded mode")
}

func TestRedisCache_DeleteInDegradedMode(t *testing.T) {
	t.Parallel()

	cache := NewRedisAuthorizationCache(nil, 5*time.Minute, slog.Default())
	defer cache.Stop()

	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Delete("user1", "projects/eng", "get")

	_, found := cache.Get("user1", "projects/eng", "get")
	assert.False(t, found, "expected miss after delete in degraded mode")
}

func TestRedisCache_InvalidateByPrefixInDegradedMode(t *testing.T) {
	t.Parallel()

	cache := NewRedisAuthorizationCache(nil, 5*time.Minute, slog.Default())
	defer cache.Stop()

	cache.Set("user:alice", "projects/eng", "get", true, 5*time.Second)
	cache.Set("user:bob", "projects/eng", "get", true, 5*time.Second)

	count := cache.InvalidateByPrefix("user:alice\x00")
	assert.Equal(t, 1, count) // In-memory cache uses key format with null byte

	_, found := cache.Get("user:alice", "projects/eng", "get")
	assert.False(t, found)

	_, found = cache.Get("user:bob", "projects/eng", "get")
	assert.True(t, found)
}
