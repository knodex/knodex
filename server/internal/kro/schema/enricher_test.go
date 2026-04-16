// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"strings"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/models"
)

// TestEnrichSchemaFromResources_NilInputs tests error handling for nil inputs
func TestEnrichSchemaFromResources_NilInputs(t *testing.T) {
	tests := []struct {
		name    string
		schema  *models.FormSchema
		graph   *parser.ResourceGraph
		wantErr bool
	}{
		{
			name:    "nil schema",
			schema:  nil,
			graph:   &parser.ResourceGraph{},
			wantErr: true,
		},
		{
			name:    "nil graph",
			schema:  &models.FormSchema{},
			graph:   nil,
			wantErr: true,
		},
		{
			name:    "both nil",
			schema:  nil,
			graph:   nil,
			wantErr: true,
		},
		{
			name: "valid inputs",
			schema: &models.FormSchema{
				Properties: map[string]models.FormProperty{},
			},
			graph:   &parser.ResourceGraph{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnrichSchemaFromResources(tt.schema, tt.graph)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnrichSchemaFromResources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestExtractConditionalSections_EmptyGraph tests handling of empty resource graph
func TestExtractConditionalSections_EmptyGraph(t *testing.T) {
	graph := &parser.ResourceGraph{}
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty graph, got %d", len(sections))
	}
}

// TestExtractConditionalSections_SingleConditionalResource tests basic conditional section extraction
func TestExtractConditionalSections_SingleConditionalResource(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enabled == true",
					SchemaFields: []string{"enabled"},
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"enabled": {
				Type: "boolean",
			},
		},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	if sections[0].ControllingField != "enabled" {
		t.Errorf("expected controlling field 'enabled', got %q", sections[0].ControllingField)
	}

	if sections[0].ExpectedValue != true {
		t.Errorf("expected ExpectedValue true, got %v", sections[0].ExpectedValue)
	}

	// Verify CEL AST analysis populated new fields
	if !sections[0].ClientEvaluable {
		t.Error("expected ClientEvaluable to be true for simple boolean condition")
	}
	if len(sections[0].Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(sections[0].Rules))
	}
	if sections[0].Rules[0].Field != "spec.enabled" {
		t.Errorf("expected rule field 'spec.enabled', got %q", sections[0].Rules[0].Field)
	}
	if sections[0].Rules[0].Op != "==" {
		t.Errorf("expected rule op '==', got %q", sections[0].Rules[0].Op)
	}
	if sections[0].Rules[0].Value != true {
		t.Errorf("expected rule value true, got %v", sections[0].Rules[0].Value)
	}
}

// TestExtractConditionalSections_MultipleResourcesSameController tests HashMap grouping
func TestExtractConditionalSections_MultipleResourcesSameController(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap1",
				Kind: "ConfigMap",
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enabled == true",
					SchemaFields: []string{"enabled"},
				},
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec: true,
					SchemaField:    "spec.configMap1",
					APIVersion:     "v1",
					Kind:           "ConfigMap",
				},
			},
			{
				ID:   "1-ConfigMap2",
				Kind: "ConfigMap",
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enabled == true",
					SchemaFields: []string{"enabled"},
				},
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec: true,
					SchemaField:    "spec.configMap2",
					APIVersion:     "v1",
					Kind:           "ConfigMap",
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"enabled": {
				Type: "boolean",
			},
			"configMap1": {
				Type: "string",
			},
			"configMap2": {
				Type: "string",
			},
		},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should group into a single section
	if len(sections) != 1 {
		t.Fatalf("expected 1 section (grouped), got %d", len(sections))
	}

	// Should have both affected properties
	if len(sections[0].AffectedProperties) != 2 {
		t.Errorf("expected 2 affected properties, got %d", len(sections[0].AffectedProperties))
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, prop := range sections[0].AffectedProperties {
		if seen[prop] {
			t.Errorf("duplicate affected property: %s", prop)
		}
		seen[prop] = true
	}
}

// TestExtractConditionalSections_InvalidControllingField tests validation
func TestExtractConditionalSections_InvalidControllingField(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.nonExistent == true",
					SchemaFields: []string{"nonExistent"},
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"enabled": {
				Type: "boolean",
			},
		},
	}

	_, err := extractConditionalSections(graph, schema)
	if err == nil {
		t.Fatal("expected error for invalid controlling field, got nil")
	}

	// Error should mention the invalid field
	if !strings.Contains(err.Error(), "nonExistent") {
		t.Errorf("error message should mention 'nonExistent', got: %v", err)
	}
}

// TestExtractExpectedValue tests expected value extraction from expressions
func TestExtractExpectedValue(t *testing.T) {
	tests := []struct {
		expr     string
		expected interface{}
	}{
		{"schema.spec.enabled == true", true},
		{"schema.spec.enabled == false", false},
		{"schema.spec.enabled != true", false},
		{"schema.spec.enabled != false", true},
		{"schema.spec.enabled", true},
		{"schema.spec.enabled && schema.spec.other", nil},
		{"some complex expression", nil},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result := extractExpectedValue(tt.expr)
			if result != tt.expected {
				t.Errorf("extractExpectedValue(%q) = %v, want %v", tt.expr, result, tt.expected)
			}
		})
	}
}

// TestDeriveExpectedValue tests that deriveExpectedValue prefers CEL rules over string matching
func TestDeriveExpectedValue(t *testing.T) {
	tests := []struct {
		name            string
		clientEvaluable bool
		rules           []models.ConditionRule
		expr            string
		expected        any
	}{
		{
			name:            "single boolean == true rule",
			clientEvaluable: true,
			rules:           []models.ConditionRule{{Field: "spec.enabled", Op: "==", Value: true}},
			expr:            "schema.spec.enabled == true",
			expected:        true,
		},
		{
			name:            "single boolean == false rule",
			clientEvaluable: true,
			rules:           []models.ConditionRule{{Field: "spec.enabled", Op: "==", Value: false}},
			expr:            "schema.spec.enabled == false",
			expected:        false,
		},
		{
			name:            "single boolean != true rule",
			clientEvaluable: true,
			rules:           []models.ConditionRule{{Field: "spec.enabled", Op: "!=", Value: true}},
			expr:            "schema.spec.enabled != true",
			expected:        false,
		},
		{
			name:            "compound expression - no false positive from string matching",
			clientEvaluable: true,
			rules: []models.ConditionRule{
				{Field: "spec.replicas", Op: ">", Value: int64(3)},
				{Field: "spec.enabled", Op: "==", Value: true},
			},
			expr:     "schema.spec.replicas > 3 && schema.spec.enabled == true",
			expected: nil, // multiple rules, not a single boolean
		},
		{
			name:            "non-boolean single rule falls back to string matching",
			clientEvaluable: true,
			rules:           []models.ConditionRule{{Field: "spec.replicas", Op: ">", Value: int64(3)}},
			expr:            "schema.spec.replicas > 3",
			expected:        nil, // not a boolean value
		},
		{
			name:            "not client-evaluable falls back to string matching",
			clientEvaluable: false,
			rules:           nil,
			expr:            "schema.spec.enabled == true",
			expected:        true, // fallback string match
		},
		{
			name:            "bare field reference - not client-evaluable fallback",
			clientEvaluable: true,
			rules:           []models.ConditionRule{{Field: "spec.enabled", Op: "==", Value: true}},
			expr:            "schema.spec.enabled",
			expected:        true, // single boolean rule
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveExpectedValue(tt.clientEvaluable, tt.rules, tt.expr)
			if result != tt.expected {
				t.Errorf("deriveExpectedValue() = %v (%T), want %v (%T)",
					result, result, tt.expected, tt.expected)
			}
		})
	}
}

// TestValidateControllingField tests field validation logic
func TestValidateControllingField(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"simple": {
				Type: "string",
			},
			"nested": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"field": {
						Type: "string",
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		field   string
		wantErr bool
	}{
		{"valid simple field", "simple", false},
		{"valid nested field", "nested.field", false},
		{"invalid simple field", "nonExistent", true},
		{"invalid nested path", "nested.nonExistent", true},
		{"invalid nested in non-object", "simple.field", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateControllingField(tt.field, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateControllingField(%q) error = %v, wantErr %v", tt.field, err, tt.wantErr)
			}
		})
	}
}

// TestValidateControllingField_NilProperties tests handling of nil properties
func TestValidateControllingField_NilProperties(t *testing.T) {
	schema := &models.FormSchema{}

	err := validateControllingField("any", schema)
	if err == nil {
		t.Error("expected error for nil properties, got nil")
	}
}

