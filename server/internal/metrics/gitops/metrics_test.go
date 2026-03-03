package gitops

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistered(t *testing.T) {
	t.Parallel()

	// AC-METRIC-05: Metrics registered in global Prometheus registry
	// promauto automatically registers metrics

	// Verify metrics are registered by checking they're not nil
	if CommitDuration == nil {
		t.Error("CommitDuration metric not initialized")
	}

	if CommitErrors == nil {
		t.Error("CommitErrors metric not initialized")
	}

	if RateLimitRemaining == nil {
		t.Error("RateLimitRemaining metric not initialized")
	}

	if CommitRetries == nil {
		t.Error("CommitRetries metric not initialized")
	}
}

func TestCommitDuration(t *testing.T) {
	t.Parallel()

	// AC-METRIC-01: gitops_commit_duration_seconds histogram (labels: repo, success)
	metric, err := CommitDuration.GetMetricWith(prometheus.Labels{
		"repo":    "test/repo",
		"success": "true",
	})
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}

	// Observe a value
	metric.Observe(1.5)
}

func TestCommitErrors(t *testing.T) {
	t.Parallel()

	// AC-METRIC-02: gitops_commit_errors_total counter (labels: repo, error_type)
	metric, err := CommitErrors.GetMetricWith(prometheus.Labels{
		"repo":       "test/repo",
		"error_type": ErrorTypeServerError,
	})
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}

	// Increment the counter
	metric.Inc()
}

func TestRateLimitRemaining(t *testing.T) {
	t.Parallel()

	// AC-METRIC-03: gitops_rate_limit_remaining gauge (labels: repo)
	metric, err := RateLimitRemaining.GetMetricWith(prometheus.Labels{
		"repo": "test/repo",
	})
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}

	// Set a value
	metric.Set(4500)
}

func TestCommitRetries(t *testing.T) {
	t.Parallel()

	// AC-METRIC-04: gitops_commit_retries_total counter (labels: repo, attempt)
	metric, err := CommitRetries.GetMetricWith(prometheus.Labels{
		"repo":    "test/repo",
		"attempt": "1",
	})
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}

	// Increment the counter
	metric.Inc()
}

func TestErrorTypeConstants(t *testing.T) {
	t.Parallel()

	// Verify error type constants are defined
	errorTypes := []string{
		ErrorTypeRateLimit,
		ErrorTypeServerError,
		ErrorTypeClientError,
		ErrorTypeNetwork,
		ErrorTypeTimeout,
		ErrorTypeIdempotent,
		ErrorTypeValidation,
		ErrorTypeUnauthorized,
	}

	for _, errType := range errorTypes {
		if errType == "" {
			t.Error("error type constant is empty")
		}
	}
}

func TestMetricLabels(t *testing.T) {
	t.Parallel()

	// Test that metrics work with various label combinations
	repos := []string{"owner/repo1", "owner/repo2", "org/project"}
	successVals := []string{"true", "false"}

	for _, repo := range repos {
		for _, success := range successVals {
			_, err := CommitDuration.GetMetricWith(prometheus.Labels{
				"repo":    repo,
				"success": success,
			})
			if err != nil {
				t.Errorf("failed to get metric for repo=%s, success=%s: %v", repo, success, err)
			}
		}
	}
}

func TestMetricErrorTypes(t *testing.T) {
	t.Parallel()

	// Test all error types can be used as labels
	errorTypes := []string{
		ErrorTypeRateLimit,
		ErrorTypeServerError,
		ErrorTypeClientError,
		ErrorTypeNetwork,
		ErrorTypeTimeout,
		ErrorTypeIdempotent,
		ErrorTypeValidation,
		ErrorTypeUnauthorized,
	}

	for _, errType := range errorTypes {
		_, err := CommitErrors.GetMetricWith(prometheus.Labels{
			"repo":       "test/repo",
			"error_type": errType,
		})
		if err != nil {
			t.Errorf("failed to get metric for error_type=%s: %v", errType, err)
		}
	}
}
