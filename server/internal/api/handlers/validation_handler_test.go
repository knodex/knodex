// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/rbac"
)

// TestNewValidationHandler tests handler construction
func TestNewValidationHandler(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	enforcer := &mockPolicyEnforcer{}

	handler := NewValidationHandler(svc, enforcer)

	if handler == nil {
		t.Fatal("expected handler to be created")
	}
	if handler.projectService != svc {
		t.Error("expected project service to be set")
	}
	if handler.policyEnforcer != enforcer {
		t.Error("expected policy enforcer to be set")
	}
}

// TestValidateProjectCreation_ValidProject tests successful validation
func TestValidateProjectCreation_ValidProject(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name:        "valid-project",
			Description: "A valid project",
			Roles: []RoleToValidate{
				{
					Name:        "admin",
					Description: "Admin role",
					Policies:    []string{"p, admin, projects/valid-project/*, *"},
				},
				{
					Name:        "viewer",
					Description: "Viewer role",
					Policies:    []string{"p, viewer, projects/valid-project/*, get"},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got false. Errors: %+v", result.Errors)
	}
}

// TestValidateProjectCreation_InvalidPolicySyntax tests invalid policy format
func TestValidateProjectCreation_InvalidPolicySyntax(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name:     "broken-role",
					Policies: []string{"invalid policy without proper format"},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d (validation returns 200), got %d", http.StatusOK, resp.StatusCode)
	}

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for invalid policy syntax")
	}

	// Check that error is in the correct field
	found := false
	for _, e := range result.Errors {
		if e.Field == "roles[0].policies[0]" && e.Severity == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error at roles[0].policies[0], got: %+v", result.Errors)
	}
}

// TestValidateProjectCreation_InvalidPolicyType tests invalid policy type (not p or g)
func TestValidateProjectCreation_InvalidPolicyType(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name:     "broken-role",
					Policies: []string{"x, subject, object, action"}, // 'x' is not valid
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for invalid policy type")
	}
}

// TestValidateProjectCreation_InvalidAction tests invalid action in policy
func TestValidateProjectCreation_InvalidAction(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name:     "broken-role",
					Policies: []string{"p, user, projects/test-project/*, invalid-action"},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for invalid action")
	}
}

// TestValidateProjectCreation_InvalidObjectPrefix tests object not starting with projects/
func TestValidateProjectCreation_InvalidObjectPrefix(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name:     "broken-role",
					Policies: []string{"p, user, wrong/prefix/path, get"},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for invalid object prefix")
	}
}

// TestValidateProjectCreation_NonExistentRoleReference tests referencing non-existent role
func TestValidateProjectCreation_NonExistentRoleReference(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name:     "admin",
					Policies: []string{"p, proj:test-project:nonexistent-role, projects/test-project/*, get"},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for non-existent role reference")
	}

	// Check that error mentions role reference
	found := false
	for _, e := range result.Errors {
		if e.Severity == "error" && contains(e.Message, "non-existent role") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about non-existent role reference, got: %+v", result.Errors)
	}
}

// TestValidateProjectCreation_DuplicatePoliciesWarning tests duplicate policies generate warning
func TestValidateProjectCreation_DuplicatePoliciesWarning(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name: "admin",
					Policies: []string{
						"p, admin, projects/test-project/*, get",
						"p, admin, projects/test-project/*, get", // Duplicate
					},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Duplicates are warnings, not errors - so still valid
	if !result.Valid {
		t.Error("expected valid=true (duplicates are warnings)")
	}

	// Check for warning
	found := false
	for _, e := range result.Errors {
		if e.Severity == "warning" && contains(e.Message, "duplicate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about duplicate policy, got: %+v", result.Errors)
	}
}

// TestValidateProjectCreation_InvalidProjectName tests invalid DNS-1123 name
func TestValidateProjectCreation_InvalidProjectName(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name:        "Invalid_Name!", // Invalid characters
			Description: "A project with invalid name",
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for invalid project name")
	}

	// Check error is on name field
	found := false
	for _, e := range result.Errors {
		if e.Field == "name" && e.Severity == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error on 'name' field, got: %+v", result.Errors)
	}
}

// TestValidateProjectCreation_EmptyProjectName tests missing name
func TestValidateProjectCreation_EmptyProjectName(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "",
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for empty project name")
	}
}

