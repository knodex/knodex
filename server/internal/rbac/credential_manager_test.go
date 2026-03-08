package rbac

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// Test PEM keys for validation tests
const (
	validSSHPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxAAAAA
-----END OPENSSH PRIVATE KEY-----`

	validRSAPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAtest
-----END RSA PRIVATE KEY-----`

	validCertificate = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpqsample
-----END CERTIFICATE-----`

	invalidPEMKey = "not-a-valid-pem-key"

	missingEndPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAtest`
)

func TestNewCredentialManager(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	t.Run("uses provided namespace", func(t *testing.T) {
		cm := NewCredentialManager(fakeClient, "custom-namespace")
		if cm.namespace != "custom-namespace" {
			t.Errorf("expected namespace 'custom-namespace', got %s", cm.namespace)
		}
	})

	t.Run("uses default namespace when empty", func(t *testing.T) {
		cm := NewCredentialManager(fakeClient, "")
		if cm.namespace != CredentialSecretNamespace {
			t.Errorf("expected namespace '%s', got %s", CredentialSecretNamespace, cm.namespace)
		}
	})
}

func TestCredentialManager_CreateCredentialSecret_SSH(t *testing.T) {
	ctx := context.Background()

	t.Run("creates SSH credential secret successfully", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-123",
			AuthType:     repository.AuthTypeSSH,
			SSHAuth: &repository.SSHAuthConfig{
				PrivateKey: validSSHPrivateKey,
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if secretRef == nil {
			t.Fatal("expected secret reference, got nil")
		}

		if secretRef.Namespace != "kro-system" {
			t.Errorf("expected namespace 'kro-system', got %s", secretRef.Namespace)
		}

		if !strings.HasPrefix(secretRef.Name, CredentialSecretPrefix) {
			t.Errorf("expected secret name to start with '%s', got %s", CredentialSecretPrefix, secretRef.Name)
		}

		// Verify secret was created
		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get created secret: %v", err)
		}

		// Verify secret data
		if string(secret.Data[SecretKeySSHPrivateKey]) != validSSHPrivateKey {
			t.Error("SSH private key not stored correctly in secret")
		}

		// Verify labels
		if secret.Labels[LabelManagedBy] != LabelManagedByVal {
			t.Errorf("expected label %s=%s, got %s", LabelManagedBy, LabelManagedByVal, secret.Labels[LabelManagedBy])
		}
		if secret.Labels[LabelRepoConfigID] != "test-repo-123" {
			t.Errorf("expected label %s=test-repo-123, got %s", LabelRepoConfigID, secret.Labels[LabelRepoConfigID])
		}
		if secret.Labels[LabelAuthType] != repository.AuthTypeSSH {
			t.Errorf("expected label %s=%s, got %s", LabelAuthType, repository.AuthTypeSSH, secret.Labels[LabelAuthType])
		}
	})

	t.Run("fails with invalid PEM key", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-123",
			AuthType:     repository.AuthTypeSSH,
			SSHAuth: &repository.SSHAuthConfig{
				PrivateKey: invalidPEMKey,
			},
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for invalid PEM key, got nil")
		}

		if !strings.Contains(err.Error(), "PEM format") {
			t.Errorf("expected error about PEM format, got: %v", err)
		}
	})

	t.Run("fails with missing SSH auth config", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-123",
			AuthType:     repository.AuthTypeSSH,
			SSHAuth:      nil,
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for missing SSH auth config, got nil")
		}
	})
}

