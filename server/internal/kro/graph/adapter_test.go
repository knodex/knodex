// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	krocel "github.com/kubernetes-sigs/kro/pkg/cel"
	krograph "github.com/kubernetes-sigs/kro/pkg/graph"
)

// makeNode is a test helper that creates a KRO Node with minimal required fields.
func makeNode(id string, index int, kind string, nodeType krograph.NodeType, deps ...string) *krograph.Node {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetAPIVersion("apps/v1")

	return &krograph.Node{
		Meta: krograph.NodeMeta{
			ID:           id,
			Index:        index,
			Type:         nodeType,
			Dependencies: deps,
		},
		Template: obj,
	}
}

// makeGraph builds a minimal *krograph.Graph from the provided nodes.
// The topological order is set to the slice of IDs in the order given.
func makeGraph(nodes []*krograph.Node, topoOrder []string) *krograph.Graph {
	nodeMap := make(map[string]*krograph.Node, len(nodes))
	for _, n := range nodes {
		nodeMap[n.Meta.ID] = n
	}
	return &krograph.Graph{
		Nodes:            nodeMap,
		TopologicalOrder: topoOrder,
	}
}

// TestNodeID verifies the UI-format ID generation: "{index}-{kind}".
func TestNodeID(t *testing.T) {
	node := makeNode("vpc", 0, "VPC", krograph.NodeTypeResource)
	got := nodeID(node)
	if got != "0-VPC" {
		t.Errorf("nodeID() = %q, want %q", got, "0-VPC")
	}

	// nil template → empty kind
	nodeNoTemplate := &krograph.Node{
		Meta: krograph.NodeMeta{ID: "x", Index: 3},
	}
	got = nodeID(nodeNoTemplate)
	if got != "3-" {
		t.Errorf("nodeID() with nil template = %q, want %q", got, "3-")
	}
}

// TestGetResources_DependsOnUsesUIFormatIDs is the regression test for H1.
// When the KRO builder is used, DependsOn must use UI-format IDs ("{index}-{kind}")
// not raw KRO internal IDs (node.Meta.ID). The frontend's useGraphLayout uses
// DependsOn values as map keys against resource.id — a mismatch silently breaks layout.
func TestGetResources_DependsOnUsesUIFormatIDs(t *testing.T) {
	// vpc: index 0, no deps
	// service: index 1, depends on "vpc" (KRO internal ID)
	vpc := makeNode("vpc", 0, "Deployment", krograph.NodeTypeResource)
	svc := makeNode("service", 1, "Service", krograph.NodeTypeResource, "vpc")
	g := makeGraph([]*krograph.Node{vpc, svc}, []string{"vpc", "service"})

	adapter := NewUIGraphAdapter(g)
	resources := adapter.GetResources()

	if len(resources) != 2 {
		t.Fatalf("GetResources() returned %d resources, want 2", len(resources))
	}

	// Find service resource
	var serviceRes *struct {
		id        string
		dependsOn []string
	}
	for _, r := range resources {
		if r.ID == "1-Service" {
			serviceRes = &struct {
				id        string
				dependsOn []string
			}{r.ID, r.DependsOn}
		}
	}
	if serviceRes == nil {
		t.Fatal("GetResources() did not produce a resource with ID '1-Service'")
	}

	if len(serviceRes.dependsOn) != 1 {
		t.Fatalf("service.DependsOn has %d entries, want 1", len(serviceRes.dependsOn))
	}

	// Must be UI format "0-Deployment", NOT raw KRO internal ID "vpc"
	got := serviceRes.dependsOn[0]
	want := "0-Deployment"
	if got != want {
		t.Errorf("service.DependsOn[0] = %q, want %q (raw KRO ID leaked into API response)", got, want)
	}
}

// TestGetResources_OrderedByIndex verifies deterministic ordering by Meta.Index.
func TestGetResources_OrderedByIndex(t *testing.T) {
	a := makeNode("b-resource", 2, "ConfigMap", krograph.NodeTypeResource)
	b := makeNode("a-resource", 0, "Deployment", krograph.NodeTypeResource)
	c := makeNode("c-resource", 1, "Service", krograph.NodeTypeResource)
	g := makeGraph([]*krograph.Node{a, b, c}, nil)

	adapter := NewUIGraphAdapter(g)
	resources := adapter.GetResources()

	if len(resources) != 3 {
		t.Fatalf("want 3 resources, got %d", len(resources))
	}
	// Should be ordered by index: 0, 1, 2
	wantIDs := []string{"0-Deployment", "1-Service", "2-ConfigMap"}
	for i, r := range resources {
		if r.ID != wantIDs[i] {
			t.Errorf("resources[%d].ID = %q, want %q", i, r.ID, wantIDs[i])
		}
	}
}

