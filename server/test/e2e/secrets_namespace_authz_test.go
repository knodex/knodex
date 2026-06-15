// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==============================================================================
// Secrets Namespace-Keyed Authorization E2E Tests
//
// Verifies the secrets-namespace-keyed-authz spec at the live API layer:
//
//   - AC10: serveradmin can read secrets in any namespace.
//   - AC9:  a user without a destinations binding for a namespace is denied
//           (404 or 403) — the handler does not leak existence.
//   - AC4:  the old /api/v1/secrets/{name} route is unregistered (404).
//   - AC5:  Create does NOT stamp knodex.io/project on the K8s Secret.
//   - AC6:  X-Knodex-Project lands in audit.Event.Project verbatim.
//
// AC2 (shared-namespace cross-project read) and AC7 (absent lens → empty
// Event.Project) are pinned by the unit tests in secrets_handler_test.go
// and the rbac unit tests; this file targets the URL contract, audit
// integration, and route shape — the things that broke the old
// project-coupled implementation.
// ==============================================================================

const (
	// Pre-existing namespace in every QA cluster.
	secretsAuthzTestNamespace = "default"

	// Admin user — pre-configured in CASBIN_ADMIN_USERS in QA deployments.
	secretsAuthzAdminUser = "user-global-admin"
	secretsAuthzNoAccess  = "secrets-authz-no-access@e2e.local"
)

func secretsAuthzAdminToken() string {
	return GenerateTestJWT(JWTClaims{
		Subject:     secretsAuthzAdminUser,
		Email:       secretsAuthzAdminUser + "@e2e.local",
		CasbinRoles: []string{"role:serveradmin"},
	})
}

func secretsAuthzNoAccessToken() string {
	// No project bindings, no casbin roles → empty accessible namespace set.
	return GenerateTestJWT(JWTClaims{
		Subject:  secretsAuthzNoAccess,
		Email:    secretsAuthzNoAccess,
		Projects: []string{},
	})
}

// secretsAuthzUniqueName returns a stable-per-call unique secret name.
// time.Now().UnixNano() gives microsecond-grade uniqueness across the few
// parallel tests in this file.
func secretsAuthzUniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// makeRequestWithHeaders is a tiny variant of makeAuthenticatedRequest that
// lets callers attach extra request headers (X-Knodex-Project).
func makeRequestWithHeaders(t *testing.T, method, path, token string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()

	var bodyReader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(raw)
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, apiBaseURL+path, bodyReader)
	} else {
		req, err = http.NewRequest(method, apiBaseURL+path, nil)
	}
	require.NoError(t, err)

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	return resp
}

// TestSecretsAuthz_AC10_ServerAdminCanReadAnyNamespace pins that the
// built-in serveradmin policy (secrets/*, *, allow) still matches the
// URL-derived shape secrets/{ns}/{name}.
func TestSecretsAuthz_AC10_ServerAdminCanReadAnyNamespace(t *testing.T) {
	if err := waitForServer(); err != nil {
		t.Skipf("server not available: %v", err)
	}

	adminToken := secretsAuthzAdminToken()
	secretName := secretsAuthzUniqueName("e2e-authz-admin")

	// Admin creates a secret in the test namespace.
	createBody := map[string]interface{}{
		"name": secretName,
		"data": map[string]string{"k": "v"},
	}
	resp, err := makeAuthenticatedRequest("POST",
		fmt.Sprintf("/api/v1/namespaces/%s/secrets", secretsAuthzTestNamespace),
		adminToken, createBody)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equalf(t, http.StatusCreated, resp.StatusCode, "create expected 201")

	defer func() {
		// Best-effort cleanup.
		dr, _ := makeAuthenticatedRequest("DELETE",
			fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", secretsAuthzTestNamespace, secretName),
			adminToken, nil)
		if dr != nil {
			dr.Body.Close()
		}
	}()

	// Admin reads it back via the namespace-keyed route.
	getResp, err := makeAuthenticatedRequest("GET",
		fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", secretsAuthzTestNamespace, secretName),
		adminToken, nil)
	require.NoError(t, err)
	defer getResp.Body.Close()
	assert.Equal(t, http.StatusOK, getResp.StatusCode, "admin should read any-namespace secret (AC10)")
}