// TestAddExternalRefSelectors_PairedPattern tests the new paired externalRef.<id>.name/namespace detection
func TestAddExternalRefSelectors_PairedPattern(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.permissionResults.name",
					NamespaceSchemaField: "spec.externalRef.permissionResults.namespace",
					APIVersion:           "v1",
					Kind:                 "ConfigMap",
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"permissionResults": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	err := addExternalRefSelectors(schema, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify selector was attached to the parent object (permissionResults), not individual fields
	extRefProp := schema.Properties["externalRef"]
	parentProp, exists := extRefProp.Properties["permissionResults"]
	if !exists {
		t.Fatal("permissionResults property not found")
	}

	if parentProp.ExternalRefSelector == nil {
		t.Fatal("ExternalRefSelector not set on parent object")
	}

	if parentProp.ExternalRefSelector.Kind != "ConfigMap" {
		t.Errorf("expected Kind 'ConfigMap', got %q", parentProp.ExternalRefSelector.Kind)
	}

	if parentProp.ExternalRefSelector.APIVersion != "v1" {
		t.Errorf("expected APIVersion 'v1', got %q", parentProp.ExternalRefSelector.APIVersion)
	}

	if parentProp.ExternalRefSelector.UseInstanceNamespace {
		t.Error("expected UseInstanceNamespace to be false (paired name/namespace has explicit namespace field)")
	}

	// Verify AutoFillFields
	if parentProp.ExternalRefSelector.AutoFillFields == nil {
		t.Fatal("AutoFillFields not set")
	}

	if parentProp.ExternalRefSelector.AutoFillFields["name"] != "name" {
		t.Errorf("expected AutoFillFields[name]='name', got %q", parentProp.ExternalRefSelector.AutoFillFields["name"])
	}

	if parentProp.ExternalRefSelector.AutoFillFields["namespace"] != "namespace" {
		t.Errorf("expected AutoFillFields[namespace]='namespace', got %q", parentProp.ExternalRefSelector.AutoFillFields["namespace"])
	}

	// Verify child string fields do NOT have selectors
	nameProp := parentProp.Properties["name"]
	if nameProp.ExternalRefSelector != nil {
		t.Error("child 'name' property should NOT have ExternalRefSelector")
	}

	nsProp := parentProp.Properties["namespace"]
	if nsProp.ExternalRefSelector != nil {
		t.Error("child 'namespace' property should NOT have ExternalRefSelector")
	}
}

// TestAddExternalRefSelectors_SkipsSingleFieldPattern tests that the old single-field pattern
// (only SchemaField set, no NamespaceSchemaField) is correctly ignored
func TestAddExternalRefSelectors_SkipsSingleFieldPattern(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec: true,
					SchemaField:    "spec.configMapName",
					// NamespaceSchemaField is empty - old single-field pattern
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"configMapName": {
				Type: "string",
			},
		},
	}

	err := addExternalRefSelectors(schema, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no selector was added (old pattern is ignored)
	prop := schema.Properties["configMapName"]
	if prop.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set for single-field pattern")
	}
}

// TestAddExternalRefSelectors_SkipsMismatchedParents tests that externalRefs where name and
// namespace have different parent paths are correctly ignored
func TestAddExternalRefSelectors_SkipsMismatchedParents(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.db.name",
					NamespaceSchemaField: "spec.other.path.namespace",
					APIVersion:           "v1",
					Kind:                 "ConfigMap",
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"db": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name": {Type: "string"},
						},
					},
				},
			},
		},
	}

	err := addExternalRefSelectors(schema, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no selector was attached (mismatched parents)
	dbProp := schema.Properties["externalRef"].Properties["db"]
	if dbProp.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set for mismatched parent paths")
	}
}

// TestSectionBuilder_DuplicateDetection tests the O(1) duplicate detection
func TestSectionBuilder_DuplicateDetection(t *testing.T) {
	builder := &sectionBuilder{
		ConditionalSection: models.ConditionalSection{
			AffectedProperties: []string{},
		},
		affectedSet: make(map[string]struct{}),
	}

	// Add same field multiple times
	for i := 0; i < 10; i++ {
		builder.addAffectedField("field1")
		builder.addAffectedField("field2")
	}

	// Should only have 2 unique fields
	if len(builder.AffectedProperties) != 2 {
		t.Errorf("expected 2 unique fields, got %d", len(builder.AffectedProperties))
	}
}

// TestExtractConditionalSections_Performance tests that optimization achieves target performance
func TestExtractConditionalSections_Performance(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	// Create a large graph with 50 conditional resources
	graph := generateLargeResourceGraph(50)
	schema := &models.FormSchema{
		Properties: generateLargeSchema(50),
	}

	// Measure performance
	start := time.Now()
	sections, err := extractConditionalSections(graph, schema)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should complete in <100ms
	if duration > 100*time.Millisecond {
		t.Errorf("performance target not met: took %v, want <100ms", duration)
	}

	// Verify sections were created
	if len(sections) == 0 {
		t.Error("expected sections to be created")
	}

	t.Logf("Processed %d conditional resources in %v", 50, duration)
}

// generateLargeResourceGraph creates a graph with n conditional resources for benchmarking
func generateLargeResourceGraph(n int) *parser.ResourceGraph {
	resources := make([]parser.ResourceDefinition, 0, n)

	for i := 0; i < n; i++ {
		// Create resources with various controlling fields to test grouping
		controllingField := "enabled"
		if i%5 == 0 {
			controllingField = "advanced"
		}
		if i%7 == 0 {
			controllingField = "feature"
		}

		resources = append(resources, parser.ResourceDefinition{
			ID:   "resource-" + strings.Repeat("x", i%10), // Vary ID lengths
			Kind: "ConfigMap",
			IncludeWhen: &parser.ConditionExpr{
				Expression:   "schema.spec." + controllingField + " == true",
				SchemaFields: []string{controllingField},
			},
			ExternalRef: &parser.ExternalRefInfo{
				UsesSchemaSpec: true,
				SchemaField:    "spec.config" + strings.Repeat("x", i%10),
				APIVersion:     "v1",
				Kind:           "ConfigMap",
			},
		})
	}

	return &parser.ResourceGraph{
		Resources: resources,
	}
}

// generateLargeSchema creates a schema with n properties for benchmarking
func generateLargeSchema(n int) map[string]models.FormProperty {
	props := make(map[string]models.FormProperty, n+10)

	// Add controlling fields
	props["enabled"] = models.FormProperty{Type: "boolean"}
	props["advanced"] = models.FormProperty{Type: "boolean"}
	props["feature"] = models.FormProperty{Type: "boolean"}

	// Add affected fields
	for i := 0; i < n; i++ {
		fieldName := "config" + strings.Repeat("x", i%10)
		props[fieldName] = models.FormProperty{
			Type: "string",
		}
	}

	return props
}

// TestAddExternalRefSelectors_MultiplePairedRefs tests handling of multiple paired externalRefs
func TestAddExternalRefSelectors_MultiplePairedRefs(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.config.name",
					NamespaceSchemaField: "spec.externalRef.config.namespace",
					APIVersion:           "v1",
					Kind:                 "ConfigMap",
				},
			},
			{
				ID:   "1-Secret",
				Kind: "Secret",
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.credentials.name",
					NamespaceSchemaField: "spec.externalRef.credentials.namespace",
					APIVersion:           "v1",
					Kind:                 "Secret",
				},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"config": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
					"credentials": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	err := addExternalRefSelectors(schema, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both parent objects should have selectors
	extRefProp := schema.Properties["externalRef"]

	configProp := extRefProp.Properties["config"]
	if configProp.ExternalRefSelector == nil {
		t.Fatal("ExternalRefSelector not set on config")
	}
	if configProp.ExternalRefSelector.Kind != "ConfigMap" {
		t.Errorf("config: expected Kind 'ConfigMap', got %q", configProp.ExternalRefSelector.Kind)
	}

	credsProp := extRefProp.Properties["credentials"]
	if credsProp.ExternalRefSelector == nil {
		t.Fatal("ExternalRefSelector not set on credentials")
	}
	if credsProp.ExternalRefSelector.Kind != "Secret" {
		t.Errorf("credentials: expected Kind 'Secret', got %q", credsProp.ExternalRefSelector.Kind)
	}
}

// TestAddExternalRefSelectors_ErrorOnMissingParent tests that addExternalRefSelectors returns
// an error when the schema is missing the expected parent property for a paired externalRef.
func TestAddExternalRefSelectors_ErrorOnMissingParent(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:   "0-ConfigMap",
				Kind: "ConfigMap",
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.missing.name",
					NamespaceSchemaField: "spec.externalRef.missing.namespace",
					APIVersion:           "v1",
					Kind:                 "ConfigMap",
				},
			},
		},
	}

	// Schema does NOT have the "externalRef.missing" path
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
		},
	}

	err := addExternalRefSelectors(schema, graph)
	if err == nil {
		t.Fatal("expected error for missing parent property, got nil")
	}

	if !strings.Contains(err.Error(), "externalRef") {
		t.Errorf("error should mention 'externalRef', got: %v", err)
	}
}

// TestExtractAdvancedSections_NoAdvancedProperty tests schema without advanced property
func TestExtractAdvancedSections_NoAdvancedProperty(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"port": {Type: "integer"},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 0 {
		t.Errorf("expected no sections for schema without advanced property, got %d", len(sections))
	}
}

// TestExtractAdvancedSections_EmptyAdvancedProperty tests advanced property with no nested fields
func TestExtractAdvancedSections_EmptyAdvancedProperty(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type:       "object",
				Properties: map[string]models.FormProperty{},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 0 {
		t.Errorf("expected no sections for empty advanced property, got %d", len(sections))
	}
}

// TestExtractAdvancedSections_BasicAdvancedFields tests basic advanced field detection
func TestExtractAdvancedSections_BasicAdvancedFields(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer"},
					"enabled":  {Type: "boolean"},
				},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	if sections[0].Path != "advanced" {
		t.Errorf("expected path 'advanced', got %q", sections[0].Path)
	}

	if len(sections[0].AffectedProperties) != 2 {
		t.Errorf("expected 2 affected properties, got %d", len(sections[0].AffectedProperties))
	}
}