// TestValidateProjectCreation_DuplicateRoleNames tests duplicate role names
func TestValidateProjectCreation_DuplicateRoleNames(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{Name: "admin", Policies: []string{"p, admin, projects/test-project/*, *"}},
				{Name: "admin", Policies: []string{"p, admin2, projects/test-project/*, get"}}, // Duplicate name
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for duplicate role names")
	}
}

// TestValidateProjectCreation_MissingProject tests nil project in request
func TestValidateProjectCreation_MissingProject(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: nil,
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestValidateProjectCreation_InvalidJSON tests malformed JSON
func TestValidateProjectCreation_InvalidJSON(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", []byte("invalid json"), userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestValidateProjectCreation_Unauthorized tests missing user context
func TestValidateProjectCreation_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
		},
	}
	body, _ := json.Marshal(reqBody)

	// No user context
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/validate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// TestValidateProjectUpdate_ValidUpdate tests successful update validation
func TestValidateProjectUpdate_ValidUpdate(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("existing-project", rbac.ProjectSpec{
		Description: "Original description",
		Roles: []rbac.ProjectRole{
			{Name: "admin", Policies: []string{"p, admin, projects/existing-project/*, *"}},
		},
	})

	handler := NewValidationHandler(svc, nil)

	reqBody := ValidateProjectUpdateRequest{
		Roles: []RoleToValidate{
			{Name: "admin", Policies: []string{"p, admin, projects/existing-project/*, *"}},
			{Name: "viewer", Policies: []string{"p, viewer, projects/existing-project/*, get"}},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/existing-project/validate", body, userCtx)
	req.SetPathValue("name", "existing-project")
	rec := httptest.NewRecorder()

	handler.ValidateProjectUpdate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got false. Errors: %+v", result.Errors)
	}
}

// TestValidateProjectUpdate_AdminRoleRemovalWarning tests warning when removing admin role
func TestValidateProjectUpdate_AdminRoleRemovalWarning(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("existing-project", rbac.ProjectSpec{
		Roles: []rbac.ProjectRole{
			{Name: "admin", Policies: []string{"p, admin, projects/existing-project/*, *"}},
			{Name: "viewer", Policies: []string{"p, viewer, projects/existing-project/*, get"}},
		},
	})

	handler := NewValidationHandler(svc, nil)

	// Update removes admin role
	reqBody := ValidateProjectUpdateRequest{
		Roles: []RoleToValidate{
			{Name: "viewer", Policies: []string{"p, viewer, projects/existing-project/*, get"}},
			// admin role removed
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/existing-project/validate", body, userCtx)
	req.SetPathValue("name", "existing-project")
	rec := httptest.NewRecorder()

	handler.ValidateProjectUpdate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should be valid (just warning)
	if !result.Valid {
		t.Error("expected valid=true (removing admin is a warning, not error)")
	}

	// Check for warning about admin removal
	found := false
	for _, e := range result.Errors {
		if e.Severity == "warning" && contains(e.Message, "admin") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about removing admin role, got: %+v", result.Errors)
	}
}

// TestValidateProjectUpdate_OrphanedUsersWarning tests warning when removing roles with users
func TestValidateProjectUpdate_OrphanedUsersWarning(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	svc.addProject("existing-project", rbac.ProjectSpec{
		Roles: []rbac.ProjectRole{
			{Name: "admin", Policies: []string{"p, admin, projects/existing-project/*, *"}},
			{Name: "developer", Policies: []string{"p, developer, projects/existing-project/*, create"}},
		},
	})

	handler := NewValidationHandler(svc, nil)

	// Update removes developer role
	reqBody := ValidateProjectUpdateRequest{
		Roles: []RoleToValidate{
			{Name: "admin", Policies: []string{"p, admin, projects/existing-project/*, *"}},
			// developer role removed - may orphan users
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/existing-project/validate", body, userCtx)
	req.SetPathValue("name", "existing-project")
	rec := httptest.NewRecorder()

	handler.ValidateProjectUpdate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check for warning about orphaned users
	found := false
	for _, e := range result.Errors {
		if e.Severity == "warning" && contains(e.Message, "orphan") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about orphaning users, got: %+v", result.Errors)
	}
}

// TestValidateProjectUpdate_ProjectNotFound tests validation for non-existent project
func TestValidateProjectUpdate_ProjectNotFound(t *testing.T) {
	t.Parallel()

	svc := newMockProjectService()
	handler := NewValidationHandler(svc, nil)

	reqBody := ValidateProjectUpdateRequest{}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/nonexistent/validate", body, userCtx)
	req.SetPathValue("name", "nonexistent")
	rec := httptest.NewRecorder()

	handler.ValidateProjectUpdate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

// TestValidateProjectUpdate_MissingName tests missing project name in path
func TestValidateProjectUpdate_MissingName(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectUpdateRequest{}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects//validate", body, userCtx)
	req.SetPathValue("name", "")
	rec := httptest.NewRecorder()

	handler.ValidateProjectUpdate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// TestValidateProjectUpdate_NilProjectService tests handler with nil project service
func TestValidateProjectUpdate_NilProjectService(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(nil, nil)

	reqBody := ValidateProjectUpdateRequest{}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/test/validate", body, userCtx)
	req.SetPathValue("name", "test")
	rec := httptest.NewRecorder()

	handler.ValidateProjectUpdate(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, resp.StatusCode)
	}
}

// TestValidatePolicyRule tests individual policy rule validation
func TestValidatePolicyRule(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(nil, nil)

	tests := []struct {
		name        string
		projectName string
		policy      string
		expectErr   bool
	}{
		{
			name:        "valid permission policy",
			projectName: "my-project",
			policy:      "p, user, projects/my-project/instances, get",
			expectErr:   false,
		},
		{
			name:        "valid permission with wildcard action",
			projectName: "my-project",
			policy:      "p, admin, projects/my-project/*, *",
			expectErr:   false,
		},
		{
			name:        "valid role binding",
			projectName: "my-project",
			policy:      "g, user1, admin",
			expectErr:   false,
		},
		{
			name:        "valid group subject",
			projectName: "my-project",
			policy:      "p, group:developers, projects/my-project/instances, create",
			expectErr:   false,
		},
		{
			name:        "valid proj subject",
			projectName: "my-project",
			policy:      "p, proj:my-project:admin, projects/my-project/*, *",
			expectErr:   false,
		},
		{
			name:        "invalid policy type",
			projectName: "my-project",
			policy:      "x, user, object, action",
			expectErr:   true,
		},
		{
			name:        "invalid action",
			projectName: "my-project",
			policy:      "p, user, projects/my-project/instances, invalid",
			expectErr:   true,
		},
		{
			name:        "wrong object prefix",
			projectName: "my-project",
			policy:      "p, user, other/path, get",
			expectErr:   true,
		},
		{
			name:        "too few parts",
			projectName: "my-project",
			policy:      "p, user",
			expectErr:   true,
		},
		{
			name:        "permission with 3 parts",
			projectName: "my-project",
			policy:      "p, user, object",
			expectErr:   true,
		},
		{
			name:        "wildcard object allowed",
			projectName: "my-project",
			policy:      "p, admin, *, *",
			expectErr:   false,
		},
		{
			name:        "list action valid",
			projectName: "my-project",
			policy:      "p, user, projects/my-project/instances, list",
			expectErr:   false,
		},
		{
			name:        "delete action valid",
			projectName: "my-project",
			policy:      "p, user, projects/my-project/instances, delete",
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := handler.validatePolicyRule(tt.projectName, tt.policy)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for policy %q", tt.policy)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error for policy %q: %v", tt.policy, err)
			}
		})
	}
}

// TestIsValidPolicySubject tests subject validation
func TestIsValidPolicySubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		subject  string
		expected bool
	}{
		{"empty string", "", true},
		{"wildcard", "*", true},
		{"simple user id", "user123", true},
		{"user with email format", "user@example.com", true},
		{"user with hyphen", "user-name", true},
		{"user with underscore", "user_name", true},
		{"user with dot", "user.name", true},
		{"group format", "group:developers", true},
		{"group with slashes", "group:org/team", true},
		{"proj format", "proj:my-project:admin", true},
		{"invalid group empty name", "group:", false},
		{"invalid proj format", "proj:only-one-part", false},
		{"invalid proj empty role", "proj:project:", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isValidPolicySubject(tt.subject)
			if result != tt.expected {
				t.Errorf("isValidPolicySubject(%q) = %v, want %v", tt.subject, result, tt.expected)
			}
		})
	}
}

