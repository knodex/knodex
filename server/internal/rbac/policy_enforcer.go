package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Common errors for PolicyEnforcer
var (
	// ErrAccessDenied is returned when a user doesn't have permission to perform an action
	ErrAccessDenied = errors.New("access denied")

	// ErrInvalidPolicy is returned when a policy string is malformed
	ErrInvalidPolicy = errors.New("invalid policy")

	// ErrProjectNotFound is returned when a project doesn't exist
	ErrProjectNotFound = errors.New("project not found")

	// ErrInvalidRole is returned when a role reference is invalid
	ErrInvalidRole = errors.New("invalid role")

	// ErrAlreadyExists is returned when attempting to create a resource that already exists
	ErrAlreadyExists = errors.New("resource already exists")

	// ErrNotFound is returned when a resource is not found (generic)
	ErrNotFound = errors.New("resource not found")

	// ErrConflict is returned when there is a version conflict during update
	ErrConflict = errors.New("resource version conflict")
)

// resourceTypeMappings maps singular resource types from ArgoCD policies
// to plural forms used in HTTP routes
var resourceTypeMappings = map[string]string{
	"rgd":        "rgds",
	"instance":   "instances",
	"project":    "projects",
	"repository": "repositories",
	// Keep * as-is for wildcard
	"*": "*",
}

// normalizeResourceType converts singular resource types to plural forms
// to match HTTP route naming conventions. If no mapping exists, returns
// the input unchanged.
func normalizeResourceType(resourceType string) string {
	if plural, ok := resourceTypeMappings[strings.ToLower(resourceType)]; ok {
		return plural
	}
	return resourceType
}

// normalizeAction expands ArgoCD-style actions to HTTP method actions
// ArgoCD uses: view, create, update, delete, *
// HTTP routes use: list, get, create, update, delete
// "view" expands to ["list", "get"] for read-only access
func normalizeAction(action string) []string {
	switch strings.ToLower(action) {
	case "view":
		// "view" grants read-only access (list and get)
		return []string{"list", "get"}
	case "*":
		// Wildcard keeps as-is (Casbin glob matches all)
		return []string{"*"}
	case "deploy":
		// "deploy" maps to "create" (creating instances)
		return []string{"create"}
	default:
		// Other actions pass through unchanged
		return []string{action}
	}
}

// ProjectReader provides access to Project CRDs for policy loading
// This interface abstracts the Kubernetes client for testing
type ProjectReader interface {
	// GetProject retrieves a project by name
	GetProject(ctx context.Context, name string) (*Project, error)

	// ListProjects retrieves all projects
	ListProjects(ctx context.Context) ([]Project, error)

	// ProjectExists checks if a project exists
	ProjectExists(ctx context.Context, name string) (bool, error)

	// FindProjectForNamespace finds the project that owns a given namespace
	// by checking project destinations. This implements ArgoCD-style namespace
	// to project resolution for authorization.
	// Returns the project if found, nil if no project owns the namespace.
	FindProjectForNamespace(ctx context.Context, namespace string) (*Project, error)
}

// PolicyEnforcerConfig holds configuration for PolicyEnforcer with caching
type PolicyEnforcerConfig struct {
	// CacheEnabled determines if authorization caching is enabled
	CacheEnabled bool

	// CacheTTL is the time-to-live for cache entries
	CacheTTL time.Duration

	// Logger is the structured logger for policy enforcement logging
	Logger *slog.Logger
}

// DefaultPolicyEnforcerConfig returns default configuration
func DefaultPolicyEnforcerConfig() PolicyEnforcerConfig {
	return PolicyEnforcerConfig{
		CacheEnabled: true,
		CacheTTL:     5 * time.Minute,
		Logger:       slog.Default(),
	}
}

// PolicyMetrics tracks policy enforcement metrics
type PolicyMetrics struct {
	PolicyReloads   int64 // atomic counter
	BackgroundSyncs int64 // atomic counter
	WatcherRestarts int64 // atomic counter
}

// ============================================================================
// Focused Interfaces (Interface Segregation Principle)
// ============================================================================
// These focused interfaces allow consumers to depend only on the methods they need.
// This improves testability (smaller mocks) and makes dependencies explicit.

// Authorizer provides authorization checks for RBAC decisions.
// Use this interface when you only need to check permissions.
type Authorizer interface {
	// CanAccess checks if user can perform action on object
	// Returns true if access is allowed, false otherwise
	// User format: "user:{email}" or "user:{id}"
	// Object format: "{resource_type}/{resource_name}"
	// Action: get, list, create, update, delete
	CanAccess(ctx context.Context, user, object, action string) (bool, error)

	// CanAccessWithGroups checks if user OR any of their OIDC groups OR any of their server-side
	// Casbin roles can perform action on object.
	// This enables Project CRD spec.roles.groups to grant access via runtime group evaluation.
	// Groups are checked against Casbin grouping policies (g, group:<name>, role).
	// Roles are sourced from Casbin's authoritative state (GetImplicitRolesForUser), NOT from JWT.
	// Returns true if user has permission OR any group has permission OR any role has permission.
	// This follows ArgoCD's runtime group and role evaluation pattern.
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)

	// EnforceProjectAccess enforces project-specific access
	// Returns nil if access is allowed, ErrAccessDenied if denied
	EnforceProjectAccess(ctx context.Context, user, projectName, action string) error

	// GetAccessibleProjects returns the list of project names the user can access
	// This uses a UNIFIED approach - same code path for all users including global admins
	// Global admins get all projects because their Casbin policies grant access to all,
	// NOT because of a special code path.
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error)

	// HasRole checks if a user has a specific role
	HasRole(ctx context.Context, user, role string) (bool, error)
}

// PolicyLoader handles loading and syncing policies from Project CRDs.
// Use this interface for policy synchronization operations.
type PolicyLoader interface {
	// LoadProjectPolicies loads policies from a Project CRD into Casbin
	// Policies are loaded from project.Spec.Roles
	// This operation is idempotent (can be called multiple times)
	LoadProjectPolicies(ctx context.Context, project *Project) error

	// SyncPolicies synchronizes all Project policies from Kubernetes
	// This removes stale policies from deleted projects
	SyncPolicies(ctx context.Context) error

	// RemoveProjectPolicies removes all policies for a project
	// Used when a Project is deleted
	RemoveProjectPolicies(ctx context.Context, projectName string) error
}

// RoleManager handles user role assignments.
// Use this interface for role binding operations.
// Note: HasRole is in Authorizer interface since it's used for authorization checks.
type RoleManager interface {
	// AssignUserRoles assigns roles to a user, replacing existing roles
	// Supports global roles ("role:serveradmin") and project roles ("proj:project:developer")
	AssignUserRoles(ctx context.Context, user string, roles []string) error

	// GetUserRoles returns all roles for a user
	// Returns both directly assigned and inherited roles
	GetUserRoles(ctx context.Context, user string) ([]string, error)

	// RemoveUserRoles removes all roles from a user
	RemoveUserRoles(ctx context.Context, user string) error

	// RemoveUserRole removes a specific role from a user
	// Used for targeted role removal in role binding endpoints
	RemoveUserRole(ctx context.Context, user, role string) error

	// RestorePersistedRoles loads all user-role assignments from Redis into Casbin.
	// Should be called once during startup, after SyncPolicies has loaded project policies.
	// No-op if Redis role store is not configured.
	RestorePersistedRoles(ctx context.Context) error
}

// CacheController manages the authorization cache.
// Use this interface when you need to invalidate cached authorization decisions.
type CacheController interface {
	// InvalidateCache clears all cached authorization decisions
	// Should be called when policies are reloaded or modified
	InvalidateCache()

	// InvalidateCacheForUser invalidates cache entries for a specific user
	// Returns the number of entries removed
	// Should be called when user's roles or project membership changes
	InvalidateCacheForUser(user string) int

	// InvalidateCacheForProject invalidates cache entries for a project
	// Returns the number of entries removed
	// Should be called when project policies change
	InvalidateCacheForProject(projectName string) int

	// CacheStats returns cache performance statistics
	// Returns empty stats if caching is disabled
	CacheStats() CacheStats
}

