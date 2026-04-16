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
// RBAC Namespace-Scoped Policy E2E Tests (STORY-437)
//
// Tests verify that roles[].destinations generates namespace-scoped Casbin
// policies. Different roles within the same project get access to different
// namespaces. This is the core RBAC evolution from STORY-437.
//
// Test Matrix:
//   - Platform Operator: deploy in rbac-platform + rbac-shared, denied elsewhere
//   - App Developer:     deploy in rbac-app, read secrets in rbac-shared, denied elsewhere
//   - QA Tester:         deploy in rbac-staging only
//   - Project Viewer:    read-only project-wide (no destinations = backward compat)
//   - Scoped Admin:      full access in rbac-app + rbac-staging only
//   - Cross-project:     all users denied in other projects
// ==============================================================================

const (
	// Project
	nsScopedProject = "e2e-ns-scoped-rbac"

	// Test namespaces (destinations)
	nsScopedNsPlatform = "e2e-ns-rbac-platform"
	nsScopedNsApp      = "e2e-ns-rbac-app"
	nsScopedNsShared   = "e2e-ns-rbac-shared"
	nsScopedNsStaging  = "e2e-ns-rbac-staging"

	// Test users
	nsScopedPlatformEng = "ns-scoped-platform-eng@e2e.local"
	nsScopedAppDev      = "ns-scoped-app-dev@e2e.local"
	nsScopedQATester    = "ns-scoped-qa-tester@e2e.local"
	nsScopedViewer      = "ns-scoped-viewer@e2e.local"
	nsScopedTeamLead    = "ns-scoped-team-lead@e2e.local"

	// Admin user (pre-configured in CASBIN_ADMIN_USERS)
	nsScopedAdminUser = "user-global-admin"
)

var nsScopedSetupOnce sync.Once
var nsScopedReady = make(chan struct{})
var nsScopedSetupFailed bool
var nsScopedK8sClient kubernetes.Interface

