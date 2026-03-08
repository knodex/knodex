package auth

import "time"

// JWTClaims represents the claims in a JWT token
//
// ArgoCD-Aligned Authorization Model:
// - CasbinRoles: Contains Casbin role names (e.g., ["role:serveradmin", "proj:acme:developer"])
// - Permissions: Pre-computed permission flags for frontend UI rendering
//
// The Permissions field allows the frontend to make optimistic UI decisions without
// checking role strings directly. The backend remains the source of truth for authorization.

type JWTClaims struct {
	UserID         string            `json:"sub"`                       // User ID (subject)
	Email          string            `json:"email"`                     // User email
	DisplayName    string            `json:"name,omitempty"`            // User display name
	Projects       []string          `json:"projects,omitempty"`        // Project IDs
	DefaultProject string            `json:"default_project,omitempty"` // Default project ID
	Groups         []string          `json:"groups,omitempty"`          // OIDC groups for runtime authorization
	Roles          map[string]string `json:"roles,omitempty"`           // Project ID -> role name mapping for frontend permission checks
	CasbinRoles    []string          `json:"casbin_roles,omitempty"`    // Casbin roles (e.g., ["role:serveradmin", "proj:acme:developer"])
	Permissions    map[string]bool   `json:"permissions,omitempty"`     // Pre-computed permissions for frontend UI (ArgoCD-aligned)
	JTI            string            `json:"jti,omitempty"`             // JWT ID for server-side revocation
	ExpiresAt      int64             `json:"exp"`                       // Expiration timestamp
	IssuedAt       int64             `json:"iat"`                       // Issued at timestamp
	Issuer         string            `json:"iss,omitempty"`             // Token issuer (OIDC provider URL)
}

// LocalLoginRequest represents a local admin login request
type LocalLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response from a successful login.
// The Token field is excluded from JSON serialization — the JWT is
// delivered via HttpOnly cookie (Set-Cookie header), not in the response body.
type LoginResponse struct {
	Token     string    `json:"-"`         // JWT access token (set via HttpOnly cookie, not in JSON)
	ExpiresAt time.Time `json:"expiresAt"` // Token expiration time
	User      UserInfo  `json:"user"`      // User information
}

// UserInfo is a simplified user representation for login responses
//
// ArgoCD-Aligned Authorization Model:
// - CasbinRoles: Contains Casbin role names (e.g., ["role:serveradmin", "proj:acme:developer"])
// - Permissions: Pre-computed permission flags for frontend UI rendering

type UserInfo struct {
	ID             string            `json:"id"`
	Email          string            `json:"email"`
	DisplayName    string            `json:"displayName,omitempty"`
	Projects       []string          `json:"projects,omitempty"`
	DefaultProject string            `json:"defaultProject,omitempty"`
	Groups         []string          `json:"groups,omitempty"`      // OIDC groups for runtime authorization
	Roles          map[string]string `json:"roles,omitempty"`       // Project ID -> role name mapping for frontend permission checks
	CasbinRoles    []string          `json:"casbinRoles,omitempty"` // Casbin roles (e.g., ["role:serveradmin", "proj:acme:developer"])
	Permissions    map[string]bool   `json:"permissions,omitempty"` // Pre-computed permissions for frontend UI (ArgoCD-aligned)
}

// Config holds authentication configuration
type Config struct {
	// JWTSecret is the secret key used to sign JWT tokens
	JWTSecret string

	// JWTExpiry is the JWT token expiration duration (default: 1 hour)
	JWTExpiry time.Duration

	// LocalAdmin configuration
	LocalAdminUsername string
	LocalAdminPassword string // Plaintext password, will be hashed

	// OIDC configuration
	OIDCEnabled   bool
	OIDCProviders []OIDCProviderConfig
}

// OIDCProviderConfig represents an OIDC provider configuration
type OIDCProviderConfig struct {
	Name         string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// OIDCLoginRequest represents the query parameters for OIDC login
type OIDCLoginRequest struct {
	Provider string `json:"provider"` // Provider name
}

// OIDCCallbackRequest represents the query parameters from OIDC callback
type OIDCCallbackRequest struct {
	Code  string `json:"code"`  // Authorization code
	State string `json:"state"` // CSRF state token
}
