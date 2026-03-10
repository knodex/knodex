// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package vcs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestComputeContentHash(t *testing.T) {
	// AC-IDEM-01: Each commit includes idempotency key (SHA256 of manifest content)
	content := []byte("test content")
	hash := ComputeContentHash(content)

	// SHA256 produces 64 hex characters
	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Same content should produce same hash
	hash2 := ComputeContentHash(content)
	if hash != hash2 {
		t.Error("same content produced different hashes")
	}

	// Different content should produce different hash
	hash3 := ComputeContentHash([]byte("different content"))
	if hash == hash3 {
		t.Error("different content produced same hash")
	}
}

func TestComputeContentHashString(t *testing.T) {
	content := "test content"
	hash := ComputeContentHashString(content)

	// Should produce same result as byte version
	hashBytes := ComputeContentHash([]byte(content))
	if hash != hashBytes {
		t.Error("string and byte versions produced different hashes")
	}
}

func TestFormatMessageWithIdempotencyKey(t *testing.T) {
	// AC-IDEM-01: Include hash in commit message for traceability
	// SECURITY: Idempotency key truncated (16 chars) to avoid leaking full hash
	message := "Test commit message"
	contentHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	formatted := FormatMessageWithIdempotencyKey(message, contentHash)

	if !strings.Contains(formatted, message) {
		t.Error("formatted message should contain original message")
	}

	if !strings.Contains(formatted, "Idempotency-Key:") {
		t.Error("formatted message should contain Idempotency-Key")
	}

	// Verify key is truncated to 16 chars
	expectedKey := contentHash[:IdempotencyKeyLength]
	if !strings.Contains(formatted, expectedKey) {
		t.Errorf("formatted message should contain truncated key: %s", expectedKey)
	}

	// Verify full hash is not present (security)
	if strings.Contains(formatted, contentHash) {
		t.Error("formatted message should NOT contain full hash")
	}
}

func TestFormatMessageWithIdempotencyKey_ShortHash(t *testing.T) {
	message := "Test commit"
	shortHash := "abc123"

	formatted := FormatMessageWithIdempotencyKey(message, shortHash)

	// Short hashes should be used as-is
	if !strings.Contains(formatted, shortHash) {
		t.Error("formatted message should contain short hash as-is")
	}
}

func TestIdempotencyKeyLength(t *testing.T) {
	// SECURITY: Verify key length is 16
	if IdempotencyKeyLength != 16 {
		t.Errorf("expected IdempotencyKeyLength=16, got %d", IdempotencyKeyLength)
	}
}

