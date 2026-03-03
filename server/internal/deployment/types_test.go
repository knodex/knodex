package deployment

import (
	"testing"
)

func TestRepositoryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *RepositoryConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "repository config is nil",
		},
		{
			name: "valid config",
			config: &RepositoryConfig{
				Owner:           "my-org",
				Repo:            "my-repo",
				DefaultBranch:   "main",
				BasePath:        "instances",
				SecretName:      "github-token",
				SecretNamespace: "kro-system",
				SecretKey:       "token",
			},
			wantErr: false,
		},
		{
			name: "empty owner",
			config: &RepositoryConfig{
				Owner:      "",
				Repo:       "my-repo",
				SecretName: "github-token",
			},
			wantErr: true,
			errMsg:  "repository owner is required",
		},
		{
			name: "invalid owner - starts with hyphen",
			config: &RepositoryConfig{
				Owner:      "-invalid",
				Repo:       "my-repo",
				SecretName: "github-token",
			},
			wantErr: true,
			errMsg:  "invalid repository owner \"-invalid\": must be 1-39 alphanumeric characters or hyphens, cannot start or end with hyphen",
		},
		{
			name: "invalid owner - ends with hyphen",
			config: &RepositoryConfig{
				Owner:      "invalid-",
				Repo:       "my-repo",
				SecretName: "github-token",
			},
			wantErr: true,
			errMsg:  "invalid repository owner \"invalid-\": must be 1-39 alphanumeric characters or hyphens, cannot start or end with hyphen",
		},
		{
			name: "empty repo",
			config: &RepositoryConfig{
				Owner:      "my-org",
				Repo:       "",
				SecretName: "github-token",
			},
			wantErr: true,
			errMsg:  "repository name is required",
		},
		{
			name: "invalid repo - special chars",
			config: &RepositoryConfig{
				Owner:      "my-org",
				Repo:       "my-repo!@#",
				SecretName: "github-token",
			},
			wantErr: true,
			errMsg:  "invalid repository name \"my-repo!@#\": must be 1-100 alphanumeric characters, hyphens, underscores, or dots",
		},
		{
			name: "empty secret name",
			config: &RepositoryConfig{
				Owner:      "my-org",
				Repo:       "my-repo",
				SecretName: "",
			},
			wantErr: true,
			errMsg:  "secret name is required",
		},
		{
			name: "invalid secret name - uppercase",
			config: &RepositoryConfig{
				Owner:      "my-org",
				Repo:       "my-repo",
				SecretName: "MySecret",
			},
			wantErr: true,
			errMsg:  "invalid secret name \"MySecret\": must be lowercase alphanumeric with hyphens (DNS-1123)",
		},
		{
			name: "invalid branch - path traversal",
			config: &RepositoryConfig{
				Owner:         "my-org",
				Repo:          "my-repo",
				SecretName:    "github-token",
				DefaultBranch: "main/../etc",
			},
			wantErr: true,
			errMsg:  "invalid branch: branch name cannot contain path traversal sequences",
		},
		{
			name: "invalid base path - path traversal",
			config: &RepositoryConfig{
				Owner:      "my-org",
				Repo:       "my-repo",
				SecretName: "github-token",
				BasePath:   "../../../etc",
			},
			wantErr: true,
			errMsg:  "invalid base path: base path cannot contain path traversal sequences",
		},
		{
			name: "invalid base path - absolute path",
			config: &RepositoryConfig{
				Owner:      "my-org",
				Repo:       "my-repo",
				SecretName: "github-token",
				BasePath:   "/etc/passwd",
			},
			wantErr: true,
			errMsg:  "invalid base path: base path cannot be absolute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid branch - main",
			branch:  "main",
			wantErr: false,
		},
		{
			name:    "valid branch - with slash",
			branch:  "feature/my-feature",
			wantErr: false,
		},
		{
			name:    "valid branch - with dots",
			branch:  "release/v1.0.0",
			wantErr: false,
		},
		{
			name:    "empty branch",
			branch:  "",
			wantErr: true,
			errMsg:  "branch name cannot be empty",
		},
		{
			name:    "path traversal",
			branch:  "main/../etc",
			wantErr: true,
			errMsg:  "branch name cannot contain path traversal sequences",
		},
		{
			name:    "contains backslash",
			branch:  "main\\feature",
			wantErr: true,
			errMsg:  "branch name cannot contain \"\\\\\"",
		},
		{
			name:    "contains tilde",
			branch:  "main~1",
			wantErr: true,
			errMsg:  "branch name cannot contain \"~\"",
		},
		{
			name:    "contains caret",
			branch:  "main^1",
			wantErr: true,
			errMsg:  "branch name cannot contain \"^\"",
		},
		{
			name:    "contains colon",
			branch:  "main:ref",
			wantErr: true,
			errMsg:  "branch name cannot contain \":\"",
		},
		{
			name:    "contains space",
			branch:  "my branch",
			wantErr: true,
			errMsg:  "branch name cannot contain spaces",
		},
		{
			name:    "contains reflog syntax",
			branch:  "main@{1}",
			wantErr: true,
			errMsg:  "branch name cannot contain \"@{\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branch)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateBranchName() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("ValidateBranchName() error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateBranchName() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBasePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid path - simple",
			path:    "instances",
			wantErr: false,
		},
		{
			name:    "valid path - with slash",
			path:    "manifests/instances",
			wantErr: false,
		},
		{
			name:    "empty path - allowed",
			path:    "",
			wantErr: false, // Empty uses default
		},
		{
			name:    "path traversal",
			path:    "../etc",
			wantErr: true,
			errMsg:  "base path cannot contain path traversal sequences",
		},
		{
			name:    "absolute path - unix",
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "base path cannot be absolute",
		},
		{
			name:    "absolute path - windows",
			path:    "\\etc\\passwd",
			wantErr: true,
			errMsg:  "base path cannot be absolute",
		},
		{
			name:    "contains backslash",
			path:    "manifests\\instances",
			wantErr: true,
			errMsg:  "base path cannot contain backslashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBasePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateBasePath() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("ValidateBasePath() error = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateBasePath() unexpected error: %v", err)
			}
		})
	}
}

// Note: TestIsValidDeploymentMode and TestParseDeploymentMode are in controller_test.go
