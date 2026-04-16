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
