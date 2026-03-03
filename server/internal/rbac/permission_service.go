package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Additional permissions for instances and RGDs
const (
	PermissionInstanceDeploy Permission = "instance:deploy"
	PermissionInstanceDelete Permission = "instance:delete"
	PermissionInstanceView   Permission = "instance:view"
	PermissionRGDView        Permission = "rgd:view"
)

// Role represents a user's role (including global admin)
type Role string

const (
	RoleGlobalAdmin Role = "global-admin"
	// Project-scoped roles are defined in types.go:
	// RolePlatformAdmin, RoleDeveloper, RoleViewer
)

// PermissionServiceEnforcer defines the subset of policy enforcement methods
// needed by PermissionService. This follows Interface Segregation Principle.
type PermissionServiceEnforcer interface {
	Authorizer
	CacheController
}

// PermissionService implements the RBAC permission evaluation logic
// Removed UserService dependency - user project access is now determined by:
// - Casbin policies (via Authorizer.GetAccessibleProjects)
// - JWT claims containing OIDC groups (for group-to-project mapping)
type PermissionService struct {
	projectService *ProjectService
	policyEnforcer PermissionServiceEnforcer
	logger         *slog.Logger
}

// PermissionServiceConfig holds configuration for the permission service
type PermissionServiceConfig struct {
	ProjectService *ProjectService
	PolicyEnforcer PermissionServiceEnforcer
	Logger         *slog.Logger
}

// NewPermissionService creates a new RBAC permission service
func NewPermissionService(config PermissionServiceConfig) *PermissionService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &PermissionService{
		projectService: config.ProjectService,
		policyEnforcer: config.PolicyEnforcer,
		logger:         logger,
	}
}

// CheckPermission method removed - use PolicyEnforcer.CanAccess() or CanAccessWithGroups() instead.
// All authorization decisions now go through the Casbin-based PolicyEnforcer exclusively.

