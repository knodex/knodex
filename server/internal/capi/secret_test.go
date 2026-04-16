// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package capi

import (
	"testing"
)

func TestKubeconfigSecretName(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		want        string
	}{
		{
			name:        "simple cluster name",
			clusterName: "prod-eu-west",
			want:        "prod-eu-west-kubeconfig",
		},
		{
			name:        "single word cluster",
			clusterName: "management",
			want:        "management-kubeconfig",
		},
		{
			name:        "cluster with dots",
			clusterName: "cluster.example.com",
			want:        "cluster.example.com-kubeconfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KubeconfigSecretName(tt.clusterName)
			if got != tt.want {
				t.Errorf("KubeconfigSecretName(%q) = %q, want %q", tt.clusterName, got, tt.want)
			}
		})
	}
}