func initNsScopedK8sClient() error {
	if nsScopedK8sClient != nil {
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
	nsScopedK8sClient = client
	return nil
}

// getProjectNamespace returns the namespace where Project CRDs are stored.
// This matches the server's KNODEX_NAMESPACE / POD_NAMESPACE config.
func getProjectNamespace() string {
	if ns := os.Getenv("KNODEX_NAMESPACE"); ns != "" {
		return ns
	}
	if ns := os.Getenv("E2E_PROJECT_NAMESPACE"); ns != "" {
		return ns
	}
	return "knodex" // Default for QA deployments
}

// setupNsScopedFixtures creates the project, namespaces, and role assignments
// for namespace-scoped RBAC testing. Runs once via sync.Once.
func setupNsScopedFixtures(t *testing.T) {
	t.Helper()

	nsScopedSetupOnce.Do(func() {
		defer close(nsScopedReady)

		t.Log("Setting up namespace-scoped RBAC test fixtures (once)")
		ctx := context.Background()

		if err := initNsScopedK8sClient(); err != nil {
			t.Errorf("Failed to initialize K8s client: %v", err)
			nsScopedSetupFailed = true
			return
		}

		adminToken := nsScopedAdminToken()
		projectNS := getProjectNamespace()

		// Step 1: Create test namespaces
		for _, ns := range []string{nsScopedNsPlatform, nsScopedNsApp, nsScopedNsShared, nsScopedNsStaging} {
			nsObj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
					Labels: map[string]string{
						"e2e-test":           "true",
						"e2e-ns-scoped-rbac": "true",
					},
				},
			}
			_, err := nsScopedK8sClient.CoreV1().Namespaces().Create(ctx, nsObj, metav1.CreateOptions{})
			if err != nil {
				t.Logf("Namespace %s may already exist: %v", ns, err)
			} else {
				t.Logf("Created namespace: %s", ns)
			}
		}

		// Step 2: Create Project CRD with destination-scoped roles
		_ = dynamicClient.Resource(projectGVR).Namespace(projectNS).Delete(ctx, nsScopedProject, metav1.DeleteOptions{})
		time.Sleep(1 * time.Second)

		project := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "knodex.io/v1alpha1",
				"kind":       "Project",
				"metadata": map[string]interface{}{
					"name":      nsScopedProject,
					"namespace": projectNS,
					"labels": map[string]interface{}{
						"e2e-test":           "true",
						"e2e-ns-scoped-rbac": "true",
					},
				},
				"spec": map[string]interface{}{
					"description": "STORY-437 namespace-scoped RBAC E2E test project",
					"destinations": []interface{}{
						map[string]interface{}{"namespace": nsScopedNsPlatform},
						map[string]interface{}{"namespace": nsScopedNsApp},
						map[string]interface{}{"namespace": nsScopedNsShared},
						map[string]interface{}{"namespace": nsScopedNsStaging},
					},
					"roles": []interface{}{
						map[string]interface{}{
							"name":         "platform-operator",
							"description":  "Full access to platform and shared namespaces",
							"policies":     []interface{}{"instances/*, *, allow", "secrets/*, *, allow", "repositories/*, *, allow"},
							"destinations": []interface{}{nsScopedNsPlatform, nsScopedNsShared},
						},
						map[string]interface{}{
							"name":         "app-developer",
							"description":  "Full access to app namespace only",
							"policies":     []interface{}{"instances/*, *, allow", "secrets/*, *, allow"},
							"destinations": []interface{}{nsScopedNsApp},
						},
						map[string]interface{}{
							"name":         "shared-reader",
							"description":  "Read-only secrets in shared namespace",
							"policies":     []interface{}{"secrets/*, get, allow"},
							"destinations": []interface{}{nsScopedNsShared},
						},
						map[string]interface{}{
							"name":         "staging-deployer",
							"description":  "Create and read instances in staging",
							"policies":     []interface{}{"instances/*, create, allow", "instances/*, get, allow"},
							"destinations": []interface{}{nsScopedNsStaging},
						},
						map[string]interface{}{
							"name":        "project-viewer",
							"description": "Read-only across entire project (no destinations = project-wide)",
							"policies":    []interface{}{"instances/*, get, allow", "secrets/*, get, allow"},
							// No "destinations" — project-wide backward compatible
						},
						map[string]interface{}{
							"name":         "admin",
							"description":  "Full admin scoped to app and staging only",
							"policies":     []interface{}{"*, *, allow"},
							"destinations": []interface{}{nsScopedNsApp, nsScopedNsStaging},
						},
					},
				},
			},
		}

		_, err := dynamicClient.Resource(projectGVR).Namespace(projectNS).Create(ctx, project, metav1.CreateOptions{})
		if err != nil {
			t.Errorf("Failed to create project %s: %v", nsScopedProject, err)
			nsScopedSetupFailed = true
			return
		}
		t.Logf("Created Project CRD: %s in namespace %s", nsScopedProject, projectNS)

		// Wait for Casbin to sync
		t.Log("Waiting for Casbin policy sync...")
		time.Sleep(5 * time.Second)

		// Step 3: Assign users to roles
		roleAssignments := []struct {
			user string
			role string
		}{
			{nsScopedPlatformEng, "platform-operator"},
			{nsScopedAppDev, "app-developer"},
			{nsScopedAppDev, "shared-reader"}, // Same user, second role
			{nsScopedQATester, "staging-deployer"},
			{nsScopedViewer, "project-viewer"},
			{nsScopedTeamLead, "admin"},
		}

		for _, ra := range roleAssignments {
			path := fmt.Sprintf("/api/v1/projects/%s/roles/%s/users/%s", nsScopedProject, ra.role, ra.user)
			resp, err := makeAuthenticatedRequest("POST", path, adminToken, nil)
			if err != nil {
				t.Errorf("Failed to assign %s → %s: %v", ra.user, ra.role, err)
				nsScopedSetupFailed = true
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				t.Errorf("Unexpected status %d assigning %s → %s", resp.StatusCode, ra.user, ra.role)
				nsScopedSetupFailed = true
				return
			}
			t.Logf("Assigned %s → %s", ra.user, ra.role)
		}

		// Wait for role assignments to propagate
		time.Sleep(3 * time.Second)
	})

	// Wait for setup
	select {
	case <-nsScopedReady:
		if nsScopedSetupFailed {
			t.Skip("Skipping: namespace-scoped RBAC fixture setup failed")
		}
	case <-time.After(120 * time.Second):
		t.Fatal("Timeout waiting for namespace-scoped RBAC fixtures")
	}
}

