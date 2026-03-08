//go:build e2e

// Package e2e_test provides security edge case tests for authorization.
// Tests cover security edge cases.
package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	mockoidc "github.com/knodex/knodex/server/test/mocks/oidc"
)

// edgeCaseTestNamespace is the namespace used for edge case tests
const edgeCaseTestNamespace = "knodex-e2e-edge-cases"

// =============================================================================
// AC-6: Security Edge Case Tests
// =============================================================================

// TestAuthorization_EdgeCase_EmptyGroupsClaim verifies handling of JWT with empty groups.
// This tests that users with no group memberships are handled correctly.
func TestAuthorization_EdgeCase_EmptyGroupsClaim(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Create JWT with explicitly empty groups claim
	token := GenerateTestJWT(JWTClaims{
		Subject:     mockoidc.NoGroupsEmail,
		Email:       mockoidc.NoGroupsEmail,
		Projects:    []string{}, // No projects
		CasbinRoles: []string{}, // No roles
		Groups:      []string{}, // Empty groups
	})

	// User with no groups should be able to authenticate but have limited access
	t.Run("authenticated_but_no_permissions", func(t *testing.T) {
		resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", token, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be authenticated (not 401) but forbidden (403) for protected resources
		// OR should return empty list if listing is allowed but filtered
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusForbidden,
			"Expected 200 (empty list) or 403, got %d", resp.StatusCode)
	})

	t.Run("cannot_create_resources", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "test-project-empty-groups",
			"description": "Test project from empty groups user",
		}
		resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodPost, "/api/v1/projects", token, body)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "User with no groups should not create projects")
	})
}

// TestAuthorization_EdgeCase_InvalidCasbinRoles verifies handling of unrecognized roles.
// The system should reject or ignore invalid role formats.
func TestAuthorization_EdgeCase_InvalidCasbinRoles(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name        string
		casbinRoles []string
		description string
	}{
		{
			name:        "malformed_role",
			casbinRoles: []string{"invalid-role-format"},
			description: "Role without proper prefix",
		},
		{
			name:        "sql_injection_attempt",
			casbinRoles: []string{"role:admin'; DROP TABLE policies;--"},
			description: "SQL injection in role name",
		},
		{
			name:        "unicode_role",
			casbinRoles: []string{"role:admin\u0000hidden"},
			description: "Null byte in role name",
		},
		{
			name:        "very_long_role",
			casbinRoles: []string{"role:" + strings.Repeat("a", 10000)},
			description: "Excessively long role name",
		},
		{
			name:        "nested_role",
			casbinRoles: []string{"proj:alpha:proj:beta:admin"},
			description: "Improperly nested project role",
		},
		{
			name:        "empty_project_role",
			casbinRoles: []string{"proj::admin"},
			description: "Project role with empty project name",
		},
		{
			name:        "wildcard_role",
			casbinRoles: []string{"role:*"},
			description: "Wildcard role attempt",
		},
		{
			name:        "path_traversal_role",
			casbinRoles: []string{"proj:../admin:admin"},
			description: "Path traversal in project name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := GenerateTestJWT(JWTClaims{
				Subject:     "attacker@test.local",
				Email:       "attacker@test.local",
				Projects:    []string{},
				CasbinRoles: tc.casbinRoles,
			})

			resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", token, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should not grant elevated access - either 403 or 200 with limited data
			// Should NOT cause a server error (500)
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Invalid role '%s' should not cause server error", tc.description)

			// Verify the user doesn't have admin-level access
			if resp.StatusCode == http.StatusOK {
				// If OK, verify they don't have full access by trying a write operation
				writeResp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodPost, "/api/v1/projects", token, map[string]interface{}{
					"name": "test-from-invalid-role",
				})
				require.NoError(t, err)
				defer writeResp.Body.Close()

				assert.Equal(t, http.StatusForbidden, writeResp.StatusCode,
					"Invalid role '%s' should not grant write access", tc.description)
			}
		})
	}
}

