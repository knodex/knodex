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

// GraphRevisionGVR returns the GroupVersionResource for KRO GraphRevisions.
// Returned by value to prevent callers from mutating the canonical GVR.
func GraphRevisionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    GraphRevisionGroup,
		Version:  GraphRevisionVersion,
		Resource: GraphRevisionResource,
	}
}

// KRO CRD identifiers derived from upstream constants.
const (
	RGDGroup    = v1alpha1.KRODomainName // "kro.run"
	RGDVersion  = "v1alpha1"
	RGDResource = "resourcegraphdefinitions"
	RGDKind     = "ResourceGraphDefinition"
)

// GraphRevision CRD identifiers (internal.kro.run API group).
const (
	GraphRevisionGroup    = "internal.kro.run"
	GraphRevisionVersion  = "v1alpha1"
	GraphRevisionResource = "graphrevisions"
)

// Knodex annotation keys for RGD catalog discovery.
const (
	CatalogAnnotation         = "knodex.io/catalog"
	DescriptionAnnotation     = "knodex.io/description"
	TagsAnnotation            = "knodex.io/tags"
	CategoryAnnotation        = "knodex.io/category"
	IconAnnotation            = "knodex.io/icon"
	TitleAnnotation           = "knodex.io/title"
	DeploymentModesAnnotation = "knodex.io/deployment-modes"
	ExtendsKindAnnotation     = "knodex.io/extends-kind"
	DocsURLAnnotation         = "knodex.io/docs-url"
	PropertyOrderAnnotation   = "knodex.io/property-order"
)

// Label keys for RBAC project filtering on cluster-scoped RGDs.
const (
	RGDProjectLabel      = "knodex.io/project"
	RGDOrganizationLabel = "knodex.io/organization"
	RGDPackageLabel      = "knodex.io/package"
)

// Agent RGD schema kinds. The catalog annotation is the ONLY publishing
// gateway (whether an RGD is ingested and user-fetchable at all); these kinds
// only ROUTE a published RGD to the Agents workspace instead of the main
// catalog list. Agent RGDs therefore carry knodex.io/catalog: "true" like
// every other published RGD.
const (
	AgentKind               = "KagentAgent"
	AgentTemplateKind       = "KnodexAgentTemplate"
	AgentModelConfigKind    = "KnodexAgentModelConfig"
	AgentProviderConfigKind = "KnodexProviderConfig"
)

// IsAgentSchemaKind reports whether kind belongs to the Agents workspace.
// Routing only: the main catalog list excludes these kinds (they have their
// own pages); it grants no visibility by itself.
func IsAgentSchemaKind(kind string) bool {
	switch kind {
	case AgentKind, AgentTemplateKind, AgentModelConfigKind, AgentProviderConfigKind:
		return true
	}
	return false
}
