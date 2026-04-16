// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package graph provides a thin adapter layer between KRO's native *graph.Graph
// and Knodex's UI-oriented resource graph types.
package graph

import (
	"fmt"
	"sort"
	"strings"

	krograph "github.com/kubernetes-sigs/kro/pkg/graph"

	"github.com/knodex/knodex/server/internal/kro/parser"
)

// UIGraphAdapter wraps a KRO *graph.Graph and exposes data in the shape
// expected by Knodex handlers, enricher, and frontend consumers.
//
// It produces the same types (parser.ResourceDefinition, parser.ResourceEdge, etc.)
// so downstream consumers can be migrated incrementally.
type UIGraphAdapter struct {
	graph *krograph.Graph
}

// NewUIGraphAdapter creates a new adapter wrapping the given KRO graph.
// Returns nil if g is nil.
func NewUIGraphAdapter(g *krograph.Graph) *UIGraphAdapter {
	if g == nil {
		return nil
	}
	return &UIGraphAdapter{graph: g}
}

// Graph returns the underlying KRO graph.
func (a *UIGraphAdapter) Graph() *krograph.Graph {
	return a.graph
}

// GetResourceGraph converts the KRO graph into a parser.ResourceGraph
// compatible with all existing consumers.
func (a *UIGraphAdapter) GetResourceGraph(rgdName, rgdNamespace string, rawSpec map[string]interface{}) *parser.ResourceGraph {
	if a == nil || a.graph == nil {
		return &parser.ResourceGraph{
			RGDName:      rgdName,
			RGDNamespace: rgdNamespace,
			Resources:    make([]parser.ResourceDefinition, 0),
			Edges:        make([]parser.ResourceEdge, 0),
		}
	}

	resources := a.GetResources()
	edges := a.GetEdges()
	secretRefs := ExtractSecretRefs(a.graph, rawSpec)

	return &parser.ResourceGraph{
		RGDName:      rgdName,
		RGDNamespace: rgdNamespace,
		Resources:    resources,
		Edges:        edges,
		SecretRefs:   secretRefs,
	}
}

// GetResources converts all KRO nodes to parser.ResourceDefinition entries.
// The instance node is excluded. Resources are ordered by their original index.
// DependsOn entries are converted from KRO internal IDs to UI-format IDs ("{index}-{kind}").
func (a *UIGraphAdapter) GetResources() []parser.ResourceDefinition {
	if a == nil || a.graph == nil {
		return nil
	}

	// Build a lookup map: KRO internal node ID → UI format nodeID.
	// This is required to convert DependsOn entries, which KRO stores as internal node
	// IDs (e.g. "vpc", "deployment1"), into the UI format ("{index}-{kind}") that the
	// frontend uses as primary keys for nodes and layout.
	internalToUIID := make(map[string]string, len(a.graph.Nodes))
	for id, node := range a.graph.Nodes {
		if node.Meta.Type != krograph.NodeTypeInstance {
			internalToUIID[id] = nodeID(node)
		}
	}

	// Collect non-instance nodes
	nodes := make([]*krograph.Node, 0, len(a.graph.Nodes))
	for _, node := range a.graph.Nodes {
		if node.Meta.Type == krograph.NodeTypeInstance {
			continue
		}
		nodes = append(nodes, node)
	}

	// Sort by original index for deterministic ordering
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Meta.Index < nodes[j].Meta.Index
	})

	resources := make([]parser.ResourceDefinition, 0, len(nodes))
	for _, node := range nodes {
		rd := nodeToResourceDefinition(node)
		// Translate DependsOn from KRO internal IDs to UI-format IDs
		for i, depID := range rd.DependsOn {
			if uiID, ok := internalToUIID[depID]; ok {
				rd.DependsOn[i] = uiID
			}
		}
		resources = append(resources, rd)
	}
	return resources
}

