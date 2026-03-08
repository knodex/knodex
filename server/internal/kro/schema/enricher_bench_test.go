// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"testing"

	"github.com/knodex/knodex/server/internal/models"
)

// BenchmarkExtractConditionalSections_Small benchmarks with 10 conditional resources
func BenchmarkExtractConditionalSections_Small(b *testing.B) {
	graph := generateLargeResourceGraph(10)
	schema := &models.FormSchema{
		Properties: generateLargeSchema(10),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractConditionalSections(graph, schema)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractConditionalSections_Medium benchmarks with 25 conditional resources
func BenchmarkExtractConditionalSections_Medium(b *testing.B) {
	graph := generateLargeResourceGraph(25)
	schema := &models.FormSchema{
		Properties: generateLargeSchema(25),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractConditionalSections(graph, schema)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractConditionalSections_Large benchmarks with 50 conditional resources
func BenchmarkExtractConditionalSections_Large(b *testing.B) {
	graph := generateLargeResourceGraph(50)
	schema := &models.FormSchema{
		Properties: generateLargeSchema(50),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractConditionalSections(graph, schema)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractConditionalSections_VeryLarge benchmarks with 100 conditional resources
func BenchmarkExtractConditionalSections_VeryLarge(b *testing.B) {
	graph := generateLargeResourceGraph(100)
	schema := &models.FormSchema{
		Properties: generateLargeSchema(100),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractConditionalSections(graph, schema)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSectionBuilder_AddField benchmarks the O(1) duplicate detection
func BenchmarkSectionBuilder_AddField(b *testing.B) {
	builder := &sectionBuilder{
		ConditionalSection: models.ConditionalSection{
			AffectedProperties: make([]string, 0, 100),
		},
		affectedSet: make(map[string]struct{}, 100),
	}

	fields := make([]string, 100)
	for i := 0; i < 100; i++ {
		fields[i] = "field" + string(rune(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.AffectedProperties = builder.AffectedProperties[:0]
		builder.affectedSet = make(map[string]struct{}, 100)

		for _, field := range fields {
			builder.addAffectedField(field)
		}
	}
}

// BenchmarkValidateControllingField benchmarks field validation
func BenchmarkValidateControllingField(b *testing.B) {
	schema := &models.FormSchema{
		Properties: generateLargeSchema(100),
	}

	fields := []string{"enabled", "advanced", "feature", "config10", "config50"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, field := range fields {
			_ = validateControllingField(field, schema)
		}
	}
}

// BenchmarkExtractExpectedValue benchmarks expression parsing
func BenchmarkExtractExpectedValue(b *testing.B) {
	expressions := []string{
		"schema.spec.enabled == true",
		"schema.spec.enabled == false",
		"schema.spec.enabled",
		"schema.spec.complex && other == true",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			_ = extractExpectedValue(expr)
		}
	}
}

// BenchmarkEnrichSchemaFromResources_Complete benchmarks the full enrichment process
func BenchmarkEnrichSchemaFromResources_Complete(b *testing.B) {
	graph := generateLargeResourceGraph(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schema := &models.FormSchema{
			Properties: generateLargeSchema(50),
		}

		err := EnrichSchemaFromResources(schema, graph)
		if err != nil {
			b.Fatal(err)
		}
	}
}
