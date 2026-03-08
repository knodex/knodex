// Package auth provides authentication and authorization services.
// This file implements the AutoRoleBinder service for automatic role binding based on OIDC groups.
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/knodex/knodex/server/internal/rbac"
)

// AutoRoleBinder automatically creates role bindings from OIDC group mappings.
// On successful authentication, it evaluates the user's OIDC groups against
// configured mappings and assigns the appropriate project roles and admin status.
//
// Design:
// 1. Takes user ID and OIDC groups from authentication context
// 2. Calls GroupMapper.EvaluateMappings() to get project memberships and global roles
// 3. Assigns project roles via PolicyEnforcer using "proj:{project}:{role}" format
// 4. Assigns global roles from GlobalRoles slice (e.g., role:serveradmin)
// 5. Logs all automatic role assignments for audit trail
//
// Thread Safety:
// AutoRoleBinder is safe for concurrent use. It uses a mutex to prevent
// race conditions when syncing roles for the same user from multiple goroutines.
//
// UserService dependency removed - admin status is now solely
// determined by Casbin role:serveradmin, not User CRD flags.
//
// Server-Level Role Model (1-Global-Role):
// - role:serveradmin - Full access (assigned when GlobalRoles contains "role:serveradmin")
// Project-scoped roles (admin, developer, readonly) are defined in Project CRD spec.roles.
type AutoRoleBinder struct {
	groupMapper    *GroupMapper
	policyEnforcer rbac.PolicyEnforcer
	logger         *slog.Logger
	defaultRole    string

	// mu protects concurrent role syncs for the same user
	mu sync.Mutex
}

// AutoRoleBinderOption configures AutoRoleBinder behavior
type AutoRoleBinderOption func(*AutoRoleBinder)

// WithLogger sets a custom logger for AutoRoleBinder
func WithLogger(logger *slog.Logger) AutoRoleBinderOption {
	return func(b *AutoRoleBinder) {
		b.logger = logger
	}
}

// WithDefaultRole sets the default Casbin role assigned to users with no matched
// group mappings. Follows ArgoCD's policy.default pattern.
// An empty string disables default role assignment.
func WithDefaultRole(role string) AutoRoleBinderOption {
	return func(b *AutoRoleBinder) {
		b.defaultRole = role
	}
}

// NewAutoRoleBinder creates a new AutoRoleBinder with the given dependencies.
//
// Parameters:
//   - groupMapper: Service that evaluates OIDC group-to-project mappings
//   - policyEnforcer: Service that manages Casbin role assignments
//   - opts: Optional configuration functions
//
// Returns:
//   - *AutoRoleBinder: The configured auto role binder
//   - error: If any required dependency is nil
//
// Removed userService parameter - global admin status is now solely
// determined by Casbin role:serveradmin, not User CRD flags.
func NewAutoRoleBinder(
	groupMapper *GroupMapper,
	policyEnforcer rbac.PolicyEnforcer,
	opts ...AutoRoleBinderOption,
) (*AutoRoleBinder, error) {
	if groupMapper == nil {
		return nil, fmt.Errorf("groupMapper is required")
	}
	if policyEnforcer == nil {
		return nil, fmt.Errorf("policyEnforcer is required")
	}

	binder := &AutoRoleBinder{
		groupMapper:    groupMapper,
		policyEnforcer: policyEnforcer,
		logger:         slog.Default(),
	}

	// Apply options
	for _, opt := range opts {
		opt(binder)
	}

	return binder, nil
}

