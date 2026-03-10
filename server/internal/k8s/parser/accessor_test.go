// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"errors"
	"testing"
)

func TestGetString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         map[string]interface{}
		path        []string
		expected    string
		expectError error
	}{
		{
			name:     "simple path",
			obj:      map[string]interface{}{"key": "value"},
			path:     []string{"key"},
			expected: "value",
		},
		{
			name: "nested path",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1beta1",
					},
				},
			},
			path:     []string{"spec", "schema", "apiVersion"},
			expected: "v1beta1",
		},
		{
			name:        "nil object",
			obj:         nil,
			path:        []string{"key"},
			expectError: ErrNilObject,
		},
		{
			name:        "empty path",
			obj:         map[string]interface{}{"key": "value"},
			path:        []string{},
			expectError: ErrEmptyPath,
		},
		{
			name:        "field not found",
			obj:         map[string]interface{}{"other": "value"},
			path:        []string{"key"},
			expectError: ErrFieldNotFound,
		},
		{
			name:        "type mismatch",
			obj:         map[string]interface{}{"key": 123},
			path:        []string{"key"},
			expectError: ErrTypeMismatch,
		},
		{
			name: "intermediate path not found",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			path:        []string{"spec", "schema", "apiVersion"},
			expectError: ErrFieldNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetString(tt.obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGetStringOrDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		obj        map[string]interface{}
		defaultVal string
		path       []string
		expected   string
	}{
		{
			name:       "value exists",
			obj:        map[string]interface{}{"key": "value"},
			defaultVal: "default",
			path:       []string{"key"},
			expected:   "value",
		},
		{
			name:       "value missing",
			obj:        map[string]interface{}{},
			defaultVal: "default",
			path:       []string{"key"},
			expected:   "default",
		},
		{
			name:       "wrong type",
			obj:        map[string]interface{}{"key": 123},
			defaultVal: "default",
			path:       []string{"key"},
			expected:   "default",
		},
		{
			name:       "nil object",
			obj:        nil,
			defaultVal: "default",
			path:       []string{"key"},
			expected:   "default",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := GetStringOrDefault(tt.obj, tt.defaultVal, tt.path...)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGetMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         map[string]interface{}
		path        []string
		expectError error
	}{
		{
			name: "simple path",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{"key": "value"},
			},
			path: []string{"spec"},
		},
		{
			name: "nested path",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{"name": "test"},
				},
			},
			path: []string{"spec", "template"},
		},
		{
			name:        "nil object",
			obj:         nil,
			path:        []string{"spec"},
			expectError: ErrNilObject,
		},
		{
			name:        "field not found",
			obj:         map[string]interface{}{},
			path:        []string{"spec"},
			expectError: ErrFieldNotFound,
		},
		{
			name:        "type mismatch",
			obj:         map[string]interface{}{"spec": "not a map"},
			path:        []string{"spec"},
			expectError: ErrTypeMismatch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetMap(tt.obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Error("expected non-nil map")
			}
		})
	}
}

func TestGetSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         map[string]interface{}
		path        []string
		expected    int
		expectError error
	}{
		{
			name: "simple path",
			obj: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
			},
			path:     []string{"items"},
			expected: 3,
		},
		{
			name: "nested path",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"resources": []interface{}{1, 2},
				},
			},
			path:     []string{"spec", "resources"},
			expected: 2,
		},
		{
			name:        "nil object",
			obj:         nil,
			path:        []string{"items"},
			expectError: ErrNilObject,
		},
		{
			name:        "type mismatch",
			obj:         map[string]interface{}{"items": "not a slice"},
			path:        []string{"items"},
			expectError: ErrTypeMismatch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetSlice(tt.obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.expected {
				t.Errorf("expected slice length %d, got %d", tt.expected, len(got))
			}
		})
	}
}

func TestGetInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         map[string]interface{}
		path        []string
		expected    int64
		expectError error
	}{
		{
			name:     "int64 value",
			obj:      map[string]interface{}{"count": int64(42)},
			path:     []string{"count"},
			expected: 42,
		},
		{
			name:     "int value",
			obj:      map[string]interface{}{"count": 42},
			path:     []string{"count"},
			expected: 42,
		},
		{
			name:     "float64 value (JSON number)",
			obj:      map[string]interface{}{"count": float64(42)},
			path:     []string{"count"},
			expected: 42,
		},
		{
			name:        "string value",
			obj:         map[string]interface{}{"count": "42"},
			path:        []string{"count"},
			expectError: ErrTypeMismatch,
		},
		{
			name:        "nil object",
			obj:         nil,
			path:        []string{"count"},
			expectError: ErrNilObject,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetInt64(tt.obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, got)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         map[string]interface{}
		path        []string
		expected    bool
		expectError error
	}{
		{
			name:     "true value",
			obj:      map[string]interface{}{"enabled": true},
			path:     []string{"enabled"},
			expected: true,
		},
		{
			name:     "false value",
			obj:      map[string]interface{}{"enabled": false},
			path:     []string{"enabled"},
			expected: false,
		},
		{
			name:        "string value",
			obj:         map[string]interface{}{"enabled": "true"},
			path:        []string{"enabled"},
			expectError: ErrTypeMismatch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetBool(tt.obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGetFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         map[string]interface{}
		path        []string
		expected    float64
		expectError error
	}{
		{
			name:     "float64 value",
			obj:      map[string]interface{}{"ratio": 3.14},
			path:     []string{"ratio"},
			expected: 3.14,
		},
		{
			name:     "int value",
			obj:      map[string]interface{}{"ratio": 42},
			path:     []string{"ratio"},
			expected: 42.0,
		},
		{
			name:        "string value",
			obj:         map[string]interface{}{"ratio": "3.14"},
			path:        []string{"ratio"},
			expectError: ErrTypeMismatch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetFloat64(tt.obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, got)
			}
		})
	}
}

