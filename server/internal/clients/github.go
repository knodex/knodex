// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package clients provides client wrappers for external services.
package clients

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultSecretsNamespace is the default namespace where GitHub secrets are stored
	DefaultSecretsNamespace = "kro-system"
)

var (
	// githubTokenPattern validates GitHub token formats
	// Supports: ghp_ (personal), gho_ (OAuth), ghu_ (user), ghs_ (server), ghr_ (refresh)
	// Fixed: Non-backtracking pattern to prevent ReDoS attacks
	githubTokenPattern = regexp.MustCompile(`^ghp_[a-zA-Z0-9]{36}$|^gho_[a-zA-Z0-9]{36}$|^ghu_[a-zA-Z0-9]{36}$|^ghs_[a-zA-Z0-9]{36}$|^ghr_[a-zA-Z0-9]{76}$`)
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

// getSecretKeys returns a list of keys available in a secret's data map
func getSecretKeys(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	return keys
}
