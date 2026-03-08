// Package cel provides CEL expression utilities for KRO ResourceGraphDefinitions.
//
// This package wraps KRO's pkg/cel for expression compilation and provides
// Knodex-specific CEL analysis for UI condition decomposition.
//
// Architecture:
//   - Expression compilation: delegates to KRO's pkg/cel (typed/untyped environments)
//   - Condition analysis: Knodex-specific AST parsing for frontend rule extraction
//
// The condition analyzer uses a lightweight cel.NewEnv() (parse-only) rather than
// KRO's DefaultEnvironment because it only needs AST parsing, not type checking
// or program compilation.
package cel

import (
	celgo "github.com/google/cel-go/cel"
	krocel "github.com/kubernetes-sigs/kro/pkg/cel"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// NewTypedEnvironment creates a CEL environment with type checking enabled
// using KRO's pkg/cel. This should be used when validating CEL expressions
// against OpenAPI schemas at build time.
func NewTypedEnvironment(schemas map[string]*spec.Schema) (*celgo.Env, error) {
	return krocel.TypedEnvironment(schemas)
}

// NewUntypedEnvironment creates a CEL environment without type declarations
// using KRO's pkg/cel. Resource IDs are declared as variables of type 'any'.
func NewUntypedEnvironment(resourceIDs []string) (*celgo.Env, error) {
	return krocel.UntypedEnvironment(resourceIDs)
}

// NewExpression creates an uncompiled KRO Expression with only the original
// expression string set. References and Program are populated later by the builder.
func NewExpression(expr string) *krocel.Expression {
	return krocel.NewUncompiled(expr)
}

// NewExpressionSlice creates a slice of uncompiled KRO Expressions from strings.
func NewExpressionSlice(exprs ...string) []*krocel.Expression {
	return krocel.NewUncompiledSlice(exprs...)
}
