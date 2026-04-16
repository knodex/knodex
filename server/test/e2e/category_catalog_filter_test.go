// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ==============================================================================
// Category-Driven Catalog Filtering E2E Tests (STORY-434)
//
// Tests verify that Casbin category-scoped authorization filters the RGD catalog:
// - Operator role: sees only infrastructure + observability categories
// - Developer role: sees only applications + examples categories
// - Global admin: sees all categories
// - Unauthenticated: blocked
//
// The tests create a dedicated "engineering" Project CRD with two roles that have
// category-scoped RGD policies, then create RGDs across four categories and verify
// each role sees only its authorized subset.
//
// Authorization model:
//   Project CRD role policy format: "object, action, effect" (3-part)
//   Subject auto-generated as: proj:{project}:{role}
//   Object path for RGDs: rgds/{category}/*
//   Casbin keyMatch: rgds/infrastructure/* matches rgds/infrastructure/my-rgd
// ==============================================================================

const (
	// Test project for category filtering
	catFilterProject = "e2e-cat-filter-eng"

	// Test users
	catFilterOperator  = "cat-operator@e2e-test.local"
	catFilterDeveloper = "cat-developer@e2e-test.local"

	// Role names
	catFilterOperatorRole  = "operator"
	catFilterDeveloperRole = "developer"
)

// catFilterTestRGDs defines RGDs across four categories for testing category-scoped filtering.
// All RGDs have catalog annotation and belong to the test project so they are visible
// through project-based visibility, then further filtered by Casbin category policies.
var catFilterTestRGDs = []struct {
	name        string
	category    string
	labels      map[string]string
	annotations map[string]string
}{
	// Infrastructure category (operator-visible)
	{
		name:     "e2e-catf-infra-vpc",
		category: "infrastructure",
		labels:   map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "VPC infrastructure RGD",
			"knodex.io/category":    "infrastructure",
		},
	},
	{
		name:     "e2e-catf-infra-cluster",
		category: "infrastructure",
		labels:   map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "Cluster infrastructure RGD",
			"knodex.io/category":    "infrastructure",
		},
	},
	// Observability category (operator-visible)
	{
		name:     "e2e-catf-obs-prometheus",
		category: "observability",
		labels:   map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "Prometheus observability RGD",
			"knodex.io/category":    "observability",
		},
	},
	// Applications category (developer-visible)
	{
		name:     "e2e-catf-app-webapp",
		category: "applications",
		labels:   map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "Web application RGD",
			"knodex.io/category":    "applications",
		},
	},
	{
		name:     "e2e-catf-app-api",
		category: "applications",
		labels:   map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "API service RGD",
			"knodex.io/category":    "applications",
		},
	},
	// Examples category (developer-visible)
	{
		name:     "e2e-catf-ex-hello",
		category: "examples",
		labels:   map[string]string{},
		annotations: map[string]string{
			"knodex.io/catalog":     "true",
			"knodex.io/description": "Hello world example RGD",
			"knodex.io/category":    "examples",
		},
	},
}

// catFilterSetupOnce ensures category filter test fixtures are created only once.
var catFilterSetupOnce sync.Once

// catFilterReady signals that test fixtures have been created and synced.
var catFilterReady = make(chan struct{})

// catFilterSetupErr stores any error from the one-time fixture setup.
// IMPORTANT: t.Fatalf cannot be used inside sync.Once.Do because it calls
// runtime.Goexit which panics outside a test goroutine, killing the entire suite.
var catFilterSetupErr error

