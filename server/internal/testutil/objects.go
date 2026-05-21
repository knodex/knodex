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
	annotations      map[string]string
	labels           map[string]string
	status           string
	pluralName       string
	scope            string
	schemaAPIVersion string // overrides default "example.com/v1"; "-" to omit entirely
	crdVersions      []map[string]interface{}
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

// WithSchemaAPIVersion overrides spec.schema.apiVersion on the unstructured RGD.
// The default is "example.com/v1". Pass "-" to omit the field entirely.
func WithSchemaAPIVersion(v string) RGDOption {
	return func(c *rgdConfig) { c.schemaAPIVersion = v }
}

// WithCRDVersions sets spec.schema.crd.spec.versions on the unstructured RGD.
// Each entry should include at least "name" and "served". Used to test the
// multi-served-version invariant in the RGD watcher.
func WithCRDVersions(versions []map[string]interface{}) RGDOption {
	return func(c *rgdConfig) { c.crdVersions = versions }
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
		"kind": "TestResource",
	}
	// Default apiVersion is "example.com/v1"; "-" sentinel omits it.
	switch cfg.schemaAPIVersion {
	case "":
		schemaMap["apiVersion"] = "example.com/v1"
	case "-":
		// omit
	default:
		schemaMap["apiVersion"] = cfg.schemaAPIVersion
	}
	// Build crd.spec block when names or versions are set
	crdSpec := map[string]interface{}{}
	namesBlock := map[string]interface{}{}
	if cfg.pluralName != "" {
		namesBlock["plural"] = cfg.pluralName
	}
	if cfg.scope != "" {
		namesBlock["scope"] = cfg.scope
	}
	if len(namesBlock) > 0 {
		crdSpec["names"] = namesBlock
	}
	if cfg.crdVersions != nil {
		versionsSlice := make([]interface{}, 0, len(cfg.crdVersions))
		for _, v := range cfg.crdVersions {
			versionsSlice = append(versionsSlice, v)
		}
		crdSpec["versions"] = versionsSlice
	}
	if len(crdSpec) > 0 {
		schemaMap["crd"] = map[string]interface{}{
			"spec": crdSpec,
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

// NewCatalogRGD creates a models.CatalogRGD for service-level tests.
func NewCatalogRGD(name, namespace string, opts ...CatalogRGDOption) models.CatalogRGD {
	rgd := models.CatalogRGD{
		Name:        name,
		Namespace:   namespace,
		Description: "Test RGD " + name,
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
