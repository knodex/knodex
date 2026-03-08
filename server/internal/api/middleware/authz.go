package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

// Legacy Authz middleware and related types removed.
// All authorization now uses CasbinAuthz middleware exclusively.
// See CasbinAuthz below for the unified authorization implementation.

// writeJSONError writes a JSON error response
func writeJSONError(w http.ResponseWriter, statusCode int, code, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	if details != nil {
		errorResponse["error"].(map[string]interface{})["details"] = details
	}

	// Add request ID if available
	if requestID := w.Header().Get("X-Request-ID"); requestID != "" {
		errorResponse["error"].(map[string]interface{})["request_id"] = requestID
	}

	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		slog.Error("failed to write error response", "error", err)
	}
}

// containsControlChars returns true if s contains any ASCII control character
// (0x00–0x1F or DEL 0x7F) that could cause string truncation attacks.
func containsControlChars(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b <= 0x1F || b == 0x7F {
			return true
		}
	}
	return false
}

// matchesPathPrefix securely matches a path against a prefix
// Prevents path traversal attacks by ensuring exact prefix matching
// SECURITY: Rejects empty and root path prefixes to prevent matching all paths
func matchesPathPrefix(requestPath, pathPrefix string) bool {
	// SECURITY: Reject empty and root path prefixes to prevent matching all paths
	// An empty or "/" prefix would match every request, bypassing authorization
	if pathPrefix == "" || pathPrefix == "/" {
		return false
	}

	// Normalize both paths (remove trailing slashes for consistent comparison)
	requestPath = strings.TrimRight(requestPath, "/")
	pathPrefix = strings.TrimRight(pathPrefix, "/")

	// Exact match
	if requestPath == pathPrefix {
		return true
	}

	// Prefix match: request must start with pathPrefix followed by /
	// This ensures /api/v1/instances matches /api/v1/instances/123
	// But /api/v1/instancesmalicious does NOT match /api/v1/instances
	return strings.HasPrefix(requestPath+"/", pathPrefix+"/")
}

// GetUserProjects returns the list of project IDs the user has access to.
// Note: Primarily used for testing purposes
func GetUserProjects(r *http.Request) []string {
	userCtx, ok := GetUserContext(r)
	if !ok {
		return []string{}
	}
	return userCtx.Projects
}

// GetUserID returns the user ID from the request context.
// Note: Primarily used for testing purposes
func GetUserID(r *http.Request) (string, error) {
	userCtx, ok := GetUserContext(r)
	if !ok {
		return "", fmt.Errorf("no user context found")
	}
	return userCtx.UserID, nil
}

// ============================================================================
// Casbin PolicyEnforcer-based Authorization Middleware
// ============================================================================

// CasbinPolicyEnforcer defines the interface for Casbin-based policy enforcement
// This interface is satisfied by rbac.PolicyEnforcer
type CasbinPolicyEnforcer interface {
	CanAccess(ctx context.Context, user, object, action string) (bool, error)
	// CanAccessWithGroups checks if user OR any of their OIDC groups OR any of their server-side
	// Casbin roles can perform action on object.
	// This enables Project CRD spec.roles.groups to grant access via runtime group evaluation.
	// Groups are checked against Casbin grouping policies (g, group:<name>, role).
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
	// HasRole checks if a user has a specific role (for global admin check)
	HasRole(ctx context.Context, user, role string) (bool, error)
}

// RBACReadyChecker checks whether RBAC policies have been synced.
// During startup, before the initial policy sync completes, returns false
// so the middleware can return 503 instead of a misleading 403.
type RBACReadyChecker interface {
	IsPolicySynced() bool
}

// CasbinAuthzConfig holds configuration for Casbin authorization middleware
type CasbinAuthzConfig struct {
	Enforcer     CasbinPolicyEnforcer
	ReadyChecker RBACReadyChecker
	Logger       *slog.Logger
}