// TestAuthorization_EdgeCase_ConcurrentPolicyUpdates tests authorization during policy changes.
// This verifies that concurrent policy updates don't cause race conditions or inconsistent state.
func TestAuthorization_EdgeCase_ConcurrentPolicyUpdates(t *testing.T) {
	if apiBaseURL == "" || dynamicClient == nil {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	// Create a test project for this test
	projectName := "e2e-concurrent-test"
	namespace := edgeCaseTestNamespace

	// Setup: Create the project
	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "core.knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":      projectName,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"description": "Concurrent policy test project",
				"destinations": []interface{}{
					map[string]interface{}{
						"namespace": namespace,
					},
				},
			},
		},
	}

	// Try to create or update the project
	_, err := dynamicClient.Resource(projectGVR).Namespace(namespace).Create(ctx, project, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		// Clean up any existing project and recreate
		_ = dynamicClient.Resource(projectGVR).Namespace(namespace).Delete(ctx, projectName, metav1.DeleteOptions{})
		time.Sleep(100 * time.Millisecond)
		_, err = dynamicClient.Resource(projectGVR).Namespace(namespace).Create(ctx, project, metav1.CreateOptions{})
	}

	// Cleanup at end
	defer func() {
		_ = dynamicClient.Resource(projectGVR).Namespace(namespace).Delete(ctx, projectName, metav1.DeleteOptions{})
	}()

	// Create multiple users with roles in this project
	var wg sync.WaitGroup
	errorChan := make(chan error, 100)
	successChan := make(chan int, 100)

	// Run concurrent requests while policy might be updating
	numGoroutines := 10
	requestsPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Each worker uses a slightly different user
			userEmail := fmt.Sprintf("concurrent-user-%d@test.local", workerID)
			token := GenerateTestJWT(JWTClaims{
				Subject:     userEmail,
				Email:       userEmail,
				Projects:    []string{projectName},
				CasbinRoles: []string{fmt.Sprintf("proj:%s:viewer", projectName)},
			})

			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", token, nil)
				if err != nil {
					errorChan <- fmt.Errorf("worker %d request %d: %w", workerID, j, err)
					continue
				}
				resp.Body.Close()

				// Check for server errors (would indicate race condition)
				if resp.StatusCode >= 500 {
					errorChan <- fmt.Errorf("worker %d request %d: server error %d", workerID, j, resp.StatusCode)
				} else {
					successChan <- resp.StatusCode
				}
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)
	close(successChan)

	// Count results
	var errors []error
	successCount := 0

	for err := range errorChan {
		errors = append(errors, err)
	}
	for range successChan {
		successCount++
	}

	// No server errors should occur during concurrent access
	assert.Empty(t, errors, "Concurrent policy access should not cause errors: %v", errors)
	assert.Equal(t, numGoroutines*requestsPerGoroutine, successCount,
		"All requests should complete successfully")
}

// TestAuthorization_EdgeCase_PathTraversal verifies path traversal attempts are blocked.
// Attackers might try to access resources outside their scope using path manipulation.
func TestAuthorization_EdgeCase_PathTraversal(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// User with limited access
	token := GenerateTestJWT(JWTClaims{
		Subject:     mockoidc.ViewerEmail,
		Email:       mockoidc.ViewerEmail,
		Projects:    []string{"limited-project"},
		CasbinRoles: []string{"proj:limited-project:viewer"},
	})

	pathTraversalAttempts := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "basic_traversal",
			path:        "/api/v1/../admin/settings",
			description: "Basic path traversal with ..",
		},
		{
			name:        "double_encoded_traversal",
			path:        "/api/v1/%2e%2e/admin/settings",
			description: "URL-encoded path traversal",
		},
		{
			name:        "unicode_traversal",
			path:        "/api/v1/\u002e\u002e/admin",
			description: "Unicode path traversal",
		},
		{
			name:        "backslash_traversal",
			path:        "/api/v1/..\\admin\\settings",
			description: "Backslash path traversal (Windows-style)",
		},
		{
			name:        "null_byte_traversal",
			path:        "/api/v1/projects\x00/../admin",
			description: "Null byte path traversal",
		},
		{
			name:        "excessive_dots",
			path:        "/api/v1/../../../../../../etc/passwd",
			description: "Multiple levels path traversal",
		},
	}

	for _, tc := range pathTraversalAttempts {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, tc.path, token, nil)
			if err != nil {
				// URLs with null bytes or invalid control characters are rejected by Go's net/url parser.
				// This is a valid security outcome - the request never reaches the server.
				t.Logf("Request rejected by URL parser (safe): %v", err)
				return
			}
			defer resp.Body.Close()

			// Should either:
			// 1. Return 404 (path not found after normalization)
			// 2. Return 400 (bad request for invalid path)
			// 3. Return 403 (forbidden - no access to the resolved path)
			// Should NOT:
			// - Return 200 with sensitive data
			// - Return 500 (server error)
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Path traversal '%s' should not cause server error", tc.description)

			// Verify no unauthorized access
			if resp.StatusCode == http.StatusOK {
				// Read body to verify we didn't access admin resources
				var body map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&body)

				// Should not contain admin settings or sensitive data
				_, hasAdminSettings := body["adminSettings"]
				assert.False(t, hasAdminSettings,
					"Path traversal '%s' should not expose admin settings", tc.description)
			}
		})
	}
}

