package parser

import (
	"fmt"
	"regexp"
	"strings"

	k8sparser "github.com/provops-org/knodex/server/internal/k8s/parser"
)

// schemaSpecPattern matches ${schema.spec.*} expressions.
// Limit capture group to 200 chars to prevent ReDoS
var schemaSpecPattern = regexp.MustCompile(`\$\{schema\.spec\.([^}]{1,200})\}`)

// schemaFieldPattern matches schema.spec.* in CEL expressions (without ${}).
// Limit capture group to 200 chars to prevent ReDoS
var schemaFieldPattern = regexp.MustCompile(`schema\.spec\.([a-zA-Z0-9_.?\-]{1,200})`)

// celExpressionPattern matches ${...} CEL expressions in RGD specs.
// Captures the content inside the braces.
var celExpressionPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// maxRecursionDepth prevents stack overflow from deeply nested objects
const maxRecursionDepth = 100

// ResourceParser parses RGD spec.resources arrays to extract resource definitions.
type ResourceParser struct{}

// NewResourceParser creates a new ResourceParser.
func NewResourceParser() *ResourceParser {
	return &ResourceParser{}
}

// ParseRGDResources parses an RGD spec to extract resource definitions and their dependencies.
func (p *ResourceParser) ParseRGDResources(rgdName, rgdNamespace string, rgdSpec map[string]interface{}) (*ResourceGraph, error) {
	graph := &ResourceGraph{
		RGDName:      rgdName,
		RGDNamespace: rgdNamespace,
		Resources:    make([]ResourceDefinition, 0),
		Edges:        make([]ResourceEdge, 0),
	}

	// Extract spec.resources array using parser helper
	resources, err := k8sparser.GetSlice(rgdSpec, "resources")
	if err != nil {
		// No resources array, return empty graph
		return graph, nil
	}

	// Map to track resource IDs by their internal ID (used for dependency resolution)
	idByInternalID := make(map[string]string)

	// First pass: extract all resource definitions
	for i, res := range resources {
		resMap, ok := res.(map[string]interface{})
		if !ok {
			continue
		}

		resDef := p.parseResource(i, resMap)
		if resDef != nil {
			graph.Resources = append(graph.Resources, *resDef)

			// Map internal ID to our generated ID
			if internalID := extractInternalID(resMap); internalID != "" {
				idByInternalID[internalID] = resDef.ID
			}
		}
	}

	// Second pass: resolve internal dependencies and build edges
	for i := range graph.Resources {
		res := &graph.Resources[i]
		p.resolveDependencies(res, resources[i].(map[string]interface{}), idByInternalID, graph)
	}

	return graph, nil
}

// parseResource parses a single resource from the spec.resources array.
func (p *ResourceParser) parseResource(index int, resMap map[string]interface{}) *ResourceDefinition {
	res := &ResourceDefinition{
		DependsOn: make([]string, 0),
	}

	// Check if this is a template or externalRef using type-safe accessors
	template, err := k8sparser.GetMap(resMap, "template")
	if err == nil {
		// Template resource
		res.IsTemplate = true
		res.APIVersion, res.Kind = extractAPIVersionKind(template)
	} else if extRef, err := k8sparser.GetMap(resMap, "externalRef"); err == nil {
		// ExternalRef resource
		res.IsTemplate = false
		res.ExternalRef = p.parseExternalRef(extRef)
		if res.ExternalRef != nil {
			res.APIVersion = res.ExternalRef.APIVersion
			res.Kind = res.ExternalRef.Kind
		}
	} else {
		// Unknown resource type
		return nil
	}

	// Generate ID
	res.ID = fmt.Sprintf("%d-%s", index, res.Kind)

	// Collect all schema.spec.* field references from the resource
	if res.IsTemplate {
		res.SchemaFields = extractSchemaFieldRefs(template)
	} else if extRef, err := k8sparser.GetMap(resMap, "externalRef"); err == nil {
		res.SchemaFields = extractSchemaFieldRefs(extRef)
	}

	// Parse includeWhen condition - can be a string or array of strings
	if includeWhen, err := k8sparser.GetString(resMap, "includeWhen"); err == nil && includeWhen != "" {
		res.IncludeWhen = p.parseCondition(includeWhen)
	} else if includeWhenArr, err := k8sparser.GetSlice(resMap, "includeWhen"); err == nil && len(includeWhenArr) > 0 {
		// KRO spec defines includeWhen as an array of CEL expressions
		// For now, we join them with && for combined condition
		var conditions []string
		for _, cond := range includeWhenArr {
			if condStr, ok := cond.(string); ok && condStr != "" {
				conditions = append(conditions, condStr)
			}
		}
		if len(conditions) > 0 {
			res.IncludeWhen = p.parseCondition(strings.Join(conditions, " && "))
		}
	}

	return res
}

