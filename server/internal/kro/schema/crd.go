// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/models"
)

// CachedSchema holds a cached schema with expiration
type CachedSchema struct {
	Schema    *models.FormSchema
	ExpiresAt time.Time
	Error     error
	// Degraded indicates this was built from RGD-only (no CRD available)
	Degraded bool
}

// Extractor extracts schemas from CRDs created by Kro
type Extractor struct {
	apiextClient     apiextensionsclient.Interface
	cache            map[string]*CachedSchema
	cacheMu          sync.RWMutex
	cacheTTL         time.Duration
	degradedCacheTTL time.Duration
	logger           *slog.Logger
}

// NewExtractor creates a new schema extractor
func NewExtractor(cfg *config.Kubernetes) (*Extractor, error) {
	var restConfig *rest.Config
	var err error

	if cfg.InCluster {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
		}
	}

	apiextClient, err := apiextensionsclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	return &Extractor{
		apiextClient:     apiextClient,
		cache:            make(map[string]*CachedSchema),
		cacheTTL:         5 * time.Minute,
		degradedCacheTTL: 30 * time.Second,
		logger:           slog.Default().With("component", "schema-extractor"),
	}, nil
}

// NewExtractorWithClient creates an Extractor with a provided apiextensions client.
// This is primarily useful for testing where a fake client is injected.
func NewExtractorWithClient(client apiextensionsclient.Interface) *Extractor {
	return &Extractor{
		apiextClient:     client,
		cache:            make(map[string]*CachedSchema),
		cacheTTL:         5 * time.Minute,
		degradedCacheTTL: 30 * time.Second,
		logger:           slog.Default().With("component", "schema-extractor"),
	}
}

// ExtractSchema extracts the schema for a given RGD
func (e *Extractor) ExtractSchema(ctx context.Context, rgd *models.CatalogRGD) (*models.FormSchema, error) {
	cacheKey := fmt.Sprintf("%s/%s", rgd.Namespace, rgd.Name)

	// Check cache first
	e.cacheMu.RLock()
	cached, ok := e.cache[cacheKey]
	e.cacheMu.RUnlock()

	if ok && time.Now().Before(cached.ExpiresAt) {
		if cached.Error != nil {
			return nil, cached.Error
		}
		return cached.Schema, nil
	}

	// Extract schema
	schema, err := e.extractSchemaFromCRD(ctx, rgd)

	// Determine TTL: use shorter TTL for degraded (not-found) results so the
	// full schema replaces it quickly once the CRD becomes available.
	ttl := e.cacheTTL
	degraded := false
	if err != nil && IsNotFoundError(err) {
		ttl = e.degradedCacheTTL
		degraded = true
	}

	// Cache the result (even errors, to avoid hammering the API)
	e.cacheMu.Lock()
	e.cache[cacheKey] = &CachedSchema{
		Schema:    schema,
		ExpiresAt: time.Now().Add(ttl),
		Error:     err,
		Degraded:  degraded,
	}
	e.cacheMu.Unlock()

	return schema, err
}

// InvalidateCache removes a cached schema
func (e *Extractor) InvalidateCache(namespace, name string) {
	cacheKey := fmt.Sprintf("%s/%s", namespace, name)
	e.cacheMu.Lock()
	delete(e.cache, cacheKey)
	e.cacheMu.Unlock()
	e.logger.Debug("invalidated schema cache", "key", cacheKey)
}

// IsNotFoundError checks if the error (potentially wrapped) is a Kubernetes NotFound error.
func IsNotFoundError(err error) bool {
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		return apierrors.IsNotFound(statusErr)
	}
	return false
}

