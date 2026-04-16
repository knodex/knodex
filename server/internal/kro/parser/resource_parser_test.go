// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"testing"
)

func TestResourceParser_ParseRGDResources(t *testing.T) {
	parser := NewResourceParser()

	tests := []struct {
		name         string
		rgdName      string
		rgdNamespace string
		spec         map[string]interface{}
		wantCount    int
		wantExtRefs  int
		wantConds    int
	}{
		{
			name:         "empty resources",
			rgdName:      "test-rgd",
			rgdNamespace: "default",
			spec:         map[string]interface{}{},
			wantCount:    0,
			wantExtRefs:  0,
			wantConds:    0,
		},
		{
			name:         "single template resource",
			rgdName:      "simple-app",
			rgdNamespace: "default",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"template": map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
						},
					},
				},
			},
			wantCount:   1,
			wantExtRefs: 0,
			wantConds:   0,
		},
		{
			name:         "externalRef with schema spec",
			rgdName:      "app-with-configmap",
			rgdNamespace: "default",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"name":       "${schema.spec.sharedConfigMapName}",
							"namespace":  "${schema.spec.namespace}",
						},
					},
				},
			},
			wantCount:   1,
			wantExtRefs: 1,
			wantConds:   0,
		},
		{
			name:         "conditional resource",
			rgdName:      "app-with-ingress",
			rgdNamespace: "default",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"template": map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
						},
					},
					map[string]interface{}{
						"includeWhen": "schema.spec.ingress.enabled == true",
						"template": map[string]interface{}{
							"apiVersion": "networking.k8s.io/v1",
							"kind":       "Ingress",
						},
					},
				},
			},
			wantCount:   2,
			wantExtRefs: 0,
			wantConds:   1,
		},
		{
			name:         "mixed resources",
			rgdName:      "fullstack-app",
			rgdNamespace: "platform-system",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"name":       "${schema.spec.configMapName}",
						},
					},
					map[string]interface{}{
						"template": map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
						},
					},
					map[string]interface{}{
						"template": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
						},
					},
					map[string]interface{}{
						"includeWhen": "schema.spec.ingress.enabled",
						"template": map[string]interface{}{
							"apiVersion": "networking.k8s.io/v1",
							"kind":       "Ingress",
						},
					},
				},
			},
			wantCount:   4,
			wantExtRefs: 1,
			wantConds:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := parser.ParseRGDResources(tt.rgdName, tt.rgdNamespace, tt.spec)
			if err != nil {
				t.Fatalf("ParseRGDResources() error = %v", err)
			}

			if len(graph.Resources) != tt.wantCount {
				t.Errorf("got %d resources, want %d", len(graph.Resources), tt.wantCount)
			}

			if len(graph.GetExternalRefs()) != tt.wantExtRefs {
				t.Errorf("got %d external refs, want %d", len(graph.GetExternalRefs()), tt.wantExtRefs)
			}

			if len(graph.GetConditionalResources()) != tt.wantConds {
				t.Errorf("got %d conditional resources, want %d", len(graph.GetConditionalResources()), tt.wantConds)
			}
		})
	}
}

func TestResourceParser_ParseExternalRef(t *testing.T) {
	parser := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"name":       "${schema.spec.sharedConfigMapName}",
					"namespace":  "platform-system",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	res := graph.Resources[0]
	if res.IsTemplate {
		t.Error("expected externalRef, got template")
	}

	if res.ExternalRef == nil {
		t.Fatal("externalRef is nil")
	}

	if res.ExternalRef.APIVersion != "v1" {
		t.Errorf("apiVersion = %q, want %q", res.ExternalRef.APIVersion, "v1")
	}

	if res.ExternalRef.Kind != "ConfigMap" {
		t.Errorf("kind = %q, want %q", res.ExternalRef.Kind, "ConfigMap")
	}

	if !res.ExternalRef.UsesSchemaSpec {
		t.Error("expected UsesSchemaSpec to be true")
	}

	if res.ExternalRef.SchemaField != "spec.sharedConfigMapName" {
		t.Errorf("schemaField = %q, want %q", res.ExternalRef.SchemaField, "spec.sharedConfigMapName")
	}
}

func TestResourceParser_ParseCondition(t *testing.T) {
	parser := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"includeWhen": "schema.spec.ingress.enabled == true && schema.spec.tls.enabled",
				"template": map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	res := graph.Resources[0]
	if res.IncludeWhen == nil {
		t.Fatal("includeWhen is nil")
	}

	if res.IncludeWhen.Expression != "schema.spec.ingress.enabled == true && schema.spec.tls.enabled" {
		t.Errorf("expression = %q", res.IncludeWhen.Expression)
	}

	if len(res.IncludeWhen.SchemaFields) != 2 {
		t.Errorf("expected 2 schema fields, got %d: %v", len(res.IncludeWhen.SchemaFields), res.IncludeWhen.SchemaFields)
	}

	// Check that both fields are present (order may vary)
	fieldSet := make(map[string]bool)
	for _, f := range res.IncludeWhen.SchemaFields {
		fieldSet[f] = true
	}

	expectedFields := []string{"spec.ingress.enabled", "spec.tls.enabled"}
	for _, expected := range expectedFields {
		if !fieldSet[expected] {
			t.Errorf("expected field %q not found in %v", expected, res.IncludeWhen.SchemaFields)
		}
	}
}

func TestResourceGraph_GetResourceByID(t *testing.T) {
	graph := &ResourceGraph{
		Resources: []ResourceDefinition{
			{ID: "0-Deployment", Kind: "Deployment"},
			{ID: "1-Service", Kind: "Service"},
		},
	}

	res := graph.GetResourceByID("0-Deployment")
	if res == nil {
		t.Fatal("expected to find resource")
	}

	if res.Kind != "Deployment" {
		t.Errorf("kind = %q, want %q", res.Kind, "Deployment")
	}

	res = graph.GetResourceByID("nonexistent")
	if res != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

// E2E Test Scenarios for RGD Resource Parsing

func TestResourceParser_IncludeWhenArray(t *testing.T) {
	parser := NewResourceParser()

	// Test KRO spec format: includeWhen as array of strings
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"includeWhen": []interface{}{
					"${schema.spec.ingress.enabled == true}",
				},
				"template": map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	res := graph.Resources[0]
	if res.IncludeWhen == nil {
		t.Fatal("includeWhen is nil")
	}

	expected := "${schema.spec.ingress.enabled == true}"
	if res.IncludeWhen.Expression != expected {
		t.Errorf("expression = %q, want %q", res.IncludeWhen.Expression, expected)
	}
}

func TestResourceParser_IncludeWhenMultipleConditions(t *testing.T) {
	parser := NewResourceParser()

	// Test multiple conditions in array - should be joined with &&
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"includeWhen": []interface{}{
					"${schema.spec.ingress.enabled}",
					"${schema.spec.tls.enabled}",
				},
				"template": map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	res := graph.Resources[0]
	if res.IncludeWhen == nil {
		t.Fatal("includeWhen is nil")
	}

	// Should be combined with &&
	expected := "${schema.spec.ingress.enabled} && ${schema.spec.tls.enabled}"
	if res.IncludeWhen.Expression != expected {
		t.Errorf("expression = %q, want %q", res.IncludeWhen.Expression, expected)
	}
}

func TestResourceParser_ExternalRefWithMetadata(t *testing.T) {
	parser := NewResourceParser()

	// Test KRO spec format: externalRef with nested metadata
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.configMapName}",
						"namespace": "${schema.metadata.namespace}",
					},
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	res := graph.Resources[0]
	if res.ExternalRef == nil {
		t.Fatal("externalRef is nil")
	}

	if res.ExternalRef.NameExpr != "${schema.spec.configMapName}" {
		t.Errorf("nameExpr = %q, want %q", res.ExternalRef.NameExpr, "${schema.spec.configMapName}")
	}

	if res.ExternalRef.NamespaceExpr != "${schema.metadata.namespace}" {
		t.Errorf("namespaceExpr = %q, want %q", res.ExternalRef.NamespaceExpr, "${schema.metadata.namespace}")
	}

	if !res.ExternalRef.UsesSchemaSpec {
		t.Error("expected UsesSchemaSpec to be true")
	}

	if res.ExternalRef.SchemaField != "spec.configMapName" {
		t.Errorf("schemaField = %q, want %q", res.ExternalRef.SchemaField, "spec.configMapName")
	}
}