// parseExternalRef parses an externalRef section.
func (p *ResourceParser) parseExternalRef(extRef map[string]interface{}) *ExternalRefInfo {
	info := &ExternalRefInfo{}

	info.APIVersion, info.Kind = extractAPIVersionKind(extRef)

	// Extract name/namespace from metadata (KRO spec nests these under metadata)
	metadata, err := k8sparser.GetMap(extRef, "metadata")
	if err == nil {
		// Extract name expression from metadata
		if nameExpr, err := k8sparser.GetString(metadata, "name"); err == nil {
			info.NameExpr = nameExpr

			// Check if name uses schema.spec.*
			if matches := schemaSpecPattern.FindStringSubmatch(nameExpr); len(matches) > 1 {
				info.UsesSchemaSpec = true
				info.SchemaField = "spec." + matches[1]
			}
		}

		// Extract namespace expression from metadata
		if nsExpr, err := k8sparser.GetString(metadata, "namespace"); err == nil {
			info.NamespaceExpr = nsExpr

			// Check if namespace also uses schema.spec.*
			if matches := schemaSpecPattern.FindStringSubmatch(nsExpr); len(matches) > 1 {
				info.NamespaceSchemaField = "spec." + matches[1]
			}
		}
	} else {
		// Fallback: try direct name/namespace fields (legacy format)
		if nameExpr, err := k8sparser.GetString(extRef, "name"); err == nil {
			info.NameExpr = nameExpr

			// Check if name uses schema.spec.*
			if matches := schemaSpecPattern.FindStringSubmatch(nameExpr); len(matches) > 1 {
				info.UsesSchemaSpec = true
				info.SchemaField = "spec." + matches[1]
			}
		}

		if nsExpr, err := k8sparser.GetString(extRef, "namespace"); err == nil {
			info.NamespaceExpr = nsExpr

			// Check if namespace also uses schema.spec.*
			if matches := schemaSpecPattern.FindStringSubmatch(nsExpr); len(matches) > 1 {
				info.NamespaceSchemaField = "spec." + matches[1]
			}
		}
	}

	return info
}

// parseCondition parses an includeWhen CEL expression.
func (p *ResourceParser) parseCondition(expr string) *ConditionExpr {
	condition := &ConditionExpr{
		Expression:   expr,
		SchemaFields: make([]string, 0),
	}

	// Extract all schema.spec.* field references
	matches := schemaFieldPattern.FindAllStringSubmatch(expr, -1)
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			field := "spec." + match[1]
			if !seen[field] {
				seen[field] = true
				condition.SchemaFields = append(condition.SchemaFields, field)
			}
		}
	}

	return condition
}

// resolveDependencies finds internal dependencies between resources within the RGD.
func (p *ResourceParser) resolveDependencies(res *ResourceDefinition, resMap map[string]interface{}, idByInternalID map[string]string, graph *ResourceGraph) {
	// Use type-safe accessors to get template or externalRef
	content, err := k8sparser.GetMap(resMap, "template")
	if err != nil {
		content, err = k8sparser.GetMap(resMap, "externalRef")
		if err != nil {
			return
		}
	}

	// Find all CEL expressions that reference other resources in the RGD
	depIDs := p.findInternalReferences(content, idByInternalID)

	for depID := range depIDs {
		if depID != res.ID {
			res.DependsOn = append(res.DependsOn, depID)
			graph.Edges = append(graph.Edges, ResourceEdge{
				From: res.ID,
				To:   depID,
				Type: "reference",
			})
		}
	}
}

