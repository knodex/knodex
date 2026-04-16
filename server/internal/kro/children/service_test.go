// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package children

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	kroparser "github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/models"
)

// --- Test doubles ---

type mockRGDProvider struct {
	rgds map[string]*models.CatalogRGD
}

func (m *mockRGDProvider) GetRGD(namespace, name string) (*models.CatalogRGD, bool) {
	rgd, ok := m.rgds[namespace+"/"+name]
	return rgd, ok
}

type mockInstanceProvider struct {
	instances map[string]*models.Instance
}

func (m *mockInstanceProvider) GetInstance(namespace, kind, name string) (*models.Instance, bool) {
	key := namespace + "/" + kind + "/" + name
	inst, ok := m.instances[key]
	return inst, ok
}

// --- Helper builders ---

func makeChildPod(name, namespace, nodeID, phase string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
				"labels": map[string]interface{}{
					"kro.run/instance-name":      "demo-app",
					"kro.run/instance-kind":      "TestPodPair",
					"kro.run/instance-namespace": namespace,
					"kro.run/node-id":            nodeID,
					"kro.run/owned":              "true",
				},
			},
			"status": map[string]interface{}{
				"phase": phase,
			},
		},
	}
	return obj
}

func makeChildService(name, namespace, nodeID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
				"labels": map[string]interface{}{
					"kro.run/instance-name":      "demo-app",
					"kro.run/instance-kind":      "TestPodPair",
					"kro.run/instance-namespace": namespace,
					"kro.run/node-id":            nodeID,
					"kro.run/owned":              "true",
				},
			},
		},
	}
}

// simpleRGDSpec creates a minimal RGD spec with the given resource kinds.
func simpleRGDSpec(resources ...struct{ apiVersion, kind string }) map[string]interface{} {
	resList := make([]interface{}, 0, len(resources))
	for _, r := range resources {
		resList = append(resList, map[string]interface{}{
			"id": r.kind,
			"template": map[string]interface{}{
				"apiVersion": r.apiVersion,
				"kind":       r.kind,
				"metadata": map[string]interface{}{
					"name": "placeholder",
				},
			},
		})
	}
	return map[string]interface{}{
		"schema": map[string]interface{}{
			"apiVersion": "kro.run/v1alpha1",
			"kind":       "TestPodPair",
			"spec":       map[string]interface{}{},
		},
		"resources": resList,
	}
}

// --- Tests ---

func TestBuildChildLabelSelector(t *testing.T) {
	tests := []struct {
		name      string
		instName  string
		instNS    string
		instKind  string
		wantParts []string
	}{
		{
			name:     "namespaced instance",
			instName: "demo-app",
			instNS:   "default",
			instKind: "TestPodPair",
			wantParts: []string{
				"kro.run/instance-name=demo-app",
				"kro.run/instance-kind=TestPodPair",
				"kro.run/instance-namespace=default",
			},
		},
		{
			name:     "cluster-scoped instance (no namespace)",
			instName: "global-app",
			instNS:   "",
			instKind: "ClusterApp",
			wantParts: []string{
				"kro.run/instance-name=global-app",
				"kro.run/instance-kind=ClusterApp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildChildLabelSelector(tt.instName, tt.instNS, tt.instKind)
			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("selector %q missing expected part %q", got, part)
				}
			}
			// Cluster-scoped should NOT contain namespace selector
			if tt.instNS == "" && strings.Contains(got, "instance-namespace=") {
				t.Errorf("cluster-scoped selector should not contain namespace: %q", got)
			}
		})
	}
}

func TestResolveGVR(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		wantGVR    schema.GroupVersionResource
		wantErr    bool
	}{
		{
			name:       "core v1 Pod",
			apiVersion: "v1",
			kind:       "Pod",
			wantGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		{
			name:       "apps/v1 Deployment",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			wantGVR:    schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
		{
			name:       "core v1 Service",
			apiVersion: "v1",
			kind:       "Service",
			wantGVR:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
		},
		{
			name:       "networking Ingress",
			apiVersion: "networking.k8s.io/v1",
			kind:       "Ingress",
			wantGVR:    schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		},
		{
			name:       "unknown CRD falls back to lowercase+s",
			apiVersion: "example.com/v1",
			kind:       "Widget",
			wantGVR:    schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"},
		},
		{
			name:       "invalid apiVersion",
			apiVersion: "///invalid",
			kind:       "Pod",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveGVR(tt.apiVersion, tt.kind)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantGVR {
				t.Errorf("got %v, want %v", got, tt.wantGVR)
			}
		})
	}
}

