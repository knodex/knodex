// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"os"
	"strings"
	"testing"
)

// Note: Password generation tests moved to internal/auth/bootstrap_test.go
// as password management is now handled by bootstrap.GetOrCreateAdminPassword()

func TestLoad_Success(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify default values are set
	if cfg.Server.Address == "" {
		t.Error("expected Server.Address to be set")
	}

	if cfg.Server.Port == 0 {
		t.Error("expected Server.Port to be set")
	}

	// Admin password is now set by bootstrap, not by config.Load()
	// So we expect it to be empty here
	if cfg.Auth.AdminPassword != "" {
		t.Error("expected AdminPassword to be empty (set by bootstrap)")
	}

	if cfg.Auth.AdminPasswordGenerated {
		t.Error("expected AdminPasswordGenerated to be false (set by bootstrap)")
	}
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	// Save original env vars
	originalPort := os.Getenv("SERVER_PORT")
	originalLogLevel := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalPort != "" {
			os.Setenv("SERVER_PORT", originalPort)
		} else {
			os.Unsetenv("SERVER_PORT")
		}
		if originalLogLevel != "" {
			os.Setenv("LOG_LEVEL", originalLogLevel)
		} else {
			os.Unsetenv("LOG_LEVEL")
		}
	}()

	// Set custom values
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify env vars are respected
	if cfg.Server.Port != 9090 {
		t.Errorf("expected Server.Port to be 9090, got %d", cfg.Server.Port)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("expected Log.Level to be 'debug', got %s", cfg.Log.Level)
	}
}

// =============================================================================
// Default Role Validation Tests
// =============================================================================

func TestValidateDefaultRole_EmptyIsValid(t *testing.T) {
	err := ValidateDefaultRole("")
	if err != nil {
		t.Errorf("ValidateDefaultRole() should accept empty string (disabled), got error: %v", err)
	}
}

func TestValidateDefaultRole_ValidRoles(t *testing.T) {
	validRoles := []string{"role:serveradmin"}
	for _, role := range validRoles {
		t.Run(role, func(t *testing.T) {
			err := ValidateDefaultRole(role)
			if err != nil {
				t.Errorf("ValidateDefaultRole() should accept %q, got error: %v", role, err)
			}
		})
	}
}

func TestValidateDefaultRole_InvalidRoles(t *testing.T) {
	invalidRoles := []string{
		"admin",
		"readonly",
		"role:admin",
		"role:readonly",
		"role:superuser",
		"role:developer",
		"role:viewer",
		"ROLE:ADMIN",
		"some-random-role",
	}
	for _, role := range invalidRoles {
		t.Run(role, func(t *testing.T) {
			err := ValidateDefaultRole(role)
			if err == nil {
				t.Errorf("ValidateDefaultRole() should reject %q", role)
			}
			if !strings.Contains(err.Error(), "RBAC_DEFAULT_ROLE") {
				t.Errorf("Error should mention RBAC_DEFAULT_ROLE, got: %v", err)
			}
		})
	}
}

func TestLoad_DefaultRoleDefault(t *testing.T) {
	// Ensure RBAC_DEFAULT_ROLE is truly unset (not empty string) so getEnv returns default.
	// Cannot use t.Setenv here because t.Setenv sets the var (even to ""),
	// and we need the var to be absent from the environment entirely.
	if original, ok := os.LookupEnv("RBAC_DEFAULT_ROLE"); ok {
		t.Cleanup(func() { os.Setenv("RBAC_DEFAULT_ROLE", original) })
	}
	os.Unsetenv("RBAC_DEFAULT_ROLE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Auth.DefaultRole != "" {
		t.Errorf("Expected default role '' (deny-by-default), got %q", cfg.Auth.DefaultRole)
	}
}

func TestLoad_DefaultRoleFromEnv(t *testing.T) {
	t.Setenv("RBAC_DEFAULT_ROLE", "role:serveradmin")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Auth.DefaultRole != "role:serveradmin" {
		t.Errorf("Expected default role 'role:serveradmin', got %q", cfg.Auth.DefaultRole)
	}
}

