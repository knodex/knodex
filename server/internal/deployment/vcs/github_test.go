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
	"time"
)

func TestNewGitHubClient(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		token   string
		owner   string
		repo    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid parameters",
			token:   "ghp_valid_token",
			owner:   "test-owner",
			repo:    "test-repo",
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			owner:   "test-owner",
			repo:    "test-repo",
			wantErr: true,
			errMsg:  "github token cannot be empty",
		},
		{
			name:    "empty owner",
			token:   "ghp_valid_token",
			owner:   "",
			repo:    "test-repo",
			wantErr: true,
			errMsg:  "github owner cannot be empty",
		},
		{
			name:    "empty repo",
			token:   "ghp_valid_token",
			owner:   "test-owner",
			repo:    "",
			wantErr: true,
			errMsg:  "github repo cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewGitHubClient(ctx, tt.token, tt.owner, tt.repo)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewGitHubClient() expected error, got nil")
				}
				if err.Error() != tt.errMsg {
					t.Errorf("NewGitHubClient() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("NewGitHubClient() unexpected error: %v", err)
				return
			}
			if client == nil {
				t.Error("NewGitHubClient() returned nil client")
			}
			if client.owner != tt.owner {
				t.Errorf("NewGitHubClient() owner = %v, want %v", client.owner, tt.owner)
			}
			if client.repo != tt.repo {
				t.Errorf("NewGitHubClient() repo = %v, want %v", client.repo, tt.repo)
			}
		})
	}
}

func TestClient_SetBaseURL(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitHubClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitHubClient() error: %v", err)
	}

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid HTTPS URL with trailing slash",
			url:     "https://github.com/api/v3/",
			wantErr: false,
		},
		{
			name:    "valid HTTPS URL without trailing slash",
			url:     "https://github.com/api/v3",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
			errMsg:  "base URL cannot be empty",
		},
		{
			name:    "localhost blocked",
			url:     "http://localhost:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "127.0.0.1 blocked",
			url:     "http://127.0.0.1:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "private IP 10.x.x.x blocked",
			url:     "http://10.0.0.1:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "private IP 192.168.x.x blocked",
			url:     "http://192.168.1.1:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "private IP 172.16.x.x blocked",
			url:     "http://172.16.0.1:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "invalid scheme",
			url:     "ftp://github.com",
			wantErr: true,
			errMsg:  "URL scheme must be http or https, got \"ftp\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SetBaseURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SetBaseURL() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("SetBaseURL() error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("SetBaseURL() unexpected error: %v", err)
			}
		})
	}
}

func TestClient_GetRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/test-owner/test-repo" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodGet {
				t.Errorf("unexpected method: %s", r.Method)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RepositoryInfo{
				DefaultBranch: "main",
				FullName:      "test-owner/test-repo",
				Private:       false,
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		info, err := client.GetRepository(ctx)
		if err != nil {
			t.Fatalf("GetRepository() error: %v", err)
		}
		if info.DefaultBranch != "main" {
			t.Errorf("GetRepository() default_branch = %v, want main", info.DefaultBranch)
		}
		if info.FullName != "test-owner/test-repo" {
			t.Errorf("GetRepository() full_name = %v, want test-owner/test-repo", info.FullName)
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message": "Not Found"}`))
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		_, err := client.GetRepository(ctx)
		if err == nil {
			t.Error("GetRepository() expected error for not found")
		}
	})
}

