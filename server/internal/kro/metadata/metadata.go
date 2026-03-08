// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package metadata re-exports KRO upstream label constants and provides
// Knodex-specific metadata helpers for working with KRO resources.
package metadata

import (
	krometa "github.com/kubernetes-sigs/kro/pkg/metadata"
)

// Re-export KRO upstream label constants for use across Knodex packages.
// This avoids scattering direct upstream imports throughout the codebase.
const (
	// LabelKROPrefix is the label key prefix used by KRO ("kro.run/").
	LabelKROPrefix = krometa.LabelKROPrefix

	// ResourceGraphDefinitionNameLabel identifies the RGD that owns a resource.
	ResourceGraphDefinitionNameLabel = krometa.ResourceGraphDefinitionNameLabel

	// ResourceGraphDefinitionIDLabel is the UID of the owning RGD.
	ResourceGraphDefinitionIDLabel = krometa.ResourceGraphDefinitionIDLabel

	// ResourceGraphDefinitionNamespaceLabel is the namespace of the owning RGD.
	ResourceGraphDefinitionNamespaceLabel = krometa.ResourceGraphDefinitionNamespaceLabel

	// ResourceGraphDefinitionVersionLabel is the version of the owning RGD.
	ResourceGraphDefinitionVersionLabel = krometa.ResourceGraphDefinitionVersionLabel

	// InstanceLabel is the name of the instance that created the resource.
	InstanceLabel = krometa.InstanceLabel

	// InstanceIDLabel is the UID of the instance that created the resource.
	InstanceIDLabel = krometa.InstanceIDLabel

	// InstanceNamespaceLabel is the namespace of the instance.
	InstanceNamespaceLabel = krometa.InstanceNamespaceLabel

	// ManagedByLabelKey is the standard Kubernetes managed-by label key.
	ManagedByLabelKey = krometa.ManagedByLabelKey

	// ManagedByKROValue is the value used for managed-by when KRO owns the resource.
	ManagedByKROValue = krometa.ManagedByKROValue
)

// IsKROOwned delegates to the upstream KRO metadata package.
var IsKROOwned = krometa.IsKROOwned

// LabelWithFallback looks up a label value by trying each key in order.
// Returns the value of the first key that exists and is non-empty, or ""
// if none match. This supports KRO version migrations where label keys
// may change between releases.
func LabelWithFallback(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if v := labels[key]; v != "" {
			return v
		}
	}
	return ""
}
