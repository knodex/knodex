// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/rbac"
)

// AgentsRunResultHandler serves GET /api/v1/agents/runs/{id}/result
// (Story 50.1): the full terminal payload of an agent invocation. 404 while
// no result exists — in-flight or unknown run id, indistinguishable by
// design (the page just created the run, so it knows the difference).
//
// Visibility mirrors the runs-list filter (Story 53.1): a result is readable
// only when its AgentNamespace matches the caller's Casbin-derived
// accessible-namespace set — fail closed, 404 non-leak. The empty-namespace
// global-readable carve-out is gone.
type AgentsRunResultHandler struct {
	resultStore runs.ResultStore
	authz       AccessibleNamespacesProvider
}

// NewAgentsRunResultHandler creates an AgentsRunResultHandler. A nil
// resultStore fails soft (404 — nothing is ever readable); a nil authz fails
// closed (no result is readable).
func NewAgentsRunResultHandler(resultStore runs.ResultStore, authz AccessibleNamespacesProvider) *AgentsRunResultHandler {
	return &AgentsRunResultHandler{resultStore: resultStore, authz: authz}
}

// GetResult handles GET /api/v1/agents/runs/{id}/result.
func (h *AgentsRunResultHandler) GetResult(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	id := r.PathValue("id")

	// Nil store ⇒ 404, same shape as "no result yet": the page's hard
	// timeout turns a persistent 404 into the actionable error (AC #4).
	if h.resultStore == nil {
		response.NotFound(w, "agent run result", id)
		return
	}

	result, err := h.resultStore.Get(r.Context(), id)
	if err != nil {
		response.InternalError(w, "Failed to get agent run result")
		return
	}
	if result == nil {
		response.NotFound(w, "agent run result", id)
		return
	}

	// Visibility (Story 53.1): a result is readable only when its namespace
	// matches the caller's Casbin-accessible set — applied unconditionally,
	// fail-closed, 404 non-leak.
	userNamespaces := []string{}
	if h.authz != nil {
		userNamespaces, err = h.authz.GetAccessibleNamespaces(r.Context(), userCtx)
		if err != nil {
			response.InternalError(w, "Failed to get user namespaces")
			return
		}
	}
	if !rbac.MatchNamespaceInList(result.AgentNamespace, userNamespaces) {
		response.NotFound(w, "agent run result", id)
		return
	}

	response.WriteJSON(w, http.StatusOK, result)
}