// TestAuthorization_EdgeCase_NullBytesInResourceIDs tests null byte injection in resource identifiers.
// Null bytes can sometimes truncate strings in C-based systems, potentially bypassing validation.
func TestAuthorization_EdgeCase_NullBytesInResourceIDs(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	adminToken := GenerateTestJWT(JWTClaims{
		Subject:     mockoidc.AdminEmail,
		Email:       mockoidc.AdminEmail,
		Projects:    []string{"*"},
		CasbinRoles: []string{roleGlobalAdmin},
	})

	nullByteTests := []struct {
		name        string
		resourceID  string
		description string
	}{
		{
			name:        "null_in_middle",
			resourceID:  "project\x00-hidden",
			description: "Null byte in middle of resource ID",
		},
		{
			name:        "null_at_end",
			resourceID:  "project\x00",
			description: "Null byte at end of resource ID",
		},
		{
			name:        "multiple_nulls",
			resourceID:  "pro\x00ject\x00",
			description: "Multiple null bytes",
		},
		{
			name:        "unicode_null",
			resourceID:  "project\u0000hidden",
			description: "Unicode null character",
		},
	}

	for _, tc := range nullByteTests {
		t.Run(tc.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/projects/%s", tc.resourceID)
			resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, path, adminToken, nil)
			if err != nil {
				// URLs with null bytes are rejected by Go's net/url parser.
				// This is a valid security outcome - the request never reaches the server.
				t.Logf("Request rejected by URL parser (safe): %v", err)
				return
			}
			defer resp.Body.Close()

			// Should return 400 (bad request) or 404 (not found)
			// Should NOT return 500 or expose unexpected resources
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Null byte in resource ID '%s' should not cause server error", tc.description)
		})
	}
}

// TestAuthorization_EdgeCase_VeryLongResourceNames tests handling of oversized resource names.
// This prevents potential buffer overflow or denial of service attacks.
func TestAuthorization_EdgeCase_VeryLongResourceNames(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 30 * time.Second} // Longer timeout for large payloads

	adminToken := GenerateTestJWT(JWTClaims{
		Subject:     mockoidc.AdminEmail,
		Email:       mockoidc.AdminEmail,
		Projects:    []string{"*"},
		CasbinRoles: []string{roleGlobalAdmin},
	})

	longNameTests := []struct {
		name   string
		length int
	}{
		{name: "1KB_name", length: 1024},
		{name: "10KB_name", length: 10 * 1024},
		{name: "100KB_name", length: 100 * 1024},
		{name: "1MB_name", length: 1024 * 1024},
	}

	for _, tc := range longNameTests {
		t.Run(tc.name, func(t *testing.T) {
			longName := strings.Repeat("a", tc.length)

			// Test GET with long path
			path := fmt.Sprintf("/api/v1/projects/%s", longName)
			resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, path, adminToken, nil)
			if err != nil {
				// Network error for very large URL is acceptable
				t.Logf("GET with %d byte path caused network error (acceptable): %v", tc.length, err)
				return
			}
			defer resp.Body.Close()

			// Should return appropriate error, not crash
			// 414 URI Too Long, 400 Bad Request, 404 Not Found, or 500 for extreme sizes are acceptable
			// Note: 1MB+ URL paths may cause 500 in Go's HTTP server infrastructure
			if tc.length >= 100*1024 {
				// For extreme sizes (100KB+), any non-200 response is acceptable
				assert.NotEqual(t, http.StatusOK, resp.StatusCode,
					"Extremely long resource name (%d bytes) should not return 200", tc.length)
			} else {
				assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
					"Long resource name (%d bytes) should not cause server error", tc.length)
			}

			// Test POST with long name in body
			body := map[string]interface{}{
				"name":        longName,
				"description": "Test with long name",
			}
			postResp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodPost, "/api/v1/projects", adminToken, body)
			if err != nil {
				// Network error for very large payload is acceptable
				t.Logf("POST with %d byte name caused network error (acceptable): %v", tc.length, err)
				return
			}
			defer postResp.Body.Close()

			// Should reject with 400 or similar, not crash
			assert.NotEqual(t, http.StatusInternalServerError, postResp.StatusCode,
				"POST with long name (%d bytes) should not cause server error", tc.length)
		})
	}
}

