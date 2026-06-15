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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/knodex/knodex/server/internal/rbac"
)

// Story 50.2 AC #5 — POST /api/v1/rgds sits behind the standard middleware
// chain on protectedMux with CasbinAuthz inference (object rgds/*, action
// create). Unlike the GET catalog routes (hybrid bypass), the POST goes
// through the enforcer: an authenticated user without a matching policy gets
// 403; a serveradmin-equivalent (wildcard) user gets 201.

const integrationRGDYAML = "apiVersion: kro.run/v1alpha1\n" +
	"kind: ResourceGraphDefinition\n" +
	"metadata:\n" +
	"  name: integration-stack\n" +
	"spec:\n" +
	"  schema:\n" +
	"    apiVersion: v1alpha1\n" +
	"    kind: IntegrationStack\n" +
	"  resources: []\n"

// selectivePolicyEnforcer implements rbac.PolicyEnforcer, granting only
// adminUserID and denying everyone else — so the REAL CasbinAuthz middleware
// runs its inference + enforcement path and the test exercises both verdicts
// deterministically.
type selectivePolicyEnforcer struct {
	adminUserID string
}

var _ rbac.PolicyEnforcer = (*selectivePolicyEnforcer)(nil)

// Authorizer
func (s *selectivePolicyEnforcer) CanAccess(_ context.Context, user, _, _ string) (bool, error) {
	return user == s.adminUserID, nil
}
func (s *selectivePolicyEnforcer) CanAccessWithGroups(_ context.Context, user string, _ []string, _, _ string) (bool, error) {
	return user == s.adminUserID, nil
}
func (s *selectivePolicyEnforcer) EnforceProjectAccess(_ context.Context, _, _, _ string) error {
	return nil
}
func (s *selectivePolicyEnforcer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}
func (s *selectivePolicyEnforcer) HasRole(_ context.Context, user, _ string) (bool, error) {
	return user == s.adminUserID, nil
}

// PolicyLoader
func (s *selectivePolicyEnforcer) LoadProjectPolicies(_ context.Context, _ *rbac.Project) error {
	return nil
}
func (s *selectivePolicyEnforcer) SyncPolicies(_ context.Context) error { return nil }
func (s *selectivePolicyEnforcer) RemoveProjectPolicies(_ context.Context, _ string) error {
	return nil
}

// RoleManager
func (s *selectivePolicyEnforcer) AssignUserRoles(_ context.Context, _ string, _ []string) error {
	return nil
}
func (s *selectivePolicyEnforcer) GetUserRoles(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (s *selectivePolicyEnforcer) RemoveUserRoles(_ context.Context, _ string) error   { return nil }
func (s *selectivePolicyEnforcer) RemoveUserRole(_ context.Context, _, _ string) error { return nil }
func (s *selectivePolicyEnforcer) RestorePersistedRoles(_ context.Context) error       { return nil }

// CacheController
func (s *selectivePolicyEnforcer) InvalidateCache()                       {}
func (s *selectivePolicyEnforcer) InvalidateCacheForUser(_ string) int    { return 0 }
func (s *selectivePolicyEnforcer) InvalidateCacheForProject(_ string) int { return 0 }
func (s *selectivePolicyEnforcer) CacheStats() rbac.CacheStats            { return rbac.CacheStats{} }

// MetricsProvider
func (s *selectivePolicyEnforcer) Metrics() rbac.PolicyMetrics { return rbac.PolicyMetrics{} }
func (s *selectivePolicyEnforcer) IncrementPolicyReloads()     {}
func (s *selectivePolicyEnforcer) IncrementBackgroundSyncs()   {}
func (s *selectivePolicyEnforcer) IncrementWatcherRestarts()   {}

func newRGDCreateIntegrationConfig(adminUserID string) func(*RouterConfig) {
	return func(cfg *RouterConfig) {
		cfg.DynamicClient = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
		cfg.PolicyEnforcer = &selectivePolicyEnforcer{adminUserID: adminUserID}
	}
}

func postRGD(t *testing.T, url, token string) *http.Response {
	t.Helper()
	body := map[string]string{"yaml": integrationRGDYAML}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, url+"/api/v1/rgds", strings.NewReader(string(raw)))
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestIntegration_RGDCreate_Unauthenticated_401(t *testing.T) {
	server, _ := setupAuthTestServerWithConfig(t, newRGDCreateIntegrationConfig("user-admin"))
	defer server.Close()

	resp := postRGD(t, server.URL, "")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"unauthenticated POST /api/v1/rgds must get 401 from the middleware chain")
}

func TestIntegration_RGDCreate_RolelessUser_403(t *testing.T) {
	server, authSvc := setupAuthTestServerWithConfig(t, newRGDCreateIntegrationConfig("user-admin"))
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-roleless", "roleless@test.local", "Roleless User", nil)
	require.NoError(t, err)

	resp := postRGD(t, server.URL, token)
	defer resp.Body.Close()

	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"an authenticated user without an rgds/* create policy must be denied by CasbinAuthz")

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &errResp))
	assert.Equal(t, "FORBIDDEN", errResp["code"])
}

func TestIntegration_RGDCreate_ServerAdmin_201(t *testing.T) {
	server, authSvc := setupAuthTestServerWithConfig(t, newRGDCreateIntegrationConfig("user-admin"))
	defer server.Close()

	token, _, err := authSvc.GenerateTokenWithGroups(
		"user-admin", "admin@test.local", "Server Admin", nil)
	require.NoError(t, err)

	resp := postRGD(t, server.URL, token)
	defer resp.Body.Close()

	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"a wildcard-policy (serveradmin) user must be able to create the RGD")

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &created))
	assert.Equal(t, "integration-stack", created["name"])
	assert.Equal(t, "ResourceGraphDefinition", created["kind"])
	assert.Nil(t, created["namespace"], "RGDs are cluster-scoped; namespace must be absent from the response")
}
