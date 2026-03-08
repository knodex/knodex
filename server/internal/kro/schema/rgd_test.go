package schema

import (
	"testing"

	"github.com/kubernetes-sigs/kro/pkg/simpleschema/types"

	"github.com/knodex/knodex/server/internal/models"
)

func TestParseRGDSchema_NilSpec(t *testing.T) {
	intent, err := ParseRGDSchema(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent != nil {
		t.Fatal("expected nil intent for nil spec")
	}
}

func TestParseRGDSchema_NoSchemaSection(t *testing.T) {
	intent, err := ParseRGDSchema(map[string]interface{}{
		"resources": []interface{}{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent != nil {
		t.Fatal("expected nil intent when schema section is missing")
	}
}

func TestParseRGDSchema_NoSpecInSchema(t *testing.T) {
	intent, err := ParseRGDSchema(map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "MyApp",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent != nil {
		t.Fatal("expected nil intent when schema.spec is missing")
	}
}

func TestParseRGDSchema_BasicFields(t *testing.T) {
	rawSpec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "WebApp",
			"spec": map[string]interface{}{
				"name":     "string",
				"replicas": "integer | default=3",
				"enabled":  "boolean | default=true",
				"ratio":    "float",
			},
		},
	}

	intent, err := ParseRGDSchema(rawSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent == nil {
		t.Fatal("expected non-nil intent")
	}

	tests := []struct {
		path        string
		wantType    string
		wantDefault string
	}{
		{"name", "string", ""},
		{"replicas", "integer", "3"},
		{"enabled", "boolean", "true"},
		{"ratio", "number", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			fi, ok := intent.Fields[tt.path]
			if !ok {
				t.Fatalf("field %q not found in intent", tt.path)
			}
			if fi.Type != tt.wantType {
				t.Errorf("type = %q, want %q", fi.Type, tt.wantType)
			}
			if fi.Default != tt.wantDefault {
				t.Errorf("default = %q, want %q", fi.Default, tt.wantDefault)
			}
		})
	}
}

func TestParseRGDSchema_Markers(t *testing.T) {
	rawSpec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "TestApp",
			"spec": map[string]interface{}{
				"name": `string | required=true description="The app name"`,
				"port": `integer | default=8080 minimum=1 maximum=65535`,
			},
		},
	}

	intent, err := ParseRGDSchema(rawSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// name: description extracted (required marker is parsed but not stored — CRD is authoritative)
	name := intent.Fields["name"]
	if name.Description != "The app name" {
		t.Errorf("name description = %q, want %q", name.Description, "The app name")
	}

	// port: default extracted (validation markers are parsed by simpleschema but not stored)
	port := intent.Fields["port"]
	if port.Default != "8080" {
		t.Errorf("port default = %q, want %q", port.Default, "8080")
	}
}

func TestParseRGDSchema_NestedObject(t *testing.T) {
	rawSpec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "TestApp",
			"spec": map[string]interface{}{
				"name": "string",
				"config": map[string]interface{}{
					"replicas": "integer | default=2",
					"resources": map[string]interface{}{
						"cpu":    `string | default="500m"`,
						"memory": `string | default="512Mi"`,
					},
				},
			},
		},
	}

	intent, err := ParseRGDSchema(rawSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check top-level
	if _, ok := intent.Fields["name"]; !ok {
		t.Error("missing field 'name'")
	}

	// Parent objects should NOT be stored (only leaf fields are stored)
	if _, ok := intent.Fields["config"]; ok {
		t.Error("parent object 'config' should not be stored in intent")
	}
	if _, ok := intent.Fields["config.resources"]; ok {
		t.Error("parent object 'config.resources' should not be stored in intent")
	}

	// Check nested leaf fields
	replicas := intent.Fields["config.replicas"]
	if replicas.Type != "integer" {
		t.Errorf("config.replicas type = %q, want %q", replicas.Type, "integer")
	}
	if replicas.Default != "2" {
		t.Errorf("config.replicas default = %q, want %q", replicas.Default, "2")
	}

	cpu := intent.Fields["config.resources.cpu"]
	if cpu.Default != "500m" {
		t.Errorf("config.resources.cpu default = %q, want %q", cpu.Default, "500m")
	}
}

func TestParseRGDSchema_SliceType(t *testing.T) {
	rawSpec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "TestApp",
			"spec": map[string]interface{}{
				"tags": "[]string",
			},
		},
	}

	intent, err := ParseRGDSchema(rawSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tags := intent.Fields["tags"]
	if tags.Type != "array" {
		t.Errorf("tags type = %q, want %q", tags.Type, "array")
	}
}

