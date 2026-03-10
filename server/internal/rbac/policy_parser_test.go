// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"errors"
	"strings"
	"testing"
)

func TestParsePolicyString_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected []string
	}{
		{
			"p, role:admin, projects/*, *, allow",
			[]string{"role:admin", "projects/*", "*", "allow"},
		},
		{
			"proj:engineering:developer, projects/engineering, get, allow",
			[]string{"proj:engineering:developer", "projects/engineering", "get", "allow"},
		},
		{
			"role:viewer, instances/*, get, allow",
			[]string{"role:viewer", "instances/*", "get", "allow"},
		},
		{
			"p, proj:prod:developer, applications/prod/*, delete, deny",
			[]string{"proj:prod:developer", "applications/prod/*", "delete", "deny"},
		},
		// Test wildcard object
		{
			"p, role:admin, *, *, allow",
			[]string{"role:admin", "*", "*", "allow"},
		},
		// Test sync action (ArgoCD-specific)
		{
			"p, proj:default:developer, applications/*, sync, allow",
			[]string{"proj:default:developer", "applications/*", "sync", "allow"},
		},
	}

	for _, tt := range tests {
		result, err := ParsePolicyString(tt.input)
		if err != nil {
			t.Errorf("ParsePolicyString(%s) error: %v", tt.input, err)
			continue
		}

		if len(result) != len(tt.expected) {
			t.Errorf("ParsePolicyString(%s) length = %d, want %d",
				tt.input, len(result), len(tt.expected))
			continue
		}

		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("ParsePolicyString(%s)[%d] = %s, want %s",
					tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestParsePolicyString_InvalidFieldCount(t *testing.T) {
	t.Parallel()

	tests := []string{
		"p, role:admin, projects/*",                    // 2 fields
		"p, role:admin",                                // 1 field
		"p, role:admin, projects/*, get, allow, extra", // 5 fields
		"",                 // Empty
		"p,",               // Just prefix
		"p, single-field",  // 1 field
		"p, first, second", // 2 fields
	}

	for _, input := range tests {
		_, err := ParsePolicyString(input)
		if err == nil {
			t.Errorf("ParsePolicyString(%s) expected error, got nil", input)
			continue
		}
		if !errors.Is(err, ErrInvalidFieldCount) {
			t.Errorf("ParsePolicyString(%s) error should be ErrInvalidFieldCount, got: %v",
				input, err)
		}
	}
}

func TestParsePolicyString_InvalidEffect(t *testing.T) {
	t.Parallel()

	tests := []string{
		"p, role:admin, projects/*, get, permit", // 'permit' instead of 'allow'
		"p, role:admin, projects/*, get, reject", // 'reject' instead of 'deny'
		"p, role:admin, projects/*, get, ALLOW",  // case-sensitive
		"p, role:admin, projects/*, get, Allow",  // case-sensitive
	}

	for _, input := range tests {
		_, err := ParsePolicyString(input)
		if err == nil {
			t.Errorf("ParsePolicyString(%s) expected error, got nil", input)
			continue
		}
		if !errors.Is(err, ErrInvalidEffect) {
			t.Errorf("ParsePolicyString(%s) expected ErrInvalidEffect, got: %v", input, err)
		}
	}
}

func TestParsePolicyString_EmptyFields(t *testing.T) {
	t.Parallel()

	tests := []string{
		"p, , projects/*, get, allow",        // empty subject
		"p, role:admin, , get, allow",        // empty object
		"p, role:admin, projects/*, , allow", // empty action
	}

	for _, input := range tests {
		_, err := ParsePolicyString(input)
		if err == nil {
			t.Errorf("ParsePolicyString(%s) expected error, got nil", input)
			continue
		}
		if !errors.Is(err, ErrEmptyField) {
			t.Errorf("ParsePolicyString(%s) expected ErrEmptyField, got: %v", input, err)
		}
	}
}

func TestParsePolicyString_WhitespaceTrimming(t *testing.T) {
	t.Parallel()

	input := "p,  role:admin  ,  projects/*  ,  get  ,  allow  "
	expected := []string{"role:admin", "projects/*", "get", "allow"}

	result, err := ParsePolicyString(input)
	if err != nil {
		t.Fatalf("ParsePolicyString error: %v", err)
	}

	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("ParsePolicyString()[%d] = '%s', want '%s'",
				i, result[i], expected[i])
		}
	}
}

func TestValidateObjectFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		object string
		valid  bool
	}{
		{"projects/*", true},
		{"projects/engineering", true},
		{"instances/engineering/*", true},
		{"*", true},
		{"projects/eng-*", true},
		{"applications/myproject/*", true},
		{"applications/myproject/myapp", true},
		{"rgds/*", true},
		{"users/user-123", true},
		{"invalid", false}, // No slash, no wildcard
	}

	for _, tt := range tests {
		err := validateObjectFormat(tt.object)
		if tt.valid && err != nil {
			t.Errorf("validateObjectFormat(%s) should be valid, got error: %v",
				tt.object, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateObjectFormat(%s) should be invalid, got nil error",
				tt.object)
		}
	}
}

func TestValidateAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		action string
		valid  bool
	}{
		{"get", true},
		{"create", true},
		{"update", true},
		{"delete", true},
		{"list", true},
		{"sync", true},
		{"*", true},
		{"invalid-action", false},
		{"GET", false},    // case-sensitive
		{"Create", false}, // case-sensitive
		{"read", false},   // not a valid action
		{"write", false},  // not a valid action
	}

	for _, tt := range tests {
		err := validateAction(tt.action)
		if tt.valid && err != nil {
			t.Errorf("validateAction(%s) should be valid, got error: %v",
				tt.action, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateAction(%s) should be invalid, got nil error",
				tt.action)
		}
	}
}

func TestValidatePolicyFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policy []string
		valid  bool
	}{
		{[]string{"role:admin", "projects/*", "get", "allow"}, true},
		{[]string{"proj:test:dev", "instances/*", "*", "deny"}, true},
		{[]string{"role:admin", "projects/*", "get"}, false}, // 3 fields
		{[]string{"role:admin", "projects/*"}, false},        // 2 fields
		{[]string{}, false}, // empty
		{[]string{"role:admin", "projects/*", "get", "allow", "x"}, false}, // 5 fields
	}

	for _, tt := range tests {
		err := ValidatePolicyFormat(tt.policy)
		if tt.valid && err != nil {
			t.Errorf("ValidatePolicyFormat(%v) should be valid, got error: %v",
				tt.policy, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidatePolicyFormat(%v) should be invalid, got nil error",
				tt.policy)
		}
	}
}

func TestParseProjectRole(t *testing.T) {
	t.Parallel()

	role := ProjectRole{
		Name: "developer",
		Policies: []string{
			"p, proj:test:developer, projects/test, *, allow",
			"p, proj:test:developer, instances/test/*, get, allow",
		},
	}

	policies, err := ParseProjectRole("test-project", role)
	if err != nil {
		t.Fatalf("ParseProjectRole error: %v", err)
	}

	if len(policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(policies))
	}

	// Check first policy
	if policies[0].Subject != "proj:test-project:developer" {
		t.Errorf("Policy subject = %s, want proj:test-project:developer",
			policies[0].Subject)
	}
	if policies[0].Object != "projects/test" {
		t.Errorf("Policy object = %s, want projects/test", policies[0].Object)
	}
	if policies[0].Action != "*" {
		t.Errorf("Policy action = %s, want *", policies[0].Action)
	}
	if policies[0].Effect != "allow" {
		t.Errorf("Policy effect = %s, want allow", policies[0].Effect)
	}

	// Check second policy
	if policies[1].Object != "instances/test/*" {
		t.Errorf("Policy object = %s, want instances/test/*", policies[1].Object)
	}
	if policies[1].Action != "get" {
		t.Errorf("Policy action = %s, want get", policies[1].Action)
	}
}

func TestParseProjectRole_InvalidPolicy(t *testing.T) {
	t.Parallel()

	role := ProjectRole{
		Name: "developer",
		Policies: []string{
			"p, invalid-policy-string", // Invalid - not enough fields
		},
	}

	_, err := ParseProjectRole("test-project", role)
	if err == nil {
		t.Error("ParseProjectRole expected error for invalid policy, got nil")
	}
}

func TestNormalizePolicySubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"proj:engineering:developer", "proj:engineering:developer"},
		{"role:admin", "role:admin"},
		{"admin", "role:admin"},
		{"  proj:test:viewer  ", "proj:test:viewer"},
		{"readonly", "role:readonly"},
		{"  readonly  ", "role:readonly"},
	}

	for _, tt := range tests {
		result := NormalizePolicySubject(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizePolicySubject(%s) = %s, want %s",
				tt.input, result, tt.expected)
		}
	}
}

func TestSplitPolicySubject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		typ     string
		project string
		role    string
	}{
		{"proj:engineering:developer", "proj", "engineering", "developer"},
		{"role:admin", "role", "", "admin"},
		{"admin", "role", "", "admin"},
		{"user:john", "user", "", "john"},
		{"group:admins", "group", "", "admins"},
	}

	for _, tt := range tests {
		typ, project, role := SplitPolicySubject(tt.input)
		if typ != tt.typ || project != tt.project || role != tt.role {
			t.Errorf("SplitPolicySubject(%s) = (%s, %s, %s), want (%s, %s, %s)",
				tt.input, typ, project, role, tt.typ, tt.project, tt.role)
		}
	}
}

func TestParsePolicyString_InvalidAction(t *testing.T) {
	t.Parallel()

	tests := []string{
		"p, role:admin, projects/*, invalid-action, allow",
		"p, role:admin, projects/*, READ, allow",    // uppercase
		"p, role:admin, projects/*, write, allow",   // not a valid action
		"p, role:admin, projects/*, execute, allow", // not a valid action
	}

	for _, input := range tests {
		_, err := ParsePolicyString(input)
		if err == nil {
			t.Errorf("ParsePolicyString(%s) expected error for invalid action, got nil", input)
		}
	}
}

func TestParsePolicyString_InvalidObject(t *testing.T) {
	t.Parallel()

	tests := []string{
		"p, role:admin, invalid, get, allow", // no slash, no wildcard
	}

	for _, input := range tests {
		_, err := ParsePolicyString(input)
		if err == nil {
			t.Errorf("ParsePolicyString(%s) expected error for invalid object, got nil", input)
		}
	}
}

