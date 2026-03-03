package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/provops-org/knodex/server/internal/api/middleware"
)

// mockCanIService is a mock implementation of CanIServiceInterface
type mockCanIService struct {
	canIResult      bool
	canIErr         error
	mappedGroups    []string
	mappedGroupsErr error
}

func (m *mockCanIService) CanI(userID string, groups []string, resource, action, subresource string) (bool, error) {
	if m.canIErr != nil {
		return false, m.canIErr
	}
	return m.canIResult, nil
}

func (m *mockCanIService) GetMappedGroups(groups []string) ([]string, error) {
	if m.mappedGroupsErr != nil {
		return nil, m.mappedGroupsErr
	}
	if m.mappedGroups != nil {
		return m.mappedGroups, nil
	}
	// Default: return all groups (backwards compatible behavior)
	return groups, nil
}

// Note: Uses mockPolicyEnforcer from project_handler_test.go (same package)

// Helper to create request with user context
func createAccountTestRequest(t *testing.T, method, path string, userID string, groups []string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	userCtx := &middleware.UserContext{
		UserID: userID,
		Groups: groups,
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	return req.WithContext(ctx)
}

// Helper to create request with full user context for Info endpoint
func createAccountInfoRequest(t *testing.T, userCtx *middleware.UserContext) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/info", nil)
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	return req.WithContext(ctx)
}

// Helper to setup request with path values
func setupAccountRequestWithPathValues(req *http.Request, resource, action, subresource string) *http.Request {
	req.SetPathValue("resource", resource)
	req.SetPathValue("action", action)
	req.SetPathValue("subresource", subresource)
	return req
}

func TestAccountHandler_CanI_Success(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		resource       string
		action         string
		subresource    string
		canIResult     bool
		expectedValue  string
		expectedStatus int
	}{
		{
			name:           "allowed_instances_create",
			resource:       "instances",
			action:         "create",
			subresource:    "my-project",
			canIResult:     true,
			expectedValue:  "yes",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "denied_instances_create",
			resource:       "instances",
			action:         "create",
			subresource:    "restricted-project",
			canIResult:     false,
			expectedValue:  "no",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "allowed_projects_delete",
			resource:       "projects",
			action:         "delete",
			subresource:    "-",
			canIResult:     true,
			expectedValue:  "yes",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "allowed_settings_update",
			resource:       "settings",
			action:         "update",
			subresource:    "-",
			canIResult:     true,
			expectedValue:  "yes",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "denied_repositories_create",
			resource:       "repositories",
			action:         "create",
			subresource:    "my-project",
			canIResult:     false,
			expectedValue:  "no",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Setup mocks
			canIService := &mockCanIService{canIResult: tt.canIResult}
			handler := NewAccountHandler(canIService)

			// Create request with user context
			req := createAccountTestRequest(t, http.MethodGet,
				"/api/v1/account/can-i/"+tt.resource+"/"+tt.action+"/"+tt.subresource,
				"test-user@example.com",
				[]string{"developers"},
			)
			req = setupAccountRequestWithPathValues(req, tt.resource, tt.action, tt.subresource)

			// Execute
			rr := httptest.NewRecorder()
			handler.CanI(rr, req)

			// Assert status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Assert response body
			var response CanIResponse
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response.Value != tt.expectedValue {
				t.Errorf("expected value %q, got %q", tt.expectedValue, response.Value)
			}
		})
	}
}

