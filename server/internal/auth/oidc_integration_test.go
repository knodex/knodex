// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/knodex/knodex/server/internal/config"
	mockoidc "github.com/knodex/knodex/server/test/mocks/oidc"
)

// mockAuthServiceForIntegration implements ServiceInterface for integration testing
// Updated to match new interface (uses GenerateTokenWithGroups)
type mockAuthServiceForIntegration struct {
	generateTokenWithGroupsFunc func(userID, email, displayName string, groups []string) (string, time.Time, error)
	localLoginEnabled           bool // configurable; default false suits OIDC integration tests
}

func (m *mockAuthServiceForIntegration) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*LoginResponse, error) {
	return nil, nil
}

func (m *mockAuthServiceForIntegration) GenerateTokenForAccount(account *Account, userID string) (string, time.Time, error) {
	return "mock-token", time.Now().Add(1 * time.Hour), nil
}

func (m *mockAuthServiceForIntegration) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	if m.generateTokenWithGroupsFunc != nil {
		return m.generateTokenWithGroupsFunc(userID, email, displayName, groups)
	}
	return "mock-token", time.Now().Add(1 * time.Hour), nil
}

func (m *mockAuthServiceForIntegration) ValidateToken(_ context.Context, tokenString string) (*JWTClaims, error) {
	return nil, nil
}

func (m *mockAuthServiceForIntegration) RevokeToken(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockAuthServiceForIntegration) IsLocalLoginEnabled() bool { return m.localLoginEnabled }

// MockOIDCProvider is a mock OIDC provider for integration testing
type MockOIDCProvider struct {
	server      *httptest.Server
	privateKey  *rsa.PrivateKey
	issuerURL   string
	clientID    string
	redirectURI string
	codes       map[string]*mockAuthCode
}

type mockAuthCode struct {
	email        string
	sub          string
	name         string
	verified     bool
	groups       []string
	nonce        string // OIDC nonce echoed back in ID token
	omitVerified bool   // when true, email_verified claim is omitted from ID token
}

// NewMockOIDCProvider creates a new mock OIDC provider
func NewMockOIDCProvider() (*MockOIDCProvider, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	mock := &MockOIDCProvider{
		privateKey:  privateKey,
		clientID:    "test-client-id",
		redirectURI: "http://localhost:8080/callback",
		codes:       make(map[string]*mockAuthCode),
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// OIDC Discovery endpoint
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		config := map[string]interface{}{
			"issuer":                 mock.issuerURL,
			"authorization_endpoint": mock.issuerURL + "/authorize",
			"token_endpoint":         mock.issuerURL + "/token",
			"jwks_uri":               mock.issuerURL + "/jwks",
			"userinfo_endpoint":      mock.issuerURL + "/userinfo",
			"response_types_supported": []string{
				"code",
				"token",
				"id_token",
			},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		json.NewEncoder(w).Encode(config)
	})

	// Authorization endpoint (not used in tests, but needed for completeness)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not implemented in mock", http.StatusNotImplemented)
	})

	// Token endpoint
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		if grantType != "authorization_code" {
			http.Error(w, "Invalid grant_type", http.StatusBadRequest)
			return
		}

		code := r.FormValue("code")
		authCode, ok := mock.codes[code]
		if !ok {
			http.Error(w, "Invalid authorization code", http.StatusBadRequest)
			return
		}

		// Generate ID token
		idToken, err := mock.generateIDToken(authCode)
		if err != nil {
			http.Error(w, "Failed to generate ID token", http.StatusInternalServerError)
			return
		}

		// Delete used code
		delete(mock.codes, code)

		response := map[string]interface{}{
			"access_token":  "mock-access-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"id_token":      idToken,
			"refresh_token": "mock-refresh-token",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// JWKS endpoint
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwks := map[string]interface{}{
			"keys": []map[string]interface{}{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "test-key-id",
					"n":   base64URLEncode(privateKey.PublicKey.N.Bytes()),
					"e":   base64URLEncode(bigIntToBytes(int64(privateKey.PublicKey.E))),
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	})

	mock.server = httptest.NewServer(mux)
	mock.issuerURL = mock.server.URL

	return mock, nil
}

// Close shuts down the mock OIDC provider
func (m *MockOIDCProvider) Close() {
	m.server.Close()
}

// AddAuthCodeWithNonce adds an authorization code with a nonce to echo in the ID token
func (m *MockOIDCProvider) AddAuthCodeWithNonce(code, email, sub, name string, verified bool, nonce string, groups ...string) {
	m.codes[code] = &mockAuthCode{
		email:    email,
		sub:      sub,
		name:     name,
		verified: verified,
		nonce:    nonce,
		groups:   groups,
	}
}

