// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	k8sparser "github.com/knodex/knodex/server/internal/k8s/parser"

	kroparser "github.com/kubernetes-sigs/kro/pkg/graph/parser"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
)

// maxRecursionDepth prevents stack overflow from deeply nested objects
const maxRecursionDepth = 100

// schemaSpecExprPrefix is the prefix for schema spec references inside CEL expressions.
const schemaSpecExprPrefix = "schema.spec."

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

	// Track original resource indices for the second pass (dependency resolution)
	// since skipped resources cause graph.Resources indices to diverge from resources indices.
	type resourceEntry struct {
		graphIdx    int
		originalMap map[string]interface{}
	}
	var entries []resourceEntry

	// First pass: extract all resource definitions
	for i, res := range resources {
		resMap, ok := res.(map[string]interface{})
		if !ok {
			continue
		}

		resDef, parseErr := p.parseResource(i, resMap)
		if parseErr != nil {
			graph.ParseErrors = append(graph.ParseErrors, *parseErr)
		}
		if resDef != nil {
			graph.Resources = append(graph.Resources, *resDef)
			entries = append(entries, resourceEntry{
				graphIdx:    len(graph.Resources) - 1,
				originalMap: resMap,
			})

			// Map internal ID to our generated ID
			if internalID := extractInternalID(resMap); internalID != "" {
				idByInternalID[internalID] = resDef.ID
			}
		}
	}

	// Second pass: resolve internal dependencies and build edges
	for _, entry := range entries {
		res := &graph.Resources[entry.graphIdx]
		p.resolveDependencies(res, entry.originalMap, idByInternalID, graph)
	}

	// Build reverse map: graph resource ID → internal ID (e.g., "0-Secret" → "dbSecret")
	resIDToInternalID := make(map[string]string, len(idByInternalID))
	for internalID, resID := range idByInternalID {
		resIDToInternalID[resID] = internalID
	}

	// Extract schema.spec.externalRef map for description lookup
	schemaExternalRefs := extractSchemaExternalRefMap(rgdSpec)

	// Extract secret references from externalRef resources
	graph.SecretRefs = extractSecretRefs(graph, schemaExternalRefs, resIDToInternalID)

	return graph, nil
}

// parseResource parses a single resource from the spec.resources array.
// Returns the parsed definition and an optional non-fatal parse error (e.g., invalid forEach+externalRef).
// A non-nil error does not prevent the resource from being returned; callers accumulate errors in graph.ParseErrors.
func (p *ResourceParser) parseResource(index int, resMap map[string]interface{}) (*ResourceDefinition, *ParseError) {
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
		return nil, nil
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
		res.IncludeWhen = parseCondition(includeWhen)
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
			res.IncludeWhen = parseCondition(strings.Join(conditions, " && "))
		}
	}

	// Parse forEach collection iterators (mutually exclusive with externalRef per KRO spec)
	hasExternalRef := !res.IsTemplate && res.ExternalRef != nil
	iterators, parseErr := p.parseForEach(resMap, hasExternalRef)
	if parseErr != nil {
		parseErr.Path = res.ID
		// Still return the resource; caller accumulates the error
		return res, parseErr
	}
	res.ForEach = iterators
	res.IsCollection = len(iterators) > 0

	// Parse readyWhen - store raw strings, including "each.*" for collections
	if readyWhenSlice, err := k8sparser.GetSlice(resMap, "readyWhen"); err == nil {
		for _, rw := range readyWhenSlice {
			if s, ok := rw.(string); ok && s != "" {
				res.ReadyWhen = append(res.ReadyWhen, s)
			}
		}
	}

	return res, nil
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
			extractSchemaSpecFromExpr(nameExpr, &info.UsesSchemaSpec, &info.SchemaField)
		}

		// Extract namespace expression from metadata
		if nsExpr, err := k8sparser.GetString(metadata, "namespace"); err == nil {
			info.NamespaceExpr = nsExpr
			extractSchemaSpecFromExpr(nsExpr, nil, &info.NamespaceSchemaField)
		}
	} else {
		// Fallback: try direct name/namespace fields (legacy format)
		if nameExpr, err := k8sparser.GetString(extRef, "name"); err == nil {
			info.NameExpr = nameExpr
			extractSchemaSpecFromExpr(nameExpr, &info.UsesSchemaSpec, &info.SchemaField)
		}

		if nsExpr, err := k8sparser.GetString(extRef, "namespace"); err == nil {
			info.NamespaceExpr = nsExpr
			extractSchemaSpecFromExpr(nsExpr, nil, &info.NamespaceSchemaField)
		}
	}

	return info
}

