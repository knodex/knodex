package deployment

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Validation regex patterns for GitHub and Kubernetes naming conventions
var (
	// githubOwnerRegex validates GitHub owner/organization names
	// Rules: 1-39 chars, alphanumeric + hyphens, cannot start/end with hyphen
	githubOwnerRegex = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

	// githubRepoRegex validates GitHub repository names
	// Rules: 1-100 chars, alphanumeric + hyphens + underscores + dots
	githubRepoRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,100}$`)

	// gitBranchRegex validates Git branch names
	// Rules: alphanumeric + common separators, no path traversal, no special git chars
	gitBranchRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_.-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)

	// kubernetesNameRegex validates Kubernetes resource names (DNS-1123)
	kubernetesNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	// basePathRegex validates base path (no traversal, simple path)
	basePathRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_.-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)
)

// DeploymentMode represents how an instance should be deployed
type DeploymentMode string

const (
	// ModeDirect applies manifest directly to Kubernetes cluster (default, existing behavior)
	ModeDirect DeploymentMode = "direct"
	// ModeGitOps pushes manifest to Git repository only (for GitOps tools like ArgoCD/Flux)
	ModeGitOps DeploymentMode = "gitops"
	// ModeHybrid applies to cluster AND pushes to Git (immediate deployment + audit trail)
	ModeHybrid DeploymentMode = "hybrid"
)

// ValidDeploymentModes contains all valid deployment modes
var ValidDeploymentModes = []DeploymentMode{ModeDirect, ModeGitOps, ModeHybrid}

// IsValid checks if the deployment mode is valid
func (m DeploymentMode) IsValid() bool {
	switch m {
	case ModeDirect, ModeGitOps, ModeHybrid:
		return true
	default:
		return false
	}
}

// String returns the string representation of the deployment mode
func (m DeploymentMode) String() string {
	return string(m)
}

// IsValidDeploymentMode checks if a deployment mode is valid
func IsValidDeploymentMode(mode DeploymentMode) bool {
	return mode.IsValid()
}

// ParseDeploymentMode converts a string to DeploymentMode, defaults to Direct
func ParseDeploymentMode(s string) DeploymentMode {
	mode := DeploymentMode(s)
	if mode.IsValid() {
		return mode
	}
	return ModeDirect
}

// GitPushStatus represents the status of a Git push operation
type GitPushStatus string

const (
	// GitPushPending indicates Git push has not started
	GitPushPending GitPushStatus = "pending"
	// GitPushInProgress indicates Git push is in progress
	GitPushInProgress GitPushStatus = "in_progress"
	// GitPushSuccess indicates Git push completed successfully
	GitPushSuccess GitPushStatus = "success"
	// GitPushFailed indicates Git push failed
	GitPushFailed GitPushStatus = "failed"
	// GitPushNotApplicable indicates Git push is not applicable (direct mode)
	GitPushNotApplicable GitPushStatus = "not_applicable"
)

// GitInfo contains Git-related information for an instance
type GitInfo struct {
	// RepositoryID is the ID of the configured repository
	RepositoryID string `json:"repositoryId,omitempty"`
	// CommitSHA is the Git commit SHA if pushed successfully
	CommitSHA string `json:"commitSha,omitempty"`
	// CommitURL is the URL to view the commit in the Git provider
	CommitURL string `json:"commitUrl,omitempty"`
	// Branch is the target branch for the manifest
	Branch string `json:"branch,omitempty"`
	// Path is the path where the manifest was written
	Path string `json:"path,omitempty"`
	// PushStatus is the current status of the Git push
	PushStatus GitPushStatus `json:"pushStatus"`
	// PushError contains error message if push failed
	PushError string `json:"pushError,omitempty"`
	// PushedAt is when the manifest was pushed to Git
	PushedAt string `json:"pushedAt,omitempty"`
}

// InstanceStatus represents the deployment status of an instance
type InstanceStatus string

const (
	// StatusPending - deployment request received
	StatusPending InstanceStatus = "Pending"
	// StatusManifestGenerated - manifest YAML generated
	StatusManifestGenerated InstanceStatus = "ManifestGenerated"
	// StatusPushedToGit - manifest pushed to Git repository (GitOps/Hybrid)
	StatusPushedToGit InstanceStatus = "PushedToGit"
	// StatusWaitingForSync - waiting for GitOps tool to sync (GitOps mode)
	StatusWaitingForSync InstanceStatus = "WaitingForSync"
	// StatusSyncing - GitOps tool is syncing the manifest
	StatusSyncing InstanceStatus = "Syncing"
	// StatusCreating - resource being created in cluster
	StatusCreating InstanceStatus = "Creating"
	// StatusReady - instance is ready and running
	StatusReady InstanceStatus = "Ready"
	// StatusDegraded - instance is running but not fully healthy
	StatusDegraded InstanceStatus = "Degraded"
	// StatusFailed - deployment failed
	StatusFailed InstanceStatus = "Failed"
	// StatusGitOpsFailed - manifest pushed to Git but deployment failed
	StatusGitOpsFailed InstanceStatus = "GitOpsFailed"
)

// DeployRequest contains all information needed to deploy an instance
type DeployRequest struct {
	// Instance information
	InstanceID   string                 `json:"instanceId"`
	Name         string                 `json:"name"`
	Namespace    string                 `json:"namespace"`
	RGDName      string                 `json:"rgdName"`
	RGDNamespace string                 `json:"rgdNamespace"`
	APIVersion   string                 `json:"apiVersion"`
	Kind         string                 `json:"kind"`
	Spec         map[string]interface{} `json:"spec"`

	// Deployment configuration
	DeploymentMode DeploymentMode `json:"deploymentMode"`

	// Project context (optional, for future project-based deployments)
	ProjectID string `json:"projectId,omitempty"`

	// Repository configuration (required for GitOps/Hybrid modes)
	Repository *RepositoryConfig `json:"repository,omitempty"`

	// Per-deployment GitOps overrides
	// GitBranch overrides the repository's default branch for this deployment
	GitBranch string `json:"gitBranch,omitempty"`
	// GitPath overrides the auto-generated semantic path for this deployment
	GitPath string `json:"gitPath,omitempty"`

	// User context
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
}

// Validate validates the DeployRequest fields
// SECURITY: Validates GitOps override fields to prevent injection attacks
func (r *DeployRequest) Validate() error {
	// Validate GitBranch override if provided
	if r.GitBranch != "" {
		if err := ValidateBranchName(r.GitBranch); err != nil {
			return fmt.Errorf("invalid gitBranch: %w", err)
		}
	}

	// Validate GitPath override if provided
	if r.GitPath != "" {
		if err := ValidateBasePath(r.GitPath); err != nil {
			return fmt.Errorf("invalid gitPath: %w", err)
		}
	}

	// Validate repository config if present
	if r.Repository != nil {
		if err := r.Repository.Validate(); err != nil {
			return fmt.Errorf("invalid repository config: %w", err)
		}
	}

	return nil
}

// GetEffectiveBranch returns the branch to use for this deployment
// Uses GitBranch override if set, otherwise falls back to repository default
func (r *DeployRequest) GetEffectiveBranch() string {
	if r.GitBranch != "" {
		return r.GitBranch
	}
	if r.Repository != nil {
		return r.Repository.GetBranch()
	}
	return "main"
}

// DeployResult contains the result of a deployment operation
type DeployResult struct {
	// Instance identification
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`

	// Deployment information
	Mode   DeploymentMode `json:"mode"`
	Status InstanceStatus `json:"status"`

	// Direct deployment results
	ClusterDeployed bool   `json:"clusterDeployed"`
	ClusterError    string `json:"clusterError,omitempty"`

	// GitOps results
	GitPushed    bool   `json:"gitPushed"`
	GitCommitSHA string `json:"gitCommitSha,omitempty"`
	ManifestPath string `json:"manifestPath,omitempty"`
	GitError     string `json:"gitError,omitempty"`

	// Timing
	DeployedAt time.Time `json:"deployedAt"`
}

