// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
)

// AccountInfoResponse represents the response from the account info endpoint
type AccountInfoResponse struct {
	UserID         string            `json:"userID"`
	Email          string            `json:"email"`
	DisplayName    string            `json:"displayName"`
	Groups         []string          `json:"groups"`
	CasbinRoles    []string          `json:"casbinRoles"`
	Projects       []string          `json:"projects"`
	Roles          map[string]string `json:"roles"`
	Issuer         string            `json:"issuer"`
	TokenExpiresAt int64             `json:"tokenExpiresAt"`
	TokenIssuedAt  int64             `json:"tokenIssuedAt"`
}

// CanIServiceInterface defines the interface for permission checking operations
type CanIServiceInterface interface {
	CanI(userID string, groups []string, resource, action, subresource string) (bool, error)
	// GetMappedGroups returns only the groups that have associated Casbin policies.
	// Groups without role mappings are filtered out.
	GetMappedGroups(groups []string) ([]string, error)
}

// AccountHandler handles account-related endpoints (ArgoCD-style)
type AccountHandler struct {
	canIService         CanIServiceInterface
	projectService      rbac.ProjectServiceInterface // optional, nil = skip project existence check
	enterpriseResources map[string]bool              // EE-only resources registered at startup
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(canIService CanIServiceInterface) *AccountHandler {
	return &AccountHandler{
		canIService:         canIService,
		enterpriseResources: make(map[string]bool),
	}
}

// SetProjectService sets the project service for project existence validation in can-i.
func (h *AccountHandler) SetProjectService(ps rbac.ProjectServiceInterface) {
	h.projectService = ps
}

// RegisterEnterpriseResource marks a resource as valid for can-i checks in EE builds.
// Call this at startup for each enterprise feature that is enabled (e.g., "secrets", "compliance").
// Resources not registered here return 400 "invalid resource type" in OSS builds.
func (h *AccountHandler) RegisterEnterpriseResource(resource string) {
	h.enterpriseResources[resource] = true
}

// projectScopedResources are resources that require a valid project name in the subresource.
var projectScopedResources = map[string]bool{
	"instances":    true,
	"projects":     true,
	"repositories": true,
	"secrets":      true,
	"rgds":         true,
	"compliance":   true,
	"applications": true,
}

// isProjectScopedResource returns true if the resource requires project scoping.
func isProjectScopedResource(resource string) bool {
	return projectScopedResources[resource]
}

// extractProjectName extracts the project name from a subresource string.
// For subresources like "project/namespace", returns just the project name.
// Returns empty string if the subresource is empty or starts with "/".
func extractProjectName(subresource string) string {
	if idx := strings.Index(subresource, "/"); idx > 0 {
		return subresource[:idx]
	}
	if strings.HasPrefix(subresource, "/") {
		return ""
	}
	return subresource
}

// validateSubresource returns an error if the subresource parameter contains
// characters that could manipulate Casbin policy evaluation or indicate injection.
// Valid subresources: project names, "project/namespace" pairs, or "-" for none.
func validateSubresource(subresource string) error {
	// Allow empty string and sentinel "-"
	if subresource == "" || subresource == "-" {
		return nil
	}
	// Reject Casbin policy syntax characters (including tab which some CSV parsers treat as separator)
	if strings.ContainsAny(subresource, "*,|\n\r\t") {
		return errors.New("subresource contains invalid characters")
	}
	// Reject path traversal sequences (raw and URL-encoded, forward and backslash)
	lower := strings.ToLower(subresource)
	if strings.Contains(subresource, "../") || strings.Contains(subresource, "..\\") ||
		strings.Contains(lower, "..%2f") || strings.Contains(lower, "..%5c") {
		return errors.New("subresource contains path traversal sequence")
	}
	return nil
}

// CanIResponse represents the response from the can-i endpoint
type CanIResponse struct {
	Value string `json:"value"` // "yes" or "no"
}

// CanI handles GET /api/v1/account/can-i/{resource}/{action}/{subresource}
// This is an ArgoCD-style endpoint for real-time permission checks.
//
// Path parameters:
//   - resource: The resource type (e.g., "instances", "projects", "repositories", "settings")
//   - action: The action to check (e.g., "create", "delete", "get", "update")
//   - subresource: Optional subresource (e.g., project name, namespace). Use "-" for none.
//
// Response:
//
//	{
//	  "value": "yes"  // or "no"
//	}
//
// Example requests:
//
//	GET /api/v1/account/can-i/instances/create/my-project
//	GET /api/v1/account/can-i/projects/delete/-
//	GET /api/v1/account/can-i/settings/update/-
func (h *AccountHandler) CanI(w http.ResponseWriter, r *http.Request) {
	// Get path parameters
	resource := r.PathValue("resource")
	action := r.PathValue("action")
	subresource := r.PathValue("subresource")

	// Validate required parameters
	if resource == "" {
		response.BadRequest(w, "resource parameter is required", nil)
		return
	}
	if action == "" {
		response.BadRequest(w, "action parameter is required", nil)
		return
	}

	// Validate resource type.
	// Base set covers OSS resources; enterprise resources (e.g., "secrets", "compliance") are
	// added at startup via RegisterEnterpriseResource so OSS builds return 400 for them.
	validResources := map[string]bool{
		"instances":    true,
		"projects":     true,
		"repositories": true,
		"settings":     true,
		"rgds":         true,
		"users":        true,
		"applications": true,
	}
	for r := range h.enterpriseResources {
		validResources[r] = true
	}
	if !validResources[resource] {
		response.BadRequest(w, "invalid resource type", map[string]string{
			"resource": resource,
		})
		return
	}

	// Validate action
	validActions := map[string]bool{
		"get":    true,
		"list":   true,
		"create": true,
		"update": true,
		"delete": true,
	}
	if !validActions[action] {
		response.BadRequest(w, "invalid action", map[string]string{
			"action": action,
		})
		return
	}

	// Validate subresource parameter to prevent Casbin policy injection
	if err := validateSubresource(subresource); err != nil {
		slog.Warn("rejected invalid subresource in can-i request",
			"subresource", subresource,
			"error", err,
		)
		response.BadRequest(w, "invalid subresource parameter", nil)
		return
	}

	// Get user context from request (set by auth middleware)
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.WriteError(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "authentication required", nil)
		return
	}

	// Project existence check (LOG-VULN-04): prevent permission enumeration for nonexistent projects.
	// Runs BEFORE canIService.CanI() so even admins get 404 for nonexistent projects.
	if h.projectService != nil && isProjectScopedResource(resource) && subresource != "" && subresource != "-" {
		projectName := extractProjectName(subresource)
		if projectName == "" {
			response.BadRequest(w, "invalid subresource parameter", map[string]string{"subresource": subresource})
			return
		}
		exists, err := h.projectService.Exists(r.Context(), projectName)
		if err != nil {
			slog.Error("failed to check project existence in can-i",
				"project", projectName, "error", err)
			response.InternalError(w, "failed to check project existence")
			return
		}
		if !exists {
			response.NotFound(w, "project", projectName)
			return
		}
	}

	// Check permission using the service
	allowed, err := h.canIService.CanI(userCtx.UserID, userCtx.Groups, resource, action, subresource)
	if err != nil {
		slog.Error("failed to check permission",
			"user_id", userCtx.UserID,
			"resource", resource,
			"action", action,
			"subresource", subresource,
			"error", err,
		)
		response.InternalError(w, "failed to check permission")
		return
	}

	// Return ArgoCD-style response
	resp := CanIResponse{
		Value: "no",
	}
	if allowed {
		resp.Value = "yes"
	}

	slog.Debug("can-i permission check",
		"user_id", userCtx.UserID,
		"resource", resource,
		"action", action,
		"subresource", subresource,
		"allowed", allowed,
	)

	response.WriteJSON(w, http.StatusOK, resp)
}