// ==============================================================================
// Token Helpers
// ==============================================================================

func nsScopedAdminToken() string {
	return GenerateTestJWT(JWTClaims{
		Subject:     nsScopedAdminUser,
		Email:       nsScopedAdminUser + "@e2e.local",
		CasbinRoles: []string{"role:serveradmin"},
	})
}

func nsScopedUserToken(email string) string {
	return GenerateTestJWT(JWTClaims{
		Subject:  email,
		Email:    email,
		Projects: []string{nsScopedProject},
	})
}

// ==============================================================================
// Deploy Helper
// ==============================================================================

// nsScopedDeploy attempts to create an instance in the given namespace
// via the deployment validator. Returns the HTTP status code.
func nsScopedDeploy(t *testing.T, token, namespace string) int {
	t.Helper()
	body := map[string]interface{}{
		"name":      fmt.Sprintf("e2e-ns-test-%d", time.Now().UnixNano()),
		"namespace": namespace,
		"projectId": nsScopedProject,
		"rgdName":   "simple-webapp", // RGD may not exist — we test RBAC, not RGD
		"spec":      map[string]interface{}{},
	}
	// Use K8s-aligned route: POST /api/v1/namespaces/{ns}/instances/{kind}
	path := fmt.Sprintf("/api/v1/namespaces/%s/instances/SimpleWebApp", namespace)
	resp, err := makeAuthenticatedRequest("POST", path, token, body)
	require.NoError(t, err, "HTTP request should not fail")
	defer resp.Body.Close()
	return resp.StatusCode
}

// nsScopedDeployAllowed returns true if the deploy request was NOT blocked by RBAC.
// Any non-403 response (201, 404 for missing RGD, 409, 500) means RBAC passed.
func nsScopedDeployAllowed(status int) bool {
	return status != http.StatusForbidden
}

// ==============================================================================
// TEST 1: Platform Operator — destination-scoped to platform + shared
// ==============================================================================