func TestCredentialManager_CreateCredentialSecret_HTTPS(t *testing.T) {
	ctx := context.Background()

	t.Run("creates HTTPS credential with username/password", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-https",
			AuthType:     repository.AuthTypeHTTPS,
			HTTPSAuth: &repository.HTTPSAuthConfig{
				Username: "testuser",
				Password: "testpass",
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get created secret: %v", err)
		}

		if string(secret.Data[SecretKeyUsername]) != "testuser" {
			t.Error("username not stored correctly")
		}
		if string(secret.Data[SecretKeyPassword]) != "testpass" {
			t.Error("password not stored correctly")
		}
	})

	t.Run("creates HTTPS credential with bearer token", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-token",
			AuthType:     repository.AuthTypeHTTPS,
			HTTPSAuth: &repository.HTTPSAuthConfig{
				BearerToken: "ghp_xxxxxxxxxxxxxxxxxxxx",
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get created secret: %v", err)
		}

		if string(secret.Data[SecretKeyBearerToken]) != "ghp_xxxxxxxxxxxxxxxxxxxx" {
			t.Error("bearer token not stored correctly")
		}
	})

	t.Run("creates HTTPS credential with TLS client cert", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-tls",
			AuthType:     repository.AuthTypeHTTPS,
			HTTPSAuth: &repository.HTTPSAuthConfig{
				TLSClientCert: validCertificate,
				TLSClientKey:  validRSAPrivateKey,
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get created secret: %v", err)
		}

		if string(secret.Data[SecretKeyTLSClientCert]) != validCertificate {
			t.Error("TLS client cert not stored correctly")
		}
		if string(secret.Data[SecretKeyTLSClientKey]) != validRSAPrivateKey {
			t.Error("TLS client key not stored correctly")
		}
	})

	t.Run("fails when no HTTPS auth method provided", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-empty",
			AuthType:     repository.AuthTypeHTTPS,
			HTTPSAuth:    &repository.HTTPSAuthConfig{
				// No credentials provided
			},
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for empty HTTPS auth, got nil")
		}

		if !strings.Contains(err.Error(), "at least one HTTPS authentication method") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("fails with invalid TLS cert PEM", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-bad-tls",
			AuthType:     repository.AuthTypeHTTPS,
			HTTPSAuth: &repository.HTTPSAuthConfig{
				TLSClientCert: invalidPEMKey,
			},
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for invalid TLS cert, got nil")
		}
	})
}

func TestCredentialManager_CreateCredentialSecret_GitHubApp(t *testing.T) {
	ctx := context.Background()

	t.Run("creates GitHub App credential successfully", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-ghapp",
			AuthType:     repository.AuthTypeGitHubApp,
			GitHubAppAuth: &repository.GitHubAppAuthConfig{
				AppType:        repository.GitHubAppTypeGitHub,
				AppID:          "12345",
				InstallationID: "67890",
				PrivateKey:     validRSAPrivateKey,
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get created secret: %v", err)
		}

		if string(secret.Data[SecretKeyGitHubAppID]) != "12345" {
			t.Error("GitHub App ID not stored correctly")
		}
		if string(secret.Data[SecretKeyGitHubInstallID]) != "67890" {
			t.Error("GitHub Installation ID not stored correctly")
		}
		if string(secret.Data[SecretKeyGitHubAppKey]) != validRSAPrivateKey {
			t.Error("GitHub App private key not stored correctly")
		}
		if string(secret.Data[SecretKeyGitHubAppType]) != repository.GitHubAppTypeGitHub {
			t.Error("GitHub App type not stored correctly")
		}
	})

	t.Run("creates GitHub Enterprise App credential with enterprise URL", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-ghent",
			AuthType:     repository.AuthTypeGitHubApp,
			GitHubAppAuth: &repository.GitHubAppAuthConfig{
				AppType:        repository.GitHubAppTypeGitHubEnterprise,
				AppID:          "12345",
				InstallationID: "67890",
				PrivateKey:     validRSAPrivateKey,
				EnterpriseURL:  "https://github.com",
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get created secret: %v", err)
		}

		if string(secret.Data[SecretKeyGitHubEnterpriseURL]) != "https://github.com" {
			t.Error("GitHub Enterprise URL not stored correctly")
		}
	})

	t.Run("rejects GitHub Enterprise with SSRF enterprise URL", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-ghent-ssrf",
			AuthType:     repository.AuthTypeGitHubApp,
			GitHubAppAuth: &repository.GitHubAppAuthConfig{
				AppType:        repository.GitHubAppTypeGitHubEnterprise,
				AppID:          "12345",
				InstallationID: "67890",
				PrivateKey:     validRSAPrivateKey,
				EnterpriseURL:  "https://169.254.169.254/",
			},
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for SSRF enterprise URL, got nil")
		}
		if !strings.Contains(err.Error(), "private or internal") {
			t.Errorf("expected SSRF error, got: %v", err)
		}
	})

	t.Run("rejects GitHub Enterprise with HTTP enterprise URL", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-ghent-http",
			AuthType:     repository.AuthTypeGitHubApp,
			GitHubAppAuth: &repository.GitHubAppAuthConfig{
				AppType:        repository.GitHubAppTypeGitHubEnterprise,
				AppID:          "12345",
				InstallationID: "67890",
				PrivateKey:     validRSAPrivateKey,
				EnterpriseURL:  "http://github.com",
			},
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for HTTP enterprise URL, got nil")
		}
		if !strings.Contains(err.Error(), "must use HTTPS") {
			t.Errorf("expected HTTPS error, got: %v", err)
		}
	})

	t.Run("fails when GitHub Enterprise missing enterprise URL", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo-ghent-no-url",
			AuthType:     repository.AuthTypeGitHubApp,
			GitHubAppAuth: &repository.GitHubAppAuthConfig{
				AppType:        repository.GitHubAppTypeGitHubEnterprise,
				AppID:          "12345",
				InstallationID: "67890",
				PrivateKey:     validRSAPrivateKey,
				// Missing EnterpriseURL
			},
		}

		_, err := cm.CreateCredentialSecret(ctx, req)
		if err == nil {
			t.Fatal("expected error for missing enterprise URL, got nil")
		}

		if !strings.Contains(err.Error(), "Enterprise URL is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("fails with missing required fields", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		testCases := []struct {
			name        string
			config      *repository.GitHubAppAuthConfig
			expectError string
		}{
			{
				name: "missing app ID",
				config: &repository.GitHubAppAuthConfig{
					InstallationID: "67890",
					PrivateKey:     validRSAPrivateKey,
				},
				expectError: "App ID is required",
			},
			{
				name: "missing installation ID",
				config: &repository.GitHubAppAuthConfig{
					AppID:      "12345",
					PrivateKey: validRSAPrivateKey,
				},
				expectError: "Installation ID is required",
			},
			{
				name: "missing private key",
				config: &repository.GitHubAppAuthConfig{
					AppID:          "12345",
					InstallationID: "67890",
				},
				expectError: "private key is required",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := repository.CreateCredentialRequest{
					RepoConfigID:  "test-repo",
					AuthType:      repository.AuthTypeGitHubApp,
					GitHubAppAuth: tc.config,
				}

				_, err := cm.CreateCredentialSecret(ctx, req)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.expectError) {
					t.Errorf("expected error containing '%s', got: %v", tc.expectError, err)
				}
			})
		}
	})
}