// MetricsProvider exposes policy enforcement metrics for observability.
// Use this interface for monitoring and metrics endpoints.
type MetricsProvider interface {
	// Metrics returns policy enforcement metrics
	Metrics() PolicyMetrics

	// IncrementPolicyReloads increments the policy reload counter
	IncrementPolicyReloads()

	// IncrementBackgroundSyncs increments the background sync counter
	IncrementBackgroundSyncs()

	// IncrementWatcherRestarts increments the watcher restart counter
	IncrementWatcherRestarts()
}

// ============================================================================
// Composite Interface
// ============================================================================

// PolicyEnforcer is a composite interface that embeds all focused interfaces.
// Prefer using focused interfaces (Authorizer, PolicyLoader, etc.) in your code.
// This composite exists for DI wiring where a single dependency is needed.
type PolicyEnforcer interface {
	Authorizer
	PolicyLoader
	RoleManager
	CacheController
	MetricsProvider
}

// SyncResult contains summary information about a sync operation
type SyncResult struct {
	// ProjectsLoaded is the number of projects successfully processed
	ProjectsLoaded int

	// PoliciesAdded is the total number of policies added
	PoliciesAdded int

	// Errors contains any non-fatal errors during sync
	Errors []error
}

// policyEnforcer implements PolicyEnforcer
type policyEnforcer struct {
	enforcer      *CasbinEnforcer
	projectReader ProjectReader
	mu            sync.RWMutex

	// loadedProjects tracks which projects have policies loaded
	// Used to detect stale policies during sync
	loadedProjects map[string]bool

	// Authorization cache (optional)
	cache        AuthorizationCache
	cacheEnabled bool
	cacheTTL     time.Duration

	// Redis role store for persisting user-role assignments across restarts (optional)
	roleStore *RedisRoleStore

	// Structured logger
	logger *slog.Logger

	// Metrics
	metrics PolicyMetrics
}

// Compile-time interface compliance assertions
var (
	_ Authorizer      = (*policyEnforcer)(nil)
	_ PolicyLoader    = (*policyEnforcer)(nil)
	_ RoleManager     = (*policyEnforcer)(nil)
	_ CacheController = (*policyEnforcer)(nil)
	_ MetricsProvider = (*policyEnforcer)(nil)
	_ PolicyEnforcer  = (*policyEnforcer)(nil)
)

// NewPolicyEnforcer creates a new PolicyEnforcer with default configuration.
// The returned PolicyEnforcer can be assigned to any focused interface:
// Authorizer, PolicyLoader, RoleManager, CacheController, MetricsProvider.
func NewPolicyEnforcer(enforcer *CasbinEnforcer, projectReader ProjectReader) PolicyEnforcer {
	return NewPolicyEnforcerWithConfig(enforcer, projectReader, DefaultPolicyEnforcerConfig())
}

// PolicyEnforcerOption is a functional option for configuring PolicyEnforcer
type PolicyEnforcerOption func(*policyEnforcer)

// WithRedisRoleStore sets the Redis role store for persisting user-role assignments
func WithRedisRoleStore(store *RedisRoleStore) PolicyEnforcerOption {
	return func(pe *policyEnforcer) {
		pe.roleStore = store
	}
}

// NewPolicyEnforcerWithConfig creates a new PolicyEnforcer with custom configuration.
// The returned PolicyEnforcer can be assigned to any focused interface:
// Authorizer, PolicyLoader, RoleManager, CacheController, MetricsProvider.
func NewPolicyEnforcerWithConfig(enforcer *CasbinEnforcer, projectReader ProjectReader, config PolicyEnforcerConfig, opts ...PolicyEnforcerOption) PolicyEnforcer {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	pe := &policyEnforcer{
		enforcer:       enforcer,
		projectReader:  projectReader,
		loadedProjects: make(map[string]bool),
		cacheEnabled:   config.CacheEnabled,
		cacheTTL:       config.CacheTTL,
		logger:         logger,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(pe)
	}

	// Initialize cache if enabled
	if config.CacheEnabled {
		pe.cache = NewAuthorizationCache()
		logger.Info("authorization cache enabled",
			"ttl", config.CacheTTL.String(),
		)
	} else {
		logger.Info("authorization cache disabled")
	}

	if pe.roleStore != nil {
		logger.Info("redis role store enabled for user-role persistence")
	}

	return pe
}

// CanAccess checks if user can perform action on object
// If caching is enabled, cached decisions are returned when available
func (pe *policyEnforcer) CanAccess(ctx context.Context, user, object, action string) (bool, error) {
	// Validate user format
	if user == "" {
		return false, fmt.Errorf("%w: user cannot be empty", ErrAccessDenied)
	}

	// Validate object format
	if object == "" {
		return false, fmt.Errorf("%w: object cannot be empty", ErrAccessDenied)
	}

	// Validate action
	if action == "" {
		return false, fmt.Errorf("%w: action cannot be empty", ErrAccessDenied)
	}

	// Check cache first if enabled
	if pe.cacheEnabled && pe.cache != nil {
		if allowed, found := pe.cache.Get(user, object, action); found {
			pe.logger.Debug("cache hit",
				"user", user,
				"object", object,
				"action", action,
				"allowed", allowed,
			)
			return allowed, nil
		}
	}

	// Cache miss or cache disabled - check Casbin
	pe.mu.RLock()
	allowed, err := pe.enforcer.Enforce(user, object, action)
	pe.mu.RUnlock()

	if err != nil {
		pe.logger.Error("policy enforcement failed",
			"user", user,
			"object", object,
			"action", action,
			"error", err,
		)
		return false, fmt.Errorf("policy enforcement error: %w", err)
	}

	// Cache the result if enabled
	if pe.cacheEnabled && pe.cache != nil && pe.cacheTTL > 0 {
		pe.cache.Set(user, object, action, allowed, pe.cacheTTL)
		pe.logger.Debug("cache set",
			"user", user,
			"object", object,
			"action", action,
			"allowed", allowed,
			"ttl", pe.cacheTTL.String(),
		)
	}

	return allowed, nil
}

// CanAccessWithGroups checks if user OR any of their OIDC groups OR any of their server-side
// Casbin roles can perform action on object.
// This implements ArgoCD-style runtime group and role evaluation for Project CRD spec.roles.groups.
// Groups are checked against Casbin grouping policies (g, group:<name>, role).
// Roles are sourced from the server's authoritative Casbin state via GetImplicitRolesForUser,
// NOT from JWT claims. This ensures role revocations take effect immediately.
// Returns true if user has permission OR any group has permission OR any role has permission.
//
// ArgoCD-style Namespace Resolution:
// When the object contains a namespace (e.g., "instances/ns-beta-team/foo"), this method
// also tries to resolve the namespace to its owning project via destinations and check
// authorization with the project-scoped object (e.g., "instances/proj-beta-team/*").
// This matches ArgoCD's two-step authorization: 1) check role, 2) check destination.
//

