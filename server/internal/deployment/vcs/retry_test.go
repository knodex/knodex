package vcs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/provops-org/knodex/server/internal/metrics/gitops"
)

func TestDefaultRetryConfig(t *testing.T) {
	// AC-RETRY-02: Maximum 3 retry attempts before failing
	if DefaultRetryConfig.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", DefaultRetryConfig.MaxAttempts)
	}

	// AC-RETRY-01: exponential backoff (1s, 2s, 4s)
	if DefaultRetryConfig.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", DefaultRetryConfig.BaseDelay)
	}

	if DefaultRetryConfig.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay=10s, got %v", DefaultRetryConfig.MaxDelay)
	}
}

func TestDoWithRetry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client, err := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.setBaseURLForTesting(server.URL)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", nil)
	resp, err := client.doWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDoWithRetry_ServerErrorRetry(t *testing.T) {
	// AC-RETRY-01: GitHub API calls retry on 5xx errors with exponential backoff
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
			return
		}
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client, err := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.setBaseURLForTesting(server.URL)
	// Speed up test by reducing delays
	client.retryConfig.BaseDelay = 10 * time.Millisecond
	client.retryConfig.MaxDelay = 50 * time.Millisecond

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", nil)
	resp, err := client.doWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetry_MaxRetriesExceeded(t *testing.T) {
	// AC-RETRY-02: Maximum 3 retry attempts before failing
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "server error"}`))
	}))
	defer server.Close()

	client, err := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.setBaseURLForTesting(server.URL)
	client.retryConfig.BaseDelay = 10 * time.Millisecond
	client.retryConfig.MaxDelay = 50 * time.Millisecond

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", nil)
	_, err = client.doWithRetry(context.Background(), req)
	if err == nil {
		t.Fatal("expected error after max retries")
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetry_RateLimitRetry(t *testing.T) {
	// AC-RETRY-03: 429 (Too Many Requests) triggers wait based on Retry-After header
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "rate limited"}`))
			return
		}
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	client, err := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.setBaseURLForTesting(server.URL)
	client.retryConfig.BaseDelay = 10 * time.Millisecond
	client.retryConfig.MaxDelay = 50 * time.Millisecond

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", nil)
	resp, err := client.doWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts after rate limit, got %d", attempts)
	}
}

func TestDoWithRetry_ClientErrorNoRetry(t *testing.T) {
	// AC-RETRY-04: Non-retryable errors (4xx except 429) fail immediately without retry
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	client, err := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.setBaseURLForTesting(server.URL)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/test", nil)
	resp, err := client.doWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// 4xx should return immediately without retry
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt for 4xx error, got %d", attempts)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestDoWithRetry_ContextCancellation(t *testing.T) {
	// AC-RETRY-05: Context cancellation stops retry loop immediately
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.setBaseURLForTesting(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/test", nil)
	_, err = client.doWithRetry(ctx, req)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "5", 5 * time.Second},
		{"large seconds", "60", 60 * time.Second},
		{"invalid", "invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRetryAfter(tt.header)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		retryable bool
	}{
		{"nil error", "", false},
		{"connection reset", "connection reset by peer", true},
		{"connection refused", "dial tcp: connection refused", true},
		{"timeout", "i/o timeout", true},
		{"EOF", "unexpected EOF", true},
		{"no such host", "dial tcp: no such host", true},
		{"temporary failure", "temporary failure in name resolution", true},
		{"normal error", "some other error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = &testError{msg: tt.errMsg}
			}
			result := isRetryableError(err)
			if result != tt.retryable {
				t.Errorf("expected %v, got %v", tt.retryable, result)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{"nil error", "", ""},
		{"rate limit", "rate limit exceeded", gitops.ErrorTypeRateLimit},
		{"unauthorized", "401 unauthorized", gitops.ErrorTypeUnauthorized},
		{"timeout", "request timeout", gitops.ErrorTypeTimeout},
		{"network error", "connection failed", gitops.ErrorTypeNetwork},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = &testError{msg: tt.errMsg}
			}
			result := classifyError(err)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestWaitBeforeRetry(t *testing.T) {
	// AC-RETRY-01: exponential backoff (1s, 2s, 4s)
	client := &GitHubClient{
		retryConfig: RetryConfig{
			BaseDelay: 10 * time.Millisecond,
			MaxDelay:  100 * time.Millisecond,
		},
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 10 * time.Millisecond},  // 2^0 * 10ms = 10ms
		{1, 20 * time.Millisecond},  // 2^1 * 10ms = 20ms
		{2, 40 * time.Millisecond},  // 2^2 * 10ms = 40ms
		{3, 80 * time.Millisecond},  // 2^3 * 10ms = 80ms
		{4, 100 * time.Millisecond}, // capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run("attempt_"+string(rune('0'+tt.attempt)), func(t *testing.T) {
			start := time.Now()
			client.waitBeforeRetry(context.Background(), tt.attempt)
			elapsed := time.Since(start)

			// Allow 50% tolerance for timing
			minExpected := tt.expected / 2
			maxExpected := tt.expected * 2
			if elapsed < minExpected || elapsed > maxExpected {
				t.Errorf("expected wait around %v, got %v", tt.expected, elapsed)
			}
		})
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