func TestCredentialManager_UpdateCredentialSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("updates existing secret successfully", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		// Create initial secret
		initialReq := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo",
			AuthType:     repository.AuthTypeSSH,
			SSHAuth: &repository.SSHAuthConfig{
				PrivateKey: validSSHPrivateKey,
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, initialReq)
		if err != nil {
			t.Fatalf("failed to create initial secret: %v", err)
		}

		// Update to HTTPS
		updateReq := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo",
			AuthType:     repository.AuthTypeHTTPS,
			HTTPSAuth: &repository.HTTPSAuthConfig{
				BearerToken: "new-token",
			},
		}

		err = cm.UpdateCredentialSecret(ctx, *secretRef, updateReq)
		if err != nil {
			t.Fatalf("failed to update secret: %v", err)
		}

		// Verify updated secret
		secret, err := fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get secret: %v", err)
		}

		// Should have new auth type
		if secret.Labels[LabelAuthType] != repository.AuthTypeHTTPS {
			t.Errorf("expected auth type label '%s', got '%s'", repository.AuthTypeHTTPS, secret.Labels[LabelAuthType])
		}

		// Should have new token
		if string(secret.Data[SecretKeyBearerToken]) != "new-token" {
			t.Error("bearer token not updated correctly")
		}

		// Should not have old SSH key
		if _, hasSSH := secret.Data[SecretKeySSHPrivateKey]; hasSSH {
			t.Error("old SSH key should have been removed")
		}
	})

	t.Run("fails for non-existent secret", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo",
			AuthType:     repository.AuthTypeSSH,
			SSHAuth: &repository.SSHAuthConfig{
				PrivateKey: validSSHPrivateKey,
			},
		}

		err := cm.UpdateCredentialSecret(ctx, repository.SecretReference{
			Name:      "non-existent-secret",
			Namespace: "kro-system",
		}, req)

		if err == nil {
			t.Fatal("expected error for non-existent secret, got nil")
		}
	})
}

func TestCredentialManager_DeleteCredentialSecret(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes existing secret successfully", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		// Create secret
		req := repository.CreateCredentialRequest{
			RepoConfigID: "test-repo",
			AuthType:     repository.AuthTypeSSH,
			SSHAuth: &repository.SSHAuthConfig{
				PrivateKey: validSSHPrivateKey,
			},
		}

		secretRef, err := cm.CreateCredentialSecret(ctx, req)
		if err != nil {
			t.Fatalf("failed to create secret: %v", err)
		}

		// Delete secret
		err = cm.DeleteCredentialSecret(ctx, *secretRef)
		if err != nil {
			t.Fatalf("failed to delete secret: %v", err)
		}

		// Verify secret is deleted
		_, err = fakeClient.CoreV1().Secrets("kro-system").Get(ctx, secretRef.Name, metav1.GetOptions{})
		if !errors.IsNotFound(err) {
			t.Errorf("expected not found error, got: %v", err)
		}
	})

	t.Run("succeeds silently for non-existent secret", func(t *testing.T) {
		fakeClient := fake.NewSimpleClientset()
		cm := NewCredentialManager(fakeClient, "kro-system")

		// Should not error when deleting non-existent secret
		err := cm.DeleteCredentialSecret(ctx, repository.SecretReference{
			Name:      "non-existent",
			Namespace: "kro-system",
		})

		if err != nil {
			t.Errorf("expected no error for non-existent secret, got: %v", err)
		}
	})
}

