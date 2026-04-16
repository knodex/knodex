// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	fakeapiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/knodex/knodex/server/internal/models"
)

// testLogger returns a no-op logger for tests
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestExtractCRDInfo(t *testing.T) {
	extractor := &Extractor{
		logger: testLogger(),
	}

	tests := []struct {
		name        string
		rgd         *models.CatalogRGD
		wantGroup   string
		wantKind    string
		wantVersion string
		wantErr     bool
	}{
		{
			name: "pre-computed APIVersion with group/version",
			rgd: &models.CatalogRGD{
				Name:       "aso-credential",
				APIVersion: "kro.run/v1alpha1",
				Kind:       "ASOCredential",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
						"kind":       "ASOCredential",
					},
				},
			},
			wantGroup:   "kro.run",
			wantKind:    "ASOCredential",
			wantVersion: "v1alpha1",
		},
		{
			name: "short apiVersion in rawSpec but pre-computed has group",
			rgd: &models.CatalogRGD{
				Name:       "simple-app",
				APIVersion: "kro.run/v1alpha1",
				Kind:       "SimpleApp",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
						"kind":       "SimpleApp",
					},
				},
			},
			wantGroup:   "kro.run",
			wantKind:    "SimpleApp",
			wantVersion: "v1alpha1",
		},
		{
			name: "full apiVersion in schema",
			rgd: &models.CatalogRGD{
				Name:       "some-rgd",
				APIVersion: "custom.example.com/v1beta1",
				Kind:       "MyResource",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "custom.example.com/v1beta1",
						"kind":       "MyResource",
					},
				},
			},
			wantGroup:   "custom.example.com",
			wantKind:    "MyResource",
			wantVersion: "v1beta1",
		},
		{
			name: "fallback to rawSpec when pre-computed empty",
			rgd: &models.CatalogRGD{
				Name: "fallback-rgd",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "example.io/v1",
						"kind":       "FallbackKind",
					},
				},
			},
			wantGroup:   "example.io",
			wantKind:    "FallbackKind",
			wantVersion: "v1",
		},
		{
			name: "rawSpec with separate group field",
			rgd: &models.CatalogRGD{
				Name: "group-field-rgd",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"group":   "explicit.group.io",
						"kind":    "GroupKind",
						"version": "v2",
					},
				},
			},
			wantGroup:   "explicit.group.io",
			wantKind:    "GroupKind",
			wantVersion: "v2",
		},
		{
			name: "missing group fails",
			rgd: &models.CatalogRGD{
				Name: "no-group",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
						"kind":       "NoGroupKind",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing kind fails",
			rgd: &models.CatalogRGD{
				Name:       "no-kind",
				APIVersion: "kro.run/v1alpha1",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"apiVersion": "v1alpha1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "nil rawSpec with pre-computed values",
			rgd: &models.CatalogRGD{
				Name:       "no-rawspec",
				APIVersion: "kro.run/v1alpha1",
				Kind:       "NoRawSpecKind",
			},
			wantGroup:   "kro.run",
			wantKind:    "NoRawSpecKind",
			wantVersion: "v1alpha1",
		},
		{
			name: "nil rawSpec without pre-computed values fails",
			rgd: &models.CatalogRGD{
				Name: "empty-rgd",
			},
			wantErr: true,
		},
		{
			name: "default version when not specified",
			rgd: &models.CatalogRGD{
				Name: "no-version",
				Kind: "NoVersionKind",
				RawSpec: map[string]interface{}{
					"schema": map[string]interface{}{
						"group": "example.io",
						"kind":  "NoVersionKind",
					},
				},
			},
			wantGroup:   "example.io",
			wantKind:    "NoVersionKind",
			wantVersion: "v1alpha1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, kind, version, err := extractor.extractCRDInfo(tt.rgd)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if group != tt.wantGroup {
				t.Errorf("group = %q, want %q", group, tt.wantGroup)
			}
			if kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", kind, tt.wantKind)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
		})
	}
}

