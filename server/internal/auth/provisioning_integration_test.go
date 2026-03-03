package auth

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/provops-org/knodex/server/internal/config"
)

// TestEvaluateOIDCUser_FullFlow tests the complete OIDC user evaluation flow
// Updated to use EvaluateOIDCUser instead of ProvisionUser
func TestEvaluateOIDCUser_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test provisioning service
	provSvc, _ := createTestOIDCProvisioningService()

	ctx := context.Background()
	oidcSubject := "test-subject-123"
	email := "testuser@example.com"
	displayName := "Test User"

	// Evaluate OIDC user with groups
	groups := []string{"engineering", "platform-team"}
	result, err := provSvc.EvaluateOIDCUser(ctx, oidcSubject, email, displayName, groups)
	if err != nil {
		t.Fatalf("EvaluateOIDCUser() failed: %v", err)
	}

	// Verify user info was populated
	if result == nil {
		t.Fatal("EvaluateOIDCUser() returned nil result")
	}
	if result.Email != email {
		t.Errorf("User email = %s, want %s", result.Email, email)
	}
	if result.DisplayName != displayName {
		t.Errorf("User display name = %s, want %s", result.DisplayName, displayName)
	}
	if result.Subject != oidcSubject {
		t.Errorf("User OIDC subject = %s, want %s", result.Subject, oidcSubject)
	}

	// Verify user ID was generated
	if result.UserID == "" {
		t.Error("Expected UserID to be generated, got empty string")
	}

	// OIDC users are ephemeral - no default project is assigned
	// Project membership comes from group mappings only
	if len(result.ProjectMemberships) != 0 {
		t.Errorf("Expected no project memberships (no group mappings configured), got %v", result.ProjectMemberships)
	}

	// Verify groups are stored in result
	if len(result.Groups) != len(groups) {
		t.Errorf("Expected %d groups in result, got %d", len(groups), len(result.Groups))
	}
	for i, g := range groups {
		if result.Groups[i] != g {
			t.Errorf("Group[%d] = %s, want %s", i, result.Groups[i], g)
		}
	}
}

// TestEvaluateOIDCUser_GlobalAdminGroup tests global admin detection via group mapping
// Tests that users in global admin groups get role:serveradmin in AssignedRoles
func TestEvaluateOIDCUser_GlobalAdminGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create provisioning service with global admin group mapping
	groupMappings := []config.OIDCGroupMapping{
		{
			Group:       "platform-admins",
			GlobalAdmin: true,
		},
	}
	groupMapper := NewGroupMapper(groupMappings)
	provSvc := createTestOIDCProvisioningServiceWithMapper(groupMapper)

	ctx := context.Background()

	// User in global admin group
	result, err := provSvc.EvaluateOIDCUser(ctx, "admin-subject", "admin@example.com", "Admin User", []string{"platform-admins"})
	if err != nil {
		t.Fatalf("EvaluateOIDCUser() failed: %v", err)
	}

	// Verify user has global admin role via HasGlobalAdminRole() helper
	if !result.HasGlobalAdminRole() {
		t.Error("Expected HasGlobalAdminRole()=true for user in global admin group")
	}

	// Also verify AssignedRoles contains the admin role
	hasAdminInRoles := false
	for _, role := range result.AssignedRoles {
		if role == "role:serveradmin" {
			hasAdminInRoles = true
			break
		}
	}
	if !hasAdminInRoles {
		t.Errorf("Expected AssignedRoles to contain 'role:serveradmin', got %v", result.AssignedRoles)
	}
}

// TestEvaluateOIDCUser_Idempotent tests that evaluation is idempotent
// Since OIDC users are ephemeral, multiple evaluations should produce consistent results
func TestEvaluateOIDCUser_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provSvc, _ := createTestOIDCProvisioningService()
	ctx := context.Background()

	oidcSubject := "existing-user-subject"
	email := "existing@example.com"
	displayName := "Existing User"
	groups := []string{"group-a", "group-b"}

	// First evaluation
	result1, err := provSvc.EvaluateOIDCUser(ctx, oidcSubject, email, displayName, groups)
	if err != nil {
		t.Fatalf("First EvaluateOIDCUser() failed: %v", err)
	}

	// Second evaluation - should return identical results
	result2, err := provSvc.EvaluateOIDCUser(ctx, oidcSubject, email, displayName, groups)
	if err != nil {
		t.Fatalf("Second EvaluateOIDCUser() failed: %v", err)
	}

	// Verify same UserID is generated (deterministic based on subject)
	if result1.UserID != result2.UserID {
		t.Errorf("Different UserIDs generated: %s vs %s", result1.UserID, result2.UserID)
	}

	// Verify other fields match
	if result1.Email != result2.Email {
		t.Errorf("Email mismatch: %s vs %s", result1.Email, result2.Email)
	}
}

