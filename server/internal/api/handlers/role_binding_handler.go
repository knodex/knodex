// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/rbac"
)

// RoleBindingEnforcer defines the focused interface for role binding operations.
// This follows Interface Segregation Principle - only the methods we need.
type RoleBindingEnforcer interface {
	rbac.Authorizer
	rbac.RoleManager
	rbac.CacheController
	rbac.PolicyLoader
}

// RoleBindingHandler handles HTTP requests for role binding operations
type RoleBindingHandler struct {
	projectService rbac.ProjectServiceInterface
	enforcer       RoleBindingEnforcer
	recorder       audit.Recorder
}

// NewRoleBindingHandler creates a new RoleBindingHandler
func NewRoleBindingHandler(
	projectService rbac.ProjectServiceInterface,
	enforcer RoleBindingEnforcer,
	recorder audit.Recorder,
) *RoleBindingHandler {
	return &RoleBindingHandler{
		projectService: projectService,
		enforcer:       enforcer,
		recorder:       recorder,
	}
}

// AssignUserRole handles POST /api/v1/projects/{name}/roles/{role}/users/{user}
// Assigns a user to a project role
func (h *RoleBindingHandler) AssignUserRole(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("name")
	roleName := r.PathValue("role")
	userID := r.PathValue("user")
	requestID := r.Header.Get("X-Request-ID")

	if projectName == "" || roleName == "" || userID == "" {
		response.BadRequest(w, "project name, role, and user are required", nil)
		return
	}

	// SECURITY: Validate subject identifier to prevent Casbin injection
	if err := rbac.ValidateSubjectIdentifier(userID); err != nil {
		response.BadRequest(w, fmt.Sprintf("invalid user identifier: %v", err), nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Check authorization for role assignment
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "projects/"+projectName, "member-add", requestID) {
		return
	}

	// Get project to validate it exists and role is defined
	project, err := h.projectService.GetProject(r.Context(), projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		slog.Error("failed to get project",
			"requestId", requestID,
			"project", projectName,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Validate role exists in project
	if !roleExistsInProject(project, roleName) {
		response.BadRequest(w, fmt.Sprintf("role '%s' not defined in project '%s'", roleName, projectName), nil)
		return
	}

	// Check for existing role to distinguish member_add from role_change
	oldRole := h.existingProjectRole(r.Context(), userID, projectName)

	// Assign role via enforcer using project:role format
	roleSubject := fmt.Sprintf("proj:%s:%s", projectName, roleName)
	err = h.enforcer.AssignUserRoles(r.Context(), userID, []string{roleSubject})
	if err != nil {
		slog.Error("failed to assign role",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"user", userID,
			"error", err,
		)
		response.InternalError(w, "Failed to assign role")
		return
	}

	// Persist role binding in project annotations
	if err := h.addRoleBindingToProject(r.Context(), project, roleName, userID, "user"); err != nil {
		slog.Warn("failed to persist role binding in project annotations",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"user", userID,
			"error", err,
		)
		// Continue - assignment succeeded in Casbin
	}

	// Note: We do NOT call reloadProjectPolicies here because:
	// 1. AssignUserRoles already added the user-role mapping to Casbin in-memory
	// 2. LoadProjectPolicies would wipe out this mapping (it only restores role.Groups, not user bindings)
	// 3. The in-memory Casbin binding provides immediate permission effect
	// 4. The watcher will sync from CRD annotations for persistence across restarts

	// Invalidate cache to ensure the new permission is reflected immediately
	if h.enforcer != nil {
		h.enforcer.InvalidateCacheForProject(projectName)
	}

	slog.Info("user assigned to project role",
		"requestId", requestID,
		"project", projectName,
		"role", roleName,
		"user", userID,
		"assignedBy", userCtx.UserID,
	)

	auditAction := "member_add"
	auditDetails := map[string]any{"targetUser": userID, "role": roleName, "type": "user"}
	if oldRole != "" && oldRole != roleName {
		auditAction = "role_change"
		auditDetails["previousRole"] = oldRole
	}
	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    auditAction,
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "success",
		Details:   auditDetails,
	})

	response.WriteJSON(w, http.StatusCreated, RoleBindingResponse{
		Project: projectName,
		Role:    roleName,
		Subject: userID,
		Type:    "user",
	})
}

// AssignGroupRole handles POST /api/v1/projects/{name}/roles/{role}/groups/{group}
// Assigns a group to a project role
func (h *RoleBindingHandler) AssignGroupRole(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("name")
	roleName := r.PathValue("role")
	groupName := r.PathValue("group")
	requestID := r.Header.Get("X-Request-ID")

	if projectName == "" || roleName == "" || groupName == "" {
		response.BadRequest(w, "project name, role, and group are required", nil)
		return
	}

	// SECURITY: Validate subject identifier to prevent Casbin injection
	if err := rbac.ValidateSubjectIdentifier(groupName); err != nil {
		response.BadRequest(w, fmt.Sprintf("invalid group identifier: %v", err), nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Check authorization for role assignment
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "projects/"+projectName, "member-add", requestID) {
		return
	}

	// Get project to validate it exists and role is defined
	project, err := h.projectService.GetProject(r.Context(), projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		slog.Error("failed to get project",
			"requestId", requestID,
			"project", projectName,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Validate role exists in project
	if !roleExistsInProject(project, roleName) {
		response.BadRequest(w, fmt.Sprintf("role '%s' not defined in project '%s'", roleName, projectName), nil)
		return
	}

	// Check for existing role to distinguish member_add from role_change
	groupSubject := fmt.Sprintf("group:%s", groupName)
	oldRole := h.existingProjectRole(r.Context(), groupSubject, projectName)

	// Assign role to group via enforcer
	roleSubject := fmt.Sprintf("proj:%s:%s", projectName, roleName)
	err = h.enforcer.AssignUserRoles(r.Context(), groupSubject, []string{roleSubject})
	if err != nil {
		slog.Error("failed to assign role to group",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"group", groupName,
			"error", err,
		)
		response.InternalError(w, "Failed to assign role to group")
		return
	}

	// Persist role binding in project annotations
	if err := h.addRoleBindingToProject(r.Context(), project, roleName, groupName, "group"); err != nil {
		slog.Warn("failed to persist group role binding in project annotations",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"group", groupName,
			"error", err,
		)
		// Continue - assignment succeeded in Casbin
	}

	// Note: We do NOT call reloadProjectPolicies here because:
	// 1. AssignGroupRole already added the group-role mapping to Casbin in-memory
	// 2. LoadProjectPolicies would wipe out this mapping (it removes all project policies first)
	// 3. The in-memory Casbin binding provides immediate permission effect
	// 4. The watcher will sync from CRD annotations for persistence across restarts

	// Invalidate cache to ensure the new permission is reflected immediately
	if h.enforcer != nil {
		h.enforcer.InvalidateCacheForProject(projectName)
	}

	slog.Info("group assigned to project role",
		"requestId", requestID,
		"project", projectName,
		"role", roleName,
		"group", groupName,
		"assignedBy", userCtx.UserID,
	)

	auditAction := "member_add"
	auditDetails := map[string]any{"targetGroup": groupName, "role": roleName, "type": "group"}
	if oldRole != "" && oldRole != roleName {
		auditAction = "role_change"
		auditDetails["previousRole"] = oldRole
	}
	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    auditAction,
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "success",
		Details:   auditDetails,
	})

	response.WriteJSON(w, http.StatusCreated, RoleBindingResponse{
		Project: projectName,
		Role:    roleName,
		Subject: groupName,
		Type:    "group",
	})
}

// ListRoleBindings handles GET /api/v1/projects/{name}/role-bindings
// Lists all role bindings for a project
func (h *RoleBindingHandler) ListRoleBindings(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("name")
	requestID := r.Header.Get("X-Request-ID")

	if projectName == "" {
		response.BadRequest(w, "project name is required", nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Check authorization to view project
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "projects/"+projectName, "get", requestID) {
		return
	}

	// Get project
	project, err := h.projectService.GetProject(r.Context(), projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		slog.Error("failed to get project",
			"requestId", requestID,
			"project", projectName,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Extract role bindings from project annotations
	bindings := extractRoleBindingsFromProject(project)

	slog.Debug("role bindings listed",
		"requestId", requestID,
		"project", projectName,
		"count", len(bindings),
	)

	response.WriteJSON(w, http.StatusOK, ListRoleBindingsResponse{
		Project:  projectName,
		Bindings: bindings,
	})
}

// RemoveUserRole handles DELETE /api/v1/projects/{name}/roles/{role}/users/{user}
// Removes a user from a project role
func (h *RoleBindingHandler) RemoveUserRole(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("name")
	roleName := r.PathValue("role")
	userID := r.PathValue("user")
	requestID := r.Header.Get("X-Request-ID")

	if projectName == "" || roleName == "" || userID == "" {
		response.BadRequest(w, "project name, role, and user are required", nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Check authorization for role removal
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "projects/"+projectName, "member-remove", requestID) {
		return
	}

	// Get project to validate it exists
	project, err := h.projectService.GetProject(r.Context(), projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		slog.Error("failed to get project",
			"requestId", requestID,
			"project", projectName,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Remove role from user via enforcer
	roleSubject := fmt.Sprintf("proj:%s:%s", projectName, roleName)
	err = h.enforcer.RemoveUserRole(r.Context(), userID, roleSubject)
	if err != nil {
		slog.Error("failed to remove role",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"user", userID,
			"error", err,
		)
		response.InternalError(w, "Failed to remove role")
		return
	}

	// Remove role binding from project annotations
	if err := h.removeRoleBindingFromProject(r.Context(), project, roleName, userID, "user"); err != nil {
		slog.Warn("failed to remove role binding from project annotations",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"user", userID,
			"error", err,
		)
		// Continue - removal succeeded in Casbin
	}

	// Note: We do NOT call reloadProjectPolicies here because:
	// 1. RemoveUserRole already removed the user-role mapping from Casbin in-memory
	// 2. LoadProjectPolicies would be redundant (the mapping is already gone)
	// 3. The in-memory Casbin removal provides immediate permission effect

	// Invalidate cache to ensure the permission removal is reflected immediately
	if h.enforcer != nil {
		h.enforcer.InvalidateCacheForProject(projectName)
	}

	slog.Info("user removed from project role",
		"requestId", requestID,
		"project", projectName,
		"role", roleName,
		"user", userID,
		"removedBy", userCtx.UserID,
	)

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "member_remove",
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"targetUser": userID, "role": roleName, "type": "user"},
	})

	w.WriteHeader(http.StatusNoContent)
}

// RemoveGroupRole handles DELETE /api/v1/projects/{name}/roles/{role}/groups/{group}
// Removes a group from a project role
func (h *RoleBindingHandler) RemoveGroupRole(w http.ResponseWriter, r *http.Request) {
	projectName := r.PathValue("name")
	roleName := r.PathValue("role")
	groupName := r.PathValue("group")
	requestID := r.Header.Get("X-Request-ID")

	if projectName == "" || roleName == "" || groupName == "" {
		response.BadRequest(w, "project name, role, and group are required", nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Check authorization for role removal
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "projects/"+projectName, "member-remove", requestID) {
		return
	}

	// Get project to validate it exists
	project, err := h.projectService.GetProject(r.Context(), projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		slog.Error("failed to get project",
			"requestId", requestID,
			"project", projectName,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Remove role from group via enforcer
	roleSubject := fmt.Sprintf("proj:%s:%s", projectName, roleName)
	groupSubject := fmt.Sprintf("group:%s", groupName)
	err = h.enforcer.RemoveUserRole(r.Context(), groupSubject, roleSubject)
	if err != nil {
		slog.Error("failed to remove role from group",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"group", groupName,
			"error", err,
		)
		response.InternalError(w, "Failed to remove role from group")
		return
	}

	// Remove role binding from project annotations
	if err := h.removeRoleBindingFromProject(r.Context(), project, roleName, groupName, "group"); err != nil {
		slog.Warn("failed to remove group role binding from project annotations",
			"requestId", requestID,
			"project", projectName,
			"role", roleName,
			"group", groupName,
			"error", err,
		)
		// Continue - removal succeeded in Casbin
	}

	// Note: We do NOT call reloadProjectPolicies here because:
	// 1. RemoveGroupRole already removed the group-role mapping from Casbin in-memory
	// 2. LoadProjectPolicies would be redundant (the mapping is already gone)
	// 3. The in-memory Casbin removal provides immediate permission effect

	// Invalidate cache to ensure the permission removal is reflected immediately
	if h.enforcer != nil {
		h.enforcer.InvalidateCacheForProject(projectName)
	}

	slog.Info("group removed from project role",
		"requestId", requestID,
		"project", projectName,
		"role", roleName,
		"group", groupName,
		"removedBy", userCtx.UserID,
	)

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "member_remove",
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"targetGroup": groupName, "role": roleName, "type": "group"},
	})

	w.WriteHeader(http.StatusNoContent)
}

// Helper Functions

// roleExistsInProject checks if a role is defined in the project spec
func roleExistsInProject(project *rbac.Project, roleName string) bool {
	for _, role := range project.Spec.Roles {
		if role.Name == roleName {
			return true
		}
	}
	return false
}

// existingProjectRole returns the current project role for a subject (user or group),
// or empty string if the subject has no role in the project.
// This is used to distinguish member_add from role_change audit actions.
func (h *RoleBindingHandler) existingProjectRole(ctx context.Context, subject, projectName string) string {
	roles, err := h.enforcer.GetUserRoles(ctx, subject)
	if err != nil {
		return ""
	}
	prefix := fmt.Sprintf("proj:%s:", projectName)
	for _, role := range roles {
		if strings.HasPrefix(role, prefix) {
			return strings.TrimPrefix(role, prefix)
		}
	}
	return ""
}

// addRoleBindingToProject adds a role binding to project annotations
func (h *RoleBindingHandler) addRoleBindingToProject(
	ctx context.Context,
	project *rbac.Project,
	roleName, subject, subjectType string,
) error {
	if project.Annotations == nil {
		project.Annotations = make(map[string]string)
	}

	// Store role bindings as JSON in annotation
	bindingKey := "knodex.io/role-bindings"
	existingBindings := extractRoleBindingsFromProject(project)

	// Check if binding already exists
	for _, b := range existingBindings {
		if b.Role == roleName && b.Subject == subject && b.Type == subjectType {
			// Binding already exists, no need to add
			return nil
		}
	}

	// Add new binding
	newBinding := RoleBinding{
		Role:    roleName,
		Subject: subject,
		Type:    subjectType,
	}
	existingBindings = append(existingBindings, newBinding)

	// Serialize back to JSON
	bindingsJSON, err := json.Marshal(existingBindings)
	if err != nil {
		return fmt.Errorf("failed to marshal role bindings: %w", err)
	}

	project.Annotations[bindingKey] = string(bindingsJSON)

	// Update project
	_, err = h.projectService.UpdateProject(ctx, project, "system")
	return err
}

// removeRoleBindingFromProject removes a role binding from project annotations
func (h *RoleBindingHandler) removeRoleBindingFromProject(
	ctx context.Context,
	project *rbac.Project,
	roleName, subject, subjectType string,
) error {
	bindingKey := "knodex.io/role-bindings"
	existingBindings := extractRoleBindingsFromProject(project)

	// Filter out the binding to remove
	filteredBindings := []RoleBinding{}
	for _, binding := range existingBindings {
		if binding.Role != roleName || binding.Subject != subject || binding.Type != subjectType {
			filteredBindings = append(filteredBindings, binding)
		}
	}

	// Serialize back to JSON
	bindingsJSON, err := json.Marshal(filteredBindings)
	if err != nil {
		return fmt.Errorf("failed to marshal role bindings: %w", err)
	}

	if project.Annotations == nil {
		project.Annotations = make(map[string]string)
	}
	project.Annotations[bindingKey] = string(bindingsJSON)

	// Update project
	_, err = h.projectService.UpdateProject(ctx, project, "system")
	return err
}

// extractRoleBindingsFromProject extracts role bindings from project annotations
func extractRoleBindingsFromProject(project *rbac.Project) []RoleBinding {
	if project.Annotations == nil {
		return []RoleBinding{}
	}

	bindingKey := "knodex.io/role-bindings"
	bindingsJSON, ok := project.Annotations[bindingKey]
	if !ok || bindingsJSON == "" {
		return []RoleBinding{}
	}

	var bindings []RoleBinding
	if err := json.Unmarshal([]byte(bindingsJSON), &bindings); err != nil {
		return []RoleBinding{}
	}

	return bindings
}