func TestLoad_DefaultRoleEmptyDisables(t *testing.T) {
	t.Setenv("RBAC_DEFAULT_ROLE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Auth.DefaultRole != "" {
		t.Errorf("Expected empty default role (disabled), got %q", cfg.Auth.DefaultRole)
	}
}

func TestLoad_RejectsInvalidDefaultRole(t *testing.T) {
	t.Setenv("RBAC_DEFAULT_ROLE", "role:superuser")

	_, err := Load()
	if err == nil {
		t.Error("Load() should fail with invalid RBAC_DEFAULT_ROLE")
	}
	if !strings.Contains(err.Error(), "invalid RBAC default role") {
		t.Errorf("Error should mention RBAC default role, got: %v", err)
	}
}

// =============================================================================
// OIDC Group Mappings Tests
// =============================================================================

func TestValidateGroupMappings_EmptyArray(t *testing.T) {
	// Empty array is valid (default state)
	mappings := []OIDCGroupMapping{}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept empty array, got error: %v", err)
	}
}

func TestValidateGroupMappings_ValidProjectMapping(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:   "engineering",
			Project: "eng-project",
			Role:    "developer",
		},
	}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept valid project mapping, got error: %v", err)
	}
}

func TestValidateGroupMappings_ValidGlobalAdminMapping(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:       "kro-admins",
			GlobalAdmin: true,
		},
	}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept valid globalAdmin mapping, got error: %v", err)
	}
}

func TestValidateGroupMappings_AllValidRoles(t *testing.T) {
	roles := []string{"platform-admin", "developer", "viewer"}
	for _, role := range roles {
		mappings := []OIDCGroupMapping{
			{
				Group:   "test-group",
				Project: "test-project",
				Role:    role,
			},
		}
		err := ValidateGroupMappings(mappings)
		if err != nil {
			t.Errorf("ValidateGroupMappings() should accept role %q, got error: %v", role, err)
		}
	}
}

func TestValidateGroupMappings_MultipleValidMappings(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:   "engineering",
			Project: "eng-project",
			Role:    "developer",
		},
		{
			Group:   "platform-team",
			Project: "platform-project",
			Role:    "platform-admin",
		},
		{
			Group:       "super-admins",
			GlobalAdmin: true,
		},
	}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept multiple valid mappings, got error: %v", err)
	}
}

func TestValidateGroupMappings_ErrorEmptyGroupName(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:   "", // Empty group name
			Project: "eng-project",
			Role:    "developer",
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject empty group name")
	}
	if !strings.Contains(err.Error(), "group name is required") {
		t.Errorf("Error message should mention 'group name is required', got: %v", err)
	}
	if !strings.Contains(err.Error(), "groupMappings[0]") {
		t.Errorf("Error message should include mapping index, got: %v", err)
	}
}

func TestValidateGroupMappings_ErrorBothProjectAndGlobalAdmin(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:       "test-group",
			Project:     "test-project",
			Role:        "developer",
			GlobalAdmin: true, // Cannot have both
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject mapping with both project and globalAdmin")
	}
	if !strings.Contains(err.Error(), "cannot set both") {
		t.Errorf("Error message should mention 'cannot set both', got: %v", err)
	}
}

func TestValidateGroupMappings_ErrorRoleAndGlobalAdmin(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:       "test-group",
			Role:        "developer",
			GlobalAdmin: true, // Cannot have both role and globalAdmin
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject mapping with both role and globalAdmin")
	}
	if !strings.Contains(err.Error(), "cannot set both") {
		t.Errorf("Error message should mention 'cannot set both', got: %v", err)
	}
}

func TestValidateGroupMappings_ErrorNeitherProjectNorGlobalAdmin(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group: "test-group",
			// No project, no role, no globalAdmin
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject mapping with neither project nor globalAdmin")
	}
	if !strings.Contains(err.Error(), "must set either") {
		t.Errorf("Error message should mention 'must set either', got: %v", err)
	}
}

func TestValidateGroupMappings_ErrorProjectWithoutRole(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:   "test-group",
			Project: "test-project",
			// Missing role
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject mapping with project but no role")
	}
	if !strings.Contains(err.Error(), "role is required when project is set") {
		t.Errorf("Error message should mention 'role is required', got: %v", err)
	}
}

