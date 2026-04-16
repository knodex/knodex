// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

import kroparser "github.com/knodex/knodex/server/internal/kro/parser"

// FormSchema represents a form-friendly JSON schema for generating dynamic forms
type FormSchema struct {
	// Name is the RGD name this schema belongs to
	Name string `json:"name"`
	// Namespace is the RGD namespace
	Namespace string `json:"namespace"`
	// Group is the API group of the CRD
	Group string `json:"group"`
	// Kind is the Kind of resources created
	Kind string `json:"kind"`
	// Version is the API version
	Version string `json:"version"`
	// Title is a human-readable title for the form
	Title string `json:"title,omitempty"`
	// Description is a human-readable description
	Description string `json:"description,omitempty"`
	// Properties are the form fields
	Properties map[string]FormProperty `json:"properties"`
	// PropertyOrder defines the display order for form fields.
	// When set, listed fields appear first in this order; unlisted fields follow alphabetically.
	PropertyOrder []string `json:"propertyOrder,omitempty"`
	// Required lists the required field names
	Required []string `json:"required,omitempty"`
	// ConditionalSections defines form sections that should be hidden based on conditions
	ConditionalSections []ConditionalSection `json:"conditionalSections,omitempty"`
	// IsClusterScoped indicates the RGD produces cluster-scoped instances.
	// When true, the frontend should hide the namespace selector in deploy forms.
	IsClusterScoped bool `json:"isClusterScoped"`
	// AdvancedSections defines per-feature and global advanced configuration toggles
	AdvancedSections []AdvancedSection `json:"advancedSections,omitempty"`
}

// AdvancedSection defines configuration that is hidden by default
type AdvancedSection struct {
	// Path is the base path for advanced config (e.g., "advanced")
	Path string `json:"path"`
	// AffectedProperties lists all property paths under advanced
	AffectedProperties []string `json:"affectedProperties"`
}

// ConditionRule represents a single comparison extracted from a CEL expression via AST analysis.
// Used by the frontend to evaluate visibility conditions without parsing CEL strings.
type ConditionRule struct {
	// Field is the schema field path (e.g., "spec.enableDatabase")
	Field string `json:"field"`
	// Op is the comparison operator ("==", "!=", ">", "<", ">=", "<=")
	Op string `json:"op"`
	// Value is the comparison target (true, false, 42, "premium")
	Value interface{} `json:"value"`
}

// ConditionalSection defines a section of the form that is conditionally visible
// based on the value of a controlling field
type ConditionalSection struct {
	// Condition is the CEL expression from includeWhen
	Condition string `json:"condition"`
	// ControllingField is the schema.spec.* path that controls visibility
	ControllingField string `json:"controllingField"`
	// ExpectedValue is the value that makes the section visible (for non-boolean fields)
	ExpectedValue interface{} `json:"expectedValue,omitempty"`
	// AffectedProperties lists the property paths that should be hidden/shown
	AffectedProperties []string `json:"affectedProperties"`
	// Rules contains structured condition rules extracted via CEL AST analysis.
	// When ClientEvaluable is true, the frontend can evaluate these rules directly.
	Rules []ConditionRule `json:"rules,omitempty"`
	// ClientEvaluable indicates whether the frontend can evaluate this condition
	// using the structured Rules. When false (zero value), the frontend should
	// fall back to ExpectedValue evaluation or show the fields (fail open).
	ClientEvaluable bool `json:"clientEvaluable"`
}

// ExternalRefSelectorMetadata provides metadata for fields that should show K8s resource dropdowns
type ExternalRefSelectorMetadata struct {
	// APIVersion is the resource API version (e.g., "v1", "apps/v1")
	APIVersion string `json:"apiVersion"`
	// Kind is the resource kind (e.g., "ConfigMap", "Secret")
	Kind string `json:"kind"`
	// UseInstanceNamespace indicates the dropdown should filter by the deployment namespace.
	// When false, the picker queries all namespaces (for cross-namespace externalRefs).
	UseInstanceNamespace bool `json:"useInstanceNamespace"`
	// AutoFillFields maps resource attributes to sub-field names for auto-populating
	// multiple form fields from a single resource picker selection.
	// Example: {"name": "name", "namespace": "namespace"}
	AutoFillFields map[string]string `json:"autoFillFields,omitempty"`
}