func TestGroupByNodeID(t *testing.T) {
	children := []models.ChildResource{
		{Name: "frontend-pod-1", NodeID: "frontend", Kind: "Pod", Health: models.HealthHealthy},
		{Name: "frontend-pod-2", NodeID: "frontend", Kind: "Pod", Health: models.HealthHealthy},
		{Name: "backend-pod-1", NodeID: "backend", Kind: "Pod", Health: models.HealthUnhealthy},
		{Name: "unknown-svc", NodeID: "", Kind: "Service", Health: models.HealthHealthy},
	}

	groups := groupByNodeID(children)

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// First group should be "frontend" (insertion order)
	if groups[0].NodeID != "frontend" {
		t.Errorf("first group nodeID = %q, want %q", groups[0].NodeID, "frontend")
	}
	if groups[0].Count != 2 {
		t.Errorf("frontend count = %d, want 2", groups[0].Count)
	}
	if groups[0].ReadyCount != 2 {
		t.Errorf("frontend readyCount = %d, want 2", groups[0].ReadyCount)
	}
	if groups[0].Health != models.HealthHealthy {
		t.Errorf("frontend health = %q, want %q", groups[0].Health, models.HealthHealthy)
	}

	// Backend group should be unhealthy
	if groups[1].NodeID != "backend" {
		t.Errorf("second group nodeID = %q, want %q", groups[1].NodeID, "backend")
	}
	if groups[1].Health != models.HealthUnhealthy {
		t.Errorf("backend health = %q, want %q", groups[1].Health, models.HealthUnhealthy)
	}

	// Unknown node-id group
	if groups[2].NodeID != "unknown" {
		t.Errorf("third group nodeID = %q, want %q", groups[2].NodeID, "unknown")
	}
}

func TestDeriveChildHealth(t *testing.T) {
	tests := []struct {
		name string
		obj  *unstructured.Unstructured
		want models.InstanceHealth
	}{
		{
			name: "running pod",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{"phase": "Running"},
			}},
			want: models.HealthHealthy,
		},
		{
			name: "pending pod",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{"phase": "Pending"},
			}},
			want: models.HealthProgressing,
		},
		{
			name: "failed pod",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{"phase": "Failed"},
			}},
			want: models.HealthUnhealthy,
		},
		{
			name: "ready condition true",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
					},
				},
			}},
			want: models.HealthHealthy,
		},
		{
			name: "ready condition false",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "False",
							"reason": "SomeError",
						},
					},
				},
			}},
			want: models.HealthUnhealthy,
		},
		{
			name: "no status at all",
			obj:  &unstructured.Unstructured{Object: map[string]interface{}{}},
			want: models.HealthNone,
		},
		{
			name: "service - no health concept",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind": "Service",
			}},
			want: models.HealthNone,
		},
		{
			name: "configmap - no health concept",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind": "ConfigMap",
			}},
			want: models.HealthNone,
		},
		{
			name: "secret - no health concept",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind": "Secret",
			}},
			want: models.HealthNone,
		},
		{
			name: "statefulset - all replicas ready",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "StatefulSet",
				"status": map[string]interface{}{"replicas": int64(3), "readyReplicas": int64(3)},
			}},
			want: models.HealthHealthy,
		},
		{
			name: "statefulset - no replicas ready",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "StatefulSet",
				"status": map[string]interface{}{"replicas": int64(3), "readyReplicas": int64(0)},
			}},
			want: models.HealthProgressing,
		},
		{
			name: "statefulset - partially ready",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "StatefulSet",
				"status": map[string]interface{}{"replicas": int64(3), "readyReplicas": int64(1)},
			}},
			want: models.HealthDegraded,
		},
		{
			name: "statefulset - scaled to zero",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "StatefulSet",
				"status": map[string]interface{}{"replicas": int64(0), "readyReplicas": int64(0)},
			}},
			want: models.HealthHealthy,
		},
		{
			name: "daemonset - all scheduled ready",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "DaemonSet",
				"status": map[string]interface{}{"desiredNumberScheduled": int64(2), "numberReady": int64(2)},
			}},
			want: models.HealthHealthy,
		},
		{
			name: "daemonset - partially ready",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "DaemonSet",
				"status": map[string]interface{}{"desiredNumberScheduled": int64(3), "numberReady": int64(1)},
			}},
			want: models.HealthDegraded,
		},
		{
			name: "job - complete",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind": "Job",
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Complete", "status": "True"},
					},
				},
			}},
			want: models.HealthHealthy,
		},
		{
			name: "job - failed",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind": "Job",
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "Failed", "status": "True"},
					},
				},
			}},
			want: models.HealthUnhealthy,
		},
		{
			name: "job - in progress",
			obj: &unstructured.Unstructured{Object: map[string]interface{}{
				"kind":   "Job",
				"status": map[string]interface{}{},
			}},
			want: models.HealthProgressing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := tt.obj.Object["status"].(map[string]interface{})
			if status == nil {
				status = map[string]interface{}{}
			}
			got := deriveChildHealth(tt.obj, status)
			if got != tt.want {
				t.Errorf("deriveChildHealth() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListChildResources_InstanceNotFound(t *testing.T) {
	svc := NewService(
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{}},
		&mockInstanceProvider{instances: map[string]*models.Instance{}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)

	_, err := svc.ListChildResources(context.Background(), "default", "TestPodPair", "missing")
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
}

func TestListChildResources_RGDNotFound(t *testing.T) {
	svc := NewService(
		dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"default/TestPodPair/demo-app": {
				Name: "demo-app", Namespace: "default", Kind: "TestPodPair",
				RGDName: "test-pod-pair", RGDNamespace: "default",
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)

	_, err := svc.ListChildResources(context.Background(), "default", "TestPodPair", "demo-app")
	if err == nil {
		t.Fatal("expected error for missing RGD")
	}
}

func TestListChildResources_Success(t *testing.T) {
	// Create fake dynamic client with child pods
	scheme := runtime.NewScheme()
	pod1 := makeChildPod("frontend-pod", "default", "frontend", "Running")
	pod2 := makeChildPod("backend-pod", "default", "backend", "Pending")

	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		pod1, pod2,
	)

	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		fakeClient,
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-pod-pair": {
				Name: "test-pod-pair", Namespace: "default",
				Kind: "TestPodPair", RawSpec: rgdSpec,
			},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"default/TestPodPair/demo-app": {
				Name: "demo-app", Namespace: "default", Kind: "TestPodPair",
				RGDName: "test-pod-pair", RGDNamespace: "default",
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)

	resp, err := svc.ListChildResources(context.Background(), "default", "TestPodPair", "demo-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.InstanceName != "demo-app" {
		t.Errorf("InstanceName = %q, want %q", resp.InstanceName, "demo-app")
	}
	if resp.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", resp.TotalCount)
	}
	if len(resp.Groups) != 2 {
		t.Errorf("Groups count = %d, want 2", len(resp.Groups))
	}
}

