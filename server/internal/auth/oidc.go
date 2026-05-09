// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"

	"github.com/knodex/knodex/server/internal/rbac"
	utilrand "github.com/knodex/knodex/server/internal/util/rand"
)

const (
	// StateTokenLength is the length of the random state token in bytes
	StateTokenLength = 32
	// MaxOIDCGroups is the maximum number of groups allowed from OIDC claims
	MaxOIDCGroups = 500
	// MaxOIDCGroupNameLength is the maximum length of a single group name
	MaxOIDCGroupNameLength = 256

	// HTTP client timeouts for OIDC provider discovery.
	// Tighter than github.go (10s vs 30s) because OIDC discovery is critical-path during login.

	// OIDCClientTimeout is the overall HTTP request timeout for OIDC provider calls
	OIDCClientTimeout = 10 * time.Second
	// OIDCDialTimeout is the TCP dial timeout — half of OIDCClientTimeout to leave headroom for TLS + request.
	OIDCDialTimeout = 5 * time.Second
	// OIDCTLSHandshakeTimeout is the TLS handshake timeout for OIDC provider connections
	OIDCTLSHandshakeTimeout = 5 * time.Second
	// OIDCIdleConnTimeout is how long idle connections to OIDC providers stay open
	OIDCIdleConnTimeout = 90 * time.Second
	// OIDCMaxIdleConns is the max idle connections across all OIDC provider hosts
	OIDCMaxIdleConns = 100
	// OIDCMaxIdleConnsPerHost is the max idle connections per OIDC provider host
	OIDCMaxIdleConnsPerHost = 10
)

// RedisClient defines the interface for Redis operations needed by OIDC service
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
}

// OIDCService provides OIDC authentication operations
// No longer uses UserService - OIDC users are not persisted to CRD
//
// OIDCService MUST NOT be copied: it embeds sync primitives (sync.RWMutex,
// sync.Once) that cannot be safely copied after first use. Pass *OIDCService.
// The unexported _ noCopy field below trips `go vet`'s copylocks check on
// accidental copies.
type OIDCService struct {
	_ noCopy

	config              *Config
	redisClient         RedisClient
	authService         ServiceInterface
	provisioningService *OIDCProvisioningService
	roleManager         AuthRoleManager
	rolePersister       RolePersister // Optional: persists roles to Redis + invalidates cache. Nil = in-memory only (legacy).

	mu        sync.RWMutex // protects providers map — readers (auth flows) take RLock, reload takes Lock
	providers map[string]*oidcProvider

	// httpClient overrides the default OIDC HTTP client. Used in tests to inject
	// custom transports (e.g., recording RoundTripper). Production callers must
	// not mutate this after construction; use defaultClient() to get the shared
	// production client.
	httpClient *http.Client

	// defaultClientOnce guards lazy construction of defaultClient. One
	// *http.Client is shared across discovery, JWKS, and token exchange so the
	// underlying *http.Transport's idle-connection pool is reused.
	defaultClientOnce sync.Once
	defaultClient     *http.Client
}

// noCopy is an empty marker type whose Lock/Unlock methods cause `go vet`'s
// copylocks check to flag any value-copy of a struct that embeds it. It carries
// no runtime cost and is the same idiom used in the standard library
// (sync.Cond, sync.WaitGroup) for the same purpose.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

