package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/knodex/knodex/server/internal/api/cookie"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/auth"
)

// UserContextKey is the context key for user information
type contextKey string

const (
	UserContextKey contextKey = "user"
)

// UserContext contains the authenticated user's information

// Use policyEnforcer.HasRole(ctx, userID, "role:serveradmin") for global admin checks
type UserContext struct {
	UserID         string
	Email          string
	DisplayName    string
	Projects       []string
	DefaultProject string
	Groups         []string // OIDC groups for runtime authorization
	CasbinRoles    []string
	Roles          map[string]string // Project ID -> role name mapping
	Issuer         string            // JWT issuer (OIDC provider URL or empty for local)
	TokenExpiresAt int64             // Token expiry as Unix timestamp
	TokenIssuedAt  int64             // Token issued-at as Unix timestamp
}

// AuthConfig holds configuration for authentication middleware
type AuthConfig struct {
	AuthService auth.ServiceInterface
}

// extractToken extracts a JWT token from the request.
// Priority: 1) knodex_session cookie, 2) Authorization: Bearer header.
func extractToken(r *http.Request) (string, bool) {
	// 1. Check for session cookie
	if c, err := r.Cookie(cookie.SessionCookieName); err == nil && c.Value != "" {
		return c.Value, true
	}

	// 2. Fall back to Authorization: Bearer header (API clients, CLI, E2E tests)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", false
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", false
	}
	return parts[1], true
}

// Auth middleware extracts and validates JWT from cookie or Authorization header
func Auth(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := extractToken(r)
			if !ok {
				response.Unauthorized(w, "missing authentication credentials")
				return
			}

			// Validate JWT token
			claims, err := config.AuthService.ValidateToken(r.Context(), tokenString)
			if err != nil {
				response.Unauthorized(w, "invalid or expired token")
				return
			}

			// Create user context

			userCtx := &UserContext{
				UserID:         claims.UserID,
				Email:          claims.Email,
				DisplayName:    claims.DisplayName,
				Projects:       claims.Projects,
				DefaultProject: claims.DefaultProject,
				Groups:         claims.Groups, // OIDC groups for runtime authorization
				CasbinRoles:    claims.CasbinRoles,
				Roles:          claims.Roles,
				Issuer:         claims.Issuer,
				TokenExpiresAt: claims.ExpiresAt,
				TokenIssuedAt:  claims.IssuedAt,
			}

			// Attach user context to request context
			ctx := context.WithValue(r.Context(), UserContextKey, userCtx)

			// Also store JWT claims as map for GetUserGroupsFromContext
			// This enables CasbinAuthz middleware to extract groups for policy evaluation

			jwtClaimsMap := map[string]interface{}{
				"sub":             claims.UserID,
				"email":           claims.Email,
				"name":            claims.DisplayName,
				"projects":        claims.Projects,
				"default_project": claims.DefaultProject,
				"groups":          claims.Groups,
				"casbin_roles":    claims.CasbinRoles,
				"exp":             claims.ExpiresAt,
				"iat":             claims.IssuedAt,
				"iss":             claims.Issuer,
			}
			ctx = WithJWTClaims(ctx, jwtClaimsMap)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserContext retrieves the user context from the request context
func GetUserContext(r *http.Request) (*UserContext, bool) {
	ctx := r.Context().Value(UserContextKey)
	if ctx == nil {
		return nil, false
	}
	userCtx, ok := ctx.(*UserContext)
	return userCtx, ok
}

// OptionalAuth middleware attempts to extract JWT but doesn't require it.
// Reserved for future use on endpoints that behave differently for authenticated vs anonymous users
// (e.g., public RGD catalog with enhanced features for authenticated users).
func OptionalAuth(config AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := extractToken(r)
			if !ok {
				// No credentials, continue without user context
				next.ServeHTTP(w, r)
				return
			}

			claims, err := config.AuthService.ValidateToken(r.Context(), tokenString)
			if err != nil {
				// Invalid token, continue without user context
				next.ServeHTTP(w, r)
				return
			}

			// Valid token, attach user context

			userCtx := &UserContext{
				UserID:         claims.UserID,
				Email:          claims.Email,
				DisplayName:    claims.DisplayName,
				Projects:       claims.Projects,
				DefaultProject: claims.DefaultProject,
				Groups:         claims.Groups, // OIDC groups for runtime authorization
				CasbinRoles:    claims.CasbinRoles,
				Roles:          claims.Roles,
				Issuer:         claims.Issuer,
				TokenExpiresAt: claims.ExpiresAt,
				TokenIssuedAt:  claims.IssuedAt,
			}

			ctx := context.WithValue(r.Context(), UserContextKey, userCtx)

			// Also store JWT claims as map for GetUserGroupsFromContext

			jwtClaimsMap := map[string]interface{}{
				"sub":             claims.UserID,
				"email":           claims.Email,
				"name":            claims.DisplayName,
				"projects":        claims.Projects,
				"default_project": claims.DefaultProject,
				"groups":          claims.Groups,
				"casbin_roles":    claims.CasbinRoles,
				"exp":             claims.ExpiresAt,
				"iat":             claims.IssuedAt,
				"iss":             claims.Issuer,
			}
			ctx = WithJWTClaims(ctx, jwtClaimsMap)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
