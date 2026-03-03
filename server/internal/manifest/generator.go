package manifest

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Generator generates Kubernetes manifests from instance specifications
//
// SECURITY NOTE: This is a pure manifest generation library. It does NOT perform
// authorization checks. The caller MUST verify that the user (spec.CreatedBy) has
// permission to create resources in the target namespace (spec.Namespace) BEFORE
// calling any generation methods. Authorization should be enforced at the API layer
// using Kubernetes RBAC, project membership checks, or other access control
// mechanisms appropriate for your deployment environment.
type Generator struct {
	// validator performs schema validation (optional)
	validator SchemaValidator
}

// SchemaValidator defines the interface for validating manifests against CRD schemas
type SchemaValidator interface {
	Validate(manifest map[string]interface{}, rgdName string) error
}

// NewGenerator creates a new manifest generator
func NewGenerator(validator SchemaValidator) *Generator {
	return &Generator{
		validator: validator,
	}
}

// GenerateManifest converts an instance specification to a Kubernetes YAML manifest.
//
// The generated manifest includes:
//   - apiVersion, kind from the RGD schema
//   - metadata with name, namespace, labels, and annotations
//   - Labels: app.kubernetes.io/name, knodex.io/deployment-mode
//   - Annotations: instance ID, creator, timestamp (knodex.io prefix)
//   - Spec section populated from the form data
//
// Note: KRO automatically sets kro.run/resource-graph-definition-name label.
// Note: app.kubernetes.io/managed-by is set by KRO or GitOps tool (ArgoCD/Flux).
//
// SECURITY REQUIREMENT: The caller MUST perform authorization checks before calling
// this method to ensure spec.CreatedBy has permission to create resources in
// spec.Namespace. This function performs input validation and manifest generation
// only - it does NOT check permissions or enforce access control.
func (g *Generator) GenerateManifest(spec *InstanceSpec) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("instance spec cannot be nil")
	}

	// Validate required fields
	if err := g.validateInstanceSpec(spec); err != nil {
		return "", fmt.Errorf("invalid instance spec: %w", err)
	}

	// Build the manifest structure
	// Note: KRO automatically sets "kro.run/resource-graph-definition-name" label on instances
	// Note: app.kubernetes.io/managed-by will be set by KRO or GitOps tool (ArgoCD/Flux)
	manifest := map[string]interface{}{
		"apiVersion": spec.APIVersion,
		"kind":       spec.Kind,
		"metadata": map[string]interface{}{
			"name":      spec.Name,
			"namespace": spec.Namespace,
			"labels": map[string]string{
				"app.kubernetes.io/name":    spec.Name,
				"knodex.io/deployment-mode": string(spec.DeploymentMode),
			},
			"annotations": map[string]string{
				"knodex.io/instance-id": spec.InstanceID,
				"knodex.io/created-by":  spec.CreatedBy,
				"knodex.io/created-at":  spec.CreatedAt.Format(time.RFC3339),
			},
		},
		"spec": spec.Spec,
	}

	// Add project label if present
	if spec.ProjectID != "" {
		labels := manifest["metadata"].(map[string]interface{})["labels"].(map[string]string)
		labels["knodex.io/project"] = spec.ProjectID
	}

	// Validate against CRD schema if validator is available
	if g.validator != nil {
		if err := g.validator.Validate(manifest, spec.RGDName); err != nil {
			return "", fmt.Errorf("schema validation failed: %w", err)
		}
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal manifest to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// GenerateMetadata creates the companion metadata file for an instance.
// The metadata file is stored at .metadata/{name}.yaml and includes:
// instanceId, rgdName, createdBy, deploymentMode, and other tracking fields.
func (g *Generator) GenerateMetadata(spec *InstanceSpec) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("instance spec cannot be nil")
	}

	// Validate required fields
	if err := g.validateInstanceSpec(spec); err != nil {
		return "", fmt.Errorf("invalid instance spec: %w", err)
	}

	metadata := map[string]interface{}{
		"instanceId":     spec.InstanceID,
		"name":           spec.Name,
		"namespace":      spec.Namespace,
		"rgdName":        spec.RGDName,
		"rgdVersion":     spec.RGDVersion,
		"rgdNamespace":   spec.RGDNamespace,
		"createdBy":      spec.CreatedBy,
		"createdAt":      spec.CreatedAt.Format(time.RFC3339),
		"deploymentMode": string(spec.DeploymentMode),
	}

	// Add optional fields
	if spec.ProjectID != "" {
		metadata["projectId"] = spec.ProjectID
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// Generate creates both manifest and metadata files and returns complete output.
// Validates manifests against CRD schema and produces properly formatted YAML.
func (g *Generator) Generate(spec *InstanceSpec) (*ManifestOutput, error) {
	// Generate manifest
	manifest, err := g.GenerateManifest(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to generate manifest: %w", err)
	}

	// Generate metadata
	metadata, err := g.GenerateMetadata(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to generate metadata: %w", err)
	}

	// Calculate file paths
	manifestPath, err := GenerateManifestPath(spec.Namespace, spec.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate manifest path: %w", err)
	}

	metadataPath, err := GenerateMetadataPath(spec.Namespace, spec.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to generate metadata path: %w", err)
	}

	return &ManifestOutput{
		Manifest:     manifest,
		Metadata:     metadata,
		ManifestPath: manifestPath,
		MetadataPath: metadataPath,
	}, nil
}

// GenerateManifestPath creates the file path for the manifest
// Format: instances/{namespace}/{name}.yaml
// Returns error if path contains traversal sequences or invalid characters
func GenerateManifestPath(namespace, name string) (string, error) {
	// Validate no path traversal sequences
	if strings.Contains(namespace, "..") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid path: contains directory traversal sequence")
	}
	if strings.Contains(namespace, "/") || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid path: contains path separator")
	}
	if strings.Contains(namespace, "\\") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid path: contains path separator")
	}

	return fmt.Sprintf("instances/%s/%s.yaml", namespace, name), nil
}

