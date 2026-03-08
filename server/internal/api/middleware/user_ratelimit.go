// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/knodex/knodex/server/internal/api/response"
)

// UserRateLimitConfig holds configuration for per-user rate limiting
type UserRateLimitConfig struct {
	// RequestsPerMinute is the number of requests allowed per minute per user
	RequestsPerMinute int
	// BurstSize is the burst size for the rate limiter
	BurstSize int
	// FallbackToIP if true, uses IP-based rate limiting for unauthenticated requests
	FallbackToIP bool
	// TrustedProxies is a list of trusted proxy IP addresses or CIDR ranges
	TrustedProxies []string
	// RetryAfterSeconds is the value for the Retry-After header on 429 responses.
	// If 0, defaults to 60 seconds. Set to match the expected wait time for the rate limit.
	RetryAfterSeconds int
}

// UserRateLimiter manages rate limiters for different users
type UserRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   UserRateLimitConfig
	// IP-based limiter for unauthenticated requests (if FallbackToIP is true)
	ipLimiter   *IPRateLimiter
	stopCleanup chan struct{}
}

// NewUserRateLimiter creates a new user rate limiter
func NewUserRateLimiter(config UserRateLimitConfig) *UserRateLimiter {
	var ipLimiter *IPRateLimiter
	if config.FallbackToIP {
		ipLimiter = NewIPRateLimiter(RateLimitConfig{
			RequestsPerMinute: config.RequestsPerMinute,
			BurstSize:         config.BurstSize,
			TrustedProxies:    config.TrustedProxies,
		})
	}

	rl := &UserRateLimiter{
		limiters:    make(map[string]*rate.Limiter),
		config:      config,
		ipLimiter:   ipLimiter,
		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.periodicCleanup()

	return rl
}

// GetLimiter returns the rate limiter for the given user ID
func (u *UserRateLimiter) GetLimiter(userID string) *rate.Limiter {
	// First check with read lock for existing limiter
	u.mu.RLock()
	limiter, exists := u.limiters[userID]
	u.mu.RUnlock()

	if exists {
		return limiter
	}

	// Need to create new limiter, acquire write lock
	u.mu.Lock()
	defer u.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have created it)
	if limiter, exists := u.limiters[userID]; exists {
		return limiter
	}

	// Create a new rate limiter
	// rate.Limit is requests per second, so divide by 60 for per-minute rate
	limiter = rate.NewLimiter(rate.Limit(float64(u.config.RequestsPerMinute)/60.0), u.config.BurstSize)
	u.limiters[userID] = limiter

	return limiter
}

// periodicCleanup runs in the background and periodically removes inactive limiters
func (u *UserRateLimiter) periodicCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Snapshot current limiters while holding lock
			u.mu.Lock()
			snapshot := make(map[string]*rate.Limiter, len(u.limiters))
			for userID, limiter := range u.limiters {
				snapshot[userID] = limiter
			}
			u.mu.Unlock()

			// Check tokens outside of lock (Tokens() is thread-safe)
			toDelete := make([]string, 0)
			for userID, limiter := range snapshot {
				// Only delete if completely unused (all tokens available)
				// Add small epsilon to avoid floating point comparison issues
				if limiter.Tokens() >= float64(u.config.BurstSize)-0.001 {
					toDelete = append(toDelete, userID)
				}
			}

			// Delete identified limiters while holding lock
			if len(toDelete) > 0 {
				u.mu.Lock()
				for _, userID := range toDelete {
					// Recheck before deletion (limiter might have been used)
					if limiter, exists := u.limiters[userID]; exists {
						if limiter.Tokens() >= float64(u.config.BurstSize)-0.001 {
							delete(u.limiters, userID)
						}
					}
				}
				u.mu.Unlock()
			}
		case <-u.stopCleanup:
			return
		}
	}
}

// Stop stops the background cleanup goroutine.
// Should be called during server shutdown to prevent resource leaks.
// Note: Currently not wired into main.go shutdown sequence.
func (u *UserRateLimiter) Stop() {
	close(u.stopCleanup)
}

// UserRateLimit creates a per-user rate limiting middleware
// This middleware should be placed AFTER Auth middleware in the chain
func UserRateLimit(config UserRateLimitConfig) func(http.Handler) http.Handler {
	limiter := NewUserRateLimiter(config)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to get user context
			userCtx, ok := GetUserContext(r)

			var rateLimiter *rate.Limiter
			var identifier string

			if ok {
				// User is authenticated, use user-based rate limiting
				identifier = userCtx.UserID
				rateLimiter = limiter.GetLimiter(userCtx.UserID)
			} else if limiter.ipLimiter != nil {
				// User is not authenticated, fallback to IP-based rate limiting
				identifier = getClientIP(r, limiter.config.TrustedProxies)
				rateLimiter = limiter.ipLimiter.GetLimiter(identifier)
			} else {
				// No rate limiting for unauthenticated requests
				next.ServeHTTP(w, r)
				return
			}

			if !rateLimiter.Allow() {
				slog.Warn("rate limit exceeded",
					"identifier", identifier,
					"user_id", func() string {
						if ok {
							return userCtx.UserID
						}
						return ""
					}(),
					"path", r.URL.Path,
					"method", r.Method,
				)
				retryAfter := limiter.config.RetryAfterSeconds
				if retryAfter <= 0 {
					retryAfter = 60
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				response.WriteError(w, http.StatusTooManyRequests, response.ErrCodeRateLimit, "too many requests, please try again later", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
