package testutil

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knodex/knodex/server/internal/models"
)

// --- Unstructured RGD builder ---

type rgdConfig struct {
	annotations map[string]string
	labels      map[string]string
	status      string
}

// RGDOption configures NewUnstructuredRGD.
type RGDOption func(*rgdConfig)

// WithAnnotations sets annotations on the unstructured RGD.
func WithAnnotations(a map[string]string) RGDOption {
	return func(c *rgdConfig) { c.annotations = a }
}

// WithLabels sets labels on the unstructured RGD.
func WithLabels(l map[string]string) RGDOption {
	return func(c *rgdConfig) { c.labels = l }
}

// WithStatus sets the status state on the unstructured RGD.
func WithStatus(state string) RGDOption {
	return func(c *rgdConfig) { c.status = state }
}

// NewUnstructuredRGD creates an *unstructured.Unstructured RGD for K8s-level tests.
// Default status state is "Active". Use WithStatus("") to omit status.
func NewUnstructuredRGD(name, namespace string, opts ...RGDOption) *unstructured.Unstructured {
	cfg := &rgdConfig{
		status: "Active",
	}
	for _, o := range opts {
		o(cfg)
	}

	annotationsInterface := make(map[string]interface{})
	for k, v := range cfg.annotations {
		annotationsInterface[k] = v
	}

	labelsInterface := make(map[string]interface{})
	for k, v := range cfg.labels {
		labelsInterface[k] = v
	}

	obj := map[string]interface{}{
		"apiVersion": "kro.run/v1alpha1",
		"kind":       "ResourceGraphDefinition",
		"metadata": map[string]interface{}{
			"name":              name,
			"namespace":         namespace,
			"annotations":       annotationsInterface,
			"labels":            labelsInterface,
			"resourceVersion":   "1",
			"creationTimestamp": time.Now().Format(time.RFC3339),
		},
		"spec": map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
		},
	}

	if cfg.status != "" {
		obj["status"] = map[string]interface{}{
			"state": cfg.status,
		}
	}

	return &unstructured.Unstructured{Object: obj}
}

// --- Catalog RGD builder ---

// CatalogRGDOption configures NewCatalogRGD.
type CatalogRGDOption func(*models.CatalogRGD)

// WithCatalogLabels sets labels on the catalog RGD.
func WithCatalogLabels(l map[string]string) CatalogRGDOption {
	return func(r *models.CatalogRGD) { r.Labels = l }
}

// WithCategory sets the category on the catalog RGD.
func WithCategory(c string) CatalogRGDOption {
	return func(r *models.CatalogRGD) { r.Category = c }
}

// NewCatalogRGD creates a models.CatalogRGD for service-level tests.
func NewCatalogRGD(name, namespace string, opts ...CatalogRGDOption) models.CatalogRGD {
	rgd := models.CatalogRGD{
		Name:        name,
		Namespace:   namespace,
		Description: "Test RGD " + name,
		Version:     "v1.0.0",
		Tags:        []string{"test"},
		Category:    "Testing",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	for _, o := range opts {
		o(&rgd)
	}
	return rgd
}
