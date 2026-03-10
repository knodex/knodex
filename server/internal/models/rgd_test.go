// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

import (
	"testing"
)

func TestParseDeploymentModes(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		want       []string
	}{
		{
			name:       "empty string returns nil",
			annotation: "",
			want:       nil,
		},
		{
			name:       "whitespace only returns nil",
			annotation: "   ",
			want:       nil,
		},
		{
			name:       "single mode - direct",
			annotation: "direct",
			want:       []string{"direct"},
		},
		{
			name:       "single mode - gitops",
			annotation: "gitops",
			want:       []string{"gitops"},
		},
		{
			name:       "single mode - hybrid",
			annotation: "hybrid",
			want:       []string{"hybrid"},
		},
		{
			name:       "two modes - direct,gitops",
			annotation: "direct,gitops",
			want:       []string{"direct", "gitops"},
		},
		{
			name:       "two modes - gitops,hybrid",
			annotation: "gitops,hybrid",
			want:       []string{"gitops", "hybrid"},
		},
		{
			name:       "all three modes",
			annotation: "direct,gitops,hybrid",
			want:       []string{"direct", "gitops", "hybrid"},
		},
		{
			name:       "case insensitive - uppercase",
			annotation: "DIRECT,GITOPS,HYBRID",
			want:       []string{"direct", "gitops", "hybrid"},
		},
		{
			name:       "case insensitive - mixed case",
			annotation: "Direct,GitOps,Hybrid",
			want:       []string{"direct", "gitops", "hybrid"},
		},
		{
			name:       "with whitespace",
			annotation: " direct , gitops , hybrid ",
			want:       []string{"direct", "gitops", "hybrid"},
		},
		{
			name:       "invalid mode ignored",
			annotation: "direct,invalid,gitops",
			want:       []string{"direct", "gitops"},
		},
		{
			name:       "all invalid modes returns nil",
			annotation: "foo,bar,baz",
			want:       nil,
		},
		{
			name:       "duplicates removed",
			annotation: "direct,direct,gitops",
			want:       []string{"direct", "gitops"},
		},
		{
			name:       "empty parts ignored",
			annotation: "direct,,gitops",
			want:       []string{"direct", "gitops"},
		},
		{
			name:       "trailing comma",
			annotation: "direct,gitops,",
			want:       []string{"direct", "gitops"},
		},
		{
			name:       "leading comma",
			annotation: ",direct,gitops",
			want:       []string{"direct", "gitops"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDeploymentModes(tt.annotation)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("ParseDeploymentModes(%q) = %v, want %v", tt.annotation, got, tt.want)
			}
		})
	}
}

func TestIsDeploymentModeAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedModes []string
		mode         string
		want         bool
	}{
		{
			name:         "nil allowed modes - all allowed (direct)",
			allowedModes: nil,
			mode:         "direct",
			want:         true,
		},
		{
			name:         "nil allowed modes - all allowed (gitops)",
			allowedModes: nil,
			mode:         "gitops",
			want:         true,
		},
		{
			name:         "empty allowed modes - all allowed",
			allowedModes: []string{},
			mode:         "hybrid",
			want:         true,
		},
		{
			name:         "single mode allowed - match",
			allowedModes: []string{"gitops"},
			mode:         "gitops",
			want:         true,
		},
		{
			name:         "single mode allowed - no match",
			allowedModes: []string{"gitops"},
			mode:         "direct",
			want:         false,
		},
		{
			name:         "multiple modes allowed - match first",
			allowedModes: []string{"direct", "hybrid"},
			mode:         "direct",
			want:         true,
		},
		{
			name:         "multiple modes allowed - match second",
			allowedModes: []string{"direct", "hybrid"},
			mode:         "hybrid",
			want:         true,
		},
		{
			name:         "multiple modes allowed - no match",
			allowedModes: []string{"direct", "hybrid"},
			mode:         "gitops",
			want:         false,
		},
		{
			name:         "case insensitive match",
			allowedModes: []string{"gitops"},
			mode:         "GITOPS",
			want:         true,
		},
		{
			name:         "case insensitive match - mixed case",
			allowedModes: []string{"GitOps"},
			mode:         "gitops",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDeploymentModeAllowed(tt.allowedModes, tt.mode)
			if got != tt.want {
				t.Errorf("IsDeploymentModeAllowed(%v, %q) = %v, want %v", tt.allowedModes, tt.mode, got, tt.want)
			}
		})
	}
}

func TestParseDeploymentModesWithInvalid(t *testing.T) {
	tests := []struct {
		name        string
		annotation  string
		wantValid   []string
		wantInvalid []string
	}{
		{
			name:        "empty string",
			annotation:  "",
			wantValid:   nil,
			wantInvalid: nil,
		},
		{
			name:        "all valid modes",
			annotation:  "direct,gitops,hybrid",
			wantValid:   []string{"direct", "gitops", "hybrid"},
			wantInvalid: nil,
		},
		{
			name:        "some invalid modes",
			annotation:  "gitops,invalid,direct",
			wantValid:   []string{"gitops", "direct"},
			wantInvalid: []string{"invalid"},
		},
		{
			name:        "multiple invalid modes",
			annotation:  "gitops,foo,bar,hybrid",
			wantValid:   []string{"gitops", "hybrid"},
			wantInvalid: []string{"foo", "bar"},
		},
		{
			name:        "all invalid modes",
			annotation:  "foo,bar,baz",
			wantValid:   nil,
			wantInvalid: []string{"foo", "bar", "baz"},
		},
		{
			name:        "invalid with whitespace",
			annotation:  " gitops , unknown , hybrid ",
			wantValid:   []string{"gitops", "hybrid"},
			wantInvalid: []string{"unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDeploymentModesWithInvalid(tt.annotation)
			if !stringSliceEqual(result.ValidModes, tt.wantValid) {
				t.Errorf("ValidModes = %v, want %v", result.ValidModes, tt.wantValid)
			}
			if !stringSliceEqual(result.InvalidModes, tt.wantInvalid) {
				t.Errorf("InvalidModes = %v, want %v", result.InvalidModes, tt.wantInvalid)
			}
		})
	}
}

func TestAllInvalidModesResultsInUnrestricted(t *testing.T) {
	// This test verifies the important behavior: when all annotation values are invalid,
	// ParseDeploymentModes returns nil, and nil means "all modes allowed" (unrestricted).
	// This ensures backward compatibility - invalid annotations don't break deployments.

	// Parse an annotation with only invalid modes
	parsed := ParseDeploymentModes("foo,bar,invalid")
	if parsed != nil {
		t.Errorf("Expected nil for all-invalid modes, got %v", parsed)
	}

	// Verify that nil means all modes are allowed
	modes := []string{"direct", "gitops", "hybrid"}
	for _, mode := range modes {
		if !IsDeploymentModeAllowed(parsed, mode) {
			t.Errorf("IsDeploymentModeAllowed(nil, %q) = false, want true (nil should allow all modes)", mode)
		}
	}
}

// Helper function to compare string slices
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
