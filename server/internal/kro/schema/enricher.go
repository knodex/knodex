// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	krocel "github.com/knodex/knodex/server/internal/kro/cel"
	"github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/models"
)

// RGDProvider abstracts RGD catalog lookups for the enricher.
// This decouples the enricher from the watcher implementation.
type RGDProvider interface {
	GetRGDByKind(kind string) (*models.CatalogRGD, bool)
}

// String constants for repeated prefix operations
const (
	specPrefix       = "spec."
	schemaSpecPrefix = "schema.spec."
	schemaPrefix     = "schema."
	advancedKey      = "advanced"
)

// defaultResourceParser is a package-level ResourceParser instance reused across calls.
// ResourceParser is stateless, so a single instance is safe for concurrent use.
var defaultResourceParser = parser.NewResourceParser()

// EnrichSchemaFromResources enriches a FormSchema with metadata from the RGD resource graph.
// It adds:
// - ConditionalSections based on includeWhen conditions
// - ExternalRefSelector metadata for fields used in externalRef name expressions
// - Nested ExternalRefSelector metadata for template resources with ${schema.spec.externalRef.*} patterns
// - AdvancedSection for fields under spec.advanced (hidden by default in UI)
//
// The optional rgdProvider enables cross-RGD Kind resolution for nested externalRef dropdowns.
// If nil, nested externalRef enrichment is skipped.
//
// Returns an error if schema or resourceGraph is nil, or if validation fails.
func EnrichSchemaFromResources(schema *models.FormSchema, resourceGraph *parser.ResourceGraph, rgdProvider ...RGDProvider) error {
	if schema == nil {
		return fmt.Errorf("schema cannot be nil")
	}
	if resourceGraph == nil {
		return fmt.Errorf("resource graph cannot be nil")
	}

	// Extract conditional sections from resources with includeWhen
	sections, err := extractConditionalSections(resourceGraph, schema)
	if err != nil {
		return fmt.Errorf("failed to extract conditional sections: %w", err)
	}
	schema.ConditionalSections = sections

	// Add externalRefSelector metadata to properties used in externalRef names
	if err := addExternalRefSelectors(schema, resourceGraph); err != nil {
		return fmt.Errorf("failed to add external ref selectors: %w", err)
	}

	// Add nested externalRef selectors from template resources
	var provider RGDProvider
	if len(rgdProvider) > 0 {
		provider = rgdProvider[0]
	}
	if err := addNestedExternalRefSelectors(schema, resourceGraph, provider, defaultResourceParser); err != nil {
		return fmt.Errorf("failed to add nested external ref selectors: %w", err)
	}

	// Extract advanced section (fields hidden by default in UI)
	advancedSection := extractAdvancedSection(schema)
	if advancedSection != nil {
		schema.AdvancedSection = advancedSection
	}

	return nil
}

// sectionBuilder is an internal helper for building ConditionalSections with O(1) duplicate checking
type sectionBuilder struct {
	models.ConditionalSection
	affectedSet map[string]struct{} // Set for O(1) duplicate detection
}

// addAffectedField adds a field to the affected properties if not already present (O(1))
func (sb *sectionBuilder) addAffectedField(field string) {
	if sb.affectedSet == nil {
		sb.affectedSet = make(map[string]struct{})
	}
	if _, exists := sb.affectedSet[field]; !exists {
		sb.affectedSet[field] = struct{}{}
		sb.AffectedProperties = append(sb.AffectedProperties, field)
	}
}

// collectNonConditionalSchemaFields returns the set of schema field names (without "spec." prefix)
// that are referenced by non-conditional (always-present) resources. These fields should never
// be hidden by conditional sections, because the always-present resources still need them.
func collectNonConditionalSchemaFields(graph *parser.ResourceGraph) map[string]bool {
	fields := make(map[string]bool)

	for _, res := range graph.Resources {
		// Skip conditional resources — they have IncludeWhen set
		if res.IncludeWhen != nil {
			continue
		}

		// Collect schema fields from the resource's SchemaFields (populated by parser)
		for _, field := range res.SchemaFields {
			if strings.HasPrefix(field, specPrefix) {
				fields[strings.TrimPrefix(field, specPrefix)] = true
			}
		}

		// Also check externalRef schema field (for non-conditional externalRefs)
		if res.ExternalRef != nil && res.ExternalRef.UsesSchemaSpec {
			fieldPath := res.ExternalRef.SchemaField
			if strings.HasPrefix(fieldPath, specPrefix) {
				fields[strings.TrimPrefix(fieldPath, specPrefix)] = true
			}
		}
	}

	return fields
}

