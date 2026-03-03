package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/repository"
)

// newTestRequest creates a test HTTP request with user context pre-set.
func newTestRequest(t *testing.T, method, url string, body interface{}) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(method, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &middleware.UserContext{
		UserID: "admin@test.local",
		Email:  "admin@test.local",
		Groups: []string{"knodex-admins"},
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	return req.WithContext(ctx)
}

// TestCreateRepositoryConfig_RejectsInsecureSkipTLS tests that
// insecureSkipTLSVerify: true is rejected with 400 Bad Request.
func TestCreateRepositoryConfig_RejectsInsecureSkipTLS(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	reqBody := CreateRepositoryConfigRequest{
		Name:          "test-repo",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/repo.git",
		AuthType:      repository.AuthTypeHTTPS,
		DefaultBranch: "main",
		Enabled:       true,
		HTTPSAuth: &repository.HTTPSAuthConfig{
			BearerToken:           "ghp_test",
			InsecureSkipTLSVerify: true,
		},
	}

	req := newTestRequest(t, http.MethodPost, "/api/v1/repositories", reqBody)
	rec := httptest.NewRecorder()
	handler.CreateRepositoryConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Code != response.ErrCodeBadRequest {
		t.Errorf("expected error code %s, got %s", response.ErrCodeBadRequest, errResp.Code)
	}
}

// TestCreateRepositoryConfig_AllowsSecureTLS tests that HTTPS without
// insecureSkipTLSVerify proceeds past validation (it will fail later at auth check
// since we have no enforcer, but the point is it passes TLS validation).
func TestCreateRepositoryConfig_AllowsSecureTLS(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	reqBody := CreateRepositoryConfigRequest{
		Name:          "test-repo",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/repo.git",
		AuthType:      repository.AuthTypeHTTPS,
		DefaultBranch: "main",
		Enabled:       true,
		HTTPSAuth: &repository.HTTPSAuthConfig{
			BearerToken:           "ghp_test",
			InsecureSkipTLSVerify: false,
		},
	}

	req := newTestRequest(t, http.MethodPost, "/api/v1/repositories", reqBody)
	rec := httptest.NewRecorder()
	handler.CreateRepositoryConfig(rec, req)

	// Should NOT be 400 (it will be 403 since no enforcer, but not 400 for TLS)
	if rec.Code == http.StatusBadRequest {
		var errResp response.ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		if errResp.Message == "insecureSkipTLSVerify is not allowed; use a trusted CA certificate instead" {
			t.Error("secure TLS config was incorrectly rejected")
		}
	}
}

// TestCreateRepositoryConfig_SSHAuth tests POST /api/v1/repositories with SSH auth body.
func TestCreateRepositoryConfig_SSHAuth(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	reqBody := CreateRepositoryConfigRequest{
		Name:          "ssh-repo",
		ProjectID:     "alpha",
		RepoURL:       "git@github.com:example/repo.git",
		AuthType:      repository.AuthTypeSSH,
		DefaultBranch: "main",
		Enabled:       true,
		SSHAuth: &repository.SSHAuthConfig{
			PrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
		},
	}

	req := newTestRequest(t, http.MethodPost, "/api/v1/repositories", reqBody)
	rec := httptest.NewRecorder()
	handler.CreateRepositoryConfig(rec, req)

	// Should pass validation and fail at enforcer (403) since we have no enforcer
	// The key check is it doesn't fail with 400
	if rec.Code == http.StatusBadRequest {
		var errResp response.ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		t.Errorf("SSH auth was incorrectly rejected with 400: %s", errResp.Message)
	}
}

// TestCreateRepositoryConfig_HTTPSAuth tests POST /api/v1/repositories with HTTPS bearer token body.
func TestCreateRepositoryConfig_HTTPSAuth(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	reqBody := CreateRepositoryConfigRequest{
		Name:          "https-repo",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/repo.git",
		AuthType:      repository.AuthTypeHTTPS,
		DefaultBranch: "main",
		Enabled:       true,
		HTTPSAuth: &repository.HTTPSAuthConfig{
			BearerToken: "ghp_test_token_123",
		},
	}

	req := newTestRequest(t, http.MethodPost, "/api/v1/repositories", reqBody)
	rec := httptest.NewRecorder()
	handler.CreateRepositoryConfig(rec, req)

	// Should pass validation and fail at enforcer (403) since we have no enforcer
	if rec.Code == http.StatusBadRequest {
		var errResp response.ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		t.Errorf("HTTPS auth was incorrectly rejected with 400: %s", errResp.Message)
	}
}

// TestCreateRepositoryConfig_GitHubAppAuth tests POST /api/v1/repositories with GitHub App body.
func TestCreateRepositoryConfig_GitHubAppAuth(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	reqBody := CreateRepositoryConfigRequest{
		Name:          "ghapp-repo",
		ProjectID:     "alpha",
		RepoURL:       "https://github.com/example/repo.git",
		AuthType:      repository.AuthTypeGitHubApp,
		DefaultBranch: "main",
		Enabled:       true,
		GitHubAppAuth: &repository.GitHubAppAuthConfig{
			AppType:        "github",
			AppID:          "12345",
			InstallationID: "67890",
			PrivateKey:     "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
		},
	}

	req := newTestRequest(t, http.MethodPost, "/api/v1/repositories", reqBody)
	rec := httptest.NewRecorder()
	handler.CreateRepositoryConfig(rec, req)

	// Should pass validation and fail at enforcer (403) since we have no enforcer
	if rec.Code == http.StatusBadRequest {
		var errResp response.ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		t.Errorf("GitHub App auth was incorrectly rejected with 400: %s", errResp.Message)
	}
}

// TestCreateRepositoryConfig_LegacyFormatRejected tests that POST with legacy owner/repo fields returns 400.
func TestCreateRepositoryConfig_LegacyFormatRejected(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	// Send a request with legacy fields
	body := map[string]interface{}{
		"name":          "legacy-repo",
		"owner":         "example-org",
		"repo":          "my-repo",
		"defaultBranch": "main",
		"secretName":    "my-secret",
		"secretKey":     "token",
		"enabled":       true,
	}

	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/repositories", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	userCtx := &middleware.UserContext{
		UserID: "admin@test.local",
		Email:  "admin@test.local",
		Groups: []string{"knodex-admins"},
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.CreateRepositoryConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for legacy format, got %d", http.StatusBadRequest, rec.Code)
	}

	var errResp response.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Code != response.ErrCodeBadRequest {
		t.Errorf("expected error code %s, got %s", response.ErrCodeBadRequest, errResp.Code)
	}

	expectedMsg := "Legacy format (owner/repo/secretName) is no longer supported. Use repoURL/authType format."
	if errResp.Message != expectedMsg {
		t.Errorf("expected message %q, got %q", expectedMsg, errResp.Message)
	}
}