func TestParseRGDSchema_MapType(t *testing.T) {
	rawSpec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "TestApp",
			"spec": map[string]interface{}{
				"labels": "map[string]string",
			},
		},
	}

	intent, err := ParseRGDSchema(rawSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	labels := intent.Fields["labels"]
	if labels.Type != "object" {
		t.Errorf("labels type = %q, want %q", labels.Type, "object")
	}
}

func TestParseRGDSchema_WebappFullFeatured(t *testing.T) {
	// Mirrors the reference webapp-full-featured.yaml example
	rawSpec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "WebApp",
			"spec": map[string]interface{}{
				"name":              "string",
				"image":             `string | default="nginx:latest"`,
				"port":              "integer | default=8080",
				"visibleAnnotation": `string | default=""`,
				"hiddenAnnotation":  `string | default=""`,
				"enableDatabase":    "boolean | default=false",
				"enableCache":       "boolean | default=false",
				"database": map[string]interface{}{
					"name": `string | default="mydb"`,
				},
				"advanced": map[string]interface{}{
					"replicas": "integer | default=2",
					"resources": map[string]interface{}{
						"limits": map[string]interface{}{
							"cpu":    `string | default="500m"`,
							"memory": `string | default="512Mi"`,
						},
					},
					"healthCheck": map[string]interface{}{
						"enabled":  "boolean | default=true",
						"path":     `string | default="/health"`,
						"interval": "integer | default=30",
					},
				},
			},
		},
	}

	intent, err := ParseRGDSchema(rawSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only leaf fields are stored (parent objects are not recorded in intent).
	// Top-level leaves: name, image, port, visibleAnnotation, hiddenAnnotation, enableDatabase, enableCache
	// Nested leaves: database.name, advanced.replicas, advanced.resources.limits.cpu, advanced.resources.limits.memory,
	//               advanced.healthCheck.enabled, advanced.healthCheck.path, advanced.healthCheck.interval
	expectedFields := []string{
		"name", "image", "port", "visibleAnnotation", "hiddenAnnotation",
		"enableDatabase", "enableCache",
		"database.name",
		"advanced.replicas",
		"advanced.resources.limits.cpu", "advanced.resources.limits.memory",
		"advanced.healthCheck.enabled",
		"advanced.healthCheck.path", "advanced.healthCheck.interval",
	}

	for _, path := range expectedFields {
		if _, ok := intent.Fields[path]; !ok {
			t.Errorf("missing field %q", path)
		}
	}

	// Verify specific values
	if intent.Fields["enableDatabase"].Type != "boolean" {
		t.Errorf("enableDatabase type = %q, want boolean", intent.Fields["enableDatabase"].Type)
	}
	if intent.Fields["enableDatabase"].Default != "false" {
		t.Errorf("enableDatabase default = %q, want false", intent.Fields["enableDatabase"].Default)
	}
	if intent.Fields["advanced.replicas"].Default != "2" {
		t.Errorf("advanced.replicas default = %q, want 2", intent.Fields["advanced.replicas"].Default)
	}
}

func TestTypeToOpenAPI(t *testing.T) {
	tests := []struct {
		name string
		typ  types.Type
		want string
	}{
		{"string", types.Atomic("string"), "string"},
		{"integer", types.Atomic("integer"), "integer"},
		{"boolean", types.Atomic("boolean"), "boolean"},
		{"float->number", types.Atomic("float"), "number"},
		{"slice->array", types.Slice{Elem: types.Atomic("string")}, "array"},
		{"map->object", types.Map{Value: types.Atomic("string")}, "object"},
		{"object->object", types.Object{}, "object"},
		{"custom->string", types.Custom("MyType"), "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeToOpenAPI(tt.typ)
			if got != tt.want {
				t.Errorf("typeToOpenAPI(%v) = %q, want %q", tt.typ, got, tt.want)
			}
		})
	}
}

func TestParseFieldDefinition_InvalidInput(t *testing.T) {
	// Empty type should error
	_, err := parseFieldDefinition("")
	if err == nil {
		t.Error("expected error for empty definition")
	}
}