// Info handles GET /api/v1/account/info
// Returns the authenticated user's account information including identity,
// groups, roles, projects, and token metadata.
// Accessible to ALL authenticated users (no special RBAC permission needed).
func (h *AccountHandler) Info(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	// Determine issuer display value.
	// Local admin tokens have no issuer set (empty string) in JWTClaims.
	// Using Issuer field instead of email suffix avoids spoofing if an OIDC
	// provider issues tokens with an @local email domain.
	issuer := userCtx.Issuer
	if issuer == "" {
		issuer = "Local"
	}

	// Filter groups to only those with associated Casbin policies.
	// This avoids displaying all OIDC groups from the IdP when most have no
	// Knodex role mapping.
	groups, err := h.canIService.GetMappedGroups(userCtx.Groups)
	if err != nil {
		slog.Warn("failed to filter mapped groups, returning all groups",
			"user_id", userCtx.UserID,
			"error", err,
		)
		groups = userCtx.Groups
	}
	// Ensure slices are non-nil for consistent JSON output ([] not null)
	if groups == nil {
		groups = []string{}
	}
	casbinRoles := userCtx.CasbinRoles
	if casbinRoles == nil {
		casbinRoles = []string{}
	}
	projects := userCtx.Projects
	if projects == nil {
		projects = []string{}
	}
	roles := userCtx.Roles
	if roles == nil {
		roles = map[string]string{}
	}

	resp := AccountInfoResponse{
		UserID:         userCtx.UserID,
		Email:          userCtx.Email,
		DisplayName:    userCtx.DisplayName,
		Groups:         groups,
		CasbinRoles:    casbinRoles,
		Projects:       projects,
		Roles:          roles,
		Issuer:         issuer,
		TokenExpiresAt: userCtx.TokenExpiresAt,
		TokenIssuedAt:  userCtx.TokenIssuedAt,
	}

	response.WriteJSON(w, http.StatusOK, resp)
}