// extractSchemaSpecFromExpr extracts a schema.spec.* field reference from a CEL expression string.
// Uses KRO's ParseSchemalessResource to properly extract expressions from ${...} wrappers,
// handling nested braces and string literals correctly.
// If usesSchema is non-nil, it is set to true when a schema.spec.* reference is found.
// The extracted field (with "spec." prefix) is stored in fieldOut.
func extractSchemaSpecFromExpr(expr string, usesSchema *bool, fieldOut *string) {
	exprs := extractExpressionsFromValue(expr)
	for _, e := range exprs {
		// Use extractBareSchemaFields (substring-aware) instead of HasPrefix because
		// KRO v0.9.0 compiles embedded templates into concatenation expressions where
		// schema.spec.* no longer appears at position 0.
		fields := extractBareSchemaFields(e)
		if len(fields) > 0 {
			if usesSchema != nil {
				*usesSchema = true
			}
			*fieldOut = fields[0]
			return
		}
	}
}

// extractExpressionsFromValue extracts CEL expression contents from a string value
// using KRO's parser. Returns the Expression.Original strings from each FieldDescriptor.
// For standalone expressions, e.g., "${schema.spec.name}" returns ["schema.spec.name"].
// For embedded templates (KRO v0.9.0+), e.g., "prefix-${schema.spec.name}-suffix",
// returns a single concatenated CEL expression like ["\"prefix-\" + (schema.spec.name) + \"-suffix\""].
func extractExpressionsFromValue(value string) []string {
	if !strings.Contains(value, "${") {
		return nil
	}

	descriptors, _, err := kroparser.ParseSchemalessResource(map[string]interface{}{"v": value})
	if err != nil {
		slog.Debug("KRO ParseSchemalessResource failed for value", "error", err, "value", value)
		return nil
	}

	var exprs []string
	for _, fd := range descriptors {
		if fd.Expression != nil {
			exprs = append(exprs, fd.Expression.Original)
		}
	}
	return exprs
}

// parseCondition parses an includeWhen CEL expression.
func parseCondition(expr string) *ConditionExpr {
	condition := &ConditionExpr{
		Expression:   expr,
		SchemaFields: make([]string, 0),
	}

	// Extract all schema.spec.* field references from the expression.
	// Conditions may contain bare references (schema.spec.X == true) or
	// wrapped expressions (${schema.spec.X == true}).
	// Try KRO extraction first for wrapped expressions, fall back to bare parsing.
	fields := extractSchemaFieldNames(expr)

	seen := make(map[string]bool)
	for _, field := range fields {
		if !seen[field] {
			seen[field] = true
			condition.SchemaFields = append(condition.SchemaFields, field)
		}
	}

	return condition
}

// extractSchemaFieldNames extracts all schema.spec.* field names from a string.
// Handles both bare references (schema.spec.X) and ${}-wrapped expressions.
// Returns field paths with "spec." prefix (e.g., "spec.ingress.enabled").
func extractSchemaFieldNames(s string) []string {
	var fields []string

	// First, try extracting from ${}-wrapped expressions using KRO's parser
	if strings.Contains(s, "${") {
		exprs := extractExpressionsFromValue(s)
		for _, expr := range exprs {
			fields = append(fields, extractBareSchemaFields(expr)...)
		}
		if len(fields) > 0 {
			return fields
		}
	}

	// Fall back to bare schema.spec.* references (for conditions without ${} wrapper)
	return extractBareSchemaFields(s)
}

// extractBareSchemaFields finds all schema.spec.* references in a plain string
// (no ${} wrappers). Returns field paths with "spec." prefix.
func extractBareSchemaFields(s string) []string {
	var fields []string
	searchFrom := 0

	for searchFrom < len(s) {
		idx := strings.Index(s[searchFrom:], schemaSpecExprPrefix)
		if idx == -1 {
			break
		}
		idx += searchFrom

		// Extract the field name: valid chars are alphanumeric, dot, underscore, hyphen
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
			// Trim trailing dots that might be syntax artifacts
			field = strings.TrimRight(field, ".")
			fields = append(fields, field)
		}

		searchFrom = fieldEnd
	}

	return fields
}

