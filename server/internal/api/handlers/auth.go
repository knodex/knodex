package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/provops-org/knodex/server/internal/api/helpers"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/audit"
	"github.com/provops-org/knodex/server/internal/auth"
)

// isAllowedRedirectURL validates that a redirect URL is safe (not an open redirect).
// It accepts:
// - Relative paths (e.g., "/auth/callback", "/dashboard")
// - URLs whose origin matches one of the allowedOrigins
// It rejects:
// - Absolute URLs to unknown/external domains
// - Protocol-relative URLs (//evil.com)
// - URLs with javascript: or data: schemes
func isAllowedRedirectURL(redirectURL string, allowedOrigins []string) bool {
	if redirectURL == "" {
		return true // Empty redirect is always safe (server returns JSON)
	}

	// Reject protocol-relative URLs (//evil.com)
	if strings.HasPrefix(redirectURL, "//") {
		return false
	}

	// Allow relative paths (start with / but not //)
	if strings.HasPrefix(redirectURL, "/") {
		return true
	}

	// Parse the URL to check the origin
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		return false
	}

	// Reject dangerous schemes
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}

	// Check against allowed origins
	redirectOrigin := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
	for _, allowed := range allowedOrigins {
		if strings.ToLower(strings.TrimRight(allowed, "/")) == redirectOrigin {
			return true
		}
	}

	return false
}

// OIDCServiceInterface defines the interface for OIDC authentication operations
type OIDCServiceInterface interface {
	GenerateStateToken(ctx context.Context, providerName, redirectURL string) (string, error)
	ValidateStateToken(ctx context.Context, state string) (providerName, redirectURL string, err error)
	GetAuthCodeURL(providerName, state string) (string, error)
	ExchangeCodeForToken(ctx context.Context, providerName, code string) (*auth.LoginResponse, error)
	ListProviders() []string
	ReloadProviders(ctx context.Context, providers []auth.OIDCProviderConfig) error
}

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService            auth.ServiceInterface
	oidcService            OIDCServiceInterface
	auditRecorder          audit.Recorder
	allowedRedirectOrigins []string
	redisClient            *redis.Client // For opaque auth code exchange
}

// SetAuditRecorder sets the audit recorder for recording login/logout events.
func (h *AuthHandler) SetAuditRecorder(r audit.Recorder) {
	h.auditRecorder = r
}

// SetAllowedRedirectOrigins sets the allowed redirect origins for OIDC callbacks.
func (h *AuthHandler) SetAllowedRedirectOrigins(origins []string) {
	h.allowedRedirectOrigins = origins
}

// SetRedisClient sets the Redis client for opaque auth code exchange.
func (h *AuthHandler) SetRedisClient(c *redis.Client) {
	h.redisClient = c
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(authService auth.ServiceInterface, oidcService OIDCServiceInterface) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		oidcService: oidcService,
	}
}

// LocalLogin handles POST /api/v1/auth/local/login
func (h *AuthHandler) LocalLogin(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req auth.LocalLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("failed to parse login request", "error", err)
		response.BadRequest(w, "invalid request body", nil)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		response.BadRequest(w, "username and password are required", nil)
		return
	}

	// Extract source IP for rate limiting
	sourceIP := audit.SourceIP(r)

	// Authenticate
	loginResp, err := h.authService.AuthenticateLocal(r.Context(), req.Username, req.Password, sourceIP)
	if err != nil {
		// Check if rate-limited — return 429 with retry_after so the UI can show a countdown
		var rateLimitErr *auth.ErrRateLimited
		if errors.As(err, &rateLimitErr) {
			retryAfterSecs := int(math.Ceil(rateLimitErr.RetryAfter.Seconds()))
			response.WriteError(w, http.StatusTooManyRequests, response.ErrCodeRateLimit,
				"too many failed login attempts, please try again later",
				map[string]string{"retry_after": strconv.Itoa(retryAfterSecs)},
			)
			return
		}

		slog.Warn("local authentication failed",
			"username", req.Username,
			"error", err,
		)
		// Return generic error to prevent username enumeration
		response.WriteError(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "invalid credentials", nil)
		return
	}

	// Log successful login
	slog.Info("local admin login successful",
		"user_id", loginResp.User.ID,
		"username", req.Username,
	)

	// Pass verified user identity to audit middleware via context signal.
	// Local login also writes the user in the response body (which the middleware
	// can parse), but the context signal is the canonical path for both OIDC and local.
	audit.SetLoginIdentity(r, loginResp.User.ID, loginResp.User.Email)

	// Return JWT token
	response.WriteJSON(w, http.StatusOK, loginResp)
}

