// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ==============================================================================
// RGD Visibility E2E Tests
//
// Simplified Visibility Model
//
// The knodex.io/catalog annotation is the GATEWAY to the catalog:
// - No catalog annotation → NOT in catalog (invisible to everyone, including admins)
// - catalog: true (no project label) → PUBLIC (visible to ALL authenticated users)
// - catalog: true + project label → RESTRICTED (visible only to project members)
//
// Casbin Controls Route Access:
// - Casbin (route level): Controls WHO can access the /api/v1/rgds endpoint
// - Handler (data level): Filters RGDs based on catalog + project visibility
//
// Key Change from prior changes:
// - Removed knodex.io/visibility label entirely
// - catalog: true without project label = public (was previously catalog-only)
// ==============================================================================

var (
	rgdGVR = schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}

	// setupOnce ensures visibility test RGDs are created only once across all tests
	// This prevents race conditions from rapid delete-create cycles that overwhelm
	// the Kubernetes informer
	setupOnce sync.Once

	// testRGDsReady signals that test RGDs have been created and synced to cache
	testRGDsReady = make(chan struct{})
)

// Test RGD definitions for visibility testing
//
// Visibility Rules:
// - knodex.io/catalog: "true" → Gateway to catalog (REQUIRED for visibility)
// - catalog: true (no project label) → PUBLIC (visible to ALL authenticated users)
// - catalog: true + knodex.io/project: "<project>" → RESTRICTED (visible to project members only)
// - No catalog annotation → NOT in catalog (invisible to everyone, including admins)
var visibilityTestRGDs = []struct {
	name        string
	labels      map[string]string
	annotations map[string]string
	description string
}{
	{
		name:   "e2e-public-rgd",
		labels: map[string]string{
			// No project label = PUBLIC (visible to all authenticated users)
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true", // Gateway to catalog
			"knodex.io/description": "Public RGD for E2E testing",
			"knodex.io/category":    "testing",
		},
		description: "Public RGD - catalog: true with no project label = visible to all",
	},
	{
		name: "e2e-alpha-project-rgd",
		labels: map[string]string{
			"knodex.io/project": "proj-alpha-team", // Restricts to alpha team
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true", // Gateway to catalog
			"knodex.io/description": "Alpha team project RGD for E2E testing",
			"knodex.io/category":    "testing",
		},
		description: "Alpha project RGD - catalog: true + project label = visible only to alpha team",
	},
	{
		name: "e2e-beta-project-rgd",
		labels: map[string]string{
			"knodex.io/project": "proj-beta-team", // Restricts to beta team
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true", // Gateway to catalog
			"knodex.io/description": "Beta team project RGD for E2E testing",
			"knodex.io/category":    "testing",
		},
		description: "Beta project RGD - catalog: true + project label = visible only to beta team",
	},
	{
		name:   "e2e-another-public-rgd",
		labels: map[string]string{
			// No project label = PUBLIC
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true", // Gateway to catalog
			"knodex.io/description": "Another public RGD for testing",
			"knodex.io/category":    "testing",
		},
		description: "Another public RGD - catalog: true with no project label = visible to all",
	},
	{
		name:   "e2e-catalog-public-rgd",
		labels: map[string]string{
			// No project label = PUBLIC (change: was catalog-only, now PUBLIC)
		},
		annotations: map[string]string{
			"knodex.io/catalog":     "true", // Gateway + no project = PUBLIC
			"knodex.io/description": "Catalog-only RGD (now PUBLIC )",
			"knodex.io/category":    "testing",
		},
		description: "Note: catalog: true with no project = PUBLIC (not admin-only)",
	},
	{
		name:   "e2e-non-catalog-rgd",
		labels: map[string]string{
			// No project label
		},
		annotations: map[string]string{
			// NO catalog annotation = NOT in catalog (invisible to everyone)
		},
		description: "Non-catalog RGD - no catalog annotation = invisible to everyone (including admins)",
	},
}

