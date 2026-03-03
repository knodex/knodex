// Package auth provides authentication and authorization services for knodex.
// This file defines consumer interfaces to decouple auth from concrete rbac types.
package auth

import (
	"context"

	"github.com/provops-org/knodex/server/internal/rbac"
)

// AuthRoleManager defines the role management operations needed by auth package.
// This is a consumer-defined interface following the Interface Segregation Principle.
// Implemented by *rbac.CasbinEnforcer.
type AuthRoleManager interface {
	// HasUserRole checks if a user has a specific role
	HasUserRole(user, role string) (bool, error)

	// AddUserRole assigns a role to a user
	AddUserRole(user, role string) (bool, error)

	// GetRolesForUser returns all roles for a user
	GetRolesForUser(user string) ([]string, error)

	// Enforce checks if a subject can perform an action on an object
	Enforce(sub, obj, act string) (bool, error)

	// GetPoliciesForRole returns all policies for a role
	GetPoliciesForRole(role string) ([][]string, error)
}

// RolePersister provides role assignment with Redis persistence and cache invalidation.
// This is a consumer-defined interface following the Interface Segregation Principle.
// Implemented by *rbac.policyEnforcer (the PolicyEnforcer composite).
//
// Unlike AuthRoleManager (which wraps CasbinEnforcer for in-memory-only operations),
// RolePersister ensures role assignments survive pod restarts and that stale cached
// authorization decisions are invalidated immediately.
type RolePersister interface {
	// AssignUserRoles replaces a user's roles with persistence to Redis
	// and invalidation of the user's cached authorization decisions.
	AssignUserRoles(ctx context.Context, user string, roles []string) error
}

// AuthProjectService defines project operations needed by auth package.
// This is a consumer-defined interface following the Interface Segregation Principle.
// Implemented by *rbac.ProjectService.
type AuthProjectService interface {
	// GetProject retrieves a project by name
	GetProject(ctx context.Context, name string) (*rbac.Project, error)

	// CreateProject creates a new Project CRD
	CreateProject(ctx context.Context, name string, spec rbac.ProjectSpec, createdBy string) (*rbac.Project, error)

	// AddGroupToRole adds an OIDC group to a project role
	AddGroupToRole(ctx context.Context, projectName, roleName, groupName, updatedBy string) (*rbac.Project, error)

	// AddRole adds a new role to a project
	AddRole(ctx context.Context, projectID string, role rbac.ProjectRole, updatedBy string) (*rbac.Project, error)

	// GetUserProjectRolesByGroup returns projectID -> roleName mapping based on OIDC groups
	GetUserProjectRolesByGroup(ctx context.Context, groups []string) (map[string]string, error)
}
