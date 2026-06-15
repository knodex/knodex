// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kagent"
)

// KagentPresenceChecker is the narrow interface the agents status handler
// needs from kagent.Checker. Kept as an interface for test stubbing.
type KagentPresenceChecker interface {
	Check(ctx context.Context) kagent.Result
}

// AgentsStatusHandler handles the kagent presence status endpoint.
// Auth-only by design (explicit Story 49.1 scope decision): registered on the
// protected mux with no Casbin resource — any authenticated user may read it.
type AgentsStatusHandler struct {
	checker KagentPresenceChecker
}

// NewAgentsStatusHandler creates an AgentsStatusHandler. checker may be nil
// when no Kubernetes client is available; the handler then reports degraded.
func NewAgentsStatusHandler(checker KagentPresenceChecker) *AgentsStatusHandler {
	return &AgentsStatusHandler{checker: checker}
}

// GetStatus handles GET /api/v1/agents/status.
// Always responds 200 with one of three states (ready | not_installed |
// degraded) — degraded is a structured payload, never a 5xx (AC #3).
func (h *AgentsStatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	if h.checker == nil {
		response.WriteJSON(w, http.StatusOK, kagent.Result{
			Status:  kagent.StatusDegraded,
			Message: "Kubernetes client unavailable; kagent presence cannot be determined",
		})
		return
	}

	response.WriteJSON(w, http.StatusOK, h.checker.Check(r.Context()))
}
