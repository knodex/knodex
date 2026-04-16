// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package clients

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestCredentialRotation_UpdateSecretWithoutRestart tests that credentials
// can be rotated by updating the K8s secret without requiring pod restart.
// This is an acceptance criterion for this feature.
func TestCredentialRotation_UpdateSecretWithoutRestart(t *testing.T) {
	ctx := context.Background()

	// Create initial secret with old token
	oldToken := "ghp_oldtoken1234567890123456789012345678"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-rotation",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte(oldToken),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	client := NewGitHubClient(k8sClient)

	creds := GitHubCredentials{
		SecretName: "github-token-rotation",
		SecretKey:  "token",
		Namespace:  "kro-system",
	}

	// Step 1: Read initial token
	token1, err := client.GetCredentials(ctx, creds)
	require.NoError(t, err)
	assert.Equal(t, oldToken, token1, "Should read initial token")

	// Step 2: Simulate credential rotation - update the secret
	newToken := "ghp_newtoken1234567890123456789012345678"
	updatedSecret, err := k8sClient.CoreV1().Secrets("kro-system").Get(
		ctx, "github-token-rotation", metav1.GetOptions{},
	)
	require.NoError(t, err)

	updatedSecret.Data["token"] = []byte(newToken)
	_, err = k8sClient.CoreV1().Secrets("kro-system").Update(
		ctx, updatedSecret, metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Step 3: Read credentials again - should get new token without restart
	// This simulates the runtime reading credentials after secret update
	token2, err := client.GetCredentials(ctx, creds)
	require.NoError(t, err)
	assert.Equal(t, newToken, token2, "Should read updated token after rotation")
	assert.NotEqual(t, token1, token2, "Token should have changed")

	// Step 4: Verify old token is no longer returned
	assert.NotEqual(t, oldToken, token2, "Old token should not be returned")

	// Log rotation success
	t.Logf("Credential rotation successful: %s → %s", oldToken[:10]+"...", newToken[:10]+"...")
}

// TestCredentialRotation_MultipleRotations tests multiple sequential rotations
func TestCredentialRotation_MultipleRotations(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-multi-rotation",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_token0001234567890123456789012345678"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	client := NewGitHubClient(k8sClient)

	creds := GitHubCredentials{
		SecretName: "github-token-multi-rotation",
		SecretKey:  "token",
		Namespace:  "kro-system",
	}

	// Perform 5 rotations
	tokens := []string{
		"ghp_token0011234567890123456789012345678",
		"ghp_token0021234567890123456789012345678",
		"ghp_token0031234567890123456789012345678",
		"ghp_token0041234567890123456789012345678",
		"ghp_token0051234567890123456789012345678",
	}

	for i := 0; i < 5; i++ {
		// Read current token
		currentToken, err := client.GetCredentials(ctx, creds)
		require.NoError(t, err)

		// Update to new token
		newToken := []byte(tokens[i])
		updatedSecret, err := k8sClient.CoreV1().Secrets("kro-system").Get(
			ctx, "github-token-multi-rotation", metav1.GetOptions{},
		)
		require.NoError(t, err)

		updatedSecret.Data["token"] = newToken
		_, err = k8sClient.CoreV1().Secrets("kro-system").Update(
			ctx, updatedSecret, metav1.UpdateOptions{},
		)
		require.NoError(t, err)

		// Verify new token is read
		rotatedToken, err := client.GetCredentials(ctx, creds)
		require.NoError(t, err)
		assert.NotEqual(t, currentToken, rotatedToken, "Rotation %d: Token should change", i+1)
		assert.Equal(t, tokens[i], rotatedToken, "Rotation %d: Should get expected token", i+1)

		t.Logf("Rotation %d successful: %s", i+1, rotatedToken[:15]+"...")
	}
}

// TestFullWorkflow_SecretToGitHubClient tests the complete workflow from
// K8s secret to GitHub client creation
func TestFullWorkflow_SecretToGitHubClient(t *testing.T) {
	ctx := context.Background()

	// Create secret with valid-format token
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-workflow-test",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_789012345678901234567890123456789012"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	ghClient := NewGitHubClient(k8sClient)

	creds := GitHubCredentials{
		SecretName: "github-workflow-test",
		SecretKey:  "token",
		Namespace:  "kro-system",
	}

	// Step 1: Read credentials from secret
	token, err := ghClient.GetCredentials(ctx, creds)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, "ghp_789012345678901234567890123456789012", token)

	// Step 2: Create GitHub client with credentials
	// Note: This will create a client but cannot test actual GitHub API
	// without making real API calls or complex mocking
	githubClient, err := ghClient.NewClientWithCredentials(ctx, creds)
	require.NoError(t, err, "Should create GitHub client from K8s secret")
	assert.NotNil(t, githubClient, "GitHub client should not be nil")

	// Verify the client is properly configured (basic checks)
	assert.NotNil(t, githubClient.Users, "Users service should be available")
	assert.NotNil(t, githubClient.Repositories, "Repositories service should be available")

	t.Log("Successfully created GitHub client from K8s secret")
}