func TestKindToResource(t *testing.T) {
	tests := []struct {
		kind string
		want string
	}{
		{"Pod", "pods"},
		{"Deployment", "deployments"},
		{"Service", "services"},
		{"ConfigMap", "configmaps"},
		{"Ingress", "ingresses"},
		{"HorizontalPodAutoscaler", "horizontalpodautoscalers"},
		{"CustomThing", "customthings"}, // fallback
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := kindToResource(tt.kind)
			if got != tt.want {
				t.Errorf("kindToResource(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestAggregateGroupHealth(t *testing.T) {
	tests := []struct {
		name      string
		resources []models.ChildResource
		want      models.InstanceHealth
	}{
		{"empty", nil, models.HealthUnknown},
		{"all healthy", []models.ChildResource{
			{Health: models.HealthHealthy}, {Health: models.HealthHealthy},
		}, models.HealthHealthy},
		{"one unhealthy", []models.ChildResource{
			{Health: models.HealthHealthy}, {Health: models.HealthUnhealthy},
		}, models.HealthUnhealthy},
		{"one degraded", []models.ChildResource{
			{Health: models.HealthHealthy}, {Health: models.HealthDegraded},
		}, models.HealthDegraded},
		{"one progressing", []models.ChildResource{
			{Health: models.HealthHealthy}, {Health: models.HealthProgressing},
		}, models.HealthProgressing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.AggregateGroupHealth(tt.resources)
			if got != tt.want {
				t.Errorf("AggregateGroupHealth() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Mock RemoteClientProvider ---

type mockRemoteClientProvider struct {
	clients    map[string]*dynamicfake.FakeDynamicClient
	reachable  map[string]bool
	clientErrs map[string]error
}

func (m *mockRemoteClientProvider) GetDynamicClient(clusterRef string) (dynamic.Interface, error) {
	if err, ok := m.clientErrs[clusterRef]; ok && err != nil {
		return nil, err
	}
	if c, ok := m.clients[clusterRef]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("cluster %s not found", clusterRef)
}

func (m *mockRemoteClientProvider) IsClusterReachable(clusterRef string) bool {
	if m.reachable == nil {
		return false
	}
	return m.reachable[clusterRef]
}

// makeChildPodForInstance creates a child pod with labels matching the given instance.
func makeChildPodForInstance(podName, namespace, nodeID, phase, instanceName, instanceKind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":              podName,
				"namespace":         namespace,
				"creationTimestamp": time.Now().Format(time.RFC3339),
				"labels": map[string]interface{}{
					"kro.run/instance-name":      instanceName,
					"kro.run/instance-kind":      instanceKind,
					"kro.run/instance-namespace": namespace,
					"kro.run/node-id":            nodeID,
					"kro.run/owned":              "true",
				},
			},
			"status": map[string]interface{}{
				"phase": phase,
			},
		},
	}
}

// --- STORY-421 Tests: Multi-cluster child resource routing ---

func TestListChildResources_TargetCluster_PopulatesClusterField(t *testing.T) {
	scheme := runtime.NewScheme()
	pod := makeChildPodForInstance("frontend-pod", "team-alpha", "frontend", "Running", "demo", "TestApp")

	remoteClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		pod,
	)

	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		dynamicfake.NewSimpleDynamicClient(scheme), // management cluster (empty)
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-rgd": {Name: "test-rgd", Namespace: "default", Kind: "TestApp", RawSpec: rgdSpec},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"team-alpha/TestApp/demo": {
				Name: "demo", Namespace: "team-alpha", Kind: "TestApp",
				RGDName: "test-rgd", RGDNamespace: "default",
				TargetCluster: "prod-eu-west",
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)
	svc.SetRemoteClientProvider(&mockRemoteClientProvider{
		clients:   map[string]*dynamicfake.FakeDynamicClient{"prod-eu-west": remoteClient},
		reachable: map[string]bool{"prod-eu-west": true},
	})

	resp, err := svc.ListChildResources(context.Background(), "team-alpha", "TestApp", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Fatalf("expected 1 child, got %d", resp.TotalCount)
	}
	if resp.Groups[0].Resources[0].Cluster != "prod-eu-west" {
		t.Errorf("Cluster = %q, want %q", resp.Groups[0].Resources[0].Cluster, "prod-eu-west")
	}
	if resp.ClusterUnreachable {
		t.Error("expected ClusterUnreachable to be false")
	}
}

func TestListChildResources_NoAnnotation_NoClusterField(t *testing.T) {
	scheme := runtime.NewScheme()
	pod := makeChildPodForInstance("frontend-pod", "default", "frontend", "Running", "demo", "TestApp")

	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		pod,
	)

	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		fakeClient,
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-rgd": {Name: "test-rgd", Namespace: "default", Kind: "TestApp", RawSpec: rgdSpec},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"default/TestApp/demo": {
				Name: "demo", Namespace: "default", Kind: "TestApp",
				RGDName: "test-rgd", RGDNamespace: "default",
				// No TargetCluster
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)

	resp, err := svc.ListChildResources(context.Background(), "default", "TestApp", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Fatalf("expected 1 child, got %d", resp.TotalCount)
	}
	if resp.Groups[0].Resources[0].Cluster != "" {
		t.Errorf("Cluster = %q, want empty (management cluster)", resp.Groups[0].Resources[0].Cluster)
	}
}

func TestListChildResources_UnreachableCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		dynamicfake.NewSimpleDynamicClient(scheme),
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-rgd": {Name: "test-rgd", Namespace: "default", Kind: "TestApp", RawSpec: rgdSpec},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"default/TestApp/demo": {
				Name: "demo", Namespace: "default", Kind: "TestApp",
				RGDName: "test-rgd", RGDNamespace: "default",
				TargetCluster: "prod-us-east",
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)
	svc.SetRemoteClientProvider(&mockRemoteClientProvider{
		reachable: map[string]bool{"prod-us-east": false},
	})

	resp, err := svc.ListChildResources(context.Background(), "default", "TestApp", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.ClusterUnreachable {
		t.Error("expected ClusterUnreachable to be true")
	}
	if len(resp.UnreachableClusters) != 1 || resp.UnreachableClusters[0] != "prod-us-east" {
		t.Errorf("UnreachableClusters = %v, want [prod-us-east]", resp.UnreachableClusters)
	}
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0 (no data from unreachable cluster)", resp.TotalCount)
	}
}

