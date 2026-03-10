// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// crdGVR is the GVR for CustomResourceDefinitions (used for cleanup)
var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

// TestSchemaCacheInvalidationOnRGDChange verifies that when an RGD's schema
// changes, the backend schema cache is invalidated and subsequent requests
// return the updated schema.
//
// Flow:
// 1. Create an RGD with fieldA in spec (KRO creates the CRD + sets Active)
// 2. Wait for Active status
// 3. Fetch schema — verify fieldA present (populates cache)
// 4. Update the RGD spec to add fieldB (KRO updates the CRD)
// 5. The watcher fires update → InvalidateCache()
// 6. Poll schema endpoint until fieldB appears
func TestSchemaCacheInvalidationOnRGDChange(t *testing.T) {
	if os.Getenv("E2E_SKIP_KRO_TESTS") == "true" {
		t.Skip("Skipping KRO-dependent test (E2E_SKIP_KRO_TESTS=true)")
	}

	ctx := context.Background()

	const (
		rgdName = "e2e-schema-cache-test"
		crdName = "e2eschemacaches.kro.run" // CRD that KRO creates for this RGD
	)

	// Generate admin JWT for schema access
	adminToken := GenerateTestJWT(JWTClaims{
		Subject:     "schema-test-admin@example.com",
		Email:       "schema-test-admin@example.com",
		CasbinRoles: []string{"role:serveradmin"},
	})

	// --- Cleanup from previous runs ---
	// Delete both the RGD and any leftover CRD that KRO created.
	// KRO detects "breaking changes" if an old CRD has fields the new RGD doesn't define.
	_ = dynamicClient.Resource(rgdGVR).Delete(ctx, rgdName, metav1.DeleteOptions{})
	_ = dynamicClient.Resource(crdGVR).Delete(ctx, crdName, metav1.DeleteOptions{})
	time.Sleep(5 * time.Second)

	// --- Step 1: Create RGD with fieldA in spec ---
	// KRO will create the CRD and set status to Active
	t.Log("Creating RGD with fieldA (KRO will create CRD and set Active)")
	rgd := buildSchemaTestRGD(rgdName, map[string]interface{}{
		"fieldA": "string",
	})
	_, err := dynamicClient.Resource(rgdGVR).Create(ctx, rgd, metav1.CreateOptions{})
	require.NoError(t, err, "failed to create RGD")
	t.Cleanup(func() {
		_ = dynamicClient.Resource(rgdGVR).Delete(ctx, rgdName, metav1.DeleteOptions{})
		// Also delete the CRD that KRO created to avoid "breaking changes" on re-runs
		time.Sleep(2 * time.Second) // Give KRO a moment to notice the RGD deletion
		_ = dynamicClient.Resource(crdGVR).Delete(ctx, crdName, metav1.DeleteOptions{})
	})

	// --- Step 2: Wait for KRO to set Active ---
	t.Log("Waiting for KRO to process RGD and set Active")
	waitForRGDActive(t, ctx, rgdName)

	// Give the watcher time to pick up the Active RGD via informer resync.
	// The informer may not deliver the event instantly after status changes.
	time.Sleep(5 * time.Second)

	// --- Wait for watcher to pick up the RGD ---
	t.Log("Waiting for RGD to appear in API")
	waitForRGDInSchemaAPI(t, adminToken, rgdName)

	// --- Step 3: Fetch schema — verify fieldA present ---
	t.Log("Fetching initial schema (should contain fieldA)")
	schemaResp := fetchSchemaProperties(t, adminToken, rgdName)
	require.NotEmpty(t, schemaResp, "schema properties should not be empty")
	assert.Contains(t, schemaResp, "fieldA", "initial schema should contain fieldA")

	// --- Step 4: Update RGD to add fieldB ---
	// This makes KRO update the CRD, AND the watcher fires InvalidateCache()
	t.Log("Updating RGD spec to add fieldB")
	updatedRGD := buildSchemaTestRGD(rgdName, map[string]interface{}{
		"fieldA": "string",
		"fieldB": "string",
	})

	// Get current RGD to preserve resourceVersion
	current, err := dynamicClient.Resource(rgdGVR).Get(ctx, rgdName, metav1.GetOptions{})
	require.NoError(t, err, "failed to get current RGD")
	updatedRGD.SetResourceVersion(current.GetResourceVersion())

	_, err = dynamicClient.Resource(rgdGVR).Update(ctx, updatedRGD, metav1.UpdateOptions{})
	require.NoError(t, err, "failed to update RGD with fieldB")

	// --- Step 5: Wait for KRO to reconcile ---
	// KRO needs to update the CRD with the new field before the schema extractor can see it
	t.Log("Waiting for KRO to reconcile updated RGD")
	time.Sleep(3 * time.Second)

	// --- Step 6: Poll schema endpoint until fieldB appears ---
	t.Log("Polling schema endpoint for fieldB (cache should be invalidated)")
	deadline := time.Now().Add(60 * time.Second)
	var lastProps map[string]interface{}
	for time.Now().Before(deadline) {
		props := fetchSchemaProperties(t, adminToken, rgdName)
		if props != nil {
			lastProps = props
			if _, ok := props["fieldB"]; ok {
				t.Log("fieldB found in schema — cache invalidation confirmed")
				return // Success!
			}
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("fieldB did not appear in schema within 60s; last schema properties: %v", mapKeys(lastProps))
}

// buildSchemaTestRGD creates an RGD with a configmap resource template.
// The spec fields map contains field names to types (e.g., {"fieldA": "string"}).
// KRO uses these to generate the CRD's openAPIV3Schema.
func buildSchemaTestRGD(name string, specFields map[string]interface{}) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "ResourceGraphDefinition",
			"metadata": map[string]interface{}{
				"name": name,
				"annotations": map[string]interface{}{
					"knodex.io/catalog":     "true",
					"knodex.io/description": "Schema cache invalidation E2E test",
					"knodex.io/category":    "testing",
				},
			},
			"spec": map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "v1alpha1",
					"kind":       "E2ESchemaCache",
					"spec":       specFields,
					"status":     map[string]interface{}{},
				},
				"resources": []interface{}{
					map[string]interface{}{
						"id": "configmap",
						"template": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name": name + "-cm",
							},
							"data": map[string]interface{}{
								"fieldA": "${schema.spec.fieldA}",
							},
						},
					},
				},
			},
		},
	}
}

