package schema

import (
	"strings"
	"testing"
	"time"

	"github.com/provops-org/knodex/server/internal/models"
	"github.com/provops-org/knodex/server/internal/parser"
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

	if !parentProp.ExternalRefSelector.UseInstanceNamespace {
		t.Error("expected UseInstanceNamespace to be true")
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

// TestExtractAdvancedSection_NoAdvancedProperty tests schema without advanced property
func TestExtractAdvancedSection_NoAdvancedProperty(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"port": {Type: "integer"},
		},
	}

	section := extractAdvancedSection(schema)

	if section != nil {
		t.Error("expected nil section for schema without advanced property")
	}
}

// TestExtractAdvancedSection_EmptyAdvancedProperty tests advanced property with no nested fields
func TestExtractAdvancedSection_EmptyAdvancedProperty(t *testing.T) {
	schema := &models.FormSchema{
		Properties: map[string]models.FormProperty{
			"name": {Type: "string"},
			"advanced": {
				Type:       "object",
				Properties: map[string]models.FormProperty{},
			},
		},
	}

	section := extractAdvancedSection(schema)

	if section != nil {
		t.Error("expected nil section for empty advanced property")
	}
}

// TestExtractAdvancedSection_BasicAdvancedFields tests basic advanced field detection
func TestExtractAdvancedSection_BasicAdvancedFields(t *testing.T) {
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

	section := extractAdvancedSection(schema)

	if section == nil {
		t.Fatal("expected non-nil section")
	}

	if section.Path != "advanced" {
		t.Errorf("expected path 'advanced', got %q", section.Path)
	}

	if len(section.AffectedProperties) != 2 {
		t.Errorf("expected 2 affected properties, got %d", len(section.AffectedProperties))
	}
}

// TestExtractAdvancedSection_NestedAdvancedFields tests nested advanced field detection
func TestExtractAdvancedSection_NestedAdvancedFields(t *testing.T) {
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

	section := extractAdvancedSection(schema)

	if section == nil {
		t.Fatal("expected non-nil section")
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
		for _, actual := range section.AffectedProperties {
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

// TestExtractAdvancedSection_RGDDefaultPreserved tests that RGD defaults are preserved
func TestExtractAdvancedSection_RGDDefaultPreserved(t *testing.T) {
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

	section := extractAdvancedSection(schema)

	if section == nil {
		t.Fatal("expected non-nil section")
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

// TestExtractAdvancedSection_MarksPropertiesAdvanced tests that properties are marked with IsAdvanced
func TestExtractAdvancedSection_MarksPropertiesAdvanced(t *testing.T) {
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

	_ = extractAdvancedSection(schema)

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

	if schema.AdvancedSection == nil {
		t.Fatal("expected AdvancedSection to be set")
	}

	if len(schema.AdvancedSection.AffectedProperties) < 3 {
		t.Errorf("expected at least 3 affected properties, got %d", len(schema.AdvancedSection.AffectedProperties))
	}
}
