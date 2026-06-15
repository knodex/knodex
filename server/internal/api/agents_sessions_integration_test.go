// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/kagent/runs"
)

// Story 50.6 / Story 53.1 — the chat-session list/replay endpoints sit behind
// the standard auth middleware chain on protectedMux (auth-only, no Casbin
// can-i resource; the same single enforcement layer as agents/runs applied
// INSIDE the handler). Story 53.1 removed the empty-namespace global-visibility
// carve-out: a roleless user now sees NOTHING. These tests exercise the real
// router: a 401 from the middleware, a roleless user seeing no sessions, and
// the non-leaking 404 for any session that user cannot view.

// newSeededSessionsConfig returns a RouterConfig override wiring a run store
// pre-seeded with one empty-namespace conversation and one BYOA conversation,
// plus a handle on the store for assertions.
func newSeededSessionsConfig(t *testing.T) func(*RouterConfig) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := runs.NewRedisStore(client)

	base := time.Unix(1_700_000_000, 0).UTC()
	seed := []runs.Run{
		{ID: "bi-1", ConversationID: "conv-bi", AgentType: "rgd-builder", AgentNamespace: "", InputSummary: "build me a web app", Status: runs.StatusCompleted, Timestamp: base, TriggerType: runs.TriggerOnDemand},
		{ID: "byoa-1", ConversationID: "conv-byoa", AgentType: "helper", AgentNamespace: "alpha-apps", InputSummary: "byoa", Status: runs.StatusCompleted, Timestamp: base.Add(time.Minute), TriggerType: runs.TriggerOnDemand},
	}
	for i := range seed {
		run := seed[i]
		require.NoError(t, store.Create(context.Background(), &run))
	}
	return func(cfg *RouterConfig) { cfg.AgentRunStore = store }
}

func authedGet(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestIntegration_AgentsSessions_Unauthenticated_401(t *testing.T) {
	server, _ := setupAuthTestServerWithConfig(t, newSeededSessionsConfig(t))
	defer server.Close()

	for _, path := range []string{"/api/v1/agents/sessions", "/api/v1/agents/sessions/conv-bi"} {
		resp, err := http.Get(server.URL + path)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
	}
}

func TestIntegration_AgentsSessions_RolelessUser_SeesNothing(t *testing.T) {
	server, authSvc := setupAuthTestServerWithConfig(t, newSeededSessionsConfig(t))
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	resp := authedGet(t, server.URL+"/api/v1/agents/sessions", token)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var payload struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Empty(t, payload.Items, "roleless user sees no sessions (Story 53.1 — empty-namespace not globally visible)")
	assert.Equal(t, 0, payload.Total)
}

func TestIntegration_AgentsSessions_GetEmptyNamespace_RolelessUser_404(t *testing.T) {
	server, authSvc := setupAuthTestServerWithConfig(t, newSeededSessionsConfig(t))
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	resp := authedGet(t, server.URL+"/api/v1/agents/sessions/conv-bi", token)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"an empty-namespace session is no longer globally readable — 404 for a roleless caller")
}

func TestIntegration_AgentsSessions_GetBYOA_RolelessUser_404_FailClosed(t *testing.T) {
	server, authSvc := setupAuthTestServerWithConfig(t, newSeededSessionsConfig(t))
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	resp := authedGet(t, server.URL+"/api/v1/agents/sessions/conv-byoa", token)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"a BYOA session the user cannot view must be the non-leaking 404, never 403")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "NOT_FOUND", errResp["code"])
}
