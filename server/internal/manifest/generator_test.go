package manifest

import (
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// mockValidator is a mock implementation of SchemaValidator for testing
type mockValidator struct {
	validateFunc func(manifest map[string]interface{}, rgdName string) error
}

func (m *mockValidator) Validate(manifest map[string]interface{}, rgdName string) error {
	if m.validateFunc != nil {
		return m.validateFunc(manifest, rgdName)
	}
	return nil
}

func TestGenerateManifest(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		spec        *InstanceSpec
		wantErr     bool
		errContains string
		validate    func(t *testing.T, manifest string)
	}{
		{
			name: "valid instance spec generates correct manifest",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				RGDVersion:     "1.0.0",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{"replicas": 3, "storage": "10Gi"},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				DeploymentMode: ModeDirect,
				InstanceID:     "inst-123",
				ProjectID:      "proj-456",
			},
			wantErr: false,
			validate: func(t *testing.T, manifest string) {
				// Parse YAML
				var obj map[string]interface{}
				if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
					t.Fatalf("Failed to parse manifest YAML: %v", err)
				}

				// Validate apiVersion
				if obj["apiVersion"] != "example.com/v1" {
					t.Errorf("Expected apiVersion example.com/v1, got %v", obj["apiVersion"])
				}

				// Validate kind
				if obj["kind"] != "TestResource" {
					t.Errorf("Expected kind TestResource, got %v", obj["kind"])
				}

				// Validate metadata
				metadata, ok := obj["metadata"].(map[string]interface{})
				if !ok {
					t.Fatal("metadata is not a map")
				}

				if metadata["name"] != "test-instance" {
					t.Errorf("Expected name test-instance, got %v", metadata["name"])
				}
				if metadata["namespace"] != "test-namespace" {
					t.Errorf("Expected namespace test-namespace, got %v", metadata["namespace"])
				}

				// Validate labels
				labels, ok := metadata["labels"].(map[string]interface{})
				if !ok {
					t.Fatal("labels is not a map")
				}

				// Note: KRO automatically sets kro.run/resource-graph-definition-name label
				// Note: app.kubernetes.io/managed-by is set by KRO or GitOps tool (ArgoCD/Flux)
				expectedLabels := map[string]string{
					"app.kubernetes.io/name":    "test-instance",
					"knodex.io/deployment-mode": "direct",
					"knodex.io/project":         "proj-456",
				}

				for key, expected := range expectedLabels {
					if labels[key] != expected {
						t.Errorf("Expected label %s=%s, got %v", key, expected, labels[key])
					}
				}

				// Validate annotations - all dashboard-specific metadata uses knodex.io prefix
				annotations, ok := metadata["annotations"].(map[string]interface{})
				if !ok {
					t.Fatal("annotations is not a map")
				}

				if annotations["knodex.io/instance-id"] != "inst-123" {
					t.Errorf("Expected instance ID inst-123, got %v", annotations["knodex.io/instance-id"])
				}
				if annotations["knodex.io/created-by"] != "user@example.com" {
					t.Errorf("Expected creator user@example.com, got %v", annotations["knodex.io/created-by"])
				}

				// Validate created-at timestamp
				createdAt, ok := annotations["knodex.io/created-at"].(string)
				if !ok || createdAt == "" {
					t.Error("Expected created-at annotation to be a non-empty string")
				}

				// Validate spec
				spec, ok := obj["spec"].(map[string]interface{})
				if !ok {
					t.Fatal("spec is not a map")
				}

				if spec["replicas"] != 3 {
					t.Errorf("Expected replicas 3, got %v", spec["replicas"])
				}
				if spec["storage"] != "10Gi" {
					t.Errorf("Expected storage 10Gi, got %v", spec["storage"])
				}
			},
		},
		{
			name:        "nil spec returns error",
			spec:        nil,
			wantErr:     true,
			errContains: "cannot be nil",
		},
		{
			name: "missing name returns error",
			spec: &InstanceSpec{
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "missing namespace returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				RGDName:        "test-rgd",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "namespace is required",
		},
		{
			name: "missing rgdName returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "rgdName is required",
		},
		{
			name: "missing apiVersion returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "apiVersion is required",
		},
		{
			name: "missing kind returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				APIVersion:     "example.com/v1",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "kind is required",
		},
		{
			name: "missing spec returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "spec is required",
		},
		{
			name: "missing createdBy returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "createdBy is required",
		},
		{
			name: "missing instanceID returns error",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				DeploymentMode: ModeDirect,
			},
			wantErr:     true,
			errContains: "instanceId is required",
		},
		{
			name: "gitops deployment mode",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				DeploymentMode: ModeGitOps,
				InstanceID:     "inst-123",
			},
			wantErr: false,
			validate: func(t *testing.T, manifest string) {
				var obj map[string]interface{}
				if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
					t.Fatalf("Failed to parse manifest YAML: %v", err)
				}

				metadata := obj["metadata"].(map[string]interface{})
				labels := metadata["labels"].(map[string]interface{})

				// Deployment mode is now a label, not an annotation
				if labels["knodex.io/deployment-mode"] != "gitops" {
					t.Errorf("Expected deployment mode gitops, got %v", labels["knodex.io/deployment-mode"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator(nil)
			manifest, err := gen.GenerateManifest(tt.spec)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if manifest == "" {
				t.Error("Expected non-empty manifest")
			}

			if tt.validate != nil {
				tt.validate(t, manifest)
			}
		})
	}
}

