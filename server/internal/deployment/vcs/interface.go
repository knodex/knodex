// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package vcs

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Provider represents a VCS provider type
type Provider string

const (
	// ProviderGitHub represents GitHub
	ProviderGitHub Provider = "github"
	// ProviderGitLab represents GitLab
	ProviderGitLab Provider = "gitlab"
	// ProviderBitbucket represents Bitbucket
	ProviderBitbucket Provider = "bitbucket"
)

// Client defines the interface for interacting with a Version Control System
// All VCS providers (GitHub, GitLab, Bitbucket) must implement this interface
type Client interface {
	// GetRepository returns basic repository information
	GetRepository(ctx context.Context) (*RepositoryInfo, error)

	// GetFileContent retrieves a file's content and SHA from the repository
	GetFileContent(ctx context.Context, path, branch string) (*FileContent, error)

	// CommitFile creates or updates a single file in the repository
	CommitFile(ctx context.Context, req *CommitFileRequest) (*CommitResult, error)

	// CommitMultipleFiles commits multiple files in a single commit
	CommitMultipleFiles(ctx context.Context, req *CommitMultipleFilesRequest) (*CommitResult, error)

	// CommitWithIdempotency commits a file with idempotency checking
	// Returns (result, skipped, error) where skipped=true if content already matches
	CommitWithIdempotency(ctx context.Context, req *CommitFileRequest) (*CommitResult, bool, error)

	// CommitMultipleWithIdempotency commits multiple files with idempotency checking
	CommitMultipleWithIdempotency(ctx context.Context, req *CommitMultipleFilesRequest) (*CommitResult, bool, error)

	// DeleteFile deletes a file from the repository
	DeleteFile(ctx context.Context, path, branch, message string) error

	// ValidateToken checks if the token has access to the repository
	ValidateToken(ctx context.Context) error

	// GetRateLimitState returns the current rate limit state
	GetRateLimitState() (remaining, limit int, resetTime time.Time)

	// Close releases resources and clears sensitive data
	Close()
}

// RepositoryInfo contains basic repository information
type RepositoryInfo struct {
	DefaultBranch string `json:"default_branch"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
}

// FileContent represents file content from a VCS
type FileContent struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
	Path    string `json:"path"`
}

// CommitFileRequest contains parameters for committing a file
type CommitFileRequest struct {
	Path    string
	Content string
	Message string
	Branch  string
	SHA     string // Required for updates, empty for new files
}

// CommitResult contains the result of a commit operation
type CommitResult struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// CommitMultipleFilesRequest contains parameters for committing multiple files
type CommitMultipleFilesRequest struct {
	Files   map[string]string // path -> content
	Message string
	Branch  string
}

// ScopeError represents an error when a token has insufficient scopes/permissions
type ScopeError struct {
	Provider       Provider
	RequiredScopes []string
	Message        string
}

func (e *ScopeError) Error() string {
	return e.Message
}

// RequiredScopes returns the scopes required for each provider to perform GitOps operations
func RequiredScopes(provider Provider) []string {
	switch provider {
	case ProviderGitHub:
		return []string{"repo"}
	case ProviderGitLab:
		return []string{"api"}
	case ProviderBitbucket:
		return []string{"repository:write"}
	default:
		return nil
	}
}

// ProviderScopeHelp returns user-friendly help text for obtaining proper scopes
func ProviderScopeHelp(provider Provider) string {
	switch provider {
	case ProviderGitHub:
		return `GitHub token requires the "repo" scope for private repositories or "public_repo" for public repositories.
Create a Personal Access Token at: https://github.com/settings/tokens/new
Required scopes: repo (Full control of private repositories)`
	case ProviderGitLab:
		return `GitLab token requires the "api" scope for full API access, or at minimum "read_repository" + "write_repository".
Create a Personal Access Token at: https://gitlab.com/-/user_settings/personal_access_tokens
Required scopes: api (Grants complete read/write access to the API)
Note: Tokens with only Git-over-HTTP access (no API scope) are not supported for GitOps operations.`
	case ProviderBitbucket:
		return `Bitbucket App Password requires "Repositories: Write" permission.
Create an App Password at: https://bitbucket.org/account/settings/app-passwords/`
	default:
		return ""
	}
}

// RepoURL contains parsed repository URL information
type RepoURL struct {
	Provider Provider
	Owner    string
	Repo     string
	Host     string // e.g., "github.com", "gitlab.com", or custom host
}

// ParseRepoURL parses a git repository URL and returns the provider, owner, and repo
// Supports HTTPS, SSH, and git:// formats for GitHub, GitLab, and Bitbucket
func ParseRepoURL(rawURL string) (*RepoURL, error) {
	rawURL = strings.TrimSpace(rawURL)

	// HTTPS format: https://github.com/owner/repo.git
	httpsRegex := regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	if matches := httpsRegex.FindStringSubmatch(rawURL); matches != nil {
		host := matches[1]
		owner := matches[2]
		repo := matches[3]
		provider := detectProviderFromHost(host)
		return &RepoURL{
			Provider: provider,
			Owner:    owner,
			Repo:     repo,
			Host:     host,
		}, nil
	}

	// SSH format: git@github.com:owner/repo.git
	sshRegex := regexp.MustCompile(`^git@([^:]+):([^/]+)/([^/]+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(rawURL); matches != nil {
		host := matches[1]
		owner := matches[2]
		repo := matches[3]
		provider := detectProviderFromHost(host)
		return &RepoURL{
			Provider: provider,
			Owner:    owner,
			Repo:     repo,
			Host:     host,
		}, nil
	}

	// SSH format with ssh://: ssh://git@github.com/owner/repo.git
	sshAltRegex := regexp.MustCompile(`^ssh://[^@]+@([^/]+)/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	if matches := sshAltRegex.FindStringSubmatch(rawURL); matches != nil {
		host := matches[1]
		owner := matches[2]
		repo := matches[3]
		provider := detectProviderFromHost(host)
		return &RepoURL{
			Provider: provider,
			Owner:    owner,
			Repo:     repo,
			Host:     host,
		}, nil
	}

	return nil, fmt.Errorf("unable to parse repository URL: %s", rawURL)
}

// detectProviderFromHost determines the VCS provider from the hostname
func detectProviderFromHost(host string) Provider {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "github"):
		return ProviderGitHub
	case strings.Contains(host, "gitlab"):
		return ProviderGitLab
	case strings.Contains(host, "bitbucket"):
		return ProviderBitbucket
	default:
		// Unknown host - could be self-hosted, default to GitHub-compatible API
		return ""
	}
}