// generateIDToken generates a mock ID token
func (m *MockOIDCProvider) generateIDToken(authCode *mockAuthCode) (string, error) {
	now := time.Now()

	claims := jwt.MapClaims{
		"iss":   m.issuerURL,
		"sub":   authCode.sub,
		"aud":   m.clientID,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"email": authCode.email,
		"name":  authCode.name,
	}
	if !authCode.omitVerified {
		claims["email_verified"] = authCode.verified
	}

	// Add groups claim if provided
	if len(authCode.groups) > 0 {
		claims["groups"] = authCode.groups
	}

	// Add nonce claim if provided (echoed back from authorization request)
	if authCode.nonce != "" {
		claims["nonce"] = authCode.nonce
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key-id"

	return token.SignedString(m.privateKey)
}

// Helper functions
func base64URLEncode(data []byte) string {
	// First base64 encode, then convert to URL-safe format
	encoded := base64.StdEncoding.EncodeToString(data)
	return strings.TrimRight(
		strings.ReplaceAll(
			strings.ReplaceAll(
				encoded,
				"+", "-"),
			"/", "_"),
		"=")
}

func bigIntToBytes(n int64) []byte {
	bytes := make([]byte, 4)
	bytes[0] = byte(n >> 24)
	bytes[1] = byte(n >> 16)
	bytes[2] = byte(n >> 8)
	bytes[3] = byte(n)
	return bytes
}

// newTestOIDCService creates an OIDCService for tests. The production HTTP client
// now allows loopback, so no override is required.
func newTestOIDCService(
	cfg *Config,
	redisClient RedisClient,
	authService ServiceInterface,
	provisioningService *OIDCProvisioningService,
	roleManager AuthRoleManager,
	rolePersister RolePersister,
) (*OIDCService, error) {
	return NewOIDCService(cfg, redisClient, authService, provisioningService, roleManager, rolePersister)
}

// TestOIDCIntegration_FullFlow tests the complete OIDC flow with a mock provider
// Updated to use OIDCProvisioningService (no User CRD persistence)
func TestOIDCIntegration_FullFlow(t *testing.T) {
	// Skip if in short test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock OIDC provider
	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	// Create mock dependencies
	redisClient := NewMockRedisClient()

	// Use mockAuthServiceForIntegration with GenerateTokenWithGroups
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}

	// Create OIDC provisioning service (no User CRD)
	provisioningService, _ := createTestOIDCProvisioningService()

	// Create OIDC service with mock provider
	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
				Scopes:       []string{"openid", "email", "profile"},
			},
		},
	}

	// NewOIDCService no longer takes userService parameter
	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Step 1: Generate state token, nonce, and PKCE verifier
	state, nonce, verifier, err := svc.GenerateStateToken(ctx, "mock", "http://localhost:3000/auth/callback")
	if err != nil {
		t.Fatalf("GenerateStateToken() failed: %v", err)
	}

	// Step 2: Get authorization URL (includes nonce parameter + S256 PKCE challenge)
	authURL, err := svc.GetAuthCodeURL("mock", state, nonce, verifier)
	if err != nil {
		t.Fatalf("GetAuthCodeURL() failed: %v", err)
	}

	if !strings.Contains(authURL, mockProvider.issuerURL) {
		t.Errorf("Authorization URL does not contain issuer URL: %s", authURL)
	}

	// Step 3: Simulate authorization code from provider (nonce echoed in ID token)
	authCode := "test-auth-code-12345"
	mockProvider.AddAuthCodeWithNonce(authCode, "test@example.com", "test-subject", "Test User", true, nonce)

	// Step 4: Validate state token (as callback would do)
	provider, redirectURL, err := svc.ValidateStateToken(ctx, state)
	if err != nil {
		t.Fatalf("ValidateStateToken() failed: %v", err)
	}
	if redirectURL == "" {
		t.Error("ValidateStateToken() returned empty redirectURL")
	}
	if provider != "mock" {
		t.Fatalf("ValidateStateToken() returned provider %s, want mock", provider)
	}

	// Step 5: Retrieve nonce from Redis (as callback handler would do)
	storedNonce, err := redisClient.GetDel(ctx, NoncePrefix+state).Result()
	if err != nil {
		t.Fatalf("Failed to retrieve nonce from Redis: %v", err)
	}

	// Step 6: Retrieve PKCE verifier from Redis (as callback handler would do)
	storedVerifier, err := redisClient.GetDel(ctx, PKCEVerifierPrefix+state).Result()
	if err != nil {
		t.Fatalf("Failed to retrieve PKCE verifier from Redis: %v", err)
	}

	// Step 7: Exchange code for token (with nonce + PKCE validation)
	loginResp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, storedNonce, storedVerifier)
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() failed: %v", err)
	}

	// Verify login response
	if loginResp.Token == "" {
		t.Errorf("Token is empty, expected a token to be generated")
	}

	if loginResp.User.Email != "test@example.com" {
		t.Errorf("User email = %s, want test@example.com", loginResp.User.Email)
	}

	// Verify user was provisioned (ID should be generated, not empty)
	if loginResp.User.ID == "" {
		t.Errorf("User ID is empty, expected a generated user ID")
	}

	// Note: As of commit 9992c2a, personal project creation is prevented.
	// New OIDC users are created WITHOUT a default project.
	// They will only be added to a project if the system-wide default project already exists.

	// Verify user was created without project (no personal project creation)
	if loginResp.User.DefaultProject != "" {
		t.Errorf("User DefaultProject = %s, expected empty (no personal project creation)", loginResp.User.DefaultProject)
	}

	// Verify user has no projects (unless default project exists, which it doesn't in this test)
	if len(loginResp.User.Projects) != 0 {
		t.Errorf("User Projects = %v, expected empty (no personal project creation)", loginResp.User.Projects)
	}
}

