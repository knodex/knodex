package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/collection"
)

// ProjectHandlerEnforcer defines the focused interface for project operations.
// This follows Interface Segregation Principle - only the methods we need.
type ProjectHandlerEnforcer interface {
	rbac.Authorizer
	rbac.PolicyLoader
	rbac.CacheController
}

// ProjectHandler handles project-related HTTP requests
type ProjectHandler struct {
	projectService rbac.ProjectServiceInterface
	policyEnforcer ProjectHandlerEnforcer
	recorder       audit.Recorder
}

// NewProjectHandler creates a new project handler
func NewProjectHandler(projectService rbac.ProjectServiceInterface, policyEnforcer ProjectHandlerEnforcer, recorder audit.Recorder) *ProjectHandler {
	return &ProjectHandler{
		projectService: projectService,
		policyEnforcer: policyEnforcer,
		recorder:       recorder,
	}
}

// ListProjects handles GET /api/v1/projects
// Lists all projects the user has access to view
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	// Get user context from middleware
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	slog.Info("listing projects",
		"requestId", requestID,
		"userId", userCtx.UserID,
	)

	// List all projects
	projectList, err := h.projectService.ListProjects(ctx)
	if err != nil {
		slog.Error("failed to list projects",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to list projects")
		return
	}

	// Global admins see all projects because their Casbin policies grant wildcard access,
	// NOT because of special code paths. Same code for all users.
	var accessibleProjects []ProjectResponse

	// Security: Fail closed if policy enforcer is not available
	if h.policyEnforcer == nil {
		slog.Warn("policy enforcer unavailable, returning empty project list",
			"requestId", requestID,
			"userId", userCtx.UserID,
		)
		accessibleProjects = []ProjectResponse{}
	} else {
		// Get the list of project names the user can access via Casbin
		accessibleProjectNames, err := h.policyEnforcer.GetAccessibleProjects(ctx, userCtx.UserID, userCtx.Groups)
		if err != nil {
			slog.Error("failed to get accessible projects",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"error", err,
			)
			response.InternalError(w, "Failed to check project access")
			return
		}

		// Build a set of accessible project names for O(1) lookup
		accessibleSet := make(map[string]bool, len(accessibleProjectNames))
		for _, name := range accessibleProjectNames {
			accessibleSet[name] = true
		}

		// Filter the project list by accessible names
		for _, project := range projectList.Items {
			if accessibleSet[project.Name] {
				accessibleProjects = append(accessibleProjects, toProjectResponse(&project))
			}
		}
	}

	if accessibleProjects == nil {
		accessibleProjects = []ProjectResponse{}
	}

	resp := ProjectListResponse{
		Items:      accessibleProjects,
		TotalCount: len(accessibleProjects),
	}

	slog.Info("projects listed successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"count", resp.TotalCount,
	)

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetProject handles GET /api/v1/projects/{name}
// Returns a specific project by name
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	// Get user context from middleware
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Extract project name from path
	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "Project name is required", nil)
		return
	}

	slog.Info("getting project",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", name,
	)

	// Global admins pass because their Casbin policies grant access, NOT because of special code
	// SECURITY: Return 403 Forbidden for authorization failures, not 404
	// This provides clear feedback that the user lacks permission
	// AC-4: Project admin attempting GET on other project returns 403 Forbidden
	if !helpers.RequireAccess(w, ctx, h.policyEnforcer, userCtx, "projects/"+name, "get", requestID) {
		return
	}

	// Get the project
	project, err := h.projectService.GetProject(ctx, name)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			slog.Info("project not found",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"project", name,
			)
			response.NotFound(w, "Project", name)
			return
		}
		slog.Error("failed to get project",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	slog.Info("project retrieved successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", name,
	)

	response.WriteJSON(w, http.StatusOK, toProjectResponse(project))
}

