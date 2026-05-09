// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Server is a mock OIDC provider for testing.
// It implements the OpenID Connect Discovery, Authorization, Token,
// JWKS, and UserInfo endpoints.
type Server struct {
	config     *Config
	httpServer *http.Server
	privateKey *rsa.PrivateKey
	wrongKey   *rsa.PrivateKey // For invalid signature scenarios
	issuerURL  string
	keyID      string

	// User and auth code storage
	mu     sync.RWMutex
	users  map[string]*TestUser // email -> user
	codes  map[string]*authCode // code -> auth info
	tokens map[string]*TestUser // access_token -> user

	// Logger
	logger *slog.Logger
}

// authCode represents a pending authorization code.
type authCode struct {
	user                *TestUser
	redirectURI         string
	state               string
	nonce               string
	codeChallenge       string
	codeChallengeMethod string
	expiresAt           time.Time
}

// NewServer creates a new mock OIDC server with the given options.
func NewServer(opts ...Option) (*Server, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Generate RSA key pair if not provided
	privateKey := cfg.PrivateKey
	if privateKey == nil {
		var err error
		privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %w", err)
		}
	}

	// Generate a second key for invalid signature scenarios
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate wrong RSA key: %w", err)
	}

	s := &Server{
		config:     cfg,
		privateKey: privateKey,
		wrongKey:   wrongKey,
		keyID:      "mock-key-id",
		users:      make(map[string]*TestUser),
		codes:      make(map[string]*authCode),
		tokens:     make(map[string]*TestUser),
		logger:     slog.Default().With("component", "mock-oidc"),
	}

	// Add default test users
	for _, user := range DefaultTestUsers() {
		s.AddUser(user)
	}

	return s, nil
}

// AddUser adds a test user to the mock server.
func (s *Server) AddUser(user *TestUser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user.Email] = user
}

// GetUser retrieves a test user by email.
func (s *Server) GetUser(email string) *TestUser {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.users[email]
}

// GenerateAuthCode generates an authorization code for the given user.
// The code can be exchanged for tokens via the token endpoint.
// When codeChallenge is non-empty, the matching code_verifier must be supplied
// at the token endpoint (PKCE / RFC 7636).
func (s *Server) GenerateAuthCode(email, redirectURI, state, nonce, codeChallenge, codeChallengeMethod string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[email]
	if !ok {
		return "", fmt.Errorf("user not found: %s", email)
	}

	code := generateRandomString(32)
	s.codes[code] = &authCode{
		user:                user,
		redirectURI:         redirectURI,
		state:               state,
		nonce:               nonce,
		codeChallenge:       codeChallenge,
		codeChallengeMethod: codeChallengeMethod,
		expiresAt:           time.Now().Add(10 * time.Minute),
	}

	return code, nil
}