func TestValidateGroupMappings_ErrorRoleWithoutProject(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group: "test-group",
			Role:  "developer",
			// Missing project
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject mapping with role but no project")
	}
	if !strings.Contains(err.Error(), "project is required when role is set") {
		t.Errorf("Error message should mention 'project is required', got: %v", err)
	}
}

func TestValidateGroupMappings_ErrorInvalidRole(t *testing.T) {
	invalidRoles := []string{"admin", "superuser", "root", "DEVELOPER", "Developer", ""}
	for _, role := range invalidRoles {
		if role == "" {
			continue // Empty role tested separately
		}
		mappings := []OIDCGroupMapping{
			{
				Group:   "test-group",
				Project: "test-project",
				Role:    role,
			},
		}
		err := ValidateGroupMappings(mappings)
		if err == nil {
			t.Errorf("ValidateGroupMappings() should reject invalid role %q", role)
		}
		if !strings.Contains(err.Error(), "invalid role") {
			t.Errorf("Error message should mention 'invalid role', got: %v", err)
		}
		if !strings.Contains(err.Error(), "platform-admin, developer, viewer") {
			t.Errorf("Error message should list valid roles, got: %v", err)
		}
	}
}

func TestValidateGroupMappings_ErrorIndexAndGroupInMessage(t *testing.T) {
	mappings := []OIDCGroupMapping{
		{
			Group:   "valid-group",
			Project: "valid-project",
			Role:    "developer",
		},
		{
			Group:   "second-group",
			Project: "test-project",
			// Missing role - error should indicate index 1
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should return error for invalid mapping")
	}
	if !strings.Contains(err.Error(), "groupMappings[1]") {
		t.Errorf("Error message should include mapping index [1], got: %v", err)
	}
	if !strings.Contains(err.Error(), "second-group") {
		t.Errorf("Error message should include group name, got: %v", err)
	}
}

func TestValidateGroupMappings_GlobalAdminFalseRequiresProjectRole(t *testing.T) {
	// globalAdmin: false is same as not set - should require project+role
	mappings := []OIDCGroupMapping{
		{
			Group:       "test-group",
			GlobalAdmin: false, // Explicitly false
		},
	}
	err := ValidateGroupMappings(mappings)
	if err == nil {
		t.Error("ValidateGroupMappings() should reject mapping with globalAdmin=false and no project/role")
	}
}

func TestValidateGroupMappings_UnicodeGroupName(t *testing.T) {
	// Unicode in group names should be allowed (IdP responsibility)
	mappings := []OIDCGroupMapping{
		{
			Group:   "开发团队", // Chinese characters
			Project: "dev-project",
			Role:    "developer",
		},
		{
			Group:   "équipe-développeurs", // French with accents
			Project: "dev-project",
			Role:    "developer",
		},
	}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept unicode group names, got error: %v", err)
	}
}

func TestValidateGroupMappings_LongGroupName(t *testing.T) {
	// Very long group name should be allowed (IdP responsibility)
	longName := strings.Repeat("a", 1000)
	mappings := []OIDCGroupMapping{
		{
			Group:   longName,
			Project: "test-project",
			Role:    "developer",
		},
	}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept long group names, got error: %v", err)
	}
}

func TestValidateGroupMappings_DuplicateGroupNames(t *testing.T) {
	// Duplicate group names are allowed (all matching mappings apply)
	mappings := []OIDCGroupMapping{
		{
			Group:   "multi-project-team",
			Project: "project-1",
			Role:    "developer",
		},
		{
			Group:   "multi-project-team", // Same group, different project
			Project: "project-2",
			Role:    "viewer",
		},
	}
	err := ValidateGroupMappings(mappings)
	if err != nil {
		t.Errorf("ValidateGroupMappings() should accept duplicate group names, got error: %v", err)
	}
}

// =============================================================================
// OIDC Group Mappings Loading Tests
// =============================================================================

