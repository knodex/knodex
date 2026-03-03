package vcs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitThreshold(t *testing.T) {
	t.Parallel()

	// AC-RATE-02: When remaining < 10% of limit, commits return rate-limited error
	if RateLimitThreshold != 0.10 {
		t.Errorf("expected threshold 0.10, got %f", RateLimitThreshold)
	}
}

func TestUpdateRateLimit(t *testing.T) {
	t.Parallel()

	// AC-RATE-01: GitHub API responses parsed for X-RateLimit-Remaining header
	// AC-RATE-04: X-RateLimit-Reset timestamp used to calculate wait time
	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	// Create a mock response with rate limit headers
	resp := &http.Response{
		Header: make(http.Header),
	}
	resp.Header.Set("X-RateLimit-Remaining", "4500")
	resp.Header.Set("X-RateLimit-Limit", "5000")
	resetTime := time.Now().Add(1 * time.Hour).Unix()
	resp.Header.Set("X-RateLimit-Reset", formatUnixTime(resetTime))

	client.updateRateLimit(resp)

	remaining, limit, reset := client.GetRateLimitState()
	if remaining != 4500 {
		t.Errorf("expected remaining=4500, got %d", remaining)
	}
	if limit != 5000 {
		t.Errorf("expected limit=5000, got %d", limit)
	}
	if reset.Unix() != resetTime {
		t.Errorf("expected reset=%d, got %d", resetTime, reset.Unix())
	}
}

func TestCheckRateLimit_BelowThreshold(t *testing.T) {
	t.Parallel()

	// AC-RATE-02: When remaining < 10% of limit, commits return rate-limited error
	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	// Set rate limit state below threshold (10% of 5000 = 500)
	client.rateLimit.Limit = 5000
	client.rateLimit.Remaining = 100 // Below 10%
	client.rateLimit.Reset = time.Now().Add(1 * time.Hour)

	err := client.checkRateLimit()
	if err == nil {
		t.Fatal("expected rate limit error when below threshold")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}

	if rateLimitErr.Remaining != 100 {
		t.Errorf("expected remaining=100, got %d", rateLimitErr.Remaining)
	}
}

func TestCheckRateLimit_AboveThreshold(t *testing.T) {
	t.Parallel()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	// Set rate limit state above threshold
	client.rateLimit.Limit = 5000
	client.rateLimit.Remaining = 4000 // Well above 10%
	client.rateLimit.Reset = time.Now().Add(1 * time.Hour)

	err := client.checkRateLimit()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckRateLimit_NoLimitInfo(t *testing.T) {
	t.Parallel()

	// Edge case: First request has no rate limit info - proceed normally
	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	// Default state has limit=0
	err := client.checkRateLimit()
	if err != nil {
		t.Errorf("unexpected error when no rate limit info: %v", err)
	}
}

func TestCheckRateLimit_ZeroRemaining(t *testing.T) {
	t.Parallel()

	// Edge case: GitHub API returns 0 for rate limit headers - ignore rate limiting
	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	client.rateLimit.Limit = 0
	client.rateLimit.Remaining = 0

	err := client.checkRateLimit()
	if err != nil {
		t.Errorf("unexpected error when limit is 0: %v", err)
	}
}

func TestCheckRateLimit_ResetInPast(t *testing.T) {
	t.Parallel()

	// Edge case: Rate limit reset time in past - retry immediately
	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	client.rateLimit.Limit = 5000
	client.rateLimit.Remaining = 100
	client.rateLimit.Reset = time.Now().Add(-1 * time.Hour) // In the past

	err := client.checkRateLimit()
	if err == nil {
		t.Fatal("expected rate limit error")
	}

	rateLimitErr := err.(*RateLimitError)
	if rateLimitErr.WaitTime > 0 {
		t.Errorf("expected WaitTime=0 for reset in past, got %v", rateLimitErr.WaitTime)
	}
}

func TestRateLimitError_Error(t *testing.T) {
	t.Parallel()

	err := &RateLimitError{
		Remaining: 50,
		Reset:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		WaitTime:  30 * time.Minute,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestUpdateRateLimitFromResponse(t *testing.T) {
	t.Parallel()

	// AC-RATE-01: Parse headers from actual HTTP response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Reset", formatUnixTime(time.Now().Add(1*time.Hour).Unix()))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	client.setBaseURLForTesting(server.URL)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := client.doWithRetry(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	remaining, limit, _ := client.GetRateLimitState()
	if remaining != 4999 {
		t.Errorf("expected remaining=4999, got %d", remaining)
	}
	if limit != 5000 {
		t.Errorf("expected limit=5000, got %d", limit)
	}
}

func TestGetRateLimitState_NilRateLimit(t *testing.T) {
	t.Parallel()

	client := &GitHubClient{}
	client.rateLimit = nil

	remaining, limit, reset := client.GetRateLimitState()
	if remaining != 0 || limit != 0 || !reset.IsZero() {
		t.Error("expected zero values for nil rate limit")
	}
}

// formatUnixTime formats a Unix timestamp as a string
func formatUnixTime(unix int64) string {
	return strconv.FormatInt(unix, 10)
}
