// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/rbac"
)

// ServiceInterface defines the interface for authentication operations
type ServiceInterface interface {
	AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*LoginResponse, error)
	// GenerateTokenForAccount generates JWT for a local account
	GenerateTokenForAccount(account *Account, userID string) (string, time.Time, error)
	// GenerateTokenWithGroups generates JWT with OIDC groups for runtime authorization
	GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error)
	ValidateToken(ctx context.Context, tokenString string) (*JWTClaims, error)
	// RevokeToken blacklists a JWT by its jti claim for server-side session revocation
	RevokeToken(ctx context.Context, jti string, remainingTTL time.Duration) error
	// IsLocalLoginEnabled reports whether the local user login pathway is active.
	// Returns true only when BOTH conditions hold:
	//   1. LOCAL_LOGIN_ENABLED is true (operator did not disable the feature)
	//   2. A local admin password was successfully bootstrapped at startup
	// In current operation, condition 2 is a hard precondition: app/app.go
	// fails fast if bootstrap fails while LocalLoginEnabled is true. The two
	// terms are kept distinct so the predicate stays correct if that startup
	// behavior is ever loosened.
	IsLocalLoginEnabled() bool
}

const (
	// BcryptCost is the cost factor for bcrypt hashing (12 is recommended for production)
	BcryptCost = 12

	// DefaultJWTExpiry is the default JWT token expiration (1 hour)
	DefaultJWTExpiry = 1 * time.Hour

	// StateTokenTTL is the TTL for OIDC state tokens (5 minutes)
	StateTokenTTL = 5 * time.Minute

	// StateTokenPrefix is the Redis key prefix for OIDC state tokens
	StateTokenPrefix = "oidc:state:"

	// NonceLength is the length of the OIDC nonce in bytes
	NonceLength = 32

	// NonceTTL is the TTL for OIDC nonce tokens (5 minutes, matching state token TTL)
	NonceTTL = 5 * time.Minute

	// NoncePrefix is the Redis key prefix for OIDC nonce tokens (keyed on state)
	NoncePrefix = "oidc:nonce:"

	// PKCEVerifierTTL is the TTL for OIDC PKCE code_verifier (matches state token TTL)
	PKCEVerifierTTL = 5 * time.Minute

	// PKCEVerifierPrefix is the Redis key prefix for OIDC PKCE verifiers (keyed on state)
	PKCEVerifierPrefix = "oidc:pkce:"
)

// Compile-time interface compliance check
var _ ServiceInterface = (*Service)(nil)

// Service provides authentication operations
type Service struct {
	config           *Config
	accountStore     *AccountStore
	projectService   AuthProjectService
	redisClient      *redis.Client
	bootstrapService *ProjectBootstrapService
	roleManager      AuthRoleManager
	groupMapper      *GroupMapper
	k8sClient        kubernetes.Interface
	blacklist        JWTBlacklistInterface
}

// validatePasswordComplexity checks if password meets complexity requirements
// SECURITY: Require 3 of 4 character classes (CWE-521 mitigation)
func validatePasswordComplexity(password string) error {
	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Count how many character classes are present
	classCount := 0
	if hasUpper {
		classCount++
	}
	if hasLower {
		classCount++
	}
	if hasDigit {
		classCount++
	}
	if hasSpecial {
		classCount++
	}

	if classCount < 3 {
		return fmt.Errorf("password must contain at least 3 of the following: uppercase letters, lowercase letters, digits, special characters")
	}

	return nil
}