func TestResourceParser_ExternalRefLegacyFormat(t *testing.T) {
	parser := NewResourceParser()

	// Test legacy format: externalRef with direct name/namespace (fallback)
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"name":       "${schema.spec.secretName}",
					"namespace":  "platform-system",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	res := graph.Resources[0]
	if res.ExternalRef == nil {
		t.Fatal("externalRef is nil")
	}

	if res.ExternalRef.NameExpr != "${schema.spec.secretName}" {
		t.Errorf("nameExpr = %q, want %q", res.ExternalRef.NameExpr, "${schema.spec.secretName}")
	}

	if res.ExternalRef.NamespaceExpr != "platform-system" {
		t.Errorf("namespaceExpr = %q, want %q", res.ExternalRef.NamespaceExpr, "platform-system")
	}
}

func TestResourceParser_FullStackApp(t *testing.T) {
	parser := NewResourceParser()

	// Test realistic fullstack-app scenario
	spec := map[string]interface{}{
		"resources": []interface{}{
			// ExternalRef to ConfigMap
			map[string]interface{}{
				"id": "configmap",
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.configMapName}",
						"namespace": "${schema.metadata.namespace}",
					},
				},
			},
			// Deployment that depends on configmap
			map[string]interface{}{
				"id": "deployment",
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"envFrom": []interface{}{
											map[string]interface{}{
												"configMapRef": map[string]interface{}{
													"name": "${configmap.metadata.name}",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			// Service
			map[string]interface{}{
				"id": "service",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
				},
			},
			// Conditional Ingress
			map[string]interface{}{
				"id": "ingress",
				"includeWhen": []interface{}{
					"${schema.spec.ingress.enabled == true}",
				},
				"template": map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("fullstack-app", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	// Should have 4 resources
	if len(graph.Resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(graph.Resources))
	}

	// Should have 1 external ref
	extRefs := graph.GetExternalRefs()
	if len(extRefs) != 1 {
		t.Errorf("expected 1 external ref, got %d", len(extRefs))
	}
	if len(extRefs) > 0 && extRefs[0].Kind != "ConfigMap" {
		t.Errorf("expected ConfigMap external ref, got %s", extRefs[0].Kind)
	}

	// Should have 1 conditional resource
	conditionals := graph.GetConditionalResources()
	if len(conditionals) != 1 {
		t.Errorf("expected 1 conditional resource, got %d", len(conditionals))
	}
	if len(conditionals) > 0 && conditionals[0].Kind != "Ingress" {
		t.Errorf("expected Ingress conditional, got %s", conditionals[0].Kind)
	}

	// Deployment should depend on ConfigMap
	var deployment *ResourceDefinition
	for i := range graph.Resources {
		if graph.Resources[i].Kind == "Deployment" {
			deployment = &graph.Resources[i]
			break
		}
	}
	if deployment == nil {
		t.Fatal("deployment not found")
	}

	if len(deployment.DependsOn) != 1 {
		t.Errorf("expected 1 dependency for deployment, got %d", len(deployment.DependsOn))
	}

	// Should have edge from deployment to configmap
	if len(graph.Edges) < 1 {
		t.Errorf("expected at least 1 edge, got %d", len(graph.Edges))
	}
}

func TestResourceParser_PostgresCluster(t *testing.T) {
	parser := NewResourceParser()

	// Test postgres cluster scenario with multiple resources
	spec := map[string]interface{}{
		"resources": []interface{}{
			// Primary Secret for credentials
			map[string]interface{}{
				"id": "credentials",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
				},
			},
			// Primary StatefulSet
			map[string]interface{}{
				"id": "primary",
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"env": []interface{}{
											map[string]interface{}{
												"valueFrom": map[string]interface{}{
													"secretKeyRef": map[string]interface{}{
														"name": "${credentials.metadata.name}",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			// Replica StatefulSet (conditional)
			map[string]interface{}{
				"id": "replica",
				"includeWhen": []interface{}{
					"${schema.spec.replicas > 0}",
				},
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
				},
			},
			// Service
			map[string]interface{}{
				"id": "service",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("postgres-cluster", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	// Should have 4 resources
	if len(graph.Resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(graph.Resources))
	}

	// Should have 1 conditional resource (replica StatefulSet)
	conditionals := graph.GetConditionalResources()
	if len(conditionals) != 1 {
		t.Errorf("expected 1 conditional resource, got %d", len(conditionals))
	}

	// Primary should depend on credentials
	var primary *ResourceDefinition
	for i := range graph.Resources {
		if graph.Resources[i].ID == "1-StatefulSet" {
			primary = &graph.Resources[i]
			break
		}
	}
	if primary != nil && len(primary.DependsOn) < 1 {
		t.Errorf("expected primary StatefulSet to depend on credentials")
	}
}

func TestResourceParser_RecursionDepthLimit(t *testing.T) {
	parser := NewResourceParser()

	// Create deeply nested structure to test recursion limit
	// Build nested map structure 150 levels deep
	deepNested := make(map[string]interface{})
	current := deepNested
	for i := 0; i < 150; i++ {
		next := make(map[string]interface{})
		current["nested"] = next
		current = next
	}
	// Add a reference at the deepest level (should not be found due to depth limit)
	current["value"] = "${someresource.status.value}"

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "someresource",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec":       deepNested,
				},
			},
		},
	}

	// This should not panic or cause stack overflow
	graph, err := parser.ParseRGDResources("deep-nested", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	// Should still return 2 resources
	if len(graph.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(graph.Resources))
	}
}

func TestResourceParser_EmptyIncludeWhen(t *testing.T) {
	parser := NewResourceParser()

	// Test empty includeWhen array - should be treated as unconditional
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"includeWhen": []interface{}{},
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	// Empty includeWhen should result in nil IncludeWhen
	if graph.Resources[0].IncludeWhen != nil {
		t.Error("expected nil IncludeWhen for empty array")
	}
}

func TestResourceParser_NoResources(t *testing.T) {
	parser := NewResourceParser()

	// Test RGD with no resources array
	spec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "EmptyApp",
		},
	}

	graph, err := parser.ParseRGDResources("empty-app", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(graph.Resources))
	}

	if graph.RGDName != "empty-app" {
		t.Errorf("rgdName = %q, want %q", graph.RGDName, "empty-app")
	}
}

func TestResourceParser_MixedConditionalAndUnconditional(t *testing.T) {
	parser := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			// Unconditional Deployment
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			// Conditional Service (for internal only)
			map[string]interface{}{
				"includeWhen": []interface{}{"${schema.spec.internal}"},
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
				},
			},
			// Conditional Ingress (for external)
			map[string]interface{}{
				"includeWhen": []interface{}{"${schema.spec.external}"},
				"template": map[string]interface{}{
					"apiVersion": "networking.k8s.io/v1",
					"kind":       "Ingress",
				},
			},
			// Unconditional ConfigMap
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
		},
	}

	graph, err := parser.ParseRGDResources("mixed-app", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(graph.Resources))
	}

	conditionals := graph.GetConditionalResources()
	if len(conditionals) != 2 {
		t.Errorf("expected 2 conditional resources, got %d", len(conditionals))
	}

	// Check that correct resources are conditional
	condKinds := make(map[string]bool)
	for _, c := range conditionals {
		condKinds[c.Kind] = true
	}

	if !condKinds["Service"] {
		t.Error("expected Service to be conditional")
	}
	if !condKinds["Ingress"] {
		t.Error("expected Ingress to be conditional")
	}
}