func TestListChildResources_NilProvider_UsesManagementCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	pod := makeChildPodForInstance("frontend-pod", "default", "frontend", "Running", "demo", "TestApp")

	fakeClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		pod,
	)

	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		fakeClient,
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-rgd": {Name: "test-rgd", Namespace: "default", Kind: "TestApp", RawSpec: rgdSpec},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"default/TestApp/demo": {
				Name: "demo", Namespace: "default", Kind: "TestApp",
				RGDName: "test-rgd", RGDNamespace: "default",
				TargetCluster: "prod-eu-west", // Has annotation but no provider
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)
	// No SetRemoteClientProvider — provider is nil

	resp, err := svc.ListChildResources(context.Background(), "default", "TestApp", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to management cluster (no error, no unreachable)
	if resp.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1 (management cluster fallback)", resp.TotalCount)
	}
	if resp.ClusterUnreachable {
		t.Error("expected ClusterUnreachable to be false when provider is nil")
	}
}

func TestListChildResources_GetDynamicClientError(t *testing.T) {
	scheme := runtime.NewScheme()
	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		dynamicfake.NewSimpleDynamicClient(scheme),
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-rgd": {Name: "test-rgd", Namespace: "default", Kind: "TestApp", RawSpec: rgdSpec},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"default/TestApp/demo": {
				Name: "demo", Namespace: "default", Kind: "TestApp",
				RGDName: "test-rgd", RGDNamespace: "default",
				TargetCluster: "prod-eu-west",
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)
	svc.SetRemoteClientProvider(&mockRemoteClientProvider{
		reachable:  map[string]bool{"prod-eu-west": true},
		clientErrs: map[string]error{"prod-eu-west": fmt.Errorf("connection refused")},
	})

	resp, err := svc.ListChildResources(context.Background(), "default", "TestApp", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.ClusterUnreachable {
		t.Error("expected ClusterUnreachable to be true when GetDynamicClient fails")
	}
	if resp.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", resp.TotalCount)
	}
}

