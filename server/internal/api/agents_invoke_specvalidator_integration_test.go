// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/kagent/runs"
)

// Story 53.5 — the spec-validation seam (Story 50.3), homeless after 53.1
// deleted the built-in invoke handler, is re-homed onto the surviving BYOA
// invoke path. This integration test is the ported
// TestIntegration_AgentsBuiltinInvoke_SpecValidatorWired: it proves the seam
// end-to-end through the REAL middleware chain on the namespaced route
// (202 → poll result → policyValidation present). Unlike the global built-in
// route, BYOA requires real namespace access — granted here via a wildcard
// PolicyEnforcer — plus a seeded Agent CR at that namespace/name.

// byoaSpecAgentCR is a plain (non-hub) BYOA Agent CR the namespaced invoke
// existence check resolves against.
func byoaSpecAgentCR(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kagent.dev/v1alpha2",
		"kind":       "Agent",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{"description": "byoa helper"},
	}}
}

// byoaStubInvoker returns a canned successful A2A result.
type byoaStubInvoker struct{}

func (byoaStubInvoker) Invoke(_ context.Context, _, _, _, _, _ string) (*kagent.A2AResult, error) {
	return &kagent.A2AResult{ContextID: "ctx-byoa", Text: "integration spec"}, nil
}

// byoaStubValidator answers a canned "passed" validation — proving the
// RouterConfig.AgentSpecValidator seam reaches the BYOA completion path.
type byoaStubValidator struct{}

func (byoaStubValidator) ValidateSpec(_ context.Context, _ string) *runs.PolicyValidation {
	return &runs.PolicyValidation{Status: runs.PolicyStatusPassed}
}

// TestIntegration_AgentsInvoke_SpecValidatorWired proves the Story 50.3 seam
// end-to-end on the 53.5-re-homed BYOA route: a validator wired via
// RouterConfig.AgentSpecValidator lands its outcome on the fetchable result.
func TestIntegration_AgentsInvoke_SpecValidatorWired(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	const adminUserID = "user-admin"
	server, authSvc := setupAuthTestServerWithConfig(t, func(cfg *RouterConfig) {
		cfg.AgentRunStore = runs.NewRedisStore(redisClient)
		cfg.AgentRunResultStore = runs.NewRedisResultStore(redisClient)
		cfg.AgentInvoker = byoaStubInvoker{}
		cfg.DynamicClient = seededAgentsDynamicClient(byoaSpecAgentCR("helper", "alpha-apps"))
		// Wildcard policy ⇒ GetAccessibleNamespaces returns ["*"], so the BYOA
		// namespace gate admits the call (the route's prerequisite the global
		// built-in route never had).
		cfg.PolicyEnforcer = &selectivePolicyEnforcer{adminUserID: adminUserID}
		cfg.AgentSpecValidator = byoaStubValidator{}
	})
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		adminUserID, "admin@test.local", "Server Admin", nil)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost,
		server.URL+"/api/v1/agents/alpha-apps/helper/invoke",
		strings.NewReader(`{"message":"I need a web app with Redis"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var run runs.Run
	require.NoError(t, json.Unmarshal(body, &run))
	require.NotEmpty(t, run.ID)

	// The run completes in the background — poll the result endpoint until the
	// 404 (in-flight) flips to 200.
	var result runs.Result
	require.Eventually(t, func() bool {
		resReq, reqErr := http.NewRequest(http.MethodGet,
			server.URL+"/api/v1/agents/runs/"+run.ID+"/result", nil)
		if reqErr != nil {
			return false
		}
		resReq.Header.Set("Authorization", "Bearer "+token)
		resResp, doErr := http.DefaultClient.Do(resReq)
		if doErr != nil {
			return false
		}
		defer resResp.Body.Close()
		if resResp.StatusCode != http.StatusOK {
			return false
		}
		resBody, readErr := io.ReadAll(resResp.Body)
		if readErr != nil {
			return false
		}
		return json.Unmarshal(resBody, &result) == nil
	}, 5*time.Second, 50*time.Millisecond, "result must become fetchable after the 202")

	require.NotNil(t, result.PolicyValidation,
		"the RouterConfig-wired validator's outcome must land on the result")
	assert.Equal(t, runs.PolicyStatusPassed, result.PolicyValidation.Status)
	assert.Equal(t, "integration spec", result.Response)
	assert.Equal(t, "ctx-byoa", result.KagentSessionID)
}
