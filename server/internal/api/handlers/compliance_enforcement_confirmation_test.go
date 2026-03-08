// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/testutil"
)

func TestEnforcementConfirmation_EscalationWithoutToken_Returns409(t *testing.T) {
	t.Parallel()

	_, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sTest")
	req.SetPathValue("name", "my-constraint")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", resp.StatusCode)
	}

	var escalationResp EnforcementEscalationResponse
	if err := json.NewDecoder(resp.Body).Decode(&escalationResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if escalationResp.ConfirmationToken == "" {
		t.Error("expected non-empty confirmationToken")
	}
	if escalationResp.ExpiresIn != 300 {
		t.Errorf("expected expiresIn 300, got %d", escalationResp.ExpiresIn)
	}
	if escalationResp.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestEnforcementConfirmation_EscalationWithValidToken_Returns200(t *testing.T) {
	t.Parallel()

	_, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	// Phase 1: Get confirmation token
	body1 := `{"enforcementAction": "deny"}`
	req1 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body1), userCtx)
	req1.SetPathValue("kind", "K8sTest")
	req1.SetPathValue("name", "my-constraint")
	rec1 := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec1, req1)

	if rec1.Code != http.StatusConflict {
		t.Fatalf("phase 1: expected 409, got %d", rec1.Code)
	}

	var escalationResp EnforcementEscalationResponse
	if err := json.NewDecoder(rec1.Body).Decode(&escalationResp); err != nil {
		t.Fatalf("failed to decode phase 1 response: %v", err)
	}

	// Phase 2: Confirm with token
	body2 := `{"enforcementAction": "deny", "confirmationToken": "` + escalationResp.ConfirmationToken + `"}`
	req2 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body2), userCtx)
	req2.SetPathValue("kind", "K8sTest")
	req2.SetPathValue("name", "my-constraint")
	rec2 := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("phase 2: expected 200, got %d; body: %s", rec2.Code, rec2.Body.String())
	}

	var constraint services.Constraint
	if err := json.NewDecoder(rec2.Body).Decode(&constraint); err != nil {
		t.Fatalf("failed to decode phase 2 response: %v", err)
	}

	if constraint.EnforcementAction != "deny" {
		t.Errorf("expected enforcement 'deny', got '%s'", constraint.EnforcementAction)
	}
}

func TestEnforcementConfirmation_TokenSingleUse(t *testing.T) {
	t.Parallel()

	_, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	// Phase 1: Get token
	body1 := `{"enforcementAction": "deny"}`
	req1 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body1), userCtx)
	req1.SetPathValue("kind", "K8sTest")
	req1.SetPathValue("name", "my-constraint")
	rec1 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec1, req1)

	var resp1 EnforcementEscalationResponse
	json.NewDecoder(rec1.Body).Decode(&resp1)

	// Phase 2: Use token (succeeds)
	body2 := `{"enforcementAction": "deny", "confirmationToken": "` + resp1.ConfirmationToken + `"}`
	req2 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body2), userCtx)
	req2.SetPathValue("kind", "K8sTest")
	req2.SetPathValue("name", "my-constraint")
	rec2 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("first use: expected 200, got %d", rec2.Code)
	}

	// Reset constraint back to warn for re-escalation attempt
	svc.constraints[0].EnforcementAction = "warn"

	// Phase 3: Reuse same token (should fail with 410)
	req3 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body2), userCtx)
	req3.SetPathValue("kind", "K8sTest")
	req3.SetPathValue("name", "my-constraint")
	rec3 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec3, req3)

	if rec3.Code != http.StatusGone {
		t.Errorf("token reuse: expected 410, got %d", rec3.Code)
	}
}

