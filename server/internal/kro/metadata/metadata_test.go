// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import (
	"testing"
)

func TestLabelWithFallback(t *testing.T) {
	labels := map[string]string{
		"kro.run/resource-graph-definition-name": "my-rgd",
		"legacy-key":                             "old-value",
	}

	tests := []struct {
		name string
		keys []string
		want string
	}{
		{"primary key found", []string{"kro.run/resource-graph-definition-name"}, "my-rgd"},
		{"fallback key used", []string{"missing-key", "legacy-key"}, "old-value"},
		{"no keys match", []string{"no-such-key", "also-missing"}, ""},
		{"empty keys", []string{}, ""},
		{"nil labels", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := labels
			if tt.name == "nil labels" {
				input = nil
			}
			got := LabelWithFallback(input, tt.keys...)
			if got != tt.want {
				t.Errorf("LabelWithFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLabelWithFallback_DualPrefix verifies that LabelWithFallback correctly
// resolves labels during KRO's planned label migration from "kro.run/" to
// "internal.kro.run/". Covers all four scenarios: new prefix only, old prefix
// only, both present (new wins), and neither present.
func TestLabelWithFallback_DualPrefix(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   string
	}{
		{
			name: "new prefix only (Phase 2 KRO)",
			labels: map[string]string{
				InternalResourceGraphDefinitionNameLabel: "my-rgd",
			},
			want: "my-rgd",
		},
		{
			name: "old prefix only (pre-migration KRO)",
			labels: map[string]string{
				ResourceGraphDefinitionNameLabel: "my-rgd",
			},
			want: "my-rgd",
		},
		{
			name: "both present - new prefix wins (Phase 1 dual-write KRO)",
			labels: map[string]string{
				InternalResourceGraphDefinitionNameLabel: "new-rgd",
				ResourceGraphDefinitionNameLabel:         "old-rgd",
			},
			want: "new-rgd",
		},
		{
			name:   "neither present",
			labels: map[string]string{"unrelated": "value"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LabelWithFallback(tt.labels, InternalResourceGraphDefinitionNameLabel, ResourceGraphDefinitionNameLabel)
			if got != tt.want {
				t.Errorf("LabelWithFallback() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestInternalLabelConstants_Values verifies the internal-prefix label constants
// have the expected string values matching the KRO KREP migration plan.
func TestInternalLabelConstants_Values(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"LabelInternalKROPrefix", LabelInternalKROPrefix, "internal.kro.run/"},
		{"InternalResourceGraphDefinitionNameLabel", InternalResourceGraphDefinitionNameLabel, "internal.kro.run/resource-graph-definition-name"},
		{"InternalInstanceLabel", InternalInstanceLabel, "internal.kro.run/instance-name"},
		{"InternalInstanceIDLabel", InternalInstanceIDLabel, "internal.kro.run/instance-id"},
		{"InternalInstanceKindLabel", InternalInstanceKindLabel, "internal.kro.run/instance-kind"},
		{"InternalInstanceNamespaceLabel", InternalInstanceNamespaceLabel, "internal.kro.run/instance-namespace"},
		{"InternalNodeIDLabel", InternalNodeIDLabel, "internal.kro.run/node-id"},
		{"InternalOwnedLabel", InternalOwnedLabel, "internal.kro.run/owned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestUpstreamLabelConstants_Values verifies that re-exported upstream KRO
// label constants match expected string values, catching any upstream renames.
func TestUpstreamLabelConstants_Values(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"LabelKROPrefix", LabelKROPrefix, "kro.run/"},
		{"InstanceLabel", InstanceLabel, "kro.run/instance-name"},
		{"InstanceIDLabel", InstanceIDLabel, "kro.run/instance-id"},
		{"InstanceNamespaceLabel", InstanceNamespaceLabel, "kro.run/instance-namespace"},
		{"InstanceKindLabel", InstanceKindLabel, "kro.run/instance-kind"},
		{"InstanceGroupLabel", InstanceGroupLabel, "kro.run/instance-group"},
		{"InstanceVersionLabel", InstanceVersionLabel, "kro.run/instance-version"},
		{"NodeIDLabel", NodeIDLabel, "kro.run/node-id"},
		{"OwnedLabel", OwnedLabel, "kro.run/owned"},
		{"KROVersionLabel", KROVersionLabel, "kro.run/kro-version"},
		{"CollectionIndexLabel", CollectionIndexLabel, "kro.run/collection-index"},
		{"CollectionSizeLabel", CollectionSizeLabel, "kro.run/collection-size"},
		{"ResourceGraphDefinitionNameLabel", ResourceGraphDefinitionNameLabel, "kro.run/resource-graph-definition-name"},
		{"ManagedByLabelKey", ManagedByLabelKey, "app.kubernetes.io/managed-by"},
		{"ManagedByKROValue", ManagedByKROValue, "kro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestLabelWithFallback_ChildResourceLabels verifies dual-prefix resolution
// for the child resource labels used by the ChildResourceService.
func TestLabelWithFallback_ChildResourceLabels(t *testing.T) {
	tests := []struct {
		name      string
		labels    map[string]string
		internal  string
		external  string
		wantValue string
	}{
		{
			name:      "node-id from external prefix",
			labels:    map[string]string{NodeIDLabel: "frontend"},
			internal:  InternalNodeIDLabel,
			external:  NodeIDLabel,
			wantValue: "frontend",
		},
		{
			name:      "node-id from internal prefix",
			labels:    map[string]string{InternalNodeIDLabel: "backend"},
			internal:  InternalNodeIDLabel,
			external:  NodeIDLabel,
			wantValue: "backend",
		},
		{
			name:      "instance-kind from external prefix",
			labels:    map[string]string{InstanceKindLabel: "TestPodPair"},
			internal:  InternalInstanceKindLabel,
			external:  InstanceKindLabel,
			wantValue: "TestPodPair",
		},
		{
			name:      "internal prefix wins when both present",
			labels:    map[string]string{InternalNodeIDLabel: "new-id", NodeIDLabel: "old-id"},
			internal:  InternalNodeIDLabel,
			external:  NodeIDLabel,
			wantValue: "new-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LabelWithFallback(tt.labels, tt.internal, tt.external)
			if got != tt.wantValue {
				t.Errorf("LabelWithFallback() = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

// BenchmarkLabelWithFallback enforces the NFR-K2 performance contract: label
// resolution must show no measurable regression compared to direct map access.
// Run with: go test -bench=BenchmarkLabelWithFallback -benchmem ./internal/kro/metadata/
func BenchmarkLabelWithFallback(b *testing.B) {
	labels := map[string]string{
		ResourceGraphDefinitionNameLabel: "my-rgd",
		InstanceLabel:                    "my-instance",
		InstanceIDLabel:                  "abc-123",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LabelWithFallback(labels, InternalResourceGraphDefinitionNameLabel, ResourceGraphDefinitionNameLabel)
	}
}