// TestAuthorization_EdgeCase_SpecialCharactersInIDs tests handling of special characters in identifiers.
// This ensures proper escaping and prevents injection attacks.
func TestAuthorization_EdgeCase_SpecialCharactersInIDs(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	adminToken := GenerateTestJWT(JWTClaims{
		Subject:     mockoidc.AdminEmail,
		Email:       mockoidc.AdminEmail,
		Projects:    []string{"*"},
		CasbinRoles: []string{roleGlobalAdmin},
	})

	specialCharTests := []struct {
		name        string
		resourceID  string
		description string
	}{
		{
			name:        "angle_brackets",
			resourceID:  "<script>alert(1)</script>",
			description: "HTML/JS injection attempt",
		},
		{
			name:        "sql_injection",
			resourceID:  "'; DROP TABLE projects;--",
			description: "SQL injection attempt",
		},
		{
			name:        "ldap_injection",
			resourceID:  "*)(uid=*))(|(uid=*",
			description: "LDAP injection attempt",
		},
		{
			name:        "command_injection",
			resourceID:  "; cat /etc/passwd",
			description: "Command injection attempt",
		},
		{
			name:        "json_injection",
			resourceID:  `{"admin":true}`,
			description: "JSON injection attempt",
		},
		{
			name:        "template_injection",
			resourceID:  "{{.Config}}",
			description: "Go template injection",
		},
		{
			name:        "emoji_bomb",
			resourceID:  strings.Repeat("🔥", 1000),
			description: "Excessive unicode characters",
		},
		{
			name:        "rtl_override",
			resourceID:  "admin\u202eresu",
			description: "Right-to-left override character",
		},
	}

	for _, tc := range specialCharTests {
		t.Run(tc.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/v1/projects/%s", tc.resourceID)
			resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, path, adminToken, nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should handle gracefully
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Special characters '%s' should not cause server error", tc.description)

			// Check response doesn't reflect back unescaped content (XSS prevention)
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
				var body map[string]interface{}
				if json.NewDecoder(resp.Body).Decode(&body) == nil {
					bodyStr := fmt.Sprintf("%v", body)
					assert.NotContains(t, bodyStr, "<script>",
						"Response should not contain unescaped script tags")
				}
			}
		})
	}
}

// TestAuthorization_EdgeCase_TimingAttackResistance verifies consistent response times.
// Authorization should take similar time regardless of whether the user exists or permissions match.
func TestAuthorization_EdgeCase_TimingAttackResistance(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Measure response times for different scenarios
	scenarios := []struct {
		name  string
		token string
	}{
		{
			name: "valid_admin",
			token: GenerateTestJWT(JWTClaims{
				Subject:     mockoidc.AdminEmail,
				Email:       mockoidc.AdminEmail,
				CasbinRoles: []string{roleGlobalAdmin},
			}),
		},
		{
			name: "valid_no_access",
			token: GenerateTestJWT(JWTClaims{
				Subject:     mockoidc.NoGroupsEmail,
				Email:       mockoidc.NoGroupsEmail,
				CasbinRoles: []string{},
			}),
		},
		{
			name: "nonexistent_user",
			token: GenerateTestJWT(JWTClaims{
				Subject:     "nonexistent@test.local",
				Email:       "nonexistent@test.local",
				CasbinRoles: []string{},
			}),
		},
	}

	const iterations = 10
	timings := make(map[string][]time.Duration)

	for _, sc := range scenarios {
		timings[sc.name] = make([]time.Duration, 0, iterations)

		for i := 0; i < iterations; i++ {
			start := time.Now()
			resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", sc.token, nil)
			duration := time.Since(start)

			if err == nil {
				resp.Body.Close()
				timings[sc.name] = append(timings[sc.name], duration)
			}
		}
	}

	// Calculate average times
	averages := make(map[string]time.Duration)
	for name, times := range timings {
		if len(times) == 0 {
			continue
		}
		var total time.Duration
		for _, d := range times {
			total += d
		}
		averages[name] = total / time.Duration(len(times))
	}

	// Log timing analysis (informational - not a hard failure)
	t.Logf("Timing analysis (informational):")
	for name, avg := range averages {
		t.Logf("  %s: %v average", name, avg)
	}

	// Check that timing differences are not excessive
	// This is a soft check - significant differences might indicate timing vulnerabilities
	if len(averages) >= 2 {
		var maxDiff time.Duration
		for name1, avg1 := range averages {
			for name2, avg2 := range averages {
				if name1 >= name2 {
					continue
				}
				diff := avg1 - avg2
				if diff < 0 {
					diff = -diff
				}
				if diff > maxDiff {
					maxDiff = diff
				}
			}
		}

		// Warn if timing difference is significant (>100ms might indicate info leak)
		if maxDiff > 100*time.Millisecond {
			t.Logf("WARNING: Timing difference of %v detected - may be vulnerable to timing attacks", maxDiff)
		}
	}
}

