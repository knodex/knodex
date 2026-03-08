package retry

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

// Ensure mockNetError implements net.Error
var _ net.Error = (*mockNetError)(nil)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"generic error", errors.New("something"), false},
		{"network timeout", &mockNetError{timeout: true}, true},
		{"network temporary", &mockNetError{temporary: true}, true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"connection refused", errors.New("dial tcp: connection refused"), true},
		{"no such host", errors.New("no such host"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"EOF", errors.New("unexpected EOF"), true},
		{"wrapped network error", fmt.Errorf("wrapped: %w", &mockNetError{timeout: true}), true},
		{"auth error", errors.New("401 unauthorized"), false},
		{"not found", errors.New("404 not found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{200, false},
		{201, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			if got := IsRetryableHTTPStatus(tt.status); got != tt.want {
				t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "5", 5 * time.Second},
		{"zero seconds", "0", 0},
		{"invalid", "not-a-number", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRetryAfter(tt.header)
			if got != tt.want {
				t.Errorf("ParseRetryAfter(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	// Test HTTP-date format with a future date
	futureTime := time.Now().Add(10 * time.Second)
	header := futureTime.UTC().Format(http.TimeFormat)

	got := ParseRetryAfter(header)
	// Should be approximately 10 seconds (allow 2s tolerance)
	if got < 8*time.Second || got > 12*time.Second {
		t.Errorf("ParseRetryAfter(%q) = %v, want ~10s", header, got)
	}
}

func TestParseRetryAfter_PastDate(t *testing.T) {
	pastTime := time.Now().Add(-10 * time.Second)
	header := pastTime.UTC().Format(http.TimeFormat)

	got := ParseRetryAfter(header)
	if got != 0 {
		t.Errorf("ParseRetryAfter(past date) = %v, want 0", got)
	}
}