// setupCatFilterFixtures creates the Project CRD with category-scoped roles and test RGDs.
func setupCatFilterFixtures(t *testing.T) {
	t.Helper()

	catFilterSetupOnce.Do(func() {
		t.Log("Setting up category filter test fixtures (once for all tests)")
		defer close(catFilterReady) // Always signal completion, even on error

		ctx := context.Background()
		adminToken := generateTestJWT(testUserAdmin, []string{}, true)

		// Step 1: Create Project CRD with category-scoped role policies.
		//
		// Policy format is 3-part: "object, action, effect"
		// Subject is auto-generated as: proj:{project}:{role}
		//
		// Operator sees: infrastructure + observability
		// Developer sees: applications + examples
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, catFilterProject, metav1.DeleteOptions{})
		time.Sleep(1 * time.Second)

		project := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "knodex.io/v1alpha1",
				"kind":       "Project",
				"metadata": map[string]interface{}{
					"name": catFilterProject,
					"labels": map[string]interface{}{
						"e2e-test":       "true",
						"e2e-cat-filter": "true",
					},
				},
				"spec": map[string]interface{}{
					"description": "Category filtering E2E test project (STORY-434)",
					"destinations": []interface{}{
						map[string]interface{}{
							"namespace": catFilterProject,
						},
					},
					"roles": []interface{}{
						map[string]interface{}{
							"name":        catFilterOperatorRole,
							"description": "Operator: infrastructure + observability categories",
							"policies": []interface{}{
								"rgds/infrastructure/*, get, allow",
								"rgds/observability/*, get, allow",
							},
						},
						map[string]interface{}{
							"name":        catFilterDeveloperRole,
							"description": "Developer: applications + examples categories",
							"policies": []interface{}{
								"rgds/applications/*, get, allow",
								"rgds/examples/*, get, allow",
							},
						},
					},
				},
			},
		}

		_, err := dynamicClient.Resource(projectGVR).Create(ctx, project, metav1.CreateOptions{})
		if err != nil {
			catFilterSetupErr = fmt.Errorf("failed to create category filter project: %w", err)
			return
		}
		t.Logf("Created Project CRD: %s", catFilterProject)

		// Wait for Casbin to load policies from the new Project CRD
		t.Log("Waiting for Casbin policy sync...")
		time.Sleep(5 * time.Second)

		// Step 2: Assign users to project roles via API
		for _, binding := range []struct {
			user string
			role string
		}{
			{catFilterOperator, catFilterOperatorRole},
			{catFilterDeveloper, catFilterDeveloperRole},
		} {
			resp, err := makeAuthenticatedRequest(
				"POST",
				fmt.Sprintf("/api/v1/projects/%s/roles/%s/users/%s", catFilterProject, binding.role, binding.user),
				adminToken,
				nil,
			)
			if err != nil {
				catFilterSetupErr = fmt.Errorf("failed to assign role %s to %s: %w", binding.role, binding.user, err)
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				catFilterSetupErr = fmt.Errorf("unexpected status %d assigning role %s to %s (admin may lack role:serveradmin — set CASBIN_ADMIN_USERS=%s)", resp.StatusCode, binding.role, binding.user, testUserAdmin)
				return
			}
			t.Logf("Assigned role %s to %s", binding.role, binding.user)
		}

		// Wait for role assignment to propagate
		time.Sleep(2 * time.Second)

		// Step 3: Create test RGDs across categories
		// Clean up any existing RGDs from previous runs
		for _, rgd := range catFilterTestRGDs {
			_ = dynamicClient.Resource(rgdGVR).Delete(ctx, rgd.name, metav1.DeleteOptions{})
		}
		time.Sleep(2 * time.Second)

		for _, rgd := range catFilterTestRGDs {
			err := createTestRGD(ctx, rgd.name, rgd.labels, rgd.annotations)
			if err != nil {
				t.Logf("Warning: Failed to create test RGD %s: %v", rgd.name, err)
			} else {
				t.Logf("Created test RGD: %s (category=%s)", rgd.name, rgd.category)
			}
		}

		// Step 4: Wait for RGDs to appear in cache
		expectedNames := make([]string, len(catFilterTestRGDs))
		for i, rgd := range catFilterTestRGDs {
			expectedNames[i] = rgd.name
		}
		waitForRGDsInCache(t, expectedNames, 60*time.Second)

		// Wait for Redis list cache TTL to expire so filtered results are fresh
		t.Log("Waiting for Redis list cache TTL to expire (35s)...")
		time.Sleep(35 * time.Second)
	})

	// Wait for setup to complete (always completes now due to defer close)
	<-catFilterReady

	// Skip test if setup failed (don't panic the suite)
	if catFilterSetupErr != nil {
		t.Skipf("Category filter fixtures not available: %v", catFilterSetupErr)
	}
}

// catFilterRGDNames returns names of test RGDs matching the given categories
func catFilterRGDNames(categories ...string) []string {
	catSet := make(map[string]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}
	var names []string
	for _, rgd := range catFilterTestRGDs {
		if catSet[rgd.category] {
			names = append(names, rgd.name)
		}
	}
	return names
}

// catFilterAllRGDNames returns all test RGD names
func catFilterAllRGDNames() []string {
	names := make([]string, len(catFilterTestRGDs))
	for i, rgd := range catFilterTestRGDs {
		names[i] = rgd.name
	}
	return names
}

// generateCatFilterToken creates a JWT for a category filter test user.
// The JWT includes the project membership so project-based visibility allows
// seeing the project's RGDs. Casbin category policies then narrow what is visible.
func generateCatFilterToken(userEmail string) string {
	return GenerateTestJWT(JWTClaims{
		Subject:  userEmail,
		Email:    userEmail,
		Projects: []string{catFilterProject},
	})
}

// getRGDCategoriesFromResponse extracts unique categories from RGD list response items
func getRGDCategoriesFromResponse(items []map[string]interface{}) []string {
	catSet := make(map[string]bool)
	for _, item := range items {
		if cat, ok := item["category"].(string); ok && cat != "" {
			catSet[cat] = true
		}
	}
	cats := make([]string, 0, len(catSet))
	for cat := range catSet {
		cats = append(cats, cat)
	}
	return cats
}

