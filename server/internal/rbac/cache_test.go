// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"sync"
	"testing"
	"time"
)

func TestNewAuthorizationCache(t *testing.T) {
	cache := NewAuthorizationCache()
	if cache == nil {
		t.Fatal("expected cache to be created")
	}

	// Clean up
	if c, ok := cache.(*inMemoryCache); ok {
		c.Stop()
	}
}

func TestCache_GetSet(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour) // Long interval for tests
	defer cache.Stop()

	// Set value
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)

	// Get value
	allowed, found := cache.Get("user1", "projects/eng", "get")
	if !found {
		t.Error("expected cache hit, got miss")
	}
	if !allowed {
		t.Error("expected allowed=true, got false")
	}
}

func TestCache_GetSet_Denied(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Set denied value
	cache.Set("user1", "projects/secret", "delete", false, 5*time.Second)

	// Get value
	allowed, found := cache.Get("user1", "projects/secret", "delete")
	if !found {
		t.Error("expected cache hit, got miss")
	}
	if allowed {
		t.Error("expected allowed=false, got true")
	}
}

func TestCache_Miss(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	_, found := cache.Get("user1", "projects/eng", "get")
	if found {
		t.Error("expected cache miss, got hit")
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Set with very short TTL
	cache.Set("user1", "projects/eng", "get", true, 100*time.Millisecond)

	// Should be cached immediately
	_, found := cache.Get("user1", "projects/eng", "get")
	if !found {
		t.Error("expected cache hit immediately after set")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired
	_, found = cache.Get("user1", "projects/eng", "get")
	if found {
		t.Error("expected cache miss after TTL expiration")
	}
}

func TestCache_ZeroTTL(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Set with zero TTL should not cache
	cache.Set("user1", "projects/eng", "get", true, 0)

	// Should not be cached
	_, found := cache.Get("user1", "projects/eng", "get")
	if found {
		t.Error("expected cache miss for zero TTL")
	}
}

func TestCache_NegativeTTL(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Set with negative TTL should not cache
	cache.Set("user1", "projects/eng", "get", true, -1*time.Second)

	// Should not be cached
	_, found := cache.Get("user1", "projects/eng", "get")
	if found {
		t.Error("expected cache miss for negative TTL")
	}
}

func TestCache_Delete(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Set value
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)

	// Verify it exists
	_, found := cache.Get("user1", "projects/eng", "get")
	if !found {
		t.Fatal("expected cache hit before delete")
	}

	// Delete it
	cache.Delete("user1", "projects/eng", "get")

	// Verify it's gone
	_, found = cache.Get("user1", "projects/eng", "get")
	if found {
		t.Error("expected cache miss after delete")
	}
}

func TestCache_Clear(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Set multiple values
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Set("user2", "projects/ops", "create", false, 5*time.Second)
	cache.Set("user3", "instances/*", "delete", true, 5*time.Second)

	// Clear cache
	cache.Clear()

	// All should be missing
	_, found1 := cache.Get("user1", "projects/eng", "get")
	_, found2 := cache.Get("user2", "projects/ops", "create")
	_, found3 := cache.Get("user3", "instances/*", "delete")

	if found1 || found2 || found3 {
		t.Error("expected all entries cleared")
	}
}

func TestCache_Stats(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Reset stats for clean test
	cache.ResetStats()

	// Generate hits and misses
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Get("user1", "projects/eng", "get") // hit
	cache.Get("user1", "projects/eng", "get") // hit
	cache.Get("user2", "projects/ops", "get") // miss

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("expected size 1, got %d", stats.Size)
	}
	// Hit rate should be ~66.67%
	if stats.HitRate < 66 || stats.HitRate > 67 {
		t.Errorf("expected hit rate ~66%%, got %.2f%%", stats.HitRate)
	}
}

func TestCache_StatsZeroRequests(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("expected zero hits and misses for new cache")
	}
	if stats.HitRate != 0 {
		t.Errorf("expected hit rate 0%% for no requests, got %.2f%%", stats.HitRate)
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Set("user", "object", "action", true, 5*time.Second)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Get("user", "object", "action")
			}
		}(i)
	}
	wg.Wait()

	// Mixed reads and writes
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Set("user", "object", "action", true, 5*time.Second)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cache.Get("user", "object", "action")
			}
		}(i)
	}
	wg.Wait()

	// Test passed if no data races or panics
}