// catalogTestRGDNames returns names of test RGDs that have catalog annotation
// These are the RGDs we expect to see in catalog API results
func catalogTestRGDNames() []string {
	var names []string
	for _, rgd := range visibilityTestRGDs {
		// Only include RGDs with catalog annotation
		if rgd.annotations["knodex.io/catalog"] == "true" {
			names = append(names, rgd.name)
		}
	}
	return names
}

// visibilityTestProjects defines the Project CRDs that must exist for visibility
// testing. RGDs with project labels (e.g., "proj-alpha-team") are only visible to
// users whose GetAccessibleProjects includes that project. GetAccessibleProjects
// enumerates K8s Project CRDs and checks Casbin access, so the Project CRDs MUST
// exist for the visibility model to work correctly.
var visibilityTestProjects = []struct {
	name  string
	roles []map[string]interface{}
}{
	{
		name: "proj-alpha-team",
		roles: []map[string]interface{}{
			{
				"name":        "viewer",
				"description": "Read-only access",
				"policies": []interface{}{
					// 3-part format: "object, action, effect"
					// Wildcard "*" auto-scopes to this project's resources
					"*, get, allow",
				},
			},
		},
	},
	{
		name: "proj-beta-team",
		roles: []map[string]interface{}{
			{
				"name":        "developer",
				"description": "Developer access",
				"policies": []interface{}{
					"*, *, allow",
				},
			},
		},
	},
}

// setupVisibilityTestRGDs creates test Project CRDs and RGDs for visibility testing.
// This function uses sync.Once to ensure resources are created only once across all tests.
// This prevents race conditions from rapid delete-create cycles that overwhelm
// the Kubernetes informer.
func setupVisibilityTestRGDs(t *testing.T) {
	t.Helper()

	setupOnce.Do(func() {
		t.Log("Setting up visibility test Projects and RGDs (once for all tests)")
		ctx := context.Background()

		// Create Project CRDs required for visibility filtering.
		// GetAccessibleProjects enumerates K8s Project CRDs, so these must exist
		// for users to see project-restricted RGDs.
		for _, proj := range visibilityTestProjects {
			// Delete if exists (cleanup from previous run)
			_ = dynamicClient.Resource(projectGVR).Delete(ctx, proj.name, metav1.DeleteOptions{})
		}
		time.Sleep(1 * time.Second)

		for _, proj := range visibilityTestProjects {
			project := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "knodex.io/v1alpha1",
					"kind":       "Project",
					"metadata": map[string]interface{}{
						"name": proj.name,
						"labels": map[string]interface{}{
							"e2e-test":            "true",
							"e2e-visibility-test": "true",
						},
					},
					"spec": map[string]interface{}{
						"description": "Visibility test project: " + proj.name,
						"destinations": []interface{}{
							map[string]interface{}{
								"namespace": proj.name,
							},
						},
						"roles": proj.roles,
					},
				},
			}
			_, err := dynamicClient.Resource(projectGVR).Create(ctx, project, metav1.CreateOptions{})
			if err != nil {
				t.Logf("Warning: Failed to create Project %s: %v", proj.name, err)
			} else {
				t.Logf("Created Project CRD: %s", proj.name)
			}
		}

		// Wait for Casbin to load policies from new Project CRDs
		t.Log("Waiting for Casbin policy sync...")
		time.Sleep(5 * time.Second)

		// Cleanup any existing test RGDs from previous runs
		for _, rgd := range visibilityTestRGDs {
			_ = dynamicClient.Resource(rgdGVR).Delete(ctx, rgd.name, metav1.DeleteOptions{})
		}
		// Wait for deletes to propagate
		time.Sleep(3 * time.Second)

		// Create test RGDs
		for _, rgd := range visibilityTestRGDs {
			err := createTestRGD(ctx, rgd.name, rgd.labels, rgd.annotations)
			if err != nil {
				t.Logf("Warning: Failed to create test RGD %s: %v", rgd.name, err)
			} else {
				t.Logf("Created test RGD: %s", rgd.name)
			}
		}

		// Wait for RGDs to be synced to cache using polling
		// Only wait for RGDs that have catalog annotation (others won't appear in API)
		expectedRGDs := catalogTestRGDNames()
		waitForRGDsInCache(t, expectedRGDs, 60*time.Second)

		// Wait for Redis list cache to expire so non-admin users get fresh results.
		// The server caches RGD list queries in Redis with 30s TTL. If earlier tests
		// (e.g., RBAC tests) cached results for users with empty AccessibleProjects,
		// those stale entries would miss our newly-created RGDs.
		t.Log("Waiting for Redis list cache TTL to expire (35s)...")
		time.Sleep(35 * time.Second)

		close(testRGDsReady)
	})

	// For subsequent tests, wait until RGDs are ready
	select {
	case <-testRGDsReady:
		// RGDs are ready
	case <-time.After(65 * time.Second):
		t.Fatal("Timeout waiting for test RGDs to be ready")
	}
}

