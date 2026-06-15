// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
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

func newResultHarness(t *testing.T, namespaces []string) (*AgentsRunResultHandler, *runs.RedisResultStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	resultStore := runs.NewRedisResultStore(client)
	return NewAgentsRunResultHandler(resultStore, &stubNamespaces{namespaces: namespaces}), resultStore
}

func resultRequestFor(t *testing.T, id string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/runs/"+id+"/result", nil)
	req.SetPathValue("id", id)
	userCtx := &middleware.UserContext{UserID: "user-123", Email: "dev@example.com"}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func TestAgentsRunResultHandler_MissingUserContext_401(t *testing.T) {
	t.Parallel()

	handler, _ := newResultHarness(t, []string{"*"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/runs/run-1/result", nil)
	req.SetPathValue("id", "run-1")
	w := httptest.NewRecorder()
	handler.GetResult(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentsRunResultHandler_NoResultYet_404(t *testing.T) {
	t.Parallel()

	handler, _ := newResultHarness(t, []string{"*"})

	w := httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "in-flight-or-unknown"))

	assert.Equal(t, http.StatusNotFound, w.Code,
		"in-flight and unknown run ids are the same 404 by design")

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "NOT_FOUND", errResp["code"])
}

func TestAgentsRunResultHandler_CompletedResult_200(t *testing.T) {
	t.Parallel()

	// Wildcard caller: "*" matches the (legacy empty) namespace too.
	handler, store := newResultHarness(t, []string{"*"})
	completedAt := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	require.NoError(t, store.Put(context.Background(), &runs.Result{
		RunID:          "run-ok",
		AgentNamespace: "",
		Status:         runs.StatusCompleted,
		Response:       "Here is the spec:\n```yaml\nkind: ResourceGraphDefinition\n```",
		CompletedAt:    completedAt,
	}))

	w := httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "run-ok"))

	require.Equal(t, http.StatusOK, w.Code)
	var got runs.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "run-ok", got.RunID)
	assert.Equal(t, runs.StatusCompleted, got.Status)
	assert.Contains(t, got.Response, "ResourceGraphDefinition")
	assert.Empty(t, got.Error)
	assert.True(t, completedAt.Equal(got.CompletedAt))
}

func TestAgentsRunResultHandler_FailedResult_200(t *testing.T) {
	t.Parallel()

	handler, store := newResultHarness(t, []string{"*"})
	require.NoError(t, store.Put(context.Background(), &runs.Result{
		RunID:       "run-bad",
		Status:      runs.StatusFailed,
		Error:       "a2a call failed: connection refused",
		CompletedAt: time.Now().UTC(),
	}))

	w := httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "run-bad"))

	require.Equal(t, http.StatusOK, w.Code)
	var got runs.Result
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, runs.StatusFailed, got.Status)
	assert.Contains(t, got.Error, "connection refused")
}

// TestAgentsRunResultHandler_NamespaceVisibilityMatrix future-proofs the BYOA
// case: a namespaced result requires namespace access; denial is the
// non-leaking 404.
func TestAgentsRunResultHandler_NamespaceVisibilityMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		namespaces []string
		wantStatus int
	}{
		{"accessible namespace", []string{"alpha-apps"}, http.StatusOK},
		{"wildcard admin", []string{"*"}, http.StatusOK},
		{"inaccessible namespace", []string{"beta-apps"}, http.StatusNotFound},
		{"empty namespace set", []string{}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handler, store := newResultHarness(t, tc.namespaces)
			require.NoError(t, store.Put(context.Background(), &runs.Result{
				RunID:          "run-ns",
				AgentNamespace: "alpha-apps",
				Status:         runs.StatusCompleted,
				Response:       "agent says hi",
				CompletedAt:    time.Now().UTC(),
			}))

			w := httptest.NewRecorder()
			handler.GetResult(w, resultRequestFor(t, "run-ns"))
			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

func TestAgentsRunResultHandler_NilAuthz_FailsClosed(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := runs.NewRedisResultStore(client)
	handler := NewAgentsRunResultHandler(store, nil)

	require.NoError(t, store.Put(context.Background(), &runs.Result{
		RunID:          "run-ns",
		AgentNamespace: "alpha-apps",
		Status:         runs.StatusCompleted,
		CompletedAt:    time.Now().UTC(),
	}))
	require.NoError(t, store.Put(context.Background(), &runs.Result{
		RunID:       "run-empty",
		Status:      runs.StatusCompleted,
		Response:    "spec",
		CompletedAt: time.Now().UTC(),
	}))

	// Namespaced result: fail closed.
	w := httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "run-ns"))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Empty-namespace result: also fail closed now (Story 53.1 removed the
	// global-readable carve-out).
	w = httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "run-empty"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentsRunResultHandler_NilStore_404_FailSoft(t *testing.T) {
	t.Parallel()

	handler := NewAgentsRunResultHandler(nil, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "run-1"))

	assert.Equal(t, http.StatusNotFound, w.Code, "nil result store fails soft as 404, never 5xx")
}

func TestAgentsRunResultHandler_StoreError_500(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := runs.NewRedisResultStore(client)
	handler := NewAgentsRunResultHandler(store, &stubNamespaces{namespaces: []string{"*"}})
	mr.Close() // kill Redis so Get errors

	w := httptest.NewRecorder()
	handler.GetResult(w, resultRequestFor(t, "run-1"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
