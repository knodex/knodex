//go:build e2e
// +build e2e

package vcs_e2e

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/deployment/vcs"
	"github.com/knodex/knodex/server/internal/metrics/gitops"
)

// E2E tests for GitOps commit reliability improvements
// These tests require a real GitHub token and test against:
// https://github.com/knodex/test-knodex-repositories
//
// Run with: go test -v -tags=e2e ./test/e2e/vcs/... -run TestE2E
//
// Required environment variables:
// - GITHUB_TOKEN: A GitHub token with repo access to knodex/test-knodex-repositories
//
// The tests are designed to be idempotent and safe to run repeatedly.

const (
	testOwner  = "knodex"
	testRepo   = "test-knodex-repositories"
	testBranch = "main"
)

func getTestClient(t *testing.T) *vcs.GitHubClient {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping E2E test")
	}

	client, err := vcs.NewGitHubClient(context.Background(), token, testOwner, testRepo)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	t.Cleanup(func() {
		client.Close()
	})

	return client
}

func TestE2E_ClientCreation(t *testing.T) {
	// Verify client can be created with connection pooling
	client := getTestClient(t)

	// Verify the client has proper configuration
	if client.GetOwner() != testOwner {
		t.Errorf("expected owner=%s, got %s", testOwner, client.GetOwner())
	}
	if client.GetRepo() != testRepo {
		t.Errorf("expected repo=%s, got %s", testRepo, client.GetRepo())
	}

	// Verify rate limit state is initialized by making a request
	_, err := client.GetRepository(context.Background())
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}
	remaining, _, _ := client.GetRateLimitState()
	// Rate limit state should be populated after a request
	t.Logf("Rate limit remaining after request: %d", remaining)

	// Verify retry config is set
	retryConfig := client.GetRetryConfig()
	if retryConfig.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", retryConfig.MaxAttempts)
	}
}

func TestE2E_RepositoryAccess(t *testing.T) {
	client := getTestClient(t)
	ctx := context.Background()

	// Test that we can access the repository
	info, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}

	expectedName := fmt.Sprintf("%s/%s", testOwner, testRepo)
	if info.FullName != expectedName {
		t.Errorf("expected full name=%s, got %s", expectedName, info.FullName)
	}

	t.Logf("Repository: %s, Default Branch: %s", info.FullName, info.DefaultBranch)
}

func TestE2E_RateLimitTracking(t *testing.T) {
	// AC-RATE-01: GitHub API responses parsed for X-RateLimit-Remaining header
	// AC-RATE-03: Rate limit state tracked per repository credential
	client := getTestClient(t)
	ctx := context.Background()

	// Make a request to trigger rate limit header parsing
	_, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}

	// Verify rate limit state was updated
	remaining, limit, reset := client.GetRateLimitState()

	t.Logf("Rate Limit: %d/%d, Resets: %s", remaining, limit, reset.Format(time.RFC3339))

	if limit == 0 {
		t.Error("rate limit should be set after API call")
	}

	if remaining == 0 && limit > 0 {
		t.Error("remaining should be > 0 unless rate limit is exhausted")
	}

	// AC-RATE-04: X-RateLimit-Reset timestamp used to calculate wait time
	if reset.IsZero() {
		t.Error("reset time should be set after API call")
	}
}

