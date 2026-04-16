// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

// compat_test.go — Upstream KRO contract tests for the metadata package.
//
// These tests assert the behavior and API surface of KRO's pkg/metadata that
// Knodex depends on. When a KRO version bump (via Renovate) causes a failure
// here, it means the upstream contract changed and the wrapper or test must
// be updated.
//
// NOTE: Tests for Knodex-authored helpers (e.g., LabelWithFallback) live in
// metadata_test.go, not here. This file is strictly for upstream contracts.

import (
	"testing"

	krometa "github.com/kubernetes-sigs/kro/pkg/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCompat_LabelConstants_ExpectedValues verifies that key KRO label constants
// have the expected string values. If KRO changes these values, Knodex label
// queries and watches will silently break unless caught here.
func TestCompat_LabelConstants_ExpectedValues(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{
			name:     "ResourceGraphDefinitionNameLabel",
			got:      ResourceGraphDefinitionNameLabel,
			expected: "kro.run/resource-graph-definition-name",
		},
		{
			name:     "ManagedByLabelKey",
			got:      ManagedByLabelKey,
			expected: "app.kubernetes.io/managed-by",
		},
		{
			name:     "ManagedByKROValue",
			got:      ManagedByKROValue,
			expected: "kro",
		},
		{
			name:     "LabelKROPrefix",
			got:      LabelKROPrefix,
			expected: "kro.run/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestCompat_LabelConstants_MatchUpstream verifies that all re-exported
// constants still match their upstream KRO values. A mismatch means KRO
// changed the constant and the re-export must be updated.
func TestCompat_LabelConstants_MatchUpstream(t *testing.T) {
	pairs := []struct {
		name     string
		local    string
		upstream string
	}{
		{"ResourceGraphDefinitionNameLabel", ResourceGraphDefinitionNameLabel, krometa.ResourceGraphDefinitionNameLabel},
		{"ResourceGraphDefinitionIDLabel", ResourceGraphDefinitionIDLabel, krometa.ResourceGraphDefinitionIDLabel},
		{"ResourceGraphDefinitionNamespaceLabel", ResourceGraphDefinitionNamespaceLabel, krometa.ResourceGraphDefinitionNamespaceLabel},
		{"ResourceGraphDefinitionVersionLabel", ResourceGraphDefinitionVersionLabel, krometa.ResourceGraphDefinitionVersionLabel},
		{"InstanceLabel", InstanceLabel, krometa.InstanceLabel},
		{"InstanceIDLabel", InstanceIDLabel, krometa.InstanceIDLabel},
		{"InstanceNamespaceLabel", InstanceNamespaceLabel, krometa.InstanceNamespaceLabel},
		{"ManagedByLabelKey", ManagedByLabelKey, krometa.ManagedByLabelKey},
		{"ManagedByKROValue", ManagedByKROValue, krometa.ManagedByKROValue},
	}

	for _, p := range pairs {
		if p.local != p.upstream {
			t.Errorf("%s: local = %q, upstream = %q", p.name, p.local, p.upstream)
		}
	}
}

// TestCompat_InternalPrefix_MatchesUpstreamSuffix verifies that each
// Knodex-defined "internal.kro.run/" constant uses the same suffix as the
// upstream "kro.run/" constant. If KRO renames a suffix in a future release,
// this test will catch the divergence.
func TestCompat_InternalPrefix_MatchesUpstreamSuffix(t *testing.T) {
	pairs := []struct {
		name     string
		internal string
		upstream string
	}{
		{"ResourceGraphDefinitionNameLabel", InternalResourceGraphDefinitionNameLabel, krometa.ResourceGraphDefinitionNameLabel},
		{"InstanceLabel", InternalInstanceLabel, krometa.InstanceLabel},
		{"InstanceIDLabel", InternalInstanceIDLabel, krometa.InstanceIDLabel},
	}

	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			// Strip each prefix and compare suffixes
			internalSuffix := p.internal[len(LabelInternalKROPrefix):]
			upstreamSuffix := p.upstream[len(LabelKROPrefix):]
			if internalSuffix != upstreamSuffix {
				t.Errorf("internal suffix %q != upstream suffix %q — KRO may have renamed the label", internalSuffix, upstreamSuffix)
			}
		})
	}
}

// TestCompat_IsKROOwned_Behavior verifies the behavioral contract of
// KRO's IsKROOwned function: it returns true only when the managed-by
// label is set to "kro".
func TestCompat_IsKROOwned_Behavior(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{
			name:   "owned by KRO",
			labels: map[string]string{ManagedByLabelKey: ManagedByKROValue},
			want:   true,
		},
		{
			name:   "managed by something else",
			labels: map[string]string{ManagedByLabelKey: "helm"},
			want:   false,
		},
		{
			name:   "no managed-by label",
			labels: map[string]string{},
			want:   false,
		},
		{
			name:   "nil labels",
			labels: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{Labels: tt.labels}
			got := IsKROOwned(obj)
			if got != tt.want {
				t.Errorf("IsKROOwned(%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}
}