func TestCache_ConcurrentClear(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	var wg sync.WaitGroup

	// Concurrent clears while writing
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			cache.Set("user", "object", "action", true, 5*time.Second)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			cache.Clear()
			time.Sleep(1 * time.Millisecond)
		}
	}()
	wg.Wait()

	// Test passed if no data races or panics
}

func TestCache_ResetStats(t *testing.T) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	// Generate some stats
	cache.Set("user1", "projects/eng", "get", true, 5*time.Second)
	cache.Get("user1", "projects/eng", "get")
	cache.Get("user2", "projects/ops", "get")

	// Verify stats exist
	stats := cache.Stats()
	if stats.Hits == 0 && stats.Misses == 0 {
		t.Fatal("expected stats before reset")
	}

	// Reset
	cache.ResetStats()

	// Verify reset
	stats = cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Errorf("expected zero stats after reset, got hits=%d misses=%d", stats.Hits, stats.Misses)
	}
}

func TestCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		object   string
		action   string
		expected string
	}{
		{"basic", "user1", "projects/eng", "get", "user1\x00projects/eng\x00get"},
		{"user with colon", "user:john@example.com", "instances/team-a/*", "delete", "user:john@example.com\x00instances/team-a/*\x00delete"},
		{"group prefix", "group:admins", "users/*", "create", "group:admins\x00users/*\x00create"},
		{"empty strings", "", "", "", "\x00\x00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cacheKey(tt.user, tt.object, tt.action)
			if result != tt.expected {
				t.Errorf("cacheKey(%q, %q, %q) = %q; want %q",
					tt.user, tt.object, tt.action, result, tt.expected)
			}
		})
	}
}

// TestCacheKey_NoCollision verifies that the null-byte separator prevents
// key collisions when user IDs or objects contain colons (STORY-236 AC-7).
func TestCacheKey_NoCollision(t *testing.T) {
	// With the old colon separator, these would produce the same key
	key1 := cacheKey("user:a", "b", "get")
	key2 := cacheKey("user", "a:b", "get")
	if key1 == key2 {
		t.Errorf("cache key collision: %q == %q", key1, key2)
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired - future",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  false,
		},
		{
			name:      "expired - past",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
		{
			name:      "just expired - past by millisecond",
			expiresAt: time.Now().Add(-1 * time.Millisecond),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &CacheEntry{
				Allowed:   true,
				ExpiresAt: tt.expiresAt,
			}
			if got := entry.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v; want %v", got, tt.expected)
			}
		})
	}
}

func TestCache_Cleanup(t *testing.T) {
	// Create cache with very short cleanup interval for testing
	cache := NewAuthorizationCacheWithCleanupInterval(50 * time.Millisecond)
	defer cache.Stop()

	// Add entry with short TTL
	cache.Set("user1", "projects/eng", "get", true, 25*time.Millisecond)

	// Verify it exists
	_, found := cache.Get("user1", "projects/eng", "get")
	if !found {
		t.Fatal("expected entry to exist initially")
	}

	// Wait for expiration + cleanup cycle
	time.Sleep(100 * time.Millisecond)

	// Entry should be cleaned up
	stats := cache.Stats()
	if stats.Size > 0 {
		t.Errorf("expected cache to be empty after cleanup, got size=%d", stats.Size)
	}
}

// Benchmark tests

func BenchmarkCache_Get_Hit(b *testing.B) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	cache.Set("user1", "projects/eng", "get", true, 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("user1", "projects/eng", "get")
	}
}

func BenchmarkCache_Get_Miss(b *testing.B) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get("user1", "projects/eng", "get")
	}
}

func BenchmarkCache_Set(b *testing.B) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set("user1", "projects/eng", "get", true, 5*time.Minute)
	}
}

func BenchmarkCache_ConcurrentGetSet(b *testing.B) {
	cache := NewAuthorizationCacheWithCleanupInterval(1 * time.Hour)
	defer cache.Stop()

	cache.Set("user1", "projects/eng", "get", true, 1*time.Hour)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get("user1", "projects/eng", "get")
			cache.Set("user2", "projects/ops", "create", false, 5*time.Minute)
		}
	})
}