// TestGetResources_ExcludesInstanceNode verifies the instance node is not exposed.
func TestGetResources_ExcludesInstanceNode(t *testing.T) {
	res := makeNode("deploy", 0, "Deployment", krograph.NodeTypeResource)
	inst := makeNode(krograph.InstanceNodeID, 1, "MyApp", krograph.NodeTypeInstance)
	g := makeGraph([]*krograph.Node{res, inst}, nil)

	adapter := NewUIGraphAdapter(g)
	resources := adapter.GetResources()

	for _, r := range resources {
		if r.Kind == "MyApp" {
			t.Errorf("GetResources() included the instance node (kind %q)", r.Kind)
		}
	}
	if len(resources) != 1 {
		t.Errorf("GetResources() returned %d resources, want 1", len(resources))
	}
}

// TestGetEdges_UsesUIFormatIDs verifies edges use UI-format IDs for from/to.
func TestGetEdges_UsesUIFormatIDs(t *testing.T) {
	vpc := makeNode("vpc", 0, "Deployment", krograph.NodeTypeResource)
	svc := makeNode("service", 1, "Service", krograph.NodeTypeResource, "vpc")
	g := makeGraph([]*krograph.Node{vpc, svc}, nil)

	adapter := NewUIGraphAdapter(g)
	edges := adapter.GetEdges()

	if len(edges) != 1 {
		t.Fatalf("GetEdges() returned %d edges, want 1", len(edges))
	}
	e := edges[0]
	if e.From != "1-Service" {
		t.Errorf("edge.From = %q, want %q", e.From, "1-Service")
	}
	if e.To != "0-Deployment" {
		t.Errorf("edge.To = %q, want %q", e.To, "0-Deployment")
	}
	if e.Type != "reference" {
		t.Errorf("edge.Type = %q, want %q", e.Type, "reference")
	}
}

// TestGetTopologicalOrder_UsesUIFormatIDs verifies topo order uses UI IDs.
func TestGetTopologicalOrder_UsesUIFormatIDs(t *testing.T) {
	vpc := makeNode("vpc", 0, "Deployment", krograph.NodeTypeResource)
	svc := makeNode("service", 1, "Service", krograph.NodeTypeResource)
	// Instance should be filtered out of topo order
	inst := makeNode(krograph.InstanceNodeID, 2, "MyApp", krograph.NodeTypeInstance)
	g := makeGraph([]*krograph.Node{vpc, svc, inst}, []string{"vpc", "service"})

	adapter := NewUIGraphAdapter(g)
	order := adapter.GetTopologicalOrder()

	want := []string{"0-Deployment", "1-Service"}
	if len(order) != len(want) {
		t.Fatalf("GetTopologicalOrder() returned %d entries, want %d: %v", len(order), len(want), order)
	}
	for i, id := range order {
		if id != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, id, want[i])
		}
	}
}

// TestGetExternalRefs verifies external node filtering.
func TestGetExternalRefs(t *testing.T) {
	res := makeNode("deploy", 0, "Deployment", krograph.NodeTypeResource)
	ext := makeNode("secret", 1, "Secret", krograph.NodeTypeExternal)
	g := makeGraph([]*krograph.Node{res, ext}, nil)

	adapter := NewUIGraphAdapter(g)
	refs := adapter.GetExternalRefs()

	if len(refs) != 1 {
		t.Fatalf("GetExternalRefs() returned %d refs, want 1", len(refs))
	}
	if refs[0].Kind != "Secret" {
		t.Errorf("ExternalRef kind = %q, want %q", refs[0].Kind, "Secret")
	}
	if refs[0].IsTemplate {
		t.Error("ExternalRef should not have IsTemplate=true")
	}
}

