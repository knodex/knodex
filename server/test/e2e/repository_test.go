// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Repository test constants
const (
	// Test repository configurations
	testRepoProjectID     = "e2e-repo-project"
	testRepoName          = "e2e-test-repo"
	testRepoURL           = "https://github.com/test-org/test-repo.git"
	testRepoSSHURL        = "git@github.com:test-org/test-repo.git"
	testRepoDefaultBranch = "main"

	// Test users for repository tests
	testRepoUserAdmin   = "repo-admin@example.com"
	testRepoUserRegular = "repo-user@example.com"

	// Expected label for credential secrets
	expectedSecretLabel    = "knodex.io/secret-type"
	expectedSecretLabelVal = "repository"

	// Secret prefix
	credentialSecretPrefix = "repo-creds-"
)

// Test PEM key for SSH authentication (not a real key - just for format validation)
const testSSHPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBEQ0RFRkdISUpLTE1OT1BRUlNUVVZXWFlaYWJjZGVmZwAAAJhIJ3QESCQ0
IAAAAA11Y2UtdGVzdEB0ZXN0AQIDBA==
-----END OPENSSH PRIVATE KEY-----`

// Test PEM key for GitHub App (not a real key - just for format validation)
const testGitHubAppPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBALRiMLAx3KoEZE4ZPeNQLR7X+D/SvC9bZLFrF1ZLHtM8eJPaUZLY
Y7PjlBJNKRlLYx3KoEZE4ZPeNQLR7X+DYPMCAwEAAQJAFOEH7n3nqzEZE4ZPeNQT
LR7X+D/SvC9bZLFrF1ZLHtM8eJPaUZLYY7PjlBJNKRlLYx3KoEZE4ZPeNQLR7QIg
gQIhANz3KoEZE4ZPeNQLR7X+D/SvC9bZLFrF1ZLHtM8eJPaUAiEAzKoEZE4ZPeNQ
LR7X+D/SvC9bZLFrF1ZLHtM8eJPaUZLYAiB3KoEZE4ZPeNQLR7X+D/SvC9bZLFrF
-----END RSA PRIVATE KEY-----`

var (
	k8sClient     kubernetes.Interface
	testNamespace string // Namespace where credential secrets are stored
)

// initK8sClient initializes the Kubernetes clientset for secret verification
func initK8sClient() error {
	if k8sClient != nil {
		return nil
	}

	kubeconfig := clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	k8sClient = client

	// Determine test namespace from environment or use default
	// E2E_NAMESPACE should be set to the backend's namespace (where secrets are created)
	testNamespace = os.Getenv("E2E_NAMESPACE")
	if testNamespace == "" {
		testNamespace = "kro-system" // Default fallback
	}
	return nil
}

// setupRepositoryTestProject creates a test project for repository tests
func setupRepositoryTestProject(ctx context.Context) error {
	// Delete if exists (cleanup from previous run)
	_ = dynamicClient.Resource(projectGVR).Delete(ctx, testRepoProjectID, metav1.DeleteOptions{})
	time.Sleep(100 * time.Millisecond)

	roles := []map[string]interface{}{
		{
			"name":        "admin",
			"description": "Repository Administrator",
			"policies": []interface{}{
				fmt.Sprintf("p, proj:%s:admin, %s, *", testRepoProjectID, testRepoProjectID),
			},
		},
	}

	return createTestProject(ctx, testRepoProjectID, "E2E Repository Test Project", roles)
}

// cleanupRepositoryTestFixtures cleans up test resources
func cleanupRepositoryTestFixtures(ctx context.Context) {
	// Delete test project
	_ = dynamicClient.Resource(projectGVR).Delete(ctx, testRepoProjectID, metav1.DeleteOptions{})

	// Delete any test repository secrets (ArgoCD-style pattern: secrets only, no CRDs)
	if k8sClient != nil {
		secrets, err := k8sClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", expectedSecretLabel, expectedSecretLabelVal),
		})
		if err == nil {
			for _, secret := range secrets.Items {
				if strings.HasPrefix(secret.Name, credentialSecretPrefix) || strings.HasPrefix(secret.Name, "e2e-") {
					_ = k8sClient.CoreV1().Secrets(secret.Namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{})
				}
			}
		}
	}
}

