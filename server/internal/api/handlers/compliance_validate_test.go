// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/compliance"
)

func withAuth(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID: "user@test.local",
	})
	return r.WithContext(ctx)
}

func TestComplianceValidate_Returns401_NoAuth(t *testing.T) {
	t.Parallel()
	handler := NewComplianceValidateHandler(nil, nil)
	rec := httptest.NewRecorder()
	body := `{"rgdName":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestComplianceValidate_Returns400_InvalidBody(t *testing.T) {
	t.Parallel()
	handler := NewComplianceValidateHandler(nil, nil)
	rec := httptest.NewRecorder()
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString("not json")))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestComplianceValidate_Returns400_MissingRGDName(t *testing.T) {
	t.Parallel()
	handler := NewComplianceValidateHandler(nil, nil)
	rec := httptest.NewRecorder()
	body := `{"project":"alpha","namespace":"default"}`
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body)))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestComplianceValidate_OSS_ReturnsPass(t *testing.T) {
	t.Parallel()
	// nil checker simulates OSS build
	handler := NewComplianceValidateHandler(nil, nil)
	rec := httptest.NewRecorder()
	body := `{"rgdName":"postgres","project":"alpha","namespace":"default","values":{}}`
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body)))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp ComplianceValidateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "pass", resp.Result)
	assert.Empty(t, resp.Violations)
}

func TestComplianceValidate_NoopChecker_ReturnsPass(t *testing.T) {
	t.Parallel()
	// Use the actual noopChecker from the compliance package
	handler := NewComplianceValidateHandler(compliance.GetChecker(), nil)
	rec := httptest.NewRecorder()
	body := `{"rgdName":"postgres","project":"alpha","namespace":"default","values":{}}`
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body)))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp ComplianceValidateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "pass", resp.Result)
	assert.Empty(t, resp.Violations)
}

func TestComplianceValidate_WarningResult(t *testing.T) {
	t.Parallel()
	checker := &mockComplianceChecker{
		result: &compliance.AuditResult{
			Passed: false,
			Findings: []compliance.Finding{
				{Severity: compliance.SeverityMedium, Rule: "prefer-private-registry", Description: "Use private registry", Passed: false, Remediation: "Use registry.example.com"},
			},
		},
	}
	handler := NewComplianceValidateHandler(checker, nil)
	rec := httptest.NewRecorder()
	body := `{"rgdName":"postgres","project":"alpha","namespace":"default","values":{}}`
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body)))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp ComplianceValidateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "warning", resp.Result)
	assert.Len(t, resp.Violations, 1)
	assert.Equal(t, "warning", resp.Violations[0].Severity)
	assert.Equal(t, "prefer-private-registry", resp.Violations[0].Policy)
}

func TestComplianceValidate_BlockResult(t *testing.T) {
	t.Parallel()
	checker := &mockComplianceChecker{
		result: &compliance.AuditResult{
			Passed: false,
			Findings: []compliance.Finding{
				{Severity: compliance.SeverityCritical, Rule: "require-labels", Description: "Missing required labels", Passed: false, Remediation: "Add app label"},
			},
		},
	}
	handler := NewComplianceValidateHandler(checker, nil)
	rec := httptest.NewRecorder()
	body := `{"rgdName":"postgres","project":"alpha","namespace":"default","values":{}}`
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body)))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp ComplianceValidateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "block", resp.Result)
	assert.Len(t, resp.Violations, 1)
	assert.Equal(t, "error", resp.Violations[0].Severity)
}

func TestComplianceValidate_PassResult_AllFindingsPass(t *testing.T) {
	t.Parallel()
	checker := &mockComplianceChecker{
		result: &compliance.AuditResult{
			Passed: true,
			Findings: []compliance.Finding{
				{Severity: compliance.SeverityMedium, Rule: "check-1", Description: "Passed", Passed: true},
			},
		},
	}
	handler := NewComplianceValidateHandler(checker, nil)
	rec := httptest.NewRecorder()
	body := `{"rgdName":"postgres","project":"alpha","namespace":"default","values":{}}`
	req := withAuth(httptest.NewRequest(http.MethodPost, "/api/v1/compliance/validate", bytes.NewBufferString(body)))

	handler.Validate(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp ComplianceValidateResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "pass", resp.Result)
	assert.Empty(t, resp.Violations)
}

// --- Mock ---

type mockComplianceChecker struct {
	result *compliance.AuditResult
	err    error
}

func (m *mockComplianceChecker) AuditDeployment(_ context.Context, _ *compliance.Deployment) (*compliance.AuditResult, error) {
	return m.result, m.err
}

func (m *mockComplianceChecker) IsEnabled() bool {
	return true
}
