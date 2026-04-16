// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"strings"
	"testing"
)

func TestToYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		obj     interface{}
		wantErr bool
	}{
		{
			name:    "nil object",
			obj:     nil,
			wantErr: true,
		},
		{
			name: "simple map",
			obj: map[string]interface{}{
				"key": "value",
			},
			wantErr: false,
		},
		{
			name: "nested map",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ToYAML(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) == 0 {
				t.Error("ToYAML() returned empty bytes for valid input")
			}
		})
	}
}

func TestToYAMLString(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
	}

	got, err := ToYAMLString(obj)
	if err != nil {
		t.Fatalf("ToYAMLString() error: %v", err)
	}

	if !strings.Contains(got, "apiVersion: v1") {
		t.Error("ToYAMLString() missing expected content")
	}
	if !strings.Contains(got, "kind: ConfigMap") {
		t.Error("ToYAMLString() missing expected content")
	}
}

func TestToYAMLString_NilObject(t *testing.T) {
	t.Parallel()

	_, err := ToYAMLString(nil)
	if err == nil {
		t.Error("ToYAMLString(nil) should return error")
	}
}