// TestOIDCIntegration_AuthURLContainsNonce verifies AC-1: the authorization URL includes the nonce parameter.
func TestOIDCIntegration_AuthURLContainsNonce(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Generate state, nonce, and PKCE verifier
	state, nonce, verifier, err := svc.GenerateStateToken(ctx, "mock", "")
	if err != nil {
		t.Fatalf("GenerateStateToken() failed: %v", err)
	}

	// Get authorization URL
	authURL, err := svc.GetAuthCodeURL("mock", state, nonce, verifier)
	if err != nil {
		t.Fatalf("GetAuthCodeURL() failed: %v", err)
	}

	// Verify nonce parameter is in the URL
	if !strings.Contains(authURL, "nonce=") {
		t.Errorf("authorization URL does not contain nonce parameter: %s", authURL)
	}

	// Verify the nonce value in the URL matches what we passed
	// Parse URL to handle URL-encoding (e.g., base64 '=' encoded as '%3D')
	parsedURL, parseErr := url.Parse(authURL)
	if parseErr != nil {
		t.Fatalf("failed to parse authorization URL: %v", parseErr)
	}
	if got := parsedURL.Query().Get("nonce"); got != nonce {
		t.Errorf("authorization URL nonce value doesn't match: got=%s, expected=%s", got, nonce)
	}

	// Verify state parameter is also present
	if !strings.Contains(authURL, "state=") {
		t.Errorf("authorization URL does not contain state parameter: %s", authURL)
	}

	// Verify PKCE code_challenge is present (AC-2: S256 always sent)
	challenge := parsedURL.Query().Get("code_challenge")
	if challenge == "" {
		t.Errorf("authorization URL is missing code_challenge parameter: %s", authURL)
	}
	// S256 of a 32-byte verifier base64url-encodes to 43 chars (no padding)
	if len(challenge) < 43 {
		t.Errorf("code_challenge too short (got %d chars, want ≥43): %s", len(challenge), challenge)
	}
	if got := parsedURL.Query().Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
}

// TestOIDCIntegration_InvalidCode tests token exchange with invalid code
// Updated to use OIDCProvisioningService (no User CRD)
func TestOIDCIntegration_InvalidCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Try to exchange invalid code
	_, err = svc.ExchangeCodeForToken(ctx, "mock", "invalid-code", "some-nonce", "")
	if err == nil {
		t.Error("ExchangeCodeForToken() with invalid code should fail")
	}
}

// TestOIDCIntegration_EmailNotVerified tests that unverified emails are rejected
func TestOIDCIntegration_EmailNotVerified(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Add auth code with email_verified=false and a nonce
	authCode := "unverified-email-code"
	testNonce := "test-nonce-unverified"
	mockProvider.AddAuthCodeWithNonce(authCode, "unverified@example.com", "unverified-sub", "Unverified User", false, testNonce)

	// Exchange should fail because email is not verified
	_, err = svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err == nil {
		t.Error("ExchangeCodeForToken() with unverified email should fail")
	} else if !strings.Contains(err.Error(), "email address has not been verified by the identity provider") {
		t.Errorf("ExchangeCodeForToken() error = %v, want error containing 'email address has not been verified by the identity provider'", err)
	}
}

// TestOIDCIntegration_EmailVerifiedOmitted tests that tokens from IdPs that do not
// include the email_verified claim at all are accepted (not rejected).
// Some providers (e.g., certain Azure AD, GitHub OIDC) omit the claim entirely.
func TestOIDCIntegration_EmailVerifiedOmitted(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "mock-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Add auth code WITHOUT email_verified claim (omitVerified=true)
	authCode := "no-verified-claim-code"
	testNonce := "test-nonce-omitted"
	mockProvider.codes[authCode] = &mockAuthCode{
		email:        "user@corporate-idp.com",
		sub:          "corporate-sub",
		name:         "Corporate User",
		verified:     false, // irrelevant since omitVerified=true
		omitVerified: true,
		nonce:        testNonce,
	}

	// Exchange should succeed — absent email_verified is NOT the same as false
	resp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() with omitted email_verified should succeed, got error: %v", err)
	}
	if resp == nil {
		t.Fatal("ExchangeCodeForToken() returned nil response")
	}
}

