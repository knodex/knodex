// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// ValidationHandler handles policy validation endpoints
type ValidationHandler struct {
	projectService rbac.ProjectServiceInterface
	policyEnforcer rbac.PolicyEnforcer
}

// NewValidationHandler creates a new ValidationHandler
func NewValidationHandler(projectService rbac.ProjectServiceInterface, policyEnforcer rbac.PolicyEnforcer) *ValidationHandler {
	return &ValidationHandler{
		projectService: projectService,
		policyEnforcer: policyEnforcer,
	}
}

// ValidateProjectCreation handles POST /api/v1/projects/validate
// Validates a new project without creating it (dry-run)
func (h *ValidationHandler) ValidateProjectCreation(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Get user context from middleware (for audit logging)
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	slog.Info("validating project creation",
		"requestId", requestID,
		"userId", userCtx.UserID,
	)

	// Parse and decode request body with size limit
	req, err := helpers.DecodeJSON[ValidateProjectRequest](r, w, 0)
	if err != nil {
		slog.Warn("failed to decode validation request",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.BadRequest(w, "Invalid request body: "+err.Error(), nil)
		return
	}

	// Validate the request itself
	if req.Project == nil {
		response.BadRequest(w, "project field is required", nil)
		return
	}

	// Convert to rbac.Project for validation
	project := toRbacProject(req.Project)

	// Perform validation
	result := h.validateProject(project, nil)

	slog.Info("project validation completed",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", req.Project.Name,
		"valid", result.Valid,
		"errorCount", len(result.Errors),
	)

	response.WriteJSON(w, http.StatusOK, result)
}

// ValidateProjectUpdate handles POST /api/v1/projects/{name}/validate
// Validates updates to an existing project without applying them (dry-run)
func (h *ValidationHandler) ValidateProjectUpdate(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	// Get user context from middleware
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Extract project name from path
	projectName := r.PathValue("name")
	if projectName == "" {
		response.BadRequest(w, "Project name is required", nil)
		return
	}

	slog.Info("validating project update",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", projectName,
	)

	// Check if projectService is available
	if h.projectService == nil {
		slog.Error("project service unavailable",
			"requestId", requestID,
		)
		response.InternalError(w, "Project service unavailable")
		return
	}

	// Get current project
	current, err := h.projectService.GetProject(ctx, projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			slog.Info("project not found for validation",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"project", projectName,
			)
			response.NotFound(w, "Project", projectName)
			return
		}
		slog.Error("failed to get project for validation",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", projectName,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Parse and decode request body with size limit
	req, err := helpers.DecodeJSON[ValidateProjectUpdateRequest](r, w, 0)
	if err != nil {
		slog.Warn("failed to decode validation update request",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", projectName,
			"error", err,
		)
		response.BadRequest(w, "Invalid request body: "+err.Error(), nil)
		return
	}

	// Merge updates with current project
	updated := applyValidationUpdateToProject(current, req)

	// Perform validation with current state for update-specific checks
	result := h.validateProject(updated, current)

	slog.Info("project update validation completed",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", projectName,
		"valid", result.Valid,
		"errorCount", len(result.Errors),
	)

	response.WriteJSON(w, http.StatusOK, result)
}

// validateProject performs comprehensive project validation
func (h *ValidationHandler) validateProject(project *rbac.Project, current *rbac.Project) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// 1. Validate basic project metadata
	if errs := h.validateMetadata(project); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
		// Check if any errors (not warnings) mark the result as invalid
		for _, e := range errs {
			if e.Severity == "error" {
				result.Valid = false
			}
		}
	}

	// 2. Validate roles
	if errs := h.validateRoles(project); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
		for _, e := range errs {
			if e.Severity == "error" {
				result.Valid = false
			}
		}
	}

	// 3. Validate policies (including role references)
	if errs := h.validatePolicies(project); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
		for _, e := range errs {
			if e.Severity == "error" {
				result.Valid = false
			}
		}
	}

	// 4. Check policy conflicts
	if errs := h.checkPolicyConflicts(project); len(errs) > 0 {
		result.Errors = append(result.Errors, errs...)
		// Conflicts are typically warnings, not errors
	}

	// 5. Validate against current state (if update)
	if current != nil {
		if errs := h.validateUpdate(project, current); len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
			// Update warnings don't necessarily mark as invalid
			for _, e := range errs {
				if e.Severity == "error" {
					result.Valid = false
				}
			}
		}
	}

	return result
}