// NewService creates a new authentication service
// Uses AccountStore (ConfigMap/Secret) for local user authentication following the ArgoCD pattern
func NewService(config *Config, accountStore *AccountStore, projectService AuthProjectService, k8sClient kubernetes.Interface, redisClient *redis.Client, roleManager AuthRoleManager) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if accountStore == nil {
		return nil, fmt.Errorf("accountStore cannot be nil")
	}
	if projectService == nil {
		return nil, fmt.Errorf("projectService cannot be nil")
	}
	if k8sClient == nil {
		return nil, fmt.Errorf("k8sClient cannot be nil")
	}
	if redisClient == nil {
		return nil, fmt.Errorf("redisClient cannot be nil")
	}

	// Set default JWT expiry if not configured
	if config.JWTExpiry == 0 {
		config.JWTExpiry = DefaultJWTExpiry
	}

	// JWT secret can come from config or AccountStore (server.secretkey in Secret)
	// If not provided via config, it will be retrieved from AccountStore at runtime
	if config.JWTSecret == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		secretKey, err := accountStore.GetServerSecretKey(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get JWT secret from account store: %w", err)
		}
		config.JWTSecret = secretKey
		slog.Info("loaded JWT secret from knodex-secret")
	}

	// Bootstrap admin password to AccountStore if provided via config.
	// Validation runs unconditionally — a malformed password rejected at startup
	// is better than silently swallowed config. The AccountStore SetPassword call
	// is gated on LocalLoginEnabled (defense in depth — primary gate is in app.go).
	adminUsername := config.LocalAdminUsername
	if adminUsername == "" {
		adminUsername = "admin"
	}
	if config.LocalAdminPassword != "" {
		if len(config.LocalAdminPassword) < 12 {
			return nil, fmt.Errorf("LocalAdminPassword must be at least 12 characters, got %d characters", len(config.LocalAdminPassword))
		}
		if err := validatePasswordComplexity(config.LocalAdminPassword); err != nil {
			return nil, fmt.Errorf("LocalAdminPassword complexity validation failed: %w", err)
		}

		if config.LocalLoginEnabled {
			// Set admin password in AccountStore (this persists to Secret)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := accountStore.SetPassword(ctx, adminUsername, config.LocalAdminPassword); err != nil {
				slog.Warn("failed to bootstrap admin password to AccountStore",
					"error", err,
				)
				// Continue anyway - password may already be set in Secret
			} else {
				slog.Info("bootstrapped admin password to AccountStore")
			}
		} else {
			slog.Info("local login disabled, skipping admin password bootstrap to AccountStore")
		}
	}

	// Load accounts from ConfigMap/Secret
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := accountStore.LoadAccounts(ctx); err != nil {
		return nil, fmt.Errorf("failed to load accounts: %w", err)
	}

	// Validate that the admin account is properly configured after loading.
	// The password hash is stored in knodex-secret, but the account definition
	// (capabilities, enabled status) must exist in the knodex-accounts ConfigMap.
	// Without it, login will fail with a misleading "invalid credentials" error.
	// Only run when local login is intended to function — when disabled, the
	// missing/invalid admin account is not a misconfiguration.
	if config.LocalLoginEnabled && config.LocalAdminPassword != "" {
		adminAccount, err := accountStore.GetAccount(ctx, adminUsername)
		if err != nil {
			slog.Error("admin password was bootstrapped but admin account is not defined in knodex-accounts ConfigMap — local admin login will fail",
				"username", adminUsername,
				"configmap", AccountConfigMapName,
				"help", fmt.Sprintf("Add 'accounts.%s: apiKey, login' and 'accounts.%s.enabled: true' to the %s ConfigMap", adminUsername, adminUsername, AccountConfigMapName),
			)
		} else if !adminAccount.CanLogin() {
			slog.Error("admin account exists but cannot login — check capabilities and enabled status",
				"username", adminUsername,
				"enabled", adminAccount.Enabled,
				"capabilities", adminAccount.Capabilities,
				"configmap", AccountConfigMapName,
				"help", fmt.Sprintf("Ensure 'accounts.%s' includes 'login' capability and 'accounts.%s.enabled' is 'true'", adminUsername, adminUsername),
			)
		}
	}

	// Create bootstrap service for default project
	bootstrapService := NewProjectBootstrapService(projectService, k8sClient)

	service := &Service{
		config:           config,
		accountStore:     accountStore,
		projectService:   projectService,
		redisClient:      redisClient,
		bootstrapService: bootstrapService,
		roleManager:      roleManager,
		k8sClient:        k8sClient,
		blacklist:        newRedisJWTBlacklist(redisClient),
	}

	slog.Info("local login status",
		"enabled", config.LocalLoginEnabled,
		"password_provided", config.LocalAdminPassword != "",
	)

	return service, nil
}

// SetGroupMapper sets the OIDC group mapper for resolving group-to-role mappings.
// This is separate from the constructor to avoid circular dependencies since
// GroupMapper is created after the auth Service in the initialization order.
func (s *Service) SetGroupMapper(mapper *GroupMapper) {
	s.groupMapper = mapper
}