// extractSchemaFromCRD fetches the CRD and extracts the OpenAPI schema
func (e *Extractor) extractSchemaFromCRD(ctx context.Context, rgd *models.CatalogRGD) (*models.FormSchema, error) {
	// Get group and kind from RGD spec
	group, kind, version, err := e.extractCRDInfo(rgd)
	if err != nil {
		return nil, fmt.Errorf("failed to extract CRD info: %w", err)
	}

	e.logger.Debug("looking up CRD",
		"group", group,
		"kind", kind,
		"version", version)

	// List all CRDs and filter by group+kind instead of constructing the CRD
	// name from a plural form. This avoids needing to know the plural upfront,
	// which is fragile for irregular plurals (e.g., "Proxy" → "proxies", not "proxys").
	// The overhead of listing is acceptable since schema extraction is infrequent
	// and user-initiated (deploy form rendering).
	crds, err := e.apiextClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	var crd *apiextensionsv1.CustomResourceDefinition
	for i := range crds.Items {
		if crds.Items[i].Spec.Group == group && crds.Items[i].Spec.Names.Kind == kind {
			crd = &crds.Items[i]
			break
		}
	}
	if crd == nil {
		e.logger.Warn("CRD not found for group+kind", "group", group, "kind", kind, "rgdName", rgd.Name, "rgdNamespace", rgd.Namespace)
		// Wrap as a K8s NotFound error so the caller (ExtractSchema) uses the
		// shorter degraded-cache TTL, matching the previous Get()-based behavior.
		return nil, apierrors.NewNotFound(
			k8sschema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
			fmt.Sprintf("group=%s kind=%s", group, kind),
		)
	}

	// Find the schema for the requested version
	var openAPISchema *apiextensionsv1.JSONSchemaProps
	for _, v := range crd.Spec.Versions {
		if v.Name == version || version == "" {
			if v.Schema != nil && v.Schema.OpenAPIV3Schema != nil {
				openAPISchema = v.Schema.OpenAPIV3Schema
				if version == "" {
					version = v.Name
				}
				break
			}
		}
	}

	if openAPISchema == nil {
		return nil, fmt.Errorf("no OpenAPI schema found in CRD for group=%s kind=%s", group, kind)
	}

	// Convert OpenAPI schema to form schema
	formSchema := e.convertToFormSchema(rgd, group, kind, version, openAPISchema)

	return formSchema, nil
}

// extractCRDInfo extracts the CRD group, kind, and version from the RGD spec.
// It uses pre-computed CatalogRGD.APIVersion and Kind (which include group-prepending
// logic from the watcher) as the primary source, falling back to rawSpec parsing.
func (e *Extractor) extractCRDInfo(rgd *models.CatalogRGD) (group, kind, version string, err error) {
	// Primary source: use pre-computed APIVersion and Kind from the watcher.
	// The watcher already handles group-prepending for short apiVersions
	// (e.g., "v1alpha1" → "kro.run/v1alpha1") which rawSpec parsing misses.
	if rgd.APIVersion != "" {
		parts := strings.Split(rgd.APIVersion, "/")
		if len(parts) == 2 {
			group = parts[0]
			version = parts[1]
		} else if len(parts) == 1 {
			version = parts[0]
		}
	}
	kind = rgd.Kind

	// Fallback: parse from RawSpec if pre-computed values are incomplete
	if (group == "" || kind == "" || version == "") && rgd.RawSpec != nil {
		// Try top-level spec.apiVersion / spec.kind
		if apiVersion, ok := rgd.RawSpec["apiVersion"].(string); ok && (group == "" || version == "") {
			parts := strings.Split(apiVersion, "/")
			if len(parts) == 2 {
				if group == "" {
					group = parts[0]
				}
				if version == "" {
					version = parts[1]
				}
			} else if len(parts) == 1 && version == "" {
				version = parts[0]
			}
		}
		if k, ok := rgd.RawSpec["kind"].(string); ok && kind == "" {
			kind = k
		}

		// Try spec.schema.apiVersion / spec.schema.kind / spec.schema.group
		if schema, ok := rgd.RawSpec["schema"].(map[string]interface{}); ok {
			if apiVer, ok := schema["apiVersion"].(string); ok && (group == "" || version == "") {
				parts := strings.Split(apiVer, "/")
				if len(parts) == 2 {
					if group == "" {
						group = parts[0]
					}
					if version == "" {
						version = parts[1]
					}
				} else if len(parts) == 1 && version == "" {
					version = parts[0]
				}
			}
			if g, ok := schema["group"].(string); ok && group == "" {
				group = g
			}
			if k, ok := schema["kind"].(string); ok && kind == "" {
				kind = k
			}
			if v, ok := schema["version"].(string); ok && version == "" {
				version = v
			}
		}
	}

	if group == "" || kind == "" {
		return "", "", "", fmt.Errorf("could not determine group and kind from RGD spec: group=%q, kind=%q", group, kind)
	}

	// Default version if not specified
	if version == "" {
		version = "v1alpha1"
	}

	e.logger.Debug("extracted CRD info",
		"group", group,
		"kind", kind,
		"version", version,
		"rgdName", rgd.Name)

	return group, kind, version, nil
}