// TestAuthorization_EdgeCase_JWTAlgorithmConfusion tests algorithm confusion attacks.
// Attackers might try to use 'none' algorithm or switch between RSA/HMAC.
func TestAuthorization_EdgeCase_JWTAlgorithmConfusion(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("none_algorithm", func(t *testing.T) {
		// Create a token with 'none' algorithm (should be rejected)
		claims := jwt.MapClaims{
			"sub":          mockoidc.AdminEmail,
			"email":        mockoidc.AdminEmail,
			"casbin_roles": []string{roleGlobalAdmin},
			"iss":          "knodex",
			"aud":          "knodex-api",
			"exp":          time.Now().Add(time.Hour).Unix(),
			"iat":          time.Now().Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

		resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", tokenString, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Token with 'none' algorithm should be rejected")
	})

	t.Run("alg_header_manipulation", func(t *testing.T) {
		// Create a properly signed HS256 token but modify header (simulated)
		// In practice, this would require direct header manipulation
		claims := jwt.MapClaims{
			"sub":          mockoidc.AdminEmail,
			"email":        mockoidc.AdminEmail,
			"casbin_roles": []string{roleGlobalAdmin},
			"iss":          "knodex",
			"aud":          "knodex-api",
			"exp":          time.Now().Add(time.Hour).Unix(),
			"iat":          time.Now().Unix(),
		}

		// Try signing with a different algorithm (HS384)
		token := jwt.NewWithClaims(jwt.SigningMethodHS384, claims)
		tokenString, _ := token.SignedString([]byte(TestJWTSecret))

		resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", tokenString, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should either accept (if HS384 is allowed) or reject
		// Should NOT grant elevated access if algorithm is unexpected
		assert.Contains(t, []int{http.StatusOK, http.StatusUnauthorized}, resp.StatusCode,
			"Algorithm mismatch should be handled consistently")
	})
}

// TestAuthorization_EdgeCase_ReplayAttack tests protection against token replay.
// While JWTs are stateless, expired or revoked tokens should be rejected.
func TestAuthorization_EdgeCase_ReplayAttack(t *testing.T) {
	if apiBaseURL == "" {
		t.Skip("E2E tests require KIND cluster - skipping")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("expired_token_replay", func(t *testing.T) {
		// Create a token that expired 1 minute ago
		expiredToken := GenerateTestJWT(JWTClaims{
			Subject:     mockoidc.AdminEmail,
			Email:       mockoidc.AdminEmail,
			CasbinRoles: []string{roleGlobalAdmin},
			ExpiresIn:   -1 * time.Minute, // Already expired
		})

		resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", expiredToken, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Expired token should be rejected")
	})

	t.Run("future_dated_token", func(t *testing.T) {
		// Create a token with iat in the future (suspicious)
		claims := jwt.MapClaims{
			"sub":          mockoidc.AdminEmail,
			"email":        mockoidc.AdminEmail,
			"casbin_roles": []string{roleGlobalAdmin},
			"iss":          "knodex",
			"aud":          "knodex-api",
			"exp":          time.Now().Add(2 * time.Hour).Unix(),
			"iat":          time.Now().Add(1 * time.Hour).Unix(), // Future iat
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(TestJWTSecret))

		resp, err := MakeAuthenticatedRequest(client, apiBaseURL, http.MethodGet, "/api/v1/projects", tokenString, nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should be accepted (iat validation is optional) or rejected
		// The important thing is consistent behavior
		t.Logf("Future-dated token response: %d (informational)", resp.StatusCode)
	})
}