// TestExtractAdvancedSections_NestedAdvancedFields tests nested advanced field detection
func TestExtractAdvancedSections_NestedAdvancedFields(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"securityContext": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"runAsNonRoot":           {Type: "boolean"},
							"readOnlyRootFilesystem": {Type: "boolean"},
						},
					},
					"resources": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"limits": {
								Type: "object",
								Properties: map[string]models.FormProperty{
									"memory": {Type: "string"},
									"cpu":    {Type: "string"},
								},
							},
						},
					},
				},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	// Verify nested paths are in affected properties
	expectedPaths := []string{
		"advanced.securityContext",
		"advanced.securityContext.runAsNonRoot",
		"advanced.securityContext.readOnlyRootFilesystem",
		"advanced.resources",
		"advanced.resources.limits",
		"advanced.resources.limits.memory",
		"advanced.resources.limits.cpu",
	}

	for _, expected := range expectedPaths {
		found := false
		for _, actual := range sections[0].AffectedProperties {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected affected property %q not found", expected)
		}
	}
}

// TestExtractAdvancedSections_RGDDefaultPreserved tests that RGD defaults are preserved
func TestExtractAdvancedSections_RGDDefaultPreserved(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {
						Type:    "integer",
						Default: 3, // RGD specifies default
					},
				},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	// The property should still be marked as advanced
	advancedProp := schema.Properties["advanced"]
	replicasProp := advancedProp.Properties["replicas"]
	if !replicasProp.IsAdvanced {
		t.Error("expected replicas to be marked as advanced")
	}

	// Original default should be preserved
	if replicasProp.Default != 3 {
		t.Errorf("expected replicas default to be 3, got %v", replicasProp.Default)
	}
}

// TestExtractAdvancedSections_MarksPropertiesAdvanced tests that properties are marked with IsAdvanced
func TestExtractAdvancedSections_MarksPropertiesAdvanced(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer"},
				},
			},
		},
	}

	_ = extractAdvancedSections(schema)

	// Check that advanced property is marked
	advancedProp := schema.Properties["advanced"]
	if !advancedProp.IsAdvanced {
		t.Error("expected 'advanced' property to be marked as advanced")
	}

	// Check that nested property is marked
	replicasProp := advancedProp.Properties["replicas"]
	if !replicasProp.IsAdvanced {
		t.Error("expected 'replicas' property to be marked as advanced")
	}

	// Check that non-advanced property is NOT marked
	nameProp := schema.Properties["name"]
	if nameProp.IsAdvanced {
		t.Error("expected 'name' property to NOT be marked as advanced")
	}
}

// TestExtractAdvancedSections_PerFeatureBastion tests per-feature bastion.advanced detection
func TestExtractAdvancedSections_PerFeatureBastion(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"bastion": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"enabled":      {Type: "boolean"},
					"subnetPrefix": {Type: "string"},
					"advanced": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"asoCredentialSecretName": {Type: "string"},
						},
					},
				},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	if sections[0].Path != "bastion.advanced" {
		t.Errorf("expected path 'bastion.advanced', got %q", sections[0].Path)
	}

	// Verify the advanced child property path is in affected properties
	found := false
	for _, p := range sections[0].AffectedProperties {
		if p == "bastion.advanced.asoCredentialSecretName" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'bastion.advanced.asoCredentialSecretName' in affected properties, got %v", sections[0].AffectedProperties)
	}
}

// TestExtractAdvancedSections_TopLevelBackwardCompat tests top-level advanced backward compat
func TestExtractAdvancedSections_TopLevelBackwardCompat(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer"},
				},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	if sections[0].Path != "advanced" {
		t.Errorf("expected path 'advanced', got %q", sections[0].Path)
	}
}

// TestExtractAdvancedSections_BothCoexist tests both global and per-feature sections
func TestExtractAdvancedSections_BothCoexist(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer"},
				},
			},
			"bastion": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"enabled": {Type: "boolean"},
					"advanced": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"asoCredentialSecretName": {Type: "string"},
						},
					},
				},
			},
		},
	}

	sections := extractAdvancedSections(schema)

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}

	// Should be sorted by path
	if sections[0].Path != "advanced" {
		t.Errorf("expected first section path 'advanced', got %q", sections[0].Path)
	}
	if sections[1].Path != "bastion.advanced" {
		t.Errorf("expected second section path 'bastion.advanced', got %q", sections[1].Path)
	}
}

// TestExtractConditionalSections_SharedFieldNotAffected tests that a schema field used by both
// a non-conditional template and a conditional externalRef is NOT added to affectedProperties.
// This is the aso-credential bug: spec.name was incorrectly hidden because a conditional
// externalRef used it, even though the main template also needed it.
func TestExtractConditionalSections_SharedFieldNotAffected(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				// Non-conditional template that uses schema.spec.name
				ID:           "0-ConfigMap",
				Kind:         "ConfigMap",
				IsTemplate:   true,
				SchemaFields: []string{"spec.name"}, // populated by parser from ${schema.spec.name}
			},
			{
				// Conditional externalRef that ALSO uses schema.spec.name
				ID:         "1-ConfigMap",
				Kind:       "ConfigMap",
				IsTemplate: false,
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "${schema.spec.advanced.permissionCheck.enabled == true}",
					SchemaFields: []string{"spec.advanced.permissionCheck.enabled"},
				},
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec: true,
					SchemaField:    "spec.name",
					APIVersion:     "v1",
					Kind:           "ConfigMap",
				},
				SchemaFields: []string{"spec.name"},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"permissionCheck": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"enabled": {Type: "boolean"},
						},
					},
				},
			},
		},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 section for the controlling field
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	// "name" should NOT be in affectedProperties because it's also used by
	// the non-conditional template resource
	for _, prop := range sections[0].AffectedProperties {
		if prop == "name" {
			t.Error("'name' should NOT be in affectedProperties — it's used by a non-conditional resource")
		}
	}
}

// TestExtractConditionalSections_ExclusiveFieldIsAffected tests that a schema field used
// ONLY by a conditional externalRef IS correctly added to affectedProperties.
func TestExtractConditionalSections_ExclusiveFieldIsAffected(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				// Non-conditional template, does NOT use tlsSecretName
				ID:           "0-Deployment",
				Kind:         "Deployment",
				IsTemplate:   true,
				SchemaFields: []string{"spec.name", "spec.replicas"},
			},
			{
				// Conditional externalRef that uses an exclusive field (tlsSecretName)
				ID:         "1-Secret",
				Kind:       "Secret",
				IsTemplate: false,
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.ingress.enabled == true",
					SchemaFields: []string{"spec.ingress.enabled"},
				},
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec: true,
					SchemaField:    "spec.tlsSecretName",
					APIVersion:     "v1",
					Kind:           "Secret",
				},
				SchemaFields: []string{"spec.tlsSecretName"},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name":          {Type: "string"},
			"replicas":      {Type: "integer"},
			"tlsSecretName": {Type: "string"},
			"ingress": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"enabled": {Type: "boolean"},
				},
			},
		},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	// "tlsSecretName" SHOULD be in affectedProperties since only the conditional resource uses it
	found := false
	for _, prop := range sections[0].AffectedProperties {
		if prop == "tlsSecretName" {
			found = true
		}
	}
	if !found {
		t.Error("'tlsSecretName' should be in affectedProperties — it's exclusively used by the conditional resource")
	}
}

// TestCollectNonConditionalSchemaFields tests the helper that collects schema fields
// used by non-conditional (always-present) resources
func TestCollectNonConditionalSchemaFields(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				// Non-conditional template with schema fields
				ID:           "0-ConfigMap",
				Kind:         "ConfigMap",
				IsTemplate:   true,
				SchemaFields: []string{"spec.name", "spec.namespace"},
			},
			{
				// Non-conditional externalRef
				ID:         "1-Secret",
				Kind:       "Secret",
				IsTemplate: false,
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec: true,
					SchemaField:    "spec.secretName",
					APIVersion:     "v1",
					Kind:           "Secret",
				},
				SchemaFields: []string{"spec.secretName"},
			},
			{
				// Conditional resource — should be excluded
				ID:         "2-Service",
				Kind:       "Service",
				IsTemplate: true,
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.expose == true",
					SchemaFields: []string{"spec.expose"},
				},
				SchemaFields: []string{"spec.expose", "spec.port"},
			},
		},
	}

	fields := collectNonConditionalSchemaFields(graph)

	// Should include fields from non-conditional resources
	if !fields["name"] {
		t.Error("expected 'name' in non-conditional fields")
	}
	if !fields["namespace"] {
		t.Error("expected 'namespace' in non-conditional fields")
	}
	if !fields["secretName"] {
		t.Error("expected 'secretName' in non-conditional fields")
	}

	// Should NOT include fields from conditional resources
	if fields["expose"] {
		t.Error("'expose' should not be in non-conditional fields — it's from a conditional resource")
	}
	if fields["port"] {
		t.Error("'port' should not be in non-conditional fields — it's from a conditional resource")
	}
}

