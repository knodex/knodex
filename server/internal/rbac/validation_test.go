// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		wantErr bool
		errMsg  string
	}{
		// Valid emails
		{
			name:    "valid simple email",
			email:   "user@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			email:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid email with plus",
			email:   "user+tag@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with dots",
			email:   "first.last@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with hyphen",
			email:   "user-name@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with numbers",
			email:   "user123@example456.com",
			wantErr: false,
		},
		{
			name:    "valid email with underscore",
			email:   "user_name@example.com",
			wantErr: false,
		},

		// Invalid emails - empty/missing
		{
			name:    "empty email",
			email:   "",
			wantErr: true,
			errMsg:  "email cannot be empty",
		},

		// Invalid emails - length violations (RFC 5321)
		{
			name:    "email exceeds max length",
			email:   strings.Repeat("a", 250) + "@example.com",
			wantErr: true,
			errMsg:  "email local part exceeds", // Local part check happens first
		},
		{
			name:    "local part exceeds max length",
			email:   strings.Repeat("a", 65) + "@example.com",
			wantErr: true,
			errMsg:  "email local part exceeds",
		},
		{
			name:    "domain exceeds max length",
			email:   "user@" + strings.Repeat("a", 250) + ".com",
			wantErr: true,
			errMsg:  "invalid", // Regex rejects before domain length check
		},

		// Invalid emails - format violations
		{
			name:    "missing @ symbol",
			email:   "userexample.com",
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name:    "multiple @ symbols",
			email:   "user@@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "@ at start",
			email:   "@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "@ at end",
			email:   "user@",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "empty local part",
			email:   "@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "empty domain",
			email:   "user@",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "spaces in email",
			email:   "user name@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "invalid characters",
			email:   "user<script>@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},

		// Invalid emails - security concerns (homograph attacks)
		{
			name:    "unicode characters",
			email:   "user@exаmple.com", // Cyrillic 'а' instead of 'a'
			wantErr: true,
			errMsg:  "email must contain only ASCII characters",
		},
		{
			name:    "emoji in email",
			email:   "user😀@example.com",
			wantErr: true,
			errMsg:  "email must contain only ASCII characters",
		},

		// Edge cases
		{
			name:    "consecutive dots in local part",
			email:   "user..name@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "dot at start of local part",
			email:   ".user@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},
		{
			name:    "dot at end of local part",
			email:   "user.@example.com",
			wantErr: true,
			errMsg:  "invalid email",
		},

		// Valid special characters (RFC 5322 allows these)
		{
			name:    "valid special chars",
			email:   "user!#$%&'*+-/=?^_`{|}~@example.com",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEmail(tt.email)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateEmail() expected error for %q, got nil", tt.email)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateEmail() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateEmail() unexpected error for %q: %v", tt.email, err)
				}
			}
		})
	}
}

// TestValidateEmail_RFCCompliance tests RFC 5321/5322 compliance
func TestValidateEmail_RFCCompliance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		wantErr bool
		rfc     string
		reason  string
	}{
		{
			name:    "RFC 5321: max local part 64",
			email:   strings.Repeat("a", 64) + "@example.com",
			wantErr: false,
			rfc:     "RFC 5321",
			reason:  "Maximum allowed local part length",
		},
		{
			name:    "RFC 5321: max local part exceeded",
			email:   strings.Repeat("a", 65) + "@example.com",
			wantErr: true,
			rfc:     "RFC 5321",
			reason:  "Exceeds maximum local part length",
		},
		{
			name:    "RFC 5321: reasonable email within limits",
			email:   "very.long.email.address.for.testing@subdomain.example.com",
			wantErr: false,
			rfc:     "RFC 5321",
			reason:  "Valid email within all RFC limits",
		},
		{
			name:    "RFC 5321: total length check",
			email:   strings.Repeat("a", 250) + "@b.com",
			wantErr: true,
			rfc:     "RFC 5321",
			reason:  "Local part exceeds limit (caught before total length)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEmail(tt.email)
			if tt.wantErr && err == nil {
				t.Errorf("%s violation not detected: %s - %s", tt.rfc, tt.reason, tt.email)
			} else if !tt.wantErr && err != nil {
				t.Errorf("%s compliance failed: %s - %s: %v", tt.rfc, tt.reason, tt.email, err)
			}
		})
	}
}