// extractConditionalSections builds ConditionalSection entries from resources with includeWhen.
// Uses HashMap for O(n) complexity instead of O(n²).
func extractConditionalSections(graph *parser.ResourceGraph, schema *models.FormSchema) ([]models.ConditionalSection, error) {
	conditionalResources := graph.GetConditionalResources()
	if len(conditionalResources) == 0 {
		return []models.ConditionalSection{}, nil
	}

	// Collect schema fields used by non-conditional resources.
	// These fields should NOT be hidden when a conditional resource also uses them,
	// because the field is still needed by always-present resources.
	nonConditionalSchemaFields := collectNonConditionalSchemaFields(graph)

	// Build set of top-level property names used by non-conditional resources.
	// Used to prevent hiding an entire object when some of its sub-fields
	// are needed by always-present resources.
	nonConditionalTopLevel := make(map[string]bool, len(nonConditionalSchemaFields))
	for field := range nonConditionalSchemaFields {
		topLevel := strings.SplitN(field, ".", 2)[0]
		nonConditionalTopLevel[topLevel] = true
	}

	// Use HashMap for O(1) lookups - pre-allocate with estimated capacity
	sectionMap := make(map[string]*sectionBuilder, len(conditionalResources))

	for _, res := range conditionalResources {
		if res.IncludeWhen == nil {
			continue
		}

		// Extract schema fields used by this conditional resource
		// Pre-allocate with capacity of 4 (most resources have 1-4 affected fields)
		affectedFields := make([]string, 0, 4)

		// If this is an externalRef that uses schema.spec.*, add that field
		// BUT only if the field is not also used by a non-conditional resource.
		// Example: spec.name might be used by both the main template (non-conditional)
		// and a conditional permission-check resource — hiding it would break deployment.
		if res.ExternalRef != nil && res.ExternalRef.UsesSchemaSpec {
			fieldPath := res.ExternalRef.SchemaField
			if strings.HasPrefix(fieldPath, specPrefix) {
				fieldName := strings.TrimPrefix(fieldPath, specPrefix)
				if !nonConditionalSchemaFields[fieldName] {
					affectedFields = append(affectedFields, fieldName)
				}
			}
		}

		// Add template schema fields that are exclusively used by conditional resources.
		// Convert to top-level property names since the frontend checks visibility at the top level.
		for _, field := range res.SchemaFields {
			if strings.HasPrefix(field, specPrefix) {
				fieldName := strings.TrimPrefix(field, specPrefix)
				topLevel := strings.SplitN(fieldName, ".", 2)[0]
				if !nonConditionalTopLevel[topLevel] {
					affectedFields = append(affectedFields, topLevel)
				}
			}
		}

		// Group resources by their controlling field using HashMap (O(1) lookup)
		for _, schemaField := range res.IncludeWhen.SchemaFields {
			// Validate that controlling field exists in schema
			if err := validateControllingField(schemaField, schema); err != nil {
				return nil, fmt.Errorf("invalid controlling field %q: %w", schemaField, err)
			}

			// Check if we already have a section for this controlling field (O(1) lookup)
			builder, exists := sectionMap[schemaField]
			if exists {
				// Add affected fields to existing section
				for _, field := range affectedFields {
					builder.addAffectedField(field)
				}
			} else {
				// Create a new section builder
				builder = &sectionBuilder{
					ConditionalSection: models.ConditionalSection{
						Condition:          res.IncludeWhen.Expression,
						ControllingField:   schemaField,
						AffectedProperties: make([]string, 0, len(affectedFields)),
					},
					affectedSet: make(map[string]struct{}),
				}

				// Use CEL AST analysis for structured condition rules
				clientEvaluable, rules := krocel.AnalyzeCondition(res.IncludeWhen.Expression)
				builder.ClientEvaluable = clientEvaluable
				builder.Rules = rules

				// Derive ExpectedValue from Rules when possible (single boolean == check),
				// otherwise fall back to string-based extraction for legacy compatibility.
				builder.ExpectedValue = deriveExpectedValue(clientEvaluable, rules, res.IncludeWhen.Expression)

				// Add initial affected fields
				for _, field := range affectedFields {
					builder.addAffectedField(field)
				}

				sectionMap[schemaField] = builder
			}
		}
	}

	// Convert map to sorted slice for deterministic output.
	// Map iteration in Go is non-deterministic, so we sort by ControllingField
	// and sort AffectedProperties within each section.
	sections := make([]models.ConditionalSection, 0, len(sectionMap))
	for _, builder := range sectionMap {
		sort.Strings(builder.AffectedProperties)
		sections = append(sections, builder.ConditionalSection)
	}
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].ControllingField < sections[j].ControllingField
	})

	return sections, nil
}

