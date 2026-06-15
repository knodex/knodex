// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package auth provisions OIDC users at login: it persists a canonical user
// record via services.IdentityService (JIT on first login, last_seen_at refresh
// + resurrect-on-login thereafter). Casbin keys on group claims; the persisted
// record is for data ownership and seat billing, not for Enforce().
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/knodex/knodex/server/internal/rbac"
	utilhash "github.com/knodex/knodex/server/internal/util/hash"
)

// IdentityObserveFunc persists a canonical user record for a successful OIDC
// login (Story 15.2). It is a flat, primitive-typed seam so the auth package
// does NOT import internal/services — that would close an import cycle
// (services → api/middleware → auth). The composition root (app.go) adapts the
// services.IdentityService.ObserveLogin call into this signature, capturing the
// org scope and SourceKind on the store side. A nil func is a no-op.
type IdentityObserveFunc func(ctx context.Context, issuer, sub, email, displayName string, emailVerified bool) error

var (
	// oidcGroupNameRegex validates OIDC group names from external IdP
	oidcGroupNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-./]+$`)
)

// OIDCUserInfo contains user information extracted from OIDC ID token
// This replaces User CRD for OIDC users
type OIDCUserInfo struct {
	// Subject is the OIDC subject identifier (unique user ID from IdP)
	Subject string
	// Email is the user's email address
	Email string
	// DisplayName is the user's display name
	DisplayName string
	// Groups are the OIDC groups from the ID token
	Groups []string
	// UserID is the generated internal user ID (for Casbin)
	UserID string
	// ProjectMemberships are the projects the user has access to via group mappings
	ProjectMemberships []ProjectMembership
	// AssignedRoles contains all Casbin roles assigned to the user.
	// This includes both project roles (proj:project:role) and global roles (role:serveradmin).
	// ArgoCD-aligned: all authorization decisions flow through Casbin policy evaluation.
	AssignedRoles []string
	// Issuer is the OIDC provider issuer URL — part of the federated identity key
	// (org_id, issuer, sub) used to persist the canonical user (Story 15.2).
	Issuer string
	// EmailVerified reflects the IdP's email_verified claim; gates verified-email
	// updates in the identity store (an unverified divergence is not persisted).
	EmailVerified bool
}

// OIDCProvisioningService handles OIDC user evaluation and Casbin role assignment
// No longer creates User CRD - just evaluates group mappings and assigns Casbin roles
type OIDCProvisioningService struct {
	projectService AuthProjectService
	groupMapper    *GroupMapper    // For OIDC group-to-project mapping
	roleManager    AuthRoleManager // For Casbin role assignment
	defaultRole    string          // Default Casbin role for users with no matched group mappings

	// observeLogin persists the canonical user record on each login (Story 15.2).
	// Nil-tolerant: tests and OIDC-disabled paths leave it unset and the call is
	// simply skipped. SourceKind + org scope live on the base store (R5-6/R5-9),
	// captured by the adapter the composition root installs here.
	observeLogin IdentityObserveFunc
}

// NewOIDCProvisioningService creates a new OIDC provisioning service.
// defaultRole is the Casbin role assigned when no group mappings match (e.g., "role:serveradmin").
// Pass empty string to disable default role assignment.
func NewOIDCProvisioningService(projectService AuthProjectService, groupMapper *GroupMapper, roleManager AuthRoleManager, defaultRole string) *OIDCProvisioningService {
	return &OIDCProvisioningService{
		projectService: projectService,
		groupMapper:    groupMapper,
		roleManager:    roleManager,
		defaultRole:    defaultRole,
	}
}

// SetIdentityObserver injects the login-persistence adapter. Wired by the
// composition root (app.go) after the database manager + audit recorder are up.
// Nil-tolerant — a nil func makes ObserveLogin a no-op.
func (s *OIDCProvisioningService) SetIdentityObserver(fn IdentityObserveFunc) {
	s.observeLogin = fn
}

// ValidateOIDCGroups validates OIDC groups from external IdP to prevent injection attacks and DoS
// Uses constants from oidc.go: MaxOIDCGroups (500), MaxOIDCGroupNameLength (256)
func ValidateOIDCGroups(groups []string) error {
	if len(groups) > MaxOIDCGroups {
		return fmt.Errorf("too many OIDC groups: %d (max: %d)", len(groups), MaxOIDCGroups)
	}

	for i, group := range groups {
		// Check for empty strings
		if group == "" {
			return fmt.Errorf("group at index %d is empty", i)
		}

		// Check length to prevent DoS
		if len(group) > MaxOIDCGroupNameLength {
			return fmt.Errorf("group at index %d exceeds max length: %d > %d", i, len(group), MaxOIDCGroupNameLength)
		}

		// Check for null bytes (security: prevent null byte injection)
		if strings.Contains(group, "\x00") {
			return fmt.Errorf("group at index %d contains null byte", i)
		}

		// Validate format (alphanumeric, hyphens, underscores, dots, slashes only)
		if !oidcGroupNameRegex.MatchString(group) {
			return fmt.Errorf("group at index %d has invalid format: %s", i, group)
		}
	}

	return nil
}

// GenerateOIDCUserID generates a stable user ID from OIDC subject
// Format: "user-oidc-{hash}" where hash is first 12 chars of sha256(subject)
func GenerateOIDCUserID(oidcSubject string) string {
	return "user-oidc-" + utilhash.SHA256String(oidcSubject)[:12]
}

// EvaluateOIDCUser processes an OIDC user and returns user info with group mappings.
// Casbin role assignment uses group claims directly. AFTER group resolution it
// also persists a canonical user record via the identity service (Story 15.2);
// that persistence is best-effort — a failure NEVER fails the login (NFR-U1).
//
// issuer + emailVerified come from the OIDC ID token (plumbed by oidc.go) and
// form the federated identity key (org, issuer, sub) plus the verified-email gate.
func (s *OIDCProvisioningService) EvaluateOIDCUser(ctx context.Context, oidcSubject, email, displayName string, groups []string, issuer string, emailVerified bool) (*OIDCUserInfo, error) {
	// Validate email format per RFC 5321/5322
	if err := rbac.ValidateEmail(email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	// Ensure groups is never nil
	if groups == nil {
		groups = []string{}
	}

	// Validate OIDC groups (untrusted input from external IdP)
	if len(groups) > 0 {
		if err := ValidateOIDCGroups(groups); err != nil {
			// Log security event (potential attack)
			slog.Warn("invalid OIDC groups from IdP",
				"error", err,
				"oidc_subject", oidcSubject,
				"email", email,
				"group_count", len(groups),
			)
			return nil, fmt.Errorf("invalid OIDC groups: %w", err)
		}
	}

	// Generate stable user ID from OIDC subject
	userID := GenerateOIDCUserID(oidcSubject)

	// Evaluate group mappings
	var globalRoles []string
	var projectMemberships []ProjectMembership

	if s.groupMapper != nil && len(groups) > 0 {
		mappingResult := s.groupMapper.EvaluateMappings(groups)
		globalRoles = mappingResult.GlobalRoles
		projectMemberships = mappingResult.ProjectMemberships
	}

	// If no roles were resolved from group mappings, assign the default role as fallback
	isDefaultRoleAssignment := false
	if len(globalRoles) == 0 && len(projectMemberships) == 0 && s.defaultRole != "" {
		globalRoles = []string{s.defaultRole}
		isDefaultRoleAssignment = true
		slog.Info("assigning default role to OIDC user (no group mappings matched)",
			"user_id", userID,
			"email", email,
			"default_role", s.defaultRole,
			"source", "default-role-config",
		)
	}

	// Collect all assigned roles for tracking
	assignedRoles := make([]string, 0, len(globalRoles)+len(projectMemberships))

	// Assign Casbin roles based on group mappings
	if s.roleManager != nil {
		// Assign global roles (e.g., role:serveradmin)
		for _, globalRole := range globalRoles {
			hasRole, _ := s.roleManager.HasUserRole(userID, globalRole)
			if !hasRole {
				if _, err := s.roleManager.AddUserRole(userID, globalRole); err != nil {
					slog.Warn("failed to assign global Casbin role",
						"user_id", userID,
						"role", globalRole,
						"error", err,
					)
				} else {
					// Log security event for admin privilege escalation
					if globalRole == rbac.CasbinRoleServerAdmin {
						roleSource := "oidc-group-mapping"
						if isDefaultRoleAssignment {
							roleSource = "default-role-config"
						}
						slog.Warn("SECURITY: Global admin privilege granted",
							"event", "global_admin_granted",
							"event_category", "privilege_escalation",
							"severity", "high",
							"source", roleSource,
							"user_id", userID,
							"user_email", email,
							"oidc_subject", oidcSubject,
							"oidc_groups", groups,
						)
					}
				}
			}
			assignedRoles = append(assignedRoles, globalRole)
		}

		// Assign project roles based on group mappings
		for _, membership := range projectMemberships {
			projectRole := fmt.Sprintf("proj:%s:%s", membership.ProjectID, membership.Role)
			hasRole, _ := s.roleManager.HasUserRole(userID, projectRole)
			if !hasRole {
				if _, err := s.roleManager.AddUserRole(userID, projectRole); err != nil {
					slog.Warn("failed to assign project Casbin role",
						"user_id", userID,
						"project_id", membership.ProjectID,
						"role", membership.Role,
						"error", err,
					)
				} else {
					slog.Info("assigned project role from OIDC group mapping",
						"user_id", userID,
						"project_id", membership.ProjectID,
						"role", membership.Role,
					)
				}
			}
			assignedRoles = append(assignedRoles, projectRole)
		}
	}

	slog.Info("OIDC user evaluated",
		"user_id", userID,
		"email", email,
		"groups_count", len(groups),
		"global_roles", globalRoles,
		"project_memberships_count", len(projectMemberships),
	)

	// Persist the canonical user record AFTER group resolution (NFR-U9: this call
	// feeds nothing back into the group mapper). Best-effort: a failure is logged
	// + metered and the login still succeeds (FR-U3 / NFR-U1). No tx threaded (R5-7).
	if s.observeLogin != nil {
		if err := s.observeLogin(ctx, issuer, oidcSubject, email, displayName, emailVerified); err != nil {
			identityObserveFailures.WithLabelValues(observeFailureReason(err)).Inc()
			slog.Warn("identity observe failed",
				"subsys", "identity",
				"user_id", userID,
				"error", err,
			)
		}
	}

	return &OIDCUserInfo{
		Subject:            oidcSubject,
		Email:              email,
		DisplayName:        displayName,
		Groups:             groups,
		UserID:             userID,
		ProjectMemberships: projectMemberships,
		AssignedRoles:      assignedRoles,
		Issuer:             issuer,
		EmailVerified:      emailVerified,
	}, nil
}

// GetProjectsForUser returns the list of project IDs the user has access to
func (info *OIDCUserInfo) GetProjects() []string {
	projects := make([]string, 0, len(info.ProjectMemberships))
	seen := make(map[string]bool)
	for _, membership := range info.ProjectMemberships {
		if !seen[membership.ProjectID] {
			projects = append(projects, membership.ProjectID)
			seen[membership.ProjectID] = true
		}
	}
	return projects
}

// GetDefaultProject returns the default project for the user (first admin project, or first project)
func (info *OIDCUserInfo) GetDefaultProject() string {
	// Prefer admin projects
	for _, membership := range info.ProjectMemberships {
		if membership.Role == "admin" {
			return membership.ProjectID
		}
	}
	// Fall back to first project
	if len(info.ProjectMemberships) > 0 {
		return info.ProjectMemberships[0].ProjectID
	}
	return ""
}

// HasGlobalAdminRole returns true if the user has the server admin role assigned.
// This checks the AssignedRoles slice for role:serveradmin via Casbin (single source of truth).
func (info *OIDCUserInfo) HasGlobalAdminRole() bool {
	for _, role := range info.AssignedRoles {
		if role == rbac.CasbinRoleServerAdmin {
			return true
		}
	}
	return false
}