func TestGenerateManifest_WithValidator(t *testing.T) {
	now := time.Now()

	t.Run("validation passes", func(t *testing.T) {
		validator := &mockValidator{
			validateFunc: func(manifest map[string]interface{}, rgdName string) error {
				return nil
			},
		}

		gen := NewGenerator(validator)
		spec := &InstanceSpec{
			Name:           "test-instance",
			Namespace:      "test-namespace",
			RGDName:        "test-rgd",
			RGDNamespace:   "kro-system",
			APIVersion:     "example.com/v1",
			Kind:           "TestResource",
			Spec:           map[string]interface{}{},
			CreatedBy:      "user@example.com",
			CreatedAt:      now,
			DeploymentMode: ModeDirect,
			InstanceID:     "inst-123",
		}

		_, err := gen.GenerateManifest(spec)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("validation fails", func(t *testing.T) {
		validator := &mockValidator{
			validateFunc: func(manifest map[string]interface{}, rgdName string) error {
				return &ValidationError{Field: "spec.replicas", Message: "must be positive"}
			},
		}

		gen := NewGenerator(validator)
		spec := &InstanceSpec{
			Name:           "test-instance",
			Namespace:      "test-namespace",
			RGDName:        "test-rgd",
			RGDNamespace:   "kro-system",
			APIVersion:     "example.com/v1",
			Kind:           "TestResource",
			Spec:           map[string]interface{}{"replicas": -1},
			CreatedBy:      "user@example.com",
			CreatedAt:      now,
			DeploymentMode: ModeDirect,
			InstanceID:     "inst-123",
		}

		_, err := gen.GenerateManifest(spec)
		if err == nil {
			t.Error("Expected validation error, got nil")
		}
		if !strings.Contains(err.Error(), "schema validation failed") {
			t.Errorf("Expected schema validation error, got %v", err)
		}
	})
}

func TestGenerateMetadata(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		spec        *InstanceSpec
		wantErr     bool
		errContains string
		validate    func(t *testing.T, metadata string)
	}{
		{
			name: "valid instance spec generates correct metadata",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				RGDVersion:     "1.0.0",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				DeploymentMode: ModeGitOps,
				InstanceID:     "inst-123",
				ProjectID:      "proj-456",
			},
			wantErr: false,
			validate: func(t *testing.T, metadata string) {
				var obj map[string]interface{}
				if err := yaml.Unmarshal([]byte(metadata), &obj); err != nil {
					t.Fatalf("Failed to parse metadata YAML: %v", err)
				}

				expected := map[string]string{
					"instanceId":     "inst-123",
					"name":           "test-instance",
					"namespace":      "test-namespace",
					"rgdName":        "test-rgd",
					"rgdVersion":     "1.0.0",
					"rgdNamespace":   "kro-system",
					"createdBy":      "user@example.com",
					"deploymentMode": "gitops",
					"projectId":      "proj-456",
				}

				for key, expectedVal := range expected {
					if obj[key] != expectedVal {
						t.Errorf("Expected %s=%s, got %v", key, expectedVal, obj[key])
					}
				}

				// Validate createdAt timestamp
				createdAt, ok := obj["createdAt"].(string)
				if !ok || createdAt == "" {
					t.Error("Expected createdAt to be a non-empty string")
				}
			},
		},
		{
			name:        "nil spec returns error",
			spec:        nil,
			wantErr:     true,
			errContains: "cannot be nil",
		},
		{
			name: "metadata without projectId",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "test-namespace",
				RGDName:        "test-rgd",
				RGDVersion:     "1.0.0",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				DeploymentMode: ModeDirect,
				InstanceID:     "inst-123",
			},
			wantErr: false,
			validate: func(t *testing.T, metadata string) {
				var obj map[string]interface{}
				if err := yaml.Unmarshal([]byte(metadata), &obj); err != nil {
					t.Fatalf("Failed to parse metadata YAML: %v", err)
				}

				if _, exists := obj["projectId"]; exists {
					t.Error("Expected projectId to be absent when not provided")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator(nil)
			metadata, err := gen.GenerateMetadata(tt.spec)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if metadata == "" {
				t.Error("Expected non-empty metadata")
			}

			if tt.validate != nil {
				tt.validate(t, metadata)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	now := time.Now()

	spec := &InstanceSpec{
		Name:           "test-instance",
		Namespace:      "production",
		RGDName:        "test-rgd",
		RGDVersion:     "1.0.0",
		RGDNamespace:   "kro-system",
		APIVersion:     "example.com/v1",
		Kind:           "TestResource",
		Spec:           map[string]interface{}{"replicas": 3},
		CreatedBy:      "user@example.com",
		CreatedAt:      now,
		DeploymentMode: ModeGitOps,
		InstanceID:     "inst-123",
		ProjectID:      "proj-456",
	}

	gen := NewGenerator(nil)
	output, err := gen.Generate(spec)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if output.Manifest == "" {
		t.Error("Expected non-empty manifest")
	}
	if output.Metadata == "" {
		t.Error("Expected non-empty metadata")
	}
	if output.ManifestPath != "instances/production/test-instance.yaml" {
		t.Errorf("Expected manifest path instances/production/test-instance.yaml, got %s", output.ManifestPath)
	}
	if output.MetadataPath != "instances/production/.metadata/test-instance.yaml" {
		t.Errorf("Expected metadata path instances/production/.metadata/test-instance.yaml, got %s", output.MetadataPath)
	}
}

func TestGenerateManifestPath(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		instanceName string
		expected     string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid production path",
			namespace:    "production",
			instanceName: "app-db",
			expected:     "instances/production/app-db.yaml",
			wantErr:      false,
		},
		{
			name:         "valid dev path",
			namespace:    "dev",
			instanceName: "test-service",
			expected:     "instances/dev/test-service.yaml",
			wantErr:      false,
		},
		{
			name:         "valid staging path",
			namespace:    "staging",
			instanceName: "frontend",
			expected:     "instances/staging/frontend.yaml",
			wantErr:      false,
		},
		{
			name:         "namespace with directory traversal",
			namespace:    "../etc",
			instanceName: "passwd",
			wantErr:      true,
			errContains:  "directory traversal",
		},
		{
			name:         "name with directory traversal",
			namespace:    "production",
			instanceName: "../../etc/passwd",
			wantErr:      true,
			errContains:  "directory traversal",
		},
		{
			name:         "namespace with forward slash",
			namespace:    "prod/duction",
			instanceName: "app",
			wantErr:      true,
			errContains:  "path separator",
		},
		{
			name:         "name with forward slash",
			namespace:    "production",
			instanceName: "app/db",
			wantErr:      true,
			errContains:  "path separator",
		},
		{
			name:         "namespace with backslash",
			namespace:    "prod\\duction",
			instanceName: "app",
			wantErr:      true,
			errContains:  "path separator",
		},
		{
			name:         "name with backslash",
			namespace:    "production",
			instanceName: "app\\db",
			wantErr:      true,
			errContains:  "path separator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GenerateManifestPath(tt.namespace, tt.instanceName)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if path != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestGenerateMetadataPath(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		instanceName string
		expected     string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "valid production path",
			namespace:    "production",
			instanceName: "app-db",
			expected:     "instances/production/.metadata/app-db.yaml",
			wantErr:      false,
		},
		{
			name:         "valid dev path",
			namespace:    "dev",
			instanceName: "test-service",
			expected:     "instances/dev/.metadata/test-service.yaml",
			wantErr:      false,
		},
		{
			name:         "valid staging path",
			namespace:    "staging",
			instanceName: "frontend",
			expected:     "instances/staging/.metadata/frontend.yaml",
			wantErr:      false,
		},
		{
			name:         "namespace with directory traversal",
			namespace:    "../etc",
			instanceName: "passwd",
			wantErr:      true,
			errContains:  "directory traversal",
		},
		{
			name:         "name with directory traversal",
			namespace:    "production",
			instanceName: "../../etc/passwd",
			wantErr:      true,
			errContains:  "directory traversal",
		},
		{
			name:         "namespace with forward slash",
			namespace:    "prod/duction",
			instanceName: "app",
			wantErr:      true,
			errContains:  "path separator",
		},
		{
			name:         "name with forward slash",
			namespace:    "production",
			instanceName: "app/db",
			wantErr:      true,
			errContains:  "path separator",
		},
		{
			name:         "namespace with backslash",
			namespace:    "prod\\duction",
			instanceName: "app",
			wantErr:      true,
			errContains:  "path separator",
		},
		{
			name:         "name with backslash",
			namespace:    "production",
			instanceName: "app\\db",
			wantErr:      true,
			errContains:  "path separator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GenerateMetadataPath(tt.namespace, tt.instanceName)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if path != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, path)
			}
		})
	}
}

