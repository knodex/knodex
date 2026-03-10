// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package oidc

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)
	require.NotNil(t, server)
	require.NotNil(t, server.privateKey)
	require.NotNil(t, server.users)
	require.NotNil(t, server.codes)
	require.NotNil(t, server.tokens)

	// Verify default configuration
	assert.Equal(t, 8081, server.config.Port)
	assert.Equal(t, "test-client-id", server.config.ClientID)
	assert.Equal(t, "test-client-secret", server.config.ClientSecret)
}

func TestNewServerWithOptions(t *testing.T) {
	server, err := NewServer(
		WithPort(9000),
		WithIssuerURL("http://custom-issuer:9000"),
		WithClientCredentials("custom-client", "custom-secret"),
		WithTokenExpiry(2*time.Hour),
	)

	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, 9000, server.config.Port)
	assert.Equal(t, "http://custom-issuer:9000", server.config.IssuerURL)
	assert.Equal(t, "custom-client", server.config.ClientID)
	assert.Equal(t, "custom-secret", server.config.ClientSecret)
	assert.Equal(t, 2*time.Hour, server.config.TokenExpiry)
}

func TestServerStartStop(t *testing.T) {
	server, err := NewServer(WithPort(0)) // Use port 0 for random available port
	require.NoError(t, err)
	require.NotNil(t, server)

	ctx := context.Background()

	// Start the server
	err = server.Start(ctx)
	require.NoError(t, err)
	require.NotNil(t, server.httpServer)

	// Stop the server
	err = server.Stop(ctx)
	require.NoError(t, err)
}

func TestAddUser(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	user := &TestUser{
		Email:         "newuser@test.local",
		Subject:       "new-user-id",
		Name:          "New User",
		Groups:        []string{"group1", "group2"},
		EmailVerified: true,
	}

	server.AddUser(user)

	// Verify user was added
	addedUser := server.GetUser(user.Email)
	require.NotNil(t, addedUser)
	assert.Equal(t, user.Email, addedUser.Email)
	assert.Equal(t, user.Subject, addedUser.Subject)
	assert.Equal(t, user.Groups, addedUser.Groups)
}

func TestGetUser(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	// Test getting an existing default user
	adminUser := server.GetUser(AdminEmail)
	require.NotNil(t, adminUser)
	assert.Equal(t, AdminEmail, adminUser.Email)

	// Test getting non-existent user
	notFound := server.GetUser("nonexistent@test.local")
	assert.Nil(t, notFound)
}

func TestGenerateAuthCode(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	// Generate auth code for existing user
	code, err := server.GenerateAuthCode(AdminEmail, "http://localhost:8080/callback", "test-state", "test-nonce")
	require.NoError(t, err)
	require.NotEmpty(t, code)

	// Verify code was stored
	server.mu.RLock()
	storedCode, exists := server.codes[code]
	server.mu.RUnlock()

	require.True(t, exists)
	assert.Equal(t, AdminEmail, storedCode.user.Email)
	assert.Equal(t, "http://localhost:8080/callback", storedCode.redirectURI)
	assert.Equal(t, "test-state", storedCode.state)
	assert.Equal(t, "test-nonce", storedCode.nonce)
}