// CreateProject handles POST /api/v1/projects
// Creates a new project
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	// Get user context from middleware
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Parse and decode request body with size limit
	req, err := helpers.DecodeJSON[CreateProjectRequest](r, w, 0)
	if err != nil {
		slog.Warn("failed to decode create project request",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.BadRequest(w, err.Error(), nil)
		return
	}

	slog.Info("creating project",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", req.Name,
	)

	// Validate request
	validationErrors := validateCreateProjectRequest(req)
	if len(validationErrors) > 0 {
		response.BadRequest(w, "Validation failed", validationErrors)
		return
	}

	// Check project creation permission via single Casbin check (STORY-228: never trust JWT roles)
	if h.policyEnforcer == nil {
		slog.Error("policy enforcer not configured", "requestId", requestID)
		response.InternalError(w, "Authorization service unavailable")
		return
	}
	canCreate, err := h.policyEnforcer.CanAccessWithGroups(ctx, userCtx.UserID, userCtx.Groups, "projects/*", "create")
	if err != nil {
		slog.Error("authorization check failed",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Authorization check failed")
		return
	}
	if !canCreate {
		slog.Warn("unauthorized attempt to create project",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", req.Name,
		)
		response.Forbidden(w, "You do not have permission to create projects")
		return
	}

	// Check if project already exists
	exists, err := h.projectService.Exists(ctx, req.Name)
	if err != nil {
		slog.Error("failed to check project existence",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", req.Name,
			"error", err,
		)
		response.InternalError(w, "Failed to check project existence")
		return
	}
	if exists {
		slog.Info("project already exists",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", req.Name,
		)
		response.WriteError(w, http.StatusConflict, "CONFLICT",
			"Project already exists: "+req.Name, nil)
		return
	}

	// Convert request to ProjectSpec
	spec := toProjectSpec(req)

	// Create the project
	project, err := h.projectService.CreateProject(ctx, req.Name, spec, userCtx.UserID)
	if err != nil {
		if errors.Is(err, rbac.ErrAlreadyExists) {
			response.WriteError(w, http.StatusConflict, "CONFLICT",
				"Project already exists: "+req.Name, nil)
			return
		}
		// Surface validation errors from the project service as 400 Bad Request
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid project") {
			slog.Warn("project validation failed",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"project", req.Name,
				"error", err,
			)
			response.BadRequest(w, errMsg, nil)
			return
		}
		slog.Error("failed to create project",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", req.Name,
			"error", err,
		)
		response.InternalError(w, "Failed to create project")
		return
	}

	// AC-1: Immediately load project policies to ensure permission changes take effect
	// This ensures the new project's roles are immediately available for authorization
	h.reloadProjectPolicies(ctx, project, requestID)

	slog.Info("project created successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", req.Name,
	)

	// Build audit details with destination and role names
	createDetails := map[string]any{"description": req.Description}
	if len(req.Destinations) > 0 {
		destNames := make([]string, len(req.Destinations))
		for i, d := range req.Destinations {
			destNames[i] = d.Namespace
		}
		createDetails["destinations"] = destNames
	}
	if len(req.Roles) > 0 {
		roleNames := make([]string, len(req.Roles))
		for i, r := range req.Roles {
			roleNames[i] = r.Name
		}
		createDetails["roles"] = roleNames
	}

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "projects",
		Name:      req.Name,
		Project:   req.Name,
		RequestID: requestID,
		Result:    "success",
		Details:   createDetails,
	})

	response.WriteJSON(w, http.StatusCreated, toProjectResponse(project))
}

