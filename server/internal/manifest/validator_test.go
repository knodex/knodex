// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package manifest

import (
	"strings"
	"testing"
)

func TestCRDSchemaValidator_Validate(t *testing.T) {
	validator := NewCRDSchemaValidator(nil)

	tests := []struct {
		name        string
		manifest    map[string]interface{}
		rgdName     string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid manifest passes validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name":      "test-instance",
					"namespace": "test-namespace",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
			rgdName: "test-rgd",
			wantErr: false,
		},
		{
			name: "missing apiVersion fails validation",
			manifest: map[string]interface{}{
				"kind": "TestResource",
				"metadata": map[string]interface{}{
					"name":      "test-instance",
					"namespace": "test-namespace",
				},
				"spec": map[string]interface{}{},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "apiVersion is required",
		},
		{
			name: "missing kind fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"metadata": map[string]interface{}{
					"name":      "test-instance",
					"namespace": "test-namespace",
				},
				"spec": map[string]interface{}{},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "kind is required",
		},
		{
			name: "missing metadata fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"spec":       map[string]interface{}{},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "metadata is required",
		},
		{
			name: "missing spec fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name":      "test-instance",
					"namespace": "test-namespace",
				},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "spec is required",
		},
		{
			name: "invalid metadata type fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"metadata":   "invalid",
				"spec":       map[string]interface{}{},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "metadata must be an object",
		},
		{
			name: "missing metadata.name fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"namespace": "test-namespace",
				},
				"spec": map[string]interface{}{},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "metadata.name is required",
		},
		{
			name: "missing metadata.namespace fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name": "test-instance",
				},
				"spec": map[string]interface{}{},
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "metadata.namespace is required",
		},
		{
			name: "invalid spec type fails validation",
			manifest: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name":      "test-instance",
					"namespace": "test-namespace",
				},
				"spec": "invalid",
			},
			rgdName:     "test-rgd",
			wantErr:     true,
			errContains: "spec must be an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.manifest, tt.rgdName)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "spec.replicas",
		Message: "must be positive",
	}

	expected := "spec.replicas: must be positive"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}