// TestTestConnection_ArgoCD tests POST /api/v1/repositories/test-connection with ArgoCD format body.
func TestTestConnection_ArgoCD(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	reqBody := repository.TestConnectionWithCredentialsRequest{
		RepoURL:  "https://github.com/example/repo.git",
		AuthType: repository.AuthTypeHTTPS,
		HTTPSAuth: &repository.HTTPSAuthConfig{
			BearerToken: "ghp_test_token",
		},
	}

	req := newTestRequest(t, http.MethodPost, "/api/v1/repositories/test-connection", reqBody)
	rec := httptest.NewRecorder()
	handler.TestConnection(rec, req)

	// Without a real service, this will fail at service call (nil pointer).
	// The test validates that validation passes and request is correctly parsed.
	// A nil repoService will cause a panic — the important assertion is that
	// the request is NOT rejected with 400 for format issues.
	// To avoid the panic, we just check it doesn't return 400.
	// Real service integration is tested in E2E tests.
	if rec.Code == http.StatusBadRequest {
		var errResp response.ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		t.Errorf("ArgoCD format test connection was incorrectly rejected with 400: %s", errResp.Message)
	}
}

// TestCreateRepositoryConfig_MissingRequiredFields tests that missing required fields return 400.
func TestCreateRepositoryConfig_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	handler := NewRepositoryHandler(nil, nil, nil, nil)

	tests := []struct {
		name    string
		body    CreateRepositoryConfigRequest
		wantMsg string
	}{
		{
			name:    "missing name",
			body:    CreateRepositoryConfigRequest{ProjectID: "alpha", RepoURL: "https://github.com/x/y.git", AuthType: "https", DefaultBranch: "main"},
			wantMsg: "name is required",
		},
		{
			name:    "missing projectId",
			body:    CreateRepositoryConfigRequest{Name: "test", RepoURL: "https://github.com/x/y.git", AuthType: "https", DefaultBranch: "main"},
			wantMsg: "projectId is required",
		},
		{
			name:    "missing repoURL",
			body:    CreateRepositoryConfigRequest{Name: "test", ProjectID: "alpha", AuthType: "https", DefaultBranch: "main"},
			wantMsg: "repoURL is required",
		},
		{
			name:    "missing authType",
			body:    CreateRepositoryConfigRequest{Name: "test", ProjectID: "alpha", RepoURL: "https://github.com/x/y.git", DefaultBranch: "main"},
			wantMsg: "authType is required",
		},
		{
			name:    "missing defaultBranch",
			body:    CreateRepositoryConfigRequest{Name: "test", ProjectID: "alpha", RepoURL: "https://github.com/x/y.git", AuthType: "https"},
			wantMsg: "defaultBranch is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := newTestRequest(t, http.MethodPost, "/api/v1/repositories", tt.body)
			rec := httptest.NewRecorder()
			handler.CreateRepositoryConfig(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}

			var errResp response.ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if errResp.Message != tt.wantMsg {
				t.Errorf("expected message %q, got %q", tt.wantMsg, errResp.Message)
			}
		})
	}
}
