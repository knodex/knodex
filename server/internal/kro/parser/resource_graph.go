// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

// ResourceDefinition represents a Kubernetes resource defined in an RGD's spec.resources array.
type ResourceDefinition struct {
	// ID is a unique identifier for this resource within the RGD.
	// Format: "{index}-{kind}" (e.g., "0-ConfigMap", "1-Deployment")
	ID string `json:"id"`

	// APIVersion is the Kubernetes API version (e.g., "v1", "apps/v1")
	APIVersion string `json:"apiVersion"`

	// Kind is the Kubernetes resource kind (e.g., "ConfigMap", "Deployment")
	Kind string `json:"kind"`

	// IsTemplate indicates whether this is a template resource (true) or an externalRef (false)
	IsTemplate bool `json:"isTemplate"`

	// IncludeWhen contains the conditional creation expression, if any
	IncludeWhen *ConditionExpr `json:"includeWhen,omitempty"`

	// DependsOn lists the IDs of resources this resource depends on within the RGD
	DependsOn []string `json:"dependsOn"`

	// ExternalRef contains external reference info if this is an externalRef resource
	ExternalRef *ExternalRefInfo `json:"externalRef,omitempty"`

	// SchemaFields lists all schema.spec.* field paths referenced by this resource
	// (e.g., "spec.name", "spec.namespace"). Populated from ${schema.spec.*} patterns.
	SchemaFields []string `json:"schemaFields,omitempty"`
}

// ExternalRefInfo contains information about an externalRef resource.
type ExternalRefInfo struct {
	// APIVersion is the API version of the external resource
	APIVersion string `json:"apiVersion"`

	// Kind is the kind of the external resource
	Kind string `json:"kind"`

	// NameExpr is the name expression, which may contain CEL (e.g., "${schema.spec.configMapName}")
	NameExpr string `json:"nameExpr"`

	// NamespaceExpr is the namespace expression, which may contain CEL
	NamespaceExpr string `json:"namespaceExpr,omitempty"`

	// UsesSchemaSpec indicates whether the name uses ${schema.spec.*} expressions
	UsesSchemaSpec bool `json:"usesSchemaSpec"`

	// SchemaField is the extracted field path if UsesSchemaSpec is true (e.g., "spec.configMapName")
	SchemaField string `json:"schemaField,omitempty"`

	// NamespaceSchemaField is the extracted namespace field path when namespace also uses
	// ${schema.spec.*} expressions (e.g., "spec.externalRef.permissionResults.namespace")
	NamespaceSchemaField string `json:"namespaceSchemaField,omitempty"`
}

// ConditionExpr represents an includeWhen conditional expression.
type ConditionExpr struct {
	// Expression is the original CEL expression (e.g., "schema.spec.ingress.enabled == true")
	Expression string `json:"expression"`

	// SchemaFields are the schema.spec.* field paths used in the expression
	SchemaFields []string `json:"schemaFields"`
}

// SecretRef represents a detected secret reference from an externalRef resource.
// This is the canonical definition used by models.CatalogRGD, models.SchemaResponse,
// the RGD watcher, and the schema handler.
type SecretRef struct {
	// Type is "dynamic" (either name or namespace contains a CEL expression) or "fixed" (both are literals)
	Type string `json:"type"`

	// Name is the literal secret name (for fixed refs only)
	Name string `json:"name,omitempty"`

	// Namespace is the literal secret namespace (for fixed refs only)
	Namespace string `json:"namespace,omitempty"`

	// NameExpr is the name value for dynamic refs. May be a CEL expression (contains "${...}")
	// when the name is parameterised, or a literal string when only the namespace is dynamic.
	NameExpr string `json:"nameExpr,omitempty"`

	// NamespaceExpr is the namespace value for dynamic refs. May be a CEL expression or a literal
	// when only the name is dynamic.
	NamespaceExpr string `json:"namespaceExpr,omitempty"`

	// ID is the resource ID within the RGD (e.g., "0-Secret")
	ID string `json:"id"`

	// Description is the human-readable purpose of this secret reference,
	// extracted from the RGD schema's externalRef field description.
	Description string `json:"description,omitempty"`

	// ExternalRefID is the semantic identifier for this secret reference,
	// matching the field name in spec.schema.spec.externalRef (e.g., "dbSecret").
	// This is distinct from ID (which is the graph resource ID like "0-Secret").
	ExternalRefID string `json:"externalRefId,omitempty"`
}

// ResourceGraph represents the parsed resource graph from an RGD.
type ResourceGraph struct {
	// RGDName is the name of the RGD this graph belongs to
	RGDName string `json:"rgdName"`

	// RGDNamespace is the namespace of the RGD
	RGDNamespace string `json:"rgdNamespace"`

	// Resources are all the resource definitions in the RGD
	Resources []ResourceDefinition `json:"resources"`

	// Edges represent dependencies between resources (from ID -> to ID)
	Edges []ResourceEdge `json:"edges"`

	// SecretRefs are externalRef resources that reference Kubernetes Secrets
	SecretRefs []SecretRef `json:"secretRefs,omitempty"`
}

// ResourceEdge represents a dependency edge between two resources.
type ResourceEdge struct {
	// From is the ID of the dependent resource
	From string `json:"from"`

	// To is the ID of the resource being depended on
	To string `json:"to"`

	// Type describes the nature of the dependency
	Type string `json:"type"` // "reference", "externalRef"
}

// Dependency represents a detected dependency reference from a CEL expression.
type Dependency struct {
	// Name is the referenced RGD name extracted from the expression.
	Name string `json:"name"`

	// Expression is the full CEL expression that was parsed.
	Expression string `json:"expression"`

	// Path is the location within the RGD spec where this dependency was found.
	Path string `json:"path"`

	// OutputField is the specific output field being referenced (e.g., "status.conditions").
	OutputField string `json:"outputField,omitempty"`
}

// ParseResult contains all dependencies extracted from an RGD spec.
type ParseResult struct {
	// Dependencies is the list of all detected dependencies.
	Dependencies []Dependency `json:"dependencies"`

	// Errors contains any parsing errors encountered.
	Errors []ParseError `json:"errors,omitempty"`
}

// ParseError represents an error encountered during expression parsing.
type ParseError struct {
	// Path is the location where the error occurred.
	Path string `json:"path"`

	// Expression is the problematic expression.
	Expression string `json:"expression"`

	// Message describes the error.
	Message string `json:"message"`
}

// GetResourceByID finds a resource by its ID in the graph.
func (g *ResourceGraph) GetResourceByID(id string) *ResourceDefinition {
	for i := range g.Resources {
		if g.Resources[i].ID == id {
			return &g.Resources[i]
		}
	}
	return nil
}

// GetExternalRefs returns all resources that are external references.
func (g *ResourceGraph) GetExternalRefs() []ResourceDefinition {
	var refs []ResourceDefinition
	for _, res := range g.Resources {
		if !res.IsTemplate && res.ExternalRef != nil {
			refs = append(refs, res)
		}
	}
	return refs
}

// GetConditionalResources returns all resources with includeWhen conditions.
func (g *ResourceGraph) GetConditionalResources() []ResourceDefinition {
	var conditional []ResourceDefinition
	for _, res := range g.Resources {
		if res.IncludeWhen != nil {
			conditional = append(conditional, res)
		}
	}
	return conditional
}
