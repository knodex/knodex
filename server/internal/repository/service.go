// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/clients"
	"github.com/knodex/knodex/server/internal/deployment/vcs"
	"github.com/knodex/knodex/server/internal/netutil"
	utilhash "github.com/knodex/knodex/server/internal/util/hash"
)

var (
	// Validation regex patterns for input sanitization
	// GitHub allows: alphanumeric, hyphens, underscores, periods
	githubOwnerRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	githubRepoRegex  = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

	// Kubernetes secret name validation (DNS-1123 subdomain)
	secretNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	// Git branch name validation (simplified - no spaces, no special chars except /-_)
	branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`)
)

// AuditLogger interface for audit logging
type AuditLogger interface {
	LogEvent(ctx context.Context, event string, attrs ...slog.Attr)
}

// newSSRFSafeClient returns an HTTP client that blocks connections and redirects to private/internal hosts.
// Uses NewSSRFSafeDialer to pin resolved IPs at connect time (prevents DNS rebinding TOCTOU attacks).
// CheckRedirect is kept as defense-in-depth for redirect hops.
func newSSRFSafeClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: netutil.NewSSRFSafeDialer(),
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if netutil.IsPrivateHost(req.URL.Hostname()) {
				return fmt.Errorf("redirect to private or internal host blocked")
			}
			return nil
		},
	}
}

// Service provides operations on RepositoryConfig CRDs
type Service struct {
	k8sClient         kubernetes.Interface
	dynamicClient     dynamic.Interface
	auditLogger       AuditLogger
	credentialManager *CredentialManager
}

// NewService creates a new repository Service.
// namespace specifies where credential secrets will be stored (typically the application namespace).
func NewService(k8sClient kubernetes.Interface, dynamicClient dynamic.Interface, auditLogger AuditLogger, namespace string) (*Service, error) {
	credMgr, err := NewCredentialManager(k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential manager: %w", err)
	}

	return &Service{
		k8sClient:         k8sClient,
		dynamicClient:     dynamicClient,
		auditLogger:       auditLogger,
		credentialManager: credMgr,
	}, nil
}

// CredentialManager returns the credential manager for direct secret operations
func (s *Service) CredentialManager() *CredentialManager {
	return s.credentialManager
}

// UpdateRepositorySecret updates an existing repository secret's data fields
func (s *Service) UpdateRepositorySecret(ctx context.Context, secretName string, req CreateRepositorySecretRequest) error {
	if s.credentialManager == nil {
		return fmt.Errorf("credential manager not initialized")
	}

	// Get existing secret
	secret, err := s.credentialManager.GetRepositorySecret(ctx, secretName)
	if err != nil {
		return fmt.Errorf("failed to get existing secret: %w", err)
	}

	// Build new secret data
	data, err := s.credentialManager.buildRepositorySecretData(req)
	if err != nil {
		return fmt.Errorf("failed to build updated secret data: %w", err)
	}

	// Update secret data
	secret.Data = data

	// Update audit annotation
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations["knodex.io/updated-by"] = req.CreatedBy

	_, err = s.credentialManager.k8sClient.CoreV1().Secrets(s.credentialManager.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update repository secret: %w", err)
	}

	return nil
}

// UpdateRepositoryMetadata updates metadata fields (name, defaultBranch) on an existing repository secret.
// This is used by the PATCH handler for non-credential updates. Credential updates use UpdateRepositorySecret.
func (s *Service) UpdateRepositoryMetadata(ctx context.Context, repoConfigID string, name, defaultBranch string, updatedBy string) (*RepositoryConfig, error) {
	if s.credentialManager == nil {
		return nil, fmt.Errorf("credential manager not initialized")
	}

	// Get existing secret
	secret, err := s.credentialManager.GetRepositorySecret(ctx, repoConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository secret: %w", err)
	}

	// Verify this is a repository secret
	if secret.Labels == nil || secret.Labels[LabelSecretType] != LabelSecretTypeVal {
		return nil, fmt.Errorf("repository config %s not found", repoConfigID)
	}

	// Update metadata fields in secret data
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[SecretKeyRepoName] = []byte(name)
	secret.Data[SecretKeyDefaultBranch] = []byte(defaultBranch)

	// Update audit annotations
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations["knodex.io/updated-by"] = updatedBy
	secret.Annotations["knodex.io/updated-at"] = time.Now().UTC().Format(time.RFC3339)

	// Save updated secret
	updatedSecret, err := s.credentialManager.k8sClient.CoreV1().Secrets(s.credentialManager.namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update repository metadata: %w", err)
	}

	return secretToRepositoryConfig(updatedSecret), nil
}

// GenerateRepositoryConfigIDFromURL creates a unique repository config ID from a repo URL
func GenerateRepositoryConfigIDFromURL(repoURL string) string {
	// Create a stable ID from the normalized URL
	normalizedURL := strings.ToLower(repoURL)
	// Remove .git suffix if present for consistency
	normalizedURL = strings.TrimSuffix(normalizedURL, ".git")
	return "repoconfig-" + utilhash.Truncate(utilhash.SHA256String(normalizedURL), 32)
}

// ValidateRepositoryConfigSpec validates the repository config specification
// with comprehensive security checks against injection attacks
func ValidateRepositoryConfigSpec(spec RepositoryConfigSpec) error {
	// ProjectID is required
	if spec.ProjectID == "" {
		return fmt.Errorf("projectId is required")
	}
	if len(spec.ProjectID) > 100 {
		return fmt.Errorf("projectId must not exceed 100 characters")
	}

	// Name validation
	if spec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(spec.Name) < 3 || len(spec.Name) > 100 {
		return fmt.Errorf("name must be between 3 and 100 characters")
	}

	return validateNewFormatSpec(spec)
}

// validateNewFormatSpec validates the new ArgoCD-style repository config format
func validateNewFormatSpec(spec RepositoryConfigSpec) error {
	// Validate auth type
	if !ValidateAuthType(spec.AuthType) {
		return fmt.Errorf("authType must be one of: %v", ValidAuthTypes())
	}

	// Validate repoURL format matches authType
	if err := ValidateRepoURLForAuthType(spec.RepoURL, spec.AuthType); err != nil {
		return err
	}

	// Validate repoURL length
	if len(spec.RepoURL) > 2000 {
		return fmt.Errorf("repoURL must not exceed 2000 characters")
	}

	// Default branch validation
	if spec.DefaultBranch == "" {
		return fmt.Errorf("defaultBranch is required")
	}
	if len(spec.DefaultBranch) > 255 {
		return fmt.Errorf("defaultBranch must not exceed 255 characters")
	}
	if !branchNameRegex.MatchString(spec.DefaultBranch) {
		return fmt.Errorf("defaultBranch must be a valid git branch name")
	}

	// Validate no null bytes and valid UTF-8
	fieldsToValidate := map[string]string{
		"name":          spec.Name,
		"projectId":     spec.ProjectID,
		"repoURL":       spec.RepoURL,
		"defaultBranch": spec.DefaultBranch,
	}
	for fieldName, fieldValue := range fieldsToValidate {
		if !utf8.ValidString(fieldValue) {
			return fmt.Errorf("%s must be valid UTF-8", fieldName)
		}
		if strings.ContainsRune(fieldValue, 0) {
			return fmt.Errorf("%s must not contain null bytes", fieldName)
		}
	}

	return nil
}

// ValidateUserID validates a user ID string
func ValidateUserID(userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if len(userID) > 256 {
		return fmt.Errorf("user ID must not exceed 256 characters")
	}
	if !utf8.ValidString(userID) {
		return fmt.Errorf("user ID must be valid UTF-8")
	}
	if strings.ContainsRune(userID, 0) {
		return fmt.Errorf("user ID must not contain null bytes")
	}
	return nil
}

// CreateRepositoryConfigWithCredentials creates a repository using ArgoCD-style pattern
// Following ArgoCD: ALL repository data is stored in a single labeled Secret (no CRD)
func (s *Service) CreateRepositoryConfigWithCredentials(ctx context.Context, req CreateRepositoryRequest, createdBy string) (*RepositoryConfig, error) {
	// Validate createdBy
	if err := ValidateUserID(createdBy); err != nil {
		return nil, fmt.Errorf("invalid createdBy user ID: %w", err)
	}

	// Build spec for validation
	spec := RepositoryConfigSpec{
		ProjectID:     req.ProjectID,
		Name:          req.Name,
		RepoURL:       req.RepoURL,
		AuthType:      req.AuthType,
		DefaultBranch: req.DefaultBranch,
	}

	// Validate spec
	if err := ValidateRepositoryConfigSpec(spec); err != nil {
		return nil, fmt.Errorf("invalid repository config spec: %w", err)
	}

	// Generate deterministic ID from URL (this becomes the secret name)
	secretName := GenerateRepositoryConfigIDFromURL(req.RepoURL)

	// Create ArgoCD-style repository secret (contains ALL data)
	secretReq := CreateRepositorySecretRequest{
		Name:          req.Name,
		ProjectID:     req.ProjectID,
		RepoURL:       req.RepoURL,
		AuthType:      req.AuthType,
		DefaultBranch: req.DefaultBranch,
		SSHAuth:       req.SSHAuth,
		HTTPSAuth:     req.HTTPSAuth,
		GitHubAppAuth: req.GitHubAppAuth,
		CreatedBy:     createdBy,
	}

	secret, err := s.credentialManager.CreateRepositorySecret(ctx, secretName, secretReq)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("repository config already exists for %s", req.RepoURL)
		}
		return nil, fmt.Errorf("failed to create repository secret: %w", err)
	}

	// Convert secret to RepositoryConfig for API response compatibility
	now := metav1.NewTime(secret.CreationTimestamp.Time)
	result := &RepositoryConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              secret.Name,
			Namespace:         secret.Namespace,
			ResourceVersion:   secret.ResourceVersion,
			CreationTimestamp: secret.CreationTimestamp,
			Labels:            secret.Labels,
			Annotations:       secret.Annotations,
		},
		Spec: RepositoryConfigSpec{
			ProjectID:     req.ProjectID,
			Name:          req.Name,
			RepoURL:       req.RepoURL,
			AuthType:      req.AuthType,
			DefaultBranch: req.DefaultBranch,
			SecretRef: SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
		},
		Status: RepositoryConfigStatus{
			CreatedAt:        &now,
			CreatedBy:        createdBy,
			ValidationStatus: "valid",
		},
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogEvent(ctx, "repository_config.created",
			slog.String("repository_config_id", secretName),
			slog.String("repo_url", req.RepoURL),
			slog.String("auth_type", req.AuthType),
			slog.String("project_id", req.ProjectID),
			slog.String("created_by", createdBy),
		)
	}

	slog.Info("repository config created (ArgoCD-style secret)",
		"secret_name", secretName,
		"project_id", req.ProjectID,
		"repo_url", req.RepoURL,
		"auth_type", req.AuthType,
		"created_by", createdBy,
	)

	return result, nil
}

// ListRepositoryConfigs lists repository configs with optional project filter
// Following ArgoCD pattern: lists Secrets with label knodex.io/secret-type=repository
// If projectID is empty, lists all repository configs
// Note: Access control is handled by authorization middleware
func (s *Service) ListRepositoryConfigs(ctx context.Context, projectID string) (*RepositoryConfigList, error) {
	// List repository secrets (ArgoCD-style)
	secrets, err := s.credentialManager.ListRepositorySecrets(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list repository secrets: %w", err)
	}

	// Convert to typed list
	result := &RepositoryConfigList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "SecretList",
		},
		Items: make([]RepositoryConfig, 0, len(secrets.Items)),
	}

	for _, secret := range secrets.Items {
		repoConfig := secretToRepositoryConfig(&secret)
		result.Items = append(result.Items, *repoConfig)
	}

	return result, nil
}

// secretToRepositoryConfig converts an ArgoCD-style repository secret to RepositoryConfig
func secretToRepositoryConfig(secret *corev1.Secret) *RepositoryConfig {
	// Extract metadata from secret data
	repoURL := string(secret.Data[SecretKeyURL])
	projectID := string(secret.Data[SecretKeyProject])
	name := string(secret.Data[SecretKeyRepoName])
	authType := string(secret.Data[SecretKeyType])
	defaultBranch := string(secret.Data[SecretKeyDefaultBranch])

	// Get audit metadata from annotations
	createdBy := ""
	if secret.Annotations != nil {
		createdBy = secret.Annotations["knodex.io/created-by"]
	}

	createdAt := metav1.NewTime(secret.CreationTimestamp.Time)

	return &RepositoryConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              secret.Name,
			Namespace:         secret.Namespace,
			ResourceVersion:   secret.ResourceVersion,
			CreationTimestamp: secret.CreationTimestamp,
			Labels:            secret.Labels,
			Annotations:       secret.Annotations,
		},
		Spec: RepositoryConfigSpec{
			ProjectID:     projectID,
			Name:          name,
			RepoURL:       repoURL,
			AuthType:      authType,
			DefaultBranch: defaultBranch,
			SecretRef: SecretReference{
				Name:      secret.Name,
				Namespace: secret.Namespace,
			},
		},
		Status: RepositoryConfigStatus{
			CreatedAt:        &createdAt,
			CreatedBy:        createdBy,
			ValidationStatus: "valid",
		},
	}
}

// GetRepositoryConfig gets a repository config by ID (secret name)
// Following ArgoCD pattern: reads from Secret instead of CRD
func (s *Service) GetRepositoryConfig(ctx context.Context, repoConfigID string) (*RepositoryConfig, error) {
	secret, err := s.credentialManager.GetRepositorySecret(ctx, repoConfigID)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("repository config %s not found", repoConfigID)
		}
		return nil, fmt.Errorf("failed to get repository config: %w", err)
	}

	// Verify this is a repository secret (has the correct label)
	if secret.Labels == nil || secret.Labels[LabelSecretType] != LabelSecretTypeVal {
		return nil, fmt.Errorf("repository config %s not found", repoConfigID)
	}

	return secretToRepositoryConfig(secret), nil
}

// DeleteRepositoryConfig deletes a repository config (secret)
// Following ArgoCD pattern: just delete the single secret containing all data
// Note: Access control is handled by authorization middleware
func (s *Service) DeleteRepositoryConfig(ctx context.Context, repoConfigID string, deletedBy string) error {
	// Get the config first for audit logging
	repoConfig, getErr := s.GetRepositoryConfig(ctx, repoConfigID)
	if getErr != nil {
		return getErr
	}

	// Delete the repository secret (this is all that exists now - ArgoCD style)
	err := s.credentialManager.DeleteRepositorySecret(ctx, repoConfigID)
	if err != nil {
		return fmt.Errorf("failed to delete repository config: %w", err)
	}

	// Audit log
	if s.auditLogger != nil {
		logAttrs := []slog.Attr{
			slog.String("repository_config_id", repoConfigID),
			slog.String("deleted_by", deletedBy),
		}
		if repoConfig.Spec.ProjectID != "" {
			logAttrs = append(logAttrs, slog.String("project_id", repoConfig.Spec.ProjectID))
		}
		s.auditLogger.LogEvent(ctx, "repository_config.deleted", logAttrs...)
	}

	slog.Info("repository config deleted (ArgoCD-style secret)",
		"secret_name", repoConfigID,
		"project_id", repoConfig.Spec.ProjectID,
		"deleted_by", deletedBy,
	)

	return nil
}

// TestConnectionWithCredentials tests repository connection using the new ArgoCD-style authentication
// This validates credentials work before saving the repository configuration
func (s *Service) TestConnectionWithCredentials(ctx context.Context, req TestConnectionWithCredentialsRequest) (*TestConnectionResponse, error) {
	// Validate required fields
	if req.RepoURL == "" {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "repoURL is required",
		}, nil
	}
	if req.AuthType == "" {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "authType is required",
		}, nil
	}

	// Validate auth type
	if !ValidateAuthType(req.AuthType) {
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("authType must be one of: %v", ValidAuthTypes()),
		}, nil
	}

	// Validate URL format matches auth type
	if err := ValidateRepoURLForAuthType(req.RepoURL, req.AuthType); err != nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	// Test connection based on auth type
	switch req.AuthType {
	case AuthTypeSSH:
		return s.testSSHConnection(ctx, req.RepoURL, req.SSHAuth)
	case AuthTypeHTTPS:
		return s.testHTTPSConnection(ctx, req.RepoURL, req.HTTPSAuth)
	case AuthTypeGitHubApp:
		return s.testGitHubAppConnection(ctx, req.RepoURL, req.GitHubAppAuth)
	default:
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("unsupported auth type: %s", req.AuthType),
		}, nil
	}
}

// testSSHConnection tests SSH authentication by attempting to connect to the repository
func (s *Service) testSSHConnection(ctx context.Context, repoURL string, auth *SSHAuthConfig) (*TestConnectionResponse, error) {
	if auth == nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "SSH authentication config is required",
		}, nil
	}

	// Validate PEM format
	if err := ValidatePEMFormat(auth.PrivateKey, "SSH private key"); err != nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	slog.Info("SSH connection test - format validated",
		"repo_url", repoURL,
	)

	return &TestConnectionResponse{
		Valid:   true,
		Message: "SSH private key format validated (full connection test pending implementation)",
	}, nil
}

// testHTTPSConnection tests HTTPS authentication against the repository
func (s *Service) testHTTPSConnection(ctx context.Context, repoURL string, auth *HTTPSAuthConfig) (*TestConnectionResponse, error) {
	if auth == nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "HTTPS authentication config is required",
		}, nil
	}

	// Validate at least one auth method is provided
	hasAuth := auth.Username != "" || auth.BearerToken != "" || auth.TLSClientCert != ""
	if !hasAuth {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "at least one HTTPS authentication method must be provided (username, bearerToken, or tlsClientCert)",
		}, nil
	}

	// Validate TLS certificate format if provided
	if auth.TLSClientCert != "" {
		if err := ValidatePEMFormat(auth.TLSClientCert, "TLS client certificate"); err != nil {
			return &TestConnectionResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
	}
	if auth.TLSClientKey != "" {
		if err := ValidatePEMFormat(auth.TLSClientKey, "TLS client key"); err != nil {
			return &TestConnectionResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
	}

	// Parse owner/repo from URL using VCS URL parser
	parsedURL, err := vcs.ParseRepoURL(repoURL)
	if err != nil {
		slog.Info("HTTPS connection test - unrecognized URL format, credentials validated",
			"repo_url", repoURL,
		)
		return &TestConnectionResponse{
			Valid:   true,
			Message: "credentials format validated (unrecognized repository format)",
		}, nil
	}

	// Determine which API to test based on the provider
	token := auth.BearerToken
	if token == "" && auth.Password != "" {
		token = auth.Password // Some providers use password field for token
	}

	if token == "" {
		// TLS cert-only auth
		slog.Info("HTTPS connection test - TLS cert format validated",
			"repo_url", repoURL,
		)
		return &TestConnectionResponse{
			Valid:   true,
			Message: "TLS certificate format validated",
		}, nil
	}

	switch parsedURL.Provider {
	case vcs.ProviderGitLab:
		return s.testGitLabAPIConnection(ctx, parsedURL.Owner, parsedURL.Repo, token)
	case vcs.ProviderGitHub:
		return s.testGitHubAPIConnection(ctx, parsedURL.Owner, parsedURL.Repo, token)
	default:
		// Unknown provider, try GitHub API format as fallback
		slog.Info("HTTPS connection test - unknown provider, trying GitHub API format",
			"repo_url", repoURL,
			"host", parsedURL.Host,
		)
		return s.testGitHubAPIConnection(ctx, parsedURL.Owner, parsedURL.Repo, token)
	}
}

// testGitHubAPIConnection tests connection to GitHub using bearer token
func (s *Service) testGitHubAPIConnection(ctx context.Context, owner, repo, token string) (*TestConnectionResponse, error) {
	// Validate token format
	validPrefixes := []string{
		"ghp_",
		"github_pat_",
		"gho_",
		"ghu_",
		"ghs_",
		"ghr_",
	}
	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(token, prefix) {
			hasValidPrefix = true
			break
		}
	}
	if !hasValidPrefix {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "invalid GitHub token format (missing valid prefix)",
		}, nil
	}

	// Build API URL with proper escaping
	escapedOwner := url.PathEscape(owner)
	escapedRepo := url.PathEscape(repo)
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", escapedOwner, escapedRepo)

	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "invalid repository path format",
		}, nil
	}

	if parsedURL.Host != "api.github.com" {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "invalid repository reference",
		}, nil
	}

	client := newSSRFSafeClient(10 * time.Second)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(httpReq)
	if err != nil {
		slog.Warn("GitHub API request failed",
			"owner", owner,
			"repo", repo,
			"error", err,
		)
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("failed to connect to GitHub: %v", err),
		}, nil
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch resp.StatusCode {
	case 200:
		slog.Info("GitHub connection test successful",
			"owner", owner,
			"repo", repo,
		)
		return &TestConnectionResponse{
			Valid:   true,
			Message: fmt.Sprintf("successfully connected to %s/%s", owner, repo),
		}, nil

	case 401:
		return &TestConnectionResponse{
			Valid:   false,
			Message: "invalid GitHub token or token has been revoked",
		}, nil

	case 403:
		return &TestConnectionResponse{
			Valid:   false,
			Message: "GitHub token does not have permission to access this repository",
		}, nil

	case 404:
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("repository %s/%s not found or token does not have access", owner, repo),
		}, nil

	default:
		slog.Warn("GitHub API returned unexpected status",
			"status_code", resp.StatusCode,
			"owner", owner,
			"repo", repo,
		)
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("GitHub API returned status %d", resp.StatusCode),
		}, nil
	}
}

// testGitLabAPIConnection tests connection to GitLab using bearer token
func (s *Service) testGitLabAPIConnection(ctx context.Context, owner, repo, token string) (*TestConnectionResponse, error) {
	// GitLab tokens can have various formats:
	// - Personal Access Token: glpat-XXXXX
	// - Project Access Token: glpat-XXXXX (same prefix)
	// - Group Access Token: glpat-XXXXX (same prefix)
	// - Deploy Token: any string
	// We won't validate the format strictly, just test the connection

	// URL-encode the project path (owner/repo)
	projectPath := url.PathEscape(owner + "/" + repo)
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", projectPath)

	client := newSSRFSafeClient(10 * time.Second)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create API request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		slog.Warn("GitLab API request failed",
			"owner", owner,
			"repo", repo,
			"error", err,
		)
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("failed to connect to GitLab: %v", err),
		}, nil
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	// Read body for error parsing
	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case 200:
		slog.Info("GitLab connection test successful",
			"owner", owner,
			"repo", repo,
		)
		return &TestConnectionResponse{
			Valid:   true,
			Message: fmt.Sprintf("successfully connected to %s/%s", owner, repo),
		}, nil

	case 401:
		return &TestConnectionResponse{
			Valid:   false,
			Message: "GitLab authentication failed: token is invalid or expired",
		}, nil

	case 403:
		// Parse the error to check for scope issues
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error == "insufficient_scope" {
			return &TestConnectionResponse{
				Valid: false,
				Message: fmt.Sprintf(`GitLab API access denied: insufficient token scope.

Your token does not have the required API permissions. %s

Error from GitLab: %s`,
					vcs.ProviderScopeHelp(vcs.ProviderGitLab),
					errResp.ErrorDescription),
			}, nil
		}

		return &TestConnectionResponse{
			Valid: false,
			Message: fmt.Sprintf(`GitLab API access forbidden (403).

This usually means your token does not have the required API scopes.
%s

Error details: %s`, vcs.ProviderScopeHelp(vcs.ProviderGitLab), string(body)),
		}, nil

	case 404:
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("repository %s/%s not found or token does not have access", owner, repo),
		}, nil

	default:
		slog.Warn("GitLab API returned unexpected status",
			"status_code", resp.StatusCode,
			"owner", owner,
			"repo", repo,
		)
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("GitLab API returned status %d: %s", resp.StatusCode, string(body)),
		}, nil
	}
}

// testGitHubAppConnection tests GitHub App authentication
func (s *Service) testGitHubAppConnection(ctx context.Context, repoURL string, auth *GitHubAppAuthConfig) (*TestConnectionResponse, error) {
	if auth == nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "GitHub App authentication config is required",
		}, nil
	}

	// Validate required fields
	if auth.AppID == "" {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "GitHub App ID is required",
		}, nil
	}
	if auth.InstallationID == "" {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "GitHub App Installation ID is required",
		}, nil
	}
	if auth.PrivateKey == "" {
		return &TestConnectionResponse{
			Valid:   false,
			Message: "GitHub App private key is required",
		}, nil
	}

	// Validate PEM format
	if err := ValidatePEMFormat(auth.PrivateKey, "GitHub App private key"); err != nil {
		return &TestConnectionResponse{
			Valid:   false,
			Message: err.Error(),
		}, nil
	}

	// Validate app type
	appType := auth.AppType
	if appType == "" {
		appType = GitHubAppTypeGitHub
	}

	if appType != GitHubAppTypeGitHub && appType != GitHubAppTypeGitHubEnterprise {
		return &TestConnectionResponse{
			Valid:   false,
			Message: fmt.Sprintf("invalid GitHub App type: %s (must be 'github' or 'github-enterprise')", appType),
		}, nil
	}

	// For GitHub Enterprise, validate enterprise URL (HTTPS + SSRF protection)
	if appType == GitHubAppTypeGitHubEnterprise {
		if auth.EnterpriseURL == "" {
			return &TestConnectionResponse{
				Valid:   false,
				Message: "Enterprise URL is required for GitHub Enterprise App",
			}, nil
		}
		if err := ValidateEnterpriseURL(auth.EnterpriseURL); err != nil {
			return &TestConnectionResponse{
				Valid:   false,
				Message: err.Error(),
			}, nil
		}
	}

	slog.Info("GitHub App connection test - format validated",
		"repo_url", repoURL,
		"app_id", auth.AppID,
		"installation_id", auth.InstallationID,
		"app_type", appType,
	)

	return &TestConnectionResponse{
		Valid:   true,
		Message: "GitHub App configuration validated (full connection test pending implementation)",
	}, nil
}

// TestRepositoryConnection tests connectivity to a GitHub repository using a saved repository config
// This tests a repository configuration that has already been created
// Note: Access control is handled by authorization middleware
func (s *Service) TestRepositoryConnection(ctx context.Context, repoConfigID string, userID string) error {
	// Get the repository config
	repoConfig, err := s.GetRepositoryConfig(ctx, repoConfigID)
	if err != nil {
		return err
	}

	// Parse owner/repo from RepoURL
	parsedURL, err := vcs.ParseRepoURL(repoConfig.Spec.RepoURL)
	if err != nil {
		return fmt.Errorf("failed to parse repository URL: %w", err)
	}

	// Create GitHub client wrapper
	githubClient := clients.NewGitHubClient(s.k8sClient)

	// Determine the correct secret key based on auth type
	// For ArgoCD-style secrets, we use known keys based on auth type
	var secretKey string
	switch repoConfig.Spec.AuthType {
	case AuthTypeHTTPS:
		secretKey = SecretKeyBearerToken
	case AuthTypeSSH:
		// SSH testing would need a different approach
		return fmt.Errorf("SSH connection testing not yet implemented")
	case AuthTypeGitHubApp:
		// GitHub App testing would need JWT generation
		return fmt.Errorf("GitHub App connection testing not yet implemented")
	default:
		secretKey = SecretKeyBearerToken
	}

	// Create credentials struct from repository config
	creds := clients.GitHubCredentials{
		SecretName: repoConfig.Spec.SecretRef.Name,
		SecretKey:  secretKey,
		Namespace:  repoConfig.Spec.SecretRef.Namespace,
	}

	// Test connection using GitHub client
	err = githubClient.TestConnection(ctx, creds, parsedURL.Owner, parsedURL.Repo)
	if err != nil {
		// Audit log the failed test
		if s.auditLogger != nil {
			s.auditLogger.LogEvent(ctx, "repository_config.test_connection_failed",
				slog.String("repository_config_id", repoConfigID),
				slog.String("repo_url", repoConfig.Spec.RepoURL),
				slog.String("project_id", repoConfig.Spec.ProjectID),
				slog.String("user_id", userID),
				slog.String("error", err.Error()),
			)
		}
		return fmt.Errorf("connection test failed: %w", err)
	}

	// Audit log the successful test
	if s.auditLogger != nil {
		s.auditLogger.LogEvent(ctx, "repository_config.test_connection_success",
			slog.String("repository_config_id", repoConfigID),
			slog.String("repo_url", repoConfig.Spec.RepoURL),
			slog.String("project_id", repoConfig.Spec.ProjectID),
			slog.String("user_id", userID),
		)
	}

	slog.Info("repository connection test successful",
		"repo_config_id", repoConfigID,
		"project_id", repoConfig.Spec.ProjectID,
		"repo_url", repoConfig.Spec.RepoURL,
		"user_id", userID,
	)

	return nil
}