// validateMetadata validates project metadata
func (h *ValidationHandler) validateMetadata(project *rbac.Project) []ValidationError {
	var errors []ValidationError

	// Validate name
	if project.Name == "" {
		errors = append(errors, ValidationError{
			Field:    "name",
			Message:  "name is required",
			Severity: "error",
		})
	} else if !isValidProjectName(project.Name) {
		errors = append(errors, ValidationError{
			Field:    "name",
			Message:  "name must be a valid DNS-1123 subdomain (lowercase alphanumeric with hyphens, 1-63 chars)",
			Severity: "error",
		})
	}

	// Note: destinations are optional for validation
	// They may be added later during project lifecycle

	return errors
}

// validateRoles validates project roles
func (h *ValidationHandler) validateRoles(project *rbac.Project) []ValidationError {
	var errors []ValidationError
	roleNames := make(map[string]bool)

	for i, role := range project.Spec.Roles {
		// Check for empty role name
		if role.Name == "" {
			errors = append(errors, ValidationError{
				Field:    fmt.Sprintf("roles[%d].name", i),
				Message:  "role name is required",
				Severity: "error",
			})
			continue
		}

		// Check for duplicate role names
		if roleNames[role.Name] {
			errors = append(errors, ValidationError{
				Field:    fmt.Sprintf("roles[%d].name", i),
				Message:  fmt.Sprintf("duplicate role name: %s", role.Name),
				Severity: "error",
			})
		}
		roleNames[role.Name] = true

		// Validate role name format (DNS-1123 subdomain)
		if !isValidProjectName(role.Name) {
			errors = append(errors, ValidationError{
				Field:    fmt.Sprintf("roles[%d].name", i),
				Message:  "role name must be a valid DNS-1123 subdomain (lowercase alphanumeric with hyphens)",
				Severity: "error",
			})
		}

		// Validate each policy in the role
		for j, policy := range role.Policies {
			if err := h.validatePolicyRule(project.Name, policy); err != nil {
				errors = append(errors, ValidationError{
					Field:    fmt.Sprintf("roles[%d].policies[%d]", i, j),
					Message:  err.Error(),
					Severity: "error",
				})
			}
		}

		// Validate group names if provided
		for j, group := range role.Groups {
			if !isValidGroupName(group) {
				errors = append(errors, ValidationError{
					Field:    fmt.Sprintf("roles[%d].groups[%d]", i, j),
					Message:  fmt.Sprintf("invalid group name format: %s", group),
					Severity: "error",
				})
			}
		}
	}

	return errors
}

// validatePolicies validates all project policies including role references
func (h *ValidationHandler) validatePolicies(project *rbac.Project) []ValidationError {
	var errors []ValidationError

	// Build map of valid roles for reference checking
	validRoles := make(map[string]bool)
	for _, role := range project.Spec.Roles {
		validRoles[role.Name] = true
	}

	// Check each role's policies for role references
	for i, role := range project.Spec.Roles {
		for j, policy := range role.Policies {
			// Check if policy references other roles (proj:{project}:{role} format)
			if strings.Contains(policy, "proj:") {
				// Extract role references from policy
				referencedRole := extractRoleFromPolicy(policy, project.Name)
				if referencedRole != "" && !validRoles[referencedRole] {
					errors = append(errors, ValidationError{
						Field:    fmt.Sprintf("roles[%d].policies[%d]", i, j),
						Message:  fmt.Sprintf("policy references non-existent role: %s", referencedRole),
						Severity: "error",
					})
				}
			}
		}
	}

	return errors
}