// TestExtractConditionalSections_TemplateFieldsAffected validates that template SchemaFields
// from conditional resources are correctly added to affectedProperties, while fields shared
// with non-conditional resources are excluded.
func TestExtractConditionalSections_TemplateFieldsAffected(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				// Non-conditional template using name, visibleAnnotation, image, port
				ID:           "0-Deployment",
				Kind:         "Deployment",
				IsTemplate:   true,
				SchemaFields: []string{"spec.name", "spec.visibleAnnotation", "spec.image", "spec.port"},
			},
			{
				// Conditional template using database.name, visibleAnnotation, hiddenAnnotation
				ID:         "1-StatefulSet",
				Kind:       "StatefulSet",
				IsTemplate: true,
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enableDatabase == true",
					SchemaFields: []string{"enableDatabase"},
				},
				SchemaFields: []string{"spec.database.name", "spec.visibleAnnotation", "spec.hiddenAnnotation"},
			},
			{
				// Another conditional template using name, visibleAnnotation, hiddenAnnotation
				ID:         "2-ConfigMap",
				Kind:       "ConfigMap",
				IsTemplate: true,
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enableCache == true",
					SchemaFields: []string{"enableCache"},
				},
				SchemaFields: []string{"spec.name", "spec.visibleAnnotation", "spec.hiddenAnnotation"},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name":              {Type: "string"},
			"visibleAnnotation": {Type: "string"},
			"hiddenAnnotation":  {Type: "string"},
			"image":             {Type: "string"},
			"port":              {Type: "integer"},
			"enableDatabase":    {Type: "boolean"},
			"enableCache":       {Type: "boolean"},
			"database": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"name": {Type: "string"},
				},
			},
		},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections (enableDatabase, enableCache), got %d", len(sections))
	}

	// Build lookup by controlling field
	sectionByField := make(map[string]models.ConditionalSection)
	for _, s := range sections {
		sectionByField[s.ControllingField] = s
	}

	// enableDatabase section should have: database, hiddenAnnotation
	dbSection, ok := sectionByField["enableDatabase"]
	if !ok {
		t.Fatal("expected section for enableDatabase")
	}

	dbAffected := make(map[string]bool)
	for _, p := range dbSection.AffectedProperties {
		dbAffected[p] = true
	}
	if !dbAffected["database"] {
		t.Error("enableDatabase section should have 'database' in affectedProperties (template-exclusive)")
	}
	if !dbAffected["hiddenAnnotation"] {
		t.Error("enableDatabase section should have 'hiddenAnnotation' in affectedProperties (template-exclusive)")
	}
	if dbAffected["visibleAnnotation"] {
		t.Error("enableDatabase section should NOT have 'visibleAnnotation' — used by non-conditional resource")
	}
	if dbAffected["name"] {
		t.Error("enableDatabase section should NOT have 'name' — used by non-conditional resource")
	}

	// enableCache section should have: hiddenAnnotation
	cacheSection, ok := sectionByField["enableCache"]
	if !ok {
		t.Fatal("expected section for enableCache")
	}

	cacheAffected := make(map[string]bool)
	for _, p := range cacheSection.AffectedProperties {
		cacheAffected[p] = true
	}
	if !cacheAffected["hiddenAnnotation"] {
		t.Error("enableCache section should have 'hiddenAnnotation' in affectedProperties")
	}
	if cacheAffected["name"] {
		t.Error("enableCache section should NOT have 'name' — used by non-conditional resource")
	}
	if cacheAffected["visibleAnnotation"] {
		t.Error("enableCache section should NOT have 'visibleAnnotation' — used by non-conditional resource")
	}
}

// TestExtractConditionalSections_NestedFieldTopLevel validates that nested schema field paths
// like "database.name" are extracted to their top-level property name "database" in affectedProperties.
func TestExtractConditionalSections_NestedFieldTopLevel(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				// Non-conditional template, does NOT use database.*
				ID:           "0-Deployment",
				Kind:         "Deployment",
				IsTemplate:   true,
				SchemaFields: []string{"spec.name"},
			},
			{
				// Conditional template using nested database.name and database.host
				ID:         "1-StatefulSet",
				Kind:       "StatefulSet",
				IsTemplate: true,
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enableDatabase == true",
					SchemaFields: []string{"enableDatabase"},
				},
				SchemaFields: []string{"spec.database.name", "spec.database.host"},
			},
		},
	}

	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name":           {Type: "string"},
			"enableDatabase": {Type: "boolean"},
			"database": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"name": {Type: "string"},
					"host": {Type: "string"},
				},
			},
		},
	}

	sections, err := extractConditionalSections(graph, schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}

	// Should have "database" as the top-level affected property (not "database.name" or "database.host")
	found := false
	for _, prop := range sections[0].AffectedProperties {
		if prop == "database" {
			found = true
		}
		if prop == "database.name" || prop == "database.host" {
			t.Errorf("affectedProperties should have top-level 'database', not nested path %q", prop)
		}
	}
	if !found {
		t.Error("expected 'database' in affectedProperties (top-level extraction from 'database.name')")
	}

	// Should NOT have duplicate "database" entries
	count := 0
	for _, prop := range sections[0].AffectedProperties {
		if prop == "database" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected 'database' once in affectedProperties, got %d times", count)
	}
}

// TestEnrichSchemaFromResources_WithAdvancedSection tests full enrichment with advanced section
func TestEnrichSchemaFromResources_WithAdvancedSection(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer"},
					"securityContext": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"runAsNonRoot": {Type: "boolean"},
						},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{}

	err := EnrichSchemaFromResources(schema, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.AdvancedSections) == 0 {
		t.Fatal("expected AdvancedSections to be set")
	}

	if len(schema.AdvancedSections[0].AffectedProperties) < 3 {
		t.Errorf("expected at least 3 affected properties, got %d", len(schema.AdvancedSections[0].AffectedProperties))
	}
}

// --- Nested ExternalRef Tests (Story 3-2) ---

// mockRGDProvider implements RGDProvider for testing
type mockRGDProvider struct {
	rgds map[string]*models.CatalogRGD
}

func (m *mockRGDProvider) GetRGDByKind(kind string) (*models.CatalogRGD, bool) {
	rgd, ok := m.rgds[kind]
	return rgd, ok
}

// TestAddNestedExternalRefSelectors_BasicNestedRef tests detection of nested externalRef
// patterns in template resource SchemaFields (AC-1)
func TestAddNestedExternalRefSelectors_BasicNestedRef(t *testing.T) {
	// Template resource with paired spec.externalRef.keyVaultRef.name/namespace schema fields
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-AKVESOBinding",
				Kind:       "AKVESOBinding",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.keyVaultRef.name",
					"spec.externalRef.keyVaultRef.namespace",
					"spec.name",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"keyVaultRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// Child RGD has an externalRef resource pointing to AzureKeyVault
	childRGDSpec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "keyVault",
				"externalRef": map[string]interface{}{
					"apiVersion": "kro.run/v1alpha1",
					"kind":       "AzureKeyVault",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.externalRef.keyVaultRef.name}",
						"namespace": "${schema.spec.externalRef.keyVaultRef.namespace}",
					},
				},
			},
		},
	}

	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"AKVESOBinding": {
				Name:    "akv-eso-binding",
				Kind:    "AKVESOBinding",
				RawSpec: childRGDSpec,
			},
		},
	}

	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	// Verify selector was attached to externalRef.keyVaultRef
	extRefProp := formSchema.Properties["externalRef"]
	keyVaultProp, exists := extRefProp.Properties["keyVaultRef"]
	if !exists {
		t.Fatal("keyVaultRef property not found")
	}

	if keyVaultProp.ExternalRefSelector == nil {
		t.Fatal("ExternalRefSelector not set on keyVaultRef")
	}

	if keyVaultProp.ExternalRefSelector.APIVersion != "kro.run/v1alpha1" {
		t.Errorf("expected APIVersion 'kro.run/v1alpha1', got %q", keyVaultProp.ExternalRefSelector.APIVersion)
	}

	if keyVaultProp.ExternalRefSelector.Kind != "AzureKeyVault" {
		t.Errorf("expected Kind 'AzureKeyVault', got %q", keyVaultProp.ExternalRefSelector.Kind)
	}

	if keyVaultProp.ExternalRefSelector.UseInstanceNamespace {
		t.Error("expected UseInstanceNamespace to be false (nested externalRefs have explicit namespace fields)")
	}

	if keyVaultProp.ExternalRefSelector.AutoFillFields["name"] != "name" {
		t.Errorf("expected AutoFillFields[name]='name', got %q", keyVaultProp.ExternalRefSelector.AutoFillFields["name"])
	}

	if keyVaultProp.ExternalRefSelector.AutoFillFields["namespace"] != "namespace" {
		t.Errorf("expected AutoFillFields[namespace]='namespace', got %q", keyVaultProp.ExternalRefSelector.AutoFillFields["namespace"])
	}
}