// TestExtractSchemaFieldRefs tests extraction of ${schema.spec.*} references from resource maps
func TestExtractSchemaFieldRefs(t *testing.T) {
	tests := []struct {
		name       string
		data       interface{}
		wantFields []string
	}{
		{
			name:       "nil data",
			data:       nil,
			wantFields: nil,
		},
		{
			name: "template with schema.spec references",
			data: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "${schema.spec.name}",
					"namespace": "${schema.spec.namespace}",
				},
				"data": map[string]interface{}{
					"key": "static-value",
				},
			},
			wantFields: []string{"spec.name", "spec.namespace"},
		},
		{
			name: "externalRef with metadata schema ref",
			data: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name": "${schema.spec.configMapName}",
				},
			},
			wantFields: []string{"spec.configMapName"},
		},
		{
			name: "no schema references",
			data: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Service",
				"metadata": map[string]interface{}{
					"name": "static-name",
				},
			},
			wantFields: nil,
		},
		{
			name: "nested array with schema refs",
			data: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "${schema.spec.appName}",
							"image": "${schema.spec.image}",
						},
					},
				},
			},
			wantFields: []string{"spec.appName", "spec.image"},
		},
		{
			name: "duplicate references deduplicated",
			data: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "${schema.spec.name}",
					"labels": map[string]interface{}{
						"app": "${schema.spec.name}",
					},
				},
			},
			wantFields: []string{"spec.name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSchemaFieldRefs(tt.data)

			if len(got) != len(tt.wantFields) {
				t.Errorf("extractSchemaFieldRefs() returned %d fields, want %d\ngot: %v\nwant: %v",
					len(got), len(tt.wantFields), got, tt.wantFields)
				return
			}

			// Build set for comparison (order doesn't matter)
			gotSet := make(map[string]bool)
			for _, f := range got {
				gotSet[f] = true
			}
			for _, want := range tt.wantFields {
				if !gotSet[want] {
					t.Errorf("expected field %q not found in result %v", want, got)
				}
			}
		})
	}
}

// TestResourceParser_ExternalRefNamespaceSchemaField tests that NamespaceSchemaField is populated
// when the namespace expression uses ${schema.spec.*} pattern
func TestResourceParser_ExternalRefNamespaceSchemaField(t *testing.T) {
	parser := NewResourceParser()

	tests := []struct {
		name                     string
		spec                     map[string]interface{}
		wantNamespaceSchemaField string
		wantSchemaField          string
	}{
		{
			name: "metadata namespace with schema.spec pattern",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "${schema.spec.externalRef.permissionResults.name}",
								"namespace": "${schema.spec.externalRef.permissionResults.namespace}",
							},
						},
					},
				},
			},
			wantSchemaField:          "spec.externalRef.permissionResults.name",
			wantNamespaceSchemaField: "spec.externalRef.permissionResults.namespace",
		},
		{
			name: "metadata namespace with schema.metadata.namespace (not schema.spec)",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]interface{}{
								"name":      "${schema.spec.configMapName}",
								"namespace": "${schema.metadata.namespace}",
							},
						},
					},
				},
			},
			wantSchemaField:          "spec.configMapName",
			wantNamespaceSchemaField: "", // not a schema.spec.* pattern
		},
		{
			name: "legacy format namespace with schema.spec pattern",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"name":       "${schema.spec.externalRef.db.name}",
							"namespace":  "${schema.spec.externalRef.db.namespace}",
						},
					},
				},
			},
			wantSchemaField:          "spec.externalRef.db.name",
			wantNamespaceSchemaField: "spec.externalRef.db.namespace",
		},
		{
			name: "legacy format with static namespace",
			spec: map[string]interface{}{
				"resources": []interface{}{
					map[string]interface{}{
						"externalRef": map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Secret",
							"name":       "${schema.spec.secretName}",
							"namespace":  "platform-system",
						},
					},
				},
			},
			wantSchemaField:          "spec.secretName",
			wantNamespaceSchemaField: "", // static namespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			graph, err := parser.ParseRGDResources("test", "default", tt.spec)
			if err != nil {
				t.Fatalf("ParseRGDResources() error = %v", err)
			}

			if len(graph.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
			}

			res := graph.Resources[0]
			if res.ExternalRef == nil {
				t.Fatal("externalRef is nil")
			}

			if res.ExternalRef.SchemaField != tt.wantSchemaField {
				t.Errorf("SchemaField = %q, want %q", res.ExternalRef.SchemaField, tt.wantSchemaField)
			}

			if res.ExternalRef.NamespaceSchemaField != tt.wantNamespaceSchemaField {
				t.Errorf("NamespaceSchemaField = %q, want %q", res.ExternalRef.NamespaceSchemaField, tt.wantNamespaceSchemaField)
			}
		})
	}
}

// TestResourceParser_SchemaFieldsPopulated tests that SchemaFields is populated during parsing
func TestResourceParser_SchemaFieldsPopulated(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			// Template with schema.spec references
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": "${schema.spec.name}",
					},
					"data": map[string]interface{}{
						"env": "${schema.spec.environment}",
					},
				},
			},
			// ExternalRef with schema.spec reference
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name": "${schema.spec.secretName}",
					},
				},
			},
			// Template without schema.spec references
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]interface{}{
						"name": "static-service",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(graph.Resources))
	}

	// First resource (ConfigMap template) should have schema fields
	cm := graph.Resources[0]
	if len(cm.SchemaFields) != 2 {
		t.Errorf("ConfigMap: expected 2 schema fields, got %d: %v", len(cm.SchemaFields), cm.SchemaFields)
	}
	cmFields := make(map[string]bool)
	for _, f := range cm.SchemaFields {
		cmFields[f] = true
	}
	if !cmFields["spec.name"] {
		t.Error("ConfigMap: expected 'spec.name' in schema fields")
	}
	if !cmFields["spec.environment"] {
		t.Error("ConfigMap: expected 'spec.environment' in schema fields")
	}

	// Second resource (Secret externalRef) should have schema fields
	secret := graph.Resources[1]
	if len(secret.SchemaFields) != 1 {
		t.Errorf("Secret: expected 1 schema field, got %d: %v", len(secret.SchemaFields), secret.SchemaFields)
	}
	if len(secret.SchemaFields) > 0 && secret.SchemaFields[0] != "spec.secretName" {
		t.Errorf("Secret: expected 'spec.secretName', got %q", secret.SchemaFields[0])
	}

	// Third resource (Service template) should have no schema fields
	svc := graph.Resources[2]
	if len(svc.SchemaFields) != 0 {
		t.Errorf("Service: expected 0 schema fields, got %d: %v", len(svc.SchemaFields), svc.SchemaFields)
	}
}