// oidcProvider holds the OIDC provider configuration and clients
type oidcProvider struct {
	name         string
	config       *OIDCProviderConfig
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// NewOIDCService creates a new OIDC authentication service.
// rolePersister is optional: when provided, OIDC role assignments are persisted to Redis
// and the authorization cache is invalidated, fixing the cold-start permission delay.
// When nil, falls back to in-memory-only role assignment (legacy behavior).
func NewOIDCService(
	config *Config,
	redisClient RedisClient,
	authService ServiceInterface,
	provisioningService *OIDCProvisioningService,
	roleManager AuthRoleManager,
	rolePersister RolePersister,
) (*OIDCService, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if authService == nil {
		return nil, fmt.Errorf("authService cannot be nil")
	}

	if !config.OIDCEnabled {
		slog.Info("OIDC authentication is disabled")
		return &OIDCService{
			config:              config,
			redisClient:         redisClient,
			authService:         authService,
			provisioningService: provisioningService,
			roleManager:         roleManager,
			rolePersister:       rolePersister,
			providers:           make(map[string]*oidcProvider),
		}, nil
	}

	// OIDC is enabled, validate required dependencies
	if redisClient == nil {
		return nil, fmt.Errorf("redisClient cannot be nil when OIDC is enabled")
	}
	if provisioningService == nil {
		return nil, fmt.Errorf("provisioningService cannot be nil when OIDC is enabled")
	}

	service := &OIDCService{
		config:              config,
		redisClient:         redisClient,
		authService:         authService,
		provisioningService: provisioningService,
		roleManager:         roleManager,
		rolePersister:       rolePersister,
		providers:           make(map[string]*oidcProvider),
	}

	// Initialize OIDC providers
	for _, providerConfig := range config.OIDCProviders {
		// Create a context with timeout for provider initialization.
		// cancel() is called explicitly rather than deferred to avoid
		// accumulating deferred calls across loop iterations.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		if err := service.initializeProvider(ctx, providerConfig); err != nil {
			cancel()
			slog.Error("failed to initialize OIDC provider",
				"provider", providerConfig.Name,
				"error", err,
			)
			// Continue with other providers even if one fails
			continue
		}
		cancel()
		slog.Info("initialized OIDC provider",
			"provider", providerConfig.Name,
			"issuer", providerConfig.IssuerURL,
		)
	}

	if len(service.providers) == 0 && config.OIDCEnabled {
		slog.Warn("OIDC enabled but no providers successfully initialized")
	}

	return service, nil
}

// initializeProvider initializes a single OIDC provider into s.providers.
func (s *OIDCService) initializeProvider(ctx context.Context, config OIDCProviderConfig) error {
	return s.initializeProviderInto(ctx, config, s.providers)
}

// GenerateStateToken generates a cryptographically secure random state token,
// a nonce for ID token replay prevention, and a PKCE code_verifier for
// authorization code interception protection (RFC 7636 / RFC 9449).
// All three are stored in Redis with the same 5-minute TTL, keyed on the state
// token so they share lifecycle and can be retrieved on the callback by any replica.
// Returns (state, nonce, verifier, error).
func (s *OIDCService) GenerateStateToken(ctx context.Context, providerName, redirectURL string) (string, string, string, error) {
	if providerName == "" {
		return "", "", "", fmt.Errorf("provider name cannot be empty")
	}

	// Generate random state and nonce tokens
	state := utilrand.GenerateRandomString(StateTokenLength)
	nonce := utilrand.GenerateRandomString(NonceLength)
	// PKCE code_verifier — RFC 7636 §4.1: 43-128 chars, unreserved ASCII.
	verifier := oauth2.GenerateVerifier()

	// Store provider name and redirect URL (format: "provider|redirectURL")
	value := providerName
	if redirectURL != "" {
		value = fmt.Sprintf("%s|%s", providerName, redirectURL)
	}

	stateKey := fmt.Sprintf("oidc:state:%s", state)
	if err := s.redisClient.Set(ctx, stateKey, value, StateTokenTTL).Err(); err != nil {
		return "", "", "", fmt.Errorf("failed to store state token in Redis: %w", err)
	}

	// Store nonce keyed on state token (shares lifecycle with state)
	nonceKey := fmt.Sprintf("%s%s", NoncePrefix, state)
	if err := s.redisClient.Set(ctx, nonceKey, nonce, NonceTTL).Err(); err != nil {
		return "", "", "", fmt.Errorf("failed to store nonce in Redis: %w", err)
	}

	// Store PKCE verifier keyed on state token (shares lifecycle with state)
	verifierKey := fmt.Sprintf("%s%s", PKCEVerifierPrefix, state)
	if err := s.redisClient.Set(ctx, verifierKey, verifier, PKCEVerifierTTL).Err(); err != nil {
		return "", "", "", fmt.Errorf("failed to store PKCE verifier in Redis: %w", err)
	}

	slog.Debug("generated OIDC state token, nonce, and PKCE verifier",
		"provider", providerName,
		"redirect_url", redirectURL,
		"state_prefix", state[:min(8, len(state))],
		"ttl_seconds", int(StateTokenTTL.Seconds()),
	)

	return state, nonce, verifier, nil
}

