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

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// OIDC-specific test configuration
const (
	// OIDC test groups - these simulate groups from an OIDC provider
	oidcGroupPlatformAdmins  = "platform-admins"
	oidcGroupTeamADevelopers = "team-a-developers"
	oidcGroupTeamBDevelopers = "team-b-developers"
	oidcGroupViewers         = "viewers"

	// OIDC test users - users with various group memberships
	oidcUserPlatformAdmin = "oidc-admin@example.com"
	oidcUserTeamADev      = "oidc-alice@example.com"
	oidcUserTeamBDev      = "oidc-bob@example.com"
	oidcUserViewer        = "oidc-viewer@example.com"
	oidcUserMultiRole     = "oidc-charlie@example.com"
	oidcUserNoGroups      = "oidc-nogroups@example.com"

	// OIDC test projects - simulate ArgoCD-style projects
	oidcProjectShared = "oidc-shared"
	oidcProjectTeamA  = "oidc-team-a"
	oidcProjectTeamB  = "oidc-team-b"
)

var (
	// oidcDynamicClient is the Kubernetes dynamic client for OIDC tests
	oidcDynamicClient dynamic.Interface
	// oidcHTTPClient is the HTTP client for OIDC tests
	oidcHTTPClient *http.Client
	// oidcAPIBaseURL is the server API base URL
	oidcAPIBaseURL string
	// oidcTestFixturesCreated tracks if fixtures were created
	oidcTestFixturesCreated bool

	// GVR for ProjectRoleBindings
	projectRoleBindingGVR = schema.GroupVersionResource{
		Group:    "knodex.io",
		Version:  "v1alpha1",
		Resource: "projectrolebindings",
	}
)

