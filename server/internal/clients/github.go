// Package clients provides client wrappers for external services.
package clients

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultSecretsNamespace is the default namespace where GitHub secrets are stored
	DefaultSecretsNamespace = "kro-system"

	// Security limits for file operations
	maxFilePath      = 255              // Maximum file path length
	maxFileSize      = 10 * 1024 * 1024 // 10MB maximum file size
	maxCommitMessage = 65536            // 64KB maximum commit message length
)

var (
	// githubTokenPattern validates GitHub token formats
	// Supports: ghp_ (personal), gho_ (OAuth), ghu_ (user), ghs_ (server), ghr_ (refresh)
	// Fixed: Non-backtracking pattern to prevent ReDoS attacks
	githubTokenPattern = regexp.MustCompile(`^ghp_[a-zA-Z0-9]{36}$|^gho_[a-zA-Z0-9]{36}$|^ghu_[a-zA-Z0-9]{36}$|^ghs_[a-zA-Z0-9]{36}$|^ghr_[a-zA-Z0-9]{76}$`)

	// Input validation patterns
	validRepoName   = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	validBranchName = regexp.MustCompile(`^[a-zA-Z0-9_./\-]+$`)
)

// GitHubClient wraps the Kubernetes client for GitHub credential management
type GitHubClient struct {
	k8sClient  kubernetes.Interface
	namespace  string
	httpClient *http.Client // Shared HTTP client with proper timeouts and connection limits
}

// GitHubCredentials contains references to a Kubernetes secret containing GitHub credentials
type GitHubCredentials struct {
	SecretName string // Name of the Kubernetes secret
	SecretKey  string // Key within the secret that contains the GitHub token
	Namespace  string // Namespace of the secret (optional, uses default if empty)
}

// NewGitHubClient creates a new GitHub client wrapper
func NewGitHubClient(k8sClient kubernetes.Interface) *GitHubClient {
	// Create a shared HTTP client with proper timeouts and connection limits
	// to prevent resource exhaustion from unbounded client creation
	transport := &http.Transport{
		MaxIdleConns:        100,              // Max idle connections across all hosts
		MaxIdleConnsPerHost: 10,               // Max idle connections per host
		IdleConnTimeout:     90 * time.Second, // How long idle connections stay open
		TLSHandshakeTimeout: 10 * time.Second, // Timeout for TLS handshake
	}

	httpClient := &http.Client{
		Timeout:   30 * time.Second, // Overall request timeout
		Transport: transport,
	}

	return &GitHubClient{
		k8sClient:  k8sClient,
		namespace:  DefaultSecretsNamespace,
		httpClient: httpClient,
	}
}

// SetNamespace sets the default namespace for reading secrets
func (c *GitHubClient) SetNamespace(namespace string) {
	c.namespace = namespace
}

// GetCredentials reads a GitHub token from a Kubernetes secret
func (c *GitHubClient) GetCredentials(ctx context.Context, creds GitHubCredentials) (string, error) {
	namespace := creds.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	secret, err := c.k8sClient.CoreV1().Secrets(namespace).Get(
		ctx, creds.SecretName, metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to read secret %s in namespace %s: %w",
			creds.SecretName, namespace, err)
	}

	tokenBytes, ok := secret.Data[creds.SecretKey]
	if !ok {
		availableKeys := getSecretKeys(secret.Data)
		return "", fmt.Errorf("key %s not found in secret %s (available keys: %v)",
			creds.SecretKey, creds.SecretName, availableKeys)
	}

	token := string(tokenBytes)
	if token == "" {
		return "", fmt.Errorf("token value is empty in secret %s key %s",
			creds.SecretName, creds.SecretKey)
	}

	// Validate token format to prevent injection attacks and catch configuration errors early
	if !githubTokenPattern.MatchString(token) {
		return "", fmt.Errorf("GitHub token has invalid format (must be ghp_, gho_, ghu_, ghs_, or ghr_ followed by correct length)")
	}

	return token, nil
}

