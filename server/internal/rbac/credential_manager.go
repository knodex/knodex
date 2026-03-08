package rbac

import (
	"context"
	"fmt"

	utilrand "github.com/knodex/knodex/server/internal/util/rand"
	"github.com/knodex/knodex/server/internal/util/sanitize"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/repository"
)

const (
	// CredentialSecretPrefix is the prefix for all repository credential secrets
	CredentialSecretPrefix = "repo-creds-"

	// CredentialSecretNamespace is the namespace where credential secrets are stored
	CredentialSecretNamespace = "kro-system"

	// Secret data keys for different auth types
	SecretKeySSHPrivateKey       = "sshPrivateKey"
	SecretKeyUsername            = "username"
	SecretKeyPassword            = "password"
	SecretKeyBearerToken         = "bearerToken"
	SecretKeyTLSClientCert       = "tlsClientCert"
	SecretKeyTLSClientKey        = "tlsClientKey"
	SecretKeyGitHubAppID         = "githubAppId"
	SecretKeyGitHubInstallID     = "githubInstallationId"
	SecretKeyGitHubAppKey        = "githubAppPrivateKey"
	SecretKeyGitHubAppType       = "githubAppType"
	SecretKeyGitHubEnterpriseURL = "githubEnterpriseUrl"

	// Labels for credential secrets
	LabelManagedBy    = "app.kubernetes.io/managed-by"
	LabelManagedByVal = "knodex"
	LabelRepoConfigID = "knodex.io/repository-config-id"
	LabelAuthType     = "knodex.io/auth-type"
)

// CredentialManager handles the creation, update, and deletion of
// Kubernetes Secrets for repository credentials
type CredentialManager struct {
	k8sClient kubernetes.Interface
	namespace string
}

// NewCredentialManager creates a new CredentialManager instance
func NewCredentialManager(k8sClient kubernetes.Interface, namespace string) *CredentialManager {
	if namespace == "" {
		namespace = CredentialSecretNamespace
	}
	return &CredentialManager{
		k8sClient: k8sClient,
		namespace: namespace,
	}
}