// extractSecretRefs filters externalRef resources for Secret kind and classifies them.
// schemaExternalRefs is the parsed schema.spec.externalRef map for description lookup.
// resIDToInternalID maps resource graph IDs (e.g., "0-Secret") to their internal IDs (e.g., "dbSecret").
func extractSecretRefs(graph *ResourceGraph, schemaExternalRefs map[string]interface{}, resIDToInternalID map[string]string) []SecretRef {
	refs := make([]SecretRef, 0)
	for _, res := range graph.Resources {
		if res.IsTemplate || res.ExternalRef == nil {
			continue
		}
		// Kubernetes kinds are case-sensitive PascalCase — exact match required.
		if res.ExternalRef.Kind != "Secret" {
			continue
		}

		ref := SecretRef{ID: res.ID}

		// Extract semantic name and description first — we need internalID for passthrough detection.
		internalID := ""
		if id, ok := resIDToInternalID[res.ID]; ok {
			internalID = id
			ref.ExternalRefID = id
			ref.Description = extractExternalRefDescription(schemaExternalRefs, id)
		}

		// Classify the secret ref type:
		//
		// "provided" — passthrough input pattern: both name and namespace reference
		//   ${schema.spec.externalRef.<id>.name} / ${schema.spec.externalRef.<id>.namespace}.
		//   The user supplies the secret name/namespace at deploy time; showing the CEL
		//   expression is meaningless in the catalog detail view.
		//
		// "dynamic"  — computed from other resources (non-passthrough CEL expression).
		//   e.g., ${schema.metadata.namespace} or ${someResource.metadata.name}.
		//
		// "fixed"    — literal string, no expressions.
		nameExpr := res.ExternalRef.NameExpr
		nsExpr := res.ExternalRef.NamespaceExpr
		isPassthrough := internalID != "" &&
			nameExpr == fmt.Sprintf("${schema.spec.externalRef.%s.name}", internalID) &&
			(nsExpr == "" || nsExpr == fmt.Sprintf("${schema.spec.externalRef.%s.namespace}", internalID))
		isDynamic := !isPassthrough && (strings.Contains(nameExpr, "${") || strings.Contains(nsExpr, "${"))

		switch {
		case isPassthrough:
			ref.Type = "provided"
		case isDynamic:
			ref.Type = "dynamic"
			ref.NameExpr = nameExpr
			ref.NamespaceExpr = nsExpr
		default:
			ref.Type = "fixed"
			ref.Name = nameExpr
			ref.Namespace = nsExpr
		}

		refs = append(refs, ref)
	}
	return refs
}

// extractSchemaExternalRefMap extracts the schema.spec.externalRef map from an RGD spec.
func extractSchemaExternalRefMap(rgdSpec map[string]interface{}) map[string]interface{} {
	schemaMap, err := k8sparser.GetMap(rgdSpec, "schema")
	if err != nil {
		return nil
	}
	specMap, err := k8sparser.GetMap(schemaMap, "spec")
	if err != nil {
		return nil
	}
	extRefMap, err := k8sparser.GetMap(specMap, "externalRef")
	if err != nil {
		return nil
	}
	return extRefMap
}