func TestEnforcementConfirmation_DeescalationNoConfirmation(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "deny", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	// No Redis client — de-escalation should not need it

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	body := `{"enforcementAction": "warn"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sTest")
	req.SetPathValue("name", "my-constraint")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("de-escalation: expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}
}

func TestEnforcementConfirmation_NilRedis_Returns503(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	// No Redis client set

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sTest")
	req.SetPathValue("name", "my-constraint")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("nil redis: expected 503, got %d", rec.Code)
	}
}

func TestEnforcementConfirmation_ExpiredToken_Returns410(t *testing.T) {
	t.Parallel()

	mr, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	// Get token
	body1 := `{"enforcementAction": "deny"}`
	req1 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body1), userCtx)
	req1.SetPathValue("kind", "K8sTest")
	req1.SetPathValue("name", "my-constraint")
	rec1 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec1, req1)

	var resp1 EnforcementEscalationResponse
	json.NewDecoder(rec1.Body).Decode(&resp1)

	// Fast-forward past TTL
	mr.FastForward(6 * enforcementConfirmationTTL)

	// Try to use expired token
	body2 := `{"enforcementAction": "deny", "confirmationToken": "` + resp1.ConfirmationToken + `"}`
	req2 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body2), userCtx)
	req2.SetPathValue("kind", "K8sTest")
	req2.SetPathValue("name", "my-constraint")
	rec2 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec2, req2)

	if rec2.Code != http.StatusGone {
		t.Errorf("expired token: expected 410, got %d", rec2.Code)
	}
}

func TestEnforcementConfirmation_DryrunToDeny_RequiresConfirmation(t *testing.T) {
	t.Parallel()

	_, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("my-constraint", "K8sTest", "k8stest", "dryrun", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/my-constraint/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sTest")
	req.SetPathValue("name", "my-constraint")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("dryrun→deny: expected 409, got %d", rec.Code)
	}
}

func TestEnforcementConfirmation_BulkEscalation_EachGetsOwnToken(t *testing.T) {
	t.Parallel()

	_, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("constraint-a", "K8sTestA", "k8stesta", "warn", 0)
	svc.addConstraint("constraint-b", "K8sTestB", "k8stestb", "warn", 0)
	svc.addConstraint("constraint-c", "K8sTestC", "k8stestc", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "attacker",
		Groups: []string{"admins"},
	}

	constraints := []struct{ kind, name string }{
		{"K8sTestA", "constraint-a"},
		{"K8sTestB", "constraint-b"},
		{"K8sTestC", "constraint-c"},
	}

	tokens := make(map[string]string)
	for _, c := range constraints {
		body := `{"enforcementAction": "deny"}`
		req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/"+c.kind+"/"+c.name+"/enforcement", []byte(body), userCtx)
		req.SetPathValue("kind", c.kind)
		req.SetPathValue("name", c.name)
		rec := httptest.NewRecorder()

		handler.UpdateConstraintEnforcement(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("constraint %s: expected 409, got %d", c.name, rec.Code)
			continue
		}

		var resp EnforcementEscalationResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		tokens[c.name] = resp.ConfirmationToken
	}

	// All tokens should be different
	if tokens["constraint-a"] == tokens["constraint-b"] || tokens["constraint-b"] == tokens["constraint-c"] {
		t.Error("expected unique tokens for each constraint")
	}
}

func TestEnforcementConfirmation_CrossConstraintTokenRejected(t *testing.T) {
	t.Parallel()

	_, redisClient := testutil.NewRedis(t)

	svc := newMockComplianceService(true)
	svc.addConstraint("constraint-a", "K8sTestA", "k8stesta", "warn", 0)
	svc.addConstraint("constraint-b", "K8sTestB", "k8stestb", "warn", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	handler.SetRedisClient(redisClient)

	userCtx := &middleware.UserContext{
		UserID: "admin",
		Groups: []string{"admins"},
	}

	// Get token for constraint-a
	body1 := `{"enforcementAction": "deny"}`
	req1 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTestA/constraint-a/enforcement", []byte(body1), userCtx)
	req1.SetPathValue("kind", "K8sTestA")
	req1.SetPathValue("name", "constraint-a")
	rec1 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec1, req1)

	if rec1.Code != http.StatusConflict {
		t.Fatalf("expected 409 for constraint-a, got %d", rec1.Code)
	}

	var resp1 EnforcementEscalationResponse
	json.NewDecoder(rec1.Body).Decode(&resp1)

	// Try to use constraint-a's token against constraint-b — should be rejected
	body2 := `{"enforcementAction": "deny", "confirmationToken": "` + resp1.ConfirmationToken + `"}`
	req2 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTestB/constraint-b/enforcement", []byte(body2), userCtx)
	req2.SetPathValue("kind", "K8sTestB")
	req2.SetPathValue("name", "constraint-b")
	rec2 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec2, req2)

	// Token binding mismatch should cause rejection (410 Gone for invalid/expired token)
	if rec2.Code != http.StatusGone {
		t.Errorf("cross-constraint token: expected 410, got %d; body: %s", rec2.Code, rec2.Body.String())
	}

	// Verify the original token is NOT consumed — it should still work for constraint-a
	body3 := `{"enforcementAction": "deny", "confirmationToken": "` + resp1.ConfirmationToken + `"}`
	req3 := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTestA/constraint-a/enforcement", []byte(body3), userCtx)
	req3.SetPathValue("kind", "K8sTestA")
	req3.SetPathValue("name", "constraint-a")
	rec3 := httptest.NewRecorder()
	handler.UpdateConstraintEnforcement(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("original token should still work for constraint-a: expected 200, got %d; body: %s", rec3.Code, rec3.Body.String())
	}
}

func TestIsEscalation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		old, new string
		want     bool
	}{
		{"warn", "deny", true},
		{"dryrun", "deny", true},
		{"", "deny", true},
		{"deny", "deny", false},
		{"deny", "warn", false},
		{"deny", "dryrun", false},
		{"warn", "dryrun", false},
		{"dryrun", "warn", false},
	}

	for _, tt := range tests {
		got := isEscalation(tt.old, tt.new)
		if got != tt.want {
			t.Errorf("isEscalation(%q, %q) = %v, want %v", tt.old, tt.new, got, tt.want)
		}
	}
}
