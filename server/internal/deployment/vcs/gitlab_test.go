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

func TestNewGitLabClient(t *testing.T) {
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
			token:   "glpat-valid_token",
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
			errMsg:  "gitlab token cannot be empty",
		},
		{
			name:    "empty owner",
			token:   "glpat-valid_token",
			owner:   "",
			repo:    "test-repo",
			wantErr: true,
			errMsg:  "gitlab owner cannot be empty",
		},
		{
			name:    "empty repo",
			token:   "glpat-valid_token",
			owner:   "test-owner",
			repo:    "",
			wantErr: true,
			errMsg:  "gitlab repo cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewGitLabClient(ctx, tt.token, tt.owner, tt.repo)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewGitLabClient() expected error, got nil")
				}
				if err.Error() != tt.errMsg {
					t.Errorf("NewGitLabClient() error = %v, want %v", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("NewGitLabClient() unexpected error: %v", err)
				return
			}
			if client == nil {
				t.Error("NewGitLabClient() returned nil client")
			}
			if client.owner != tt.owner {
				t.Errorf("NewGitLabClient() owner = %v, want %v", client.owner, tt.owner)
			}
			if client.repo != tt.repo {
				t.Errorf("NewGitLabClient() repo = %v, want %v", client.repo, tt.repo)
			}
			// Verify projectPath is URL-encoded
			expectedPath := tt.owner + "%2F" + tt.repo
			if client.projectPath != expectedPath {
				t.Errorf("NewGitLabClient() projectPath = %v, want %v", client.projectPath, expectedPath)
			}
		})
	}
}

func TestGitLabClient_SetBaseURL(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitLabClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
	}

	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid HTTPS URL with trailing slash",
			url:     "https://gitlab.com/api/v4/",
			wantErr: false,
		},
		{
			name:    "valid HTTPS URL without trailing slash",
			url:     "https://gitlab.com/api/v4",
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
			name:    "invalid scheme",
			url:     "ftp://gitlab.com",
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

func TestGitLabClient_GetRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GitLab uses URL-encoded project path
			// Check RawPath for encoded path (Path is automatically decoded by Go)
			expectedRawPath := "/projects/test-owner%2Ftest-repo"
			if r.URL.RawPath != expectedRawPath {
				t.Errorf("unexpected raw path: %s, want %s", r.URL.RawPath, expectedRawPath)
			}
			if r.Method != http.MethodGet {
				t.Errorf("unexpected method: %s", r.Method)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"default_branch":      "main",
				"path_with_namespace": "test-owner/test-repo",
				"visibility":          "private",
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
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
		if !info.Private {
			t.Error("GetRepository() expected private=true")
		}
	})

	t.Run("not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message": "404 Project Not Found"}`))
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		_, err := client.GetRepository(ctx)
		if err == nil {
			t.Error("GetRepository() expected error for not found")
		}
	})
}

func TestGitLabClient_GetFileContent(t *testing.T) {
	t.Run("file exists", func(t *testing.T) {
		encodedContent := base64.StdEncoding.EncodeToString([]byte("file content"))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GitLab uses URL-encoded paths
			// Check RawPath for encoded path (Path is automatically decoded by Go)
			expectedRawPath := "/projects/test-owner%2Ftest-repo/repository/files/path%2Fto%2Ffile.yaml"
			if r.URL.RawPath != expectedRawPath {
				t.Errorf("unexpected raw path: %s, want %s", r.URL.RawPath, expectedRawPath)
			}
			if r.URL.Query().Get("ref") != "main" {
				t.Errorf("unexpected ref: %s", r.URL.Query().Get("ref"))
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"blob_id":   "abc123",
				"content":   encodedContent,
				"file_path": "path/to/file.yaml",
				"encoding":  "base64",
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
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
		if content.Content != "file content" {
			t.Errorf("GetFileContent() Content = %v, want 'file content'", content.Content)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
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

func TestGitLabClient_CommitFile(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")

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
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++

			// First request: check if file exists (GET)
			if requestCount == 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			// Second request: create file (POST)
			if requestCount == 2 {
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				var payload map[string]string
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if payload["commit_message"] != "Add app manifest" {
					t.Errorf("unexpected message: %s", payload["commit_message"])
				}
				if payload["branch"] != "main" {
					t.Errorf("unexpected branch: %s", payload["branch"])
				}

				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]string{
					"branch": "main",
				})
				return
			}

			// Third request: get latest commit (GET commits)
			if requestCount == 3 {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]map[string]string{
					{"id": "abc123def456"},
				})
				return
			}
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
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

	t.Run("api error", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount == 1 {
				// File doesn't exist
				w.WriteHeader(http.StatusNotFound)
				return
			}
			// Create fails
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte(`{"message": "Validation failed"}`))
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
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

func TestGitLabClient_CommitMultipleFiles_Validation(t *testing.T) {
	ctx := context.Background()
	client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")

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

func TestGitLabClient_ValidateToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"default_branch":      "main",
				"path_with_namespace": "test-owner/test-repo",
				"visibility":          "private",
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		err := client.ValidateToken(ctx)
		if err != nil {
			t.Errorf("ValidateToken() unexpected error: %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message": "401 Unauthorized"}`))
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "invalid-token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		err := client.ValidateToken(ctx)
		if err == nil {
			t.Error("ValidateToken() expected error for invalid token")
		}
	})
}