// extractExternalRefDescription extracts the description from a schema externalRef field.
// It looks up the "name" sub-field's description marker in the SimpleSchema definition.
//
// Why the "name" sub-field? In KRO SimpleSchema, the parent object (e.g., "dbSecret: {...}")
// has no description slot of its own — descriptions live on leaf fields. The "name" sub-field
// describes what the secret is (e.g., "Name of the K8s Secret containing DB credentials"),
// which best characterizes the secret's purpose. The "namespace" sub-field describes where it
// lives, which is secondary. This convention matches the webapp-with-secret RGD example.
func extractExternalRefDescription(schemaExternalRefs map[string]interface{}, fieldName string) string {
	if schemaExternalRefs == nil || fieldName == "" {
		return ""
	}
	fieldMap, ok := schemaExternalRefs[fieldName].(map[string]interface{})
	if !ok {
		return ""
	}
	nameValue, ok := fieldMap["name"].(string)
	if !ok {
		return ""
	}
	// Parse simpleschema field definition to extract description marker
	_, markers, err := simpleschema.ParseField(nameValue)
	if err != nil {
		return ""
	}
	for _, m := range markers {
		if m.MarkerType == simpleschema.MarkerTypeDescription {
			return m.Value
		}
	}
	return ""
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

// findInternalReferences finds references to other resources within the RGD
// using KRO's ParseSchemalessResource for expression extraction.
func (p *ResourceParser) findInternalReferences(data interface{}, idByInternalID map[string]string) map[string]bool {
	refs := make(map[string]bool)

	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return refs
	}

	// Use KRO's parser to extract all expressions from the resource
	descriptors, _, err := kroparser.ParseSchemalessResource(dataMap)
	if err != nil {
		slog.Debug("KRO ParseSchemalessResource failed, falling back to manual traversal", "error", err)
		p.traverseForReferences(data, idByInternalID, refs, 0)
		return refs
	}

	for _, fd := range descriptors {
		if fd.Expression != nil {
			inner := fd.Expression.Original
			// Check if this references another resource (not schema.*, spec.*, etc.)
			// Use isResourceRef instead of HasPrefix because KRO v0.9.0 compiles
			// string templates into single concatenation expressions, e.g.,
			// "${a.b}-${c.d}" becomes '(a.b) + "-" + (c.d)'.
			for internalID, resID := range idByInternalID {
				if isResourceRef(inner, internalID) {
					refs[resID] = true
				}
			}
		}
	}

	return refs
}

// traverseForReferences is the fallback recursive traversal for finding CEL expressions.
// Used only when KRO's ParseSchemalessResource encounters an error.
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
		// Use KRO's parser via extractExpressionsFromValue for individual strings
		exprs := extractExpressionsFromValue(v)
		for _, inner := range exprs {
			for internalID, resID := range idByInternalID {
				if isResourceRef(inner, internalID) {
					refs[resID] = true
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

// extractSchemaFieldRefs traverses a resource map and collects all schema.spec.* field references
// using KRO's ParseSchemalessResource for proper expression extraction.
func extractSchemaFieldRefs(data interface{}) []string {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	// Use KRO's parser to extract all expressions
	descriptors, _, err := kroparser.ParseSchemalessResource(dataMap)
	if err != nil {
		slog.Debug("KRO ParseSchemalessResource failed for schema field extraction, falling back to manual traversal", "error", err)
		seen := make(map[string]bool)
		collectSchemaRefs(data, seen, 0)
		return mapToSlice(seen)
	}

	seen := make(map[string]bool)
	for _, fd := range descriptors {
		if fd.Expression != nil {
			inner := fd.Expression.Original
			// Extract schema.spec.* references from the expression
			for _, field := range extractBareSchemaFields(inner) {
				seen[field] = true
			}
		}
	}

	if len(seen) == 0 {
		return nil
	}
	return mapToSlice(seen)
}

// collectSchemaRefs is the fallback recursive traversal for schema.spec.* extraction.
// Used only when KRO's ParseSchemalessResource encounters an error.
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
		exprs := extractExpressionsFromValue(v)
		for _, inner := range exprs {
			for _, field := range extractBareSchemaFields(inner) {
				seen[field] = true
			}
		}
	}
}

// isResourceRef reports whether expr contains a top-level reference to internalID
// (i.e., internalID followed immediately by ".") as a root CEL identifier.
//
// A plain strings.Contains would produce false positives: for example, resource ID "a"
// would match "schema.spec.a" because "a." appears as a substring at the field-access
// boundary ("spec.a"). This function avoids that by requiring that the character
// immediately before internalID is NOT an identifier character or a dot — both of which
// indicate that internalID is a field accessor rather than a root identifier.
//
// This is necessary because KRO v0.9.0 compiles string templates into single CEL
// concatenation expressions (e.g., "${a.b}-${c.d}" → `(a.b) + "-" + (c.d)`),
// so HasPrefix is insufficient; we need Contains with boundary awareness.
func isResourceRef(expr, internalID string) bool {
	target := internalID + "."
	searchFrom := 0
	for {
		pos := strings.Index(expr[searchFrom:], target)
		if pos == -1 {
			return false
		}
		pos += searchFrom
		if pos == 0 {
			return true // at start of expression — definitely a root reference
		}
		prev := expr[pos-1]
		// Reject if preceded by an identifier char or '.':
		// - identifier char  → internalID is a sub-identifier (e.g., "schema.myResource")
		// - '.'              → internalID is a field access  (e.g., "schema.spec.myResource")
		isIdentOrDot := (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
			(prev >= '0' && prev <= '9') || prev == '_' || prev == '.'
		if !isIdentOrDot {
			return true
		}
		searchFrom = pos + 1
	}
}

// stripForEachDelimiters removes the ${...} wrapper from a KRO CEL expression.
// Returns the inner expression unchanged if no delimiters are present.
// Mirrors the unexported stripDelimiters in the cel package to avoid a cross-package dependency.
func stripForEachDelimiters(expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		return strings.TrimSpace(expr[2 : len(expr)-1])
	}
	return expr
}

// isForEachIdentifier reports whether s is a valid CEL root identifier
// (letter or underscore start, followed by letters, digits, or underscores).
func isForEachIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') {
				return false
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
				return false
			}
		}
	}
	return true
}

