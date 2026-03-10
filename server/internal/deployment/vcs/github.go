// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

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
// It implements the Client interface defined in interface.go
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

// SetBaseURL allows overriding the base URL (useful for GitHub Enterprise or testing)
// SECURITY: Validates URL to prevent SSRF attacks against internal networks
func (c *GitHubClient) SetBaseURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("base URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(strings.TrimSuffix(rawURL, "/"))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Require HTTPS for security (except localhost in tests)
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("URL scheme must be http or https, got %q", parsedURL.Scheme)
	}

	// Get the hostname
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
// Production code should use SetBaseURL which validates against SSRF.
func (c *GitHubClient) setBaseURLForTesting(rawURL string) {
	c.baseURL = strings.TrimSuffix(rawURL, "/")
	// Replace the SSRF-safe dialer so tests can reach localhost httptest servers.
	if oauthTransport, ok := c.httpClient.Transport.(*oauth2.Transport); ok {
		oauthTransport.Base = http.DefaultTransport
	}
}

// GetRepository returns basic repository information
func (c *GitHubClient) GetRepository(ctx context.Context) (*RepositoryInfo, error) {
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get repository: status %d, body: %s", resp.StatusCode, string(body))
	}

	var info RepositoryInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode repository info: %w", err)
	}

	return &info, nil
}

// GetFileContent retrieves a file's content and SHA from the repository
func (c *GitHubClient) GetFileContent(ctx context.Context, path, branch string) (*FileContent, error) {
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
		body, _ := io.ReadAll(resp.Body)
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
		body, _ := io.ReadAll(resp.Body)
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

// CommitMultipleFiles commits multiple files in a single commit using the Git Data API
func (c *GitHubClient) CommitMultipleFiles(ctx context.Context, req *CommitMultipleFilesRequest) (*CommitResult, error) {
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

	// Step 1: Get the reference for the branch
	ref, err := c.getRef(ctx, branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get branch reference: %w", err)
	}

	// Step 2: Get the current commit
	baseTree, err := c.getCommitTree(ctx, ref.Object.SHA)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit tree: %w", err)
	}

	// Step 3: Create blobs for each file
	treeEntries := make([]treeEntry, 0, len(req.Files))
	for path, content := range req.Files {
		blob, err := c.createBlob(ctx, content)
		if err != nil {
			return nil, fmt.Errorf("failed to create blob for %s: %w", path, err)
		}
		treeEntries = append(treeEntries, treeEntry{
			Path: path,
			Mode: "100644",
			Type: "blob",
			SHA:  blob.SHA,
		})
	}

	// Step 4: Create new tree
	newTree, err := c.createTree(ctx, baseTree, treeEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to create tree: %w", err)
	}

	// Step 5: Create commit
	commit, err := c.createCommit(ctx, req.Message, newTree.SHA, ref.Object.SHA)
	if err != nil {
		return nil, fmt.Errorf("failed to create commit: %w", err)
	}

	// Step 6: Update reference
	if err := c.updateRef(ctx, branch, commit.SHA); err != nil {
		return nil, fmt.Errorf("failed to update reference: %w", err)
	}

	slog.Info("committed multiple files to GitHub",
		"owner", c.owner,
		"repo", c.repo,
		"branch", branch,
		"sha", commit.SHA,
		"files", len(req.Files),
	)

	return &CommitResult{
		SHA:     commit.SHA,
		Message: req.Message,
	}, nil
}

// Git Data API types
type gitRef struct {
	Ref    string `json:"ref"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
	} `json:"object"`
}

type gitBlob struct {
	SHA string `json:"sha"`
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

type gitTree struct {
	SHA string `json:"sha"`
}

type gitCommit struct {
	SHA string `json:"sha"`
}

// getRef gets a reference by branch name
func (c *GitHubClient) getRef(ctx context.Context, branch string) (*gitRef, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", c.baseURL, c.owner, c.repo, branch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get ref: status %d, body: %s", resp.StatusCode, string(body))
	}

	var ref gitRef
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return nil, err
	}

	return &ref, nil
}

// getCommitTree gets the tree SHA from a commit
func (c *GitHubClient) getCommitTree(ctx context.Context, commitSHA string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/commits/%s", c.baseURL, c.owner, c.repo, commitSHA)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get commit: status %d, body: %s", resp.StatusCode, string(body))
	}

	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return "", err
	}

	return commit.Tree.SHA, nil
}

// createBlob creates a blob for file content
func (c *GitHubClient) createBlob(ctx context.Context, content string) (*gitBlob, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/blobs", c.baseURL, c.owner, c.repo)

	payload := map[string]string{
		"content":  content,
		"encoding": "utf-8",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create blob: status %d, body: %s", resp.StatusCode, string(body))
	}

	var blob gitBlob
	if err := json.NewDecoder(resp.Body).Decode(&blob); err != nil {
		return nil, err
	}

	return &blob, nil
}

// createTree creates a new tree with the given entries
func (c *GitHubClient) createTree(ctx context.Context, baseTreeSHA string, entries []treeEntry) (*gitTree, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees", c.baseURL, c.owner, c.repo)

	payload := map[string]interface{}{
		"base_tree": baseTreeSHA,
		"tree":      entries,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create tree: status %d, body: %s", resp.StatusCode, string(body))
	}

	var tree gitTree
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, err
	}

	return &tree, nil
}

// createCommit creates a new commit
func (c *GitHubClient) createCommit(ctx context.Context, message, treeSHA, parentSHA string) (*gitCommit, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/git/commits", c.baseURL, c.owner, c.repo)

	payload := map[string]interface{}{
		"message": message,
		"tree":    treeSHA,
		"parents": []string{parentSHA},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create commit: status %d, body: %s", resp.StatusCode, string(body))
	}

	var commit gitCommit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return nil, err
	}

	return &commit, nil
}

// updateRef updates a reference to point to a new commit
func (c *GitHubClient) updateRef(ctx context.Context, branch, sha string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", c.baseURL, c.owner, c.repo, branch)

	payload := map[string]interface{}{
		"sha":   sha,
		"force": false,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	// Use doWithRetry for rate limit tracking and retry logic
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update ref: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ValidateToken checks if the token has access to the repository
func (c *GitHubClient) ValidateToken(ctx context.Context) error {
	_, err := c.GetRepository(ctx)
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
	fileContent, err := c.GetFileContent(ctx, path, branch)
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete file: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}
