package retry

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// IsRetryable determines if an error is retryable.
// Network errors, timeouts, and connection resets are considered retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Network errors (including timeouts)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Common retryable error patterns
	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"no such host",
		"temporary failure",
		"i/o timeout",
		"eof",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// IsRetryableHTTPStatus returns true if the HTTP status code is retryable.
// 5xx server errors and 429 (Too Many Requests) are retryable.
func IsRetryableHTTPStatus(statusCode int) bool {
	return statusCode >= 500 || statusCode == http.StatusTooManyRequests
}

// ParseRetryAfter parses a Retry-After header value.
// It supports both seconds (integer) and HTTP-date formats.
// Returns 0 if the header is empty or unparseable.
func ParseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	// Try parsing as seconds
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(header); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}

	return 0
}