// CasbinAuthz returns middleware that enforces Casbin policies
// This middleware uses PolicyEnforcer.CanAccess() for authorization decisions
func CasbinAuthz(config CasbinAuthzConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user context (should be set by Auth middleware)
			userCtx, ok := GetUserContext(r)
			if !ok {
				if config.Logger != nil {
					config.Logger.Warn("authorization failed: missing user context",
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
					)
				}
				writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required", nil)
				return
			}

			// Defense-in-depth: return 503 during startup before RBAC policies are loaded.
			// This prevents misleading 403 responses when Casbin has no policies yet.
			if config.ReadyChecker != nil && !config.ReadyChecker.IsPolicySynced() {
				if config.Logger != nil {
					config.Logger.Warn("RBAC policies not yet synced, returning 503",
						slog.String("path", r.URL.Path),
						slog.String("user_id", userCtx.UserID),
					)
				}
				writeJSONError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
					"authorization service initializing, please retry", nil)
				return
			}

			// Clean the path to prevent traversal attacks
			cleanPath := path.Clean(r.URL.Path)
			if cleanPath != r.URL.Path {
				if config.Logger != nil {
					config.Logger.Warn("path traversal attempt detected",
						slog.String("original_path", r.URL.Path),
						slog.String("clean_path", cleanPath),
						slog.String("user_id", userCtx.UserID),
						slog.String("method", r.Method),
					)
				}
				writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid path", nil)
				return
			}

			// Reject null bytes and control characters in path parameters (LOG-VULN-02)
			if containsControlChars(r.URL.Path) || (r.URL.RawPath != "" && containsControlChars(r.URL.RawPath)) {
				if config.Logger != nil {
					config.Logger.Warn("null byte or control character in path parameter",
						slog.String("original_path", r.URL.Path),
						slog.String("user_id", userCtx.UserID),
						slog.String("method", r.Method),
					)
				}
				writeJSONError(w, http.StatusBadRequest, "BAD_REQUEST",
					"invalid path parameter: contains null byte or control character", nil)
				return
			}

			// Users with "*, *" permission (granted via role:serveradmin policies) can access all resources
			if config.Enforcer != nil {
				hasAdminPermission, err := config.Enforcer.CanAccess(r.Context(), userCtx.UserID, "*", "*")
				if err != nil {
					if config.Logger != nil {
						config.Logger.Warn("failed to check admin permissions",
							slog.String("user_id", userCtx.UserID),
							slog.String("error", err.Error()),
						)
					}
					// Fall through to normal authorization on error
				} else if hasAdminPermission {
					if config.Logger != nil {
						config.Logger.Debug("authorization granted: admin permission (wildcard access)",
							slog.String("user_id", userCtx.UserID),
							slog.String("path", cleanPath),
							slog.String("method", r.Method),
						)
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			// RGD Catalog Hybrid Authorization Model
			// All authenticated users can access the RGD catalog API (list/get RGDs).
			// The RGD handler filters which specific RGDs each user can see based on:
			// - knodex.io/catalog: "true" is the GATEWAY to the catalog
			// - catalog: true (no project label) → visible to ALL authenticated users
			// - catalog: true + knodex.io/project → visible only to project members
			// - No catalog annotation → NOT in catalog (invisible to everyone)
			// This implements the hybrid Casbin + Handler filtering model where:
			// - Casbin (route level): Controls WHO can access the API (all authenticated users)
			// - Handler (data level): Controls WHICH RGDs users see (project-based filtering)
			if isRGDCatalogRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: RGD catalog access (hybrid model)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Project List Hybrid Authorization Model (similar to RGD Catalog)
			// All authenticated users can list projects via GET /api/v1/projects.
			// The project handler filters which specific projects each user can see based on:
			// - Global admin: sees all projects
			// - Regular user: sees only projects they have access to (via CanAccessWithGroups)
			// This implements the same hybrid Casbin + Handler filtering model as RGDs.
			if isProjectListRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: project list access (hybrid model)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Instance List/Count Hybrid Authorization Model (similar to RGD Catalog)
			// All authenticated users can list/count instances via GET /api/v1/instances*.
			// The instance handler filters which specific instances each user can see based on:
			// - Global admin: sees all instances (getUserNamespaces returns nil)
			// - Regular user: sees only instances in their project namespaces (via OIDC groups)
			// This implements the same hybrid Casbin + Handler filtering model as RGDs.
			if isInstanceListRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: instance list access (hybrid model)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Instance Create - Delegated Authorization Model
			// Instance creation requires project context (projectId from request body) to check
			// the correct permission: "instances/{projectId}/*" instead of generic "instances/*".
			//
			// Authorization is DELEGATED to DeploymentValidator middleware which performs:
			// 1. Project existence validation
			// 2. Project-scoped permission check: CanAccessWithGroups(instances/{projectId}/*, create)
			// 3. Source repository policy validation
			// 4. Destination namespace policy validation
			//
			// This is MORE thorough than CasbinAuthz could do with just URL path information.
			// SECURITY: User is still authenticated (JWT validated). DeploymentValidator enforces authorization.
			if isInstanceCreateRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization delegated to DeploymentValidator",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
						slog.String("reason", "requires project context from request body"),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Project Namespace Hybrid Authorization Model
			// The project namespace handler already performs proper authorization using
			// CanAccessWithGroups with object "projects/{name}". The middleware would
			// incorrectly construct object "projects/{name}/namespaces" which doesn't
			// match any policy. We bypass middleware and let the handler do authorization.
			if isProjectNamespaceRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: project namespace access (hybrid model)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// License Status Endpoint - Readable by all authenticated users
			// GET /api/v1/license returns license status without any role requirement.
			// POST /api/v1/license (update) has its own admin check in the handler.
			if isLicenseStatusRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: license status access (all authenticated users)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// WebSocket Ticket - accessible to ALL authenticated users
			// POST /api/v1/ws/ticket generates a short-lived ticket (30s TTL, single-use)
			// that embeds the user's identity from the JWT. This is a credential format
			// translation (JWT -> opaque ticket), not a resource access decision.
			// The WebSocket handler validates the ticket and enforces per-connection auth.
			if isWSTicketRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: WebSocket ticket (all authenticated users)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Account endpoints - accessible to ALL authenticated users (JWT already validated).
			// Both can-i and info are self-referencing endpoints (users checking their own data).
			// No Casbin resource-level authorization needed - authentication is sufficient.
			if isAccountCanIRequest(cleanPath, r.Method) || isAccountInfoRequest(cleanPath, r.Method) {
				if config.Logger != nil {
					config.Logger.Debug("authorization granted: account endpoint access (all authenticated users)",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
						slog.String("method", r.Method),
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// SECURITY: Deny access if enforcer not configured (fail-safe)
			if config.Enforcer == nil {
				if config.Logger != nil {
					config.Logger.Error("casbin enforcer not configured - denying access",
						slog.String("user_id", userCtx.UserID),
						slog.String("path", cleanPath),
					)
				}
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN",
					"authorization system not available", nil)
				return
			}

			// Determine resource object and action from request
			object, action := inferCasbinObjectAndAction(r, cleanPath)

			// Extract user's OIDC groups from JWT claims in context
			// This enables Project CRD spec.roles.groups to grant access
			groups, err := GetUserGroupsFromContext(r.Context())
			if err != nil {
				// Groups extraction failure is not fatal - continue with empty groups
				// This maintains backward compatibility for non-OIDC users
				if config.Logger != nil {
					config.Logger.Debug("could not extract groups from context",
						slog.String("user_id", userCtx.UserID),
						slog.String("error", err.Error()),
					)
				}
				groups = []string{}
			}

			// Check authorization using Casbin PolicyEnforcer with groups and roles
			// CanAccessWithGroups enables Project CRD spec.roles.groups to grant access
			// via runtime group evaluation (ArgoCD-style)

			allowed, err := config.Enforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, groups, object, action)
			if err != nil {
				if config.Logger != nil {
					config.Logger.Error("casbin authorization check failed",
						slog.String("user_id", userCtx.UserID),
						slog.String("object", object),
						slog.String("action", action),
						slog.Int("groups_count", len(groups)),
						slog.String("error", err.Error()),
					)
				}
				writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
					"failed to check permissions", nil)
				return
			}

			if !allowed {
				if config.Logger != nil {
					config.Logger.Warn("authorization denied",
						slog.String("user_id", userCtx.UserID),
						slog.String("object", object),
						slog.String("action", action),
					)
				}
				writeJSONError(w, http.StatusForbidden, "FORBIDDEN",
					"insufficient permissions",
					map[string]interface{}{
						"object": object,
						"action": action,
					})
				return
			}

			// Authorization succeeded
			if config.Logger != nil {
				config.Logger.Debug("authorization granted",
					slog.String("user_id", userCtx.UserID),
					slog.String("object", object),
					slog.String("action", action),
				)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserFromJWTContext extracts user identity from JWT claims in context
// This is used when working with raw JWT claims rather than UserContext
func GetUserFromJWTContext(ctx context.Context) (string, error) {
	claims, ok := ctx.Value(jwtClaimsContextKey).(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("jwt claims not found in context")
	}

	// Extract subject (user identifier) from OIDC claims
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", fmt.Errorf("sub claim missing or invalid")
	}

	return sub, nil
}

// jwtClaimsContextKey is the context key for JWT claims
type jwtClaimsKey struct{}

var jwtClaimsContextKey = jwtClaimsKey{}

// GetUserGroupsFromContext extracts user groups from JWT claims in context
func GetUserGroupsFromContext(ctx context.Context) ([]string, error) {
	claims, ok := ctx.Value(jwtClaimsContextKey).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("jwt claims not found in context")
	}

	// Extract groups claim (OIDC standard)
	groupsClaim, ok := claims["groups"]
	if !ok {
		return []string{}, nil // No groups is valid - return empty list
	}

	// Handle different group claim formats from various OIDC providers
	switch v := groupsClaim.(type) {
	case []interface{}:
		groups := make([]string, 0, len(v))
		for _, g := range v {
			if groupStr, ok := g.(string); ok {
				groups = append(groups, groupStr)
			}
		}
		return groups, nil
	case []string:
		return v, nil
	default:
		return nil, fmt.Errorf("groups claim has invalid type: %T", groupsClaim)
	}
}

// inferCasbinObjectAndAction extracts resource object and action from HTTP request
// Object format: {resource_type}/{resource_name} (e.g., "projects/engineering")
// Action: HTTP method mapped to action (get, list, create, update, delete)
func inferCasbinObjectAndAction(r *http.Request, cleanPath string) (object, action string) {
	// Strip API prefix to get resource path
	resourcePath := strings.TrimPrefix(cleanPath, "/api/v1/")

	// Determine action from HTTP method
	switch r.Method {
	case http.MethodGet:
		// Distinguish between get (single resource) and list (collection)
		pathParts := strings.Split(resourcePath, "/")
		if len(pathParts) > 1 && pathParts[len(pathParts)-1] != "" {
			action = "get"
		} else {
			action = "list"
		}
	case http.MethodPost:
		action = "create"
	case http.MethodPut, http.MethodPatch:
		action = "update"
	case http.MethodDelete:
		action = "delete"
	default:
		action = "unknown"
	}

	// Determine object from path
	// Examples:
	//   /api/v1/projects -> projects/*
	//   /api/v1/projects/engineering -> projects/engineering
	//   /api/v1/instances/default/WebApp/my-app -> instances/default/WebApp/my-app
	//   /api/v1/rgds -> rgds/*
	pathParts := strings.Split(resourcePath, "/")
	if len(pathParts) > 0 && pathParts[0] != "" {
		resource := pathParts[0] // "projects", "instances", "rgds"
		if len(pathParts) > 1 && pathParts[1] != "" {
			// Specific resource - build full path
			object = strings.Join(pathParts, "/")
		} else {
			// Collection (list operation) - use wildcard
			object = resource + "/*"
		}
	} else {
		object = "*"
	}

	return object, action
}

// WithJWTClaims adds JWT claims to the request context
// This should be called by authentication middleware after validating the token
func WithJWTClaims(ctx context.Context, claims map[string]interface{}) context.Context {
	return context.WithValue(ctx, jwtClaimsContextKey, claims)
}

// isRGDCatalogRequest checks if the request is for RGD catalog access (list or get RGDs)
// RGD Catalog Hybrid Authorization Model
// All authenticated users can access the RGD catalog API.
// The RGD handler filters which specific RGDs each user can see based on visibility labels.
func isRGDCatalogRequest(path, method string) bool {
	// Only allow GET requests (list and get operations)
	if method != http.MethodGet {
		return false
	}

	// Check if path is for RGDs
	// Matches: /api/v1/rgds, /api/v1/rgds/, /api/v1/rgds/{name}
	return strings.HasPrefix(path, "/api/v1/rgds")
}

// isProjectListRequest checks if the request is for listing projects
// Similar to RGD Catalog, all authenticated users can list projects.
// The project handler filters which specific projects each user can see based on their roles.
func isProjectListRequest(path, method string) bool {
	// Only allow GET requests for the projects collection
	if method != http.MethodGet {
		return false
	}

	// Match exactly /api/v1/projects (list) but NOT /api/v1/projects/{name} (get specific)
	// The list endpoint uses hybrid model - handler filters based on user's project access
	cleanPath := strings.TrimRight(path, "/")
	return cleanPath == "/api/v1/projects"
}

// isInstanceListRequest checks if the request is for listing or counting instances
// Instance Hybrid Authorization Model: All authenticated users can list/count instances.
// The instance handler filters which specific instances each user can see based on their
// project namespaces (via getUserNamespaces which resolves OIDC groups to project destinations).
func isInstanceListRequest(path, method string) bool {
	// Only allow GET requests (list, count, pending, stuck operations)
	if method != http.MethodGet {
		return false
	}

	cleanPath := strings.TrimRight(path, "/")

	// Match instance collection endpoints that use handler-level filtering:
	// - /api/v1/instances (list)
	// - /api/v1/instances/count (count)
	// - /api/v1/instances/pending (pending instances)
	// - /api/v1/instances/stuck (stuck instances)
	// Note: /api/v1/instances/{namespace}/{kind}/{name} requires specific instance access
	return cleanPath == "/api/v1/instances" ||
		cleanPath == "/api/v1/instances/count" ||
		cleanPath == "/api/v1/instances/pending" ||
		cleanPath == "/api/v1/instances/stuck"
}

// isInstanceCreateRequest checks if the request is for creating an instance
// Instance Create Hybrid Authorization Model: Instance creation is handled by the
// DeploymentValidator middleware which has access to the request body (projectId).
// The DeploymentValidator checks project-scoped permissions using "instances/{projectId}/*"
// which matches both built-in roles (instances/*) and project roles (instances/proj-name/*).
func isInstanceCreateRequest(path, method string) bool {
	if method != http.MethodPost {
		return false
	}

	cleanPath := strings.TrimRight(path, "/")
	return cleanPath == "/api/v1/instances"
}

// isProjectNamespaceRequest checks if the request is for listing namespaces in a project
// Project Namespace Hybrid Authorization Model: The handler already performs proper
// authorization using CanAccessWithGroups with object "projects/{name}" (not the full path).
// The middleware would incorrectly construct object "projects/{name}/namespaces" which
// doesn't match any policy, so we bypass it here and let the handler do the authorization.
func isProjectNamespaceRequest(path, method string) bool {
	// Only allow GET requests
	if method != http.MethodGet {
		return false
	}

	cleanPath := strings.TrimRight(path, "/")

	// Match /api/v1/projects/{name}/namespaces
	// Must start with /api/v1/projects/ and end with /namespaces
	if !strings.HasPrefix(cleanPath, "/api/v1/projects/") {
		return false
	}

	// Extract the part after /api/v1/projects/
	remainder := strings.TrimPrefix(cleanPath, "/api/v1/projects/")
	parts := strings.Split(remainder, "/")

	// Should have exactly 2 parts: {projectName} and "namespaces"
	return len(parts) == 2 && parts[0] != "" && parts[1] == "namespaces"
}

// isLicenseStatusRequest checks if the request is for reading license status
// License status is readable by ALL authenticated users (no role required).
// The UpdateLicense handler has its own admin permission check.
func isLicenseStatusRequest(path, method string) bool {
	if method != http.MethodGet {
		return false
	}

	cleanPath := strings.TrimRight(path, "/")
	return cleanPath == "/api/v1/license"
}

// isAccountCanIRequest checks if the request is for the ArgoCD-style can-i endpoint
// Account Can-I Hybrid Authorization Model: The can-i endpoint is accessible to ALL
// authenticated users. It's a reflection API that allows users to check their own
// permissions. The handler uses the correct Casbin object format (e.g., "repositories/project/*")
// rather than the URL path format (e.g., "account/can-i/repositories/create/project").
func isAccountCanIRequest(path, method string) bool {
	// Only allow GET requests
	if method != http.MethodGet {
		return false
	}

	// Match /api/v1/account/can-i/{resource}/{action}/{subresource}
	return strings.HasPrefix(path, "/api/v1/account/can-i/")
}

// isAccountInfoRequest checks if the request is for the account info endpoint.
// The info endpoint is accessible to ALL authenticated users (no RBAC needed).
// Users can always view their own identity, groups, roles, and token metadata.
func isAccountInfoRequest(path, method string) bool {
	if method != http.MethodGet {
		return false
	}
	cleanPath := strings.TrimRight(path, "/")
	return cleanPath == "/api/v1/account/info"
}

// isWSTicketRequest checks if the request is for WebSocket ticket creation.
// POST /api/v1/ws/ticket generates a short-lived, single-use ticket that
// embeds the user's JWT identity. This is a credential format translation,
// not a resource access decision. Authentication (JWT) is sufficient.
func isWSTicketRequest(path, method string) bool {
	if method != http.MethodPost {
		return false
	}
	cleanPath := strings.TrimRight(path, "/")
	return cleanPath == "/api/v1/ws/ticket"
}
