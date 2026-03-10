// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package cel

import (
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"

	"github.com/knodex/knodex/server/internal/models"
)

// celEnv is a parse-only CEL environment (no type checking, no variable declarations).
// Initialized lazily via sync.Once for thread-safe singleton access.
//
// NOTE (AC4): KRO's pkg/cel.DefaultEnvironment() is a full compilation environment
// with OpenAPI type providers and Kubernetes CEL libraries. The condition analyzer
// only needs env.Parse() for AST decomposition, so we keep the lightweight cel.NewEnv()
// to avoid unnecessary overhead and dependencies.
var (
	celEnv     *cel.Env
	celEnvOnce sync.Once
	celEnvErr  error
)

// getCELEnv returns the singleton parse-only CEL environment.
func getCELEnv() (*cel.Env, error) {
	celEnvOnce.Do(func() {
		celEnv, celEnvErr = cel.NewEnv()
	})
	return celEnv, celEnvErr
}

// stripDelimiters removes the ${...} wrapper from KRO CEL expressions.
// Returns the inner expression unchanged if no delimiters are present.
func stripDelimiters(expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		return strings.TrimSpace(expr[2 : len(expr)-1])
	}
	return expr
}

// AnalyzeCondition parses a CEL expression and extracts structured rules
// that the frontend can evaluate directly.
//
// Returns:
//   - clientEvaluable: true if the expression can be fully evaluated by structured rules
//   - rules: the extracted comparison rules (nil if not client-evaluable)
//
// For expressions that cannot be decomposed (OR, functions, resource references),
// returns clientEvaluable=false with nil rules — the frontend falls back to
// ExpectedValue evaluation or shows the fields (fail open).
func AnalyzeCondition(expr string) (bool, []models.ConditionRule) {
	if expr == "" {
		return false, nil
	}

	env, err := getCELEnv()
	if err != nil {
		return false, nil
	}

	inner := stripDelimiters(expr)
	if inner == "" {
		return false, nil
	}

	astResult, issues := env.Parse(inner)
	if issues != nil && issues.Err() != nil {
		return false, nil
	}

	navExpr := ast.NavigateAST(astResult.NativeRep())

	rules, ok := extractRules(navExpr)
	if !ok || len(rules) == 0 {
		return false, nil
	}

	return true, rules
}

// extractRules recursively extracts ConditionRules from a CEL AST expression.
// Returns the rules and whether extraction was successful.
// Supports: simple comparisons, AND chains, and bare field references (truthy checks).
func extractRules(expr ast.Expr) ([]models.ConditionRule, bool) {
	if expr == nil {
		return nil, false
	}

	switch expr.Kind() {
	case ast.CallKind:
		return extractCallRules(expr)
	case ast.SelectKind:
		// Bare field reference like "schema.spec.enabled" — implicit truthy check
		fieldPath, ok := extractFieldPath(expr)
		if !ok {
			return nil, false
		}
		return []models.ConditionRule{{Field: fieldPath, Op: "==", Value: true}}, true
	case ast.IdentKind:
		// Single identifier — not a schema field path, can't evaluate
		return nil, false
	default:
		return nil, false
	}
}

// extractCallRules handles call expressions (operators and functions).
func extractCallRules(expr ast.Expr) ([]models.ConditionRule, bool) {
	fn := expr.AsCall().FunctionName()

	switch fn {
	case "_&&_":
		// AND chain — recurse both sides and collect all rules
		args := expr.AsCall().Args()
		if len(args) != 2 {
			return nil, false
		}
		leftRules, leftOK := extractRules(args[0])
		if !leftOK {
			return nil, false
		}
		rightRules, rightOK := extractRules(args[1])
		if !rightOK {
			return nil, false
		}
		return append(leftRules, rightRules...), true

	case "_==_", "_!=_", "_>_", "_<_", "_>=_", "_<=_":
		// Comparison operator
		return extractComparison(expr, fn)

	case "_||_", "!_":
		// OR and NOT — not client-evaluable
		return nil, false

	default:
		// Any other function (exists, size, matches, etc.) — not client-evaluable
		return nil, false
	}
}

// extractComparison extracts a single comparison rule from an operator call.
func extractComparison(expr ast.Expr, fn string) ([]models.ConditionRule, bool) {
	args := expr.AsCall().Args()
	if len(args) != 2 {
		return nil, false
	}

	// Try: field OP literal
	fieldPath, fieldOK := extractFieldPath(args[0])
	literal, litOK := extractLiteral(args[1])

	if !fieldOK || !litOK {
		// Try reversed: literal OP field (e.g., true == schema.spec.x)
		fieldPath, fieldOK = extractFieldPath(args[1])
		literal, litOK = extractLiteral(args[0])

		if !fieldOK || !litOK {
			return nil, false
		}

		// Reverse the operator for swapped operands
		fn = reverseOp(fn)
	}

	op := celFnToOp(fn)
	if op == "" {
		return nil, false
	}

	return []models.ConditionRule{{Field: fieldPath, Op: op, Value: literal}}, true
}

// extractFieldPath walks nested SelectKind nodes to reconstruct a field path.
// Returns the path without the "schema." prefix (e.g., "spec.ingress.enabled").
// Returns false if the root identifier is not "schema" (indicating a resource reference).
func extractFieldPath(expr ast.Expr) (string, bool) {
	if expr == nil {
		return "", false
	}

	switch expr.Kind() {
	case ast.SelectKind:
		sel := expr.AsSelect()
		parentPath, ok := extractFieldPath(sel.Operand())
		if !ok {
			return "", false
		}
		if parentPath == "" {
			return sel.FieldName(), true
		}
		return parentPath + "." + sel.FieldName(), true

	case ast.IdentKind:
		name := expr.AsIdent()
		if name != "schema" {
			// Non-schema identifier (e.g., "database", "deployment") — resource reference
			return "", false
		}
		// Return empty string so the first Select adds the field name
		return "", true

	default:
		return "", false
	}
}

// extractLiteral extracts a Go value from a CEL literal node.
func extractLiteral(expr ast.Expr) (any, bool) {
	if expr == nil || expr.Kind() != ast.LiteralKind {
		return nil, false
	}

	lit := expr.AsLiteral()
	val := lit.Value()

	switch v := val.(type) {
	case bool:
		return v, true
	case int64:
		return v, true
	case uint64:
		return v, true
	case float64:
		return v, true
	case string:
		return v, true
	default:
		return nil, false
	}
}

// celFnToOp converts a CEL function name to a comparison operator string.
func celFnToOp(fn string) string {
	switch fn {
	case "_==_":
		return "=="
	case "_!=_":
		return "!="
	case "_>_":
		return ">"
	case "_<_":
		return "<"
	case "_>=_":
		return ">="
	case "_<=_":
		return "<="
	default:
		return ""
	}
}

// reverseOp reverses a comparison operator for swapped operands.
// For commutative ops (==, !=) this is identity; for ordering ops it flips direction.
func reverseOp(fn string) string {
	switch fn {
	case "_>_":
		return "_<_"
	case "_<_":
		return "_>_"
	case "_>=_":
		return "_<=_"
	case "_<=_":
		return "_>=_"
	default:
		return fn // == and != are commutative
	}
}