// validateControllingField checks if the controlling field exists in the schema properties
func validateControllingField(field string, schema *models.FormSchema) error {
	if schema.Properties == nil {
		return fmt.Errorf("schema has no properties")
	}

	// Strip schema.spec. or spec. prefix since schema properties don't include these prefixes
	fieldPath := field
	if strings.HasPrefix(fieldPath, schemaSpecPrefix) {
		fieldPath = strings.TrimPrefix(fieldPath, schemaSpecPrefix)
	} else if strings.HasPrefix(fieldPath, specPrefix) {
		fieldPath = strings.TrimPrefix(fieldPath, specPrefix)
	}

	// Simple field (no dots)
	if !strings.Contains(fieldPath, ".") {
		if _, exists := schema.Properties[fieldPath]; !exists {
			return fmt.Errorf("field not found in schema properties")
		}
		return nil
	}

	// Nested field - validate path exists
	parts := strings.Split(fieldPath, ".")
	props := schema.Properties
	for i, part := range parts {
		prop, exists := props[part]
		if !exists {
			return fmt.Errorf("field path segment %q not found at level %d", part, i)
		}
		if i < len(parts)-1 {
			// Not the last part, need to traverse deeper
			if prop.Properties == nil {
				return fmt.Errorf("field %q has no nested properties", part)
			}
			props = prop.Properties
		}
	}

	return nil
}

// deriveExpectedValue extracts the expected value for a conditional section.
// It prefers CEL AST-derived rules (accurate) over string matching (legacy fallback).
//
// When the CEL analyzer produces exactly one rule with a boolean "==" or "!=" comparison,
// the expected value is derived directly from the rule. This avoids false positives from
// naive string matching (e.g., compound expressions containing "== true" as a substring).
func deriveExpectedValue(clientEvaluable bool, rules []models.ConditionRule, expr string) any {
	// When CEL analyzer successfully decomposed the expression, use it as authority
	if clientEvaluable {
		// Single-rule boolean comparison → derive expected value directly
		if len(rules) == 1 {
			rule := rules[0]
			if boolVal, ok := rule.Value.(bool); ok {
				switch rule.Op {
				case "==":
					return boolVal
				case "!=":
					return !boolVal
				}
			}
		}
		// Multi-rule or non-boolean: no single expected value
		return nil
	}

	// Legacy fallback for expressions the CEL analyzer can't decompose
	return extractExpectedValue(expr)
}

// extractExpectedValue tries to parse simple boolean conditions via string matching.
// This is a legacy fallback — prefer deriveExpectedValue which uses CEL AST rules first.
// Supports patterns like: "schema.spec.X == true", "schema.spec.X == false", "schema.spec.X"
func extractExpectedValue(expr string) any {
	expr = strings.TrimSpace(expr)

	// Check for "== true" or "== false" patterns
	if strings.Contains(expr, "== true") {
		return true
	}
	if strings.Contains(expr, "== false") {
		return false
	}
	if strings.Contains(expr, "!= true") {
		return false
	}
	if strings.Contains(expr, "!= false") {
		return true
	}

	// For simple field references like "schema.spec.X", assume truthy check
	if strings.HasPrefix(expr, schemaSpecPrefix) && !strings.Contains(expr, " ") {
		return true
	}

	// Unable to determine expected value
	return nil
}