// GetUserProjects returns all projects a user belongs to
// Now uses PolicyEnforcer to get accessible projects instead of User CRD.
// For OIDC users, projects come from group mappings via Casbin policies.
// For local users, projects are assigned via direct Casbin role assignments.
func (s *PermissionService) GetUserProjects(ctx context.Context, userID string) ([]*Project, error) {
	// Use PolicyEnforcer to get accessible projects via Casbin policies
	if s.policyEnforcer == nil {
		return []*Project{}, nil
	}

	// Get accessible project names from Casbin policies
	accessibleProjects, err := s.policyEnforcer.GetAccessibleProjects(ctx, fmt.Sprintf("user:%s", userID), nil)
	if err != nil {
		s.logger.Error("failed to get accessible projects",
			"error", err,
			"user_id", userID,
		)
		return nil, fmt.Errorf("failed to retrieve projects")
	}

	// If user has no projects, return empty list
	if len(accessibleProjects) == 0 {
		return []*Project{}, nil
	}

	// Fetch all projects the user has access to
	var projects []*Project
	for _, projectID := range accessibleProjects {
		project, err := s.projectService.GetProject(ctx, projectID)
		if err != nil {
			s.logger.Warn("failed to get project",
				"error", err,
				"user_id", userID,
				"project_id", projectID,
			)
			continue
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// GetUserRole returns the user's role within a specific project
// Returns empty string if user is not a member
func (s *PermissionService) GetUserRole(ctx context.Context, userID string, projectID string) (string, error) {
	// Use the project service's existing GetUserRole method
	role, err := s.projectService.GetUserRole(ctx, projectID, userID)
	if err != nil {
		// Check if error is "user is not a member" - this is not an error, return empty string
		if strings.Contains(err.Error(), "is not a member of project") {
			return "", nil
		}
		// Log detailed error internally (CWE-209 mitigation)
		s.logger.Error("failed to get user role",
			"error", err,
			"user_id", userID,
			"project_id", projectID,
		)
		// Other errors (e.g., project not found) should be propagated with generic message
		return "", fmt.Errorf("failed to retrieve role")
	}

	return role, nil
}

// GetUserNamespaces returns all Kubernetes namespaces the user has access to

// Kept for backward compatibility with existing tests
func (s *PermissionService) GetUserNamespaces(ctx context.Context, userID string) ([]string, error) {
	// Delegate to the groups and roles aware version with empty groups/roles
	return s.GetUserNamespacesWithGroups(ctx, userID, nil)
}

// GetUserNamespacesWithGroups returns all Kubernetes namespaces the user has access to

// Global admins see all project namespaces because their Casbin policies grant access to all projects.
// Returns:
// - nil: Only when policyEnforcer is not configured (backward compatibility)
// - empty slice: User has no namespace access
// - non-empty slice: User can only see instances in these namespaces

func (s *PermissionService) GetUserNamespacesWithGroups(ctx context.Context, userID string, groups []string) ([]string, error) {
	// Security: Add timeout to prevent unbounded operations (CWE-400 mitigation)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Security: Validate groups array size to prevent DoS (CWE-20 mitigation)
	const maxGroups = 100 // Reasonable limit for OIDC groups
	if len(groups) > maxGroups {
		s.logger.Warn("groups array exceeds maximum allowed size, truncating",
			"user_id", userID,
			"groups_count", len(groups),
			"max_allowed", maxGroups,
		)
		groups = groups[:maxGroups]
	}

	// This returns projects based on Casbin policies (same code path for all users)
	// Global admins will get all projects because their policies grant wildcard access
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims
	if s.policyEnforcer != nil {
		accessibleProjects, err := s.policyEnforcer.GetAccessibleProjects(ctx, fmt.Sprintf("user:%s", userID), groups)
		if err != nil {
			s.logger.Warn("failed to get accessible projects",
				"error", err,
				"user_id", userID,
			)
			// Fall back to legacy lookup on error
		} else if len(accessibleProjects) > 0 {
			// Get namespaces from accessible projects
			namespaceSet := make(map[string]bool)
			for _, projectName := range accessibleProjects {
				project, err := s.projectService.GetProject(ctx, projectName)
				if err != nil {
					s.logger.Debug("failed to get project for namespace lookup, skipping",
						"project", projectName,
						"error", err,
					)
					continue
				}
				for _, dest := range project.Spec.Destinations {
					// Handle universal wildcard "*" (access to ALL namespaces)
					// Return nil to signal no namespace filtering should be applied
					if dest.Namespace == "*" {
						s.logger.Debug("project has wildcard namespace access, disabling filtering",
							"user_id", userID,
							"project", projectName,
						)
						return nil, nil
					}
					// Include exact namespaces and wildcard patterns (e.g., "staging*")
					if dest.Namespace != "" {
						namespaceSet[dest.Namespace] = true
					}
				}
			}

			// Convert set to slice
			namespaces := make([]string, 0, len(namespaceSet))
			for ns := range namespaceSet {
				namespaces = append(namespaces, ns)
			}

			s.logger.Debug("resolved user namespaces via unified approach",
				"user_id", userID,
				"groups_count", len(groups),
				"accessible_projects", len(accessibleProjects),
				"namespaces_count", len(namespaces),
			)

			return namespaces, nil
		}
		// If no accessible projects found, fall through to legacy lookup
	}

	// Legacy fallback: Get namespaces from direct user project assignments and OIDC groups
	// This path is used when policyEnforcer returns empty (no accessible projects)
	// or when policyEnforcer is not configured
	namespaceSet := make(map[string]bool)

	// Step 1: Get namespaces from direct user project assignments
	directProjects, err := s.GetUserProjects(ctx, userID)
	if err != nil {
		s.logger.Warn("failed to get user's direct projects",
			"error", err,
			"user_id", userID,
		)
		// Continue with group-based lookup even if direct lookup fails
	} else {
		for _, project := range directProjects {
			for _, dest := range project.Spec.Destinations {
				// Handle universal wildcard "*" (access to ALL namespaces)
				// Return nil to signal no namespace filtering should be applied
				if dest.Namespace == "*" {
					s.logger.Debug("project has wildcard namespace access, disabling filtering",
						"user_id", userID,
						"project", project.Name,
					)
					return nil, nil
				}
				// Include exact namespaces and wildcard patterns (e.g., "staging*")
				if dest.Namespace != "" {
					namespaceSet[dest.Namespace] = true
				}
			}
		}
	}

	// Step 2: Get namespaces from OIDC group-based project access
	if len(groups) > 0 {
		groupProjects, err := s.projectService.GetUserProjectsByGroup(ctx, groups)
		if err != nil {
			s.logger.Warn("failed to get projects for OIDC groups",
				"error", err,
				"user_id", userID,
				"groups_count", len(groups),
			)
			// Continue with whatever namespaces we have from direct assignments
		} else {
			for _, project := range groupProjects {
				for _, dest := range project.Spec.Destinations {
					// Handle universal wildcard "*" (access to ALL namespaces)
					// Return nil to signal no namespace filtering should be applied
					if dest.Namespace == "*" {
						s.logger.Debug("group project has wildcard namespace access, disabling filtering",
							"user_id", userID,
							"project", project.Name,
						)
						return nil, nil
					}
					// Include exact namespaces and wildcard patterns (e.g., "staging*")
					if dest.Namespace != "" {
						namespaceSet[dest.Namespace] = true
					}
				}
			}
		}
	}

	// Convert set to slice
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	s.logger.Debug("resolved user namespaces via legacy lookup",
		"user_id", userID,
		"groups_count", len(groups),
		"namespaces_count", len(namespaces),
	)

	return namespaces, nil
}

// Permission matrix methods removed.
// All role-based permissions are now defined in casbin.go loadBuiltinPoliciesLocked().
// Use PolicyEnforcer.CanAccess() or CanAccessWithGroups() for permission checks.
//
// For reference, the permission mapping is documented in docs/rbac-permissions.md.
// The getPermissionsForRole() function below provides a read-only lookup for
// GetUserPermissions() compatibility.

// getPermissionsForRole returns the list of permissions for a given role.
// This is used by GetUserPermissions() for backward compatibility.
// IMPORTANT: This must stay in sync with loadBuiltinPoliciesLocked() in casbin.go.
func getPermissionsForRole(role string) []Permission {
	switch role {
	case CasbinRoleServerAdmin:
		// Server admin has all permissions (role:serveradmin grants full access)
		return []Permission{
			PermissionProjectCreate,
			PermissionProjectRead,
			PermissionProjectUpdate,
			PermissionProjectDelete,
			PermissionProjectForceDelete,
			PermissionProjectMemberAdd,
			PermissionProjectMemberRemove,
			PermissionProjectMemberUpdate,
			PermissionInstanceDeploy,
			PermissionInstanceDelete,
			PermissionInstanceView,
			PermissionRGDView,
		}
	case RolePlatformAdmin:
		return []Permission{
			// Project: read + update + member management
			PermissionProjectRead,
			PermissionProjectUpdate,
			PermissionProjectMemberAdd,
			PermissionProjectMemberRemove,
			PermissionProjectMemberUpdate,
			// Instances: full access
			PermissionInstanceDeploy,
			PermissionInstanceDelete,
			PermissionInstanceView,
			// RGDs: view
			PermissionRGDView,
		}
	case RoleDeveloper:
		return []Permission{
			// Project: read-only
			PermissionProjectRead,
			// Instances: deploy and delete
			PermissionInstanceDeploy,
			PermissionInstanceDelete,
			PermissionInstanceView,
			// RGDs: view
			PermissionRGDView,
		}
	case RoleViewer:
		return []Permission{
			// Project: read-only
			PermissionProjectRead,
			// Instances: view-only
			PermissionInstanceView,
			// RGDs: view
			PermissionRGDView,
		}
	default:
		return []Permission{}
	}
}

// Cache consolidation - removed Redis-based caching
// All authorization caching is now handled by PolicyEnforcer using in-memory AuthorizationCache.
// This eliminates duplicate caching, improves performance, and simplifies cache management.

// HasRole checks if a user has a specific role using Casbin PolicyEnforcer

func (s *PermissionService) HasRole(ctx context.Context, userID, role string) (bool, error) {
	if s.policyEnforcer == nil {
		return false, nil
	}
	return s.policyEnforcer.HasRole(ctx, userID, role)
}

// InvalidateUserCache invalidates all cached authorization decisions for a user
// Should be called when user's project membership or roles change
// Now delegates to PolicyEnforcer's unified AuthorizationCache
func (s *PermissionService) InvalidateUserCache(ctx context.Context, userID string) error {
	if s.policyEnforcer == nil {
		return nil // No-op if PolicyEnforcer not available
	}

	// Delegate to PolicyEnforcer which uses the consolidated AuthorizationCache
	// The cache uses prefix-based invalidation for user-specific entries
	count := s.policyEnforcer.InvalidateCacheForUser(fmt.Sprintf("user:%s", userID))

	if count > 0 {
		s.logger.Info("invalidated user authorization cache",
			"user_id", userID,
			"entries_removed", count,
		)
	}

	return nil
}

// InvalidateProjectCache invalidates all cached authorization decisions for a project
// Should be called when project membership or policies change
// Now delegates to PolicyEnforcer's unified AuthorizationCache
func (s *PermissionService) InvalidateProjectCache(ctx context.Context, projectID string) error {
	if s.policyEnforcer == nil {
		return nil // No-op if PolicyEnforcer not available
	}

	// Delegate to PolicyEnforcer which uses the consolidated AuthorizationCache
	// Project cache invalidation clears the entire cache since project references
	// are embedded in objects (e.g., "projects/alpha", "instances/alpha/...")
	s.policyEnforcer.InvalidateCacheForProject(projectID)

	s.logger.Info("invalidated project authorization cache",
		"project_id", projectID,
	)

	return nil
}

// CanPerform and CanPerformOnProject methods removed.
// All authorization decisions now go through PolicyEnforcer.CanAccess() or CanAccessWithGroups() exclusively.

// GetUserPermissions returns all permissions a user has on a resource (project)
// This method uses the consolidated getPermissionsForRole() function for ALL roles,
// including role:serveradmin. The role is determined via Casbin role lookup (HasRole),
// then mapped to permissions through the same getPermissionsForRole() path.
// This stays in sync with the Casbin policies defined in loadBuiltinPoliciesLocked().
func (s *PermissionService) GetUserPermissions(ctx context.Context, userID string, resourceID string) ([]Permission, error) {
	projectID := resourceID

	// Check for server admin role via Casbin (role lookup, not boolean flag)
	if s.policyEnforcer != nil {
		hasAdminRole, err := s.policyEnforcer.HasRole(ctx, fmt.Sprintf("user:%s", userID), CasbinRoleServerAdmin)
		if err != nil {
			s.logger.Warn("failed to check admin role, continuing with project check",
				"error", err,
				"user_id", userID,
			)
		} else if hasAdminRole {
			// Use unified getPermissionsForRole() path for admin role
			return getPermissionsForRole(CasbinRoleServerAdmin), nil
		}
	}

	// Get user's role in the project
	role, err := s.GetUserRole(ctx, userID, projectID)
	if err != nil {
		// Log detailed error internally (CWE-209 mitigation)
		s.logger.Error("failed to get user role for permissions",
			"error", err,
			"user_id", userID,
			"project_id", projectID,
		)
		// Return generic error to prevent information disclosure
		return nil, fmt.Errorf("failed to retrieve permissions")
	}

	if role == "" {
		// User is not a member - no permissions
		return []Permission{}, nil
	}

	// Use consolidated role-to-permission mapping
	// See docs/rbac-permissions.md for the complete permission reference
	return getPermissionsForRole(role), nil
}
