package integration

import (
	"net/http"
	"testing"

	"github.com/knodex/knodex/server/internal/rbac"
)

// TestValidationAPI_ValidateCreation_Valid tests validation of a valid project
func TestValidationAPI_ValidateCreation_Valid(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create validation request for a valid project
	reqBody := ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name:        "my-valid-project",
			Description: "A valid project for testing",
			Roles: []RoleToValidate{
				{
					Name:        "admin",
					Description: "Project admin role",
					Policies:    []string{"p, proj:my-valid-project:admin, projects/my-valid-project, *"},
				},
				{
					Name:        "developer",
					Description: "Developer role",
					Policies:    []string{"p, proj:my-valid-project:developer, projects/my-valid-project, get"},
				},
			},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got valid=false with errors: %+v", result.Errors)
	}
}

// TestValidationAPI_ValidateCreation_InvalidName tests validation with invalid project name
func TestValidationAPI_ValidateCreation_InvalidName(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	testCases := []struct {
		name        string
		projectName string
		expectError bool
	}{
		{"empty name", "", true},
		{"uppercase", "MyProject", true},
		{"starts with hyphen", "-invalid", true},
		{"ends with hyphen", "invalid-", true},
		{"special chars", "my_project!", true},
		{"valid name", "my-valid-project", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := ValidateProjectRequestBody{
				Project: &ProjectToValidate{
					Name:        tc.projectName,
					Description: "Test project",
				},
			}

			resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}

			ts.AssertStatus(resp, http.StatusOK)

			result, err := DecodeJSON[ValidationResponse](resp)
			if err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tc.expectError && result.Valid {
				t.Errorf("expected valid=false for name '%s', got valid=true", tc.projectName)
			}
			if !tc.expectError && !result.Valid {
				t.Errorf("expected valid=true for name '%s', got valid=false with errors: %+v", tc.projectName, result.Errors)
			}
		})
	}
}

// TestValidationAPI_ValidateCreation_InvalidPolicy tests validation with invalid policy rules
func TestValidationAPI_ValidateCreation_InvalidPolicy(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	testCases := []struct {
		name        string
		policies    []string
		expectError bool
	}{
		{
			"valid permission policy",
			[]string{"p, proj:test-project:admin, projects/test-project, *"},
			false,
		},
		{
			"missing parts",
			[]string{"p, subject"},
			true,
		},
		{
			"invalid action",
			[]string{"p, proj:test-project:admin, projects/test-project, invalid-action"},
			true,
		},
		{
			"invalid policy type",
			[]string{"x, subject, object, action"},
			true,
		},
		{
			"valid role binding",
			[]string{"g, user123, proj:test-project:admin"},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := ValidateProjectRequestBody{
				Project: &ProjectToValidate{
					Name:        "test-project",
					Description: "Test project",
					Roles: []RoleToValidate{
						{
							Name:     "admin",
							Policies: tc.policies,
						},
					},
				},
			}

			resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}

			ts.AssertStatus(resp, http.StatusOK)

			result, err := DecodeJSON[ValidationResponse](resp)
			if err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tc.expectError && result.Valid {
				t.Errorf("expected valid=false for policies %v, got valid=true", tc.policies)
			}
			if !tc.expectError && !result.Valid {
				t.Errorf("expected valid=true for policies %v, got valid=false with errors: %+v", tc.policies, result.Errors)
			}
		})
	}
}

// TestValidationAPI_ValidateCreation_DuplicateRoles tests validation with duplicate role names
func TestValidationAPI_ValidateCreation_DuplicateRoles(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	reqBody := ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name:        "test-project",
			Description: "Test project",
			Roles: []RoleToValidate{
				{Name: "developer", Description: "Developer role"},
				{Name: "developer", Description: "Duplicate developer role"},
			},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for duplicate role names, got valid=true")
	}

	// Check that there's an error about duplicate roles
	found := false
	for _, e := range result.Errors {
		if e.Severity == "error" && e.Field != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about duplicate role names")
	}
}

// TestValidationAPI_ValidateCreation_EmptyRoleName tests validation with empty role name
func TestValidationAPI_ValidateCreation_EmptyRoleName(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	reqBody := ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name:        "test-project",
			Description: "Test project",
			Roles: []RoleToValidate{
				{Name: "", Description: "Role with empty name"},
			},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for empty role name, got valid=true")
	}
}

// TestValidationAPI_ValidateCreation_MissingProjectField tests validation with missing project field
func TestValidationAPI_ValidateCreation_MissingProjectField(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Request without project field
	reqBody := ValidateProjectRequestBody{
		Project: nil,
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusBadRequest)
}

// TestValidationAPI_ValidateCreation_Unauthorized tests validation without authentication
func TestValidationAPI_ValidateCreation_Unauthorized(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	reqBody := ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name: "test-project",
		},
	}

	// Request without token
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, "")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusUnauthorized)
}

