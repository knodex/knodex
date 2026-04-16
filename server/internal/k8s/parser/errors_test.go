// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"errors"
	"testing"
)

func TestPathError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *PathError
		contains []string
	}{
		{
			name: "field not found",
			err: &PathError{
				Op:   "GetString",
				Path: []string{"spec", "template"},
				Err:  ErrFieldNotFound,
			},
			contains: []string{"GetString", "spec.template", "not found"},
		},
		{
			name: "type mismatch",
			err: &PathError{
				Op:           "GetString",
				Path:         []string{"spec", "value"},
				ExpectedType: "string",
				ActualType:   "int",
				Err:          ErrTypeMismatch,
			},
			contains: []string{"GetString", "spec.value", "expected string", "got int"},
		},
		{
			name: "nil object",
			err: &PathError{
				Op:  "GetString",
				Err: ErrNilObject,
			},
			contains: []string{"GetString", "nil object"},
		},
		{
			name: "empty path",
			err: &PathError{
				Op:  "GetString",
				Err: ErrEmptyPath,
			},
			contains: []string{"GetString", "empty path"},
		},
		{
			name: "root path",
			err: &PathError{
				Op:   "GetString",
				Path: []string{},
				Err:  ErrFieldNotFound,
			},
			contains: []string{"(root)"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errStr := tt.err.Error()
			for _, s := range tt.contains {
				if !containsString(errStr, s) {
					t.Errorf("expected error to contain %q, got %q", s, errStr)
				}
			}
		})
	}
}

func TestPathError_Unwrap(t *testing.T) {
	t.Parallel()

	pathErr := &PathError{
		Op:   "GetString",
		Path: []string{"spec"},
		Err:  ErrFieldNotFound,
	}

	if !errors.Is(pathErr, ErrFieldNotFound) {
		t.Error("expected PathError to wrap ErrFieldNotFound")
	}
}

func TestPathError_Is(t *testing.T) {
	t.Parallel()

	pathErr := &PathError{
		Op:   "GetString",
		Path: []string{"spec"},
		Err:  ErrTypeMismatch,
	}

	if !pathErr.Is(ErrTypeMismatch) {
		t.Error("expected PathError.Is to match ErrTypeMismatch")
	}
	if pathErr.Is(ErrFieldNotFound) {
		t.Error("expected PathError.Is to not match ErrFieldNotFound")
	}
}

func TestTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"nil", nil, "nil"},
		{"string", "hello", "string"},
		{"int", 42, "int"},
		{"map", map[string]interface{}{}, "map[string]interface {}"},
		{"slice", []interface{}{}, "[]interface {}"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := typeName(tt.value); got != tt.expected {
				t.Errorf("typeName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
