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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ==============================================================================
// K8s Resource Listing Namespace Security E2E Tests
//
// Tests verify that the GET /api/v1/resources endpoint correctly filters
// K8s resources (Secrets, ConfigMaps) by the user's accessible namespaces
// derived from project membership, preventing data leakage across namespaces.
//
// Test scenarios:
// 1. Non-admin sees resources ONLY in project destination namespaces
// 2. Non-admin can retrieve resources from an accessible namespace
// 3. Non-admin gets 403 when querying a namespace they lack access to
// 4. Global admin sees resources across all namespaces
// ==============================================================================

const (
	// Test project for resource namespace security
	resNsProject = "e2e-res-ns-security"

	// Test users
	resNsDeveloper = "res-developer@e2e-test.local"

	// Test namespaces — the project grants access to these two
	resNsAccessible1 = "e2e-res-accessible-1"
	resNsAccessible2 = "e2e-res-accessible-2"
	// This namespace is NOT in the project destinations
	resNsInaccessible = "e2e-res-inaccessible"

	// Secret names (prefixed to avoid collisions)
	resSecretAccessible1  = "e2e-res-secret-accessible-1"
	resSecretAccessible2  = "e2e-res-secret-accessible-2"
	resSecretInaccessible = "e2e-res-secret-inaccessible"
)

var resNsSetupOnce sync.Once
var resNsReady = make(chan struct{})

// resNsK8sClient is a typed Kubernetes client for creating Secrets and Namespaces.
var resNsK8sClient kubernetes.Interface

// initResNsK8sClient initializes the Kubernetes clientset for this test suite.
func initResNsK8sClient() error {
	if resNsK8sClient != nil {
		return nil
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename()
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	resNsK8sClient = client
	return nil
}

// setupResNsFixtures creates the project, namespaces, secrets, and role bindings
// needed for resource namespace security tests.
func setupResNsFixtures(t *testing.T) {
	t.Helper()

	resNsSetupOnce.Do(func() {
		t.Log("Setting up resource namespace security test fixtures (once for all tests)")
		ctx := context.Background()

		// Initialize typed K8s client for Secret/Namespace operations
		if err := initResNsK8sClient(); err != nil {
			t.Fatalf("Failed to initialize K8s client: %v", err)
		}

		adminToken := generateTestJWT(testUserAdmin, []string{}, true)

		// Step 1: Create test namespaces (accessible + inaccessible)
		for _, ns := range []string{resNsAccessible1, resNsAccessible2, resNsInaccessible} {
			nsObj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Labels: map[string]string{
						"e2e-test":            "true",
						"e2e-res-ns-security": "true",
					},
				},
			}
			_, err := resNsK8sClient.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
			if err != nil {
				t.Logf("Namespace %s may already exist: %v", ns, err)
			} else {
				t.Logf("Created namespace: %s", ns)
			}
		}

		// Step 2: Create test Secrets in each namespace
		secrets := []struct {
			name      string
			namespace string
		}{
			{resSecretAccessible1, resNsAccessible1},
			{resSecretAccessible2, resNsAccessible2},
			{resSecretInaccessible, resNsInaccessible},
		}

		for _, s := range secrets {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      s.name,
					Namespace: s.namespace,
					Labels: map[string]string{
						"e2e-test":            "true",
						"e2e-res-ns-security": "true",
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"test-key": []byte("test-value"),
				},
			}
			// Delete first to ensure clean state
			_ = resNsK8sClient.CoreV1().Secrets(s.namespace).Delete(ctx, s.name, metav1.DeleteOptions{})
			time.Sleep(200 * time.Millisecond)

			_, err := resNsK8sClient.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create secret %s/%s: %v", s.namespace, s.name, err)
			}
			t.Logf("Created secret: %s/%s", s.namespace, s.name)
		}

		// Step 3: Create Project CRD with destination namespaces for accessible-1 and accessible-2
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, resNsProject, metav1.DeleteOptions{})
		time.Sleep(1 * time.Second)

		project := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "knodex.io/v1alpha1",
				"kind":       "Project",
				"metadata": map[string]interface{}{
					"name": resNsProject,
					"labels": map[string]interface{}{
						"e2e-test":            "true",
						"e2e-res-ns-security": "true",
					},
				},
				"spec": map[string]interface{}{
					"description": "Resource namespace security E2E test project",
					"destinations": []interface{}{
						map[string]interface{}{
							"namespace": resNsAccessible1,
						},
						map[string]interface{}{
							"namespace": resNsAccessible2,
						},
					},
					"roles": []interface{}{
						map[string]interface{}{
							"name":        "developer",
							"description": "Developer with access to project namespaces",
							"policies": []interface{}{
								"*, *, allow",
							},
						},
					},
				},
			},
		}

		_, err := dynamicClient.Resource(projectGVR).Create(ctx, project, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("Failed to create project %s: %v", resNsProject, err)
		}
		t.Logf("Created Project CRD: %s with destinations: [%s, %s]",
			resNsProject, resNsAccessible1, resNsAccessible2)

		// Wait for Casbin to sync
		t.Log("Waiting for Casbin policy sync...")
		time.Sleep(5 * time.Second)

		// Step 4: Assign developer user to the project role
		resp, err := makeAuthenticatedRequest(
			"POST",
			fmt.Sprintf("/api/v1/projects/%s/roles/developer/users/%s", resNsProject, resNsDeveloper),
			adminToken,
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to assign developer role: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			t.Fatalf("Unexpected status %d assigning developer role", resp.StatusCode)
		}
		t.Logf("Assigned developer role to %s in project %s", resNsDeveloper, resNsProject)

		// Wait for role assignment to propagate
		time.Sleep(2 * time.Second)

		close(resNsReady)
	})

	// For subsequent tests, wait until fixtures are ready
	select {
	case <-resNsReady:
		// Fixtures are ready
	case <-time.After(120 * time.Second):
		t.Fatal("Timeout waiting for resource namespace security fixtures to be ready")
	}
}

