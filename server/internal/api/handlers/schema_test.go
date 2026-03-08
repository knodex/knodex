package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	fakeapiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kroschema "github.com/knodex/knodex/server/internal/kro/schema"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
)

// testSchemaRGD creates a CatalogRGD with a simpleschema spec for testing
func testSchemaRGD() *models.CatalogRGD {
	return &models.CatalogRGD{
		Name:        "test-app",
		Namespace:   "default",
		Description: "A test application",
		APIVersion:  "example.com/v1alpha1",
		Kind:        "TestApp",
		Annotations: map[string]string{models.CatalogAnnotation: "true"},
		Labels:      map[string]string{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "example.com/v1alpha1",
				"kind":       "TestApp",
				"spec": map[string]interface{}{
					"name":     "string",
					"replicas": "integer | default=3",
					"enabled":  "boolean | default=true",
				},
			},
		},
	}
}

// testCRD creates a CRD that matches the test RGD
func testCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapps.example.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "TestApp",
				Plural: "testapps",
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v1alpha1",
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"name": {
											Type: "string",
										},
										"replicas": {
											Type: "integer",
											Default: &apiextensionsv1.JSON{
												Raw: []byte("3"),
											},
										},
										"enabled": {
											Type: "boolean",
											Default: &apiextensionsv1.JSON{
												Raw: []byte("true"),
											},
										},
									},
									Required: []string{"name"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// setupSchemaHandler creates a SchemaHandler with a watcher containing the test RGD
// and an extractor backed by the provided K8s objects.
func setupSchemaHandler(t *testing.T, k8sObjects ...runtime.Object) *SchemaHandler {
	t.Helper()

	// Set up watcher with the test RGD
	cache := watcher.NewRGDCache()
	rgd := testSchemaRGD()
	cache.Set(rgd)
	w := watcher.NewRGDWatcherWithCache(cache)

	// Set up extractor with fake apiextensions client
	fakeClient := fakeapiext.NewSimpleClientset(k8sObjects...)
	extractor := kroschema.NewExtractorWithClient(fakeClient)

	return NewSchemaHandler(w, extractor)
}

func TestSchemaHandler_GetSchema_DegradedWhenCRDNotFound(t *testing.T) {
	t.Parallel()

	// No CRD objects — extractor will return 404
	handler := setupSchemaHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/test-app/schema", nil)
	req.SetPathValue("name", "test-app")
	w := httptest.NewRecorder()

	handler.GetSchema(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var schemaResp models.SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// AC2: source should be "rgd-only"
	if schemaResp.Source != "rgd-only" {
		t.Errorf("source = %q, want %q", schemaResp.Source, "rgd-only")
	}

	// AC6: crdFound should be false
	if schemaResp.CRDFound {
		t.Error("crdFound should be false when CRD is not available")
	}

	// AC1: schema should NOT be nil — this is the key behavior change
	if schemaResp.Schema == nil {
		t.Fatal("schema should not be nil in degraded mode")
	}

	// Schema should have properties from the RGD spec.schema
	if len(schemaResp.Schema.Properties) == 0 {
		t.Error("degraded schema should have properties from RGD spec")
	}

	// Check that expected properties exist
	for _, field := range []string{"name", "replicas", "enabled"} {
		if _, ok := schemaResp.Schema.Properties[field]; !ok {
			t.Errorf("missing expected property %q in degraded schema", field)
		}
	}

	// Should have a warning about missing validation constraints
	hasWarning := false
	for _, w := range schemaResp.Warnings {
		if len(w) > 0 {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("degraded response should include a warning about missing validation constraints")
	}
}

func TestSchemaHandler_GetSchema_FullSchemaWhenCRDExists(t *testing.T) {
	t.Parallel()

	// Provide the CRD — extractor will find it
	handler := setupSchemaHandler(t, testCRD())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/test-app/schema", nil)
	req.SetPathValue("name", "test-app")
	w := httptest.NewRecorder()

	handler.GetSchema(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var schemaResp models.SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// AC2: source should be "crd+rgd" for full schema
	if schemaResp.Source != "crd+rgd" {
		t.Errorf("source = %q, want %q", schemaResp.Source, "crd+rgd")
	}

	// CRD should be found
	if !schemaResp.CRDFound {
		t.Error("crdFound should be true when CRD exists")
	}

	// Schema should not be nil
	if schemaResp.Schema == nil {
		t.Fatal("schema should not be nil when CRD exists")
	}

	// Should have properties from the CRD
	if len(schemaResp.Schema.Properties) == 0 {
		t.Error("full schema should have properties")
	}

	// Full schema should have required fields (from CRD)
	if len(schemaResp.Schema.Required) == 0 {
		t.Error("full schema should have required fields from CRD")
	}
}

func TestSchemaHandler_GetSchema_DegradedResponseStructure(t *testing.T) {
	t.Parallel()

	// No CRD — triggers degraded path
	handler := setupSchemaHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/test-app/schema", nil)
	req.SetPathValue("name", "test-app")
	w := httptest.NewRecorder()

	handler.GetSchema(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var schemaResp models.SchemaResponse
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all degraded response fields per AC
	if schemaResp.CRDFound != false {
		t.Error("crdFound must be false in degraded response")
	}
	if schemaResp.Source != "rgd-only" {
		t.Errorf("source = %q, want %q", schemaResp.Source, "rgd-only")
	}
	if schemaResp.Schema == nil {
		t.Fatal("schema must not be nil in degraded response")
	}
	if len(schemaResp.Warnings) == 0 {
		t.Error("degraded response must contain at least one warning")
	}

	// Verify schema metadata
	if schemaResp.Schema.Kind != "TestApp" {
		t.Errorf("schema.kind = %q, want %q", schemaResp.Schema.Kind, "TestApp")
	}
	if schemaResp.Schema.Group != "example.com" {
		t.Errorf("schema.group = %q, want %q", schemaResp.Schema.Group, "example.com")
	}

	// Verify field types from simpleschema
	nameField, ok := schemaResp.Schema.Properties["name"]
	if !ok {
		t.Fatal("missing 'name' field in degraded schema")
	}
	if nameField.Type != "string" {
		t.Errorf("name.type = %q, want %q", nameField.Type, "string")
	}

	replicasField, ok := schemaResp.Schema.Properties["replicas"]
	if !ok {
		t.Fatal("missing 'replicas' field in degraded schema")
	}
	if replicasField.Type != "integer" {
		t.Errorf("replicas.type = %q, want %q", replicasField.Type, "integer")
	}

	enabledField, ok := schemaResp.Schema.Properties["enabled"]
	if !ok {
		t.Fatal("missing 'enabled' field in degraded schema")
	}
	if enabledField.Type != "boolean" {
		t.Errorf("enabled.type = %q, want %q", enabledField.Type, "boolean")
	}

	// Degraded schema should NOT have Required list (comes from CRD only)
	if len(schemaResp.Schema.Required) > 0 {
		t.Errorf("degraded schema should not have required fields, got %v", schemaResp.Schema.Required)
	}
}

func TestSchemaHandler_GetSchema_RGDNotFound(t *testing.T) {
	t.Parallel()

	handler := setupSchemaHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds/nonexistent/schema", nil)
	req.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetSchema(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestSchemaHandler_GetSchema_EmptyName(t *testing.T) {
	t.Parallel()

	handler := setupSchemaHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rgds//schema", nil)
	req.SetPathValue("name", "")
	w := httptest.NewRecorder()

	handler.GetSchema(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}
}