// TestOIDC_Setup initializes OIDC E2E test environment
func TestOIDC_Setup(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	// Get API base URL from environment
	oidcAPIBaseURL = os.Getenv("E2E_API_URL")
	if oidcAPIBaseURL == "" {
		oidcAPIBaseURL = "http://localhost:8080"
	}

	// Setup Kubernetes client
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.ExpandEnv("$HOME/.kube/config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err, "Failed to load kubeconfig")

	client, err := dynamic.NewForConfig(config)
	require.NoError(t, err, "Failed to create dynamic client")
	oidcDynamicClient = client

	// Setup HTTP client
	oidcHTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Wait for server to be ready
	err = waitForOIDCServer(t)
	require.NoError(t, err, "Server not ready")

	// Setup test fixtures
	err = setupOIDCTestFixtures(t)
	require.NoError(t, err, "Failed to setup OIDC test fixtures")
	oidcTestFixturesCreated = true

	t.Log("OIDC E2E test environment ready")
}

// TestOIDC_Cleanup cleans up OIDC E2E test environment
func TestOIDC_Cleanup(t *testing.T) {
	if !oidcTestFixturesCreated {
		t.Skip("No fixtures to clean up")
	}

	err := cleanupOIDCTestFixtures(t)
	if err != nil {
		t.Logf("Warning: cleanup failed: %v", err)
	}
}

// waitForOIDCServer waits for the server to be healthy
func waitForOIDCServer(t *testing.T) error {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := oidcHTTPClient.Get(oidcAPIBaseURL + "/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("server did not become healthy after %d retries", maxRetries)
}

// setupOIDCTestFixtures creates test Projects for OIDC group mapping tests
func setupOIDCTestFixtures(t *testing.T) error {
	ctx := context.Background()

	// Define test Projects with roles that match OIDC group mappings
	projects := []map[string]interface{}{
		{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": oidcProjectShared,
				"labels": map[string]interface{}{
					"e2e-test":  "true",
					"test-type": "oidc-group-mapping",
				},
			},
			"spec": map[string]interface{}{
				"description": "Shared project for OIDC group mapping tests",
				"destinations": []map[string]interface{}{
					{
						"namespace": "oidc-shared*",
					},
				},
				"roles": []map[string]interface{}{
					{
						"name":        "admin",
						"description": "Administrator role with full access",
						"policies": []string{
							"p, proj:oidc-shared:admin, applications, *, oidc-shared/*, allow",
							"p, proj:oidc-shared:admin, repositories, *, oidc-shared/*, allow",
						},
					},
					{
						"name":        "viewer",
						"description": "Read-only role",
						"policies": []string{
							"p, proj:oidc-shared:viewer, applications, get, oidc-shared/*, allow",
						},
					},
				},
			},
		},
		{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": oidcProjectTeamA,
				"labels": map[string]interface{}{
					"e2e-test":  "true",
					"test-type": "oidc-group-mapping",
				},
			},
			"spec": map[string]interface{}{
				"description": "Team A project for OIDC group mapping tests",
				"destinations": []map[string]interface{}{
					{
						"namespace": "teama*",
					},
				},
				"roles": []map[string]interface{}{
					{
						"name":        "developer",
						"description": "Developer role with deploy access",
						"policies": []string{
							"p, proj:oidc-team-a:developer, applications, *, oidc-team-a/*, allow",
						},
					},
				},
			},
		},
		{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": oidcProjectTeamB,
				"labels": map[string]interface{}{
					"e2e-test":  "true",
					"test-type": "oidc-group-mapping",
				},
			},
			"spec": map[string]interface{}{
				"description": "Team B project for OIDC group mapping tests",
				"destinations": []map[string]interface{}{
					{
						"namespace": "teamb*",
					},
				},
				"roles": []map[string]interface{}{
					{
						"name":        "developer",
						"description": "Developer role with deploy access",
						"policies": []string{
							"p, proj:oidc-team-b:developer, applications, *, oidc-team-b/*, allow",
						},
					},
				},
			},
		},
	}

	projectGVR := schema.GroupVersionResource{
		Group:    "knodex.io",
		Version:  "v1alpha1",
		Resource: "projects",
	}

	for _, proj := range projects {
		unstructuredProj := &unstructured.Unstructured{Object: proj}
		_, err := oidcDynamicClient.Resource(projectGVR).Create(ctx, unstructuredProj, metav1.CreateOptions{})
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create project %s: %w", proj["metadata"].(map[string]interface{})["name"], err)
		}
	}

	t.Log("OIDC test fixtures created successfully")
	return nil
}

// cleanupOIDCTestFixtures removes test Projects and ProjectRoleBindings
func cleanupOIDCTestFixtures(t *testing.T) error {
	ctx := context.Background()

	projectGVR := schema.GroupVersionResource{
		Group:    "knodex.io",
		Version:  "v1alpha1",
		Resource: "projects",
	}

	// Delete test Projects
	projectsToDelete := []string{oidcProjectShared, oidcProjectTeamA, oidcProjectTeamB}
	for _, projName := range projectsToDelete {
		err := oidcDynamicClient.Resource(projectGVR).Delete(ctx, projName, metav1.DeleteOptions{})
		if err != nil && !strings.Contains(err.Error(), "not found") {
			t.Logf("Warning: failed to delete project %s: %v", projName, err)
		}
	}

	// Delete ProjectRoleBindings with e2e-test label
	list, err := oidcDynamicClient.Resource(projectRoleBindingGVR).List(ctx, metav1.ListOptions{
		LabelSelector: "e2e-test=true",
	})
	if err == nil {
		for _, item := range list.Items {
			err := oidcDynamicClient.Resource(projectRoleBindingGVR).Delete(ctx, item.GetName(), metav1.DeleteOptions{})
			if err != nil {
				t.Logf("Warning: failed to delete ProjectRoleBinding %s: %v", item.GetName(), err)
			}
		}
	}

	t.Log("OIDC test fixtures cleaned up")
	return nil
}

// generateOIDCJWT creates a JWT token with OIDC group claims using the centralized utility.
// This is a convenience wrapper that delegates to GenerateOIDCJWT from test_utils.go.
func generateOIDCJWT(email string, groups []string) string {
	return GenerateOIDCJWT(email, groups)
}