func TestGitLabClient_DeleteFile(t *testing.T) {
	t.Run("validation errors", func(t *testing.T) {
		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")

		_, err := client.CommitFile(ctx, &CommitFileRequest{Path: "", Content: "x", Message: "x"})
		if err == nil || !strings.Contains(err.Error(), "file path cannot be empty") {
			t.Errorf("expected path validation error, got: %v", err)
		}
	})

	t.Run("file not found - no error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		err := client.DeleteFile(ctx, "nonexistent.yaml", "main", "Delete file")
		if err != nil {
			t.Errorf("DeleteFile() expected no error for nonexistent file, got: %v", err)
		}
	})

	t.Run("delete success", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++

			// First request: check if file exists (GET)
			if requestCount == 1 {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"blob_id":   "abc123",
					"content":   base64.StdEncoding.EncodeToString([]byte("content")),
					"file_path": "file.yaml",
					"encoding":  "base64",
				})
				return
			}

			// Second request: delete file (DELETE)
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE method, got: %s", r.Method)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		err := client.DeleteFile(ctx, "file.yaml", "main", "Delete file")
		if err != nil {
			t.Errorf("DeleteFile() error: %v", err)
		}
	})
}

func TestGitLabClient_Close(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitLabClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
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
	var nilClient *GitLabClient
	nilClient.Close() // Should not panic
}

func TestGitLabClient_CommitWithIdempotency(t *testing.T) {
	t.Run("content unchanged - skip", func(t *testing.T) {
		existingContent := "existing content"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"blob_id":   "existing-sha",
				"content":   base64.StdEncoding.EncodeToString([]byte(existingContent)),
				"file_path": "file.yaml",
				"encoding":  "base64",
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		result, skipped, err := client.CommitWithIdempotency(ctx, &CommitFileRequest{
			Path:    "file.yaml",
			Content: existingContent,
			Message: "Update file",
			Branch:  "main",
		})
		if err != nil {
			t.Fatalf("CommitWithIdempotency() error: %v", err)
		}
		if !skipped {
			t.Error("expected skipped=true for unchanged content")
		}
		if result.SHA != "existing-sha" {
			t.Errorf("expected existing SHA, got: %s", result.SHA)
		}
	})

	t.Run("content changed - commit", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++

			// First: check existing content
			if requestCount == 1 {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"blob_id":   "old-sha",
					"content":   base64.StdEncoding.EncodeToString([]byte("old content")),
					"file_path": "file.yaml",
					"encoding":  "base64",
				})
				return
			}

			// Second: check for update (file exists)
			if requestCount == 2 {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"blob_id":   "old-sha",
					"content":   base64.StdEncoding.EncodeToString([]byte("old content")),
					"file_path": "file.yaml",
					"encoding":  "base64",
				})
				return
			}

			// Third: commit file
			if requestCount == 3 {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"branch": "main"})
				return
			}

			// Fourth: get latest commit
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]string{
				{"id": "new-sha"},
			})
		}))
		defer server.Close()

		ctx := context.Background()
		client, _ := NewGitLabClient(ctx, "token", "test-owner", "test-repo")
		client.setBaseURLForTesting(server.URL)

		result, skipped, err := client.CommitWithIdempotency(ctx, &CommitFileRequest{
			Path:    "file.yaml",
			Content: "new content",
			Message: "Update file",
			Branch:  "main",
		})
		if err != nil {
			t.Fatalf("CommitWithIdempotency() error: %v", err)
		}
		if skipped {
			t.Error("expected skipped=false for changed content")
		}
		if result.SHA != "new-sha" {
			t.Errorf("expected new SHA, got: %s", result.SHA)
		}
	})
}

func TestGitLabClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewGitLabClient(context.Background(), "token", "test-owner", "test-repo")
	client.setBaseURLForTesting(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetRepository(ctx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestGitLabClient_GetOwnerAndRepo(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitLabClient(ctx, "token", "my-owner", "my-repo")
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
	}

	if client.GetOwner() != "my-owner" {
		t.Errorf("GetOwner() = %v, want my-owner", client.GetOwner())
	}
	if client.GetRepo() != "my-repo" {
		t.Errorf("GetRepo() = %v, want my-repo", client.GetRepo())
	}
}

func TestGitLabClient_GetRetryConfig(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitLabClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
	}

	config := client.GetRetryConfig()
	if config.MaxAttempts != DefaultRetryConfig.MaxAttempts {
		t.Errorf("GetRetryConfig().MaxAttempts = %v, want %v", config.MaxAttempts, DefaultRetryConfig.MaxAttempts)
	}
}

func TestGitLabClient_GetRateLimitState(t *testing.T) {
	ctx := context.Background()
	client, err := NewGitLabClient(ctx, "token", "owner", "repo")
	if err != nil {
		t.Fatalf("NewGitLabClient() error: %v", err)
	}

	remaining, limit, resetTime := client.GetRateLimitState()
	// Initial state should be zeros
	if remaining != 0 || limit != 0 {
		t.Logf("Initial rate limit state: remaining=%d, limit=%d, resetTime=%v", remaining, limit, resetTime)
	}
}
