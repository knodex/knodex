// Package repository provides repository configuration and credential management
// for GitOps deployments in knodex.
package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/provops-org/knodex/server/internal/netutil"
)

// CredentialManager handles the creation, update, and deletion of
// Kubernetes Secrets for repository credentials
type CredentialManager struct {
	k8sClient kubernetes.Interface
	namespace string
}

// NewCredentialManager creates a new CredentialManager instance.
// namespace is required and must not be empty - it specifies where credential secrets
// will be stored (typically the same namespace where the application is deployed).
func NewCredentialManager(k8sClient kubernetes.Interface, namespace string) (*CredentialManager, error) {
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required for CredentialManager")
	}
	return &CredentialManager{
		k8sClient: k8sClient,
		namespace: namespace,
	}, nil
}

// Namespace returns the namespace where credential secrets are stored
func (cm *CredentialManager) Namespace() string {
	return cm.namespace
}

// CreateRepositorySecretRequest contains all information for creating an ArgoCD-style repository secret
type CreateRepositorySecretRequest struct {
	// Repository metadata (stored in secret data)
	Name          string
	ProjectID     string
	RepoURL       string
	AuthType      string
	DefaultBranch string

	// Credential data
	SSHAuth       *SSHAuthConfig
	HTTPSAuth     *HTTPSAuthConfig
	GitHubAppAuth *GitHubAppAuthConfig

	// Audit metadata (stored in annotations)
	CreatedBy string
}