// UpdateProject handles PUT /api/v1/projects/{name}
// Updates an existing project
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	// Get user context from middleware
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Extract project name from path
	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "Project name is required", nil)
		return
	}

	// Parse and decode request body with size limit
	req, err := helpers.DecodeJSON[UpdateProjectRequest](r, w, 0)
	if err != nil {
		slog.Warn("failed to decode update project request",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"error", err,
		)
		response.BadRequest(w, err.Error(), nil)
		return
	}

	slog.Info("updating project",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", name,
	)

	// Validate request
	validationErrors := validateUpdateProjectRequest(req)
	if len(validationErrors) > 0 {
		response.BadRequest(w, "Validation failed", validationErrors)
		return
	}

	// Global admins pass because their Casbin policies grant access, NOT because of special code
	if !helpers.RequireAccess(w, ctx, h.policyEnforcer, userCtx, "projects/"+name, "update", requestID) {
		return
	}

	// SECURITY FIX: Restrict role/policy updates via Casbin permission check
	// This prevents privilege escalation where a project admin could grant
	// themselves permissions outside their project scope (e.g., settings access)
	// Reference: ArgoCD pattern - use Casbin Enforce, never check roles directly
	if len(req.Roles) > 0 {
		// Check if user has permission to manage project roles
		// Only global-admin has this permission via wildcard policy: {role:serveradmin, *, *, allow}
		canManageRoles, err := h.policyEnforcer.CanAccessWithGroups(ctx, userCtx.UserID, userCtx.Groups, "projects/roles", "update")
		if err != nil {
			slog.Error("failed to check project role management permission",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"project", name,
				"error", err,
			)
			response.InternalError(w, "Failed to check permissions")
			return
		}

		if !canManageRoles {
			slog.Warn("user attempted to modify project roles without permission",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"project", name,
				"rolesInRequest", len(req.Roles),
			)
			response.Forbidden(w, "You do not have permission to modify project roles and policies")
			return
		}
	}

	// Get the existing project
	project, err := h.projectService.GetProject(ctx, name)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "Project", name)
			return
		}
		slog.Error("failed to get project for update",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"error", err,
		)
		response.InternalError(w, "Failed to get project")
		return
	}

	// Check resource version for optimistic locking
	if req.ResourceVersion != "" && project.ResourceVersion != req.ResourceVersion {
		slog.Info("resource version conflict",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"expected", req.ResourceVersion,
			"actual", project.ResourceVersion,
		)
		response.WriteError(w, http.StatusConflict, "CONFLICT",
			"Resource version conflict. The project has been modified by another request.", nil)
		return
	}

	// Capture old state BEFORE mutation for audit change tracking
	oldDescription := project.Spec.Description
	oldDestinations := make([]string, len(project.Spec.Destinations))
	for i, d := range project.Spec.Destinations {
		oldDestinations[i] = d.Namespace
	}
	oldRoles := make([]string, len(project.Spec.Roles))
	oldRolePolicies := make(map[string][]string, len(project.Spec.Roles))
	for i, r := range project.Spec.Roles {
		oldRoles[i] = r.Name
		oldRolePolicies[r.Name] = r.Policies
	}

	// Apply updates to the project
	applyUpdateToProject(project, req)

	// Update the project
	updatedProject, err := h.projectService.UpdateProject(ctx, project, userCtx.UserID)
	if err != nil {
		if errors.Is(err, rbac.ErrConflict) {
			response.WriteError(w, http.StatusConflict, "CONFLICT",
				"Resource version conflict", nil)
			return
		}
		slog.Error("failed to update project",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"error", err,
		)
		response.InternalError(w, "Failed to update project")
		return
	}

	// AC-1: Immediately reload project policies to ensure permission changes take effect
	// This ensures role definition updates are immediately available for authorization
	h.reloadProjectPolicies(ctx, updatedProject, requestID)

	slog.Info("project updated successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", name,
	)

	// Build change details for audit
	changes := map[string]any{}
	if oldDescription != updatedProject.Spec.Description {
		changes["description"] = audit.SafeChanges(oldDescription, updatedProject.Spec.Description)
	}

	// Compute destination changes
	newDestinations := make([]string, len(updatedProject.Spec.Destinations))
	for i, d := range updatedProject.Spec.Destinations {
		newDestinations[i] = d.Namespace
	}
	addedDests, removedDests := collection.Diff(oldDestinations, newDestinations)
	if len(addedDests) > 0 {
		changes["addedDestinations"] = addedDests
	}
	if len(removedDests) > 0 {
		changes["removedDestinations"] = removedDests
	}

	// Compute role changes (added, removed, modified)
	newRoles := make([]string, len(updatedProject.Spec.Roles))
	newRolePolicies := make(map[string][]string, len(updatedProject.Spec.Roles))
	for i, r := range updatedProject.Spec.Roles {
		newRoles[i] = r.Name
		newRolePolicies[r.Name] = r.Policies
	}
	addedRoles, removedRoles := collection.Diff(oldRoles, newRoles)
	if len(addedRoles) > 0 {
		changes["addedRoles"] = addedRoles
	}
	if len(removedRoles) > 0 {
		changes["removedRoles"] = removedRoles
	}
	// Detect roles that exist in both but have different policies
	modifiedRoles := detectModifiedRoles(oldRolePolicies, newRolePolicies)
	if len(modifiedRoles) > 0 {
		changes["modifiedRoles"] = modifiedRoles
	}

	updateDetails := map[string]any{}
	if len(changes) > 0 {
		updateDetails["changes"] = changes
	}

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "projects",
		Name:      name,
		Project:   name,
		RequestID: requestID,
		Result:    "success",
		Details:   updateDetails,
	})

	response.WriteJSON(w, http.StatusOK, toProjectResponse(updatedProject))
}

