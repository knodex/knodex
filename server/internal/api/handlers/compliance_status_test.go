package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
)

// --- TDD Tests for Compliance Status Differentiation ---
// These tests verify that the API properly distinguishes between:
// 1. OSS build (nil service) → 402 Payment Required
// 2. Enterprise build with Gatekeeper unavailable → 503 Service Unavailable
// 3. Enterprise build with Gatekeeper available → 200 OK

// ComplianceStatus represents the response from GET /api/v1/compliance/status
type ComplianceStatus struct {
	Available  bool   `json:"available"`
	Enterprise bool   `json:"enterprise"`
	Message    string `json:"message"`
	Gatekeeper string `json:"gatekeeper,omitempty"` // "installed", "not_installed", "syncing"
}

// TestComplianceHandler_NilService_Returns402_WithLicenseMessage tests that
// OSS builds (nil service) return 402 with enterprise license message.
func TestComplianceHandler_NilService_Returns402_WithLicenseMessage(t *testing.T) {
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(nil, enforcer, nil) // nil service = OSS build

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/summary", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should return 402 Payment Required
	if resp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("expected status %d, got %d", http.StatusPaymentRequired, resp.StatusCode)
	}

	// Should have error body with enterprise license message
	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if code, ok := errResp["code"].(string); !ok || code != "LICENSE_REQUIRED" {
		t.Errorf("expected error code LICENSE_REQUIRED, got %v", errResp["code"])
	}

	if msg, ok := errResp["message"].(string); !ok {
		t.Errorf("expected message in response, got %v", errResp)
	} else if msg != "This feature requires a valid enterprise license" {
		t.Errorf("expected enterprise license message, got: %s", msg)
	}
}

// TestComplianceHandler_ServiceDisabled_Returns503_WithGatekeeperMessage tests that
// Enterprise builds with Gatekeeper unavailable return 503 with helpful message.
func TestComplianceHandler_ServiceDisabled_Returns503_WithGatekeeperMessage(t *testing.T) {
	svc := newMockComplianceService(false) // Service exists but IsEnabled() = false
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/summary", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should return 503 Service Unavailable (not 402)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d (Service Unavailable), got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}

	// Should have error body with Gatekeeper message
	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if code, ok := errResp["code"].(string); !ok || code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected error code SERVICE_UNAVAILABLE, got %v", errResp["code"])
	}

	// Message should mention Gatekeeper, not enterprise license
	if msg, ok := errResp["message"].(string); !ok {
		t.Errorf("expected message in response, got %v", errResp)
	} else {
		if msg == "compliance feature requires enterprise license" {
			t.Errorf("message should NOT be about enterprise license when service exists")
		}
		// Should mention Gatekeeper
		if !containsSubstring(msg, "Gatekeeper") && !containsSubstring(msg, "gatekeeper") {
			t.Errorf("expected message to mention Gatekeeper, got: %s", msg)
		}
	}
}

// TestComplianceHandler_GetStatus_NilService tests status endpoint for OSS builds.
func TestComplianceHandler_GetStatus_NilService(t *testing.T) {
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(nil, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/status", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Status endpoint should always return 200 with status info
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var status ComplianceStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	// OSS build: not available, not enterprise
	if status.Available {
		t.Errorf("expected available=false for OSS build")
	}
	if status.Enterprise {
		t.Errorf("expected enterprise=false for OSS build")
	}
	if status.Message == "" {
		t.Errorf("expected non-empty message")
	}
}

// TestComplianceHandler_GetStatus_GatekeeperUnavailable tests status for EE without Gatekeeper.
func TestComplianceHandler_GetStatus_GatekeeperUnavailable(t *testing.T) {
	svc := newMockComplianceService(false) // EE build but Gatekeeper not available
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/status", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var status ComplianceStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	// EE build but Gatekeeper unavailable
	if status.Available {
		t.Errorf("expected available=false when Gatekeeper is unavailable")
	}
	if !status.Enterprise {
		t.Errorf("expected enterprise=true for EE build")
	}
	if status.Gatekeeper != "not_installed" && status.Gatekeeper != "syncing" {
		t.Errorf("expected gatekeeper status to indicate unavailability, got: %s", status.Gatekeeper)
	}
}

// TestComplianceHandler_GetStatus_FullyAvailable tests status when everything is working.
func TestComplianceHandler_GetStatus_FullyAvailable(t *testing.T) {
	svc := newMockComplianceService(true) // EE build with Gatekeeper available
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/status", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetStatus(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var status ComplianceStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}

	// EE build with Gatekeeper available
	if !status.Available {
		t.Errorf("expected available=true when Gatekeeper is available")
	}
	if !status.Enterprise {
		t.Errorf("expected enterprise=true for EE build")
	}
	if status.Gatekeeper != "installed" {
		t.Errorf("expected gatekeeper=installed, got: %s", status.Gatekeeper)
	}
}

// TestComplianceHandler_ListTemplates_ServiceDisabled_Returns503 tests list templates with unavailable Gatekeeper.
func TestComplianceHandler_ListTemplates_ServiceDisabled_Returns503(t *testing.T) {
	svc := newMockComplianceService(false)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/templates", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListTemplates(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// TestComplianceHandler_ListConstraints_ServiceDisabled_Returns503 tests list constraints with unavailable Gatekeeper.
func TestComplianceHandler_ListConstraints_ServiceDisabled_Returns503(t *testing.T) {
	svc := newMockComplianceService(false)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/constraints", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListConstraints(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// TestComplianceHandler_ListViolations_ServiceDisabled_Returns503 tests list violations with unavailable Gatekeeper.
func TestComplianceHandler_ListViolations_ServiceDisabled_Returns503(t *testing.T) {
	svc := newMockComplianceService(false)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolations(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

// Helper function to check substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsSubstring(s[1:], substr)))
}
