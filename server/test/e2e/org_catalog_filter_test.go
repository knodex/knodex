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

// ==============================================================================
// Organization Catalog Filtering E2E Tests (Story 1.3)
//
// Tests verify that organization-scoped catalog filtering works correctly:
// - Enterprise build with KNODEX_ORGANIZATION: filters RGDs by org annotation
// - OSS build (no enterprise tag): all RGDs visible regardless of org annotation
//
// Prerequisites:
// - Enterprise build: ENTERPRISE_BUILD=true make qa
//   with KNODEX_ORGANIZATION=test-org set on the server deployment
// - OSS build: make qa (default, no enterprise flags)
//
// These tests create RGDs with different org annotations and verify
// the catalog API response matches the expected filtering behavior.
// ==============================================================================

// orgFilterTestRGDs defines RGDs with different organization annotations for testing
var orgFilterTestRGDs = []struct {
	name         string
	labels       map[string]string
	annotations  map[string]string
	organization string // Expected org value after parsing
}{
	{
		name:   "e2e-org-shared-rgd",
		labels: map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "Shared RGD with no org annotation",
			"knodex.io/category":    "org-test",
		},
		organization: "", // No org = shared, always visible
	},
	{
		name: "e2e-org-matching-rgd",
		labels: map[string]string{
			"knodex.io/organization": "test-org",
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "RGD belonging to test-org",
			"knodex.io/category":    "org-test",
		},
		organization: "test-org", // Matches server org = visible in EE
	},
	{
		name: "e2e-org-other-rgd",
		labels: map[string]string{
			"knodex.io/organization": "other-org",
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "RGD belonging to other-org",
			"knodex.io/category":    "org-test",
		},
		organization: "other-org", // Does NOT match server org = hidden in EE
	},
}

// createOrgTestRGDs creates the test RGDs in the cluster
func createOrgTestRGDs(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	gvr := schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	for _, rgd := range orgFilterTestRGDs {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kro.run/v1alpha1",
				"kind":       "ResourceGraphDefinition",
				"metadata": map[string]interface{}{
					"name":        rgd.name,
					"labels":      toStringInterfaceMap(rgd.labels),
					"annotations": toStringInterfaceMap(rgd.annotations),
				},
				"spec": map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
					},
					"resources": []interface{}{},
				},
			},
		}

		// Delete first if exists (idempotent setup)
		_ = dynamicClient.Resource(gvr).Delete(ctx, rgd.name, metav1.DeleteOptions{})
		time.Sleep(100 * time.Millisecond)

		_, err := dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
		require.NoError(t, err, "failed to create org test RGD: %s", rgd.name)
	}

	// Wait for watcher to sync
	time.Sleep(3 * time.Second)
}

// cleanupOrgTestRGDs removes the test RGDs
func cleanupOrgTestRGDs(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	gvr := schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	for _, rgd := range orgFilterTestRGDs {
		_ = dynamicClient.Resource(gvr).Delete(ctx, rgd.name, metav1.DeleteOptions{})
	}
}

// toStringInterfaceMap converts map[string]string to map[string]interface{} for unstructured
func toStringInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// TestOrgFilter_EnterpriseWithOrgSet tests catalog filtering when server has KNODEX_ORGANIZATION set.
// Requires: ENTERPRISE_BUILD=true and KNODEX_ORGANIZATION=test-org in server deployment.
//
// AC #1: shared visible, matching org visible, mismatching org hidden
// AC #7: GetRGD returns 404 for non-matching org
//
// To run: ENTERPRISE_BUILD=true KNODEX_ORGANIZATION=test-org make e2e
func TestOrgFilter_EnterpriseWithOrgSet(t *testing.T) {
	serverOrg := os.Getenv("KNODEX_ORGANIZATION")
	if serverOrg == "" {
		t.Skip("SKIP [prerequisite]: KNODEX_ORGANIZATION not set. This test requires an enterprise deployment with KNODEX_ORGANIZATION=test-org. Run: ENTERPRISE_BUILD=true KNODEX_ORGANIZATION=test-org make e2e")
	}
	if serverOrg != "test-org" {
		t.Skipf("SKIP [prerequisite]: expected KNODEX_ORGANIZATION=test-org, got %q", serverOrg)
	}

	createOrgTestRGDs(t)
	defer cleanupOrgTestRGDs(t)

	baseURL := os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Authenticate as admin
	adminToken := GenerateSimpleJWT("admin@test.local", nil, true)
	client := &http.Client{Timeout: 10 * time.Second}

	// AC #1: List RGDs — should see shared + matching org, NOT other-org
	resp, err := MakeSimpleAuthenticatedRequest(client, baseURL, "/api/v1/rgds?category=org-test&pageSize=100", adminToken)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
		TotalCount int `json:"totalCount"`
	}
	err = json.NewDecoder(resp.Body).Decode(&listResult)
	require.NoError(t, err)

	// Build name set from results
	visibleNames := make(map[string]bool)
	for _, item := range listResult.Items {
		visibleNames[item.Name] = true
	}

	// Shared RGD should be visible
	assert.True(t, visibleNames["e2e-org-shared-rgd"], "shared RGD (no org annotation) should be visible")

	// Matching org RGD should be visible
	assert.True(t, visibleNames["e2e-org-matching-rgd"], "matching org RGD (test-org) should be visible")

	// Other-org RGD should be hidden
	assert.False(t, visibleNames["e2e-org-other-rgd"], "other-org RGD should be hidden (org mismatch)")

	// AC #7: GetRGD for non-matching org should return 404
	resp2, err := MakeSimpleAuthenticatedRequest(client, baseURL, "/api/v1/rgds/e2e-org-other-rgd", adminToken)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp2.StatusCode, "GetRGD should return 404 to hide non-matching org RGD existence (AC #7)")
}