// TestResourceParser_SkippedResourcesDependencyResolution tests that dependency resolution
// uses the correct original resource map when some resources are skipped (e.g., unknown types).
// This is a regression test for an index mismatch bug where graph.Resources[i] could be
// resolved against the wrong resources[i] when entries were skipped in the first pass.
func TestResourceParser_SkippedResourcesDependencyResolution(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			// First resource: unknown type (no template or externalRef) — will be SKIPPED
			map[string]interface{}{
				"id":    "unknown",
				"bogus": "this has no template or externalRef",
			},
			// Second resource: template Secret (credentials)
			map[string]interface{}{
				"id": "credentials",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
				},
			},
			// Third resource: template Deployment that depends on credentials
			map[string]interface{}{
				"id": "deployment",
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"env": []interface{}{
											map[string]interface{}{
												"valueFrom": map[string]interface{}{
													"secretKeyRef": map[string]interface{}{
														"name": "${credentials.metadata.name}",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("skip-test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	// Should have 2 resources (unknown type was skipped)
	if len(graph.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(graph.Resources))
	}

	// Deployment should depend on Secret (credentials)
	var deployment *ResourceDefinition
	for i := range graph.Resources {
		if graph.Resources[i].Kind == "Deployment" {
			deployment = &graph.Resources[i]
			break
		}
	}
	if deployment == nil {
		t.Fatal("deployment not found")
	}

	if len(deployment.DependsOn) != 1 {
		t.Errorf("expected 1 dependency for deployment, got %d: %v", len(deployment.DependsOn), deployment.DependsOn)
	}
}

// TestTraverseForReferences_Fallback tests the manual traversal fallback
// that executes when KRO's ParseSchemalessResource fails.
func TestTraverseForReferences_Fallback(t *testing.T) {
	p := NewResourceParser()

	idByInternalID := map[string]string{
		"configmap": "0-ConfigMap",
		"secret":    "1-Secret",
	}

	tests := []struct {
		name     string
		data     interface{}
		wantRefs map[string]bool
	}{
		{
			name: "finds references in nested maps",
			data: map[string]interface{}{
				"spec": map[string]interface{}{
					"envFrom": map[string]interface{}{
						"configMapRef": map[string]interface{}{
							"name": "${configmap.metadata.name}",
						},
					},
				},
			},
			wantRefs: map[string]bool{"0-ConfigMap": true},
		},
		{
			name: "finds references in arrays",
			data: []interface{}{
				"${secret.data.password}",
				"static-value",
				"${configmap.data.key}",
			},
			wantRefs: map[string]bool{"1-Secret": true, "0-ConfigMap": true},
		},
		{
			name:     "no references in static strings",
			data:     "just-a-string",
			wantRefs: map[string]bool{},
		},
		{
			name:     "nil data",
			data:     nil,
			wantRefs: map[string]bool{},
		},
		{
			name:     "integer data",
			data:     42,
			wantRefs: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := make(map[string]bool)
			p.traverseForReferences(tt.data, idByInternalID, refs, 0)

			if len(refs) != len(tt.wantRefs) {
				t.Errorf("got %d refs, want %d\ngot: %v\nwant: %v",
					len(refs), len(tt.wantRefs), refs, tt.wantRefs)
				return
			}
			for wantRef := range tt.wantRefs {
				if !refs[wantRef] {
					t.Errorf("expected ref %q not found in %v", wantRef, refs)
				}
			}
		})
	}
}

// TestTraverseForReferences_DepthLimit tests that recursion depth is bounded
func TestTraverseForReferences_DepthLimit(t *testing.T) {
	p := NewResourceParser()

	// Build structure deeper than maxRecursionDepth
	deepNested := make(map[string]interface{})
	current := deepNested
	for i := 0; i < maxRecursionDepth+10; i++ {
		next := make(map[string]interface{})
		current["nested"] = next
		current = next
	}
	current["value"] = "${configmap.metadata.name}"

	idByInternalID := map[string]string{"configmap": "0-ConfigMap"}
	refs := make(map[string]bool)

	// Should not panic and should not find the deeply nested reference
	p.traverseForReferences(deepNested, idByInternalID, refs, 0)

	if len(refs) != 0 {
		t.Errorf("expected 0 refs due to depth limit, got %d: %v", len(refs), refs)
	}
}

// TestCollectSchemaRefs_Fallback tests the manual schema.spec.* collection fallback
func TestCollectSchemaRefs_Fallback(t *testing.T) {
	tests := []struct {
		name       string
		data       interface{}
		wantFields []string
	}{
		{
			name: "finds schema refs in nested maps",
			data: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "${schema.spec.name}",
				},
				"data": map[string]interface{}{
					"env": "${schema.spec.environment}",
				},
			},
			wantFields: []string{"spec.environment", "spec.name"},
		},
		{
			name: "finds schema refs in arrays",
			data: []interface{}{
				"${schema.spec.first}",
				"static",
				"${schema.spec.second}",
			},
			wantFields: []string{"spec.first", "spec.second"},
		},
		{
			name:       "no schema refs",
			data:       map[string]interface{}{"key": "static-value"},
			wantFields: nil,
		},
		{
			name: "deduplicates refs",
			data: map[string]interface{}{
				"a": "${schema.spec.name}",
				"b": "${schema.spec.name}",
			},
			wantFields: []string{"spec.name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(map[string]bool)
			collectSchemaRefs(tt.data, seen, 0)

			got := mapToSlice(seen)

			if len(got) != len(tt.wantFields) {
				t.Errorf("got %d fields, want %d\ngot: %v\nwant: %v",
					len(got), len(tt.wantFields), got, tt.wantFields)
				return
			}
			for i, want := range tt.wantFields {
				if got[i] != want {
					t.Errorf("field[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// TestExtractExpressionsFromValue tests the helper that wraps KRO's parser
// for single-string expression extraction.
//
// NOTE: In KRO v0.9.0, string templates like "prefix-${a}-${b}" are compiled
// into a single CEL concatenation expression at parse time, e.g.,
// '"prefix-" + (a) + "-" + (b)'. This means extractExpressionsFromValue
// returns one expression for templates rather than individual expressions.
// Downstream consumers use extractBareSchemaFields (substring matching)
// which handles both forms correctly.
func TestExtractExpressionsFromValue(t *testing.T) {
	t.Run("simple expression", func(t *testing.T) {
		got := extractExpressionsFromValue("${schema.spec.name}")
		if len(got) != 1 || got[0] != "schema.spec.name" {
			t.Errorf("got %v, want [schema.spec.name]", got)
		}
	})

	t.Run("no expression", func(t *testing.T) {
		got := extractExpressionsFromValue("static-value")
		if got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})

	t.Run("expression with string literal containing brace", func(t *testing.T) {
		got := extractExpressionsFromValue(`${schema.spec.name + "}"}`)
		if len(got) != 1 || got[0] != `schema.spec.name + "}"` {
			t.Errorf("got %v, want [schema.spec.name + \"}\"]", got)
		}
	})

	t.Run("embedded expression", func(t *testing.T) {
		// In KRO v0.9.0, templates compile into concatenation expressions.
		// The returned expression contains the schema.spec reference.
		got := extractExpressionsFromValue("prefix-${schema.spec.name}-suffix")
		if len(got) != 1 {
			t.Fatalf("got %d expressions, want 1; got: %v", len(got), got)
		}
		// Verify the expression contains the schema reference
		// (may be wrapped in concatenation: "prefix-" + (schema.spec.name) + "-suffix")
		fields := extractBareSchemaFields(got[0])
		if len(fields) != 1 || fields[0] != "spec.name" {
			t.Errorf("schema fields from expression %q = %v, want [spec.name]", got[0], fields)
		}
	})

	t.Run("multiple expressions", func(t *testing.T) {
		// In KRO v0.9.0, "${a}-${b}" compiles into a single concatenation expression.
		got := extractExpressionsFromValue("${schema.spec.a}-${schema.spec.b}")
		if len(got) != 1 {
			t.Fatalf("got %d expressions, want 1; got: %v", len(got), got)
		}
		// Verify both schema references are extractable from the concatenated expression
		fields := extractBareSchemaFields(got[0])
		fieldSet := make(map[string]bool)
		for _, f := range fields {
			fieldSet[f] = true
		}
		if !fieldSet["spec.a"] || !fieldSet["spec.b"] {
			t.Errorf("schema fields from expression %q = %v, want [spec.a, spec.b]", got[0], fields)
		}
	})
}

// TestExtractBareSchemaFields tests extraction of schema.spec.* from bare strings
func TestExtractBareSchemaFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple field reference",
			input: "schema.spec.name",
			want:  []string{"spec.name"},
		},
		{
			name:  "field in condition",
			input: "schema.spec.ingress.enabled == true",
			want:  []string{"spec.ingress.enabled"},
		},
		{
			name:  "multiple fields",
			input: "schema.spec.a == true && schema.spec.b",
			want:  []string{"spec.a", "spec.b"},
		},
		{
			name:  "no schema reference",
			input: "some.other.field == true",
			want:  nil,
		},
		{
			name:  "nested field path",
			input: "schema.spec.externalRef.db.name",
			want:  []string{"spec.externalRef.db.name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBareSchemaFields(tt.input)

			if len(got) != len(tt.want) {
				t.Errorf("extractBareSchemaFields() returned %d fields, want %d\ngot: %v\nwant: %v",
					len(got), len(tt.want), got, tt.want)
				return
			}

			gotSet := make(map[string]bool)
			for _, f := range got {
				gotSet[f] = true
			}
			for _, w := range tt.want {
				if !gotSet[w] {
					t.Errorf("expected field %q not found in %v", w, got)
				}
			}
		})
	}
}

// TestParseCondition_WithWrappedExpressions tests condition parsing with ${}-wrapped expressions
func TestParseCondition_WithWrappedExpressions(t *testing.T) {
	condition := parseCondition("${schema.spec.ingress.enabled == true}")

	if condition.Expression != "${schema.spec.ingress.enabled == true}" {
		t.Errorf("expression = %q", condition.Expression)
	}

	if len(condition.SchemaFields) != 1 {
		t.Fatalf("expected 1 schema field, got %d: %v", len(condition.SchemaFields), condition.SchemaFields)
	}

	if condition.SchemaFields[0] != "spec.ingress.enabled" {
		t.Errorf("field = %q, want %q", condition.SchemaFields[0], "spec.ingress.enabled")
	}
}

// TestResourceParser_EdgeCase_NestedBracesInTemplate tests that nested braces
// in template values are handled correctly (regression test for regex-based parsing)
func TestResourceParser_EdgeCase_NestedBracesInTemplate(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"data": map[string]interface{}{
						"config": `${schema.spec.prefix + "}"}`,
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("edge-case", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	if len(graph.Resources[0].SchemaFields) != 1 {
		t.Errorf("expected 1 schema field, got %d: %v",
			len(graph.Resources[0].SchemaFields), graph.Resources[0].SchemaFields)
	}
}

// TestResourceParser_EdgeCase_StringConcatenation tests expression with string concatenation
func TestResourceParser_EdgeCase_StringConcatenation(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name": `${schema.spec.prefix + "-suffix"}`,
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("concat-test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	fields := graph.Resources[0].SchemaFields
	if len(fields) != 1 {
		t.Errorf("expected 1 schema field, got %d: %v", len(fields), fields)
	}
	if len(fields) > 0 && fields[0] != "spec.prefix" {
		t.Errorf("expected 'spec.prefix', got %q", fields[0])
	}
}

// === SecretRef extraction tests ===

func TestResourceParser_SecretRef_Dynamic(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.externalRef.asoCredentialSecret.name}",
						"namespace": "${schema.spec.externalRef.asoCredentialSecret.namespace}",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secret ref, got %d", len(graph.SecretRefs))
	}

	sr := graph.SecretRefs[0]
	if sr.Type != "dynamic" {
		t.Errorf("type = %q, want %q", sr.Type, "dynamic")
	}
	if sr.NameExpr != "${schema.spec.externalRef.asoCredentialSecret.name}" {
		t.Errorf("nameExpr = %q", sr.NameExpr)
	}
	if sr.NamespaceExpr != "${schema.spec.externalRef.asoCredentialSecret.namespace}" {
		t.Errorf("namespaceExpr = %q", sr.NamespaceExpr)
	}
	if sr.ID != "0-Secret" {
		t.Errorf("id = %q, want %q", sr.ID, "0-Secret")
	}
}

func TestResourceParser_SecretRef_Fixed(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "my-static-secret",
						"namespace": "platform-system",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secret ref, got %d", len(graph.SecretRefs))
	}

	sr := graph.SecretRefs[0]
	if sr.Type != "fixed" {
		t.Errorf("type = %q, want %q", sr.Type, "fixed")
	}
	if sr.Name != "my-static-secret" {
		t.Errorf("name = %q, want %q", sr.Name, "my-static-secret")
	}
	if sr.Namespace != "platform-system" {
		t.Errorf("namespace = %q, want %q", sr.Namespace, "platform-system")
	}
}

func TestResourceParser_SecretRef_MixedExternalRefs(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			// Secret externalRef (should be in SecretRefs)
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.secretName}",
						"namespace": "${schema.spec.secretNamespace}",
					},
				},
			},
			// ConfigMap externalRef (should NOT be in SecretRefs)
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.configMapName}",
						"namespace": "${schema.spec.configMapNamespace}",
					},
				},
			},
			// Template resource (should NOT be in SecretRefs)
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secret ref, got %d: %+v", len(graph.SecretRefs), graph.SecretRefs)
	}

	if graph.SecretRefs[0].ID != "0-Secret" {
		t.Errorf("secret ref ID = %q, want %q", graph.SecretRefs[0].ID, "0-Secret")
	}
}