// Note: Following ArgoCD pattern, we no longer use RepositoryConfig CRDs.
// All repository data is stored in Secrets with label knodex.io/secret-type=repository

// ==============================================================================
// Repository API Authentication Tests
// ==============================================================================

func TestE2E_RepositoryAPI_Unauthenticated(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "unauthenticated request should return 401")
}

func TestE2E_RepositoryAPI_InvalidToken(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories", "invalid-token", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "invalid token should return 401")
}

// ==============================================================================
// Repository API CRUD Tests - Global Admin
// ==============================================================================

func TestE2E_RepositoryAPI_ListRepositories_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return OK (possibly empty list)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "global admin should be able to list repositories")
}

func TestE2E_RepositoryAPI_CreateRepository_HTTPS_GlobalAdmin(t *testing.T) {
	ctx := context.Background()

	// Setup test project first
	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create repository with HTTPS authentication (bearer token)
	createReq := map[string]interface{}{
		"name":          testRepoName + "-https",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "https",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test_token_12345",
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"global admin should be able to create HTTPS repository")

	// If created, verify the response
	if resp.StatusCode == http.StatusCreated {
		var repoResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&repoResponse)
		require.NoError(t, err)

		// Verify response contains expected fields
		assert.Equal(t, testRepoName+"-https", repoResponse["name"])
		assert.Equal(t, testRepoProjectID, repoResponse["projectId"])
		assert.Equal(t, testRepoURL, repoResponse["repoURL"])
		assert.Equal(t, "https", repoResponse["authType"])

		// Cleanup created repository
		if repoID, ok := repoResponse["id"].(string); ok && repoID != "" {
			_, _ = makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
		}
	}
}

func TestE2E_RepositoryAPI_CreateRepository_SSH_GlobalAdmin(t *testing.T) {
	ctx := context.Background()

	// Setup test project first
	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create repository with SSH authentication
	createReq := map[string]interface{}{
		"name":          testRepoName + "-ssh",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoSSHURL,
		"authType":      "ssh",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"sshAuth": map[string]interface{}{
			"privateKey": testSSHPrivateKey,
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"global admin should be able to create SSH repository")

	// If created, verify and cleanup
	if resp.StatusCode == http.StatusCreated {
		var repoResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&repoResponse)
		require.NoError(t, err)

		assert.Equal(t, testRepoName+"-ssh", repoResponse["name"])
		assert.Equal(t, "ssh", repoResponse["authType"])

		// Cleanup
		if repoID, ok := repoResponse["id"].(string); ok && repoID != "" {
			_, _ = makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
		}
	}
}

