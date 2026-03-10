// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"testing"
)

// buildNestedMap creates a deeply nested map for benchmarking
func buildNestedMap(depth int) map[string]interface{} {
	result := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app": "test",
			},
			"annotations": map[string]interface{}{
				"knodex.io/catalog": "true",
			},
		},
		"spec": map[string]interface{}{
			"replicas": int64(3),
			"enabled":  true,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "test",
					},
				},
			},
		},
	}

	// Add deeper nesting
	current := result["spec"].(map[string]interface{})
	for i := 0; i < depth; i++ {
		nested := map[string]interface{}{
			"value": "nested",
			"level": int64(i),
		}
		current["nested"] = nested
		current = nested
	}

	return result
}

// BenchmarkGetString_ShallowPath benchmarks string access at depth 1
func BenchmarkGetString_ShallowPath(b *testing.B) {
	obj := buildNestedMap(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetString(obj, "apiVersion")
	}
}

// BenchmarkGetString_NestedPath benchmarks string access at depth 3
func BenchmarkGetString_NestedPath(b *testing.B) {
	obj := buildNestedMap(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetString(obj, "spec", "template", "metadata", "labels", "app")
	}
}

// BenchmarkGetStringOrDefault benchmarks default-returning accessor
func BenchmarkGetStringOrDefault(b *testing.B) {
	obj := buildNestedMap(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetStringOrDefault(obj, "fallback", "spec", "nonexistent", "key")
	}
}

// BenchmarkGetMap benchmarks map access at nested path
func BenchmarkGetMap(b *testing.B) {
	obj := buildNestedMap(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetMap(obj, "metadata", "labels")
	}
}

// BenchmarkGetSlice benchmarks slice access
func BenchmarkGetSlice(b *testing.B) {
	obj := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{"name": "r1"},
			map[string]interface{}{"name": "r2"},
			map[string]interface{}{"name": "r3"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetSlice(obj, "resources")
	}
}

// BenchmarkGetInt64 benchmarks integer access
func BenchmarkGetInt64(b *testing.B) {
	obj := buildNestedMap(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetInt64(obj, "spec", "replicas")
	}
}

// BenchmarkGetBool benchmarks boolean access
func BenchmarkGetBool(b *testing.B) {
	obj := buildNestedMap(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetBool(obj, "spec", "enabled")
	}
}

// BenchmarkGetString_DeeplyNested benchmarks deep nesting traversal (10 levels)
func BenchmarkGetString_DeeplyNested(b *testing.B) {
	obj := buildNestedMap(10)

	// Build path to deepest level
	path := []string{"spec"}
	for i := 0; i < 10; i++ {
		path = append(path, "nested")
	}
	path = append(path, "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetString(obj, path...)
	}
}

// BenchmarkAccessorMixed benchmarks a realistic mix of accessor operations
func BenchmarkAccessorMixed(b *testing.B) {
	obj := buildNestedMap(3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetString(obj, "apiVersion")
		_, _ = GetString(obj, "kind")
		_, _ = GetString(obj, "metadata", "name")
		_, _ = GetMap(obj, "metadata", "labels")
		_ = GetStringOrDefault(obj, "", "metadata", "annotations", "knodex.io/catalog")
		_, _ = GetInt64(obj, "spec", "replicas")
		_, _ = GetBool(obj, "spec", "enabled")
	}
}
