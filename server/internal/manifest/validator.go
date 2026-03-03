package manifest

import (
	"k8s.io/client-go/dynamic"
)

// CRDSchemaValidator validates manifests against their CRD schemas
type CRDSchemaValidator struct {
	dynamicClient dynamic.Interface
}

// NewCRDSchemaValidator creates a new schema validator
func NewCRDSchemaValidator(dynamicClient dynamic.Interface) *CRDSchemaValidator {
	return &CRDSchemaValidator{
		dynamicClient: dynamicClient,
	}
}

// Validate validates a manifest against its CRD schema
// This implements the SchemaValidator interface
func (v *CRDSchemaValidator) Validate(manifest map[string]interface{}, rgdName string) error {
	// For now, perform basic structural validation
	// Full CRD schema validation would require fetching the CRD definition
	// and validating against the OpenAPI schema

	// Validate required fields exist
	if manifest["apiVersion"] == nil {
		return &ValidationError{Field: "apiVersion", Message: "apiVersion is required"}
	}
	if manifest["kind"] == nil {
		return &ValidationError{Field: "kind", Message: "kind is required"}
	}
	if manifest["metadata"] == nil {
		return &ValidationError{Field: "metadata", Message: "metadata is required"}
	}
	if manifest["spec"] == nil {
		return &ValidationError{Field: "spec", Message: "spec is required"}
	}

	// Validate metadata structure
	metadata, ok := manifest["metadata"].(map[string]interface{})
	if !ok {
		return &ValidationError{Field: "metadata", Message: "metadata must be an object"}
	}
	if metadata["name"] == nil {
		return &ValidationError{Field: "metadata.name", Message: "metadata.name is required"}
	}
	if metadata["namespace"] == nil {
		return &ValidationError{Field: "metadata.namespace", Message: "metadata.namespace is required"}
	}

	// Validate spec is an object
	if _, ok := manifest["spec"].(map[string]interface{}); !ok {
		return &ValidationError{Field: "spec", Message: "spec must be an object"}
	}

	return nil
}
