package rbac

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CacheEntry represents a cached authorization decision
type CacheEntry struct {
	Allowed   bool
	ExpiresAt time.Time
}

// IsExpired checks if cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// AuthorizationCache caches authorization decisions
type AuthorizationCache interface {
	// Get retrieves a cached authorization decision
	// Returns (allowed, found) where found indicates cache hit
	Get(user, object, action string) (bool, bool)

	// Set stores an authorization decision in cache with TTL
	Set(user, object, action string, allowed bool, ttl time.Duration)

	// Delete removes a cached decision
	Delete(user, object, action string)

	// Clear removes all cached decisions
	Clear()

	// InvalidateByPrefix removes all cache entries matching the prefix
	// Returns the number of entries removed
	// Example prefixes: "user:alice:" invalidates all entries for user alice
	InvalidateByPrefix(prefix string) int

	// Stats returns cache performance metrics
	Stats() CacheStats
}

// CacheStats holds cache performance metrics
type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Size       int     `json:"size"`
	HitRate    float64 `json:"hit_rate"`
	TTLSeconds int64   `json:"ttl_seconds"`
}

// DefaultCacheTTL is the default TTL for cache entries (5 minutes)
const DefaultCacheTTL = 5 * time.Minute

// inMemoryCache implements AuthorizationCache using sync.Map
type inMemoryCache struct {
	data   sync.Map
	hits   int64 // atomic counter for thread safety
	misses int64 // atomic counter for thread safety
	ttl    time.Duration
	stopCh chan struct{}

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// NewAuthorizationCache creates a new in-memory cache with default TTL
func NewAuthorizationCache() AuthorizationCache {
	return NewAuthorizationCacheWithTTL(DefaultCacheTTL)
}

// NewAuthorizationCacheWithTTL creates a new in-memory cache with custom TTL
func NewAuthorizationCacheWithTTL(ttl time.Duration) AuthorizationCache {
	cache := &inMemoryCache{
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cache.cleanupLoop()

	return cache
}

// NewAuthorizationCacheWithCleanupInterval creates cache with custom cleanup interval
// This is primarily for testing to avoid long waits
func NewAuthorizationCacheWithCleanupInterval(cleanupInterval time.Duration) *inMemoryCache {
	cache := &inMemoryCache{
		ttl:    DefaultCacheTTL,
		stopCh: make(chan struct{}),
	}

	// Start background cleanup goroutine with custom interval
	go cache.cleanupLoopWithInterval(cleanupInterval)

	return cache
}

// Get retrieves a cached authorization decision
func (c *inMemoryCache) Get(user, object, action string) (bool, bool) {
	key := cacheKey(user, object, action)

	value, ok := c.data.Load(key)
	if !ok {
		atomic.AddInt64(&c.misses, 1)
		return false, false
	}

	entry := value.(*CacheEntry)
	if entry.IsExpired() {
		c.data.Delete(key)
		atomic.AddInt64(&c.misses, 1)
		return false, false
	}

	atomic.AddInt64(&c.hits, 1)
	return entry.Allowed, true
}

// Set stores an authorization decision in cache
func (c *inMemoryCache) Set(user, object, action string, allowed bool, ttl time.Duration) {
	// SECURITY: Don't cache with zero or negative TTL
	if ttl <= 0 {
		return
	}

	key := cacheKey(user, object, action)
	entry := &CacheEntry{
		Allowed:   allowed,
		ExpiresAt: time.Now().Add(ttl),
	}
	c.data.Store(key, entry)
}

// Delete removes a cached decision
func (c *inMemoryCache) Delete(user, object, action string) {
	key := cacheKey(user, object, action)
	c.data.Delete(key)
}

// Clear removes all cached decisions
func (c *inMemoryCache) Clear() {
	c.data.Range(func(key, value interface{}) bool {
		c.data.Delete(key)
		return true
	})
}

// InvalidateByPrefix removes all cache entries with keys starting with the given prefix
// Returns the number of entries removed
// Cache keys use null byte separator (see cacheKey()), so prefix should end with \x00:
//   - InvalidateByPrefix("user:alice\x00") removes all cached decisions for user alice
//   - InvalidateByPrefix("project:default\x00") removes all cached decisions for project default
func (c *inMemoryCache) InvalidateByPrefix(prefix string) int {
	count := 0
	c.data.Range(func(key, value interface{}) bool {
		keyStr, ok := key.(string)
		if ok && strings.HasPrefix(keyStr, prefix) {
			c.data.Delete(key)
			count++
		}
		return true
	})
	return count
}

// Stats returns cache performance metrics
func (c *inMemoryCache) Stats() CacheStats {
	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)

	size := 0
	c.data.Range(func(_, _ interface{}) bool {
		size++
		return true
	})

	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:       hits,
		Misses:     misses,
		Size:       size,
		HitRate:    hitRate,
		TTLSeconds: int64(c.ttl.Seconds()),
	}
}

// Stop gracefully stops the background cleanup goroutine
// Safe to call multiple times - uses sync.Once to prevent double-close panic
func (c *inMemoryCache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
}

// cleanupLoop periodically removes expired entries
func (c *inMemoryCache) cleanupLoop() {
	c.cleanupLoopWithInterval(1 * time.Minute)
}

// cleanupLoopWithInterval runs cleanup with custom interval
func (c *inMemoryCache) cleanupLoopWithInterval(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes expired entries from cache
func (c *inMemoryCache) cleanup() {
	c.data.Range(func(key, value interface{}) bool {
		entry := value.(*CacheEntry)
		if entry.IsExpired() {
			c.data.Delete(key)
		}
		return true
	})
}

// cacheKey generates a cache key from user, object, action using a null byte
// separator that cannot appear in HTTP request parameters, preventing key
// collisions when user IDs or objects contain colons.
func cacheKey(user, object, action string) string {
	return user + "\x00" + object + "\x00" + action
}

// ResetStats resets hit/miss counters (useful for testing)
func (c *inMemoryCache) ResetStats() {
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}