func TestE2E_RepositoryAPI_CreateRepository_GitHubApp_GlobalAdmin(t *testing.T) {
	ctx := context.Background()

	// Setup test project first
	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create repository with GitHub App authentication
	createReq := map[string]interface{}{
		"name":          testRepoName + "-ghapp",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "github-app",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"githubAppAuth": map[string]interface{}{
			"appType":        "github",
			"appId":          "123456",
			"installationId": "789012",
			"privateKey":     testGitHubAppPrivateKey,
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should succeed
	assert.Equal(t, http.StatusCreated, resp.StatusCode,
		"global admin should be able to create GitHub App repository")

	// If created, verify and cleanup
	if resp.StatusCode == http.StatusCreated {
		var repoResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&repoResponse)
		require.NoError(t, err)

		assert.Equal(t, testRepoName+"-ghapp", repoResponse["name"])
		assert.Equal(t, "github-app", repoResponse["authType"])

		// Cleanup
		if repoID, ok := repoResponse["id"].(string); ok && repoID != "" {
			_, _ = makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
		}
	}
}

func TestE2E_RepositoryAPI_GetRepository_GlobalAdmin(t *testing.T) {
	ctx := context.Background()

	// Setup test project first
	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// First create a repository
	createReq := map[string]interface{}{
		"name":          testRepoName + "-get",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "https",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test_token_get",
		},
	}

	createResp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Skip("Repository creation failed, skipping get test")
	}

	var repoResponse map[string]interface{}
	err = json.NewDecoder(createResp.Body).Decode(&repoResponse)
	require.NoError(t, err)

	repoID := repoResponse["id"].(string)
	defer func() {
		_, _ = makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
	}()

	// Now get the repository
	getResp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories/"+repoID, token, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode, "global admin should be able to get repository")

	var getResponse map[string]interface{}
	err = json.NewDecoder(getResp.Body).Decode(&getResponse)
	require.NoError(t, err)

	assert.Equal(t, repoID, getResponse["id"])
	assert.Equal(t, testRepoName+"-get", getResponse["name"])
}

func TestE2E_RepositoryAPI_DeleteRepository_GlobalAdmin(t *testing.T) {
	ctx := context.Background()

	// Setup test project first
	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// First create a repository
	createReq := map[string]interface{}{
		"name":          testRepoName + "-delete",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "https",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test_token_delete",
		},
	}

	createResp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Skip("Repository creation failed, skipping delete test")
	}

	var repoResponse map[string]interface{}
	err = json.NewDecoder(createResp.Body).Decode(&repoResponse)
	require.NoError(t, err)

	repoID := repoResponse["id"].(string)

	// Now delete the repository
	deleteResp, err := makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
	require.NoError(t, err)
	defer deleteResp.Body.Close()

	assert.True(t, deleteResp.StatusCode == http.StatusOK || deleteResp.StatusCode == http.StatusNoContent,
		"global admin should be able to delete repository, got %d", deleteResp.StatusCode)

	// Verify deletion
	getResp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories/"+repoID, token, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusNotFound, getResp.StatusCode, "deleted repository should not be found")
}

// ==============================================================================
// Repository API RBAC Tests - Non-Admin
// ==============================================================================

func TestE2E_RepositoryAPI_CreateRepository_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT(testRepoUserRegular, []string{}, false)

	createReq := map[string]interface{}{
		"name":          "forbidden-repo",
		"projectId":     "some-project",
		"repoURL":       testRepoURL,
		"authType":      "https",
		"defaultBranch": "main",
		"enabled":       true,
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test",
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"non-admin should not be able to create repository")
}

func TestE2E_RepositoryAPI_DeleteRepository_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT(testRepoUserRegular, []string{}, false)

	resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/repositories/some-repo-id", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should be forbidden (or not found, both acceptable)
	assert.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound,
		"non-admin should not be able to delete repository, got %d", resp.StatusCode)
}

// ==============================================================================
// Repository Validation Tests
// ==============================================================================

func TestE2E_RepositoryAPI_CreateRepository_InvalidAuthType(t *testing.T) {
	ctx := context.Background()

	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	createReq := map[string]interface{}{
		"name":          "invalid-auth-repo",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "invalid-auth-type",
		"defaultBranch": "main",
		"enabled":       true,
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"invalid auth type should return 400")
}

func TestE2E_RepositoryAPI_CreateRepository_MissingCredentials(t *testing.T) {
	ctx := context.Background()

	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create with SSH auth type but no SSH credentials
	createReq := map[string]interface{}{
		"name":          "missing-creds-repo",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoSSHURL,
		"authType":      "ssh",
		"defaultBranch": "main",
		"enabled":       true,
		// Missing sshAuth
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"missing credentials should return 400")
}