func (pe *policyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	// Build list of objects to check: original object + project-scoped version if namespace found
	objectsToCheck := []string{object}

	// ArgoCD-style namespace-to-project resolution
	// If the object contains a namespace (e.g., "instances/ns-beta-team/foo"),
	// try to resolve it to the owning project and create a project-scoped object
	if projectScopedObject := pe.resolveNamespaceToProjectObject(ctx, object); projectScopedObject != "" {
		objectsToCheck = append(objectsToCheck, projectScopedObject)
		pe.logger.Debug("resolved namespace to project-scoped object",
			"original_object", object,
			"project_scoped_object", projectScopedObject,
		)
	}

	// For wildcard list operations (e.g., "instances/*" with action "list"),
	// also check project-scoped objects for each accessible project.
	// This enables users to list resources even when they only have access via
	// project-specific policies (e.g., "instances/proj-beta-team/*").
	// The handler will filter results to only show accessible items.
	if action == "list" && strings.HasSuffix(object, "/*") && pe.projectReader != nil {
		resourceType := strings.TrimSuffix(object, "/*") // e.g., "instances"
		additionalObjects := pe.getProjectScopedWildcards(ctx, groups, resourceType)
		if len(additionalObjects) > 0 {
			objectsToCheck = append(objectsToCheck, additionalObjects...)
			pe.logger.Debug("added project-scoped wildcards for list operation",
				"resource_type", resourceType,
				"additional_objects", additionalObjects,
			)
		}
	}

	// 1. First check direct user permission for all objects
	for _, obj := range objectsToCheck {
		allowed, err := pe.CanAccess(ctx, user, obj, action)
		if err != nil {
			return false, err
		}
		if allowed {
			pe.logger.Debug("access granted via direct user permission",
				"user", user,
				"object", obj,
				"action", action,
			)
			return true, nil
		}
	}

	// 2. Check each OIDC group (ArgoCD-style runtime evaluation)
	// Groups inherit permissions from roles via Casbin grouping policies:
	//   g, group:<group-name>, proj:<project>:<role>
	if len(groups) > 0 {
		pe.logger.Debug("checking OIDC groups for access",
			"user", user,
			"groups_count", len(groups),
			"objects_to_check", objectsToCheck,
			"action", action,
		)
	}
	for _, group := range groups {
		if group == "" {
			continue
		}

		groupSubject, err := FormatGroupSubject(group)
		if err != nil {
			pe.logger.Debug("skipping invalid group name",
				"group", group,
				"error", err,
			)
			continue // Skip invalid group names
		}

		// Check permission for all objects (original + project-scoped)
		for _, obj := range objectsToCheck {
			// Check cache first if enabled
			if pe.cacheEnabled && pe.cache != nil {
				if cachedAllowed, found := pe.cache.Get(groupSubject, obj, action); found {
					if cachedAllowed {
						pe.logger.Debug("access granted via group (cached)",
							"user", user,
							"group", group,
							"object", obj,
							"action", action,
						)
						return true, nil
					}
					continue // Cached deny, try next object
				}
			}

			// Check Casbin for group permission
			pe.mu.RLock()
			groupAllowed, err := pe.enforcer.Enforce(groupSubject, obj, action)
			pe.mu.RUnlock()

			if err != nil {
				pe.logger.Warn("group permission check failed",
					"user", user,
					"group", group,
					"object", obj,
					"error", err,
				)
				continue
			}

			// Log Casbin enforcement result for debugging
			pe.logger.Debug("Casbin group enforcement result",
				"group_subject", groupSubject,
				"object", obj,
				"action", action,
				"allowed", groupAllowed,
			)

			// Cache the result if enabled
			if pe.cacheEnabled && pe.cache != nil && pe.cacheTTL > 0 {
				pe.cache.Set(groupSubject, obj, action, groupAllowed, pe.cacheTTL)
			}

			if groupAllowed {
				pe.logger.Debug("access granted via group permission",
					"user", user,
					"group", group,
					"object", obj,
					"action", action,
				)
				return true, nil
			}
		}
	}

	// 3. Check server-side Casbin roles (authoritative source, NOT from JWT)
	// SECURITY: Roles are sourced from Casbin's state via GetImplicitRolesForUser,
	// ensuring role revocations take effect immediately. A compromised JWT with stale
	// casbin_roles cannot be used to escalate privileges.
	// PERF TODO: When called from GetAccessibleProjects (N projects), this fetches
	// the same roles N times. Consider hoisting the lookup into GetAccessibleProjects.
	//
	// FORMAT NOTE: Role assignments may be stored with or without "user:" prefix
	// depending on the source (local admin login vs OIDC auto_role_binder).
	// We check both formats to ensure consistent behavior regardless of storage format.
	pe.mu.RLock()
	serverRoles, rolesErr := pe.enforcer.GetImplicitRolesForUser(user)
	// Also check the alternate format (with/without "user:" prefix)
	if rolesErr == nil && len(serverRoles) == 0 {
		var altUser string
		if strings.HasPrefix(user, "user:") {
			altUser = strings.TrimPrefix(user, "user:")
		} else {
			altUser = "user:" + user
		}
		altRoles, altErr := pe.enforcer.GetImplicitRolesForUser(altUser)
		if altErr == nil && len(altRoles) > 0 {
			serverRoles = altRoles
		}
	}
	pe.mu.RUnlock()
	if len(serverRoles) > 0 {
		pe.logger.Debug("checking server-side Casbin roles for access",
			"user", user,
			"roles_count", len(serverRoles),
			"objects_to_check", objectsToCheck,
			"action", action,
		)
	}
	if rolesErr != nil {
		pe.logger.Warn("failed to get server-side roles for user",
			"user", user,
			"error", rolesErr,
		)
		// Don't fail the entire check - user and group checks already ran.
		// Log the error but continue to the deny path.
	} else {
		for _, role := range serverRoles {
			if role == "" {
				continue
			}

			// Check permission for all objects (original + project-scoped)
			for _, obj := range objectsToCheck {
				// Check cache first if enabled
				if pe.cacheEnabled && pe.cache != nil {
					if cachedAllowed, found := pe.cache.Get(role, obj, action); found {
						if cachedAllowed {
							pe.logger.Debug("access granted via role (cached)",
								"user", user,
								"role", role,
								"object", obj,
								"action", action,
							)
							return true, nil
						}
						continue // Cached deny, try next object
					}
				}

				// Check Casbin for role permission
				pe.mu.RLock()
				roleAllowed, err := pe.enforcer.Enforce(role, obj, action)
				pe.mu.RUnlock()

				if err != nil {
					pe.logger.Warn("role permission check failed",
						"user", user,
						"role", role,
						"object", obj,
						"error", err,
					)
					continue
				}

				// Cache the result if enabled
				if pe.cacheEnabled && pe.cache != nil && pe.cacheTTL > 0 {
					pe.cache.Set(role, obj, action, roleAllowed, pe.cacheTTL)
				}

				if roleAllowed {
					pe.logger.Debug("access granted via server-side role permission",
						"user", user,
						"role", role,
						"object", obj,
						"action", action,
					)
					return true, nil
				}
			}
		}
	}

	pe.logger.Debug("access denied - no matching user, group, or role permission",
		"user", user,
		"groups_checked", len(groups),
		"objects_checked", objectsToCheck,
		"action", action,
	)
	return false, nil
}

// resolveNamespaceToProjectObject implements ArgoCD-style namespace-to-project resolution.
// Given an object like "instances/ns-beta-team/foo", it:
// 1. Extracts the namespace ("ns-beta-team")
// 2. Finds the project that owns this namespace via destinations
// 3. Returns a project-scoped object ("instances/proj-beta-team/*")
//
// Returns empty string if:
// - Object doesn't contain a namespace segment
// - No project owns the namespace
// - ProjectReader is not configured
func (pe *policyEnforcer) resolveNamespaceToProjectObject(ctx context.Context, object string) string {
	// Only resolve for objects that might contain namespaces
	// Format: "instances/{namespace}/{kind}/{name}" or "instances/{namespace}"
	if !strings.HasPrefix(object, "instances/") {
		return ""
	}

	// Parse the object path
	parts := strings.Split(object, "/")
	if len(parts) < 2 {
		return ""
	}

	// Extract namespace (second part after "instances/")
	namespace := parts[1]
	if namespace == "" || namespace == "*" {
		return "" // No namespace or wildcard - nothing to resolve
	}

	// Check if we have a ProjectReader
	if pe.projectReader == nil {
		pe.logger.Debug("namespace resolution skipped - no project reader configured",
			"namespace", namespace,
		)
		return ""
	}

	// Find the project that owns this namespace
	project, err := pe.projectReader.FindProjectForNamespace(ctx, namespace)
	if err != nil || project == nil {
		pe.logger.Debug("no project found for namespace",
			"namespace", namespace,
			"error", err,
		)
		return ""
	}

	// Create project-scoped object: "instances/{project-id}/*"
	// This matches how policies are defined: "instances/proj-beta-team/*"
	resourceType := parts[0] // "instances"
	return resourceType + "/" + project.Name + "/*"
}

