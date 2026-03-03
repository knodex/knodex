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

func TestToYAMLWithHeader(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"key": "value",
	}

	tests := []struct {
		name       string
		header     string
		wantPrefix string
	}{
		{
			name:       "with header",
			header:     "# This is a comment\n",
			wantPrefix: "# This is a comment\n---\n",
		},
		{
			name:       "empty header",
			header:     "",
			wantPrefix: "---\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ToYAMLWithHeader(obj, tt.header)
			if err != nil {
				t.Fatalf("ToYAMLWithHeader() error: %v", err)
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("ToYAMLWithHeader() = %q, want prefix %q", got, tt.wantPrefix)
			}
		})
	}
}

func TestToJSON(t *testing.T) {
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
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ToJSON(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) == 0 {
				t.Error("ToJSON() returned empty bytes for valid input")
			}
		})
	}
}

func TestToJSONString(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"key": "value",
	}

	got, err := ToJSONString(obj)
	if err != nil {
		t.Fatalf("ToJSONString() error: %v", err)
	}

	if got != `{"key":"value"}` {
		t.Errorf("ToJSONString() = %q, want %q", got, `{"key":"value"}`)
	}
}

func TestToPrettyJSON(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"key": "value",
	}

	got, err := ToPrettyJSON(obj)
	if err != nil {
		t.Fatalf("ToPrettyJSON() error: %v", err)
	}

	// Pretty JSON should have newlines and indentation
	if !strings.Contains(string(got), "\n") {
		t.Error("ToPrettyJSON() should have newlines")
	}
	if !strings.Contains(string(got), "  ") {
		t.Error("ToPrettyJSON() should have indentation")
	}
}

func TestToPrettyJSON_NilObject(t *testing.T) {
	t.Parallel()

	_, err := ToPrettyJSON(nil)
	if err == nil {
		t.Error("ToPrettyJSON(nil) should return error")
	}
}

func TestToPrettyJSONString(t *testing.T) {
	t.Parallel()

	obj := map[string]interface{}{
		"key": "value",
	}

	got, err := ToPrettyJSONString(obj)
	if err != nil {
		t.Fatalf("ToPrettyJSONString() error: %v", err)
	}

	if !strings.Contains(got, "\n") {
		t.Error("ToPrettyJSONString() should have newlines")
	}
}

func TestFromYAML(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`)

	var result map[string]interface{}
	err := FromYAML(yamlData, &result)
	if err != nil {
		t.Fatalf("FromYAML() error: %v", err)
	}

	if result["apiVersion"] != "v1" {
		t.Errorf("apiVersion = %v, want v1", result["apiVersion"])
	}
	if result["kind"] != "ConfigMap" {
		t.Errorf("kind = %v, want ConfigMap", result["kind"])
	}
}

func TestFromYAML_NilTarget(t *testing.T) {
	t.Parallel()

	err := FromYAML([]byte("key: value"), nil)
	if err == nil {
		t.Error("FromYAML with nil target should return error")
	}
}

func TestFromYAML_InvalidData(t *testing.T) {
	t.Parallel()

	var result map[string]interface{}
	err := FromYAML([]byte(":::invalid:::yaml"), &result)
	if err == nil {
		t.Error("FromYAML with invalid YAML should return error")
	}
}

func TestFromYAMLString(t *testing.T) {
	t.Parallel()

	yamlStr := "key: value"

	var result map[string]interface{}
	err := FromYAMLString(yamlStr, &result)
	if err != nil {
		t.Fatalf("FromYAMLString() error: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("key = %v, want value", result["key"])
	}
}

func TestFromJSON(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{"apiVersion":"v1","kind":"ConfigMap"}`)

	var result map[string]interface{}
	err := FromJSON(jsonData, &result)
	if err != nil {
		t.Fatalf("FromJSON() error: %v", err)
	}

	if result["apiVersion"] != "v1" {
		t.Errorf("apiVersion = %v, want v1", result["apiVersion"])
	}
}

func TestFromJSON_NilTarget(t *testing.T) {
	t.Parallel()

	err := FromJSON([]byte(`{"key":"value"}`), nil)
	if err == nil {
		t.Error("FromJSON with nil target should return error")
	}
}

func TestFromJSON_InvalidData(t *testing.T) {
	t.Parallel()

	var result map[string]interface{}
	err := FromJSON([]byte("not json"), &result)
	if err == nil {
		t.Error("FromJSON with invalid JSON should return error")
	}
}

