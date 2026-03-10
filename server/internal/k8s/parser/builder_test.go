// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewUnstructured(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		apiVersion string
		kind       string
		namespace  string
		objName    string
	}{
		{
			name:       "namespaced object",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			namespace:  "default",
			objName:    "my-deployment",
		},
		{
			name:       "cluster-scoped object",
			apiVersion: "v1",
			kind:       "Namespace",
			namespace:  "",
			objName:    "my-namespace",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obj := NewUnstructured(tt.apiVersion, tt.kind, tt.namespace, tt.objName)

			if obj == nil {
				t.Fatal("NewUnstructured() returned nil")
			}
			if got := obj.GetAPIVersion(); got != tt.apiVersion {
				t.Errorf("apiVersion = %q, want %q", got, tt.apiVersion)
			}
			if got := obj.GetKind(); got != tt.kind {
				t.Errorf("kind = %q, want %q", got, tt.kind)
			}
			if got := obj.GetName(); got != tt.objName {
				t.Errorf("name = %q, want %q", got, tt.objName)
			}
			if tt.namespace != "" {
				if got := obj.GetNamespace(); got != tt.namespace {
					t.Errorf("namespace = %q, want %q", got, tt.namespace)
				}
			}
		})
	}
}

func TestNewClusterScopedUnstructured(t *testing.T) {
	t.Parallel()

	obj := NewClusterScopedUnstructured("v1", "Namespace", "my-namespace")

	if obj.GetNamespace() != "" {
		t.Errorf("namespace = %q, want empty", obj.GetNamespace())
	}
	if obj.GetName() != "my-namespace" {
		t.Errorf("name = %q, want \"my-namespace\"", obj.GetName())
	}
}

func TestSetSpec(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	spec := map[string]interface{}{
		"replicas": int64(3),
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"app": "test",
			},
		},
	}

	SetSpec(obj, spec)

	got, err := GetSpec(obj)
	if err != nil {
		t.Fatalf("GetSpec() error: %v", err)
	}
	if got == nil {
		t.Error("spec is nil after SetSpec()")
	}

	// Verify nested value
	selector, err := GetMap(got, "selector")
	if err != nil {
		t.Fatalf("GetMap(selector) error: %v", err)
	}
	matchLabels, err := GetMap(selector, "matchLabels")
	if err != nil {
		t.Fatalf("GetMap(matchLabels) error: %v", err)
	}
	if matchLabels["app"] != "test" {
		t.Errorf("matchLabels.app = %v, want \"test\"", matchLabels["app"])
	}
}

func TestSetSpec_NilObject(t *testing.T) {
	t.Parallel()

	// Should not panic
	SetSpec(nil, map[string]interface{}{"key": "value"})
}

func TestSetStatus(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	status := map[string]interface{}{
		"replicas":      int64(3),
		"readyReplicas": int64(3),
	}

	SetStatus(obj, status)

	got, err := GetStatus(obj)
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}
	if got == nil {
		t.Error("status is nil after SetStatus()")
	}
}

func TestSetLabels(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	labels := map[string]string{
		"app":     "test",
		"version": "v1",
	}

	SetLabels(obj, labels)

	got := obj.GetLabels()
	if len(got) != 2 {
		t.Errorf("len(labels) = %d, want 2", len(got))
	}
	if got["app"] != "test" {
		t.Errorf("labels[app] = %q, want \"test\"", got["app"])
	}
}

func TestSetAnnotations(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	annotations := map[string]string{
		"knodex.io/catalog": "true",
	}

	SetAnnotations(obj, annotations)

	got := obj.GetAnnotations()
	if got["knodex.io/catalog"] != "true" {
		t.Errorf("annotations[knodex.io/catalog] = %q, want \"true\"", got["knodex.io/catalog"])
	}
}

func TestMergeLabels(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	obj.SetLabels(map[string]string{"existing": "value"})

	MergeLabels(obj, map[string]string{"new": "label"})

	got := obj.GetLabels()
	if got["existing"] != "value" {
		t.Error("existing label was lost")
	}
	if got["new"] != "label" {
		t.Error("new label was not added")
	}
}