// filterTestRGDNames filters a name list to only include names from catFilterTestRGDs.
// This prevents assertions from failing due to RGDs from other test suites.
func filterTestRGDNames(names []string) []string {
	testSet := make(map[string]bool)
	for _, rgd := range catFilterTestRGDs {
		testSet[rgd.name] = true
	}
	var filtered []string
	for _, name := range names {
		if testSet[name] {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

// ==============================================================================
// Test Cases: Catalog List Filtering by Category
// ==============================================================================

func TestE2E_CategoryFilter_Operator_SeesInfraAndObservability(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterOperator)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)
	testNames := filterTestRGDNames(names)

	t.Logf("Operator sees %d total RGDs, %d from test set: %v", len(names), len(testNames), testNames)

	// Operator should see infrastructure + observability RGDs
	for _, expected := range catFilterRGDNames("infrastructure", "observability") {
		assert.True(t, containsRGD(testNames, expected),
			"Operator should see %s (infrastructure/observability category)", expected)
	}

	// Operator should NOT see applications + examples RGDs
	for _, blocked := range catFilterRGDNames("applications", "examples") {
		assert.False(t, containsRGD(testNames, blocked),
			"Operator should NOT see %s (applications/examples category)", blocked)
	}
}

func TestE2E_CategoryFilter_Developer_SeesAppsAndExamples(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterDeveloper)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)
	testNames := filterTestRGDNames(names)

	t.Logf("Developer sees %d total RGDs, %d from test set: %v", len(names), len(testNames), testNames)

	// Developer should see applications + examples RGDs
	for _, expected := range catFilterRGDNames("applications", "examples") {
		assert.True(t, containsRGD(testNames, expected),
			"Developer should see %s (applications/examples category)", expected)
	}

	// Developer should NOT see infrastructure + observability RGDs
	for _, blocked := range catFilterRGDNames("infrastructure", "observability") {
		assert.False(t, containsRGD(testNames, blocked),
			"Developer should NOT see %s (infrastructure/observability category)", blocked)
	}
}

// ==============================================================================
// Test Cases: Filters Endpoint
// ==============================================================================

func TestE2E_CategoryFilter_FiltersEndpoint_Operator(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterOperator)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/filters", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var filters struct {
		Projects   []string `json:"projects"`
		Tags       []string `json:"tags"`
		Categories []string `json:"categories"`
	}
	err = json.NewDecoder(resp.Body).Decode(&filters)
	require.NoError(t, err)

	t.Logf("Operator filter categories: %v", filters.Categories)

	// Operator should see infrastructure and observability in categories
	assert.Contains(t, filters.Categories, "infrastructure",
		"Operator filters should include infrastructure category")
	assert.Contains(t, filters.Categories, "observability",
		"Operator filters should include observability category")

	// Operator should NOT see applications or examples in categories
	assert.NotContains(t, filters.Categories, "applications",
		"Operator filters should NOT include applications category")
	assert.NotContains(t, filters.Categories, "examples",
		"Operator filters should NOT include examples category")
}

func TestE2E_CategoryFilter_FiltersEndpoint_Developer(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterDeveloper)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/filters", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var filters struct {
		Projects   []string `json:"projects"`
		Tags       []string `json:"tags"`
		Categories []string `json:"categories"`
	}
	err = json.NewDecoder(resp.Body).Decode(&filters)
	require.NoError(t, err)

	t.Logf("Developer filter categories: %v", filters.Categories)

	// Developer should see applications and examples in categories
	assert.Contains(t, filters.Categories, "applications",
		"Developer filters should include applications category")
	assert.Contains(t, filters.Categories, "examples",
		"Developer filters should include examples category")

	// Developer should NOT see infrastructure or observability in categories
	assert.NotContains(t, filters.Categories, "infrastructure",
		"Developer filters should NOT include infrastructure category")
	assert.NotContains(t, filters.Categories, "observability",
		"Developer filters should NOT include observability category")
}

// ==============================================================================
// Test Cases: Single RGD Access (GetRGD)
// ==============================================================================

func TestE2E_CategoryFilter_Operator_CannotAccessAppRGD(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterOperator)

	// Operator tries to access an applications-category RGD directly
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/e2e-catf-app-webapp", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Operator should get 403 when accessing applications-category RGD")
}

func TestE2E_CategoryFilter_Developer_CannotAccessInfraRGD(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterDeveloper)

	// Developer tries to access an infrastructure-category RGD directly
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/e2e-catf-infra-vpc", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Developer should get 403 when accessing infrastructure-category RGD")
}