func TestParsePolicyString_SpecialCharacters(t *testing.T) {
	t.Parallel()

	// Test policy with special characters in object names
	tests := []struct {
		input    string
		expected []string
	}{
		{
			"p, role:admin, projects/my-project, get, allow",
			[]string{"role:admin", "projects/my-project", "get", "allow"},
		},
		{
			"p, role:admin, projects/my_project, get, allow",
			[]string{"role:admin", "projects/my_project", "get", "allow"},
		},
		{
			"p, role:admin, projects/MyProject123, get, allow",
			[]string{"role:admin", "projects/MyProject123", "get", "allow"},
		},
	}

	for _, tt := range tests {
		result, err := ParsePolicyString(tt.input)
		if err != nil {
			t.Errorf("ParsePolicyString(%s) error: %v", tt.input, err)
			continue
		}

		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("ParsePolicyString(%s)[%d] = %s, want %s",
					tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

// --- Security Tests ---

func TestParsePolicyString_InputSizeValidation(t *testing.T) {
	t.Parallel()

	// SECURITY: Test input size validation to prevent DoS
	// Create a policy string exceeding MaxPolicyStringLength (1024 bytes)
	longSubject := "role:" + strings.Repeat("a", 1100)
	longPolicy := "p, " + longSubject + ", projects/*, get, allow"

	_, err := ParsePolicyString(longPolicy)
	if err == nil {
		t.Error("ParsePolicyString should reject input exceeding MaxPolicyStringLength")
	}
	if !errors.Is(err, ErrInputTooLarge) {
		t.Errorf("ParsePolicyString expected ErrInputTooLarge, got: %v", err)
	}
}

func TestValidateObjectFormat_PathTraversal(t *testing.T) {
	t.Parallel()

	// SECURITY: Test path traversal prevention
	tests := []struct {
		object      string
		shouldError bool
		desc        string
	}{
		{"projects/../etc/passwd", true, "path traversal with .."},
		{"projects/../../root", true, "multiple path traversal"},
		{"../projects/test", true, "leading path traversal"},
		{"projects/test/..", true, "trailing path traversal"},
		{"projects%2e%2e/test", true, "URL-encoded path traversal"},
		{"projects/%2E%2E/test", true, "uppercase URL-encoded traversal"},
		{"projects/test/valid", false, "valid nested path"},
		{"projects/*", false, "valid wildcard path"},
	}

	for _, tt := range tests {
		err := validateObjectFormat(tt.object)
		if tt.shouldError && err == nil {
			t.Errorf("validateObjectFormat(%s) should error for %s", tt.object, tt.desc)
		}
		if !tt.shouldError && err != nil {
			t.Errorf("validateObjectFormat(%s) should pass for %s, got: %v", tt.object, tt.desc, err)
		}
	}
}

func TestValidateObjectFormat_WildcardLimit(t *testing.T) {
	t.Parallel()

	// SECURITY: Test wildcard limit to prevent ReDoS
	// MaxObjectWildcards = 5
	tests := []struct {
		object      string
		shouldError bool
		desc        string
	}{
		{"projects/*/*/*/*/*/*", true, "6 wildcards exceeds limit"},
		{"a/*/b/*/c/*/d/*/e/*/f/*", true, "6 wildcards in complex path"},
		{"projects/*/*/*/*/*", false, "5 wildcards at limit"},
		{"projects/*/*", false, "2 wildcards within limit"},
		{"projects/*", false, "1 wildcard within limit"},
	}

	for _, tt := range tests {
		err := validateObjectFormat(tt.object)
		if tt.shouldError && err == nil {
			t.Errorf("validateObjectFormat(%s) should error for %s", tt.object, tt.desc)
		}
		if !tt.shouldError && err != nil {
			t.Errorf("validateObjectFormat(%s) should pass for %s, got: %v", tt.object, tt.desc, err)
		}
	}
}

func TestValidateObjectFormat_PathDepth(t *testing.T) {
	t.Parallel()

	// SECURITY: Test path depth limit
	// MaxObjectPathDepth = 10
	tests := []struct {
		object      string
		shouldError bool
		desc        string
	}{
		{"a/b/c/d/e/f/g/h/i/j/k", true, "11 segments exceeds limit"},
		{"a/b/c/d/e/f/g/h/i/j/k/l", true, "12 segments exceeds limit"},
		{"a/b/c/d/e/f/g/h/i/j", false, "10 segments at limit"},
		{"projects/org/app", false, "3 segments within limit"},
	}

	for _, tt := range tests {
		err := validateObjectFormat(tt.object)
		if tt.shouldError && err == nil {
			t.Errorf("validateObjectFormat(%s) should error for %s", tt.object, tt.desc)
		}
		if !tt.shouldError && err != nil {
			t.Errorf("validateObjectFormat(%s) should pass for %s, got: %v", tt.object, tt.desc, err)
		}
	}
}

func TestSplitPolicySubjectSafe(t *testing.T) {
	t.Parallel()

	// SECURITY: Test safe subject splitting with validation
	tests := []struct {
		input       string
		wantType    string
		wantProject string
		wantRole    string
		wantErr     bool
		desc        string
	}{
		{"role:admin", "role", "", "admin", false, "valid role"},
		{"user:john", "user", "", "john", false, "valid user"},
		{"group:admins", "group", "", "admins", false, "valid group"},
		{"proj:engineering:developer", "proj", "engineering", "developer", false, "valid project role"},
		{"admin", "role", "", "admin", false, "implicit role"},
		{"", "", "", "", true, "empty subject"},
		{"invalid:type:value", "", "", "", true, "invalid subject type for three-part"},
		{"badtype:value", "", "", "", true, "invalid subject type"},
		{strings.Repeat("a", 300), "", "", "", true, "subject exceeds max length"},
	}

	for _, tt := range tests {
		typ, project, role, err := SplitPolicySubjectSafe(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("SplitPolicySubjectSafe(%s) expected error for %s", tt.input, tt.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("SplitPolicySubjectSafe(%s) unexpected error for %s: %v", tt.input, tt.desc, err)
			continue
		}
		if typ != tt.wantType || project != tt.wantProject || role != tt.wantRole {
			t.Errorf("SplitPolicySubjectSafe(%s) = (%s, %s, %s), want (%s, %s, %s) for %s",
				tt.input, typ, project, role, tt.wantType, tt.wantProject, tt.wantRole, tt.desc)
		}
	}
}

func TestNormalizePolicySubjectSafe(t *testing.T) {
	t.Parallel()

	// SECURITY: Test safe subject normalization with validation
	tests := []struct {
		input    string
		expected string
		wantErr  bool
		desc     string
	}{
		{"admin", "role:admin", false, "adds role prefix"},
		{"role:admin", "role:admin", false, "keeps role prefix"},
		{"user:john", "user:john", false, "keeps user prefix"},
		{"group:admins", "group:admins", false, "keeps group prefix"},
		{"proj:test:dev", "proj:test:dev", false, "keeps proj prefix"},
		{"", "", true, "empty subject"},
		{strings.Repeat("a", 300), "", true, "exceeds max length"},
		{"role:invalid:colon", "", true, "invalid subject with extra colons"},
	}

	for _, tt := range tests {
		result, err := NormalizePolicySubjectSafe(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("NormalizePolicySubjectSafe(%s) expected error for %s", tt.input, tt.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizePolicySubjectSafe(%s) unexpected error for %s: %v", tt.input, tt.desc, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("NormalizePolicySubjectSafe(%s) = %s, want %s for %s",
				tt.input, result, tt.expected, tt.desc)
		}
	}
}

func TestParseProjectRole_PolicyCountLimit(t *testing.T) {
	t.Parallel()

	// SECURITY: Test policy count limit to prevent DoS
	// MaxPoliciesPerRole = 100

	// Create a role with too many policies
	policies := make([]string, 101)
	for i := range policies {
		policies[i] = "p, role:admin, projects/*, get, allow"
	}

	role := ProjectRole{
		Name:     "developer",
		Policies: policies,
	}

	_, err := ParseProjectRole("test-project", role)
	if err == nil {
		t.Error("ParseProjectRole should reject role with too many policies")
	}
	if !errors.Is(err, ErrInputTooLarge) {
		t.Errorf("ParseProjectRole expected ErrInputTooLarge, got: %v", err)
	}
}

func TestParseProjectRole_InvalidProjectName(t *testing.T) {
	t.Parallel()

	// SECURITY: Test project name validation
	role := ProjectRole{
		Name: "developer",
		Policies: []string{
			"p, role:admin, projects/*, get, allow",
		},
	}

	// Project name with colon (invalid)
	_, err := ParseProjectRole("test:project", role)
	if err == nil {
		t.Error("ParseProjectRole should reject project name with colon")
	}

	// Empty project name
	_, err = ParseProjectRole("", role)
	if err == nil {
		t.Error("ParseProjectRole should reject empty project name")
	}
}

func TestParseProjectRole_InvalidRoleName(t *testing.T) {
	t.Parallel()

	// SECURITY: Test role name validation
	role := ProjectRole{
		Name: "dev:ops", // Invalid - contains colon
		Policies: []string{
			"p, role:admin, projects/*, get, allow",
		},
	}

	_, err := ParseProjectRole("test-project", role)
	if err == nil {
		t.Error("ParseProjectRole should reject role name with colon")
	}
}

func TestParsePolicyString_SubjectValidation(t *testing.T) {
	t.Parallel()

	// SECURITY: Test subject format validation
	tests := []struct {
		input   string
		wantErr bool
		desc    string
	}{
		{"p, role:admin, projects/*, get, allow", false, "valid role subject"},
		{"p, user:john, projects/*, get, allow", false, "valid user subject"},
		{"p, group:admins, projects/*, get, allow", false, "valid group subject"},
		{"p, proj:test:dev, projects/*, get, allow", false, "valid project role subject"},
		{"p, invalid:type:value, projects/*, get, allow", true, "invalid subject type for three-part"},
		{"p, badtype:value, projects/*, get, allow", true, "unknown subject type"},
	}

	for _, tt := range tests {
		_, err := ParsePolicyString(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("ParsePolicyString(%s) expected error for %s", tt.input, tt.desc)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ParsePolicyString(%s) unexpected error for %s: %v", tt.input, tt.desc, err)
		}
	}
}

func TestSplitPolicySubjectSafe_InvalidIdentifiers(t *testing.T) {
	t.Parallel()

	// SECURITY: Test invalid subject identifiers in various positions
	// ValidateSubjectIdentifier rejects: colons, newlines, tabs, spaces
	tests := []struct {
		input   string
		wantErr bool
		desc    string
	}{
		// Case 1: implicit role with invalid name (spaces/tabs/newlines)
		{"admin invalid", true, "implicit role with space"},
		{"admin\tinvalid", true, "implicit role with tab"},
		{"admin\ninvalid", true, "implicit role with newline"},

		// Case 2: type:name with invalid/empty identifier
		{"user:john doe", true, "user with space"},
		{"group:", true, "empty group name"},
		{"role:", true, "empty role name"},
		{"user:john\ttab", true, "user with tab"},

		// Case 3: proj:project:role with invalid/empty project or role
		{"proj::developer", true, "empty project name"},
		{"proj:engineering:", true, "empty role name in project"},
		{"proj:bad name:dev", true, "project name with space"},
		{"proj:test:bad role", true, "role name with space"},
	}

	for _, tt := range tests {
		_, _, _, err := SplitPolicySubjectSafe(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("SplitPolicySubjectSafe(%q) expected error for %s", tt.input, tt.desc)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("SplitPolicySubjectSafe(%q) unexpected error for %s: %v", tt.input, tt.desc, err)
		}
	}
}

func TestValidateSubjectFormat_AllBranches(t *testing.T) {
	t.Parallel()

	// SECURITY: Test validateSubjectFormat covering all branches
	tests := []struct {
		input   string
		wantErr bool
		desc    string
	}{
		// Valid cases
		{"role:admin", false, "valid role subject"},
		{"user:john", false, "valid user subject"},
		{"group:developers", false, "valid group subject"},
		{"proj:myproject:developer", false, "valid project role"},
		{"admin", false, "implicit role (no prefix)"},

		// Invalid cases - empty
		{"", true, "empty subject"},

		// Invalid cases - length
		{strings.Repeat("a", 300), true, "exceeds max length"},

		// Invalid cases - type
		{"invalid:admin", true, "invalid subject type"},
		{"unknown:value", true, "unknown subject type"},

		// Invalid cases - three-part without proj
		{"role:project:name", true, "three-part must use proj"},
		{"user:a:b", true, "user cannot be three-part"},

		// Invalid cases - identifier validation
		{"role:", true, "empty role name"},
		{"user:", true, "empty user name"},
		{"proj:test:", true, "empty role in project"},
		{"proj::dev", true, "empty project name"},
	}

	for _, tt := range tests {
		err := validateSubjectFormat(tt.input)
		if tt.wantErr && err == nil {
			t.Errorf("validateSubjectFormat(%q) expected error for %s", tt.input, tt.desc)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("validateSubjectFormat(%q) unexpected error for %s: %v", tt.input, tt.desc, err)
		}
	}
}

func TestNormalizePolicySubjectSafe_AllBranches(t *testing.T) {
	t.Parallel()

	// SECURITY: Test all branches of NormalizePolicySubjectSafe
	tests := []struct {
		input    string
		expected string
		wantErr  bool
		desc     string
	}{
		// Valid with existing prefixes
		{"role:admin", "role:admin", false, "role prefix preserved"},
		{"proj:test:dev", "proj:test:dev", false, "proj prefix preserved"},
		{"user:alice", "user:alice", false, "user prefix preserved"},
		{"group:admins", "group:admins", false, "group prefix preserved"},

		// Valid without prefix - adds role:
		{"admin", "role:admin", false, "adds role prefix to bare name"},
		{"developer", "role:developer", false, "adds role prefix"},

		// Invalid - empty
		{"", "", true, "empty subject"},
		{"  ", "", true, "whitespace only"},

		// Invalid - too long
		{strings.Repeat("x", 300), "", true, "exceeds max length"},

		// Invalid - bad format after prefix
		{"role:invalid:extra:parts", "", true, "role with too many parts"},
		{"user:", "", true, "user with empty name"},
	}

	for _, tt := range tests {
		result, err := NormalizePolicySubjectSafe(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("NormalizePolicySubjectSafe(%q) expected error for %s", tt.input, tt.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizePolicySubjectSafe(%q) unexpected error for %s: %v", tt.input, tt.desc, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("NormalizePolicySubjectSafe(%q) = %q, want %q for %s", tt.input, result, tt.expected, tt.desc)
		}
	}
}