func TestMergeLabels_Overwrite(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	obj.SetLabels(map[string]string{"key": "old"})

	MergeLabels(obj, map[string]string{"key": "new"})

	got := obj.GetLabels()
	if got["key"] != "new" {
		t.Errorf("labels[key] = %q, want \"new\"", got["key"])
	}
}

func TestMergeAnnotations(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	obj.SetAnnotations(map[string]string{"existing": "value"})

	MergeAnnotations(obj, map[string]string{"new": "annotation"})

	got := obj.GetAnnotations()
	if got["existing"] != "value" {
		t.Error("existing annotation was lost")
	}
	if got["new"] != "annotation" {
		t.Error("new annotation was not added")
	}
}

func TestSetLabel(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	SetLabel(obj, "app", "test")

	got := obj.GetLabels()
	if got["app"] != "test" {
		t.Errorf("labels[app] = %q, want \"test\"", got["app"])
	}
}

func TestSetAnnotation(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	SetAnnotation(obj, "key", "value")

	got := obj.GetAnnotations()
	if got["key"] != "value" {
		t.Errorf("annotations[key] = %q, want \"value\"", got["key"])
	}
}

func TestRemoveLabel(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	obj.SetLabels(map[string]string{"app": "test", "other": "value"})

	RemoveLabel(obj, "app")

	got := obj.GetLabels()
	if _, exists := got["app"]; exists {
		t.Error("label 'app' still exists after removal")
	}
	if got["other"] != "value" {
		t.Error("other label was accidentally removed")
	}
}

func TestRemoveAnnotation(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	obj.SetAnnotations(map[string]string{"key": "value", "other": "value"})

	RemoveAnnotation(obj, "key")

	got := obj.GetAnnotations()
	if _, exists := got["key"]; exists {
		t.Error("annotation 'key' still exists after removal")
	}
	if got["other"] != "value" {
		t.Error("other annotation was accidentally removed")
	}
}

func TestAddFinalizer(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	AddFinalizer(obj, "finalizer.example.com")

	got := obj.GetFinalizers()
	if len(got) != 1 || got[0] != "finalizer.example.com" {
		t.Errorf("finalizers = %v, want [\"finalizer.example.com\"]", got)
	}

	// Adding same finalizer again should be idempotent
	AddFinalizer(obj, "finalizer.example.com")
	got = obj.GetFinalizers()
	if len(got) != 1 {
		t.Errorf("duplicate finalizer was added, len = %d, want 1", len(got))
	}
}

func TestRemoveFinalizer(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	obj.SetFinalizers([]string{"finalizer.example.com", "other.example.com"})

	RemoveFinalizer(obj, "finalizer.example.com")

	got := obj.GetFinalizers()
	if len(got) != 1 || got[0] != "other.example.com" {
		t.Errorf("finalizers = %v, want [\"other.example.com\"]", got)
	}
}

func TestSetNestedField(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	err := SetNestedField(obj, "value", "spec", "template", "name")
	if err != nil {
		t.Fatalf("SetNestedField() error: %v", err)
	}

	got, err := GetString(obj.Object, "spec", "template", "name")
	if err != nil {
		t.Fatalf("GetString() error: %v", err)
	}
	if got != "value" {
		t.Errorf("nested value = %q, want \"value\"", got)
	}
}

func TestSetNestedField_NilObject(t *testing.T) {
	t.Parallel()

	err := SetNestedField(nil, "value", "key")
	if err == nil {
		t.Error("expected error for nil object")
	}
}

func TestClone(t *testing.T) {
	t.Parallel()

	original := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	SetLabel(original, "app", "test")

	clone := Clone(original)

	if clone == nil {
		t.Fatal("Clone() returned nil")
	}
	if clone == original {
		t.Error("Clone() returned same object, want deep copy")
	}

	// Verify data was copied
	if GetName(clone) != "my-deployment" {
		t.Error("clone has wrong name")
	}

	// Modify original, verify clone is unaffected
	SetLabel(original, "app", "modified")
	cloneLabels := clone.GetLabels()
	if cloneLabels["app"] != "test" {
		t.Error("modifying original affected clone")
	}
}