func TestValidateLabelAnnotationValue(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		value       string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid alphanumeric value",
			fieldName: "name",
			value:     "test-instance",
			wantErr:   false,
		},
		{
			name:      "valid email",
			fieldName: "createdBy",
			value:     "user@example.com",
			wantErr:   false,
		},
		{
			name:      "valid uuid",
			fieldName: "instanceId",
			value:     "550e8400-e29b-41d4-a716-446655440000",
			wantErr:   false,
		},
		{
			name:      "valid with dots and underscores",
			fieldName: "name",
			value:     "my_app.v1.test",
			wantErr:   false,
		},
		{
			name:      "empty value passes (handled by field validation)",
			fieldName: "projectId",
			value:     "",
			wantErr:   false,
		},
		{
			name:        "value with newline",
			fieldName:   "name",
			value:       "test\ninstance",
			wantErr:     true,
			errContains: "newline",
		},
		{
			name:        "value with carriage return",
			fieldName:   "name",
			value:       "test\rinstance",
			wantErr:     true,
			errContains: "newline",
		},
		{
			name:        "value with YAML key-value separator",
			fieldName:   "name",
			value:       "test: malicious",
			wantErr:     true,
			errContains: ": ",
		},
		{
			name:        "value with YAML list indicator",
			fieldName:   "name",
			value:       "test- item",
			wantErr:     true,
			errContains: "- ",
		},
		{
			name:        "value with literal block scalar",
			fieldName:   "name",
			value:       "test| block",
			wantErr:     true,
			errContains: "| ",
		},
		{
			name:        "value with folded block scalar",
			fieldName:   "name",
			value:       "test> folded",
			wantErr:     true,
			errContains: "> ",
		},
		{
			name:        "value with template injection",
			fieldName:   "name",
			value:       "{{malicious}}",
			wantErr:     true,
			errContains: "{{",
		},
		{
			name:        "value with variable substitution",
			fieldName:   "name",
			value:       "${INJECT}",
			wantErr:     true,
			errContains: "${",
		},
		{
			name:        "value exceeding max length",
			fieldName:   "name",
			value:       strings.Repeat("a", 300),
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name:        "value with invalid characters",
			fieldName:   "name",
			value:       "test*instance",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "value starting with dash",
			fieldName:   "name",
			value:       "-test",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "value ending with dash",
			fieldName:   "name",
			value:       "test-",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "value with spaces",
			fieldName:   "name",
			value:       "test instance",
			wantErr:     true,
			errContains: "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLabelAnnotationValue(tt.fieldName, tt.value)

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

func TestGenerateManifest_RejectsInjectionAttempts(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		spec        *InstanceSpec
		errContains string
	}{
		{
			name: "name with YAML injection",
			spec: &InstanceSpec{
				Name:           "test: malicious",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: ": ",
		},
		{
			name: "instanceID with newline",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123\nmalicious: value",
				DeploymentMode: ModeDirect,
			},
			errContains: "newline",
		},
		{
			name: "createdBy with template injection",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "{{.Values.secret}}",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: "{{",
		},
		{
			name: "projectId with variable substitution",
			spec: &InstanceSpec{
				Name:           "test-instance",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
				ProjectID:      "${MALICIOUS_VAR}",
			},
			errContains: "${",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator(nil)
			_, err := gen.GenerateManifest(tt.spec)

			if err == nil {
				t.Error("Expected error for injection attempt, got nil")
			} else if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
			}
		})
	}
}

