// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"log/slog"

	"github.com/knodex/knodex/server/internal/api/middleware"
)

// PolicyEnforcer defines the interface for Casbin policy enforcement.
// This matches the existing rbac.PolicyEnforcer interface to allow dependency injection.
type PolicyEnforcer interface {
	// CanAccessWithGroups checks if user/groups/server-side roles can perform action on object.
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
	// GetAccessibleProjects returns the list of project names the user can access.
	// Global admins see all projects because their Casbin policies grant wildcard access.
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	GetAccessibleProjects(ctx context.Context, user string, groups []string) ([]string, error)
}

// NamespaceProvider defines the interface for getting user namespace access.
// This matches the existing rbac.PermissionService interface.
type NamespaceProvider interface {
	// GetUserNamespacesWithGroups returns all Kubernetes namespaces the user has access to.
	// Roles are sourced from Casbin's authoritative state, NOT from JWT claims.
	// Returns:
	// - nil: user can access all namespaces (global admin)
	// - empty slice: user has no namespace access
	// - non-empty slice: user can access these specific namespaces
	GetUserNamespacesWithGroups(ctx context.Context, userID string, groups []string) ([]string, error)
}

// AuthorizationService consolidates authorization logic that was previously scattered
// across 10+ handlers. It provides a unified interface for:
// - Getting accessible projects (replaces repeated GetAccessibleProjects calls)
// - Getting accessible namespaces (replaces duplicated getUserNamespaces methods)
// - Checking resource access via Casbin
type AuthorizationService struct {
	policyEnforcer    PolicyEnforcer
	namespaceProvider NamespaceProvider
	logger            *slog.Logger
}

// AuthorizationServiceConfig holds configuration for creating an AuthorizationService.
type AuthorizationServiceConfig struct {
	PolicyEnforcer    PolicyEnforcer
	NamespaceProvider NamespaceProvider
	Logger            *slog.Logger
}

// NewAuthorizationService creates a new AuthorizationService.
func NewAuthorizationService(config AuthorizationServiceConfig) *AuthorizationService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthorizationService{
		policyEnforcer:    config.PolicyEnforcer,
		namespaceProvider: config.NamespaceProvider,
		logger:            logger.With("component", "authorization-service"),
	}
}

// GetUserAuthContext computes the full authorization context for a user.
// This consolidates the pattern repeated across handlers:
// 1. Get accessible projects via Casbin
// 2. Get accessible namespaces via PermissionService
// 3. Return a unified context for filtering
//
// Returns nil for unauthenticated requests (nil userCtx).
// For authenticated users, returns context with computed access lists.
func (s *AuthorizationService) GetUserAuthContext(ctx context.Context, userCtx *middleware.UserContext) (*UserAuthContext, error) {
	// Unauthenticated request - return nil (handler should apply appropriate defaults)
	if userCtx == nil {
		return nil, nil
	}

	authCtx := &UserAuthContext{
		UserID: userCtx.UserID,
		Groups: userCtx.Groups,
		Roles:  userCtx.CasbinRoles, // JWT roles for display only; enforcement uses server-side Casbin
	}

	// Get accessible projects from Casbin
	// Global admins get all projects because their policies grant wildcard access
	if s.policyEnforcer != nil {
		projects, err := s.policyEnforcer.GetAccessibleProjects(ctx, userCtx.UserID, userCtx.Groups)
		if err != nil {
			s.logger.Error("failed to get accessible projects",
				"user_id", userCtx.UserID,
				"error", err)
			// Secure default on error: empty projects (will only see public RGDs)
			projects = []string{}
		}
		authCtx.AccessibleProjects = projects
	} else {
		// Fallback: use projects from JWT claims if policy enforcer not set
		authCtx.AccessibleProjects = userCtx.Projects
	}

	// Get accessible namespaces
	if s.namespaceProvider != nil {
		namespaces, err := s.namespaceProvider.GetUserNamespacesWithGroups(
			ctx,
			userCtx.UserID,
			userCtx.Groups,
		)
		if err != nil {
			s.logger.Error("failed to get user namespaces",
				"user_id", userCtx.UserID,
				"error", err)
			// Secure default on error: empty list (not nil which means all)
			namespaces = []string{}
		}
		authCtx.AccessibleNamespaces = namespaces
		// nil namespaces indicates global access (admin)
		authCtx.IsGlobalAccess = namespaces == nil
	} else {
		// No namespace provider - allow all namespaces (backward compatibility)
		authCtx.AccessibleNamespaces = nil
		authCtx.IsGlobalAccess = true
	}

	s.logger.Debug("computed user auth context",
		"user_id", userCtx.UserID,
		"accessible_projects", len(authCtx.AccessibleProjects),
		"accessible_namespaces_count", len(authCtx.AccessibleNamespaces),
		"is_global_access", authCtx.IsGlobalAccess)

	return authCtx, nil
}

