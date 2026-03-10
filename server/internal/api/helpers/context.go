// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package helpers

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
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
