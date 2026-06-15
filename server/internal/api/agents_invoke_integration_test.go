// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Story 49.4 AC #4/#5 — the invoke and runs endpoints sit behind the standard
// auth middleware chain on protectedMux (no Casbin can-i resource): a 401 must
// come from the middleware, and an authenticated user with no groups and no
// Casbin roles must fail CLOSED — 404 for invoke (existence non-leak; the
// accessible-namespace set is empty). These tests exercise the real router
// (NewRouterWithConfig + auth middleware), complementing the handler unit
// tests in handlers/agents_invoke_test.go and handlers/agents_runs_test.go.

func TestIntegration_AgentsInvoke_Unauthenticated_401(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	resp, err := http.Post(server.URL+"/api/v1/agents/alpha-apps/helper/invoke",
		"application/json", strings.NewReader(`{"message":"hi"}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"unauthenticated invoke must get 401 from the middleware chain")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp["code"])
}

func TestIntegration_AgentsInvoke_RolelessUser_404_FailClosed(t *testing.T) {
	server, authSvc := setupAuthTestServer(t)
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		server.URL+"/api/v1/agents/alpha-apps/helper/invoke",
		strings.NewReader(`{"message":"hi"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"roleless user must fail closed with the non-leaking 404 — never 403, never 202")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "NOT_FOUND", errResp["code"])
}

func TestIntegration_AgentsRuns_Unauthenticated_401(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/agents/runs")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_AgentsRuns_RolelessUser_200_EmptyList(t *testing.T) {
	server, authSvc := setupAuthTestServer(t)
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agents/runs", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// No store wired in this harness ⇒ fail-soft empty envelope, never 5xx.
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload))
	assert.Equal(t, float64(0), payload["total"])
	assert.Empty(t, payload["items"])
}