// SyncUserRoles synchronizes a user's roles based on their OIDC group memberships.
// This method is idempotent and safe to call on every authentication.
//
// Behavior:
// 1. Evaluates OIDC groups against configured mappings
// 2. For each matched project membership, assigns the role via PolicyEnforcer
// 3. For each matched global role (e.g., role:serveradmin), assigns via PolicyEnforcer
// 4. Logs all role assignments for audit trail
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - userID: The user identifier (typically email or OIDC subject)
//   - groups: List of OIDC group names from the token
//
// Returns:
//   - *SyncResult: Summary of role assignments
//   - error: If role assignment fails (non-fatal errors are logged but don't fail)
//
// Thread Safety:
// Uses mutex to prevent concurrent syncs for the same user.
func (b *AutoRoleBinder) SyncUserRoles(ctx context.Context, userID string, groups []string) (*SyncResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := &SyncResult{
		UserID:             userID,
		GroupsEvaluated:    len(groups),
		RolesAssigned:      []RoleAssignment{},
		GlobalAdminGranted: false,
	}

	// Handle no groups: assign default role if configured, otherwise return empty
	if len(groups) == 0 {
		if b.defaultRole != "" {
			b.logger.Info("assigning default role (no OIDC groups)",
				slog.String("user_id", userID),
				slog.String("default_role", b.defaultRole),
			)
			return b.assignDefaultRole(ctx, userID, result)
		}
		b.logger.Info("no OIDC groups to evaluate",
			slog.String("user_id", userID),
		)
		return result, nil
	}

	// Evaluate group mappings
	mappingResult := b.groupMapper.EvaluateMappings(groups)

	b.logger.Info("evaluated OIDC group mappings",
		slog.String("user_id", userID),
		slog.Int("groups", len(groups)),
		slog.Int("project_memberships", len(mappingResult.ProjectMemberships)),
		slog.Int("global_roles", len(mappingResult.GlobalRoles)),
	)

	// Collect all roles to assign
	rolesToAssign := make([]string, 0, len(mappingResult.ProjectMemberships)+len(mappingResult.GlobalRoles))

	// Add project roles
	for _, membership := range mappingResult.ProjectMemberships {
		// Convert to Casbin role format: proj:{project}:{role}
		casbinRole := fmt.Sprintf("proj:%s:%s", membership.ProjectID, membership.Role)
		rolesToAssign = append(rolesToAssign, casbinRole)

		result.RolesAssigned = append(result.RolesAssigned, RoleAssignment{
			ProjectID: membership.ProjectID,
			Role:      membership.Role,
			Source:    "oidc-group-mapping",
		})

		b.logger.Info("mapped group to project role",
			slog.String("user_id", userID),
			slog.String("project_id", membership.ProjectID),
			slog.String("role", membership.Role),
		)
	}

	// Add global roles from group mappings (e.g., role:serveradmin)
	for _, globalRole := range mappingResult.GlobalRoles {
		rolesToAssign = append(rolesToAssign, globalRole)

		// Check if this is the server admin role for result tracking
		if globalRole == rbac.CasbinRoleServerAdmin {
			result.GlobalAdminGranted = true
		}

		b.logger.Info("granted global role from OIDC group mapping",
			slog.String("user_id", userID),
			slog.String("role", globalRole),
		)
	}

	// Handle no matched roles: assign default role if configured
	if len(rolesToAssign) == 0 {
		if b.defaultRole != "" {
			b.logger.Info("assigning default role (no group mappings matched)",
				slog.String("user_id", userID),
				slog.String("default_role", b.defaultRole),
				slog.Int("groups", len(groups)),
			)
			return b.assignDefaultRole(ctx, userID, result)
		}
		b.logger.Info("no role mappings matched for user",
			slog.String("user_id", userID),
			slog.Int("groups", len(groups)),
		)
		return result, nil
	}

	// Assign all roles via PolicyEnforcer
	// Format user as "user:{userID}" for Casbin
	casbinUser := fmt.Sprintf("user:%s", userID)

	if err := b.policyEnforcer.AssignUserRoles(ctx, casbinUser, rolesToAssign); err != nil {
		b.logger.Error("failed to assign user roles",
			slog.String("user_id", userID),
			slog.Any("roles", rolesToAssign),
			slog.Any("error", err),
		)
		return result, fmt.Errorf("failed to assign user roles: %w", err)
	}

	b.logger.Info("assigned roles from OIDC group mappings",
		slog.String("user_id", userID),
		slog.Int("roles_assigned", len(rolesToAssign)),
		slog.Bool("global_admin", result.GlobalAdminGranted),
	)

	// Global admin status is now solely determined by Casbin role:serveradmin
	// which was assigned above via policyEnforcer.AssignUserRoles()

	return result, nil
}

// assignDefaultRole assigns the configured default role to a user when no group
// mappings matched. This is a helper to avoid code duplication between the
// "no groups" and "no matched mappings" paths.
func (b *AutoRoleBinder) assignDefaultRole(ctx context.Context, userID string, result *SyncResult) (*SyncResult, error) {
	result.RolesAssigned = append(result.RolesAssigned, RoleAssignment{
		Role:   b.defaultRole,
		Source: "default-role-config",
	})

	// Check if the default role is server admin (for result tracking)
	if b.defaultRole == rbac.CasbinRoleServerAdmin {
		result.GlobalAdminGranted = true
	}

	casbinUser := fmt.Sprintf("user:%s", userID)
	if err := b.policyEnforcer.AssignUserRoles(ctx, casbinUser, []string{b.defaultRole}); err != nil {
		b.logger.Error("failed to assign default role",
			slog.String("user_id", userID),
			slog.String("role", b.defaultRole),
			slog.Any("error", err),
		)
		return result, fmt.Errorf("failed to assign default role: %w", err)
	}

	b.logger.Info("assigned default role",
		slog.String("user_id", userID),
		slog.String("role", b.defaultRole),
		slog.String("source", "default-role-config"),
	)

	return result, nil
}

// Admin status is solely determined by Casbin role:serveradmin (ArgoCD-aligned).
// All admin authorization flows through Casbin policy evaluation.

// SyncResult contains the results of a role synchronization operation.
type SyncResult struct {
	// UserID is the user whose roles were synchronized
	UserID string

	// GroupsEvaluated is the number of OIDC groups that were evaluated
	GroupsEvaluated int

	// RolesAssigned is the list of role assignments that were made
	RolesAssigned []RoleAssignment

	// GlobalAdminGranted indicates if global admin was granted from group mapping
	GlobalAdminGranted bool
}

// RoleAssignment represents a single role assignment from OIDC group mapping.
type RoleAssignment struct {
	// ProjectID is the project the role was assigned for
	ProjectID string

	// Role is the role name (platform-admin, developer, viewer)
	Role string

	// Source indicates how the role was assigned (e.g., "oidc-group-mapping")
	Source string
}
