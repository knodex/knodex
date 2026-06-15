// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"strings"
	"testing"
)

func TestValidateTeamName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "platform", false},
		{"valid with hyphen", "alpha-team", false},
		{"valid alphanumeric", "team123", false},
		{"empty", "", true},
		{"uppercase", "AlphaTeam", true},
		{"leading hyphen", "-alpha", true},
		{"trailing hyphen", "alpha-", true},
		{"underscore", "alpha_team", true},
		{"space", "alpha team", true},
		{"over length", strings.Repeat("a", 254), true},
		{"at length limit", strings.Repeat("a", 253), false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTeamName(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateTeamName(%q) expected error, got nil", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateTeamName(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

func TestValidateTeamSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    TeamSpec
		wantErr bool
	}{
		{"valid single group", TeamSpec{OIDCGroups: []string{"devs"}}, false},
		{"valid multiple groups", TeamSpec{OIDCGroups: []string{"devs", "ops"}}, false},
		{"valid with description", TeamSpec{Description: "team", OIDCGroups: []string{"devs"}}, false},
		{"zero groups", TeamSpec{OIDCGroups: []string{}}, true},
		{"nil groups", TeamSpec{}, true},
		{"empty group string", TeamSpec{OIDCGroups: []string{"devs", ""}}, true},
		{"over length group", TeamSpec{OIDCGroups: []string{strings.Repeat("g", 254)}}, true},
		{"group at length limit", TeamSpec{OIDCGroups: []string{strings.Repeat("g", 253)}}, false},
		{"duplicate groups", TeamSpec{OIDCGroups: []string{"devs", "devs"}}, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTeamSpec(tt.spec)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateTeamSpec(%+v) expected error, got nil", tt.spec)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateTeamSpec(%+v) unexpected error: %v", tt.spec, err)
			}
		})
	}
}
