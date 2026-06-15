// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent/runs"
)

// sessFixture is a run fixture carrying a conversationId + inputSummary — the
// inputs GroupSessions folds on. (runFixture in agents_runs_test.go omits both.)
func sessFixture(id, convID, agentType, namespace, input, status string, ts time.Time) runs.Run {
	return runs.Run{
		ID:             id,
		ConversationID: convID,
		Actor:          "dev@example.com",
		AgentType:      agentType,
		AgentNamespace: namespace,
		InputSummary:   input,
		Status:         status,
		Timestamp:      ts,
		TriggerType:    runs.TriggerOnDemand,
	}
}

func authedSessionsRequest(t *testing.T, query string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/sessions"+query, nil)
	userCtx := &middleware.UserContext{UserID: "user-123", Email: "dev@example.com"}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func authedSessionRequest(t *testing.T, id string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/sessions/"+id, nil)
	req.SetPathValue("id", id)
	userCtx := &middleware.UserContext{UserID: "user-123", Email: "dev@example.com"}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func decodeSessionsResponse(t *testing.T, w *httptest.ResponseRecorder) AgentSessionsResponse {
	t.Helper()
	var resp AgentSessionsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func sessionItemIDs(resp AgentSessionsResponse) []string {
	out := make([]string, len(resp.Items))
	for i, s := range resp.Items {
		out[i] = s.ID
	}
	return out
}

// multiTurnSeed: a 2-turn alpha conversation, a 1-turn alpha BYOA
// conversation, and a 1-turn beta BYOA conversation. Created oldest-first.
// Story 53.1 removed the empty-namespace global-visibility carve-out, so every
// fixture here is namespaced and governed by the single Casbin filter.
func multiTurnSeed(base time.Time) []runs.Run {
	return []runs.Run{
		sessFixture("bi-1", "conv-bi", "rgd-builder", "alpha-apps", "build a web app", runs.StatusCompleted, base),
		sessFixture("bi-2", "conv-bi", "rgd-builder", "alpha-apps", "add redis", runs.StatusCompleted, base.Add(time.Minute)),
		sessFixture("a-1", "conv-alpha", "helper", "alpha-apps", "alpha question", runs.StatusCompleted, base.Add(2*time.Minute)),
		sessFixture("b-1", "conv-beta", "helper", "beta-apps", "beta question", runs.StatusCompleted, base.Add(3*time.Minute)),
	}
}

func TestAgentsSessionsHandler_ListGroupsAndVisibility(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, ""))

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeSessionsResponse(t, w)
	// beta-apps is invisible; both alpha conversations (last activity +2m and
	// +1m) are visible.
	assert.Equal(t, []string{"conv-alpha", "conv-bi"}, sessionItemIDs(resp), "most-recent-first, beta filtered out")
	assert.Equal(t, 2, resp.Total)

	byID := map[string]SessionSummary{}
	for _, s := range resp.Items {
		byID[s.ID] = s
	}
	assert.Equal(t, 2, byID["conv-bi"].RunCount, "the 2-turn conversation folds both turns")
	assert.Equal(t, "build a web app", byID["conv-bi"].FirstPrompt, "first prompt = oldest turn")
	assert.Equal(t, "alpha-apps", byID["conv-bi"].AgentNamespace)
	assert.Equal(t, "alpha-apps", byID["conv-alpha"].AgentNamespace)
}

func TestAgentsSessionsHandler_NilAuthz_Empty(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, nil)

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, ""))

	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeSessionsResponse(t, w)
	assert.Empty(t, resp.Items, "nil authz must fail closed — no sessions visible")
}

// TestAgentsSessionsHandler_EmptyNamespaceFilteredOut pins Story 53.1: an
// empty-AgentNamespace (legacy built-in) session is NOT globally visible —
// only a "*" caller (whose patterns match "") sees it; a namespace-scoped
// caller does not.
func TestAgentsSessionsHandler_EmptyNamespaceFilteredOut(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	seed := []runs.Run{
		sessFixture("e-1", "conv-empty", "rgd-builder", "", "legacy turn", runs.StatusCompleted, base),
	}

	// Namespace-scoped caller: the empty-namespace session is filtered out.
	scoped := NewAgentsSessionsHandler(seededRunsStore(t, seed), &stubNamespaces{namespaces: []string{"alpha-apps"}})
	w := httptest.NewRecorder()
	scoped.ListSessions(w, authedSessionsRequest(t, ""))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, decodeSessionsResponse(t, w).Items, "empty-namespace session must not leak to a scoped caller")

	// Wildcard caller: "*" matches the empty namespace, so it is visible.
	wild := NewAgentsSessionsHandler(seededRunsStore(t, seed), &stubNamespaces{namespaces: []string{"*"}})
	w = httptest.NewRecorder()
	wild.ListSessions(w, authedSessionsRequest(t, ""))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"conv-empty"}, sessionItemIDs(decodeSessionsResponse(t, w)))
}