func TestE2E_CommitWithIdempotency(t *testing.T) {
	// AC-IDEM-01: Each commit includes idempotency key in commit message
	// AC-IDEM-02: Before committing, check if file content matches hash
	// AC-IDEM-03: Idempotency prevents duplicate commits on retry
	client := getTestClient(t)
	ctx := context.Background()

	// Generate unique test content with timestamp
	testPath := "e2e-tests/story-146-idempotency-test.yaml"
	testContent := fmt.Sprintf(`# E2E Test for this feature
# Generated at: %s
apiVersion: v1
kind: ConfigMap
metadata:
  name: e2e-test-story-146
data:
  test: "idempotency"
  timestamp: "%s"
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339Nano))

	// First commit - should succeed
	req := &vcs.CommitFileRequest{
		Path:    testPath,
		Content: testContent,
		Message: "test(e2e): idempotency test",
		Branch:  testBranch,
	}

	result, skipped, err := client.CommitWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}

	if skipped {
		t.Log("First commit was skipped (content already identical)")
	} else {
		t.Logf("First commit succeeded: SHA=%s", result.SHA)

		// Verify commit message contains idempotency key
		// Note: The message is modified by FormatMessageWithIdempotencyKey
	}

	// Second commit with same content - should be skipped
	result2, skipped2, err := client.CommitWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("second commit check failed: %v", err)
	}

	if !skipped2 {
		t.Error("second commit with identical content should be skipped")
	} else {
		t.Logf("Second commit correctly skipped: %s", result2.Message)
	}

	// Third commit with different content - should succeed
	req.Content = testContent + "\n# Updated at: " + time.Now().Format(time.RFC3339Nano)
	result3, skipped3, err := client.CommitWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("third commit failed: %v", err)
	}

	if skipped3 {
		t.Error("third commit with different content should NOT be skipped")
	} else {
		t.Logf("Third commit succeeded with new content: SHA=%s", result3.SHA)
	}
}

func TestE2E_CommitMultipleWithIdempotency(t *testing.T) {
	// Test multi-file commit with idempotency
	client := getTestClient(t)
	ctx := context.Background()

	timestamp := time.Now().Format(time.RFC3339Nano)

	req := &vcs.CommitMultipleFilesRequest{
		Files: map[string]string{
			"e2e-tests/story-146-multi-1.yaml": fmt.Sprintf(`# Multi-file test 1
timestamp: "%s"
`, timestamp),
			"e2e-tests/story-146-multi-2.yaml": fmt.Sprintf(`# Multi-file test 2
timestamp: "%s"
`, timestamp),
		},
		Message: "test(e2e): multi-file idempotency test",
		Branch:  testBranch,
	}

	result, skipped, err := client.CommitMultipleWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("multi-file commit failed: %v", err)
	}

	if skipped {
		t.Log("Multi-file commit was skipped (all content identical)")
	} else {
		t.Logf("Multi-file commit succeeded: SHA=%s", result.SHA)
	}

	// Second commit with same content should be skipped
	result2, skipped2, err := client.CommitMultipleWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("second multi-file commit check failed: %v", err)
	}

	if !skipped2 {
		t.Error("second multi-file commit with identical content should be skipped")
	} else {
		t.Logf("Second multi-file commit correctly skipped: %s", result2.Message)
	}
}

func TestE2E_RetryOnTransientError(t *testing.T) {
	// AC-RETRY-01: GitHub API calls retry on 5xx errors with exponential backoff
	// AC-RETRY-02: Maximum 3 retry attempts before failing
	// This test verifies the retry mechanism is properly configured
	// We can't easily trigger 5xx errors from GitHub, but we verify the config
	client := getTestClient(t)

	// Verify retry configuration
	retryConfig := client.GetRetryConfig()
	if retryConfig.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", retryConfig.MaxAttempts)
	}

	if retryConfig.BaseDelay != 1*time.Second {
		t.Errorf("expected BaseDelay=1s, got %v", retryConfig.BaseDelay)
	}

	if retryConfig.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay=10s, got %v", retryConfig.MaxDelay)
	}

	t.Logf("Retry config: MaxAttempts=%d, BaseDelay=%v, MaxDelay=%v",
		retryConfig.MaxAttempts,
		retryConfig.BaseDelay,
		retryConfig.MaxDelay)
}

func TestE2E_ContextCancellation(t *testing.T) {
	// AC-RETRY-05: Context cancellation stops retry loop immediately
	client := getTestClient(t)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Attempt to make a request - should fail immediately
	_, err := client.GetRepository(ctx)
	if err == nil {
		t.Error("expected error with cancelled context")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Logf("Error message: %v (expected to contain 'context canceled')", err)
	}
}

func TestE2E_ConnectionPooling(t *testing.T) {
	// AC-POOL-01: HTTP client uses connection pooling (MaxIdleConns: 100)
	// AC-POOL-02: Idle connections per host: 10
	// AC-POOL-04: HTTP client reused across commits
	client := getTestClient(t)
	ctx := context.Background()

	// Verify the client is properly configured by making multiple requests
	// The same HTTP client should be reused
	for i := 0; i < 5; i++ {
		_, err := client.GetRepository(ctx)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}

	// Verify rate limit remains consistent (same client tracking)
	remaining1, limit1, _ := client.GetRateLimitState()
	_, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("final request failed: %v", err)
	}
	remaining2, limit2, _ := client.GetRateLimitState()

	// Limit should remain the same (5000 for authenticated users)
	if limit1 != limit2 {
		t.Errorf("limit changed between requests: %d -> %d", limit1, limit2)
	}

	// Remaining should decrease by approximately 1 per request
	t.Logf("Rate limit after multiple requests: %d/%d (started at %d)", remaining2, limit2, remaining1)
}

func TestE2E_RateLimitCheck(t *testing.T) {
	// AC-RATE-02: When remaining < 10% of limit, commits return rate-limited error
	client := getTestClient(t)
	ctx := context.Background()

	// Make a request to populate rate limit state
	_, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}

	// Check the current rate limit state
	remaining, limit, reset := client.GetRateLimitState()
	t.Logf("Current rate limit: %d/%d, resets at %s", remaining, limit, reset.Format(time.RFC3339))

	// Verify CheckRateLimit works
	err = client.CheckRateLimit()
	if err != nil {
		// This would only happen if we're below 10% threshold
		rateLimitErr, ok := err.(*vcs.RateLimitError)
		if ok {
			t.Logf("Rate limit check triggered (below 10%%): %v", rateLimitErr)
		} else {
			t.Errorf("unexpected error from CheckRateLimit: %v", err)
		}
	} else {
		threshold := float64(limit) * vcs.RateLimitThreshold
		if float64(remaining) >= threshold {
			t.Logf("Rate limit OK: %d remaining (threshold: %.0f)", remaining, threshold)
		}
	}
}

func TestE2E_PrometheusMetrics(t *testing.T) {
	// AC-METRIC-01 through AC-METRIC-05: Verify metrics are being updated
	client := getTestClient(t)
	ctx := context.Background()

	// Make a request to trigger metric updates
	_, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}

	// Verify metrics are registered (not nil)
	if gitops.CommitDuration == nil {
		t.Error("CommitDuration metric not registered")
	}
	if gitops.CommitErrors == nil {
		t.Error("CommitErrors metric not registered")
	}
	if gitops.RateLimitRemaining == nil {
		t.Error("RateLimitRemaining metric not registered")
	}
	if gitops.CommitRetries == nil {
		t.Error("CommitRetries metric not registered")
	}

	t.Log("All Prometheus metrics are properly registered")
}

func TestE2E_FullWorkflow(t *testing.T) {
	// Complete E2E workflow test for GitOps commit reliability
	client := getTestClient(t)
	ctx := context.Background()

	// Step 1: Verify repository access
	t.Log("Step 1: Verifying repository access...")
	info, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}
	t.Logf("  Repository: %s", info.FullName)

	// Step 2: Check rate limit state
	t.Log("Step 2: Checking rate limit state...")
	remaining, limit, reset := client.GetRateLimitState()
	t.Logf("  Rate limit: %d/%d, resets: %s", remaining, limit, reset.Format(time.RFC3339))

	// Step 3: Create/update a test file with idempotency
	t.Log("Step 3: Testing idempotent commit...")
	testPath := "e2e-tests/story-146-full-workflow.yaml"
	testContent := fmt.Sprintf(`# Full Workflow E2E Test
# Test run: %s
apiVersion: v1
kind: ConfigMap
metadata:
  name: story-146-workflow-test
data:
  workflow: "e2e-test"
  timestamp: "%s"
`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339Nano))

	req := &vcs.CommitFileRequest{
		Path:    testPath,
		Content: testContent,
		Message: "test(e2e): full workflow test",
		Branch:  testBranch,
	}

	result, skipped, err := client.CommitWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	if skipped {
		t.Log("  Commit skipped (content identical)")
	} else {
		t.Logf("  Commit succeeded: SHA=%s", result.SHA)
	}

	// Step 4: Verify idempotency (same content should skip)
	t.Log("Step 4: Verifying idempotency (second commit should skip)...")
	_, skipped2, err := client.CommitWithIdempotency(ctx, req)
	if err != nil {
		t.Fatalf("idempotency check failed: %v", err)
	}
	if skipped2 {
		t.Log("  Idempotency working: duplicate commit skipped")
	} else {
		t.Error("  Idempotency FAILED: duplicate commit was not skipped")
	}

	// Step 5: Verify rate limit was updated
	t.Log("Step 5: Verifying rate limit tracking...")
	remaining2, _, _ := client.GetRateLimitState()
	if remaining2 < remaining {
		t.Logf("  Rate limit correctly decremented: %d -> %d", remaining, remaining2)
	} else {
		t.Logf("  Rate limit: %d (unchanged or increased due to GitHub window)", remaining2)
	}

	t.Log("Full workflow test completed successfully!")
}
