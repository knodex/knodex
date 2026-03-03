package handlers

import (
	"log/slog"
	"net/http"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
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
	canIService CanIServiceInterface
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(canIService CanIServiceInterface) *AccountHandler {
	return &AccountHandler{
		canIService: canIService,
	}
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

	// Validate resource type
	validResources := map[string]bool{
		"instances":    true,
		"projects":     true,
		"repositories": true,
		"settings":     true,
		"rgds":         true,
		"users":        true,
		"applications": true,
		"compliance":   true,
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

	// Get user context from request (set by auth middleware)
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.WriteError(w, http.StatusUnauthorized, response.ErrCodeUnauthorized, "authentication required", nil)
		return
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
