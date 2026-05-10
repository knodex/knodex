// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

//go:build e2e

package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireLocalLoginDisabledEnv guards the suite. Both env vars must be set:
//   - E2E_TESTS=true (consumed by the shared TestMain in rbac_test.go); without
//     it the package's TestMain calls os.Exit(0) before any test runs and the
//     suite silently passes.
//   - E2E_LOCAL_LOGIN_DISABLED=true (this suite specifically requires the server
//     to be deployed with LOCAL_LOGIN_ENABLED=false).
//
// Run with:
//
//	E2E_TESTS=true E2E_LOCAL_LOGIN_DISABLED=true go test -tags=e2e ./test/e2e/...
func requireLocalLoginDisabledEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping: requires E2E_TESTS=true (shared TestMain guard)")
	}
	if os.Getenv("E2E_LOCAL_LOGIN_DISABLED") != "true" {
		t.Skip("Skipping: requires server deployed with LOCAL_LOGIN_ENABLED=false (set E2E_LOCAL_LOGIN_DISABLED=true)")
	}
}

// TestLocalLoginDisabled_LoginReturns404 verifies that when the server is
// deployed with LOCAL_LOGIN_ENABLED=false, the local login route is not
// registered and the request returns 404. The route is omitted entirely so
// attackers cannot drain the rate-limit budget or pollute the audit log.
func TestLocalLoginDisabled_LoginReturns404(t *testing.T) {
	requireLocalLoginDisabledEnv(t)

	body, err := json.Marshal(map[string]string{
		"username": "admin",
		"password": "any-password",
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, apiBaseURL+"/api/v1/auth/local/login", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"local login route should not be registered when LOCAL_LOGIN_ENABLED=false")

	// Drain the body so we don't leak the connection — content is irrelevant.
	_, _ = io.ReadAll(resp.Body)
}

// TestLocalLoginDisabled_ProvidersReportsFalse verifies that the OIDC providers
// endpoint reports localLoginEnabled=false when the server is deployed with
// LOCAL_LOGIN_ENABLED=false. The frontend uses this to hide the login form.
func TestLocalLoginDisabled_ProvidersReportsFalse(t *testing.T) {
	requireLocalLoginDisabledEnv(t)

	resp, err := httpClient.Get(apiBaseURL + "/api/v1/auth/oidc/providers")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var providersResp struct {
		Providers         []string `json:"providers"`
		LocalLoginEnabled bool     `json:"localLoginEnabled"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&providersResp))

	assert.False(t, providersResp.LocalLoginEnabled,
		"localLoginEnabled must be false when LOCAL_LOGIN_ENABLED=false")
}