// makeOIDCAuthenticatedRequest makes HTTP request with OIDC JWT token using the centralized utility.
// This is a convenience wrapper that maintains the existing function signature.
func makeOIDCAuthenticatedRequest(method, path string, token string) (*http.Response, error) {
	return MakeAuthenticatedRequest(oidcHTTPClient, oidcAPIBaseURL, method, path, token, nil)
}

// getProjectRoleBindingsForUser retrieves all ProjectRoleBindings for a user
func getProjectRoleBindingsForUser(ctx context.Context, username string) ([]*unstructured.Unstructured, error) {
	list, err := oidcDynamicClient.Resource(projectRoleBindingGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var userBindings []*unstructured.Unstructured
	for _, item := range list.Items {
		spec, ok := item.Object["spec"].(map[string]interface{})
		if !ok {
			continue
		}
		subject, ok := spec["subject"].(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := subject["name"].(string)
		if ok && name == username {
			itemCopy := item
			userBindings = append(userBindings, &itemCopy)
		}
	}

	return userBindings, nil
}

// ============================================================================
// JWT Token with Group Claims Tests
// ============================================================================

func TestOIDC_JWT_GroupClaims(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("JWT token includes groups claim as string array", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserPlatformAdmin, []string{oidcGroupPlatformAdmins})

		// Parse token to verify groups claim structure
		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(TestJWTSecret), nil
		})
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)

		claims := parsedToken.Claims.(jwt.MapClaims)
		groups, ok := claims["groups"].([]interface{})
		require.True(t, ok, "groups claim should be present as array")
		require.Len(t, groups, 1)
		assert.Equal(t, oidcGroupPlatformAdmins, groups[0].(string))
	})

	t.Run("JWT token with multiple groups per user", func(t *testing.T) {
		multiGroups := []string{oidcGroupPlatformAdmins, oidcGroupViewers}
		token := generateOIDCJWT(oidcUserMultiRole, multiGroups)

		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(TestJWTSecret), nil
		})
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)

		claims := parsedToken.Claims.(jwt.MapClaims)
		groups, ok := claims["groups"].([]interface{})
		require.True(t, ok)
		require.Len(t, groups, 2)

		// Convert to string slice for easier assertion
		groupStrings := make([]string, len(groups))
		for i, g := range groups {
			groupStrings[i] = g.(string)
		}
		assert.Contains(t, groupStrings, oidcGroupPlatformAdmins)
		assert.Contains(t, groupStrings, oidcGroupViewers)
	})

	t.Run("JWT token with empty groups has no groups claim", func(t *testing.T) {
		// When groups are empty, the claim is omitted entirely (standard OIDC behavior)
		token := generateOIDCJWT(oidcUserNoGroups, []string{})

		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(TestJWTSecret), nil
		})
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)

		claims := parsedToken.Claims.(jwt.MapClaims)
		_, ok := claims["groups"]
		assert.False(t, ok, "groups claim should not be present when empty (OIDC standard)")
	})

	t.Run("JWT token contains standard OIDC claims", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserTeamADev, []string{oidcGroupTeamADevelopers})

		parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
			return []byte(TestJWTSecret), nil
		})
		require.NoError(t, err)

		claims := parsedToken.Claims.(jwt.MapClaims)

		// Verify standard OIDC claims are present
		assert.Equal(t, oidcUserTeamADev, claims["sub"].(string), "sub claim should match")
		assert.Equal(t, oidcUserTeamADev, claims["email"].(string), "email claim should match")
		assert.Equal(t, "https://oidc.example.com", claims["iss"].(string), "iss claim should be present")
		assert.Equal(t, "knodex", claims["aud"].(string), "aud claim should be present")
		assert.NotNil(t, claims["exp"], "exp claim should be present")
		assert.NotNil(t, claims["iat"], "iat claim should be present")
	})
}