// TestSecretsAuthz_AC4_OldRouteUnregistered pins that the legacy
// /api/v1/secrets/{name}?project=... endpoint is gone — there is no
// 308 redirect, no deprecation alias, no double-handling.
func TestSecretsAuthz_AC4_OldRouteUnregistered(t *testing.T) {
	if err := waitForServer(); err != nil {
		t.Skipf("server not available: %v", err)
	}

	adminToken := secretsAuthzAdminToken()

	// The old shape — query-driven project + namespace.
	resp, err := makeAuthenticatedRequest("GET",
		"/api/v1/secrets/some-name?project=any&namespace=default", adminToken, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equalf(t, http.StatusNotFound, resp.StatusCode,
		"legacy /api/v1/secrets/{name} route must be unregistered (AC4)")
}

// TestSecretsAuthz_AC5_NoProjectLabelStamped pins that Create never stamps
// knodex.io/project on the K8s Secret. The label is reserved for legacy
// Instances usage; under the new model, namespace alone defines reach.
func TestSecretsAuthz_AC5_NoProjectLabelStamped(t *testing.T) {
	if err := waitForServer(); err != nil {
		t.Skipf("server not available: %v", err)
	}

	adminToken := secretsAuthzAdminToken()
	secretName := secretsAuthzUniqueName("e2e-authz-label")

	createResp, err := makeAuthenticatedRequest("POST",
		fmt.Sprintf("/api/v1/namespaces/%s/secrets", secretsAuthzTestNamespace),
		adminToken, map[string]interface{}{
			"name": secretName,
			"data": map[string]string{"k": "v"},
		})
	require.NoError(t, err)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	defer func() {
		dr, _ := makeAuthenticatedRequest("DELETE",
			fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", secretsAuthzTestNamespace, secretName),
			adminToken, nil)
		if dr != nil {
			dr.Body.Close()
		}
	}()

	var resp struct {
		Labels map[string]string `json:"labels"`
	}
	require.NoError(t, json.NewDecoder(createResp.Body).Decode(&resp))
	assert.Equal(t, "knodex", resp.Labels["knodex.io/managed-by"],
		"managed-by label is always stamped")
	_, hasProjectLabel := resp.Labels["knodex.io/project"]
	assert.Falsef(t, hasProjectLabel,
		"knodex.io/project must NOT be stamped under the namespace-keyed model (AC5); got labels=%v",
		resp.Labels)
}

// TestSecretsAuthz_AC6_AuditLensFromHeader pins that the X-Knodex-Project
// request header lands in the audit Event.Project field verbatim. This
// validation goes via the audit API rather than direct DB inspection,
// matching how authorization_audit_log_test.go reads audit events.
//
// Skipped when the audit API is not exposed in this build — the unit test
// in secrets_handler_test.go pins the in-handler behavior already.
func TestSecretsAuthz_AC6_AuditLensFromHeader(t *testing.T) {
	if err := waitForServer(); err != nil {
		t.Skipf("server not available: %v", err)
	}

	adminToken := secretsAuthzAdminToken()
	secretName := secretsAuthzUniqueName("e2e-authz-lens")
	const lensValue = "audit-lens-acme"

	createResp := makeRequestWithHeaders(t, "POST",
		fmt.Sprintf("/api/v1/namespaces/%s/secrets", secretsAuthzTestNamespace),
		adminToken,
		map[string]interface{}{
			"name": secretName,
			"data": map[string]string{"k": "v"},
		},
		map[string]string{"X-Knodex-Project": lensValue},
	)
	defer createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	defer func() {
		dr, _ := makeAuthenticatedRequest("DELETE",
			fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", secretsAuthzTestNamespace, secretName),
			adminToken, nil)
		if dr != nil {
			dr.Body.Close()
		}
	}()

	// Verify via the audit API.
	auditResp, err := makeAuthenticatedRequest("GET",
		fmt.Sprintf("/api/v1/audit?resource=secrets&name=%s&result=success", secretName),
		adminToken, nil)
	require.NoError(t, err)
	defer auditResp.Body.Close()
	if auditResp.StatusCode == http.StatusNotFound {
		t.Skip("audit API not available in this build — covered by unit tests")
	}
	require.Equal(t, http.StatusOK, auditResp.StatusCode)

	var auditPage struct {
		Events []struct {
			Name    string `json:"name"`
			Project string `json:"project"`
		} `json:"events"`
	}
	require.NoError(t, json.NewDecoder(auditResp.Body).Decode(&auditPage))
	require.NotEmptyf(t, auditPage.Events, "expected at least one audit event for %s", secretName)

	matched := false
	for _, ev := range auditPage.Events {
		if ev.Name == secretName {
			matched = true
			assert.Equal(t, lensValue, ev.Project,
				"X-Knodex-Project header must land in audit Event.Project (AC6)")
		}
	}
	assert.True(t, matched, "no audit event found for created secret")
}

// TestSecretsAuthz_AC9_CrossNamespaceDenied pins that a user without
// destinations covering the target namespace receives a denial response
// (404 from the handler's not-leak path, or 403 from the middleware) when
// reading a secret. Either is acceptable — both are non-200 outcomes that
// preserve namespace isolation.
func TestSecretsAuthz_AC9_CrossNamespaceDenied(t *testing.T) {
	if err := waitForServer(); err != nil {
		t.Skipf("server not available: %v", err)
	}

	adminToken := secretsAuthzAdminToken()
	noAccessToken := secretsAuthzNoAccessToken()
	secretName := secretsAuthzUniqueName("e2e-authz-deny")

	// Admin seeds a secret.
	createResp, err := makeAuthenticatedRequest("POST",
		fmt.Sprintf("/api/v1/namespaces/%s/secrets", secretsAuthzTestNamespace),
		adminToken, map[string]interface{}{
			"name": secretName,
			"data": map[string]string{"k": "v"},
		})
	require.NoError(t, err)
	createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Skipf("seed expected 201, got %d — cluster fixtures not ready", createResp.StatusCode)
	}

	defer func() {
		dr, _ := makeAuthenticatedRequest("DELETE",
			fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", secretsAuthzTestNamespace, secretName),
			adminToken, nil)
		if dr != nil {
			dr.Body.Close()
		}
	}()

	// No-access user reads → either status code is a correct denial:
	//
	//   403 — CasbinAuthz middleware blocked the request before the handler
	//         ran. The middleware-level enforcement is the primary line of
	//         defense; a 403 here proves the route-level Casbin check is
	//         working. Acceptable.
	//   404 — Middleware was permissive (e.g., user has a partial wildcard
	//         that matched at the middleware layer) but the handler's
	//         defense-in-depth authorizeSecretAccess + namespace-membership
	//         check denied with "not leaking existence" semantics. Preferred
	//         because it matches the Instance handler's contract.
	//
	// We accept the union because the migration spec deliberately does not
	// guarantee which layer denies first — both are correct and the security
	// invariant (no read of an unauthorized secret) is upheld in either case.
	denyResp, err := makeAuthenticatedRequest("GET",
		fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", secretsAuthzTestNamespace, secretName),
		noAccessToken, nil)
	require.NoError(t, err)
	defer denyResp.Body.Close()
	assert.Containsf(t, []int{http.StatusNotFound, http.StatusForbidden}, denyResp.StatusCode,
		"no-access user must be denied (AC9) with 403 (middleware) or 404 (handler not-leak); got %d",
		denyResp.StatusCode)
	assert.NotEqual(t, http.StatusOK, denyResp.StatusCode, "must not succeed")
}