func TestLoadOIDCGroupMappings_EmptyEnvVar(t *testing.T) {
	// Save and restore
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	os.Unsetenv("OIDC_GROUP_MAPPINGS")

	mappings, err := loadOIDCGroupMappings("")
	if err != nil {
		t.Fatalf("loadOIDCGroupMappings() should not error on empty env var: %v", err)
	}
	if len(mappings) != 0 {
		t.Errorf("loadOIDCGroupMappings() should return empty slice, got %d mappings", len(mappings))
	}
}

func TestLoadOIDCGroupMappings_ValidJSON(t *testing.T) {
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	jsonValue := `[{"group":"engineering","project":"eng-project","role":"developer"},{"group":"admins","globalAdmin":true}]`
	os.Setenv("OIDC_GROUP_MAPPINGS", jsonValue)

	mappings, err := loadOIDCGroupMappings("")
	if err != nil {
		t.Fatalf("loadOIDCGroupMappings() failed: %v", err)
	}

	if len(mappings) != 2 {
		t.Fatalf("Expected 2 mappings, got %d", len(mappings))
	}

	// Check first mapping
	if mappings[0].Group != "engineering" {
		t.Errorf("Expected group 'engineering', got %q", mappings[0].Group)
	}
	if mappings[0].Project != "eng-project" {
		t.Errorf("Expected project 'eng-project', got %q", mappings[0].Project)
	}
	if mappings[0].Role != "developer" {
		t.Errorf("Expected role 'developer', got %q", mappings[0].Role)
	}

	// Check second mapping
	if mappings[1].Group != "admins" {
		t.Errorf("Expected group 'admins', got %q", mappings[1].Group)
	}
	if !mappings[1].GlobalAdmin {
		t.Error("Expected globalAdmin to be true")
	}
}

func TestLoadOIDCGroupMappings_InvalidJSON(t *testing.T) {
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	os.Setenv("OIDC_GROUP_MAPPINGS", "not valid json")

	_, err := loadOIDCGroupMappings("")
	if err == nil {
		t.Error("loadOIDCGroupMappings() should error on invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("Error should mention parsing failure, got: %v", err)
	}
}

func TestLoadOIDCGroupMappings_EmptyJSONArray(t *testing.T) {
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	os.Setenv("OIDC_GROUP_MAPPINGS", "[]")

	mappings, err := loadOIDCGroupMappings("")
	if err != nil {
		t.Fatalf("loadOIDCGroupMappings() should not error on empty JSON array: %v", err)
	}
	if len(mappings) != 0 {
		t.Errorf("Expected empty slice, got %d mappings", len(mappings))
	}
}

// =============================================================================
// Integration Test: Load with Group Mappings
// =============================================================================

// =============================================================================
// Organization Identity Tests
// =============================================================================

func TestOrganization_DefaultWhenNotSet(t *testing.T) {
	// Ensure KNODEX_ORGANIZATION is truly unset (not empty string) so getEnv returns default.
	// Cannot use t.Setenv here because t.Setenv sets the var (even to ""),
	// and we need the var to be absent from the environment entirely.
	if original, ok := os.LookupEnv("KNODEX_ORGANIZATION"); ok {
		t.Cleanup(func() { os.Setenv("KNODEX_ORGANIZATION", original) })
	}
	os.Unsetenv("KNODEX_ORGANIZATION")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Organization != "default" {
		t.Errorf("Expected Organization 'default' when env var not set, got %q", cfg.Organization)
	}
}

func TestOrganization_ExplicitValue(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"simple", "orgA"},
		{"with_hyphen", "my-org"},
		{"with_numbers", "org123"},
		{"with_dots", "my.org"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KNODEX_ORGANIZATION", tc.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if cfg.Organization != tc.value {
				t.Errorf("Expected Organization %q, got %q", tc.value, cfg.Organization)
			}
		})
	}
}

func TestOrganization_EmptyStringNormalizesToDefault(t *testing.T) {
	t.Setenv("KNODEX_ORGANIZATION", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Organization != "default" {
		t.Errorf("Expected Organization 'default' for empty string, got %q", cfg.Organization)
	}
}

func TestOrganization_WhitespaceOnlyNormalizesToDefault(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"spaces", "   "},
		{"tabs", "\t\t"},
		{"mixed", " \t \n "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KNODEX_ORGANIZATION", tc.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if cfg.Organization != "default" {
				t.Errorf("Expected Organization 'default' for whitespace-only %q, got %q", tc.value, cfg.Organization)
			}
		})
	}
}