// TestOIDCIntegration_UserProvisioning tests that OIDC users are evaluated (not persisted)
// OIDC users are ephemeral - evaluated for group/project membership but not stored in CRD
func TestOIDCIntegration_UserProvisioning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()

	// Use mockAuthServiceForIntegration and OIDCProvisioningService
	authService := &mockAuthServiceForIntegration{}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Add auth code for a new user with nonce
	authCode := "test-code"
	testNonce := "test-nonce-provisioning"
	mockProvider.AddAuthCodeWithNonce(authCode, "newuser@example.com", "new-subject", "New User", true, testNonce)

	// Exchange code - should succeed and provision new user WITHOUT personal project
	loginResp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() failed: %v (user provisioning should have created the user)", err)
	}

	// Verify user was provisioned
	if loginResp.User.Email != "newuser@example.com" {
		t.Errorf("User email = %s, want newuser@example.com", loginResp.User.Email)
	}

	if loginResp.User.ID == "" {
		t.Errorf("User ID is empty, expected auto-generated ID")
	}

	// Note: As of commit 9992c2a, personal project creation is prevented.
	// Verify user was created without project (no personal project creation)
	if loginResp.User.DefaultProject != "" {
		t.Errorf("DefaultProject = %s, expected empty (no personal project creation)", loginResp.User.DefaultProject)
	}

	if len(loginResp.User.Projects) != 0 {
		t.Errorf("Projects = %v, expected empty (no personal project creation)", loginResp.User.Projects)
	}
}

// TestOIDCIntegration_ListProviders tests provider listing
// Updated to use OIDCProvisioningService
func TestOIDCIntegration_ListProviders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock1",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     "client1",
				ClientSecret: "secret1",
				RedirectURL:  mockProvider.redirectURI,
			},
			{
				Name:         "mock2",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     "client2",
				ClientSecret: "secret2",
				RedirectURL:  mockProvider.redirectURI,
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	providers := svc.ListProviders()

	if len(providers) != 2 {
		t.Errorf("ListProviders() = %d providers, want 2", len(providers))
	}

	// Check that both providers are present
	providerMap := make(map[string]bool)
	for _, p := range providers {
		providerMap[p] = true
	}

	if !providerMap["mock1"] {
		t.Error("ListProviders() missing mock1")
	}
	if !providerMap["mock2"] {
		t.Error("ListProviders() missing mock2")
	}
}

