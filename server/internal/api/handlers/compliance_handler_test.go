package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/services"
)

// mockComplianceService is a mock implementation of services.ComplianceService
type mockComplianceService struct {
	enabled                bool
	templates              []services.ConstraintTemplate
	constraints            []services.Constraint
	violations             []services.Violation
	summary                *services.ComplianceSummary
	getTemplateErr         error
	getConstraintErr       error
	listErr                error
	summaryErr             error
	updateEnforcementErr   error
	createConstraintErr    error
	createdConstraint      *services.Constraint
	violationsByConstraint map[string][]services.Violation
	violationsByResource   map[string][]services.Violation
}

func newMockComplianceService(enabled bool) *mockComplianceService {
	return &mockComplianceService{
		enabled:                enabled,
		templates:              []services.ConstraintTemplate{},
		constraints:            []services.Constraint{},
		violations:             []services.Violation{},
		violationsByConstraint: make(map[string][]services.Violation),
		violationsByResource:   make(map[string][]services.Violation),
	}
}

func (m *mockComplianceService) IsEnabled() bool {
	return m.enabled
}

func (m *mockComplianceService) GetStatus() *services.ComplianceStatus {
	status := &services.ComplianceStatus{
		Enterprise: true, // Mock is always EE build
	}
	if m.enabled {
		status.Available = true
		status.Message = "Compliance features are available"
		status.Gatekeeper = "installed"
	} else {
		status.Available = false
		status.Message = "OPA Gatekeeper is not available. Please verify Gatekeeper is installed in your cluster."
		status.Gatekeeper = "not_installed"
	}
	return status
}

func (m *mockComplianceService) ListConstraintTemplates(ctx context.Context) ([]services.ConstraintTemplate, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.templates, nil
}

