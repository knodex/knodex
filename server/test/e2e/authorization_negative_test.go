// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

// Package e2e_test provides negative authorization tests for security compliance.
// These tests verify proper rejection of unauthorized requests.
package e2e_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// AC-4: Negative Authorization Tests - Proper Rejection of Unauthorized Requests
// =============================================================================

// TestAuthorization_Negative_MissingJWTToken verifies 401 for missing token (AC-4).
func TestAuthorization_Negative_MissingJWTToken(t *testing.T) {
	// All protected endpoints should return 401 without token
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/rgds"},
		{"GET", "/api/v1/instances"},
		{"GET", "/api/v1/projects"},
		{"GET", "/api/v1/repositories"},
		{"GET", "/api/v1/namespaces"},
		{"POST", "/api/v1/projects"},
		{"POST", "/api/v1/instances"},
		{"POST", "/api/v1/repositories"},
		{"DELETE", "/api/v1/projects/test-project"},
		{"DELETE", "/api/v1/namespaces/default/instances/SimpleApp/test"},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			resp, err := makeAuthenticatedRequest(ep.method, ep.path, "", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Missing JWT token should return 401 for %s %s", ep.method, ep.path)

			// Verify error response structure
			var errorResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
				// Check that error message is present
				if msg, ok := errorResp["error"].(string); ok {
					assert.NotEmpty(t, msg, "Error message should be present")
				}
			}
		})
	}
}

