// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package auth provides authentication and authorization services.
// This file implements the GroupMapper service for OIDC group-to-project mapping.
package auth

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// ProjectMembership represents a project membership derived from OIDC group mapping.
// It contains the target project ID and the role to assign within that project.
type ProjectMembership struct {
	// ProjectID is the Project CRD name (e.g., "engineering-project")
	ProjectID string

	// Role is the role to assign: platform-admin, developer, or viewer
	Role string
}

// GroupMappingResult contains the result of evaluating OIDC group mappings.
// It includes the list of project memberships to apply and any global roles.
type GroupMappingResult struct {
	// ProjectMemberships is the list of project memberships derived from group mappings.
	// Each membership specifies a project and the role within it.
	ProjectMemberships []ProjectMembership

	// GlobalRoles contains global Casbin roles assigned from group mappings.
	// When a user belongs to a group with globalAdmin: true mapping,
	// this will contain ["role:serveradmin"]. ArgoCD-aligned: all admin status
	// is expressed as Casbin roles, never as boolean flags.
	GlobalRoles []string
}

// GroupMapper evaluates OIDC group-to-project mappings.
// It is a stateless service that takes a user's OIDC groups and configured mappings,
// evaluates which mappings apply, and returns the resulting project/role assignments.
//
// Algorithm:
//  1. For mappings without wildcards: build hash set of user groups for O(1) lookup
//  2. For mappings with wildcards: use filepath.Match for pattern matching
//  3. If globalAdmin mapping matches, add "role:serveradmin" to GlobalRoles
//  4. If project mapping matches, track highest role per project
//  5. Return combined result with all project memberships and global roles
//
// Wildcard support:
//   - Uses filepath.Match patterns: *, ?, [abc], [a-z]
//   - Examples: "dev-*" matches "dev-team1", "*-admin" matches "platform-admin"
//
// Role precedence (higher wins): platform-admin > developer > viewer
type GroupMapper struct {
	mappings []config.OIDCGroupMapping
}

// NewGroupMapper creates a new GroupMapper with the given mappings.
// Mappings should already be validated by config.ValidateGroupMappings().
// If mappings is nil, an empty slice is used.
func NewGroupMapper(mappings []config.OIDCGroupMapping) *GroupMapper {
	if mappings == nil {
		mappings = []config.OIDCGroupMapping{}
	}
	return &GroupMapper{
		mappings: mappings,
	}
}

// EvaluateMappings evaluates which configured mappings apply to the given OIDC groups
// and returns the resulting project memberships and globalAdmin status.
//
// Behavior:
//   - Empty groups input returns empty result (not error)
//   - Empty mappings returns empty result (not error)
//   - Group matching supports both exact match and wildcard patterns
//   - Wildcards use filepath.Match patterns: *, ?, [abc], [a-z]
//   - When multiple groups map to the same project, the highest role wins
//   - GlobalAdmin mappings add "role:serveradmin" to GlobalRoles (independent of project mappings)
//
// Time complexity:
//   - O(n + m) for exact matches (using hash set)
//   - O(n * w) for wildcard patterns where w = number of wildcard mappings
//
// Space complexity: O(n + k) where k = number of unique projects matched
func (m *GroupMapper) EvaluateMappings(groups []string) *GroupMappingResult {
	// Handle empty inputs efficiently
	if len(groups) == 0 || len(m.mappings) == 0 {
		return &GroupMappingResult{
			ProjectMemberships: []ProjectMembership{},
			GlobalRoles:        []string{},
		}
	}

	// Build hash set of user groups for O(1) exact lookup
	// Filter empty strings as a defensive measure
	nonEmpty := collection.Filter(groups, func(g string) bool { return g != "" })
	groupSet := collection.ToSet(nonEmpty)

	// Track highest role per project
	projectRoles := make(map[string]string)
	globalRoles := []string{}

	// Evaluate each mapping against user's groups
	for _, mapping := range m.mappings {
		// Check if user matches this group pattern
		matched := m.matchGroup(mapping.Group, groups, groupSet)
		if !matched {
			continue // User doesn't match this group pattern
		}

		// Handle globalAdmin mapping - assign role:serveradmin as Casbin role
		if mapping.GlobalAdmin {
			// Only add if not already present (avoid duplicates)
			if !collection.Contains(globalRoles, rbac.CasbinRoleServerAdmin) {
				globalRoles = append(globalRoles, rbac.CasbinRoleServerAdmin)
			}
			continue // GlobalAdmin mappings don't have project/role
		}

		// Handle project mapping
		// Defensive validation: skip unknown roles (should be caught by config validation)
		if mapping.Project != "" {
			if _, knownRole := rolePrecedence[mapping.Role]; !knownRole {
				continue // Skip mappings with unknown roles
			}
			existing := projectRoles[mapping.Project]
			if existing == "" || roleIsHigher(mapping.Role, existing) {
				projectRoles[mapping.Project] = mapping.Role
			}
		}
	}

	// Convert map to slice with deterministic ordering
	memberships := make([]ProjectMembership, 0, len(projectRoles))
	for projectID, role := range projectRoles {
		memberships = append(memberships, ProjectMembership{
			ProjectID: projectID,
			Role:      role,
		})
	}

	// Sort by ProjectID for stable, deterministic output
	sort.Slice(memberships, func(i, j int) bool {
		return memberships[i].ProjectID < memberships[j].ProjectID
	})

	return &GroupMappingResult{
		ProjectMemberships: memberships,
		GlobalRoles:        globalRoles,
	}
}

// matchGroup checks if any user group matches the given pattern.
// For exact patterns (no wildcards), uses O(1) hash set lookup.
// For wildcard patterns, iterates through groups and uses filepath.Match.
//
// Supported wildcards (from filepath.Match):
//   - '*' matches any sequence of characters
//   - '?' matches any single character
//   - '[abc]' matches any character in the set
//   - '[a-z]' matches any character in the range
func (m *GroupMapper) matchGroup(pattern string, groups []string, groupSet map[string]bool) bool {
	// Check if pattern contains wildcards
	if !isWildcardPattern(pattern) {
		// Exact match: O(1) hash lookup
		return groupSet[pattern]
	}

	// Wildcard match: iterate through groups
	for _, group := range groups {
		matched, err := filepath.Match(pattern, group)
		if err != nil {
			// Invalid pattern - skip (should be caught by validation)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// isWildcardPattern checks if a pattern contains wildcard characters.
// Wildcards are: *, ?, [
func isWildcardPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// rolePrecedence defines the hierarchy of roles.
// Higher values indicate higher privilege levels.
var rolePrecedence = map[string]int{
	rbac.RolePlatformAdmin: 3,
	rbac.RoleDeveloper:     2,
	rbac.RoleViewer:        1,
}

// roleIsHigher returns true if role1 has higher precedence than role2.
// Precedence order: platform-admin (3) > developer (2) > viewer (1)
// Unknown roles are treated as having precedence 0 (lowest).
func roleIsHigher(role1, role2 string) bool {
	p1 := rolePrecedence[role1]
	p2 := rolePrecedence[role2]
	return p1 > p2
}