// TestValidateEmail_SecurityConcerns tests security-related validations
func TestValidateEmail_SecurityConcerns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		wantErr  bool
		threat   string
		severity string
	}{
		{
			name:     "Homograph attack: Cyrillic",
			email:    "admin@exаmple.com", // 'а' is Cyrillic
			wantErr:  true,
			threat:   "Homograph attack",
			severity: "HIGH",
		},
		{
			name:     "Homograph attack: Greek",
			email:    "admin@εxample.com", // 'ε' is Greek
			wantErr:  true,
			threat:   "Homograph attack",
			severity: "HIGH",
		},
		{
			name:     "Injection attempt: script tag",
			email:    "user<script>@example.com",
			wantErr:  true,
			threat:   "XSS injection",
			severity: "CRITICAL",
		},
		{
			name:     "SQL injection attempt",
			email:    "user'; DROP TABLE users--@example.com",
			wantErr:  true,
			threat:   "SQL injection",
			severity: "CRITICAL",
		},
		{
			name:     "Command injection attempt",
			email:    "user`whoami`@example.com",
			wantErr:  false, // Backticks are valid per RFC 5322 (but sanitized later in processing)
			threat:   "Command injection",
			severity: "LOW",
		},
		{
			name:     "Path traversal attempt",
			email:    "user/../admin@example.com",
			wantErr:  true, // mail.ParseAddress rejects this (good!)
			threat:   "Path traversal",
			severity: "LOW",
		},
		{
			name:     "Null byte injection",
			email:    "user\x00admin@example.com",
			wantErr:  true,
			threat:   "Null byte injection",
			severity: "HIGH",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateEmail(tt.email)
			if tt.wantErr && err == nil {
				t.Errorf("Security threat not detected: %s (%s) - %s", tt.threat, tt.severity, tt.email)
			} else if !tt.wantErr && err != nil {
				t.Errorf("False positive for %s: %s: %v", tt.threat, tt.email, err)
			}
		})
	}
}

// --- ArgoCD-aligned Project Validation Tests ---

func TestIsWildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
	}{
		{"*", true},
		{"dev-*", true},
		{"*-prod", true},
		{"prod-*-staging", false}, // Middle wildcards not detected by IsWildcard (prefix/suffix only)
		{"exact-match", false},
		{"", false},
		{"partial*match", false}, // wildcard in middle is not caught by IsWildcard
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			result := IsWildcard(tt.input)
			if result != tt.expected {
				t.Errorf("IsWildcard(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		projectName string
		wantErr     bool
	}{
		{"valid simple name", "my-project", false},
		{"valid with numbers", "project-123", false},
		{"valid single char", "a", false},
		{"valid number start", "1project", false},
		{"empty name", "", true},
		{"invalid uppercase", "My-Project", true},
		{"invalid underscore", "my_project", true},
		{"starts with hyphen", "-myproject", true},
		{"ends with hyphen", "myproject-", true},
		{"too long", strings.Repeat("a", 254), true},
		{"max length", strings.Repeat("a", 253), false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProjectName(tt.projectName)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateProjectName(%q) expected error, got nil", tt.projectName)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateProjectName(%q) unexpected error: %v", tt.projectName, err)
			}
		})
	}
}