// TestAddNestedExternalRefSelectors_CrossRGDResolution tests that cross-RGD Kind resolution
// correctly extracts apiVersion/kind from the child RGD (AC-2)
func TestAddNestedExternalRefSelectors_CrossRGDResolution(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-AKVESOBinding",
				Kind:       "AKVESOBinding",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.keyVaultRef.name",
					"spec.externalRef.keyVaultRef.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"keyVaultRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// Mock: child RGD has externalRef for keyVaultRef pointing to AzureKeyVault
	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"AKVESOBinding": {
				Name: "akv-eso-binding",
				Kind: "AKVESOBinding",
				RawSpec: map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"id": "keyVault",
							"externalRef": map[string]interface{}{
								"apiVersion": "kro.run/v1alpha1",
								"kind":       "AzureKeyVault",
								"metadata": map[string]interface{}{
									"name":      "${schema.spec.externalRef.keyVaultRef.name}",
									"namespace": "${schema.spec.externalRef.keyVaultRef.namespace}",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	prop := formSchema.Properties["externalRef"].Properties["keyVaultRef"]
	if prop.ExternalRefSelector == nil {
		t.Fatal("expected ExternalRefSelector on keyVaultRef")
	}
	if prop.ExternalRefSelector.APIVersion != "kro.run/v1alpha1" {
		t.Errorf("expected APIVersion 'kro.run/v1alpha1', got %q", prop.ExternalRefSelector.APIVersion)
	}
	if prop.ExternalRefSelector.Kind != "AzureKeyVault" {
		t.Errorf("expected Kind 'AzureKeyVault', got %q", prop.ExternalRefSelector.Kind)
	}
}

// TestAddNestedExternalRefSelectors_GracefulDegradation tests that missing child RGD
// doesn't crash and the field renders as plain text (AC-2 graceful degradation)
func TestAddNestedExternalRefSelectors_GracefulDegradation(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-UnknownKind",
				Kind:       "UnknownKind",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.someRef.name",
					"spec.externalRef.someRef.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"someRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// Provider returns false — child RGD not found
	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{},
	}

	// Should not panic or error
	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	// No selector should be attached (graceful degradation)
	prop := formSchema.Properties["externalRef"].Properties["someRef"]
	if prop.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set when child RGD is not found")
	}
}

// TestAddNestedExternalRefSelectors_NilProvider tests graceful handling when no provider is given
func TestAddNestedExternalRefSelectors_NilProvider(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-SomeKind",
				Kind:       "SomeKind",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.ref.name",
					"spec.externalRef.ref.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"ref": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// Should not panic or error with nil provider
	if err := addNestedExternalRefSelectors(formSchema, graph, nil, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	prop := formSchema.Properties["externalRef"].Properties["ref"]
	if prop.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set with nil provider")
	}
}

// TestAddNestedExternalRefSelectors_ExistingResourceLevelRef tests that existing resource-level
// externalRef selectors are not overwritten (AC-3, AC-4)
func TestAddNestedExternalRefSelectors_ExistingResourceLevelRef(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			// Resource-level externalRef (already processed by addExternalRefSelectors)
			{
				ID:         "0-ArgoCDCluster",
				Kind:       "ArgoCDAKSCluster",
				IsTemplate: false,
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.argocdClusterRef.name",
					NamespaceSchemaField: "spec.externalRef.argocdClusterRef.namespace",
					APIVersion:           "kro.run/v1alpha1",
					Kind:                 "ArgoCDAKSCluster",
				},
			},
			// Template resource that also references the same externalRef field
			{
				ID:         "1-SomeTemplate",
				Kind:       "SomeTemplate",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.argocdClusterRef.name",
					"spec.externalRef.argocdClusterRef.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"argocdClusterRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
						// Pre-existing selector from addExternalRefSelectors
						ExternalRefSelector: &models.ExternalRefSelectorMetadata{
							APIVersion:           "kro.run/v1alpha1",
							Kind:                 "ArgoCDAKSCluster",
							UseInstanceNamespace: true,
							AutoFillFields:       map[string]string{"name": "name", "namespace": "namespace"},
						},
					},
				},
			},
		},
	}

	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"SomeTemplate": {
				Name: "some-template",
				Kind: "SomeTemplate",
				RawSpec: map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"externalRef": map[string]interface{}{
								"apiVersion": "different/v1",
								"kind":       "DifferentKind",
								"metadata": map[string]interface{}{
									"name":      "${schema.spec.externalRef.argocdClusterRef.name}",
									"namespace": "${schema.spec.externalRef.argocdClusterRef.namespace}",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	// The original resource-level selector should be preserved, not overwritten
	prop := formSchema.Properties["externalRef"].Properties["argocdClusterRef"]
	if prop.ExternalRefSelector == nil {
		t.Fatal("ExternalRefSelector should still be set")
	}
	if prop.ExternalRefSelector.Kind != "ArgoCDAKSCluster" {
		t.Errorf("expected Kind 'ArgoCDAKSCluster' (original), got %q", prop.ExternalRefSelector.Kind)
	}
}

// TestAddNestedExternalRefSelectors_ESOExample tests with the realistic ESO RGD example
// (AKSApplicationExternalSecretOperator → AKVESOBinding → AzureKeyVault)
func TestAddNestedExternalRefSelectors_ESOExample(t *testing.T) {
	// Parent RGD: AKSApplicationExternalSecretOperator
	// Has template resource esoBinding (kind: AKVESOBinding) that references
	// spec.externalRef.keyVaultRef.name/namespace via ${schema.spec.*}
	// Also has template resource clusterSecretStore that references
	// spec.externalRef.argocdClusterRef.name/namespace
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				// Resource-level externalRef (already works)
				ID:         "0-ArgoCDAKSCluster",
				Kind:       "ArgoCDAKSCluster",
				IsTemplate: false,
				ExternalRef: &parser.ExternalRefInfo{
					UsesSchemaSpec:       true,
					SchemaField:          "spec.externalRef.argocdClusterRef.name",
					NamespaceSchemaField: "spec.externalRef.argocdClusterRef.namespace",
					APIVersion:           "kro.run/v1alpha1",
					Kind:                 "ArgoCDAKSCluster",
				},
			},
			{
				// Template resource: esoBinding (kind: AKVESOBinding)
				ID:         "1-AKVESOBinding",
				Kind:       "AKVESOBinding",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.name",
					"spec.externalRef.keyVaultRef.name",
					"spec.externalRef.keyVaultRef.namespace",
				},
			},
			{
				// Template resource: clusterSecretStore
				ID:         "2-ClusterSecretStore",
				Kind:       "ClusterSecretStore",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.managedClusterRef.name",
					"spec.externalRef.managedClusterRef.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"argocdClusterRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
					"keyVaultRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
					"managedClusterRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// First: run resource-level externalRef enrichment (for argocdClusterRef)
	if err := addExternalRefSelectors(formSchema, graph); err != nil {
		t.Fatalf("addExternalRefSelectors failed: %v", err)
	}

	// Then: run nested externalRef enrichment
	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"AKVESOBinding": {
				Name: "akv-eso-binding",
				Kind: "AKVESOBinding",
				RawSpec: map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"id": "keyVault",
							"externalRef": map[string]interface{}{
								"apiVersion": "kro.run/v1alpha1",
								"kind":       "AzureKeyVault",
								"metadata": map[string]interface{}{
									"name":      "${schema.spec.externalRef.keyVaultRef.name}",
									"namespace": "${schema.spec.externalRef.keyVaultRef.namespace}",
								},
							},
						},
					},
				},
			},
			"ClusterSecretStore": {
				Name: "cluster-secret-store",
				Kind: "ClusterSecretStore",
				RawSpec: map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"id": "managedCluster",
							"externalRef": map[string]interface{}{
								"apiVersion": "kro.run/v1alpha1",
								"kind":       "AKSManagedCluster",
								"metadata": map[string]interface{}{
									"name":      "${schema.spec.externalRef.managedClusterRef.name}",
									"namespace": "${schema.spec.externalRef.managedClusterRef.namespace}",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	extRefProp := formSchema.Properties["externalRef"]

	// argocdClusterRef: should have selector from resource-level enrichment
	argocdProp := extRefProp.Properties["argocdClusterRef"]
	if argocdProp.ExternalRefSelector == nil {
		t.Fatal("argocdClusterRef should have ExternalRefSelector from resource-level enrichment")
	}
	if argocdProp.ExternalRefSelector.Kind != "ArgoCDAKSCluster" {
		t.Errorf("argocdClusterRef: expected Kind 'ArgoCDAKSCluster', got %q", argocdProp.ExternalRefSelector.Kind)
	}

	// keyVaultRef: should have selector from nested enrichment
	keyVaultProp := extRefProp.Properties["keyVaultRef"]
	if keyVaultProp.ExternalRefSelector == nil {
		t.Fatal("keyVaultRef should have ExternalRefSelector from nested enrichment")
	}
	if keyVaultProp.ExternalRefSelector.Kind != "AzureKeyVault" {
		t.Errorf("keyVaultRef: expected Kind 'AzureKeyVault', got %q", keyVaultProp.ExternalRefSelector.Kind)
	}

	// managedClusterRef: should have selector from nested enrichment
	managedClusterProp := extRefProp.Properties["managedClusterRef"]
	if managedClusterProp.ExternalRefSelector == nil {
		t.Fatal("managedClusterRef should have ExternalRefSelector from nested enrichment")
	}
	if managedClusterProp.ExternalRefSelector.Kind != "AKSManagedCluster" {
		t.Errorf("managedClusterRef: expected Kind 'AKSManagedCluster', got %q", managedClusterProp.ExternalRefSelector.Kind)
	}
}

// TestGetRGDByKind tests the RGDWatcher.GetRGDByKind method
func TestGetRGDByKind(t *testing.T) {
	tests := []struct {
		name       string
		cacheRGDs  []*models.CatalogRGD
		lookupKind string
		wantFound  bool
		wantName   string
	}{
		{
			name: "found by kind",
			cacheRGDs: []*models.CatalogRGD{
				{Name: "akv-eso-binding", Kind: "AKVESOBinding"},
				{Name: "azure-key-vault", Kind: "AzureKeyVault"},
			},
			lookupKind: "AKVESOBinding",
			wantFound:  true,
			wantName:   "akv-eso-binding",
		},
		{
			name: "not found",
			cacheRGDs: []*models.CatalogRGD{
				{Name: "akv-eso-binding", Kind: "AKVESOBinding"},
			},
			lookupKind: "NonExistentKind",
			wantFound:  false,
		},
		{
			name:       "empty cache",
			cacheRGDs:  []*models.CatalogRGD{},
			lookupKind: "AnyKind",
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockRGDProvider{
				rgds: make(map[string]*models.CatalogRGD),
			}
			for _, rgd := range tt.cacheRGDs {
				provider.rgds[rgd.Kind] = rgd
			}

			rgd, found := provider.GetRGDByKind(tt.lookupKind)
			if found != tt.wantFound {
				t.Errorf("GetRGDByKind(%q) found = %v, want %v", tt.lookupKind, found, tt.wantFound)
			}
			if found && rgd.Name != tt.wantName {
				t.Errorf("GetRGDByKind(%q) name = %q, want %q", tt.lookupKind, rgd.Name, tt.wantName)
			}
		})
	}
}

// TestHasExternalRefSelector tests the duplicate detection helper
func TestHasExternalRefSelector(t *testing.T) {
	props := map[string]models.FormProperty{
		"externalRef": {
			Type: "object",
			Properties: map[string]models.FormProperty{
				"withSelector": {
					Type: "object",
					ExternalRefSelector: &models.ExternalRefSelectorMetadata{
						Kind: "SomeKind",
					},
				},
				"withoutSelector": {
					Type: "object",
				},
			},
		},
	}

	tests := []struct {
		path string
		want bool
	}{
		{"externalRef.withSelector", true},
		{"externalRef.withoutSelector", false},
		{"externalRef.nonExistent", false},
		{"nonExistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := hasExternalRefSelector(props, tt.path)
			if got != tt.want {
				t.Errorf("hasExternalRefSelector(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestAddNestedExternalRefSelectors_NilRawSpec tests that a child RGD with nil RawSpec
// is handled gracefully (no crash, no selector attached)
func TestAddNestedExternalRefSelectors_NilRawSpec(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-SomeKind",
				Kind:       "SomeKind",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.ref.name",
					"spec.externalRef.ref.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"ref": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// Child RGD exists but has nil RawSpec
	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"SomeKind": {
				Name:    "some-kind",
				Kind:    "SomeKind",
				RawSpec: nil,
			},
		},
	}

	// Should not panic or error
	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	prop := formSchema.Properties["externalRef"].Properties["ref"]
	if prop.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set when child RGD has nil RawSpec")
	}
}

// TestAddNestedExternalRefSelectors_SkipsNonTemplateResources tests that non-template
// resources are not processed by the nested enricher
func TestAddNestedExternalRefSelectors_SkipsNonTemplateResources(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-ExternalRef",
				Kind:       "SomeKind",
				IsTemplate: false, // not a template
				SchemaFields: []string{
					"spec.externalRef.ref.name",
					"spec.externalRef.ref.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"ref": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"SomeKind": {
				Name: "some-kind",
				Kind: "SomeKind",
				RawSpec: map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"externalRef": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name":      "${schema.spec.externalRef.ref.name}",
									"namespace": "${schema.spec.externalRef.ref.namespace}",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := addNestedExternalRefSelectors(formSchema, graph, provider, parser.NewResourceParser()); err != nil {
		t.Fatalf("addNestedExternalRefSelectors failed: %v", err)
	}

	prop := formSchema.Properties["externalRef"].Properties["ref"]
	if prop.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set on non-template resources")
	}
}

// TestEnrichSchemaFromResources_WithRGDProvider tests the full EnrichSchemaFromResources pipeline
// with an RGDProvider, verifying that nested externalRef selectors are attached end-to-end.
// This is the integration test ensuring the variadic rgdProvider parameter works correctly.
func TestEnrichSchemaFromResources_WithRGDProvider(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-AKVESOBinding",
				Kind:       "AKVESOBinding",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.keyVaultRef.name",
					"spec.externalRef.keyVaultRef.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"keyVaultRef": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	provider := &mockRGDProvider{
		rgds: map[string]*models.CatalogRGD{
			"AKVESOBinding": {
				Name:      "akv-eso-binding",
				Namespace: "default",
				Kind:      "AKVESOBinding",
				RawSpec: map[string]interface{}{
					"resources": []interface{}{
						map[string]interface{}{
							"id": "keyVault",
							"externalRef": map[string]interface{}{
								"apiVersion": "kro.run/v1alpha1",
								"kind":       "AzureKeyVault",
								"metadata": map[string]interface{}{
									"name":      "${schema.spec.externalRef.keyVaultRef.name}",
									"namespace": "${schema.spec.externalRef.keyVaultRef.namespace}",
								},
							},
						},
					},
				},
			},
		},
	}

	// Call the full pipeline with RGDProvider (integration test)
	err := EnrichSchemaFromResources(formSchema, graph, provider)
	if err != nil {
		t.Fatalf("EnrichSchemaFromResources with RGDProvider failed: %v", err)
	}

	// Verify nested selector was attached via the full pipeline
	extRefProp := formSchema.Properties["externalRef"]
	keyVaultProp, exists := extRefProp.Properties["keyVaultRef"]
	if !exists {
		t.Fatal("keyVaultRef property not found after enrichment")
	}

	if keyVaultProp.ExternalRefSelector == nil {
		t.Fatal("ExternalRefSelector not set on keyVaultRef via EnrichSchemaFromResources pipeline")
	}

	if keyVaultProp.ExternalRefSelector.APIVersion != "kro.run/v1alpha1" {
		t.Errorf("expected APIVersion 'kro.run/v1alpha1', got %q", keyVaultProp.ExternalRefSelector.APIVersion)
	}

	if keyVaultProp.ExternalRefSelector.Kind != "AzureKeyVault" {
		t.Errorf("expected Kind 'AzureKeyVault', got %q", keyVaultProp.ExternalRefSelector.Kind)
	}
}

// TestEnrichSchemaFromResources_WithoutRGDProvider verifies backward compatibility
// when no RGDProvider is passed (existing callers unaffected).
func TestEnrichSchemaFromResources_WithoutRGDProvider(t *testing.T) {
	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:         "0-SomeTemplate",
				Kind:       "SomeTemplate",
				IsTemplate: true,
				SchemaFields: []string{
					"spec.externalRef.ref.name",
					"spec.externalRef.ref.namespace",
				},
			},
		},
	}

	formSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"externalRef": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"ref": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"name":      {Type: "string"},
							"namespace": {Type: "string"},
						},
					},
				},
			},
		},
	}

	// Call without RGDProvider — should not error
	err := EnrichSchemaFromResources(formSchema, graph)
	if err != nil {
		t.Fatalf("EnrichSchemaFromResources without RGDProvider failed: %v", err)
	}

	// No selector should be attached (no provider to resolve Kind)
	prop := formSchema.Properties["externalRef"].Properties["ref"]
	if prop.ExternalRefSelector != nil {
		t.Error("ExternalRefSelector should NOT be set without RGDProvider")
	}
}

