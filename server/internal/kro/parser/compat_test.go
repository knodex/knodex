// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"testing"

	kroparser "github.com/kubernetes-sigs/kro/pkg/graph/parser"
)

// TestKROExtractExpressions_Compatibility validates KRO's ParseSchemalessResource
// correctly handles edge cases that the old regex-based extraction could not.
// These tests serve as a compatibility contract with KRO's expression parsing.
//
// NOTE: In KRO v0.9.0, FieldDescriptor changed from Expressions []*Expression
// (plural slice) to Expression *Expression (singular pointer). String templates
// like "prefix-${a}-${b}" are now compiled into a single CEL concatenation
// expression at parse time, yielding one FieldDescriptor per template string.

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

	if descriptors[0].Expression == nil {
		t.Fatal("expected non-nil Expression")
	}

	expr := descriptors[0].Expression.Original
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

	if descriptors[0].Expression == nil {
		t.Fatal("expected non-nil Expression")
	}

	expr := descriptors[0].Expression.Original
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

	if descriptors[0].Expression == nil {
		t.Fatal("expected non-nil Expression")
	}

	expr := descriptors[0].Expression.Original
	if expr != "schema.spec.name" {
		t.Errorf("expression = %q, want %q", expr, "schema.spec.name")
	}
}

func TestKROExtractExpressions_EmbeddedExpression(t *testing.T) {
	// String with embedded expression — in v0.9.0, templates are compiled
	// into a single CEL concatenation expression at parse time.
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

	if descriptors[0].Expression == nil {
		t.Fatal("expected non-nil Expression")
	}

	// The expression should contain the schema.spec.name reference — verify it
	// is extractable by the downstream field-extraction logic, not just non-empty.
	expr := descriptors[0].Expression.Original
	fields := extractBareSchemaFields(expr)
	if len(fields) != 1 || fields[0] != "spec.name" {
		t.Errorf("schema fields from expression %q = %v, want [spec.name]", expr, fields)
	}
}

func TestKROExtractExpressions_MultipleExpressions(t *testing.T) {
	// Multiple expressions in one string — in v0.9.0, these are compiled
	// into a single concatenation expression, yielding one FieldDescriptor.
	// Asserting exactly 1 descriptor locks down the v0.9.0 contract; if KRO
	// reverts to returning 2 (v0.8.x behaviour), this test will catch it.
	input := map[string]interface{}{
		"value": "${schema.spec.first}-${schema.spec.second}",
	}
	descriptors, _, err := kroparser.ParseSchemalessResource(input)
	if err != nil {
		t.Fatalf("ParseSchemalessResource() error = %v", err)
	}

	if len(descriptors) != 1 {
		t.Fatalf("expected exactly 1 descriptor (v0.9.0 compiles templates to single concatenation expression), got %d", len(descriptors))
	}

	if descriptors[0].Expression == nil {
		t.Fatal("expected non-nil Expression")
	}

	// Both schema references must be extractable from the single concatenated expression.
	expr := descriptors[0].Expression.Original
	fields := extractBareSchemaFields(expr)
	fieldSet := make(map[string]bool)
	for _, f := range fields {
		fieldSet[f] = true
	}
	if !fieldSet["spec.first"] || !fieldSet["spec.second"] {
		t.Errorf("expected both spec.first and spec.second in expression %q, got fields: %v", expr, fields)
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
		if d.Expression != nil {
			exprs[d.Expression.Original] = true
		}
	}

	if !exprs["schema.spec.containerName"] {
		t.Error("missing 'schema.spec.containerName'")
	}
	if !exprs["schema.spec.image"] {
		t.Error("missing 'schema.spec.image'")
	}
}