// TestOIDCIntegration_GlobalAdmin tests end-to-end OIDC login with Global Admin group mapping
// This test verifies AC11: Integration test for Global Admin via OIDC
// Updated to use OIDCProvisioningService with GroupMapper
func TestOIDCIntegration_GlobalAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock OIDC provider
	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()

	// Use mockAuthServiceForIntegration
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}

	// Configure group mappings with globalAdmin: true
	groupMappings := []config.OIDCGroupMapping{
		{
			Group:       "platform-admins",
			GlobalAdmin: true, // Global Admin mapping
		},
		{
			Group:   "engineering",
			Project: "engineering-project",
			Role:    "developer",
		},
	}

	// Create OIDC provisioning service with group mapper and shared Casbin enforcer
	groupMapper := NewGroupMapper(groupMappings)
	provisioningService, casbinEnforcer := createTestOIDCProvisioningServiceWithMapperAndEnforcer(groupMapper)

	// Create OIDC service with mock provider
	oidcConfig := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
				Scopes:       []string{"openid", "email", "profile", "groups"},
			},
		},
	}

	// Pass the same casbinEnforcer so OIDCService can retrieve roles assigned by provisioning service
	svc, err := newTestOIDCService(oidcConfig, redisClient, authService, provisioningService, casbinEnforcer, nil)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Simulate OIDC login with user in platform-admins group
	authCode := "test-global-admin-code"
	testNonce := "test-nonce-global-admin"
	mockProvider.AddAuthCodeWithNonce(
		authCode,
		"admin@example.com",
		"global-admin-subject",
		"Global Admin User",
		true,
		testNonce,
		"platform-admins", // User is in globalAdmin group
		"engineering",     // User is also in engineering group
	)

	// Exchange code for token - should provision user as Global Admin
	loginResp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() failed: %v", err)
	}

	// AC11: Verify user is provisioned as Global Admin via Casbin roles
	hasGlobalAdminRole := false
	for _, role := range loginResp.User.CasbinRoles {
		if role == "role:serveradmin" {
			hasGlobalAdminRole = true
			break
		}
	}
	if !hasGlobalAdminRole {
		t.Error("Expected user to have role:serveradmin in CasbinRoles from OIDC group mapping")
	}

	// Verify user email
	if loginResp.User.Email != "admin@example.com" {
		t.Errorf("User email = %s, want admin@example.com", loginResp.User.Email)
	}

	// Verify user has a token
	if loginResp.Token == "" {
		t.Error("Token is empty, expected JWT token to be generated")
	}

	// AC6: Verify user can be Global Admin AND have project memberships
	// (User was in both platform-admins and engineering groups)
	// Note: Project membership depends on project existing
	// In this test, we're primarily verifying Global Admin flag is set

	// AC11: Verify CasbinRoles contains role:serveradmin
	// The loginResp.User.CasbinRoles comes from the provisioning service
	// which set it based on the group mapping via Casbin
	hasRole := false
	for _, role := range loginResp.User.CasbinRoles {
		if role == "role:serveradmin" {
			hasRole = true
			break
		}
	}
	if !hasRole {
		t.Error("User.CasbinRoles should contain role:serveradmin for user in globalAdmin group")
	}

	// Verify user has an ID (was successfully created)
	if loginResp.User.ID == "" {
		t.Error("User ID should not be empty")
	}

	// The log output verifies:
	// - Global Admin privilege was granted (WARN: SECURITY: Global admin privilege granted)
	// - Source is tracked (source=oidc-group-mapping)
	// - User was provisioned (INFO: provisioned new user from OIDC login)
	// This integration test verifies the end-to-end flow from OIDC login to Global Admin grant

	// AC3, AC4, AC5: Verify Global Admin has all permissions
	// This would require a PermissionService instance, which is tested in permission_service_test.go
	// The provisioning assigns role:serveradmin via Casbin, which is the single source of truth for authorization

	// AC7, AC8: Audit logging is verified by the presence of logs during provisioning
	// The provisioning.go code logs: "SECURITY: Global admin privilege granted" with source="oidc-group-mapping"
	// This is visible in test output when running with -v flag
}

// mockRolePersister records calls to AssignUserRoles for test verification
type mockRolePersister struct {
	mu          sync.Mutex
	calls       []rolePersisterCall
	errToReturn error
}

type rolePersisterCall struct {
	user  string
	roles []string
}

func (m *mockRolePersister) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, rolePersisterCall{user: user, roles: roles})
	return m.errToReturn
}