// cleanupVisibilityTestRGDs removes test RGDs and Project CRDs
func cleanupVisibilityTestRGDs(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	for _, rgd := range visibilityTestRGDs {
		_ = dynamicClient.Resource(rgdGVR).Delete(ctx, rgd.name, metav1.DeleteOptions{})
	}
	for _, proj := range visibilityTestProjects {
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, proj.name, metav1.DeleteOptions{})
	}
}

// createTestRGD creates a minimal RGD for testing and sets its status to Active.
// The watcher's shouldIncludeInCatalog requires status.state == "Active",
// so we set the status subresource after creation to ensure test RGDs
// are visible regardless of whether KRO processes them.
func createTestRGD(ctx context.Context, name string, labels, annotations map[string]string) error {
	rgd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "ResourceGraphDefinition",
			"metadata": map[string]interface{}{
				"name":        name,
				"labels":      labels,
				"annotations": annotations,
			},
			"spec": map[string]interface{}{
				"schema": map[string]interface{}{
					"apiVersion": "v1alpha1",
					"kind":       "E2ETestResource",
					"spec":       map[string]interface{}{},
					"status":     map[string]interface{}{},
				},
				"resources": []interface{}{},
			},
		},
	}

	created, err := dynamicClient.Resource(rgdGVR).Create(ctx, rgd, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Set status.state to "Active" via status subresource so the watcher includes
	// this RGD in the catalog. Without this, the watcher would skip it because
	// status.state != "Active" (KRO may not process test RGDs with empty resources).
	created.Object["status"] = map[string]interface{}{
		"state": "Active",
	}
	_, err = dynamicClient.Resource(rgdGVR).UpdateStatus(ctx, created, metav1.UpdateOptions{})
	return err
}

// getRGDListFromResponse extracts RGD items from API response
func getRGDListFromResponse(t *testing.T, resp *http.Response) []map[string]interface{} {
	t.Helper()

	var result struct {
		Items []map[string]interface{} `json:"items"`
	}
	err := json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	return result.Items
}