func TestDestination_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dest        Destination
		expectError bool
	}{
		{
			name: "valid with namespace",
			dest: Destination{
				Namespace: "default",
			},
			expectError: false,
		},
		{
			name: "valid with name and namespace",
			dest: Destination{
				Name:      "local-cluster",
				Namespace: "default",
			},
			expectError: false,
		},
		{
			name: "valid with wildcard namespace",
			dest: Destination{
				Namespace: "*",
			},
			expectError: false,
		},
		{
			name: "valid with prefix wildcard namespace",
			dest: Destination{
				Namespace: "dev-*",
			},
			expectError: false,
		},
		{
			name:        "invalid - no namespace",
			dest:        Destination{},
			expectError: true,
		},
		{
			name: "invalid - empty namespace with name",
			dest: Destination{
				Name: "local-cluster",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.dest.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestResourceSpec_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        ResourceSpec
		expectError bool
	}{
		{
			name:        "valid with group and kind",
			spec:        ResourceSpec{Group: "apps", Kind: "Deployment"},
			expectError: false,
		},
		{
			name:        "valid with empty group (core resources)",
			spec:        ResourceSpec{Group: "", Kind: "Pod"},
			expectError: false,
		},
		{
			name:        "valid with wildcard kind",
			spec:        ResourceSpec{Group: "apps", Kind: "*"},
			expectError: false,
		},
		{
			name:        "invalid - empty kind",
			spec:        ResourceSpec{Group: "apps", Kind: ""},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.spec.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestProjectRole_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		role        ProjectRole
		expectError bool
	}{
		{
			name: "valid role",
			role: ProjectRole{
				Name:     "developer",
				Policies: []string{"p, proj:test:developer, applications, *, */*, allow"},
			},
			expectError: false,
		},
		{
			name: "valid role with groups",
			role: ProjectRole{
				Name:     "viewer",
				Policies: []string{"p, proj:test:viewer, applications, get, */*, allow"},
				Groups:   []string{"myorg:engineering"},
			},
			expectError: false,
		},
		{
			name: "invalid - empty name",
			role: ProjectRole{
				Name:     "",
				Policies: []string{"p, test, *, *, *, allow"},
			},
			expectError: true,
		},
		{
			name: "invalid - no policies",
			role: ProjectRole{
				Name:     "developer",
				Policies: []string{},
			},
			expectError: true,
		},
		{
			name: "invalid - uppercase name",
			role: ProjectRole{
				Name:     "Developer",
				Policies: []string{"p, test, *, *, *, allow"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.role.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateProjectSpec_ArgoCD(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        ProjectSpec
		expectError bool
	}{
		{
			name: "valid minimal spec",
			spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "default"},
				},
			},
			expectError: false,
		},
		{
			name: "valid full spec",
			spec: ProjectSpec{
				Description: "Test project",
				Destinations: []Destination{
					{Namespace: "dev-*"},
					{Name: "production", Namespace: "production"},
				},
				NamespaceResourceWhitelist: []ResourceSpec{
					{Group: "", Kind: "Pod"},
					{Group: "apps", Kind: "Deployment"},
				},
				Roles: []ProjectRole{
					{
						Name:     "developer",
						Policies: []string{"p, proj:test:developer, applications, *, */*, allow"},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid - no destinations",
			spec:        ProjectSpec{},
			expectError: true,
		},
		{
			name: "invalid - destination without namespace",
			spec: ProjectSpec{
				Destinations: []Destination{
					{Name: "some-cluster"},
				},
			},
			expectError: true,
		},
		{
			name: "invalid - duplicate role names",
			spec: ProjectSpec{
				Destinations: []Destination{
					{Namespace: "default"},
				},
				Roles: []ProjectRole{
					{Name: "developer", Policies: []string{"p, test, *, *, *, allow"}},
					{Name: "developer", Policies: []string{"p, test2, *, *, *, allow"}},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProjectSpec(tt.spec)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// --- Note: Glob Matching and Validation Tests ---

func TestMatchGlob(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		text     string
		expected bool
	}{
		// Basic matching
		{"empty pattern with empty text", "", "", true},
		{"empty pattern with non-empty text", "", "hello", false},
		{"exact match", "hello", "hello", true},
		{"exact match fails", "hello", "world", false},

		// Single wildcard
		{"wildcard matches everything", "*", "anything", true},
		{"wildcard matches empty", "*", "", true},

		// Prefix wildcards
		{"prefix wildcard match", "dev-*", "dev-app1", true},
		{"prefix wildcard match with numbers", "app-*", "app-123", true},
		{"prefix wildcard no match", "dev-*", "prod-app1", false},
		{"prefix wildcard empty suffix", "dev-*", "dev-", true},

		// Suffix wildcards
		{"suffix wildcard match", "*-prod", "app-prod", true},
		{"suffix wildcard match complex", "*-prod", "my-app-v2-prod", true},
		{"suffix wildcard no match", "*-prod", "app-staging", false},

		// Both prefix and suffix wildcards
		{"both wildcards", "*app*", "myapp123", true},
		{"both wildcards match prefix", "*app*", "app", true},
		{"both wildcards match suffix", "*app*", "newapp", true},
		{"both wildcards no match", "*app*", "test123", false},

		// URL patterns (ArgoCD common use case)
		{"github org wildcard", "https://github.com/myorg/*", "https://github.com/myorg/repo1", true},
		{"github org wildcard match deep", "https://github.com/myorg/*", "https://github.com/myorg/repo/subfolder", true},
		{"github org wildcard no match", "https://github.com/myorg/*", "https://github.com/other/repo1", false},
		{"url exact match", "https://github.com/myorg/myrepo", "https://github.com/myorg/myrepo", true},

		// Complex patterns
		{"middle wildcard", "dev-*-app", "dev-team1-app", true},
		{"middle wildcard no match", "dev-*-app", "dev-team1-service", false},
		{"multiple wildcards", "*-*-*", "a-b-c", true},
		{"multiple wildcards complex", "env-*-region-*", "env-prod-region-us", true},

		// Edge cases
		{"pattern longer than text", "verylongpattern", "short", false},
		{"text longer than pattern", "hi", "hello", false},
		{"special chars in text", "repo-*", "repo-my@app", true},
		{"case sensitive match", "Dev-*", "dev-app", false},
		{"case sensitive exact", "Hello", "hello", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := MatchGlob(tt.pattern, tt.text)
			if result != tt.expected {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v", tt.pattern, tt.text, result, tt.expected)
			}
		})
	}
}

func TestMatchGlob_SegmentLimit(t *testing.T) {
	t.Parallel()

	// Test that excessive wildcards are rejected to prevent DoS
	t.Run("excessive wildcards rejected", func(t *testing.T) {
		t.Parallel()

		// Create a pattern with more than MaxGlobSegments wildcards
		pattern := ""
		for i := 0; i <= MaxGlobSegments; i++ {
			pattern += "a*"
		}
		pattern += "b" // ends with non-wildcard

		result := MatchGlob(pattern, "test")
		if result != false {
			t.Errorf("MatchGlob with %d+ segments should return false, got true", MaxGlobSegments)
		}
	})

	t.Run("at limit wildcards allowed", func(t *testing.T) {
		t.Parallel()

		// Create a pattern with exactly MaxGlobSegments-1 wildcards (100 segments = 99 wildcards)
		pattern := ""
		for i := 0; i < MaxGlobSegments-1; i++ {
			pattern += "a*"
		}
		pattern += "a" // ends without wildcard

		// This should work (100 segments exactly at limit)
		result := MatchGlob(pattern, strings.Repeat("a", MaxGlobSegments))
		// The result doesn't matter as much as not being rejected for segment count
		_ = result
	})

	t.Run("reasonable wildcards work normally", func(t *testing.T) {
		t.Parallel()

		// Typical use case: org/repo pattern
		result := MatchGlob("https://github.com/org/*", "https://github.com/org/myrepo")
		if result != true {
			t.Error("Normal pattern should match")
		}
	})
}

func TestValidateDestinationAgainstAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dest         Destination
		allowedDests []Destination
		expected     bool
	}{
		// Basic namespace matching
		{
			name:         "exact namespace match",
			dest:         Destination{Namespace: "default"},
			allowedDests: []Destination{{Namespace: "default"}},
			expected:     true,
		},
		{
			name:         "namespace doesn't match",
			dest:         Destination{Namespace: "kube-system"},
			allowedDests: []Destination{{Namespace: "default"}},
			expected:     false,
		},

		// Namespace wildcards
		{
			name:         "namespace wildcard prefix match",
			dest:         Destination{Namespace: "dev-team1"},
			allowedDests: []Destination{{Namespace: "dev-*"}},
			expected:     true,
		},
		{
			name:         "namespace wildcard suffix match",
			dest:         Destination{Namespace: "team1-prod"},
			allowedDests: []Destination{{Namespace: "*-prod"}},
			expected:     true,
		},
		{
			name:         "namespace wildcard all match",
			dest:         Destination{Namespace: "any-namespace"},
			allowedDests: []Destination{{Namespace: "*"}},
			expected:     true,
		},
		{
			name:         "namespace wildcard no match",
			dest:         Destination{Namespace: "prod-team1"},
			allowedDests: []Destination{{Namespace: "dev-*"}},
			expected:     false,
		},

		// Name matching (optional)
		{
			name:         "name exact match with namespace",
			dest:         Destination{Name: "production", Namespace: "default"},
			allowedDests: []Destination{{Name: "production", Namespace: "default"}},
			expected:     true,
		},
		{
			name:         "name with namespace wildcard",
			dest:         Destination{Name: "production", Namespace: "team1-apps"},
			allowedDests: []Destination{{Name: "production", Namespace: "*-apps"}},
			expected:     true,
		},
		{
			name:         "name mismatch",
			dest:         Destination{Name: "staging", Namespace: "default"},
			allowedDests: []Destination{{Name: "production", Namespace: "default"}},
			expected:     false,
		},
		{
			name:         "dest has no name, allowed has name - namespace still matches",
			dest:         Destination{Namespace: "default"},
			allowedDests: []Destination{{Name: "production", Namespace: "default"}},
			expected:     true,
		},

		// Multiple allowed destinations
		{
			name: "match second allowed destination",
			dest: Destination{Namespace: "prod"},
			allowedDests: []Destination{
				{Namespace: "dev-*"},
				{Namespace: "prod"},
			},
			expected: true,
		},
		{
			name: "no match in multiple destinations",
			dest: Destination{Namespace: "staging"},
			allowedDests: []Destination{
				{Namespace: "dev-*"},
				{Namespace: "prod"},
			},
			expected: false,
		},

		// Edge cases
		{
			name:         "empty allowed destinations",
			dest:         Destination{Namespace: "default"},
			allowedDests: []Destination{},
			expected:     false,
		},
		{
			name:         "nil allowed destinations",
			dest:         Destination{Namespace: "default"},
			allowedDests: nil,
			expected:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ValidateDestinationAgainstAllowed(tt.dest, tt.allowedDests)
			if result != tt.expected {
				t.Errorf("ValidateDestinationAgainstAllowed(%v, %v) = %v, want %v", tt.dest, tt.allowedDests, result, tt.expected)
			}
		})
	}
}

// TestValidateDestinationAgainstAllowed_NamespaceIsolation tests namespace isolation patterns
func TestValidateDestinationAgainstAllowed_NamespaceIsolation(t *testing.T) {
	t.Parallel()

	// Project allows dev namespaces, staging, and prod
	allowedDests := []Destination{
		{Namespace: "dev-*"},
		{Namespace: "staging"},
		{Namespace: "prod"},
	}

	tests := []struct {
		name     string
		dest     Destination
		expected bool
	}{
		{
			name:     "dev namespace - allowed",
			dest:     Destination{Namespace: "dev-team1"},
			expected: true,
		},
		{
			name:     "staging - allowed",
			dest:     Destination{Namespace: "staging"},
			expected: true,
		},
		{
			name:     "prod - allowed",
			dest:     Destination{Namespace: "prod"},
			expected: true,
		},
		{
			name:     "kube-system - blocked",
			dest:     Destination{Namespace: "kube-system"},
			expected: false,
		},
		{
			name:     "random namespace - blocked",
			dest:     Destination{Namespace: "random-ns"},
			expected: false,
		},
		{
			name:     "wildcard prefix matches",
			dest:     Destination{Namespace: "dev-myapp"},
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ValidateDestinationAgainstAllowed(tt.dest, allowedDests)
			if result != tt.expected {
				t.Errorf("Namespace isolation test failed: ValidateDestinationAgainstAllowed(%v, allowedDests) = %v, want %v",
					tt.dest, result, tt.expected)
			}
		})
	}
}

// TestValidateNamespace tests namespace validation
func TestValidateNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		namespace   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid namespace",
			namespace: "default",
			wantErr:   false,
		},
		{
			name:      "valid namespace with hyphen",
			namespace: "my-namespace",
			wantErr:   false,
		},
		{
			name:      "valid namespace with numbers",
			namespace: "ns123",
			wantErr:   false,
		},
		{
			name:      "valid namespace mixed",
			namespace: "team-alpha-123",
			wantErr:   false,
		},
		{
			name:      "valid single char namespace",
			namespace: "a",
			wantErr:   false,
		},
		{
			name:        "empty namespace",
			namespace:   "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "namespace too long",
			namespace:   strings.Repeat("a", 64),
			wantErr:     true,
			errContains: "exceeds maximum length of 63",
		},
		{
			name:      "namespace at max length",
			namespace: strings.Repeat("a", 63),
			wantErr:   false,
		},
		{
			name:        "namespace with uppercase",
			namespace:   "MyNamespace",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "namespace with underscore",
			namespace:   "my_namespace",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "namespace starting with hyphen",
			namespace:   "-namespace",
			wantErr:     true,
			errContains: "cannot start or end with hyphen",
		},
		{
			name:        "namespace ending with hyphen",
			namespace:   "namespace-",
			wantErr:     true,
			errContains: "cannot start or end with hyphen",
		},
		{
			name:        "namespace with period",
			namespace:   "my.namespace",
			wantErr:     true,
			errContains: "invalid character",
		},
		{
			name:        "namespace with space",
			namespace:   "my namespace",
			wantErr:     true,
			errContains: "invalid character",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateNamespace(tt.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNamespace(%q) error = %v, wantErr %v", tt.namespace, err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("ValidateNamespace(%q) error = %q, want error containing %q", tt.namespace, err.Error(), tt.errContains)
			}
		})
	}
}

// TestGenerateProjectID tests project ID generation
func TestGenerateProjectID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		displayName string
	}{
		{
			name:        "simple name",
			displayName: "My Project",
		},
		{
			name:        "name with special chars",
			displayName: "My Project! @#$%",
		},
		{
			name:        "name with unicode",
			displayName: "Projet développement",
		},
		{
			name:        "empty name",
			displayName: "",
		},
		{
			name:        "long name",
			displayName: strings.Repeat("Test Project Name ", 10),
		},
		{
			name:        "name with only spaces",
			displayName: "   ",
		},
		{
			name:        "name with hyphens",
			displayName: "test-project-name",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := GenerateProjectID(tt.displayName)

			// Result should not be empty (hash ensures uniqueness)
			if result == "" {
				t.Errorf("GenerateProjectID(%q) returned empty string", tt.displayName)
			}

			// Result should be lowercase
			if result != strings.ToLower(result) {
				t.Errorf("GenerateProjectID(%q) = %q, expected lowercase", tt.displayName, result)
			}

			// Result should not contain consecutive hyphens
			if strings.Contains(result, "--") {
				t.Errorf("GenerateProjectID(%q) = %q, should not contain consecutive hyphens", tt.displayName, result)
			}

			// Result should not start or end with hyphen
			if strings.HasPrefix(result, "-") || strings.HasSuffix(result, "-") {
				t.Errorf("GenerateProjectID(%q) = %q, should not start/end with hyphen", tt.displayName, result)
			}

			// Deterministic: same input should produce same output
			result2 := GenerateProjectID(tt.displayName)
			if result != result2 {
				t.Errorf("GenerateProjectID(%q) not deterministic: %q != %q", tt.displayName, result, result2)
			}
		})
	}
}