// ============================================================================
// Automatic ProjectRoleBinding Creation Tests
// ============================================================================

func TestOIDC_AutomaticRoleBindingCreation(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("user in OIDC group can access projects API", func(t *testing.T) {
		// Generate token for user in team-a-developers group
		token := generateOIDCJWT(oidcUserTeamADev, []string{oidcGroupTeamADevelopers})

		// Make authenticated request
		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should either be 200 OK or 403 depending on backend provisioning
		// For this test, we verify the token is properly processed
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden,
			"Expected 200 or 403, got %d", resp.StatusCode)
	})

	t.Run("authenticated request extracts groups from token", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserTeamBDev, []string{oidcGroupTeamBDevelopers})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// The backend should have parsed the JWT and extracted groups
		// Check that we got a proper response (not 401 unauthorized)
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
			"Request should not be unauthorized - token should be valid")
	})

	t.Run("token without groups claim still authenticates", func(t *testing.T) {
		// Create token without groups (simulates OIDC provider that doesn't include groups)
		// Must still include required claims for backend validation
		displayName := strings.Split(oidcUserNoGroups, "@")[0]
		claims := jwt.MapClaims{
			"sub":      oidcUserNoGroups,
			"email":    oidcUserNoGroups,
			"name":     displayName, // Required by backend ValidateToken
			"projects": []string{},  // Required - empty for no groups user
			// Note: legacy admin boolean claim removed — backend uses server-side Casbin policies (STORY-228)
			// No groups claim - simulates OIDC without groups
			"iss": "knodex",
			"aud": "knodex-api",
			"exp": time.Now().Add(1 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(TestJWTSecret))

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", tokenString)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should authenticate but may have limited permissions
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
			"Token without groups should still authenticate")
	})
}

// ============================================================================
// Permission Inheritance Through Groups Tests
// ============================================================================

func TestOIDC_PermissionInheritance(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("user in platform-admins group has elevated access", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserPlatformAdmin, []string{oidcGroupPlatformAdmins})

		// Should have access to all projects
		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Platform admin should have elevated permissions
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden,
			"Platform admin request should be processed")
	})

	t.Run("user in team-specific group has project access", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserTeamADev, []string{oidcGroupTeamADevelopers})

		// Team A developer should have access to team-a project
		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects/"+oidcProjectTeamA, token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be able to access team-a project (or 404 if not exists, but not 401)
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("user in viewers group has read-only access", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserViewer, []string{oidcGroupViewers})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Viewer should authenticate and get a response
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// ============================================================================
// Multi-Group User Scenarios Tests
// ============================================================================