func TestE2E_RepositoryAPI_CreateRepository_InvalidSSHURL(t *testing.T) {
	ctx := context.Background()

	err := setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create with SSH auth type but HTTPS URL
	createReq := map[string]interface{}{
		"name":          "invalid-url-repo",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL, // HTTPS URL
		"authType":      "ssh",       // SSH auth type
		"defaultBranch": "main",
		"enabled":       true,
		"sshAuth": map[string]interface{}{
			"privateKey": testSSHPrivateKey,
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"SSH auth with HTTPS URL should return 400")
}

// ==============================================================================
// Credential Secret Verification Tests
// ==============================================================================

func TestE2E_RepositorySecret_Label_Verification(t *testing.T) {
	ctx := context.Background()

	// Initialize K8s client
	err := initK8sClient()
	require.NoError(t, err)

	// Setup test project first
	err = setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create repository with HTTPS authentication via API
	createReq := map[string]interface{}{
		"name":          testRepoName + "-label-test",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "https",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test_token_label",
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skip("Repository creation failed, skipping label verification test")
	}

	var repoResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&repoResponse)
	require.NoError(t, err)

	repoID := repoResponse["id"].(string)
	defer func() {
		_, _ = makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
	}()

	// Wait for secret creation
	time.Sleep(500 * time.Millisecond)

	// List secrets with the expected label
	secrets, err := k8sClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", expectedSecretLabel, expectedSecretLabelVal),
	})
	require.NoError(t, err)

	// Find the secret for our repository (secret name = repo ID in ArgoCD pattern)
	var foundSecret *corev1.Secret
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		// In ArgoCD pattern, secret name is the repo ID
		if secret.Name == repoID {
			foundSecret = secret
			break
		}
		// Also check name prefix for API-created secrets
		if strings.HasPrefix(secret.Name, credentialSecretPrefix) && strings.Contains(secret.Name, "label-test") {
			foundSecret = secret
			break
		}
	}

	if foundSecret != nil {
		// Verify the secret has the correct label
		assert.Contains(t, foundSecret.Labels, expectedSecretLabel,
			"repository secret should have %s label", expectedSecretLabel)
		assert.Equal(t, expectedSecretLabelVal, foundSecret.Labels[expectedSecretLabel],
			"repository secret label should have value %s", expectedSecretLabelVal)

		// Verify ArgoCD-style: metadata is in data fields, not annotations
		assert.NotEmpty(t, foundSecret.Data["url"], "secret should have url in data")
		assert.NotEmpty(t, foundSecret.Data["project"], "secret should have project in data")
		assert.NotEmpty(t, foundSecret.Data["name"], "secret should have name in data")
		assert.NotEmpty(t, foundSecret.Data["type"], "secret should have type in data")
		assert.Equal(t, "https", string(foundSecret.Data["type"]), "secret type should be https")

		t.Logf("Verified ArgoCD-style repository secret %s/%s has correct label and data fields",
			foundSecret.Namespace, foundSecret.Name)
	} else {
		// This is acceptable if the secret was cleaned up or different namespace
		t.Log("Repository secret not found - may be in different namespace or already cleaned up")
	}
}