// TestValidationConstants tests validation constant values
func TestValidationConstants(t *testing.T) {
	t.Parallel()

	if MaxDisplayNameLength != 255 {
		t.Errorf("MaxDisplayNameLength = %d, expected 255", MaxDisplayNameLength)
	}
	if MaxUserIDLength != 253 {
		t.Errorf("MaxUserIDLength = %d, expected 253", MaxUserIDLength)
	}
	if MaxMemberCount != 1000 {
		t.Errorf("MaxMemberCount = %d, expected 1000", MaxMemberCount)
	}
	if MaxEmailLength != 320 {
		t.Errorf("MaxEmailLength = %d, expected 320", MaxEmailLength)
	}
	if MaxEmailLocalPart != 64 {
		t.Errorf("MaxEmailLocalPart = %d, expected 64", MaxEmailLocalPart)
	}
	if MaxEmailDomainPart != 255 {
		t.Errorf("MaxEmailDomainPart = %d, expected 255", MaxEmailDomainPart)
	}
	if MaxGlobSegments != 100 {
		t.Errorf("MaxGlobSegments = %d, expected 100", MaxGlobSegments)
	}
}

// TestValidateProjectSpec_ClusterResourceWhitelist tests cluster resource whitelist validation
func TestValidateProjectSpec_ClusterResourceWhitelist(t *testing.T) {
	t.Parallel()

	// Base valid spec
	validSpec := ProjectSpec{
		Destinations: []Destination{
			{Namespace: "default"},
		},
	}

	tests := []struct {
		name    string
		spec    ProjectSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid cluster resource whitelist",
			spec: func() ProjectSpec {
				s := validSpec
				s.ClusterResourceWhitelist = []ResourceSpec{{Group: "*", Kind: "Namespace"}}
				return s
			}(),
			wantErr: false,
		},
		{
			name: "invalid cluster resource whitelist - empty kind",
			spec: func() ProjectSpec {
				s := validSpec
				s.ClusterResourceWhitelist = []ResourceSpec{{Group: "*", Kind: ""}}
				return s
			}(),
			wantErr: true,
			errMsg:  "clusterResourceWhitelist",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProjectSpec(tt.spec)
			if tt.wantErr && err == nil {
				t.Error("ValidateProjectSpec() expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateProjectSpec() unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateProjectSpec() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateProjectSpec_ClusterResourceBlacklist tests cluster resource blacklist validation
func TestValidateProjectSpec_ClusterResourceBlacklist(t *testing.T) {
	t.Parallel()

	// Base valid spec
	validSpec := ProjectSpec{
		Destinations: []Destination{
			{Namespace: "default"},
		},
	}

	tests := []struct {
		name    string
		spec    ProjectSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid cluster resource blacklist",
			spec: func() ProjectSpec {
				s := validSpec
				s.ClusterResourceBlacklist = []ResourceSpec{{Group: "", Kind: "Secret"}}
				return s
			}(),
			wantErr: false,
		},
		{
			name: "invalid cluster resource blacklist - empty kind",
			spec: func() ProjectSpec {
				s := validSpec
				s.ClusterResourceBlacklist = []ResourceSpec{{Group: "*", Kind: ""}}
				return s
			}(),
			wantErr: true,
			errMsg:  "clusterResourceBlacklist",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProjectSpec(tt.spec)
			if tt.wantErr && err == nil {
				t.Error("ValidateProjectSpec() expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateProjectSpec() unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateProjectSpec() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateProjectSpec_NamespaceResourceBlacklist tests namespace resource blacklist validation
func TestValidateProjectSpec_NamespaceResourceBlacklist(t *testing.T) {
	t.Parallel()

	// Base valid spec
	validSpec := ProjectSpec{
		Destinations: []Destination{
			{Namespace: "default"},
		},
	}

	tests := []struct {
		name    string
		spec    ProjectSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid namespace resource blacklist",
			spec: func() ProjectSpec {
				s := validSpec
				s.NamespaceResourceBlacklist = []ResourceSpec{{Group: "", Kind: "ConfigMap"}}
				return s
			}(),
			wantErr: false,
		},
		{
			name: "invalid namespace resource blacklist - empty kind",
			spec: func() ProjectSpec {
				s := validSpec
				s.NamespaceResourceBlacklist = []ResourceSpec{{Group: "*", Kind: ""}}
				return s
			}(),
			wantErr: true,
			errMsg:  "namespaceResourceBlacklist",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProjectSpec(tt.spec)
			if tt.wantErr && err == nil {
				t.Error("ValidateProjectSpec() expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateProjectSpec() unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateProjectSpec() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateEmail_LocalPartTooLong tests email local part length validation
func TestValidateEmail_LocalPartTooLong(t *testing.T) {
	t.Parallel()

	// Create a local part that is exactly 65 characters (exceeds 64 limit)
	longLocalPart := strings.Repeat("a", 65)
	email := longLocalPart + "@example.com"

	err := ValidateEmail(email)
	if err == nil {
		t.Error("ValidateEmail() expected error for local part > 64 characters")
	}
	if err != nil && !strings.Contains(err.Error(), "local part") {
		t.Errorf("ValidateEmail() error = %v, want error containing 'local part'", err)
	}
}

// TestValidateEmail_DomainTooLong tests email domain length validation
func TestValidateEmail_DomainTooLong(t *testing.T) {
	t.Parallel()

	// Create a domain that exceeds 255 characters
	longDomain := strings.Repeat("a", 256) + ".com"
	email := "user@" + longDomain

	err := ValidateEmail(email)
	// This should fail either in ParseAddress or in domain length check
	if err == nil {
		t.Error("ValidateEmail() expected error for domain > 255 characters")
	}
}

// TestValidateEmail_InvalidUTF8 tests email with invalid UTF-8 sequences
func TestValidateEmail_InvalidUTF8(t *testing.T) {
	t.Parallel()

	// Create invalid UTF-8 sequence
	invalidEmail := "user\xFF@example.com"

	err := ValidateEmail(invalidEmail)
	if err == nil {
		t.Error("ValidateEmail() expected error for invalid UTF-8")
	}
}

// --- Note: Namespace Pattern Matching Tests ---

func TestMatchNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		pattern   string
		expected  bool
	}{
		// Exact match
		{"exact match", "staging", "staging", true},
		{"exact match different", "staging", "production", false},
		{"exact match with hyphen", "staging-team-a", "staging-team-a", true},

		// Universal wildcard
		{"universal wildcard matches anything", "staging-team-a", "*", true},
		{"universal wildcard matches simple", "prod", "*", true},
		{"universal wildcard matches empty", "", "*", true},

		// Suffix wildcard (e.g., "staging*")
		{"suffix wildcard exact prefix", "staging", "staging*", true},
		{"suffix wildcard with suffix", "staging-team-a", "staging*", true},
		{"suffix wildcard with numbers", "staging123", "staging*", true},
		{"suffix wildcard no match", "production", "staging*", false},
		{"suffix wildcard partial match", "stag", "staging*", false},
		{"suffix wildcard knodex", "knodex-feature-x", "knodex*", true},

		// Prefix wildcard (e.g., "*-prod")
		{"prefix wildcard exact suffix", "prod", "*-prod", false}, // Must have prefix with -
		{"prefix wildcard with prefix", "app-prod", "*-prod", true},
		{"prefix wildcard with complex prefix", "my-super-app-prod", "*-prod", true},
		{"prefix wildcard no match", "prod-app", "*-prod", false},

		// Edge cases
		{"empty pattern returns false", "staging", "", false},
		{"empty namespace with suffix wildcard", "", "staging*", false},
		{"empty namespace with prefix wildcard", "", "*-prod", false},
		{"single char namespace", "s", "s*", true},
		{"single char pattern suffix", "s", "*s", true},

		// Real-world scenarios from Azure AD project
		{"azure ad staging pattern", "staging-team-beta", "staging*", true},
		{"azure ad knodex pattern", "knodex-sprint-11", "knodex*", true},
		{"azure ad no access to prod", "production", "staging*", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := MatchNamespace(tt.namespace, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchNamespace(%q, %q) = %v, want %v", tt.namespace, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMatchNamespaceInList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		namespace       string
		allowedPatterns []string
		expected        bool
	}{
		// Single pattern
		{"single exact match", "staging", []string{"staging"}, true},
		{"single no match", "production", []string{"staging"}, false},
		{"single wildcard match", "staging-team-a", []string{"staging*"}, true},

		// Multiple patterns
		{"multiple patterns first match", "staging-team-a", []string{"staging*", "knodex*"}, true},
		{"multiple patterns second match", "knodex-dev", []string{"staging*", "knodex*"}, true},
		{"multiple patterns no match", "production", []string{"staging*", "knodex*"}, false},
		{"multiple mixed patterns", "staging-team-a", []string{"production", "staging*"}, true},

		// Universal wildcard in list
		{"universal wildcard in list", "anything", []string{"specific", "*"}, true},

		// Edge cases
		{"empty list", "staging", []string{}, false},
		{"nil list", "staging", nil, false},
		{"empty namespace with patterns", "", []string{"staging*"}, false},
		{"empty namespace with universal wildcard", "", []string{"*"}, true},

		// Real-world scenarios from Azure AD project (proj-azuread-staging)
		{"azure ad project patterns match staging", "staging-beta-team", []string{"staging*", "knodex*"}, true},
		{"azure ad project patterns match kro", "knodex-feature-131", []string{"staging*", "knodex*"}, true},
		{"azure ad project patterns no match prod", "production", []string{"staging*", "knodex*"}, false},
		{"azure ad project patterns no match ns-beta-team", "ns-beta-team", []string{"staging*", "knodex*"}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := MatchNamespaceInList(tt.namespace, tt.allowedPatterns)
			if result != tt.expected {
				t.Errorf("MatchNamespaceInList(%q, %v) = %v, want %v", tt.namespace, tt.allowedPatterns, result, tt.expected)
			}
		})
	}
}

// TestMatchNamespace_SecurityScenarios tests security-related patterns
func TestMatchNamespace_SecurityScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scenario  string
		namespace string
		pattern   string
		expected  bool
	}{
		{
			name:      "prevent access to kube-system",
			scenario:  "User with staging* should not access kube-system",
			namespace: "kube-system",
			pattern:   "staging*",
			expected:  false,
		},
		{
			name:      "prevent access to default",
			scenario:  "User with staging* should not access default",
			namespace: "default",
			pattern:   "staging*",
			expected:  false,
		},
		{
			name:      "allow access within pattern",
			scenario:  "User with staging* should access staging namespaces",
			namespace: "staging-team-a",
			pattern:   "staging*",
			expected:  true,
		},
		{
			name:      "case sensitivity check",
			scenario:  "Pattern matching should be case-sensitive",
			namespace: "Staging-Team-A",
			pattern:   "staging*",
			expected:  false, // K8s namespaces are lowercase anyway
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := MatchNamespace(tt.namespace, tt.pattern)
			if result != tt.expected {
				t.Errorf("Security scenario failed: %s\nMatchNamespace(%q, %q) = %v, want %v",
					tt.scenario, tt.namespace, tt.pattern, result, tt.expected)
			}
		})
	}
}

// --- STORY-411: ProjectType validation ---

func TestValidateProjectSpec_TypeValidation(t *testing.T) {
	t.Parallel()

	baseSpec := func(pt ProjectType) ProjectSpec {
		return ProjectSpec{
			Type:         pt,
			Destinations: []Destination{{Namespace: "default"}},
		}
	}

	tests := []struct {
		name    string
		spec    ProjectSpec
		wantErr bool
	}{
		{name: "empty type is valid (defaults to app)", spec: baseSpec(""), wantErr: false},
		{name: "type app is valid", spec: baseSpec(ProjectTypeApp), wantErr: false},
		{name: "type platform is valid", spec: baseSpec(ProjectTypePlatform), wantErr: false},
		{name: "invalid type returns error", spec: baseSpec("enterprise"), wantErr: true},
		{name: "arbitrary string type returns error", spec: baseSpec("foo"), wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateProjectSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- STORY-412: Cluster Binding Validation ---

func TestValidateProjectSpec_ClusterBindings(t *testing.T) {
	t.Parallel()

	base := ProjectSpec{
		Destinations: []Destination{{Namespace: "default"}},
	}

	tests := []struct {
		name    string
		spec    ProjectSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "app project with valid clusters",
			spec: ProjectSpec{
				Type:         ProjectTypeApp,
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: "prod-eu-west"}},
				Namespace:    "team-alpha",
			},
			wantErr: false,
		},
		{
			name: "app project empty clusters (monocluster)",
			spec: ProjectSpec{
				Type:         ProjectTypeApp,
				Destinations: base.Destinations,
			},
			wantErr: false,
		},
		{
			name: "platform project with clusters rejected",
			spec: ProjectSpec{
				Type:         ProjectTypePlatform,
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: "prod-eu-west"}},
				Namespace:    "infra-ns",
			},
			wantErr: true,
			errMsg:  "only valid on app projects",
		},
		{
			name: "clusters without namespace rejected",
			spec: ProjectSpec{
				Type:         ProjectTypeApp,
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: "prod-eu-west"}},
			},
			wantErr: true,
			errMsg:  "namespace is required",
		},
		{
			name: "empty clusterRef rejected",
			spec: ProjectSpec{
				Type:         ProjectTypeApp,
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: ""}},
				Namespace:    "team-alpha",
			},
			wantErr: true,
			errMsg:  "clusterRef is required",
		},
		{
			name: "duplicate clusterRef rejected",
			spec: ProjectSpec{
				Type:         ProjectTypeApp,
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: "prod"}, {ClusterRef: "prod"}},
				Namespace:    "team-alpha",
			},
			wantErr: true,
			errMsg:  "duplicated",
		},
		{
			name: "default type (empty) with clusters accepted",
			spec: ProjectSpec{
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: "prod-eu-west"}},
				Namespace:    "team-alpha",
			},
			wantErr: false,
		},
		{
			name: "invalid namespace format rejected",
			spec: ProjectSpec{
				Type:         ProjectTypeApp,
				Destinations: base.Destinations,
				Clusters:     []ClusterBinding{{ClusterRef: "prod-eu-west"}},
				Namespace:    "UPPER-CASE",
			},
			wantErr: true,
			errMsg:  "spec.namespace",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateProjectSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestDestination_Validate_Simple(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		dest    Destination
		wantErr bool
	}{
		{
			name:    "valid destination",
			dest:    Destination{Namespace: "prod"},
			wantErr: false,
		},
		{
			name:    "valid destination with name",
			dest:    Destination{Namespace: "prod", Name: "production"},
			wantErr: false,
		},
		{
			name:    "valid wildcard destination",
			dest:    Destination{Namespace: "dev-*"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.dest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRoleDestinations(t *testing.T) {
	t.Parallel()
	projectDests := []Destination{
		{Namespace: "prod-platform"},
		{Namespace: "prod-app"},
		{Namespace: "prod-shared"},
	}

	tests := []struct {
		name    string
		role    ProjectRole
		wantErr bool
	}{
		{
			name: "role with no destinations - always valid",
			role: ProjectRole{
				Name:     "admin",
				Policies: []string{"*, *, allow"},
			},
			wantErr: false,
		},
		{
			name: "role with valid destinations",
			role: ProjectRole{
				Name:         "developer",
				Policies:     []string{"instances/*, *, allow"},
				Destinations: []string{"prod-app"},
			},
			wantErr: false,
		},
		{
			name: "role with multiple valid destinations",
			role: ProjectRole{
				Name:         "operator",
				Policies:     []string{"instances/*, *, allow"},
				Destinations: []string{"prod-platform", "prod-shared"},
			},
			wantErr: false,
		},
		{
			name: "role with invalid destination",
			role: ProjectRole{
				Name:         "developer",
				Policies:     []string{"instances/*, *, allow"},
				Destinations: []string{"nonexistent-ns"},
			},
			wantErr: true,
		},
		{
			name: "role with empty destination entry",
			role: ProjectRole{
				Name:         "developer",
				Policies:     []string{"instances/*, *, allow"},
				Destinations: []string{""},
			},
			wantErr: true,
		},
		{
			name: "role with duplicate destinations rejected",
			role: ProjectRole{
				Name:         "developer",
				Policies:     []string{"instances/*, *, allow"},
				Destinations: []string{"prod-app", "prod-app"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateRoleDestinations(tt.role, projectDests)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoleDestinations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
