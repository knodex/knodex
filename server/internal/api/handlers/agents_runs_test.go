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

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent/runs"
)

// seededRunsStore builds a miniredis-backed store pre-populated with runs
// across namespaces. Runs are created oldest-first so List returns them
// newest-first.
func seededRunsStore(t *testing.T, seed []runs.Run) *runs.RedisStore {
	t.Helper()
	mr := miniredis.RunT(t)
	store := runs.NewRedisStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	for i := range seed {
		// Copy each fixture: Create sanitizes its argument in place, and the
		// seed slice is shared across t.Parallel() subtests (data race under -race).
		run := seed[i]
		require.NoError(t, store.Create(context.Background(), &run))
	}
	return store
}

func runFixture(id, agentType, namespace, status string, ts time.Time) runs.Run {
	return runs.Run{
		ID:             id,
		Actor:          "dev@example.com",
		AgentType:      agentType,
		AgentNamespace: namespace,
		Timestamp:      ts,
		Status:         status,
		TriggerType:    runs.TriggerOnDemand,
	}
}

func authedRunsRequest(t *testing.T, query string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/runs"+query, nil)
	userCtx := &middleware.UserContext{UserID: "user-123", Email: "dev@example.com"}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func decodeRunsResponse(t *testing.T, w *httptest.ResponseRecorder) AgentRunsResponse {
	t.Helper()
	var resp AgentRunsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func itemIDs(resp AgentRunsResponse) []string {
	out := make([]string, len(resp.Items))
	for i, r := range resp.Items {
		out[i] = r.ID
	}
	return out
}

func TestAgentsRunsHandler_NamespaceVisibilityMatrix(t *testing.T) {
	t.Parallel()

	base := time.Unix(1_700_000_000, 0).UTC()
	// run-empty carries an empty AgentNamespace (the legacy built-in
	// convention). Story 53.1 removed the global-visibility carve-out: it is now
	// visible ONLY to a caller whose patterns match "" — i.e. the "*" wildcard.
	seed := []runs.Run{
		runFixture("run-empty", "rgd-builder", "", runs.StatusCompleted, base),
		runFixture("run-alpha", "helper", "alpha-apps", runs.StatusCompleted, base.Add(time.Second)),
		runFixture("run-beta", "helper", "beta-apps", runs.StatusCompleted, base.Add(2*time.Second)),
	}

	cases := []struct {
		name       string
		namespaces []string
		wantIDs    []string // newest-first
	}{
		{
			name:       "single namespace sees only its runs (empty-ns filtered out)",
			namespaces: []string{"alpha-apps"},
			wantIDs:    []string{"run-alpha"},
		},
		{
			name:       "global admin wildcard sees all (incl. empty namespace)",
			namespaces: []string{"*"},
			wantIDs:    []string{"run-beta", "run-alpha", "run-empty"},
		},
		{
			name:       "wildcard pattern matches prefix only (empty-ns filtered out)",
			namespaces: []string{"alpha-*"},
			wantIDs:    []string{"run-alpha"},
		},
		{
			name:       "no access sees nothing",
			namespaces: []string{},
			wantIDs:    []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := seededRunsStore(t, seed)
			handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: tc.namespaces})

			w := httptest.NewRecorder()
			handler.ListRuns(w, authedRunsRequest(t, ""))

			assert.Equal(t, http.StatusOK, w.Code)
			resp := decodeRunsResponse(t, w)
			assert.Equal(t, tc.wantIDs, itemIDs(resp))
			assert.Equal(t, len(tc.wantIDs), resp.Total)
		})
	}
}

func TestAgentsRunsHandler_NilAuthz_Empty(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, []runs.Run{
		runFixture("run-empty", "rgd-builder", "", runs.StatusCompleted, base),
		runFixture("run-alpha", "helper", "alpha-apps", runs.StatusCompleted, base.Add(time.Second)),
	})
	handler := NewAgentsRunsHandler(store, nil)

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeRunsResponse(t, w)
	assert.Empty(t, resp.Items, "nil authz must fail closed — no runs visible, not even empty-namespace runs")
}