// GenerateMetadataPath creates the file path for the metadata
// Format: instances/{namespace}/.metadata/{name}.yaml
// Returns error if path contains traversal sequences or invalid characters
func GenerateMetadataPath(namespace, name string) (string, error) {
	// Validate no path traversal sequences
	if strings.Contains(namespace, "..") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid path: contains directory traversal sequence")
	}
	if strings.Contains(namespace, "/") || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid path: contains path separator")
	}
	if strings.Contains(namespace, "\\") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("invalid path: contains path separator")
	}

	return fmt.Sprintf("instances/%s/.metadata/%s.yaml", namespace, name), nil
}

// validateSpecMap recursively validates the spec map to prevent YAML injection and DoS attacks
// Checks for:
// - Excessive nesting depth (prevents stack overflow)
// - Excessive size (prevents memory exhaustion)
// - Malicious map keys (prevents YAML injection)
func validateSpecMap(spec map[string]interface{}, currentDepth, maxDepth int) error {
	const maxSpecSize = 1048576 // 1MB max size for spec

	// Check depth limit to prevent deeply nested structures
	if currentDepth > maxDepth {
		return &ValidationError{
			Field:   "spec",
			Message: fmt.Sprintf("spec nesting depth exceeds maximum of %d levels", maxDepth),
		}
	}

	for key, value := range spec {
		// Validate map keys to prevent YAML injection
		if strings.ContainsAny(key, "\n\r") {
			return &ValidationError{
				Field:   "spec",
				Message: fmt.Sprintf("spec contains key with newline: %q", key),
			}
		}

		// Check for YAML injection sequences in keys
		yamlSpecialChars := []string{": ", "- ", "| ", "> ", "{{", "}}", "${"}
		for _, special := range yamlSpecialChars {
			if strings.Contains(key, special) {
				return &ValidationError{
					Field:   "spec",
					Message: fmt.Sprintf("spec contains key with prohibited sequence %q: %q", special, key),
				}
			}
		}

		// Recursively validate nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if err := validateSpecMap(nestedMap, currentDepth+1, maxDepth); err != nil {
				return err
			}
		}

		// Recursively validate slices containing maps
		if slice, ok := value.([]interface{}); ok {
			for i, item := range slice {
				if nestedMap, ok := item.(map[string]interface{}); ok {
					if err := validateSpecMap(nestedMap, currentDepth+1, maxDepth); err != nil {
						return err
					}
				}
				// Validate string values in slices
				if str, ok := item.(string); ok {
					// Check for excessively long strings that could cause DoS
					if len(str) > 65536 { // 64KB max per string
						return &ValidationError{
							Field:   "spec",
							Message: fmt.Sprintf("spec contains excessively long string at index %d", i),
						}
					}
				}
			}
		}

		// Validate string values
		if str, ok := value.(string); ok {
			// Check for excessively long strings that could cause DoS
			if len(str) > 65536 { // 64KB max per string
				return &ValidationError{
					Field:   "spec",
					Message: fmt.Sprintf("spec key %q contains excessively long string", key),
				}
			}
		}
	}

	return nil
}

// validateKubernetesName validates that a string conforms to Kubernetes name requirements (RFC 1123 DNS subdomain)
// Names must:
// - Contain only lowercase alphanumeric characters or '-'
// - Start with an alphanumeric character
// - End with an alphanumeric character
// - Be 253 characters or less
// Note: This function assumes value is non-empty (empty check should be done separately)
func validateKubernetesName(fieldName, value string) error {

	// Check max length (RFC 1123 DNS subdomain)
	maxLength := 253
	if len(value) > maxLength {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("value exceeds maximum length of %d characters", maxLength),
		}
	}

	// Must start with alphanumeric
	if !regexp.MustCompile(`^[a-z0-9]`).MatchString(value) {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must start with a lowercase alphanumeric character",
		}
	}

	// Must end with alphanumeric
	if !regexp.MustCompile(`[a-z0-9]$`).MatchString(value) {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must end with a lowercase alphanumeric character",
		}
	}

	// Must contain only lowercase alphanumeric or '-'
	validPattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validPattern.MatchString(value) {
		return &ValidationError{
			Field:   fieldName,
			Message: "value must contain only lowercase alphanumeric characters or '-'",
		}
	}

	return nil
}