// CreateCredentialSecret creates a new Kubernetes Secret for repository credentials
// Returns the SecretReference to store in the RepositoryConfig CRD
func (cm *CredentialManager) CreateCredentialSecret(ctx context.Context, req repository.CreateCredentialRequest) (*repository.SecretReference, error) {
	// Validate auth type
	if !repository.ValidateAuthType(req.AuthType) {
		return nil, fmt.Errorf("invalid auth type: %s", req.AuthType)
	}

	// Generate unique secret name
	secretName := cm.generateSecretName(req.RepoConfigID)

	// Build secret data based on auth type
	data, err := cm.buildSecretData(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build secret data: %w", err)
	}

	// Create the secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cm.namespace,
			Labels: map[string]string{
				LabelManagedBy:    LabelManagedByVal,
				LabelRepoConfigID: req.RepoConfigID,
				LabelAuthType:     req.AuthType,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	_, err = cm.k8sClient.CoreV1().Secrets(cm.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create credential secret: %w", err)
	}

	return &repository.SecretReference{
		Name:      secretName,
		Namespace: cm.namespace,
	}, nil
}

// UpdateCredentialSecret updates an existing credential secret
func (cm *CredentialManager) UpdateCredentialSecret(ctx context.Context, secretRef repository.SecretReference, req repository.CreateCredentialRequest) error {
	// Validate auth type
	if !repository.ValidateAuthType(req.AuthType) {
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

	// Update secret data and labels
	secret.Data = data
	secret.Labels[LabelAuthType] = req.AuthType

	_, err = cm.k8sClient.CoreV1().Secrets(secretRef.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update credential secret: %w", err)
	}

	return nil
}

// DeleteCredentialSecret deletes a credential secret
func (cm *CredentialManager) DeleteCredentialSecret(ctx context.Context, secretRef repository.SecretReference) error {
	err := cm.k8sClient.CoreV1().Secrets(secretRef.Namespace).Delete(ctx, secretRef.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete credential secret: %w", err)
	}
	return nil
}

// GetAuthTypeFromSecret determines the auth type from a credential secret
func (cm *CredentialManager) GetAuthTypeFromSecret(secret *corev1.Secret) string {
	if authType, ok := secret.Labels[LabelAuthType]; ok {
		return authType
	}
	// Fallback: determine from secret keys
	if _, hasSSH := secret.Data[SecretKeySSHPrivateKey]; hasSSH {
		return repository.AuthTypeSSH
	}
	if _, hasGHApp := secret.Data[SecretKeyGitHubAppID]; hasGHApp {
		return repository.AuthTypeGitHubApp
	}
	return repository.AuthTypeHTTPS
}

// generateSecretName creates a unique secret name for a repository config
func (cm *CredentialManager) generateSecretName(repoConfigID string) string {
	return fmt.Sprintf("%s%s-%s", CredentialSecretPrefix, sanitize.K8sName(repoConfigID), utilrand.GenerateRandomHex(4))
}

// buildSecretData constructs the secret data map based on auth type
func (cm *CredentialManager) buildSecretData(req repository.CreateCredentialRequest) (map[string][]byte, error) {
	data := make(map[string][]byte)

	switch req.AuthType {
	case repository.AuthTypeSSH:
		if req.SSHAuth == nil {
			return nil, fmt.Errorf("SSH auth config is required for auth type 'ssh'")
		}
		if err := ValidatePEMFormat(req.SSHAuth.PrivateKey, "SSH private key"); err != nil {
			return nil, err
		}
		data[SecretKeySSHPrivateKey] = []byte(req.SSHAuth.PrivateKey)

	case repository.AuthTypeHTTPS:
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

	case repository.AuthTypeGitHubApp:
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
			appType = repository.GitHubAppTypeGitHub
		}
		data[SecretKeyGitHubAppType] = []byte(appType)

		if appType == repository.GitHubAppTypeGitHubEnterprise {
			if req.GitHubAppAuth.EnterpriseURL == "" {
				return nil, fmt.Errorf("Enterprise URL is required for GitHub Enterprise App")
			}
			if err := repository.ValidateEnterpriseURL(req.GitHubAppAuth.EnterpriseURL); err != nil {
				return nil, err
			}
			data[SecretKeyGitHubEnterpriseURL] = []byte(req.GitHubAppAuth.EnterpriseURL)
		}

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", req.AuthType)
	}

	return data, nil
}

// ValidatePEMFormat validates that a string is in PEM format.
// Delegates to repository.ValidatePEMFormat to avoid duplication.
func ValidatePEMFormat(data, fieldName string) error {
	return repository.ValidatePEMFormat(data, fieldName)
}

// ValidateSSHRepoURL validates that a repository URL is in SSH format and does
// not point to a private or internal address (SSRF protection).
// Delegates to repository.ValidateSSHRepoURL to avoid duplication.
func ValidateSSHRepoURL(repoURL string) error {
	return repository.ValidateSSHRepoURL(repoURL)
}

// ValidateHTTPSRepoURL validates that a repository URL uses HTTPS and does not
// point to a private or internal address (SSRF protection).
// Delegates to repository.ValidateHTTPSRepoURL to avoid duplication.
func ValidateHTTPSRepoURL(repoURL string) error {
	return repository.ValidateHTTPSRepoURL(repoURL)
}

// ValidateRepoURLForAuthType validates the repository URL format matches the auth type.
// Delegates to repository.ValidateRepoURLForAuthType to avoid duplication.
func ValidateRepoURLForAuthType(url, authType string) error {
	return repository.ValidateRepoURLForAuthType(url, authType)
}

// ParseGitHubRepoFromURL extracts owner and repo from a GitHub URL.
// Delegates to repository.ParseGitHubRepoFromURL to avoid duplication.
func ParseGitHubRepoFromURL(url string) (owner, repo string, err error) {
	return repository.ParseGitHubRepoFromURL(url)
}
