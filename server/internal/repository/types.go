// Package repository provides repository configuration and credential management
// for GitOps deployments in knodex.
package repository

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knodex/knodex/server/internal/deployment/vcs"
)

// RepositoryConfig CRD constants
const (
	RepositoryConfigGroup    = "knodex.io"
	RepositoryConfigVersion  = "v1alpha1"
	RepositoryConfigResource = "repositoryconfigs"
	RepositoryConfigKind     = "RepositoryConfig"
)

var (
	// RepositoryConfigGVR is the GroupVersionResource for RepositoryConfig CRD
	RepositoryConfigGVR = schema.GroupVersionResource{
		Group:    RepositoryConfigGroup,
		Version:  RepositoryConfigVersion,
		Resource: RepositoryConfigResource,
	}
)

// AuthType constants for repository authentication methods
const (
	AuthTypeSSH       = "ssh"
	AuthTypeHTTPS     = "https"
	AuthTypeGitHubApp = "github-app"
)

// GitHubAppType constants for GitHub vs GitHub Enterprise
const (
	GitHubAppTypeGitHub           = "github"
	GitHubAppTypeGitHubEnterprise = "github-enterprise"
)

// Secret label for repository credentials
const (
	// LabelSecretType identifies repository credential secrets
	// Single label as per ArgoCD pattern
	LabelSecretType    = "knodex.io/secret-type"
	LabelSecretTypeVal = "repository"
)

// Secret data keys for ArgoCD-style repository secrets
// Following ArgoCD pattern: ALL repository data is stored in a single secret
const (
	// Metadata keys (ArgoCD-style)
	SecretKeyURL           = "url"           // Repository URL
	SecretKeyProject       = "project"       // Project ID this repo belongs to
	SecretKeyRepoName      = "name"          // Friendly name for the repository
	SecretKeyType          = "type"          // Auth type: ssh, https, github-app
	SecretKeyDefaultBranch = "defaultBranch" // Default branch (e.g., main)
	SecretKeyEnabled       = "enabled"       // "true" or "false"

	// SSH authentication keys
	SecretKeySSHPrivateKey = "sshPrivateKey"

	// HTTPS authentication keys
	SecretKeyUsername    = "username"
	SecretKeyPassword    = "password"
	SecretKeyBearerToken = "bearerToken"

	// TLS certificate keys (for HTTPS with client certs)
	SecretKeyTLSClientCert = "tlsClientCert"
	SecretKeyTLSClientKey  = "tlsClientKey"

	// GitHub App authentication keys
	SecretKeyGitHubAppID         = "githubAppId"
	SecretKeyGitHubInstallID     = "githubInstallationId"
	SecretKeyGitHubAppKey        = "githubAppPrivateKey"
	SecretKeyGitHubAppType       = "githubAppType"       // "github" or "github-enterprise"
	SecretKeyGitHubEnterpriseURL = "githubEnterpriseUrl" // Only for github-enterprise

	// Deprecated keys (kept for backwards compatibility during migration)
	SecretKeyRepoConfigID = "repoConfigId" // Deprecated: ID is now the secret name
	SecretKeyAuthType     = "authType"     // Deprecated: Use SecretKeyType instead
)

// CredentialSecretPrefix is the prefix for all repository credential secrets
const CredentialSecretPrefix = "repo-creds-"

// ValidAuthTypes returns all valid authentication types
func ValidAuthTypes() []string {
	return []string{AuthTypeSSH, AuthTypeHTTPS, AuthTypeGitHubApp}
}

// ValidateAuthType checks if an auth type string is valid
func ValidateAuthType(authType string) bool {
	switch authType {
	case AuthTypeSSH, AuthTypeHTTPS, AuthTypeGitHubApp:
		return true
	default:
		return false
	}
}

// SecretReference references a Kubernetes Secret containing credentials
type SecretReference struct {
	// Name is the name of the Kubernetes Secret
	Name string `json:"name"`

	// Namespace is the namespace of the Kubernetes Secret
	Namespace string `json:"namespace"`
}

// SSHAuthConfig holds SSH authentication configuration
// These fields are NOT stored in the CRD, only used for API requests
type SSHAuthConfig struct {
	// PrivateKey is the SSH private key in PEM format
	PrivateKey string `json:"privateKey"`
}

// HTTPSAuthConfig holds HTTPS authentication configuration
// These fields are NOT stored in the CRD, only used for API requests
type HTTPSAuthConfig struct {
	// Username for HTTPS basic authentication
	Username string `json:"username,omitempty"`

	// Password for HTTPS basic authentication
	Password string `json:"password,omitempty"`

	// BearerToken for HTTPS token-based authentication (e.g., GitHub PAT)
	BearerToken string `json:"bearerToken,omitempty"`

	// TLSClientCert is the TLS client certificate in PEM format
	TLSClientCert string `json:"tlsClientCert,omitempty"`

	// TLSClientKey is the TLS client certificate key in PEM format
	TLSClientKey string `json:"tlsClientKey,omitempty"`

	// InsecureSkipTLSVerify disables TLS certificate verification
	InsecureSkipTLSVerify bool `json:"insecureSkipTLSVerify,omitempty"`
}