func TestAgentsRunsHandler_PaginationBoundaries(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	seed := make([]runs.Run, 25)
	for i := 0; i < 25; i++ {
		seed[i] = runFixture(fmt.Sprintf("run-%02d", i), "helper", "alpha-apps", runs.StatusCompleted, base.Add(time.Duration(i)*time.Second))
	}
	store := seededRunsStore(t, seed)
	handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	// Page 1 (default pageSize 20): newest-first run-24..run-05.
	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, ""))
	resp := decodeRunsResponse(t, w)
	require.Len(t, resp.Items, 20)
	assert.Equal(t, "run-24", resp.Items[0].ID)
	assert.Equal(t, "run-05", resp.Items[19].ID)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)

	// Page 2: the last partial page (5 items).
	w = httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, "?page=2"))
	resp = decodeRunsResponse(t, w)
	require.Len(t, resp.Items, 5)
	assert.Equal(t, "run-04", resp.Items[0].ID)
	assert.Equal(t, "run-00", resp.Items[4].ID)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 2, resp.Page)

	// Out-of-range page: empty items, correct total.
	w = httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, "?page=99"))
	resp = decodeRunsResponse(t, w)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 25, resp.Total)
	assert.Equal(t, 99, resp.Page)

	// Custom pageSize.
	w = httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, "?page=3&pageSize=10"))
	resp = decodeRunsResponse(t, w)
	require.Len(t, resp.Items, 5)
	assert.Equal(t, 10, resp.PageSize)
}

func TestAgentsRunsHandler_PaginationAfterVisibilityFilter(t *testing.T) {
	t.Parallel()
	// Interleave visible and invisible runs: page numbers must be stable over
	// the FILTERED sequence, not the raw store order.
	base := time.Unix(1_700_000_000, 0).UTC()
	seed := make([]runs.Run, 0, 12)
	for i := 0; i < 6; i++ {
		seed = append(seed,
			runFixture(fmt.Sprintf("vis-%d", i), "helper", "alpha-apps", runs.StatusCompleted, base.Add(time.Duration(2*i)*time.Second)),
			runFixture(fmt.Sprintf("hid-%d", i), "helper", "beta-apps", runs.StatusCompleted, base.Add(time.Duration(2*i+1)*time.Second)),
		)
	}
	store := seededRunsStore(t, seed)
	handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, "?page=2&pageSize=4"))
	resp := decodeRunsResponse(t, w)

	assert.Equal(t, 6, resp.Total, "total counts only visible runs")
	assert.Equal(t, []string{"vis-1", "vis-0"}, itemIDs(resp), "page 2 of the filtered set")
}

func TestAgentsRunsHandler_FilterPassthrough(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, []runs.Run{
		runFixture("run-1", "helper", "alpha-apps", runs.StatusCompleted, base),
		runFixture("run-2", "other", "alpha-apps", runs.StatusFailed, base.Add(time.Second)),
		runFixture("run-3", "helper", "alpha-apps", runs.StatusFailed, base.Add(2*time.Second)),
	})
	handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, "?agentType=helper&status=failed"))
	resp := decodeRunsResponse(t, w)
	assert.Equal(t, []string{"run-3"}, itemIDs(resp))
	assert.Equal(t, 1, resp.Total)
}

func TestAgentsRunsHandler_EmptyStore(t *testing.T) {
	t.Parallel()
	store := seededRunsStore(t, nil)
	handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeRunsResponse(t, w)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}

func TestAgentsRunsHandler_NilStore_Empty200(t *testing.T) {
	t.Parallel()
	handler := NewAgentsRunsHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeRunsResponse(t, w)
	assert.Empty(t, resp.Items)
	assert.Equal(t, 0, resp.Total)
}

func TestAgentsRunsHandler_InvalidPagination_400(t *testing.T) {
	t.Parallel()
	store := seededRunsStore(t, nil)
	handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	for _, query := range []string{"?page=0", "?page=-1", "?page=abc", "?pageSize=0", "?pageSize=101", "?pageSize=x"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ListRuns(w, authedRunsRequest(t, query))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestAgentsRunsHandler_PageSizeMaxBoundaryAccepted proves the cap is
// inclusive: pageSize=100 (the documented maximum, and a CompliancePagination
// page-size option) must be accepted, while 101 is rejected (covered above).
func TestAgentsRunsHandler_PageSizeMaxBoundaryAccepted(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, []runs.Run{
		runFixture("run-1", "helper", "alpha-apps", runs.StatusCompleted, base),
	})
	handler := NewAgentsRunsHandler(store, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, "?pageSize=100"))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeRunsResponse(t, w)
	assert.Equal(t, 100, resp.PageSize)
	assert.Equal(t, []string{"run-1"}, itemIDs(resp))
}

func TestAgentsRunsHandler_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	handler := NewAgentsRunsHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/runs", nil)
	w := httptest.NewRecorder()
	handler.ListRuns(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentsRunsHandler_AuthzError_500(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := seededRunsStore(t, []runs.Run{
		runFixture("run-1", "helper", "alpha-apps", runs.StatusCompleted, base),
	})
	handler := NewAgentsRunsHandler(store, &stubNamespaces{err: errors.New("casbin exploded")})

	w := httptest.NewRecorder()
	handler.ListRuns(w, authedRunsRequest(t, ""))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
