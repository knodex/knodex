// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"os"
	"testing"

	"sigs.k8s.io/yaml"
)

// agentRGDPath is the shipped agent-wrapping RGD (Story 53.3): the canonical
// "Create Agent" artifact the Agents tab's Deploy drawer binds to by name.
const agentRGDPath = "../../../../deploy/charts/knodex/files/agents/kagent-agent.yaml"

// TestKagentAgentRGD_Contract guards the contract the Catalog/Deploy drawer and
// KRO depend on (Story 53.3 AC #1, #2, #3) — not cosmetics:
//   - knodex.io/catalog gateway: the single publishing gateway (non-catalog
//     RGDs are never cached → GET /rgds/{name} 404 → Deploy drawer can't open)
//   - the schema kind (instances are kro.run/v1alpha1 KagentAgent) — an agent
//     kind, which routes the RGD out of the catalog LIST into the Agents pages
//   - exactly TWO resources: the Agent template + the ModelConfig externalRef
//   - schema interpolation flows the deploy form fields into the Agent
func TestKagentAgentRGD_Contract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(agentRGDPath)
	if err != nil {
		t.Fatalf("failed to read agent RGD: %v", err)
	}

	var rgd map[string]interface{}
	if err := yaml.Unmarshal(raw, &rgd); err != nil {
		t.Fatalf("failed to unmarshal agent RGD YAML: %v", err)
	}

	metadata, ok := rgd["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("agent RGD has no metadata map")
	}
	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		t.Fatal("agent RGD has no metadata.annotations map")
	}
	// Catalog gateway annotation — KEPT so the RGD is cached/gettable.
	if got := annotations["knodex.io/catalog"]; got != "true" {
		t.Errorf("knodex.io/catalog annotation = %v, want \"true\"", got)
	}
	spec, ok := rgd["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("agent RGD has no spec map")
	}

	// Schema kind — instances will be kro.run/v1alpha1 KagentAgent.
	schema, ok := spec["schema"].(map[string]interface{})
	if !ok {
		t.Fatal("agent RGD has no spec.schema map")
	}
	if got := schema["kind"]; got != "KagentAgent" {
		t.Errorf("spec.schema.kind = %v, want \"KagentAgent\"", got)
	}

	graph, err := NewResourceParser().ParseRGDResources("kagent-agent", "default", spec)
	if err != nil {
		t.Fatalf("ParseRGDResources failed: %v", err)
	}
	if len(graph.ParseErrors) != 0 {
		t.Fatalf("ParseRGDResources reported parse errors: %+v", graph.ParseErrors)
	}
	// Two resources now: the Agent template + the ModelConfig externalRef.
	if len(graph.Resources) != 2 {
		t.Fatalf("expected exactly 2 resources, got %d", len(graph.Resources))
	}

	// Classify by IsTemplate: exactly one Agent template + one ModelConfig
	// externalRef, both kagent.dev/v1alpha2.
	var agent, modelRef *ResourceDefinition
	for i := range graph.Resources {
		r := &graph.Resources[i]
		if r.IsTemplate {
			agent = r
		} else {
			modelRef = r
		}
	}
	if agent == nil {
		t.Fatal("expected one template resource (the Agent)")
	}
	if modelRef == nil {
		t.Fatal("expected one externalRef resource (the ModelConfig)")
	}

	if agent.APIVersion != "kagent.dev/v1alpha2" || agent.Kind != "Agent" {
		t.Errorf("agent template GVK = %q/%q, want \"kagent.dev/v1alpha2\"/\"Agent\"", agent.APIVersion, agent.Kind)
	}
	if modelRef.APIVersion != "kagent.dev/v1alpha2" || modelRef.Kind != "ModelConfig" {
		t.Errorf("externalRef GVK = %q/%q, want \"kagent.dev/v1alpha2\"/\"ModelConfig\"", modelRef.APIVersion, modelRef.Kind)
	}

	// The Agent template must interpolate every deploy-form field — including the
	// new externalRef ModelConfig name (a name-only field would not render a
	// picker, see Dev Notes).
	got := make(map[string]bool, len(agent.SchemaFields))
	for _, f := range agent.SchemaFields {
		got[f] = true
	}
	for _, want := range []string{"spec.agentName", "spec.description", "spec.systemMessage", "spec.externalRef.modelConfig.name"} {
		if !got[want] {
			t.Errorf("schema field %q not detected in Agent template; got %v", want, agent.SchemaFields)
		}
	}
}