func (m *mockComplianceService) GetConstraintTemplate(ctx context.Context, name string) (*services.ConstraintTemplate, error) {
	if m.getTemplateErr != nil {
		return nil, m.getTemplateErr
	}
	for _, t := range m.templates {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockComplianceService) ListConstraints(ctx context.Context) ([]services.Constraint, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.constraints, nil
}

func (m *mockComplianceService) GetConstraint(ctx context.Context, kind, name string) (*services.Constraint, error) {
	if m.getConstraintErr != nil {
		return nil, m.getConstraintErr
	}
	for _, c := range m.constraints {
		if c.Kind == kind && c.Name == name {
			return &c, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockComplianceService) ListViolations(ctx context.Context) ([]services.Violation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.violations, nil
}

func (m *mockComplianceService) GetViolationsByConstraint(ctx context.Context, kind, name string) ([]services.Violation, error) {
	key := kind + "/" + name
	if violations, ok := m.violationsByConstraint[key]; ok {
		return violations, nil
	}
	return []services.Violation{}, nil
}

func (m *mockComplianceService) GetViolationsByResource(ctx context.Context, kind, namespace, name string) ([]services.Violation, error) {
	key := kind + "/" + namespace + "/" + name
	if violations, ok := m.violationsByResource[key]; ok {
		return violations, nil
	}
	return []services.Violation{}, nil
}

func (m *mockComplianceService) GetSummary(ctx context.Context) (*services.ComplianceSummary, error) {
	if m.summaryErr != nil {
		return nil, m.summaryErr
	}
	if m.summary != nil {
		return m.summary, nil
	}
	return &services.ComplianceSummary{
		TotalTemplates:   len(m.templates),
		TotalConstraints: len(m.constraints),
		TotalViolations:  len(m.violations),
		ByEnforcement:    map[string]int{},
	}, nil
}

func (m *mockComplianceService) UpdateConstraintEnforcement(ctx context.Context, kind, name, newAction string) (*services.Constraint, error) {
	if m.updateEnforcementErr != nil {
		return nil, m.updateEnforcementErr
	}
	for i, c := range m.constraints {
		if c.Kind == kind && c.Name == name {
			m.constraints[i].EnforcementAction = newAction
			return &m.constraints[i], nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockComplianceService) CreateConstraint(ctx context.Context, req services.CreateConstraintRequest) (*services.Constraint, error) {
	if m.createConstraintErr != nil {
		return nil, m.createConstraintErr
	}

	// Check if template exists
	var templateFound bool
	var templateKind string
	for _, t := range m.templates {
		if t.Name == req.TemplateName {
			templateFound = true
			templateKind = t.Kind
			break
		}
	}
	if !templateFound {
		return nil, errors.New("not found")
	}

	// Check if constraint already exists
	for _, c := range m.constraints {
		if c.Name == req.Name && c.Kind == templateKind {
			return nil, errors.New("already exists")
		}
	}

	// Create the constraint
	enforcementAction := req.EnforcementAction
	if enforcementAction == "" {
		enforcementAction = "deny"
	}

	constraint := services.Constraint{
		Name:              req.Name,
		Kind:              templateKind,
		TemplateName:      req.TemplateName,
		EnforcementAction: enforcementAction,
		ViolationCount:    0,
		CreatedAt:         time.Now(),
		Parameters:        req.Parameters,
		Labels:            req.Labels,
	}

	if req.Match != nil {
		constraint.Match = services.ConstraintMatch{
			Kinds:      req.Match.Kinds,
			Namespaces: req.Match.Namespaces,
			Scope:      req.Match.Scope,
		}
	}

	m.constraints = append(m.constraints, constraint)

	// Allow test to override the returned constraint
	if m.createdConstraint != nil {
		return m.createdConstraint, nil
	}
	return &constraint, nil
}

// Helper to add test templates
func (m *mockComplianceService) addTemplate(name, kind, description string) {
	m.templates = append(m.templates, services.ConstraintTemplate{
		Name:        name,
		Kind:        kind,
		Description: description,
		CreatedAt:   time.Now(),
	})
}

// Helper to add test constraints
func (m *mockComplianceService) addConstraint(name, kind, templateName, enforcement string, violationCount int) {
	m.constraints = append(m.constraints, services.Constraint{
		Name:              name,
		Kind:              kind,
		TemplateName:      templateName,
		EnforcementAction: enforcement,
		ViolationCount:    violationCount,
		CreatedAt:         time.Now(),
	})
}

// Helper to add test violations
func (m *mockComplianceService) addViolation(constraintName, constraintKind, resourceKind, resourceNs, resourceName, message string) {
	violation := services.Violation{
		ConstraintName: constraintName,
		ConstraintKind: constraintKind,
		Resource: services.ViolationResource{
			Kind:      resourceKind,
			Namespace: resourceNs,
			Name:      resourceName,
		},
		Message:           message,
		EnforcementAction: "deny",
	}
	m.violations = append(m.violations, violation)

	// Also add to constraint lookup
	key := constraintKind + "/" + constraintName
	m.violationsByConstraint[key] = append(m.violationsByConstraint[key], violation)

	// Also add to resource lookup
	resourceKey := resourceKind + "/" + resourceNs + "/" + resourceName
	m.violationsByResource[resourceKey] = append(m.violationsByResource[resourceKey], violation)
}

// complianceTestSetup provides shared test setup for compliance handler tests.
type complianceTestSetup struct {
	handler  *ComplianceHandler
	service  *mockComplianceService
	enforcer *mockPolicyEnforcer
	recorder *httptest.ResponseRecorder
}

func newComplianceTestSetup(t *testing.T) *complianceTestSetup {
	t.Helper()
	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	return &complianceTestSetup{
		handler:  handler,
		service:  svc,
		enforcer: enforcer,
		recorder: httptest.NewRecorder(),
	}
}

func (s *complianceTestSetup) makeRequest(t *testing.T, method, path string, body []byte, userID string, roles ...string) *http.Request {
	t.Helper()
	if len(roles) == 0 {
		roles = []string{"role:serveradmin"}
	}
	userCtx := &middleware.UserContext{UserID: userID, CasbinRoles: roles}
	return newRequestWithUserContext(method, path, body, userCtx)
}

// --- Test: Enterprise Check (402 Payment Required vs 503 Service Unavailable) ---

// TestComplianceHandler_EnterpriseCheck_ServiceDisabled_Returns503 tests that
// EE builds with Gatekeeper unavailable return 503 (not 402).
func TestComplianceHandler_EnterpriseCheck_ServiceDisabled_Returns503(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(false) // EE build but Gatekeeper unavailable
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

	// Verify error message mentions Gatekeeper
	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if code, ok := errResp["code"].(string); !ok || code != "SERVICE_UNAVAILABLE" {
		t.Errorf("expected error code SERVICE_UNAVAILABLE, got %v", errResp["code"])
	}
}

func TestComplianceHandler_EnterpriseCheck_NilService(t *testing.T) {
	t.Parallel()

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(nil, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/summary", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("expected status %d, got %d", http.StatusPaymentRequired, resp.StatusCode)
	}
}

// --- Test: Permission Check (403 Forbidden) ---

func TestComplianceHandler_PermissionDenied(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "regular-user",
		Groups: []string{},
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/summary", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestComplianceHandler_PermissionError(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessErr: errors.New("policy check failed")}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/summary", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

func TestComplianceHandler_NilEnforcerFailsClosed(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	handler := NewComplianceHandler(svc, nil, nil) // nil enforcer

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/summary", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// Should fail closed (403) when enforcer is nil
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d (fail closed), got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestComplianceHandler_MissingUserContext(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	// No user context
	req := httptest.NewRequest(http.MethodGet, "/api/v1/compliance/summary", nil)
	rec := httptest.NewRecorder()

	handler.GetSummary(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// --- Test: GetSummary ---

func TestComplianceHandler_GetSummary_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels")
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 5)
	svc.summary = &services.ComplianceSummary{
		TotalTemplates:   1,
		TotalConstraints: 1,
		TotalViolations:  5,
		ByEnforcement: map[string]int{
			"deny": 5,
		},
	}

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

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var summary services.ComplianceSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if summary.TotalTemplates != 1 {
		t.Errorf("expected 1 template, got %d", summary.TotalTemplates)
	}
	if summary.TotalConstraints != 1 {
		t.Errorf("expected 1 constraint, got %d", summary.TotalConstraints)
	}
	if summary.TotalViolations != 5 {
		t.Errorf("expected 5 violations, got %d", summary.TotalViolations)
	}
}

func TestComplianceHandler_GetSummary_ServiceError(t *testing.T) {
	t.Parallel()

	s := newComplianceTestSetup(t)
	s.service.summaryErr = errors.New("database error")

	req := s.makeRequest(t, http.MethodGet, "/api/v1/compliance/summary", nil, "admin-user")
	s.handler.GetSummary(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// --- Test: ListTemplates ---

func TestComplianceHandler_ListTemplates_Success(t *testing.T) {
	t.Parallel()

	s := newComplianceTestSetup(t)
	s.service.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels")
	s.service.addTemplate("k8sallowedrepos", "K8sAllowedRepos", "Allowed container repos")

	req := s.makeRequest(t, http.MethodGet, "/api/v1/compliance/templates", nil, "admin-user")
	s.handler.ListTemplates(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.ConstraintTemplate]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 2 {
		t.Errorf("expected total 2, got %d", listResp.Total)
	}
	if len(listResp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(listResp.Items))
	}
}

func TestComplianceHandler_ListTemplates_Pagination(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	// Add 25 templates
	for i := 0; i < 25; i++ {
		svc.addTemplate("template"+string(rune('a'+i)), "Kind"+string(rune('A'+i)), "Description")
	}

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}

	// Page 1 with pageSize=10
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/templates?page=1&pageSize=10", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListTemplates(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.ConstraintTemplate]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 25 {
		t.Errorf("expected total 25, got %d", listResp.Total)
	}
	if len(listResp.Items) != 10 {
		t.Errorf("expected 10 items, got %d", len(listResp.Items))
	}
	if listResp.Page != 1 {
		t.Errorf("expected page 1, got %d", listResp.Page)
	}
	if listResp.PageSize != 10 {
		t.Errorf("expected pageSize 10, got %d", listResp.PageSize)
	}
}

func TestComplianceHandler_ListTemplates_ServiceError(t *testing.T) {
	t.Parallel()

	s := newComplianceTestSetup(t)
	s.service.listErr = errors.New("service error")

	req := s.makeRequest(t, http.MethodGet, "/api/v1/compliance/templates", nil, "admin-user")
	s.handler.ListTemplates(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// --- Test: GetTemplate ---

func TestComplianceHandler_GetTemplate_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/templates/k8srequiredlabels", nil, userCtx)
	req.SetPathValue("name", "k8srequiredlabels")
	rec := httptest.NewRecorder()

	handler.GetTemplate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var template services.ConstraintTemplate
	if err := json.NewDecoder(resp.Body).Decode(&template); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if template.Name != "k8srequiredlabels" {
		t.Errorf("expected name 'k8srequiredlabels', got '%s'", template.Name)
	}
	if template.Kind != "K8sRequiredLabels" {
		t.Errorf("expected kind 'K8sRequiredLabels', got '%s'", template.Kind)
	}
}

func TestComplianceHandler_GetTemplate_NotFound(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/templates/nonexistent", nil, userCtx)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.GetTemplate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestComplianceHandler_GetTemplate_MissingName(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/templates/", nil, userCtx)
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()

	handler.GetTemplate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// --- Test: ListConstraints ---

func TestComplianceHandler_ListConstraints_Success(t *testing.T) {
	t.Parallel()

	s := newComplianceTestSetup(t)
	s.service.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 5)
	s.service.addConstraint("allow-gcr-only", "K8sAllowedRepos", "k8sallowedrepos", "warn", 2)

	req := s.makeRequest(t, http.MethodGet, "/api/v1/compliance/constraints", nil, "admin-user")
	s.handler.ListConstraints(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.Constraint]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 2 {
		t.Errorf("expected total 2, got %d", listResp.Total)
	}
}

func TestComplianceHandler_ListConstraints_FilterByKind(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 5)
	svc.addConstraint("require-owner-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 3)
	svc.addConstraint("allow-gcr-only", "K8sAllowedRepos", "k8sallowedrepos", "warn", 2)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/constraints?kind=K8sRequiredLabels", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListConstraints(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.Constraint]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 2 {
		t.Errorf("expected total 2 (filtered by kind), got %d", listResp.Total)
	}
}

func TestComplianceHandler_ListConstraints_FilterByEnforcement(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 5)
	svc.addConstraint("allow-gcr-only", "K8sAllowedRepos", "k8sallowedrepos", "warn", 2)
	svc.addConstraint("test-policy", "K8sTestPolicy", "k8stestpolicy", "dryrun", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/constraints?enforcement=deny", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListConstraints(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.Constraint]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 1 {
		t.Errorf("expected total 1 (filtered by enforcement=deny), got %d", listResp.Total)
	}
}

// --- Test: GetConstraint ---

func TestComplianceHandler_GetConstraint_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 5)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label", nil, userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.GetConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var constraint services.Constraint
	if err := json.NewDecoder(resp.Body).Decode(&constraint); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if constraint.Name != "require-team-label" {
		t.Errorf("expected name 'require-team-label', got '%s'", constraint.Name)
	}
	if constraint.Kind != "K8sRequiredLabels" {
		t.Errorf("expected kind 'K8sRequiredLabels', got '%s'", constraint.Kind)
	}
}

func TestComplianceHandler_GetConstraint_NotFound(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/constraints/K8sRequiredLabels/nonexistent", nil, userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.GetConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestComplianceHandler_GetConstraint_MissingParams(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/constraints//", nil, userCtx)
	req.SetPathValue("kind", "")
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()

	handler.GetConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// --- Test: ListViolations ---

func TestComplianceHandler_ListViolations_Success(t *testing.T) {
	t.Parallel()

	s := newComplianceTestSetup(t)
	s.service.addViolation("require-team-label", "K8sRequiredLabels", "Pod", "default", "nginx", "Missing team label")
	s.service.addViolation("require-team-label", "K8sRequiredLabels", "Deployment", "default", "web", "Missing team label")

	req := s.makeRequest(t, http.MethodGet, "/api/v1/compliance/violations", nil, "admin-user")
	s.handler.ListViolations(s.recorder, req)

	resp := s.recorder.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.Violation]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 2 {
		t.Errorf("expected total 2, got %d", listResp.Total)
	}
}

func TestComplianceHandler_ListViolations_FilterByConstraint(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addViolation("require-team-label", "K8sRequiredLabels", "Pod", "default", "nginx", "Missing team label")
	svc.addViolation("allow-gcr-only", "K8sAllowedRepos", "Pod", "default", "nginx", "Invalid repo")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations?constraint=K8sRequiredLabels/require-team-label", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolations(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.Violation]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 1 {
		t.Errorf("expected total 1 (filtered by constraint), got %d", listResp.Total)
	}
}