func TestCredentialManager_GetAuthTypeFromSecret(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	cm := NewCredentialManager(fakeClient, "kro-system")

	testCases := []struct {
		name       string
		secret     *corev1.Secret
		expectType string
	}{
		{
			name: "returns auth type from label",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelAuthType: repository.AuthTypeGitHubApp,
					},
				},
			},
			expectType: repository.AuthTypeGitHubApp,
		},
		{
			name: "detects SSH from secret data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Data: map[string][]byte{
					SecretKeySSHPrivateKey: []byte("ssh-key"),
				},
			},
			expectType: repository.AuthTypeSSH,
		},
		{
			name: "detects GitHub App from secret data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Data: map[string][]byte{
					SecretKeyGitHubAppID: []byte("12345"),
				},
			},
			expectType: repository.AuthTypeGitHubApp,
		},
		{
			name: "defaults to HTTPS",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{},
				},
				Data: map[string][]byte{
					SecretKeyBearerToken: []byte("token"),
				},
			},
			expectType: repository.AuthTypeHTTPS,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			authType := cm.GetAuthTypeFromSecret(tc.secret)
			if authType != tc.expectType {
				t.Errorf("expected auth type '%s', got '%s'", tc.expectType, authType)
			}
		})
	}
}

func TestValidatePEMFormat(t *testing.T) {
	testCases := []struct {
		name        string
		data        string
		fieldName   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid RSA key",
			data:        validRSAPrivateKey,
			fieldName:   "test key",
			expectError: false,
		},
		{
			name:        "valid OpenSSH key",
			data:        validSSHPrivateKey,
			fieldName:   "test key",
			expectError: false,
		},
		{
			name:        "valid certificate",
			data:        validCertificate,
			fieldName:   "test cert",
			expectError: false,
		},
		{
			name:        "empty data",
			data:        "",
			fieldName:   "test key",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "missing BEGIN header",
			data:        "MIIEpAIBAAKCAQEAtest\n-----END RSA PRIVATE KEY-----",
			fieldName:   "test key",
			expectError: true,
			errorMsg:    "missing BEGIN header",
		},
		{
			name:        "missing END header",
			data:        missingEndPEM,
			fieldName:   "test key",
			expectError: true,
			errorMsg:    "missing END header",
		},
		{
			name:        "invalid format",
			data:        invalidPEMKey,
			fieldName:   "test key",
			expectError: true,
			errorMsg:    "PEM format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePEMFormat(tc.data, tc.fieldName)
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error containing '%s', got: %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateSSHRepoURL(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "valid git@ format",
			url:         "git@github.com:org/repo.git",
			expectError: false,
		},
		{
			name:        "valid ssh:// format",
			url:         "ssh://git@github.com/org/repo.git",
			expectError: false,
		},
		{
			name:        "invalid https URL",
			url:         "https://github.com/org/repo.git",
			expectError: true,
		},
		{
			name:        "invalid plain URL",
			url:         "github.com/org/repo",
			expectError: true,
		},
		// SSRF protection test cases
		{
			name:        "SSRF: git@ loopback",
			url:         "git@127.0.0.1:org/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: git@ RFC1918 10.x",
			url:         "git@10.0.0.1:org/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: git@ RFC1918 192.168.x",
			url:         "git@192.168.1.1:org/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: ssh:// loopback",
			url:         "ssh://git@127.0.0.1/org/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: ssh:// RFC1918",
			url:         "ssh://git@10.0.0.1/org/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: git@ cloud metadata",
			url:         "git@169.254.169.254:org/repo.git",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSSHRepoURL(tc.url)
			if tc.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateHTTPSRepoURL(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name:        "valid https URL",
			url:         "https://github.com/org/repo.git",
			expectError: false,
		},
		{
			name:        "valid https URL without .git",
			url:         "https://github.com/org/repo",
			expectError: false,
		},
		{
			name:        "invalid http URL",
			url:         "http://github.com/org/repo.git",
			expectError: true,
		},
		{
			name:        "invalid SSH URL",
			url:         "git@github.com:org/repo.git",
			expectError: true,
		},
		// SSRF protection test cases
		{
			name:        "SSRF: cloud metadata endpoint",
			url:         "https://169.254.169.254/latest/meta-data",
			expectError: true,
		},
		{
			name:        "SSRF: loopback address",
			url:         "https://127.0.0.1/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: private RFC1918 10.x",
			url:         "https://10.0.0.1/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: private RFC1918 192.168.x",
			url:         "https://192.168.1.1/repo.git",
			expectError: true,
		},
		{
			name:        "SSRF: private RFC1918 172.16.x",
			url:         "https://172.16.0.1/repo.git",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHTTPSRepoURL(tc.url)
			if tc.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRepoURLForAuthType(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		authType    string
		expectError bool
	}{
		{
			name:        "SSH auth with SSH URL",
			url:         "git@github.com:org/repo.git",
			authType:    repository.AuthTypeSSH,
			expectError: false,
		},
		{
			name:        "SSH auth with HTTPS URL",
			url:         "https://github.com/org/repo.git",
			authType:    repository.AuthTypeSSH,
			expectError: true,
		},
		{
			name:        "HTTPS auth with HTTPS URL",
			url:         "https://github.com/org/repo.git",
			authType:    repository.AuthTypeHTTPS,
			expectError: false,
		},
		{
			name:        "HTTPS auth with SSH URL",
			url:         "git@github.com:org/repo.git",
			authType:    repository.AuthTypeHTTPS,
			expectError: true,
		},
		{
			name:        "GitHub App auth with HTTPS URL",
			url:         "https://github.com/org/repo.git",
			authType:    repository.AuthTypeGitHubApp,
			expectError: false,
		},
		{
			name:        "GitHub App auth with SSH URL",
			url:         "git@github.com:org/repo.git",
			authType:    repository.AuthTypeGitHubApp,
			expectError: true,
		},
		{
			name:        "unknown auth type",
			url:         "https://github.com/org/repo.git",
			authType:    "unknown",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRepoURLForAuthType(tc.url, tc.authType)
			if tc.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseGitHubRepoFromURL(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectOwner string
		expectRepo  string
		expectError bool
	}{
		{
			name:        "HTTPS URL with .git",
			url:         "https://github.com/myorg/myrepo.git",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "HTTPS URL without .git",
			url:         "https://github.com/myorg/myrepo",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "HTTPS URL with trailing slash",
			url:         "https://github.com/myorg/myrepo/",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "SSH URL git@ format",
			url:         "git@github.com:myorg/myrepo.git",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "SSH URL git@ format without .git",
			url:         "git@github.com:myorg/myrepo",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "SSH URL ssh:// format",
			url:         "ssh://git@github.com/myorg/myrepo.git",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "Enterprise HTTPS URL",
			url:         "https://github.mycompany.com/myorg/myrepo.git",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "Enterprise SSH URL",
			url:         "git@github.mycompany.com:myorg/myrepo.git",
			expectOwner: "myorg",
			expectRepo:  "myrepo",
			expectError: false,
		},
		{
			name:        "invalid URL format",
			url:         "not-a-valid-url",
			expectError: true,
		},
		{
			name:        "HTTP URL (unsupported)",
			url:         "http://github.com/myorg/myrepo",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, err := ParseGitHubRepoFromURL(tc.url)
			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if owner != tc.expectOwner {
					t.Errorf("expected owner '%s', got '%s'", tc.expectOwner, owner)
				}
				if repo != tc.expectRepo {
					t.Errorf("expected repo '%s', got '%s'", tc.expectRepo, repo)
				}
			}
		})
	}
}

func TestSanitizeK8sName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "my-repo-123",
			expected: "my-repo-123",
		},
		{
			input:    "MyRepo_Test",
			expected: "myrepo-test",
		},
		{
			input:    "repo@#$%name",
			expected: "repo-name",
		},
		{
			input:    "---repo---",
			expected: "repo",
		},
		{
			input:    strings.Repeat("a", 100),
			expected: strings.Repeat("a", 40),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitize.K8sName(tc.input)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestCredentialManager_InvalidAuthType(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	cm := NewCredentialManager(fakeClient, "kro-system")

	req := repository.CreateCredentialRequest{
		RepoConfigID: "test-repo",
		AuthType:     "invalid-auth-type",
	}

	_, err := cm.CreateCredentialSecret(ctx, req)
	if err == nil {
		t.Fatal("expected error for invalid auth type, got nil")
	}

	if !strings.Contains(err.Error(), "invalid auth type") {
		t.Errorf("unexpected error message: %v", err)
	}
}