// getProjectScopedWildcards returns project-scoped wildcard objects for the user's accessible projects.
// For example, if the user's groups have access to proj-beta-team and proj-alpha-team,
// this returns ["instances/proj-beta-team/*", "instances/proj-alpha-team/*"] for resourceType "instances".
// This enables list operations to check project-specific policies.
func (pe *policyEnforcer) getProjectScopedWildcards(ctx context.Context, groups []string, resourceType string) []string {
	var wildcards []string
	seenProjects := make(map[string]bool)

	// For each group, find projects they have access to via grouping policies
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	for _, group := range groups {
		if group == "" {
			continue
		}

		groupSubject, err := FormatGroupSubject(group)
		if err != nil {
			continue
		}

		// Get roles this group inherits from (via Casbin grouping policies)
		// Format: g, group:<name>, proj:<project>:<role>
		roles, err := pe.enforcer.GetRolesForUser(groupSubject)
		if err != nil {
			pe.logger.Debug("failed to get roles for group",
				"group", group,
				"error", err,
			)
			continue
		}

		// Extract project names from role strings
		// Role format: proj:<project>:<role>
		for _, role := range roles {
			if strings.HasPrefix(role, "proj:") {
				parts := strings.Split(role, ":")
				if len(parts) >= 2 {
					projectName := parts[1]
					if !seenProjects[projectName] {
						seenProjects[projectName] = true
						wildcard := resourceType + "/" + projectName + "/*"
						wildcards = append(wildcards, wildcard)
					}
				}
			}
		}
	}

	return wildcards
}

// EnforceProjectAccess enforces project-specific access
func (pe *policyEnforcer) EnforceProjectAccess(ctx context.Context, user, projectName, action string) error {
	// Validate project name
	if projectName == "" {
		return fmt.Errorf("%w: project name cannot be empty", ErrProjectNotFound)
	}

	// Format object as projects/{projectName}
	object, err := FormatObject(ResourceProjects, projectName)
	if err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	allowed, err := pe.CanAccess(ctx, user, object, action)
	if err != nil {
		return err
	}

	if !allowed {
		return fmt.Errorf("%w: user %s cannot %s project %s", ErrAccessDenied, user, action, projectName)
	}

	return nil
}

// LoadProjectPolicies loads policies from a Project CRD into Casbin
func (pe *policyEnforcer) LoadProjectPolicies(ctx context.Context, project *Project) error {
	if project == nil {
		return fmt.Errorf("%w: project cannot be nil", ErrProjectNotFound)
	}

	if project.Name == "" {
		return fmt.Errorf("%w: project name cannot be empty", ErrProjectNotFound)
	}

	pe.mu.Lock()
	err := pe.loadProjectPoliciesLocked(project)
	pe.mu.Unlock()

	if err != nil {
		pe.logger.Error("failed to load project policies",
			"project", project.Name,
			"error", err,
		)
		return err
	}

	// Invalidate cache when policies change
	if pe.cacheEnabled && pe.cache != nil {
		pe.cache.Clear()
		pe.logger.Info("cache invalidated due to policy reload",
			"project", project.Name,
		)
	}

	// Increment policy reload counter
	pe.IncrementPolicyReloads()

	return nil
}

// getBuiltInAdminPolicies returns built-in policies for project admin roles.
// These policies are automatically applied to any role named "admin" within a project,
// following the ArgoCD pattern where project admins can manage their project resources.
//
// Policies granted:
//   - repositories/{project}/*, *, allow - Full repository management for project
//   - projects/{project}, update, allow - Update project settings
//   - projects/{project}, member-add, allow - Add members to project
//   - projects/{project}, member-remove, allow - Remove members from project
//   - instances/{project}/*, *, allow - Full instance management for project
//   - rgds/*, get, allow - Read access to all RGDs (catalog browsing)
//   - rgds/*, list, allow - List access to all RGDs (catalog browsing)
//   - compliance/{project}/*, get, allow - Read access to compliance data (enterprise feature)
//   - compliance/{project}/*, list, allow - List access to compliance data (enterprise feature)
func getBuiltInAdminPolicies(projectName string) []string {
	return []string{
		// Repository management - scoped to project
		fmt.Sprintf("repositories/%s/*, *, allow", projectName),

		// Project management - update, member-add, member-remove
		fmt.Sprintf("projects/%s, update, allow", projectName),
		fmt.Sprintf("projects/%s, member-add, allow", projectName),
		fmt.Sprintf("projects/%s, member-remove, allow", projectName),

		// Instance management - scoped to project
		fmt.Sprintf("instances/%s/*, *, allow", projectName),

		// RGD read access - global for catalog browsing
		"rgds/*, get, allow",
		"rgds/*, list, allow",

		// Compliance read access - scoped to project (enterprise feature)
		fmt.Sprintf("compliance/%s/*, get, allow", projectName),
		fmt.Sprintf("compliance/%s/*, list, allow", projectName),
	}
}

// getBuiltInReadonlyPolicies returns built-in policies for project readonly roles.
// These policies are automatically applied to any role named "readonly" within a project,
// following the ArgoCD pattern where readonly users can view project resources.
//
// Policies granted:
//   - projects/{project}, get, allow - View project settings
//   - instances/{project}/*, get, allow - View instances in project
//   - instances/{project}/*, list, allow - List instances in project
//   - rgds/*, get, allow - Read access to all RGDs (catalog browsing)
//   - rgds/*, list, allow - List access to all RGDs (catalog browsing)
//   - repositories/{project}/*, get, allow - View repositories in project
//   - repositories/{project}/*, list, allow - List repositories in project
//   - applications/{project}/*, get, allow - View applications in project
//   - applications/{project}/*, list, allow - List applications in project
func getBuiltInReadonlyPolicies(projectName string) []string {
	return []string{
		// Project: view only
		fmt.Sprintf("projects/%s, get, allow", projectName),

		// Instance: view and list only
		fmt.Sprintf("instances/%s/*, get, allow", projectName),
		fmt.Sprintf("instances/%s/*, list, allow", projectName),

		// RGD read access - global for catalog browsing
		"rgds/*, get, allow",
		"rgds/*, list, allow",

		// Repository: view and list only
		fmt.Sprintf("repositories/%s/*, get, allow", projectName),
		fmt.Sprintf("repositories/%s/*, list, allow", projectName),

		// Applications: view and list only
		fmt.Sprintf("applications/%s/*, get, allow", projectName),
		fmt.Sprintf("applications/%s/*, list, allow", projectName),
	}
}