func TestDegradedCacheExpiresFaster(t *testing.T) {
	// Use a fake client with NO CRDs — ExtractSchema will get a 404
	fakeClient := fakeapiext.NewSimpleClientset()
	extractor := NewExtractorWithClient(fakeClient)

	rgd := &models.CatalogRGD{
		Name:       "test-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1alpha1",
		Kind:       "TestApp",
		RawSpec: map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "example.com/v1alpha1",
				"kind":       "TestApp",
				"spec": map[string]interface{}{
					"name": "string",
				},
			},
		},
	}

	// First call: triggers CRD lookup → 404 → cached with short TTL
	_, err := extractor.ExtractSchema(context.Background(), rgd)
	if err == nil {
		t.Fatal("expected error for missing CRD")
	}

	cacheKey := "default/test-rgd"
	extractor.cacheMu.RLock()
	cached, ok := extractor.cache[cacheKey]
	extractor.cacheMu.RUnlock()

	if !ok {
		t.Fatal("expected cache entry after ExtractSchema")
	}
	if !cached.Degraded {
		t.Error("cache entry should be marked as degraded for 404")
	}

	// Verify degraded entry expires within ~30s (not 5min)
	timeUntilExpiry := time.Until(cached.ExpiresAt)
	if timeUntilExpiry > 35*time.Second {
		t.Errorf("degraded cache TTL should be ~30s, got %v", timeUntilExpiry)
	}
	if timeUntilExpiry > 5*time.Minute-30*time.Second {
		t.Errorf("degraded cache TTL (%v) is too close to normal TTL (5min)", timeUntilExpiry)
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "generic error",
			err:  fmt.Errorf("something went wrong"),
			want: false,
		},
		{
			name: "wrapped not-found error",
			err: fmt.Errorf("CRD not found: test: %w",
				apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "test")),
			want: true,
		},
		{
			// extractSchemaFromCRD now returns apierrors.NewNotFound directly (unwrapped).
			// Verify IsNotFoundError handles this without a wrapping fmt.Errorf.
			name: "direct not-found error (unwrapped)",
			err:  apierrors.NewNotFound(schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"}, "group=x kind=y"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFoundError(tt.err)
			if got != tt.want {
				t.Errorf("IsNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractSchemaFromCRD_IrregularPlural(t *testing.T) {
	// Create a CRD with an irregular plural (proxies, not proxys)
	crd := newTestCRD("proxies.example.com", "example.com", "Proxy", "proxies", "v1alpha1",
		map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"host": {Type: "string"},
					"port": {Type: "integer"},
				},
				Required: []string{"host"},
			},
		})

	fakeClient := fakeapiext.NewSimpleClientset(crd)
	extractor := NewExtractorWithClient(fakeClient)

	rgd := &models.CatalogRGD{
		Name:       "proxy-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1alpha1",
		Kind:       "Proxy",
	}

	schema, err := extractor.ExtractSchema(context.Background(), rgd)
	if err != nil {
		t.Fatalf("expected no error for irregular plural CRD, got: %v", err)
	}
	if schema == nil {
		t.Fatal("expected schema, got nil")
	}
	if schema.Kind != "Proxy" {
		t.Errorf("schema.Kind = %q, want %q", schema.Kind, "Proxy")
	}
	if _, ok := schema.Properties["host"]; !ok {
		t.Error("expected 'host' property in schema")
	}
	if _, ok := schema.Properties["port"]; !ok {
		t.Error("expected 'port' property in schema")
	}
	// Verify Required field propagated from CRD spec
	if len(schema.Required) != 1 || schema.Required[0] != "host" {
		t.Errorf("schema.Required = %v, want [host]", schema.Required)
	}
}

func TestExtractSchemaFromCRD_NoCRDMatch(t *testing.T) {
	// Create a CRD that does NOT match what we'll look up
	crd := newTestCRD("widgets.other.io", "other.io", "Widget", "widgets", "v1",
		map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {Type: "object"},
		})

	fakeClient := fakeapiext.NewSimpleClientset(crd)
	extractor := NewExtractorWithClient(fakeClient)

	rgd := &models.CatalogRGD{
		Name:       "missing-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1alpha1",
		Kind:       "NonExistent",
	}

	_, err := extractor.ExtractSchema(context.Background(), rgd)
	if err == nil {
		t.Fatal("expected error for non-matching CRD, got nil")
	}
	if !strings.Contains(err.Error(), "group=example.com") || !strings.Contains(err.Error(), "kind=NonExistent") {
		t.Errorf("error should contain group and kind, got: %v", err)
	}
	// Verify it's detected as a NotFound error for degraded cache behavior
	if !IsNotFoundError(err) {
		t.Error("expected error to be detected as NotFound for degraded cache TTL")
	}
}