// ValidateStateToken validates a state token by checking if it exists in Redis
// and deletes it after validation (one-time use). Returns the provider name and optional redirect URL.
func (s *OIDCService) ValidateStateToken(ctx context.Context, state string) (providerName, redirectURL string, err error) {
	if state == "" {
		return "", "", fmt.Errorf("state token cannot be empty")
	}

	key := fmt.Sprintf("oidc:state:%s", state)

	// Check if key exists and delete it atomically
	value, err := s.redisClient.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", "", fmt.Errorf("invalid or expired state token")
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to validate state token: %w", err)
	}

	if value == "" {
		return "", "", fmt.Errorf("invalid state token value: provider name is empty")
	}

	// Parse value (format: "provider" or "provider|redirectURL")
	providerName, redirectURL, _ = strings.Cut(value, "|")

	slog.Debug("validated and consumed OIDC state token",
		"provider", providerName,
		"redirect_url", redirectURL,
		"state_prefix", state[:min(8, len(state))],
	)

	return providerName, redirectURL, nil
}

// GetAuthCodeURL returns the authorization URL for the specified provider.
// The nonce parameter is included in the authorization URL to bind the ID token
// to this specific authentication request, preventing token replay attacks.
// The verifier is the PKCE code_verifier (RFC 7636); its S256 challenge is
// added to the URL so the IdP can later verify possession on the token exchange.
func (s *OIDCService) GetAuthCodeURL(providerName, state, nonce, verifier string) (string, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerName]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("unknown OIDC provider: %s", providerName)
	}

	// S256ChallengeOption appends both code_challenge and code_challenge_method=S256.
	url := provider.oauth2Config.AuthCodeURL(state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.S256ChallengeOption(verifier),
	)

	slog.Info("generated OIDC authorization URL",
		"provider", providerName,
		"state_prefix", state[:min(8, len(state))],
	)

	return url, nil
}