func TestE2E_NsScoped_PlatformOperator_DeployToPlatform(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	status := nsScopedDeploy(t, token, nsScopedNsPlatform)
	assert.True(t, nsScopedDeployAllowed(status),
		"Platform operator should deploy to rbac-platform (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_PlatformOperator_DeployToShared(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	status := nsScopedDeploy(t, token, nsScopedNsShared)
	assert.True(t, nsScopedDeployAllowed(status),
		"Platform operator should deploy to rbac-shared (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_PlatformOperator_DeniedApp(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	status := nsScopedDeploy(t, token, nsScopedNsApp)
	assert.Equal(t, http.StatusForbidden, status,
		"Platform operator should be DENIED in rbac-app (not in destinations)")
}

func TestE2E_NsScoped_PlatformOperator_DeniedStaging(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	status := nsScopedDeploy(t, token, nsScopedNsStaging)
	assert.Equal(t, http.StatusForbidden, status,
		"Platform operator should be DENIED in rbac-staging (not in destinations)")
}

// ==============================================================================
// TEST 2: App Developer — destination-scoped to app only (+ shared reader)
// ==============================================================================

func TestE2E_NsScoped_AppDeveloper_DeployToApp(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedDeploy(t, token, nsScopedNsApp)
	assert.True(t, nsScopedDeployAllowed(status),
		"App developer should deploy to rbac-app (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_AppDeveloper_DeniedSharedCreate(t *testing.T) {
	setupNsScopedFixtures(t)
	// App developer has shared-reader role (get only) in rbac-shared.
	// Deploy (create) should be denied.
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedDeploy(t, token, nsScopedNsShared)
	assert.Equal(t, http.StatusForbidden, status,
		"App developer should be DENIED create in rbac-shared (shared-reader has get only)")
}

func TestE2E_NsScoped_AppDeveloper_DeniedPlatform(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedDeploy(t, token, nsScopedNsPlatform)
	assert.Equal(t, http.StatusForbidden, status,
		"App developer should be DENIED in rbac-platform (not in destinations)")
}

func TestE2E_NsScoped_AppDeveloper_DeniedStaging(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedDeploy(t, token, nsScopedNsStaging)
	assert.Equal(t, http.StatusForbidden, status,
		"App developer should be DENIED in rbac-staging (not in destinations)")
}

// ==============================================================================
// TEST 3: QA Tester — staging deployer, destination-scoped to staging only
// ==============================================================================

func TestE2E_NsScoped_QATester_DeployToStaging(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedQATester)
	status := nsScopedDeploy(t, token, nsScopedNsStaging)
	assert.True(t, nsScopedDeployAllowed(status),
		"QA tester should deploy to rbac-staging (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_QATester_DeniedApp(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedQATester)
	status := nsScopedDeploy(t, token, nsScopedNsApp)
	assert.Equal(t, http.StatusForbidden, status,
		"QA tester should be DENIED in rbac-app (not in destinations)")
}

func TestE2E_NsScoped_QATester_DeniedPlatform(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedQATester)
	status := nsScopedDeploy(t, token, nsScopedNsPlatform)
	assert.Equal(t, http.StatusForbidden, status,
		"QA tester should be DENIED in rbac-platform (not in destinations)")
}

func TestE2E_NsScoped_QATester_DeniedShared(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedQATester)
	status := nsScopedDeploy(t, token, nsScopedNsShared)
	assert.Equal(t, http.StatusForbidden, status,
		"QA tester should be DENIED in rbac-shared (not in destinations)")
}

// ==============================================================================
// TEST 4: Project Viewer — NO destinations = project-wide read-only
// ==============================================================================

func TestE2E_NsScoped_Viewer_DeniedDeployAnywhere(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedViewer)

	// Viewer has get-only policy, no create. Should be denied in all namespaces.
	for _, ns := range []string{nsScopedNsPlatform, nsScopedNsApp, nsScopedNsShared, nsScopedNsStaging} {
		status := nsScopedDeploy(t, token, ns)
		assert.Equal(t, http.StatusForbidden, status,
			"Viewer should be DENIED deploy in %s (read-only role)", ns)
	}
}

func TestE2E_NsScoped_Viewer_CanReadProjectWide(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedViewer)

	// Viewer should be able to GET the project (project-wide access)
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+nsScopedProject, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"Viewer should read project (project-wide role, no destinations)")
}

// ==============================================================================
// TEST 5: Scoped Admin — destination-scoped to app + staging
// ==============================================================================

