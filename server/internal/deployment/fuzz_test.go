// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

import (
	"testing"
	"unicode/utf8"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// MEDIUM PRIORITY: Fuzzing tests for security-critical validation functions
// These tests help discover edge cases in input validation that could lead to
// security bypasses like path traversal, command injection, or DoS attacks.

// FuzzValidateBranchName fuzzes the branch name validation function
// to find inputs that might bypass security checks
func FuzzValidateBranchName(f *testing.F) {
	// Seed corpus with interesting edge cases
	seeds := []string{
		// Valid branches
		"main",
		"develop",
		"feature/my-feature",
		"release/v1.0.0",
		"hotfix-123",

		// Path traversal attempts
		"../etc/passwd",
		"main/../etc",
		"..%2F..%2Fetc",
		"....//....//etc",

		// Git special characters
		"main@{1}",
		"main~1",
		"main^1",
		"main:ref",

		// Control characters
		"main\x00hidden",
		"main\ninjection",
		"main\rinjection",

		// Unicode edge cases
		"main\u2028line",  // Line separator
		"main\u2029para",  // Paragraph separator
		"mäin",            // Unicode
		"main\u0000null",  // Null byte
		"feature/émoji-🚀", // Emoji

		// Long inputs
		"",
		"a",
		string(make([]byte, 256)), // Just over limit
		string(make([]byte, 1000)),

		// Backslash variants
		"main\\feature",
		"main\\\\feature",

		// Whitespace
		" main",
		"main ",
		"main feature",
		"\tmain",
		"main\t",

		// Special patterns
		"refs/heads/main",
		"HEAD",
		"FETCH_HEAD",
		"-",
		"--",
		"-main",
		"main-",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Call the validation function
		err := ValidateBranchName(input)

		// If validation passes, verify the input is actually safe
		if err == nil {
			// Should not be empty
			if input == "" {
				t.Errorf("ValidateBranchName accepted empty string")
			}

			// Should not contain path traversal
			if containsPathTraversal(input) {
				t.Errorf("ValidateBranchName accepted path traversal: %q", input)
			}

			// Should not contain dangerous git characters
			dangerousChars := []string{"@{", "~", "^", ":", "\\"}
			for _, dc := range dangerousChars {
				if contains(input, dc) {
					t.Errorf("ValidateBranchName accepted dangerous char %q in: %q", dc, input)
				}
			}

			// Should not contain control characters
			for _, r := range input {
				if r < 32 || r == 127 {
					t.Errorf("ValidateBranchName accepted control character in: %q", input)
				}
			}

			// Should not contain spaces
			if contains(input, " ") {
				t.Errorf("ValidateBranchName accepted space in: %q", input)
			}
		}
	})
}

// FuzzValidateBasePath fuzzes the base path validation function
// to find inputs that might bypass security checks
func FuzzValidateBasePath(f *testing.F) {
	// Seed corpus with interesting edge cases
	seeds := []string{
		// Valid paths
		"instances",
		"manifests/instances",
		"deploy/k8s/instances",

		// Path traversal attempts
		"../etc",
		"..\\etc",
		"instances/../etc",
		"instances/..%2Fetc",
		"....//....//etc",
		"%2e%2e%2f",
		"instances/\x00/../etc",

		// Absolute paths
		"/etc/passwd",
		"\\etc\\passwd",
		"C:\\Windows",

		// Backslash variants
		"instances\\subdir",
		"instances\\\\subdir",

		// Unicode tricks
		"instances\u2044subdir", // Fraction slash
		"instances\uFF0Fsubdir", // Fullwidth slash
		"instances\u2215subdir", // Division slash

		// Edge cases
		"",
		"a",
		string(make([]byte, 256)),
		string(make([]byte, 1000)),
		".",
		"..",
		"...",
		"./instances",
		"instances/./subdir",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		err := ValidateBasePath(input)

		// If validation passes, verify the input is actually safe
		if err == nil {
			// Empty is allowed (will use default)
			if input == "" {
				return
			}

			// Should not contain path traversal
			if containsPathTraversal(input) {
				t.Errorf("ValidateBasePath accepted path traversal: %q", input)
			}

			// Should not be absolute
			if isAbsolutePath(input) {
				t.Errorf("ValidateBasePath accepted absolute path: %q", input)
			}

			// Should not contain backslashes
			if contains(input, "\\") {
				t.Errorf("ValidateBasePath accepted backslash in: %q", input)
			}
		}
	})
}