// ExchangeCodeForToken exchanges an authorization code for tokens and user info.
// The nonce parameter is the stored nonce that was sent in the authorization request;
// it is validated against the nonce claim in the returned ID token to prevent replay attacks.
// The verifier is the PKCE code_verifier matching the challenge sent on the authorization
// request; the IdP recomputes S256(verifier) and compares to the stored challenge.
func (s *OIDCService) ExchangeCodeForToken(ctx context.Context, providerName, code, nonce, verifier string) (*LoginResponse, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerName]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown OIDC provider: %s", providerName)
	}

	// Bind the configured OIDC HTTP client into ctx so token exchange uses the
	// same transport as discovery/JWKS rather than falling through to http.DefaultClient.
	client := s.httpClient
	if client == nil {
		client = s.defaultHTTPClient()
	}
	ctx = oidc.ClientContext(ctx, client)

	// Exchange authorization code for OAuth2 token (with PKCE verifier).
	oauth2Token, err := provider.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Extract ID token from OAuth2 token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in OAuth2 response")
	}

	// Verify ID token
	idToken, err := provider.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract user info from ID token
	// Include Azure AD-specific claims for overage detection and nonce for replay prevention
	var claims struct {
		Email         string   `json:"email"`
		Name          string   `json:"name"`
		GivenName     string   `json:"given_name"`
		FamilyName    string   `json:"family_name"`
		Subject       string   `json:"sub"`
		EmailVerified *bool    `json:"email_verified"`
		Groups        []string `json:"groups"` // OIDC groups claim for project mapping
		Nonce         string   `json:"nonce"`  // OIDC nonce for replay prevention
		// Azure AD overage pattern: when user has too many groups, Azure returns these instead of groups
		HasGroups   bool `json:"hasgroups,omitempty"`
		XMSHasGroup bool `json:"_claim_names,omitempty"` // Alternative overage indicator
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims from ID token: %w", err)
	}

	// Validate nonce: the ID token's nonce claim must match the stored nonce
	// to prevent token replay attacks (AUTH-VULN-04 mitigation)
	if nonce == "" || claims.Nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonce)) != 1 {
		slog.Error("OIDC nonce validation failed",
			"provider", providerName,
			"nonce_present", claims.Nonce != "",
		)
		return nil, fmt.Errorf("ID token nonce validation failed")
	}

	// Log Azure AD overage condition - this requires using Microsoft Graph API to fetch groups
	if claims.HasGroups || claims.XMSHasGroup {
		slog.Warn("Azure AD overage detected: user has too many groups for token",
			"provider", providerName,
			"email", claims.Email,
			"has_groups_claim", claims.HasGroups,
			"help", "Configure groupMembershipClaims in Azure AD app manifest or use Graph API",
		)
	}

	// Enforce email_verified: reject when IdP explicitly returns false.
	// If the claim is absent (nil), we allow it — some IdPs (e.g., certain
	// Azure AD or GitHub OIDC configs) do not include email_verified at all.
	if claims.EmailVerified != nil && !*claims.EmailVerified {
		slog.Warn("OIDC authentication rejected: email not verified by identity provider",
			"provider", providerName,
			"subject", claims.Subject,
		)
		return nil, fmt.Errorf("email address has not been verified by the identity provider")
	}

	// Sanitize OIDC groups (defense-in-depth)
	claims.Groups = sanitizeOIDCGroups(claims.Groups)

	// Validate required claims
	if claims.Email == "" {
		return nil, fmt.Errorf("email claim missing from ID token")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("subject claim missing from ID token")
	}

	// Build display name from available claims
	displayName := claims.Name
	if displayName == "" && claims.GivenName != "" {
		displayName = claims.GivenName
		if claims.FamilyName != "" {
			displayName += " " + claims.FamilyName
		}
	}
	if displayName == "" {
		displayName = claims.Email
	}

	slog.Info("extracted user info from OIDC token",
		"provider", providerName,
		"email", claims.Email,
		"subject", claims.Subject,
		"email_verified", claims.EmailVerified != nil && *claims.EmailVerified,
		"groups_count", len(claims.Groups),
		"has_groups", len(claims.Groups) > 0,
	)

	// Log groups at INFO level for troubleshooting (show first 3 groups, masked for privacy)
	if len(claims.Groups) > 0 {
		// Show up to first 3 groups for debugging (masked to show only first/last 4 chars)
		debugGroups := make([]string, 0, min(3, len(claims.Groups)))
		for i := 0; i < len(claims.Groups) && i < 3; i++ {
			g := claims.Groups[i]
			if len(g) > 12 {
				debugGroups = append(debugGroups, g[:4]+"..."+g[len(g)-4:])
			} else {
				debugGroups = append(debugGroups, g)
			}
		}
		slog.Info("OIDC groups received (first 3, masked)",
			"provider", providerName,
			"groups_preview", debugGroups,
			"total_count", len(claims.Groups),
		)
	} else {
		slog.Warn("no OIDC groups received from provider",
			"provider", providerName,
			"email", claims.Email,
			"help", "Ensure 'groups' optional claim is configured in Azure AD app registration",
		)
	}

	// Log warning if user has many groups (potential performance concern)
	// Use threshold indicator instead of exact count to prevent information disclosure
	if len(claims.Groups) > 100 {
		slog.Warn("user has large number of OIDC groups exceeding threshold",
			"provider", providerName,
			"threshold", 100,
		)
	}

	// Validate email format per RFC 5321/5322 (defense in depth)
	if err := rbac.ValidateEmail(claims.Email); err != nil {
		slog.Error("invalid email from OIDC provider",
			"provider", providerName,
			"email", claims.Email,
			"error", err,
		)
		return nil, fmt.Errorf("invalid email from OIDC provider: %w", err)
	}

	// Create OIDC subject identifier (provider:subject)
	oidcSubject := fmt.Sprintf("%s:%s", providerName, claims.Subject)

	// Evaluate OIDC user (no CRD persistence - uses JWT claims directly)
	// This evaluates group mappings and assigns Casbin roles without creating User CRD
	userInfo, err := s.provisioningService.EvaluateOIDCUser(ctx, oidcSubject, claims.Email, displayName, claims.Groups)
	if err != nil {
		slog.Error("failed to evaluate OIDC user",
			"error", err,
			"oidc_subject", oidcSubject,
			"email", claims.Email,
			"has_groups", len(claims.Groups) > 0,
		)
		return nil, fmt.Errorf("failed to evaluate OIDC user: %w", err)
	}

	slog.Info("OIDC user authenticated",
		"user_id", userInfo.UserID,
		"email", userInfo.Email,
		"assigned_roles", userInfo.AssignedRoles,
		"project_memberships", len(userInfo.ProjectMemberships),
	)

	// Persist roles to Redis and invalidate stale cached authorization decisions.
	// This fixes the cold-start permission delay: without persistence, OIDC roles
	// are lost on pod restart and cached denials block access for up to 5 minutes.
	// EvaluateOIDCUser assigns roles in-memory (Casbin); this step durably persists
	// them so RestorePersistedRoles can restore them after restart.
	if s.rolePersister != nil && len(userInfo.AssignedRoles) > 0 {
		if err := s.rolePersister.AssignUserRoles(ctx, userInfo.UserID, userInfo.AssignedRoles); err != nil {
			// Non-fatal: in-memory Casbin state is still valid for this pod's lifetime.
			// Log at Error level because persistence failure means the cold-start bug
			// will recur on next restart.
			slog.Error("DEGRADED: OIDC user roles not persisted to Redis - will be lost on restart",
				"user_id", userInfo.UserID,
				"email", userInfo.Email,
				"roles", userInfo.AssignedRoles,
				"error", err,
			)
		} else {
			slog.Info("OIDC user roles persisted to Redis",
				"user_id", userInfo.UserID,
				"roles_count", len(userInfo.AssignedRoles),
			)
		}
	}

	// Generate JWT token with OIDC groups for runtime authorization
	// Groups enable Project CRD spec.roles.groups to grant access via ArgoCD-style evaluation
	token, expiresAt, err := s.authService.GenerateTokenWithGroups(userInfo.UserID, userInfo.Email, userInfo.DisplayName, claims.Groups)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	// Get Casbin roles for user - Casbin is the single source of truth for authorization
	// No fallback logic: if Casbin lookup fails, we log the error but trust Casbin state
	var casbinRoles []string
	if s.roleManager != nil {
		userRoles, err := s.roleManager.GetRolesForUser(userInfo.UserID)
		if err != nil {
			slog.Warn("failed to get Casbin roles for OIDC login response",
				"user_id", userInfo.UserID,
				"error", err,
			)
			// Casbin is single source of truth - if lookup fails, return empty roles
			// Do NOT use fallback logic to add roles from other sources
		} else {
			casbinRoles = userRoles
		}
	}

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:             userInfo.UserID,
			Email:          userInfo.Email,
			DisplayName:    userInfo.DisplayName,
			Projects:       userInfo.GetProjects(),
			DefaultProject: userInfo.GetDefaultProject(),
			Groups:         claims.Groups, // Include groups in login response
			CasbinRoles:    casbinRoles,
		},
	}, nil
}