// CreateRepositorySecret creates an ArgoCD-style repository secret containing ALL repository data
// Following ArgoCD pattern: single secret with label contains both metadata and credentials
func (cm *CredentialManager) CreateRepositorySecret(ctx context.Context, secretName string, req CreateRepositorySecretRequest) (*corev1.Secret, error) {
	// Validate auth type
	if !ValidateAuthType(req.AuthType) {
		return nil, fmt.Errorf("invalid auth type: %s", req.AuthType)
	}

	// Build secret data with all repository metadata + credentials
	data, err := cm.buildRepositorySecretData(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build secret data: %w", err)
	}

	// Create the secret with ArgoCD-style label
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cm.namespace,
			Labels: map[string]string{
				LabelSecretType: LabelSecretTypeVal,
			},
			// Store audit metadata as annotations
			Annotations: map[string]string{
				"knodex.io/created-by": req.CreatedBy,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	created, err := cm.k8sClient.CoreV1().Secrets(cm.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create repository secret: %w", err)
	}

	return created, nil
}

// ValidateEnterpriseURL checks that an Enterprise URL uses HTTPS and does not
// point to a private or internal address (SSRF protection).
func ValidateEnterpriseURL(enterpriseURL string) error {
	u, err := url.Parse(enterpriseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("enterpriseUrl must be a valid URL")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("enterpriseUrl must use HTTPS")
	}
	if netutil.IsPrivateHost(u.Hostname()) {
		return fmt.Errorf("enterpriseUrl must not point to a private or internal address")
	}
	return nil
}

// buildRepositorySecretData constructs the complete secret data with metadata and credentials
func (cm *CredentialManager) buildRepositorySecretData(req CreateRepositorySecretRequest) (map[string][]byte, error) {
	data := make(map[string][]byte)

	// Store repository metadata (ArgoCD-style)
	data[SecretKeyURL] = []byte(req.RepoURL)
	data[SecretKeyProject] = []byte(req.ProjectID)
	data[SecretKeyRepoName] = []byte(req.Name)
	data[SecretKeyType] = []byte(req.AuthType)
	data[SecretKeyDefaultBranch] = []byte(req.DefaultBranch)

	// Add credential data based on auth type
	switch req.AuthType {
	case AuthTypeSSH:
		if req.SSHAuth == nil {
			return nil, fmt.Errorf("SSH auth config is required for auth type 'ssh'")
		}
		if err := ValidatePEMFormat(req.SSHAuth.PrivateKey, "SSH private key"); err != nil {
			return nil, err
		}
		data[SecretKeySSHPrivateKey] = []byte(req.SSHAuth.PrivateKey)

	case AuthTypeHTTPS:
		if req.HTTPSAuth == nil {
			return nil, fmt.Errorf("HTTPS auth config is required for auth type 'https'")
		}
		hasAuth := req.HTTPSAuth.Username != "" ||
			req.HTTPSAuth.BearerToken != "" ||
			req.HTTPSAuth.TLSClientCert != ""
		if !hasAuth {
			return nil, fmt.Errorf("at least one HTTPS authentication method must be provided")
		}
		if req.HTTPSAuth.Username != "" {
			data[SecretKeyUsername] = []byte(req.HTTPSAuth.Username)
		}
		if req.HTTPSAuth.Password != "" {
			data[SecretKeyPassword] = []byte(req.HTTPSAuth.Password)
		}
		if req.HTTPSAuth.BearerToken != "" {
			data[SecretKeyBearerToken] = []byte(req.HTTPSAuth.BearerToken)
		}
		if req.HTTPSAuth.TLSClientCert != "" {
			if err := ValidatePEMFormat(req.HTTPSAuth.TLSClientCert, "TLS client certificate"); err != nil {
				return nil, err
			}
			data[SecretKeyTLSClientCert] = []byte(req.HTTPSAuth.TLSClientCert)
		}
		if req.HTTPSAuth.TLSClientKey != "" {
			if err := ValidatePEMFormat(req.HTTPSAuth.TLSClientKey, "TLS client key"); err != nil {
				return nil, err
			}
			data[SecretKeyTLSClientKey] = []byte(req.HTTPSAuth.TLSClientKey)
		}

	case AuthTypeGitHubApp:
		if req.GitHubAppAuth == nil {
			return nil, fmt.Errorf("GitHub App auth config is required for auth type 'github-app'")
		}
		if req.GitHubAppAuth.AppID == "" {
			return nil, fmt.Errorf("GitHub App ID is required")
		}
		if req.GitHubAppAuth.InstallationID == "" {
			return nil, fmt.Errorf("GitHub App Installation ID is required")
		}
		if req.GitHubAppAuth.PrivateKey == "" {
			return nil, fmt.Errorf("GitHub App private key is required")
		}
		if err := ValidatePEMFormat(req.GitHubAppAuth.PrivateKey, "GitHub App private key"); err != nil {
			return nil, err
		}
		data[SecretKeyGitHubAppID] = []byte(req.GitHubAppAuth.AppID)
		data[SecretKeyGitHubInstallID] = []byte(req.GitHubAppAuth.InstallationID)
		data[SecretKeyGitHubAppKey] = []byte(req.GitHubAppAuth.PrivateKey)

		appType := req.GitHubAppAuth.AppType
		if appType == "" {
			appType = GitHubAppTypeGitHub
		}
		data[SecretKeyGitHubAppType] = []byte(appType)

		if appType == GitHubAppTypeGitHubEnterprise {
			if req.GitHubAppAuth.EnterpriseURL == "" {
				return nil, fmt.Errorf("Enterprise URL is required for GitHub Enterprise App")
			}
			if err := ValidateEnterpriseURL(req.GitHubAppAuth.EnterpriseURL); err != nil {
				return nil, err
			}
			data[SecretKeyGitHubEnterpriseURL] = []byte(req.GitHubAppAuth.EnterpriseURL)
		}

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", req.AuthType)
	}

	return data, nil
}

// GetRepositorySecret retrieves a repository secret by name
func (cm *CredentialManager) GetRepositorySecret(ctx context.Context, secretName string) (*corev1.Secret, error) {
	secret, err := cm.k8sClient.CoreV1().Secrets(cm.namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get repository secret: %w", err)
	}
	return secret, nil
}

// DeleteRepositorySecret deletes a repository secret by name
func (cm *CredentialManager) DeleteRepositorySecret(ctx context.Context, secretName string) error {
	err := cm.k8sClient.CoreV1().Secrets(cm.namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete repository secret: %w", err)
	}
	return nil
}

// ListRepositorySecrets lists all repository secrets (ArgoCD-style)
func (cm *CredentialManager) ListRepositorySecrets(ctx context.Context, projectID string) (*corev1.SecretList, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", LabelSecretType, LabelSecretTypeVal),
	}
	secrets, err := cm.k8sClient.CoreV1().Secrets(cm.namespace).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list repository secrets: %w", err)
	}

	// Filter by project if specified
	if projectID != "" {
		var filtered []corev1.Secret
		for _, s := range secrets.Items {
			if string(s.Data[SecretKeyProject]) == projectID {
				filtered = append(filtered, s)
			}
		}
		secrets.Items = filtered
	}

	return secrets, nil
}