// GetEdges builds dependency edges from node dependencies.
// Edges are ordered deterministically by source node index, then dependency order.
func (a *UIGraphAdapter) GetEdges() []parser.ResourceEdge {
	if a == nil || a.graph == nil {
		return nil
	}

	// Collect and sort nodes by index for deterministic edge order
	nodes := make([]*krograph.Node, 0, len(a.graph.Nodes))
	for _, node := range a.graph.Nodes {
		if node.Meta.Type != krograph.NodeTypeInstance {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Meta.Index < nodes[j].Meta.Index
	})

	var edges []parser.ResourceEdge
	for _, node := range nodes {
		fromID := nodeID(node)
		for _, depID := range node.Meta.Dependencies {
			depNode, ok := a.graph.Nodes[depID]
			if !ok || depNode.Meta.Type == krograph.NodeTypeInstance {
				continue
			}
			edges = append(edges, parser.ResourceEdge{
				From: fromID,
				To:   nodeID(depNode),
				Type: "reference",
			})
		}
	}
	return edges
}

// GetTopologicalOrder returns the topological sort order from the KRO graph.
// Node IDs are converted to the UI format ("{index}-{kind}").
func (a *UIGraphAdapter) GetTopologicalOrder() []string {
	if a == nil || a.graph == nil {
		return nil
	}

	order := make([]string, 0, len(a.graph.TopologicalOrder))
	for _, id := range a.graph.TopologicalOrder {
		node, ok := a.graph.Nodes[id]
		if !ok {
			continue
		}
		order = append(order, nodeID(node))
	}
	return order
}

// GetExternalRefs returns all external reference resources, ordered by index.
func (a *UIGraphAdapter) GetExternalRefs() []parser.ResourceDefinition {
	if a == nil || a.graph == nil {
		return nil
	}

	var nodes []*krograph.Node
	for _, node := range a.graph.Nodes {
		if node.Meta.Type == krograph.NodeTypeExternal || node.Meta.Type == krograph.NodeTypeExternalCollection {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Meta.Index < nodes[j].Meta.Index
	})

	refs := make([]parser.ResourceDefinition, 0, len(nodes))
	for _, node := range nodes {
		refs = append(refs, nodeToResourceDefinition(node))
	}
	return refs
}

// GetConditionalResources returns all resources with includeWhen conditions, ordered by index.
func (a *UIGraphAdapter) GetConditionalResources() []parser.ResourceDefinition {
	if a == nil || a.graph == nil {
		return nil
	}

	var nodes []*krograph.Node
	for _, node := range a.graph.Nodes {
		if node.Meta.Type == krograph.NodeTypeInstance {
			continue
		}
		if len(node.IncludeWhen) > 0 {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Meta.Index < nodes[j].Meta.Index
	})

	conditional := make([]parser.ResourceDefinition, 0, len(nodes))
	for _, node := range nodes {
		conditional = append(conditional, nodeToResourceDefinition(node))
	}
	return conditional
}

// GetCollectionResources returns all resources that use forEach expansion, ordered by index.
func (a *UIGraphAdapter) GetCollectionResources() []parser.ResourceDefinition {
	if a == nil || a.graph == nil {
		return nil
	}

	var nodes []*krograph.Node
	for _, node := range a.graph.Nodes {
		if node.Meta.Type == krograph.NodeTypeCollection {
			nodes = append(nodes, node)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Meta.Index < nodes[j].Meta.Index
	})

	collections := make([]parser.ResourceDefinition, 0, len(nodes))
	for _, node := range nodes {
		collections = append(collections, nodeToResourceDefinition(node))
	}
	return collections
}

// GetResourceByID finds a resource by its UI ID ("{index}-{kind}").
func (a *UIGraphAdapter) GetResourceByID(id string) *parser.ResourceDefinition {
	if a == nil || a.graph == nil {
		return nil
	}

	for _, node := range a.graph.Nodes {
		if node.Meta.Type == krograph.NodeTypeInstance {
			continue
		}
		if nodeID(node) == id {
			rd := nodeToResourceDefinition(node)
			return &rd
		}
	}
	return nil
}

