//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// =============================================================================
// STORY-187: AC-3 Race Condition Integration Tests
//
// Purpose: Test the safety net for UI-to-API timing gaps
//
// Scenario: When the RGD annotation changes between:
// 1. UI loading the RGD (sees all modes allowed)
// 2. User selecting a mode and submitting
// 3. Backend validating the request
//
// The backend MUST validate the current RGD state at request time, not rely
// on cached/stale data from when the UI loaded.
// =============================================================================

const (
	// RGD annotation key for deployment mode restrictions
	DeploymentModesAnnotation = "knodex.io/deployment-modes"

	// Retry configuration for watcher sync
	watcherSyncTimeout  = 90 * time.Second
	watcherSyncInterval = 500 * time.Millisecond

	// HTTP client timeout
	httpClientTimeout = 10 * time.Second
)

// waitForRGDInCatalog polls the catalog API until the RGD appears or timeout
func waitForRGDInCatalog(client *http.Client, backendURL, rgdName, token string) error {
	deadline := time.Now().Add(watcherSyncTimeout)

	for time.Now().Before(deadline) {
		resp, err := MakeAuthenticatedRequest(client, backendURL, "GET",
			fmt.Sprintf("/api/v1/rgds/%s", rgdName), token, nil)
		if err != nil {
			time.Sleep(watcherSyncInterval)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		time.Sleep(watcherSyncInterval)
	}

	return fmt.Errorf("timeout waiting for RGD %s to appear in catalog", rgdName)
}

// waitForRGDAnnotationSync polls until the RGD has the expected annotation value
func waitForRGDAnnotationSync(client *http.Client, backendURL, rgdName, expectedModes, token string) error {
	deadline := time.Now().Add(watcherSyncTimeout)

	for time.Now().Before(deadline) {
		resp, err := MakeAuthenticatedRequest(client, backendURL, "GET",
			fmt.Sprintf("/api/v1/rgds/%s", rgdName), token, nil)
		if err != nil {
			time.Sleep(watcherSyncInterval)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			time.Sleep(watcherSyncInterval)
			continue
		}

		// Check if the RGD has the expected allowedDeploymentModes
		var rgd map[string]interface{}
		if err := json.Unmarshal(body, &rgd); err != nil {
			time.Sleep(watcherSyncInterval)
			continue
		}

		// Check allowedDeploymentModes field
		if modes, ok := rgd["allowedDeploymentModes"].([]interface{}); ok {
			modesStr := make([]string, len(modes))
			for i, m := range modes {
				modesStr[i] = m.(string)
			}
			if strings.Join(modesStr, ",") == expectedModes {
				return nil
			}
		} else if expectedModes == "" {
			// No modes expected (all allowed)
			return nil
		}

		time.Sleep(watcherSyncInterval)
	}

	return fmt.Errorf("timeout waiting for RGD %s annotation sync (expected modes: %s)", rgdName, expectedModes)
}

// getK8sClients creates Kubernetes clients for test setup/teardown
func getK8sClients(t *testing.T) (dynamic.Interface, kubernetes.Interface) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Skipf("Cannot build kube config: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		t.Skipf("Cannot create dynamic client: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Skipf("Cannot create kube client: %v", err)
	}

	return dynamicClient, kubeClient
}

// createRaceTestRGD creates a test RGD with specified deployment modes for race condition tests
func createRaceTestRGD(ctx context.Context, dynamicClient dynamic.Interface, name string, allowedModes []string) error {
	rgdGVR := schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	annotations := map[string]interface{}{
		"knodex.io/catalog":     "true",
		"knodex.io/description": "Test RGD for race condition testing",
	}

	if len(allowedModes) > 0 {
		annotations[DeploymentModesAnnotation] = strings.Join(allowedModes, ",")
	}

	rgd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "ResourceGraphDefinition",
			"metadata": map[string]interface{}{
				"name":        name,
				"annotations": annotations,
			},
			"spec": map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "v1alpha1",
					"kind":       "RaceTestResource",
					"spec": map[string]interface{}{
						"replicas": map[string]interface{}{
							"type":    "integer",
							"default": 1,
						},
					},
				},
				"resources": []interface{}{},
			},
		},
	}

	// RGDs are cluster-scoped resources (no namespace)
	created, err := dynamicClient.Resource(rgdGVR).Create(ctx, rgd, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Set status.state to "Active" via status subresource so the watcher includes
	// this RGD in the catalog (shouldIncludeInCatalog requires Active status).
	created.Object["status"] = map[string]interface{}{
		"state": "Active",
	}
	_, err = dynamicClient.Resource(rgdGVR).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	return err
}