func TestValidateSpecMap(t *testing.T) {
	tests := []struct {
		name        string
		spec        map[string]interface{}
		maxDepth    int
		wantErr     bool
		errContains string
	}{
		{
			name: "valid simple spec",
			spec: map[string]interface{}{
				"replicas": 3,
				"image":    "nginx:latest",
			},
			maxDepth: 10,
			wantErr:  false,
		},
		{
			name: "valid nested spec",
			spec: map[string]interface{}{
				"replicas": 3,
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "myapp",
						},
					},
				},
			},
			maxDepth: 10,
			wantErr:  false,
		},
		{
			name: "valid spec with array",
			spec: map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
					map[string]interface{}{
						"name":  "sidecar",
						"image": "sidecar:v1",
					},
				},
			},
			maxDepth: 10,
			wantErr:  false,
		},
		{
			name: "spec exceeding depth limit",
			spec: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"level4": map[string]interface{}{
								"level5": map[string]interface{}{
									"level6": "too deep",
								},
							},
						},
					},
				},
			},
			maxDepth:    4,
			wantErr:     true,
			errContains: "nesting depth exceeds maximum",
		},
		{
			name: "spec with newline in key",
			spec: map[string]interface{}{
				"valid":          "value",
				"malicious\nkey": "value",
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: "prohibited control character",
		},
		{
			name: "spec with YAML injection in key",
			spec: map[string]interface{}{
				"malicious: injected": "value",
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: ": ",
		},
		{
			name: "spec with template injection in key",
			spec: map[string]interface{}{
				"{{.Values.secret}}": "value",
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: "{{",
		},
		{
			name: "spec with excessively long string value",
			spec: map[string]interface{}{
				"data": strings.Repeat("a", 70000),
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: "excessively long string",
		},
		{
			name: "spec with excessively long string in array",
			spec: map[string]interface{}{
				"items": []interface{}{
					"valid",
					strings.Repeat("b", 70000),
				},
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: "excessively long string",
		},
		{
			name: "spec with malicious key inside nested array",
			spec: map[string]interface{}{
				"items": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"malicious\nkey": "value",
						},
					},
				},
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: "prohibited control character",
		},
		{
			name: "spec with deeply nested arrays exceeding depth",
			spec: map[string]interface{}{
				"a": []interface{}{
					[]interface{}{
						[]interface{}{
							[]interface{}{
								[]interface{}{
									[]interface{}{
										[]interface{}{
											[]interface{}{
												[]interface{}{
													[]interface{}{
														[]interface{}{
															"deep",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			maxDepth:    10,
			wantErr:     true,
			errContains: "nesting depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpecMap(tt.spec, 0, tt.maxDepth)

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

func TestGenerateManifest_RejectsSpecInjection(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		spec        *InstanceSpec
		errContains string
	}{
		{
			name: "spec with YAML injection in key",
			spec: &InstanceSpec{
				Name:         "test-instance",
				Namespace:    "default",
				RGDName:      "test-rgd",
				RGDNamespace: "kro-system",
				APIVersion:   "example.com/v1",
				Kind:         "TestResource",
				Spec: map[string]interface{}{
					"malicious: injected": "value",
				},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: ": ",
		},
		{
			name: "spec exceeding depth limit",
			spec: &InstanceSpec{
				Name:         "test-instance",
				Namespace:    "default",
				RGDName:      "test-rgd",
				RGDNamespace: "kro-system",
				APIVersion:   "example.com/v1",
				Kind:         "TestResource",
				Spec: map[string]interface{}{
					"l1": map[string]interface{}{
						"l2": map[string]interface{}{
							"l3": map[string]interface{}{
								"l4": map[string]interface{}{
									"l5": map[string]interface{}{
										"l6": map[string]interface{}{
											"l7": map[string]interface{}{
												"l8": map[string]interface{}{
													"l9": map[string]interface{}{
														"l10": map[string]interface{}{
															"l11": map[string]interface{}{
																"l12": "too deep",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: "nesting depth",
		},
		{
			name: "spec with excessively long string",
			spec: &InstanceSpec{
				Name:         "test-instance",
				Namespace:    "default",
				RGDName:      "test-rgd",
				RGDNamespace: "kro-system",
				APIVersion:   "example.com/v1",
				Kind:         "TestResource",
				Spec: map[string]interface{}{
					"data": strings.Repeat("a", 70000),
				},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: "excessively long string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator(nil)
			_, err := gen.GenerateManifest(tt.spec)

			if err == nil {
				t.Error("Expected error for spec injection attempt, got nil")
			} else if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
			}
		})
	}
}

func TestDeploymentModeDefaults(t *testing.T) {
	now := time.Now()

	spec := &InstanceSpec{
		Name:         "test-instance",
		Namespace:    "test-namespace",
		RGDName:      "test-rgd",
		RGDNamespace: "kro-system",
		APIVersion:   "example.com/v1",
		Kind:         "TestResource",
		Spec:         map[string]interface{}{},
		CreatedBy:    "user@example.com",
		CreatedAt:    now,
		InstanceID:   "inst-123",
		// DeploymentMode not set
	}

	gen := NewGenerator(nil)
	manifest, err := gen.GenerateManifest(spec)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifest), &obj); err != nil {
		t.Fatalf("Failed to parse manifest YAML: %v", err)
	}

	metadata := obj["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})

	// Deployment mode is now a label, not an annotation
	if labels["knodex.io/deployment-mode"] != "direct" {
		t.Errorf("Expected default deployment mode direct, got %v", labels["knodex.io/deployment-mode"])
	}
}

func TestValidateKubernetesName(t *testing.T) {
	tests := []struct {
		name        string
		fieldName   string
		value       string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid lowercase name",
			fieldName: "name",
			value:     "my-instance",
			wantErr:   false,
		},
		{
			name:      "valid numeric start",
			fieldName: "namespace",
			value:     "123-namespace",
			wantErr:   false,
		},
		{
			name:      "valid single character",
			fieldName: "name",
			value:     "a",
			wantErr:   false,
		},
		{
			name:      "valid with many hyphens",
			fieldName: "name",
			value:     "my-very-long-instance-name",
			wantErr:   false,
		},
		{
			name:        "uppercase characters",
			fieldName:   "name",
			value:       "My-Instance",
			wantErr:     true,
			errContains: "lowercase alphanumeric",
		},
		{
			name:        "starts with hyphen",
			fieldName:   "name",
			value:       "-my-instance",
			wantErr:     true,
			errContains: "must start with a lowercase alphanumeric",
		},
		{
			name:        "ends with hyphen",
			fieldName:   "name",
			value:       "my-instance-",
			wantErr:     true,
			errContains: "must end with a lowercase alphanumeric",
		},
		{
			name:        "contains underscore",
			fieldName:   "name",
			value:       "my_instance",
			wantErr:     true,
			errContains: "must contain only lowercase alphanumeric",
		},
		{
			name:        "contains dot",
			fieldName:   "name",
			value:       "my.instance",
			wantErr:     true,
			errContains: "must contain only lowercase alphanumeric",
		},
		{
			name:        "exceeds max length",
			fieldName:   "name",
			value:       strings.Repeat("a", 254),
			wantErr:     true,
			errContains: "exceeds maximum length",
		},
		{
			name:        "contains spaces",
			fieldName:   "name",
			value:       "my instance",
			wantErr:     true,
			errContains: "must contain only lowercase alphanumeric",
		},
		{
			name:        "contains special characters",
			fieldName:   "name",
			value:       "my@instance",
			wantErr:     true,
			errContains: "must contain only lowercase alphanumeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKubernetesName(tt.fieldName, tt.value)

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

func TestGenerateManifest_RejectsInvalidKubernetesNames(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		spec        *InstanceSpec
		errContains string
	}{
		{
			name: "invalid namespace with uppercase",
			spec: &InstanceSpec{
				Name:           "valid-name",
				Namespace:      "Invalid-Namespace",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{"replicas": 3},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: "namespace",
		},
		{
			name: "invalid name starting with hyphen",
			spec: &InstanceSpec{
				Name:           "-invalid-name",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{"replicas": 3},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: "must start with a lowercase alphanumeric",
		},
		{
			name: "invalid name with underscore",
			spec: &InstanceSpec{
				Name:           "invalid_name",
				Namespace:      "default",
				RGDName:        "test-rgd",
				RGDNamespace:   "kro-system",
				APIVersion:     "example.com/v1",
				Kind:           "TestResource",
				Spec:           map[string]interface{}{"replicas": 3},
				CreatedBy:      "user@example.com",
				CreatedAt:      now,
				InstanceID:     "inst-123",
				DeploymentMode: ModeDirect,
			},
			errContains: "must contain only lowercase alphanumeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator(nil)
			_, err := gen.GenerateManifest(tt.spec)

			if err == nil {
				t.Error("Expected error for invalid Kubernetes name, got nil")
			} else if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error containing %q, got %v", tt.errContains, err)
			}
		})
	}
}