// DeleteProject handles DELETE /api/v1/projects/{name}
// Deletes a project
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	// Get user context from middleware
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Extract project name from path
	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "Project name is required", nil)
		return
	}

	slog.Info("deleting project",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", name,
	)

	// Check authorization - only users with delete permission can delete projects
	// Uses Casbin to check "projects/*" delete permission (includes role:serveradmin via wildcard policy)
	if !helpers.RequireAccess(w, ctx, h.policyEnforcer, userCtx, "projects/*", "delete", requestID) {
		return
	}

	// Check if project exists
	exists, err := h.projectService.Exists(ctx, name)
	if err != nil {
		slog.Error("failed to check project existence",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"error", err,
		)
		response.InternalError(w, "Failed to check project existence")
		return
	}
	if !exists {
		response.NotFound(w, "Project", name)
		return
	}

	// Fetch project state before deletion for audit snapshot
	projectSnapshot, snapshotErr := h.projectService.GetProject(ctx, name)
	var deleteDescription string
	var deleteDestsCount, deleteRolesCount int
	if snapshotErr == nil && projectSnapshot != nil {
		deleteDescription = projectSnapshot.Spec.Description
		deleteDestsCount = len(projectSnapshot.Spec.Destinations)
		deleteRolesCount = len(projectSnapshot.Spec.Roles)
	}

	// Remove project policies from enforcer before deletion
	if h.policyEnforcer != nil {
		if err := h.policyEnforcer.RemoveProjectPolicies(ctx, name); err != nil {
			slog.Warn("failed to remove project policies",
				"requestId", requestID,
				"project", name,
				"error", err,
			)
			// Continue with deletion
		}
	}

	// Delete the project
	if err := h.projectService.DeleteProject(ctx, name); err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "Project", name)
			return
		}
		slog.Error("failed to delete project",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", name,
			"error", err,
		)
		response.InternalError(w, "Failed to delete project")
		return
	}

	slog.Info("project deleted successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", name,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "projects",
		Name:      name,
		Project:   name,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"description":       deleteDescription,
			"destinationsCount": deleteDestsCount,
			"rolesCount":        deleteRolesCount,
		},
	})

	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "deleted",
		"project": name,
	})
}