func TestOrganization_TrimsSurroundingWhitespace(t *testing.T) {
	t.Setenv("KNODEX_ORGANIZATION", "  my-org  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Organization != "my-org" {
		t.Errorf("Expected Organization 'my-org' (trimmed), got %q", cfg.Organization)
	}
}

func TestValidateOrganization_TooLong(t *testing.T) {
	longOrg := strings.Repeat("a", MaxOrganizationLength+1)
	err := ValidateOrganization(longOrg)
	if err == nil {
		t.Errorf("ValidateOrganization() should reject %d-char org", len(longOrg))
	}
	if !strings.Contains(err.Error(), "at most") {
		t.Errorf("Error should mention length limit, got: %v", err)
	}
}

func TestValidateOrganization_ExactMaxLength(t *testing.T) {
	org := strings.Repeat("a", MaxOrganizationLength)
	err := ValidateOrganization(org)
	if err != nil {
		t.Errorf("ValidateOrganization() should accept exactly %d-char org, got error: %v", MaxOrganizationLength, err)
	}
}

func TestValidateOrganization_ControlCharacters(t *testing.T) {
	cases := []struct {
		name string
		org  string
	}{
		{"newline", "org\ninjected"},
		{"carriage_return", "org\rinjected"},
		{"tab", "org\tinjected"},
		{"null_byte", "org\x00injected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOrganization(tc.org)
			if err == nil {
				t.Errorf("ValidateOrganization() should reject org with control character")
			}
			if !strings.Contains(err.Error(), "control character") {
				t.Errorf("Error should mention control character, got: %v", err)
			}
		})
	}
}

func TestLoad_RejectsOrganizationWithNewline(t *testing.T) {
	t.Setenv("KNODEX_ORGANIZATION", "org\ninjected")

	_, err := Load()
	if err == nil {
		t.Error("Load() should fail with org containing newline")
	}
	if !strings.Contains(err.Error(), "invalid organization") {
		t.Errorf("Error should mention invalid organization, got: %v", err)
	}
}

func TestLoad_WithValidGroupMappings(t *testing.T) {
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	jsonValue := `[{"group":"eng","project":"engineering","role":"developer"}]`
	os.Setenv("OIDC_GROUP_MAPPINGS", jsonValue)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed with valid group mappings: %v", err)
	}

	if len(cfg.Auth.OIDCGroupMappings) != 1 {
		t.Errorf("Expected 1 group mapping, got %d", len(cfg.Auth.OIDCGroupMappings))
	}
}

func TestLoad_WithInvalidGroupMappings(t *testing.T) {
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	// Invalid: group has project but no role
	jsonValue := `[{"group":"eng","project":"engineering"}]`
	os.Setenv("OIDC_GROUP_MAPPINGS", jsonValue)

	_, err := Load()
	if err == nil {
		t.Error("Load() should fail with invalid group mappings configuration")
	}
	if !strings.Contains(err.Error(), "invalid OIDC group mappings") {
		t.Errorf("Error should mention invalid configuration, got: %v", err)
	}
}

// =============================================================================
// Redis Configuration Tests
// =============================================================================

func TestLoad_RedisDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Redis.Username != "" {
		t.Errorf("Expected empty Redis.Username by default, got %q", cfg.Redis.Username)
	}
	if cfg.Redis.TLSEnabled {
		t.Error("Expected Redis.TLSEnabled to be false by default")
	}
	if cfg.Redis.TLSInsecureSkipVerify {
		t.Error("Expected Redis.TLSInsecureSkipVerify to be false by default")
	}
}

func TestLoad_RedisUsername(t *testing.T) {
	t.Setenv("REDIS_USERNAME", "myuser")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Redis.Username != "myuser" {
		t.Errorf("Expected Redis.Username 'myuser', got %q", cfg.Redis.Username)
	}
}