// resNsResourceResponse represents the GET /api/v1/resources response.
type resNsResourceResponse struct {
	Items []struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"items"`
	Count int `json:"count"`
}

// generateResNsDeveloperToken creates a JWT for the developer test user with project membership.
func generateResNsDeveloperToken() string {
	return GenerateTestJWT(JWTClaims{
		Subject:  resNsDeveloper,
		Email:    resNsDeveloper,
		Projects: []string{resNsProject},
	})
}

// generateResNsAdminToken creates a JWT for a global admin.
func generateResNsAdminToken() string {
	return GenerateTestJWT(JWTClaims{
		Subject:     testUserAdmin,
		Email:       testUserAdmin,
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})
}

// ==============================================================================
// Test Cases
// ==============================================================================

// TestE2E_ResourceNsSecurity_Developer_OnlySeesAccessibleNamespaces verifies
// that a non-admin user listing Secrets without a namespace filter only sees
// resources from their project's destination namespaces.
func TestE2E_ResourceNsSecurity_Developer_OnlySeesAccessibleNamespaces(t *testing.T) {
	setupResNsFixtures(t)

	token := generateResNsDeveloperToken()

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/resources?apiVersion=v1&kind=Secret", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result resNsResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	t.Logf("Developer sees %d secrets total", result.Count)

	// Collect secret names and namespaces from our test set
	var testSecretNames []string
	var testNamespaces []string
	for _, item := range result.Items {
		// Only inspect our test secrets (other tests may have created secrets)
		if item.Name == resSecretAccessible1 || item.Name == resSecretAccessible2 || item.Name == resSecretInaccessible {
			testSecretNames = append(testSecretNames, item.Name)
			testNamespaces = append(testNamespaces, item.Namespace)
		}
	}

	t.Logf("Test secrets visible to developer: %v in namespaces: %v", testSecretNames, testNamespaces)

	// Developer SHOULD see secrets from accessible namespaces
	assert.Contains(t, testSecretNames, resSecretAccessible1,
		"Developer should see secret in accessible namespace 1")
	assert.Contains(t, testSecretNames, resSecretAccessible2,
		"Developer should see secret in accessible namespace 2")

	// Developer should NOT see secret from inaccessible namespace
	assert.NotContains(t, testSecretNames, resSecretInaccessible,
		"Developer should NOT see secret in inaccessible namespace")

	// Verify none of the returned items are from the inaccessible namespace
	for _, item := range result.Items {
		assert.NotEqual(t, resNsInaccessible, item.Namespace,
			"Developer should not see ANY resource from inaccessible namespace %s, but found %s",
			resNsInaccessible, item.Name)
	}
}

// TestE2E_ResourceNsSecurity_Developer_CanQueryAccessibleNamespace verifies
// that a non-admin user can explicitly query a namespace they have access to.
func TestE2E_ResourceNsSecurity_Developer_CanQueryAccessibleNamespace(t *testing.T) {
	setupResNsFixtures(t)

	token := generateResNsDeveloperToken()

	path := fmt.Sprintf("/api/v1/resources?apiVersion=v1&kind=Secret&namespace=%s", resNsAccessible1)
	resp, err := makeAuthenticatedRequest("GET", path, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Developer should be able to query accessible namespace")

	var result resNsResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	t.Logf("Developer sees %d secrets in %s", result.Count, resNsAccessible1)

	// Should contain our test secret
	var foundTestSecret bool
	for _, item := range result.Items {
		if item.Name == resSecretAccessible1 {
			foundTestSecret = true
			assert.Equal(t, resNsAccessible1, item.Namespace,
				"Secret should be in the requested namespace")
		}
	}
	assert.True(t, foundTestSecret,
		"Developer should find test secret %s in namespace %s", resSecretAccessible1, resNsAccessible1)
}

// TestE2E_ResourceNsSecurity_Developer_ForbiddenForInaccessibleNamespace verifies
// that a non-admin user gets 403 when querying a namespace outside their projects.
func TestE2E_ResourceNsSecurity_Developer_ForbiddenForInaccessibleNamespace(t *testing.T) {
	setupResNsFixtures(t)

	token := generateResNsDeveloperToken()

	path := fmt.Sprintf("/api/v1/resources?apiVersion=v1&kind=Secret&namespace=%s", resNsInaccessible)
	resp, err := makeAuthenticatedRequest("GET", path, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Developer should get 403 when querying inaccessible namespace %s", resNsInaccessible)

	var errResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	require.NoError(t, err)

	t.Logf("403 response body: %v", errResp)
}

// TestE2E_ResourceNsSecurity_Developer_ForbiddenForSystemNamespace verifies
// that a non-admin user gets 403 when querying a system namespace.
func TestE2E_ResourceNsSecurity_Developer_ForbiddenForSystemNamespace(t *testing.T) {
	setupResNsFixtures(t)

	token := generateResNsDeveloperToken()

	// kube-system is a well-known system namespace no project should grant access to
	resp, err := makeAuthenticatedRequest("GET",
		"/api/v1/resources?apiVersion=v1&kind=Secret&namespace=kube-system",
		token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Developer should get 403 when querying kube-system namespace")
}

// TestE2E_ResourceNsSecurity_Admin_SeesAllNamespaces verifies that a global
// admin can see resources across all namespaces including the inaccessible one.
func TestE2E_ResourceNsSecurity_Admin_SeesAllNamespaces(t *testing.T) {
	setupResNsFixtures(t)

	token := generateResNsAdminToken()

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/resources?apiVersion=v1&kind=Secret", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result resNsResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	t.Logf("Admin sees %d secrets total", result.Count)

	// Collect our test secrets from the response
	foundSecrets := make(map[string]string) // name -> namespace
	for _, item := range result.Items {
		switch item.Name {
		case resSecretAccessible1, resSecretAccessible2, resSecretInaccessible:
			foundSecrets[item.Name] = item.Namespace
		}
	}

	t.Logf("Admin test secrets: %v", foundSecrets)

	// Admin should see ALL three test secrets
	assert.Contains(t, foundSecrets, resSecretAccessible1,
		"Admin should see secret in accessible namespace 1")
	assert.Contains(t, foundSecrets, resSecretAccessible2,
		"Admin should see secret in accessible namespace 2")
	assert.Contains(t, foundSecrets, resSecretInaccessible,
		"Admin should see secret in inaccessible namespace (admin has global access)")

	// Verify namespace attribution is correct
	if ns, ok := foundSecrets[resSecretAccessible1]; ok {
		assert.Equal(t, resNsAccessible1, ns)
	}
	if ns, ok := foundSecrets[resSecretInaccessible]; ok {
		assert.Equal(t, resNsInaccessible, ns)
	}
}

// TestE2E_ResourceNsSecurity_Admin_CanQueryAnyNamespace verifies that a global
// admin can explicitly query any namespace including ones not in any project.
func TestE2E_ResourceNsSecurity_Admin_CanQueryAnyNamespace(t *testing.T) {
	setupResNsFixtures(t)

	token := generateResNsAdminToken()

	path := fmt.Sprintf("/api/v1/resources?apiVersion=v1&kind=Secret&namespace=%s", resNsInaccessible)
	resp, err := makeAuthenticatedRequest("GET", path, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Admin should be able to query any namespace")

	var result resNsResourceResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	t.Logf("Admin sees %d secrets in %s", result.Count, resNsInaccessible)

	var foundTestSecret bool
	for _, item := range result.Items {
		if item.Name == resSecretInaccessible {
			foundTestSecret = true
			assert.Equal(t, resNsInaccessible, item.Namespace)
		}
	}
	assert.True(t, foundTestSecret,
		"Admin should find test secret %s in namespace %s", resSecretInaccessible, resNsInaccessible)
}

// TestE2E_ResourceNsSecurity_Unauthenticated_Blocked verifies that
// unauthenticated requests to the resources endpoint are rejected.
func TestE2E_ResourceNsSecurity_Unauthenticated_Blocked(t *testing.T) {
	resp, err := makeAuthenticatedRequest("GET",
		"/api/v1/resources?apiVersion=v1&kind=Secret", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Unauthenticated request should return 401")
}