func TestCommitWithIdempotency_IdenticalContent(t *testing.T) {
	// AC-IDEM-02: Before committing, check if file content matches hash (skip if identical)
	// AC-IDEM-03: Idempotency prevents duplicate commits on retry
	existingContent := "existing file content"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(existingContent))
	contentSHA := "abc123def456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/") {
			// Return existing file with same content
			response := FileContent{
				SHA:     contentSHA,
				Content: encodedContent,
				Path:    "test/file.yaml",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		t.Error("unexpected request - should not reach here for identical content")
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	client.setBaseURLForTesting(server.URL)

	req := &CommitFileRequest{
		Path:    "test/file.yaml",
		Content: existingContent, // Same as existing
		Message: "Test commit",
		Branch:  "main",
	}

	result, skipped, err := client.CommitWithIdempotency(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !skipped {
		t.Error("expected commit to be skipped for identical content")
	}

	if result.Message != "skipped: content identical" {
		t.Errorf("unexpected result message: %s", result.Message)
	}
}

func TestCommitWithIdempotency_DifferentContent(t *testing.T) {
	// When content differs, commit should proceed
	existingContent := "old content"
	newContent := "new content"
	encodedContent := base64.StdEncoding.EncodeToString([]byte(existingContent))
	contentSHA := "abc123def456"
	newCommitSHA := "newcommit123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/") {
			response := FileContent{
				SHA:     contentSHA,
				Content: encodedContent,
				Path:    "test/file.yaml",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/") {
			// Commit request
			response := map[string]interface{}{
				"commit": map[string]string{
					"sha":     newCommitSHA,
					"message": "Test commit",
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}

		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	client.setBaseURLForTesting(server.URL)

	req := &CommitFileRequest{
		Path:    "test/file.yaml",
		Content: newContent,
		Message: "Test commit",
		Branch:  "main",
	}

	result, skipped, err := client.CommitWithIdempotency(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skipped {
		t.Error("commit should NOT be skipped for different content")
	}

	if result.SHA != newCommitSHA {
		t.Errorf("expected SHA=%s, got %s", newCommitSHA, result.SHA)
	}
}

func TestCommitWithIdempotency_NewFile(t *testing.T) {
	// When file doesn't exist, commit should proceed
	newContent := "new file content"
	newCommitSHA := "newcommit456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/") {
			// File doesn't exist
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/") {
			response := map[string]interface{}{
				"commit": map[string]string{
					"sha":     newCommitSHA,
					"message": "Create new file",
				},
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
			return
		}

		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	client.setBaseURLForTesting(server.URL)

	req := &CommitFileRequest{
		Path:    "test/new-file.yaml",
		Content: newContent,
		Message: "Create new file",
		Branch:  "main",
	}

	result, skipped, err := client.CommitWithIdempotency(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skipped {
		t.Error("commit should NOT be skipped for new file")
	}

	if result.SHA != newCommitSHA {
		t.Errorf("expected SHA=%s, got %s", newCommitSHA, result.SHA)
	}
}

func TestCommitWithIdempotency_Validation(t *testing.T) {
	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")

	tests := []struct {
		name   string
		req    *CommitFileRequest
		errMsg string
	}{
		{
			name:   "empty path",
			req:    &CommitFileRequest{Content: "test", Message: "test"},
			errMsg: "file path cannot be empty",
		},
		{
			name:   "empty content",
			req:    &CommitFileRequest{Path: "test.yaml", Message: "test"},
			errMsg: "file content cannot be empty",
		},
		{
			name:   "empty message",
			req:    &CommitFileRequest{Path: "test.yaml", Content: "test"},
			errMsg: "commit message cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := client.CommitWithIdempotency(context.Background(), tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

func TestCommitMultipleWithIdempotency_AllIdentical(t *testing.T) {
	// AC-IDEM-02: Skip commit when all files have identical content
	existingContent1 := "content 1"
	existingContent2 := "content 2"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/") {
			var content string
			if strings.Contains(r.URL.Path, "file1.yaml") {
				content = existingContent1
			} else {
				content = existingContent2
			}
			response := FileContent{
				SHA:     "sha123",
				Content: base64.StdEncoding.EncodeToString([]byte(content)),
				Path:    r.URL.Path,
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		t.Error("should not reach commit for identical content")
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	client.setBaseURLForTesting(server.URL)

	req := &CommitMultipleFilesRequest{
		Files: map[string]string{
			"file1.yaml": existingContent1,
			"file2.yaml": existingContent2,
		},
		Message: "Multi-file commit",
		Branch:  "main",
	}

	result, skipped, err := client.CommitMultipleWithIdempotency(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !skipped {
		t.Error("expected commit to be skipped for all identical files")
	}

	if result.Message != "skipped: all files identical" {
		t.Errorf("unexpected message: %s", result.Message)
	}
}

func TestCommitMultipleWithIdempotency_SomeDifferent(t *testing.T) {
	// When at least one file differs, commit should proceed
	existingContent := "existing"
	newContent := "new content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/") {
			if strings.Contains(r.URL.Path, "file1.yaml") {
				response := FileContent{
					SHA:     "sha123",
					Content: base64.StdEncoding.EncodeToString([]byte(existingContent)),
					Path:    "file1.yaml",
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			// file2 doesn't exist
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Handle Git Data API calls for multi-file commit
		if strings.Contains(r.URL.Path, "/git/refs/heads/") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ref": "refs/heads/main",
				"object": map[string]string{
					"sha": "basesha123",
				},
			})
			return
		}

		if strings.Contains(r.URL.Path, "/git/commits/") && r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"tree": map[string]string{
					"sha": "treesha123",
				},
			})
			return
		}

		if strings.Contains(r.URL.Path, "/git/blobs") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"sha": "blobsha"})
			return
		}

		if strings.Contains(r.URL.Path, "/git/trees") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"sha": "newtreesha"})
			return
		}

		if strings.Contains(r.URL.Path, "/git/commits") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"sha": "newcommitsha"})
			return
		}

		if strings.Contains(r.URL.Path, "/git/refs/heads/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"sha": "newcommitsha"})
			return
		}
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "test-token", "owner", "repo")
	client.setBaseURLForTesting(server.URL)

	req := &CommitMultipleFilesRequest{
		Files: map[string]string{
			"file1.yaml": existingContent, // Same as existing
			"file2.yaml": newContent,      // New file
		},
		Message: "Multi-file commit",
		Branch:  "main",
	}

	result, skipped, err := client.CommitMultipleWithIdempotency(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skipped {
		t.Error("commit should NOT be skipped when at least one file differs")
	}

	if result.SHA != "newcommitsha" {
		t.Errorf("expected SHA=newcommitsha, got %s", result.SHA)
	}
}

func TestDecodeBase64Content(t *testing.T) {
	// Test decoding base64 content with potential newlines
	original := "test content with unicode: héllo"
	encoded := base64.StdEncoding.EncodeToString([]byte(original))

	// Add newlines like GitHub might
	encodedWithNewlines := encoded[:10] + "\n" + encoded[10:]

	decoded := decodeBase64Content(encodedWithNewlines)
	if string(decoded) != original {
		t.Errorf("expected %q, got %q", original, string(decoded))
	}
}

func TestDecodeBase64WithNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean", "SGVsbG8gV29ybGQ=", "Hello World"},
		{"with newlines", "SGVs\nbG8g\nV29y\nbGQ=", "Hello World"},
		{"with spaces", "SGVs bG8g V29y bGQ=", "Hello World"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := decodeBase64WithNewlines(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(decoded) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(decoded))
			}
		})
	}
}
