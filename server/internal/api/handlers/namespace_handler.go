package handlers

import (
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// NamespaceHandler handles namespace-related API requests
type NamespaceHandler struct {
	namespaceService *rbac.NamespaceService
	policyEnforcer   rbac.Authorizer
	logger           *slog.Logger
}

// NewNamespaceHandler creates a new NamespaceHandler
func NewNamespaceHandler(namespaceService *rbac.NamespaceService, policyEnforcer rbac.Authorizer) *NamespaceHandler {
	return &NamespaceHandler{
		namespaceService: namespaceService,
		policyEnforcer:   policyEnforcer,
		logger:           slog.Default().With("handler", "namespace"),
	}
}

// NamespaceListResponse represents the response for namespace listing
type NamespaceListResponse struct {
	Namespaces []string `json:"namespaces"`
	Count      int      `json:"count"`
}

// ListNamespaces returns namespaces the user can access based on their project memberships.
// Admins see all namespaces; non-admin users see only namespaces matching their projects' destinations.
// GET /api/v1/namespaces
// Query params:
//   - exclude_system: "true" to exclude system namespaces (default: true)
func (h *NamespaceHandler) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := r.Header.Get("X-Request-ID")

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	h.logger.Info("listing namespaces",
		"user", userCtx.UserID,
	)

	// Parse query parameters
	excludeSystemStr := r.URL.Query().Get("exclude_system")
	excludeSystem := !(excludeSystemStr == "false") // Default to excluding system namespaces

	// Check if user has global settings:get access (admins see all namespaces)
	hasGlobalAccess, err := helpers.CheckAccess(ctx, h.policyEnforcer, userCtx, "settings/*", "get")
	if err != nil {
		h.logger.Error("failed to check global access", "error", err, "requestId", requestID)
		response.InternalError(w, "failed to check authorization")
		return
	}

	if hasGlobalAccess {
		// Admin: return all namespaces
		namespaces, err := h.namespaceService.ListNamespaces(ctx, excludeSystem)
		if err != nil {
			h.logger.Error("failed to list namespaces", "error", err)
			response.InternalError(w, "failed to list namespaces")
			return
		}
		resp := NamespaceListResponse{
			Namespaces: namespaces,
			Count:      len(namespaces),
		}
		response.WriteJSON(w, http.StatusOK, resp)
		return
	}

	// Non-admin: filter namespaces by accessible projects
	namespaces, err := h.namespaceService.ListNamespacesForUser(ctx, h.policyEnforcer, userCtx.UserID, userCtx.Groups, excludeSystem)
	if err != nil {
		h.logger.Error("failed to list namespaces for user", "error", err, "user", userCtx.UserID)
		response.InternalError(w, "failed to list namespaces")
		return
	}

	resp := NamespaceListResponse{
		Namespaces: namespaces,
		Count:      len(namespaces),
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// ListProjectNamespaces returns namespaces allowed for a specific project
// GET /api/v1/projects/{name}/namespaces
func (h *NamespaceHandler) ListProjectNamespaces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := r.Header.Get("X-Request-ID")

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Extract project name from URL path
	projectName := r.PathValue("name")
	if projectName == "" {
		response.BadRequest(w, "project name is required", nil)
		return
	}

	// SECURITY (H-2): Validate project name format (DNS-1123 subdomain)
	// Prevents path traversal, injection attacks, and invalid Casbin object construction
	if err := rbac.ValidateProjectName(projectName); err != nil {
		h.logger.Warn("invalid project name format",
			"user", userCtx.UserID,
			"project_name", projectName,
			"error", err,
		)
		response.BadRequest(w, "invalid project name format", nil)
		return
	}

	h.logger.Info("listing project namespaces",
		"user", userCtx.UserID,
		"project", projectName,
	)

	// Check project access via Casbin
	if !helpers.RequireAccess(w, ctx, h.policyEnforcer, userCtx, "projects/"+projectName, "get", requestID) {
		return
	}

	// Get namespaces matching the project's destination patterns
	namespaces, err := h.namespaceService.ListProjectNamespaces(ctx, projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		h.logger.Error("failed to list project namespaces", "error", err, "project", projectName)
		response.InternalError(w, "failed to list namespaces for project")
		return
	}

	// Return the namespace list
	resp := NamespaceListResponse{
		Namespaces: namespaces,
		Count:      len(namespaces),
	}
	response.WriteJSON(w, http.StatusOK, resp)
}