// loadProjectPoliciesLocked loads policies from a Project CRD into Casbin
// IMPORTANT: Caller MUST hold the mutex lock before calling this method
func (pe *policyEnforcer) loadProjectPoliciesLocked(project *Project) error {
	// Remove existing policies for this project
	if err := pe.removeProjectPoliciesLocked(project.Name); err != nil {
		return fmt.Errorf("failed to remove old policies for project %s: %w", project.Name, err)
	}

	// Load new policies from Project.Spec.Roles
	for _, role := range project.Spec.Roles {
		if role.Name == "" {
			continue // Skip roles without names
		}

		// Format role name as proj:{project}:{role}
		roleName, err := FormatProjectRole(project.Name, role.Name)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidRole, err)
		}

		// Add each policy from the role
		// SECURITY: Custom policies from Project CRD are scoped to the project automatically
		// This prevents project admins from granting themselves access to other projects
		// For example, a policy "*, *, allow" becomes scoped to only this project's resources
		for _, policyStr := range role.Policies {
			// Parse and add policy with project scoping
			// Policy format: "object, action, effect" (3-part only for security)
			if err := pe.addProjectScopedPolicyFromString(project.Name, roleName, policyStr); err != nil {
				return fmt.Errorf("%w: failed to add policy for role %s: %v", ErrInvalidPolicy, role.Name, err)
			}
		}

		// Add built-in policies for admin roles
		// Admin roles automatically get repository and member management permissions
		// This follows the ArgoCD pattern where project admins have full control of their project
		if strings.EqualFold(role.Name, "admin") {
			builtInPolicies := getBuiltInAdminPolicies(project.Name)
			pe.logger.Info("adding built-in admin policies for project",
				"project", project.Name,
				"role", role.Name,
				"policies_count", len(builtInPolicies),
			)
			for _, policyStr := range builtInPolicies {
				if err := pe.addPolicyFromString(roleName, policyStr); err != nil {
					return fmt.Errorf("%w: failed to add built-in admin policy for role %s: %v", ErrInvalidPolicy, role.Name, err)
				}
			}
		}

		// Add built-in policies for readonly roles (project-scoped)
		// Readonly roles automatically get view permissions for project resources
		if strings.EqualFold(role.Name, "readonly") {
			builtInPolicies := getBuiltInReadonlyPolicies(project.Name)
			pe.logger.Info("adding built-in readonly policies for project",
				"project", project.Name,
				"role", role.Name,
				"policies_count", len(builtInPolicies),
			)
			for _, policyStr := range builtInPolicies {
				if err := pe.addPolicyFromString(roleName, policyStr); err != nil {
					return fmt.Errorf("%w: failed to add built-in readonly policy for role %s: %v", ErrInvalidPolicy, role.Name, err)
				}
			}
		}

		// Add group-to-role mappings for OIDC groups
		if len(role.Groups) > 0 {
			pe.logger.Info("loading OIDC group mappings for project role",
				"project", project.Name,
				"role", role.Name,
				"groups_count", len(role.Groups),
				"groups", role.Groups,
			)
		}
		for _, group := range role.Groups {
			groupSubject, err := FormatGroupSubject(group)
			if err != nil {
				// Skip invalid group names with warning
				pe.logger.Warn("skipping invalid OIDC group name",
					"project", project.Name,
					"role", role.Name,
					"group", group,
					"error", err,
				)
				continue
			}

			// Add grouping policy: group -> role
			if _, err := pe.enforcer.AddUserRole(groupSubject, roleName); err != nil {
				return fmt.Errorf("failed to add group %s to role %s: %w", group, roleName, err)
			}
			pe.logger.Info("added OIDC group to role mapping",
				"project", project.Name,
				"role", role.Name,
				"group_subject", groupSubject,
				"role_name", roleName,
			)
		}
	}

	// Load user/group role bindings from annotations
	// These are persisted when users are assigned to roles via the API
	// Format: knodex.io/role-bindings annotation contains JSON array of bindings
	if bindings := pe.extractRoleBindingsFromAnnotations(project); len(bindings) > 0 {
		pe.logger.Info("loading role bindings from project annotations",
			"project", project.Name,
			"bindings_count", len(bindings),
		)
		for _, binding := range bindings {
			roleName, err := FormatProjectRole(project.Name, binding.Role)
			if err != nil {
				pe.logger.Warn("skipping invalid role binding",
					"project", project.Name,
					"role", binding.Role,
					"subject", binding.Subject,
					"error", err,
				)
				continue
			}

			var subject string
			if binding.Type == "group" {
				subject, err = FormatGroupSubject(binding.Subject)
				if err != nil {
					pe.logger.Warn("skipping invalid group binding",
						"project", project.Name,
						"group", binding.Subject,
						"error", err,
					)
					continue
				}
			} else {
				// User binding - subject is the user ID directly
				subject = binding.Subject
			}

			// Add the grouping policy
			if _, err := pe.enforcer.AddUserRole(subject, roleName); err != nil {
				pe.logger.Warn("failed to add role binding",
					"project", project.Name,
					"subject", subject,
					"role", roleName,
					"error", err,
				)
				continue
			}
			pe.logger.Debug("loaded role binding from annotation",
				"project", project.Name,
				"type", binding.Type,
				"subject", binding.Subject,
				"role", binding.Role,
			)
		}
	}

	// Mark project as loaded
	pe.loadedProjects[project.Name] = true

	return nil
}

// roleBinding represents a user or group role binding stored in project annotations
type roleBinding struct {
	Role    string `json:"role"`
	Subject string `json:"subject"`
	Type    string `json:"type"` // "user" or "group"
}

// extractRoleBindingsFromAnnotations extracts role bindings from project annotations
func (pe *policyEnforcer) extractRoleBindingsFromAnnotations(project *Project) []roleBinding {
	if project.Annotations == nil {
		return nil
	}

	bindingsJSON, ok := project.Annotations["knodex.io/role-bindings"]
	if !ok || bindingsJSON == "" {
		return nil
	}

	var bindings []roleBinding
	if err := json.Unmarshal([]byte(bindingsJSON), &bindings); err != nil {
		pe.logger.Warn("failed to parse role bindings from annotation",
			"project", project.Name,
			"error", err,
		)
		return nil
	}

	return bindings
}