func TestResourceParser_SecretRef_NoExternalRefs(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 0 {
		t.Errorf("expected empty SecretRefs, got %+v", graph.SecretRefs)
	}
}

func TestResourceParser_SecretRef_MultipleSecrets(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.dbSecret.name}",
						"namespace": "${schema.spec.dbSecret.namespace}",
					},
				},
			},
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "static-tls-secret",
						"namespace": "cert-manager",
					},
				},
			},
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.apiKeySecret.name}",
						"namespace": "${schema.spec.apiKeySecret.namespace}",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 3 {
		t.Fatalf("expected 3 secret refs, got %d", len(graph.SecretRefs))
	}

	// First is dynamic
	if graph.SecretRefs[0].Type != "dynamic" {
		t.Errorf("ref[0] type = %q, want dynamic", graph.SecretRefs[0].Type)
	}
	// Second is fixed
	if graph.SecretRefs[1].Type != "fixed" {
		t.Errorf("ref[1] type = %q, want fixed", graph.SecretRefs[1].Type)
	}
	if graph.SecretRefs[1].Name != "static-tls-secret" {
		t.Errorf("ref[1] name = %q, want static-tls-secret", graph.SecretRefs[1].Name)
	}
	// Third is dynamic
	if graph.SecretRefs[2].Type != "dynamic" {
		t.Errorf("ref[2] type = %q, want dynamic", graph.SecretRefs[2].Type)
	}
}

