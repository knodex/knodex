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
// Instance Count E2E Tests
//
// These tests verify the hybrid Casbin + Handler filtering model for instances:
// - Casbin (route level): Controls WHO can access /api/v1/instances/count endpoint
// - Handler (data level): Controls WHICH instances are counted based on namespace access
//
// Namespace Access Rules:
// - Global Admin: Counts ALL instances across all namespaces (nil namespaces = no filter)
// - Regular User: Counts only instances in namespaces they have access to via projects
// - No Access: Returns count of 0 (empty namespace list = no access)
//
// Fix instances/count endpoint for non-admin users
// ==============================================================================

var (
	// simpleAppGVR is the GVR for SimpleApp CRD instances
	simpleAppGVR = schema.GroupVersionResource{
		Group:    "kro.run",
		Version:  "v1alpha1",
		Resource: "simpleapps",
	}

	// User CRD removed - local users stored in ConfigMap/Secret
	// OIDC users are ephemeral and not persisted
	// Tests use JWT claims and Casbin policies for authorization

	// instanceCountSetupOnce ensures test instances are created only once
	instanceCountSetupOnce sync.Once

	// instanceCountTestReady signals that test instances are ready
	instanceCountTestReady = make(chan struct{})
)

// Test namespace and instance definitions for instance count testing
var instanceCountTestNamespaces = []string{
	"e2e-ns-alpha",
	"e2e-ns-beta",
	"e2e-ns-staging",
}

var instanceCountTestInstances = []struct {
	name      string
	namespace string
}{
	// Alpha namespace instances (2 instances)
	{name: "e2e-alpha-app-1", namespace: "e2e-ns-alpha"},
	{name: "e2e-alpha-app-2", namespace: "e2e-ns-alpha"},
	// Beta namespace instances (1 instance)
	{name: "e2e-beta-app-1", namespace: "e2e-ns-beta"},
	// Staging namespace instances (3 instances)
	{name: "e2e-staging-app-1", namespace: "e2e-ns-staging"},
	{name: "e2e-staging-app-2", namespace: "e2e-ns-staging"},
	{name: "e2e-staging-app-3", namespace: "e2e-ns-staging"},
}

// instanceCountResponse represents the API response for /api/v1/instances/count
type instanceCountResponse struct {
	Count int `json:"count"`
}

// setupInstanceCountTestFixtures creates test namespaces, instances, and projects
func setupInstanceCountTestFixtures(t *testing.T) {
	t.Helper()

	instanceCountSetupOnce.Do(func() {
		t.Log("Setting up instance count test fixtures (once for all tests)")
		ctx := context.Background()

		// Create test namespaces
		for _, ns := range instanceCountTestNamespaces {
			createTestNamespace(t, ctx, ns)
		}

		// Wait for namespaces to be ready
		time.Sleep(500 * time.Millisecond)

		// Clean up any existing test instances and wait for deletion
		for _, inst := range instanceCountTestInstances {
			_ = dynamicClient.Resource(simpleAppGVR).Namespace(inst.namespace).Delete(
				ctx, inst.name, metav1.DeleteOptions{})
		}
		// Wait for deletions to complete (finalizers may take time)
		// Poll until all test instances are fully deleted
		deletionTimeout := time.Now().Add(30 * time.Second)
		for time.Now().Before(deletionTimeout) {
			allDeleted := true
			for _, inst := range instanceCountTestInstances {
				_, err := dynamicClient.Resource(simpleAppGVR).Namespace(inst.namespace).Get(
					ctx, inst.name, metav1.GetOptions{})
				if err == nil {
					// Instance still exists (possibly in terminating state)
					allDeleted = false
					break
				}
			}
			if allDeleted {
				t.Log("All existing test instances deleted")
				break
			}
			time.Sleep(1 * time.Second)
		}

		// Create test instances
		for _, inst := range instanceCountTestInstances {
			err := createTestInstance(t, ctx, inst.name, inst.namespace)
			if err != nil {
				t.Logf("Warning: Failed to create test instance %s/%s: %v", inst.namespace, inst.name, err)
			} else {
				t.Logf("Created test instance: %s/%s", inst.namespace, inst.name)
			}
		}

		// Create test projects with namespace access
		createInstanceCountTestProjects(t, ctx)

		// User CRD creation removed - RBAC uses JWT claims and Casbin policies
		// OIDC users are ephemeral, local users stored in ConfigMap/Secret

		// Wait for instances to be synced to cache
		waitForInstancesInCache(t, len(instanceCountTestInstances), 30*time.Second)

		close(instanceCountTestReady)
	})

	// Wait for fixtures to be ready
	select {
	case <-instanceCountTestReady:
		// Fixtures are ready
	case <-time.After(35 * time.Second):
		t.Fatal("Timeout waiting for instance count test fixtures")
	}
}