func TestClient_GetFileContent(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		encodedContent := base64.StdEncoding.EncodeToString([]byte("file content"))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/test-owner/test-repo/contents/path/to/file.yaml" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.URL.Query().Get("ref") != "main" {
				t.Errorf("unexpected ref: %s", r.URL.Query().Get("ref"))
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(FileContent{
				SHA:     "abc123",
				Content: encodedContent,
				Path:    "path/to/file.yaml",
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		content, err := client.GetFileContent(ctx, "path/to/file.yaml", "main")
		if err != nil {
			t.Fatalf("GetFileContent() error: %v", err)
		}
		if content == nil {
			t.Fatal("GetFileContent() returned nil")
		}
		if content.SHA != "abc123" {
			t.Errorf("GetFileContent() SHA = %v, want abc123", content.SHA)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		content, err := client.GetFileContent(ctx, "nonexistent.yaml", "main")
		if err != nil {
			t.Fatalf("GetFileContent() unexpected error: %v", err)
		}
		if content != nil {
			t.Errorf("GetFileContent() expected nil for not found, got %v", content)
		}
	})
}

func TestClient_CommitFile(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")

		tests := []struct {
			name    string
			req     *CommitFileRequest
			wantErr string
		}{
			{
				name:    "empty path",
				req:     &CommitFileRequest{Path: "", Content: "content", Message: "msg"},
				wantErr: "file path cannot be empty",
			},
			{
				name:    "empty content",
				req:     &CommitFileRequest{Path: "file.yaml", Content: "", Message: "msg"},
				wantErr: "file content cannot be empty",
			},
			{
				name:    "empty message",
				req:     &CommitFileRequest{Path: "file.yaml", Content: "content", Message: ""},
				wantErr: "commit message cannot be empty",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := client.CommitFile(ctx, tt.req)
				if err == nil {
					t.Error("expected error")
					return
				}
				if err.Error() != tt.wantErr {
					t.Errorf("CommitFile() error = %v, want %v", err.Error(), tt.wantErr)
				}
			})
		}
	})

	t.Run("create new file success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("unexpected method: %s", r.Method)
			}
			if r.URL.Path != "/repos/test-owner/test-repo/contents/instances/default/app.yaml" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			// Verify request body
			var payload commitFilePayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("failed to decode request body: %v", err)
			}
			if payload.Message != "Add app manifest" {
				t.Errorf("unexpected message: %s", payload.Message)
			}
			if payload.Branch != "main" {
				t.Errorf("unexpected branch: %s", payload.Branch)
			}

			// Decode and verify content
			decoded, _ := base64.StdEncoding.DecodeString(payload.Content)
			if string(decoded) != "apiVersion: v1\nkind: Application" {
				t.Errorf("unexpected content: %s", string(decoded))
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"commit": map[string]string{
					"sha":     "abc123def456",
					"message": "Add app manifest",
				},
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		result, err := client.CommitFile(ctx, &CommitFileRequest{
			Path:    "instances/default/app.yaml",
			Content: "apiVersion: v1\nkind: Application",
			Message: "Add app manifest",
			Branch:  "main",
		})
		if err != nil {
			t.Fatalf("CommitFile() error: %v", err)
		}
		if result.SHA != "abc123def456" {
			t.Errorf("CommitFile() SHA = %v, want abc123def456", result.SHA)
		}
	})

	t.Run("update existing file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload commitFilePayload
			json.NewDecoder(r.Body).Decode(&payload)

			// Verify SHA is included for update
			if payload.SHA != "existing-sha" {
				t.Errorf("expected SHA for update, got: %s", payload.SHA)
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"commit": map[string]string{
					"sha":     "new-sha-after-update",
					"message": "Update manifest",
				},
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		result, err := client.CommitFile(ctx, &CommitFileRequest{
			Path:    "file.yaml",
			Content: "updated content",
			Message: "Update manifest",
			Branch:  "main",
			SHA:     "existing-sha",
		})
		if err != nil {
			t.Fatalf("CommitFile() error: %v", err)
		}
		if result.SHA != "new-sha-after-update" {
			t.Errorf("CommitFile() SHA = %v, want new-sha-after-update", result.SHA)
		}
	})

	t.Run("api error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"message": "Validation failed"}`))
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		_, err := client.CommitFile(ctx, &CommitFileRequest{
			Path:    "file.yaml",
			Content: "content",
			Message: "msg",
			Branch:  "main",
		})
		if err == nil {
			t.Error("expected error for API failure")
		}
	})
}