func TestExtractSchemaFromCRD_ListFailure(t *testing.T) {
	fakeClient := fakeapiext.NewSimpleClientset()
	// Inject a reactor that returns an error for any List call.
	fakeClient.PrependReactor("list", "customresourcedefinitions", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("api server unavailable")
	})
	extractor := NewExtractorWithClient(fakeClient)

	rgd := &models.CatalogRGD{
		Name:       "fail-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1alpha1",
		Kind:       "FailKind",
	}

	_, err := extractor.ExtractSchema(context.Background(), rgd)
	if err == nil {
		t.Fatal("expected error when List fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list CRDs") {
		t.Errorf("expected 'failed to list CRDs' in error, got: %v", err)
	}
	// Verify the error is cached with normal TTL (not degraded), since a List
	// failure is transient, not a "CRD not found" condition.
	cacheKey := "default/fail-rgd"
	extractor.cacheMu.RLock()
	cached, ok := extractor.cache[cacheKey]
	extractor.cacheMu.RUnlock()
	if !ok {
		t.Fatal("expected cache entry after List failure")
	}
	if cached.Degraded {
		t.Error("List failure should NOT be cached as degraded (that's for NotFound only)")
	}
	ttl := time.Until(cached.ExpiresAt)
	if ttl < 4*time.Minute {
		t.Errorf("List failure should use normal cache TTL (~5min), got %v", ttl)
	}
}

func TestExtractSchemaFromCRD_CaseSensitiveKind(t *testing.T) {
	// CRD has Kind "Proxy" (PascalCase). Looking up "proxy" (lowercase) must NOT match.
	crd := newTestCRD("proxies.example.com", "example.com", "Proxy", "proxies", "v1alpha1",
		map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {Type: "object", Properties: map[string]apiextensionsv1.JSONSchemaProps{
				"host": {Type: "string"},
			}},
		})

	fakeClient := fakeapiext.NewSimpleClientset(crd)
	extractor := NewExtractorWithClient(fakeClient)

	rgd := &models.CatalogRGD{
		Name:       "proxy-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1alpha1",
		Kind:       "proxy", // lowercase — should NOT match "Proxy"
	}

	_, err := extractor.ExtractSchema(context.Background(), rgd)
	if err == nil {
		t.Fatal("expected error: lowercase 'proxy' should not match PascalCase 'Proxy'")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestExtractSchema_IsClusterScoped(t *testing.T) {
	crd := newTestCRD("tenants.example.io", "example.io", "Tenant", "tenants", "v1alpha1",
		map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"tenantName": {Type: "string"},
				},
			},
		})

	fakeClient := fakeapiext.NewSimpleClientset(crd)
	extractor := NewExtractorWithClient(fakeClient)

	t.Run("cluster-scoped RGD produces IsClusterScoped=true in FormSchema", func(t *testing.T) {
		rgd := &models.CatalogRGD{
			Name:            "tenant-provisioner",
			Namespace:       "default",
			APIVersion:      "example.io/v1alpha1",
			Kind:            "Tenant",
			IsClusterScoped: true,
		}
		schema, err := extractor.ExtractSchema(context.Background(), rgd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !schema.IsClusterScoped {
			t.Error("expected IsClusterScoped=true in FormSchema for cluster-scoped RGD (CRD path)")
		}
	})

	t.Run("namespace-scoped RGD produces IsClusterScoped=false in FormSchema", func(t *testing.T) {
		rgd := &models.CatalogRGD{
			Name:            "tenant-provisioner",
			Namespace:       "default",
			APIVersion:      "example.io/v1alpha1",
			Kind:            "Tenant",
			IsClusterScoped: false,
		}
		// Invalidate cache so we get a fresh result
		extractor.InvalidateCache("default", "tenant-provisioner")
		schema, err := extractor.ExtractSchema(context.Background(), rgd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if schema.IsClusterScoped {
			t.Error("expected IsClusterScoped=false in FormSchema for namespace-scoped RGD (CRD path)")
		}
	})
}

// newTestCRD creates a CRD for testing
func newTestCRD(name, group, kind, plural, version string, props map[string]apiextensionsv1.JSONSchemaProps) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   kind,
				Plural: plural,
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: version,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:       "object",
							Properties: props,
						},
					},
				},
			},
		},
	}
}

// --- Property Order Annotation Tests ---