// TestEnrichSchema_NilCRDSchema tests that EnrichSchema returns an error for nil CRD schema.
func TestEnrichSchema_NilCRDSchema(t *testing.T) {
	err := EnrichSchema(nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil CRD schema")
	}
}

// TestEnrichSchema_NilIntent_NilGraph tests that EnrichSchema succeeds with nil intent and nil graph.
func TestEnrichSchema_NilIntent_NilGraph(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string", Path: "spec.name"},
		},
	}
	err := EnrichSchema(schema, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestEnrichSchema_MergesRGDDefaults tests that dual-source merges RGD defaults into CRD schema.
func TestEnrichSchema_MergesRGDDefaults(t *testing.T) {
	// CRD schema has types but no defaults
	crdSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name":     {Type: "string", Path: "spec.name"},
			"replicas": {Type: "integer", Path: "spec.replicas"},
			"enabled":  {Type: "boolean", Path: "spec.enabled"},
		},
	}

	// RGD intent has defaults and descriptions
	rgdIntent := &RGDSchemaIntent{
		Fields: map[string]FieldIntent{
			"name":     {Type: "string", Default: "my-app", Description: "App name"},
			"replicas": {Type: "integer", Default: "3"},
			"enabled":  {Type: "boolean", Default: "true"},
		},
	}

	err := EnrichSchema(crdSchema, rgdIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check defaults were applied
	if crdSchema.Properties["name"].Default != "my-app" {
		t.Errorf("name default = %v, want %q", crdSchema.Properties["name"].Default, "my-app")
	}
	if crdSchema.Properties["replicas"].Default != int64(3) {
		t.Errorf("replicas default = %v (type %T), want int64(3)", crdSchema.Properties["replicas"].Default, crdSchema.Properties["replicas"].Default)
	}
	if crdSchema.Properties["enabled"].Default != true {
		t.Errorf("enabled default = %v, want true", crdSchema.Properties["enabled"].Default)
	}

	// Check description was applied
	if crdSchema.Properties["name"].Description != "App name" {
		t.Errorf("name description = %q, want %q", crdSchema.Properties["name"].Description, "App name")
	}
}

// TestEnrichSchema_CRDDefaultsTakePrecedence tests that CRD defaults are NOT overwritten by RGD.
func TestEnrichSchema_CRDDefaultsTakePrecedence(t *testing.T) {
	crdSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string", Path: "spec.name", Default: "crd-default", Description: "CRD desc"},
		},
	}

	rgdIntent := &RGDSchemaIntent{
		Fields: map[string]FieldIntent{
			"name": {Type: "string", Default: "rgd-default", Description: "RGD desc"},
		},
	}

	err := EnrichSchema(crdSchema, rgdIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CRD values should be preserved
	if crdSchema.Properties["name"].Default != "crd-default" {
		t.Errorf("default = %v, want %q (CRD should take precedence)", crdSchema.Properties["name"].Default, "crd-default")
	}
	if crdSchema.Properties["name"].Description != "CRD desc" {
		t.Errorf("description = %q, want %q (CRD should take precedence)", crdSchema.Properties["name"].Description, "CRD desc")
	}
}

