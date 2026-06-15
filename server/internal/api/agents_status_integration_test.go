// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Story 49.1 AC #4 — GET /api/v1/agents/status is auth-only: a 401 must come
// from the standard middleware chain (not handler-internal simulation), and
// ANY authenticated user — including one with no groups and no Casbin roles —
// must receive the presence payload. These tests exercise the real router
// (NewRouterWithConfig + auth middleware), complementing the handler unit
// tests in handlers/agents_status_test.go which stub the UserContext directly.

// TestIntegration_AgentsStatus_Unauthenticated_401 verifies an unauthenticated
// request is rejected by the auth middleware with the standard 401 envelope.
func TestIntegration_AgentsStatus_Unauthenticated_401(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/agents/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"unauthenticated request must get 401 from the middleware chain")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp["code"])
}

// TestIntegration_AgentsStatus_MalformedToken_401 verifies a garbage bearer
// token is rejected by the middleware (not the handler).
func TestIntegration_AgentsStatus_MalformedToken_401(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agents/status", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer not-a-real-jwt")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestIntegration_AgentsStatus_RolelessUser_200_NoCasbinGate verifies the
// endpoint is reachable by ANY authenticated user: a token with no groups and
// no Casbin roles must get 200 (auth-only — a Casbin layer would 403 here).
// With no KagentChecker wired in this harness the payload is the structured
// degraded state — also proving the no-checker path is data, never a 5xx.
func TestIntegration_AgentsStatus_RolelessUser_200_NoCasbinGate(t *testing.T) {
	server, authSvc := setupAuthTestServer(t)
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agents/status", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"authenticated role-less user must get 200 — endpoint is auth-only, no Casbin resource")
	assert.Less(t, resp.StatusCode, 500, "presence endpoint must never return 5xx")
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &payload))

	// Structured payload contract: status + nullable check fields + message.
	assert.Contains(t, []interface{}{"ready", "not_installed", "degraded"}, payload["status"])
	for _, key := range []string{"status", "crdPresent", "controllerHealthy", "message"} {
		_, present := payload[key]
		assert.True(t, present, "payload must contain %q", key)
	}
	// This harness wires no KagentChecker → graceful degraded payload.
	assert.Equal(t, "degraded", payload["status"])
	assert.NotEmpty(t, payload["message"])
}
