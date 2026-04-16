// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		wantErr bool
		errType error
	}{
		{
			name:    "nil object",
			obj:     nil,
			wantErr: true,
			errType: ErrNilObject,
		},
		{
			name: "object with spec",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "object without spec",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
			wantErr: true,
			errType: ErrFieldNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetSpec(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil {
				if !isErrorType(err, tt.errType) {
					t.Errorf("GetSpec() error type = %T, want %T", err, tt.errType)
				}
			}
			if !tt.wantErr && got == nil {
				t.Error("GetSpec() returned nil spec for valid object")
			}
		})
	}
}

func TestGetSpecOrEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectEmpty bool
	}{
		{
			name:        "nil object",
			obj:         nil,
			expectEmpty: true,
		},
		{
			name: "object with spec",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			expectEmpty: false,
		},
		{
			name: "object without spec",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetSpecOrEmpty(tt.obj)
			if got == nil {
				t.Error("GetSpecOrEmpty() returned nil, want non-nil map")
			}
			if tt.expectEmpty && len(got) != 0 {
				t.Errorf("GetSpecOrEmpty() returned non-empty map, want empty")
			}
			if !tt.expectEmpty && len(got) == 0 {
				t.Errorf("GetSpecOrEmpty() returned empty map, want non-empty")
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		wantErr bool
		errType error
	}{
		{
			name:    "nil object",
			obj:     nil,
			wantErr: true,
			errType: ErrNilObject,
		},
		{
			name: "object with status",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "object without status",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			wantErr: true,
			errType: ErrFieldNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetStatus(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil {
				if !isErrorType(err, tt.errType) {
					t.Errorf("GetStatus() error type = %T, want %T", err, tt.errType)
				}
			}
			if !tt.wantErr && got == nil {
				t.Error("GetStatus() returned nil status for valid object")
			}
		})
	}
}

func TestGetStatusOrEmpty(t *testing.T) {
	t.Parallel()

	got := GetStatusOrEmpty(nil)
	if got == nil {
		t.Error("GetStatusOrEmpty(nil) returned nil, want non-nil map")
	}
	if len(got) != 0 {
		t.Errorf("GetStatusOrEmpty(nil) returned non-empty map, want empty")
	}
}

func TestGetSpecFieldString(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
				"nested": map[string]interface{}{
					"field": "value",
				},
			},
		},
	}

	tests := []struct {
		name     string
		path     []string
		expected string
		wantErr  bool
	}{
		{
			name:     "direct string field",
			path:     []string{"name"},
			expected: "my-app",
			wantErr:  false,
		},
		{
			name:     "nested string field",
			path:     []string{"nested", "field"},
			expected: "value",
			wantErr:  false,
		},
		{
			name:    "empty path",
			path:    []string{},
			wantErr: true,
		},
		{
			name:    "missing field",
			path:    []string{"nonexistent"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetSpecFieldString(obj, tt.path...)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpecFieldString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("GetSpecFieldString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGetSpecFieldString_NilObject(t *testing.T) {
	t.Parallel()

	_, err := GetSpecFieldString(nil, "field")
	if err == nil {
		t.Error("GetSpecFieldString(nil) should return error")
	}
}

func TestGetSpecFieldStringOrDefault(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"name": "my-app",
			},
		},
	}

	if got := GetSpecFieldStringOrDefault(obj, "default", "name"); got != "my-app" {
		t.Errorf("GetSpecFieldStringOrDefault() = %q, want 'my-app'", got)
	}

	if got := GetSpecFieldStringOrDefault(obj, "default", "missing"); got != "default" {
		t.Errorf("GetSpecFieldStringOrDefault() = %q, want 'default'", got)
	}

	if got := GetSpecFieldStringOrDefault(nil, "default", "field"); got != "default" {
		t.Errorf("GetSpecFieldStringOrDefault(nil) = %q, want 'default'", got)
	}
}

func TestGetStatusField(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase":    "Running",
				"replicas": int64(3),
			},
		},
	}

	// Empty path returns status
	got, err := GetStatusField(obj)
	if err != nil {
		t.Fatalf("GetStatusField() with empty path error: %v", err)
	}
	if got == nil {
		t.Error("GetStatusField() returned nil")
	}

	// Direct field
	got, err = GetStatusField(obj, "phase")
	if err != nil {
		t.Fatalf("GetStatusField() error: %v", err)
	}
	if got != "Running" {
		t.Errorf("phase = %v, want 'Running'", got)
	}
}

func TestGetStatusField_NilObject(t *testing.T) {
	t.Parallel()

	_, err := GetStatusField(nil, "field")
	if err == nil {
		t.Error("GetStatusField(nil) should return error")
	}
}