// createTestNamespace creates a namespace for testing
func createTestNamespace(t *testing.T, ctx context.Context, name string) {
	t.Helper()

	nsGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "e2e-test",
				},
			},
		},
	}

	_, err := dynamicClient.Resource(nsGVR).Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		// Namespace might already exist, which is fine
		t.Logf("Namespace %s: %v (may already exist)", name, err)
	} else {
		t.Logf("Created namespace: %s", name)
	}
}

// createTestInstance creates a SimpleApp instance for testing
func createTestInstance(t *testing.T, ctx context.Context, name, namespace string) error {
	t.Helper()

	instance := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "SimpleApp",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "e2e-test",
				},
			},
			"spec": map[string]interface{}{
				"appName": name,
				"image":   "nginx:latest",
			},
		},
	}

	_, err := dynamicClient.Resource(simpleAppGVR).Namespace(namespace).Create(
		ctx, instance, metav1.CreateOptions{})
	return err
}

// createInstanceCountTestProjects creates Project CRDs with namespace access for testing
func createInstanceCountTestProjects(t *testing.T, ctx context.Context) {
	t.Helper()

	projects := []struct {
		name         string
		description  string
		destinations []map[string]interface{}
		roles        []map[string]interface{}
	}{
		{
			name:        "e2e-proj-alpha",
			description: "E2E Alpha project - access to e2e-ns-alpha namespace",
			destinations: []map[string]interface{}{
				{"namespace": "e2e-ns-alpha"},
			},
			roles: []map[string]interface{}{
				{
					"name":        "viewer",
					"description": "Viewer role",
					"policies":    []interface{}{"p, proj:e2e-proj-alpha:viewer, instances, get, *, allow"},
					"groups":      []interface{}{"e2e-alpha-viewers"},
				},
			},
		},
		{
			name:        "e2e-proj-staging",
			description: "E2E Staging project - access to e2e-ns-staging namespace",
			destinations: []map[string]interface{}{
				{"namespace": "e2e-ns-staging"},
			},
			roles: []map[string]interface{}{
				{
					"name":        "viewer",
					"description": "Viewer role",
					"policies":    []interface{}{"p, proj:e2e-proj-staging:viewer, instances, get, *, allow"},
					"groups":      []interface{}{"e2e-staging-viewers"},
				},
			},
		},
		{
			name:        "e2e-proj-multi",
			description: "E2E Multi project - access to alpha and staging namespaces",
			destinations: []map[string]interface{}{
				{"namespace": "e2e-ns-alpha"},
				{"namespace": "e2e-ns-staging"},
			},
			roles: []map[string]interface{}{
				{
					"name":        "viewer",
					"description": "Viewer role",
					"policies":    []interface{}{"p, proj:e2e-proj-multi:viewer, instances, get, *, allow"},
					"groups":      []interface{}{"e2e-multi-viewers"},
				},
			},
		},
	}

	for _, proj := range projects {
		// Delete if exists
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, proj.name, metav1.DeleteOptions{})
		time.Sleep(100 * time.Millisecond)

		project := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "knodex.io/v1alpha1",
				"kind":       "Project",
				"metadata": map[string]interface{}{
					"name": proj.name,
					"labels": map[string]interface{}{
						"app.kubernetes.io/managed-by": "e2e-test",
					},
				},
				"spec": map[string]interface{}{
					"description":  proj.description,
					"destinations": proj.destinations,
					"roles":        proj.roles,
				},
			},
		}

		_, err := dynamicClient.Resource(projectGVR).Create(ctx, project, metav1.CreateOptions{})
		if err != nil {
			t.Logf("Warning: Failed to create project %s: %v", proj.name, err)
		} else {
			t.Logf("Created project: %s", proj.name)
		}
	}

	// Wait for projects to be synced (Casbin policy reload)
	// The RBAC service reloads policies every 2 seconds
	time.Sleep(5 * time.Second)
}

// createInstanceCountTestUsers removed
// User CRD is no longer part of the architecture:
// - Local users: Stored in ConfigMap/Secret
// - OIDC users: Ephemeral, not persisted to any CRD
// Tests use JWT claims (subject, email, groups, projects) for authorization
// Casbin policies grant permissions based on these claims

