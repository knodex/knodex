// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package vcs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/knodex/knodex/server/internal/netutil"
)

const (
	// GitHubAPIBaseURL is the base URL for GitHub API
	GitHubAPIBaseURL = "https://api.github.com"
	// DefaultTimeout for GitHub API requests
	DefaultTimeout = 30 * time.Second
)

// GitHubClient provides methods for interacting with GitHub API
type GitHubClient struct {
	httpClient  *http.Client
	owner       string
	repo        string
	baseURL     string
	closed      bool
	retryConfig RetryConfig     // AC-RETRY-01: Retry configuration
	rateLimit   *RateLimitState // AC-RATE-03: Rate limit state per repository credential
}

// Close releases resources and clears sensitive data from the client.
// SECURITY: Should be called via defer after creating a client to minimize
// token exposure time in memory. While Go's GC will eventually collect
// the memory, this explicitly nils references to reduce the exposure window.
func (c *GitHubClient) Close() {
	if c == nil || c.closed {
		return
	}
	// Nil out the HTTP client which holds the OAuth2 token source
	// This removes our reference to the token-bearing transport
	c.httpClient = nil
	c.closed = true
}

// Connection pooling constants
// AC-POOL-01: HTTP client uses connection pooling (MaxIdleConns: 100)
// AC-POOL-02: Idle connections per host: 10
// AC-POOL-03: Connection timeout: 10s, response header timeout: 30s
const (
	MaxIdleConns        = 100
	MaxIdleConnsPerHost = 10
	IdleConnTimeout     = 90 * time.Second
	ConnectTimeout      = 10 * time.Second
	ResponseTimeout     = 30 * time.Second
)

// NewGitHubClient creates a new GitHub client with the given token, owner, and repo
// AC-POOL-04: HTTP client reused across commits (not created per request)
func NewGitHubClient(ctx context.Context, token, owner, repo string) (*GitHubClient, error) {
	if token == "" {
		return nil, fmt.Errorf("github token cannot be empty")
	}
	if owner == "" {
		return nil, fmt.Errorf("github owner cannot be empty")
	}
	if repo == "" {
		return nil, fmt.Errorf("github repo cannot be empty")
	}

	// AC-POOL-01, AC-POOL-02, AC-POOL-03: Configure connection pooling
	// SECURITY: Uses SSRF-safe dialer to pin resolved IPs at connect time,
	// preventing DNS rebinding TOCTOU attacks against GitHub Enterprise URLs.
	transport := &http.Transport{
		MaxIdleConns:          MaxIdleConns,
		MaxIdleConnsPerHost:   MaxIdleConnsPerHost,
		IdleConnTimeout:       IdleConnTimeout,
		DialContext:           netutil.NewSSRFSafeDialer(),
		ResponseHeaderTimeout: ResponseTimeout,
		ForceAttemptHTTP2:     true,
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	// Wrap the transport with OAuth2
	oauthTransport := &oauth2.Transport{
		Source: ts,
		Base:   transport,
	}

	httpClient := &http.Client{
		Transport: oauthTransport,
		Timeout:   DefaultTimeout,
	}

	return &GitHubClient{
		httpClient:  httpClient,
		owner:       owner,
		repo:        repo,
		baseURL:     GitHubAPIBaseURL,
		retryConfig: DefaultRetryConfig, // AC-RETRY-01: Use default retry config
		rateLimit:   &RateLimitState{},  // AC-RATE-03: Initialize rate limit state
	}, nil
}

// setBaseURLForTesting allows tests to set localhost URLs for httptest servers.
// It also replaces the SSRF-safe transport with the default Go transport so
// connections to localhost httptest servers are not blocked.
// SECURITY: This function is package-private and should ONLY be used in tests.
func (c *GitHubClient) setBaseURLForTesting(rawURL string) {
	c.baseURL = strings.TrimSuffix(rawURL, "/")
	// Replace the SSRF-safe dialer so tests can reach localhost httptest servers.
	if oauthTransport, ok := c.httpClient.Transport.(*oauth2.Transport); ok {
		oauthTransport.Base = http.DefaultTransport
	}
}

// getRepository returns basic repository information
func (c *GitHubClient) getRepository(ctx context.Context) (*RepositoryInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, c.owner, c.repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxVCSResponseSize))
		return nil, fmt.Errorf("failed to get repository: status %d, body: %s", resp.StatusCode, string(body))
	}

	var info RepositoryInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode repository info: %w", err)
	}

	return &info, nil
}

// getFileContent retrieves a file's content and SHA from the repository
func (c *GitHubClient) getFileContent(ctx context.Context, path, branch string) (*FileContent, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, c.owner, c.repo, path)
	if branch != "" {
		url += "?ref=" + branch
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // File doesn't exist
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxVCSResponseSize))
		return nil, fmt.Errorf("failed to get file content: status %d, body: %s", resp.StatusCode, string(body))
	}

	var content FileContent
	if err := json.NewDecoder(resp.Body).Decode(&content); err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	return &content, nil
}

// commitFilePayload is the request payload for the GitHub API
type commitFilePayload struct {
	Message string `json:"message"`
	Content string `json:"content"`
	Branch  string `json:"branch,omitempty"`
	SHA     string `json:"sha,omitempty"`
}

// CommitFile creates or updates a single file in the repository
func (c *GitHubClient) CommitFile(ctx context.Context, req *CommitFileRequest) (*CommitResult, error) {
	if req.Path == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("file content cannot be empty")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("commit message cannot be empty")
	}

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, c.owner, c.repo, req.Path)

	// Encode content to base64
	encodedContent := base64.StdEncoding.EncodeToString([]byte(req.Content))

	payload := commitFilePayload{
		Message: req.Message,
		Content: encodedContent,
		Branch:  req.Branch,
		SHA:     req.SHA,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Content-Type", "application/json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to commit file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxVCSResponseSize))
		return nil, fmt.Errorf("failed to commit file: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response to get commit SHA
	var result struct {
		Commit struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode commit result: %w", err)
	}

	return &CommitResult{
		SHA:     result.Commit.SHA,
		Message: result.Commit.Message,
	}, nil
}

// ValidateToken checks if the token has access to the repository
func (c *GitHubClient) ValidateToken(ctx context.Context) error {
	_, err := c.getRepository(ctx)
	return err
}

// DeleteFile deletes a file from the repository
func (c *GitHubClient) DeleteFile(ctx context.Context, path, branch, message string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	if message == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	// First, get the file's SHA (required for deletion)
	fileContent, err := c.getFileContent(ctx, path, branch)
	if err != nil {
		return fmt.Errorf("failed to get file SHA: %w", err)
	}
	if fileContent == nil {
		// File doesn't exist, nothing to delete
		return nil
	}

	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, c.owner, c.repo, path)

	payload := map[string]string{
		"message": message,
		"sha":     fileContent.SHA,
	}
	if branch != "" {
		payload["branch"] = branch
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxVCSResponseSize))
		return fmt.Errorf("failed to delete file: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