// updateRGDModes updates the deployment modes annotation on an existing RGD
func updateRGDModes(ctx context.Context, dynamicClient dynamic.Interface, name string, newModes []string) error {
	rgdGVR := schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	rgd, err := dynamicClient.Resource(rgdGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	annotations := rgd.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if len(newModes) > 0 {
		annotations[DeploymentModesAnnotation] = strings.Join(newModes, ",")
	} else {
		delete(annotations, DeploymentModesAnnotation)
	}
	rgd.SetAnnotations(annotations)

	_, err = dynamicClient.Resource(rgdGVR).Update(ctx, rgd, metav1.UpdateOptions{})
	return err
}

// deleteTestRGD cleans up the test RGD
func deleteTestRGD(ctx context.Context, dynamicClient dynamic.Interface, name string) error {
	rgdGVR := schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	return dynamicClient.Resource(rgdGVR).Delete(ctx, name, metav1.DeleteOptions{})
}

// TestDeploymentModeRaceCondition tests the race condition scenario:
// 1. Create RGD with all modes allowed
// 2. Simulate "UI loads RGD" (implicit - represents user seeing all modes)
// 3. Update RGD annotation to restrict to gitops-only
// 4. Attempt deployment with direct mode (simulating delayed submission)
// 5. Verify 422 rejection (fail-fast validation)
//
// This is AC-3: Race Condition Integration Test
func TestDeploymentModeRaceCondition(t *testing.T) {
	// Get backend URL
	backendURL := os.Getenv("E2E_API_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	// Check backend is healthy
	client := &http.Client{Timeout: httpClientTimeout}
	healthResp, err := client.Get(backendURL + "/healthz")
	if err != nil {
		t.Skipf("Backend not available: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Skipf("Backend not healthy: %d", healthResp.StatusCode)
	}

	// Get Kubernetes clients
	dynamicClient, _ := getK8sClients(t)
	ctx := context.Background()

	// Generate unique RGD name to avoid conflicts
	rgdName := fmt.Sprintf("race-test-rgd-%d", time.Now().UnixNano())

	// Track if RGD was created for conditional cleanup
	rgdCreated := false

	// Cleanup on test completion (only if RGD was created)
	defer func() {
		if rgdCreated {
			_ = deleteTestRGD(ctx, dynamicClient, rgdName)
		}
	}()

	t.Run("AC-3: RGD annotation changes between UI load and submission", func(t *testing.T) {
		// Step 1: Create RGD with all modes allowed (no annotation = all modes)
		err := createRaceTestRGD(ctx, dynamicClient, rgdName, nil)
		if err != nil {
			t.Fatalf("Failed to create test RGD: %v", err)
		}
		rgdCreated = true

		// Allow informer time to observe the new RGD before polling
		time.Sleep(2 * time.Second)

		// Wait for watcher to pick up the new RGD using retry/poll
		token := GenerateSimpleJWT("admin@test.local", []string{"default"}, true)
		if err := waitForRGDInCatalog(client, backendURL, rgdName, token); err != nil {
			t.Skipf("RGD not found in catalog after watcher sync (infrastructure timing): %v", err)
		}

		// Step 2: Simulate "UI loads RGD" - at this point, user sees all modes available
		// (This is implicit - the user's browser would show Direct/GitOps/Hybrid buttons)

		// Step 3: Update RGD annotation to restrict to gitops-only
		// This simulates an admin changing the RGD policy while user is filling the form
		err = updateRGDModes(ctx, dynamicClient, rgdName, []string{"gitops"})
		if err != nil {
			t.Fatalf("Failed to update RGD modes: %v", err)
		}

		// Wait for watcher to sync the annotation change using retry/poll
		// CRITICAL: We MUST wait for sync to complete, otherwise we're not testing the race condition
		if err := waitForRGDAnnotationSync(client, backendURL, rgdName, "gitops", token); err != nil {
			t.Fatalf("Watcher sync failed - cannot proceed with race condition test: %v", err)
		}

		// Step 4: Attempt deployment with direct mode (user submits stale form)
		deployReq := map[string]interface{}{
			"name":           "race-test-instance",
			"namespace":      "default",
			"rgdName":        rgdName,
			"spec":           map[string]interface{}{"replicas": 1},
			"deploymentMode": "direct", // This should be rejected - RGD now only allows gitops
		}

		resp, err := MakeAuthenticatedRequest(client, backendURL, "POST", "/api/v1/instances", token, deployReq)
		if err != nil {
			t.Fatalf("Failed to make deployment request: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Response status: %d", resp.StatusCode)
		t.Logf("Response body: %s", string(body))

		// Step 5: Verify 422 rejection
		if resp.StatusCode != http.StatusUnprocessableEntity {
			// If we got 404, the RGD might not be cataloged yet
			if resp.StatusCode == http.StatusNotFound {
				t.Skip("RGD not found in catalog - watcher may need more time")
			}
			t.Errorf("Expected 422 Unprocessable Entity, got %d: %s", resp.StatusCode, string(body))
			return
		}

		// Verify error response format
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err != nil {
			t.Fatalf("Failed to parse error response: %v", err)
		}

		// Verify error code
		code, ok := errorResp["code"].(string)
		if !ok || code != "DEPLOYMENT_MODE_NOT_ALLOWED" {
			t.Errorf("Expected code DEPLOYMENT_MODE_NOT_ALLOWED, got: %v", errorResp["code"])
		}

		// Verify details contain allowedModes as array
		details, ok := errorResp["details"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected details object, got: %v", errorResp["details"])
			return
		}

		allowedModes, ok := details["allowedModes"].([]interface{})
		if !ok {
			t.Errorf("Expected allowedModes to be an array, got: %T", details["allowedModes"])
			return
		}

		// Verify only gitops is allowed
		if len(allowedModes) != 1 {
			t.Errorf("Expected 1 allowed mode, got %d: %v", len(allowedModes), allowedModes)
		} else if allowedModes[0].(string) != "gitops" {
			t.Errorf("Expected gitops mode, got: %v", allowedModes[0])
		}

		// Verify requestedMode
		requestedMode, ok := details["requestedMode"].(string)
		if !ok || requestedMode != "direct" {
			t.Errorf("Expected requestedMode 'direct', got: %v", details["requestedMode"])
		}
	})
}

// TestDeploymentModeAnnotationFormats tests various annotation format edge cases
func TestDeploymentModeAnnotationFormats(t *testing.T) {
	backendURL := os.Getenv("E2E_API_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: httpClientTimeout}
	healthResp, err := client.Get(backendURL + "/healthz")
	if err != nil {
		t.Skipf("Backend not available: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Skipf("Backend not healthy: %d", healthResp.StatusCode)
	}

	dynamicClient, _ := getK8sClients(t)
	ctx := context.Background()

	testCases := []struct {
		name          string
		modes         []string
		attemptMode   string
		expectAllowed bool
	}{
		{
			name:          "empty annotation allows all modes",
			modes:         nil,
			attemptMode:   "direct",
			expectAllowed: true,
		},
		{
			name:          "gitops-only rejects direct",
			modes:         []string{"gitops"},
			attemptMode:   "direct",
			expectAllowed: false,
		},
		{
			name:          "gitops-only allows gitops",
			modes:         []string{"gitops"},
			attemptMode:   "gitops",
			expectAllowed: true,
		},
		{
			name:          "direct,hybrid allows direct",
			modes:         []string{"direct", "hybrid"},
			attemptMode:   "direct",
			expectAllowed: true,
		},
		{
			name:          "direct,hybrid rejects gitops",
			modes:         []string{"direct", "hybrid"},
			attemptMode:   "gitops",
			expectAllowed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rgdName := fmt.Sprintf("mode-format-test-%d", time.Now().UnixNano())

			// Track if RGD was created for conditional cleanup
			rgdCreated := false
			defer func() {
				if rgdCreated {
					_ = deleteTestRGD(ctx, dynamicClient, rgdName)
				}
			}()

			// Create RGD with specified modes
			err := createRaceTestRGD(ctx, dynamicClient, rgdName, tc.modes)
			if err != nil {
				t.Fatalf("Failed to create test RGD: %v", err)
			}
			rgdCreated = true

			// Allow informer time to observe the new RGD before polling
			time.Sleep(2 * time.Second)

			// Wait for watcher to pick up using retry/poll
			token := GenerateSimpleJWT("admin@test.local", []string{"default"}, true)
			if err := waitForRGDInCatalog(client, backendURL, rgdName, token); err != nil {
				t.Skipf("RGD not found in catalog after watcher sync (infrastructure timing): %v", err)
			}

			// Attempt deployment
			deployReq := map[string]interface{}{
				"name":           "test-instance",
				"namespace":      "default",
				"rgdName":        rgdName,
				"spec":           map[string]interface{}{"replicas": 1},
				"deploymentMode": tc.attemptMode,
			}

			// For gitops mode, we need a repository
			if tc.attemptMode == "gitops" || tc.attemptMode == "hybrid" {
				deployReq["repositoryId"] = "test-repo"
			}

			resp, err := MakeAuthenticatedRequest(client, backendURL, "POST", "/api/v1/instances", token, deployReq)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if tc.expectAllowed {
				// Mode should be allowed - verify we get success (201) or acceptable errors
				// Acceptable errors: 404 (CRD not found), 400 (validation), 500 (repo not found)
				// NOT acceptable: 422 with DEPLOYMENT_MODE_NOT_ALLOWED
				acceptableStatuses := map[int]bool{
					201: true, // Success - instance created
					400: true, // Bad request - validation error (e.g., missing repo for gitops)
					404: true, // Not found - CRD or RGD not found
					500: true, // Server error - repo connection failed, etc.
				}

				if resp.StatusCode == 422 {
					var errorResp map[string]interface{}
					if err := json.Unmarshal(body, &errorResp); err == nil {
						if code, ok := errorResp["code"].(string); ok && code == "DEPLOYMENT_MODE_NOT_ALLOWED" {
							t.Errorf("Expected mode %s to be allowed, but got DEPLOYMENT_MODE_NOT_ALLOWED", tc.attemptMode)
						}
					}
				} else if !acceptableStatuses[resp.StatusCode] {
					t.Errorf("Unexpected status code %d for allowed mode %s: %s", resp.StatusCode, tc.attemptMode, string(body))
				}
			} else {
				// Should be rejected with 422 DEPLOYMENT_MODE_NOT_ALLOWED
				if resp.StatusCode != 422 {
					// If 404, RGD might not be cataloged yet - this is a test setup issue
					if resp.StatusCode == 404 {
						t.Fatalf("RGD not found in catalog after watcher sync - test setup failed")
					}
					// Any other non-422 status is a test failure
					t.Errorf("Expected 422 for disallowed mode %s, got %d: %s", tc.attemptMode, resp.StatusCode, string(body))
				} else {
					// Verify error code is correct
					var errorResp map[string]interface{}
					if err := json.Unmarshal(body, &errorResp); err != nil {
						t.Errorf("Failed to parse error response: %v", err)
					} else {
						code, ok := errorResp["code"].(string)
						if !ok {
							t.Errorf("Missing 'code' field in error response")
						} else if code != "DEPLOYMENT_MODE_NOT_ALLOWED" {
							t.Errorf("Expected DEPLOYMENT_MODE_NOT_ALLOWED, got: %s", code)
						}
					}
				}
			}
		})
	}
}

// TestAllowedModesResponseFormat verifies the error response format per AC-4
func TestAllowedModesResponseFormat(t *testing.T) {
	backendURL := os.Getenv("E2E_API_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}

	client := &http.Client{Timeout: httpClientTimeout}
	healthResp, err := client.Get(backendURL + "/healthz")
	if err != nil {
		t.Skipf("Backend not available: %v", err)
	}
	healthResp.Body.Close()

	dynamicClient, _ := getK8sClients(t)
	ctx := context.Background()

	rgdName := fmt.Sprintf("format-test-rgd-%d", time.Now().UnixNano())

	// Track if RGD was created for conditional cleanup
	rgdCreated := false
	defer func() {
		if rgdCreated {
			_ = deleteTestRGD(ctx, dynamicClient, rgdName)
		}
	}()

	// Create gitops-only RGD
	err = createRaceTestRGD(ctx, dynamicClient, rgdName, []string{"gitops"})
	if err != nil {
		t.Fatalf("Failed to create test RGD: %v", err)
	}
	rgdCreated = true

	// Allow informer time to observe the new RGD before polling
	time.Sleep(2 * time.Second)

	// Wait for watcher to pick up using retry/poll
	token := GenerateSimpleJWT("admin@test.local", []string{"default"}, true)
	if err := waitForRGDInCatalog(client, backendURL, rgdName, token); err != nil {
		t.Skipf("RGD not found in catalog after watcher sync (infrastructure timing): %v", err)
	}

	// Attempt direct deployment
	deployReq := map[string]interface{}{
		"name":           "format-test-instance",
		"namespace":      "default",
		"rgdName":        rgdName,
		"spec":           map[string]interface{}{"replicas": 1},
		"deploymentMode": "direct",
	}

	resp, err := MakeAuthenticatedRequest(client, backendURL, "POST", "/api/v1/instances", token, deployReq)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 404 {
		t.Fatalf("RGD not found in catalog after watcher sync - test setup failed")
	}

	if resp.StatusCode != 422 {
		t.Fatalf("Expected 422, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse and validate response format
	var errorResp map[string]interface{}
	if err := json.Unmarshal(body, &errorResp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify structure: {code, message, details: {allowedModes: [], requestedMode}}
	t.Run("has code field", func(t *testing.T) {
		if _, ok := errorResp["code"]; !ok {
			t.Error("Missing 'code' field")
		}
	})

	t.Run("has message field", func(t *testing.T) {
		if _, ok := errorResp["message"]; !ok {
			t.Error("Missing 'message' field")
		}
	})

	t.Run("has details object", func(t *testing.T) {
		details, ok := errorResp["details"].(map[string]interface{})
		if !ok {
			t.Error("Missing or invalid 'details' object")
			return
		}

		t.Run("allowedModes is array", func(t *testing.T) {
			modes, ok := details["allowedModes"]
			if !ok {
				t.Error("Missing 'allowedModes' in details")
				return
			}

			// CRITICAL: Must be an array, not a string
			switch v := modes.(type) {
			case []interface{}:
				// Correct - it's an array
				if len(v) == 0 {
					t.Error("allowedModes array is empty")
				}
			case string:
				// WRONG - this is the old bug format
				t.Errorf("allowedModes should be array, not string: %q", v)
			default:
				t.Errorf("allowedModes has unexpected type: %T", modes)
			}
		})

		t.Run("requestedMode is string", func(t *testing.T) {
			mode, ok := details["requestedMode"].(string)
			if !ok {
				t.Error("Missing or invalid 'requestedMode' in details")
			}
			if mode != "direct" {
				t.Errorf("Expected requestedMode 'direct', got %q", mode)
			}
		})
	})
}
