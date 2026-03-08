package vcs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/knodex/knodex/server/internal/netutil"
)

const (
	// GitLabAPIBaseURL is the base URL for GitLab API v4
	GitLabAPIBaseURL = "https://gitlab.com/api/v4"
	// DefaultGitLabTimeout for GitLab API requests
	DefaultGitLabTimeout = 30 * time.Second
)

// GitLabClient provides methods for interacting with GitLab API
// It implements the Client interface defined in interface.go
type GitLabClient struct {
	httpClient  *http.Client
	owner       string // GitLab namespace (user or group)
	repo        string // GitLab project name
	projectPath string // URL-encoded owner/repo for API calls
	baseURL     string
	closed      bool
	retryConfig RetryConfig     // Retry configuration
	rateLimit   *RateLimitState // Rate limit state
}

// gitLabErrorResponse represents a GitLab API error response
type gitLabErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Scope            string `json:"scope"`
	Message          string `json:"message"`
}

// parseGitLabError parses the response body and returns an appropriate error
// It detects scope/permission errors and returns a ScopeError with helpful guidance
func parseGitLabError(statusCode int, body []byte) error {
	var errResp gitLabErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// Not a JSON error, return generic error
		return fmt.Errorf("status %d: %s", statusCode, string(body))
	}

	// Check for insufficient scope error
	if errResp.Error == "insufficient_scope" {
		return &ScopeError{
			Provider:       ProviderGitLab,
			RequiredScopes: RequiredScopes(ProviderGitLab),
			Message: fmt.Sprintf(`GitLab API access denied: insufficient token scope.

Your token has insufficient permissions to access this repository via the API.
Required scopes: %s

%s

Error from GitLab: %s`,
				strings.Join(RequiredScopes(ProviderGitLab), ", "),
				ProviderScopeHelp(ProviderGitLab),
				errResp.ErrorDescription),
		}
	}

	// Check for 403 Forbidden which may indicate permission issues
	if statusCode == http.StatusForbidden {
		return &ScopeError{
			Provider:       ProviderGitLab,
			RequiredScopes: RequiredScopes(ProviderGitLab),
			Message: fmt.Sprintf(`GitLab API access forbidden (403).

This usually means your token does not have the required API scopes.
%s

Error details: %s`,
				ProviderScopeHelp(ProviderGitLab),
				string(body)),
		}
	}

	// Check for 401 Unauthorized
	if statusCode == http.StatusUnauthorized {
		return fmt.Errorf("GitLab authentication failed (401): token is invalid or expired")
	}

	// Return the message or error from response
	if errResp.Message != "" {
		return fmt.Errorf("GitLab API error (status %d): %s", statusCode, errResp.Message)
	}
	if errResp.Error != "" {
		return fmt.Errorf("GitLab API error (status %d): %s - %s", statusCode, errResp.Error, errResp.ErrorDescription)
	}

	return fmt.Errorf("GitLab API error (status %d): %s", statusCode, string(body))
}

// Close releases resources and clears sensitive data from the client.
// SECURITY: Should be called via defer after creating a client to minimize
// token exposure time in memory.
func (c *GitLabClient) Close() {
	if c == nil || c.closed {
		return
	}
	c.httpClient = nil
	c.closed = true
}

// NewGitLabClient creates a new GitLab client with the given token, owner, and repo
func NewGitLabClient(ctx context.Context, token, owner, repo string) (*GitLabClient, error) {
	if token == "" {
		return nil, fmt.Errorf("gitlab token cannot be empty")
	}
	if owner == "" {
		return nil, fmt.Errorf("gitlab owner cannot be empty")
	}
	if repo == "" {
		return nil, fmt.Errorf("gitlab repo cannot be empty")
	}

	// Configure connection pooling (same as GitHub client)
	// SECURITY: Uses SSRF-safe dialer to pin resolved IPs at connect time,
	// preventing DNS rebinding TOCTOU attacks against GitLab self-hosted URLs.
	transport := &http.Transport{
		MaxIdleConns:          MaxIdleConns,
		MaxIdleConnsPerHost:   MaxIdleConnsPerHost,
		IdleConnTimeout:       IdleConnTimeout,
		DialContext:           netutil.NewSSRFSafeDialer(),
		ResponseHeaderTimeout: ResponseTimeout,
		ForceAttemptHTTP2:     true,
	}

	// Use OAuth2 bearer token for GitLab
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	oauthTransport := &oauth2.Transport{
		Source: ts,
		Base:   transport,
	}

	httpClient := &http.Client{
		Transport: oauthTransport,
		Timeout:   DefaultGitLabTimeout,
	}

	// GitLab uses URL-encoded project path (owner/repo)
	projectPath := url.PathEscape(owner + "/" + repo)

	return &GitLabClient{
		httpClient:  httpClient,
		owner:       owner,
		repo:        repo,
		projectPath: projectPath,
		baseURL:     GitLabAPIBaseURL,
		retryConfig: DefaultRetryConfig,
		rateLimit:   &RateLimitState{},
	}, nil
}