func TestParsePropertyOrderAnnotation_ValidJSON(t *testing.T) {
	logger := testLogger()
	annotations := map[string]string{
		"knodex.io/property-order": `{"": ["name", "version"], "config": ["replicas", "memory"]}`,
	}

	result := parsePropertyOrderAnnotation(annotations, logger)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(result))
	}

	topLevel := result[""]
	if len(topLevel) != 2 || topLevel[0] != "name" || topLevel[1] != "version" {
		t.Errorf("unexpected top-level order: %v", topLevel)
	}

	nested := result["config"]
	if len(nested) != 2 || nested[0] != "replicas" || nested[1] != "memory" {
		t.Errorf("unexpected config order: %v", nested)
	}
}

func TestParsePropertyOrderAnnotation_Missing(t *testing.T) {
	logger := testLogger()

	result := parsePropertyOrderAnnotation(map[string]string{}, logger)
	if result != nil {
		t.Errorf("expected nil for missing annotation, got %v", result)
	}

	result = parsePropertyOrderAnnotation(map[string]string{"knodex.io/property-order": ""}, logger)
	if result != nil {
		t.Errorf("expected nil for empty annotation, got %v", result)
	}
}

func TestParsePropertyOrderAnnotation_MalformedJSON(t *testing.T) {
	logger := testLogger()
	annotations := map[string]string{
		"knodex.io/property-order": `not valid json`,
	}

	result := parsePropertyOrderAnnotation(annotations, logger)
	if result != nil {
		t.Errorf("expected nil for malformed JSON, got %v", result)
	}
}

func TestApplyNestedPropertyOrder_SingleLevel(t *testing.T) {
	props := map[string]models.FormProperty{
		"name": {Type: "string"},
		"config": {
			Type: "object",
			Properties: map[string]models.FormProperty{
				"replicas": {Type: "integer"},
				"memory":   {Type: "string"},
				"cpu":      {Type: "string"},
			},
		},
	}

	orderMap := map[string][]string{
		"config": {"memory", "cpu", "replicas"},
	}

	applyNestedPropertyOrder(props, orderMap, "")

	configProp := props["config"]
	if len(configProp.PropertyOrder) != 3 {
		t.Fatalf("expected 3 items in PropertyOrder, got %d", len(configProp.PropertyOrder))
	}
	if configProp.PropertyOrder[0] != "memory" || configProp.PropertyOrder[1] != "cpu" || configProp.PropertyOrder[2] != "replicas" {
		t.Errorf("unexpected PropertyOrder: %v", configProp.PropertyOrder)
	}

	nameProp := props["name"]
	if nameProp.PropertyOrder != nil {
		t.Errorf("string property should not have PropertyOrder")
	}
}

func TestApplyNestedPropertyOrder_DeeplyNested(t *testing.T) {
	props := map[string]models.FormProperty{
		"config": {
			Type: "object",
			Properties: map[string]models.FormProperty{
				"resources": {
					Type: "object",
					Properties: map[string]models.FormProperty{
						"cpu":    {Type: "string"},
						"memory": {Type: "string"},
						"gpu":    {Type: "string"},
					},
				},
				"replicas": {Type: "integer"},
			},
		},
	}

	orderMap := map[string][]string{
		"config":           {"resources", "replicas"},
		"config.resources": {"memory", "gpu", "cpu"},
	}

	applyNestedPropertyOrder(props, orderMap, "")

	configProp := props["config"]
	if len(configProp.PropertyOrder) != 2 || configProp.PropertyOrder[0] != "resources" {
		t.Errorf("unexpected config PropertyOrder: %v", configProp.PropertyOrder)
	}

	resourcesProp := configProp.Properties["resources"]
	if len(resourcesProp.PropertyOrder) != 3 || resourcesProp.PropertyOrder[0] != "memory" || resourcesProp.PropertyOrder[2] != "cpu" {
		t.Errorf("unexpected resources PropertyOrder: %v", resourcesProp.PropertyOrder)
	}
}

func TestApplyNestedPropertyOrder_NoMatchingKey(t *testing.T) {
	props := map[string]models.FormProperty{
		"config": {
			Type: "object",
			Properties: map[string]models.FormProperty{
				"replicas": {Type: "integer"},
			},
		},
	}

	orderMap := map[string][]string{
		"nonexistent": {"a", "b"},
	}

	applyNestedPropertyOrder(props, orderMap, "")

	configProp := props["config"]
	if configProp.PropertyOrder != nil {
		t.Errorf("expected nil PropertyOrder when no matching key, got %v", configProp.PropertyOrder)
	}
}