func TestComplianceHandler_ListViolations_FilterByResource(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addViolation("require-team-label", "K8sRequiredLabels", "Pod", "default", "nginx", "Missing team label")
	svc.addViolation("require-team-label", "K8sRequiredLabels", "Pod", "production", "web", "Missing team label")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations?resource=Pod/default/nginx", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolations(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var listResp ListResponse[services.Violation]
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if listResp.Total != 1 {
		t.Errorf("expected total 1 (filtered by resource), got %d", listResp.Total)
	}
}

func TestComplianceHandler_ListViolations_InvalidConstraintFormat(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	// Missing slash - invalid format
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations?constraint=invalid-format", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolations(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestComplianceHandler_ListViolations_InvalidResourceFormat(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin-user",
	}
	// Missing slashes - invalid format (needs 3 parts)
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations?resource=Pod/nginx", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolations(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// --- Test: Pagination helpers ---

func TestParsePagination_Defaults(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	page, pageSize := parsePagination(req)

	if page != 1 {
		t.Errorf("expected default page 1, got %d", page)
	}
	if pageSize != 20 {
		t.Errorf("expected default pageSize 20, got %d", pageSize)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test?page=3&pageSize=50", nil)
	page, pageSize := parsePagination(req)

	if page != 3 {
		t.Errorf("expected page 3, got %d", page)
	}
	if pageSize != 50 {
		t.Errorf("expected pageSize 50, got %d", pageSize)
	}
}

func TestParsePagination_InvalidValues(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test?page=invalid&pageSize=-5", nil)
	page, pageSize := parsePagination(req)

	// Should fall back to defaults
	if page != 1 {
		t.Errorf("expected default page 1, got %d", page)
	}
	if pageSize != 20 {
		t.Errorf("expected default pageSize 20, got %d", pageSize)
	}
}

func TestParsePagination_MaxPageSize(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/test?pageSize=500", nil)
	_, pageSize := parsePagination(req)

	// Should be capped at default (invalid >100 is rejected)
	if pageSize != 20 {
		t.Errorf("expected pageSize to stay at default when >100, got %d", pageSize)
	}
}

func TestPaginateSlice(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Page 1, pageSize 3
	result := paginateSlice(items, 1, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
	if result[0] != 1 || result[2] != 3 {
		t.Errorf("expected [1,2,3], got %v", result)
	}

	// Page 2, pageSize 3
	result = paginateSlice(items, 2, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
	if result[0] != 4 || result[2] != 6 {
		t.Errorf("expected [4,5,6], got %v", result)
	}

	// Page 4, pageSize 3 (only 1 item remaining)
	result = paginateSlice(items, 4, 3)
	if len(result) != 1 {
		t.Errorf("expected 1 item, got %d", len(result))
	}
	if result[0] != 10 {
		t.Errorf("expected [10], got %v", result)
	}

	// Page 5, pageSize 3 (out of range)
	result = paginateSlice(items, 5, 3)
	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}

func TestPaginateSlice_EmptySlice(t *testing.T) {
	t.Parallel()

	var items []int
	result := paginateSlice(items, 1, 10)
	if len(result) != 0 {
		t.Errorf("expected 0 items, got %d", len(result))
	}
}

// --- Test: UpdateConstraintEnforcement ---

func TestComplianceHandler_UpdateEnforcement_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "dryrun", 5)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
		Groups: []string{"security-team"},
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var constraint services.Constraint
	if err := json.NewDecoder(resp.Body).Decode(&constraint); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if constraint.EnforcementAction != "deny" {
		t.Errorf("expected enforcement 'deny', got '%s'", constraint.EnforcementAction)
	}
}

func TestComplianceHandler_UpdateEnforcement_AllActions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		action      string
		expectValid bool
	}{
		{"deny action", "deny", true},
		{"warn action", "warn", true},
		{"dryrun action", "dryrun", true},
		{"invalid action", "block", false},
		{"empty action", "", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockComplianceService(true)
			svc.addConstraint("test-constraint", "K8sTest", "k8stest", "dryrun", 0)

			enforcer := &mockPolicyEnforcer{canAccessResult: true}
			handler := NewComplianceHandler(svc, enforcer, nil)

			userCtx := &middleware.UserContext{
				UserID: "admin",
			}

			body := `{"enforcementAction": "` + tc.action + `"}`
			req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/test-constraint/enforcement", []byte(body), userCtx)
			req.SetPathValue("kind", "K8sTest")
			req.SetPathValue("name", "test-constraint")
			rec := httptest.NewRecorder()

			handler.UpdateConstraintEnforcement(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if tc.expectValid {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected status %d for action '%s', got %d", http.StatusOK, tc.action, resp.StatusCode)
				}
			} else {
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("expected status %d for invalid action '%s', got %d", http.StatusBadRequest, tc.action, resp.StatusCode)
				}
			}
		})
	}
}

