// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package cel

import (
	"testing"

	krocel "github.com/kubernetes-sigs/kro/pkg/cel"
)

// TestKROExpression_StructFields validates that KRO's Expression type
// has the expected fields. This serves as a compatibility contract —
// if KRO changes the Expression struct, these tests will catch it.

func TestKROExpression_NewUncompiled(t *testing.T) {
	expr := krocel.NewUncompiled("schema.spec.name")

	if expr.Original != "schema.spec.name" {
		t.Errorf("Original = %q, want %q", expr.Original, "schema.spec.name")
	}

	if expr.References != nil {
		t.Errorf("References should be nil for uncompiled expression, got %v", expr.References)
	}

	if expr.Program != nil {
		t.Error("Program should be nil for uncompiled expression")
	}
}

func TestKROExpression_NewUncompiledSlice(t *testing.T) {
	exprs := krocel.NewUncompiledSlice("schema.spec.a", "schema.spec.b")

	if len(exprs) != 2 {
		t.Fatalf("expected 2 expressions, got %d", len(exprs))
	}

	if exprs[0].Original != "schema.spec.a" {
		t.Errorf("exprs[0].Original = %q, want %q", exprs[0].Original, "schema.spec.a")
	}
	if exprs[1].Original != "schema.spec.b" {
		t.Errorf("exprs[1].Original = %q, want %q", exprs[1].Original, "schema.spec.b")
	}
}

func TestKROExpression_FieldsExist(t *testing.T) {
	// Verify the Expression struct has the expected fields by assignment.
	// This is a compile-time contract test — if KRO renames or removes
	// these fields, this test will fail to compile.
	expr := &krocel.Expression{
		Original:   "schema.spec.name",
		References: []string{"schema"},
		Program:    nil,
	}

	if expr.Original != "schema.spec.name" {
		t.Errorf("Original = %q", expr.Original)
	}
	if len(expr.References) != 1 || expr.References[0] != "schema" {
		t.Errorf("References = %v", expr.References)
	}
}