// ListProviders returns the list of configured OIDC provider names
func (s *OIDCService) ListProviders() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := make([]string, 0, len(s.providers))
	for name := range s.providers {
		providers = append(providers, name)
	}
	return providers
}

// ReloadProviders replaces the current OIDC provider set with new providers.
// Active auth flows using old providers complete via Redis-backed state tokens;
// the provider config is re-read at exchange time from the new set.
// Returns an error if any providers failed to initialize (partial success is still applied).
func (s *OIDCService) ReloadProviders(ctx context.Context, providers []OIDCProviderConfig) error {
	newProviders := make(map[string]*oidcProvider, len(providers))
	var failedCount int

	for _, providerConfig := range providers {
		initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

		if err := s.initializeProviderInto(initCtx, providerConfig, newProviders); err != nil {
			cancel()
			failedCount++
			slog.Error("failed to initialize OIDC provider during reload",
				"provider", providerConfig.Name,
				"error", err,
			)
			continue
		}
		cancel()
		slog.Info("reloaded OIDC provider",
			"provider", providerConfig.Name,
			"issuer", providerConfig.IssuerURL,
		)
	}

	// Atomic swap under write lock
	s.mu.Lock()
	s.providers = newProviders
	s.mu.Unlock()

	slog.Info("OIDC providers reloaded",
		"active_count", len(newProviders),
		"configured_count", len(providers),
	)

	if failedCount > 0 {
		return fmt.Errorf("%d of %d OIDC providers failed to initialize", failedCount, len(providers))
	}

	return nil
}