// TestValidationAPI_ValidateUpdate_Valid tests validation of valid project updates
func TestValidationAPI_ValidateUpdate_Valid(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create an existing project
	_, err := ts.CreateProjectDirectly("update-validate-project", "Original description", []rbac.ProjectRole{
		{Name: "admin", Description: "Admin role", Policies: []string{"p, proj:update-validate-project:admin, projects/update-validate-project, *"}},
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:update-validate-project:developer, projects/update-validate-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Validate an update
	newDesc := "Updated description"
	reqBody := ValidateUpdateRequestBody{
		Description: &newDesc,
		Roles: []RoleToValidate{
			{Name: "admin", Description: "Admin role", Policies: []string{"p, proj:update-validate-project:admin, projects/update-validate-project, *"}},
			{Name: "developer", Description: "Updated developer role", Policies: []string{"p, proj:update-validate-project:developer, projects/update-validate-project, get"}},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/update-validate-project/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true for update validation, got valid=false with errors: %+v", result.Errors)
	}
}

// TestValidationAPI_ValidateUpdate_ProjectNotFound tests validation when project doesn't exist
func TestValidationAPI_ValidateUpdate_ProjectNotFound(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	newDesc := "Updated description"
	reqBody := ValidateUpdateRequestBody{
		Description: &newDesc,
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/nonexistent-project/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusNotFound)
}

// TestValidationAPI_ValidateUpdate_RemoveAdminRoleWarning tests warning when removing admin role
func TestValidationAPI_ValidateUpdate_RemoveAdminRoleWarning(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project with admin role
	_, err := ts.CreateProjectDirectly("admin-removal-project", "Project with admin", []rbac.ProjectRole{
		{Name: "admin", Description: "Admin role", Policies: []string{"p, proj:admin-removal-project:admin, projects/admin-removal-project, *"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Validate update that removes admin role
	reqBody := ValidateUpdateRequestBody{
		Roles: []RoleToValidate{
			{Name: "viewer", Description: "Viewer only", Policies: []string{"p, proj:admin-removal-project:viewer, projects/admin-removal-project, get"}},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/admin-removal-project/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have a warning about removing admin role
	hasWarning := false
	for _, e := range result.Errors {
		if e.Severity == "warning" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("expected warning about removing admin role")
	}
}

// TestValidationAPI_ValidateUpdate_InvalidRoles tests validation with invalid role update
func TestValidationAPI_ValidateUpdate_InvalidRoles(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Create project
	_, err := ts.CreateProjectDirectly("invalid-update-project", "Test project", []rbac.ProjectRole{
		{Name: "developer", Description: "Developer role", Policies: []string{"p, proj:invalid-update-project:developer, invalid-update-project, get"}},
	})
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Validate update with invalid roles
	reqBody := ValidateUpdateRequestBody{
		Roles: []RoleToValidate{
			{Name: "", Description: "Role with empty name"},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/invalid-update-project/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for update with empty role name, got valid=true")
	}
}

// TestValidationAPI_ValidateUpdate_Unauthorized tests update validation without authentication
func TestValidationAPI_ValidateUpdate_Unauthorized(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	// Create project first (CreateProjectDirectly doesn't need token, it calls service directly)
	_ = ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail) // Register admin for later use
	_, err := ts.CreateProjectDirectly("unauth-update-project", "Test project", nil)
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// Try to validate update without authentication
	newDesc := "Updated description"
	reqBody := ValidateUpdateRequestBody{
		Description: &newDesc,
	}

	// Need to reset the token to make unauthenticated request
	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/unauth-update-project/validate", reqBody, "")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusUnauthorized)

	// Also test with regular user - validation is allowed for authenticated users
	userToken := ts.AddUser(RegularUserID, RegularUserEmail, []string{}, "")
	resp, err = ts.Request(http.MethodPost, "/api/v1/projects/unauth-update-project/validate", reqBody, userToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	// Regular users can validate - it's a dry-run operation
	ts.AssertStatus(resp, http.StatusOK)
}

// TestValidationAPI_ValidateCreation_PolicyConflictWarning tests warning for duplicate policies
func TestValidationAPI_ValidateCreation_PolicyConflictWarning(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Request with duplicate policies
	reqBody := ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name:        "conflict-project",
			Description: "Project with duplicate policies",
			Roles: []RoleToValidate{
				{
					Name: "admin",
					Policies: []string{
						"p, proj:conflict-project:admin, projects/conflict-project, get",
						"p, proj:conflict-project:admin, projects/conflict-project, get", // Duplicate
					},
				},
			},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have a warning about duplicate policy (but still valid)
	hasWarning := false
	for _, e := range result.Errors {
		if e.Severity == "warning" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("expected warning about duplicate policies")
	}
}

// TestValidationAPI_ValidateCreation_WildcardWarning tests warning for wildcard access to non-admin
func TestValidationAPI_ValidateCreation_WildcardWarning(t *testing.T) {
	ts := NewTestServer(t)
	defer ts.Close()

	adminToken := ts.AddGlobalAdmin(GlobalAdminID, GlobalAdminEmail)

	// Request with wildcard action for non-admin role
	reqBody := ValidateProjectRequestBody{
		Project: &ProjectToValidate{
			Name:        "wildcard-project",
			Description: "Project with wildcard for non-admin",
			Roles: []RoleToValidate{
				{
					Name:     "developer",
					Policies: []string{"p, proj:wildcard-project:developer, projects/wildcard-project, *"},
				},
			},
		},
	}

	resp, err := ts.Request(http.MethodPost, "/api/v1/projects/validate", reqBody, adminToken)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}

	ts.AssertStatus(resp, http.StatusOK)

	result, err := DecodeJSON[ValidationResponse](resp)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have a warning about wildcard for non-admin role
	hasWarning := false
	for _, e := range result.Errors {
		if e.Severity == "warning" {
			hasWarning = true
			break
		}
	}

	if !hasWarning {
		t.Error("expected warning about wildcard action for non-admin role")
	}
}