// CreateCredentialSecret creates a new Kubernetes Secret for repository credentials
// Deprecated: Use CreateRepositorySecret instead for ArgoCD-style secrets
func (cm *CredentialManager) CreateCredentialSecret(ctx context.Context, req CreateCredentialRequest) (*SecretReference, error) {
	// Validate auth type
	if !ValidateAuthType(req.AuthType) {
		return nil, fmt.Errorf("invalid auth type: %s", req.AuthType)
	}

	// Generate unique secret name
	secretName := cm.generateSecretName(req.RepoConfigID)

	// Build secret data based on auth type
	data, err := cm.buildSecretData(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build secret data: %w", err)
	}

	// Create the secret with a single label for identification
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cm.namespace,
			Labels: map[string]string{
				LabelSecretType: LabelSecretTypeVal,
			},
			// Store metadata as annotations instead of labels for querying
			Annotations: map[string]string{
				"knodex.io/repository-config-id": req.RepoConfigID,
				"knodex.io/auth-type":            req.AuthType,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	_, err = cm.k8sClient.CoreV1().Secrets(cm.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create credential secret: %w", err)
	}

	return &SecretReference{
		Name:      secretName,
		Namespace: cm.namespace,
	}, nil
}

// UpdateCredentialSecret updates an existing credential secret
func (cm *CredentialManager) UpdateCredentialSecret(ctx context.Context, secretRef SecretReference, req CreateCredentialRequest) error {
	// Validate auth type
	if !ValidateAuthType(req.AuthType) {
		return fmt.Errorf("invalid auth type: %s", req.AuthType)
	}

	// Get existing secret
	secret, err := cm.k8sClient.CoreV1().Secrets(secretRef.Namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get credential secret: %w", err)
	}

	// Build new secret data
	data, err := cm.buildSecretData(req)
	if err != nil {
		return fmt.Errorf("failed to build secret data: %w", err)
	}

	// Update secret data and annotations
	secret.Data = data
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations["knodex.io/auth-type"] = req.AuthType

	_, err = cm.k8sClient.CoreV1().Secrets(secretRef.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update credential secret: %w", err)
	}

	return nil
}

// DeleteCredentialSecret deletes a credential secret
func (cm *CredentialManager) DeleteCredentialSecret(ctx context.Context, secretRef SecretReference) error {
	err := cm.k8sClient.CoreV1().Secrets(secretRef.Namespace).Delete(ctx, secretRef.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete credential secret: %w", err)
	}
	return nil
}

// GetAuthTypeFromSecret determines the auth type from a credential secret
func (cm *CredentialManager) GetAuthTypeFromSecret(secret *corev1.Secret) string {
	// Check annotation first
	if authType, ok := secret.Annotations["knodex.io/auth-type"]; ok {
		return authType
	}
	// Fallback: determine from secret keys
	if _, hasSSH := secret.Data[SecretKeySSHPrivateKey]; hasSSH {
		return AuthTypeSSH
	}
	if _, hasGHApp := secret.Data[SecretKeyGitHubAppID]; hasGHApp {
		return AuthTypeGitHubApp
	}
	return AuthTypeHTTPS
}

// generateSecretName creates a unique secret name for a repository config
func (cm *CredentialManager) generateSecretName(repoConfigID string) string {
	// Generate a short random suffix for uniqueness
	suffix := make([]byte, 4)
	rand.Read(suffix)
	return fmt.Sprintf("%s%s-%s", CredentialSecretPrefix, sanitizeK8sName(repoConfigID), hex.EncodeToString(suffix))
}

// buildSecretData constructs the secret data map based on auth type
func (cm *CredentialManager) buildSecretData(req CreateCredentialRequest) (map[string][]byte, error) {
	data := make(map[string][]byte)

	switch req.AuthType {
	case AuthTypeSSH:
		if req.SSHAuth == nil {
			return nil, fmt.Errorf("SSH auth config is required for auth type 'ssh'")
		}
		if err := ValidatePEMFormat(req.SSHAuth.PrivateKey, "SSH private key"); err != nil {
			return nil, err
		}
		data[SecretKeySSHPrivateKey] = []byte(req.SSHAuth.PrivateKey)

	case AuthTypeHTTPS:
		if req.HTTPSAuth == nil {
			return nil, fmt.Errorf("HTTPS auth config is required for auth type 'https'")
		}
		// At least one auth method must be provided
		hasAuth := req.HTTPSAuth.Username != "" ||
			req.HTTPSAuth.BearerToken != "" ||
			req.HTTPSAuth.TLSClientCert != ""
		if !hasAuth {
			return nil, fmt.Errorf("at least one HTTPS authentication method must be provided (username, bearerToken, or tlsClientCert)")
		}
		if req.HTTPSAuth.Username != "" {
			data[SecretKeyUsername] = []byte(req.HTTPSAuth.Username)
		}
		if req.HTTPSAuth.Password != "" {
			data[SecretKeyPassword] = []byte(req.HTTPSAuth.Password)
		}
		if req.HTTPSAuth.BearerToken != "" {
			data[SecretKeyBearerToken] = []byte(req.HTTPSAuth.BearerToken)
		}
		if req.HTTPSAuth.TLSClientCert != "" {
			if err := ValidatePEMFormat(req.HTTPSAuth.TLSClientCert, "TLS client certificate"); err != nil {
				return nil, err
			}
			data[SecretKeyTLSClientCert] = []byte(req.HTTPSAuth.TLSClientCert)
		}
		if req.HTTPSAuth.TLSClientKey != "" {
			if err := ValidatePEMFormat(req.HTTPSAuth.TLSClientKey, "TLS client key"); err != nil {
				return nil, err
			}
			data[SecretKeyTLSClientKey] = []byte(req.HTTPSAuth.TLSClientKey)
		}

	case AuthTypeGitHubApp:
		if req.GitHubAppAuth == nil {
			return nil, fmt.Errorf("GitHub App auth config is required for auth type 'github-app'")
		}
		if req.GitHubAppAuth.AppID == "" {
			return nil, fmt.Errorf("GitHub App ID is required")
		}
		if req.GitHubAppAuth.InstallationID == "" {
			return nil, fmt.Errorf("GitHub App Installation ID is required")
		}
		if req.GitHubAppAuth.PrivateKey == "" {
			return nil, fmt.Errorf("GitHub App private key is required")
		}
		if err := ValidatePEMFormat(req.GitHubAppAuth.PrivateKey, "GitHub App private key"); err != nil {
			return nil, err
		}
		data[SecretKeyGitHubAppID] = []byte(req.GitHubAppAuth.AppID)
		data[SecretKeyGitHubInstallID] = []byte(req.GitHubAppAuth.InstallationID)
		data[SecretKeyGitHubAppKey] = []byte(req.GitHubAppAuth.PrivateKey)

		appType := req.GitHubAppAuth.AppType
		if appType == "" {
			appType = GitHubAppTypeGitHub
		}
		data[SecretKeyGitHubAppType] = []byte(appType)

		if appType == GitHubAppTypeGitHubEnterprise {
			if req.GitHubAppAuth.EnterpriseURL == "" {
				return nil, fmt.Errorf("Enterprise URL is required for GitHub Enterprise App")
			}
			if err := ValidateEnterpriseURL(req.GitHubAppAuth.EnterpriseURL); err != nil {
				return nil, err
			}
			data[SecretKeyGitHubEnterpriseURL] = []byte(req.GitHubAppAuth.EnterpriseURL)
		}

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", req.AuthType)
	}

	return data, nil
}

// sanitizeK8sName converts a string to a valid Kubernetes resource name
func sanitizeK8sName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)
	// Replace any non-alphanumeric characters with dashes
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "-")
	// Remove consecutive dashes
	re = regexp.MustCompile(`-+`)
	name = re.ReplaceAllString(name, "-")
	// Trim dashes from start and end
	name = strings.Trim(name, "-")
	// Truncate to max length (63 chars for K8s names, but leave room for prefix)
	maxLen := 40
	if len(name) > maxLen {
		name = name[:maxLen]
	}
	return name
}