// initializeProviderInto initializes a single OIDC provider and adds it to the target map.
func (s *OIDCService) initializeProviderInto(ctx context.Context, config OIDCProviderConfig, target map[string]*oidcProvider) error {
	if config.Name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if config.IssuerURL == "" {
		return fmt.Errorf("issuer URL cannot be empty for provider %s", config.Name)
	}
	if config.ClientID == "" {
		return fmt.Errorf("client ID cannot be empty for provider %s", config.Name)
	}
	if config.RedirectURL == "" {
		return fmt.Errorf("redirect URL cannot be empty for provider %s", config.Name)
	}

	// Resolve token endpoint auth method. Empty + empty secret infers "none"
	// (public client, PKCE-only); empty + secret defaults to client_secret_basic.
	authMethod := config.TokenEndpointAuthMethod
	inferred := false
	if authMethod == "" {
		if config.ClientSecret == "" {
			authMethod = "none"
			inferred = true
		} else {
			authMethod = "client_secret_basic"
		}
	}

	switch authMethod {
	case "client_secret_basic":
		if config.ClientSecret == "" {
			return fmt.Errorf("client secret cannot be empty for provider %s with token_endpoint_auth_method=%s", config.Name, authMethod)
		}
	case "none":
		// Public client — no secret expected. Any provided value is ignored.
	default:
		return fmt.Errorf("unsupported token_endpoint_auth_method %q for provider %s (supported: client_secret_basic, none)", authMethod, config.Name)
	}

	scopes := config.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	// Explicit-endpoint bypass: when all three endpoint URLs are supplied, skip
	// HTTP discovery entirely and construct the provider from the supplied values.
	// This supports IdPs that serve an incomplete /.well-known/openid-configuration
	// (e.g., Supabase GoTrue advertises only issuer + jwks_uri; go-oidc requires
	// authorization_endpoint and rejects the document). All three are required
	// together — partial input falls through to discovery.
	var provider *oidc.Provider
	if config.AuthorizationURL != "" && config.TokenURL != "" && config.JWKSURL != "" {
		pc := oidc.ProviderConfig{
			IssuerURL: config.IssuerURL,
			AuthURL:   config.AuthorizationURL,
			TokenURL:  config.TokenURL,
			JWKSURL:   config.JWKSURL,
		}
		// Bind the OIDC HTTP client into ctx so JWKS fetches at verify time
		// share the same transport as the rest of the OIDC operations.
		client := s.httpClient
		if client == nil {
			client = s.defaultHTTPClient()
		}
		provider = pc.NewProvider(oidc.ClientContext(ctx, client))
		slog.Info("OIDC provider initialized with explicit endpoints (discovery skipped)",
			"provider", config.Name,
			"issuer", config.IssuerURL,
		)
	} else {
		// Use the configured OIDC HTTP client (s.httpClient or defaultHTTPClient()).
		// Tests may inject s.httpClient to instrument the transport (e.g., recording RoundTripper).
		client := s.httpClient
		if client == nil {
			client = s.defaultHTTPClient()
		}
		safeCtx := oidc.ClientContext(ctx, client)
		var err error
		provider, err = oidc.NewProvider(safeCtx, config.IssuerURL)
		if err != nil {
			return fmt.Errorf("failed to create OIDC provider: %w", err)
		}
	}

	// For public clients, leave ClientSecret empty so oauth2 omits the basic-auth header.
	oauthClientSecret := ""
	if authMethod == "client_secret_basic" {
		oauthClientSecret = config.ClientSecret
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: oauthClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	if authMethod == "none" {
		slog.Info("OIDC provider configured as public client (PKCE)",
			"provider", config.Name,
			"method", "none",
			"inferred", inferred,
		)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	target[config.Name] = &oidcProvider{
		name:         config.Name,
		config:       &config,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
	}

	return nil
}

// defaultHTTPClient returns the production HTTP client for OIDC operations
// (discovery, JWKS, token exchange). The client is constructed once per
// OIDCService and shared across all calls so the transport's idle-connection
// pool is reused. Honors HTTPS_PROXY/NO_PROXY env vars.
func (s *OIDCService) defaultHTTPClient() *http.Client {
	s.defaultClientOnce.Do(func() {
		s.defaultClient = newDefaultOIDCClient()
	})
	return s.defaultClient
}

// newDefaultOIDCClient builds the shared production *http.Client. Extracted so
// tests can construct it without exercising the OIDCService cache.
func newDefaultOIDCClient() *http.Client {
	return &http.Client{
		Timeout: OIDCClientTimeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         defaultOIDCDialer().DialContext,
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: OIDCTLSHandshakeTimeout,
			IdleConnTimeout:     OIDCIdleConnTimeout,
			MaxIdleConns:        OIDCMaxIdleConns,
			MaxIdleConnsPerHost: OIDCMaxIdleConnsPerHost,
		},
	}
}

// defaultOIDCDialer builds the production net.Dialer used by the OIDC HTTP
// client. Exposed so tests can assert OIDCDialTimeout is wired correctly,
// since the timeout is otherwise unreachable through Transport.DialContext.
func defaultOIDCDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   OIDCDialTimeout,
		KeepAlive: 30 * time.Second,
	}
}