func TestAccountHandler_CanI_InvalidResource(t *testing.T) {
	t.Parallel()
	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	req := createAccountTestRequest(t, http.MethodGet,
		"/api/v1/account/can-i/invalid-resource/create/my-project",
		"test-user@example.com",
		nil,
	)
	req = setupAccountRequestWithPathValues(req, "invalid-resource", "create", "my-project")

	rr := httptest.NewRecorder()
	handler.CanI(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAccountHandler_CanI_InvalidAction(t *testing.T) {
	t.Parallel()
	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	req := createAccountTestRequest(t, http.MethodGet,
		"/api/v1/account/can-i/instances/invalid-action/my-project",
		"test-user@example.com",
		nil,
	)
	req = setupAccountRequestWithPathValues(req, "instances", "invalid-action", "my-project")

	rr := httptest.NewRecorder()
	handler.CanI(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAccountHandler_CanI_MissingResource(t *testing.T) {
	t.Parallel()
	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	req := createAccountTestRequest(t, http.MethodGet,
		"/api/v1/account/can-i//create/my-project",
		"test-user@example.com",
		nil,
	)
	req = setupAccountRequestWithPathValues(req, "", "create", "my-project")

	rr := httptest.NewRecorder()
	handler.CanI(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAccountHandler_CanI_MissingAction(t *testing.T) {
	t.Parallel()
	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	req := createAccountTestRequest(t, http.MethodGet,
		"/api/v1/account/can-i/instances//my-project",
		"test-user@example.com",
		nil,
	)
	req = setupAccountRequestWithPathValues(req, "instances", "", "my-project")

	rr := httptest.NewRecorder()
	handler.CanI(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestAccountHandler_CanI_Unauthenticated(t *testing.T) {
	t.Parallel()
	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	// Create request without user context
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/account/can-i/instances/create/my-project",
		nil,
	)
	req = setupAccountRequestWithPathValues(req, "instances", "create", "my-project")

	rr := httptest.NewRecorder()
	handler.CanI(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAccountHandler_CanI_AllValidResources(t *testing.T) {
	t.Parallel()
	validResources := []string{
		"instances",
		"projects",
		"repositories",
		"settings",
		"rgds",
		"users",
		"applications",
		"compliance",
	}

	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	for _, resource := range validResources {
		resource := resource
		t.Run(resource, func(t *testing.T) {
			t.Parallel()
			req := createAccountTestRequest(t, http.MethodGet,
				"/api/v1/account/can-i/"+resource+"/get/-",
				"test-user@example.com",
				nil,
			)
			req = setupAccountRequestWithPathValues(req, resource, "get", "-")

			rr := httptest.NewRecorder()
			handler.CanI(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status %d for resource %q, got %d", http.StatusOK, resource, rr.Code)
			}
		})
	}
}

func TestAccountHandler_CanI_AllValidActions(t *testing.T) {
	t.Parallel()
	validActions := []string{
		"get",
		"list",
		"create",
		"update",
		"delete",
	}

	canIService := &mockCanIService{canIResult: true}
	handler := NewAccountHandler(canIService)

	for _, action := range validActions {
		action := action
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			req := createAccountTestRequest(t, http.MethodGet,
				"/api/v1/account/can-i/instances/"+action+"/-",
				"test-user@example.com",
				nil,
			)
			req = setupAccountRequestWithPathValues(req, "instances", action, "-")

			rr := httptest.NewRecorder()
			handler.CanI(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status %d for action %q, got %d", http.StatusOK, action, rr.Code)
			}
		})
	}
}

func TestAccountHandler_CanI_ServiceError(t *testing.T) {
	t.Parallel()
	canIService := &mockCanIService{
		canIErr: context.DeadlineExceeded,
	}
	handler := NewAccountHandler(canIService)

	req := createAccountTestRequest(t, http.MethodGet,
		"/api/v1/account/can-i/instances/create/my-project",
		"test-user@example.com",
		nil,
	)
	req = setupAccountRequestWithPathValues(req, "instances", "create", "my-project")

	rr := httptest.NewRecorder()
	handler.CanI(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

// --- Info endpoint tests ---

func TestAccountHandler_Info_OIDCUser(t *testing.T) {
	t.Parallel()
	// Mock returns only "alpha-developers" as a mapped group (engineering has no policy)
	handler := NewAccountHandler(&mockCanIService{
		mappedGroups: []string{"alpha-developers"},
	})

	userCtx := &middleware.UserContext{
		UserID:         "oidc:user123",
		Email:          "developer@example.com",
		DisplayName:    "Test Developer",
		Groups:         []string{"engineering", "alpha-developers"},
		CasbinRoles:    []string{"proj:alpha:developer"},
		Projects:       []string{"alpha", "beta"},
		Roles:          map[string]string{"alpha": "developer", "beta": "viewer"},
		Issuer:         "https://auth.example.com",
		TokenExpiresAt: 1706016000,
		TokenIssuedAt:  1706012400,
	}

	req := createAccountInfoRequest(t, userCtx)
	rr := httptest.NewRecorder()
	handler.Info(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp AccountInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.UserID != "oidc:user123" {
		t.Errorf("expected userID %q, got %q", "oidc:user123", resp.UserID)
	}
	if resp.Email != "developer@example.com" {
		t.Errorf("expected email %q, got %q", "developer@example.com", resp.Email)
	}
	if resp.DisplayName != "Test Developer" {
		t.Errorf("expected displayName %q, got %q", "Test Developer", resp.DisplayName)
	}
	if resp.Issuer != "https://auth.example.com" {
		t.Errorf("expected issuer %q, got %q", "https://auth.example.com", resp.Issuer)
	}
	// Only the mapped group should be returned (not "engineering")
	if len(resp.Groups) != 1 {
		t.Errorf("expected 1 mapped group, got %d: %v", len(resp.Groups), resp.Groups)
	}
	if len(resp.Groups) > 0 && resp.Groups[0] != "alpha-developers" {
		t.Errorf("expected group %q, got %q", "alpha-developers", resp.Groups[0])
	}
	if len(resp.CasbinRoles) != 1 {
		t.Errorf("expected 1 casbin role, got %d", len(resp.CasbinRoles))
	}
	if len(resp.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(resp.Projects))
	}
	if resp.Roles["alpha"] != "developer" {
		t.Errorf("expected role 'developer' for alpha, got %q", resp.Roles["alpha"])
	}
	if resp.TokenExpiresAt != 1706016000 {
		t.Errorf("expected tokenExpiresAt %d, got %d", 1706016000, resp.TokenExpiresAt)
	}
	if resp.TokenIssuedAt != 1706012400 {
		t.Errorf("expected tokenIssuedAt %d, got %d", 1706012400, resp.TokenIssuedAt)
	}
}

func TestAccountHandler_Info_MappedGroupsFallbackOnError(t *testing.T) {
	t.Parallel()
	// When GetMappedGroups returns an error, the handler should fall back to all groups
	handler := NewAccountHandler(&mockCanIService{
		mappedGroupsErr: context.DeadlineExceeded,
	})

	userCtx := &middleware.UserContext{
		UserID:         "oidc:user123",
		Email:          "developer@example.com",
		DisplayName:    "Test Developer",
		Groups:         []string{"engineering", "alpha-developers"},
		CasbinRoles:    []string{"proj:alpha:viewer"},
		Projects:       []string{"alpha"},
		Roles:          map[string]string{},
		Issuer:         "https://auth.example.com",
		TokenExpiresAt: 1706016000,
		TokenIssuedAt:  1706012400,
	}

	req := createAccountInfoRequest(t, userCtx)
	rr := httptest.NewRecorder()
	handler.Info(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp AccountInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should fall back to all groups when GetMappedGroups errors
	if len(resp.Groups) != 2 {
		t.Errorf("expected 2 groups (fallback), got %d: %v", len(resp.Groups), resp.Groups)
	}
}

func TestAccountHandler_Info_LocalAdmin(t *testing.T) {
	t.Parallel()
	handler := NewAccountHandler(&mockCanIService{})

	userCtx := &middleware.UserContext{
		UserID:         "local:admin",
		Email:          "admin@local",
		DisplayName:    "Local Administrator",
		Groups:         nil,
		CasbinRoles:    []string{"role:serveradmin"},
		Projects:       nil,
		Roles:          nil,
		Issuer:         "",
		TokenExpiresAt: 1706016000,
		TokenIssuedAt:  1706012400,
	}

	req := createAccountInfoRequest(t, userCtx)
	rr := httptest.NewRecorder()
	handler.Info(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp AccountInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Issuer != "Local" {
		t.Errorf("expected issuer %q for local admin, got %q", "Local", resp.Issuer)
	}
	if resp.DisplayName != "Local Administrator" {
		t.Errorf("expected displayName %q, got %q", "Local Administrator", resp.DisplayName)
	}
	// Verify nil slices are returned as empty arrays (not null in JSON)
	if resp.Groups == nil {
		t.Error("expected non-nil groups slice")
	}
	if len(resp.Groups) != 0 {
		t.Errorf("expected 0 groups for local admin, got %d", len(resp.Groups))
	}
	if resp.Projects == nil {
		t.Error("expected non-nil projects slice")
	}
	if len(resp.Projects) != 0 {
		t.Errorf("expected 0 projects for local admin, got %d", len(resp.Projects))
	}
	if resp.Roles == nil {
		t.Error("expected non-nil roles map")
	}
}

func TestAccountHandler_Info_Unauthenticated(t *testing.T) {
	t.Parallel()
	handler := NewAccountHandler(&mockCanIService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/info", nil)
	rr := httptest.NewRecorder()
	handler.Info(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestAccountHandler_Info_UserWithoutIssuer(t *testing.T) {
	t.Parallel()
	handler := NewAccountHandler(&mockCanIService{})

	// User where issuer was not set in context - falls back to "Local"
	// (empty issuer = local admin token, OIDC tokens always have an issuer)
	userCtx := &middleware.UserContext{
		UserID:         "local:admin",
		Email:          "admin@local",
		DisplayName:    "Admin",
		Groups:         nil,
		CasbinRoles:    []string{"role:serveradmin"},
		Projects:       nil,
		Issuer:         "",
		TokenExpiresAt: 1706016000,
		TokenIssuedAt:  1706012400,
	}

	req := createAccountInfoRequest(t, userCtx)
	rr := httptest.NewRecorder()
	handler.Info(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp AccountInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Issuer != "Local" {
		t.Errorf("expected issuer %q for user without explicit issuer, got %q", "Local", resp.Issuer)
	}
}
