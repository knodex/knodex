// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected string
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: "",
		},
		{
			name: "object with name",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "my-resource",
					},
				},
			},
			expected: "my-resource",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetName(tt.obj)
			if got != tt.expected {
				t.Errorf("GetName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected string
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: "",
		},
		{
			name: "object with namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expected: "default",
		},
		{
			name: "cluster-scoped object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "my-resource",
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetNamespace(tt.obj)
			if got != tt.expected {
				t.Errorf("GetNamespace() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectedLen int
	}{
		{
			name:        "nil object",
			obj:         nil,
			expectedLen: 0,
		},
		{
			name: "object with labels",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":     "test",
							"version": "v1",
						},
					},
				},
			},
			expectedLen: 2,
		},
		{
			name: "object without labels",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetLabels(tt.obj)
			if got == nil {
				t.Error("GetLabels() returned nil, want non-nil map")
			}
			if len(got) != tt.expectedLen {
				t.Errorf("GetLabels() returned %d labels, want %d", len(got), tt.expectedLen)
			}
		})
	}
}

func TestGetAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectedLen int
	}{
		{
			name:        "nil object",
			obj:         nil,
			expectedLen: 0,
		},
		{
			name: "object with annotations",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"knodex.io/catalog": "true",
						},
					},
				},
			},
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetAnnotations(tt.obj)
			if got == nil {
				t.Error("GetAnnotations() returned nil, want non-nil map")
			}
			if len(got) != tt.expectedLen {
				t.Errorf("GetAnnotations() returned %d annotations, want %d", len(got), tt.expectedLen)
			}
		})
	}
}

func TestGetAnnotation(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"knodex.io/catalog": "true",
				},
			},
		},
	}

	tests := []struct {
		name      string
		obj       *unstructured.Unstructured
		key       string
		wantValue string
		wantOK    bool
	}{
		{
			name:      "existing annotation",
			obj:       obj,
			key:       "knodex.io/catalog",
			wantValue: "true",
			wantOK:    true,
		},
		{
			name:      "missing annotation",
			obj:       obj,
			key:       "missing",
			wantValue: "",
			wantOK:    false,
		},
		{
			name:      "nil object",
			obj:       nil,
			key:       "any",
			wantValue: "",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			value, ok := GetAnnotation(tt.obj, tt.key)
			if ok != tt.wantOK {
				t.Errorf("GetAnnotation() ok = %v, want %v", ok, tt.wantOK)
			}
			if value != tt.wantValue {
				t.Errorf("GetAnnotation() value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestHasAnnotation(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"knodex.io/catalog": "true",
				},
			},
		},
	}

	if !HasAnnotation(obj, "knodex.io/catalog") {
		t.Error("HasAnnotation() = false, want true for existing annotation")
	}
	if HasAnnotation(obj, "missing") {
		t.Error("HasAnnotation() = true, want false for missing annotation")
	}
	if HasAnnotation(nil, "any") {
		t.Error("HasAnnotation() = true, want false for nil object")
	}
}

func TestGetLabel(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test",
				},
			},
		},
	}

	value, ok := GetLabel(obj, "app")
	if !ok || value != "test" {
		t.Errorf("GetLabel() = (%q, %v), want (\"test\", true)", value, ok)
	}

	value, ok = GetLabel(obj, "missing")
	if ok || value != "" {
		t.Errorf("GetLabel() = (%q, %v), want (\"\", false)", value, ok)
	}
}

func TestGetCreationTimestamp(t *testing.T) {
	t.Parallel()

	// K8s metav1.Time truncates to seconds, so we use a truncated time for comparison
	now := time.Now().Truncate(time.Second)
	obj := &unstructured.Unstructured{}
	obj.SetCreationTimestamp(metav1.NewTime(now))

	got := GetCreationTimestamp(obj)
	if !got.Equal(now) {
		t.Errorf("GetCreationTimestamp() = %v, want %v", got, now)
	}

	nilTime := GetCreationTimestamp(nil)
	if !nilTime.IsZero() {
		t.Error("GetCreationTimestamp(nil) should return zero time")
	}
}