// RepositoryConfig contains Git repository configuration for GitOps deployments
type RepositoryConfig struct {
	// ID is the unique identifier for this repository config
	ID string `json:"id"`
	// Name is a human-readable name for this repository
	Name string `json:"name"`
	// ProjectID is the project this repository belongs to (optional)
	ProjectID string `json:"projectId,omitempty"`
	// Provider is the Git provider (github, gitlab, bitbucket)
	Provider string `json:"provider,omitempty"`
	// BaseURL is the base URL for self-hosted instances (e.g., "https://github.example.com" or "https://gitlab.mycompany.com")
	// If empty, defaults to the public instance (github.com, gitlab.com, bitbucket.org)
	BaseURL string `json:"baseURL,omitempty"`
	// Owner is the GitHub organization or user that owns the repo
	Owner string `json:"owner"`
	// Repo is the repository name
	Repo string `json:"repo"`
	// Branch is the default branch for commits
	Branch string `json:"branch"`
	// DefaultBranch is an alias for Branch (for backward compatibility)
	DefaultBranch string `json:"defaultBranch,omitempty"`
	// BasePath is the base path within the repo for manifests (default: "instances")
	BasePath string `json:"basePath"`
	// SecretRef is the reference to the Kubernetes secret containing credentials
	SecretRef SecretReference `json:"secretRef,omitempty"`
	// SecretName is the Kubernetes secret name containing GitHub PAT (legacy)
	SecretName string `json:"secretName,omitempty"`
	// SecretNamespace is the namespace of the secret (default: "kro-system", legacy)
	SecretNamespace string `json:"secretNamespace,omitempty"`
	// SecretKey is the key within the secret containing the token (default: "token", legacy)
	SecretKey string `json:"secretKey,omitempty"`
	// Enabled indicates if this repository is active
	Enabled bool `json:"enabled"`
	// CreatedAt is when the config was created
	CreatedAt string `json:"createdAt,omitempty"`
	// UpdatedAt is when the config was last updated
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// SecretReference references a Kubernetes secret
type SecretReference struct {
	// Name is the secret name
	Name string `json:"name"`
	// Namespace is the secret namespace
	Namespace string `json:"namespace"`
	// Key is the key within the secret containing the token
	Key string `json:"key"`
}

// GetBranch returns the branch to use, preferring Branch over DefaultBranch
func (r *RepositoryConfig) GetBranch() string {
	if r.Branch != "" {
		return r.Branch
	}
	if r.DefaultBranch != "" {
		return r.DefaultBranch
	}
	return "main"
}

// GetSecretName returns the secret name from either SecretRef or legacy field
func (r *RepositoryConfig) GetSecretName() string {
	if r.SecretRef.Name != "" {
		return r.SecretRef.Name
	}
	return r.SecretName
}

// GetSecretNamespace returns the secret namespace from either SecretRef or legacy field
func (r *RepositoryConfig) GetSecretNamespace() string {
	if r.SecretRef.Namespace != "" {
		return r.SecretRef.Namespace
	}
	if r.SecretNamespace != "" {
		return r.SecretNamespace
	}
	return "kro-system"
}

// GetSecretKey returns the secret key from either SecretRef or legacy field
func (r *RepositoryConfig) GetSecretKey() string {
	if r.SecretRef.Key != "" {
		return r.SecretRef.Key
	}
	if r.SecretKey != "" {
		return r.SecretKey
	}
	return "token"
}

// GetProvider returns the provider, defaulting to "github" if not set
func (r *RepositoryConfig) GetProvider() string {
	if r.Provider != "" {
		return strings.ToLower(r.Provider)
	}
	return "github"
}

// GetRepositoryURL builds the full repository URL based on provider and baseURL
// Supports GitHub, GitLab, and Bitbucket (both public and self-hosted instances)
// Examples:
//   - GitHub public: https://github.com/owner/repo
//   - GitHub Enterprise: https://github.example.com/owner/repo
//   - GitLab public: https://gitlab.com/owner/repo
//   - GitLab self-hosted: https://gitlab.mycompany.com/owner/repo
//   - Bitbucket public: https://bitbucket.org/owner/repo
//   - Bitbucket self-hosted: https://bitbucket.mycompany.com/owner/repo
func (r *RepositoryConfig) GetRepositoryURL() string {
	provider := r.GetProvider()
	owner := r.Owner
	repo := r.Repo

	// If baseURL is provided, use it (for self-hosted instances)
	if r.BaseURL != "" {
		baseURL := strings.TrimRight(r.BaseURL, "/")
		return fmt.Sprintf("%s/%s/%s", baseURL, owner, repo)
	}

	// Default to public instances based on provider
	switch provider {
	case "gitlab":
		return fmt.Sprintf("https://gitlab.com/%s/%s", owner, repo)
	case "bitbucket":
		return fmt.Sprintf("https://bitbucket.org/%s/%s", owner, repo)
	default: // github
		return fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	}
}

// Validate performs security validation on all RepositoryConfig fields
// SECURITY: Prevents injection attacks via malformed repository config values
func (r *RepositoryConfig) Validate() error {
	if r == nil {
		return fmt.Errorf("repository config is nil")
	}

	// Validate Owner (required)
	if r.Owner == "" {
		return fmt.Errorf("repository owner is required")
	}
	if !githubOwnerRegex.MatchString(r.Owner) {
		return fmt.Errorf("invalid repository owner %q: must be 1-39 alphanumeric characters or hyphens, cannot start or end with hyphen", r.Owner)
	}

	// Validate Repo (required)
	if r.Repo == "" {
		return fmt.Errorf("repository name is required")
	}
	if !githubRepoRegex.MatchString(r.Repo) {
		return fmt.Errorf("invalid repository name %q: must be 1-100 alphanumeric characters, hyphens, underscores, or dots", r.Repo)
	}

	// Validate branch
	branch := r.GetBranch()
	if branch != "" {
		if err := ValidateBranchName(branch); err != nil {
			return fmt.Errorf("invalid branch: %w", err)
		}
	}

	// Validate BasePath (optional, has default)
	if r.BasePath != "" {
		if err := ValidateBasePath(r.BasePath); err != nil {
			return fmt.Errorf("invalid base path: %w", err)
		}
	}

	// Validate secret configuration
	secretName := r.GetSecretName()
	if secretName == "" {
		return fmt.Errorf("secret name is required")
	}
	if !kubernetesNameRegex.MatchString(secretName) {
		return fmt.Errorf("invalid secret name %q: must be lowercase alphanumeric with hyphens (DNS-1123)", secretName)
	}
	if len(secretName) > 253 {
		return fmt.Errorf("secret name exceeds maximum length of 253 characters")
	}

	// Validate SecretNamespace
	secretNS := r.GetSecretNamespace()
	if secretNS != "" {
		if !kubernetesNameRegex.MatchString(secretNS) {
			return fmt.Errorf("invalid secret namespace %q: must be lowercase alphanumeric with hyphens (DNS-1123)", secretNS)
		}
		if len(secretNS) > 63 {
			return fmt.Errorf("secret namespace exceeds maximum length of 63 characters")
		}
	}

	// Validate SecretKey
	secretKey := r.GetSecretKey()
	if secretKey != "" {
		// Secret keys can be more flexible but should not contain special chars
		if strings.ContainsAny(secretKey, "/\\:*?\"<>|") {
			return fmt.Errorf("invalid secret key %q: contains forbidden characters", secretKey)
		}
		if len(secretKey) > 253 {
			return fmt.Errorf("secret key exceeds maximum length of 253 characters")
		}
	}

	return nil
}

// ValidateBranchName validates a Git branch name for security
// SECURITY: Prevents command injection and path traversal via branch names
func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Check length
	if len(branch) > 255 {
		return fmt.Errorf("branch name exceeds maximum length of 255 characters")
	}

	// Check for path traversal
	if strings.Contains(branch, "..") {
		return fmt.Errorf("branch name cannot contain path traversal sequences")
	}

	// Check for forbidden patterns
	forbiddenPatterns := []string{
		"@{", // Git reflog syntax
		"~",  // Ancestor reference
		"^",  // Parent reference
		":",  // Could be used in various git contexts
		"\\", // Backslash
	}
	for _, pattern := range forbiddenPatterns {
		if strings.Contains(branch, pattern) {
			return fmt.Errorf("branch name cannot contain %q", pattern)
		}
	}

	// Check for control characters and spaces
	for _, r := range branch {
		if r < 32 || r == 127 {
			return fmt.Errorf("branch name cannot contain control characters")
		}
		if r == ' ' {
			return fmt.Errorf("branch name cannot contain spaces")
		}
	}

	// Validate with regex for allowed pattern
	if !gitBranchRegex.MatchString(branch) {
		return fmt.Errorf("branch name %q contains invalid characters", branch)
	}

	return nil
}

