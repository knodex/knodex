// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"net/http"
	"strconv"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/rbac"
)

const (
	defaultRunsPageSize = 20
	maxRunsPageSize     = 100
)

// AgentRunsResponse is the envelope for GET /api/v1/agents/runs — the same
// shape as the compliance list endpoints (ComplianceListResponse precedent).
type AgentRunsResponse struct {
	Items    []runs.Run `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// AgentsRunsHandler serves the run history table of the /agents workspace
// (Story 49.4). Visibility is the Casbin-derived accessible-namespace set
// applied inside the handler (single enforcement layer): a run is visible only
// when its agent namespace matches the caller's set (Story 53.1 removed the
// empty-namespace global carve-out). Pagination happens AFTER the visibility
// filter so page numbers are stable per-caller.
type AgentsRunsHandler struct {
	store runs.Store
	authz AccessibleNamespacesProvider
}

// NewAgentsRunsHandler creates an AgentsRunsHandler. A nil store yields an
// empty list (fail-soft); a nil authz fails closed (no runs visible).
func NewAgentsRunsHandler(store runs.Store, authz AccessibleNamespacesProvider) *AgentsRunsHandler {
	return &AgentsRunsHandler{store: store, authz: authz}
}

// parsePositiveInt parses a query parameter as a strictly positive integer,
// returning (fallback, true) when absent and (0, false) when invalid.
func parsePositiveInt(raw string, fallback int) (int, bool) {
	if raw == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// ListRuns handles GET /api/v1/agents/runs?agentType=&status=&page=&pageSize=.
func (h *AgentsRunsHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	q := r.URL.Query()
	page, ok := parsePositiveInt(q.Get("page"), 1)
	if !ok {
		response.BadRequest(w, "page must be a positive integer", map[string]string{"field": "page"})
		return
	}
	pageSize, ok := parsePositiveInt(q.Get("pageSize"), defaultRunsPageSize)
	if !ok || pageSize > maxRunsPageSize {
		response.BadRequest(w, "pageSize must be between 1 and 100", map[string]string{"field": "pageSize"})
		return
	}

	// Nil store ⇒ empty 200, mirroring the fail-soft hub posture: the runs
	// table renders its empty state instead of an error banner.
	if h.store == nil {
		response.WriteJSON(w, http.StatusOK, AgentRunsResponse{Items: []runs.Run{}, Total: 0, Page: page, PageSize: pageSize})
		return
	}

	all, err := h.store.List(r.Context(), runs.Filter{
		AgentType: q.Get("agentType"),
		Status:    q.Get("status"),
	})
	if err != nil {
		response.InternalError(w, "Failed to list agent runs")
		return
	}

	// Visibility filter (Story 53.1): nil authz or an empty accessible set
	// fails closed — no run is visible.
	userNamespaces := []string{}
	if h.authz != nil {
		userNamespaces, err = h.authz.GetAccessibleNamespaces(r.Context(), userCtx)
		if err != nil {
			response.InternalError(w, "Failed to get user namespaces")
			return
		}
	}

	visible := make([]runs.Run, 0, len(all))
	for _, run := range all {
		// A run is visible only when its agent namespace matches the caller's
		// Casbin-accessible set — uniform fail-closed filter.
		if rbac.MatchNamespaceInList(run.AgentNamespace, userNamespaces) {
			visible = append(visible, run)
		}
	}

	total := len(visible)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	response.WriteJSON(w, http.StatusOK, AgentRunsResponse{
		Items:    visible[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