// ValidatePEMFormat validates that a string is in PEM format
func ValidatePEMFormat(data, fieldName string) error {
	if data == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// Check for PEM headers
	pemRegex := regexp.MustCompile(`-----BEGIN [A-Z0-9 ]+-----`)
	if !pemRegex.MatchString(data) {
		return fmt.Errorf("%s must be in PEM format (missing BEGIN header)", fieldName)
	}

	endRegex := regexp.MustCompile(`-----END [A-Z0-9 ]+-----`)
	if !endRegex.MatchString(data) {
		return fmt.Errorf("%s must be in PEM format (missing END header)", fieldName)
	}

	return nil
}

// ValidateSSHRepoURL validates that a repository URL is in SSH format and does
// not point to a private or internal address (SSRF protection).
func ValidateSSHRepoURL(repoURL string) error {
	if strings.HasPrefix(repoURL, "ssh://") {
		parsed, err := url.Parse(repoURL)
		if err != nil || parsed.Host == "" {
			return fmt.Errorf("SSH repository URL must be a valid URL")
		}
		if netutil.IsPrivateHost(parsed.Hostname()) {
			return fmt.Errorf("repository URL must not point to a private or internal address")
		}
		return nil
	}
	if strings.HasPrefix(repoURL, "git@") {
		// git@host:org/repo.git format
		hostPart := repoURL[4:] // strip "git@"
		colonIdx := strings.Index(hostPart, ":")
		if colonIdx <= 0 {
			return fmt.Errorf("SSH repository URL must be in format 'git@host:path'")
		}
		hostname := hostPart[:colonIdx]
		if netutil.IsPrivateHost(hostname) {
			return fmt.Errorf("repository URL must not point to a private or internal address")
		}
		return nil
	}
	return fmt.Errorf("SSH repository URL must start with 'git@' or 'ssh://'")
}