// SetBaseURL allows overriding the base URL (useful for GitLab self-hosted)
// SECURITY: Validates URL to prevent SSRF attacks
func (c *GitLabClient) SetBaseURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("base URL cannot be empty")
	}

	parsedURL, err := url.Parse(strings.TrimSuffix(rawURL, "/"))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("URL scheme must be http or https, got %q", parsedURL.Scheme)
	}

	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// SECURITY: Block private/internal hosts (covers localhost, loopback, RFC1918, etc.)
	// Uses shared netutil.IsPrivateHost which is fail-closed (rejects on DNS failure)
	if netutil.IsPrivateHost(host) {
		return fmt.Errorf("SSRF protection: private or internal hosts are not allowed")
	}

	c.baseURL = parsedURL.String()
	return nil
}

// setBaseURLForTesting allows tests to set localhost URLs for httptest servers.
// It also replaces the SSRF-safe transport with the default Go transport so
// connections to localhost httptest servers are not blocked.
// SECURITY: This function is package-private and should ONLY be used in tests.
func (c *GitLabClient) setBaseURLForTesting(rawURL string) {
	c.baseURL = strings.TrimSuffix(rawURL, "/")
	// Replace the SSRF-safe dialer so tests can reach localhost httptest servers.
	if oauthTransport, ok := c.httpClient.Transport.(*oauth2.Transport); ok {
		oauthTransport.Base = http.DefaultTransport
	}
}

// GetRepository returns basic repository information
func (c *GitLabClient) GetRepository(ctx context.Context) (*RepositoryInfo, error) {
	url := fmt.Sprintf("%s/projects/%s", c.baseURL, c.projectPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseGitLabError(resp.StatusCode, body)
	}

	var project struct {
		DefaultBranch     string `json:"default_branch"`
		PathWithNamespace string `json:"path_with_namespace"`
		Visibility        string `json:"visibility"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("failed to decode project info: %w", err)
	}

	return &RepositoryInfo{
		DefaultBranch: project.DefaultBranch,
		FullName:      project.PathWithNamespace,
		Private:       project.Visibility == "private",
	}, nil
}

// GetFileContent retrieves a file's content and SHA from the repository
func (c *GitLabClient) GetFileContent(ctx context.Context, path, branch string) (*FileContent, error) {
	// GitLab requires URL-encoded path
	encodedPath := url.PathEscape(path)
	apiURL := fmt.Sprintf("%s/projects/%s/repository/files/%s", c.baseURL, c.projectPath, encodedPath)
	if branch != "" {
		apiURL += "?ref=" + url.QueryEscape(branch)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // File doesn't exist
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseGitLabError(resp.StatusCode, body)
	}

	var file struct {
		BlobID   string `json:"blob_id"`
		Content  string `json:"content"`
		FilePath string `json:"file_path"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&file); err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	// GitLab returns base64-encoded content
	content := file.Content
	if file.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 content: %w", err)
		}
		content = string(decoded)
	}

	return &FileContent{
		SHA:     file.BlobID,
		Content: content,
		Path:    file.FilePath,
	}, nil
}

// CommitFile creates or updates a single file in the repository
func (c *GitLabClient) CommitFile(ctx context.Context, req *CommitFileRequest) (*CommitResult, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("file content cannot be empty")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("commit message cannot be empty")
	}

	// Check if file exists to determine if this is a create or update
	existingFile, err := c.GetFileContent(ctx, req.Path, req.Branch)
	if err != nil {
		return nil, fmt.Errorf("failed to check if file exists: %w", err)
	}

	encodedPath := url.PathEscape(req.Path)
	apiURL := fmt.Sprintf("%s/projects/%s/repository/files/%s", c.baseURL, c.projectPath, encodedPath)

	// Encode content to base64
	encodedContent := base64.StdEncoding.EncodeToString([]byte(req.Content))

	payload := map[string]string{
		"branch":         req.Branch,
		"content":        encodedContent,
		"commit_message": req.Message,
		"encoding":       "base64",
	}

	var method string
	if existingFile == nil {
		// Create new file
		method = http.MethodPost
	} else {
		// Update existing file
		method = http.MethodPut
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, apiURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to commit file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseGitLabError(resp.StatusCode, body)
	}

	// GitLab doesn't return the commit SHA in the file API response
	// We need to get the latest commit to retrieve the SHA
	var result struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode commit result: %w", err)
	}

	// Get the latest commit SHA
	commitSHA, err := c.getLatestCommitSHA(ctx, req.Branch)
	if err != nil {
		slog.Warn("failed to get commit SHA after file commit", "error", err)
		// Return success even if we can't get the SHA
		return &CommitResult{
			SHA:     "",
			Message: req.Message,
		}, nil
	}

	return &CommitResult{
		SHA:     commitSHA,
		Message: req.Message,
	}, nil
}

