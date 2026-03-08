// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

// compat_test.go — Upstream KRO contract tests for the simpleschema package.
//
// These tests assert the behavior of KRO's pkg/simpleschema.ParseField and
// related type-recognition APIs that Knodex depends on. When a KRO version
// bump causes a failure here, it means the upstream SimpleSchema contract
// changed.
//
// NOTE: These tests import KRO packages directly (not through wrappers)
// because the schema package does not yet wrap simpleschema functions.
// Once wrappers are added, tests should migrate to use the wrapper API.

import (
	"testing"

	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema/types"
)

// TestCompat_ParseField_StringWithDefault verifies that ParseField correctly
// splits a field definition into its type and markers. This is the primary
// API for parsing KRO SimpleSchema variable definitions.
func TestCompat_ParseField_StringWithDefault(t *testing.T) {
	typ, markers, err := simpleschema.ParseField(`string | default="foo"`)
	if err != nil {
		t.Fatalf("ParseField() error = %v", err)
	}

	// Type should be "string" atomic
	if _, ok := typ.(types.Atomic); !ok {
		t.Fatalf("expected types.Atomic, got %T", typ)
	}
	if types.Atomic("string") != typ.(types.Atomic) {
		t.Errorf("type = %v, want string", typ)
	}

	// Should have exactly one marker: default="foo"
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}
	if markers[0].MarkerType != simpleschema.MarkerTypeDefault {
		t.Errorf("marker type = %q, want %q", markers[0].MarkerType, simpleschema.MarkerTypeDefault)
	}
	if markers[0].Value != "foo" {
		t.Errorf("marker value = %q, want %q", markers[0].Value, "foo")
	}
}

// TestCompat_ParseField_AtomicTypes verifies that KRO recognizes the expected
// set of atomic types. If KRO adds or removes atomic types, this test catches it.
func TestCompat_ParseField_AtomicTypes(t *testing.T) {
	atomics := []struct {
		input    string
		expected types.Atomic
	}{
		{"string", types.String},
		{"integer", types.Integer},
		{"boolean", types.Boolean},
		{"float", types.Float},
	}

	for _, tt := range atomics {
		t.Run(tt.input, func(t *testing.T) {
			typ, markers, err := simpleschema.ParseField(tt.input)
			if err != nil {
				t.Fatalf("ParseField(%q) error = %v", tt.input, err)
			}

			atomic, ok := typ.(types.Atomic)
			if !ok {
				t.Fatalf("expected types.Atomic, got %T", typ)
			}
			if atomic != tt.expected {
				t.Errorf("type = %v, want %v", atomic, tt.expected)
			}
			if len(markers) != 0 {
				t.Errorf("expected 0 markers, got %d", len(markers))
			}
		})
	}
}

// TestCompat_IsAtomic verifies the boundary between recognized and unrecognized
// atomic type names. Custom type names should NOT be recognized as atomic.
func TestCompat_IsAtomic(t *testing.T) {
	recognized := []string{"string", "integer", "boolean", "float"}
	for _, s := range recognized {
		if !types.IsAtomic(s) {
			t.Errorf("IsAtomic(%q) = false, want true", s)
		}
	}

	unrecognized := []string{"int", "bool", "str", "MyCustomType", "object", ""}
	for _, s := range unrecognized {
		if types.IsAtomic(s) {
			t.Errorf("IsAtomic(%q) = true, want false", s)
		}
	}
}

// TestCompat_ParseField_RequiredMarker verifies that the required marker is
// correctly parsed from a field definition.
func TestCompat_ParseField_RequiredMarker(t *testing.T) {
	typ, markers, err := simpleschema.ParseField("string | required=true")
	if err != nil {
		t.Fatalf("ParseField() error = %v", err)
	}

	if _, ok := typ.(types.Atomic); !ok {
		t.Fatalf("expected types.Atomic, got %T", typ)
	}

	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d", len(markers))
	}
	if markers[0].MarkerType != simpleschema.MarkerTypeRequired {
		t.Errorf("marker type = %q, want %q", markers[0].MarkerType, simpleschema.MarkerTypeRequired)
	}
	if markers[0].Value != "true" {
		t.Errorf("marker value = %q, want %q", markers[0].Value, "true")
	}
}

// TestCompat_ParseField_SliceType verifies that slice type notation ([]string)
// is correctly parsed, producing a types.Slice with the expected element type.
func TestCompat_ParseField_SliceType(t *testing.T) {
	typ, _, err := simpleschema.ParseField("[]string")
	if err != nil {
		t.Fatalf("ParseField(\"[]string\") error = %v", err)
	}

	slice, ok := typ.(types.Slice)
	if !ok {
		t.Fatalf("expected types.Slice, got %T", typ)
	}

	elem, ok := slice.Elem.(types.Atomic)
	if !ok {
		t.Fatalf("expected slice element types.Atomic, got %T", slice.Elem)
	}
	if elem != types.String {
		t.Errorf("slice element = %v, want string", elem)
	}
}