func TestClient_CommitMultipleFiles_Validation(t *testing.T) {
	ctx := context.Background()
	client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")

	t.Run("no files", func(t *testing.T) {
		_, err := client.CommitMultipleFiles(ctx, &CommitMultipleFilesRequest{
			Files:   map[string]string{},
			Message: "msg",
		})
		if err == nil {
			t.Error("expected error for empty files")
		}
		if err.Error() != "no files to commit" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty message", func(t *testing.T) {
		_, err := client.CommitMultipleFiles(ctx, &CommitMultipleFilesRequest{
			Files:   map[string]string{"file.yaml": "content"},
			Message: "",
		})
		if err == nil {
			t.Error("expected error for empty message")
		}
		if err.Error() != "commit message cannot be empty" {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestClient_ValidateToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(RepositoryInfo{
				DefaultBranch: "main",
				FullName:      "test-owner/test-repo",
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		err := client.ValidateToken(ctx)
		if err != nil {
			t.Errorf("ValidateToken() unexpected error: %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "Bad credentials"}`))
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitHubClient(ctx, "invalid-token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		err := client.ValidateToken(ctx)
		if err == nil {
			t.Error("ValidateToken() expected error for invalid token")
		}
	})
}

func TestRepositoryInfo_Fields(t *testing.T) {
	info := RepositoryInfo{
		DefaultBranch: "main",
		FullName:      "owner/repo",
		Private:       true,
	}

	if info.DefaultBranch != "main" {
		t.Errorf("DefaultBranch = %v, want main", info.DefaultBranch)
	}
	if info.FullName != "owner/repo" {
		t.Errorf("FullName = %v, want owner/repo", info.FullName)
	}
	if !info.Private {
		t.Error("Private should be true")
	}
}

func TestFileContent_Fields(t *testing.T) {
	fc := FileContent{
		SHA:     "sha123",
		Content: "base64content",
		Path:    "path/to/file",
	}

	if fc.SHA != "sha123" {
		t.Errorf("SHA = %v, want sha123", fc.SHA)
	}
	if fc.Content != "base64content" {
		t.Errorf("Content = %v, want base64content", fc.Content)
	}
	if fc.Path != "path/to/file" {
		t.Errorf("Path = %v, want path/to/file", fc.Path)
	}
}

func TestCommitResult_Fields(t *testing.T) {
	cr := CommitResult{
		SHA:     "commitsha",
		Message: "commit message",
	}

	if cr.SHA != "commitsha" {
		t.Errorf("SHA = %v, want commitsha", cr.SHA)
	}
	if cr.Message != "commit message" {
		t.Errorf("Message = %v, want commit message", cr.Message)
	}
}

func TestClient_Close(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitHubClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitHubClient() error: %v", err)
	}

	// Verify client has httpClient before close
	if client.httpClient == nil {
		t.Fatal("expected httpClient to be non-nil before Close()")
	}

	// Call Close
	client.Close()

	// Verify httpClient is nil after close
	if client.httpClient != nil {
		t.Error("expected httpClient to be nil after Close()")
	}

	// Verify Close is idempotent
	client.Close() // Should not panic

	// Verify Close on nil client doesn't panic
	var nilClient *GitHubClient
	nilClient.Close() // Should not panic
}

// HIGH PRIORITY: IPv6 SSRF Protection Tests
func TestClient_SetBaseURL_IPv6SSRF(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitHubClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitHubClient() error: %v", err)
	}

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "IPv6 localhost ::1",
			url:     "http://[::1]:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "IPv6 localhost full form",
			url:     "http://[0:0:0:0:0:0:0:1]:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "IPv6 link-local",
			url:     "http://[fe80::1]:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "IPv6 unique local (fc00::)",
			url:     "http://[fc00::1]:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "IPv6 unique local (fd00::)",
			url:     "http://[fd00::1]:8080",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
		{
			name:    "Cloud metadata AWS (169.254.169.254)",
			url:     "http://169.254.169.254/latest/meta-data/",
			wantErr: true,
			errMsg:  "SSRF protection: private or internal hosts are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.SetBaseURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SetBaseURL(%q) expected error, got nil", tt.url)
					return
				}
				// Check for security-related error
				errStr := err.Error()
				if !strings.Contains(errStr, "SSRF protection: private or internal hosts are not allowed") {
					t.Errorf("SetBaseURL(%q) error = %q, want SSRF protection error", tt.url, errStr)
				}
				return
			}
			if err != nil {
				t.Errorf("SetBaseURL(%q) unexpected error: %v", tt.url, err)
			}
		})
	}
}