// waitForRGDActive polls until the RGD status.state becomes Active
func waitForRGDActive(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		rgd, err := dynamicClient.Resource(rgdGVR).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			if status, ok := rgd.Object["status"].(map[string]interface{}); ok {
				if state, _ := status["state"].(string); state == "Active" {
					t.Logf("RGD %s is Active", name)
					return
				}
			}
		}
		time.Sleep(2 * time.Second)
	}
	// Log conditions for debugging
	rgd, err := dynamicClient.Resource(rgdGVR).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		if status, ok := rgd.Object["status"].(map[string]interface{}); ok {
			condJSON, _ := json.MarshalIndent(status["conditions"], "", "  ")
			t.Logf("RGD conditions: %s", condJSON)
		}
	}
	t.Fatalf("RGD %s did not become Active within 60s", name)
}

// waitForRGDInSchemaAPI polls the RGD detail endpoint until it returns 200
func waitForRGDInSchemaAPI(t *testing.T, token, rgdName string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := MakeSimpleAuthenticatedRequest(httpClient, apiBaseURL, "/api/v1/rgds/"+rgdName, token)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("RGD %s did not appear in API within 30s", rgdName)
}

// fetchSchemaProperties calls GET /api/v1/rgds/{name}/schema and returns the
// spec properties map (field names → schema definitions).
func fetchSchemaProperties(t *testing.T, token, rgdName string) map[string]interface{} {
	t.Helper()

	resp, err := MakeSimpleAuthenticatedRequest(httpClient, apiBaseURL, "/api/v1/rgds/"+rgdName+"/schema", token)
	require.NoError(t, err, "schema request failed")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("schema endpoint returned %d", resp.StatusCode)
		return nil
	}

	var result struct {
		RGD      string                 `json:"rgd"`
		Schema   map[string]interface{} `json:"schema"`
		CRDFound bool                   `json:"crdFound"`
		Error    string                 `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err, "failed to decode schema response")

	if !result.CRDFound {
		t.Logf("CRD not found yet (error: %s)", result.Error)
		return nil
	}

	// Navigate to spec properties: schema.properties
	if result.Schema == nil {
		return nil
	}
	props, _ := result.Schema["properties"].(map[string]interface{})
	return props
}

func mapKeys(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