// getRGDNames extracts RGD names from a list
func getRGDNames(rgds []map[string]interface{}) []string {
	names := make([]string, 0, len(rgds))
	for _, rgd := range rgds {
		if name, ok := rgd["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

// containsRGD checks if an RGD name is in the list
func containsRGD(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

// waitForRGDsInCache polls the API until all expected RGDs are visible
// Uses a global admin token to see all RGDs in the cache
func waitForRGDsInCache(t *testing.T, expectedRGDs []string, timeout time.Duration) {
	t.Helper()

	// Generate global admin token for polling
	token := GenerateTestJWT(JWTClaims{
		Subject:     "cache-poller",
		Email:       "cache-poller@e2e-test.local",
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})

	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
		if err != nil {
			t.Logf("Poll error: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		rgds := getRGDListFromResponse(t, resp)
		resp.Body.Close()
		names := getRGDNames(rgds)

		// Check if all expected RGDs are present
		allFound := true
		var missing []string
		for _, expected := range expectedRGDs {
			if !containsRGD(names, expected) {
				allFound = false
				missing = append(missing, expected)
			}
		}

		if allFound {
			t.Logf("All %d expected RGDs found in cache", len(expectedRGDs))
			return
		}

		t.Logf("Waiting for RGDs in cache... found %d/%d, missing: %v",
			len(expectedRGDs)-len(missing), len(expectedRGDs), missing)
		time.Sleep(pollInterval)
	}

	t.Fatalf("Timeout waiting for all RGDs to appear in cache after %v", timeout)
}

// ==============================================================================
// Test Cases
// ==============================================================================

func TestE2E_RGDVisibility_GlobalAdmin_SeesAllRGDs(t *testing.T) {
	setupVisibilityTestRGDs(t)

	// Global admin sees ALL catalog RGDs (catalog annotation is gateway)
	// Non-catalog RGDs are invisible to everyone, including global admins
	// Uses the global admin User CR: user-global-admin (admin@e2e-test.local)
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-global-admin",
		Email:       "admin@e2e-test.local",
		Projects:    []string{"proj-alpha-team", "proj-beta-team", "proj-shared"},
		CasbinRoles: []string{"role:serveradmin"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)

	t.Logf("Global admin sees %d RGDs: %v", len(names), names)

	// Global admin should see ALL catalog RGDs (5 with catalog annotation)
	assert.True(t, containsRGD(names, "e2e-public-rgd"), "Global admin should see public RGD")
	assert.True(t, containsRGD(names, "e2e-alpha-project-rgd"), "Global admin should see alpha project RGD")
	assert.True(t, containsRGD(names, "e2e-beta-project-rgd"), "Global admin should see beta project RGD")
	assert.True(t, containsRGD(names, "e2e-another-public-rgd"), "Global admin should see another public RGD")
	assert.True(t, containsRGD(names, "e2e-catalog-public-rgd"), "Global admin should see catalog-public RGD")

	// Non-catalog RGD is invisible to everyone (including global admin)
	assert.False(t, containsRGD(names, "e2e-non-catalog-rgd"), "Non-catalog RGD should NOT be visible (no catalog annotation)")
}

func TestE2E_RGDVisibility_AlphaUser_SeesPublicAndAlphaRGDs(t *testing.T) {
	setupVisibilityTestRGDs(t)

	// Alpha team member sees:
	// - Public RGDs (catalog: true, no project label)
	// - Alpha project RGDs (catalog: true + project: proj-alpha-team)
	// Should NOT see: beta project RGDs, non-catalog RGDs
	// Uses the alpha viewer User CR: user-alpha-viewer (alpha-viewer@e2e-test.local)
	// NOTE: CasbinRoles in JWT are for frontend display only; server-side authorization
	// uses GetImplicitRolesForUser from Casbin's authoritative state (STORY-228)
	// The viewer role has "rgd, get, *, allow" permission
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-alpha-viewer",
		Email:       "alpha-viewer@e2e-test.local",
		Projects:    []string{"proj-alpha-team"},
		CasbinRoles: []string{"proj:proj-alpha-team:viewer"},
		Groups:      []string{"alpha-viewers"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)

	t.Logf("Alpha user sees %d RGDs: %v", len(names), names)

	// Should see public RGDs (catalog: true, no project label)
	assert.True(t, containsRGD(names, "e2e-public-rgd"), "Alpha user should see public RGD")
	assert.True(t, containsRGD(names, "e2e-another-public-rgd"), "Alpha user should see another public RGD")
	assert.True(t, containsRGD(names, "e2e-catalog-public-rgd"), "Alpha user should see catalog-public RGD (it's public now)")

	// Should see alpha project RGDs
	assert.True(t, containsRGD(names, "e2e-alpha-project-rgd"), "Alpha user should see alpha project RGD")

	// Should NOT see other project RGDs
	assert.False(t, containsRGD(names, "e2e-beta-project-rgd"), "Alpha user should NOT see beta project RGD")

	// Should NOT see non-catalog RGD
	assert.False(t, containsRGD(names, "e2e-non-catalog-rgd"), "Alpha user should NOT see non-catalog RGD")
}

func TestE2E_RGDVisibility_BetaUser_SeesPublicAndBetaRGDs(t *testing.T) {
	setupVisibilityTestRGDs(t)

	// Beta team member sees:
	// - Public RGDs (catalog: true, no project label)
	// - Beta project RGDs (catalog: true + project: proj-beta-team)
	// Should NOT see: alpha project RGDs, non-catalog RGDs
	// Uses the beta viewer User CR: user-beta-developer (beta-dev@e2e-test.local)
	// NOTE: CasbinRoles in JWT are for frontend display only; server-side authorization
	// uses GetImplicitRolesForUser from Casbin's authoritative state (STORY-228)
	// The developer role has "rgd, get, *, allow" permission
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-beta-developer",
		Email:       "beta-dev@e2e-test.local",
		Projects:    []string{"proj-beta-team"},
		CasbinRoles: []string{"proj:proj-beta-team:developer"},
		Groups:      []string{"beta-developers"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)

	t.Logf("Beta user sees %d RGDs: %v", len(names), names)

	// Should see public RGDs (catalog: true, no project label)
	assert.True(t, containsRGD(names, "e2e-public-rgd"), "Beta user should see public RGD")
	assert.True(t, containsRGD(names, "e2e-another-public-rgd"), "Beta user should see another public RGD")
	assert.True(t, containsRGD(names, "e2e-catalog-public-rgd"), "Beta user should see catalog-public RGD (it's public now)")

	// Should see beta project RGDs
	assert.True(t, containsRGD(names, "e2e-beta-project-rgd"), "Beta user should see beta project RGD")

	// Should NOT see other project RGDs
	assert.False(t, containsRGD(names, "e2e-alpha-project-rgd"), "Beta user should NOT see alpha project RGD")

	// Should NOT see non-catalog RGD
	assert.False(t, containsRGD(names, "e2e-non-catalog-rgd"), "Beta user should NOT see non-catalog RGD")
}

func TestE2E_RGDVisibility_MultiProjectUser_SeesPublicAndBothProjectRGDs(t *testing.T) {
	setupVisibilityTestRGDs(t)

	// User CRD creation removed - RBAC uses JWT claims and Casbin policies
	// OIDC users are ephemeral, local users stored in ConfigMap/Secret
	// Multi-project user is simulated via JWT claims with multiple projects

	// User with both projects sees:
	// - Public RGDs (catalog: true, no project label)
	// - Alpha project RGDs (catalog: true + project: proj-alpha-team)
	// - Beta project RGDs (catalog: true + project: proj-beta-team)
	// Should NOT see: non-catalog RGDs
	// NOTE: CasbinRoles in JWT are for frontend display only; server-side authorization
	// uses GetImplicitRolesForUser from Casbin's authoritative state (STORY-228)
	// NOTE: proj-alpha-team has viewer role, proj-beta-team has developer role (no viewer)
	// Using viewer for alpha and developer for beta to match actual project role definitions
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-multi-project",
		Email:       "multi-project@e2e-test.local",
		Projects:    []string{"proj-alpha-team", "proj-beta-team"},
		CasbinRoles: []string{"proj:proj-alpha-team:viewer", "proj:proj-beta-team:developer"},
		Groups:      []string{"alpha-viewers", "beta-developers"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)

	t.Logf("Multi-project user sees %d RGDs: %v", len(names), names)

	// Should see public RGDs (catalog: true, no project label)
	assert.True(t, containsRGD(names, "e2e-public-rgd"), "Multi-project user should see public RGD")
	assert.True(t, containsRGD(names, "e2e-another-public-rgd"), "Multi-project user should see another public RGD")
	assert.True(t, containsRGD(names, "e2e-catalog-public-rgd"), "Multi-project user should see catalog-public RGD (it's public now)")

	// Should see both alpha and beta project RGDs
	assert.True(t, containsRGD(names, "e2e-alpha-project-rgd"), "Multi-project user should see alpha project RGD")
	assert.True(t, containsRGD(names, "e2e-beta-project-rgd"), "Multi-project user should see beta project RGD")

	// Should NOT see non-catalog RGD
	assert.False(t, containsRGD(names, "e2e-non-catalog-rgd"), "Multi-project user should NOT see non-catalog RGD")
}

func TestE2E_RGDVisibility_NoProjectUser_SeesOnlyPublicRGDs(t *testing.T) {
	setupVisibilityTestRGDs(t)

	// User with no projects sees:
	// - Public RGDs (catalog: true, no project label)
	// Should NOT see: project-restricted RGDs, non-catalog RGDs
	// Uses the no-projects User CR: user-no-projects (no-projects@e2e-test.local)
	token := GenerateTestJWT(JWTClaims{
		Subject:  "user-no-projects",
		Email:    "no-projects@e2e-test.local",
		Projects: []string{},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)

	t.Logf("No-project user sees %d RGDs: %v", len(names), names)

	// Should see public RGDs (catalog: true, no project label)
	assert.True(t, containsRGD(names, "e2e-public-rgd"), "No-project user should see public RGD")
	assert.True(t, containsRGD(names, "e2e-another-public-rgd"), "No-project user should see another public RGD")
	assert.True(t, containsRGD(names, "e2e-catalog-public-rgd"), "No-project user should see catalog-public RGD (it's public now)")

	// Should NOT see project-restricted RGDs
	assert.False(t, containsRGD(names, "e2e-alpha-project-rgd"), "No-project user should NOT see alpha project RGD")
	assert.False(t, containsRGD(names, "e2e-beta-project-rgd"), "No-project user should NOT see beta project RGD")

	// Should NOT see non-catalog RGD
	assert.False(t, containsRGD(names, "e2e-non-catalog-rgd"), "No-project user should NOT see non-catalog RGD")
}

// ==============================================================================
// Regression Test: Visibility Simplification
// ==============================================================================

func TestE2E_RGDVisibility_CatalogAnnotationIsGateway(t *testing.T) {
	// Simplified visibility model test
	//
	// New Rules:
	// - catalog: true (no project label) → PUBLIC (visible to all authenticated users)
	// - catalog: true + project label → RESTRICTED (visible to project members only)
	// - No catalog annotation → NOT in catalog (invisible to everyone)
	//
	// Key Change from prior changes:
	// - catalog: true without project label is now PUBLIC, not admin-only
	// - The visibility label was removed entirely

	setupVisibilityTestRGDs(t)

	// Use the no-projects user to test the new visibility model
	// This user has no projects and is not an admin
	// Uses the no-projects User CR: user-no-projects (no-projects@e2e-test.local)
	token := GenerateTestJWT(JWTClaims{
		Subject:  "user-no-projects",
		Email:    "no-projects@e2e-test.local",
		Projects: []string{},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)

	t.Logf("No-projects user sees %d RGDs: %v", len(names), names)

	// catalog: true with NO project label = PUBLIC
	// All these should be visible to the no-projects user
	assert.True(t, containsRGD(names, "e2e-public-rgd"),
		"Note: catalog: true with no project = PUBLIC")
	assert.True(t, containsRGD(names, "e2e-another-public-rgd"),
		"Note: catalog: true with no project = PUBLIC")
	assert.True(t, containsRGD(names, "e2e-catalog-public-rgd"),
		"Note: catalog: true with no project = PUBLIC (was admin-only )")

	// Project-restricted RGDs should NOT be visible
	assert.False(t, containsRGD(names, "e2e-alpha-project-rgd"),
		"Note: catalog: true + project label = RESTRICTED to project members")
	assert.False(t, containsRGD(names, "e2e-beta-project-rgd"),
		"Note: catalog: true + project label = RESTRICTED to project members")

	// Non-catalog RGD should NOT be visible (to anyone, including admins)
	assert.False(t, containsRGD(names, "e2e-non-catalog-rgd"),
		"Note: No catalog annotation = NOT in catalog (invisible to everyone)")
}

// ==============================================================================
// Unauthenticated Access Tests
// ==============================================================================

func TestE2E_RGDVisibility_Unauthenticated_Blocked(t *testing.T) {
	// Unauthenticated users should be blocked at the Casbin route level
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Unauthenticated users should be blocked (401)")
}

func TestE2E_RGDVisibility_InvalidToken_Blocked(t *testing.T) {
	// Invalid tokens should be rejected
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", "invalid-token", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Invalid tokens should be rejected (401)")
}