func (m *mockRolePersister) getCalls() []rolePersisterCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]rolePersisterCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// TestOIDCIntegration_NonceValidation tests nonce validation scenarios in ExchangeCodeForToken.
// Covers AC-3: mismatched, missing, and empty nonce in ID token must cause authentication failure.
func TestOIDCIntegration_NonceValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name        string
		storedNonce string // nonce passed to ExchangeCodeForToken (from Redis)
		tokenNonce  string // nonce echoed in ID token by mock provider
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid matching nonce",
			storedNonce: "valid-nonce-abc123",
			tokenNonce:  "valid-nonce-abc123",
			wantErr:     false,
		},
		{
			name:        "mismatched nonce",
			storedNonce: "server-nonce",
			tokenNonce:  "different-nonce",
			wantErr:     true,
			errContains: "nonce validation failed",
		},
		{
			name:        "missing nonce in token (empty string)",
			storedNonce: "server-nonce",
			tokenNonce:  "", // mock provider will omit nonce claim
			wantErr:     true,
			errContains: "nonce validation failed",
		},
		{
			name:        "empty stored nonce (expired/consumed from Redis)",
			storedNonce: "", // simulates nonce expired in Redis — handler passes empty string
			tokenNonce:  "any-nonce-from-idp",
			wantErr:     true,
			errContains: "nonce validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProvider, err := NewMockOIDCProvider()
			if err != nil {
				t.Fatalf("Failed to create mock OIDC provider: %v", err)
			}
			defer mockProvider.Close()

			redisClient := NewMockRedisClient()
			authService := &mockAuthServiceForIntegration{
				generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
					return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
				},
			}
			provisioningService, _ := createTestOIDCProvisioningService()

			cfg := &Config{
				OIDCEnabled: true,
				OIDCProviders: []OIDCProviderConfig{
					{
						Name:         "mock",
						IssuerURL:    mockProvider.issuerURL,
						ClientID:     mockProvider.clientID,
						ClientSecret: "test-secret",
						RedirectURL:  mockProvider.redirectURI,
					},
				},
			}

			svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
			if err != nil {
				t.Fatalf("Failed to create OIDC service: %v", err)
			}

			ctx := context.Background()

			// Add auth code with the token nonce (what the mock provider echoes in ID token)
			authCode := "nonce-test-code-" + tt.name
			mockProvider.AddAuthCodeWithNonce(authCode, "nonce-test@example.com", "nonce-sub", "Nonce Test User", true, tt.tokenNonce)

			// Call ExchangeCodeForToken with the stored nonce (what the server had in Redis)
			_, err = svc.ExchangeCodeForToken(ctx, "mock", authCode, tt.storedNonce, "")

			if tt.wantErr {
				if err == nil {
					t.Error("ExchangeCodeForToken() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ExchangeCodeForToken() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ExchangeCodeForToken() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestOIDCIntegration_RolePersistence verifies that ExchangeCodeForToken persists
// OIDC roles to Redis via the RolePersister interface (cold-start fix).
func TestOIDCIntegration_RolePersistence(t *testing.T) {
	// Create mock OIDC provider
	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}

	groupMappings := []config.OIDCGroupMapping{
		{
			Group:       "platform-admins",
			GlobalAdmin: true,
		},
	}

	groupMapper := NewGroupMapper(groupMappings)
	provisioningService, casbinEnforcer := createTestOIDCProvisioningServiceWithMapperAndEnforcer(groupMapper)

	// Create a mock RolePersister to verify calls
	persister := &mockRolePersister{}

	oidcConfig := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
				Scopes:       []string{"openid", "email", "profile", "groups"},
			},
		},
	}

	// Pass the mock persister as the RolePersister
	svc, err := newTestOIDCService(oidcConfig, redisClient, authService, provisioningService, casbinEnforcer, persister)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// Simulate OIDC login with a global admin user
	authCode := "test-persist-code"
	testNonce := "test-nonce-persist"
	mockProvider.AddAuthCodeWithNonce(
		authCode,
		"persist@example.com",
		"persist-subject",
		"Persist User",
		true,
		testNonce,
		"platform-admins",
	)

	loginResp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() failed: %v", err)
	}

	// Verify login succeeded
	if loginResp.Token == "" {
		t.Error("Token is empty")
	}

	// Verify RolePersister.AssignUserRoles was called
	calls := persister.getCalls()
	if len(calls) == 0 {
		t.Fatal("RolePersister.AssignUserRoles was NOT called — roles are NOT being persisted to Redis")
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 call to AssignUserRoles, got %d", len(calls))
	}

	// Verify correct user ID was passed
	if calls[0].user == "" {
		t.Error("AssignUserRoles called with empty user ID")
	}

	// Verify role:serveradmin was persisted (from globalAdmin group mapping)
	hasServerAdmin := false
	for _, role := range calls[0].roles {
		if role == "role:serveradmin" {
			hasServerAdmin = true
			break
		}
	}
	if !hasServerAdmin {
		t.Errorf("Expected role:serveradmin to be persisted, got roles: %v", calls[0].roles)
	}
}

// TestOIDCIntegration_RolePersistenceFailure verifies that ExchangeCodeForToken
// succeeds even when role persistence fails (non-fatal degradation).
func TestOIDCIntegration_RolePersistenceFailure(t *testing.T) {
	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}

	groupMappings := []config.OIDCGroupMapping{
		{
			Group:       "platform-admins",
			GlobalAdmin: true,
		},
	}

	groupMapper := NewGroupMapper(groupMappings)
	provisioningService, casbinEnforcer := createTestOIDCProvisioningServiceWithMapperAndEnforcer(groupMapper)

	// Create a failing RolePersister — simulates Redis being down
	persister := &mockRolePersister{errToReturn: fmt.Errorf("redis connection refused")}

	oidcConfig := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
				Scopes:       []string{"openid", "email", "profile", "groups"},
			},
		},
	}

	svc, err := newTestOIDCService(oidcConfig, redisClient, authService, provisioningService, casbinEnforcer, persister)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	authCode := "test-fail-persist-code"
	testNonce := "test-nonce-fail-persist"
	mockProvider.AddAuthCodeWithNonce(
		authCode,
		"failpersist@example.com",
		"failpersist-subject",
		"FailPersist User",
		true,
		testNonce,
		"platform-admins",
	)

	// Login should SUCCEED even though persistence failed (non-fatal)
	loginResp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() should succeed even when role persistence fails, got error: %v", err)
	}

	if loginResp.Token == "" {
		t.Error("Token should not be empty — login should work despite persistence failure")
	}

	// Verify persistence was attempted (even though it failed)
	calls := persister.getCalls()
	if len(calls) == 0 {
		t.Error("RolePersister.AssignUserRoles was not called — persistence should be attempted even if it fails")
	}
}