// GitHubAppAuthConfig holds GitHub App authentication configuration
// These fields are NOT stored in the CRD, only used for API requests
type GitHubAppAuthConfig struct {
	// AppType is either "github" or "github-enterprise"
	AppType string `json:"appType"`

	// AppID is the GitHub App ID
	AppID string `json:"appId"`

	// InstallationID is the GitHub App Installation ID
	InstallationID string `json:"installationId"`

	// PrivateKey is the GitHub App private key in PEM format
	PrivateKey string `json:"privateKey"`

	// EnterpriseURL is the GitHub Enterprise base URL (only for github-enterprise type)
	EnterpriseURL string `json:"enterpriseUrl,omitempty"`
}

// RepositoryConfig represents a GitHub repository configuration for GitOps deployments
// This is stored as a Kubernetes Custom Resource
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RepositoryConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositoryConfigSpec   `json:"spec"`
	Status RepositoryConfigStatus `json:"status,omitempty"`
}

// RepositoryConfigSpec defines the desired state of RepositoryConfig
type RepositoryConfigSpec struct {
	// ProjectID is the reference to the project that owns this repository config (required)
	ProjectID string `json:"projectId"`

	// Name is a friendly name for this repository configuration
	Name string `json:"name"`

	// RepoURL is the full repository URL (e.g., "https://github.com/org/repo.git" or "git@github.com:org/repo.git")
	RepoURL string `json:"repoURL"`

	// AuthType is the authentication method: "ssh", "https", or "github-app"
	AuthType string `json:"authType"`

	// SecretRef references the Kubernetes Secret containing credentials
	// The secret is managed by the backend and contains auth-specific fields
	SecretRef SecretReference `json:"secretRef"`

	// DefaultBranch is the default branch to use for commits (e.g., "main")
	DefaultBranch string `json:"defaultBranch"`
}

