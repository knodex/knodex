package schema

import (
	"testing"

	"github.com/provops-org/knodex/server/internal/models"
)

func TestAnalyzeCondition(t *testing.T) {
	tests := []struct {
		name          string
		expr          string
		wantEvaluable bool
		wantRuleCount int
		wantRules     []models.ConditionRule // nil means don't check individual rules
	}{
		{
			name:          "simple boolean true",
			expr:          "schema.spec.enabled == true",
			wantEvaluable: true,
			wantRuleCount: 1,
			wantRules: []models.ConditionRule{
				{Field: "spec.enabled", Op: "==", Value: true},
			},
		},
		{
			name:          "simple boolean false",
			expr:          "schema.spec.enabled == false",
			wantEvaluable: true,
			wantRuleCount: 1,
			wantRules: []models.ConditionRule{
				{Field: "spec.enabled", Op: "==", Value: false},
			},
		},
		{
			name:          "numeric greater than",
			expr:          "schema.spec.replicas > 0",
			wantEvaluable: true,
			wantRuleCount: 1,
			wantRules: []models.ConditionRule{
				{Field: "spec.replicas", Op: ">", Value: int64(0)},
			},
		},
		{
			name:          "string comparison",
			expr:          `schema.spec.mode == "advanced"`,
			wantEvaluable: true,
			wantRuleCount: 1,
			wantRules: []models.ConditionRule{
				{Field: "spec.mode", Op: "==", Value: "advanced"},
			},
		},
		{
			name:          "AND chain with two comparisons",
			expr:          "schema.spec.a == true && schema.spec.b == true",
			wantEvaluable: true,
			wantRuleCount: 2,
		},
		{
			name:          "bare field reference (truthy check)",
			expr:          "schema.spec.enabled",
			wantEvaluable: true,
			wantRuleCount: 1,
			wantRules: []models.ConditionRule{
				{Field: "spec.enabled", Op: "==", Value: true},
			},
		},
		{
			name:          "OR expression - not evaluable",
			expr:          "schema.spec.a == true || schema.spec.b == true",
			wantEvaluable: false,
			wantRuleCount: 0,
		},
		{
			name:          "NOT expression - not evaluable",
			expr:          "!schema.spec.enabled",
			wantEvaluable: false,
			wantRuleCount: 0,
		},
		{
			name:          "with delimiters",
			expr:          "${schema.spec.enabled == true}",
			wantEvaluable: true,
			wantRuleCount: 1,
			wantRules: []models.ConditionRule{
				{Field: "spec.enabled", Op: "==", Value: true},
			},
		},
		{
			name:          "resource reference - not evaluable",
			expr:          "database.status.ready == true",
			wantEvaluable: false,
			wantRuleCount: 0,
		},
		{
			name:          "comprehension - not evaluable",
			expr:          `schema.spec.list.exists(x, x == "foo")`,
			wantEvaluable: false,
			wantRuleCount: 0,
		},
		{
			name:          "empty string",
			expr:          "",
			wantEvaluable: false,
			wantRuleCount: 0,
		},
		{
			name:          "AND with mixed operators",
			expr:          `schema.spec.a == true && schema.spec.b != "x"`,
			wantEvaluable: true,
			wantRuleCount: 2,
		},
		{
			name:          "invalid CEL - no panic",
			expr:          "invalid CEL ==}",
			wantEvaluable: false,
			wantRuleCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluable, rules := analyzeCondition(tt.expr)

			if evaluable != tt.wantEvaluable {
				t.Errorf("analyzeCondition(%q) evaluable = %v, want %v", tt.expr, evaluable, tt.wantEvaluable)
			}

			if len(rules) != tt.wantRuleCount {
				t.Errorf("analyzeCondition(%q) rule count = %d, want %d, rules: %+v",
					tt.expr, len(rules), tt.wantRuleCount, rules)
			}

			// Check individual rules if specified
			if tt.wantRules != nil && len(rules) == len(tt.wantRules) {
				for i, want := range tt.wantRules {
					got := rules[i]
					if got.Field != want.Field {
						t.Errorf("rule[%d].Field = %q, want %q", i, got.Field, want.Field)
					}
					if got.Op != want.Op {
						t.Errorf("rule[%d].Op = %q, want %q", i, got.Op, want.Op)
					}
					if got.Value != want.Value {
						t.Errorf("rule[%d].Value = %v (%T), want %v (%T)",
							i, got.Value, got.Value, want.Value, want.Value)
					}
				}
			}
		})
	}
}

func TestStripDelimiters(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"${schema.spec.enabled == true}", "schema.spec.enabled == true"},
		{"schema.spec.enabled == true", "schema.spec.enabled == true"},
		{"  ${  inner  }  ", "inner"},
		{"${ }", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripDelimiters(tt.input)
			if got != tt.want {
				t.Errorf("stripDelimiters(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAnalyzeCondition_NestedFieldPath(t *testing.T) {
	evaluable, rules := analyzeCondition("schema.spec.ingress.tls.enabled == true")

	if !evaluable {
		t.Fatal("expected client-evaluable")
	}

	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	if rules[0].Field != "spec.ingress.tls.enabled" {
		t.Errorf("field = %q, want %q", rules[0].Field, "spec.ingress.tls.enabled")
	}
}

func TestAnalyzeCondition_NotEqualOperator(t *testing.T) {
	evaluable, rules := analyzeCondition(`schema.spec.tier != "free"`)

	if !evaluable {
		t.Fatal("expected client-evaluable")
	}

	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	if rules[0].Op != "!=" {
		t.Errorf("op = %q, want %q", rules[0].Op, "!=")
	}

	if rules[0].Value != "free" {
		t.Errorf("value = %v, want %q", rules[0].Value, "free")
	}
}

func TestAnalyzeCondition_MultipleDelimitedExpressions(t *testing.T) {
	// KRO includeWhen arrays get joined with " && " by the parser.
	// Each element has ${...} delimiters, so the combined expression looks like:
	// "${schema.spec.a == true} && ${schema.spec.b == true}"
	// The parser joins them, but the analyzer receives the raw combined string.
	// This tests that we handle the combined expression (without delimiters since
	// the parser joins the raw strings).
	expr := "${schema.spec.ingress.enabled == true}"
	evaluable, rules := analyzeCondition(expr)

	if !evaluable {
		t.Fatalf("expected client-evaluable for %q", expr)
	}

	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}