func TestFromJSONString(t *testing.T) {
	t.Parallel()

	jsonStr := `{"key":"value"}`

	var result map[string]interface{}
	err := FromJSONString(jsonStr, &result)
	if err != nil {
		t.Fatalf("FromJSONString() error: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("key = %v, want value", result["key"])
	}
}

func TestYAMLToMap(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
spec:
  replicas: 3
  template:
    name: test
`)

	result, err := YAMLToMap(yamlData)
	if err != nil {
		t.Fatalf("YAMLToMap() error: %v", err)
	}

	spec, ok := result["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec is not a map")
	}
	if spec["replicas"] != 3 {
		t.Errorf("replicas = %v, want 3", spec["replicas"])
	}
}

func TestYAMLToMap_InvalidData(t *testing.T) {
	t.Parallel()

	_, err := YAMLToMap([]byte(":::invalid"))
	if err == nil {
		t.Error("YAMLToMap with invalid YAML should return error")
	}
}

func TestYAMLStringToMap(t *testing.T) {
	t.Parallel()

	yamlStr := "key: value"

	result, err := YAMLStringToMap(yamlStr)
	if err != nil {
		t.Fatalf("YAMLStringToMap() error: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("key = %v, want value", result["key"])
	}
}

func TestJSONToMap(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{"spec":{"replicas":3}}`)

	result, err := JSONToMap(jsonData)
	if err != nil {
		t.Fatalf("JSONToMap() error: %v", err)
	}

	spec, ok := result["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec is not a map")
	}
	if spec["replicas"] != float64(3) { // JSON numbers are float64
		t.Errorf("replicas = %v, want 3", spec["replicas"])
	}
}

func TestJSONToMap_InvalidData(t *testing.T) {
	t.Parallel()

	_, err := JSONToMap([]byte("not json"))
	if err == nil {
		t.Error("JSONToMap with invalid JSON should return error")
	}
}

func TestJSONStringToMap(t *testing.T) {
	t.Parallel()

	jsonStr := `{"key":"value"}`

	result, err := JSONStringToMap(jsonStr)
	if err != nil {
		t.Fatalf("JSONStringToMap() error: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("key = %v, want value", result["key"])
	}
}

func TestMapToYAML(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"key": "value",
	}

	got, err := MapToYAML(m)
	if err != nil {
		t.Fatalf("MapToYAML() error: %v", err)
	}

	if !strings.Contains(string(got), "key: value") {
		t.Errorf("MapToYAML() = %q, want to contain 'key: value'", got)
	}
}

func TestMapToJSON(t *testing.T) {
	t.Parallel()

	m := map[string]interface{}{
		"key": "value",
	}

	got, err := MapToJSON(m)
	if err != nil {
		t.Fatalf("MapToJSON() error: %v", err)
	}

	if string(got) != `{"key":"value"}` {
		t.Errorf("MapToJSON() = %q, want %q", got, `{"key":"value"}`)
	}
}

func TestConvertYAMLToJSON(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
apiVersion: v1
kind: ConfigMap
`)

	jsonData, err := ConvertYAMLToJSON(yamlData)
	if err != nil {
		t.Fatalf("ConvertYAMLToJSON() error: %v", err)
	}

	// Parse back to verify
	result, err := JSONToMap(jsonData)
	if err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}

	if result["apiVersion"] != "v1" {
		t.Errorf("apiVersion = %v, want v1", result["apiVersion"])
	}
}

func TestConvertYAMLToJSON_InvalidData(t *testing.T) {
	t.Parallel()

	_, err := ConvertYAMLToJSON([]byte(":::invalid"))
	if err == nil {
		t.Error("ConvertYAMLToJSON with invalid YAML should return error")
	}
}

func TestConvertJSONToYAML(t *testing.T) {
	t.Parallel()

	jsonData := []byte(`{"apiVersion":"v1","kind":"ConfigMap"}`)

	yamlData, err := ConvertJSONToYAML(jsonData)
	if err != nil {
		t.Fatalf("ConvertJSONToYAML() error: %v", err)
	}

	// Parse back to verify
	result, err := YAMLToMap(yamlData)
	if err != nil {
		t.Fatalf("Failed to parse result YAML: %v", err)
	}

	if result["apiVersion"] != "v1" {
		t.Errorf("apiVersion = %v, want v1", result["apiVersion"])
	}
}

func TestConvertJSONToYAML_InvalidData(t *testing.T) {
	t.Parallel()

	_, err := ConvertJSONToYAML([]byte("not json"))
	if err == nil {
		t.Error("ConvertJSONToYAML with invalid JSON should return error")
	}
}

func TestRoundTrip_YAML(t *testing.T) {
	t.Parallel()

	original := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": 3,
		},
	}

	// Marshal to YAML
	yamlBytes, err := ToYAML(original)
	if err != nil {
		t.Fatalf("ToYAML() error: %v", err)
	}

	// Unmarshal back
	result, err := YAMLToMap(yamlBytes)
	if err != nil {
		t.Fatalf("YAMLToMap() error: %v", err)
	}

	// Verify key fields
	if result["apiVersion"] != "apps/v1" {
		t.Errorf("apiVersion = %v, want apps/v1", result["apiVersion"])
	}
	if result["kind"] != "Deployment" {
		t.Errorf("kind = %v, want Deployment", result["kind"])
	}
}

func TestRoundTrip_JSON(t *testing.T) {
	t.Parallel()

	original := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": float64(3), // JSON uses float64 for numbers
		},
	}

	// Marshal to JSON
	jsonBytes, err := ToJSON(original)
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Unmarshal back
	result, err := JSONToMap(jsonBytes)
	if err != nil {
		t.Fatalf("JSONToMap() error: %v", err)
	}

	// Verify key fields
	if result["apiVersion"] != "apps/v1" {
		t.Errorf("apiVersion = %v, want apps/v1", result["apiVersion"])
	}
	if result["kind"] != "Deployment" {
		t.Errorf("kind = %v, want Deployment", result["kind"])
	}
}