func TestListChildResources_RemoteClusterUsesRemoteClient(t *testing.T) {
	scheme := runtime.NewScheme()

	// Management cluster has no pods
	mgmtClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
	)

	// Remote cluster has the pod
	remotePod := makeChildPodForInstance("remote-pod", "team-alpha", "frontend", "Running", "demo", "TestApp")
	remoteClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "pods"}: "PodList",
		},
		remotePod,
	)

	rgdSpec := simpleRGDSpec(struct{ apiVersion, kind string }{"v1", "Pod"})

	svc := NewService(
		mgmtClient,
		&mockRGDProvider{rgds: map[string]*models.CatalogRGD{
			"default/test-rgd": {Name: "test-rgd", Namespace: "default", Kind: "TestApp", RawSpec: rgdSpec},
		}},
		&mockInstanceProvider{instances: map[string]*models.Instance{
			"team-alpha/TestApp/demo": {
				Name: "demo", Namespace: "team-alpha", Kind: "TestApp",
				RGDName: "test-rgd", RGDNamespace: "default",
				TargetCluster: "prod-eu-west",
			},
		}},
		kroparser.NewResourceParser(),
		nil,
		slog.Default(),
	)
	svc.SetRemoteClientProvider(&mockRemoteClientProvider{
		clients:   map[string]*dynamicfake.FakeDynamicClient{"prod-eu-west": remoteClient},
		reachable: map[string]bool{"prod-eu-west": true},
	})

	resp, err := svc.ListChildResources(context.Background(), "team-alpha", "TestApp", "demo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find the remote pod, not the empty management cluster
	if resp.TotalCount != 1 {
		t.Fatalf("expected 1 child from remote cluster, got %d", resp.TotalCount)
	}
	if resp.Groups[0].Resources[0].Name != "remote-pod" {
		t.Errorf("expected remote-pod, got %q", resp.Groups[0].Resources[0].Name)
	}
}

// TestUnstructuredToChildResource verifies field extraction from K8s objects.
func TestUnstructuredToChildResource(t *testing.T) {
	pod := makeChildPod("test-pod", "default", "frontend", "Running")
	child := unstructuredToChildResource(pod)

	if child.Name != "test-pod" {
		t.Errorf("Name = %q, want %q", child.Name, "test-pod")
	}
	if child.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", child.Namespace, "default")
	}
	if child.Kind != "Pod" {
		t.Errorf("Kind = %q, want %q", child.Kind, "Pod")
	}
	if child.NodeID != "frontend" {
		t.Errorf("NodeID = %q, want %q", child.NodeID, "frontend")
	}
	if child.Health != models.HealthHealthy {
		t.Errorf("Health = %q, want %q", child.Health, models.HealthHealthy)
	}
	if child.Phase != "Running" {
		t.Errorf("Phase = %q, want %q", child.Phase, "Running")
	}
}