func TestBuildFormSchemaFromRGD_NilRGD(t *testing.T) {
	_, err := BuildFormSchemaFromRGD(nil)
	if err == nil {
		t.Fatal("expected error for nil RGD")
	}
}

func TestBuildFormSchemaFromRGD_NilRawSpec(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:      "test",
		Namespace: "default",
		RawSpec:   nil,
	}
	_, err := BuildFormSchemaFromRGD(rgd)
	if err == nil {
		t.Fatal("expected error for nil rawSpec")
	}
}

func TestBuildFormSchemaFromRGD_EmptySchema(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:      "test",
		Namespace: "default",
		RawSpec:   map[string]interface{}{},
	}
	_, err := BuildFormSchemaFromRGD(rgd)
	if err == nil {
		t.Fatal("expected error when schema section is missing")
	}
}

func TestBuildFormSchemaFromRGD_BasicTypes(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:        "my-app",
		Namespace:   "default",
		Description: "A test app",
		APIVersion:  "example.com/v1alpha1",
		Kind:        "MyApp",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "example.com/v1alpha1",
				"kind":       "MyApp",
				"spec": map[string]interface{}{
					"name":     "string",
					"replicas": "integer | default=3",
					"enabled":  "boolean | default=true",
					"ratio":    "float",
				},
			},
		},
	}

	schema, err := BuildFormSchemaFromRGD(rgd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify metadata
	if schema.Name != "my-app" {
		t.Errorf("name = %q, want %q", schema.Name, "my-app")
	}
	if schema.Group != "example.com" {
		t.Errorf("group = %q, want %q", schema.Group, "example.com")
	}
	if schema.Kind != "MyApp" {
		t.Errorf("kind = %q, want %q", schema.Kind, "MyApp")
	}
	if schema.Version != "v1alpha1" {
		t.Errorf("version = %q, want %q", schema.Version, "v1alpha1")
	}

	// Verify properties
	tests := []struct {
		field       string
		wantType    string
		wantDefault interface{}
	}{
		{"name", "string", nil},
		{"replicas", "integer", int64(3)},
		{"enabled", "boolean", true},
		{"ratio", "number", nil},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			prop, ok := schema.Properties[tt.field]
			if !ok {
				t.Fatalf("property %q not found", tt.field)
			}
			if prop.Type != tt.wantType {
				t.Errorf("type = %q, want %q", prop.Type, tt.wantType)
			}
			if tt.wantDefault != nil && prop.Default != tt.wantDefault {
				t.Errorf("default = %v (%T), want %v (%T)", prop.Default, prop.Default, tt.wantDefault, tt.wantDefault)
			}
		})
	}
}

func TestBuildFormSchemaFromRGD_NestedObjects(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:       "nested-app",
		Namespace:  "default",
		APIVersion: "test.io/v1",
		Kind:       "NestedApp",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "test.io/v1",
				"kind":       "NestedApp",
				"spec": map[string]interface{}{
					"name": "string",
					"config": map[string]interface{}{
						"replicas": "integer | default=2",
						"resources": map[string]interface{}{
							"cpu":    `string | default="500m"`,
							"memory": `string | default="512Mi"`,
						},
					},
				},
			},
		},
	}

	schema, err := BuildFormSchemaFromRGD(rgd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check top-level name
	if _, ok := schema.Properties["name"]; !ok {
		t.Fatal("missing property 'name'")
	}

	// Check config is an object
	config, ok := schema.Properties["config"]
	if !ok {
		t.Fatal("missing property 'config'")
	}
	if config.Type != "object" {
		t.Errorf("config.type = %q, want %q", config.Type, "object")
	}

	// Check nested replicas
	replicas, ok := config.Properties["replicas"]
	if !ok {
		t.Fatal("missing config.replicas")
	}
	if replicas.Type != "integer" {
		t.Errorf("replicas.type = %q, want %q", replicas.Type, "integer")
	}
	if replicas.Default != int64(2) {
		t.Errorf("replicas.default = %v, want %v", replicas.Default, int64(2))
	}

	// Check doubly nested resources.cpu
	resources, ok := config.Properties["resources"]
	if !ok {
		t.Fatal("missing config.resources")
	}
	cpu, ok := resources.Properties["cpu"]
	if !ok {
		t.Fatal("missing config.resources.cpu")
	}
	if cpu.Default != "500m" {
		t.Errorf("cpu.default = %v, want %q", cpu.Default, "500m")
	}
}