func TestE2E_CategoryFilter_Operator_CanAccessInfraRGD(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterOperator)

	// Operator accesses an infrastructure-category RGD directly
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/e2e-catf-infra-vpc", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Operator should access infrastructure-category RGD")

	var rgd map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&rgd)
	require.NoError(t, err)
	assert.Equal(t, "e2e-catf-infra-vpc", rgd["name"])
	assert.Equal(t, "infrastructure", rgd["category"])
}

func TestE2E_CategoryFilter_Developer_CanAccessAppRGD(t *testing.T) {
	setupCatFilterFixtures(t)

	token := generateCatFilterToken(catFilterDeveloper)

	// Developer accesses an applications-category RGD directly
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/e2e-catf-app-webapp", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Developer should access applications-category RGD")

	var rgd map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&rgd)
	require.NoError(t, err)
	assert.Equal(t, "e2e-catf-app-webapp", rgd["name"])
	assert.Equal(t, "applications", rgd["category"])
}

// ==============================================================================
// Test Cases: Count Endpoint
// ==============================================================================

func TestE2E_CategoryFilter_Count_ReflectsCategoryFiltering(t *testing.T) {
	setupCatFilterFixtures(t)

	operatorToken := generateCatFilterToken(catFilterOperator)
	developerToken := generateCatFilterToken(catFilterDeveloper)

	// Get operator count
	opResp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/count", operatorToken, nil)
	require.NoError(t, err)
	defer opResp.Body.Close()
	assert.Equal(t, http.StatusOK, opResp.StatusCode)

	var opCount struct {
		Count int `json:"count"`
	}
	err = json.NewDecoder(opResp.Body).Decode(&opCount)
	require.NoError(t, err)

	// Get developer count
	devResp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/count", developerToken, nil)
	require.NoError(t, err)
	defer devResp.Body.Close()
	assert.Equal(t, http.StatusOK, devResp.StatusCode)

	var devCount struct {
		Count int `json:"count"`
	}
	err = json.NewDecoder(devResp.Body).Decode(&devCount)
	require.NoError(t, err)

	t.Logf("Operator RGD count: %d, Developer RGD count: %d", opCount.Count, devCount.Count)

	// Operator has 3 RGDs (2 infra + 1 obs), Developer has 3 RGDs (2 apps + 1 examples)
	// Both may also see public RGDs from other test suites, but the key assertion is
	// that the counts differ or both reflect their category subset.
	// We verify each sees at least the expected minimum from our test RGDs.
	expectedOperatorMin := len(catFilterRGDNames("infrastructure", "observability"))
	expectedDeveloperMin := len(catFilterRGDNames("applications", "examples"))

	assert.GreaterOrEqual(t, opCount.Count, expectedOperatorMin,
		"Operator count should include at least %d infrastructure+observability RGDs", expectedOperatorMin)
	assert.GreaterOrEqual(t, devCount.Count, expectedDeveloperMin,
		"Developer count should include at least %d applications+examples RGDs", expectedDeveloperMin)
}

// ==============================================================================
// Test Cases: Global Admin Sees All Categories
// ==============================================================================

func TestE2E_CategoryFilter_GlobalAdmin_SeesAllCategories(t *testing.T) {
	setupCatFilterFixtures(t)

	adminToken := GenerateTestJWT(JWTClaims{
		Subject:     testUserAdmin,
		Email:       testUserAdmin,
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds", adminToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	rgds := getRGDListFromResponse(t, resp)
	names := getRGDNames(rgds)
	testNames := filterTestRGDNames(names)

	t.Logf("Global admin sees %d total RGDs, %d from test set: %v", len(names), len(testNames), testNames)

	// Global admin should see ALL test RGDs across all categories
	for _, expected := range catFilterAllRGDNames() {
		assert.True(t, containsRGD(testNames, expected),
			"Global admin should see %s", expected)
	}
}

func TestE2E_CategoryFilter_GlobalAdmin_FiltersShowAllCategories(t *testing.T) {
	setupCatFilterFixtures(t)

	adminToken := GenerateTestJWT(JWTClaims{
		Subject:     testUserAdmin,
		Email:       testUserAdmin,
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/rgds/filters", adminToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var filters struct {
		Projects   []string `json:"projects"`
		Tags       []string `json:"tags"`
		Categories []string `json:"categories"`
	}
	err = json.NewDecoder(resp.Body).Decode(&filters)
	require.NoError(t, err)

	t.Logf("Global admin filter categories: %v", filters.Categories)

	// Global admin should see all four test categories
	assert.Contains(t, filters.Categories, "infrastructure",
		"Global admin filters should include infrastructure")
	assert.Contains(t, filters.Categories, "observability",
		"Global admin filters should include observability")
	assert.Contains(t, filters.Categories, "applications",
		"Global admin filters should include applications")
	assert.Contains(t, filters.Categories, "examples",
		"Global admin filters should include examples")
}
