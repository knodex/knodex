//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// =============================================================================
// JWT Configuration
// =============================================================================

// TestJWTSecret is the shared JWT secret for E2E tests.
// Must match backend configuration (JWT_SECRET env var in kustomize overlay).
// Reads from E2E_JWT_SECRET env var, defaulting to the qa-deploy secret.
var TestJWTSecret = getTestJWTSecret()

func getTestJWTSecret() string {
	if s := os.Getenv("E2E_JWT_SECRET"); s != "" {
		return s
	}
	return "test-jwt-secret-key-for-qa-testing-only"
}

// =============================================================================
// JWT Token Generation
// =============================================================================

// JWTClaims represents configurable JWT claims for test token generation.
// All required claims for backend validation are included with sensible defaults.

type JWTClaims struct {
	// Required claims
	Subject     string   // sub - user identifier (usually email)
	Email       string   // email - user's email address
	Name        string   // name - display name (auto-generated from email if empty)
	Projects    []string // projects - list of project IDs user has access to
	CasbinRoles []string // casbin_roles - Casbin roles (e.g., "role:serveradmin") for permission checks

	// Optional OIDC claims
	Groups   []string // groups - OIDC group memberships
	Issuer   string   // iss - token issuer
	Audience string   // aud - intended audience

	// Token validity (defaults to 1 hour if zero)
	ExpiresIn time.Duration
}

// GenerateTestJWT creates a JWT token for E2E tests with the specified claims.
// This function ensures all required claims for backend validation are included:
// sub, email, name, projects, casbin_roles, exp, iat

// Example usage:
//
//	token := GenerateTestJWT(JWTClaims{
//	    Subject:     "alice@example.com",
//	    Email:       "alice@example.com",
//	    Projects:    []string{"project-a", "project-b"},
//	    CasbinRoles: []string{"role:serveradmin"}, // for global admin
//	})
func GenerateTestJWT(opts JWTClaims) string {
	// Auto-generate display name from email if not provided
	name := opts.Name
	if name == "" && opts.Email != "" {
		name = strings.Split(opts.Email, "@")[0]
	}
	if name == "" && opts.Subject != "" {
		name = strings.Split(opts.Subject, "@")[0]
	}

	// Default expiry to 1 hour
	expiresIn := opts.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 1 * time.Hour
	}

	// Build claims map with required fields

	// Default issuer/audience to match production token generation
	issuer := opts.Issuer
	if issuer == "" {
		issuer = "knodex"
	}
	audience := opts.Audience
	if audience == "" {
		audience = "knodex-api"
	}

	claims := jwt.MapClaims{
		"sub":      opts.Subject,
		"email":    opts.Email,
		"name":     name,
		"projects": opts.Projects,
		"iss":      issuer,
		"aud":      audience,
		"exp":      time.Now().Add(expiresIn).Unix(),
		"iat":      time.Now().Unix(),
	}

	// Add casbin_roles if provided
	if len(opts.CasbinRoles) > 0 {
		claims["casbin_roles"] = opts.CasbinRoles
	}

	// Add optional OIDC claims if provided
	if len(opts.Groups) > 0 {
		claims["groups"] = opts.Groups
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(TestJWTSecret))
	return tokenString
}

// GenerateSimpleJWT creates a JWT token with minimal required claims.
// This is a convenience wrapper for simple test cases.
//
// Parameters:
//   - userID: used as both subject and email
//   - projects: list of projects the user has access to
//   - addAdminRole: when true, sets casbin_roles=["role:serveradmin"] in the JWT as a
//     UI display hint for the frontend. Actual authorization enforcement uses
//     server-side Casbin policies, not JWT claims (see STORY-228).
func GenerateSimpleJWT(userID string, projects []string, addAdminRole bool) string {
	var casbinRoles []string
	if addAdminRole {
		casbinRoles = []string{"role:serveradmin"}
	}
	return GenerateTestJWT(JWTClaims{
		Subject:     userID,
		Email:       userID,
		Projects:    projects,
		CasbinRoles: casbinRoles,
	})
}

// GenerateOIDCJWT creates a JWT token with OIDC-specific claims.
// This simulates tokens issued by OIDC providers like Google, Okta, etc.

// Parameters:
//   - email: user's email address (used as subject)
//   - groups: OIDC group memberships
func GenerateOIDCJWT(email string, groups []string) string {
	return GenerateTestJWT(JWTClaims{
		Subject:  email,
		Email:    email,
		Projects: groups, // Use groups as projects for OIDC testing
		Groups:   groups,
		Issuer:   "knodex",
		Audience: "knodex-api",
	})
}

// GenerateJWTWithoutGroups creates a JWT token without the groups claim.
// This simulates OIDC providers that don't include group information.

func GenerateJWTWithoutGroups(email string) string {
	return GenerateTestJWT(JWTClaims{
		Subject:  email,
		Email:    email,
		Projects: []string{},
		// No Groups field - simulates OIDC without groups
		// No CasbinRoles - regular user
	})
}

// =============================================================================
// HTTP Request Helpers
// =============================================================================

// MakeAuthenticatedRequest creates and executes an HTTP request with JWT authentication.
// Returns the HTTP response for further inspection.
//
// Parameters:
//   - client: HTTP client to use for the request
//   - baseURL: base URL for the API (e.g., "http://localhost:8080")
//   - method: HTTP method (GET, POST, PUT, DELETE, etc.)
//   - path: API path (e.g., "/api/v1/projects")
//   - token: JWT token for authentication (can be empty for unauthenticated requests)
//   - body: request body (will be JSON encoded, can be nil)
func MakeAuthenticatedRequest(client *http.Client, baseURL, method, path, token string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return client.Do(req)
}

// MakeSimpleAuthenticatedRequest is a convenience wrapper that only requires essential parameters.
// Uses GET method with no body.
func MakeSimpleAuthenticatedRequest(client *http.Client, baseURL, path, token string) (*http.Response, error) {
	return MakeAuthenticatedRequest(client, baseURL, "GET", path, token, nil)
}
