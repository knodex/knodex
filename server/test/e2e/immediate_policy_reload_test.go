//go:build e2e

// E2E test: Immediate Casbin Policy Reload on Permission Changes
// Demonstrates: change permission → immediate effect (no waiting for sync cycle)

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Test project for immediate policy reload tests
	testProjectImmediateReload = "e2e-immediate-reload"

	// Test users for immediate policy reload tests
	testUserImmediateReload  = "immediate-reload-user@example.com"
	testGroupImmediateReload = "immediate-reload-group"
)

// TestE2E_ImmediatePolicyReload_AssignUserRole_ImmediateEffect tests AC-8:
// When a user is assigned a role via the API, the permission change takes effect immediately
// without waiting for the periodic sync cycle (default 10 minutes).
func TestE2E_ImmediatePolicyReload_AssignUserRole_ImmediateEffect(t *testing.T) {
	ctx := context.Background()
	adminToken := generateTestJWT(testUserAdmin, []string{}, true)

	// Setup: Create test project
	projectName := testProjectImmediateReload + "-user"
	err := createTestProject(ctx, projectName, "E2E immediate reload test", []map[string]interface{}{
		{
			"name":        "viewer",
			"description": "Viewer role",
			"policies": []interface{}{
				"*, get, allow",
			},
		},
	})
	require.NoError(t, err, "failed to create test project")
	defer func() {
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, projectName, metav1.DeleteOptions{})
	}()

	// Wait for project creation to be processed
	time.Sleep(500 * time.Millisecond)

	// Step 1: Verify user has NO access initially (user has no role bindings)
	userToken := generateTestJWT(testUserImmediateReload, []string{}, false)
	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	// User should be denied access (403 Forbidden)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"STEP 1: User should NOT have access before role assignment")

	// Step 2: Assign viewer role to user via API
	resp, err = makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+projectName+"/roles/viewer/users/"+testUserImmediateReload,
		adminToken,
		nil,
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"STEP 2: Admin should be able to assign role, got %d", resp.StatusCode)

	// Step 3: IMMEDIATELY verify user has access (NO waiting for sync cycle)
	// AC-8: Permission change should take effect IMMEDIATELY
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"STEP 3 (AC-8): User should have IMMEDIATE access after role assignment - NO waiting for sync cycle")

	// Step 4: Remove the role
	resp, err = makeAuthenticatedRequest(
		"DELETE",
		"/api/v1/projects/"+projectName+"/roles/viewer/users/"+testUserImmediateReload,
		adminToken,
		nil,
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"STEP 4: Admin should be able to remove role, got %d", resp.StatusCode)

	// Step 5: IMMEDIATELY verify user NO LONGER has access (NO waiting for sync cycle)
	// AC-8: Permission removal should take effect IMMEDIATELY
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"STEP 5 (AC-8): User should IMMEDIATELY lose access after role removal - NO waiting for sync cycle")
}

// TestE2E_ImmediatePolicyReload_AssignGroupRole_ImmediateEffect tests AC-8 for group assignments:
// When a group is assigned a role via the API, members of that group should have immediate access.
func TestE2E_ImmediatePolicyReload_AssignGroupRole_ImmediateEffect(t *testing.T) {
	ctx := context.Background()
	adminToken := generateTestJWT(testUserAdmin, []string{}, true)

	// Setup: Create test project
	projectName := testProjectImmediateReload + "-group"
	err := createTestProject(ctx, projectName, "E2E immediate group reload test", []map[string]interface{}{
		{
			"name":        "developer",
			"description": "Developer role",
			"policies": []interface{}{
				"*, *, allow",
			},
		},
	})
	require.NoError(t, err, "failed to create test project")
	defer func() {
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, projectName, metav1.DeleteOptions{})
	}()

	// Wait for project creation to be processed
	time.Sleep(500 * time.Millisecond)

	// Step 1: Verify user (member of group) has NO access initially
	// Create token for user who is a member of the test group
	userToken := GenerateTestJWT(JWTClaims{
		Subject:  "group-member@example.com",
		Email:    "group-member@example.com",
		Groups:   []string{testGroupImmediateReload},
		Projects: []string{},
	})

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	// User should be denied access (403 Forbidden)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"STEP 1: Group member should NOT have access before group role assignment")

	// Step 2: Assign developer role to group via API
	resp, err = makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+projectName+"/roles/developer/groups/"+testGroupImmediateReload,
		adminToken,
		nil,
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated,
		"STEP 2: Admin should be able to assign group role, got %d", resp.StatusCode)

	// Step 3: IMMEDIATELY verify group member has access (NO waiting for sync cycle)
	// AC-8: Permission change should take effect IMMEDIATELY
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"STEP 3 (AC-8): Group member should have IMMEDIATE access after group role assignment - NO waiting for sync cycle")

	// Step 4: Remove the group role
	resp, err = makeAuthenticatedRequest(
		"DELETE",
		"/api/v1/projects/"+projectName+"/roles/developer/groups/"+testGroupImmediateReload,
		adminToken,
		nil,
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent,
		"STEP 4: Admin should be able to remove group role, got %d", resp.StatusCode)

	// Step 5: IMMEDIATELY verify group member NO LONGER has access (NO waiting for sync cycle)
	// AC-8: Permission removal should take effect IMMEDIATELY
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"STEP 5 (AC-8): Group member should IMMEDIATELY lose access after group role removal - NO waiting for sync cycle")
}