// addExternalRefSelectors adds ExternalRefSelector metadata to parent object properties
// for externalRef resources that use the paired ${schema.spec.externalRef.<id>.name/namespace} pattern.
// Only supports the new pattern where both name AND namespace use ${schema.spec.*} expressions
// with a common parent path. The selector is attached to the parent object, not individual fields.
func addExternalRefSelectors(schema *models.FormSchema, graph *parser.ResourceGraph) error {
	externalRefs := graph.GetExternalRefs()

	for _, ref := range externalRefs {
		if ref.ExternalRef == nil || !ref.ExternalRef.UsesSchemaSpec {
			continue
		}

		extRef := ref.ExternalRef

		// Only support the paired pattern: both SchemaField and NamespaceSchemaField must be set
		if extRef.SchemaField == "" || extRef.NamespaceSchemaField == "" {
			continue
		}

		// Extract parent path from the field paths
		// e.g., "spec.externalRef.permissionResults.name" → "externalRef.permissionResults"
		nameField := strings.TrimPrefix(extRef.SchemaField, specPrefix)
		nsField := strings.TrimPrefix(extRef.NamespaceSchemaField, specPrefix)

		nameLastDot := strings.LastIndex(nameField, ".")
		nsLastDot := strings.LastIndex(nsField, ".")
		if nameLastDot < 0 || nsLastDot < 0 {
			continue // Fields must be nested (have at least one dot)
		}

		parentPath := nameField[:nameLastDot]
		nsParentPath := nsField[:nsLastDot]

		// Both fields must share the same parent
		if parentPath != nsParentPath {
			continue
		}

		nameLeaf := nameField[nameLastDot+1:]
		nsLeaf := nsField[nsLastDot+1:]

		// Attach resource picker metadata to the parent object property.
		// NOTE: AutoFillFields keys "name" and "namespace" are a contract with the frontend
		// ExternalRefSelector component which reads autoFillFields.name and autoFillFields.namespace.
		if err := attachResourcePickerToParent(schema.Properties, parentPath, &models.ExternalRefSelectorMetadata{
			APIVersion:           extRef.APIVersion,
			Kind:                 extRef.Kind,
			UseInstanceNamespace: true,
			AutoFillFields:       map[string]string{"name": nameLeaf, "namespace": nsLeaf},
		}); err != nil {
			return fmt.Errorf("failed to attach resource picker to %q: %w", parentPath, err)
		}
	}

	return nil
}

// attachResourcePickerToParent navigates to a nested property by path and attaches
// ExternalRefSelectorMetadata to it.
func attachResourcePickerToParent(props map[string]models.FormProperty, path string, metadata *models.ExternalRefSelectorMetadata) error {
	parts := strings.SplitN(path, ".", 2)

	prop, exists := props[parts[0]]
	if !exists {
		return fmt.Errorf("property %q not found", parts[0])
	}

	if len(parts) == 1 {
		// This is the target parent property - attach the selector
		prop.ExternalRefSelector = metadata
		props[parts[0]] = prop
		return nil
	}

	// Need to recurse into nested properties
	if prop.Properties == nil {
		return fmt.Errorf("property %q has no nested properties", parts[0])
	}
	if err := attachResourcePickerToParent(prop.Properties, parts[1], metadata); err != nil {
		return err
	}
	props[parts[0]] = prop
	return nil
}

// externalRefPrefix is the common prefix for nested externalRef schema fields.
// Used to detect paired spec.externalRef.*.name and *.namespace patterns in template resources.
const externalRefPrefix = "spec.externalRef."

