package vcs

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/provops-org/knodex/server/internal/metrics/gitops"
)

// RateLimitState tracks the current rate limit state for a GitHub API client
// AC-RATE-03: Rate limit state tracked per repository credential
type RateLimitState struct {
	Remaining int
	Limit     int
	Reset     time.Time
	mu        sync.RWMutex
}

// RateLimitError is returned when the rate limit threshold is exceeded
type RateLimitError struct {
	Remaining int
	Reset     time.Time
	WaitTime  time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %d remaining, resets at %s (wait %s)",
		e.Remaining, e.Reset.Format(time.RFC3339), e.WaitTime.Round(time.Second))
}

// RateLimitThreshold is the percentage below which we consider the rate limit exhausted
// AC-RATE-02: When remaining < 10% of limit, commits return rate-limited error
const RateLimitThreshold = 0.10 // 10%

// updateRateLimit updates the rate limit state from a response
// AC-RATE-01: GitHub API responses parsed for X-RateLimit-Remaining header
// AC-RATE-04: X-RateLimit-Reset timestamp used to calculate wait time
func (c *GitHubClient) updateRateLimit(resp *http.Response) {
	if c.rateLimit == nil {
		return
	}

	remaining, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Remaining"))
	limit, _ := strconv.Atoi(resp.Header.Get("X-RateLimit-Limit"))
	resetUnix, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)

	c.rateLimit.mu.Lock()
	defer c.rateLimit.mu.Unlock()

	c.rateLimit.Remaining = remaining
	c.rateLimit.Limit = limit
	if resetUnix > 0 {
		c.rateLimit.Reset = time.Unix(resetUnix, 0)
	}

	// AC-RATE-05: Rate limit metrics exposed via Prometheus
	repoLabel := fmt.Sprintf("%s/%s", c.owner, c.repo)
	gitops.RateLimitRemaining.WithLabelValues(repoLabel).Set(float64(remaining))
}

// checkRateLimit checks if we're below the rate limit threshold
// AC-RATE-02: When remaining < 10% of limit, commits return rate-limited error
func (c *GitHubClient) checkRateLimit() error {
	if c.rateLimit == nil {
		return nil
	}

	c.rateLimit.mu.RLock()
	defer c.rateLimit.mu.RUnlock()

	// No rate limit info yet - proceed normally
	// Edge case: First request has no rate limit info
	if c.rateLimit.Limit == 0 {
		return nil
	}

	// Edge case: GitHub API returns 0 for rate limit headers - ignore rate limiting
	if c.rateLimit.Remaining == 0 && c.rateLimit.Limit == 0 {
		return nil
	}

	threshold := float64(c.rateLimit.Limit) * RateLimitThreshold
	if float64(c.rateLimit.Remaining) < threshold {
		waitTime := time.Until(c.rateLimit.Reset)

		// Edge case: Rate limit reset time in past - retry immediately
		if waitTime < 0 {
			waitTime = 0
		}

		return &RateLimitError{
			Remaining: c.rateLimit.Remaining,
			Reset:     c.rateLimit.Reset,
			WaitTime:  waitTime,
		}
	}

	return nil
}

// GetRateLimitState returns a copy of the current rate limit state (for testing/monitoring)
func (c *GitHubClient) GetRateLimitState() (remaining, limit int, reset time.Time) {
	if c.rateLimit == nil {
		return 0, 0, time.Time{}
	}

	c.rateLimit.mu.RLock()
	defer c.rateLimit.mu.RUnlock()

	return c.rateLimit.Remaining, c.rateLimit.Limit, c.rateLimit.Reset
}