// getLatestCommitSHA retrieves the latest commit SHA for a branch
func (c *GitLabClient) getLatestCommitSHA(ctx context.Context, branch string) (string, error) {
	apiURL := fmt.Sprintf("%s/projects/%s/repository/commits", c.baseURL, c.projectPath)
	if branch != "" {
		apiURL += "?ref_name=" + url.QueryEscape(branch) + "&per_page=1"
	} else {
		apiURL += "?per_page=1"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", parseGitLabError(resp.StatusCode, body)
	}

	var commits []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return "", err
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found")
	}

	return commits[0].ID, nil
}

// CommitMultipleFiles commits multiple files in a single commit using GitLab's Commits API
func (c *GitLabClient) CommitMultipleFiles(ctx context.Context, req *CommitMultipleFilesRequest) (*CommitResult, error) {
	if len(req.Files) == 0 {
		return nil, fmt.Errorf("no files to commit")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("commit message cannot be empty")
	}

	// Get the repository info to find default branch if not specified
	branch := req.Branch
	if branch == "" {
		repoInfo, err := c.GetRepository(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get repository info: %w", err)
		}
		branch = repoInfo.DefaultBranch
	}

	apiURL := fmt.Sprintf("%s/projects/%s/repository/commits", c.baseURL, c.projectPath)

	// Build actions for each file
	actions := make([]map[string]string, 0, len(req.Files))
	for path, content := range req.Files {
		// Check if file exists to determine action type
		existingFile, err := c.GetFileContent(ctx, path, branch)
		if err != nil {
			return nil, fmt.Errorf("failed to check if file exists at %s: %w", path, err)
		}

		action := "create"
		if existingFile != nil {
			action = "update"
		}

		actions = append(actions, map[string]string{
			"action":    action,
			"file_path": path,
			"content":   content,
		})
	}

	payload := map[string]interface{}{
		"branch":         branch,
		"commit_message": req.Message,
		"actions":        actions,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to commit files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseGitLabError(resp.StatusCode, body)
	}

	var result struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode commit result: %w", err)
	}

	slog.Info("committed multiple files to GitLab",
		"owner", c.owner,
		"repo", c.repo,
		"branch", branch,
		"sha", result.ID,
		"files", len(req.Files),
	)

	return &CommitResult{
		SHA:     result.ID,
		Message: result.Message,
	}, nil
}

// CommitWithIdempotency commits a file with idempotency checking
// Returns (result, skipped, error) where skipped=true if content already matches
func (c *GitLabClient) CommitWithIdempotency(ctx context.Context, req *CommitFileRequest) (*CommitResult, bool, error) {
	// Check if file exists and has same content
	existingFile, err := c.GetFileContent(ctx, req.Path, req.Branch)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check existing file: %w", err)
	}

	// If file exists and content matches, skip the commit
	if existingFile != nil && existingFile.Content == req.Content {
		slog.Info("skipping commit - content unchanged",
			"path", req.Path,
			"branch", req.Branch,
		)
		return &CommitResult{
			SHA:     existingFile.SHA,
			Message: "skipped: content unchanged",
		}, true, nil
	}

	// Proceed with commit
	result, err := c.CommitFile(ctx, req)
	if err != nil {
		return nil, false, err
	}

	return result, false, nil
}

// DeleteFile deletes a file from the repository
func (c *GitLabClient) DeleteFile(ctx context.Context, path, branch, message string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	// Check if file exists
	existingFile, err := c.GetFileContent(ctx, path, branch)
	if err != nil {
		return fmt.Errorf("failed to check if file exists: %w", err)
	}
	if existingFile == nil {
		// File doesn't exist, nothing to delete
		return nil
	}

	encodedPath := url.PathEscape(path)
	apiURL := fmt.Sprintf("%s/projects/%s/repository/files/%s", c.baseURL, c.projectPath, encodedPath)

	payload := map[string]string{
		"branch":         branch,
		"commit_message": message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return parseGitLabError(resp.StatusCode, body)
	}

	return nil
}

// ValidateToken checks if the token has access to the repository
func (c *GitLabClient) ValidateToken(ctx context.Context) error {
	_, err := c.GetRepository(ctx)
	return err
}

