// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// redisCacheKeyPrefix is the Redis key prefix for cached authorization decisions.
	// Key pattern: authz:cache:{user}\x00{object}\x00{action}
	redisCacheKeyPrefix = "authz:cache:"

	// redisCacheInvalidateChannel is the Redis pub/sub channel for cache invalidation.
	// When policies change on any replica, a message is published to this channel
	// so all replicas clear their local Redis-backed cache entries.
	redisCacheInvalidateChannel = "authz:invalidate"

	// redisCacheScanCount is the batch size for SCAN operations during cache operations.
	redisCacheScanCount = 100

	// fallbackCacheTTL is the reduced TTL used when Redis is unavailable.
	// This is shorter than DefaultCacheTTL (5min) to reduce staleness during degraded mode.
	fallbackCacheTTL = 30 * time.Second

	// cacheSizeRefreshInterval is how often the estimated cache size is refreshed.
	// Avoids blocking Stats() callers with a full SCAN on every call.
	cacheSizeRefreshInterval = 30 * time.Second
)

// RedisAuthorizationCache implements AuthorizationCache using Redis.
// It provides cross-replica cache consistency via Redis pub/sub invalidation.
// When Redis is unavailable, it gracefully falls back to an in-memory cache
// with a reduced TTL (30s instead of 5min).
type RedisAuthorizationCache struct {
	client *redis.Client
	ttl    time.Duration
	logger *slog.Logger

	// Metrics
	hits   int64
	misses int64

	// Fallback in-memory cache for when Redis is unavailable
	fallback *inMemoryCache
	mu       sync.RWMutex
	degraded bool // true when operating in fallback mode

	// Cached size estimate to avoid blocking Stats() with a full SCAN
	cachedSize     int
	cachedSizeTime time.Time

	// Pub/sub for cross-replica invalidation
	pubsub *redis.PubSub
	stopCh chan struct{}

	// stopOnce ensures the stop channel is only closed once
	stopOnce sync.Once
}

// NewRedisAuthorizationCache creates a new Redis-backed authorization cache.
// If the Redis client is nil, it immediately falls back to in-memory caching.
func NewRedisAuthorizationCache(client *redis.Client, ttl time.Duration, logger *slog.Logger) *RedisAuthorizationCache {
	if logger == nil {
		logger = slog.Default()
	}

	cache := &RedisAuthorizationCache{
		client: client,
		ttl:    ttl,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	// Create fallback in-memory cache with reduced TTL
	cache.fallback = &inMemoryCache{
		ttl:    fallbackCacheTTL,
		stopCh: make(chan struct{}),
	}
	go cache.fallback.cleanupLoop()

	if client == nil {
		cache.degraded = true
		logger.Warn("redis authorization cache: no Redis client, using in-memory fallback",
			"fallback_ttl", fallbackCacheTTL.String(),
		)
		return cache
	}

	// Start pub/sub listener for cross-replica invalidation
	go cache.subscribeInvalidation()

	return cache
}

// Get retrieves a cached authorization decision from Redis.
// Falls back to in-memory cache if Redis is unavailable.
func (c *RedisAuthorizationCache) Get(user, object, action string) (bool, bool) {
	c.mu.RLock()
	degraded := c.degraded
	c.mu.RUnlock()

	if degraded {
		return c.fallback.Get(user, object, action)
	}

	key := redisCacheKeyPrefix + cacheKey(user, object, action)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			atomic.AddInt64(&c.misses, 1)
			return false, false
		}
		// Redis error - switch to degraded mode
		c.enterDegradedMode()
		atomic.AddInt64(&c.misses, 1)
		return false, false
	}

	atomic.AddInt64(&c.hits, 1)
	return val == "1", true
}