// sanitizeOIDCGroups filters and cleans OIDC group claims.
// It removes empty groups, invalid UTF-8, control characters, and truncates
// overly long names. This is defense-in-depth - we don't fail auth on bad
// group data, we just filter it out.
//
// Note: This differs from validateGroups() in provisioning.go which returns
// errors for invalid data. sanitizeOIDCGroups silently filters because we
// want authentication to succeed even with partially malformed group claims
// from IdPs we don't control.
func sanitizeOIDCGroups(groups []string) []string {
	if groups == nil {
		return []string{}
	}

	// Limit total number of groups to prevent DoS
	if len(groups) > MaxOIDCGroups {
		slog.Warn("OIDC groups count exceeds maximum, truncating",
			"received", len(groups),
			"max", MaxOIDCGroups,
		)
		groups = groups[:MaxOIDCGroups]
	}

	validated := make([]string, 0, len(groups))
	for _, group := range groups {
		// Skip empty groups
		if group == "" {
			continue
		}

		// Validate UTF-8 encoding
		if !utf8.ValidString(group) {
			slog.Debug("skipping OIDC group with invalid UTF-8 encoding")
			continue
		}

		// Check for control characters (security risk)
		hasControlChar := false
		for _, r := range group {
			if unicode.IsControl(r) {
				hasControlChar = true
				break
			}
		}
		if hasControlChar {
			slog.Debug("skipping OIDC group containing control characters")
			continue
		}

		// Truncate overly long group names (UTF-8 safe)
		if len(group) > MaxOIDCGroupNameLength {
			slog.Debug("truncating OIDC group name exceeding max length")
			group = truncateUTF8(group, MaxOIDCGroupNameLength)
		}

		validated = append(validated, group)
	}

	return validated
}

// truncateUTF8 truncates a string to at most maxBytes without breaking UTF-8.
// It finds the last valid rune boundary at or before maxBytes.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	// Find the last valid rune boundary at or before maxBytes
	truncated := s[:maxBytes]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		// Remove the last byte until we have valid UTF-8
		truncated = truncated[:len(truncated)-1]
	}

	return truncated
}