func TestLoad_RedisTLSEnabled(t *testing.T) {
	t.Setenv("REDIS_TLS_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.Redis.TLSEnabled {
		t.Error("Expected Redis.TLSEnabled to be true")
	}
}

func TestLoad_RedisTLSInsecureSkipVerify(t *testing.T) {
	t.Setenv("REDIS_TLS_INSECURE_SKIP_VERIFY", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !cfg.Redis.TLSInsecureSkipVerify {
		t.Error("Expected Redis.TLSInsecureSkipVerify to be true")
	}
}

func TestLoad_RedisTLSDisabledByDefault(t *testing.T) {
	// Explicitly unset to test default
	if original, ok := os.LookupEnv("REDIS_TLS_ENABLED"); ok {
		t.Cleanup(func() { os.Setenv("REDIS_TLS_ENABLED", original) })
	}
	os.Unsetenv("REDIS_TLS_ENABLED")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Redis.TLSEnabled {
		t.Error("Expected Redis.TLSEnabled to be false when unset")
	}
}

func TestLoad_WithInvalidJSON(t *testing.T) {
	original := os.Getenv("OIDC_GROUP_MAPPINGS")
	defer func() {
		if original != "" {
			os.Setenv("OIDC_GROUP_MAPPINGS", original)
		} else {
			os.Unsetenv("OIDC_GROUP_MAPPINGS")
		}
	}()

	os.Setenv("OIDC_GROUP_MAPPINGS", "invalid json")

	_, err := Load()
	if err == nil {
		t.Error("Load() should fail with invalid JSON in OIDC_GROUP_MAPPINGS")
	}
	if !strings.Contains(err.Error(), "failed to load OIDC group mappings") {
		t.Errorf("Error should mention loading failure, got: %v", err)
	}
}

// =============================================================================
// KnodexNamespace Config Tests
// =============================================================================

func TestLoad_KnodexNamespace_DefaultFallback(t *testing.T) {
	// Clear both env vars to test the fallback default
	origNS := os.Getenv("KNODEX_NAMESPACE")
	origPod := os.Getenv("POD_NAMESPACE")
	os.Unsetenv("KNODEX_NAMESPACE")
	os.Unsetenv("POD_NAMESPACE")
	defer func() {
		if origNS != "" {
			os.Setenv("KNODEX_NAMESPACE", origNS)
		}
		if origPod != "" {
			os.Setenv("POD_NAMESPACE", origPod)
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.KnodexNamespace != "knodex-system" {
		t.Errorf("expected KnodexNamespace to be 'knodex-system' when no env vars set, got %q", cfg.KnodexNamespace)
	}
}

func TestLoad_KnodexNamespace_FromPodNamespace(t *testing.T) {
	origNS := os.Getenv("KNODEX_NAMESPACE")
	origPod := os.Getenv("POD_NAMESPACE")
	os.Unsetenv("KNODEX_NAMESPACE")
	os.Setenv("POD_NAMESPACE", "my-deploy-ns")
	defer func() {
		if origNS != "" {
			os.Setenv("KNODEX_NAMESPACE", origNS)
		} else {
			os.Unsetenv("KNODEX_NAMESPACE")
		}
		if origPod != "" {
			os.Setenv("POD_NAMESPACE", origPod)
		} else {
			os.Unsetenv("POD_NAMESPACE")
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.KnodexNamespace != "my-deploy-ns" {
		t.Errorf("expected KnodexNamespace to be 'my-deploy-ns' from POD_NAMESPACE, got %q", cfg.KnodexNamespace)
	}
}

func TestLoad_KnodexNamespace_ExplicitOverride(t *testing.T) {
	origNS := os.Getenv("KNODEX_NAMESPACE")
	origPod := os.Getenv("POD_NAMESPACE")
	os.Setenv("KNODEX_NAMESPACE", "custom-ns")
	os.Setenv("POD_NAMESPACE", "pod-ns")
	defer func() {
		if origNS != "" {
			os.Setenv("KNODEX_NAMESPACE", origNS)
		} else {
			os.Unsetenv("KNODEX_NAMESPACE")
		}
		if origPod != "" {
			os.Setenv("POD_NAMESPACE", origPod)
		} else {
			os.Unsetenv("POD_NAMESPACE")
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.KnodexNamespace != "custom-ns" {
		t.Errorf("expected KnodexNamespace to be 'custom-ns' from KNODEX_NAMESPACE, got %q", cfg.KnodexNamespace)
	}
}

func TestLoad_CatalogPackageFilter_Unset(t *testing.T) {
	t.Setenv("CATALOG_PACKAGE_FILTER", "")
	os.Unsetenv("CATALOG_PACKAGE_FILTER")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(cfg.CatalogPackageFilter) != 0 {
		t.Errorf("expected empty CatalogPackageFilter when unset, got %v", cfg.CatalogPackageFilter)
	}
}

func TestLoad_CatalogPackageFilter_SingleValue(t *testing.T) {
	t.Setenv("CATALOG_PACKAGE_FILTER", "networking")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(cfg.CatalogPackageFilter) != 1 || cfg.CatalogPackageFilter[0] != "networking" {
		t.Errorf("expected [networking], got %v", cfg.CatalogPackageFilter)
	}
}

func TestLoad_CatalogPackageFilter_MultipleValues(t *testing.T) {
	t.Setenv("CATALOG_PACKAGE_FILTER", "networking,database,platform-core")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := []string{"networking", "database", "platform-core"}
	if len(cfg.CatalogPackageFilter) != len(expected) {
		t.Fatalf("expected %d values, got %d: %v", len(expected), len(cfg.CatalogPackageFilter), cfg.CatalogPackageFilter)
	}
	for i, v := range expected {
		if cfg.CatalogPackageFilter[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, cfg.CatalogPackageFilter[i])
		}
	}
}

func TestLoad_CatalogPackageFilter_NormalizedLowercase(t *testing.T) {
	t.Setenv("CATALOG_PACKAGE_FILTER", "Networking, DATABASE , Platform-Core ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := []string{"networking", "database", "platform-core"}
	if len(cfg.CatalogPackageFilter) != len(expected) {
		t.Fatalf("expected %d values, got %d: %v", len(expected), len(cfg.CatalogPackageFilter), cfg.CatalogPackageFilter)
	}
	for i, v := range expected {
		if cfg.CatalogPackageFilter[i] != v {
			t.Errorf("expected %q at index %d, got %q", v, i, cfg.CatalogPackageFilter[i])
		}
	}
}

func TestLoad_CatalogPackageFilter_EmptyString(t *testing.T) {
	t.Setenv("CATALOG_PACKAGE_FILTER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(cfg.CatalogPackageFilter) != 0 {
		t.Errorf("expected empty CatalogPackageFilter for empty string, got %v", cfg.CatalogPackageFilter)
	}
}

func TestLoad_CatalogPackageFilter_Deduplicated(t *testing.T) {
	t.Setenv("CATALOG_PACKAGE_FILTER", "networking,database,networking,DATABASE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := []string{"networking", "database"}
	if len(cfg.CatalogPackageFilter) != len(expected) {
		t.Fatalf("expected %d deduplicated values, got %d: %v", len(expected), len(cfg.CatalogPackageFilter), cfg.CatalogPackageFilter)
	}
}

func TestLoad_KnodexNamespace_EmptyNormalized(t *testing.T) {
	origNS := os.Getenv("KNODEX_NAMESPACE")
	origPod := os.Getenv("POD_NAMESPACE")
	os.Setenv("KNODEX_NAMESPACE", "  ")
	os.Unsetenv("POD_NAMESPACE")
	defer func() {
		if origNS != "" {
			os.Setenv("KNODEX_NAMESPACE", origNS)
		} else {
			os.Unsetenv("KNODEX_NAMESPACE")
		}
		if origPod != "" {
			os.Setenv("POD_NAMESPACE", origPod)
		} else {
			os.Unsetenv("POD_NAMESPACE")
		}
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.KnodexNamespace != "knodex-system" {
		t.Errorf("expected KnodexNamespace to be 'knodex-system' when set to whitespace, got %q", cfg.KnodexNamespace)
	}
}