// FuzzSanitizePathComponent fuzzes the path component sanitization function
func FuzzSanitizePathComponent(f *testing.F) {
	// Seed corpus
	seeds := []string{
		"normal-name",
		"with-underscore",
		"mixedcase123",
		"traversal",
		"windows",
		"absolute-path",
		"name-colon",
		"name-star",
		"name-question",
		"name-less",
		"name-greater",
		"name-pipe",
		"name-quote",
		"a",
		"spaces",
		"tabs",
		"emoji-name",
		"unicode-name",
		"test-component",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result, err := sanitize.PathComponent(input)

		// If function succeeds, verify the result is safe
		if err == nil {
			// Result should never contain path traversal
			if containsPathTraversal(result) {
				t.Errorf("sanitize.PathComponent returned path traversal: input=%q result=%q", input, result)
			}

			// Result should never be empty after successful sanitization
			if result == "" {
				t.Errorf("sanitize.PathComponent returned empty string for input: %q", input)
			}

			// Result should be valid UTF-8
			if !utf8.ValidString(result) {
				t.Errorf("sanitize.PathComponent returned invalid UTF-8: input=%q result=%q", input, result)
			}

			// Result should not contain path separators
			if containsByte(result, '/') || containsByte(result, '\\') {
				t.Errorf("sanitize.PathComponent returned path separator: input=%q result=%q", input, result)
			}
		}

		// If function fails, verify error is returned for invalid inputs
		if err != nil {
			// This is expected for empty inputs or invalid characters
			// The function is correctly rejecting dangerous inputs
		}
	})
}

// FuzzRepositoryConfigValidate fuzzes the repository config validation
func FuzzRepositoryConfigValidate(f *testing.F) {
	// We use a simplified approach: fuzz each field individually
	seeds := []string{
		"valid-owner",
		"valid-repo",
		"main",
		"instances",
		"secret-name",
		"kro-system",
		"token",
		"-invalid",
		"invalid-",
		"../traversal",
		string(make([]byte, 100)),
		"",
		"a",
		"with spaces",
		"special!@#$%",
	}

	for _, seed := range seeds {
		f.Add(seed, seed, seed, seed)
	}

	f.Fuzz(func(t *testing.T, owner, repo, branch, basePath string) {
		config := &RepositoryConfig{
			Owner:         owner,
			Repo:          repo,
			DefaultBranch: branch,
			BasePath:      basePath,
			SecretName:    "valid-secret", // Use valid secret to test other fields
		}

		err := config.Validate()

		// If validation passes, verify the config is safe
		if err == nil {
			// Owner should be valid
			if owner == "" {
				t.Errorf("Validate accepted empty owner")
			}
			if startsWith(owner, "-") || endsWith(owner, "-") {
				t.Errorf("Validate accepted owner starting/ending with hyphen: %q", owner)
			}

			// Repo should be valid
			if repo == "" {
				t.Errorf("Validate accepted empty repo")
			}

			// Branch (if set) should not contain path traversal
			if branch != "" && containsPathTraversal(branch) {
				t.Errorf("Validate accepted path traversal in branch: %q", branch)
			}

			// BasePath (if set) should not contain path traversal
			if basePath != "" && containsPathTraversal(basePath) {
				t.Errorf("Validate accepted path traversal in basePath: %q", basePath)
			}
		}
	})
}

// Helper functions for fuzzing assertions

func containsPathTraversal(s string) bool {
	return contains(s, "..")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

func isAbsolutePath(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] == '/' || s[0] == '\\'
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