// validatePolicyRule validates a single policy rule
func (h *ValidationHandler) validatePolicyRule(projectName, policy string) error {
	// Trim whitespace
	policy = strings.TrimSpace(policy)

	// Policy format: "p, sub, obj, act" or role binding: "g, user, role"
	parts := strings.Split(policy, ",")
	if len(parts) < 3 {
		return fmt.Errorf("invalid policy format: expected 'p, sub, obj, act' or 'g, user, role'")
	}

	// Trim each part
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	policyType := parts[0]

	switch policyType {
	case "p":
		// Permission policy: p, sub, obj, act
		if len(parts) != 4 {
			return fmt.Errorf("permission policy must have 4 parts: p, sub, obj, act (got %d parts)", len(parts))
		}

		sub := parts[1]
		obj := parts[2]
		act := parts[3]

		// Validate subject format
		if !isValidPolicySubject(sub) {
			return fmt.Errorf("invalid subject format '%s': must be user ID, group:name, or proj:project:role", sub)
		}

		// Validate object starts with projects/{projectName}
		expectedPrefix := fmt.Sprintf("projects/%s", projectName)
		if projectName != "" && !strings.HasPrefix(obj, expectedPrefix) && obj != "*" {
			return fmt.Errorf("object must start with '%s' or be '*', got '%s'", expectedPrefix, obj)
		}

		// Validate action
		validActions := map[string]bool{
			"get": true, "create": true, "update": true, "delete": true, "list": true, "*": true,
		}
		if !validActions[act] {
			return fmt.Errorf("invalid action '%s': must be one of: get, create, update, delete, list, *", act)
		}

	case "g":
		// Role binding: g, user, role
		if len(parts) < 3 {
			return fmt.Errorf("role binding must have at least 3 parts: g, user, role (got %d parts)", len(parts))
		}

		user := parts[1]
		role := parts[2]

		// Validate user format (allow various formats: user ID, group:name, etc.)
		if user == "" {
			return fmt.Errorf("user/subject cannot be empty in role binding")
		}

		// Validate role format
		if role == "" {
			return fmt.Errorf("role cannot be empty in role binding")
		}

	default:
		return fmt.Errorf("invalid policy type '%s': must be 'p' (permission) or 'g' (role binding)", policyType)
	}

	return nil
}

// checkPolicyConflicts detects conflicting or duplicate policies
func (h *ValidationHandler) checkPolicyConflicts(project *rbac.Project) []ValidationError {
	var warnings []ValidationError
	seenPolicies := make(map[string]string) // policy -> first occurrence field path

	for i, role := range project.Spec.Roles {
		for j, policy := range role.Policies {
			// Normalize policy for comparison
			normalizedPolicy := normalizePolicy(policy)
			fieldPath := fmt.Sprintf("roles[%d].policies[%d]", i, j)

			// Check for duplicates (within and across roles)
			if firstPath, exists := seenPolicies[normalizedPolicy]; exists {
				warnings = append(warnings, ValidationError{
					Field:    fieldPath,
					Message:  fmt.Sprintf("duplicate policy (first seen at %s): %s", firstPath, policy),
					Severity: "warning",
				})
			} else {
				seenPolicies[normalizedPolicy] = fieldPath
			}

			// Warn about overly broad wildcards for non-admin roles
			if strings.Contains(policy, "projects/*") && role.Name != "admin" {
				warnings = append(warnings, ValidationError{
					Field:    fieldPath,
					Message:  fmt.Sprintf("overly broad wildcard 'projects/*' granted to non-admin role '%s'", role.Name),
					Severity: "warning",
				})
			}

			// Warn about full wildcard access (*) for non-admin roles
			parts := strings.Split(policy, ",")
			if len(parts) >= 4 {
				act := strings.TrimSpace(parts[3])
				if act == "*" && role.Name != "admin" {
					warnings = append(warnings, ValidationError{
						Field:    fieldPath,
						Message:  fmt.Sprintf("wildcard action '*' granted to non-admin role '%s'", role.Name),
						Severity: "warning",
					})
				}
			}
		}
	}

	return warnings
}