// GetRateLimitState returns the current rate limit state
func (c *GitLabClient) GetRateLimitState() (remaining, limit int, resetTime time.Time) {
	if c.rateLimit == nil {
		return 0, 0, time.Time{}
	}

	c.rateLimit.mu.RLock()
	defer c.rateLimit.mu.RUnlock()

	return c.rateLimit.Remaining, c.rateLimit.Limit, c.rateLimit.Reset
}

// doWithRetry executes an HTTP request with retry logic and rate limit tracking
func (c *GitLabClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Check rate limit before making request
	if err := c.checkRateLimit(); err != nil {
		return nil, err
	}

	var lastErr error

	// Capture request body for retries (body can only be read once)
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	for attempt := 0; attempt < c.retryConfig.MaxAttempts; attempt++ {
		// Context cancellation stops retry loop immediately
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Clone request and restore body for each attempt
		reqClone := req.Clone(ctx)
		if bodyBytes != nil {
			reqClone.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
			reqClone.ContentLength = int64(len(bodyBytes))
		}

		resp, err := c.httpClient.Do(reqClone)
		if err != nil {
			lastErr = err
			// Check if error is retryable (network errors, timeouts)
			if !isRetryableError(err) {
				return nil, err
			}
			c.waitBeforeRetry(ctx, attempt)
			continue
		}

		// Update rate limit state from headers
		c.updateRateLimit(resp)

		// Retry on 5xx server errors
		if resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: %d, body: %s", resp.StatusCode, truncateBody(body))
			c.waitBeforeRetry(ctx, attempt)
			continue
		}

		// Retry on 429 (Too Many Requests)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limited: retry after %s", retryAfter)
			c.waitForRateLimit(ctx, retryAfter)
			continue
		}

		// Success or non-retryable client error
		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded (%d attempts): %w", c.retryConfig.MaxAttempts, lastErr)
}

// waitBeforeRetry waits with exponential backoff before the next retry attempt
func (c *GitLabClient) waitBeforeRetry(ctx context.Context, attempt int) {
	delay := c.retryConfig.BaseDelay * time.Duration(1<<uint(attempt))
	if delay > c.retryConfig.MaxDelay {
		delay = c.retryConfig.MaxDelay
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
		return
	}
}

// waitForRateLimit waits for the rate limit to reset
func (c *GitLabClient) waitForRateLimit(ctx context.Context, retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = c.retryConfig.BaseDelay
	}

	// Cap the wait time to prevent excessive waits
	if retryAfter > c.retryConfig.MaxDelay*10 {
		retryAfter = c.retryConfig.MaxDelay * 10
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(retryAfter):
		return
	}
}

// updateRateLimit updates the rate limit state from response headers
// GitLab uses RateLimit-Remaining and RateLimit-Limit headers
func (c *GitLabClient) updateRateLimit(resp *http.Response) {
	if c.rateLimit == nil {
		return
	}

	remaining, _ := strconv.Atoi(resp.Header.Get("RateLimit-Remaining"))
	limit, _ := strconv.Atoi(resp.Header.Get("RateLimit-Limit"))
	resetUnix, _ := strconv.ParseInt(resp.Header.Get("RateLimit-Reset"), 10, 64)

	c.rateLimit.mu.Lock()
	defer c.rateLimit.mu.Unlock()

	c.rateLimit.Remaining = remaining
	c.rateLimit.Limit = limit
	if resetUnix > 0 {
		c.rateLimit.Reset = time.Unix(resetUnix, 0)
	}
}

// checkRateLimit checks if we're below the rate limit threshold
func (c *GitLabClient) checkRateLimit() error {
	if c.rateLimit == nil {
		return nil
	}

	c.rateLimit.mu.RLock()
	defer c.rateLimit.mu.RUnlock()

	// No rate limit info yet - proceed normally
	if c.rateLimit.Limit == 0 {
		return nil
	}

	threshold := float64(c.rateLimit.Limit) * RateLimitThreshold
	if float64(c.rateLimit.Remaining) < threshold {
		waitTime := time.Until(c.rateLimit.Reset)
		if waitTime < 0 {
			waitTime = 0
		}

		return &RateLimitError{
			Remaining: c.rateLimit.Remaining,
			Reset:     c.rateLimit.Reset,
			WaitTime:  waitTime,
		}
	}

	return nil
}

// GetOwner returns the repository owner (for testing)
func (c *GitLabClient) GetOwner() string {
	return c.owner
}

// GetRepo returns the repository name (for testing)
func (c *GitLabClient) GetRepo() string {
	return c.repo
}

// GetRetryConfig returns the retry configuration (for testing)
func (c *GitLabClient) GetRetryConfig() RetryConfig {
	return c.retryConfig
}
