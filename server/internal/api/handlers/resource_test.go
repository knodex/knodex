// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
)

// collectionRGDSpec returns a minimal RawSpec with one forEach collection resource and one plain resource.
// The collection resource includes readyWhen and includeWhen to cover AC1 assertions.
func collectionRGDSpec() map[string]interface{} {
	return map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"id": "workerPods",
				"forEach": []interface{}{
					map[string]interface{}{"worker": "${schema.spec.workers}"},
				},
				"readyWhen": []interface{}{
					"each.status.phase == 'Running'",
				},
				"includeWhen": []interface{}{
					"schema.spec.workers > 0",
				},
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
				},
			},
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
				},
			},
		},
	}
}

// plainRGDSpec returns a minimal RawSpec with one non-collection resource.
func plainRGDSpec() map[string]interface{} {
	return map[string]interface{}{
		"resources": []interface{}{
			map[string]interface{}{
				"template": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
		},
	}
}

func newResourceHandlerWithRGD(name, namespace string, rawSpec map[string]interface{}) *ResourceHandler {
	cache := watcher.NewRGDCache()
	cache.Set(&models.CatalogRGD{
		Name:        name,
		Namespace:   namespace,
		RawSpec:     rawSpec,
		Annotations: map[string]string{models.CatalogAnnotation: "true"},
	})
	return NewResourceHandler(watcher.NewRGDWatcherWithCache(cache))
}

// TestGetDefinitionGraph_CollectionNode verifies that a forEach resource has
// isCollection=true, non-nil forEach array, and readyWhen populated (AC1).
func TestGetDefinitionGraph_CollectionNode(t *testing.T) {
	t.Parallel()

	handler := newResourceHandlerWithRGD("worker-rgd", "default", collectionRGDSpec())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/worker-rgd/graph", nil)
	req.SetPathValue("name", "worker-rgd")
	rec := httptest.NewRecorder()

	handler.GetDefinitionGraph(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(body.Resources))
	}

	// First resource should be the collection node
	var collNode *ResourceNodeResponse
	for i := range body.Resources {
		if body.Resources[i].IsCollection {
			collNode = &body.Resources[i]
			break
		}
	}
	if collNode == nil {
		t.Fatal("expected a collection node (isCollection=true), none found")
	}
	if len(collNode.ForEach) == 0 {
		t.Error("expected non-empty forEach on collection node")
	}
	if collNode.ForEach[0].Name != "worker" {
		t.Errorf("forEach[0].name = %q, want %q", collNode.ForEach[0].Name, "worker")
	}
	// AC1: readyWhen must be populated
	if len(collNode.ReadyWhen) == 0 {
		t.Error("expected non-empty readyWhen on collection node (AC1)")
	} else if collNode.ReadyWhen[0] != "each.status.phase == 'Running'" {
		t.Errorf("readyWhen[0] = %q, want %q", collNode.ReadyWhen[0], "each.status.phase == 'Running'")
	}
	// AC1: conditionExpr must be populated from includeWhen
	if collNode.ConditionExpr == "" {
		t.Error("expected non-empty conditionExpr from includeWhen on collection node (AC1)")
	}
}

// TestGetDefinitionGraph_NonCollection_NoRegression verifies that non-collection nodes
// have isCollection=false and forEach=null, and existing fields are unchanged (AC2).
func TestGetDefinitionGraph_NonCollection_NoRegression(t *testing.T) {
	t.Parallel()

	handler := newResourceHandlerWithRGD("plain-rgd", "default", plainRGDSpec())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/plain-rgd/graph", nil)
	req.SetPathValue("name", "plain-rgd")
	rec := httptest.NewRecorder()

	handler.GetDefinitionGraph(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Resources) == 0 {
		t.Fatal("expected at least one resource")
	}

	for _, node := range body.Resources {
		if node.IsCollection {
			t.Errorf("resource %q: IsCollection = true, want false for non-collection RGD", node.ID)
		}
		if len(node.ForEach) != 0 {
			t.Errorf("resource %q: ForEach non-empty for non-collection resource", node.ID)
		}
		if node.Kind == "" {
			t.Errorf("resource %q: Kind is empty (regression check)", node.ID)
		}
	}
}

// TestGetResourceGraph_CollectionFields verifies that the /resources endpoint exposes
// full collection metadata (isCollection, forEach) — same as /graph.
func TestGetResourceGraph_CollectionFields(t *testing.T) {
	t.Parallel()

	handler := newResourceHandlerWithRGD("worker-rgd", "default", collectionRGDSpec())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/worker-rgd/resources", nil)
	req.SetPathValue("name", "worker-rgd")
	rec := httptest.NewRecorder()

	handler.GetResourceGraph(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body ResourceGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	var collNode *ResourceNodeResponse
	for i := range body.Resources {
		if body.Resources[i].IsCollection {
			collNode = &body.Resources[i]
			break
		}
	}
	if collNode == nil {
		t.Fatal("expected a collection node (isCollection=true) in /resources response")
	}
	if len(collNode.ForEach) == 0 {
		t.Error("expected non-empty forEach on collection node")
	}
}
