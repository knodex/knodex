//go:build integration

package vcs

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestGitLabIntegration_E2E tests the GitLab client with a real GitLab API
// Run with: GITLAB_TOKEN=xxx GITLAB_OWNER=yyy GITLAB_REPOSITORY=zzz go test -tags=integration -v -run TestGitLabIntegration ./internal/deployment/vcs/
func TestGitLabIntegration_E2E(t *testing.T) {
	token := os.Getenv("GITLAB_TOKEN")
	owner := os.Getenv("GITLAB_OWNER")
	repo := os.Getenv("GITLAB_REPOSITORY")

	if token == "" {
		t.Skip("Skipping GitLab integration test: GITLAB_TOKEN must be set")
	}
	if owner == "" {
		t.Skip("Skipping GitLab integration test: GITLAB_OWNER must be set")
	}
	if repo == "" {
		t.Skip("Skipping GitLab integration test: GITLAB_REPOSITORY must be set")
	}

	t.Logf("Testing GitLab client with owner=%s, repo=%s", owner, repo)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := NewGitLabClient(ctx, token, owner, repo)
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
	}
	defer client.Close()

	// Test 1: Validate token
	t.Run("ValidateToken", func(t *testing.T) {
		if err := client.ValidateToken(ctx); err != nil {
			t.Fatalf("ValidateToken() error: %v", err)
		}
		t.Log("✓ Token validated successfully")
	})

	// Test 2: Get repository info
	var defaultBranch string
	t.Run("GetRepository", func(t *testing.T) {
		info, err := client.GetRepository(ctx)
		if err != nil {
			t.Fatalf("GetRepository() error: %v", err)
		}
		defaultBranch = info.DefaultBranch
		t.Logf("✓ Repository: %s (default branch: %s, private: %v)",
			info.FullName, info.DefaultBranch, info.Private)
	})

	// Test 3: Get rate limit state
	t.Run("GetRateLimitState", func(t *testing.T) {
		remaining, limit, resetTime := client.GetRateLimitState()
		t.Logf("✓ Rate limit: %d/%d remaining, resets at %v", remaining, limit, resetTime)
	})

	// Test 4: Create, read, and delete a test file
	testFileName := fmt.Sprintf("test-e2e-%d.txt", time.Now().Unix())
	testContent := fmt.Sprintf("E2E test file created at %s", time.Now().Format(time.RFC3339))

	t.Run("CommitFile_Create", func(t *testing.T) {
		result, err := client.CommitFile(ctx, &CommitFileRequest{
			Path:    testFileName,
			Content: testContent,
			Message: "test(e2e): create test file for GitLab VCS client",
			Branch:  defaultBranch,
		})
		if err != nil {
			t.Fatalf("CommitFile() error: %v", err)
		}
		t.Logf("✓ Created file %s with commit SHA: %s", testFileName, result.SHA)
	})

	t.Run("GetFileContent", func(t *testing.T) {
		content, err := client.GetFileContent(ctx, testFileName, defaultBranch)
		if err != nil {
			t.Fatalf("GetFileContent() error: %v", err)
		}
		if content == nil {
			t.Fatal("GetFileContent() returned nil")
		}
		if content.Content != testContent {
			t.Errorf("GetFileContent() content = %q, want %q", content.Content, testContent)
		}
		t.Logf("✓ Retrieved file content (SHA: %s)", content.SHA)
	})

	t.Run("CommitWithIdempotency_Skip", func(t *testing.T) {
		// Same content should be skipped
		result, skipped, err := client.CommitWithIdempotency(ctx, &CommitFileRequest{
			Path:    testFileName,
			Content: testContent,
			Message: "test(e2e): should be skipped - same content",
			Branch:  defaultBranch,
		})
		if err != nil {
			t.Fatalf("CommitWithIdempotency() error: %v", err)
		}
		if !skipped {
			t.Error("CommitWithIdempotency() expected skipped=true for same content")
		}
		t.Logf("✓ Idempotent commit correctly skipped (result: %+v)", result)
	})

	t.Run("CommitFile_Update", func(t *testing.T) {
		newContent := testContent + "\nUpdated!"
		result, err := client.CommitFile(ctx, &CommitFileRequest{
			Path:    testFileName,
			Content: newContent,
			Message: "test(e2e): update test file",
			Branch:  defaultBranch,
		})
		if err != nil {
			t.Fatalf("CommitFile() update error: %v", err)
		}
		t.Logf("✓ Updated file with commit SHA: %s", result.SHA)
	})

	t.Run("DeleteFile", func(t *testing.T) {
		err := client.DeleteFile(ctx, testFileName, defaultBranch, "test(e2e): cleanup test file")
		if err != nil {
			t.Fatalf("DeleteFile() error: %v", err)
		}
		t.Logf("✓ Deleted test file %s", testFileName)
	})

	t.Run("GetFileContent_AfterDelete", func(t *testing.T) {
		content, err := client.GetFileContent(ctx, testFileName, defaultBranch)
		if err != nil {
			t.Fatalf("GetFileContent() after delete error: %v", err)
		}
		if content != nil {
			t.Error("GetFileContent() expected nil after delete")
		}
		t.Log("✓ Confirmed file no longer exists")
	})

	t.Log("All E2E tests passed!")
}

// TestGitLabIntegration_MultipleFiles tests committing multiple files at once
func TestGitLabIntegration_MultipleFiles(t *testing.T) {
	token := os.Getenv("GITLAB_TOKEN")
	owner := os.Getenv("GITLAB_OWNER")
	repo := os.Getenv("GITLAB_REPOSITORY")

	if token == "" || owner == "" || repo == "" {
		t.Skip("Skipping GitLab integration test: GITLAB_TOKEN, GITLAB_OWNER, and GITLAB_REPOSITORY must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := NewGitLabClient(ctx, token, owner, repo)
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
	}
	defer client.Close()

	// Get default branch
	repoInfo, err := client.GetRepository(ctx)
	if err != nil {
		t.Fatalf("GetRepository() error: %v", err)
	}

	timestamp := time.Now().Unix()
	files := map[string]string{
		fmt.Sprintf("test-multi-1-%d.txt", timestamp): "File 1 content",
		fmt.Sprintf("test-multi-2-%d.txt", timestamp): "File 2 content",
	}

	t.Run("CommitMultipleFiles", func(t *testing.T) {
		result, err := client.CommitMultipleFiles(ctx, &CommitMultipleFilesRequest{
			Files:   files,
			Message: "test(e2e): commit multiple files",
			Branch:  repoInfo.DefaultBranch,
		})
		if err != nil {
			t.Fatalf("CommitMultipleFiles() error: %v", err)
		}
		t.Logf("✓ Committed %d files with SHA: %s", len(files), result.SHA)
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		for path := range files {
			if err := client.DeleteFile(ctx, path, repoInfo.DefaultBranch, "test(e2e): cleanup multi-file test"); err != nil {
				t.Errorf("Failed to delete %s: %v", path, err)
			} else {
				t.Logf("✓ Deleted %s", path)
			}
		}
	})
}
