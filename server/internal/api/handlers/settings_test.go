// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsHandler_GetSettings_ReturnsOrganization(t *testing.T) {
	t.Parallel()
	handler := NewSettingsHandler("orgA")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	userCtx := &middleware.UserContext{
		UserID: "test-user",
		Email:  "test@example.com",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp SettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "orgA", resp.Organization)
}

func TestSettingsHandler_GetSettings_ReturnsDefault(t *testing.T) {
	t.Parallel()
	handler := NewSettingsHandler("default")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	userCtx := &middleware.UserContext{
		UserID: "test-user",
		Email:  "test@example.com",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "default", resp.Organization)
}

func TestSettingsHandler_GetSettings_Unauthenticated(t *testing.T) {
	t.Parallel()
	handler := NewSettingsHandler("orgA")

	// No user context set - simulates unauthenticated request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	w := httptest.NewRecorder()

	handler.GetSettings(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp["code"])
}

func TestSettingsHandler_GetSettings_ResponseFormat(t *testing.T) {
	t.Parallel()
	handler := NewSettingsHandler("test-org")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	userCtx := &middleware.UserContext{
		UserID: "test-user",
		Email:  "test@example.com",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// Verify the response is a direct JSON body with "organization" field
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))

	// Should have "organization" field
	org, ok := raw["organization"]
	assert.True(t, ok, "response should have 'organization' field")
	assert.Equal(t, "test-org", org)

	// Should NOT have wrapper fields like "code", "message", "data"
	_, hasCode := raw["code"]
	assert.False(t, hasCode, "response should not have wrapper 'code' field")
	_, hasData := raw["data"]
	assert.False(t, hasData, "response should not have wrapper 'data' field")
}

func TestSettingsHandler_GetSettings_EmptyOrganization(t *testing.T) {
	t.Parallel()
	// Documents trust boundary: handler returns whatever it's given.
	// Config layer (Story 1.1) is responsible for sanitizing empty → "default".
	handler := NewSettingsHandler("")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	userCtx := &middleware.UserContext{
		UserID: "test-user",
		Email:  "test@example.com",
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp SettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// Handler passes through without sanitizing — config layer owns the default.
	assert.Equal(t, "", resp.Organization)
}

func TestSettingsHandler_GetSettings_NilUserContext(t *testing.T) {
	t.Parallel()
	// Distinct from Unauthenticated test: context key EXISTS but value is nil.
	// Tests the `userCtx == nil` branch at settings.go:32.
	handler := NewSettingsHandler("orgA")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, (*middleware.UserContext)(nil))
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetSettings(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp["code"])
}

func TestSettingsRoute_CoexistsWithSSOSubRoutes(t *testing.T) {
	t.Parallel()
	// AC #4: Verifies GET /api/v1/settings and GET /api/v1/settings/sso/providers
	// coexist without conflict on the same ServeMux (Go 1.22+ specificity routing).
	mux := http.NewServeMux()

	settingsHandler := NewSettingsHandler("test-org")
	mux.HandleFunc("GET /api/v1/settings", settingsHandler.GetSettings)

	// Simulated SSO sub-route (mirrors router.go registration)
	mux.HandleFunc("GET /api/v1/settings/sso/providers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"providers":[]}`))
	})

	userCtx := &middleware.UserContext{UserID: "test-user", Email: "test@example.com"}

	// Settings endpoint should return organization
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	req1 = req1.WithContext(context.WithValue(req1.Context(), middleware.UserContextKey, userCtx))
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)
	var settingsResp SettingsResponse
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &settingsResp))
	assert.Equal(t, "test-org", settingsResp.Organization)

	// SSO sub-route should NOT hit settings handler
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/settings/sso/providers", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "providers")
	assert.NotContains(t, w2.Body.String(), "organization")
}