// addPolicyFromString adds a policy from a policy string
// roleName is the formatted role name (e.g., "proj:myproject:developer")
// policyStr supports two formats:
// 1. Simplified 3-part: "object, action, effect"
// 2. ArgoCD 6-part: "p, subject, resource_type, action, scope, effect"
// Subject from ArgoCD format is IGNORED - always uses roleName to prevent injection attacks.
// SECURITY: Subject is always set to roleName regardless of format.
func (pe *policyEnforcer) addPolicyFromString(roleName, policyStr string) error {
	policyStr = strings.TrimSpace(policyStr)
	if policyStr == "" {
		return nil // Empty policies are ignored
	}

	// SECURITY: Validate policy string length to prevent DoS
	if len(policyStr) > MaxPatternLength {
		return fmt.Errorf("policy string too long (max %d chars): %d", MaxPatternLength, len(policyStr))
	}

	// Split policy string by comma
	parts := strings.Split(policyStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	var subject, object, action, effect string

	// SECURITY: Subject is ALWAYS the roleName - never from user input.
	// This prevents subject injection attacks regardless of policy format.
	subject = roleName

	switch len(parts) {
	case 3:
		// Simplified format: "object, action, effect"
		object = parts[0]
		action = parts[1]
		effect = parts[2]

	case 6:
		// ArgoCD format: "p, subject, resource_type, action, scope, effect"
		// Parts: [p, subject, resource_type, action, scope, effect]
		// We IGNORE parts[0] (p) and parts[1] (subject) for security
		resourceType := parts[2]
		action = parts[3]
		scope := parts[4]
		effect = parts[5]

		// Normalize resource type to plural form to match HTTP routes
		// ArgoCD uses singular (rgd, instance, project) but our routes use plural
		resourceType = normalizeResourceType(resourceType)

		// Construct object path: resource_type/scope
		// Examples:
		// - "project, *, proj-alpha-team, allow" -> "projects/proj-alpha-team"
		// - "instance, *, proj-alpha-team/*, allow" -> "instances/proj-alpha-team/*"
		// - "rgd, view, *, allow" -> "rgds/*"
		if scope == "*" {
			object = resourceType + "/*"
		} else if resourceType == "*" {
			object = scope
		} else {
			object = resourceType + "/" + scope
		}

	default:
		return fmt.Errorf("invalid policy format: must be 3 parts 'object, action, effect' or 6 parts ArgoCD format (got %d parts)", len(parts))
	}

	// Validate effect
	if err := ValidateEffect(effect); err != nil {
		return err
	}

	// Normalize actions (expand "view" to ["list", "get"], etc.)
	actions := normalizeAction(action)

	// Add policy for each normalized action
	for _, normalizedAction := range actions {
		if _, err := pe.enforcer.AddPolicy(subject, object, normalizedAction, effect); err != nil {
			return fmt.Errorf("failed to add policy: %w", err)
		}
	}

	return nil
}

// addProjectScopedPolicyFromString adds a policy that is scoped to a specific project.
// This method is used when loading custom policies from Project CRD spec.roles[].policies.
// It automatically scopes wildcard objects to the project to prevent privilege escalation.
//
// SECURITY: This is critical for multi-tenancy. Without scoping, a policy like "*, *, allow"
// in project "team-a" would grant access to ALL resources including other projects.
// With scoping, it only grants access to resources within "team-a".
//
// Scoping rules:
//   - Object "*" becomes "projects/{project}", "instances/{project}/*", "repositories/{project}/*"
//   - Object "instances/*" becomes "instances/{project}/*"
//   - Object "repositories/*" becomes "repositories/{project}/*"
//   - Fully qualified objects (e.g., "instances/other-project/*") are NOT modified (but will fail authorization)
//
// Supports two formats:
// 1. Simplified 3-part: "object, action, effect"
// 2. ArgoCD 6-part: "p, subject, resource_type, action, scope, effect"
func (pe *policyEnforcer) addProjectScopedPolicyFromString(projectName, roleName, policyStr string) error {
	policyStr = strings.TrimSpace(policyStr)
	if policyStr == "" {
		return nil // Empty policies are ignored
	}

	// SECURITY: Validate policy string length to prevent DoS
	if len(policyStr) > MaxPatternLength {
		return fmt.Errorf("policy string too long (max %d chars): %d", MaxPatternLength, len(policyStr))
	}

	// Split policy string by comma
	parts := strings.Split(policyStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	var object, action, effect string

	switch len(parts) {
	case 3:
		// Simplified format: "object, action, effect"
		object = parts[0]
		action = parts[1]
		effect = parts[2]

	case 6:
		// ArgoCD format: "p, subject, resource_type, action, scope, effect"
		// Parts: [p, subject, resource_type, action, scope, effect]
		// We IGNORE parts[0] (p) and parts[1] (subject) for security
		resourceType := parts[2]
		action = parts[3]
		scope := parts[4]
		effect = parts[5]

		// Normalize resource type to plural form to match HTTP routes
		resourceType = normalizeResourceType(resourceType)

		// Construct object path: resource_type/scope
		if scope == "*" {
			object = resourceType + "/*"
		} else if resourceType == "*" {
			object = scope
		} else {
			object = resourceType + "/" + scope
		}

	default:
		return fmt.Errorf("invalid policy format: must be 3 parts 'object, action, effect' or 6 parts ArgoCD format (got %d parts)", len(parts))
	}

	// Validate effect
	if err := ValidateEffect(effect); err != nil {
		return err
	}

	// SECURITY: Scope wildcard objects to the project
	// This prevents project admins from accessing resources outside their project
	scopedObjects := scopeObjectToProject(projectName, object)

	// Normalize actions (expand "view" to ["list", "get"], etc.)
	actions := normalizeAction(action)

	// Add policy for each scoped object and normalized action
	for _, scopedObject := range scopedObjects {
		for _, normalizedAction := range actions {
			if _, err := pe.enforcer.AddPolicy(roleName, scopedObject, normalizedAction, effect); err != nil {
				return fmt.Errorf("failed to add policy: %w", err)
			}
		}
	}

	return nil
}

// scopeObjectToProject converts wildcard objects to project-scoped objects.
// This is the core of multi-tenant access control.
//
// Examples:
//   - "*" -> ["projects/{project}", "instances/{project}/*", "repositories/{project}/*", ...]
//   - "{project}/*" -> same as "*" (from 6-part ArgoCD format with resourceType="*")
//   - "instances/*" -> ["instances/{project}/*"]
//   - "repositories/*" -> ["repositories/{project}/*"]
//   - "rgds/*" -> ["rgds/*"] (RGDs are global/shared, not project-scoped)
//   - "projects/{other}" -> ["projects/{other}"] (explicit path not modified)
func scopeObjectToProject(projectName, object string) []string {
	// Handle global wildcard - expand to all project-scoped resources
	// Also handle "{project}/*" which results from 6-part ArgoCD format
	// when resourceType="*" and scope="{project}/*" (e.g., bootstrap platform-admin role)
	projectWildcard := projectName + "/*"
	if object == "*" || object == projectWildcard {
		return []string{
			// Project access (only their own project)
			fmt.Sprintf("projects/%s", projectName),
			// Instance management within project
			fmt.Sprintf("instances/%s/*", projectName),
			// Repository management within project
			fmt.Sprintf("repositories/%s/*", projectName),
			// Application management within project
			fmt.Sprintf("applications/%s/*", projectName),
			// RGD access is global (catalog browsing) - not scoped
			"rgds/*",
		}
	}

	// Handle resource-type wildcards
	// "instances/*" -> "instances/{project}/*"
	// "repositories/*" -> "repositories/{project}/*"
	// "applications/*" -> "applications/{project}/*"
	resourcePrefixes := []string{"instances/", "repositories/", "applications/"}
	for _, prefix := range resourcePrefixes {
		if object == prefix+"*" {
			return []string{fmt.Sprintf("%s%s/*", prefix, projectName)}
		}
	}

	// Handle "projects/*" - scope to only their project
	if object == "projects/*" {
		return []string{fmt.Sprintf("projects/%s", projectName)}
	}

	// RGDs are global (catalog browsing), don't scope them
	if strings.HasPrefix(object, "rgds/") {
		return []string{object}
	}

	// Other explicit paths are passed through as-is
	// Note: These will likely fail authorization if they reference other projects
	return []string{object}
}

// SyncPolicies synchronizes all Project policies from Kubernetes
// SECURITY: This method holds the lock for the entire sync operation to prevent
// race conditions (TOCTOU) where the loadedProjects map could be modified during sync.
func (pe *policyEnforcer) SyncPolicies(ctx context.Context) error {
	if pe.projectReader == nil {
		return fmt.Errorf("project service not configured")
	}

	// Get all projects BEFORE acquiring lock (network call should not hold lock)
	projects, err := pe.projectReader.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	// Log discovered projects for debugging OIDC group matching
	projectNames := make([]string, len(projects))
	for i, p := range projects {
		projectNames[i] = p.Name
	}
	pe.logger.Info("SyncPolicies: discovered projects",
		"count", len(projects),
		"projects", projectNames,
	)

	// SECURITY: Hold lock for entire sync operation to prevent race conditions
	pe.mu.Lock()

	// Track which projects we're loading this sync
	currentProjects := make(map[string]bool)

	// Load policies from each project (using locked version)
	var syncErrors []error
	for i := range projects {
		project := &projects[i]
		currentProjects[project.Name] = true

		// Validate project before loading (nil check already done by ListProjects)
		if project.Name == "" {
			syncErrors = append(syncErrors, fmt.Errorf("project with empty name"))
			continue
		}

		// Use locked version since we already hold the lock
		if err := pe.loadProjectPoliciesLocked(project); err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("project %s: %w", project.Name, err))
		}
	}

	// Remove policies for projects that no longer exist
	for projectName := range pe.loadedProjects {
		if !currentProjects[projectName] {
			if err := pe.removeProjectPoliciesLocked(projectName); err != nil {
				syncErrors = append(syncErrors, fmt.Errorf("failed to remove policies for deleted project %s: %w", projectName, err))
			}
			delete(pe.loadedProjects, projectName)
		}
	}

	pe.mu.Unlock()

	// Invalidate cache after sync (outside the lock)
	if pe.cacheEnabled && pe.cache != nil {
		pe.cache.Clear()
		pe.logger.Info("cache invalidated after policy sync",
			"projects_loaded", len(currentProjects),
		)
	}

	// Increment background sync counter
	pe.IncrementBackgroundSyncs()

	if len(syncErrors) > 0 {
		pe.logger.Error("policy sync completed with errors",
			"error_count", len(syncErrors),
			"projects_loaded", len(currentProjects),
			"errors", syncErrors,
		)
		return fmt.Errorf("sync completed with %d errors: %v", len(syncErrors), syncErrors)
	}

	return nil
}