// Set stores an authorization decision in Redis with TTL.
// Falls back to in-memory cache if Redis is unavailable.
func (c *RedisAuthorizationCache) Set(user, object, action string, allowed bool, ttl time.Duration) {
	// SECURITY: Don't cache with zero or negative TTL
	if ttl <= 0 {
		return
	}

	c.mu.RLock()
	degraded := c.degraded
	c.mu.RUnlock()

	if degraded {
		c.fallback.Set(user, object, action, allowed, fallbackCacheTTL)
		return
	}

	key := redisCacheKeyPrefix + cacheKey(user, object, action)
	val := "0"
	if allowed {
		val = "1"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := c.client.Set(ctx, key, val, ttl).Err(); err != nil {
		c.enterDegradedMode()
		// Store in fallback
		c.fallback.Set(user, object, action, allowed, fallbackCacheTTL)
	}
}

// Delete removes a cached decision from Redis.
func (c *RedisAuthorizationCache) Delete(user, object, action string) {
	c.mu.RLock()
	degraded := c.degraded
	c.mu.RUnlock()

	if degraded {
		c.fallback.Delete(user, object, action)
		return
	}

	key := redisCacheKeyPrefix + cacheKey(user, object, action)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := c.client.Del(ctx, key).Err(); err != nil {
		c.logger.Warn("redis cache: failed to delete key", "error", err)
	}
}

// Clear removes all cached decisions from Redis and publishes an invalidation event
// so all replicas clear their caches too.
func (c *RedisAuthorizationCache) Clear() {
	c.mu.RLock()
	degraded := c.degraded
	c.mu.RUnlock()

	if degraded {
		c.fallback.Clear()
		return
	}

	c.clearRedisCache()
	c.publishInvalidation("clear:*")
}

// clearRedisCache removes all authz cache keys from Redis using SCAN.
func (c *RedisAuthorizationCache) clearRedisCache() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cursor uint64
	pattern := redisCacheKeyPrefix + "*"
	deleted := 0

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, redisCacheScanCount).Result()
		if err != nil {
			c.logger.Warn("redis cache: failed to scan for cache keys", "error", err)
			return
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				c.logger.Warn("redis cache: failed to delete cache keys", "error", err)
			} else {
				deleted += len(keys)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if deleted > 0 {
		c.logger.Debug("redis cache: cleared entries", "count", deleted)
	}
}

// InvalidateByPrefix removes all cache entries matching the prefix.
// Returns the number of entries removed.
func (c *RedisAuthorizationCache) InvalidateByPrefix(prefix string) int {
	c.mu.RLock()
	degraded := c.degraded
	c.mu.RUnlock()

	if degraded {
		return c.fallback.InvalidateByPrefix(prefix)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cursor uint64
	pattern := redisCacheKeyPrefix + prefix + "*"
	deleted := 0

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, redisCacheScanCount).Result()
		if err != nil {
			c.logger.Warn("redis cache: failed to scan for prefix invalidation",
				"prefix", prefix,
				"error", err,
			)
			return deleted
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				c.logger.Warn("redis cache: failed to delete keys by prefix",
					"prefix", prefix,
					"error", err,
				)
			} else {
				deleted += len(keys)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	// Publish invalidation for other replicas
	c.publishInvalidation("prefix:" + prefix)

	return deleted
}

// Stats returns cache performance metrics.
func (c *RedisAuthorizationCache) Stats() CacheStats {
	c.mu.RLock()
	degraded := c.degraded
	c.mu.RUnlock()

	if degraded {
		stats := c.fallback.Stats()
		stats.TTLSeconds = int64(fallbackCacheTTL.Seconds())
		return stats
	}

	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)

	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	// Get approximate size via SCAN (limited to avoid blocking)
	size := c.estimateCacheSize()

	return CacheStats{
		Hits:       hits,
		Misses:     misses,
		Size:       size,
		HitRate:    hitRate,
		TTLSeconds: int64(c.ttl.Seconds()),
	}
}

// estimateCacheSize returns the count of cached entries.
// Uses a cached value refreshed every 30s to avoid blocking callers with a full SCAN.
func (c *RedisAuthorizationCache) estimateCacheSize() int {
	c.mu.RLock()
	if time.Since(c.cachedSizeTime) < cacheSizeRefreshInterval {
		size := c.cachedSize
		c.mu.RUnlock()
		return size
	}
	c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var count int
	var cursor uint64
	pattern := redisCacheKeyPrefix + "*"

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, redisCacheScanCount).Result()
		if err != nil {
			return count
		}
		count += len(keys)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	c.mu.Lock()
	c.cachedSize = count
	c.cachedSizeTime = time.Now()
	c.mu.Unlock()

	return count
}