// TestOrgFilter_EnterpriseDefaultOrg tests catalog filtering when KNODEX_ORGANIZATION defaults to "default".
// Requires: ENTERPRISE_BUILD=true and KNODEX_ORGANIZATION=default in server deployment.
//
// AC #2: default org shows shared + default-org RGDs, hides other-org RGDs
//
// To run: ENTERPRISE_BUILD=true KNODEX_ORGANIZATION=default make e2e
func TestOrgFilter_EnterpriseDefaultOrg(t *testing.T) {
	serverOrg := os.Getenv("KNODEX_ORGANIZATION")
	if serverOrg != "default" {
		t.Skipf("SKIP [prerequisite]: KNODEX_ORGANIZATION=%q, expected 'default'. This test requires an enterprise deployment with KNODEX_ORGANIZATION=default. Run: ENTERPRISE_BUILD=true KNODEX_ORGANIZATION=default make e2e", serverOrg)
	}

	// Create RGDs with "default" org annotation instead of "test-org"
	ctx := context.Background()
	gvr := schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	defaultOrgRGDs := []struct {
		name   string
		labels map[string]string
	}{
		{name: "e2e-default-org-shared", labels: map[string]string{}},
		{name: "e2e-default-org-match", labels: map[string]string{"knodex.io/organization": "default"}},
		{name: "e2e-default-org-other", labels: map[string]string{"knodex.io/organization": "other-org"}},
	}

	for _, rgd := range defaultOrgRGDs {
		obj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "kro.run/v1alpha1",
				"kind":       "ResourceGraphDefinition",
				"metadata": map[string]interface{}{
					"name":   rgd.name,
					"labels": toStringInterfaceMap(rgd.labels),
					"annotations": map[string]interface{}{
						"knodex.io/catalog":  "true",
						"knodex.io/category": "default-org-test",
					},
				},
				"spec": map[string]interface{}{
					"schema":    map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap"},
					"resources": []interface{}{},
				},
			},
		}
		_ = dynamicClient.Resource(gvr).Delete(ctx, rgd.name, metav1.DeleteOptions{})
		time.Sleep(100 * time.Millisecond)
		_, err := dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
		require.NoError(t, err, "failed to create default-org test RGD: %s", rgd.name)
	}
	defer func() {
		for _, rgd := range defaultOrgRGDs {
			_ = dynamicClient.Resource(gvr).Delete(ctx, rgd.name, metav1.DeleteOptions{})
		}
	}()
	time.Sleep(3 * time.Second)

	baseURL := os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	adminToken := GenerateSimpleJWT("admin@test.local", nil, true)
	client := &http.Client{Timeout: 10 * time.Second}

	// AC #2: List RGDs with default org — shared + default visible, other-org hidden
	resp, err := MakeSimpleAuthenticatedRequest(client, baseURL, "/api/v1/rgds?category=default-org-test&pageSize=100", adminToken)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
		TotalCount int `json:"totalCount"`
	}
	err = json.NewDecoder(resp.Body).Decode(&listResult)
	require.NoError(t, err)

	visibleNames := make(map[string]bool)
	for _, item := range listResult.Items {
		visibleNames[item.Name] = true
	}

	assert.True(t, visibleNames["e2e-default-org-shared"], "shared RGD should be visible with default org")
	assert.True(t, visibleNames["e2e-default-org-match"], "default-org RGD should be visible when server org is 'default'")
	assert.False(t, visibleNames["e2e-default-org-other"], "other-org RGD should be hidden when server org is 'default'")
}

// TestOrgFilter_OSSAllVisible tests that OSS builds show all RGDs regardless of org annotation.
//
// AC #3: OSS build shows all RGDs regardless of org annotation
//
// To run: make e2e (default OSS deployment, no KNODEX_ORGANIZATION env var)
func TestOrgFilter_OSSAllVisible(t *testing.T) {
	serverOrg := os.Getenv("KNODEX_ORGANIZATION")
	if serverOrg != "" {
		t.Skip("SKIP [prerequisite]: KNODEX_ORGANIZATION is set — this test requires an OSS deployment without org filtering. Run: make e2e (without KNODEX_ORGANIZATION)")
	}

	createOrgTestRGDs(t)
	defer cleanupOrgTestRGDs(t)

	baseURL := os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Authenticate as admin
	adminToken := GenerateSimpleJWT("admin@test.local", nil, true)
	client := &http.Client{Timeout: 10 * time.Second}

	// AC #3: All RGDs should be visible in OSS (no org filtering)
	resp, err := MakeSimpleAuthenticatedRequest(client, baseURL, "/api/v1/rgds?category=org-test&pageSize=100", adminToken)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResult struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
		TotalCount int `json:"totalCount"`
	}
	err = json.NewDecoder(resp.Body).Decode(&listResult)
	require.NoError(t, err)

	visibleNames := make(map[string]bool)
	for _, item := range listResult.Items {
		visibleNames[item.Name] = true
	}

	// In OSS, ALL RGDs should be visible regardless of org annotation
	assert.True(t, visibleNames["e2e-org-shared-rgd"], "shared RGD should be visible in OSS")
	assert.True(t, visibleNames["e2e-org-matching-rgd"], "test-org RGD should be visible in OSS (no filtering)")
	assert.True(t, visibleNames["e2e-org-other-rgd"], "other-org RGD should be visible in OSS (no filtering)")
}