// validateLabelAnnotationValue validates that a string is safe for use in labels/annotations
// Prevents YAML injection and ensures compliance with Kubernetes label value constraints
func validateLabelAnnotationValue(fieldName, value string) error {
	// Check for empty value (some fields are required, validated separately)
	if value == "" {
		return nil // Empty check is handled by field-specific validation
	}

	// Max length for Kubernetes label/annotation values is 63 characters for labels
	// and 256KB for annotations, but we use a reasonable limit
	maxLength := 253
	if len(value) > maxLength {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("value exceeds maximum length of %d characters", maxLength),
		}
	}

	// Check for newline characters that could break YAML structure
	if strings.ContainsAny(value, "\n\r") {
		return &ValidationError{
			Field:   fieldName,
			Message: "value contains newline characters",
		}
	}

	// Check for YAML special characters that could enable injection
	// Prevent: quotes, colons followed by space, pipe, greater-than (YAML block scalars)
	yamlSpecialChars := []string{
		": ", // Key-value separator in YAML
		"- ", // List item indicator
		"| ", // Literal block scalar
		"> ", // Folded block scalar
		"{{", // Template injection
		"}}", // Template injection
		"${", // Variable substitution
	}
	for _, special := range yamlSpecialChars {
		if strings.Contains(value, special) {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value contains prohibited sequence: %q", special),
			}
		}
	}

	// Validate against Kubernetes label value format (RFC 1123 with extensions)
	// Pattern: alphanumeric, dash, underscore, dot (max 63 chars for label values)
	// For annotation values, we're more permissive but still block injection vectors
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._@-]*[a-zA-Z0-9])?$`)
	if !validPattern.MatchString(value) {
		return &ValidationError{
			Field:   fieldName,
			Message: "value contains invalid characters (must be alphanumeric, dash, underscore, dot, or @)",
		}
	}

	return nil
}

// validateInstanceSpec validates required fields in the instance spec
func (g *Generator) validateInstanceSpec(spec *InstanceSpec) error {
	// Validate required fields first
	if spec.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if spec.Namespace == "" {
		return &ValidationError{Field: "namespace", Message: "namespace is required"}
	}

	// Validate namespace conforms to Kubernetes naming requirements (RFC 1123)
	if err := validateKubernetesName("namespace", spec.Namespace); err != nil {
		return err
	}

	// Validate name conforms to Kubernetes naming requirements (RFC 1123)
	if err := validateKubernetesName("name", spec.Name); err != nil {
		return err
	}
	if spec.RGDName == "" {
		return &ValidationError{Field: "rgdName", Message: "rgdName is required"}
	}
	if spec.APIVersion == "" {
		return &ValidationError{Field: "apiVersion", Message: "apiVersion is required"}
	}
	if spec.Kind == "" {
		return &ValidationError{Field: "kind", Message: "kind is required"}
	}
	if spec.Spec == nil {
		return &ValidationError{Field: "spec", Message: "spec is required"}
	}
	if spec.CreatedBy == "" {
		return &ValidationError{Field: "createdBy", Message: "createdBy is required"}
	}
	if spec.InstanceID == "" {
		return &ValidationError{Field: "instanceId", Message: "instanceId is required"}
	}
	if spec.DeploymentMode == "" {
		spec.DeploymentMode = ModeDirect // Default to direct mode
	}

	// Validate label/annotation values to prevent injection attacks
	// These fields are inserted into Kubernetes labels/annotations
	if err := validateLabelAnnotationValue("name", spec.Name); err != nil {
		return err
	}
	if err := validateLabelAnnotationValue("rgdName", spec.RGDName); err != nil {
		return err
	}
	if err := validateLabelAnnotationValue("rgdNamespace", spec.RGDNamespace); err != nil {
		return err
	}
	if err := validateLabelAnnotationValue("instanceId", spec.InstanceID); err != nil {
		return err
	}
	if err := validateLabelAnnotationValue("createdBy", spec.CreatedBy); err != nil {
		return err
	}
	if spec.ProjectID != "" {
		if err := validateLabelAnnotationValue("projectId", spec.ProjectID); err != nil {
			return err
		}
	}

	// Validate spec map to prevent YAML injection and DoS attacks
	// Max depth of 10 levels should be sufficient for most Kubernetes CRDs
	const maxSpecDepth = 10
	if err := validateSpecMap(spec.Spec, 0, maxSpecDepth); err != nil {
		return err
	}

	return nil
}