// TestOIDCIntegration_ProjectRolePersistence verifies that project-scoped roles
// (e.g., proj:alpha:developer) are persisted via RolePersister, not just global roles.
func TestOIDCIntegration_ProjectRolePersistence(t *testing.T) {
	mockProvider, err := NewMockOIDCProvider()
	if err != nil {
		t.Fatalf("Failed to create mock OIDC provider: %v", err)
	}
	defer mockProvider.Close()

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}

	// Configure BOTH global admin AND project role mappings
	groupMappings := []config.OIDCGroupMapping{
		{
			Group:       "platform-admins",
			GlobalAdmin: true,
		},
		{
			Group:   "alpha-developers",
			Project: "alpha",
			Role:    "developer",
		},
	}

	groupMapper := NewGroupMapper(groupMappings)
	provisioningService, casbinEnforcer := createTestOIDCProvisioningServiceWithMapperAndEnforcer(groupMapper)

	persister := &mockRolePersister{}

	oidcConfig := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:         "mock",
				IssuerURL:    mockProvider.issuerURL,
				ClientID:     mockProvider.clientID,
				ClientSecret: "test-secret",
				RedirectURL:  mockProvider.redirectURI,
				Scopes:       []string{"openid", "email", "profile", "groups"},
			},
		},
	}

	svc, err := newTestOIDCService(oidcConfig, redisClient, authService, provisioningService, casbinEnforcer, persister)
	if err != nil {
		t.Fatalf("Failed to create OIDC service: %v", err)
	}

	ctx := context.Background()

	// User belongs to alpha-developers (project role) but NOT platform-admins
	authCode := "test-project-persist-code"
	testNonce := "test-nonce-project-persist"
	mockProvider.AddAuthCodeWithNonce(
		authCode,
		"dev@example.com",
		"dev-subject",
		"Developer User",
		true,
		testNonce,
		"alpha-developers", // Only project group, no global admin
	)

	loginResp, err := svc.ExchangeCodeForToken(ctx, "mock", authCode, testNonce, "")
	if err != nil {
		t.Fatalf("ExchangeCodeForToken() failed: %v", err)
	}

	if loginResp.Token == "" {
		t.Error("Token is empty")
	}

	// Verify RolePersister was called with project role
	calls := persister.getCalls()
	if len(calls) == 0 {
		t.Fatal("RolePersister.AssignUserRoles was NOT called — project roles are NOT being persisted")
	}

	if len(calls) != 1 {
		t.Fatalf("Expected 1 call to AssignUserRoles, got %d", len(calls))
	}

	// Verify the project role (proj:alpha:developer) was included
	hasProjectRole := false
	for _, role := range calls[0].roles {
		if role == "proj:alpha:developer" {
			hasProjectRole = true
			break
		}
	}
	if !hasProjectRole {
		t.Errorf("Expected proj:alpha:developer to be persisted, got roles: %v", calls[0].roles)
	}

	// Verify NO global admin role was assigned (user is not in platform-admins)
	for _, role := range calls[0].roles {
		if role == "role:serveradmin" {
			t.Error("role:serveradmin should NOT be present — user is not in platform-admins group")
		}
	}
}

// getFreePort finds a free TCP port on localhost by opening a listener on :0
// and immediately closing it. There is a small race window between Close and
// the test server starting, but it is acceptable for unit tests.
func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("getFreePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

// startPublicClientMock starts a mock OIDC server configured for the public-client
// (PKCE-only) flow and registers testUser. It returns the server and its issuer URL.
func startPublicClientMock(t *testing.T, clientID string) (*mockoidc.Server, string) {
	t.Helper()
	port := getFreePort(t)
	issuerURL := fmt.Sprintf("http://localhost:%d", port)

	srv, err := mockoidc.NewServer(
		mockoidc.WithPort(port),
		mockoidc.WithIssuerURL(issuerURL),
		mockoidc.WithClientCredentials(clientID, ""),
		mockoidc.WithRedirectURL("http://localhost:8080/api/v1/auth/oidc/callback"),
		mockoidc.WithPublicClient(),
	)
	if err != nil {
		t.Fatalf("mockoidc.NewServer: %v", err)
	}

	ctx := context.Background()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("mock OIDC Start: %v", err)
	}
	t.Cleanup(func() { srv.Stop(ctx) }) //nolint:errcheck

	srv.AddUser(&mockoidc.TestUser{
		Email:         "pkce-user@example.com",
		Subject:       "pkce-user-id",
		Name:          "PKCE User",
		EmailVerified: true,
		Groups:        []string{"testers"},
	})

	return srv, issuerURL
}