func TestClone_Nil(t *testing.T) {
	t.Parallel()

	if Clone(nil) != nil {
		t.Error("Clone(nil) should return nil")
	}
}

func TestWithSpec(t *testing.T) {
	t.Parallel()

	original := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	SetSpec(original, map[string]interface{}{"replicas": int64(1)})

	modified := WithSpec(original, map[string]interface{}{"replicas": int64(3)})

	// Original should be unchanged
	origSpec, _ := GetSpec(original)
	if origSpec["replicas"] != int64(1) {
		t.Error("original was modified")
	}

	// Modified should have new spec
	modSpec, _ := GetSpec(modified)
	if modSpec["replicas"] != int64(3) {
		t.Error("modified has wrong spec")
	}
}

func TestWithLabels(t *testing.T) {
	t.Parallel()

	original := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	SetLabels(original, map[string]string{"app": "original"})

	modified := WithLabels(original, map[string]string{"app": "modified"})

	// Original should be unchanged
	if original.GetLabels()["app"] != "original" {
		t.Error("original was modified")
	}

	// Modified should have new labels
	if modified.GetLabels()["app"] != "modified" {
		t.Error("modified has wrong labels")
	}
}

func TestWithAnnotations(t *testing.T) {
	t.Parallel()

	original := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")
	SetAnnotations(original, map[string]string{"key": "original"})

	modified := WithAnnotations(original, map[string]string{"key": "modified"})

	// Original should be unchanged
	if original.GetAnnotations()["key"] != "original" {
		t.Error("original was modified")
	}

	// Modified should have new annotations
	if modified.GetAnnotations()["key"] != "modified" {
		t.Error("modified has wrong annotations")
	}
}

func TestSetNestedStringMap(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	err := SetNestedStringMap(obj, map[string]string{"key": "value"}, "metadata", "labels")
	if err != nil {
		t.Fatalf("SetNestedStringMap() error: %v", err)
	}

	labels := obj.GetLabels()
	if labels["key"] != "value" {
		t.Errorf("labels[key] = %q, want \"value\"", labels["key"])
	}
}

func TestSetNestedSlice(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	err := SetNestedSlice(obj, []interface{}{"a", "b", "c"}, "spec", "containers")
	if err != nil {
		t.Fatalf("SetNestedSlice() error: %v", err)
	}

	containers, _ := GetSlice(obj.Object, "spec", "containers")
	if len(containers) != 3 {
		t.Errorf("len(containers) = %d, want 3", len(containers))
	}
}

func TestSetFinalizers(t *testing.T) {
	t.Parallel()

	obj := NewUnstructured("apps/v1", "Deployment", "default", "my-deployment")

	SetFinalizers(obj, []string{"finalizer1", "finalizer2"})

	got := obj.GetFinalizers()
	if len(got) != 2 {
		t.Errorf("len(finalizers) = %d, want 2", len(got))
	}
}

func TestBuilderNilSafety(t *testing.T) {
	t.Parallel()

	var nilObj *unstructured.Unstructured

	// All these should not panic
	SetSpec(nilObj, map[string]interface{}{})
	SetStatus(nilObj, map[string]interface{}{})
	SetLabels(nilObj, map[string]string{})
	SetAnnotations(nilObj, map[string]string{})
	MergeLabels(nilObj, map[string]string{})
	MergeAnnotations(nilObj, map[string]string{})
	SetLabel(nilObj, "key", "value")
	SetAnnotation(nilObj, "key", "value")
	RemoveLabel(nilObj, "key")
	RemoveAnnotation(nilObj, "key")
	SetFinalizers(nilObj, []string{})
	AddFinalizer(nilObj, "finalizer")
	RemoveFinalizer(nilObj, "finalizer")
}