// validateUpdate validates update-specific constraints
func (h *ValidationHandler) validateUpdate(updated, current *rbac.Project) []ValidationError {
	var warnings []ValidationError

	// Build maps of current and updated role names
	currentRoles := make(map[string]bool)
	for _, role := range current.Spec.Roles {
		currentRoles[role.Name] = true
	}

	updatedRoles := make(map[string]bool)
	hasAdminRole := false
	for _, role := range updated.Spec.Roles {
		updatedRoles[role.Name] = true
		if role.Name == "admin" {
			hasAdminRole = true
		}
	}

	// Warn if admin role is being removed
	if currentRoles["admin"] && !hasAdminRole {
		warnings = append(warnings, ValidationError{
			Field:    "roles",
			Message:  "removing admin role may lock out all administrators",
			Severity: "warning",
		})
	}

	// Warn about roles being removed (may orphan users)
	for roleName := range currentRoles {
		if !updatedRoles[roleName] {
			warnings = append(warnings, ValidationError{
				Field:    "roles",
				Message:  fmt.Sprintf("removing role '%s' may orphan users with this role", roleName),
				Severity: "warning",
			})
		}
	}

	return warnings
}

// Helper functions

// isValidPolicySubject validates subject format in policy rules
func isValidPolicySubject(subject string) bool {
	if subject == "" || subject == "*" {
		return true
	}

	// Valid formats:
	// - Simple user ID: alphanumeric with hyphens, underscores, dots, @
	// - Group format: group:{group-name}
	// - Project role: proj:{project}:{role}

	// Group format
	if strings.HasPrefix(subject, "group:") {
		groupName := strings.TrimPrefix(subject, "group:")
		return groupName != "" && isValidGroupName(groupName)
	}

	// Project role format
	if strings.HasPrefix(subject, "proj:") {
		parts := strings.Split(strings.TrimPrefix(subject, "proj:"), ":")
		if len(parts) != 2 {
			return false
		}
		return parts[0] != "" && parts[1] != ""
	}

	// Simple user ID (alphanumeric, hyphens, underscores, dots, @)
	return subjectRegex.MatchString(subject)
}

// isValidGroupName validates OIDC group name format
func isValidGroupName(group string) bool {
	if group == "" {
		return false
	}
	// Group names: alphanumeric, hyphens, underscores, colons, dots, slashes
	return groupNameRegex.MatchString(group)
}

// extractRoleFromPolicy extracts the role name from a policy string
// Returns empty string if no role reference is found
func extractRoleFromPolicy(policy, projectName string) string {
	// Look for proj:{projectName}:{role} pattern
	prefix := fmt.Sprintf("proj:%s:", projectName)
	if idx := strings.Index(policy, prefix); idx >= 0 {
		remainder := policy[idx+len(prefix):]
		// Extract role name (ends at comma, space, or end of string)
		roleEnd := strings.IndexAny(remainder, ", ")
		if roleEnd == -1 {
			roleEnd = len(remainder)
		}
		return remainder[:roleEnd]
	}
	return ""
}

// normalizePolicy normalizes a policy string for duplicate detection
func normalizePolicy(policy string) string {
	parts := strings.Split(policy, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ",")
}

// Regex patterns for validation
var (
	// subjectRegex matches valid user IDs (alphanumeric, hyphens, underscores, dots, @)
	subjectRegex = regexp.MustCompile(`^[a-zA-Z0-9._@-]+$`)

	// groupNameRegex matches valid group names (alphanumeric, hyphens, underscores, colons, dots, slashes)
	groupNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._:/-]+$`)
)