func TestResourceParser_SecretRef_DynamicNamespaceFixedName(t *testing.T) {
	p := NewResourceParser()

	// Edge case: static name but dynamic namespace — should be typed "dynamic"
	// because the frontend still needs schema.spec.* resolution for the namespace.
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "well-known-secret",              // fixed literal
						"namespace": "${schema.spec.targetNamespace}", // dynamic
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secret ref, got %d", len(graph.SecretRefs))
	}

	sr := graph.SecretRefs[0]
	// Must be "dynamic" because namespace requires schema.spec resolution
	if sr.Type != "dynamic" {
		t.Errorf("type = %q, want %q (namespace is dynamic)", sr.Type, "dynamic")
	}
	if sr.NameExpr != "well-known-secret" {
		t.Errorf("nameExpr = %q, want %q", sr.NameExpr, "well-known-secret")
	}
	if sr.NamespaceExpr != "${schema.spec.targetNamespace}" {
		t.Errorf("namespaceExpr = %q, want ${schema.spec.targetNamespace}", sr.NamespaceExpr)
	}
}

func TestResourceParser_SecretRef_MetadataNamespaceDynamic(t *testing.T) {
	p := NewResourceParser()

	// Edge case: literal name + ${schema.metadata.namespace} namespace.
	// NamespaceSchemaField is empty (not a schema.spec.* pattern), but the namespace
	// still contains a CEL expression — must be classified as "dynamic", not "fixed".
	// Previously the old UsesSchemaSpec check would produce type="fixed" with a CEL
	// expression in the Namespace field, violating the contract that fixed refs have literals.
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "well-known-secret",            // fixed literal
						"namespace": "${schema.metadata.namespace}", // CEL, not schema.spec.*
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secret ref, got %d", len(graph.SecretRefs))
	}

	sr := graph.SecretRefs[0]
	// Must be "dynamic" because namespace contains a CEL expression.
	// A "fixed" result here would leak "${schema.metadata.namespace}" into the Namespace field.
	if sr.Type != "dynamic" {
		t.Errorf("type = %q, want %q", sr.Type, "dynamic")
	}
	if sr.NameExpr != "well-known-secret" {
		t.Errorf("nameExpr = %q, want %q", sr.NameExpr, "well-known-secret")
	}
	if sr.NamespaceExpr != "${schema.metadata.namespace}" {
		t.Errorf("namespaceExpr = %q, want %q", sr.NamespaceExpr, "${schema.metadata.namespace}")
	}
	// Name/Namespace fields must be empty for dynamic refs
	if sr.Name != "" {
		t.Errorf("Name should be empty for dynamic ref, got %q", sr.Name)
	}
	if sr.Namespace != "" {
		t.Errorf("Namespace should be empty for dynamic ref, got %q", sr.Namespace)
	}
}