func TestComplianceHandler_UpdateEnforcement_NotFound(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	// No constraints added

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin",
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sTest/nonexistent/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sTest")
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestComplianceHandler_UpdateEnforcement_PermissionDenied(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "dryrun", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "viewer",
		Groups: []string{},
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestComplianceHandler_UpdateEnforcement_MissingParams(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin",
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints//enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for missing kind, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestComplianceHandler_UpdateEnforcement_InvalidJSON(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "dryrun", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin",
	}

	body := `{invalid json`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestComplianceHandler_UpdateEnforcement_ServiceError(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "dryrun", 0)
	svc.updateEnforcementErr = errors.New("Kubernetes API error")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin",
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d for service error, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

func TestComplianceHandler_UpdateEnforcement_Unauthorized(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "dryrun", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	// No user context
	body := `{"enforcementAction": "deny"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label/enforcement", nil)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	req.Body = newJSONReader(body)
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d for missing user context, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestComplianceHandler_UpdateEnforcement_EnterpriseRequired(t *testing.T) {
	t.Parallel()

	// nil service simulates OSS build
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(nil, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "admin",
	}

	body := `{"enforcementAction": "deny"}`
	req := newRequestWithUserContext(http.MethodPatch, "/api/v1/compliance/constraints/K8sRequiredLabels/require-team-label/enforcement", []byte(body), userCtx)
	req.SetPathValue("kind", "K8sRequiredLabels")
	req.SetPathValue("name", "require-team-label")
	rec := httptest.NewRecorder()

	handler.UpdateConstraintEnforcement(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("expected status %d for enterprise required, got %d", http.StatusPaymentRequired, resp.StatusCode)
	}
}

// --- Test: CreateConstraint ---

func TestComplianceHandler_CreateConstraint_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
		Groups: []string{"security-team"},
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels",
		"enforcementAction": "deny",
		"parameters": {"labels": ["team"]},
		"labels": {"app": "test"}
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var constraint services.Constraint
	if err := json.NewDecoder(resp.Body).Decode(&constraint); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if constraint.Name != "require-team-label" {
		t.Errorf("expected name 'require-team-label', got '%s'", constraint.Name)
	}
	if constraint.Kind != "K8sRequiredLabels" {
		t.Errorf("expected kind 'K8sRequiredLabels', got '%s'", constraint.Kind)
	}
	if constraint.TemplateName != "k8srequiredlabels" {
		t.Errorf("expected templateName 'k8srequiredlabels', got '%s'", constraint.TemplateName)
	}
	if constraint.EnforcementAction != "deny" {
		t.Errorf("expected enforcementAction 'deny', got '%s'", constraint.EnforcementAction)
	}
}

func TestComplianceHandler_CreateConstraint_DefaultEnforcement(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	// No enforcementAction provided - should default to deny
	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var constraint services.Constraint
	if err := json.NewDecoder(resp.Body).Decode(&constraint); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if constraint.EnforcementAction != "deny" {
		t.Errorf("expected default enforcementAction 'deny', got '%s'", constraint.EnforcementAction)
	}
}

func TestComplianceHandler_CreateConstraint_WithMatchRules(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels",
		"enforcementAction": "warn",
		"match": {
			"kinds": [{"apiGroups": [""], "kinds": ["Pod", "Deployment"]}],
			"namespaces": ["production", "staging"],
			"scope": "Namespaced"
		}
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var constraint services.Constraint
	if err := json.NewDecoder(resp.Body).Decode(&constraint); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(constraint.Match.Namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(constraint.Match.Namespaces))
	}
	if constraint.Match.Scope != "Namespaced" {
		t.Errorf("expected scope 'Namespaced', got '%s'", constraint.Match.Scope)
	}
}

func TestComplianceHandler_CreateConstraint_MissingName(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for missing name, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if msg, ok := errResp["message"].(string); !ok || msg != "Constraint name is required" {
		t.Errorf("expected message 'Constraint name is required', got '%v'", errResp["message"])
	}
}

func TestComplianceHandler_CreateConstraint_MissingTemplateName(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for missing templateName, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if msg, ok := errResp["message"].(string); !ok || msg != "Template name is required" {
		t.Errorf("expected message 'Template name is required', got '%v'", errResp["message"])
	}
}

func TestComplianceHandler_CreateConstraint_InvalidEnforcementAction(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels",
		"enforcementAction": "block"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid enforcement action, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if msg, ok := errResp["message"].(string); !ok || !strings.Contains(msg, "Invalid enforcement action") {
		t.Errorf("expected message about invalid enforcement action, got '%v'", errResp["message"])
	}
}

func TestComplianceHandler_CreateConstraint_ValidEnforcementActions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		action string
	}{
		{"deny action", "deny"},
		{"warn action", "warn"},
		{"dryrun action", "dryrun"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			svc := newMockComplianceService(true)
			svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

			enforcer := &mockPolicyEnforcer{canAccessResult: true}
			handler := NewComplianceHandler(svc, enforcer, nil)

			userCtx := &middleware.UserContext{
				UserID: "security-officer",
			}

			body := `{
				"name": "test-constraint-` + tc.action + `",
				"templateName": "k8srequiredlabels",
				"enforcementAction": "` + tc.action + `"
			}`
			req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
			rec := httptest.NewRecorder()

			handler.CreateConstraint(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Errorf("expected status %d for action '%s', got %d", http.StatusCreated, tc.action, resp.StatusCode)
			}

			var constraint services.Constraint
			if err := json.NewDecoder(resp.Body).Decode(&constraint); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if constraint.EnforcementAction != tc.action {
				t.Errorf("expected enforcementAction '%s', got '%s'", tc.action, constraint.EnforcementAction)
			}
		})
	}
}

func TestComplianceHandler_CreateConstraint_TemplateNotFound(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	// No templates added

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "nonexistent-template"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d for template not found, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestComplianceHandler_CreateConstraint_AlreadyExists(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")
	svc.addConstraint("require-team-label", "K8sRequiredLabels", "k8srequiredlabels", "deny", 0)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected status %d for already exists, got %d", http.StatusConflict, resp.StatusCode)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if code, ok := errResp["code"].(string); !ok || code != "ALREADY_EXISTS" {
		t.Errorf("expected error code ALREADY_EXISTS, got '%v'", errResp["code"])
	}
}

func TestComplianceHandler_CreateConstraint_PermissionDenied(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "viewer",
		Groups: []string{},
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d for permission denied, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestComplianceHandler_CreateConstraint_Unauthorized(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	// No user context
	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/constraints", nil)
	req.Body = newJSONReader(body)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d for missing user context, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestComplianceHandler_CreateConstraint_EnterpriseRequired(t *testing.T) {
	t.Parallel()

	// nil service simulates OSS build
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(nil, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Errorf("expected status %d for enterprise required, got %d", http.StatusPaymentRequired, resp.StatusCode)
	}
}

func TestComplianceHandler_CreateConstraint_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	// Service exists but disabled (Gatekeeper not installed)
	svc := newMockComplianceService(false)

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status %d for service unavailable, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestComplianceHandler_CreateConstraint_InvalidJSON(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{invalid json`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestComplianceHandler_CreateConstraint_ServiceError(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels on resources")
	svc.createConstraintErr = errors.New("Kubernetes API error")

	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	userCtx := &middleware.UserContext{
		UserID: "security-officer",
	}

	body := `{
		"name": "require-team-label",
		"templateName": "k8srequiredlabels"
	}`
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
	rec := httptest.NewRecorder()

	handler.CreateConstraint(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d for service error, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// TestComplianceHandler_CreateConstraint_NoRawErrorLeakage verifies AC-3: when
// constraint creation fails with a Gatekeeper/K8s error, the response returns a
// generic message ("Failed to create constraint") and does NOT leak raw error details.
func TestComplianceHandler_CreateConstraint_NoRawErrorLeakage(t *testing.T) {
	t.Parallel()

	internalErrors := []string{
		"admission webhook \"validation.gatekeeper.sh\" denied the request: invalid constraint",
		"Internal error occurred: failed calling webhook: Post https://gatekeeper-webhook:443/v1/admit: dial tcp 10.96.0.1:443: connect: connection refused",
		"the server could not find the requested resource (post k8srequiredlabels.constraints.gatekeeper.sh)",
		"etcd leader changed: context deadline exceeded",
	}

	for _, internalErr := range internalErrors {
		t.Run(internalErr[:40], func(t *testing.T) {
			svc := newMockComplianceService(true)
			svc.addTemplate("k8srequiredlabels", "K8sRequiredLabels", "Require labels")
			svc.createConstraintErr = errors.New(internalErr)

			enforcer := &mockPolicyEnforcer{canAccessResult: true}
			handler := NewComplianceHandler(svc, enforcer, nil)

			userCtx := &middleware.UserContext{
				UserID: "security-officer",
			}

			body := `{
				"name": "require-team-label",
				"templateName": "k8srequiredlabels"
			}`
			req := newRequestWithUserContext(http.MethodPost, "/api/v1/compliance/constraints", []byte(body), userCtx)
			rec := httptest.NewRecorder()

			handler.CreateConstraint(rec, req)

			resp := rec.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusInternalServerError {
				t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
			}

			var errResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}

			// Verify generic message (not raw error)
			if msg, ok := errResp["message"].(string); !ok || msg != "Failed to create constraint" {
				t.Errorf("expected generic message 'Failed to create constraint', got '%v'", errResp["message"])
			}

			// Verify no raw error details in the response
			respBody, _ := json.Marshal(errResp)
			respStr := string(respBody)
			if strings.Contains(respStr, "webhook") ||
				strings.Contains(respStr, "gatekeeper") ||
				strings.Contains(respStr, "etcd") ||
				strings.Contains(respStr, "10.96.0.1") ||
				strings.Contains(respStr, "dial tcp") {
				t.Errorf("response leaks internal error details: %s", respStr)
			}
		})
	}
}

// Helper function to create an io.Reader from a string
func newJSONReader(body string) *jsonReader {
	return &jsonReader{data: []byte(body)}
}

type jsonReader struct {
	data []byte
	pos  int
}

func (r *jsonReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, errors.New("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *jsonReader) Close() error {
	return nil
}

// --- Mock ViolationHistoryService ---

type mockViolationHistoryService struct {
	available     bool
	retentionDays int
	records       []services.ViolationHistoryRecord
	count         int
	listErr       error
	countErr      error
	exportErr     error
	exportCSV     string
}

func (m *mockViolationHistoryService) IsAvailable() bool {
	return m.available
}

func (m *mockViolationHistoryService) GetRetentionDays() int {
	return m.retentionDays
}

func (m *mockViolationHistoryService) ListByTimeRange(_ context.Context, _, _ time.Time, _ services.ViolationHistoryListOptions) ([]services.ViolationHistoryRecord, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.records, len(m.records), nil
}

func (m *mockViolationHistoryService) CountByTimeRange(_ context.Context, _, _ time.Time, _ services.ViolationHistoryExportFilters) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.count, nil
}

func (m *mockViolationHistoryService) ExportCSV(_ context.Context, _ time.Time, _ services.ViolationHistoryExportFilters, w io.Writer) error {
	if m.exportErr != nil {
		return m.exportErr
	}
	if m.exportCSV != "" {
		w.Write([]byte(m.exportCSV))
	}
	return nil
}

// --- Violation History Handler Tests ---

func TestComplianceHandler_ListViolationHistory_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	historySvc := &mockViolationHistoryService{
		available:     true,
		retentionDays: 90,
		records: []services.ViolationHistoryRecord{
			{
				Key:               "test-key",
				ConstraintKind:    "K8sRequiredLabels",
				ConstraintName:    "require-app-label",
				ResourceKind:      "Pod",
				ResourceNamespace: "default",
				ResourceName:      "nginx",
				EnforcementAction: "deny",
				Message:           "Missing label: app",
				FirstSeen:         time.Now().Add(-1 * time.Hour),
				Status:            "active",
			},
		},
	}
	handler.SetViolationHistoryService(historySvc)

	userCtx := &middleware.UserContext{UserID: "admin"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations/history?since="+time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339), nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolationHistory(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result ListResponse[services.ViolationHistoryRecord]
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ConstraintKind != "K8sRequiredLabels" {
		t.Errorf("expected K8sRequiredLabels, got %s", result.Items[0].ConstraintKind)
	}
}

func TestComplianceHandler_ListViolationHistory_HistoryUnavailable(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)
	// No history service set = nil

	userCtx := &middleware.UserContext{UserID: "admin"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations/history", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ListViolationHistory(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_CountViolationHistory_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	historySvc := &mockViolationHistoryService{
		available:     true,
		retentionDays: 90,
		count:         42,
	}
	handler.SetViolationHistoryService(historySvc)

	userCtx := &middleware.UserContext{UserID: "admin"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations/history/count?since="+time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339), nil, userCtx)
	rec := httptest.NewRecorder()

	handler.CountViolationHistory(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if int(result["count"].(float64)) != 42 {
		t.Errorf("expected count 42, got %v", result["count"])
	}
	if int(result["retentionDays"].(float64)) != 90 {
		t.Errorf("expected retentionDays 90, got %v", result["retentionDays"])
	}
}

func TestComplianceHandler_ExportViolationHistory_Success(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	historySvc := &mockViolationHistoryService{
		available:     true,
		retentionDays: 90,
		exportCSV:     "Constraint Kind,Constraint Name\nK8sRequiredLabels,require-app-label\n",
	}
	handler.SetViolationHistoryService(historySvc)

	userCtx := &middleware.UserContext{UserID: "admin"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations/history/export?since="+time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339), nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ExportViolationHistory(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("expected Content-Type text/csv, got %s", contentType)
	}

	disposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") {
		t.Errorf("expected Content-Disposition with attachment, got %s", disposition)
	}
	if !strings.Contains(disposition, ".csv") {
		t.Errorf("expected .csv in filename, got %s", disposition)
	}
}

func TestComplianceHandler_ExportViolationHistory_MissingSince(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: true}
	handler := NewComplianceHandler(svc, enforcer, nil)

	historySvc := &mockViolationHistoryService{available: true, retentionDays: 90}
	handler.SetViolationHistoryService(historySvc)

	userCtx := &middleware.UserContext{UserID: "admin"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations/history/export", nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ExportViolationHistory(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}

func TestComplianceHandler_ExportViolationHistory_PermissionDenied(t *testing.T) {
	t.Parallel()

	svc := newMockComplianceService(true)
	enforcer := &mockPolicyEnforcer{canAccessResult: false}
	handler := NewComplianceHandler(svc, enforcer, nil)

	historySvc := &mockViolationHistoryService{available: true, retentionDays: 90}
	handler.SetViolationHistoryService(historySvc)

	userCtx := &middleware.UserContext{UserID: "viewer"}
	req := newRequestWithUserContext(http.MethodGet, "/api/v1/compliance/violations/history/export?since="+time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339), nil, userCtx)
	rec := httptest.NewRecorder()

	handler.ExportViolationHistory(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", resp.StatusCode)
	}
}

func TestBuildExportFilename(t *testing.T) {
	t.Parallel()

	since := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	// No filters
	name := buildExportFilename(since, services.ViolationHistoryExportFilters{})
	if !strings.HasPrefix(name, "violations_2025-01-15_") || !strings.HasSuffix(name, ".csv") {
		t.Errorf("unexpected filename: %s", name)
	}

	// With enforcement filter
	name = buildExportFilename(since, services.ViolationHistoryExportFilters{Enforcement: "deny"})
	if !strings.Contains(name, "deny") {
		t.Errorf("expected 'deny' in filename: %s", name)
	}
}

func TestParseTimeRange_Defaults(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	q := map[string][]string{}

	since, until, ok := parseTimeRange(w, q)
	if !ok {
		t.Fatal("expected ok=true")
	}

	// Since should be ~7 days ago
	expectedSince := time.Now().UTC().Add(-7 * 24 * time.Hour)
	if since.Sub(expectedSince).Abs() > time.Second {
		t.Errorf("since should be ~7 days ago, got %v", since)
	}

	// Until should be ~now
	if time.Since(until) > time.Second {
		t.Errorf("until should be ~now, got %v", until)
	}
}

func TestParseTimeRange_InvalidSince(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	q := map[string][]string{"since": {"not-a-date"}}

	_, _, ok := parseTimeRange(w, q)
	if ok {
		t.Fatal("expected ok=false for invalid since")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
