// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package retry provides generic retry with exponential backoff.
//
// It consolidates retry patterns previously embedded in VCS and deployment
// code, following the ArgoCD util/ package structure.
//
// Usage:
//
//	result, err := retry.DoWithResult(ctx, retry.DefaultConfig(), func() (string, error) {
//	    return callExternalAPI()
//	})
//
//	err := retry.Do(ctx, retry.DefaultConfig(), func() error {
//	    return sendRequest()
//	})
package retry

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

// PermanentError wraps an error to indicate it should not be retried.
// Use Permanent() to create one.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// Permanent wraps an error to signal that retry should stop immediately.
// DoWithResult unwraps PermanentError and returns the inner error directly.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// RetryConfig holds configuration for retry behavior.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	MaxAttempts int

	// BaseDelay is the initial delay between retries.
	BaseDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// MaxTotalDuration caps the total time spent retrying. Zero means no limit.
	MaxTotalDuration time.Duration
}

// DefaultConfig returns sensible defaults: 3 attempts, 1s base, 30s max.
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
	}
}

// Do executes fn with retry logic. It retries on error until MaxAttempts is
// exhausted or the context is canceled.
func Do(ctx context.Context, config RetryConfig, fn func() error) error {
	_, err := DoWithResult(ctx, config, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// DoWithResult executes fn with retry logic, returning the result on success.
// It retries on error until MaxAttempts is exhausted or the context is canceled.
func DoWithResult[T any](ctx context.Context, config RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 1
	}

	start := time.Now()
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}

		// Check total duration limit
		if config.MaxTotalDuration > 0 && time.Since(start) >= config.MaxTotalDuration {
			return zero, fmt.Errorf("retry: max total duration %s exceeded after %d attempts: %w",
				config.MaxTotalDuration, attempt, lastErr)
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}
		// Permanent error — stop retrying, return the inner error
		var pe *PermanentError
		if errors.As(err, &pe) {
			return zero, pe.Err
		}
		lastErr = err

		// Don't wait after the last attempt
		if attempt < config.MaxAttempts-1 {
			delay := backoffDelay(attempt, config.BaseDelay, config.MaxDelay)
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return zero, fmt.Errorf("retry: max attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// backoffDelay computes exponential backoff with jitter: 2^attempt * baseDelay * (0.75–1.25), capped at maxDelay.
// Jitter prevents thundering herd when multiple callers retry simultaneously.
func backoffDelay(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}

	// Apply ±25% jitter using crypto/rand
	// jitterFactor ranges from 0.75 to 1.25
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		n := binary.LittleEndian.Uint64(buf[:])
		// Map to [0.0, 1.0) then scale to [0.75, 1.25)
		jitterFactor := 0.75 + 0.5*(float64(n)/float64(math.MaxUint64))
		delay = time.Duration(float64(delay) * jitterFactor)
	}

	// Re-cap after jitter (jitter can push above maxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}