// TestKagentAgentRGD_TemplateContract guards the template mechanisms the
// deployed agent depends on (Story 53.3 AC #1, #2) plus the verified kagent
// v1alpha2 spec shape — drift breaks the deployed agent silently (KRO would
// still accept the RGD):
//
//   - NO metadata.namespace on the Agent template: namespace inheritance places
//     the Agent in the project namespace selected at deploy time.
//   - spec.description at the Agent TOP level: the GET /api/v1/agents list reads
//     exactly this path; moving it renders empty card descriptions.
//   - spec.declarative.modelConfig set to the PICKED ModelConfig's name
//     (${schema.spec.externalRef.modelConfig.name}) — kagent resolves it by name
//     in the Agent's own namespace.
//   - the modelConfigRef externalRef exposes PAIRED name+namespace under one
//     parent (spec.externalRef.modelConfig) — the only shape the enricher maps
//     to a ModelConfig picker.
func TestKagentAgentRGD_TemplateContract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(agentRGDPath)
	if err != nil {
		t.Fatalf("failed to read agent RGD: %v", err)
	}
	var rgd map[string]interface{}
	if err := yaml.Unmarshal(raw, &rgd); err != nil {
		t.Fatalf("failed to unmarshal agent RGD YAML: %v", err)
	}

	spec, ok := rgd["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("agent RGD has no spec map")
	}
	resources, ok := spec["resources"].([]interface{})
	if !ok || len(resources) != 2 {
		t.Fatalf("expected exactly 2 spec.resources entries, got %T (len %d)", spec["resources"], len(resources))
	}

	// Locate the Agent template and the ModelConfig externalRef by shape.
	var agentSpec, modelExtRef map[string]interface{}
	for _, r := range resources {
		resMap, ok := r.(map[string]interface{})
		if !ok {
			t.Fatal("spec.resources entry is not a map")
		}
		if tmpl, ok := resMap["template"].(map[string]interface{}); ok {
			// Namespace inheritance: the template must NOT pin a namespace.
			if md, ok := tmpl["metadata"].(map[string]interface{}); ok {
				if ns, present := md["namespace"]; present {
					t.Errorf("template metadata.namespace = %v; must be absent so the Agent inherits the instance's (project) namespace", ns)
				}
			}
			agentSpec, _ = tmpl["spec"].(map[string]interface{})
		} else if ext, ok := resMap["externalRef"].(map[string]interface{}); ok {
			modelExtRef = ext
		}
	}

	if agentSpec == nil {
		t.Fatal("Agent template has no spec map")
	}
	if modelExtRef == nil {
		t.Fatal("ModelConfig externalRef resource not found")
	}

	// Installed-list contract: description at the Agent top level.
	if got := agentSpec["description"]; got != "${schema.spec.description}" {
		t.Errorf("template spec.description = %v, want \"${schema.spec.description}\" at the TOP level (the agents list reads spec.description)", got)
	}

	// Verified kagent v1alpha2 shape + the new ModelConfig-name expression.
	if got := agentSpec["type"]; got != "Declarative" {
		t.Errorf("template spec.type = %v, want \"Declarative\"", got)
	}
	declarative, ok := agentSpec["declarative"].(map[string]interface{})
	if !ok {
		t.Fatal("template has no spec.declarative map")
	}
	if got := declarative["modelConfig"]; got != "${schema.spec.externalRef.modelConfig.name}" {
		t.Errorf("template spec.declarative.modelConfig = %v, want \"${schema.spec.externalRef.modelConfig.name}\"", got)
	}
	if got := declarative["systemMessage"]; got != "${schema.spec.systemMessage}" {
		t.Errorf("template spec.declarative.systemMessage = %v, want \"${schema.spec.systemMessage}\"", got)
	}

	// ModelConfig externalRef GVK + paired name/namespace (the picker contract).
	if got := modelExtRef["apiVersion"]; got != "kagent.dev/v1alpha2" {
		t.Errorf("modelConfig externalRef apiVersion = %v, want \"kagent.dev/v1alpha2\"", got)
	}
	if got := modelExtRef["kind"]; got != "ModelConfig" {
		t.Errorf("modelConfig externalRef kind = %v, want \"ModelConfig\"", got)
	}
	extMeta, ok := modelExtRef["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("modelConfig externalRef has no metadata map")
	}
	if got := extMeta["name"]; got != "${schema.spec.externalRef.modelConfig.name}" {
		t.Errorf("modelConfig externalRef metadata.name = %v, want the paired name field", got)
	}
	if got := extMeta["namespace"]; got != "${schema.spec.externalRef.modelConfig.namespace}" {
		t.Errorf("modelConfig externalRef metadata.namespace = %v, want the paired namespace field", got)
	}
}