// NewClientWithCredentials creates an authenticated GitHub client using credentials from Kubernetes secret
func (c *GitHubClient) NewClientWithCredentials(ctx context.Context, creds GitHubCredentials) (*github.Client, error) {
	token, err := c.GetCredentials(ctx, creds)
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	// Use shared transport with token auth instead of creating new HTTP client
	// This prevents resource exhaustion from unbounded client creation
	tc := &http.Client{
		Timeout: c.httpClient.Timeout,
		Transport: &oauth2.Transport{
			Base:   c.httpClient.Transport,
			Source: ts,
		},
	}

	return github.NewClient(tc), nil
}

// ValidateToken validates that a GitHub token is valid by making a test API call
func (c *GitHubClient) ValidateToken(ctx context.Context, client *github.Client) error {
	// Try to get the authenticated user - this will fail if token is invalid
	_, resp, err := client.Users.Get(ctx, "")
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return fmt.Errorf("GitHub token is invalid or expired: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// TestConnection tests GitHub repository access with the provided credentials
func (c *GitHubClient) TestConnection(ctx context.Context, creds GitHubCredentials, owner, repo string) error {
	client, err := c.NewClientWithCredentials(ctx, creds)
	if err != nil {
		return fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Validate token first
	if err := c.ValidateToken(ctx, client); err != nil {
		return err
	}

	// Check if context was canceled before making next API call
	// This prevents wasted GitHub API calls when client disconnects
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Continue
	}

	// Test repository access
	repository, resp, err := client.Repositories.Get(ctx, owner, repo)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("repository %s/%s not found or token does not have access", owner, repo)
		}
		return fmt.Errorf("failed to access repository %s/%s: %w", owner, repo, err)
	}

	// Check if token has write permissions
	if repository.Permissions == nil {
		return fmt.Errorf("unable to determine repository permissions for %s/%s", owner, repo)
	}

	hasPushAccess, exists := repository.Permissions["push"]
	if !exists || !hasPushAccess {
		return fmt.Errorf("token does not have write permissions to repository %s/%s", owner, repo)
	}

	return nil
}

// CommitFiles commits one or more files to a GitHub repository in a single commit
// This method handles directory creation automatically and supports both single and multi-file commits
func (c *GitHubClient) CommitFiles(ctx context.Context, client *github.Client, owner, repo, branch string, files map[string]string, message string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("GitHub client cannot be nil")
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no files provided to commit")
	}

	// Validate input parameters to prevent injection attacks
	if err := validateInput(owner, repo, branch); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Sanitize commit message to prevent injection
	sanitizedMessage, err := sanitizeCommitMessage(message)
	if err != nil {
		return "", fmt.Errorf("invalid commit message: %w", err)
	}

	// Validate all file paths and enforce size limits
	for path, content := range files {
		if err := validateFilePath(path); err != nil {
			return "", fmt.Errorf("invalid file path: %w", err)
		}
		if len(content) > maxFileSize {
			return "", fmt.Errorf("file %s exceeds maximum size of %d bytes", path, maxFileSize)
		}
	}

	// Get branch reference
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
	if err != nil {
		return "", fmt.Errorf("failed to get branch ref: %w", err)
	}

	// Get base tree - this is needed to create a new tree
	baseTree, _, err := client.Git.GetTree(ctx, owner, repo, ref.Object.GetSHA(), true)
	if err != nil {
		return "", fmt.Errorf("failed to get base tree: %w", err)
	}

	// Create tree entries for all files
	// Directories are created automatically by the GitHub API
	entries := []*github.TreeEntry{}
	for path, content := range files {
		entries = append(entries, &github.TreeEntry{
			Path:    github.String(path),
			Mode:    github.String("100644"), // Regular file
			Type:    github.String("blob"),
			Content: github.String(content),
		})
	}

	// Create new tree with our entries
	tree, _, err := client.Git.CreateTree(ctx, owner, repo, baseTree.GetSHA(), entries)
	if err != nil {
		return "", fmt.Errorf("failed to create tree: %w", err)
	}

	// Create commit with sanitized message
	parent := ref.Object
	commit, _, err := client.Git.CreateCommit(ctx, owner, repo, &github.Commit{
		Message: github.String(sanitizedMessage),
		Tree:    tree,
		Parents: []*github.Commit{{SHA: parent.SHA}},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	// Update reference to point to new commit
	// This is where conflicts can occur if the ref has changed
	ref.Object.SHA = commit.SHA
	_, _, err = client.Git.UpdateRef(ctx, owner, repo, ref, false)
	if err != nil {
		return "", fmt.Errorf("failed to update ref (possible conflict): %w", err)
	}

	return commit.GetSHA(), nil
}

// CommitFile commits a single file to a GitHub repository
// This is a convenience wrapper around CommitFiles for single file commits
func (c *GitHubClient) CommitFile(ctx context.Context, client *github.Client, owner, repo, branch, path, content, message string) (string, error) {
	files := map[string]string{
		path: content,
	}
	return c.CommitFiles(ctx, client, owner, repo, branch, files, message)
}

// GetDefaultBranch retrieves the default branch name for a repository
func (c *GitHubClient) GetDefaultBranch(ctx context.Context, client *github.Client, owner, repo string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("GitHub client cannot be nil")
	}
	repository, resp, err := client.Repositories.Get(ctx, owner, repo)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return "", fmt.Errorf("failed to get repository: %w", err)
	}

	defaultBranch := repository.GetDefaultBranch()
	if defaultBranch == "" {
		return "", fmt.Errorf("repository has no default branch configured")
	}

	return defaultBranch, nil
}