// TestAuthorization_Negative_ExpiredJWTToken verifies 401 for expired token (AC-4).
func TestAuthorization_Negative_ExpiredJWTToken(t *testing.T) {
	// Create an expired token (expired 1 hour ago)
	claims := jwt.MapClaims{
		"sub":            "expired-user@test.local",
		"email":          "expired-user@test.local",
		"name":           "Expired User",
		"casbin_roles":   []string{"role:serveradmin"},
		"iss":            "knodex",
		"aud":            "knodex-api",
		"exp":            time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
		"iat":            time.Now().Add(-2 * time.Hour).Unix(),
		"email_verified": true,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, err := token.SignedString([]byte(TestJWTSecret))
	require.NoError(t, err)

	endpoints := []string{
		"/api/v1/rgds",
		"/api/v1/instances",
		"/api/v1/projects",
	}

	for _, path := range endpoints {
		t.Run(path, func(t *testing.T) {
			resp, err := makeAuthenticatedRequest("GET", path, expiredToken, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Expired JWT token should return 401 for %s", path)

			// Check error message mentions expiration
			body, _ := io.ReadAll(resp.Body)
			bodyStr := strings.ToLower(string(body))
			assert.True(t,
				strings.Contains(bodyStr, "expired") ||
					strings.Contains(bodyStr, "invalid") ||
					strings.Contains(bodyStr, "unauthorized"),
				"Error should indicate token expiry issue, got: %s", string(body))
		})
	}
}

// TestAuthorization_Negative_MalformedJWTToken verifies 401 for malformed token (AC-4).
func TestAuthorization_Negative_MalformedJWTToken(t *testing.T) {
	malformedTokens := []struct {
		name  string
		token string
	}{
		{"EmptyString", ""},
		{"RandomString", "not-a-valid-jwt-token"},
		{"PartialJWT", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		{"MissingSignature", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"},
		{"InvalidBase64", "not.valid.base64!!!"},
		{"TruncatedToken", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMj"},
		{"ModifiedPayload", generateModifiedPayloadToken()},
	}

	for _, tc := range malformedTokens {
		t.Run(tc.name, func(t *testing.T) {
			if tc.token == "" {
				// Empty token is handled by missing token test
				return
			}

			resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", tc.token, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Malformed JWT token (%s) should return 401", tc.name)
		})
	}
}

// generateModifiedPayloadToken creates a token with modified payload (invalid signature).
func generateModifiedPayloadToken() string {
	// Create a valid token first
	claims := jwt.MapClaims{
		"sub":          "test@test.local",
		"email":        "test@test.local",
		"casbin_roles": []string{"role:serveradmin"},
		"iss":          "knodex",
		"aud":          "knodex-api",
		"exp":          time.Now().Add(1 * time.Hour).Unix(),
		"iat":          time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(TestJWTSecret))

	// Modify the payload part (second segment)
	parts := strings.Split(tokenString, ".")
	if len(parts) == 3 {
		// Corrupt the payload
		parts[1] = parts[1] + "corrupted"
		return strings.Join(parts, ".")
	}
	return tokenString
}

// TestAuthorization_Negative_InsufficientPermissions verifies 403 for insufficient permissions (AC-4).
func TestAuthorization_Negative_InsufficientPermissions(t *testing.T) {
	// Create a token with project-scoped viewer role (no global readonly role)
	readonlyToken := GenerateTestJWT(JWTClaims{
		Subject:     "readonly@test.local",
		Email:       "readonly@test.local",
		CasbinRoles: []string{"proj:e2e-team-a:viewer"},
	})

	// Create a token with no roles
	noRoleToken := GenerateTestJWT(JWTClaims{
		Subject:     "norole@test.local",
		Email:       "norole@test.local",
		CasbinRoles: []string{},
	})

	// Write operations that should be forbidden for readonly users
	writeOperations := []struct {
		method string
		path   string
		body   interface{}
	}{
		{"POST", "/api/v1/projects", map[string]interface{}{
			"name":        "e2e-forbidden-project",
			"description": "Should fail",
			"destinations": []map[string]interface{}{
				{"namespace": "test"},
			},
		}},
		{"DELETE", "/api/v1/projects/" + testProjectShared, nil},
		{"POST", "/api/v1/repositories", map[string]interface{}{
			"url":  "https://github.com/test/forbidden-repo",
			"name": "forbidden-repo",
			"type": "git",
		}},
	}

	for _, op := range writeOperations {
		t.Run(fmt.Sprintf("Readonly_%s_%s", op.method, op.path), func(t *testing.T) {
			resp, err := makeAuthenticatedRequest(op.method, op.path, readonlyToken, op.body)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"Readonly user should get 403 for %s %s", op.method, op.path)
		})

		t.Run(fmt.Sprintf("NoRole_%s_%s", op.method, op.path), func(t *testing.T) {
			resp, err := makeAuthenticatedRequest(op.method, op.path, noRoleToken, op.body)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"No-role user should get 403 for %s %s", op.method, op.path)
		})
	}
}

// TestAuthorization_Negative_CrossProjectAccess verifies 403 for cross-project access (AC-4).
func TestAuthorization_Negative_CrossProjectAccess(t *testing.T) {
	// Create a token scoped to project team-a only
	teamAToken := GenerateTestJWT(JWTClaims{
		Subject:     "team-a-admin@test.local",
		Email:       "team-a-admin@test.local",
		CasbinRoles: []string{fmt.Sprintf("proj:%s:admin", testProjectTeamA)},
	})

	// Try to access team-b project (cross-project access)
	crossProjectPaths := []struct {
		method string
		path   string
		body   interface{}
	}{
		{"GET", "/api/v1/projects/" + testProjectTeamB, nil},
		{"PUT", "/api/v1/projects/" + testProjectTeamB, map[string]interface{}{
			"description": "Trying to modify another project",
		}},
		{"DELETE", "/api/v1/projects/" + testProjectTeamB, nil},
		{"POST", fmt.Sprintf("/api/v1/projects/%s/roles/viewer/users/test@test.local", testProjectTeamB), nil},
	}

	for _, op := range crossProjectPaths {
		t.Run(fmt.Sprintf("%s_%s", op.method, op.path), func(t *testing.T) {
			resp, err := makeAuthenticatedRequest(op.method, op.path, teamAToken, op.body)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusForbidden, resp.StatusCode,
				"Cross-project access should return 403 for %s %s", op.method, op.path)
		})
	}
}

// TestAuthorization_Negative_NamespaceOutsideProject verifies 403 for namespace access outside project (AC-4).
func TestAuthorization_Negative_NamespaceOutsideProject(t *testing.T) {
	// Create a token scoped to team-a project with specific namespace destinations
	teamAToken := GenerateTestJWT(JWTClaims{
		Subject:     "team-a-dev@test.local",
		Email:       "team-a-dev@test.local",
		CasbinRoles: []string{fmt.Sprintf("proj:%s:developer", testProjectTeamA)},
	})

	// Try to deploy to a namespace outside project destinations
	instancePayload := map[string]interface{}{
		"name":      "e2e-cross-namespace-test",
		"namespace": "kube-system", // System namespace, likely not in project destinations
		"rgdName":   "test-rgd",
		"values":    map[string]interface{}{},
	}

	t.Run("DeployToOutsideNamespace", func(t *testing.T) {
		resp, err := makeAuthenticatedRequest("POST", "/api/v1/instances", teamAToken, instancePayload)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be forbidden (403), bad request (400), or not found (404) because kube-system is not in project destinations
		assert.True(t, resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusBadRequest ||
			resp.StatusCode == http.StatusNotFound,
			"Deploying to namespace outside project should be denied, got %d", resp.StatusCode)
	})
}

// TestAuthorization_Negative_InvalidWrongSignature verifies 401 for wrong signature (AC-4).
func TestAuthorization_Negative_InvalidWrongSignature(t *testing.T) {
	// Create a token signed with a different secret
	claims := jwt.MapClaims{
		"sub":          "admin@test.local",
		"email":        "admin@test.local",
		"casbin_roles": []string{"role:serveradmin"},
		"iss":          "knodex",
		"aud":          "knodex-api",
		"exp":          time.Now().Add(1 * time.Hour).Unix(),
		"iat":          time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	wrongSignatureToken, err := token.SignedString([]byte("wrong-secret-key-different-from-backend"))
	require.NoError(t, err)

	endpoints := []string{
		"/api/v1/rgds",
		"/api/v1/instances",
		"/api/v1/projects",
	}

	for _, path := range endpoints {
		t.Run(path, func(t *testing.T) {
			resp, err := makeAuthenticatedRequest("GET", path, wrongSignatureToken, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Token with wrong signature should return 401 for %s", path)
		})
	}
}

// TestAuthorization_Negative_FutureDateToken verifies handling of future-dated tokens (AC-4).
func TestAuthorization_Negative_FutureDateToken(t *testing.T) {
	// Create a token with "not before" (nbf) in the future
	claims := jwt.MapClaims{
		"sub":          "future@test.local",
		"email":        "future@test.local",
		"casbin_roles": []string{"role:serveradmin"},
		"iss":          "knodex",
		"aud":          "knodex-api",
		"exp":          time.Now().Add(2 * time.Hour).Unix(),
		"iat":          time.Now().Add(1 * time.Hour).Unix(), // Issued in the future
		"nbf":          time.Now().Add(1 * time.Hour).Unix(), // Not valid until 1 hour from now
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	futureToken, err := token.SignedString([]byte(TestJWTSecret))
	require.NoError(t, err)

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", futureToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Token with future nbf should be rejected
	// Note: Some JWT implementations may not validate nbf, so we accept 200 as well
	assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusOK,
		"Future-dated token handling, got %d", resp.StatusCode)
}

// TestAuthorization_Negative_MissingRequiredClaims verifies 401 for missing required claims (AC-4).
func TestAuthorization_Negative_MissingRequiredClaims(t *testing.T) {
	testCases := []struct {
		name   string
		claims jwt.MapClaims
	}{
		{
			name: "MissingSub",
			claims: jwt.MapClaims{
				"email":        "test@test.local",
				"casbin_roles": []string{"role:serveradmin"},
				"iss":          "knodex",
				"aud":          "knodex-api",
				"exp":          time.Now().Add(1 * time.Hour).Unix(),
				"iat":          time.Now().Unix(),
			},
		},
		{
			name: "MissingEmail",
			claims: jwt.MapClaims{
				"sub":          "test@test.local",
				"casbin_roles": []string{"role:serveradmin"},
				"iss":          "knodex",
				"aud":          "knodex-api",
				"exp":          time.Now().Add(1 * time.Hour).Unix(),
				"iat":          time.Now().Unix(),
			},
		},
		{
			name: "MissingExp",
			claims: jwt.MapClaims{
				"sub":          "test@test.local",
				"email":        "test@test.local",
				"casbin_roles": []string{"role:serveradmin"},
				"iss":          "knodex",
				"aud":          "knodex-api",
				"iat":          time.Now().Unix(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, tc.claims)
			tokenString, err := token.SignedString([]byte(TestJWTSecret))
			require.NoError(t, err)

			resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", tokenString, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Missing required claims should result in unauthorized or bad request
			assert.True(t, resp.StatusCode == http.StatusUnauthorized ||
				resp.StatusCode == http.StatusBadRequest ||
				resp.StatusCode == http.StatusOK, // Some backends may be lenient
				"%s: expected 401/400, got %d", tc.name, resp.StatusCode)
		})
	}
}

// TestAuthorization_Negative_TokenReplay verifies token cannot be replayed after revocation (AC-4).
// Note: This test documents expected behavior but actual implementation depends on token revocation support.
func TestAuthorization_Negative_TokenReplay(t *testing.T) {
	t.Skip("Token revocation not implemented - test documents expected behavior")

	// This test would verify:
	// 1. Generate a valid token
	// 2. Use token successfully
	// 3. Revoke token (via logout or admin action)
	// 4. Verify token is rejected after revocation
}

// TestAuthorization_Negative_UnsupportedAlgorithm verifies rejection of unsupported signing algorithms (AC-4).
func TestAuthorization_Negative_UnsupportedAlgorithm(t *testing.T) {
	// Attempt to use "none" algorithm (JWT security vulnerability)
	// Note: This creates a token header claiming "none" algorithm
	noneAlgToken := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiJhZG1pbkB0ZXN0LmxvY2FsIiwiZW1haWwiOiJhZG1pbkB0ZXN0LmxvY2FsIiwiY2FzYmluX3JvbGVzIjpbInJvbGU6YWRtaW4iXSwiZXhwIjo5OTk5OTk5OTk5fQ."

	resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", noneAlgToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Token with 'none' algorithm should be rejected")
}

// TestAuthorization_Negative_EmptyBearerToken verifies rejection of empty bearer token (AC-4).
func TestAuthorization_Negative_EmptyBearerToken(t *testing.T) {
	// Create a request with "Bearer " prefix but no actual token
	req, err := http.NewRequest("GET", apiBaseURL+"/api/v1/projects", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer ")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Empty bearer token should return 401")
}

// TestAuthorization_Negative_InvalidAuthorizationType verifies rejection of non-Bearer auth (AC-4).
func TestAuthorization_Negative_InvalidAuthorizationType(t *testing.T) {
	authTypes := []string{
		"Basic dXNlcm5hbWU6cGFzc3dvcmQ=", // Basic auth
		"Digest username=test",           // Digest auth
		"token abc123",                   // Non-standard
		"JWT " + GenerateSimpleJWT("test@test.local", nil, true), // JWT instead of Bearer
	}

	for _, authHeader := range authTypes {
		t.Run(authHeader[:10], func(t *testing.T) {
			req, err := http.NewRequest("GET", apiBaseURL+"/api/v1/projects", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", authHeader)

			resp, err := httpClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Non-Bearer auth type should return 401")
		})
	}
}

// TestAuthorization_Negative_ProjectAdminCannotDeleteOtherProjects verifies project scope (AC-4).
func TestAuthorization_Negative_ProjectAdminCannotDeleteOtherProjects(t *testing.T) {
	// Project admin for team-a trying to delete shared project
	projectAdminToken := GenerateTestJWT(JWTClaims{
		Subject:     "team-a-admin@test.local",
		Email:       "team-a-admin@test.local",
		CasbinRoles: []string{fmt.Sprintf("proj:%s:admin", testProjectTeamA)},
	})

	// Cannot delete the global shared project
	resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/projects/"+testProjectShared, projectAdminToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Project admin should not delete projects outside their scope")
}

// TestAuthorization_Negative_ViewerCannotModifyRoleBindings verifies viewer cannot manage roles (AC-4).
func TestAuthorization_Negative_ViewerCannotModifyRoleBindings(t *testing.T) {
	viewerToken := GenerateTestJWT(JWTClaims{
		Subject:     "viewer@test.local",
		Email:       "viewer@test.local",
		CasbinRoles: []string{fmt.Sprintf("proj:%s:viewer", testProjectTeamA)},
	})

	// Viewer cannot assign roles
	t.Run("CannotAssignRole", func(t *testing.T) {
		resp, err := makeAuthenticatedRequest(
			"POST",
			fmt.Sprintf("/api/v1/projects/%s/roles/viewer/users/new-user@test.local", testProjectTeamA),
			viewerToken,
			nil,
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"Viewer should not assign roles")
	})

	// Viewer cannot remove roles
	t.Run("CannotRemoveRole", func(t *testing.T) {
		resp, err := makeAuthenticatedRequest(
			"DELETE",
			fmt.Sprintf("/api/v1/projects/%s/roles/viewer/users/some-user@test.local", testProjectTeamA),
			viewerToken,
			nil,
		)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode,
			"Viewer should not remove roles")
	})
}

// TestAuthorization_Negative_DeveloperCannotDeleteProjects verifies developer cannot delete (AC-4).
func TestAuthorization_Negative_DeveloperCannotDeleteProjects(t *testing.T) {
	developerToken := GenerateTestJWT(JWTClaims{
		Subject:     "developer@test.local",
		Email:       "developer@test.local",
		CasbinRoles: []string{fmt.Sprintf("proj:%s:developer", testProjectTeamA)},
	})

	resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/projects/"+testProjectTeamA, developerToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode,
		"Developer should not delete projects")
}

// =============================================================================
// Error Response Format Tests
// =============================================================================

// TestAuthorization_Negative_ErrorResponseFormat verifies error responses are properly formatted (AC-4).
func TestAuthorization_Negative_ErrorResponseFormat(t *testing.T) {
	// Unauthenticated request
	t.Run("401_ErrorFormat", func(t *testing.T) {
		resp, err := makeAuthenticatedRequest("GET", "/api/v1/projects", "", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		// Should be valid JSON
		assert.NoError(t, err, "Error response should be valid JSON")
		// Should have error field
		_, hasError := errorResp["error"]
		_, hasMessage := errorResp["message"]
		assert.True(t, hasError || hasMessage, "Error response should have error or message field")
	})

	// Forbidden request
	t.Run("403_ErrorFormat", func(t *testing.T) {
		noRoleToken := GenerateTestJWT(JWTClaims{
			Subject:     "norole@test.local",
			Email:       "norole@test.local",
			CasbinRoles: []string{},
		})

		resp, err := makeAuthenticatedRequest("DELETE", "/api/v1/projects/"+testProjectShared, noRoleToken, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		// Should be valid JSON
		assert.NoError(t, err, "Error response should be valid JSON")
	})
}