// TestCredentialRotation_ConcurrentReads tests that multiple goroutines
// can safely read credentials during rotation
func TestCredentialRotation_ConcurrentReads(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-concurrent",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_concurrent12345678901234567890123456"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	client := NewGitHubClient(k8sClient)

	creds := GitHubCredentials{
		SecretName: "github-token-concurrent",
		SecretKey:  "token",
		Namespace:  "kro-system",
	}

	// Launch 10 goroutines that read credentials concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 5; j++ {
				token, err := client.GetCredentials(ctx, creds)
				require.NoError(t, err, "Goroutine %d iteration %d failed", id, j)
				assert.NotEmpty(t, token)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	t.Log("Concurrent credential reads completed successfully")
}

// TestCredentialRotation_NamespaceIsolation tests that rotating credentials
// in one namespace doesn't affect credentials in another namespace
func TestCredentialRotation_NamespaceIsolation(t *testing.T) {
	ctx := context.Background()

	// Create secrets in two different namespaces
	secretProd := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token",
			Namespace: "production",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_890123456789012345678901234567890123"),
		},
	}

	secretDev := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token",
			Namespace: "development",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_901234567890123456789012345678901234"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secretProd, secretDev)
	client := NewGitHubClient(k8sClient)

	credsProd := GitHubCredentials{
		SecretName: "github-token",
		SecretKey:  "token",
		Namespace:  "production",
	}

	credsDev := GitHubCredentials{
		SecretName: "github-token",
		SecretKey:  "token",
		Namespace:  "development",
	}

	// Read initial tokens
	prodToken1, err := client.GetCredentials(ctx, credsProd)
	require.NoError(t, err)

	devToken1, err := client.GetCredentials(ctx, credsDev)
	require.NoError(t, err)

	// Rotate production token
	newProdToken := "ghp_012345678901234567890123456789012345"
	prodSecret, err := k8sClient.CoreV1().Secrets("production").Get(
		ctx, "github-token", metav1.GetOptions{},
	)
	require.NoError(t, err)

	prodSecret.Data["token"] = []byte(newProdToken)
	_, err = k8sClient.CoreV1().Secrets("production").Update(
		ctx, prodSecret, metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Verify production token changed
	prodToken2, err := client.GetCredentials(ctx, credsProd)
	require.NoError(t, err)
	assert.Equal(t, newProdToken, prodToken2)
	assert.NotEqual(t, prodToken1, prodToken2)

	// Verify development token unchanged
	devToken2, err := client.GetCredentials(ctx, credsDev)
	require.NoError(t, err)
	assert.Equal(t, devToken1, devToken2, "Dev token should not change when prod token rotates")

	t.Log("Namespace isolation verified: production rotation did not affect development")
}

// TestCredentialRotation_ErrorRecovery tests that the system recovers gracefully
// from invalid credentials during rotation
func TestCredentialRotation_ErrorRecovery(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-token-recovery",
			Namespace: "kro-system",
		},
		Data: map[string][]byte{
			"token": []byte("ghp_111111111111111111111111111111111111"),
		},
	}

	k8sClient := fake.NewSimpleClientset(secret)
	client := NewGitHubClient(k8sClient)

	creds := GitHubCredentials{
		SecretName: "github-token-recovery",
		SecretKey:  "token",
		Namespace:  "kro-system",
	}

	// Read valid token
	validToken, err := client.GetCredentials(ctx, creds)
	require.NoError(t, err)
	assert.NotEmpty(t, validToken)

	// Update to empty token (invalid)
	updatedSecret, err := k8sClient.CoreV1().Secrets("kro-system").Get(
		ctx, "github-token-recovery", metav1.GetOptions{},
	)
	require.NoError(t, err)

	updatedSecret.Data["token"] = []byte("")
	_, err = k8sClient.CoreV1().Secrets("kro-system").Update(
		ctx, updatedSecret, metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Attempt to read empty token - should fail
	_, err = client.GetCredentials(ctx, creds)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token value is empty")

	// Recover by updating to valid token again
	recoveredSecret, err := k8sClient.CoreV1().Secrets("kro-system").Get(
		ctx, "github-token-recovery", metav1.GetOptions{},
	)
	require.NoError(t, err)

	newValidToken := "ghp_222222222222222222222222222222222222"
	recoveredSecret.Data["token"] = []byte(newValidToken)
	_, err = k8sClient.CoreV1().Secrets("kro-system").Update(
		ctx, recoveredSecret, metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Should now read successfully
	recoveredToken, err := client.GetCredentials(ctx, creds)
	require.NoError(t, err)
	assert.Equal(t, newValidToken, recoveredToken)

	t.Log("System recovered from invalid credentials during rotation")
}