// addNestedExternalRefSelectors scans template resources for ${schema.spec.externalRef.*.name/namespace}
// patterns and attaches ExternalRefSelectorMetadata to the corresponding schema properties.
// It uses cross-RGD lookup via RGDProvider to resolve the target apiVersion/kind.
//
// This complements addExternalRefSelectors which only handles resource-level externalRef entries.
// Template resources that reference externalRef sub-fields via ${schema.spec.*} expressions are
// handled here.
func addNestedExternalRefSelectors(schema *models.FormSchema, graph *parser.ResourceGraph, rgdProvider RGDProvider, resourceParser *parser.ResourceParser) error {
	logger := slog.Default().With("component", "schema-enricher")

	for _, res := range graph.Resources {
		if !res.IsTemplate {
			continue
		}

		// Find paired spec.externalRef.*.name / spec.externalRef.*.namespace patterns
		// Group schema fields by their externalRef parent path
		nameFields := make(map[string]string)     // parent -> name leaf
		nsFields := make(map[string]string)       // parent -> namespace leaf
		fieldParents := make(map[string]struct{}) // unique parent paths

		for _, field := range res.SchemaFields {
			if !strings.HasPrefix(field, externalRefPrefix) {
				continue
			}

			// e.g., "spec.externalRef.keyVaultRef.name" → remainder = "keyVaultRef.name"
			remainder := strings.TrimPrefix(field, externalRefPrefix)
			lastDot := strings.LastIndex(remainder, ".")
			if lastDot < 0 {
				continue
			}

			parent := remainder[:lastDot]
			leaf := remainder[lastDot+1:]

			switch leaf {
			case "name":
				nameFields[parent] = leaf
				fieldParents[parent] = struct{}{}
			case "namespace":
				nsFields[parent] = leaf
				fieldParents[parent] = struct{}{}
			}
		}

		// Process each parent that has BOTH name and namespace
		for parent := range fieldParents {
			nameLeaf, hasName := nameFields[parent]
			nsLeaf, hasNS := nsFields[parent]
			if !hasName || !hasNS {
				continue
			}

			// The full property path in schema (without "spec." prefix since schema.Properties starts at spec level)
			parentPath := "externalRef." + parent

			// Skip if selector already attached by resource-level externalRef processing (AC-4)
			if hasExternalRefSelector(schema.Properties, parentPath) {
				continue
			}

			// Resolve the target apiVersion/kind via cross-RGD lookup
			apiVersion, kind := resolveNestedExternalRefKind(res, parent, rgdProvider, resourceParser, logger)

			if apiVersion == "" || kind == "" {
				// Graceful degradation — skip the dropdown (AC-2 graceful degradation)
				continue
			}

			// Attach resource picker metadata to the parent object property
			if err := attachResourcePickerToParent(schema.Properties, parentPath, &models.ExternalRefSelectorMetadata{
				APIVersion:           apiVersion,
				Kind:                 kind,
				UseInstanceNamespace: true,
				AutoFillFields:       map[string]string{"name": nameLeaf, "namespace": nsLeaf},
			}); err != nil {
				return fmt.Errorf("failed to attach nested external ref selector to %q: %w", parentPath, err)
			}
		}
	}

	return nil
}

// resolveNestedExternalRefKind resolves the target apiVersion/kind for a nested externalRef
// by performing a cross-RGD lookup. It finds the child RGD by the template resource's Kind,
// then parses the child's resources to find the externalRef entry matching the field name.
func resolveNestedExternalRefKind(
	templateRes parser.ResourceDefinition,
	externalRefFieldName string,
	rgdProvider RGDProvider,
	resourceParser *parser.ResourceParser,
	logger *slog.Logger,
) (apiVersion, kind string) {
	if rgdProvider == nil {
		logger.Debug("no RGD provider available for cross-RGD lookup",
			"templateKind", templateRes.Kind)
		return "", ""
	}

	childKind := templateRes.Kind
	childRGD, found := rgdProvider.GetRGDByKind(childKind)
	if !found {
		logger.Warn("child RGD not found for cross-RGD externalRef resolution",
			"templateKind", childKind,
			"externalRefField", externalRefFieldName)
		return "", ""
	}

	if childRGD.RawSpec == nil {
		logger.Warn("child RGD has no raw spec",
			"templateKind", childKind)
		return "", ""
	}

	// Parse the child RGD's resources
	childGraph, err := resourceParser.ParseRGDResources(childRGD.Name, childRGD.Namespace, childRGD.RawSpec)
	if err != nil {
		logger.Warn("failed to parse child RGD resources",
			"childRGD", childRGD.Name,
			"error", err)
		return "", ""
	}

	// Find the externalRef resource in the child whose SchemaField matches
	// spec.externalRef.<fieldName>.name
	//
	// IMPORTANT: This assumes the child RGD's externalRef schema field uses the SAME
	// field name as the parent RGD's schema path. For example, if the parent schema has
	// spec.externalRef.keyVaultRef.name, the child RGD must also have an externalRef
	// resource whose SchemaField is spec.externalRef.keyVaultRef.name. If names differ
	// (e.g., parent uses "keyVaultRef" but child uses "keyVault"), the lookup fails
	// gracefully and the field renders as plain text.
	targetSchemaField := "spec.externalRef." + externalRefFieldName + ".name"

	for _, childRes := range childGraph.Resources {
		if childRes.ExternalRef == nil {
			continue
		}
		if childRes.ExternalRef.SchemaField == targetSchemaField {
			return childRes.ExternalRef.APIVersion, childRes.ExternalRef.Kind
		}
	}

	logger.Warn("no matching externalRef found in child RGD",
		"childRGD", childRGD.Name,
		"targetSchemaField", targetSchemaField)
	return "", ""
}