func TestGetStatusFieldString(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	got, err := GetStatusFieldString(obj, "phase")
	if err != nil {
		t.Fatalf("GetStatusFieldString() error: %v", err)
	}
	if got != "Running" {
		t.Errorf("phase = %q, want 'Running'", got)
	}
}

func TestGetStatusFieldString_EmptyPath(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{},
		},
	}

	_, err := GetStatusFieldString(obj)
	if err == nil {
		t.Error("GetStatusFieldString() with empty path should return error")
	}
}

func TestGetStatusFieldString_NilObject(t *testing.T) {
	t.Parallel()

	_, err := GetStatusFieldString(nil, "field")
	if err == nil {
		t.Error("GetStatusFieldString(nil) should return error")
	}
}

func TestGetStatusFieldStringOrDefault(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Running",
			},
		},
	}

	if got := GetStatusFieldStringOrDefault(obj, "Unknown", "phase"); got != "Running" {
		t.Errorf("GetStatusFieldStringOrDefault() = %q, want 'Running'", got)
	}

	if got := GetStatusFieldStringOrDefault(obj, "Unknown", "missing"); got != "Unknown" {
		t.Errorf("GetStatusFieldStringOrDefault() = %q, want 'Unknown'", got)
	}
}

func TestGetStatusFieldSlice(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{"type": "Ready", "status": "True"},
				},
			},
		},
	}

	got, err := GetStatusFieldSlice(obj, "conditions")
	if err != nil {
		t.Fatalf("GetStatusFieldSlice() error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len(conditions) = %d, want 1", len(got))
	}
}

func TestGetStatusFieldSlice_EmptyPath(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{},
		},
	}

	_, err := GetStatusFieldSlice(obj)
	if err == nil {
		t.Error("GetStatusFieldSlice() with empty path should return error")
	}
}

func TestGetStatusFieldSlice_NilObject(t *testing.T) {
	t.Parallel()

	_, err := GetStatusFieldSlice(nil, "field")
	if err == nil {
		t.Error("GetStatusFieldSlice(nil) should return error")
	}
}

func TestGetConditions(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Available",
						"status": "True",
					},
				},
			},
		},
	}

	got := GetConditions(obj)
	if len(got) != 2 {
		t.Errorf("GetConditions() returned %d conditions, want 2", len(got))
	}
}

func TestGetConditions_NoConditions(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{},
		},
	}

	got := GetConditions(obj)
	if got == nil {
		t.Error("GetConditions() returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("GetConditions() returned %d conditions, want 0", len(got))
	}
}

func TestGetConditions_NilObject(t *testing.T) {
	t.Parallel()

	got := GetConditions(nil)
	if got == nil {
		t.Error("GetConditions(nil) returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Errorf("GetConditions(nil) returned %d conditions, want 0", len(got))
	}
}

func TestGetCondition(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "MinimumReplicasAvailable",
						"message": "Deployment has minimum availability.",
					},
					map[string]interface{}{
						"type":   "Progressing",
						"status": "True",
					},
				},
			},
		},
	}

	got := GetCondition(obj, "Ready")
	if got == nil {
		t.Fatal("GetCondition() returned nil for existing condition")
	}
	if got["type"] != "Ready" {
		t.Errorf("condition type = %v, want 'Ready'", got["type"])
	}
	if got["status"] != "True" {
		t.Errorf("condition status = %v, want 'True'", got["status"])
	}
}

func TestGetCondition_NotFound(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}

	got := GetCondition(obj, "NonExistent")
	if got != nil {
		t.Errorf("GetCondition() = %v, want nil for non-existent condition", got)
	}
}

func TestGetConditionStatus(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		},
	}

	if got := GetConditionStatus(obj, "Ready"); got != "True" {
		t.Errorf("GetConditionStatus() = %q, want 'True'", got)
	}

	if got := GetConditionStatus(obj, "NonExistent"); got != "" {
		t.Errorf("GetConditionStatus() = %q, want empty string", got)
	}
}

func TestIsConditionTrue(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Failed",
						"status": "False",
					},
				},
			},
		},
	}

	if !IsConditionTrue(obj, "Ready") {
		t.Error("IsConditionTrue() = false for True condition")
	}
	if IsConditionTrue(obj, "Failed") {
		t.Error("IsConditionTrue() = true for False condition")
	}
	if IsConditionTrue(obj, "NonExistent") {
		t.Error("IsConditionTrue() = true for non-existent condition")
	}
}

// Helper function to check error type
func isErrorType(err error, target error) bool {
	if pathErr, ok := err.(*PathError); ok {
		return pathErr.Err == target
	}
	return err == target
}