func TestConvertToFormSchema_WithPropertyOrder(t *testing.T) {
	extractor := &Extractor{logger: testLogger()}

	rgd := &models.CatalogRGD{
		Name:      "test-rgd",
		Namespace: "default",
		Annotations: map[string]string{
			"knodex.io/property-order": `{"": ["name", "version", "config"], "config": ["replicas", "memory"]}`,
		},
	}

	openAPISchema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"name":    {Type: "string"},
					"version": {Type: "string"},
					"config": {
						Type: "object",
						Properties: map[string]apiextensionsv1.JSONSchemaProps{
							"replicas": {Type: "integer"},
							"memory":   {Type: "string"},
						},
					},
				},
			},
		},
	}

	formSchema := extractor.convertToFormSchema(rgd, "kro.run", "TestRGD", "v1alpha1", openAPISchema)

	if len(formSchema.PropertyOrder) != 3 {
		t.Fatalf("expected 3 items in top-level PropertyOrder, got %d", len(formSchema.PropertyOrder))
	}
	if formSchema.PropertyOrder[0] != "name" || formSchema.PropertyOrder[1] != "version" || formSchema.PropertyOrder[2] != "config" {
		t.Errorf("unexpected top-level PropertyOrder: %v", formSchema.PropertyOrder)
	}

	configProp := formSchema.Properties["config"]
	if len(configProp.PropertyOrder) != 2 || configProp.PropertyOrder[0] != "replicas" || configProp.PropertyOrder[1] != "memory" {
		t.Errorf("unexpected config PropertyOrder: %v", configProp.PropertyOrder)
	}
}

func TestPropertyOrderAnnotation_PartialMatch(t *testing.T) {
	// Annotation lists "missing" (not in schema) — it should be ignored.
	// Schema has "count" (not in annotation) — it should be appended.
	// Frontend applies the alphabetical append, but server must emit the correct PropertyOrder.
	logger := testLogger()
	annotations := map[string]string{
		"knodex.io/property-order": `{"": ["name", "missing", "version"]}`,
	}

	result := parsePropertyOrderAnnotation(annotations, logger)
	if result == nil {
		t.Fatal("expected non-nil result for partial-match annotation")
	}

	topLevel := result[""]
	if len(topLevel) != 3 {
		t.Fatalf("expected 3 entries in annotation (server stores raw, frontend handles ignore), got %d", len(topLevel))
	}
	// Verify the raw annotation values are preserved as-is; frontend filters unknowns.
	if topLevel[0] != "name" || topLevel[1] != "missing" || topLevel[2] != "version" {
		t.Errorf("unexpected partial-match order: %v", topLevel)
	}
}

func TestPropertyOrderAnnotation_DuplicateEntries(t *testing.T) {
	// Annotation contains duplicate field names within a value array.
	// Server stores the raw slice; frontend deduplicates (first wins).
	// Server must not panic or error on duplicates.
	logger := testLogger()
	annotations := map[string]string{
		"knodex.io/property-order": `{"": ["name", "version", "name"]}`,
	}

	result := parsePropertyOrderAnnotation(annotations, logger)
	if result == nil {
		t.Fatal("expected non-nil result for annotation with duplicate entries")
	}

	topLevel := result[""]
	if len(topLevel) != 3 {
		t.Fatalf("expected 3 entries (duplicates preserved for frontend dedup), got %d", len(topLevel))
	}
	if topLevel[0] != "name" || topLevel[1] != "version" || topLevel[2] != "name" {
		t.Errorf("unexpected duplicate-entry order: %v", topLevel)
	}
}

func TestConvertToFormSchema_NoPropertyOrder(t *testing.T) {
	extractor := &Extractor{logger: testLogger()}

	rgd := &models.CatalogRGD{
		Name:      "test-rgd",
		Namespace: "default",
	}

	openAPISchema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"name": {Type: "string"},
				},
			},
		},
	}

	formSchema := extractor.convertToFormSchema(rgd, "kro.run", "TestRGD", "v1alpha1", openAPISchema)

	if formSchema.PropertyOrder != nil {
		t.Errorf("expected nil PropertyOrder when no annotation, got %v", formSchema.PropertyOrder)
	}
}