// hasExternalRefSelector checks if a property at the given path already has ExternalRefSelectorMetadata.
func hasExternalRefSelector(props map[string]models.FormProperty, path string) bool {
	parts := strings.SplitN(path, ".", 2)

	prop, exists := props[parts[0]]
	if !exists {
		return false
	}

	if len(parts) == 1 {
		return prop.ExternalRefSelector != nil
	}

	if prop.Properties == nil {
		return false
	}
	return hasExternalRefSelector(prop.Properties, parts[1])
}

// extractAdvancedSection identifies fields under spec.advanced and marks them as hidden by default.
// Returns nil if the schema has no "advanced" property.
func extractAdvancedSection(schema *models.FormSchema) *models.AdvancedSection {
	if schema.Properties == nil {
		return nil
	}

	// Check if schema has "advanced" property
	advancedProp, exists := schema.Properties[advancedKey]
	if !exists {
		return nil
	}

	// Only process object-type advanced properties with nested fields
	if advancedProp.Type != "object" || advancedProp.Properties == nil || len(advancedProp.Properties) == 0 {
		return nil
	}

	section := &models.AdvancedSection{
		Path:               advancedKey,
		AffectedProperties: make([]string, 0),
	}

	// Mark all nested properties as advanced and collect paths
	markAdvancedProperties(schema.Properties, advancedKey, section)

	// Only return the section if we found affected properties
	if len(section.AffectedProperties) == 0 {
		return nil
	}

	return section
}

// markAdvancedProperties recursively marks properties under the advanced section.
func markAdvancedProperties(
	props map[string]models.FormProperty,
	basePath string,
	section *models.AdvancedSection,
) {
	// Get the advanced property
	advancedProp, exists := props[advancedKey]
	if !exists || advancedProp.Properties == nil {
		return
	}

	// Recursively process all nested properties under "advanced"
	processNestedAdvancedProperties(advancedProp.Properties, basePath, section)

	// Update the advanced property back in the schema
	advancedProp.IsAdvanced = true
	props[advancedKey] = advancedProp
}

// processNestedAdvancedProperties recursively processes nested properties under the advanced section.
func processNestedAdvancedProperties(
	props map[string]models.FormProperty,
	basePath string,
	section *models.AdvancedSection,
) {
	for name, prop := range props {
		fullPath := basePath + "." + name

		// Mark as advanced
		prop.IsAdvanced = true
		section.AffectedProperties = append(section.AffectedProperties, fullPath)

		// Recurse for nested objects
		if prop.Properties != nil && len(prop.Properties) > 0 {
			processNestedAdvancedProperties(prop.Properties, fullPath, section)
		}

		// Handle array items
		if prop.Items != nil && prop.Items.Properties != nil {
			processNestedAdvancedProperties(prop.Items.Properties, fullPath, section)
		}

		// Update the property back
		props[name] = prop
	}
}