// Stop gracefully stops the background pub/sub listener and fallback cache.
func (c *RedisAuthorizationCache) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})

	c.mu.RLock()
	pubsub := c.pubsub
	c.mu.RUnlock()

	if pubsub != nil {
		if err := pubsub.Close(); err != nil {
			c.logger.Warn("redis cache: failed to close pub/sub", "error", err)
		}
	}

	c.fallback.Stop()
}

// publishInvalidation sends a cache invalidation message to all replicas.
func (c *RedisAuthorizationCache) publishInvalidation(message string) {
	if c.client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := c.client.Publish(ctx, redisCacheInvalidateChannel, message).Err(); err != nil {
		c.logger.Warn("redis cache: failed to publish invalidation",
			"message", message,
			"error", err,
		)
	}
}

// subscribeInvalidation listens for cache invalidation messages from other replicas.
func (c *RedisAuthorizationCache) subscribeInvalidation() {
	if c.client == nil {
		return
	}

	pubsub := c.client.Subscribe(context.Background(), redisCacheInvalidateChannel)

	c.mu.Lock()
	c.pubsub = pubsub
	c.mu.Unlock()

	ch := pubsub.Channel()
	for {
		select {
		case <-c.stopCh:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.handleInvalidationMessage(msg.Payload)
		}
	}
}

// handleInvalidationMessage processes invalidation messages from other replicas.
// Since all replicas share the same Redis instance, key deletions by the originating
// replica are immediately visible to all others. The pub/sub message serves as a
// notification — no redundant SCAN+DEL is needed on receiving replicas.
func (c *RedisAuthorizationCache) handleInvalidationMessage(payload string) {
	switch {
	case payload == "clear:*":
		c.logger.Debug("redis cache: received cross-replica full invalidation")

	case len(payload) > 7 && payload[:7] == "prefix:":
		prefix := payload[7:]
		c.logger.Debug("redis cache: received cross-replica prefix invalidation",
			"prefix", prefix,
		)

	default:
		c.logger.Warn("redis cache: received unknown invalidation message format",
			"payload", payload,
		)
	}
}

// enterDegradedMode switches to in-memory fallback when Redis is unavailable.
func (c *RedisAuthorizationCache) enterDegradedMode() {
	c.mu.Lock()
	alreadyDegraded := c.degraded
	if !c.degraded {
		c.degraded = true
		c.logger.Warn("redis authorization cache: entering degraded mode, using in-memory fallback",
			"fallback_ttl", fallbackCacheTTL.String(),
		)
	}
	c.mu.Unlock()

	// Only start one recovery goroutine (on the first transition into degraded mode)
	if !alreadyDegraded {
		go c.tryRecovery()
	}
}

// tryRecovery periodically attempts to reconnect to Redis and exit degraded mode.
func (c *RedisAuthorizationCache) tryRecovery() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := c.client.Ping(ctx).Err()
			cancel()

			if err == nil {
				c.mu.Lock()
				c.degraded = false
				c.mu.Unlock()

				// Clear fallback cache since Redis is back
				c.fallback.Clear()

				c.logger.Info("redis authorization cache: recovered from degraded mode")
				return
			}
		}
	}
}

// ResetStats resets hit/miss counters (useful for testing)
func (c *RedisAuthorizationCache) ResetStats() {
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

// IsDegraded returns whether the cache is operating in fallback mode.
func (c *RedisAuthorizationCache) IsDegraded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.degraded
}

// Compile-time interface check
var _ AuthorizationCache = (*RedisAuthorizationCache)(nil)

// WithRedisAuthorizationCache sets the authorization cache to use Redis-backed implementation
func WithRedisAuthorizationCache(cache *RedisAuthorizationCache) PolicyEnforcerOption {
	return func(pe *policyEnforcer) {
		pe.cache = cache
		pe.cacheEnabled = true
		pe.logger.Info("using Redis-backed authorization cache")
	}
}