// nodeID returns the UI-format ID for a node: "{index}-{kind}".
func nodeID(node *krograph.Node) string {
	kind := ""
	if node.Template != nil {
		kind = node.Template.GetKind()
	}
	return fmt.Sprintf("%d-%s", node.Meta.Index, kind)
}

// nodeToResourceDefinition converts a KRO Node to a parser.ResourceDefinition.
func nodeToResourceDefinition(node *krograph.Node) parser.ResourceDefinition {
	rd := parser.ResourceDefinition{
		ID:        nodeID(node),
		DependsOn: make([]string, 0),
	}

	if node.Template != nil {
		rd.APIVersion = node.Template.GetAPIVersion()
		rd.Kind = node.Template.GetKind()
	}

	// Map node type to our UI classifications
	switch node.Meta.Type {
	case krograph.NodeTypeResource:
		rd.IsTemplate = true
	case krograph.NodeTypeCollection:
		rd.IsTemplate = true
		rd.IsCollection = true
	case krograph.NodeTypeExternal, krograph.NodeTypeExternalCollection:
		rd.IsTemplate = false
		rd.ExternalRef = buildExternalRefInfo(node)
	}

	// Map dependencies — raw KRO internal IDs are stored here; the caller
	// (GetResources) translates them to UI-format IDs via the internalToUIID map.
	for _, depID := range node.Meta.Dependencies {
		// Skip schema/instance dependencies — they're implicit
		if depID == krograph.InstanceNodeID {
			continue
		}
		rd.DependsOn = append(rd.DependsOn, depID)
	}

	// Map includeWhen conditions
	if len(node.IncludeWhen) > 0 {
		var expressions []string
		var schemaFields []string
		for _, expr := range node.IncludeWhen {
			if expr != nil {
				expressions = append(expressions, expr.Original)
				// Extract schema.spec.* fields from the expression
				fields := extractBareSchemaFields(expr.Original)
				schemaFields = append(schemaFields, fields...)
			}
		}
		rd.IncludeWhen = &parser.ConditionExpr{
			Expression:   strings.Join(expressions, " && "),
			SchemaFields: schemaFields,
		}
	}

	// Map readyWhen expressions
	for _, expr := range node.ReadyWhen {
		if expr != nil {
			rd.ReadyWhen = append(rd.ReadyWhen, expr.Original)
		}
	}

	// Map forEach dimensions
	for i, dim := range node.ForEach {
		exprStr := ""
		if dim.Expression != nil {
			exprStr = dim.Expression.Original
		}
		source, sourcePath := analyzeForEachSource(exprStr)
		rd.ForEach = append(rd.ForEach, parser.Iterator{
			Name:           dim.Name,
			Expression:     exprStr,
			Source:         source,
			SourcePath:     sourcePath,
			DimensionIndex: i,
		})
	}

	// Extract schema field references from variables
	rd.SchemaFields = extractSchemaFieldsFromVariables(node)

	return rd
}

// buildExternalRefInfo constructs ExternalRefInfo from a KRO external node.
func buildExternalRefInfo(node *krograph.Node) *parser.ExternalRefInfo {
	if node.Template == nil {
		return nil
	}

	info := &parser.ExternalRefInfo{
		APIVersion: node.Template.GetAPIVersion(),
		Kind:       node.Template.GetKind(),
	}

	// Extract name/namespace expressions from the template metadata
	if metadata, ok := node.Template.Object["metadata"].(map[string]interface{}); ok {
		if nameExpr, ok := metadata["name"].(string); ok {
			info.NameExpr = nameExpr
			extractSchemaSpecFromExpr(nameExpr, &info.UsesSchemaSpec, &info.SchemaField)
		}
		if nsExpr, ok := metadata["namespace"].(string); ok {
			info.NamespaceExpr = nsExpr
			extractSchemaSpecFromExpr(nsExpr, nil, &info.NamespaceSchemaField)
		}
	}

	return info
}