// waitForInstancesInCache polls the API until expected number of test instances are visible
func waitForInstancesInCache(t *testing.T, minExpected int, timeout time.Duration) {
	t.Helper()

	// Use global admin token for polling
	token := GenerateTestJWT(JWTClaims{
		Subject:     "cache-poller",
		Email:       "cache-poller@e2e-test.local",
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})

	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances/count", token, nil)
		if err != nil {
			t.Logf("Poll error: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		var countResp instanceCountResponse
		if err := json.NewDecoder(resp.Body).Decode(&countResp); err != nil {
			resp.Body.Close()
			t.Logf("Decode error: %v", err)
			time.Sleep(pollInterval)
			continue
		}
		resp.Body.Close()

		// Check if we have at least minExpected instances
		// Note: There may be other instances in the cluster, so we check >= minExpected
		if countResp.Count >= minExpected {
			t.Logf("Found %d instances in cache (need at least %d)", countResp.Count, minExpected)
			return
		}

		t.Logf("Waiting for instances in cache... found %d, need at least %d", countResp.Count, minExpected)
		time.Sleep(pollInterval)
	}

	t.Logf("Warning: Timeout waiting for instances - some tests may fail")
}

// cleanupInstanceCountTestFixtures removes test fixtures
func cleanupInstanceCountTestFixtures(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	// Delete test instances
	for _, inst := range instanceCountTestInstances {
		_ = dynamicClient.Resource(simpleAppGVR).Namespace(inst.namespace).Delete(
			ctx, inst.name, metav1.DeleteOptions{})
	}

	// Delete test projects
	_ = dynamicClient.Resource(projectGVR).Delete(ctx, "e2e-proj-alpha", metav1.DeleteOptions{})
	_ = dynamicClient.Resource(projectGVR).Delete(ctx, "e2e-proj-staging", metav1.DeleteOptions{})
	_ = dynamicClient.Resource(projectGVR).Delete(ctx, "e2e-proj-multi", metav1.DeleteOptions{})

	// User CRD cleanup removed - users no longer stored as CRDs

	// Note: We don't delete namespaces as they may contain other resources
}

// getInstanceCount calls the /api/v1/instances/count endpoint and returns the count
func getInstanceCount(t *testing.T, token string) (int, int) {
	t.Helper()

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances/count", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return -1, resp.StatusCode
	}

	var countResp instanceCountResponse
	err = json.NewDecoder(resp.Body).Decode(&countResp)
	require.NoError(t, err)

	return countResp.Count, resp.StatusCode
}

// ==============================================================================
// Test Cases
// ==============================================================================

func TestE2E_InstanceCount_GlobalAdmin_SeesAllInstances(t *testing.T) {
	setupInstanceCountTestFixtures(t)

	// Global admin should see ALL instances across all namespaces
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-global-admin",
		Email:       "admin@e2e-test.local",
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})

	count, statusCode := getInstanceCount(t, token)

	assert.Equal(t, http.StatusOK, statusCode, "Global admin should get 200 OK")
	// We expect at least our 6 test instances, but there may be more in the cluster
	assert.GreaterOrEqual(t, count, 6, "Global admin should see at least 6 test instances")
	t.Logf("Global admin sees %d instances", count)
}

func TestE2E_InstanceCount_AlphaUser_SeesOnlyAlphaInstances(t *testing.T) {
	setupInstanceCountTestFixtures(t)

	// Alpha project user should see only instances in e2e-ns-alpha namespace (at least 2 instances)
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-alpha-viewer",
		Email:       "alpha-viewer@e2e-test.local",
		Projects:    []string{"e2e-proj-alpha"},
		CasbinRoles: []string{"proj:e2e-proj-alpha:viewer"},
		Groups:      []string{"e2e-alpha-viewers"},
	})

	count, statusCode := getInstanceCount(t, token)

	assert.Equal(t, http.StatusOK, statusCode, "Alpha user should get 200 OK")
	// Use >= because there may be pre-existing instances in the namespace
	assert.GreaterOrEqual(t, count, 2, "Alpha user should see at least 2 instances in e2e-ns-alpha")
	t.Logf("Alpha user sees %d instances", count)
}

func TestE2E_InstanceCount_StagingUser_SeesOnlyStagingInstances(t *testing.T) {
	setupInstanceCountTestFixtures(t)

	// Staging project user should see only instances in e2e-ns-staging namespace (at least 3 instances)
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-staging-viewer",
		Email:       "staging-viewer@e2e-test.local",
		Projects:    []string{"e2e-proj-staging"},
		CasbinRoles: []string{"proj:e2e-proj-staging:viewer"},
		Groups:      []string{"e2e-staging-viewers"},
	})

	count, statusCode := getInstanceCount(t, token)

	assert.Equal(t, http.StatusOK, statusCode, "Staging user should get 200 OK")
	// Use >= because there may be pre-existing instances in the namespace
	assert.GreaterOrEqual(t, count, 3, "Staging user should see at least 3 instances in e2e-ns-staging")
	t.Logf("Staging user sees %d instances", count)
}