// convertToFormSchema converts an OpenAPI schema to a form-friendly schema
func (e *Extractor) convertToFormSchema(rgd *models.CatalogRGD, group, kind, version string, schema *apiextensionsv1.JSONSchemaProps) *models.FormSchema {
	formSchema := &models.FormSchema{
		Name:        rgd.Name,
		Namespace:   rgd.Namespace,
		Group:       group,
		Kind:        kind,
		Version:     version,
		Title:       rgd.Name,
		Description: rgd.Description,
		Properties:  make(map[string]models.FormProperty),
	}

	// Extract spec schema (this is what users fill in)
	if specSchema, ok := schema.Properties["spec"]; ok {
		formSchema.Properties = e.convertProperties(specSchema.Properties, "spec")
		formSchema.Required = specSchema.Required

		e.logger.Debug("extracted form schema from CRD",
			"rgdName", rgd.Name,
			"propertyCount", len(formSchema.Properties),
			"requiredFields", formSchema.Required)
	} else {
		e.logger.Warn("CRD has no spec properties",
			"rgdName", rgd.Name,
			"topLevelProperties", schemaPropertyNames(schema))
	}

	return formSchema
}

// convertProperties converts OpenAPI properties to form properties
func (e *Extractor) convertProperties(props map[string]apiextensionsv1.JSONSchemaProps, parentPath string) map[string]models.FormProperty {
	result := make(map[string]models.FormProperty)

	for name, prop := range props {
		path := name
		if parentPath != "" {
			path = parentPath + "." + name
		}
		result[name] = e.convertProperty(&prop, path)
	}

	return result
}

// convertProperty converts a single OpenAPI property to a form property
func (e *Extractor) convertProperty(prop *apiextensionsv1.JSONSchemaProps, path string) models.FormProperty {
	formProp := models.FormProperty{
		Type:        prop.Type,
		Title:       prop.Title,
		Description: prop.Description,
		Format:      prop.Format,
		Pattern:     prop.Pattern,
		Path:        path,
		Nullable:    prop.Nullable,
	}

	// Handle default value - decode JSON to get actual value
	if prop.Default != nil && len(prop.Default.Raw) > 0 {
		var defaultVal interface{}
		if err := json.Unmarshal(prop.Default.Raw, &defaultVal); err == nil {
			formProp.Default = defaultVal
		} else {
			// Fallback to raw string if JSON parsing fails
			formProp.Default = string(prop.Default.Raw)
		}
	}

	// Handle enum - decode JSON values
	if len(prop.Enum) > 0 {
		formProp.Enum = make([]interface{}, len(prop.Enum))
		for i, v := range prop.Enum {
			var enumVal interface{}
			if err := json.Unmarshal(v.Raw, &enumVal); err == nil {
				formProp.Enum[i] = enumVal
			} else {
				formProp.Enum[i] = string(v.Raw)
			}
		}
	}

	// Handle numeric constraints
	if prop.Minimum != nil {
		formProp.Minimum = prop.Minimum
	}
	if prop.Maximum != nil {
		formProp.Maximum = prop.Maximum
	}

	// Handle string constraints
	if prop.MinLength != nil {
		minLen := int(*prop.MinLength)
		formProp.MinLength = &minLen
	}
	if prop.MaxLength != nil {
		maxLen := int(*prop.MaxLength)
		formProp.MaxLength = &maxLen
	}

	// Handle x-kubernetes-preserve-unknown-fields
	if prop.XPreserveUnknownFields != nil && *prop.XPreserveUnknownFields {
		formProp.XKubernetesPreserveUnknownFields = true
	}

	// Handle nested objects
	if prop.Type == "object" && len(prop.Properties) > 0 {
		formProp.Properties = e.convertProperties(prop.Properties, path)
		formProp.Required = prop.Required
	}

	// Handle arrays
	if prop.Type == "array" && prop.Items != nil && prop.Items.Schema != nil {
		itemProp := e.convertProperty(prop.Items.Schema, path+"[]")
		formProp.Items = &itemProp
	}

	return formProp
}

// schemaPropertyNames returns the top-level property names from an OpenAPI schema for logging
func schemaPropertyNames(schema *apiextensionsv1.JSONSchemaProps) []string {
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	return names
}
