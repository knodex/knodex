// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"fmt"
	"regexp"
)

// MaxOIDCGroupLength bounds an individual OIDC group string. Mirrors the
// Project CRD's roles[].groups[] maxLength constraint.
const MaxOIDCGroupLength = 253

// teamNamePattern is the DNS-1123 subdomain pattern reused from
// ValidateProjectName. Team names follow Kubernetes object-name rules.
var teamNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// ValidateTeamName validates a team name follows DNS-1123 subdomain rules:
// lowercase alphanumeric, hyphens, max 253 chars, starts/ends with alphanumeric.
// Uniqueness is enforced by Kubernetes per-namespace (Teams are namespace-scoped);
// this only checks format.
func ValidateTeamName(name string) error {
	if name == "" {
		return fmt.Errorf("team name cannot be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("team name cannot exceed 253 characters")
	}
	if !teamNamePattern.MatchString(name) {
		return fmt.Errorf("team name must be lowercase alphanumeric with hyphens, cannot start or end with hyphen")
	}
	return nil
}

// ValidateTeamSpec validates a TeamSpec: at least one OIDC group, each group
// non-empty and within length bounds, and no duplicate group strings.
func ValidateTeamSpec(spec TeamSpec) error {
	if len(spec.OIDCGroups) == 0 {
		return fmt.Errorf("team must specify at least one oidcGroup")
	}

	seen := make(map[string]bool, len(spec.OIDCGroups))
	for i, group := range spec.OIDCGroups {
		if group == "" {
			return fmt.Errorf("oidcGroups[%d]: group cannot be empty", i)
		}
		if len(group) > MaxOIDCGroupLength {
			return fmt.Errorf("oidcGroups[%d]: group exceeds maximum length of %d characters", i, MaxOIDCGroupLength)
		}
		if seen[group] {
			return fmt.Errorf("oidcGroups[%d]: duplicate group %q", i, group)
		}
		seen[group] = true
	}

	return nil
}
