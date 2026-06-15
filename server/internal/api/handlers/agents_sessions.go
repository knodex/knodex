// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/rbac"
)

const (
	defaultSessionsPageSize = 20
	maxSessionsPageSize     = 100
)

// SessionSummary is one row of GET /api/v1/agents/sessions — the heavy Runs[]
// is omitted (the list only needs enough to identify each conversation; the
// full transcript is fetched per-session by the detail endpoint).
type SessionSummary struct {
	ID             string    `json:"id"`
	AgentType      string    `json:"agentType"`
	AgentNamespace string    `json:"agentNamespace"`
	FirstPrompt    string    `json:"firstPrompt"`
	StartedAt      time.Time `json:"startedAt"`
	LastActivityAt time.Time `json:"lastActivityAt"`
	RunCount       int       `json:"runCount"`
	Status         string    `json:"status"`
}

// AgentSessionsResponse is the paginated envelope for the sessions list — the
// same shape as AgentRunsResponse.
type AgentSessionsResponse struct {
	Items    []SessionSummary `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}

// AgentSession is the replay-source payload for GET /agents/sessions/{id}: the
// conversation's full run records ordered oldest→newest. The web fetches each
// run's full result via the existing /runs/{id}/result endpoint.
type AgentSession struct {
	ID             string     `json:"id"`
	AgentType      string     `json:"agentType"`
	AgentNamespace string     `json:"agentNamespace"`
	Runs           []runs.Run `json:"runs"`
}

// AgentsSessionsHandler serves the chat-session list/replay endpoints (Story
// 50.6). Sessions are a VIEW over the run store: List → namespace-visibility
// filter (the SAME single Casbin enforcement layer as the runs list) → group.
// No new store, no new Casbin resource.
type AgentsSessionsHandler struct {
	store runs.Store
	authz AccessibleNamespacesProvider
}

// NewAgentsSessionsHandler creates an AgentsSessionsHandler. A nil store yields
// an empty list / 404 detail (fail-soft, mirroring the runs list); a nil authz
// fails closed (no session visible).
func NewAgentsSessionsHandler(store runs.Store, authz AccessibleNamespacesProvider) *AgentsSessionsHandler {
	return &AgentsSessionsHandler{store: store, authz: authz}
}

// visibleRuns lists runs and applies the exact namespace-visibility filter
// used by the runs list (agents_runs.go): a run is visible only when its agent
// namespace matches the caller's Casbin-accessible set (Story 53.1 removed the
// empty-namespace global carve-out); nil authz / empty set fails closed. A
// conversation is with one agent, so filtering runs BEFORE grouping is
// sufficient — every run in a session shares one visibility verdict.
func (h *AgentsSessionsHandler) visibleRuns(ctx context.Context, userCtx *middleware.UserContext, agentType string) ([]runs.Run, error) {
	all, err := h.store.List(ctx, runs.Filter{AgentType: agentType})
	if err != nil {
		return nil, err
	}

	userNamespaces := []string{}
	if h.authz != nil {
		userNamespaces, err = h.authz.GetAccessibleNamespaces(ctx, userCtx)
		if err != nil {
			return nil, err
		}
	}

	visible := make([]runs.Run, 0, len(all))
	for _, run := range all {
		if rbac.MatchNamespaceInList(run.AgentNamespace, userNamespaces) {
			visible = append(visible, run)
		}
	}
	return visible, nil
}

// ListSessions handles GET /api/v1/agents/sessions?agentType=&page=&pageSize=.
func (h *AgentsSessionsHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
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
	pageSize, ok := parsePositiveInt(q.Get("pageSize"), defaultSessionsPageSize)
	if !ok || pageSize > maxSessionsPageSize {
		response.BadRequest(w, "pageSize must be between 1 and 100", map[string]string{"field": "pageSize"})
		return
	}

	// Nil store ⇒ empty 200, mirroring the runs hub's fail-soft posture.
	if h.store == nil {
		response.WriteJSON(w, http.StatusOK, AgentSessionsResponse{Items: []SessionSummary{}, Total: 0, Page: page, PageSize: pageSize})
		return
	}

	visible, err := h.visibleRuns(r.Context(), userCtx, q.Get("agentType"))
	if err != nil {
		response.InternalError(w, "Failed to list agent sessions")
		return
	}

	sessions := runs.GroupSessions(visible)

	total := len(sessions)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	items := make([]SessionSummary, 0, end-start)
	for _, s := range sessions[start:end] {
		items = append(items, toSessionSummary(s))
	}
	response.WriteJSON(w, http.StatusOK, AgentSessionsResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetSession handles GET /api/v1/agents/sessions/{id}. Visibility is enforced
// (the same filter as the list) BEFORE the session lookup, so an unknown OR
// not-visible id is the SAME 404 — existence non-leak.
func (h *AgentsSessionsHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	id := r.PathValue("id")

	// Nil store ⇒ 404, same shape as unknown id.
	if h.store == nil {
		response.NotFound(w, "agent session", id)
		return
	}

	visible, err := h.visibleRuns(r.Context(), userCtx, "")
	if err != nil {
		response.InternalError(w, "Failed to get agent session")
		return
	}

	for _, s := range runs.GroupSessions(visible) {
		if s.ID == id {
			response.WriteJSON(w, http.StatusOK, AgentSession{
				ID:             s.ID,
				AgentType:      s.AgentType,
				AgentNamespace: s.AgentNamespace,
				Runs:           s.Runs,
			})
			return
		}
	}
	// Absent OR denied — identical 404 non-leak.
	response.NotFound(w, "agent session", id)
}

// toSessionSummary projects a Session onto its list DTO (RFC3339 timestamps,
// no Runs[]).
func toSessionSummary(s runs.Session) SessionSummary {
	return SessionSummary{
		ID:             s.ID,
		AgentType:      s.AgentType,
		AgentNamespace: s.AgentNamespace,
		FirstPrompt:    s.FirstPrompt,
		StartedAt:      s.StartedAt,
		LastActivityAt: s.LastActivityAt,
		RunCount:       s.RunCount,
		Status:         s.Status,
	}
}