// MEDIUM PRIORITY: Rate Limiting Tests
func TestClient_CommitFile_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.Header().Set("Retry-After", "1") // Short retry time for testing
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	// Use a short timeout to test retry exhaustion without waiting too long
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
	client.setBaseURLForTesting(server.URL)

	_, err := client.CommitFile(ctx, &CommitFileRequest{
		Path:    "test.yaml",
		Content: "content",
		Message: "test commit",
		Branch:  "main",
	})

	if err == nil {
		t.Error("expected error for rate limited request")
	}
	// Either we get a rate limit error or context timeout/canceled - both are acceptable
	errStr := err.Error()
	if !strings.Contains(errStr, "429") && !strings.Contains(errStr, "rate limit") && !strings.Contains(errStr, "context") && !strings.Contains(errStr, "max retries") {
		t.Errorf("expected rate limit or context error, got: %v", err)
	}
}

func TestClient_GetRepository_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1") // Short retry time for testing
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	// Use a short timeout to test retry exhaustion without waiting too long
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
	client.setBaseURLForTesting(server.URL)

	_, err := client.GetRepository(ctx)
	if err == nil {
		t.Error("expected error for rate limited request")
	}
}

// MEDIUM PRIORITY: Context Cancellation Tests
func TestClient_CommitFile_ContextCancellation(t *testing.T) {
	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "token", "test-owner", "test-repo")
	client.setBaseURLForTesting(server.URL)

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	_, err := client.CommitFile(ctx, &CommitFileRequest{
		Path:    "test.yaml",
		Content: "content",
		Message: "test commit",
		Branch:  "main",
	})

	if err == nil {
		t.Error("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}

func TestClient_CommitFile_ContextTimeout(t *testing.T) {
	// Server that delays response longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "token", "test-owner", "test-repo")
	client.setBaseURLForTesting(server.URL)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.CommitFile(ctx, &CommitFileRequest{
		Path:    "test.yaml",
		Content: "content",
		Message: "test commit",
		Branch:  "main",
	})

	if err == nil {
		t.Error("expected error for context timeout")
	}
	// Error could be "context deadline exceeded" or "context canceled"
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got: %v", err)
	}
}

func TestClient_GetRepository_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewGitHubClient(context.Background(), "token", "test-owner", "test-repo")
	client.setBaseURLForTesting(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetRepository(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// Test server error responses
func TestClient_CommitFile_ServerErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"Internal Server Error", http.StatusInternalServerError, `{"message": "Internal Server Error"}`},
		{"Bad Gateway", http.StatusBadGateway, `{"message": "Bad Gateway"}`},
		{"Service Unavailable", http.StatusServiceUnavailable, `{"message": "Service Unavailable"}`},
		{"Forbidden", http.StatusForbidden, `{"message": "Resource not accessible by integration"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			ctx := context.Background()
			client, _ := NewGitHubClient(ctx, "token", "test-owner", "test-repo")
			client.setBaseURLForTesting(server.URL)

			_, err := client.CommitFile(ctx, &CommitFileRequest{
				Path:    "test.yaml",
				Content: "content",
				Message: "test commit",
				Branch:  "main",
			})

			if err == nil {
				t.Errorf("expected error for status %d", tt.statusCode)
			}
		})
	}
}