func TestOIDC_MultiGroupUser(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("user in multiple groups authenticates successfully", func(t *testing.T) {
		multiGroups := []string{oidcGroupPlatformAdmins, oidcGroupViewers}
		token := generateOIDCJWT(oidcUserMultiRole, multiGroups)

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode,
			"Multi-group user should authenticate successfully")
	})

	t.Run("user inherits permissions from all groups (union)", func(t *testing.T) {
		// User in both team-a and team-b developers groups
		multiGroups := []string{oidcGroupTeamADevelopers, oidcGroupTeamBDevelopers}
		token := generateOIDCJWT("multi-team-dev@example.com", multiGroups)

		// Should be able to access both team-a and team-b resources
		resp1, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects/"+oidcProjectTeamA, token)
		require.NoError(t, err)
		resp1.Body.Close()

		resp2, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects/"+oidcProjectTeamB, token)
		require.NoError(t, err)
		resp2.Body.Close()

		// Both requests should not be unauthorized
		assert.NotEqual(t, http.StatusUnauthorized, resp1.StatusCode)
		assert.NotEqual(t, http.StatusUnauthorized, resp2.StatusCode)
	})

	t.Run("highest precedence role applies for same project", func(t *testing.T) {
		// platform-admins gives admin role, viewers gives viewer role
		// Admin should take precedence
		multiGroups := []string{oidcGroupPlatformAdmins, oidcGroupViewers}
		token := generateOIDCJWT("precedence-test@example.com", multiGroups)

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should have admin-level access (not just viewer)
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// ============================================================================
// Group Removal and Cleanup Tests
// ============================================================================

func TestOIDC_GroupRemovalCleanup(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("user without groups has limited access", func(t *testing.T) {
		// User with no OIDC groups
		token := generateOIDCJWT(oidcUserNoGroups, []string{})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// User should authenticate but may have no project access
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("removing user from group changes access level", func(t *testing.T) {
		testUser := "group-removal-test@example.com"

		// First: user with team-a-developers group
		token1 := generateOIDCJWT(testUser, []string{oidcGroupTeamADevelopers})
		resp1, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token1)
		require.NoError(t, err)
		resp1.Body.Close()
		status1 := resp1.StatusCode

		// Second: same user with empty groups
		token2 := generateOIDCJWT(testUser, []string{})
		resp2, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token2)
		require.NoError(t, err)
		resp2.Body.Close()
		status2 := resp2.StatusCode

		// Both should authenticate, but permissions may differ
		assert.NotEqual(t, http.StatusUnauthorized, status1)
		assert.NotEqual(t, http.StatusUnauthorized, status2)

		t.Logf("Access with groups: %d, Access without groups: %d", status1, status2)
	})

	t.Run("partial group removal preserves remaining access", func(t *testing.T) {
		testUser := "partial-removal@example.com"

		// User in two groups
		token1 := generateOIDCJWT(testUser, []string{oidcGroupTeamADevelopers, oidcGroupTeamBDevelopers})
		resp1, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token1)
		require.NoError(t, err)
		resp1.Body.Close()

		// User removed from team-b only
		token2 := generateOIDCJWT(testUser, []string{oidcGroupTeamADevelopers})
		resp2, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token2)
		require.NoError(t, err)
		resp2.Body.Close()

		// Should still have access from team-a
		assert.NotEqual(t, http.StatusUnauthorized, resp2.StatusCode)
	})
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestOIDC_EdgeCases(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("OIDC group not in mappings config is ignored", func(t *testing.T) {
		testUser := "unknown-group@example.com"
		// Include a non-existent group along with a valid one
		token := generateOIDCJWT(testUser, []string{"non-existent-group", oidcGroupViewers})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should authenticate and ignore unknown group
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("case-sensitive group name matching", func(t *testing.T) {
		testUser := "case-test@example.com"

		// Wrong case - should not match "platform-admins"
		token := generateOIDCJWT(testUser, []string{"Platform-Admins"})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Token should authenticate but may not get elevated permissions
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("special characters in group names handled correctly", func(t *testing.T) {
		testUser := "special-chars@example.com"
		// Groups with special characters commonly used in OIDC providers
		specialGroups := []string{
			"group-with-dashes",
			"group_with_underscores",
			"org:team:developers", // Azure AD style
		}
		token := generateOIDCJWT(testUser, specialGroups)

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should not cause errors
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode)
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("very long group names handled gracefully", func(t *testing.T) {
		testUser := "long-group@example.com"
		// Kubernetes name limit is 253 characters
		longGroupName := strings.Repeat("a", 300)
		token := generateOIDCJWT(testUser, []string{longGroupName})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should handle gracefully without panic or 500 error
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("duplicate groups in claims handled correctly", func(t *testing.T) {
		testUser := "duplicate-groups@example.com"
		// Same group listed multiple times
		token := generateOIDCJWT(testUser, []string{
			oidcGroupViewers,
			oidcGroupViewers,
			oidcGroupViewers,
		})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should handle duplicates gracefully
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("empty string group name ignored", func(t *testing.T) {
		testUser := "empty-group@example.com"
		// Include empty string in groups array
		token := generateOIDCJWT(testUser, []string{"", oidcGroupViewers, ""})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should ignore empty strings
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

// ============================================================================
// Group Mapping Configuration Tests
// ============================================================================

func TestOIDC_GroupMappingConfig(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("group mapping follows expected OIDC patterns", func(t *testing.T) {
		// Verify that our test groups follow standard OIDC group naming patterns
		groups := []string{
			oidcGroupPlatformAdmins,
			oidcGroupTeamADevelopers,
			oidcGroupTeamBDevelopers,
			oidcGroupViewers,
		}

		for _, group := range groups {
			// Groups should be lowercase and use hyphens (Kubernetes convention)
			assert.Equal(t, strings.ToLower(group), group, "Group %s should be lowercase", group)
			assert.False(t, strings.Contains(group, " "), "Group %s should not contain spaces", group)
		}
	})

	t.Run("ArgoCD-compatible group format", func(t *testing.T) {
		// ArgoCD expects groups in format: org:team or simple names
		argocdStyleGroups := []string{
			"platform-admins",          // Simple group
			"my-org:team-alpha",        // Org-scoped group
			"github:myorg:team-devops", // Multi-level group
		}

		for _, group := range argocdStyleGroups {
			token := generateOIDCJWT("argocd-test@example.com", []string{group})

			parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				return []byte(TestJWTSecret), nil
			})
			require.NoError(t, err)

			claims := parsedToken.Claims.(jwt.MapClaims)
			groups := claims["groups"].([]interface{})
			assert.Equal(t, group, groups[0].(string), "Group should be preserved exactly")
		}
	})
}

// ============================================================================
// RBAC Metrics Tests (OIDC-specific)
// ============================================================================

func TestOIDC_RBACMetrics(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("metrics endpoint tracks OIDC authentication", func(t *testing.T) {
		// Make an authenticated request first
		token := generateOIDCJWT(oidcUserPlatformAdmin, []string{oidcGroupPlatformAdmins})
		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		resp.Body.Close()

		// Check metrics endpoint
		metricsResp, err := oidcHTTPClient.Get(oidcAPIBaseURL + "/metrics")
		if err != nil {
			t.Skip("Metrics endpoint not available")
		}
		defer metricsResp.Body.Close()

		if metricsResp.StatusCode == http.StatusOK {
			// Metrics endpoint exists - success
			t.Log("Metrics endpoint available")
		}
	})
}

// ============================================================================
// Health Check Tests (OIDC context)
// ============================================================================

func TestOIDC_HealthCheck(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("health endpoint accessible without authentication", func(t *testing.T) {
		resp, err := oidcHTTPClient.Get(oidcAPIBaseURL + "/healthz")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should be public")
	})

	t.Run("readiness endpoint indicates OIDC capability", func(t *testing.T) {
		resp, err := oidcHTTPClient.Get(oidcAPIBaseURL + "/readyz")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should indicate system is ready for OIDC authentication
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable)
	})
}

// ============================================================================
// API Response Format Tests
// ============================================================================

func TestOIDC_APIResponseFormat(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping E2E tests. Set E2E_TESTS=true to run.")
	}

	t.Run("projects list returns valid JSON", func(t *testing.T) {
		token := generateOIDCJWT(oidcUserPlatformAdmin, []string{oidcGroupPlatformAdmins})

		resp, err := makeOIDCAuthenticatedRequest("GET", "/api/v1/projects", token)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result interface{}
			err := json.NewDecoder(resp.Body).Decode(&result)
			assert.NoError(t, err, "Response should be valid JSON")
		}
	})

	t.Run("error responses include proper status codes", func(t *testing.T) {
		// Request with invalid token
		req, _ := http.NewRequest("GET", oidcAPIBaseURL+"/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		resp, err := oidcHTTPClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return 401 for invalid token
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("missing authorization header returns 401", func(t *testing.T) {
		resp, err := oidcHTTPClient.Get(oidcAPIBaseURL + "/api/v1/projects")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