// ValidateBasePath validates a base path for security
// SECURITY: Prevents path traversal attacks via base path manipulation
func ValidateBasePath(path string) error {
	if path == "" {
		return nil // Empty path is OK, will use default
	}

	// Check length
	if len(path) > 255 {
		return fmt.Errorf("base path exceeds maximum length of 255 characters")
	}

	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("base path cannot contain path traversal sequences")
	}

	// Check for absolute paths
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return fmt.Errorf("base path cannot be absolute")
	}

	// Check for backslashes (Windows-style paths)
	if strings.Contains(path, "\\") {
		return fmt.Errorf("base path cannot contain backslashes")
	}

	// Validate with regex
	if !basePathRegex.MatchString(path) {
		return fmt.Errorf("base path %q contains invalid characters", path)
	}

	return nil
}

// ManifestMetadata contains tracking metadata for a deployed manifest
type ManifestMetadata struct {
	InstanceID     string         `yaml:"instanceId" json:"instanceId"`
	Name           string         `yaml:"name" json:"name"`
	Namespace      string         `yaml:"namespace" json:"namespace"`
	RGDName        string         `yaml:"rgdName" json:"rgdName"`
	RGDNamespace   string         `yaml:"rgdNamespace" json:"rgdNamespace"`
	ProjectID      string         `yaml:"projectId,omitempty" json:"projectId,omitempty"`
	DeploymentMode DeploymentMode `yaml:"deploymentMode" json:"deploymentMode"`
	CreatedBy      string         `yaml:"createdBy" json:"createdBy"`
	CreatedAt      time.Time      `yaml:"createdAt" json:"createdAt"`
	CommitSHA      string         `yaml:"commitSha,omitempty" json:"commitSha,omitempty"`
	RepositoryID   string         `yaml:"repositoryId,omitempty" json:"repositoryId,omitempty"`
}
