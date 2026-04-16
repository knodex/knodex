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

	// InstanceKindLabel is the Kind of the instance (i.e. the RGD's generated CRD kind).
	InstanceKindLabel = krometa.InstanceKindLabel

	// InstanceGroupLabel is the API group of the instance.
	InstanceGroupLabel = krometa.InstanceGroupLabel

	// InstanceVersionLabel is the API version of the instance.
	InstanceVersionLabel = krometa.InstanceVersionLabel

	// NodeIDLabel identifies which resource node in the RGD graph produced this child resource.
	NodeIDLabel = krometa.NodeIDLabel

	// OwnedLabel marks a resource as owned/managed by KRO.
	OwnedLabel = krometa.OwnedLabel

	// KROVersionLabel records the KRO version that created the resource.
	KROVersionLabel = krometa.KROVersionLabel

	// CollectionIndexLabel is the index of a resource within a forEach collection.
	CollectionIndexLabel = krometa.CollectionIndexLabel

	// CollectionSizeLabel is the total size of a forEach collection.
	CollectionSizeLabel = krometa.CollectionSizeLabel

	// ManagedByLabelKey is the standard Kubernetes managed-by label key.
	ManagedByLabelKey = krometa.ManagedByLabelKey

	// ManagedByKROValue is the value used for managed-by when KRO owns the resource.
	ManagedByKROValue = krometa.ManagedByKROValue
)

// Internal label prefix and constants for the planned KRO label migration.
// KRO's KREP proposal (docs/design/proposals/label-migration.md) migrates all
// controller-owned labels from "kro.run/" to "internal.kro.run/" using a
// two-phase approach: Phase 1 (dual-write) followed by Phase 2 (cutover).
// These constants allow Knodex to discover instances regardless of which
// label prefix KRO uses, providing forward compatibility with the migration.
//
// Only labels actively used by Knodex in the instance processing pipeline
// have internal-prefix variants defined here.
const (
	// LabelInternalKROPrefix is the new internal label prefix ("internal.kro.run/").
	LabelInternalKROPrefix = "internal.kro.run/"

	// Internal-prefix variants of labels used by Knodex for instance discovery
	// and child resource queries.
	InternalResourceGraphDefinitionNameLabel = LabelInternalKROPrefix + "resource-graph-definition-name"
	InternalInstanceLabel                    = LabelInternalKROPrefix + "instance-name"
	InternalInstanceIDLabel                  = LabelInternalKROPrefix + "instance-id"
	InternalInstanceKindLabel                = LabelInternalKROPrefix + "instance-kind"
	InternalInstanceNamespaceLabel           = LabelInternalKROPrefix + "instance-namespace"
	InternalNodeIDLabel                      = LabelInternalKROPrefix + "node-id"
	InternalOwnedLabel                       = LabelInternalKROPrefix + "owned"
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