func TestGetUID(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{}
	obj.SetUID(types.UID("test-uid-123"))

	got := GetUID(obj)
	if got != "test-uid-123" {
		t.Errorf("GetUID() = %q, want \"test-uid-123\"", got)
	}

	if GetUID(nil) != "" {
		t.Error("GetUID(nil) should return empty string")
	}
}

func TestGetResourceVersion(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{}
	obj.SetResourceVersion("12345")

	if got := GetResourceVersion(obj); got != "12345" {
		t.Errorf("GetResourceVersion() = %q, want \"12345\"", got)
	}
}

func TestGetAPIVersion(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}

	if got := GetAPIVersion(obj); got != "apps/v1" {
		t.Errorf("GetAPIVersion() = %q, want \"apps/v1\"", got)
	}
}

func TestGetKind(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}

	if got := GetKind(obj); got != "Deployment" {
		t.Errorf("GetKind() = %q, want \"Deployment\"", got)
	}
}

func TestGetFinalizers(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{}
	obj.SetFinalizers([]string{"finalizer.example.com"})

	got := GetFinalizers(obj)
	if len(got) != 1 || got[0] != "finalizer.example.com" {
		t.Errorf("GetFinalizers() = %v, want [\"finalizer.example.com\"]", got)
	}

	empty := GetFinalizers(nil)
	if len(empty) != 0 {
		t.Error("GetFinalizers(nil) should return empty slice")
	}
}

func TestHasFinalizer(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{}
	obj.SetFinalizers([]string{"finalizer.example.com"})

	if !HasFinalizer(obj, "finalizer.example.com") {
		t.Error("HasFinalizer() = false, want true for existing finalizer")
	}
	if HasFinalizer(obj, "missing") {
		t.Error("HasFinalizer() = true, want false for missing finalizer")
	}
}

func TestNamespacedName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected string
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: "",
		},
		{
			name: "namespaced object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "my-resource",
					},
				},
			},
			expected: "default/my-resource",
		},
		{
			name: "cluster-scoped object",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "my-cluster-resource",
					},
				},
			},
			expected: "my-cluster-resource",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NamespacedName(tt.obj)
			if got != tt.expected {
				t.Errorf("NamespacedName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsBeingDeleted(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	deletingObj := &unstructured.Unstructured{}
	deletingObj.SetDeletionTimestamp(&now)

	if !IsBeingDeleted(deletingObj) {
		t.Error("IsBeingDeleted() = false, want true for object with deletion timestamp")
	}

	normalObj := &unstructured.Unstructured{}
	if IsBeingDeleted(normalObj) {
		t.Error("IsBeingDeleted() = true, want false for normal object")
	}

	if IsBeingDeleted(nil) {
		t.Error("IsBeingDeleted(nil) = true, want false")
	}
}

func TestGetAnnotationOrDefault(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"existing": "value",
				},
			},
		},
	}

	if got := GetAnnotationOrDefault(obj, "existing", "default"); got != "value" {
		t.Errorf("GetAnnotationOrDefault() = %q, want \"value\"", got)
	}
	if got := GetAnnotationOrDefault(obj, "missing", "default"); got != "default" {
		t.Errorf("GetAnnotationOrDefault() = %q, want \"default\"", got)
	}
}

func TestGetLabelOrDefault(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test",
				},
			},
		},
	}

	if got := GetLabelOrDefault(obj, "app", "default"); got != "test" {
		t.Errorf("GetLabelOrDefault() = %q, want \"test\"", got)
	}
	if got := GetLabelOrDefault(obj, "missing", "default"); got != "default" {
		t.Errorf("GetLabelOrDefault() = %q, want \"default\"", got)
	}
}