// TestGetConditionalResources verifies filtering by IncludeWhen.
func TestGetConditionalResources(t *testing.T) {
	uncond := makeNode("deploy", 0, "Deployment", krograph.NodeTypeResource)
	cond := makeNode("svc", 1, "Service", krograph.NodeTypeResource)
	cond.IncludeWhen = []*krocel.Expression{
		{Original: "schema.spec.enableService"},
	}
	g := makeGraph([]*krograph.Node{uncond, cond}, nil)

	adapter := NewUIGraphAdapter(g)
	conditionals := adapter.GetConditionalResources()

	if len(conditionals) != 1 {
		t.Fatalf("GetConditionalResources() returned %d, want 1", len(conditionals))
	}
	if conditionals[0].ID != "1-Service" {
		t.Errorf("conditional resource ID = %q, want %q", conditionals[0].ID, "1-Service")
	}
	if conditionals[0].IncludeWhen == nil {
		t.Error("conditional resource IncludeWhen should not be nil")
	}
}

// TestGetCollectionResources verifies filtering by NodeTypeCollection.
func TestGetCollectionResources(t *testing.T) {
	res := makeNode("deploy", 0, "Deployment", krograph.NodeTypeResource)
	col := makeNode("replicas", 1, "Pod", krograph.NodeTypeCollection)
	g := makeGraph([]*krograph.Node{res, col}, nil)

	adapter := NewUIGraphAdapter(g)
	collections := adapter.GetCollectionResources()

	if len(collections) != 1 {
		t.Fatalf("GetCollectionResources() returned %d, want 1", len(collections))
	}
	if !collections[0].IsCollection {
		t.Error("collection resource should have IsCollection=true")
	}
}

// TestGetResourceByID verifies lookup by UI-format ID.
func TestGetResourceByID(t *testing.T) {
	res := makeNode("deploy", 0, "Deployment", krograph.NodeTypeResource)
	g := makeGraph([]*krograph.Node{res}, nil)

	adapter := NewUIGraphAdapter(g)

	found := adapter.GetResourceByID("0-Deployment")
	if found == nil {
		t.Fatal("GetResourceByID('0-Deployment') returned nil")
	}
	if found.Kind != "Deployment" {
		t.Errorf("found resource kind = %q, want %q", found.Kind, "Deployment")
	}

	notFound := adapter.GetResourceByID("0-NotExist")
	if notFound != nil {
		t.Error("GetResourceByID for non-existent ID should return nil")
	}
}

// TestNewUIGraphAdapter_NilSafety verifies nil input and nil graph don't panic.
func TestNewUIGraphAdapter_NilSafety(t *testing.T) {
	if NewUIGraphAdapter(nil) != nil {
		t.Error("NewUIGraphAdapter(nil) should return nil")
	}

	var a *UIGraphAdapter
	// All methods on nil adapter must not panic
	_ = a.GetResources()
	_ = a.GetEdges()
	_ = a.GetTopologicalOrder()
	_ = a.GetExternalRefs()
	_ = a.GetConditionalResources()
	_ = a.GetCollectionResources()
	_ = a.GetResourceByID("anything")
	_ = a.GetResourceGraph("name", "ns", nil)
}

// TestGetResourceGraph_Integration verifies the full pipeline produces consistent IDs.
// Specifically: resources[i].id must appear in resources[j].dependsOn (not raw KRO IDs).
func TestGetResourceGraph_Integration(t *testing.T) {
	// Three-node graph: deployment depends on configmap and secret (external)
	cm := makeNode("config", 0, "ConfigMap", krograph.NodeTypeResource)
	ext := makeNode("creds", 1, "Secret", krograph.NodeTypeExternal)
	deploy := makeNode("deploy", 2, "Deployment", krograph.NodeTypeResource, "config", "creds")
	g := makeGraph([]*krograph.Node{cm, ext, deploy}, []string{"config", "creds", "deploy"})

	adapter := NewUIGraphAdapter(g)
	rg := adapter.GetResourceGraph("my-rgd", "default", nil)

	if rg == nil {
		t.Fatal("GetResourceGraph returned nil")
	}

	// Build set of all resource IDs
	resourceIDs := make(map[string]bool)
	for _, r := range rg.Resources {
		resourceIDs[r.ID] = true
	}

	// Every DependsOn entry must reference a known resource ID
	for _, r := range rg.Resources {
		for _, dep := range r.DependsOn {
			if !resourceIDs[dep] {
				t.Errorf("resource %q has DependsOn entry %q which is not a known resource ID (raw KRO ID leaked)", r.ID, dep)
			}
		}
	}
}