// findInternalReferences finds references to other resources within the RGD.
func (p *ResourceParser) findInternalReferences(data interface{}, idByInternalID map[string]string) map[string]bool {
	refs := make(map[string]bool)

	p.traverseForReferences(data, idByInternalID, refs, 0)

	return refs
}

// traverseForReferences recursively traverses the data to find CEL expressions.
// Depth parameter prevents stack overflow from deeply nested structures.
func (p *ResourceParser) traverseForReferences(data interface{}, idByInternalID map[string]string, refs map[string]bool, depth int) {
	if depth > maxRecursionDepth {
		return // Prevent stack overflow
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for _, value := range v {
			p.traverseForReferences(value, idByInternalID, refs, depth+1)
		}
	case []interface{}:
		for _, item := range v {
			p.traverseForReferences(item, idByInternalID, refs, depth+1)
		}
	case string:
		// Look for CEL expressions referencing other resources
		matches := celExpressionPattern.FindAllStringSubmatch(v, -1)
		for _, match := range matches {
			if len(match) > 1 {
				inner := match[1]
				// Check if this references another resource (not schema.*, spec.*, etc.)
				for internalID, resID := range idByInternalID {
					if strings.HasPrefix(inner, internalID+".") {
						refs[resID] = true
					}
				}
			}
		}
	}
}

// extractAPIVersionKind extracts apiVersion and kind from a resource map.
func extractAPIVersionKind(resMap map[string]interface{}) (string, string) {
	apiVersion := k8sparser.GetStringOrDefault(resMap, "", "apiVersion")
	kind := k8sparser.GetStringOrDefault(resMap, "", "kind")
	return apiVersion, kind
}

// extractInternalID extracts the internal ID used for referencing this resource.
// KRO uses the resource index or an explicit id field.
func extractInternalID(resMap map[string]interface{}) string {
	// Check for explicit id field using type-safe accessor
	if id, err := k8sparser.GetString(resMap, "id"); err == nil && id != "" {
		return id
	}

	// Try to extract from template or externalRef using type-safe accessors
	content, err := k8sparser.GetMap(resMap, "template")
	if err != nil {
		content, err = k8sparser.GetMap(resMap, "externalRef")
		if err != nil {
			return ""
		}
	}

	// Use kind as the internal ID (common pattern in KRO)
	kind := k8sparser.GetStringOrDefault(content, "", "kind")
	return strings.ToLower(kind)
}

// extractSchemaFieldRefs traverses a resource map and collects all schema.spec.* field references.
// It finds ${schema.spec.*} patterns in all string values within the resource definition.
func extractSchemaFieldRefs(data interface{}) []string {
	seen := make(map[string]bool)
	collectSchemaRefs(data, seen, 0)

	if len(seen) == 0 {
		return nil
	}

	refs := make([]string, 0, len(seen))
	for field := range seen {
		refs = append(refs, field)
	}
	return refs
}

// collectSchemaRefs recursively traverses data looking for ${schema.spec.*} CEL expressions.
func collectSchemaRefs(data interface{}, seen map[string]bool, depth int) {
	if depth > maxRecursionDepth {
		return
	}

	switch v := data.(type) {
	case map[string]interface{}:
		for _, value := range v {
			collectSchemaRefs(value, seen, depth+1)
		}
	case []interface{}:
		for _, item := range v {
			collectSchemaRefs(item, seen, depth+1)
		}
	case string:
		matches := schemaSpecPattern.FindAllStringSubmatch(v, -1)
		for _, match := range matches {
			if len(match) > 1 {
				field := "spec." + match[1]
				seen[field] = true
			}
		}
	}
}

// GetResourceByID finds a resource by its ID in the graph.
// Note: Primarily used for testing purposes
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