// Start starts the mock OIDC server on the configured port.
func (s *Server) Start(ctx context.Context) error {
	// Set issuer URL if not configured
	if s.config.IssuerURL == "" {
		s.issuerURL = fmt.Sprintf("http://localhost:%d", s.config.Port)
	} else {
		s.issuerURL = s.config.IssuerURL
	}

	mux := http.NewServeMux()

	// OIDC Discovery endpoint
	mux.HandleFunc("GET /.well-known/openid-configuration", s.handleDiscovery)

	// Authorization endpoint
	mux.HandleFunc("GET /authorize", s.handleAuthorize)

	// Token endpoint
	mux.HandleFunc("POST /token", s.handleToken)

	// JWKS endpoint
	mux.HandleFunc("GET /jwks", s.handleJWKS)
	mux.HandleFunc("GET /.well-known/jwks.json", s.handleJWKS)

	// UserInfo endpoint
	mux.HandleFunc("GET /userinfo", s.handleUserInfo)

	// Health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in background
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.logger.Info("Starting mock OIDC server",
		"addr", addr,
		"issuer", s.issuerURL,
	)

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the mock OIDC server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// IssuerURL returns the issuer URL of the mock server.
func (s *Server) IssuerURL() string {
	return s.issuerURL
}

// handleDiscovery serves the OIDC discovery document.
func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	discovery := map[string]interface{}{
		"issuer":                 s.issuerURL,
		"authorization_endpoint": s.issuerURL + "/authorize",
		"token_endpoint":         s.issuerURL + "/token",
		"jwks_uri":               s.issuerURL + "/jwks",
		"userinfo_endpoint":      s.issuerURL + "/userinfo",
		"response_types_supported": []string{
			"code",
			"token",
			"id_token",
			"code token",
			"code id_token",
			"token id_token",
			"code token id_token",
		},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "email", "profile", "groups"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic", "none"},
		"code_challenge_methods_supported":      []string{"S256"},
		"claims_supported": []string{
			"sub", "iss", "aud", "exp", "iat", "email", "email_verified",
			"name", "groups",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(discovery)
}

// handleAuthorize handles the authorization endpoint.
// In a real scenario, this would show a login form. For testing,
// it accepts the email as a query parameter and redirects with a code.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")
	nonce := r.URL.Query().Get("nonce")
	email := r.URL.Query().Get("login_hint") // Use login_hint to specify user
	codeChallenge := r.URL.Query().Get("code_challenge")
	codeChallengeMethod := r.URL.Query().Get("code_challenge_method")

	// Validate client_id
	if clientID != s.config.ClientID {
		http.Error(w, "invalid_client", http.StatusUnauthorized)
		return
	}

	// Only S256 PKCE is supported (plain is deprecated by RFC 7636 §4.2).
	if codeChallenge != "" && codeChallengeMethod != "" && codeChallengeMethod != "S256" {
		http.Error(w, "invalid_request: only S256 code_challenge_method supported", http.StatusBadRequest)
		return
	}

	// Validate redirect_uri to prevent open redirect
	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Host == "" {
		http.Error(w, "invalid_redirect_uri", http.StatusBadRequest)
		return
	}
	// Only allow localhost and internal cluster URIs (test environments)
	host := parsedRedirect.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "knodex-server" && host != "knodex-web" {
		http.Error(w, "redirect_uri_not_allowed", http.StatusBadRequest)
		return
	}

	// For testing, if no email is provided, use a default user
	if email == "" {
		email = AdminEmail
	}

	// Generate authorization code
	code, err := s.GenerateAuthCode(email, redirectURI, state, nonce, codeChallenge, codeChallengeMethod)
	if err != nil {
		s.logger.Error("Failed to generate auth code", "error", err)
		http.Error(w, "server_error", http.StatusInternalServerError)
		return
	}

	// Build redirect URL
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "invalid_redirect_uri", http.StatusBadRequest)
		return
	}

	q := redirectURL.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	redirectURL.RawQuery = q.Encode()

	s.logger.Info("Authorization request",
		"email", email,
		"redirect_uri", redirectURI,
	)

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// handleToken handles the token exchange endpoint.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	// Check for scenario: reject all tokens
	if s.config.Scenarios != nil && s.config.Scenarios.RejectAllTokens {
		http.Error(w, `{"error":"server_error","error_description":"Token service unavailable"}`, http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "authorization_code" {
		http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	codeVerifier := r.FormValue("code_verifier")

	// client_id must always match
	if clientID != s.config.ClientID {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		return
	}

	// Look up auth code
	s.mu.Lock()
	authCode, ok := s.codes[code]
	if !ok {
		s.mu.Unlock()
		http.Error(w, `{"error":"invalid_grant","error_description":"Invalid authorization code"}`, http.StatusBadRequest)
		return
	}

	// Check expiry
	if time.Now().After(authCode.expiresAt) {
		delete(s.codes, code)
		s.mu.Unlock()
		http.Error(w, `{"error":"invalid_grant","error_description":"Authorization code expired"}`, http.StatusBadRequest)
		return
	}

	// Branch on PKCE: when /authorize captured a challenge, /token requires the
	// matching verifier and the secret becomes optional (public-client flow).
	// When no challenge was captured, fall through to the legacy secret check.
	if authCode.codeChallenge != "" {
		if codeVerifier == "" {
			delete(s.codes, code)
			s.mu.Unlock()
			http.Error(w, `{"error":"invalid_grant","error_description":"code_verifier required"}`, http.StatusBadRequest)
			return
		}
		sum := sha256.Sum256([]byte(codeVerifier))
		computed := base64.RawURLEncoding.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(computed), []byte(authCode.codeChallenge)) != 1 {
			delete(s.codes, code)
			s.mu.Unlock()
			http.Error(w, `{"error":"invalid_grant","error_description":"PKCE verification failed"}`, http.StatusBadRequest)
			return
		}
		// Public clients send no secret. If a secret is sent, it must match.
		if !s.config.AllowPublicClient && clientSecret != s.config.ClientSecret {
			delete(s.codes, code)
			s.mu.Unlock()
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
		if clientSecret != "" && clientSecret != s.config.ClientSecret {
			delete(s.codes, code)
			s.mu.Unlock()
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
	} else {
		if clientSecret != s.config.ClientSecret {
			s.mu.Unlock()
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
	}

	// Delete used code (one-time use)
	delete(s.codes, code)
	user := authCode.user
	nonce := authCode.nonce
	s.mu.Unlock()

	// Generate tokens
	idToken, err := s.generateIDToken(user, nonce)
	if err != nil {
		s.logger.Error("Failed to generate ID token", "error", err)
		http.Error(w, `{"error":"server_error"}`, http.StatusInternalServerError)
		return
	}

	accessToken := generateRandomString(32)

	// Store access token for userinfo endpoint
	s.mu.Lock()
	s.tokens[accessToken] = user
	s.mu.Unlock()

	response := map[string]interface{}{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"id_token":      idToken,
		"refresh_token": generateRandomString(32),
	}

	s.logger.Info("Token issued",
		"email", user.Email,
		"groups", user.Groups,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleJWKS serves the JSON Web Key Set.
func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	pubKey := &s.privateKey.PublicKey

	// Calculate n and e for JWK
	n := base64URLEncode(pubKey.N.Bytes())
	e := base64URLEncode(bigIntToBytes(int64(pubKey.E)))

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": s.keyID,
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwks)
}

// handleUserInfo serves the userinfo endpoint.
func (s *Server) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	// Extract bearer token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Look up user by access token
	s.mu.RLock()
	user, ok := s.tokens[token]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, `{"error":"invalid_token"}`, http.StatusUnauthorized)
		return
	}

	userInfo := map[string]interface{}{
		"sub":            user.Subject,
		"email":          user.Email,
		"email_verified": user.EmailVerified && !user.ForceUnverified,
		"name":           user.Name,
	}

	if len(user.Groups) > 0 {
		userInfo["groups"] = user.Groups
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// generateIDToken generates a signed ID token for the user.
func (s *Server) generateIDToken(user *TestUser, nonce string) (string, error) {
	now := time.Now()

	// Calculate expiry based on configuration and user overrides
	expiry := now.Add(s.config.TokenExpiry)
	if s.config.Scenarios != nil && s.config.Scenarios.ExpiredTokens {
		expiry = now.Add(-1 * time.Hour) // Already expired
	}
	if user.ForceExpiredToken {
		expiry = now.Add(-1 * time.Hour) // Already expired
	}

	claims := jwt.MapClaims{
		"iss": s.issuerURL,
		"sub": user.Subject,
		"aud": s.config.ClientID,
		"exp": expiry.Unix(),
		"iat": now.Unix(),
	}

	// Add email claim unless scenario disables it
	if s.config.Scenarios == nil || !s.config.Scenarios.MissingEmailClaim {
		if !user.ForceInvalidClaims {
			claims["email"] = user.Email
			claims["email_verified"] = user.EmailVerified && !user.ForceUnverified
		}
	}

	// Add name claim
	if user.Name != "" && !user.ForceInvalidClaims {
		claims["name"] = user.Name
	}

	// Add groups claim
	if len(user.Groups) > 0 {
		claims["groups"] = user.Groups
	}

	// Add nonce if provided
	if nonce != "" {
		claims["nonce"] = nonce
	}

	// Handle invalid audience scenario
	if s.config.Scenarios != nil && s.config.Scenarios.InvalidAudience {
		claims["aud"] = "wrong-audience"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = s.keyID

	// Choose signing key based on scenario
	signingKey := s.privateKey
	if s.config.Scenarios != nil && s.config.Scenarios.InvalidSignature {
		signingKey = s.wrongKey
	}

	return token.SignedString(signingKey)
}

// Helper functions

// generateRandomString generates a random string of the specified length.
func generateRandomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:length]
}

// base64URLEncode encodes data using URL-safe base64 encoding.
func base64URLEncode(data []byte) string {
	return strings.TrimRight(
		strings.ReplaceAll(
			strings.ReplaceAll(
				base64.StdEncoding.EncodeToString(data),
				"+", "-"),
			"/", "_"),
		"=")
}

// bigIntToBytes converts an int64 to bytes.
func bigIntToBytes(n int64) []byte {
	bytes := make([]byte, 4)
	bytes[0] = byte(n >> 24)
	bytes[1] = byte(n >> 16)
	bytes[2] = byte(n >> 8)
	bytes[3] = byte(n)
	return bytes
}
