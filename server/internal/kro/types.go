// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package kro provides centralized KRO (Kubernetes Resource Orchestrator)
// integration types, constants, and utilities. All KRO-specific code should
// live under this package tree.
package kro

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubernetes-sigs/kro/api/v1alpha1"
)

// RGDGVR returns the GroupVersionResource for ResourceGraphDefinitions.
// Returned by value to prevent callers from mutating the canonical GVR.
func RGDGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    v1alpha1.KRODomainName,
		Version:  "v1alpha1",
		Resource: "resourcegraphdefinitions",
	}
}

// KRO CRD identifiers derived from upstream constants.
const (
	RGDGroup    = v1alpha1.KRODomainName // "kro.run"
	RGDVersion  = "v1alpha1"
	RGDResource = "resourcegraphdefinitions"
	RGDKind     = "ResourceGraphDefinition"
)

// Knodex annotation keys for RGD catalog discovery.
const (
	CatalogAnnotation         = "knodex.io/catalog"
	DescriptionAnnotation     = "knodex.io/description"
	TagsAnnotation            = "knodex.io/tags"
	CategoryAnnotation        = "knodex.io/category"
	IconAnnotation            = "knodex.io/icon"
	VersionAnnotation         = "knodex.io/version"
	TitleAnnotation           = "knodex.io/title"
	DeploymentModesAnnotation = "knodex.io/deployment-modes"
)

// Label keys for RBAC project filtering on cluster-scoped RGDs.
const (
	RGDProjectLabel      = "knodex.io/project"
	RGDOrganizationLabel = "knodex.io/organization"
)