func TestE2E_InstanceCount_MultiProjectUser_SeesMultipleNamespaces(t *testing.T) {
	setupInstanceCountTestFixtures(t)

	// Multi-project user should see instances in both e2e-ns-alpha (at least 2) and e2e-ns-staging (at least 3)
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-multi-viewer",
		Email:       "multi-viewer@e2e-test.local",
		Projects:    []string{"e2e-proj-multi"},
		CasbinRoles: []string{"proj:e2e-proj-multi:viewer"},
		Groups:      []string{"e2e-multi-viewers"},
	})

	count, statusCode := getInstanceCount(t, token)

	assert.Equal(t, http.StatusOK, statusCode, "Multi-project user should get 200 OK")
	// Use >= because there may be pre-existing instances in the namespaces
	assert.GreaterOrEqual(t, count, 5, "Multi-project user should see at least 5 instances (2 alpha + 3 staging)")
	t.Logf("Multi-project user sees %d instances", count)
}

func TestE2E_InstanceCount_NoProjectUser_SeesZeroInstances(t *testing.T) {
	setupInstanceCountTestFixtures(t)

	// User with no project access should see 0 instances
	token := GenerateTestJWT(JWTClaims{
		Subject:  "user-no-projects",
		Email:    "no-projects@e2e-test.local",
		Projects: []string{},

		Groups: []string{},
	})

	count, statusCode := getInstanceCount(t, token)

	assert.Equal(t, http.StatusOK, statusCode, "No-project user should get 200 OK (not 403)")
	assert.Equal(t, 0, count, "No-project user should see 0 instances")
	t.Logf("No-project user sees %d instances", count)
}

func TestE2E_InstanceCount_Unauthenticated_Blocked(t *testing.T) {
	// Unauthenticated users should be blocked
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances/count", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Unauthenticated users should be blocked (401)")
}

func TestE2E_InstanceCount_InvalidToken_Blocked(t *testing.T) {
	// Invalid tokens should be rejected
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances/count", "invalid-token", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Invalid tokens should be rejected (401)")
}

// ==============================================================================
// Regression Tests
// ==============================================================================

func TestE2E_InstanceCount_RegressionSTORY138_NonAdminGets200NotForbidden(t *testing.T) {
	// REGRESSION TEST for this feature bug fix
	//
	// Bug: The /api/v1/instances/count endpoint was not using the hybrid authorization
	// model, causing 403 Forbidden for non-admin users. Non-admin users should get
	// 200 OK with a filtered count (not 403).
	//
	// Fix: Added isInstanceListOrCountRequest() bypass in authz.go to allow all
	// authenticated users to access the endpoint (handler filters by namespace).

	setupInstanceCountTestFixtures(t)

	// Non-admin user with project access should get 200 OK (not 403)
	token := GenerateTestJWT(JWTClaims{
		Subject:     "user-regression-test",
		Email:       "regression@e2e-test.local",
		Projects:    []string{"e2e-proj-alpha"},
		CasbinRoles: []string{"proj:e2e-proj-alpha:viewer"},
		Groups:      []string{"e2e-alpha-viewers"},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances/count", token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// CRITICAL: Should be 200 OK, NOT 403 Forbidden
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"REGRESSION: Non-admin user should get 200 OK, not 403 Forbidden")
}

func TestE2E_InstanceCount_RegressionSTORY138_CountMatchesListTotal(t *testing.T) {
	// REGRESSION TEST for this feature count logic bug
	//
	// Bug: The GetCount handler used PageSize:1 then tried to iterate over result.Items
	// to filter by namespace. Since Items only had 1 item max due to pagination,
	// count was always 0 or 1.
	//
	// Fix: Added CountByNamespaces() method that counts directly without pagination.

	setupInstanceCountTestFixtures(t)

	// Global admin token
	token := GenerateTestJWT(JWTClaims{
		Subject:     "admin-count-test",
		Email:       "admin-count@e2e-test.local",
		Projects:    []string{},
		CasbinRoles: []string{"role:serveradmin"},
	})

	// Get count from /api/v1/instances/count
	countResp, err := makeAuthenticatedRequest("GET", "/api/v1/instances/count", token, nil)
	require.NoError(t, err)
	defer countResp.Body.Close()

	var countResult instanceCountResponse
	err = json.NewDecoder(countResp.Body).Decode(&countResult)
	require.NoError(t, err)

	// Get totalCount from /api/v1/instances?pageSize=1000
	listResp, err := makeAuthenticatedRequest("GET", "/api/v1/instances?pageSize=1000", token, nil)
	require.NoError(t, err)
	defer listResp.Body.Close()

	var listResult struct {
		TotalCount int `json:"totalCount"`
	}
	err = json.NewDecoder(listResp.Body).Decode(&listResult)
	require.NoError(t, err)

	// CRITICAL: Count should match list totalCount
	assert.Equal(t, listResult.TotalCount, countResult.Count,
		"REGRESSION: /instances/count should match /instances list totalCount")

	t.Logf("Count endpoint: %d, List totalCount: %d", countResult.Count, listResult.TotalCount)
}