func TestE2E_NsScoped_ScopedAdmin_DeployToApp(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedDeploy(t, token, nsScopedNsApp)
	assert.True(t, nsScopedDeployAllowed(status),
		"Scoped admin should deploy to rbac-app (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_ScopedAdmin_DeployToStaging(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedDeploy(t, token, nsScopedNsStaging)
	assert.True(t, nsScopedDeployAllowed(status),
		"Scoped admin should deploy to rbac-staging (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_ScopedAdmin_DeniedPlatform(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedDeploy(t, token, nsScopedNsPlatform)
	assert.Equal(t, http.StatusForbidden, status,
		"Scoped admin should be DENIED in rbac-platform (not in destinations)")
}

func TestE2E_NsScoped_ScopedAdmin_DeniedShared(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedDeploy(t, token, nsScopedNsShared)
	assert.Equal(t, http.StatusForbidden, status,
		"Scoped admin should be DENIED in rbac-shared (not in destinations)")
}

// ==============================================================================
// TEST 6: Cross-project isolation
// ==============================================================================

func TestE2E_NsScoped_CrossProject_AllUsersDenied(t *testing.T) {
	setupNsScopedFixtures(t)

	// All namespace-scoped users should be denied access to the default project
	users := []struct {
		name  string
		email string
	}{
		{"platform-eng", nsScopedPlatformEng},
		{"app-dev", nsScopedAppDev},
		{"qa-tester", nsScopedQATester},
		{"viewer", nsScopedViewer},
		{"team-lead", nsScopedTeamLead},
	}

	for _, u := range users {
		token := nsScopedUserToken(u.email)
		// Try to access a different project
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/default-project", token, nil)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"%s should be DENIED access to default-project (cross-project isolation)", u.name)
	}
}

// ==============================================================================
// TEST 7: Global admin bypasses all namespace scoping
// ==============================================================================

func TestE2E_NsScoped_GlobalAdmin_DeployAnywhere(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedAdminToken()

	for _, ns := range []string{nsScopedNsPlatform, nsScopedNsApp, nsScopedNsShared, nsScopedNsStaging} {
		status := nsScopedDeploy(t, token, ns)
		assert.True(t, nsScopedDeployAllowed(status),
			"Global admin should deploy to %s (bypasses all RBAC), got HTTP %d", ns, status)
	}
}

// ==============================================================================
// TEST 8: Project structure validation
// ==============================================================================

func TestE2E_NsScoped_ProjectCreatedWithRoleDestinations(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedAdminToken()

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+nsScopedProject, token, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var project struct {
		Name         string `json:"name"`
		Destinations []struct {
			Namespace string `json:"namespace"`
		} `json:"destinations"`
		Roles []struct {
			Name         string   `json:"name"`
			Destinations []string `json:"destinations"`
			Policies     []string `json:"policies"`
		} `json:"roles"`
	}
	err = json.NewDecoder(resp.Body).Decode(&project)
	require.NoError(t, err)

	assert.Equal(t, nsScopedProject, project.Name)
	assert.Len(t, project.Destinations, 4, "Project should have 4 destinations")
	assert.Len(t, project.Roles, 6, "Project should have 6 roles")

	// Verify role destinations are persisted
	rolesByName := make(map[string][]string)
	for _, r := range project.Roles {
		rolesByName[r.Name] = r.Destinations
	}

	assert.ElementsMatch(t, []string{nsScopedNsPlatform, nsScopedNsShared}, rolesByName["platform-operator"],
		"platform-operator should have platform + shared destinations")
	assert.ElementsMatch(t, []string{nsScopedNsApp}, rolesByName["app-developer"],
		"app-developer should have app destination only")
	assert.ElementsMatch(t, []string{nsScopedNsShared}, rolesByName["shared-reader"],
		"shared-reader should have shared destination only")
	assert.ElementsMatch(t, []string{nsScopedNsStaging}, rolesByName["staging-deployer"],
		"staging-deployer should have staging destination only")
	assert.Empty(t, rolesByName["project-viewer"],
		"project-viewer should have no destinations (project-wide)")
	assert.ElementsMatch(t, []string{nsScopedNsApp, nsScopedNsStaging}, rolesByName["admin"],
		"admin should have app + staging destinations")
}

// ==============================================================================
// TEST 9: Invalid role destinations rejected at API level
// ==============================================================================

func TestE2E_NsScoped_InvalidRoleDestination_Rejected(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedAdminToken()

	// Try to create a project where a role references a namespace not in destinations
	body := map[string]interface{}{
		"name":        "e2e-ns-invalid-dest",
		"description": "Should fail validation",
		"destinations": []map[string]interface{}{
			{"namespace": "ns-valid"},
		},
		"roles": []map[string]interface{}{
			{
				"name":         "bad-role",
				"policies":     []string{"instances/*, *, allow"},
				"destinations": []string{"ns-does-not-exist"}, // Not in project destinations
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects", token, body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"Project creation with invalid role destination should be rejected")
}

// ==============================================================================
// TEST 10: Duplicate role destinations rejected
// ==============================================================================

func TestE2E_NsScoped_DuplicateRoleDestination_Rejected(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedAdminToken()

	body := map[string]interface{}{
		"name":        "e2e-ns-dup-dest",
		"description": "Should fail validation - duplicate destinations",
		"destinations": []map[string]interface{}{
			{"namespace": "ns-one"},
			{"namespace": "ns-two"},
		},
		"roles": []map[string]interface{}{
			{
				"name":         "dup-role",
				"policies":     []string{"instances/*, *, allow"},
				"destinations": []string{"ns-one", "ns-one"}, // Duplicate
			},
		},
	}

	resp, err := makeAuthenticatedRequest("POST", "/api/v1/projects", token, body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
		"Project creation with duplicate role destinations should be rejected")
}

// ==============================================================================
// TEST 11: Backward compatibility — roles without destinations get project-wide
// ==============================================================================

func TestE2E_NsScoped_BackwardCompat_NoDestinations_ProjectWide(t *testing.T) {
	setupNsScopedFixtures(t)

	ctx := context.Background()
	projectNS := getProjectNamespace()
	projectName := "e2e-ns-backward-compat"

	// Create project where role has NO destinations
	_ = dynamicClient.Resource(projectGVR).Namespace(projectNS).Delete(ctx, projectName, metav1.DeleteOptions{})
	time.Sleep(500 * time.Millisecond)

	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":      projectName,
				"namespace": projectNS,
				"labels": map[string]interface{}{
					"e2e-test":           "true",
					"e2e-ns-scoped-rbac": "true",
				},
			},
			"spec": map[string]interface{}{
				"description": "Backward compat test — no destinations on role",
				"destinations": []interface{}{
					map[string]interface{}{"namespace": nsScopedNsApp},
					map[string]interface{}{"namespace": nsScopedNsStaging},
				},
				"roles": []interface{}{
					map[string]interface{}{
						"name":     "full-access",
						"policies": []interface{}{"instances/*, *, allow"},
						// No "destinations" — should get project-wide policies
					},
				},
			},
		},
	}

	_, err := dynamicClient.Resource(projectGVR).Namespace(projectNS).Create(ctx, project, metav1.CreateOptions{})
	require.NoError(t, err)
	defer func() {
		_ = dynamicClient.Resource(projectGVR).Namespace(projectNS).Delete(ctx, projectName, metav1.DeleteOptions{})
	}()

	time.Sleep(5 * time.Second)

	// Assign user to the role
	adminToken := nsScopedAdminToken()
	userEmail := "ns-compat-user@e2e.local"
	resp, err := makeAuthenticatedRequest("POST",
		fmt.Sprintf("/api/v1/projects/%s/roles/full-access/users/%s", projectName, userEmail),
		adminToken, nil)
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated)

	time.Sleep(3 * time.Second)

	// User should be able to deploy to ALL project destinations
	userToken := GenerateTestJWT(JWTClaims{
		Subject:  userEmail,
		Email:    userEmail,
		Projects: []string{projectName},
	})

	for _, ns := range []string{nsScopedNsApp, nsScopedNsStaging} {
		body := map[string]interface{}{
			"name":      fmt.Sprintf("e2e-compat-%d", time.Now().UnixNano()),
			"namespace": ns,
			"projectId": projectName,
			"rgdName":   "test-rgd",
			"spec":      map[string]interface{}{},
		}
		path := fmt.Sprintf("/api/v1/namespaces/%s/instances/TestKind", ns)
		resp, err := makeAuthenticatedRequest("POST", path, userToken, body)
		require.NoError(t, err)
		resp.Body.Close()

		assert.True(t, nsScopedDeployAllowed(resp.StatusCode),
			"Role without destinations should have project-wide access in %s, got HTTP %d", ns, resp.StatusCode)
	}
}

// ==============================================================================
// TEST 12: Instance GET — namespace-scoped authz via CasbinAuthz middleware
// ==============================================================================

// nsScopedInstanceGet attempts to GET an instance in the given namespace.
// Returns HTTP status code. Uses the CasbinAuthz middleware path which
// normalizes /namespaces/{ns}/instances/{kind}/{name} → instances/{ns}/{kind}/{name}
// and then resolveNamespaceToProjectObject converts to instances/{project}/{ns}/{kind}/{name}.
func nsScopedInstanceGet(t *testing.T, token, namespace, kind, name string) int {
	t.Helper()
	path := fmt.Sprintf("/api/v1/namespaces/%s/instances/%s/%s", namespace, kind, name)
	resp, err := makeAuthenticatedRequest("GET", path, token, nil)
	require.NoError(t, err, "HTTP request should not fail")
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestE2E_NsScoped_InstanceGet_PlatformOperator_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	// GET instance in rbac-platform — should pass RBAC (may 404 if instance doesn't exist)
	status := nsScopedInstanceGet(t, token, nsScopedNsPlatform, "WebApp", "nonexistent")
	assert.NotEqual(t, http.StatusForbidden, status,
		"Platform operator should pass RBAC for GET in rbac-platform (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_InstanceGet_PlatformOperator_DeniedOutsideDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	// GET instance in rbac-app — should be denied by RBAC
	status := nsScopedInstanceGet(t, token, nsScopedNsApp, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"Platform operator should be DENIED GET in rbac-app (not in destinations)")
}

func TestE2E_NsScoped_InstanceGet_AppDeveloper_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedInstanceGet(t, token, nsScopedNsApp, "WebApp", "nonexistent")
	assert.NotEqual(t, http.StatusForbidden, status,
		"App developer should pass RBAC for GET in rbac-app (in destinations), got HTTP %d", status)
}

func TestE2E_NsScoped_InstanceGet_AppDeveloper_DeniedInPlatform(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedInstanceGet(t, token, nsScopedNsPlatform, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"App developer should be DENIED GET in rbac-platform (not in destinations)")
}

func TestE2E_NsScoped_InstanceGet_Viewer_AllowedProjectWide(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedViewer)
	// Viewer has project-wide GET policy (no destinations) — should pass in ALL namespaces
	for _, ns := range []string{nsScopedNsPlatform, nsScopedNsApp, nsScopedNsShared, nsScopedNsStaging} {
		status := nsScopedInstanceGet(t, token, ns, "WebApp", "nonexistent")
		assert.NotEqual(t, http.StatusForbidden, status,
			"Viewer should pass RBAC for GET in %s (project-wide), got HTTP %d", ns, status)
	}
}