// AuthenticateLocal authenticates a local user using AccountStore
// Uses ConfigMap/Secret instead of User CRD
func (s *Service) AuthenticateLocal(ctx context.Context, username, password, sourceIP string) (*LoginResponse, error) {
	// Defense in depth: refuse authentication when local login is disabled.
	// The HTTP handler also short-circuits, but this guards against any future
	// internal caller (or test) that bypasses the handler.
	if !s.IsLocalLoginEnabled() {
		return nil, fmt.Errorf("local login is disabled")
	}
	// Validate password using AccountStore (includes IP-based rate limiting)
	account, err := s.accountStore.ValidatePassword(ctx, username, password, sourceIP)
	if err != nil {
		return nil, err
	}

	// Generate a stable user ID for the account
	userID := "user-local-" + username
	email := username + "@local"
	displayName := "Local " + username
	if username == "admin" {
		displayName = "Local Administrator"
	}

	// Ensure Casbin role is assigned for admin
	if username == "admin" && s.roleManager != nil {
		hasRole, _ := s.roleManager.HasUserRole(userID, rbac.CasbinRoleServerAdmin)
		if !hasRole {
			if _, err := s.roleManager.AddUserRole(userID, rbac.CasbinRoleServerAdmin); err != nil {
				slog.Warn("failed to assign server admin Casbin role to local admin",
					"user_id", userID,
					"error", err,
				)
			} else {
				slog.Info("assigned Casbin role:serveradmin to local admin",
					"user_id", userID,
				)
			}
		}
	}

	// Ensure default project exists and admin is a member
	var projects []string
	var defaultProject string
	project, err := s.bootstrapService.EnsureDefaultProject(ctx, userID)
	if err != nil {
		slog.Error("failed to bootstrap default project",
			"user_id", userID,
			"error", err,
		)
		// Don't fail login - admin can still authenticate, just without project
	} else {
		projects = []string{project.Name}
		defaultProject = project.Name
	}

	// Generate JWT token
	token, expiresAt, err := s.GenerateTokenForAccount(account, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	var casbinRoles []string
	if s.roleManager != nil {
		userRoles, err := s.roleManager.GetRolesForUser(userID)
		if err != nil {
			slog.Warn("failed to get Casbin roles for local admin login response",
				"user_id", userID,
				"error", err,
			)
		} else {
			casbinRoles = userRoles
		}
	}

	// Compute permissions for the login response (ArgoCD-aligned pattern)
	permissions := s.computePermissions(userID, casbinRoles)

	slog.Info("local user authenticated",
		"user_id", userID,
		"username", username,
	)

	return &LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:             userID,
			Email:          email,
			DisplayName:    displayName,
			Projects:       projects,
			DefaultProject: defaultProject,
			CasbinRoles:    casbinRoles,
			Permissions:    permissions,
		},
	}, nil
}

// GenerateTokenForAccount generates a JWT token for a local account
// No longer uses User CRD - derives user info from Account
func (s *Service) GenerateTokenForAccount(account *Account, userID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.config.JWTExpiry)

	email := account.Name + "@local"
	displayName := "Local " + account.Name
	if account.Name == "admin" {
		displayName = "Local Administrator"
	}

	// Get Casbin roles for the user
	var casbinRoles []string
	if s.roleManager != nil {
		userCasbinRoles, err := s.roleManager.GetRolesForUser(userID)
		if err != nil {
			slog.Warn("failed to get Casbin roles for JWT",
				"user_id", userID,
				"error", err,
			)
		} else {
			casbinRoles = userCasbinRoles
		}
	}

	// Get projects for the user from Casbin (proj:* roles)
	var projects []string
	var defaultProject string
	if s.roleManager != nil {
		for _, role := range casbinRoles {
			if len(role) > 5 && role[:5] == "proj:" {
				// Extract project name from "proj:projectname:role"
				parts := splitProjectRole(role)
				if len(parts) >= 2 {
					projectName := parts[1]
					// Avoid duplicates
					found := false
					for _, p := range projects {
						if p == projectName {
							found = true
							break
						}
					}
					if !found {
						projects = append(projects, projectName)
						if defaultProject == "" {
							defaultProject = projectName
						}
					}
				}
			}
		}
	}

	// Compute permissions for frontend UI (ArgoCD-aligned pattern)
	permissions := s.computePermissions(userID, casbinRoles)

	claims := JWTClaims{
		UserID:         userID,
		Email:          email,
		DisplayName:    displayName,
		Projects:       projects,
		DefaultProject: defaultProject,
		CasbinRoles:    casbinRoles,
		Permissions:    permissions,
		ExpiresAt:      expiresAt.Unix(),
		IssuedAt:       now.Unix(),
	}

	jti := uuid.New().String()
	claims.JTI = jti

	mapClaims := jwt.MapClaims{
		"sub":             claims.UserID,
		"email":           claims.Email,
		"name":            claims.DisplayName,
		"projects":        claims.Projects,
		"default_project": claims.DefaultProject,
		"iss":             "knodex",
		"aud":             "knodex-api",
		"exp":             claims.ExpiresAt,
		"iat":             claims.IssuedAt,
		"jti":             jti,
	}

	if len(casbinRoles) > 0 {
		mapClaims["casbin_roles"] = casbinRoles
	}

	if len(permissions) > 0 {
		mapClaims["permissions"] = permissions
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)

	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// GenerateTokenWithGroups generates a JWT token for OIDC users with groups
