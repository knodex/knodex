package handlers

// NOTE: Tests in this file are NOT safe for t.Parallel() due to package-level
// mockLicenseService interface assertion var and shared mock state patterns across subtests.
// See tech-spec: go-test-mechanics-parallel-and-setup for details.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLicenseService implements services.LicenseService for testing.
type mockLicenseService struct {
	licensed    bool
	gracePeriod bool
	readOnly    bool
	features    map[string]bool
	hasFeatures map[string]bool
	status      *services.LicenseStatus
	info        *services.LicenseInfo
	updateErr   error
	updateToken string
}

var _ services.LicenseService = (*mockLicenseService)(nil)

func (m *mockLicenseService) IsLicensed() bool                   { return m.licensed }
func (m *mockLicenseService) IsGracePeriod() bool                { return m.gracePeriod }
func (m *mockLicenseService) IsFeatureEnabled(f string) bool     { return m.features[f] }
func (m *mockLicenseService) IsReadOnly() bool                   { return m.readOnly }
func (m *mockLicenseService) HasFeature(f string) bool           { return m.hasFeatures[f] }
func (m *mockLicenseService) GetLicense() *services.LicenseInfo  { return m.info }
func (m *mockLicenseService) GetStatus() *services.LicenseStatus { return m.status }
func (m *mockLicenseService) UpdateLicense(token string) error {
	m.updateToken = token
	return m.updateErr
}

// mockLicenseAccessChecker implements licenseAccessChecker for testing.
type mockLicenseAccessChecker struct {
	allow bool
	err   error
}

func (m *mockLicenseAccessChecker) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return m.allow, m.err
}

func TestLicenseHandler_GetStatus_Licensed(t *testing.T) {
	status := &services.LicenseStatus{
		Licensed:   true,
		Enterprise: true,
		Status:     "valid",
		Message:    "Licensed to test-customer",
	}
	svc := &mockLicenseService{licensed: true, status: status}
	handler := NewLicenseHandler(svc, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp services.LicenseStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Licensed)
	assert.Equal(t, "valid", resp.Status)
}

func TestLicenseHandler_GetStatus_Unlicensed(t *testing.T) {
	status := &services.LicenseStatus{
		Licensed:   false,
		Enterprise: true,
		Status:     "missing",
		Message:    "No enterprise license installed",
	}
	svc := &mockLicenseService{licensed: false, status: status}
	handler := NewLicenseHandler(svc, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp services.LicenseStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Licensed)
	assert.Equal(t, "missing", resp.Status)
}

func TestLicenseHandler_GetStatus_NilService(t *testing.T) {
	handler := NewLicenseHandler(nil, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
	w := httptest.NewRecorder()

	handler.GetStatus(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp services.LicenseStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Licensed)
	assert.False(t, resp.Enterprise)
	assert.Equal(t, "oss", resp.Status)
}

func TestLicenseHandler_UpdateLicense_NoAuth(t *testing.T) {
	svc := &mockLicenseService{}
	checker := &mockLicenseAccessChecker{allow: true}
	handler := NewLicenseHandler(svc, checker, slog.Default())

	body := `{"token":"some-jwt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLicenseHandler_UpdateLicense_NoEnforcer(t *testing.T) {
	svc := &mockLicenseService{}
	handler := NewLicenseHandler(svc, nil, slog.Default())

	body := `{"token":"some-jwt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID: "admin@test.local",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestLicenseHandler_UpdateLicense_Forbidden(t *testing.T) {
	svc := &mockLicenseService{}
	checker := &mockLicenseAccessChecker{allow: false}
	handler := NewLicenseHandler(svc, checker, slog.Default())

	body := `{"token":"some-jwt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID:      "viewer@test.local",
		Groups:      []string{"viewers"},
		CasbinRoles: []string{"proj:test:viewer"},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestLicenseHandler_UpdateLicense_Success(t *testing.T) {
	updatedStatus := &services.LicenseStatus{
		Licensed:   true,
		Enterprise: true,
		Status:     "valid",
		Message:    "Licensed to new-customer",
	}
	svc := &mockLicenseService{status: updatedStatus}
	checker := &mockLicenseAccessChecker{allow: true}
	handler := NewLicenseHandler(svc, checker, slog.Default())

	body := `{"token":"valid-jwt-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID:      "admin@test.local",
		Groups:      []string{"admins"},
		CasbinRoles: []string{"role:serveradmin"},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "valid-jwt-token", svc.updateToken)

	var resp services.LicenseStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Licensed)
}

func TestLicenseHandler_UpdateLicense_InvalidToken(t *testing.T) {
	svc := &mockLicenseService{updateErr: errors.New("invalid license token")}
	checker := &mockLicenseAccessChecker{allow: true}
	handler := NewLicenseHandler(svc, checker, slog.Default())

	body := `{"token":"bad-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID:      "admin@test.local",
		CasbinRoles: []string{"role:serveradmin"},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestLicenseHandler_UpdateLicense_NoRawErrorLeakage verifies AC-5: when license
// token parsing fails, the response contains a generic message ("Invalid license token")
// and does NOT leak JWT parsing internals (key IDs, signature details, etc.).
func TestLicenseHandler_UpdateLicense_NoRawErrorLeakage(t *testing.T) {
	internalErrors := []string{
		"token is expired by 24h",
		"crypto/rsa: verification error",
		"square/go-jose: error in cryptographic primitive",
		"unexpected signing method: HS256, expected RS256",
		"key with ID 'abc-123' not found in JWKS",
		"invalid compact serialization format",
	}

	for _, internalErr := range internalErrors {
		t.Run(internalErr, func(t *testing.T) {
			svc := &mockLicenseService{updateErr: errors.New(internalErr)}
			checker := &mockLicenseAccessChecker{allow: true}
			handler := NewLicenseHandler(svc, checker, slog.Default())

			body := `{"token":"bad-jwt-token"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
			ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
				UserID:      "admin@test.local",
				CasbinRoles: []string{"role:serveradmin"},
			})
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.UpdateLicense(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

			// Verify generic message is returned
			assert.Equal(t, "Invalid license token", resp["message"])

			// Verify NO internal error details are leaked
			bodyStr := w.Body.String()
			assert.NotContains(t, bodyStr, internalErr,
				"response should not contain raw internal error: %s", internalErr)
		})
	}
}

func TestLicenseHandler_UpdateLicense_EmptyToken(t *testing.T) {
	svc := &mockLicenseService{}
	checker := &mockLicenseAccessChecker{allow: true}
	handler := NewLicenseHandler(svc, checker, slog.Default())

	body := `{"token":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID:      "admin@test.local",
		CasbinRoles: []string{"role:serveradmin"},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLicenseHandler_UpdateLicense_EnforcerError(t *testing.T) {
	svc := &mockLicenseService{}
	checker := &mockLicenseAccessChecker{err: errors.New("casbin error")}
	handler := NewLicenseHandler(svc, checker, slog.Default())

	body := `{"token":"some-jwt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license", bytes.NewBufferString(body))
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID: "admin@test.local",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.UpdateLicense(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