// TestEnrichSchema_NestedObjectMerge tests that RGD intent merges into nested CRD properties.
func TestEnrichSchema_NestedObjectMerge(t *testing.T) {
	crdSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"config": {
				Type: "object",
				Path: "spec.config",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer", Path: "spec.config.replicas"},
					"cpu":      {Type: "string", Path: "spec.config.cpu"},
				},
			},
		},
	}

	rgdIntent := &RGDSchemaIntent{
		Fields: map[string]FieldIntent{
			"config.replicas": {Type: "integer", Default: "2"},
			"config.cpu":      {Type: "string", Default: "500m"},
		},
	}

	err := EnrichSchema(crdSchema, rgdIntent, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	config := crdSchema.Properties["config"]
	if config.Properties["replicas"].Default != int64(2) {
		t.Errorf("config.replicas default = %v, want int64(2)", config.Properties["replicas"].Default)
	}
	if config.Properties["cpu"].Default != "500m" {
		t.Errorf("config.cpu default = %v, want %q", config.Properties["cpu"].Default, "500m")
	}
}

// TestEnrichSchema_DualSourceParity tests that dual-source enrichment produces the same
// resource-graph enrichment (conditionals, externalRef, advanced) as the legacy single-source path.
func TestEnrichSchema_DualSourceParity(t *testing.T) {
	// Build a FormSchema + ResourceGraph that exercises all enrichment paths
	makeSchema := func() *models.FormSchema {
		return &models.FormSchema{
			Properties: map[string]models.FormProperty{
				"name":           {Type: "string", Path: "spec.name"},
				"enableDatabase": {Type: "boolean", Path: "spec.enableDatabase"},
				"database": {
					Type: "object",
					Path: "spec.database",
					Properties: map[string]models.FormProperty{
						"name": {Type: "string", Path: "spec.database.name"},
					},
				},
				"advanced": {
					Type: "object",
					Path: "spec.advanced",
					Properties: map[string]models.FormProperty{
						"replicas": {Type: "integer", Path: "spec.advanced.replicas"},
					},
				},
			},
		}
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "app",
				Kind:         "Deployment",
				SchemaFields: []string{"spec.name"},
			},
			{
				ID:   "database",
				Kind: "StatefulSet",
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enableDatabase == true",
					SchemaFields: []string{"enableDatabase"},
				},
				SchemaFields: []string{"spec.database.name"},
			},
		},
	}

	// Path 1: Legacy single-source (EnrichSchemaFromResources)
	legacySchema := makeSchema()
	if err := EnrichSchemaFromResources(legacySchema, graph); err != nil {
		t.Fatalf("EnrichSchemaFromResources failed: %v", err)
	}

	// Path 2: Dual-source (EnrichSchema with nil intent — should produce same result)
	dualSchema := makeSchema()
	if err := EnrichSchema(dualSchema, nil, graph); err != nil {
		t.Fatalf("EnrichSchema failed: %v", err)
	}

	// Verify conditional sections match
	if len(legacySchema.ConditionalSections) != len(dualSchema.ConditionalSections) {
		t.Fatalf("conditional sections count: legacy=%d, dual=%d",
			len(legacySchema.ConditionalSections), len(dualSchema.ConditionalSections))
	}

	for i, legacy := range legacySchema.ConditionalSections {
		dual := dualSchema.ConditionalSections[i]
		if legacy.ControllingField != dual.ControllingField {
			t.Errorf("section[%d] controlling field: legacy=%q, dual=%q", i, legacy.ControllingField, dual.ControllingField)
		}
		if legacy.Condition != dual.Condition {
			t.Errorf("section[%d] condition: legacy=%q, dual=%q", i, legacy.Condition, dual.Condition)
		}
		if len(legacy.AffectedProperties) != len(dual.AffectedProperties) {
			t.Errorf("section[%d] affected properties count: legacy=%d, dual=%d",
				i, len(legacy.AffectedProperties), len(dual.AffectedProperties))
		}
	}

	// Verify advanced sections match
	if len(legacySchema.AdvancedSections) != len(dualSchema.AdvancedSections) {
		t.Errorf("advanced sections count mismatch: legacy=%d, dual=%d",
			len(legacySchema.AdvancedSections), len(dualSchema.AdvancedSections))
	}
	for i := range legacySchema.AdvancedSections {
		if i >= len(dualSchema.AdvancedSections) {
			break
		}
		if len(legacySchema.AdvancedSections[i].AffectedProperties) != len(dualSchema.AdvancedSections[i].AffectedProperties) {
			t.Errorf("advanced section[%d] properties count: legacy=%d, dual=%d",
				i, len(legacySchema.AdvancedSections[i].AffectedProperties), len(dualSchema.AdvancedSections[i].AffectedProperties))
		}
	}
}

// TestEnrichSchema_DualSourceWithIntent tests that dual-source enrichment with non-nil RGD intent
// produces a schema that has BOTH resource graph enrichment (conditionals, advanced) AND
// RGD intent metadata (defaults, descriptions) merged.
func TestEnrichSchema_DualSourceWithIntent(t *testing.T) {
	crdSchema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name":           {Type: "string", Path: "spec.name"},
			"replicas":       {Type: "integer", Path: "spec.replicas"},
			"enableDatabase": {Type: "boolean", Path: "spec.enableDatabase"},
			"database": {
				Type: "object",
				Path: "spec.database",
				Properties: map[string]models.FormProperty{
					"name": {Type: "string", Path: "spec.database.name"},
				},
			},
			"advanced": {
				Type: "object",
				Path: "spec.advanced",
				Properties: map[string]models.FormProperty{
					"cpu": {Type: "string", Path: "spec.advanced.cpu"},
				},
			},
		},
	}

	rgdIntent := &RGDSchemaIntent{
		Fields: map[string]FieldIntent{
			"name":          {Type: "string", Default: "my-app", Description: "Application name"},
			"replicas":      {Type: "integer", Default: "3"},
			"database.name": {Type: "string", Default: "mydb"},
			"advanced.cpu":  {Type: "string", Default: "500m"},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "app",
				Kind:         "Deployment",
				SchemaFields: []string{"spec.name", "spec.replicas"},
			},
			{
				ID:   "db",
				Kind: "StatefulSet",
				IncludeWhen: &parser.ConditionExpr{
					Expression:   "schema.spec.enableDatabase == true",
					SchemaFields: []string{"enableDatabase"},
				},
				SchemaFields: []string{"spec.database.name"},
			},
		},
	}

	err := EnrichSchema(crdSchema, rgdIntent, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify RGD defaults were merged
	if crdSchema.Properties["name"].Default != "my-app" {
		t.Errorf("name default = %v, want %q", crdSchema.Properties["name"].Default, "my-app")
	}
	if crdSchema.Properties["name"].Description != "Application name" {
		t.Errorf("name description = %q, want %q", crdSchema.Properties["name"].Description, "Application name")
	}
	if crdSchema.Properties["replicas"].Default != int64(3) {
		t.Errorf("replicas default = %v, want int64(3)", crdSchema.Properties["replicas"].Default)
	}

	// Verify nested defaults
	dbName := crdSchema.Properties["database"].Properties["name"]
	if dbName.Default != "mydb" {
		t.Errorf("database.name default = %v, want %q", dbName.Default, "mydb")
	}
	advCPU := crdSchema.Properties["advanced"].Properties["cpu"]
	if advCPU.Default != "500m" {
		t.Errorf("advanced.cpu default = %v, want %q", advCPU.Default, "500m")
	}

	// Verify resource graph enrichment was also applied
	if len(crdSchema.ConditionalSections) != 1 {
		t.Fatalf("expected 1 conditional section, got %d", len(crdSchema.ConditionalSections))
	}
	if crdSchema.ConditionalSections[0].ControllingField != "enableDatabase" {
		t.Errorf("controlling field = %q, want %q", crdSchema.ConditionalSections[0].ControllingField, "enableDatabase")
	}

	// Verify advanced section was extracted
	if len(crdSchema.AdvancedSections) == 0 {
		t.Error("expected advanced sections to be extracted")
	}
}

// TestConvertDefault tests the default value conversion logic.
func TestConvertDefault(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		propType string
		want     interface{}
	}{
		{"string default", "hello", "string", "hello"},
		{"integer default", "42", "integer", int64(42)},
		{"number default", "3.14", "number", 3.14},
		{"boolean true", "true", "boolean", true},
		{"boolean false", "false", "boolean", false},
		{"invalid integer", "abc", "integer", "abc"},
		{"invalid number", "abc", "number", "abc"},
		{"invalid boolean", "maybe", "boolean", "maybe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertDefault(tt.value, tt.propType)
			if got != tt.want {
				t.Errorf("convertDefault(%q, %q) = %v (%T), want %v (%T)",
					tt.value, tt.propType, got, got, tt.want, tt.want)
			}
		})
	}
}

// --- Collection Annotation Tests (STORY-332) ---

// TestAddCollectionAnnotations_SchemaSource verifies that a single forEach with schema source
// annotates the correct property with collection metadata (AC1).
func TestAddCollectionAnnotations_SchemaSource(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"workers": {
						Type:  "array",
						Items: &models.FormProperty{Type: "object"},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "workerPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "worker",
						Expression:     "${schema.spec.workers}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.workers",
						DimensionIndex: 0,
					},
				},
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	workersProp := schema.Properties["spec"].Properties["workers"]
	if workersProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.workers, got nil")
	}

	ann := workersProp.CollectionAnnotation
	if ann.ResourceID != "workerPods" {
		t.Errorf("ResourceID = %q, want %q", ann.ResourceID, "workerPods")
	}
	if ann.IteratorVar != "worker" {
		t.Errorf("IteratorVar = %q, want %q", ann.IteratorVar, "worker")
	}
	if ann.Dimensions != 1 {
		t.Errorf("Dimensions = %d, want 1", ann.Dimensions)
	}
	if ann.DimensionIndex != 0 {
		t.Errorf("DimensionIndex = %d, want 0", ann.DimensionIndex)
	}
	if ann.Source != "schema" {
		t.Errorf("Source = %q, want %q", ann.Source, "schema")
	}
}

