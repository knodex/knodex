// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	calls := 0
	err := Do(context.Background(), DefaultConfig(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("Do() returned unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	config := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	calls := 0
	err := Do(context.Background(), config, func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do() returned unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	config := RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	calls := 0
	err := Do(context.Background(), config, func() error {
		calls++
		return errors.New("persistent error")
	})
	if err == nil {
		t.Fatal("Do() should return error when max attempts exceeded")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := Do(ctx, DefaultConfig(), func() error {
		return errors.New("should not reach here multiple times")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestDo_MaxTotalDuration(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:      100,
		BaseDelay:        time.Millisecond,
		MaxDelay:         time.Millisecond,
		MaxTotalDuration: 50 * time.Millisecond,
	}
	calls := 0
	err := Do(context.Background(), config, func() error {
		calls++
		time.Sleep(20 * time.Millisecond)
		return errors.New("slow error")
	})
	if err == nil {
		t.Fatal("Do() should return error when total duration exceeded")
	}
	// Should have stopped before 100 attempts due to duration limit
	if calls >= 100 {
		t.Errorf("expected fewer than 100 calls due to duration limit, got %d", calls)
	}
}

func TestDo_ZeroMaxAttempts(t *testing.T) {
	config := RetryConfig{MaxAttempts: 0, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}
	calls := 0
	err := Do(context.Background(), config, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("Do() returned unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (zero defaults to 1), got %d", calls)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	config := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	result, err := DoWithResult(context.Background(), config, func() (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("DoWithResult() returned unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}

func TestDoWithResult_RetryThenSuccess(t *testing.T) {
	config := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	calls := 0
	result, err := DoWithResult(context.Background(), config, func() (int, error) {
		calls++
		if calls < 2 {
			return 0, errors.New("retry")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("DoWithResult() returned unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestDoWithResult_AllFail(t *testing.T) {
	config := RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	_, err := DoWithResult(context.Background(), config, func() (string, error) {
		return "", errors.New("fail")
	})
	if err == nil {
		t.Fatal("DoWithResult() should return error")
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", c.MaxAttempts)
	}
	if c.BaseDelay != time.Second {
		t.Errorf("BaseDelay = %v, want 1s", c.BaseDelay)
	}
	if c.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", c.MaxDelay)
	}
}

func TestBackoffDelay(t *testing.T) {
	tests := []struct {
		attempt   int
		baseDelay time.Duration
		maxDelay  time.Duration
		wantBase  time.Duration // base delay before jitter
	}{
		{0, time.Second, 30 * time.Second, 1 * time.Second},
		{1, time.Second, 30 * time.Second, 2 * time.Second},
		{2, time.Second, 30 * time.Second, 4 * time.Second},
		{3, time.Second, 30 * time.Second, 8 * time.Second},
		{10, time.Second, 30 * time.Second, 30 * time.Second}, // capped at max
	}

	for _, tt := range tests {
		got := backoffDelay(tt.attempt, tt.baseDelay, tt.maxDelay)

		// With ±25% jitter, delay should be in range [base*0.75, base*1.25]
		// but also capped at maxDelay
		minExpected := time.Duration(float64(tt.wantBase) * 0.75)
		maxExpected := time.Duration(float64(tt.wantBase) * 1.25)
		if maxExpected > tt.maxDelay {
			maxExpected = tt.maxDelay
		}

		if got < minExpected || got > maxExpected {
			t.Errorf("backoffDelay(%d, %v, %v) = %v, want in range [%v, %v]",
				tt.attempt, tt.baseDelay, tt.maxDelay, got, minExpected, maxExpected)
		}
	}
}

func TestBackoffDelay_Jitter(t *testing.T) {
	// Verify jitter produces different values across multiple calls
	seen := make(map[time.Duration]bool)
	for i := 0; i < 20; i++ {
		d := backoffDelay(2, time.Second, 30*time.Second)
		seen[d] = true
	}
	// With ±25% jitter on 4s, we should see multiple distinct values
	if len(seen) < 2 {
		t.Errorf("jitter produced only %d distinct values out of 20 calls, expected variation", len(seen))
	}
}

func TestDo_PermanentError_StopsRetry(t *testing.T) {
	config := RetryConfig{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	sentinel := errors.New("permanent failure")
	calls := 0
	err := Do(context.Background(), config, func() error {
		calls++
		return Permanent(sentinel)
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for permanent error), got %d", calls)
	}
}

func TestDo_PermanentError_AfterTransient(t *testing.T) {
	config := RetryConfig{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	sentinel := errors.New("permanent failure")
	calls := 0
	err := Do(context.Background(), config, func() error {
		calls++
		if calls == 1 {
			return errors.New("transient")
		}
		return Permanent(sentinel)
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 transient + 1 permanent), got %d", calls)
	}
}

func TestPermanent_Nil(t *testing.T) {
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) should return nil")
	}
}