// TestIsValidGroupName tests group name validation
func TestIsValidGroupName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		group    string
		expected bool
	}{
		{"simple name", "developers", true},
		{"with hyphen", "dev-team", true},
		{"with underscore", "dev_team", true},
		{"with dot", "dev.team", true},
		{"with colon", "org:team", true},
		{"with slash", "org/team", true},
		{"complex path", "org/dept/team-a", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isValidGroupName(tt.group)
			if result != tt.expected {
				t.Errorf("isValidGroupName(%q) = %v, want %v", tt.group, result, tt.expected)
			}
		})
	}
}

// TestExtractRoleFromPolicy tests role extraction from policy strings
func TestExtractRoleFromPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		policy      string
		projectName string
		expected    string
	}{
		{
			name:        "basic role reference",
			policy:      "p, proj:my-project:admin, projects/my-project/*, *",
			projectName: "my-project",
			expected:    "admin",
		},
		{
			name:        "role reference with comma after",
			policy:      "p, proj:my-project:viewer, obj, act",
			projectName: "my-project",
			expected:    "viewer",
		},
		{
			name:        "no role reference",
			policy:      "p, user, projects/my-project/*, get",
			projectName: "my-project",
			expected:    "",
		},
		{
			name:        "different project",
			policy:      "p, proj:other-project:admin, projects/other-project/*, *",
			projectName: "my-project",
			expected:    "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractRoleFromPolicy(tt.policy, tt.projectName)
			if result != tt.expected {
				t.Errorf("extractRoleFromPolicy(%q, %q) = %q, want %q", tt.policy, tt.projectName, result, tt.expected)
			}
		})
	}
}