// Groups are runtime values from the OIDC ID token, enabling spec.roles.groups authorization
// Also resolves project roles from OIDC groups and includes them in the JWT for frontend permission checks.
// No longer uses User CRD - uses provided user info directly
func (s *Service) GenerateTokenWithGroups(userID, email, displayName string, groups []string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.config.JWTExpiry)

	// Resolve project roles from OIDC groups
	// This enables the frontend to determine user permissions (e.g., useCanDeploy hook)
	var roles map[string]string
	if len(groups) > 0 && s.projectService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		slog.Info("resolving project roles from OIDC groups",
			"user_id", userID,
			"groups_count", len(groups),
		)

		var err error
		roles, err = s.projectService.GetUserProjectRolesByGroup(ctx, groups)
		if err != nil {
			// Log warning but don't fail token generation - roles will be empty
			slog.Warn("failed to resolve project roles from OIDC groups",
				"user_id", userID,
				"groups_count", len(groups),
				"error", err,
			)
			roles = make(map[string]string)
		} else {
			slog.Info("resolved project roles from OIDC groups",
				"user_id", userID,
				"groups_count", len(groups),
				"roles_count", len(roles),
				"projects", func() []string {
					ps := make([]string, 0, len(roles))
					for p := range roles {
						ps = append(ps, p)
					}
					return ps
				}(),
			)
		}
	} else if len(groups) == 0 {
		slog.Warn("no OIDC groups provided for project role resolution",
			"user_id", userID,
			"help", "OIDC token must contain groups claim for project access",
		)
	}

	// Build projects list from OIDC-derived roles
	var projects []string
	var defaultProject string
	if len(roles) > 0 {
		projectSet := make(map[string]struct{})
		for projectID := range roles {
			if _, exists := projectSet[projectID]; !exists {
				projects = append(projects, projectID)
				projectSet[projectID] = struct{}{}
				if defaultProject == "" {
					defaultProject = projectID
				}
			}
		}
	}

	// Get Casbin roles for the user
	var casbinRoles []string
	if s.roleManager != nil {
		userCasbinRoles, err := s.roleManager.GetRolesForUser(userID)
		if err != nil {
			slog.Warn("failed to get Casbin roles for JWT",
				"user_id", userID,
				"error", err,
			)
		} else {
			casbinRoles = userCasbinRoles
		}
	}

	// Compute permissions for frontend UI (ArgoCD-aligned pattern)
	permissions := s.computePermissions(userID, casbinRoles)

	jti := uuid.New().String()

	claims := JWTClaims{
		UserID:         userID,
		Email:          email,
		DisplayName:    displayName,
		Projects:       projects,
		DefaultProject: defaultProject,
		Groups:         groups,
		Roles:          roles,
		CasbinRoles:    casbinRoles,
		Permissions:    permissions,
		JTI:            jti,
		ExpiresAt:      expiresAt.Unix(),
		IssuedAt:       now.Unix(),
	}

	mapClaims := jwt.MapClaims{
		"sub":             claims.UserID,
		"email":           claims.Email,
		"name":            claims.DisplayName,
		"projects":        claims.Projects,
		"default_project": claims.DefaultProject,
		"iss":             "knodex",
		"aud":             "knodex-api",
		"exp":             claims.ExpiresAt,
		"iat":             claims.IssuedAt,
		"jti":             jti,
	}

	// Only include groups if present
	if len(groups) > 0 {
		mapClaims["groups"] = groups
	}

	// Include roles if resolved from OIDC groups
	if len(roles) > 0 {
		mapClaims["roles"] = roles
	}

	if len(casbinRoles) > 0 {
		mapClaims["casbin_roles"] = casbinRoles
	}

	if len(permissions) > 0 {
		mapClaims["permissions"] = permissions
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)

	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// splitProjectRole splits a Casbin project role like "proj:myproject:admin" into parts
func splitProjectRole(role string) []string {
	parts := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(role); i++ {
		if role[i] == ':' {
			parts = append(parts, role[start:i])
			start = i + 1
		}
	}
	if start < len(role) {
		parts = append(parts, role[start:])
	}
	return parts
}