// validateFilePath validates file paths to prevent directory traversal and other attacks
func validateFilePath(path string) error {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// Reject absolute paths
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute paths not allowed: %s", path)
	}

	// Reject paths that try to traverse outside the repository
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/../") {
		return fmt.Errorf("directory traversal not allowed: %s", path)
	}

	// Reject paths with null bytes (potential injection)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null bytes in path not allowed: %s", path)
	}

	// Enforce maximum path length
	if len(cleanPath) > maxFilePath {
		return fmt.Errorf("path too long (max %d characters): %s", maxFilePath, path)
	}

	return nil
}

// validateInput validates owner, repo, and branch names to prevent injection attacks
func validateInput(owner, repo, branch string) error {
	if owner == "" || repo == "" || branch == "" {
		return fmt.Errorf("owner, repo, and branch cannot be empty")
	}

	if len(owner) > 39 {
		return fmt.Errorf("owner name too long (max 39 characters)")
	}

	if len(repo) > 100 {
		return fmt.Errorf("repo name too long (max 100 characters)")
	}

	if len(branch) > 255 {
		return fmt.Errorf("branch name too long (max 255 characters)")
	}

	if !validRepoName.MatchString(owner) {
		return fmt.Errorf("invalid owner name format: %s", owner)
	}

	if !validRepoName.MatchString(repo) {
		return fmt.Errorf("invalid repo name format: %s", repo)
	}

	if !validBranchName.MatchString(branch) {
		return fmt.Errorf("invalid branch name format: %s", branch)
	}

	return nil
}

// sanitizeCommitMessage sanitizes commit messages to prevent injection attacks
func sanitizeCommitMessage(message string) (string, error) {
	if message == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	// Enforce maximum length
	if len(message) > maxCommitMessage {
		return "", fmt.Errorf("commit message too long (max %d characters)", maxCommitMessage)
	}

	// Remove null bytes
	message = strings.ReplaceAll(message, "\x00", "")

	// Remove control characters except newline, tab, and carriage return
	var sanitized strings.Builder
	for _, r := range message {
		if r >= 32 || r == '\n' || r == '\t' || r == '\r' {
			sanitized.WriteRune(r)
		}
	}

	result := sanitized.String()
	if result == "" {
		return "", fmt.Errorf("commit message contains only invalid characters")
	}

	return result, nil
}

// getSecretKeys returns a list of keys available in a secret's data map
func getSecretKeys(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}