// TestNormalizePolicy tests policy normalization for duplicate detection
func TestNormalizePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		policy   string
		expected string
	}{
		{
			name:     "already normalized",
			policy:   "p,user,object,action",
			expected: "p,user,object,action",
		},
		{
			name:     "with spaces",
			policy:   "p, user, object, action",
			expected: "p,user,object,action",
		},
		{
			name:     "with extra spaces",
			policy:   "p,  user,  object,  action",
			expected: "p,user,object,action",
		},
		{
			name:     "with leading/trailing spaces",
			policy:   " p, user, object, action ",
			expected: "p,user,object,action",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := normalizePolicy(tt.policy)
			if result != tt.expected {
				t.Errorf("normalizePolicy(%q) = %q, want %q", tt.policy, result, tt.expected)
			}
		})
	}
}

// TestWildcardWarningForNonAdmin tests warning for wildcards on non-admin roles
func TestWildcardWarningForNonAdmin(t *testing.T) {
	t.Parallel()

	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name: "test-project",
			Roles: []RoleToValidate{
				{
					Name:     "viewer",                               // Not admin
					Policies: []string{"p, viewer, projects/*, get"}, // Wildcard on projects
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}
	req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
	rec := httptest.NewRecorder()

	handler.ValidateProjectCreation(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check for warning about wildcard
	found := false
	for _, e := range result.Errors {
		if e.Severity == "warning" && contains(e.Message, "wildcard") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about wildcard for non-admin, got: %+v", result.Errors)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests

func BenchmarkValidateProjectCreation(b *testing.B) {
	handler := NewValidationHandler(newMockProjectService(), nil)

	reqBody := ValidateProjectRequest{
		Project: &ProjectToValidate{
			Name:        "benchmark-project",
			Description: "A project for benchmarking",
			Roles: []RoleToValidate{
				{
					Name: "admin",
					Policies: []string{
						"p, admin, projects/benchmark-project/*, *",
					},
				},
				{
					Name: "developer",
					Policies: []string{
						"p, developer, projects/benchmark-project/instances, get",
						"p, developer, projects/benchmark-project/instances, create",
						"p, developer, projects/benchmark-project/instances, update",
					},
				},
				{
					Name: "viewer",
					Policies: []string{
						"p, viewer, projects/benchmark-project/instances, get",
						"p, viewer, projects/benchmark-project/instances, list",
					},
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)

	userCtx := &middleware.UserContext{
		UserID:      "admin-user",
		CasbinRoles: []string{"role:serveradmin"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := newRequestWithUserContext(http.MethodPost, "/api/v1/projects/validate", body, userCtx)
		rec := httptest.NewRecorder()
		handler.ValidateProjectCreation(rec, req)
	}
}