// computePermissions calculates UI-relevant permissions based on Casbin policies.
// This enables the frontend to make optimistic UI decisions without checking role strings.
// The backend remains the source of truth for authorization (ArgoCD-aligned pattern).
//
// Returned permissions:
//   - *:* - Admin wildcard (full access)
//   - settings:get - Can view settings
//   - settings:update - Can modify settings
//   - projects:get - Can view projects
//   - projects:create - Can create new projects
//   - projects:update - Can update projects
//   - projects:delete - Can delete projects
//   - instances:get - Can view instances
//   - instances:create - Can create/deploy instances
//   - instances:delete - Can delete instances
//   - repositories:get - Can view repositories
//   - repositories:create - Can create repositories
//   - repositories:delete - Can delete repositories
//   - rgds:get - Can view RGDs (catalog)
//
// Project-scoped permissions are computed as {resource}:{action}:{projectName}
// (e.g., "instances:create:my-project")
func (s *Service) computePermissions(userID string, casbinRoles []string) map[string]bool {
	if s.roleManager == nil {
		return nil
	}

	permissions := make(map[string]bool)

	// Check if user has server admin role
	for _, role := range casbinRoles {
		if role == rbac.CasbinRoleServerAdmin {
			// Admin gets all permissions
			permissions["*:*"] = true
			permissions["settings:get"] = true
			permissions["settings:update"] = true
			permissions["projects:get"] = true
			permissions["projects:create"] = true
			permissions["projects:update"] = true
			permissions["projects:delete"] = true
			permissions["instances:get"] = true
			permissions["instances:create"] = true
			permissions["instances:delete"] = true
			permissions["repositories:get"] = true
			permissions["repositories:create"] = true
			permissions["repositories:delete"] = true
			permissions["rgds:get"] = true
			return permissions
		}
	}

	// Check specific permissions via Casbin Enforce
	// Key UI permissions that affect frontend rendering
	permissionChecks := []struct {
		resource string
		action   string
		key      string
	}{
		// Settings permissions
		{"settings/*", "get", "settings:get"},
		{"settings/*", "update", "settings:update"},
		// Project permissions
		{"projects/*", "get", "projects:get"},
		{"projects/*", "create", "projects:create"},
		{"projects/*", "update", "projects:update"},
		{"projects/*", "delete", "projects:delete"},
		// Instance permissions
		{"instances/*", "get", "instances:get"},
		{"instances/*", "create", "instances:create"},
		{"instances/*", "delete", "instances:delete"},
		// Repository permissions
		{"repositories/*", "get", "repositories:get"},
		{"repositories/*", "create", "repositories:create"},
		{"repositories/*", "delete", "repositories:delete"},
		// RGD (catalog) permissions
		{"rgds/*", "get", "rgds:get"},
	}

	for _, check := range permissionChecks {
		// Try user directly
		allowed, err := s.roleManager.Enforce(userID, check.resource, check.action)
		if err != nil {
			slog.Warn("failed to check permission",
				"user_id", userID,
				"resource", check.resource,
				"action", check.action,
				"error", err,
			)
			continue
		}

		// Also check via roles if not already allowed
		if !allowed {
			for _, role := range casbinRoles {
				allowed, err = s.roleManager.Enforce(role, check.resource, check.action)
				if err != nil {
					continue
				}
				if allowed {
					break
				}
			}
		}

		permissions[check.key] = allowed
	}

	// Add project-scoped permissions based on project roles in casbinRoles
	// Project roles are in format "proj:projectName:roleName"
	for _, role := range casbinRoles {
		if len(role) > 5 && role[:5] == "proj:" {
			parts := splitProjectRole(role)
			if len(parts) >= 3 {
				projectName := parts[1]
				roleName := parts[2]

				// Map project role to granular permissions
				switch roleName {
				case "admin":
					// Project admin can do everything in their project
					permissions["projects:update:"+projectName] = true
					permissions["projects:get:"+projectName] = true
					permissions["instances:create:"+projectName] = true
					permissions["instances:delete:"+projectName] = true
					permissions["instances:get:"+projectName] = true
					permissions["repositories:create:"+projectName] = true
					permissions["repositories:delete:"+projectName] = true
					permissions["repositories:get:"+projectName] = true
				case "developer":
					// Developer can create/delete instances but not manage project
					permissions["projects:get:"+projectName] = true
					permissions["instances:create:"+projectName] = true
					permissions["instances:delete:"+projectName] = true
					permissions["instances:get:"+projectName] = true
					permissions["repositories:get:"+projectName] = true
				case "readonly", "viewer":
					// Readonly/viewer can only view
					permissions["projects:get:"+projectName] = true
					permissions["instances:get:"+projectName] = true
					permissions["repositories:get:"+projectName] = true
				}
			}
		}
	}

	return permissions
}