func TestBuildFormSchemaFromRGD_FieldsWithDescriptions(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:       "desc-app",
		Namespace:  "default",
		APIVersion: "test.io/v1",
		Kind:       "DescApp",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "test.io/v1",
				"kind":       "DescApp",
				"spec": map[string]interface{}{
					"name": `string | description="The application name"`,
					"port": `integer | default=8080 description="Port number"`,
				},
			},
		},
	}

	schema, err := BuildFormSchemaFromRGD(rgd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name := schema.Properties["name"]
	if name.Description != "The application name" {
		t.Errorf("name.description = %q, want %q", name.Description, "The application name")
	}

	port := schema.Properties["port"]
	if port.Description != "Port number" {
		t.Errorf("port.description = %q, want %q", port.Description, "Port number")
	}
	if port.Default != int64(8080) {
		t.Errorf("port.default = %v, want %v", port.Default, int64(8080))
	}
}

func TestBuildFormSchemaFromRGD_ArrayFieldHasItems(t *testing.T) {
	rgd := &models.CatalogRGD{
		Name:       "array-app",
		Namespace:  "default",
		APIVersion: "test.io/v1",
		Kind:       "ArrayApp",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "test.io/v1",
				"kind":       "ArrayApp",
				"spec": map[string]interface{}{
					"tags":  "[]string",
					"ports": "[]integer",
				},
			},
		},
	}

	schema, err := BuildFormSchemaFromRGD(rgd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// tags should have Items with type "string"
	tags := schema.Properties["tags"]
	if tags.Type != "array" {
		t.Errorf("tags.type = %q, want %q", tags.Type, "array")
	}
	if tags.Items == nil {
		t.Fatal("tags.Items should not be nil for []string")
	}
	if tags.Items.Type != "string" {
		t.Errorf("tags.Items.type = %q, want %q", tags.Items.Type, "string")
	}

	// ports should have Items with type "integer"
	ports := schema.Properties["ports"]
	if ports.Items == nil {
		t.Fatal("ports.Items should not be nil for []integer")
	}
	if ports.Items.Type != "integer" {
		t.Errorf("ports.Items.type = %q, want %q", ports.Items.Type, "integer")
	}
}

func TestBuildFormSchemaFromRGD_NoValidationConstraints(t *testing.T) {
	// Verify that the degraded schema does NOT include validation constraints
	// even if simpleschema markers exist — those come from the CRD only
	rgd := &models.CatalogRGD{
		Name:       "no-validation",
		Namespace:  "default",
		APIVersion: "test.io/v1",
		Kind:       "NoVal",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "test.io/v1",
				"kind":       "NoVal",
				"spec": map[string]interface{}{
					"name": `string | required=true`,
					"port": `integer | default=8080 minimum=1 maximum=65535`,
				},
			},
		},
	}

	schema, err := BuildFormSchemaFromRGD(rgd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No Required list should be set (required comes from CRD)
	if len(schema.Required) > 0 {
		t.Errorf("expected no required fields in degraded schema, got %v", schema.Required)
	}

	port := schema.Properties["port"]
	if port.Minimum != nil {
		t.Errorf("expected no minimum in degraded schema, got %v", *port.Minimum)
	}
	if port.Maximum != nil {
		t.Errorf("expected no maximum in degraded schema, got %v", *port.Maximum)
	}
	if port.Pattern != "" {
		t.Errorf("expected no pattern in degraded schema, got %q", port.Pattern)
	}
}

func TestParseDefault(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		value     string
		want      interface{}
	}{
		{"integer", "integer", "42", int64(42)},
		{"number", "number", "3.14", 3.14},
		{"boolean true", "boolean", "true", true},
		{"boolean false", "boolean", "false", false},
		{"string", "string", "hello", "hello"},
		{"invalid integer", "integer", "abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDefault(tt.fieldType, tt.value)
			if got != tt.want {
				t.Errorf("parseDefault(%q, %q) = %v (%T), want %v (%T)",
					tt.fieldType, tt.value, got, got, tt.want, tt.want)
			}
		})
	}
}
