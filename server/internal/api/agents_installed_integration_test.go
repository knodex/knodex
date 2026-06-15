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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// agentsListKindsIntegration maps the kagent Agent GVR to its list kind so the
// fake dynamic client can serve LIST without panicking.
var agentsListKindsIntegration = map[schema.GroupVersionResource]string{
	{Group: "kagent.dev", Version: "v1alpha2", Resource: "agents"}: "AgentList",
}

// seededAgentsDynamicClient builds a fake dynamic client able to LIST kagent
// Agent CRs, seeded with the given objects.
func seededAgentsDynamicClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, agentsListKindsIntegration, objects...)
}

// Story 49.2 AC #4 / Story 53.1 — GET /api/v1/agents sits behind the standard
// auth middleware (401 from the chain, not handler simulation) and fails
// closed: a user resolving to zero accessible namespaces receives an empty
// {agents: []} list with 200. These tests exercise the real router,
// complementing the handler unit tests in handlers/agents_installed_test.go
// which stub the namespace provider.

// TestIntegration_AgentsInstalled_Unauthenticated_401 verifies an
// unauthenticated request is rejected by the auth middleware with the
// standard 401 envelope.
func TestIntegration_AgentsInstalled_Unauthenticated_401(t *testing.T) {
	server, _ := setupAuthTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/agents")
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

// TestIntegration_AgentsInstalled_RolelessUser_EmptyList200 verifies the
// fail-closed contract end-to-end: an authenticated user whose Casbin
// evaluation yields zero accessible namespaces (this harness wires no
// PolicyEnforcer/PermissionService) gets {"agents": []} with 200 — never an
// error, and never another user's agents. A dynamic client is wired (as in
// any real server) but seeded with no agents.
func TestIntegration_AgentsInstalled_RolelessUser_EmptyList200(t *testing.T) {
	server, authSvc := setupAuthTestServerWithConfig(t, func(cfg *RouterConfig) {
		cfg.DynamicClient = seededAgentsDynamicClient()
	})
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/agents", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"zero accessible namespaces must yield an empty 200, not an error")
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"agents": []}`, string(body))
}