func TestResourceParser_SecretRef_CaseSensitiveKind(t *testing.T) {
	p := NewResourceParser()

	// "secret" (lowercase) must NOT be matched — Kubernetes kinds are PascalCase.
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "secret", // lowercase — not a valid k8s kind
					"metadata": map[string]interface{}{
						"name":      "my-secret",
						"namespace": "default",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 0 {
		t.Errorf("expected 0 secretRefs for lowercase 'secret' kind, got %d", len(graph.SecretRefs))
	}
}

// TestCollectSchemaRefs_DepthLimit tests depth limiting for schema ref collection
func TestCollectSchemaRefs_DepthLimit(t *testing.T) {
	deepNested := make(map[string]interface{})
	current := deepNested
	for i := 0; i < maxRecursionDepth+10; i++ {
		next := make(map[string]interface{})
		current["nested"] = next
		current = next
	}
	current["value"] = "${schema.spec.deep}"

	seen := make(map[string]bool)
	collectSchemaRefs(deepNested, seen, 0)

	if len(seen) != 0 {
		t.Errorf("expected 0 fields due to depth limit, got %d", len(seen))
	}
}

// TestExtractExternalRefDescription tests description extraction from schema externalRef fields
func TestExtractExternalRefDescription(t *testing.T) {
	tests := []struct {
		name               string
		schemaExternalRefs map[string]interface{}
		fieldName          string
		want               string
	}{
		{
			name: "description present",
			schemaExternalRefs: map[string]interface{}{
				"dbSecret": map[string]interface{}{
					"name":      `string | default="" description="Name of the Kubernetes Secret containing database credentials"`,
					"namespace": `string | default="" description="Namespace of the Secret"`,
				},
			},
			fieldName: "dbSecret",
			want:      "Name of the Kubernetes Secret containing database credentials",
		},
		{
			name: "description missing",
			schemaExternalRefs: map[string]interface{}{
				"dbSecret": map[string]interface{}{
					"name":      `string | default=""`,
					"namespace": `string | default=""`,
				},
			},
			fieldName: "dbSecret",
			want:      "",
		},
		{
			name:               "nil schema",
			schemaExternalRefs: nil,
			fieldName:          "dbSecret",
			want:               "",
		},
		{
			name: "empty field name",
			schemaExternalRefs: map[string]interface{}{
				"dbSecret": map[string]interface{}{
					"name": `string | default="" description="test"`,
				},
			},
			fieldName: "",
			want:      "",
		},
		{
			name: "field not found",
			schemaExternalRefs: map[string]interface{}{
				"otherSecret": map[string]interface{}{
					"name": `string | default="" description="test"`,
				},
			},
			fieldName: "dbSecret",
			want:      "",
		},
		{
			name: "name field not a string",
			schemaExternalRefs: map[string]interface{}{
				"dbSecret": map[string]interface{}{
					"name": 42,
				},
			},
			fieldName: "dbSecret",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractExternalRefDescription(tt.schemaExternalRefs, tt.fieldName)
			if got != tt.want {
				t.Errorf("extractExternalRefDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestResourceParser_SecretRef_Description tests end-to-end description extraction
func TestResourceParser_SecretRef_Description(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "v1alpha1",
			"kind":       "WebAppWithSecret",
			"spec": map[string]interface{}{
				"appName": `string | default="my-webapp" description="Application name"`,
				"externalRef": map[string]interface{}{
					"dbSecret": map[string]interface{}{
						"name":      `string | default="" description="Name of the Kubernetes Secret containing database credentials"`,
						"namespace": `string | default="" description="Namespace of the Secret"`,
					},
				},
			},
		},
		"resources": []interface{}{
			map[string]interface{}{
				"id": "dbSecret",
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.externalRef.dbSecret.name}",
						"namespace": "${schema.spec.externalRef.dbSecret.namespace}",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("webapp-with-secret", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secretRef, got %d", len(graph.SecretRefs))
	}

	ref := graph.SecretRefs[0]
	wantDesc := "Name of the Kubernetes Secret containing database credentials"
	if ref.Description != wantDesc {
		t.Errorf("Description = %q, want %q", ref.Description, wantDesc)
	}
	if ref.ExternalRefID != "dbSecret" {
		t.Errorf("ExternalRefID = %q, want %q", ref.ExternalRefID, "dbSecret")
	}
}

// TestResourceParser_SecretRef_DescriptionMissing tests that missing description doesn't break parsing
func TestResourceParser_SecretRef_DescriptionMissing(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"schema": map[string]interface{}{
			"spec": map[string]interface{}{
				"externalRef": map[string]interface{}{
					"dbSecret": map[string]interface{}{
						"name":      `string | default=""`,
						"namespace": `string | default=""`,
					},
				},
			},
		},
		"resources": []interface{}{
			map[string]interface{}{
				"id": "dbSecret",
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.externalRef.dbSecret.name}",
						"namespace": "${schema.spec.externalRef.dbSecret.namespace}",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secretRef, got %d", len(graph.SecretRefs))
	}

	if graph.SecretRefs[0].Description != "" {
		t.Errorf("Description = %q, want empty", graph.SecretRefs[0].Description)
	}
}

// TestResourceParser_SecretRef_NoSchemaSection tests description extraction with no schema section
func TestResourceParser_SecretRef_NoSchemaSection(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]interface{}{
						"name":      "my-secret",
						"namespace": "default",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.SecretRefs) != 1 {
		t.Fatalf("expected 1 secretRef, got %d", len(graph.SecretRefs))
	}

	// No schema → no description, but parsing should still work
	if graph.SecretRefs[0].Description != "" {
		t.Errorf("Description = %q, want empty", graph.SecretRefs[0].Description)
	}
}

// TestIsResourceRef tests the word-boundary-aware reference checker that replaced
// plain strings.Contains for dependency detection in KRO v0.9.0 concatenated expressions.
//
// The core invariant: `isResourceRef(expr, id)` must return true when id is a root
// CEL identifier in expr, and false when id only appears as a field accessor nested
// inside another identifier (e.g., "schema.spec.id").
func TestIsResourceRef(t *testing.T) {
	tests := []struct {
		name       string
		expr       string
		internalID string
		want       bool
	}{
		// True positives — root identifier references
		{
			name:       "simple root reference",
			expr:       "configmap.metadata.name",
			internalID: "configmap",
			want:       true,
		},
		{
			name:       "reference inside concatenation — opening paren",
			expr:       `(configmap.metadata.name) + "-" + (service.spec.clusterIP)`,
			internalID: "configmap",
			want:       true,
		},
		{
			name:       "second operand in concatenation",
			expr:       `(configmap.metadata.name) + "-" + (service.spec.clusterIP)`,
			internalID: "service",
			want:       true,
		},
		{
			name:       "reference preceded by space",
			expr:       "x == configmap.metadata.name",
			internalID: "configmap",
			want:       true,
		},
		// False positives that naive strings.Contains would produce
		{
			name:       "schema field whose name equals internalID — no match",
			expr:       "schema.spec.configmap",
			internalID: "configmap",
			want:       false,
		},
		{
			name:       "single-char ID appearing in schema field path — no match",
			expr:       "schema.spec.a",
			internalID: "a",
			want:       false,
		},
		{
			name:       "ID nested under schema.spec as intermediate field — no match",
			expr:       "schema.spec.db.name",
			internalID: "db",
			want:       false,
		},
		{
			name:       "unrelated expression with no match",
			expr:       "schema.spec.name",
			internalID: "configmap",
			want:       false,
		},
		{
			name:       "empty expression",
			expr:       "",
			internalID: "configmap",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isResourceRef(tt.expr, tt.internalID)
			if got != tt.want {
				t.Errorf("isResourceRef(%q, %q) = %v, want %v", tt.expr, tt.internalID, got, tt.want)
			}
		})
	}
}

// TestResourceParser_ConcatenatedExpressionDependency tests that dependency resolution
// correctly identifies resource references inside KRO v0.9.0 concatenated expressions.
//
// In v0.9.0, "${dbSecret.metadata.name}-${appConfig.data.key}" is compiled by KRO
// into a single CEL expression like `(dbSecret.metadata.name) + "-" + (appConfig.data.key)`.
// This means dependency detection must use word-boundary-aware matching (isResourceRef),
// not HasPrefix (which would miss the second operand) or naive Contains (which produces
// false positives on schema field names).
func TestResourceParser_ConcatenatedExpressionDependency(t *testing.T) {
	p := NewResourceParser()

	spec := map[string]interface{}{
		"resources": []interface{}{
			// Secret resource (credentials)
			map[string]interface{}{
				"id": "credentials",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Secret",
				},
			},
			// ConfigMap resource (config)
			map[string]interface{}{
				"id": "config",
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
			// Deployment with a CONCATENATED reference to both resources:
			// "${credentials.metadata.name}-${config.data.url}"
			// KRO v0.9.0 compiles this to: (credentials.metadata.name) + "-" + (config.data.url)
			// HasPrefix would only match the first operand; isResourceRef must match both.
			map[string]interface{}{
				"id": "deployment",
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec": map[string]interface{}{
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"env": []interface{}{
											map[string]interface{}{
												"name":  "SECRET_NAME",
												"value": "${credentials.metadata.name}-${config.data.url}",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("concat-dep-test", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	if len(graph.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(graph.Resources))
	}

	// Find deployment
	var deployment *ResourceDefinition
	for i := range graph.Resources {
		if graph.Resources[i].Kind == "Deployment" {
			deployment = &graph.Resources[i]
			break
		}
	}
	if deployment == nil {
		t.Fatal("deployment not found")
	}

	// Deployment must declare dependency on BOTH resources
	if len(deployment.DependsOn) != 2 {
		t.Fatalf("expected 2 dependencies (credentials + config), got %d: %v",
			len(deployment.DependsOn), deployment.DependsOn)
	}

	// Verify the exact dependency IDs, not just the count
	depSet := make(map[string]bool)
	for _, d := range deployment.DependsOn {
		depSet[d] = true
	}
	if !depSet["0-Secret"] {
		t.Errorf("missing dependency on 0-Secret (credentials); got: %v", deployment.DependsOn)
	}
	if !depSet["1-ConfigMap"] {
		t.Errorf("missing dependency on 1-ConfigMap (config); got: %v", deployment.DependsOn)
	}
}

// --- forEach collection tests (STORY-330) ---

func TestResourceParser_ForEach_SchemaSource(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "workerPods",
				"forEach": []interface{}{
					map[string]interface{}{"worker": "${schema.spec.workers}"},
				},
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("worker-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}
	if len(graph.ParseErrors) != 0 {
		t.Fatalf("unexpected parse errors: %v", graph.ParseErrors)
	}
	if len(graph.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(graph.Resources))
	}

	res := graph.Resources[0]
	if !res.IsCollection {
		t.Error("expected IsCollection = true")
	}
	if len(res.ForEach) != 1 {
		t.Fatalf("expected 1 iterator, got %d", len(res.ForEach))
	}

	it := res.ForEach[0]
	if it.Name != "worker" {
		t.Errorf("Name = %q, want %q", it.Name, "worker")
	}
	if it.Expression != "${schema.spec.workers}" {
		t.Errorf("Expression = %q, want %q", it.Expression, "${schema.spec.workers}")
	}
	if it.Source != SchemaSource {
		t.Errorf("Source = %q, want %q", it.Source, SchemaSource)
	}
	if it.SourcePath != "spec.workers" {
		t.Errorf("SourcePath = %q, want %q", it.SourcePath, "spec.workers")
	}
	if it.DimensionIndex != 0 {
		t.Errorf("DimensionIndex = %d, want 0", it.DimensionIndex)
	}

	// GetCollectionResources helper
	collections := graph.GetCollectionResources()
	if len(collections) != 1 {
		t.Errorf("GetCollectionResources() = %d, want 1", len(collections))
	}
}

func TestResourceParser_ForEach_ResourceSource(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "brokerPods",
				"forEach": []interface{}{
					map[string]interface{}{"broker": "${cluster.status.brokers}"},
				},
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("broker-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}
	if len(graph.ParseErrors) != 0 {
		t.Fatalf("unexpected parse errors: %v", graph.ParseErrors)
	}

	res := graph.Resources[0]
	if !res.IsCollection {
		t.Error("expected IsCollection = true")
	}

	it := res.ForEach[0]
	if it.Name != "broker" {
		t.Errorf("Name = %q, want %q", it.Name, "broker")
	}
	if it.Source != ResourceSource {
		t.Errorf("Source = %q, want %q", it.Source, ResourceSource)
	}
	if it.SourcePath != "status.brokers" {
		t.Errorf("SourcePath = %q, want %q", it.SourcePath, "status.brokers")
	}
}

func TestResourceParser_ForEach_CartesianProduct(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "deployments",
				"forEach": []interface{}{
					map[string]interface{}{"region": "${schema.spec.regions}"},
					map[string]interface{}{"tier": "${schema.spec.tiers}"},
				},
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("multi-dim", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}
	if len(graph.ParseErrors) != 0 {
		t.Fatalf("unexpected parse errors: %v", graph.ParseErrors)
	}

	res := graph.Resources[0]
	if !res.IsCollection {
		t.Error("expected IsCollection = true")
	}
	if len(res.ForEach) != 2 {
		t.Fatalf("expected 2 iterators (cartesian), got %d", len(res.ForEach))
	}

	// Index iterators by Name for deterministic assertions (map iteration order varies)
	byName := make(map[string]Iterator, 2)
	for _, it := range res.ForEach {
		byName[it.Name] = it
	}

	// "region" dimension
	region, ok := byName["region"]
	if !ok {
		t.Fatalf("expected iterator named %q, got names: %v", "region", func() []string {
			names := make([]string, 0, len(byName))
			for n := range byName {
				names = append(names, n)
			}
			return names
		}())
	}
	if region.Source != SchemaSource {
		t.Errorf("region: Source = %q, want %q", region.Source, SchemaSource)
	}
	if region.SourcePath != "spec.regions" {
		t.Errorf("region: SourcePath = %q, want %q", region.SourcePath, "spec.regions")
	}

	// "tier" dimension
	tier, ok := byName["tier"]
	if !ok {
		t.Fatalf("expected iterator named %q", "tier")
	}
	if tier.Source != SchemaSource {
		t.Errorf("tier: Source = %q, want %q", tier.Source, SchemaSource)
	}
	if tier.SourcePath != "spec.tiers" {
		t.Errorf("tier: SourcePath = %q, want %q", tier.SourcePath, "spec.tiers")
	}

	// DimensionIndex must be 0 and 1
	dims := make(map[int]bool)
	for _, it := range res.ForEach {
		dims[it.DimensionIndex] = true
	}
	if !dims[0] || !dims[1] {
		t.Errorf("expected DimensionIndex values {0,1}, got iterators: %+v", res.ForEach)
	}
}

func TestResourceParser_ForEach_LiteralSource(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "regionPods",
				"forEach": []interface{}{
					map[string]interface{}{"region": `["us-east", "eu-west"]`},
				},
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("literal-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	res := graph.Resources[0]
	if !res.IsCollection {
		t.Error("expected IsCollection = true")
	}
	if res.ForEach[0].Source != LiteralSource {
		t.Errorf("Source = %q, want %q", res.ForEach[0].Source, LiteralSource)
	}
	if res.ForEach[0].SourcePath != "" {
		t.Errorf("SourcePath = %q, want empty for LiteralSource", res.ForEach[0].SourcePath)
	}
}

func TestResourceParser_ForEach_ExternalRefMutuallyExclusive(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "invalid",
				"forEach": []interface{}{
					map[string]interface{}{"item": "${schema.spec.items}"},
				},
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "my-cm",
						"namespace": "default",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("invalid-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() unexpected hard error: %v", err)
	}

	// Parse error should be recorded (non-fatal)
	if len(graph.ParseErrors) == 0 {
		t.Fatal("expected parse error for forEach+externalRef combination, got none")
	}
	if graph.ParseErrors[0].Expression != "forEach" {
		t.Errorf("ParseError.Expression = %q, want %q", graph.ParseErrors[0].Expression, "forEach")
	}
}

func TestResourceParser_ReadyWhen_Collection(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "workerPods",
				"forEach": []interface{}{
					map[string]interface{}{"worker": "${schema.spec.workers}"},
				},
				"readyWhen": []interface{}{
					"each.status.phase == 'Running'",
					"each.status.containerStatuses.size() > 0",
				},
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("ready-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}

	res := graph.Resources[0]
	if !res.IsCollection {
		t.Error("expected IsCollection = true")
	}
	if len(res.ReadyWhen) != 2 {
		t.Fatalf("expected 2 readyWhen entries, got %d: %v", len(res.ReadyWhen), res.ReadyWhen)
	}
	if res.ReadyWhen[0] != "each.status.phase == 'Running'" {
		t.Errorf("ReadyWhen[0] = %q, want %q", res.ReadyWhen[0], "each.status.phase == 'Running'")
	}
}

func TestResourceParser_NonCollection_IsCollectionFalse(t *testing.T) {
	p := NewResourceParser()
	spec := map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			map[string]interface{}{
				"externalRef": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]interface{}{
						"name":      "${schema.spec.cmName}",
						"namespace": "default",
					},
				},
			},
		},
	}

	graph, err := p.ParseRGDResources("standard-rgd", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources() error = %v", err)
	}
	if len(graph.ParseErrors) != 0 {
		t.Fatalf("unexpected parse errors: %v", graph.ParseErrors)
	}

	for _, res := range graph.Resources {
		if res.IsCollection {
			t.Errorf("resource %q: IsCollection = true, want false for non-forEach resource", res.ID)
		}
		if len(res.ForEach) != 0 {
			t.Errorf("resource %q: ForEach is non-empty for non-forEach resource", res.ID)
		}
	}

	// GetCollectionResources should return empty
	if cols := graph.GetCollectionResources(); len(cols) != 0 {
		t.Errorf("GetCollectionResources() = %d, want 0", len(cols))
	}
}

func TestAnalyzeForEachSource_BareExpression(t *testing.T) {
	// No ${...} delimiters — treated as a bare CEL literal
	src, path := analyzeForEachSource("schema.spec.workers")
	if src != SchemaSource {
		t.Errorf("bare schema expr: Source = %q, want %q", src, SchemaSource)
	}
	if path != "spec.workers" {
		t.Errorf("bare schema expr: SourcePath = %q, want %q", path, "spec.workers")
	}
}

func TestAnalyzeForEachSource_EmptyExpression(t *testing.T) {
	src, path := analyzeForEachSource("")
	if src != LiteralSource {
		t.Errorf("empty expr: Source = %q, want %q", src, LiteralSource)
	}
	if path != "" {
		t.Errorf("empty expr: SourcePath = %q, want empty", path)
	}
}

func TestAnalyzeForEachSource_SchemaMetadata(t *testing.T) {
	// schema.metadata.* does NOT match schema.spec.* — falls through to ResourceSource check.
	// "schema" is excluded from ResourceSource, so result is LiteralSource.
	// This documents the known boundary: only schema.spec.* yields SchemaSource.
	src, path := analyzeForEachSource("${schema.metadata.name}")
	if src != LiteralSource {
		t.Errorf("schema.metadata expr: Source = %q, want %q (known limitation: only schema.spec.* is SchemaSource)", src, LiteralSource)
	}
	if path != "" {
		t.Errorf("schema.metadata expr: SourcePath = %q, want empty", path)
	}
}

func TestAnalyzeForEachSource_CELMethodCallOnSchemaField(t *testing.T) {
	// CEL method call — e.g., schema.spec.items.filter(x, x.enabled)
	// The field path is truncated at the first non-identifier/non-dot character '('.
	// This documents the truncation behavior: SourcePath captures only the clean prefix.
	src, path := analyzeForEachSource("${schema.spec.items.filter(x, x.enabled)}")
	if src != SchemaSource {
		t.Errorf("CEL method call: Source = %q, want %q", src, SchemaSource)
	}
	// Truncated at '(' → "spec.items.filter" → TrimRight('.') → "spec.items.filter"
	if path != "spec.items.filter" {
		t.Errorf("CEL method call: SourcePath = %q, want %q", path, "spec.items.filter")
	}
}