// CanI checks if the current user can perform an action on a resource/subresource.
// This is the ArgoCD-style permission check endpoint implementation.
// It evaluates Casbin policies in real-time for the authenticated user.
//
// Parameters:
//   - userID: The authenticated user's ID
//   - groups: OIDC groups for the user (used for group-based policies)
//   - resource: The resource type (e.g., "instances", "projects", "repositories")
//   - action: The action (e.g., "create", "delete", "get", "update")
//   - subresource: Optional subresource (e.g., project name, namespace) - use "-" or empty for none
//
// Returns:
//   - bool: true if the user is allowed to perform the action
//   - error: any error that occurred during policy evaluation
func (s *Service) CanI(userID string, groups []string, resource, action, subresource string) (bool, error) {
	if s.roleManager == nil {
		return false, fmt.Errorf("casbin enforcer not configured")
	}

	// Build the object path for Casbin
	var object string
	var wildcardObject string // Also check wildcard pattern for project-scoped permissions
	isGenericCheck := subresource == "" || subresource == "-"
	if isGenericCheck {
		object = resource + "/*"
	} else {
		object = resource + "/" + subresource
		// If subresource doesn't contain a slash (it's just a project name),
		// also check with wildcard to match policies like "repositories/proj/*"
		// This handles the case where user has permission for "repositories/proj/*"
		// (all repos in project) and we're checking "can create repos in project"
		if !strings.Contains(subresource, "/") {
			wildcardObject = resource + "/" + subresource + "/*"
		}
	}

	// Helper to check permission with both exact and wildcard patterns
	checkPermission := func(subject string) (bool, error) {
		allowed, err := s.roleManager.Enforce(subject, object, action)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
		// If exact match didn't work and we have a wildcard pattern, try that too
		if wildcardObject != "" {
			allowed, err = s.roleManager.Enforce(subject, wildcardObject, action)
			if err != nil {
				return false, err
			}
			if allowed {
				return true, nil
			}
		}
		return false, nil
	}

	// Helper to check if user has permission for ANY project for a resource/action
	// This handles the case where frontend checks "can I create instances?" without specifying a project
	// We need to check if user has any project-scoped policy that allows this action
	checkAnyProjectPermission := func(roles []string) (bool, error) {
		for _, role := range roles {
			policies, err := s.roleManager.GetPoliciesForRole(role)
			if err != nil {
				continue
			}
			// Policy format: [subject, object, action, effect]
			for _, policy := range policies {
				if len(policy) < 4 {
					continue
				}
				policyObj := policy[1]
				policyAct := policy[2]
				policyEft := policy[3]

				// Check if this policy matches: resource/{project}/*, action, allow
				// For example: instances/proj-a/*, create, allow
				if policyEft != "allow" {
					continue
				}

				// Check if action matches (using * for wildcard)
				actionMatches := policyAct == "*" || policyAct == action

				// Check if object is for the same resource type
				// e.g., policyObj = "instances/proj-a/*", resource = "instances"
				if actionMatches && strings.HasPrefix(policyObj, resource+"/") {
					return true, nil
				}
			}
		}
		return false, nil
	}

	// First check direct user permission
	allowed, err := checkPermission(userID)
	if err != nil {
		return false, fmt.Errorf("failed to enforce policy: %w", err)
	}
	if allowed {
		return true, nil
	}

	// Get all user's roles (direct roles)
	userRoles, err := s.roleManager.GetRolesForUser(userID)
	if err != nil {
		slog.Warn("failed to get Casbin roles for user",
			"user_id", userID,
			"error", err,
		)
		userRoles = []string{}
	}

	// Check via user's Casbin roles
	for _, role := range userRoles {
		allowed, err = checkPermission(role)
		if err != nil {
			continue
		}
		if allowed {
			return true, nil
		}
	}

	// Get roles from OIDC groups
	var groupRoles []string
	for _, group := range groups {
		groupSubject := "group:" + group
		roles, err := s.roleManager.GetRolesForUser(groupSubject)
		if err != nil {
			continue
		}
		groupRoles = append(groupRoles, roles...)
	}

	// Check via OIDC groups (group-based policies)
	for _, group := range groups {
		// Groups are stored with "group:" prefix in Casbin
		groupSubject := "group:" + group
		allowed, err = checkPermission(groupSubject)
		if err != nil {
			continue
		}
		if allowed {
			return true, nil
		}
	}

	// Check via group roles
	for _, role := range groupRoles {
		allowed, err = checkPermission(role)
		if err != nil {
			continue
		}
		if allowed {
			return true, nil
		}
	}

	// If this is a generic check (no specific subresource) and we haven't found permission yet,
	// check if user has permission for ANY project-scoped resource
	// This handles the case where frontend checks "can I create instances anywhere?" for UI visibility
	if isGenericCheck {
		allRoles := append(userRoles, groupRoles...)
		if len(allRoles) > 0 {
			anyAllowed, err := checkAnyProjectPermission(allRoles)
			if err == nil && anyAllowed {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetMappedGroups returns only the OIDC groups that have associated role mappings.
// A group is considered "mapped" if either:
//  1. "group:{name}" has any roles assigned in Casbin (via Project CRD spec.roles[].groups), OR
//  2. The group matches a configured OIDC_GROUP_MAPPINGS entry (global or project role mapping)
//
// Both paths must be checked because OIDC_GROUP_MAPPINGS assigns roles to the user at
// login time (g, user:<email>, role:X), not to the group in Casbin (g, group:<name>, role:X).
func (s *Service) GetMappedGroups(groups []string) ([]string, error) {
	if len(groups) == 0 {
		return []string{}, nil
	}
	if s.roleManager == nil && s.groupMapper == nil {
		return []string{}, nil
	}

	// Pre-compute which groups match OIDC_GROUP_MAPPINGS config.
	// EvaluateMappings checks all groups at once (efficient for wildcard patterns).
	configMappedSet := make(map[string]bool)
	if s.groupMapper != nil {
		result := s.groupMapper.EvaluateMappings(groups)
		if len(result.GlobalRoles) > 0 || len(result.ProjectMemberships) > 0 {
			// At least one group matched a config mapping.
			// Identify which specific groups matched by testing each individually.
			for _, group := range groups {
				singleResult := s.groupMapper.EvaluateMappings([]string{group})
				if len(singleResult.GlobalRoles) > 0 || len(singleResult.ProjectMemberships) > 0 {
					configMappedSet[group] = true
				}
			}
		}
	}

	var mapped []string
	for _, group := range groups {
		// Check 1: Group has config-based mapping (OIDC_GROUP_MAPPINGS)
		if configMappedSet[group] {
			mapped = append(mapped, group)
			continue
		}

		// Check 2: Group has Casbin grouping policy (Project CRD spec.roles[].groups)
		if s.roleManager != nil {
			roles, err := s.roleManager.GetRolesForUser("group:" + group)
			if err != nil {
				slog.Warn("failed to check Casbin roles for group",
					"group", group,
					"error", err,
				)
				continue
			}
			if len(roles) > 0 {
				mapped = append(mapped, group)
			}
		}
	}

	if mapped == nil {
		mapped = []string{}
	}
	return mapped, nil
}

// GetAccountStore returns the account store for external use (e.g., password change endpoints)
func (s *Service) GetAccountStore() *AccountStore {
	return s.accountStore
}

// ValidateToken validates a JWT token and extracts claims
// Uses library built-in validation to prevent timing attacks
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*JWTClaims, error) {
	// Parse token with explicit algorithm whitelist, issuer/audience validation, and built-in expiration validation
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check the signing method type (not just the string) to prevent algorithm confusion
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method type: %v (expected HMAC)", token.Header["alg"])
		}
		// Double-check algorithm name with exact match
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected algorithm: %v (expected HS256)", token.Method.Alg())
		}
		return []byte(s.config.JWTSecret), nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer("knodex"),
		jwt.WithAudience("knodex-api"),
	)

	if err != nil {
		// Library handles expiration validation in constant time
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Extract and validate required claims
	jwtClaims := &JWTClaims{}

	// Required: subject (user ID)
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil, fmt.Errorf("missing or invalid 'sub' claim")
	}
	jwtClaims.UserID = sub

	// Required: email
	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("missing or invalid 'email' claim")
	}
	jwtClaims.Email = email

	// Required: name
	name, ok := claims["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing or invalid 'name' claim")
	}
	jwtClaims.DisplayName = name

	// Required: projects (can be empty array or nil)
	projectsInterface, ok := claims["projects"]
	if !ok {
		return nil, fmt.Errorf("missing 'projects' claim")
	}

	// Handle nil as empty array (for local admin with no projects)
	if projectsInterface == nil {
		jwtClaims.Projects = []string{}
	} else {
		projects, ok := projectsInterface.([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid 'projects' claim type")
		}
		jwtClaims.Projects = make([]string, 0, len(projects))
		for i, project := range projects {
			projectStr, ok := project.(string)
			if !ok || projectStr == "" {
				return nil, fmt.Errorf("invalid project at index %d", i)
			}
			jwtClaims.Projects = append(jwtClaims.Projects, projectStr)
		}
	}

	// Optional: default_project
	if defaultProject, ok := claims["default_project"].(string); ok {
		jwtClaims.DefaultProject = defaultProject
	}

	// Optional: casbin_roles (Casbin roles for authorization)
	// Frontend uses permissions map for UI decisions (ArgoCD-aligned pattern)
	if casbinRolesInterface, ok := claims["casbin_roles"]; ok && casbinRolesInterface != nil {
		switch v := casbinRolesInterface.(type) {
		case []interface{}:
			jwtClaims.CasbinRoles = make([]string, 0, len(v))
			for _, r := range v {
				if roleStr, ok := r.(string); ok && roleStr != "" {
					jwtClaims.CasbinRoles = append(jwtClaims.CasbinRoles, roleStr)
				}
			}
		case []string:
			jwtClaims.CasbinRoles = v
		}
	}

	// Optional: groups (OIDC groups for runtime authorization)
	// Groups enable Project CRD spec.roles.groups to grant access via ArgoCD-style evaluation
	if groupsInterface, ok := claims["groups"]; ok && groupsInterface != nil {
		switch v := groupsInterface.(type) {
		case []interface{}:
			jwtClaims.Groups = make([]string, 0, len(v))
			for _, g := range v {
				if groupStr, ok := g.(string); ok && groupStr != "" {
					jwtClaims.Groups = append(jwtClaims.Groups, groupStr)
				}
			}
		case []string:
			jwtClaims.Groups = v
		}
	}

	// Optional: roles (project ID -> role name mapping for frontend permission checks)
	// Resolved from OIDC groups at token generation time
	if rolesInterface, ok := claims["roles"]; ok && rolesInterface != nil {
		switch v := rolesInterface.(type) {
		case map[string]interface{}:
			jwtClaims.Roles = make(map[string]string, len(v))
			for projectID, roleInterface := range v {
				if roleStr, ok := roleInterface.(string); ok && roleStr != "" {
					jwtClaims.Roles[projectID] = roleStr
				}
			}
		case map[string]string:
			jwtClaims.Roles = v
		}
	}

	// Optional: permissions (pre-computed permissions for frontend UI - ArgoCD-aligned)
	// These are computed at token time based on Casbin policies
	if permissionsInterface, ok := claims["permissions"]; ok && permissionsInterface != nil {
		switch v := permissionsInterface.(type) {
		case map[string]interface{}:
			jwtClaims.Permissions = make(map[string]bool, len(v))
			for key, valueInterface := range v {
				if valueBool, ok := valueInterface.(bool); ok {
					jwtClaims.Permissions[key] = valueBool
				}
			}
		case map[string]bool:
			jwtClaims.Permissions = v
		}
	}

	// Required: expiration time
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'exp' claim")
	}
	jwtClaims.ExpiresAt = int64(exp)

	// Required: issued at time
	iat, ok := claims["iat"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'iat' claim")
	}
	jwtClaims.IssuedAt = int64(iat)

	// Library already validated expiration in constant time
	// Additional validation: check issued-at is not in future (allow 60s clock skew)
	now := time.Now().Unix()
	if jwtClaims.IssuedAt > now+60 {
		return nil, fmt.Errorf("invalid token: issued in future")
	}

	// Check if token was invalidated by password change (local users only).
	// This adds a K8s account lookup per local-user request; the AccountStore
	// cache (with TTL-based refresh) prevents excessive K8s API pressure.
	if strings.HasPrefix(jwtClaims.UserID, "user-local-") {
		username := strings.TrimPrefix(jwtClaims.UserID, "user-local-")
		issuedAt := time.Unix(jwtClaims.IssuedAt, 0)
		valid, err := s.accountStore.IsTokenValid(ctx, username, issuedAt)
		if err != nil {
			slog.WarnContext(ctx, "failed to validate token against password change",
				"user_id", jwtClaims.UserID,
				"error", err,
			)
			return nil, fmt.Errorf("token validation failed: unable to verify account status")
		}
		if !valid {
			return nil, fmt.Errorf("token invalidated by password change")
		}
	}

	// Check jti blacklist last — all other validation (expiry, iat, password change)
	// takes priority so the most specific rejection reason is returned.
	if jti, ok := claims["jti"].(string); ok && jti != "" {
		jwtClaims.JTI = jti
		if s.blacklist != nil {
			revoked, err := s.blacklist.IsRevoked(ctx, jti)
			if err != nil {
				slog.WarnContext(ctx, "failed to check JWT blacklist, allowing token",
					"jti", jti,
					"error", err,
				)
				// fail-open: continue without blocking
			} else if revoked {
				return nil, fmt.Errorf("session has been revoked")
			}
		}
	}

	return jwtClaims, nil
}

// RevokeToken blacklists a JWT by its jti claim so it cannot be reused
func (s *Service) RevokeToken(ctx context.Context, jti string, remainingTTL time.Duration) error {
	if s.blacklist == nil {
		return nil
	}
	return s.blacklist.RevokeToken(ctx, jti, remainingTTL)
}

// IsLocalLoginEnabled reports whether the local user login pathway is active.
// See ServiceInterface doc for the two preconditions.
func (s *Service) IsLocalLoginEnabled() bool {
	return s.config.LocalLoginEnabled && s.config.LocalAdminPassword != ""
}

// GetBootstrapService returns the project bootstrap service
func (s *Service) GetBootstrapService() *ProjectBootstrapService {
	return s.bootstrapService
}
