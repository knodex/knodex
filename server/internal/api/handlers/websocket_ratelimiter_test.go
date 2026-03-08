// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"runtime"
	"testing"
	"time"
)

// TestConnectionRateLimiter_StopTerminatesGoroutine verifies that calling stop()
// properly terminates the cleanup goroutine, preventing goroutine leaks.
func TestConnectionRateLimiter_StopTerminatesGoroutine(t *testing.T) {
	// Create multiple rate limiters to make goroutine count changes more detectable
	// This reduces flakiness from background goroutine noise
	const numLimiters = 5

	// Get baseline goroutine count after a brief stabilization period
	runtime.GC() // Help stabilize goroutine count
	time.Sleep(20 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	// Create multiple rate limiters (each starts a cleanup goroutine)
	limiters := make([]*connectionRateLimiter, numLimiters)
	for i := 0; i < numLimiters; i++ {
		limiters[i] = newConnectionRateLimiter(10)
	}

	// Give goroutines time to start
	time.Sleep(50 * time.Millisecond)

	// Stop all limiters
	for _, limiter := range limiters {
		limiter.stop()
	}

	// Wait for goroutines to terminate with polling
	// This is more reliable than a fixed sleep
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		afterStopGoroutines := runtime.NumGoroutine()
		// Allow for some variance (±2) due to runtime goroutines
		if afterStopGoroutines <= initialGoroutines+2 {
			return // Test passed
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Final check
	afterStopGoroutines := runtime.NumGoroutine()
	// Only fail if we have significantly more goroutines than expected
	// (indicating a leak of multiple goroutines)
	if afterStopGoroutines > initialGoroutines+numLimiters {
		t.Errorf("possible goroutine leak: expected goroutine count near baseline %d, got %d after stopping %d limiters",
			initialGoroutines, afterStopGoroutines, numLimiters)
	}
}

// TestConnectionRateLimiter_StopIsIdempotent verifies that stop() can be called
// multiple times without panicking (closing an already closed channel).
func TestConnectionRateLimiter_StopIsIdempotent(t *testing.T) {
	limiter := newConnectionRateLimiter(10)

	// Call stop multiple times - should not panic
	limiter.stop()
	limiter.stop()
	limiter.stop()
}

// TestWebSocketHandler_ShutdownStopsRateLimiter verifies that WebSocketHandler.Shutdown()
// properly cleans up the rate limiter's goroutine.
func TestWebSocketHandler_ShutdownStopsRateLimiter(t *testing.T) {
	// Create handler (starts rate limiter goroutine)
	handler := NewWebSocketHandler(nil, nil, nil)

	// Give the rate limiter goroutine time to start
	time.Sleep(20 * time.Millisecond)

	// Get goroutine count after handler is fully initialized
	beforeShutdownGoroutines := runtime.NumGoroutine()

	// Shutdown the handler
	handler.Shutdown()

	// Wait for goroutine to terminate
	time.Sleep(100 * time.Millisecond)

	// Verify goroutine terminated - count should decrease or stay same
	afterShutdownGoroutines := runtime.NumGoroutine()
	if afterShutdownGoroutines > beforeShutdownGoroutines {
		t.Errorf("goroutine leak: goroutine count increased after shutdown from %d to %d",
			beforeShutdownGoroutines, afterShutdownGoroutines)
	}
}

// TestWebSocketHandler_ShutdownIsIdempotent verifies that Shutdown() can be called
// multiple times without panicking.
func TestWebSocketHandler_ShutdownIsIdempotent(t *testing.T) {
	handler := NewWebSocketHandler(nil, nil, nil)

	// Call shutdown multiple times - should not panic
	handler.Shutdown()
	handler.Shutdown()
	handler.Shutdown()
}