// RepositoryConfigStatus defines the observed state of RepositoryConfig
type RepositoryConfigStatus struct {
	// CreatedAt is the timestamp when the repository config was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`

	// CreatedBy is the user ID who created the repository config
	CreatedBy string `json:"createdBy,omitempty"`

	// UpdatedAt is the timestamp when the repository config was last updated
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`

	// UpdatedBy is the user ID who last updated the repository config
	UpdatedBy string `json:"updatedBy,omitempty"`

	// LastValidated is the timestamp when the repository configuration was last validated
	LastValidated *metav1.Time `json:"lastValidated,omitempty"`

	// ValidationStatus indicates the result of the last validation ("valid", "invalid", or "unknown")
	ValidationStatus string `json:"validationStatus,omitempty"`

	// ValidationMessage contains details about the validation status
	ValidationMessage string `json:"validationMessage,omitempty"`
}

// RepositoryConfigList contains a list of RepositoryConfig resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RepositoryConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RepositoryConfig `json:"items"`
}

// RepositoryConfigInfo is a simplified view of RepositoryConfig for API responses
type RepositoryConfigInfo struct {
	ID                string     `json:"id"` // resource name
	ProjectID         string     `json:"projectId"`
	Name              string     `json:"name"`
	RepoURL           string     `json:"repoURL"`
	AuthType          string     `json:"authType"`
	DefaultBranch     string     `json:"defaultBranch"`
	CreatedBy         string     `json:"createdBy,omitempty"`
	CreatedAt         *time.Time `json:"createdAt,omitempty"`
	UpdatedBy         string     `json:"updatedBy,omitempty"`
	UpdatedAt         *time.Time `json:"updatedAt,omitempty"`
	ValidationStatus  string     `json:"validationStatus,omitempty"`
	ValidationMessage string     `json:"validationMessage,omitempty"`
	ResourceVersion   string     `json:"-"` // for optimistic concurrency

	// ConnectionStatus indicates the result of the last connection test
	ConnectionStatus string `json:"connectionStatus,omitempty"`

	// --- Legacy fields (for backwards compatibility) ---
	// These are populated from RepoURL parsing for legacy clients
	Owner string `json:"owner,omitempty"`
	Repo  string `json:"repo,omitempty"`
}

// ToRepositoryConfigInfo converts RepositoryConfig CRD to RepositoryConfigInfo for API responses
func (r *RepositoryConfig) ToRepositoryConfigInfo() *RepositoryConfigInfo {
	info := &RepositoryConfigInfo{
		ID:                r.Name,
		ProjectID:         r.Spec.ProjectID,
		Name:              r.Spec.Name,
		RepoURL:           r.Spec.RepoURL,
		AuthType:          r.Spec.AuthType,
		DefaultBranch:     r.Spec.DefaultBranch,
		CreatedBy:         r.Status.CreatedBy,
		UpdatedBy:         r.Status.UpdatedBy,
		ValidationStatus:  r.Status.ValidationStatus,
		ValidationMessage: r.Status.ValidationMessage,
		ResourceVersion:   r.ResourceVersion,
	}

	// Compute Owner/Repo from RepoURL for backwards compatibility
	if r.Spec.RepoURL != "" {
		if parsed, err := vcs.ParseRepoURL(r.Spec.RepoURL); err == nil {
			info.Owner = parsed.Owner
			info.Repo = parsed.Repo
		}
	}

	if r.Status.CreatedAt != nil {
		t := r.Status.CreatedAt.Time
		info.CreatedAt = &t
	}

	if r.Status.UpdatedAt != nil {
		t := r.Status.UpdatedAt.Time
		info.UpdatedAt = &t
	}

	return info
}

// DeepCopyObject implements runtime.Object interface for RepositoryConfig
func (r *RepositoryConfig) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(RepositoryConfig)
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ObjectMeta.DeepCopyInto(&out.ObjectMeta)

	// Deep copy Spec
	out.Spec.ProjectID = r.Spec.ProjectID
	out.Spec.Name = r.Spec.Name
	out.Spec.RepoURL = r.Spec.RepoURL
	out.Spec.AuthType = r.Spec.AuthType
	out.Spec.SecretRef = SecretReference{
		Name:      r.Spec.SecretRef.Name,
		Namespace: r.Spec.SecretRef.Namespace,
	}
	out.Spec.DefaultBranch = r.Spec.DefaultBranch

	// Deep copy Status
	out.Status.CreatedBy = r.Status.CreatedBy
	out.Status.UpdatedBy = r.Status.UpdatedBy
	out.Status.ValidationStatus = r.Status.ValidationStatus
	out.Status.ValidationMessage = r.Status.ValidationMessage

	if r.Status.CreatedAt != nil {
		out.Status.CreatedAt = r.Status.CreatedAt.DeepCopy()
	}
	if r.Status.UpdatedAt != nil {
		out.Status.UpdatedAt = r.Status.UpdatedAt.DeepCopy()
	}
	if r.Status.LastValidated != nil {
		out.Status.LastValidated = r.Status.LastValidated.DeepCopy()
	}

	return out
}

// DeepCopyObject implements runtime.Object interface for RepositoryConfigList
func (r *RepositoryConfigList) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(RepositoryConfigList)
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ListMeta.DeepCopyInto(&out.ListMeta)

	if r.Items != nil {
		out.Items = make([]RepositoryConfig, len(r.Items))
		for i := range r.Items {
			out.Items[i] = *r.Items[i].DeepCopyObject().(*RepositoryConfig)
		}
	}

	return out
}

// CreateCredentialRequest contains the information needed to create a credential secret
type CreateCredentialRequest struct {
	// RepoConfigID is the ID of the RepositoryConfig CRD this credential belongs to
	RepoConfigID string

	// AuthType is the authentication type (ssh, https, github-app)
	AuthType string

	// SSH authentication
	SSHAuth *SSHAuthConfig

	// HTTPS authentication
	HTTPSAuth *HTTPSAuthConfig

	// GitHub App authentication
	GitHubAppAuth *GitHubAppAuthConfig
}

// CreateRepositoryRequest represents a request to create a new repository with credentials
// This is the new ArgoCD-style format
type CreateRepositoryRequest struct {
	Name          string `json:"name"`
	ProjectID     string `json:"projectId"`
	RepoURL       string `json:"repoURL"`
	AuthType      string `json:"authType"`
	DefaultBranch string `json:"defaultBranch"`

	// Auth-specific credentials (only one should be provided based on authType)
	SSHAuth       *SSHAuthConfig       `json:"sshAuth,omitempty"`
	HTTPSAuth     *HTTPSAuthConfig     `json:"httpsAuth,omitempty"`
	GitHubAppAuth *GitHubAppAuthConfig `json:"githubAppAuth,omitempty"`
}

// TestConnectionRequest represents a request to test GitHub repository connection
type TestConnectionRequest struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	SecretName string `json:"secretName"`
	SecretKey  string `json:"secretKey"`
}

// TestConnectionResponse represents the result of a connection test
type TestConnectionResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// TestConnectionWithCredentialsRequest represents a request to test repository connection
// using the new ArgoCD-style authentication methods
type TestConnectionWithCredentialsRequest struct {
	RepoURL  string `json:"repoURL"`
	AuthType string `json:"authType"`

	// Auth-specific credentials (only one should be provided based on authType)
	SSHAuth       *SSHAuthConfig       `json:"sshAuth,omitempty"`
	HTTPSAuth     *HTTPSAuthConfig     `json:"httpsAuth,omitempty"`
	GitHubAppAuth *GitHubAppAuthConfig `json:"githubAppAuth,omitempty"`
}