// pkceS256Challenge computes the RFC 7636 §4.2 S256 code_challenge from a verifier.
func pkceS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// TestOIDCIntegration_PublicClient_PKCE verifies the complete public-client (PKCE-only)
// flow: no client secret, S256 code_challenge bound to the authorization code, verifier
// validated at token exchange. Uses the proper mock OIDC server that validates PKCE.
func TestOIDCIntegration_PublicClient_PKCE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	const clientID = "pkce-test-client"
	srv, issuerURL := startPublicClientMock(t, clientID)

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{
		generateTokenWithGroupsFunc: func(userID, email, displayName string, groups []string) (string, time.Time, error) {
			return "test-jwt-token", time.Now().Add(1 * time.Hour), nil
		},
	}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:                    "pkce-provider",
				IssuerURL:               issuerURL,
				ClientID:                clientID,
				ClientSecret:            "",
				RedirectURL:             "http://localhost:8080/api/v1/auth/oidc/callback",
				Scopes:                  []string{"openid", "email", "profile"},
				TokenEndpointAuthMethod: "none",
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("newTestOIDCService: %v", err)
	}

	ctx := context.Background()

	// Step 1: Generate state/nonce/verifier (service stores verifier in Redis)
	state, nonce, verifier, err := svc.GenerateStateToken(ctx, "pkce-provider", "http://localhost:3000/callback")
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}

	// Step 2: Compute S256 challenge (as the client browser would for the auth request)
	challenge := pkceS256Challenge(verifier)

	// Step 3: Register auth code on mock with the real challenge
	code, err := srv.GenerateAuthCode(
		"pkce-user@example.com",
		"http://localhost:8080/api/v1/auth/oidc/callback",
		state, nonce, challenge, "S256",
	)
	if err != nil {
		t.Fatalf("GenerateAuthCode: %v", err)
	}

	// Step 4: Retrieve verifier from Redis (as the callback handler does)
	storedNonce, err := redisClient.GetDel(ctx, NoncePrefix+state).Result()
	if err != nil {
		t.Fatalf("GetDel nonce: %v", err)
	}
	storedVerifier, err := redisClient.GetDel(ctx, PKCEVerifierPrefix+state).Result()
	if err != nil {
		t.Fatalf("GetDel verifier: %v", err)
	}

	// Step 5: Exchange code — mock validates that sha256(storedVerifier) == challenge
	loginResp, err := svc.ExchangeCodeForToken(ctx, "pkce-provider", code, storedNonce, storedVerifier)
	if err != nil {
		t.Fatalf("ExchangeCodeForToken: %v", err)
	}

	if loginResp.Token == "" {
		t.Error("Token is empty — public-client PKCE flow did not produce a JWT")
	}
	if loginResp.User.Email != "pkce-user@example.com" {
		t.Errorf("User.Email = %q, want pkce-user@example.com", loginResp.User.Email)
	}
}

// TestOIDCIntegration_PKCEMismatch_FailsClosed verifies that token exchange fails
// when the code_verifier does not match the registered code_challenge (tampered flow).
func TestOIDCIntegration_PKCEMismatch_FailsClosed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	const clientID = "pkce-mismatch-client"
	srv, issuerURL := startPublicClientMock(t, clientID)

	redisClient := NewMockRedisClient()
	authService := &mockAuthServiceForIntegration{}
	provisioningService, _ := createTestOIDCProvisioningService()

	cfg := &Config{
		OIDCEnabled: true,
		OIDCProviders: []OIDCProviderConfig{
			{
				Name:                    "pkce-provider",
				IssuerURL:               issuerURL,
				ClientID:                clientID,
				ClientSecret:            "",
				RedirectURL:             "http://localhost:8080/api/v1/auth/oidc/callback",
				Scopes:                  []string{"openid", "email", "profile"},
				TokenEndpointAuthMethod: "none",
			},
		},
	}

	svc, err := newTestOIDCService(cfg, redisClient, authService, provisioningService, nil, nil)
	if err != nil {
		t.Fatalf("newTestOIDCService: %v", err)
	}

	ctx := context.Background()

	// Generate state/nonce/real verifier
	_, nonce, realVerifier, err := svc.GenerateStateToken(ctx, "pkce-provider", "")
	if err != nil {
		t.Fatalf("GenerateStateToken: %v", err)
	}

	// Register auth code with the challenge for the REAL verifier
	challenge := pkceS256Challenge(realVerifier)
	code, err := srv.GenerateAuthCode(
		"pkce-user@example.com",
		"http://localhost:8080/api/v1/auth/oidc/callback",
		"any-state", nonce, challenge, "S256",
	)
	if err != nil {
		t.Fatalf("GenerateAuthCode: %v", err)
	}

	// Exchange with a WRONG verifier — mock must reject with PKCE mismatch error
	wrongVerifier := "wrongverifier-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, err = svc.ExchangeCodeForToken(ctx, "pkce-provider", code, nonce, wrongVerifier)
	if err == nil {
		t.Fatal("ExchangeCodeForToken with mismatched PKCE verifier should fail, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid_grant") && !strings.Contains(strings.ToLower(errMsg), "pkce") {
		t.Errorf("expected PKCE mismatch error (invalid_grant or pkce), got: %v", err)
	}
}
