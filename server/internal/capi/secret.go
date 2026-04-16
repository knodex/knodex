// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package capi provides thin wrappers around the Cluster API utility libraries.
// Only sigs.k8s.io/cluster-api/util/secret is imported — the full CAPI typed API
// is not used. All Cluster object access goes through the dynamic client + k8s/parser.
package capi

import (
	capisecret "sigs.k8s.io/cluster-api/util/secret"
)

// KubeconfigSecretName returns the canonical CAPI kubeconfig secret name for a cluster.
// Uses sigs.k8s.io/cluster-api/util/secret.Name() to derive the name — no hardcoded
// "-kubeconfig" suffixes anywhere in the codebase.
func KubeconfigSecretName(clusterName string) string {
	return capisecret.Name(clusterName, capisecret.Kubeconfig)
}
