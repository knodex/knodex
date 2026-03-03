package vcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/provops-org/knodex/server/internal/metrics/gitops"
)

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

// doWithRetry executes an HTTP request with retry logic
// This is the core retry function that implements all retry acceptance criteria
func (c *GitHubClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
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

	for attempt := 0; attempt < c.retryConfig.MaxAttempts; attempt++ {
		// AC-RETRY-05: Context cancellation stops retry loop immediately
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Clone request and restore body for each attempt
		reqClone := req.Clone(ctx)
		if bodyBytes != nil {
			reqClone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			reqClone.ContentLength = int64(len(bodyBytes))
		}

		resp, err := c.httpClient.Do(reqClone)
		if err != nil {
			lastErr = err

			// Check if error is retryable (network errors, timeouts)
			if !isRetryableError(err) {
				// AC-RETRY-04: Non-retryable errors fail immediately without retry
				gitops.CommitErrors.WithLabelValues(repoLabel, classifyError(err)).Inc()
				return nil, err
			}

			// Record retry metric
			if attempt > 0 {
				gitops.CommitRetries.WithLabelValues(repoLabel, strconv.Itoa(attempt)).Inc()
			}

			// Wait before retry
			c.waitBeforeRetry(ctx, attempt)
			continue
		}

		// Update rate limit state from response headers
		c.updateRateLimit(resp)

		// AC-RETRY-01: GitHub API calls retry on 5xx errors with exponential backoff
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d, body: %s", resp.StatusCode, truncateBody(body))

			// Record retry metric
			if attempt > 0 {
				gitops.CommitRetries.WithLabelValues(repoLabel, strconv.Itoa(attempt)).Inc()
			}

			c.waitBeforeRetry(ctx, attempt)
			continue
		}

		// AC-RETRY-03: 429 (Too Many Requests) triggers wait based on Retry-After header
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited: retry after %s", retryAfter)

			gitops.CommitErrors.WithLabelValues(repoLabel, gitops.ErrorTypeRateLimit).Inc()

			// Record retry metric
			if attempt > 0 {
				gitops.CommitRetries.WithLabelValues(repoLabel, strconv.Itoa(attempt)).Inc()
			}

			c.waitForRateLimit(ctx, retryAfter)
			continue
		}

		// AC-RETRY-04: Non-retryable errors (4xx except 429) fail immediately without retry
		// Success or non-retryable client error
		return resp, nil
	}

	// All retries exhausted
	gitops.CommitErrors.WithLabelValues(repoLabel, gitops.ErrorTypeServerError).Inc()
	return nil, fmt.Errorf("max retries exceeded (%d attempts): %w", c.retryConfig.MaxAttempts, lastErr)
}

// waitBeforeRetry waits with exponential backoff before the next retry attempt
// AC-RETRY-01: exponential backoff (1s, 2s, 4s)
func (c *GitHubClient) waitBeforeRetry(ctx context.Context, attempt int) {
	delay := time.Duration(math.Pow(2, float64(attempt))) * c.retryConfig.BaseDelay
	if delay > c.retryConfig.MaxDelay {
		delay = c.retryConfig.MaxDelay
	}

	// AC-RETRY-05: Context cancellation stops retry loop immediately
	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
		return
	}
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

// parseRetryAfter parses the Retry-After header value
// AC-RETRY-03: Retry-After header handling
func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(header); err == nil {
		return time.Until(t)
	}

	return 0
}

// isRetryableError determines if an error is retryable
// AC-RETRY-04: Non-retryable errors fail immediately
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		// Timeout errors are retryable
		return netErr.Timeout() || netErr.Temporary()
	}

	// Connection reset, refused, etc. are retryable
	errStr := err.Error()
	retryableMessages := []string{
		"connection reset",
		"connection refused",
		"no such host",
		"temporary failure",
		"i/o timeout",
		"EOF",
	}

	for _, msg := range retryableMessages {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(msg)) {
			return true
		}
	}

	return false
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
