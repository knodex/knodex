// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent"
)

// stubPresenceChecker returns a canned kagent.Result.
type stubPresenceChecker struct {
	result kagent.Result
}

func (s *stubPresenceChecker) Check(context.Context) kagent.Result {
	return s.result
}

func authedAgentsStatusRequest(t *testing.T) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/status", nil)
	userCtx := &middleware.UserContext{
		UserID: "test-user",
		Email:  "test@example.com",
	}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func boolPtr(b bool) *bool { return &b }

func TestAgentsStatusHandler_Ready(t *testing.T) {
	t.Parallel()
	handler := NewAgentsStatusHandler(&stubPresenceChecker{result: kagent.Result{
		Status:            kagent.StatusReady,
		CRDPresent:        boolPtr(true),
		ControllerHealthy: boolPtr(true),
		Message:           "kagent is installed and healthy",
	}})

	w := httptest.NewRecorder()
	handler.GetStatus(w, authedAgentsStatusRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Equal(t, "ready", raw["status"])
	assert.Equal(t, true, raw["crdPresent"])
	assert.Equal(t, true, raw["controllerHealthy"])
	assert.NotEmpty(t, raw["message"])
}

func TestAgentsStatusHandler_NotInstalled(t *testing.T) {
	t.Parallel()
	handler := NewAgentsStatusHandler(&stubPresenceChecker{result: kagent.Result{
		Status:     kagent.StatusNotInstalled,
		CRDPresent: boolPtr(false),
		Message:    "kagent Agent CRD (agents.kagent.dev) not found in cluster",
	}})

	w := httptest.NewRecorder()
	handler.GetStatus(w, authedAgentsStatusRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Equal(t, "not_installed", raw["status"])
	assert.Equal(t, false, raw["crdPresent"])
	// Health check short-circuited — must serialize as JSON null.
	healthy, present := raw["controllerHealthy"]
	assert.True(t, present, "controllerHealthy key must be present")
	assert.Nil(t, healthy)
}

func TestAgentsStatusHandler_Degraded_Returns200Never5xx(t *testing.T) {
	t.Parallel()
	handler := NewAgentsStatusHandler(&stubPresenceChecker{result: kagent.Result{
		Status:  kagent.StatusDegraded,
		Message: "kagent CRD discovery failed: connection reset",
	}})

	w := httptest.NewRecorder()
	handler.GetStatus(w, authedAgentsStatusRequest(t))

	// AC #3: degraded is a payload, never a 5xx.
	assert.Equal(t, http.StatusOK, w.Code)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Equal(t, "degraded", raw["status"])
	assert.Nil(t, raw["crdPresent"])
	assert.Nil(t, raw["controllerHealthy"])
	assert.NotEmpty(t, raw["message"])
}

func TestAgentsStatusHandler_NilChecker_Degraded(t *testing.T) {
	t.Parallel()
	handler := NewAgentsStatusHandler(nil)

	w := httptest.NewRecorder()
	handler.GetStatus(w, authedAgentsStatusRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Equal(t, "degraded", raw["status"])
	assert.Contains(t, raw["message"], "Kubernetes client unavailable")
}

func TestAgentsStatusHandler_Unauthenticated(t *testing.T) {
	t.Parallel()
	handler := NewAgentsStatusHandler(&stubPresenceChecker{result: kagent.Result{Status: kagent.StatusReady}})

	// No user context — simulates a request that bypassed auth middleware.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/status", nil)
	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp["code"])
}