// CollectionAnnotation provides metadata for schema fields that drive forEach collection expansion.
// Attached to FormProperty as x-knodex-collection so the frontend can identify collection-driving
// fields without parsing CEL expressions.
type CollectionAnnotation struct {
	// ResourceID is the ID of the collection resource within the RGD (e.g., "workerPods")
	ResourceID string `json:"resourceId"`
	// IteratorVar is the variable name bound in template expressions (e.g., "worker")
	IteratorVar string `json:"iteratorVar"`
	// Dimensions is the total number of forEach iterators on the resource (1 for simple, 2+ for cartesian)
	Dimensions int `json:"dimensions"`
	// DimensionIndex is the zero-based position of this iterator in the forEach array
	DimensionIndex int `json:"dimensionIndex"`
	// Source is the iterator source type (e.g., "schema")
	Source string `json:"source"`
}

// FormProperty represents a single form field
type FormProperty struct {
	// Type is the JSON schema type (string, number, integer, boolean, object, array)
	Type string `json:"type"`
	// Title is a human-readable field label
	Title string `json:"title,omitempty"`
	// Description is a human-readable field description
	Description string `json:"description,omitempty"`
	// Default is the default value
	Default interface{} `json:"default,omitempty"`
	// Enum lists the allowed values for select fields
	Enum []interface{} `json:"enum,omitempty"`
	// Format provides additional type hints (date-time, email, uri, etc.)
	Format string `json:"format,omitempty"`
	// Minimum for numeric types
	Minimum *float64 `json:"minimum,omitempty"`
	// Maximum for numeric types
	Maximum *float64 `json:"maximum,omitempty"`
	// MinLength for strings
	MinLength *int `json:"minLength,omitempty"`
	// MaxLength for strings
	MaxLength *int `json:"maxLength,omitempty"`
	// Pattern is a regex pattern for strings
	Pattern string `json:"pattern,omitempty"`
	// Properties for nested objects
	Properties map[string]FormProperty `json:"properties,omitempty"`
	// PropertyOrder defines the display order for nested object fields.
	PropertyOrder []string `json:"propertyOrder,omitempty"`
	// Required for nested objects
	Required []string `json:"required,omitempty"`
	// Items for arrays
	Items *FormProperty `json:"items,omitempty"`
	// XKubernetesPreserveUnknownFields indicates the field accepts arbitrary data
	XKubernetesPreserveUnknownFields bool `json:"x-kubernetes-preserve-unknown-fields,omitempty"`
	// Additional metadata
	Nullable bool `json:"nullable,omitempty"`
	// Path is the JSON path to this property (for nested fields)
	Path string `json:"path,omitempty"`
	// ExternalRefSelector provides metadata for fields that should show K8s resource dropdowns
	ExternalRefSelector *ExternalRefSelectorMetadata `json:"externalRefSelector,omitempty"`
	// IsAdvanced indicates this field is under the advanced section and hidden by default
	IsAdvanced bool `json:"isAdvanced,omitempty"`
	// CollectionAnnotation provides metadata when this field drives a forEach collection expansion.
	// Serialized as x-knodex-collection following OpenAPI extension convention.
	CollectionAnnotation *CollectionAnnotation `json:"x-knodex-collection,omitempty"`
}

// SchemaResponse is the API response for the schema endpoint
type SchemaResponse struct {
	// RGD is the RGD name
	RGD string `json:"rgd"`
	// Schema is the form schema
	Schema *FormSchema `json:"schema"`
	// SecretRefs lists all externalRef resources that reference Kubernetes Secrets
	SecretRefs []kroparser.SecretRef `json:"secretRefs"`
	// Error is set if schema extraction failed
	Error string `json:"error,omitempty"`
	// Warnings contains non-fatal issues encountered during schema enrichment
	Warnings []string `json:"warnings,omitempty"`
	// CRDFound indicates whether the CRD was found
	CRDFound bool `json:"crdFound"`
	// Source indicates the schema source: "crd+rgd" (full) or "rgd-only" (degraded)
	Source string `json:"source,omitempty"`
}