// validateCreateProjectRequest validates a CreateProjectRequest
func validateCreateProjectRequest(req *CreateProjectRequest) map[string]string {
	errors := make(map[string]string)

	// Name is required
	if req.Name == "" {
		errors["name"] = "name is required"
	} else if !isValidProjectName(req.Name) {
		errors["name"] = "name must be a valid DNS-1123 subdomain (lowercase alphanumeric with hyphens, 1-63 chars)"
	}

	// Validate destinations if provided
	for i, dest := range req.Destinations {
		if dest.Namespace == "" {
			errors[fmt.Sprintf("destinations[%d]", i)] = "namespace is required"
		}
	}

	// Validate roles if provided
	for i, role := range req.Roles {
		if role.Name == "" {
			errors[fmt.Sprintf("roles[%d].name", i)] = "role name is required"
		} else if !isValidProjectName(role.Name) {
			errors[fmt.Sprintf("roles[%d].name", i)] = "role name must be a valid DNS-1123 subdomain"
		}
		if len(role.Policies) == 0 {
			errors[fmt.Sprintf("roles[%d].policies", i)] = "at least one policy is required"
		}
		// Check for duplicate role names
		for j := i + 1; j < len(req.Roles); j++ {
			if role.Name == req.Roles[j].Name {
				errors[fmt.Sprintf("roles[%d].name", i)] = fmt.Sprintf("duplicate role name: %s", role.Name)
			}
		}
	}

	return errors
}

// validateUpdateProjectRequest validates an UpdateProjectRequest
func validateUpdateProjectRequest(req *UpdateProjectRequest) map[string]string {
	errors := make(map[string]string)

	// ResourceVersion is required for optimistic locking
	if req.ResourceVersion == "" {
		errors["resourceVersion"] = "resourceVersion is required for updates"
	}

	// Validate destinations if provided
	for i, dest := range req.Destinations {
		if dest.Namespace == "" {
			errors[fmt.Sprintf("destinations[%d]", i)] = "namespace is required"
		}
	}

	return errors
}

// isValidProjectName validates that a name follows DNS-1123 subdomain format
func isValidProjectName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	// Must start with alphanumeric
	first := name[0]
	if !((first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return false
	}

	// Must end with alphanumeric
	last := name[len(name)-1]
	if !((last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) {
		return false
	}

	// Check all characters
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}

	return true
}

// reloadProjectPolicies reloads Casbin policies for a project and invalidates the cache.
// This ensures permission changes take effect immediately without waiting for the periodic sync.
// AC-5: Logs policy reload with project name and duration for observability.
// AC-6: Errors are logged but do not propagate - the CRD update was already successful.
func (h *ProjectHandler) reloadProjectPolicies(ctx context.Context, project *rbac.Project, requestID string) {
	if h.policyEnforcer == nil || project == nil {
		return
	}

	start := time.Now()

	// Reload project policies to ensure all role definitions and group mappings are current
	// LoadProjectPolicies already clears the authorization cache internally.
	if err := h.policyEnforcer.LoadProjectPolicies(ctx, project); err != nil {
		// AC-6: Log error but don't fail - the CRD update already succeeded
		slog.Error("immediate policy reload failed after project update, watcher will sync eventually",
			"requestId", requestID,
			"project", project.Name,
			"error", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return
	}

	// AC-5: Log successful policy reload with duration
	slog.Info("immediate policy reload completed after project update",
		"requestId", requestID,
		"project", project.Name,
		"duration_ms", time.Since(start).Milliseconds(),
	)
}

// detectModifiedRoles returns names of roles that exist in both old and new
// but have different policy sets.
func detectModifiedRoles(oldPolicies, newPolicies map[string][]string) []string {
	var modified []string
	for name, oldP := range oldPolicies {
		newP, exists := newPolicies[name]
		if !exists {
			continue // removed role, handled by collection.Diff
		}
		if !collection.Equal(oldP, newP) {
			modified = append(modified, name)
		}
	}
	return modified
}
