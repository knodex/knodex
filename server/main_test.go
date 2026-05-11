// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"os"
	"testing"
)

// TestShouldMigrateOnly verifies the precedence rules for migrate-only mode:
// env var wins; CLI flag is the fallback; absence of both returns false.
// Empty-string env var is treated as falsy (strconv.ParseBool error path).
func TestShouldMigrateOnly(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		envSet bool
		argv   []string
		want   bool
	}{
		{
			name: "no flag, no env",
			argv: []string{"knodex-server"},
			want: false,
		},
		{
			name: "CLI flag set",
			argv: []string{"knodex-server", "--migrate-only"},
			want: true,
		},
		{
			name:   "env var true wins over absent flag",
			envVal: "true",
			envSet: true,
			argv:   []string{"knodex-server"},
			want:   true,
		},
		{
			name:   "env var false beats CLI flag (env takes precedence)",
			envVal: "false",
			envSet: true,
			argv:   []string{"knodex-server", "--migrate-only"},
			want:   false,
		},
		{
			name:   "empty env var is falsy",
			envVal: "",
			envSet: true,
			argv:   []string{"knodex-server"},
			want:   false,
		},
		{
			name:   "env var '1' is truthy",
			envVal: "1",
			envSet: true,
			argv:   []string{"knodex-server"},
			want:   true,
		},
		{
			name:   "env var 'garbage' is falsy (ParseBool error path)",
			envVal: "not-a-bool",
			envSet: true,
			argv:   []string{"knodex-server"},
			want:   false,
		},
		{
			name: "unrelated CLI args do not trip flag parsing",
			argv: []string{"knodex-server", "--unknown=1", "positional"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origArgs := os.Args
			defer func() { os.Args = origArgs }()
			os.Args = tc.argv

			if tc.envSet {
				t.Setenv("KNODEX_MIGRATE_ONLY", tc.envVal)
			} else {
				// Explicitly unset (t.Setenv only sets — Go's testing helper
				// will restore parent env, but we want to ensure no inherited
				// value leaks across subtests).
				_ = os.Unsetenv("KNODEX_MIGRATE_ONLY")
			}

			if got := shouldMigrateOnly(); got != tc.want {
				t.Errorf("shouldMigrateOnly() = %v, want %v", got, tc.want)
			}
		})
	}
}