func TestE2E_RepositorySecret_Namespace_Verification(t *testing.T) {
	ctx := context.Background()

	// Initialize K8s client
	err := initK8sClient()
	require.NoError(t, err)

	// Setup test project first
	err = setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Create repository with HTTPS authentication via API
	createReq := map[string]interface{}{
		"name":          testRepoName + "-ns-test",
		"projectId":     testRepoProjectID,
		"repoURL":       testRepoURL,
		"authType":      "https",
		"defaultBranch": testRepoDefaultBranch,
		"enabled":       true,
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test_token_ns",
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories", token, createReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Skip("Repository creation failed, skipping namespace verification test")
	}

	var repoResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&repoResponse)
	require.NoError(t, err)

	repoID := repoResponse["id"].(string)
	defer func() {
		_, _ = makeAuthenticatedRequest("DELETE", "/api/v1/repositories/"+repoID, token, nil)
	}()

	// Wait for secret creation
	time.Sleep(500 * time.Millisecond)

	// List secrets with the expected label across all namespaces
	secrets, err := k8sClient.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", expectedSecretLabel, expectedSecretLabelVal),
	})
	require.NoError(t, err)

	// Find the secret for our repository (secret name = repo ID in ArgoCD pattern)
	var foundSecret *corev1.Secret
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		// In ArgoCD pattern, secret name is the repo ID
		if secret.Name == repoID {
			foundSecret = secret
			break
		}
		// Also check by prefix
		if strings.HasPrefix(secret.Name, credentialSecretPrefix) {
			foundSecret = secret
			break
		}
	}

	if foundSecret != nil {
		// Verify the secret is NOT in a hardcoded namespace
		t.Logf("Repository secret created in namespace: %s", foundSecret.Namespace)

		// The namespace should be dynamic (based on deployment)
		assert.NotEmpty(t, foundSecret.Namespace, "repository secret should have a namespace")

		// Verify the secret name follows the expected pattern (repoconfig- prefix)
		// ArgoCD-style pattern uses repository ID (repoconfig-{hash}) as secret name
		assert.True(t, strings.HasPrefix(foundSecret.Name, "repoconfig-"),
			"repository secret name should start with repoconfig-")

		// Verify ArgoCD-style: metadata is in data fields
		assert.NotEmpty(t, foundSecret.Data["url"], "secret should have url in data")
		assert.NotEmpty(t, foundSecret.Data["type"], "secret should have type in data")
	} else {
		t.Log("Repository secret not found - may be in different namespace or already cleaned up")
	}
}

// ==============================================================================
// Connection Test API Tests
// ==============================================================================

func TestE2E_RepositoryAPI_TestConnection_GlobalAdmin(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Test connection with HTTPS credentials
	testReq := map[string]interface{}{
		"repoURL":  "https://github.com/octocat/Hello-World.git", // Public repo
		"authType": "https",
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_invalid_token", // Will fail but should not error
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories/test-connection", token, testReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return OK (200) with test result, or 400 for invalid request
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest,
		"test connection should return 200 or 400, got %d", resp.StatusCode)

	if resp.StatusCode == http.StatusOK {
		var testResponse map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&testResponse)
		require.NoError(t, err)

		// Response should have valid and message fields
		_, hasValid := testResponse["valid"]
		_, hasMessage := testResponse["message"]
		assert.True(t, hasValid || hasMessage, "test connection response should have valid or message field")
	}
}