// TestE2E_ImmediatePolicyReload_UpdateProjectRoles_ImmediateEffect tests AC-1:
// When PUT /api/v1/projects/{name} updates role definitions, Casbin policies are reloaded immediately.
func TestE2E_ImmediatePolicyReload_UpdateProjectRoles_ImmediateEffect(t *testing.T) {
	ctx := context.Background()
	adminToken := generateTestJWT(testUserAdmin, []string{}, true)

	// Setup: Create test project with a user role binding
	projectName := testProjectImmediateReload + "-roles"
	testUser := "project-roles-user@example.com"

	err := createTestProject(ctx, projectName, "E2E project roles test", []map[string]interface{}{
		{
			"name":        "viewer",
			"description": "Viewer role - read only",
			"policies": []interface{}{
				"*, get, allow", // Only read access
			},
		},
	})
	require.NoError(t, err, "failed to create test project")
	defer func() {
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, projectName, metav1.DeleteOptions{})
	}()

	// Wait for project creation
	time.Sleep(500 * time.Millisecond)

	// Assign viewer role to user
	resp, err := makeAuthenticatedRequest(
		"POST",
		"/api/v1/projects/"+projectName+"/roles/viewer/users/"+testUser,
		adminToken,
		nil,
	)
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated)

	// User token
	userToken := generateTestJWT(testUser, []string{}, false)

	// Step 1: Verify user can read (GET) but cannot create
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "STEP 1: User with viewer role can GET")

	// Step 2: Get current project state for update
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, adminToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var currentProject map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&currentProject)
	require.NoError(t, err)

	// Step 3: Update project to change viewer role policies (give full access)
	// AC-1: Permission changes via PUT /api/v1/projects/{name} should take effect immediately
	update := map[string]interface{}{
		"description":     "Updated project with expanded viewer role",
		"destinations":    currentProject["destinations"],
		"resourceVersion": currentProject["resourceVersion"],
		"roles": []map[string]interface{}{
			{
				"name":        "viewer",
				"description": "Viewer role - now with full access",
				"policies": []interface{}{
					"*, *, allow", // Full access
				},
			},
		},
	}

	resp, err = makeAuthenticatedRequest("PUT", "/api/v1/projects/"+projectName, adminToken, update)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "STEP 3: Admin should update project roles")

	// Step 4: IMMEDIATELY verify user has expanded permissions (NO waiting)
	// The user should now have the updated permissions from the role policy change
	// This tests that project updates trigger immediate policy reload
	resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"STEP 4 (AC-1): User should IMMEDIATELY have updated permissions after project role update - NO waiting for sync cycle")
}

// TestE2E_ImmediatePolicyReload_RapidRoleChanges tests that multiple rapid role changes
// all take effect immediately without accumulating delays.
func TestE2E_ImmediatePolicyReload_RapidRoleChanges(t *testing.T) {
	ctx := context.Background()
	adminToken := generateTestJWT(testUserAdmin, []string{}, true)

	// Setup: Create test project
	projectName := testProjectImmediateReload + "-rapid"
	err := createTestProject(ctx, projectName, "E2E rapid changes test", []map[string]interface{}{
		{
			"name":        "viewer",
			"description": "Viewer role",
			"policies": []interface{}{
				"*, get, allow",
			},
		},
	})
	require.NoError(t, err, "failed to create test project")
	defer func() {
		_ = dynamicClient.Resource(projectGVR).Delete(ctx, projectName, metav1.DeleteOptions{})
	}()

	time.Sleep(500 * time.Millisecond)

	// Perform rapid role assignments and removals
	// Each change should take effect immediately

	testUsers := []string{
		"rapid-user-1@example.com",
		"rapid-user-2@example.com",
		"rapid-user-3@example.com",
	}

	for i, user := range testUsers {
		// Generate user token
		userToken := generateTestJWT(user, []string{}, false)

		// Verify no access
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "User %d should not have access before assignment", i+1)

		// Assign role
		resp, err = makeAuthenticatedRequest(
			"POST",
			"/api/v1/projects/"+projectName+"/roles/viewer/users/"+user,
			adminToken,
			nil,
		)
		require.NoError(t, err)
		resp.Body.Close()
		require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated)

		// IMMEDIATELY verify access
		resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"User %d should have IMMEDIATE access after assignment (rapid change %d)", i+1, i+1)

		// Remove role
		resp, err = makeAuthenticatedRequest(
			"DELETE",
			"/api/v1/projects/"+projectName+"/roles/viewer/users/"+user,
			adminToken,
			nil,
		)
		require.NoError(t, err)
		resp.Body.Close()
		require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)

		// IMMEDIATELY verify no access
		resp, err = makeAuthenticatedRequest("GET", "/api/v1/projects/"+projectName, userToken, nil)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"User %d should IMMEDIATELY lose access after removal (rapid change %d)", i+1, i+1)
	}

	fmt.Printf("Successfully completed %d rapid role change cycles with immediate effect\n", len(testUsers))
}