// EnrichSchema performs dual-source enrichment: merges CRD-derived schema properties
// with RGD intent metadata and resource graph information.
//
// The CRD provides types, validation constraints, and OpenAPI structure.
// The RGD intent provides defaults, descriptions, and required flags parsed from simpleschema.
// The resource graph provides conditional sections, externalRef selectors, and advanced sections.
//
// If rgdIntent is nil, only resource graph enrichment is applied (backward-compatible behavior).
func EnrichSchema(crdSchema *models.FormSchema, rgdIntent *RGDSchemaIntent, resourceGraph *parser.ResourceGraph, rgdProvider ...RGDProvider) error {
	if crdSchema == nil {
		return fmt.Errorf("CRD schema cannot be nil")
	}

	// Phase 1: Merge RGD intent into CRD schema (defaults, descriptions from simpleschema)
	if rgdIntent != nil {
		mergeRGDIntent(crdSchema.Properties, rgdIntent, "")
	}

	// Phase 2: Enrich with resource graph metadata (conditionals, externalRef, advanced)
	if resourceGraph != nil {
		if err := EnrichSchemaFromResources(crdSchema, resourceGraph, rgdProvider...); err != nil {
			return fmt.Errorf("resource graph enrichment: %w", err)
		}
	}

	return nil
}

// mergeRGDIntent merges RGD schema intent into CRD-derived form properties.
// It walks the CRD property tree and applies RGD metadata (defaults, descriptions)
// to matching fields. The CRD types are authoritative; RGD only supplements metadata.
//
// Fields in RGDSchemaIntent that don't match any CRD property path are logged at
// debug level to help diagnose RGD/CRD mismatches.
func mergeRGDIntent(props map[string]models.FormProperty, intent *RGDSchemaIntent, prefix string) {
	if props == nil || intent == nil {
		return
	}

	for name, prop := range props {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		fi, ok := intent.Fields[path]
		if ok {
			// Apply RGD description if CRD doesn't have one
			if prop.Description == "" && fi.Description != "" {
				prop.Description = fi.Description
			}

			// Apply RGD default if CRD doesn't have one
			if prop.Default == nil && fi.Default != "" {
				prop.Default = convertDefault(fi.Default, prop.Type)
			}
		}

		// Recurse into nested objects
		if prop.Properties != nil {
			mergeRGDIntent(prop.Properties, intent, path)
		}

		// Recurse into array items
		if prop.Items != nil && prop.Items.Properties != nil {
			mergeRGDIntent(prop.Items.Properties, intent, path)
		}

		props[name] = prop
	}

	// Log unmatched RGD fields at the top level (prefix == "")
	// to help diagnose RGD/CRD field name mismatches.
	if prefix == "" {
		for path := range intent.Fields {
			if !crdHasField(props, path) {
				slog.Debug("RGD intent field has no matching CRD property",
					"field", path)
			}
		}
	}
}

// crdHasField checks if a dot-separated field path exists in the CRD property tree.
func crdHasField(props map[string]models.FormProperty, path string) bool {
	parts := strings.SplitN(path, ".", 2)
	prop, exists := props[parts[0]]
	if !exists {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	if prop.Properties == nil {
		return false
	}
	return crdHasField(prop.Properties, parts[1])
}

// convertDefault converts a string default value to the appropriate Go type
// based on the OpenAPI type string. Logs a warning if the value cannot be
// parsed as the expected type and falls back to the raw string.
func convertDefault(value string, propType string) interface{} {
	switch propType {
	case "integer":
		if v, err := parseInt64(value); err == nil {
			return v
		}
		slog.Debug("RGD default value not parseable as integer, using raw string",
			"value", value, "type", propType)
		return value
	case "number":
		if v, err := parseFloat64(value); err == nil {
			return v
		}
		slog.Debug("RGD default value not parseable as number, using raw string",
			"value", value, "type", propType)
		return value
	case "boolean":
		if value == "true" {
			return true
		}
		if value == "false" {
			return false
		}
		slog.Debug("RGD default value not parseable as boolean, using raw string",
			"value", value, "type", propType)
		return value
	default:
		return value
	}
}

// parseInt64 parses a string as int64.
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// parseFloat64 parses a string as float64.
func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
