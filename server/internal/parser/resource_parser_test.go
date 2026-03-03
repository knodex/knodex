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