// TestEvaluateOIDCUser_ConcurrentEvaluations tests concurrent evaluation handling
// Since OIDC users are ephemeral, concurrent evaluations should all succeed
func TestEvaluateOIDCUser_ConcurrentEvaluations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provSvc, _ := createTestOIDCProvisioningService()
	ctx := context.Background()

	oidcSubject := "concurrent-user-subject"
	email := "concurrent@example.com"
	displayName := "Concurrent User"
	groups := []string{"concurrent-group"}

	// Simulate 5 concurrent evaluations
	const concurrency = 5
	var wg sync.WaitGroup
	results := make([]*OIDCUserInfo, concurrency)
	errs := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = provSvc.EvaluateOIDCUser(ctx, oidcSubject, email, displayName, groups)
		}(i)
	}

	wg.Wait()

	// Verify all succeeded
	for i, err := range errs {
		if err != nil {
			t.Errorf("Concurrent evaluation %d failed: %v", i, err)
		}
	}

	// Verify all returned the same user ID (deterministic)
	var firstUserID string
	for i, result := range results {
		if result == nil {
			t.Fatalf("Concurrent evaluation %d returned nil result", i)
		}
		if i == 0 {
			firstUserID = result.UserID
		} else if result.UserID != firstUserID {
			t.Errorf("Concurrent evaluation %d returned different user ID: %s vs %s", i, result.UserID, firstUserID)
		}
	}
}

// TestEvaluateOIDCUser_InvalidEmail tests error handling for invalid emails
func TestEvaluateOIDCUser_InvalidEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	provSvc, _ := createTestOIDCProvisioningService()
	ctx := context.Background()

	tests := []struct {
		name        string
		email       string
		oidcSubject string
		displayName string
		wantErr     bool
	}{
		{
			name:        "empty email",
			email:       "",
			oidcSubject: "subject-1",
			displayName: "User 1",
			wantErr:     true,
		},
		{
			name:        "email without @",
			email:       "userexample.com",
			oidcSubject: "subject-2",
			displayName: "User 2",
			wantErr:     true,
		},
		{
			name:        "email with @ at start",
			email:       "@example.com",
			oidcSubject: "subject-3",
			displayName: "User 3",
			wantErr:     true,
		},
		{
			name:        "email with only special chars",
			email:       "+++@example.com",
			oidcSubject: "subject-4",
			displayName: "User 4",
			// Note: RFC 5322 allows + in local part, so this is actually valid
			wantErr: false,
		},
		{
			name:        "unicode email (homograph attack)",
			email:       "user@exаmple.com", // Cyrillic 'а'
			oidcSubject: "subject-5",
			displayName: "User 5",
			wantErr:     true,
		},
		{
			name:        "email with script injection",
			email:       "user<script>@example.com",
			oidcSubject: "subject-6",
			displayName: "User 6",
			wantErr:     true,
		},
		{
			name:        "email exceeding max length",
			email:       strings.Repeat("a", 250) + "@example.com",
			oidcSubject: "subject-7",
			displayName: "User 7",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := []string{} // Empty groups for invalid email tests
			result, err := provSvc.EvaluateOIDCUser(ctx, tt.oidcSubject, tt.email, tt.displayName, groups)
			if tt.wantErr && err == nil {
				t.Errorf("EvaluateOIDCUser() expected error for email %q, got nil", tt.email)
			}
			if tt.wantErr && result != nil {
				t.Errorf("EvaluateOIDCUser() expected nil result on error, got %v", result)
			}
		})
	}
}

// TestEvaluateOIDCUser_ProjectGroupMapping tests project membership via group mapping
// Tests that group mappings assign users to projects
func TestEvaluateOIDCUser_ProjectGroupMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create provisioning service with project group mapping
	groupMappings := []config.OIDCGroupMapping{
		{
			Group:   "engineering",
			Project: "engineering-project",
			Role:    "developer",
		},
		{
			Group:   "qa-team",
			Project: "qa-project",
			Role:    "viewer",
		},
	}
	groupMapper := NewGroupMapper(groupMappings)
	provSvc := createTestOIDCProvisioningServiceWithMapper(groupMapper)

	ctx := context.Background()

	// User in engineering group
	result, err := provSvc.EvaluateOIDCUser(ctx, "eng-subject", "engineer@example.com", "Engineer", []string{"engineering"})
	if err != nil {
		t.Fatalf("EvaluateOIDCUser() failed: %v", err)
	}

	// Verify project membership was assigned via group mapping
	// Note: The actual project must exist for the membership to be effective
	// In this test, we're verifying the mapping logic works
	if len(result.ProjectMemberships) != 1 {
		t.Errorf("Expected 1 project membership from group mapping, got %d", len(result.ProjectMemberships))
	}
	if len(result.ProjectMemberships) > 0 {
		if result.ProjectMemberships[0].ProjectID != "engineering-project" {
			t.Errorf("Expected project 'engineering-project', got %s", result.ProjectMemberships[0].ProjectID)
		}
		if result.ProjectMemberships[0].Role != "developer" {
			t.Errorf("Expected role 'developer', got %s", result.ProjectMemberships[0].Role)
		}
	}
}