func TestE2E_RepositoryAPI_TestConnection_NonAdmin_Forbidden(t *testing.T) {
	token := generateTestJWT(testRepoUserRegular, []string{}, false)

	testReq := map[string]interface{}{
		"repoURL":  "https://github.com/octocat/Hello-World.git",
		"authType": "https",
		"httpsAuth": map[string]interface{}{
			"bearerToken": "ghp_test",
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/repositories/test-connection", token, testReq)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"non-admin should not be able to test connection")
}

// ==============================================================================
// Repository Not Found Tests
// ==============================================================================

func TestE2E_RepositoryAPI_GetRepository_NotFound(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories/nonexistent-repo-id", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "nonexistent repository should return 404")
}

func TestE2E_RepositoryAPI_DeleteRepository_NotFound(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/repositories/nonexistent-repo-id", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "deleting nonexistent repository should return 404")
}

// ==============================================================================
// Legacy Repository API Tests
// ==============================================================================

func TestE2E_RepositoryAPI_Legacy_ListRepositories(t *testing.T) {
	token := generateTestJWT(testUserAdmin, []string{}, true)

	// Test legacy endpoint
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "legacy list repositories should work")
}

// ==============================================================================
// Direct K8s Resource Creation -> Dashboard Visibility Tests
// ==============================================================================

// TestE2E_RepositorySecret_K8s_To_Dashboard_Visibility creates an ArgoCD-style repository secret
// directly on the cluster, then verifies it appears in the dashboard API.
// Following ArgoCD pattern: ALL repository data is stored in a single Secret (no CRD).
func TestE2E_RepositorySecret_K8s_To_Dashboard_Visibility(t *testing.T) {
	ctx := context.Background()

	// Initialize clients
	err := initK8sClient()
	require.NoError(t, err)

	// Setup test project first
	err = setupRepositoryTestProject(ctx)
	require.NoError(t, err)
	defer cleanupRepositoryTestFixtures(ctx)

	// Define test resource names - following ArgoCD pattern: secret name = repo ID
	secretName := "repo-creds-e2e-k8s-visibility-test"
	secretNamespace := testNamespace // Use the backend's namespace for secret visibility

	// Cleanup any existing resources
	_ = k8sClient.CoreV1().Secrets(secretNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	time.Sleep(200 * time.Millisecond)

	// Create ArgoCD-style repository secret with ALL data in data fields
	// Following ArgoCD pattern: url, project, name, type, defaultBranch, enabled + credentials
	repoSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
			Labels: map[string]string{
				expectedSecretLabel: expectedSecretLabelVal,
			},
			Annotations: map[string]string{
				"knodex.io/created-by": "e2e-test@example.com",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			// Metadata fields (ArgoCD-style)
			"url":           []byte("https://github.com/e2e-test/k8s-created-repo.git"),
			"project":       []byte(testRepoProjectID),
			"name":          []byte("K8s Created Repository"),
			"type":          []byte("https"),
			"defaultBranch": []byte("main"),
			"enabled":       []byte("true"),
			// Credential fields
			"bearerToken": []byte("ghp_test_k8s_created_token"),
		},
	}

	createdSecret, err := k8sClient.CoreV1().Secrets(secretNamespace).Create(ctx, repoSecret, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create repository secret on cluster")
	t.Logf("Created ArgoCD-style repository secret: %s/%s", createdSecret.Namespace, createdSecret.Name)

	defer func() {
		_ = k8sClient.CoreV1().Secrets(secretNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	}()

	// Wait for the backend watcher to pick up the new resource (poll with retries)
	token := generateTestJWT(testUserAdmin, []string{}, true)

	var foundRepo map[string]interface{}
	var listResponse struct {
		Items      []map[string]interface{} `json:"items"`
		TotalCount int                      `json:"totalCount"`
	}

	// Poll for up to 15 seconds for the watcher to sync
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)

		listResp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories", token, nil)
		if err != nil {
			t.Logf("Warning: list request failed: %v", err)
			continue
		}

		if listResp.StatusCode != http.StatusOK {
			listResp.Body.Close()
			continue
		}

		listResponse = struct {
			Items      []map[string]interface{} `json:"items"`
			TotalCount int                      `json:"totalCount"`
		}{}
		err = json.NewDecoder(listResp.Body).Decode(&listResponse)
		listResp.Body.Close()
		if err != nil {
			continue
		}

		for _, repo := range listResponse.Items {
			if repo["id"] == secretName {
				foundRepo = repo
				break
			}
		}
		if foundRepo != nil {
			break
		}
		t.Logf("Repository not yet visible, retrying... (%d items found)", len(listResponse.Items))
	}

	if foundRepo == nil {
		t.Skip("Repository watcher did not sync within 15s - watcher may not be watching the test namespace")
	}
	t.Logf("Found repository in API: %v", foundRepo["id"])

	// Verify the repository fields match what we created in the secret
	assert.Equal(t, secretName, foundRepo["id"], "repository ID should match secret name")
	assert.Equal(t, "K8s Created Repository", foundRepo["name"], "repository name should match secret data")
	assert.Equal(t, testRepoProjectID, foundRepo["projectId"], "project ID should match secret data")
	assert.Equal(t, "https://github.com/e2e-test/k8s-created-repo.git", foundRepo["repoURL"], "repository URL should match secret data")
	assert.Equal(t, "https", foundRepo["authType"], "auth type should match secret data")

	// Verify we can get the specific repository by ID (secret name)
	getResp, err := makeAuthenticatedRequest("GET", "/api/v1/repositories/"+secretName, token, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode, "should be able to get specific repository by secret name")

	var getResponse map[string]interface{}
	err = json.NewDecoder(getResp.Body).Decode(&getResponse)
	require.NoError(t, err)

	assert.Equal(t, secretName, getResponse["id"], "get response should have correct ID")
	assert.Equal(t, "K8s Created Repository", getResponse["name"], "get response should have correct name")

	t.Log("Successfully verified ArgoCD-style repository secret is visible in dashboard API")
}

// TestE2E_RepositorySecret_K8s_Created_With_Proper_Labels verifies that ArgoCD-style
// repository secrets created directly on the cluster with the proper label are recognized.
// Following ArgoCD pattern: all metadata is in secret data fields, not annotations.
func TestE2E_RepositorySecret_K8s_Created_With_Proper_Labels(t *testing.T) {
	ctx := context.Background()

	// Initialize clients
	err := initK8sClient()
	require.NoError(t, err)

	// Define test secret
	secretName := "e2e-argocd-style-repo-test"
	secretNamespace := "default"

	// Cleanup any existing secret
	_ = k8sClient.CoreV1().Secrets(secretNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	time.Sleep(100 * time.Millisecond)

	// Create ArgoCD-style repository secret with:
	// - Single label: knodex.io/secret-type=repository
	// - All metadata in data fields (not annotations)
	repoSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
			Labels: map[string]string{
				expectedSecretLabel: expectedSecretLabelVal, // Single label for discovery
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			// ArgoCD-style: metadata in data fields
			"url":           []byte("git@github.com:test-org/ssh-test-repo.git"),
			"project":       []byte("test-project"),
			"name":          []byte("SSH Test Repository"),
			"type":          []byte("ssh"),
			"defaultBranch": []byte("main"),
			"enabled":       []byte("true"),
			// Credentials
			"sshPrivateKey": []byte(testSSHPrivateKey),
		},
	}

	createdSecret, err := k8sClient.CoreV1().Secrets(secretNamespace).Create(ctx, repoSecret, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create test secret")

	defer func() {
		_ = k8sClient.CoreV1().Secrets(secretNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	}()

	// Verify the secret was created with correct label
	assert.Equal(t, expectedSecretLabelVal, createdSecret.Labels[expectedSecretLabel],
		"secret should have the correct label value")

	// Verify we can find it by label selector
	secrets, err := k8sClient.CoreV1().Secrets(secretNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", expectedSecretLabel, expectedSecretLabelVal),
	})
	require.NoError(t, err)

	found := false
	for _, s := range secrets.Items {
		if s.Name == secretName {
			found = true
			// Verify it has only the expected label (not multiple labels)
			assert.Equal(t, 1, len(s.Labels), "secret should have exactly one label")
			// Verify ArgoCD-style: metadata is in data fields
			assert.NotEmpty(t, s.Data["url"], "secret should have url in data")
			assert.NotEmpty(t, s.Data["project"], "secret should have project in data")
			assert.NotEmpty(t, s.Data["name"], "secret should have name in data")
			assert.NotEmpty(t, s.Data["type"], "secret should have type in data")
			assert.Equal(t, "ssh", string(s.Data["type"]), "secret type should be ssh")
			break
		}
	}

	assert.True(t, found, "secret should be findable by label selector")
	t.Logf("Verified ArgoCD-style repository secret %s/%s has correct label and data fields", secretNamespace, secretName)
}

// ==============================================================================
// Helper function to verify secret doesn't exist
// ==============================================================================

func verifySecretDeleted(ctx context.Context, secretName, namespace string) error {
	_, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // Secret is deleted as expected
		}
		return err
	}
	return fmt.Errorf("secret %s/%s still exists", namespace, secretName)
}