// GetAccessibleProjects returns the list of projects the user can access.
// This consolidates the 10+ repeated calls to policyEnforcer.GetAccessibleProjects
// that were scattered across handlers.
//
// Returns:
// - Projects from Casbin evaluation (global admins get all projects via policy)
// - Projects from JWT claims as fallback
// - Empty slice on error (secure default)
func (s *AuthorizationService) GetAccessibleProjects(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if userCtx == nil {
		return []string{}, nil
	}

	if s.policyEnforcer == nil {
		// Fallback to JWT claims
		return userCtx.Projects, nil
	}

	projects, err := s.policyEnforcer.GetAccessibleProjects(ctx, userCtx.UserID, userCtx.Groups)
	if err != nil {
		s.logger.Error("failed to get accessible projects",
			"user_id", userCtx.UserID,
			"error", err)
		// Secure default on error
		return []string{}, err
	}

	return projects, nil
}

// GetAccessibleNamespaces returns the list of Kubernetes namespaces the user can access.
// This consolidates namespace access logic used by RGDHandler and instance handlers.
//
// Returns:
// - nil: User can access all namespaces (global admin)
// - empty slice: User has no namespace access (secure default, also on error)
// - non-empty slice: User can only access these namespaces
func (s *AuthorizationService) GetAccessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	// Unauthenticated - secure default
	if userCtx == nil {
		return []string{}, nil
	}

	// No namespace provider - allow all (backward compatibility)
	if s.namespaceProvider == nil {
		return nil, nil
	}

	namespaces, err := s.namespaceProvider.GetUserNamespacesWithGroups(
		ctx,
		userCtx.UserID,
		userCtx.Groups,
	)
	if err != nil {
		s.logger.Error("failed to get user namespaces",
			"user_id", userCtx.UserID,
			"error", err)
		// Secure default on error: empty list (not nil which means all)
		return []string{}, err
	}

	return namespaces, nil
}

// CanAccess checks if the user can perform an action on a resource.
// This wraps the Casbin policy check with consistent error handling.
//
// Parameters:
// - authCtx: The user's authorization context
// - resource: The resource type (e.g., "projects", "instances", "rgds")
// - action: The action being performed (e.g., "get", "create", "update", "delete")
// - object: The specific object (e.g., "projects/acme", "instances/staging/my-app")
//
// Returns true if access is allowed, false otherwise.
func (s *AuthorizationService) CanAccess(ctx context.Context, authCtx *UserAuthContext, resource, action, object string) (bool, error) {
	if authCtx == nil {
		return false, nil
	}

	if s.policyEnforcer == nil {
		// No enforcer - allow (backward compatibility)
		return true, nil
	}

	allowed, err := s.policyEnforcer.CanAccessWithGroups(
		ctx,
		authCtx.UserID,
		authCtx.Groups,
		object,
		action,
	)
	if err != nil {
		s.logger.Error("authorization check failed",
			"user_id", authCtx.UserID,
			"resource", resource,
			"action", action,
			"object", object,
			"error", err)
		return false, err
	}

	s.logger.Debug("authorization check",
		"user_id", authCtx.UserID,
		"resource", resource,
		"action", action,
		"object", object,
		"allowed", allowed)

	return allowed, nil
}

// CanAccessProject checks if the user has access to a specific project.
// This is a convenience wrapper around CanAccess for project resources.
func (s *AuthorizationService) CanAccessProject(ctx context.Context, authCtx *UserAuthContext, projectName, action string) (bool, error) {
	return s.CanAccess(ctx, authCtx, "projects", action, "projects/"+projectName)
}

// HasProjectAccess checks if the project is in the user's accessible projects list.
// This is used for visibility filtering without making additional Casbin calls.
func (s *AuthorizationService) HasProjectAccess(authCtx *UserAuthContext, projectName string) bool {
	if authCtx == nil {
		return false
	}

	for _, p := range authCtx.AccessibleProjects {
		if p == projectName {
			return true
		}
	}

	return false
}