func TestAgentsSessionsHandler_AgentTypeFilter(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, "?agentType=helper"))

	resp := decodeSessionsResponse(t, w)
	assert.Equal(t, []string{"conv-beta", "conv-alpha"}, sessionItemIDs(resp), "only helper sessions")
}

func TestAgentsSessionsHandler_LegacyRunsAreSingletonSessions(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	// Two built-in runs with NO conversationId → two singleton sessions keyed
	// by run id (never dropped).
	store := seededRunsStore(t, []runs.Run{
		sessFixture("legacy-1", "", "rgd-builder", "", "old one", runs.StatusCompleted, base),
		sessFixture("legacy-2", "", "rgd-builder", "", "old two", runs.StatusCompleted, base.Add(time.Minute)),
	})
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, ""))

	resp := decodeSessionsResponse(t, w)
	assert.Equal(t, []string{"legacy-2", "legacy-1"}, sessionItemIDs(resp))
	assert.Equal(t, 2, resp.Total)
}

func TestAgentsSessionsHandler_Pagination(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	seed := make([]runs.Run, 25)
	for i := 0; i < 25; i++ {
		// Each run its own conversation → 25 singleton sessions.
		seed[i] = sessFixture(fmt.Sprintf("s-%02d", i), fmt.Sprintf("conv-%02d", i), "rgd-builder", "", "p", runs.StatusCompleted, base.Add(time.Duration(i)*time.Second))
	}
	store := seededRunsStore(t, seed)
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, "?page=2&pageSize=10"))
	resp := decodeSessionsResponse(t, w)
	require.Len(t, resp.Items, 10)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, "conv-14", resp.Items[0].ID, "page 2 of most-recent-first singletons")
}

func TestAgentsSessionsHandler_NilStore_Empty200(t *testing.T) {
	t.Parallel()
	handler := NewAgentsSessionsHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeSessionsResponse(t, w)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}

func TestAgentsSessionsHandler_InvalidPagination_400(t *testing.T) {
	t.Parallel()
	store := seededRunsStore(t, nil)
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	for _, query := range []string{"?page=0", "?page=abc", "?pageSize=0", "?pageSize=101"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ListSessions(w, authedSessionsRequest(t, query))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestAgentsSessionsHandler_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	handler := NewAgentsSessionsHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/sessions", nil)
	w := httptest.NewRecorder()
	handler.ListSessions(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentsSessionsHandler_AuthzError_500(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{err: errors.New("casbin exploded")})

	w := httptest.NewRecorder()
	handler.ListSessions(w, authedSessionsRequest(t, ""))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetSession ---

func decodeSession(t *testing.T, w *httptest.ResponseRecorder) AgentSession {
	t.Helper()
	var s AgentSession
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &s))
	return s
}

// TestAgentsSessionsHandler_GetSession_MultiTurn returns a conversation's full
// run history oldest→newest when the caller has namespace access.
func TestAgentsSessionsHandler_GetSession_MultiTurn(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "conv-bi"))

	require.Equal(t, http.StatusOK, w.Code)
	s := decodeSession(t, w)
	assert.Equal(t, "conv-bi", s.ID)
	assert.Equal(t, "rgd-builder", s.AgentType)
	require.Len(t, s.Runs, 2)
	assert.Equal(t, []string{"bi-1", "bi-2"}, []string{s.Runs[0].ID, s.Runs[1].ID}, "runs oldest→newest")
}

// TestAgentsSessionsHandler_GetSession_EmptyNamespaceDenied pins Story 53.1: a
// legacy empty-namespace session is a non-leaking 404 for a scoped caller.
func TestAgentsSessionsHandler_GetSession_EmptyNamespaceDenied(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, []runs.Run{
		sessFixture("e-1", "conv-empty", "rgd-builder", "", "legacy turn", runs.StatusCompleted, base),
	})
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "conv-empty"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentsSessionsHandler_GetSession_BYOAVisibleWithNamespace(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "conv-alpha"))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "conv-alpha", decodeSession(t, w).ID)
}

func TestAgentsSessionsHandler_GetSession_DeniedBYOA_404NonLeak(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	// alpha access only — the beta session must be a 404, identical to unknown.
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "conv-beta"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentsSessionsHandler_GetSession_Unknown_404(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "does-not-exist"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentsSessionsHandler_GetSession_NilStore_404(t *testing.T) {
	t.Parallel()
	handler := NewAgentsSessionsHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "conv-bi"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentsSessionsHandler_GetSession_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	handler := NewAgentsSessionsHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/sessions/conv-bi", nil)
	req.SetPathValue("id", "conv-bi")
	w := httptest.NewRecorder()
	handler.GetSession(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentsSessionsHandler_GetSession_AuthzError_500(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, multiTurnSeed(base))
	handler := NewAgentsSessionsHandler(store, &stubNamespaces{err: errors.New("casbin exploded")})

	w := httptest.NewRecorder()
	handler.GetSession(w, authedSessionRequest(t, "conv-bi"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