// analyzeForEachSource determines the SourceType and field path for a forEach CEL expression.
//
// Classification rules:
//   - "schema.spec.*"           → SchemaSource,   SourcePath = "spec.<rest>"
//   - "<identifier>.<rest>"     → ResourceSource, SourcePath = "<rest>" (identifier is the resource ID)
//   - anything else (literals)  → LiteralSource,  SourcePath = ""
func analyzeForEachSource(expr string) (SourceType, string) {
	bare := stripForEachDelimiters(expr)
	if bare == "" {
		return LiteralSource, ""
	}

	// Schema source: schema.spec.fieldPath
	if strings.HasPrefix(bare, schemaSpecExprPrefix) {
		fieldPath := "spec." + bare[len(schemaSpecExprPrefix):]
		// Trim at the first non-identifier/non-dot character (operators, spaces, etc.)
		if end := strings.IndexAny(fieldPath, " \t()[]{}!=<>+-*/,"); end != -1 {
			fieldPath = fieldPath[:end]
		}
		fieldPath = strings.TrimRight(fieldPath, ".")
		return SchemaSource, fieldPath
	}

	// Literals start with '[', '{', '"', or a digit — not a root identifier
	if len(bare) > 0 {
		first := bare[0]
		if first == '[' || first == '{' || first == '"' || (first >= '0' && first <= '9') {
			return LiteralSource, ""
		}

		// Resource source: <resourceID>.<rest>  (where resourceID != "schema")
		dotIdx := strings.Index(bare, ".")
		if dotIdx > 0 {
			prefix := bare[:dotIdx]
			rest := bare[dotIdx+1:]
			if isForEachIdentifier(prefix) && prefix != "schema" {
				// Trim rest at non-identifier/non-dot characters
				if end := strings.IndexAny(rest, " \t()[]{}!=<>+-*/,"); end != -1 {
					rest = rest[:end]
				}
				rest = strings.TrimRight(rest, ".")
				return ResourceSource, rest
			}
		}
	}

	return LiteralSource, ""
}

// parseForEach extracts and analyzes forEach iterator definitions from a resource map.
// Returns a non-fatal ParseError if forEach is combined with externalRef (mutually exclusive per KRO spec).
// Returns nil iterators (not an error) when no forEach field is present.
func (p *ResourceParser) parseForEach(resMap map[string]interface{}, hasExternalRef bool) ([]Iterator, *ParseError) {
	rawSlice, err := k8sparser.GetSlice(resMap, "forEach")
	if err != nil {
		// No forEach field — not a collection resource
		return nil, nil
	}

	if hasExternalRef {
		return nil, &ParseError{
			Expression: "forEach",
			Message:    "forEach and externalRef are mutually exclusive per KRO spec",
		}
	}

	iterators := make([]Iterator, 0, len(rawSlice))
	for i, raw := range rawSlice {
		dimMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		// ForEachDimension has exactly one key-value pair (enforced by KRO kubebuilder validation)
		for varName, rawExpr := range dimMap {
			exprStr, ok := rawExpr.(string)
			if !ok {
				continue
			}
			source, sourcePath := analyzeForEachSource(exprStr)
			iterators = append(iterators, Iterator{
				Name:           varName,
				Expression:     exprStr,
				Source:         source,
				SourcePath:     sourcePath,
				DimensionIndex: i,
			})
		}
	}

	return iterators, nil
}

// mapToSlice converts a map[string]bool to a sorted string slice.
func mapToSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