// OIDCLogin handles GET /api/v1/auth/oidc/login?provider={name}
// Initiates the OIDC authentication flow
func (h *AuthHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	// Get provider name from query parameter
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		response.BadRequest(w, "provider parameter is required", nil)
		return
	}

	// Validate provider exists
	providers := h.oidcService.ListProviders()
	validProvider := false
	for _, p := range providers {
		if p == provider {
			validProvider = true
			break
		}
	}
	if !validProvider {
		availableProvidersStr := ""
		for i, p := range providers {
			if i > 0 {
				availableProvidersStr += ", "
			}
			availableProvidersStr += p
		}
		response.BadRequest(w, "unknown OIDC provider", map[string]string{
			"provider":            provider,
			"available_providers": availableProvidersStr,
		})
		return
	}

	// Get redirect URL from query parameter and validate against allowlist
	redirectURL := r.URL.Query().Get("redirect")
	if !isAllowedRedirectURL(redirectURL, h.allowedRedirectOrigins) {
		slog.Warn("OIDC login rejected: invalid redirect URL",
			"redirect", redirectURL,
		)
		response.BadRequest(w, "invalid redirect URL: only relative paths or configured origins are allowed", nil)
		return
	}

	// Generate CSRF state token (stores provider name and redirect URL)
	state, err := h.oidcService.GenerateStateToken(r.Context(), provider, redirectURL)
	if err != nil {
		slog.Error("failed to generate state token",
			"provider", provider,
			"error", err,
		)
		response.InternalError(w, "failed to initiate OIDC login")
		return
	}

	// Get authorization URL
	authURL, err := h.oidcService.GetAuthCodeURL(provider, state)
	if err != nil {
		slog.Error("failed to get authorization URL",
			"provider", provider,
			"error", err,
		)
		response.InternalError(w, "failed to initiate OIDC login")
		return
	}

	slog.Info("initiating OIDC login flow",
		"provider", provider,
		"state_prefix", state[:8],
	)

	// Redirect to provider's authorization endpoint
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDCCallback handles GET /api/v1/auth/oidc/callback?code=...&state=...
// Handles the OIDC callback and completes authentication
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	// Get code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	// Validate required parameters
	if code == "" {
		response.BadRequest(w, "code parameter is required", nil)
		return
	}
	if state == "" {
		response.BadRequest(w, "state parameter is required", nil)
		return
	}

	// Validate state token and extract provider name and redirect URL (CSRF protection)
	provider, redirectURL, err := h.oidcService.ValidateStateToken(r.Context(), state)
	if err != nil {
		slog.Warn("invalid or expired state token",
			"state_prefix", state[:min(8, len(state))],
			"error", err,
		)
		// Signal audit middleware that this is a failed login (302 redirects are non-2xx
		// but we need to distinguish success from failure for audit events).
		// Uses context-based signaling to avoid leaking internal headers to browser clients.
		audit.SetLoginResult(r, "denied")
		// If we have a redirect URL, redirect with generic error code, otherwise return JSON error
		// Internal error details are logged server-side only (above)
		if redirectURL != "" {
			errorURL := fmt.Sprintf("%s?error=%s", redirectURL, url.QueryEscape("authentication_failed"))
			http.Redirect(w, r, errorURL, http.StatusFound)
		} else {
			response.WriteError(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "authentication failed", nil)
		}
		return
	}

	// Exchange authorization code for tokens
	loginResp, err := h.oidcService.ExchangeCodeForToken(r.Context(), provider, code)
	if err != nil {
		slog.Error("failed to exchange authorization code",
			"provider", provider,
			"error", err,
		)
		audit.SetLoginResult(r, "denied")
		// If we have a redirect URL, redirect with generic error code, otherwise return JSON error
		// Internal error details are logged server-side only (above)
		if redirectURL != "" {
			errorURL := fmt.Sprintf("%s?error=%s", redirectURL, url.QueryEscape("authentication_failed"))
			http.Redirect(w, r, errorURL, http.StatusFound)
		} else {
			response.WriteError(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "authentication failed", nil)
		}
		return
	}

	// Log successful login
	slog.Info("OIDC login successful",
		"provider", provider,
		"user_id", loginResp.User.ID,
		"email", loginResp.User.Email,
	)

	// Redirect to frontend callback with opaque code (never expose JWT in URL)
	if redirectURL != "" {
		if h.redisClient != nil {
			// Store JWT in Redis with opaque code key (single-use, 30s TTL)
			opaqueCode, storeErr := StoreAuthCode(r.Context(), h.redisClient, loginResp.Token)
			if storeErr != nil {
				slog.Error("failed to store auth code",
					"provider", provider,
					"error", storeErr,
				)
				response.InternalError(w, "failed to complete authentication")
				return
			}
			// Signal audit middleware only after StoreAuthCode succeeds.
			// Both identity and result are set together so a Redis failure
			// does not produce a login_failed event with real user identity.
			audit.SetLoginIdentity(r, loginResp.User.ID, loginResp.User.Email)
			audit.SetLoginResult(r, "success")
			successURL := fmt.Sprintf("%s?code=%s", redirectURL, url.QueryEscape(opaqueCode))
			http.Redirect(w, r, successURL, http.StatusFound)
		} else {
			// Fail-closed: refuse to redirect without Redis (would expose JWT in URL or body)
			slog.Error("Redis client not configured for auth code exchange, cannot complete OIDC flow safely",
				"provider", provider,
			)
			response.InternalError(w, "authentication service misconfigured")
			return
		}
	} else {
		// No redirect URL — return JSON directly (e.g., API-only OIDC flow).
		// Identity is in the response body, but set context signal too for consistency.
		audit.SetLoginIdentity(r, loginResp.User.ID, loginResp.User.Email)
		audit.SetLoginResult(r, "success")
		response.WriteJSON(w, http.StatusOK, loginResp)
	}
}

// Logout handles POST /api/v1/auth/logout
// Records an audit event for the logout action. JWT is stateless so there is
// no server-side session to invalidate — the endpoint exists for audit trail purposes.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	audit.RecordEvent(h.auditRecorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "logout",
		Resource:  "auth",
		RequestID: r.Header.Get("X-Request-ID"),
		Result:    "success",
	})

	slog.Info("user logged out",
		"user_id", userCtx.UserID,
		"email", userCtx.Email,
	)

	response.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// ListOIDCProviders handles GET /api/v1/auth/oidc/providers
// Returns the list of available OIDC providers
func (h *AuthHandler) ListOIDCProviders(w http.ResponseWriter, r *http.Request) {
	// If OIDC is disabled, return empty list
	// Check both for nil interface and typed nil (Go interface gotcha)
	if h.oidcService == nil || reflect.ValueOf(h.oidcService).IsNil() {
		response.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"providers": []interface{}{},
		})
		return
	}

	providers := h.oidcService.ListProviders()
	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
	})
}