// extractSchemaFieldsFromVariables extracts schema.spec.* field references from a node's variables.
func extractSchemaFieldsFromVariables(node *krograph.Node) []string {
	if node.Variables == nil {
		return nil
	}

	seen := make(map[string]bool)
	for _, v := range node.Variables {
		if v.Expression != nil {
			fields := extractBareSchemaFields(v.Expression.Original)
			for _, f := range fields {
				seen[f] = true
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}

	result := make([]string, 0, len(seen))
	for f := range seen {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// schemaSpecExprPrefix is the prefix for schema spec references in CEL expressions.
const schemaSpecExprPrefix = "schema.spec."

// extractBareSchemaFields finds all schema.spec.* references in a plain string.
// Returns field paths with "spec." prefix (e.g., "spec.name").
func extractBareSchemaFields(s string) []string {
	var fields []string
	searchFrom := 0

	for searchFrom < len(s) {
		idx := strings.Index(s[searchFrom:], schemaSpecExprPrefix)
		if idx == -1 {
			break
		}
		idx += searchFrom

		fieldStart := idx + len(schemaSpecExprPrefix)
		fieldEnd := fieldStart
		for fieldEnd < len(s) {
			c := s[fieldEnd]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
				c == '.' || c == '_' || c == '-' {
				fieldEnd++
			} else {
				break
			}
		}

		if fieldEnd > fieldStart {
			field := "spec." + s[fieldStart:fieldEnd]
			field = strings.TrimRight(field, ".")
			fields = append(fields, field)
		}

		searchFrom = fieldEnd
	}

	return fields
}

// extractSchemaSpecFromExpr extracts a schema.spec.* field reference from a value string.
func extractSchemaSpecFromExpr(expr string, usesSchema *bool, fieldOut *string) {
	fields := extractBareSchemaFields(expr)
	if len(fields) > 0 {
		if usesSchema != nil {
			*usesSchema = true
		}
		*fieldOut = fields[0]
	}
}

// analyzeForEachSource determines the SourceType and field path for a forEach CEL expression.
func analyzeForEachSource(expr string) (parser.SourceType, string) {
	bare := stripDelimiters(expr)
	if bare == "" {
		return parser.LiteralSource, ""
	}

	// Schema source: schema.spec.fieldPath
	if strings.HasPrefix(bare, schemaSpecExprPrefix) {
		fieldPath := "spec." + bare[len(schemaSpecExprPrefix):]
		if end := strings.IndexAny(fieldPath, " \t()[]{}!=<>+-*/,"); end != -1 {
			fieldPath = fieldPath[:end]
		}
		fieldPath = strings.TrimRight(fieldPath, ".")
		return parser.SchemaSource, fieldPath
	}

	// Literals start with '[', '{', '"', or a digit
	if len(bare) > 0 {
		first := bare[0]
		if first == '[' || first == '{' || first == '"' || (first >= '0' && first <= '9') {
			return parser.LiteralSource, ""
		}

		// Resource source: <resourceID>.<rest>
		dotIdx := strings.Index(bare, ".")
		if dotIdx > 0 {
			prefix := bare[:dotIdx]
			rest := bare[dotIdx+1:]
			if prefix != "schema" {
				if end := strings.IndexAny(rest, " \t()[]{}!=<>+-*/,"); end != -1 {
					rest = rest[:end]
				}
				rest = strings.TrimRight(rest, ".")
				return parser.ResourceSource, rest
			}
		}
	}

	return parser.LiteralSource, ""
}

// stripDelimiters removes the ${...} wrapper from a CEL expression.
func stripDelimiters(expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		return strings.TrimSpace(expr[2 : len(expr)-1])
	}
	return expr
}