// ==============================================================================
// TEST 13: Instance UPDATE (PATCH) — namespace-scoped authz
// ==============================================================================

func nsScopedInstanceUpdate(t *testing.T, token, namespace, kind, name string) int {
	t.Helper()
	path := fmt.Sprintf("/api/v1/namespaces/%s/instances/%s/%s", namespace, kind, name)
	body := map[string]interface{}{
		"spec": map[string]interface{}{"replicas": 2},
	}
	resp, err := makeAuthenticatedRequest("PATCH", path, token, body)
	require.NoError(t, err, "HTTP request should not fail")
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestE2E_NsScoped_InstanceUpdate_PlatformOperator_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	status := nsScopedInstanceUpdate(t, token, nsScopedNsPlatform, "WebApp", "nonexistent")
	assert.NotEqual(t, http.StatusForbidden, status,
		"Platform operator should pass RBAC for UPDATE in rbac-platform, got HTTP %d", status)
}

func TestE2E_NsScoped_InstanceUpdate_PlatformOperator_DeniedOutsideDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)
	status := nsScopedInstanceUpdate(t, token, nsScopedNsApp, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"Platform operator should be DENIED UPDATE in rbac-app (not in destinations)")
}

func TestE2E_NsScoped_InstanceUpdate_Viewer_DeniedEverywhere(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedViewer)
	// Viewer has GET only — UPDATE should be denied even in accessible namespaces
	status := nsScopedInstanceUpdate(t, token, nsScopedNsApp, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"Viewer should be DENIED UPDATE (read-only role)")
}

