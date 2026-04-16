// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package vcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/knodex/knodex/server/internal/metrics/gitops"
	utilretry "github.com/knodex/knodex/server/internal/util/retry"
)

// RateLimitState tracks the current rate limit state for a VCS API client
// AC-RATE-03: Rate limit state tracked per repository credential
type RateLimitState struct {
	Remaining int
	Limit     int
	Reset     time.Time
	mu        sync.RWMutex
}

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

// GetRateLimitState returns a copy of the current rate limit state (for monitoring)
func (c *GitHubClient) GetRateLimitState() (remaining, limit int, reset time.Time) {
	if c.rateLimit == nil {
		return 0, 0, time.Time{}
	}

	c.rateLimit.mu.RLock()
	defer c.rateLimit.mu.RUnlock()

	return c.rateLimit.Remaining, c.rateLimit.Limit, c.rateLimit.Reset
}

// maxVCSResponseSize limits response body reads to prevent memory exhaustion.
// VCS API responses (file content, error bodies) can be large; 10 MB is generous
// while preventing unbounded memory consumption from oversized or malformed responses.
const maxVCSResponseSize = 10 << 20 // 10 MB

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts
	// AC-RETRY-02: Maximum 3 retry attempts before failing
	MaxAttempts int

	// BaseDelay is the initial delay between retries
	// AC-RETRY-01: exponential backoff (1s, 2s, 4s)
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration
}

// DefaultRetryConfig provides sensible defaults for retry behavior
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    10 * time.Second,
}

// doWithRetry executes an HTTP request with retry logic.
// Uses util/retry.DoWithResult for exponential backoff with ±25% jitter (prevents thundering herd).
// AC-RETRY-01: exponential backoff; AC-RETRY-02: max 3 attempts; AC-RETRY-03: 429 Retry-After;
// AC-RETRY-04: non-retryable errors fail immediately; AC-RETRY-05: context cancellation.
func (c *GitHubClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	repoLabel := fmt.Sprintf("%s/%s", c.owner, c.repo)

	// Capture request body for retries (body can only be read once)
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	retryConf := utilretry.RetryConfig{
		MaxAttempts: c.retryConfig.MaxAttempts,
		BaseDelay:   c.retryConfig.BaseDelay,
		MaxDelay:    c.retryConfig.MaxDelay,
	}

	resp, err := utilretry.DoWithResult(ctx, retryConf, func() (*http.Response, error) {
		// Clone request and restore body for each attempt
		reqClone := req.Clone(ctx)
		if bodyBytes != nil {
			reqClone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqClone.ContentLength = int64(len(bodyBytes))
		}

		resp, err := c.httpClient.Do(reqClone)
		if err != nil {
			if !isRetryableError(err) {
				// AC-RETRY-04: Non-retryable errors fail immediately
				gitops.CommitErrors.WithLabelValues(repoLabel, classifyError(err)).Inc()
				return nil, utilretry.Permanent(err)
			}
			return nil, err
		}

		// Update rate limit state from response headers
		c.updateRateLimit(resp)

		// AC-RETRY-01: GitHub API calls retry on 5xx errors
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, maxVCSResponseSize))
			resp.Body.Close()
			return nil, fmt.Errorf("server error: %d, body: %s", resp.StatusCode, truncateBody(body))
		}

		// AC-RETRY-03: 429 (Too Many Requests) — wait for Retry-After, then retry
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			gitops.CommitErrors.WithLabelValues(repoLabel, gitops.ErrorTypeRateLimit).Inc()
			c.waitForRateLimit(ctx, retryAfter)
			return nil, fmt.Errorf("rate limited: retry after %s", retryAfter)
		}

		// AC-RETRY-04: Non-retryable client errors (4xx except 429) succeed immediately
		return resp, nil
	})

	if err != nil {
		gitops.CommitErrors.WithLabelValues(repoLabel, gitops.ErrorTypeServerError).Inc()
		return nil, err
	}

	return resp, nil
}

// waitForRateLimit waits for the rate limit to reset
// AC-RETRY-03: 429 triggers wait based on Retry-After header
func (c *GitHubClient) waitForRateLimit(ctx context.Context, retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = c.retryConfig.BaseDelay
	}

	// Cap the wait time to prevent excessive waits
	if retryAfter > c.retryConfig.MaxDelay*10 {
		retryAfter = c.retryConfig.MaxDelay * 10
	}

	// AC-RETRY-05: Context cancellation stops retry loop immediately
	select {
	case <-ctx.Done():
		return
	case <-time.After(retryAfter):
		return
	}
}

// parseRetryAfter delegates to util/retry for Retry-After header parsing.
func parseRetryAfter(header string) time.Duration {
	return utilretry.ParseRetryAfter(header)
}

// isRetryableError delegates to util/retry for error classification.
func isRetryableError(err error) bool {
	return utilretry.IsRetryable(err)
}

// classifyError returns an error type label for metrics
func classifyError(err error) string {
	if err == nil {
		return ""
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return gitops.ErrorTypeTimeout
		}
		return gitops.ErrorTypeNetwork
	}

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "rate limit") {
		return gitops.ErrorTypeRateLimit
	}
	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401") {
		return gitops.ErrorTypeUnauthorized
	}
	if strings.Contains(errStr, "timeout") {
		return gitops.ErrorTypeTimeout
	}

	return gitops.ErrorTypeNetwork
}

// truncateBody truncates response body for logging
func truncateBody(body []byte) string {
	const maxLen = 200
	if len(body) > maxLen {
		return string(body[:maxLen]) + "..."
	}
	return string(body)
}