// TestCompat_ParseField_MapType verifies that map type notation (map[string]integer)
// is correctly parsed.
func TestCompat_ParseField_MapType(t *testing.T) {
	typ, _, err := simpleschema.ParseField("map[string]integer")
	if err != nil {
		t.Fatalf("ParseField(\"map[string]integer\") error = %v", err)
	}

	m, ok := typ.(types.Map)
	if !ok {
		t.Fatalf("expected types.Map, got %T", typ)
	}

	// Note: types.Map has no Key field — KRO maps always use string keys by design.

	val, ok := m.Value.(types.Atomic)
	if !ok {
		t.Fatalf("expected map value types.Atomic, got %T", m.Value)
	}
	if val != types.Integer {
		t.Errorf("map value type = %v, want integer", val)
	}
}

// TestCompat_ToOpenAPISpec_NestedObject verifies that ToOpenAPISpec correctly
// handles nested object definitions, producing the expected OpenAPI schema
// structure with properties and required fields.
func TestCompat_ToOpenAPISpec_NestedObject(t *testing.T) {
	input := map[string]interface{}{
		"name": "string | required=true",
		"config": map[string]interface{}{
			"replicas": "integer | default=3",
			"enabled":  "boolean",
		},
	}

	schema, err := simpleschema.ToOpenAPISpec(input, nil)
	if err != nil {
		t.Fatalf("ToOpenAPISpec() error = %v", err)
	}

	if schema.Type != "object" {
		t.Errorf("root type = %q, want %q", schema.Type, "object")
	}

	// "name" should be in required list
	found := false
	for _, r := range schema.Required {
		if r == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'name' should be in required list")
	}

	// "name" property should be string type
	nameProp, ok := schema.Properties["name"]
	if !ok {
		t.Fatal("missing 'name' property")
	}
	if nameProp.Type != "string" {
		t.Errorf("name type = %q, want %q", nameProp.Type, "string")
	}

	// "config" property should be an object with its own properties
	configProp, ok := schema.Properties["config"]
	if !ok {
		t.Fatal("missing 'config' property")
	}
	if configProp.Type != "object" {
		t.Errorf("config type = %q, want %q", configProp.Type, "object")
	}

	replicasProp, ok := configProp.Properties["replicas"]
	if !ok {
		t.Fatal("missing 'config.replicas' property")
	}
	if replicasProp.Type != "integer" {
		t.Errorf("replicas type = %q, want %q", replicasProp.Type, "integer")
	}
	if replicasProp.Default == nil {
		t.Error("replicas should have a default value")
	}
}

// TestCompat_ParseField_MultipleMarkers verifies that multiple markers on a
// single field definition are all correctly parsed.
func TestCompat_ParseField_MultipleMarkers(t *testing.T) {
	typ, markers, err := simpleschema.ParseField(`string | required=true default="bar" description="A field"`)
	if err != nil {
		t.Fatalf("ParseField() error = %v", err)
	}

	if _, ok := typ.(types.Atomic); !ok {
		t.Fatalf("expected types.Atomic, got %T", typ)
	}

	if len(markers) != 3 {
		t.Fatalf("expected 3 markers, got %d", len(markers))
	}

	markerTypes := make(map[simpleschema.MarkerType]string)
	for _, m := range markers {
		markerTypes[m.MarkerType] = m.Value
	}

	if v, ok := markerTypes[simpleschema.MarkerTypeRequired]; !ok || v != "true" {
		t.Errorf("expected required=true, got %q", v)
	}
	if v, ok := markerTypes[simpleschema.MarkerTypeDefault]; !ok || v != "bar" {
		t.Errorf("expected default=bar, got %q", v)
	}
	if v, ok := markerTypes[simpleschema.MarkerTypeDescription]; !ok || v != "A field" {
		t.Errorf("expected description='A field', got %q", v)
	}
}

// TestCompat_ParseField_CustomType verifies that unrecognized type names are
// treated as references to custom types (types.Custom), not as errors.
func TestCompat_ParseField_CustomType(t *testing.T) {
	typ, _, err := simpleschema.ParseField("MyCustomType")
	if err != nil {
		t.Fatalf("ParseField(\"MyCustomType\") error = %v", err)
	}

	custom, ok := typ.(types.Custom)
	if !ok {
		t.Fatalf("expected types.Custom, got %T", typ)
	}
	if string(custom) != "MyCustomType" {
		t.Errorf("custom type = %q, want %q", string(custom), "MyCustomType")
	}
}