// removeProjectPoliciesLocked removes all policies for a project
// IMPORTANT: Caller MUST hold the mutex lock before calling this method
func (pe *policyEnforcer) removeProjectPoliciesLocked(projectName string) error {
	// Get all policies
	policies, err := pe.enforcer.GetAllPolicies()
	if err != nil {
		return fmt.Errorf("failed to get policies: %w", err)
	}

	// Role prefix for this project
	rolePrefix := fmt.Sprintf("proj:%s:", projectName)

	// Find and remove policies matching this project
	for _, policy := range policies {
		if len(policy) < 1 {
			continue
		}

		// Check if subject starts with project role prefix
		if strings.HasPrefix(policy[0], rolePrefix) {
			// Remove the policy
			// Policy format: [subject, object, action, effect]
			if len(policy) >= 4 {
				if _, err := pe.enforcer.RemovePolicy(policy[0], policy[1], policy[2], policy[3]); err != nil {
					// Log but continue - policy might already be removed
					continue
				}
			}
		}
	}

	// Remove all user/group -> role mappings for project roles
	roles, err := pe.enforcer.GetAllRoles()
	if err != nil {
		return fmt.Errorf("failed to get roles: %w", err)
	}

	for _, role := range roles {
		if strings.HasPrefix(role, rolePrefix) {
			// Get all users/groups that have this role
			users, err := pe.enforcer.GetUsersForRole(role)
			if err != nil {
				continue
			}

			// Remove user/group -> role mappings
			for _, user := range users {
				if _, err := pe.enforcer.RemoveUserRole(user, role); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

// AssignUserRoles assigns roles to a user, replacing existing roles
func (pe *policyEnforcer) AssignUserRoles(ctx context.Context, user string, roles []string) error {
	if user == "" {
		return fmt.Errorf("user cannot be empty")
	}

	// Update Casbin in-memory state (under lock — lock scope excludes Redis I/O
	// to avoid blocking concurrent authorization reads)
	if err := pe.assignUserRolesInCasbin(user, roles); err != nil {
		return err
	}

	// Persist to Redis OUTSIDE the lock — Redis I/O must not block authorization reads.
	// SaveUserRoles handles errors internally via graceful degradation (logs warning, returns nil).
	if pe.roleStore != nil {
		pe.roleStore.SaveUserRoles(ctx, user, roles)
	}

	// Invalidate cache for this user — their authorization decisions may no longer be valid.
	// Uses targeted per-user invalidation instead of full cache clear to prevent
	// cache thrashing during concurrent logins (e.g., Monday morning spikes).
	count := pe.InvalidateCacheForUser(user)
	if count > 0 {
		pe.logger.Info("cache invalidated due to user role assignment",
			"user", user,
			"roles", roles,
			"entries_removed", count,
		)
	}

	return nil
}

// assignUserRolesInCasbin replaces a user's Casbin in-memory role assignments.
// Acquires pe.mu write lock internally. Caller must NOT hold the lock.
func (pe *policyEnforcer) assignUserRolesInCasbin(user string, roles []string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Remove existing roles
	existingRoles, err := pe.enforcer.GetRolesForUser(user)
	if err != nil {
		pe.logger.Error("failed to get existing roles for user",
			"user", user,
			"error", err,
		)
		return fmt.Errorf("failed to get existing roles: %w", err)
	}

	for _, role := range existingRoles {
		if _, err := pe.enforcer.RemoveUserRole(user, role); err != nil {
			pe.logger.Error("failed to remove existing role from user",
				"user", user,
				"role", role,
				"error", err,
			)
			return fmt.Errorf("failed to remove existing role %s: %w", role, err)
		}
	}

	for _, role := range roles {
		if role == "" {
			continue // Skip empty roles
		}

		// Validate role format
		// Valid formats: "role:serveradmin", "role:global-viewer", "proj:project:role"
		if !isValidRoleFormat(role) {
			pe.logger.Error("invalid role format",
				"user", user,
				"role", role,
			)
			return fmt.Errorf("%w: invalid role format: %s", ErrInvalidRole, role)
		}

		if _, err := pe.enforcer.AddUserRole(user, role); err != nil {
			pe.logger.Error("failed to assign role to user",
				"user", user,
				"role", role,
				"error", err,
			)
			return fmt.Errorf("failed to assign role %s: %w", role, err)
		}
	}

	return nil
}

// RestorePersistedRoles loads all user-role assignments from Redis into Casbin.
// This should be called once during startup, after SyncPolicies has loaded project policies.
// It restores user-to-role grouping policies (g policies) that were persisted before restart.
// If the role store is not configured, this is a no-op.
func (pe *policyEnforcer) RestorePersistedRoles(ctx context.Context) error {
	if pe.roleStore == nil {
		pe.logger.Debug("restore persisted roles: skipping, no role store configured")
		return nil
	}

	allRoles, err := pe.roleStore.LoadAllUserRoles(ctx)
	if err != nil {
		return fmt.Errorf("failed to load persisted roles from Redis: %w", err)
	}

	if len(allRoles) == 0 {
		pe.logger.Info("restore persisted roles: no persisted roles found in Redis")
		return nil
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	restoredUsers := 0
	restoredRoles := 0

	for userID, roles := range allRoles {
		userRestoredCount := 0
		for _, role := range roles {
			if role == "" {
				continue
			}

			// Validate role format before restoring
			if !isValidRoleFormat(role) {
				pe.logger.Warn("restore persisted roles: skipping invalid role format",
					"user", userID,
					"role", role,
				)
				continue
			}

			if _, err := pe.enforcer.AddUserRole(userID, role); err != nil {
				pe.logger.Warn("restore persisted roles: failed to add role",
					"user", userID,
					"role", role,
					"error", err,
				)
				continue
			}
			userRestoredCount++
			restoredRoles++
		}
		if userRestoredCount > 0 {
			restoredUsers++
		}
	}

	pe.logger.Info("restore persisted roles: completed",
		"users_restored", restoredUsers,
		"roles_restored", restoredRoles,
	)

	// Invalidate cache after restoring roles
	if pe.cacheEnabled && pe.cache != nil {
		pe.cache.Clear()
	}

	return nil
}

// isValidRoleFormat checks if a role string has a valid format
func isValidRoleFormat(role string) bool {
	// Valid formats:
	// - role:{role-name} (global roles)
	// - proj:{project}:{role} (project-specific roles)
	if strings.HasPrefix(role, "role:") {
		return len(role) > 5 // Must have something after "role:"
	}
	if strings.HasPrefix(role, "proj:") {
		// Must have format proj:project:role
		parts := strings.SplitN(role, ":", 3)
		return len(parts) == 3 && parts[1] != "" && parts[2] != ""
	}
	return false
}

// GetUserRoles returns all roles for a user
func (pe *policyEnforcer) GetUserRoles(ctx context.Context, user string) ([]string, error) {
	if user == "" {
		return nil, fmt.Errorf("user cannot be empty")
	}

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Get implicit roles (includes inherited roles)
	roles, err := pe.enforcer.GetImplicitRolesForUser(user)
	if err != nil {
		pe.logger.Error("failed to get user roles",
			"user", user,
			"error", err,
		)
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	return roles, nil
}

// HasRole checks if a user has a specific role
func (pe *policyEnforcer) HasRole(ctx context.Context, user, role string) (bool, error) {
	if user == "" || role == "" {
		return false, nil
	}

	pe.mu.RLock()
	defer pe.mu.RUnlock()

	// Check direct role assignment
	hasRole, err := pe.enforcer.HasUserRole(user, role)
	if err != nil {
		pe.logger.Error("failed to check user role",
			"user", user,
			"role", role,
			"error", err,
		)
		return false, fmt.Errorf("failed to check role: %w", err)
	}

	if hasRole {
		return true, nil
	}

	// Check inherited roles
	implicitRoles, err := pe.enforcer.GetImplicitRolesForUser(user)
	if err != nil {
		pe.logger.Error("failed to get implicit roles for user",
			"user", user,
			"error", err,
		)
		return false, fmt.Errorf("failed to get implicit roles: %w", err)
	}

	for _, r := range implicitRoles {
		if r == role {
			return true, nil
		}
	}

	return false, nil
}

// RemoveUserRoles removes all roles from a user
func (pe *policyEnforcer) RemoveUserRoles(ctx context.Context, user string) error {
	if user == "" {
		return fmt.Errorf("user cannot be empty")
	}

	// Remove from Casbin in-memory (under lock — lock scope excludes Redis I/O)
	if err := pe.removeUserRolesInCasbin(user); err != nil {
		return err
	}

	// Remove from Redis OUTSIDE the lock — Redis I/O must not block authorization reads.
	// DeleteUserRoles handles errors internally via graceful degradation (logs warning, returns nil).
	if pe.roleStore != nil {
		pe.roleStore.DeleteUserRoles(ctx, user)
	}

	// Invalidate cache — previous authorization decisions may no longer be valid
	if pe.cacheEnabled && pe.cache != nil {
		pe.cache.Clear()
		pe.logger.Info("cache invalidated due to user role removal",
			"user", user,
		)
	}

	return nil
}

// removeUserRolesInCasbin removes all Casbin in-memory role assignments for a user.
// Acquires pe.mu write lock internally. Caller must NOT hold the lock.
func (pe *policyEnforcer) removeUserRolesInCasbin(user string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	existingRoles, err := pe.enforcer.GetRolesForUser(user)
	if err != nil {
		pe.logger.Error("failed to get existing roles for removal",
			"user", user,
			"error", err,
		)
		return fmt.Errorf("failed to get existing roles: %w", err)
	}

	for _, role := range existingRoles {
		if _, err := pe.enforcer.RemoveUserRole(user, role); err != nil {
			pe.logger.Error("failed to remove role from user",
				"user", user,
				"role", role,
				"error", err,
			)
			return fmt.Errorf("failed to remove role %s: %w", role, err)
		}
	}

	return nil
}

// RemoveUserRole removes a specific role from a user
// Used for targeted role removal in role binding endpoints
func (pe *policyEnforcer) RemoveUserRole(ctx context.Context, user, role string) error {
	if user == "" {
		return fmt.Errorf("user cannot be empty")
	}
	if role == "" {
		return fmt.Errorf("role cannot be empty")
	}

	// Remove from Casbin and capture remaining roles (under lock — lock scope excludes Redis I/O)
	remainingRoles, err := pe.removeUserRoleInCasbin(user, role)
	if err != nil {
		return err
	}

	// Update Redis OUTSIDE the lock — Redis I/O must not block authorization reads.
	// SaveUserRoles handles errors internally via graceful degradation (logs warning, returns nil).
	// Note: remainingRoles may include roles from SyncPolicies (annotation-based bindings)
	// in addition to roles assigned via AssignUserRoles. These will be persisted to Redis
	// and restored on restart, but the next OIDC login refreshes all roles via AssignUserRoles.
	if pe.roleStore != nil && remainingRoles != nil {
		pe.roleStore.SaveUserRoles(ctx, user, remainingRoles)
	}

	// Invalidate cache
	if pe.cacheEnabled && pe.cache != nil {
		pe.cache.Clear()
		pe.logger.Info("cache invalidated due to user role removal",
			"user", user,
			"role", role,
		)
	}

	return nil
}

// removeUserRoleInCasbin removes a specific role from Casbin and returns remaining roles for Redis persistence.
// Acquires pe.mu write lock internally. Caller must NOT hold the lock.
// Returns (nil, nil) if roleStore is not configured or remaining roles couldn't be read.
func (pe *policyEnforcer) removeUserRoleInCasbin(user, role string) ([]string, error) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if _, err := pe.enforcer.RemoveUserRole(user, role); err != nil {
		pe.logger.Error("failed to remove specific role from user",
			"user", user,
			"role", role,
			"error", err,
		)
		return nil, fmt.Errorf("failed to remove role %s from user %s: %w", role, user, err)
	}

	// Capture remaining roles for Redis persistence while under lock
	if pe.roleStore != nil {
		remainingRoles, err := pe.enforcer.GetRolesForUser(user)
		if err != nil {
			pe.logger.Warn("failed to get remaining roles for Redis update",
				"user", user,
				"error", err,
			)
			return nil, nil // Don't fail — in-memory removal already succeeded
		}
		return remainingRoles, nil
	}

	return nil, nil
}

// RemoveProjectPolicies removes all policies for a project (public wrapper)
// Used when a Project is deleted to clean up associated policies
func (pe *policyEnforcer) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	if projectName == "" {
		return fmt.Errorf("%w: project name cannot be empty", ErrProjectNotFound)
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	// Remove from loaded projects tracking
	delete(pe.loadedProjects, projectName)

	// Invalidate cache when policies change
	if pe.cacheEnabled && pe.cache != nil {
		pe.cache.Clear()
		pe.logger.Info("cache invalidated due to project policy removal",
			"project", projectName,
		)
	}

	return pe.removeProjectPoliciesLocked(projectName)
}

// InvalidateCache clears all cached authorization decisions
// Should be called when policies are reloaded or modified
func (pe *policyEnforcer) InvalidateCache() {
	if pe.cache != nil {
		pe.cache.Clear()
		pe.logger.Info("authorization cache invalidated")
	}
}

// InvalidateCacheForUser invalidates cache entries for a specific user
// Returns the number of entries removed
// Cache keys use null byte separator: {user}\x00{object}\x00{action}
func (pe *policyEnforcer) InvalidateCacheForUser(user string) int {
	if pe.cache == nil {
		return 0
	}

	// Build the prefix to match all cache entries for this user
	// Cache keys use null byte separator (see cacheKey() in cache.go)
	prefix := user + "\x00"
	count := pe.cache.InvalidateByPrefix(prefix)

	if count > 0 {
		pe.logger.Info("invalidated cache entries for user",
			"user", user,
			"entries_removed", count,
		)
	}

	return count
}

// InvalidateCacheForProject invalidates cache entries for a project
// Returns the number of entries removed
// This invalidates all cache entries where the object contains the project name
func (pe *policyEnforcer) InvalidateCacheForProject(projectName string) int {
	if pe.cache == nil {
		return 0
	}

	// Project-specific cache invalidation is more complex because
	// the project is embedded in the object (e.g., "projects/alpha" or "instances/alpha/...")
	// We need to invalidate all entries that reference this project
	// For simplicity, we'll clear the entire cache when project policies change
	// This is safe because it's a cache - entries will be repopulated on next access
	pe.cache.Clear()

	pe.logger.Info("invalidated cache for project (full cache clear)",
		"project", projectName,
	)

	// Return -1 to indicate full cache clear (unknown count)
	return -1
}

// CacheStats returns cache performance statistics
// Returns empty stats if caching is disabled
func (pe *policyEnforcer) CacheStats() CacheStats {
	if pe.cache == nil {
		return CacheStats{}
	}
	return pe.cache.Stats()
}

// Metrics returns policy enforcement metrics
func (pe *policyEnforcer) Metrics() PolicyMetrics {
	return PolicyMetrics{
		PolicyReloads:   atomic.LoadInt64(&pe.metrics.PolicyReloads),
		BackgroundSyncs: atomic.LoadInt64(&pe.metrics.BackgroundSyncs),
		WatcherRestarts: atomic.LoadInt64(&pe.metrics.WatcherRestarts),
	}
}

// IncrementPolicyReloads increments the policy reload counter
func (pe *policyEnforcer) IncrementPolicyReloads() {
	atomic.AddInt64(&pe.metrics.PolicyReloads, 1)
}

// IncrementBackgroundSyncs increments the background sync counter
func (pe *policyEnforcer) IncrementBackgroundSyncs() {
	atomic.AddInt64(&pe.metrics.BackgroundSyncs, 1)
}

// IncrementWatcherRestarts increments the watcher restart counter
func (pe *policyEnforcer) IncrementWatcherRestarts() {
	atomic.AddInt64(&pe.metrics.WatcherRestarts, 1)
}

// GetAccessibleProjects returns the list of project names the user can access
// This uses a UNIFIED approach - same code path for all users including global admins.
// Global admins get all projects because their Casbin policies grant access to all,
// NOT because of a special code path.
//
// The method:
// 1. Lists all projects from Kubernetes
// 2. For each project, checks if user (or their groups or roles) has "get" access
// 3. Returns the list of accessible project names
//
// For users with wildcard access (e.g., role:serveradmin with "projects/*, *, allow"),
// this naturally returns all projects because the wildcard policy matches all project objects.

func (pe *policyEnforcer) GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error) {
	// If no project reader, we can't list projects
	if pe.projectReader == nil {
		pe.logger.Warn("GetAccessibleProjects called but no project reader configured")
		return nil, nil
	}

	// List all projects from Kubernetes
	projects, err := pe.projectReader.ListProjects(ctx)
	if err != nil {
		pe.logger.Error("failed to list projects for access check",
			"user", user,
			"error", err,
		)
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	// Check access for each project
	// This is the UNIFIED approach: same code for global admins and regular users
	// Global admins will have access to all because their Casbin policies grant wildcard access
	accessibleProjects := make([]string, 0, len(projects))

	for _, project := range projects {
		projectName := project.Name
		object := "projects/" + projectName

		// Check if user or their groups or their server-side roles have access to this project
		hasAccess, err := pe.CanAccessWithGroups(ctx, user, groups, object, "get")
		if err != nil {
			pe.logger.Debug("error checking project access, skipping",
				"user", user,
				"project", projectName,
				"error", err,
			)
			continue
		}

		if hasAccess {
			accessibleProjects = append(accessibleProjects, projectName)
		}
	}

	pe.logger.Debug("determined accessible projects",
		"user", user,
		"groups", groups,
		"total_projects", len(projects),
		"accessible_projects", len(accessibleProjects),
	)

	return accessibleProjects, nil
}