func TestE2E_NsScoped_InstanceUpdate_ScopedAdmin_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedInstanceUpdate(t, token, nsScopedNsApp, "WebApp", "nonexistent")
	assert.NotEqual(t, http.StatusForbidden, status,
		"Scoped admin should pass RBAC for UPDATE in rbac-app, got HTTP %d", status)
}

func TestE2E_NsScoped_InstanceUpdate_ScopedAdmin_DeniedOutsideDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedInstanceUpdate(t, token, nsScopedNsPlatform, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"Scoped admin should be DENIED UPDATE in rbac-platform (not in destinations)")
}

// ==============================================================================
// TEST 14: Instance DELETE — namespace-scoped authz
// ==============================================================================

func nsScopedInstanceDelete(t *testing.T, token, namespace, kind, name string) int {
	t.Helper()
	path := fmt.Sprintf("/api/v1/namespaces/%s/instances/%s/%s", namespace, kind, name)
	resp, err := makeAuthenticatedRequest("DELETE", path, token, nil)
	require.NoError(t, err, "HTTP request should not fail")
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestE2E_NsScoped_InstanceDelete_AppDeveloper_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedInstanceDelete(t, token, nsScopedNsApp, "WebApp", "nonexistent")
	assert.NotEqual(t, http.StatusForbidden, status,
		"App developer should pass RBAC for DELETE in rbac-app, got HTTP %d", status)
}

