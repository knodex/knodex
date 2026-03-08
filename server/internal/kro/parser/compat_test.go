package parser

import (
	"testing"

	kroparser "github.com/kubernetes-sigs/kro/pkg/graph/parser"
)

// TestKROExtractExpressions_Compatibility validates KRO's ParseSchemalessResource
// correctly handles edge cases that the old regex-based extraction could not.
// These tests serve as a compatibility contract with KRO's expression parsing.

func TestKROExtractExpressions_NestedBraces(t *testing.T) {
	// Old regex [^}]+ would fail on nested braces like map literals
	input := map[string]interface{}{
		"value": `${schema.spec.name + "}"}`,
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	expr := descriptors[0].Expressions[0].Original
	expected := `schema.spec.name + "}"`
	if expr != expected {
		t.Errorf("expression = %q, want %q", expr, expected)
	}
}

func TestKROExtractExpressions_StringLiterals(t *testing.T) {
	// Old regex would incorrectly split on } inside string literals
	input := map[string]interface{}{
		"value": `${schema.spec.prefix + "-suffix"}`,
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	expr := descriptors[0].Expressions[0].Original
	expected := `schema.spec.prefix + "-suffix"`
	if expr != expected {
		t.Errorf("expression = %q, want %q", expr, expected)
	}
}

func TestKROExtractExpressions_SimpleExpression(t *testing.T) {
	input := map[string]interface{}{
		"name": "${schema.spec.name}",
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	expr := descriptors[0].Expressions[0].Original
	if expr != "schema.spec.name" {
		t.Errorf("expression = %q, want %q", expr, "schema.spec.name")
	}

	if !descriptors[0].StandaloneExpression {
		t.Error("expected standalone expression")
	}
}

func TestKROExtractExpressions_EmbeddedExpression(t *testing.T) {
	// String with embedded expression is not standalone
	input := map[string]interface{}{
		"label": "app-${schema.spec.name}-v1",
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	if descriptors[0].StandaloneExpression {
		t.Error("expected embedded (non-standalone) expression")
	}

	expr := descriptors[0].Expressions[0].Original
	if expr != "schema.spec.name" {
		t.Errorf("expression = %q, want %q", expr, "schema.spec.name")
	}
}

func TestKROExtractExpressions_MultipleExpressions(t *testing.T) {
	// Multiple expressions in one string
	input := map[string]interface{}{
		"value": "${schema.spec.first}-${schema.spec.second}",
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 1 {
		t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
	}

	if len(descriptors[0].Expressions) != 2 {
		t.Fatalf("expected 2 expressions, got %d", len(descriptors[0].Expressions))
	}

	exprs := make(map[string]bool)
	for _, e := range descriptors[0].Expressions {
		exprs[e.Original] = true
	}
	if !exprs["schema.spec.first"] {
		t.Error("missing expression 'schema.spec.first'")
	}
	if !exprs["schema.spec.second"] {
		t.Error("missing expression 'schema.spec.second'")
	}
}

func TestKROExtractExpressions_NoExpressions(t *testing.T) {
	input := map[string]interface{}{
		"name": "static-value",
	}
	descriptors, plainPaths, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 0 {
		t.Errorf("expected 0 descriptors, got %d", len(descriptors))
	}

	if len(plainPaths) != 1 {
		t.Errorf("expected 1 plain path, got %d", len(plainPaths))
	}
}

func TestKROExtractExpressions_DeepNesting(t *testing.T) {
	// Deeply nested resource should still find expressions
	input := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "${schema.spec.containerName}",
							"image": "${schema.spec.image}",
						},
					},
				},
			},
		},
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	exprs := make(map[string]bool)
	for _, d := range descriptors {
		for _, e := range d.Expressions {
			exprs[e.Original] = true
		}
	}

	if !exprs["schema.spec.containerName"] {
		t.Error("missing 'schema.spec.containerName'")
	}
	if !exprs["schema.spec.image"] {
		t.Error("missing 'schema.spec.image'")
	}
}
