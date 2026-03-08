package handlers

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
)

// SettingsResponse represents the response from the general settings endpoint.
// This endpoint serves read-only platform configuration visible to all authenticated users.
type SettingsResponse struct {
	Organization string `json:"organization"`
}

// SettingsHandler handles general platform settings endpoints.
type SettingsHandler struct {
	organization string
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(organization string) *SettingsHandler {
	return &SettingsHandler{
		organization: organization,
	}
}

// GetSettings handles GET /api/v1/settings
// Returns platform-level settings. Any authenticated user can read this.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	resp := SettingsResponse{
		Organization: h.organization,
	}
	response.WriteJSON(w, http.StatusOK, resp)
}
