package parser

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ToYAML marshals an object to YAML bytes.
// Returns an error if the object cannot be marshaled.
func ToYAML(obj interface{}) ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot marshal nil to YAML")
	}
	return yaml.Marshal(obj)
}

// ToYAMLString marshals an object to a YAML string.
// Returns an error if the object cannot be marshaled.
func ToYAMLString(obj interface{}) (string, error) {
	bytes, err := ToYAML(obj)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToYAMLWithHeader marshals an object to a YAML string with a header comment.
// The header is prepended to the YAML output, followed by "---" separator.
func ToYAMLWithHeader(obj interface{}, header string) (string, error) {
	yamlStr, err := ToYAMLString(obj)
	if err != nil {
		return "", err
	}

	if header != "" {
		return header + "---\n" + yamlStr, nil
	}
	return "---\n" + yamlStr, nil
}

// ToJSON marshals an object to compact JSON bytes.
// Returns an error if the object cannot be marshaled.
func ToJSON(obj interface{}) ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot marshal nil to JSON")
	}
	return json.Marshal(obj)
}

// ToJSONString marshals an object to a compact JSON string.
// Returns an error if the object cannot be marshaled.
func ToJSONString(obj interface{}) (string, error) {
	bytes, err := ToJSON(obj)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToPrettyJSON marshals an object to indented JSON bytes for readability.
// Uses 2-space indentation.
// Returns an error if the object cannot be marshaled.
func ToPrettyJSON(obj interface{}) ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot marshal nil to JSON")
	}
	return json.MarshalIndent(obj, "", "  ")
}

// ToPrettyJSONString marshals an object to an indented JSON string for readability.
// Returns an error if the object cannot be marshaled.
func ToPrettyJSONString(obj interface{}) (string, error) {
	bytes, err := ToPrettyJSON(obj)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// FromYAML unmarshals YAML data into the provided object.
// The object must be a pointer.
// Returns an error if the data cannot be unmarshaled.
func FromYAML(data []byte, obj interface{}) error {
	if obj == nil {
		return fmt.Errorf("cannot unmarshal into nil")
	}
	return yaml.Unmarshal(data, obj)
}

// FromYAMLString unmarshals a YAML string into the provided object.
// The object must be a pointer.
// Returns an error if the data cannot be unmarshaled.
func FromYAMLString(data string, obj interface{}) error {
	return FromYAML([]byte(data), obj)
}

// FromJSON unmarshals JSON data into the provided object.
// The object must be a pointer.
// Returns an error if the data cannot be unmarshaled.
func FromJSON(data []byte, obj interface{}) error {
	if obj == nil {
		return fmt.Errorf("cannot unmarshal into nil")
	}
	return json.Unmarshal(data, obj)
}

// FromJSONString unmarshals a JSON string into the provided object.
// The object must be a pointer.
// Returns an error if the data cannot be unmarshaled.
func FromJSONString(data string, obj interface{}) error {
	return FromJSON([]byte(data), obj)
}

// YAMLToMap unmarshals YAML data into a map[string]interface{}.
// Useful for working with unstructured Kubernetes objects.
func YAMLToMap(data []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := FromYAML(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// YAMLStringToMap unmarshals a YAML string into a map[string]interface{}.
func YAMLStringToMap(data string) (map[string]interface{}, error) {
	return YAMLToMap([]byte(data))
}

// JSONToMap unmarshals JSON data into a map[string]interface{}.
// Useful for working with unstructured Kubernetes objects.
func JSONToMap(data []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := FromJSON(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// JSONStringToMap unmarshals a JSON string into a map[string]interface{}.
func JSONStringToMap(data string) (map[string]interface{}, error) {
	return JSONToMap([]byte(data))
}

// MapToYAML converts a map to YAML bytes.
// This is a convenience function for ToYAML with a map argument.
func MapToYAML(m map[string]interface{}) ([]byte, error) {
	return ToYAML(m)
}

// MapToJSON converts a map to JSON bytes.
// This is a convenience function for ToJSON with a map argument.
func MapToJSON(m map[string]interface{}) ([]byte, error) {
	return ToJSON(m)
}

// ConvertYAMLToJSON converts YAML data to JSON bytes.
// Useful for API requests that require JSON.
func ConvertYAMLToJSON(yamlData []byte) ([]byte, error) {
	m, err := YAMLToMap(yamlData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return ToJSON(m)
}

// ConvertJSONToYAML converts JSON data to YAML bytes.
// Useful for displaying or storing data in YAML format.
func ConvertJSONToYAML(jsonData []byte) ([]byte, error) {
	m, err := JSONToMap(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return ToYAML(m)
}
