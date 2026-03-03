package helpers

import (
	"net/http"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
)

// RequireUserContext extracts user context or writes 401 response.
// Returns nil if user context is missing (response already written).
func RequireUserContext(w http.ResponseWriter, r *http.Request) *middleware.UserContext {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.Unauthorized(w, "Authentication required")
		return nil
	}
	return userCtx
}