// ValidateHTTPSRepoURL validates that a repository URL uses HTTPS and does not
// point to a private or internal address (SSRF protection).
func ValidateHTTPSRepoURL(repoURL string) error {
	if !strings.HasPrefix(repoURL, "https://") {
		return fmt.Errorf("HTTPS repository URL must start with 'https://'")
	}
	parsed, err := url.Parse(repoURL)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("repository URL must be a valid URL")
	}
	if netutil.IsPrivateHost(parsed.Hostname()) {
		return fmt.Errorf("repository URL must not point to a private or internal address")
	}
	return nil
}

// ValidateRepoURLForAuthType validates the repository URL format matches the auth type
func ValidateRepoURLForAuthType(url, authType string) error {
	switch authType {
	case AuthTypeSSH:
		return ValidateSSHRepoURL(url)
	case AuthTypeHTTPS, AuthTypeGitHubApp:
		return ValidateHTTPSRepoURL(url)
	default:
		return fmt.Errorf("unknown auth type: %s", authType)
	}
}

// ParseGitHubRepoFromURL extracts owner and repo from a GitHub URL
// Supports both HTTPS and SSH formats
func ParseGitHubRepoFromURL(url string) (owner, repo string, err error) {
	// HTTPS format: https://github.com/owner/repo.git
	httpsRegex := regexp.MustCompile(`^https://[^/]+/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	if matches := httpsRegex.FindStringSubmatch(url); matches != nil {
		return matches[1], matches[2], nil
	}

	// SSH format: git@github.com:owner/repo.git
	sshRegex := regexp.MustCompile(`^git@[^:]+:([^/]+)/([^/]+?)(?:\.git)?$`)
	if matches := sshRegex.FindStringSubmatch(url); matches != nil {
		return matches[1], matches[2], nil
	}

	// SSH format with ssh://: ssh://git@github.com/owner/repo.git
	sshAltRegex := regexp.MustCompile(`^ssh://[^/]+/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	if matches := sshAltRegex.FindStringSubmatch(url); matches != nil {
		return matches[1], matches[2], nil
	}

	return "", "", fmt.Errorf("unable to parse owner and repo from URL: %s", url)
}
