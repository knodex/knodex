// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

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
	pluralName  string
	scope       string
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

// WithPluralName sets spec.schema.crd.spec.names.plural on the unstructured RGD.
// Used to test PluralName extraction in unstructuredToRGD.
func WithPluralName(p string) RGDOption {
	return func(c *rgdConfig) { c.pluralName = p }
}

// WithScope sets spec.schema.crd.spec.names.scope on the unstructured RGD.
// Used to test IsClusterScoped extraction in unstructuredToRGD.
func WithScope(s string) RGDOption {
	return func(c *rgdConfig) { c.scope = s }
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

	schemaMap := map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "TestResource",
	}
	// Build crd.spec.names block when pluralName or scope is set
	namesBlock := map[string]interface{}{}
	if cfg.pluralName != "" {
		namesBlock["plural"] = cfg.pluralName
	}
	if cfg.scope != "" {
		namesBlock["scope"] = cfg.scope
	}
	if len(namesBlock) > 0 {
		schemaMap["crd"] = map[string]interface{}{
			"spec": map[string]interface{}{
				"names": namesBlock,
			},
		}
	}
	spec := map[string]interface{}{
		"schema": schemaMap,
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
		"spec": spec,
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

// WithCatalogTier sets the catalog tier on the catalog RGD.
func WithCatalogTier(tier string) CatalogRGDOption {
	return func(r *models.CatalogRGD) { r.CatalogTier = tier }
}

// NewCatalogRGD creates a models.CatalogRGD for service-level tests.
func NewCatalogRGD(name, namespace string, opts ...CatalogRGDOption) models.CatalogRGD {
	rgd := models.CatalogRGD{
		Name:        name,
		Namespace:   namespace,
		Description: "Test RGD " + name,
		Tags:        []string{"test"},
		Category:    "Testing",
		CatalogTier: "both",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	for _, o := range opts {
		o(&rgd)
	}
	return rgd
}