func TestE2E_NsScoped_InstanceDelete_AppDeveloper_DeniedInPlatform(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedAppDev)
	status := nsScopedInstanceDelete(t, token, nsScopedNsPlatform, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"App developer should be DENIED DELETE in rbac-platform")
}

func TestE2E_NsScoped_InstanceDelete_QATester_DeniedEvenInStaging(t *testing.T) {
	setupNsScopedFixtures(t)
	// QA tester has create+get only in staging — DELETE should be denied
	token := nsScopedUserToken(nsScopedQATester)
	status := nsScopedInstanceDelete(t, token, nsScopedNsStaging, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"QA tester should be DENIED DELETE in rbac-staging (has create+get only, not delete)")
}

func TestE2E_NsScoped_InstanceDelete_ScopedAdmin_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedInstanceDelete(t, token, nsScopedNsStaging, "WebApp", "nonexistent")
	assert.NotEqual(t, http.StatusForbidden, status,
		"Scoped admin should pass RBAC for DELETE in rbac-staging, got HTTP %d", status)
}

func TestE2E_NsScoped_InstanceDelete_ScopedAdmin_DeniedOutsideDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedTeamLead)
	status := nsScopedInstanceDelete(t, token, nsScopedNsShared, "WebApp", "nonexistent")
	assert.Equal(t, http.StatusForbidden, status,
		"Scoped admin should be DENIED DELETE in rbac-shared (not in destinations)")
}

// ==============================================================================
// TEST 15: Instance LIST — hybrid authorization with getProjectScopedWildcards
// ==============================================================================

func TestE2E_NsScoped_InstanceList_AllRolesCanList(t *testing.T) {
	setupNsScopedFixtures(t)
	// All users with project roles should be able to list instances
	// (the handler filters results by namespace; RBAC allows the list operation)
	users := []struct {
		name  string
		email string
	}{
		{"platform-eng", nsScopedPlatformEng},
		{"app-dev", nsScopedAppDev},
		{"qa-tester", nsScopedQATester},
		{"viewer", nsScopedViewer},
		{"team-lead", nsScopedTeamLead},
	}

	for _, u := range users {
		token := nsScopedUserToken(u.email)
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/instances", token, nil)
		require.NoError(t, err)
		resp.Body.Close()

		// Instance list uses hybrid authorization — should return 200
		// (handler filters by accessible namespaces, not middleware-blocked)
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"%s should be able to list instances (hybrid auth model), got HTTP %d", u.name, resp.StatusCode)
	}
}

// ==============================================================================
// TEST 16: Instance sub-resources (graph, children, events) — namespace-scoped
// ==============================================================================

func TestE2E_NsScoped_InstanceSubresource_PlatformOp_AllowedInDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)

	subresources := []string{"graph", "children", "events"}
	for _, sub := range subresources {
		path := fmt.Sprintf("/api/v1/namespaces/%s/instances/WebApp/test-app/%s", nsScopedNsPlatform, sub)
		resp, err := makeAuthenticatedRequest("GET", path, token, nil)
		require.NoError(t, err)
		resp.Body.Close()

		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode,
			"Platform operator should pass RBAC for %s in rbac-platform, got HTTP %d", sub, resp.StatusCode)
	}
}

func TestE2E_NsScoped_InstanceSubresource_PlatformOp_DeniedOutsideDestination(t *testing.T) {
	setupNsScopedFixtures(t)
	token := nsScopedUserToken(nsScopedPlatformEng)

	subresources := []string{"graph", "children", "events"}
	for _, sub := range subresources {
		path := fmt.Sprintf("/api/v1/namespaces/%s/instances/WebApp/test-app/%s", nsScopedNsApp, sub)
		resp, err := makeAuthenticatedRequest("GET", path, token, nil)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"Platform operator should be DENIED %s in rbac-app (not in destinations)", sub)
	}
}