func TestHasField(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"name": "test",
			},
		},
	}

	tests := []struct {
		name     string
		path     []string
		expected bool
	}{
		{"existing field", []string{"spec"}, true},
		{"nested field", []string{"spec", "template", "name"}, true},
		{"missing field", []string{"status"}, false},
		{"partial path missing", []string{"spec", "other"}, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := HasField(obj, tt.path...)
			if got != tt.expected {
				t.Errorf("HasField() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSetNestedValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initial     map[string]interface{}
		value       interface{}
		path        []string
		expectError error
		verify      func(t *testing.T, obj map[string]interface{})
	}{
		{
			name:    "set simple value",
			initial: map[string]interface{}{},
			value:   "test",
			path:    []string{"key"},
			verify: func(t *testing.T, obj map[string]interface{}) {
				if obj["key"] != "test" {
					t.Errorf("expected key to be 'test', got %v", obj["key"])
				}
			},
		},
		{
			name:    "create nested path",
			initial: map[string]interface{}{},
			value:   "value",
			path:    []string{"spec", "template", "name"},
			verify: func(t *testing.T, obj map[string]interface{}) {
				val, err := GetString(obj, "spec", "template", "name")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if val != "value" {
					t.Errorf("expected 'value', got %q", val)
				}
			},
		},
		{
			name:        "nil object",
			initial:     nil,
			value:       "test",
			path:        []string{"key"},
			expectError: ErrNilObject,
		},
		{
			name:        "empty path",
			initial:     map[string]interface{}{},
			value:       "test",
			path:        []string{},
			expectError: ErrEmptyPath,
		},
		{
			name:        "intermediate is not a map",
			initial:     map[string]interface{}{"spec": "string"},
			value:       "value",
			path:        []string{"spec", "template"},
			expectError: ErrTypeMismatch,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := SetNestedValue(tt.initial, tt.value, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.verify != nil {
				tt.verify(t, tt.initial)
			}
		})
	}
}

func TestDeleteNestedValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		initial  map[string]interface{}
		path     []string
		expected bool
	}{
		{
			name: "delete existing",
			initial: map[string]interface{}{
				"spec": map[string]interface{}{"key": "value"},
			},
			path:     []string{"spec", "key"},
			expected: true,
		},
		{
			name:     "delete missing",
			initial:  map[string]interface{}{},
			path:     []string{"spec"},
			expected: false,
		},
		{
			name:     "nil object",
			initial:  nil,
			path:     []string{"key"},
			expected: false,
		},
		{
			name:     "empty path",
			initial:  map[string]interface{}{"key": "value"},
			path:     []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := DeleteNestedValue(tt.initial, tt.path...)
			if got != tt.expected {
				t.Errorf("DeleteNestedValue() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"string": "hello",
		"number": 42,
		"nested": map[string]interface{}{
			"value": true,
		},
	}

	tests := []struct {
		name        string
		path        []string
		expectError error
	}{
		{"string value", []string{"string"}, nil},
		{"number value", []string{"number"}, nil},
		{"nested value", []string{"nested", "value"}, nil},
		{"missing value", []string{"missing"}, ErrFieldNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := GetValue(obj, tt.path...)

			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetMapOrDefault(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"spec": map[string]interface{}{"key": "value"},
	}
	emptyDefault := make(map[string]interface{})

	// Existing map
	got := GetMapOrDefault(obj, emptyDefault, "spec")
	if got == nil || got["key"] != "value" {
		t.Errorf("GetMapOrDefault() for existing map failed")
	}

	// Missing map returns default
	got = GetMapOrDefault(obj, emptyDefault, "missing")
	if got == nil {
		t.Error("GetMapOrDefault() returned nil for missing map")
	}
	if len(got) != 0 {
		t.Error("GetMapOrDefault() returned non-empty map for missing key")
	}

	// Nil object returns default
	got = GetMapOrDefault(nil, emptyDefault, "key")
	if got == nil {
		t.Error("GetMapOrDefault(nil) returned nil")
	}
}

func TestGetSliceOrDefault(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}
	emptyDefault := make([]interface{}, 0)

	// Existing slice
	got := GetSliceOrDefault(obj, emptyDefault, "items")
	if len(got) != 3 {
		t.Errorf("GetSliceOrDefault() len = %d, want 3", len(got))
	}

	// Missing slice returns default
	got = GetSliceOrDefault(obj, emptyDefault, "missing")
	if got == nil {
		t.Error("GetSliceOrDefault() returned nil for missing slice")
	}
	if len(got) != 0 {
		t.Error("GetSliceOrDefault() returned non-empty slice for missing key")
	}

	// Nil object returns default
	got = GetSliceOrDefault(nil, emptyDefault, "key")
	if got == nil {
		t.Error("GetSliceOrDefault(nil) returned nil")
	}
}

func TestGetInt64OrDefault(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"count": int64(42),
	}

	// Existing value
	if got := GetInt64OrDefault(obj, 0, "count"); got != 42 {
		t.Errorf("GetInt64OrDefault() = %d, want 42", got)
	}

	// Missing value returns default
	if got := GetInt64OrDefault(obj, 99, "missing"); got != 99 {
		t.Errorf("GetInt64OrDefault() = %d, want 99", got)
	}

	// Nil object returns default
	if got := GetInt64OrDefault(nil, 100, "key"); got != 100 {
		t.Errorf("GetInt64OrDefault(nil) = %d, want 100", got)
	}
}

func TestGetBoolOrDefault(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"enabled": true,
	}

	// Existing value
	if got := GetBoolOrDefault(obj, false, "enabled"); got != true {
		t.Errorf("GetBoolOrDefault() = %v, want true", got)
	}

	// Missing value returns default
	if got := GetBoolOrDefault(obj, true, "missing"); got != true {
		t.Errorf("GetBoolOrDefault() = %v, want true", got)
	}

	// Nil object returns default
	if got := GetBoolOrDefault(nil, false, "key"); got != false {
		t.Errorf("GetBoolOrDefault(nil) = %v, want false", got)
	}
}

func TestGetFloat64OrDefault(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"ratio": 3.14,
	}

	// Existing value
	if got := GetFloat64OrDefault(obj, 0.0, "ratio"); got != 3.14 {
		t.Errorf("GetFloat64OrDefault() = %f, want 3.14", got)
	}

	// Missing value returns default
	if got := GetFloat64OrDefault(obj, 1.0, "missing"); got != 1.0 {
		t.Errorf("GetFloat64OrDefault() = %f, want 1.0", got)
	}

	// Nil object returns default
	if got := GetFloat64OrDefault(nil, 2.0, "key"); got != 2.0 {
		t.Errorf("GetFloat64OrDefault(nil) = %f, want 2.0", got)
	}
}

func TestGetInt64_Int32Value(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"count": int32(42),
	}

	got, err := GetInt64(obj, "count")
	if err != nil {
		t.Fatalf("GetInt64() error: %v", err)
	}
	if got != 42 {
		t.Errorf("GetInt64() = %d, want 42", got)
	}
}

func TestGetFloat64_Int64Value(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"value": int64(42),
	}

	got, err := GetFloat64(obj, "value")
	if err != nil {
		t.Fatalf("GetFloat64() error: %v", err)
	}
	if got != 42.0 {
		t.Errorf("GetFloat64() = %f, want 42.0", got)
	}
}

func TestDeleteNestedValue_IntermediateNotMap(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"spec": "string",
	}

	got := DeleteNestedValue(obj, "spec", "key")
	if got {
		t.Error("DeleteNestedValue() = true for path with non-map intermediate")
	}
}

func TestGetValue_NilObject(t *testing.T) {
	t.Parallel()

	_, err := GetValue(nil, "key")
	if err == nil {
		t.Error("GetValue(nil) should return error")
	}
	if !errors.Is(err, ErrNilObject) {
		t.Errorf("GetValue(nil) error = %v, want ErrNilObject", err)
	}
}

func TestGetValue_EmptyPath(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{"key": "value"}
	_, err := GetValue(obj)
	if err == nil {
		t.Error("GetValue() with empty path should return error")
	}
	if !errors.Is(err, ErrEmptyPath) {
		t.Errorf("GetValue() error = %v, want ErrEmptyPath", err)
	}
}