// TestAddCollectionAnnotations_CartesianProduct verifies that two iterators referencing
// different schema fields are both annotated with correct dimensions/dimensionIndex (AC2).
func TestAddCollectionAnnotations_CartesianProduct(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"regions": {
						Type:  "array",
						Items: &models.FormProperty{Type: "string"},
					},
					"tiers": {
						Type:  "array",
						Items: &models.FormProperty{Type: "string"},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "deployments",
				Kind:         "Deployment",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "region",
						Expression:     "${schema.spec.regions}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.regions",
						DimensionIndex: 0,
					},
					{
						Name:           "tier",
						Expression:     "${schema.spec.tiers}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.tiers",
						DimensionIndex: 1,
					},
				},
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	regionsProp := schema.Properties["spec"].Properties["regions"]
	if regionsProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.regions, got nil")
	}
	if regionsProp.CollectionAnnotation.Dimensions != 2 {
		t.Errorf("regions Dimensions = %d, want 2", regionsProp.CollectionAnnotation.Dimensions)
	}
	if regionsProp.CollectionAnnotation.DimensionIndex != 0 {
		t.Errorf("regions DimensionIndex = %d, want 0", regionsProp.CollectionAnnotation.DimensionIndex)
	}
	if regionsProp.CollectionAnnotation.IteratorVar != "region" {
		t.Errorf("regions IteratorVar = %q, want %q", regionsProp.CollectionAnnotation.IteratorVar, "region")
	}

	tiersProp := schema.Properties["spec"].Properties["tiers"]
	if tiersProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.tiers, got nil")
	}
	if tiersProp.CollectionAnnotation.Dimensions != 2 {
		t.Errorf("tiers Dimensions = %d, want 2", tiersProp.CollectionAnnotation.Dimensions)
	}
	if tiersProp.CollectionAnnotation.DimensionIndex != 1 {
		t.Errorf("tiers DimensionIndex = %d, want 1", tiersProp.CollectionAnnotation.DimensionIndex)
	}
	if tiersProp.CollectionAnnotation.IteratorVar != "tier" {
		t.Errorf("tiers IteratorVar = %q, want %q", tiersProp.CollectionAnnotation.IteratorVar, "tier")
	}
}

// TestAddCollectionAnnotations_ResourceSource_NoAnnotation verifies that resource-sourced
// forEach does NOT annotate any schema field (AC3).
func TestAddCollectionAnnotations_ResourceSource_NoAnnotation(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"name": {Type: "string"},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "brokerPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "broker",
						Expression:     "${cluster.status.brokers}",
						Source:         parser.ResourceSource,
						SourcePath:     "status.brokers",
						DimensionIndex: 0,
					},
				},
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	nameProp := schema.Properties["spec"].Properties["name"]
	if nameProp.CollectionAnnotation != nil {
		t.Error("expected no CollectionAnnotation on any field for resource-sourced forEach")
	}
}

// TestAddCollectionAnnotations_NonCollection_NoAnnotation verifies that no annotations
// are added when there are no collection resources (AC4).
func TestAddCollectionAnnotations_NonCollection_NoAnnotation(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"replicas": {Type: "integer"},
					"name":     {Type: "string"},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "deployment",
				Kind:         "Deployment",
				IsTemplate:   true,
				IsCollection: false,
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	for name, prop := range schema.Properties["spec"].Properties {
		if prop.CollectionAnnotation != nil {
			t.Errorf("expected no CollectionAnnotation on %q, got %+v", name, prop.CollectionAnnotation)
		}
	}
}

// TestAddCollectionAnnotations_NestedPath verifies that forEach referencing a nested
// schema field (spec.config.items) correctly navigates the property tree (AC5).
func TestAddCollectionAnnotations_NestedPath(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"config": {
						Type: "object",
						Properties: map[string]models.FormProperty{
							"items": {
								Type:  "array",
								Items: &models.FormProperty{Type: "object"},
							},
						},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "itemPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "item",
						Expression:     "${schema.spec.config.items}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.config.items",
						DimensionIndex: 0,
					},
				},
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	itemsProp := schema.Properties["spec"].Properties["config"].Properties["items"]
	if itemsProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.config.items, got nil")
	}
	if itemsProp.CollectionAnnotation.ResourceID != "itemPods" {
		t.Errorf("ResourceID = %q, want %q", itemsProp.CollectionAnnotation.ResourceID, "itemPods")
	}
	if itemsProp.CollectionAnnotation.IteratorVar != "item" {
		t.Errorf("IteratorVar = %q, want %q", itemsProp.CollectionAnnotation.IteratorVar, "item")
	}
}

// TestAddCollectionAnnotations_InvalidPath_GracefulDegradation verifies that a forEach
// referencing a nonexistent schema field does not error and adds no annotation (AC6).
func TestAddCollectionAnnotations_InvalidPath_GracefulDegradation(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"name": {Type: "string"},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "workerPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "worker",
						Expression:     "${schema.spec.nonexistent}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.nonexistent",
						DimensionIndex: 0,
					},
				},
			},
		},
	}

	// Should not panic or error
	addCollectionAnnotations(schema, graph)

	nameProp := schema.Properties["spec"].Properties["name"]
	if nameProp.CollectionAnnotation != nil {
		t.Error("expected no CollectionAnnotation when path doesn't exist")
	}
}

// TestAddCollectionAnnotations_MultipleCollections verifies that two collection resources
// each with different schema fields are both annotated independently (AC7).
func TestAddCollectionAnnotations_MultipleCollections(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"workers": {
						Type:  "array",
						Items: &models.FormProperty{Type: "object"},
					},
					"services": {
						Type:  "array",
						Items: &models.FormProperty{Type: "object"},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "workerPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "worker",
						Expression:     "${schema.spec.workers}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.workers",
						DimensionIndex: 0,
					},
				},
			},
			{
				ID:           "serviceLBs",
				Kind:         "Service",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "svc",
						Expression:     "${schema.spec.services}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.services",
						DimensionIndex: 0,
					},
				},
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	workersProp := schema.Properties["spec"].Properties["workers"]
	if workersProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.workers, got nil")
	}
	if workersProp.CollectionAnnotation.ResourceID != "workerPods" {
		t.Errorf("workers ResourceID = %q, want %q", workersProp.CollectionAnnotation.ResourceID, "workerPods")
	}

	servicesProp := schema.Properties["spec"].Properties["services"]
	if servicesProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.services, got nil")
	}
	if servicesProp.CollectionAnnotation.ResourceID != "serviceLBs" {
		t.Errorf("services ResourceID = %q, want %q", servicesProp.CollectionAnnotation.ResourceID, "serviceLBs")
	}
}

// TestAddCollectionAnnotations_MixedSources verifies that a resource with both schema-sourced
// and resource-sourced iterators only annotates the schema-sourced field, while dimensions
// reflects the total iterator count (both sources).
func TestAddCollectionAnnotations_MixedSources(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"workers": {
						Type:  "array",
						Items: &models.FormProperty{Type: "object"},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "workerPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "worker",
						Expression:     "${schema.spec.workers}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.workers",
						DimensionIndex: 0,
					},
					{
						Name:           "region",
						Expression:     "${cluster.status.regions}",
						Source:         parser.ResourceSource,
						SourcePath:     "status.regions",
						DimensionIndex: 1,
					},
				},
			},
		},
	}

	addCollectionAnnotations(schema, graph)

	workersProp := schema.Properties["spec"].Properties["workers"]
	if workersProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.workers for schema-sourced iterator")
	}
	if workersProp.CollectionAnnotation.Dimensions != 2 {
		t.Errorf("Dimensions = %d, want 2 (total iterators including resource-sourced)", workersProp.CollectionAnnotation.Dimensions)
	}
	if workersProp.CollectionAnnotation.DimensionIndex != 0 {
		t.Errorf("DimensionIndex = %d, want 0", workersProp.CollectionAnnotation.DimensionIndex)
	}
}

// TestEnrichSchemaFromResources_CollectionAnnotations verifies that collection annotations
// are wired into the EnrichSchemaFromResources pipeline and appear in the output.
func TestEnrichSchemaFromResources_CollectionAnnotations(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"spec": {
				Type: "object",
				Properties: map[string]models.FormProperty{
					"workers": {
						Type:  "array",
						Items: &models.FormProperty{Type: "object"},
					},
				},
			},
		},
	}

	graph := &parser.ResourceGraph{
		Resources: []parser.ResourceDefinition{
			{
				ID:           "workerPods",
				Kind:         "Pod",
				IsCollection: true,
				ForEach: []parser.Iterator{
					{
						Name:           "worker",
						Expression:     "${schema.spec.workers}",
						Source:         parser.SchemaSource,
						SourcePath:     "spec.workers",
						DimensionIndex: 0,
					},
				},
			},
		},
	}

	err := EnrichSchemaFromResources(schema, graph)
	if err != nil {
		t.Fatalf("EnrichSchemaFromResources returned error: %v", err)
	}

	workersProp := schema.Properties["spec"].Properties["workers"]
	if workersProp.CollectionAnnotation == nil {
		t.Fatal("expected CollectionAnnotation on spec.workers after full enrichment pipeline, got nil")
	}
	if workersProp.CollectionAnnotation.ResourceID != "workerPods" {
		t.Errorf("ResourceID = %q, want %q", workersProp.CollectionAnnotation.ResourceID, "workerPods")
	}
}