func TestGenerateAuthCodeNonexistentUser(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	// Try to generate auth code for non-existent user
	_, err = server.GenerateAuthCode("nonexistent@test.local", "http://localhost:8080/callback", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestDefaultTestUsers(t *testing.T) {
	users := DefaultTestUsers()
	require.NotEmpty(t, users)

	// Verify expected test users exist
	expectedEmails := []string{
		AdminEmail,
		DeveloperEmail,
		ViewerEmail,
		NoGroupsEmail,
		ExpiredEmail,
		UnverifiedEmail,
		InvalidEmail,
		MultiEmail,
		PlatformAdminEmail,
	}

	userEmails := make(map[string]bool)
	for _, user := range users {
		userEmails[user.Email] = true
	}

	for _, email := range expectedEmails {
		assert.True(t, userEmails[email], "Expected user %s not found", email)
	}
}

func TestScenarioExpiredTokens(t *testing.T) {
	server, err := NewServer(
		WithScenarios(&ScenarioConfig{
			ExpiredTokens: true,
		}),
	)
	require.NoError(t, err)

	// Verify the scenario is set
	require.NotNil(t, server.config.Scenarios)
	assert.True(t, server.config.Scenarios.ExpiredTokens)
}

func TestScenarioRejectAllTokens(t *testing.T) {
	server, err := NewServer(
		WithScenarios(&ScenarioConfig{
			RejectAllTokens: true,
		}),
	)
	require.NoError(t, err)

	// Verify the scenario is set
	require.NotNil(t, server.config.Scenarios)
	assert.True(t, server.config.Scenarios.RejectAllTokens)
}

func TestConcurrentUserAccess(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	// Test concurrent user operations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			user := &TestUser{
				Email:         fmt.Sprintf("concurrent%d@test.local", idx),
				Subject:       fmt.Sprintf("concurrent-user-%d", idx),
				EmailVerified: true,
			}
			server.AddUser(user)

			// Verify user can be retrieved
			retrieved := server.GetUser(user.Email)
			assert.NotNil(t, retrieved)
			assert.Equal(t, user.Email, retrieved.Email)

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConcurrentAuthCodeGeneration(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	// Test concurrent auth code generation
	done := make(chan bool)
	codes := make(chan string, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			code, err := server.GenerateAuthCode(AdminEmail, "http://localhost:8080/callback", fmt.Sprintf("state-%d", idx), "")
			assert.NoError(t, err)
			assert.NotEmpty(t, code)
			codes <- code
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	close(codes)

	// Verify all codes are unique
	codeSet := make(map[string]bool)
	for code := range codes {
		assert.False(t, codeSet[code], "Duplicate code found")
		codeSet[code] = true
	}
}

// Integration tests that require a running server

// waitForServerReady polls the health endpoint until the server is ready or timeout.
// This replaces flaky time.Sleep() calls that can fail on slow CI.
func waitForServerReady(t *testing.T, baseURL string) {
	t.Helper()

	healthURL := baseURL + "/healthz"
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("server at %s failed to become ready within 5 seconds", baseURL)
}

func setupTestServer(t *testing.T, opts ...Option) (*Server, func()) {
	t.Helper()

	// Use a random available port
	defaultOpts := []Option{WithPort(0)}
	opts = append(defaultOpts, opts...)

	server, err := NewServer(opts...)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)

	cleanup := func() {
		server.Stop(ctx)
	}

	return server, cleanup
}

func TestHealthEndpointIntegration(t *testing.T) {
	// Create server on a known port for integration testing
	server, err := NewServer(WithPort(18081))
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18081")

	resp, err := http.Get("http://localhost:18081/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
}

func TestDiscoveryEndpointIntegration(t *testing.T) {
	server, err := NewServer(
		WithPort(18082),
		WithIssuerURL("http://localhost:18082"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18082")

	resp, err := http.Get("http://localhost:18082/.well-known/openid-configuration")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var config map[string]interface{}
	err = json.Unmarshal(body, &config)
	require.NoError(t, err)

	// Verify required OIDC fields
	assert.Equal(t, "http://localhost:18082", config["issuer"])
	assert.Contains(t, config["authorization_endpoint"], "/authorize")
	assert.Contains(t, config["token_endpoint"], "/token")
	assert.Contains(t, config["jwks_uri"], "/jwks")
	assert.Contains(t, config["userinfo_endpoint"], "/userinfo")
}

func TestJWKSEndpointIntegration(t *testing.T) {
	server, err := NewServer(WithPort(18083))
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18083")

	resp, err := http.Get("http://localhost:18083/jwks")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var jwks map[string]interface{}
	err = json.Unmarshal(body, &jwks)
	require.NoError(t, err)

	keys := jwks["keys"].([]interface{})
	require.Len(t, keys, 1)

	key := keys[0].(map[string]interface{})
	assert.Equal(t, "RSA", key["kty"])
	assert.Equal(t, "sig", key["use"])
	assert.Equal(t, "RS256", key["alg"])
	assert.NotEmpty(t, key["kid"])
	assert.NotEmpty(t, key["n"])
	assert.NotEmpty(t, key["e"])
}

func TestAuthorizeEndpointIntegration(t *testing.T) {
	server, err := NewServer(
		WithPort(18084),
		WithIssuerURL("http://localhost:18084"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18084")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	params := url.Values{
		"client_id":     {"test-client-id"},
		"redirect_uri":  {"http://localhost:8080/callback"},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {"test-state"},
		"login_hint":    {AdminEmail},
	}

	resp, err := client.Get("http://localhost:18084/authorize?" + params.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	require.NotEmpty(t, location)

	locURL, err := url.Parse(location)
	require.NoError(t, err)

	// Should contain auth code and state
	assert.NotEmpty(t, locURL.Query().Get("code"))
	assert.Equal(t, "test-state", locURL.Query().Get("state"))
}

func TestAuthorizeEndpointInvalidClient(t *testing.T) {
	server, err := NewServer(WithPort(18085))
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18085")

	params := url.Values{
		"client_id":     {"wrong-client"},
		"redirect_uri":  {"http://localhost:8080/callback"},
		"response_type": {"code"},
	}

	resp, err := http.Get("http://localhost:18085/authorize?" + params.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestTokenEndpointIntegration(t *testing.T) {
	server, err := NewServer(
		WithPort(18086),
		WithIssuerURL("http://localhost:18086"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18086")

	// Generate auth code
	code, err := server.GenerateAuthCode(AdminEmail, "http://localhost:8080/callback", "", "test-nonce")
	require.NoError(t, err)

	// Exchange code for tokens
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:8080/callback"},
		"client_id":     {"test-client-id"},
		"client_secret": {"test-client-secret"},
	}

	resp, err := http.Post("http://localhost:18086/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var tokenResp map[string]interface{}
	err = json.Unmarshal(body, &tokenResp)
	require.NoError(t, err)

	assert.NotEmpty(t, tokenResp["access_token"])
	assert.NotEmpty(t, tokenResp["id_token"])
	assert.Equal(t, "Bearer", tokenResp["token_type"])
	assert.NotNil(t, tokenResp["expires_in"])

	// Verify the ID token can be parsed
	idToken := tokenResp["id_token"].(string)
	token, err := jwt.Parse(idToken, func(token *jwt.Token) (interface{}, error) {
		return &server.privateKey.PublicKey, nil
	})
	require.NoError(t, err)
	assert.True(t, token.Valid)

	claims := token.Claims.(jwt.MapClaims)
	assert.Equal(t, AdminEmail, claims["email"])
}

func TestTokenEndpointInvalidCode(t *testing.T) {
	server, err := NewServer(WithPort(18087))
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18087")

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"invalid-code"},
		"redirect_uri":  {"http://localhost:8080/callback"},
		"client_id":     {"test-client-id"},
		"client_secret": {"test-client-secret"},
	}

	resp, err := http.Post("http://localhost:18087/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTokenEndpointRejectAllTokensScenario(t *testing.T) {
	server, err := NewServer(
		WithPort(18088),
		WithScenarios(&ScenarioConfig{RejectAllTokens: true}),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18088")

	// Generate auth code
	code, err := server.GenerateAuthCode(AdminEmail, "http://localhost:8080/callback", "", "")
	require.NoError(t, err)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:8080/callback"},
		"client_id":     {"test-client-id"},
		"client_secret": {"test-client-secret"},
	}

	resp, err := http.Post("http://localhost:18088/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestUserInfoEndpointIntegration(t *testing.T) {
	server, err := NewServer(
		WithPort(18089),
		WithIssuerURL("http://localhost:18089"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18089")

	// Get access token
	code, err := server.GenerateAuthCode(AdminEmail, "http://localhost:8080/callback", "", "")
	require.NoError(t, err)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {"http://localhost:8080/callback"},
		"client_id":     {"test-client-id"},
		"client_secret": {"test-client-secret"},
	}

	tokenResp, err := http.Post("http://localhost:18089/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer tokenResp.Body.Close()

	var tokens map[string]interface{}
	json.NewDecoder(tokenResp.Body).Decode(&tokens)
	accessToken := tokens["access_token"].(string)

	// Request userinfo
	req, err := http.NewRequest("GET", "http://localhost:18089/userinfo", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var userInfo map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&userInfo)

	assert.NotEmpty(t, userInfo["sub"])
	assert.Equal(t, AdminEmail, userInfo["email"])
	assert.True(t, userInfo["email_verified"].(bool))
}

func TestUserInfoEndpointInvalidToken(t *testing.T) {
	server, err := NewServer(WithPort(18090))
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18090")

	req, err := http.NewRequest("GET", "http://localhost:18090/userinfo", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer invalid-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestUserInfoEndpointMissingAuth(t *testing.T) {
	server, err := NewServer(WithPort(18091))
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	waitForServerReady(t, "http://localhost:18091")

	resp, err := http.Get("http://localhost:18091/userinfo")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestPrivateKeyAccess(t *testing.T) {
	server, err := NewServer()
	require.NoError(t, err)

	// Verify private key is accessible for testing
	assert.NotNil(t, server.privateKey)
	assert.IsType(t, &rsa.PrivateKey{}, server.privateKey)
}

func TestIssuerURL(t *testing.T) {
	server, err := NewServer(
		WithPort(18092),
		WithIssuerURL("http://custom-issuer:8081"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = server.Start(ctx)
	require.NoError(t, err)
	defer server.Stop(ctx)

	assert.Equal(t, "http://custom-issuer:8081", server.IssuerURL())
}
