// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package schema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema/types"

	k8sparser "github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/models"
)

// RGDSchemaIntent represents the structured intent extracted from an RGD's spec.schema
// using KRO's simpleschema parser. It carries field-level metadata (types, defaults,
// descriptions, required flags) that the enricher merges into the CRD-derived FormSchema.
type RGDSchemaIntent struct {
	// Fields maps dot-separated field paths (e.g., "name", "config.replicas")
	// to their parsed intent.
	Fields map[string]FieldIntent
}

// FieldIntent holds the structured metadata for a single schema field,
// parsed from a simpleschema definition like `string | default="foo"`.
//
// Only Default and Description are merged into the CRD-derived FormSchema.
// Validation constraints (required, pattern, min/max, enum) come from the
// CRD's OpenAPI schema, which is authoritative for types and validation.
type FieldIntent struct {
	// Type is the OpenAPI-compatible type string (string, integer, boolean, number, array, object).
	Type string
	// ElemType is the OpenAPI-compatible element type for array fields (e.g., "string" for []string).
	// Empty for non-array types.
	ElemType string
	// Default is the default value as a string, or empty if none.
	Default string
	// Description is the field description from the description marker.
	Description string
}

// ParseRGDSchema parses the spec.schema.spec section of an RGD's raw spec,
// using KRO's simpleschema.ParseField to extract structured field intent.
// Returns nil (not an error) if the schema section is missing or empty.
func ParseRGDSchema(rawSpec map[string]interface{}) (*RGDSchemaIntent, error) {
	if rawSpec == nil {
		return nil, nil
	}

	schemaMap, err := k8sparser.GetMap(rawSpec, "schema")
	if err != nil {
		return nil, nil
	}

	specMap, err := k8sparser.GetMap(schemaMap, "spec")
	if err != nil {
		return nil, nil
	}

	intent := &RGDSchemaIntent{
		Fields: make(map[string]FieldIntent),
	}

	if err := parseFieldsRecursive(specMap, "", intent); err != nil {
		return nil, fmt.Errorf("failed to parse RGD schema: %w", err)
	}

	return intent, nil
}

// parseFieldsRecursive walks a simpleschema spec map and populates the intent.
func parseFieldsRecursive(fields map[string]interface{}, prefix string, intent *RGDSchemaIntent) error {
	for name, value := range fields {
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		switch v := value.(type) {
		case string:
			// Leaf field: parse with simpleschema
			fi, err := parseFieldDefinition(v)
			if err != nil {
				return fmt.Errorf("field %q: %w", path, err)
			}
			intent.Fields[path] = fi

		case map[string]interface{}:
			// Nested object: recurse into children only.
			// No parent entry is stored — mergeRGDIntent walks the CRD property tree
			// and only needs leaf field metadata (defaults, descriptions).
			//
			// NOTE: KRO represents nested objects as Go maps in the RGD spec, not as
			// simpleschema strings. If KRO ever supports parent-level markers
			// (e.g., `object | description="..."`), this branch would need to detect
			// string values before recursing.
			if err := parseFieldsRecursive(v, path, intent); err != nil {
				return err
			}

		default:
			return fmt.Errorf("field %q: unexpected value type %T", path, value)
		}
	}
	return nil
}

// parseFieldDefinition parses a single simpleschema field definition string
// (e.g., `string | default="foo" required=true`) into a FieldIntent.
func parseFieldDefinition(definition string) (FieldIntent, error) {
	typ, markers, err := simpleschema.ParseField(definition)
	if err != nil {
		return FieldIntent{}, fmt.Errorf("ParseField(%q): %w", definition, err)
	}

	fi := FieldIntent{
		Type:     typeToOpenAPI(typ),
		ElemType: elemTypeToOpenAPI(typ),
	}

	// Only extract default and description markers.
	// Validation constraints (required, pattern, min/max, enum) come from the
	// CRD's OpenAPI schema and are not needed from the RGD.
	for _, m := range markers {
		switch m.MarkerType {
		case simpleschema.MarkerTypeDefault:
			fi.Default = m.Value
		case simpleschema.MarkerTypeDescription:
			fi.Description = m.Value
		}
	}

	return fi, nil
}

// BuildFormSchemaFromRGD constructs a FormSchema directly from an RGD's spec.schema
// without requiring a CRD. This is the degraded/preview path used when the CRD
// hasn't been generated yet by KRO. The resulting schema has all fields, types,
// defaults, and descriptions but no OpenAPI validation constraints (minLength,
// pattern, enum, format, min/max).
func BuildFormSchemaFromRGD(rgd *models.CatalogRGD) (*models.FormSchema, error) {
	if rgd == nil {
		return nil, fmt.Errorf("rgd cannot be nil")
	}

	intent, err := ParseRGDSchema(rgd.RawSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RGD schema: %w", err)
	}
	if intent == nil {
		return nil, fmt.Errorf("RGD has no schema.spec section")
	}

	// Extract group/kind/version from RGD metadata
	group, kind, version := extractAPIInfo(rgd)

	schema := &models.FormSchema{
		Name:            rgd.Name,
		Namespace:       rgd.Namespace,
		Group:           group,
		Kind:            kind,
		Version:         version,
		Title:           rgd.Name,
		Description:     rgd.Description,
		IsClusterScoped: rgd.IsClusterScoped,
		Properties:      make(map[string]models.FormProperty),
	}

	// Build form properties from field intents.
	// Fields are stored as dot-separated paths (e.g., "config.replicas").
	// We need to reconstruct the nested property tree.
	buildPropertiesFromIntent(schema.Properties, intent.Fields)

	return schema, nil
}

// extractAPIInfo extracts group, kind, and version from an RGD's metadata and raw spec.
func extractAPIInfo(rgd *models.CatalogRGD) (group, kind, version string) {
	// Use pre-computed APIVersion and Kind
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

	// Fallback: parse from RawSpec
	if rgd.RawSpec != nil {
		if schemaMap, err := k8sparser.GetMap(rgd.RawSpec, "schema"); err == nil {
			if g := k8sparser.GetStringOrDefault(schemaMap, "", "group"); g != "" && group == "" {
				group = g
			}
			if k := k8sparser.GetStringOrDefault(schemaMap, "", "kind"); k != "" && kind == "" {
				kind = k
			}
			if v := k8sparser.GetStringOrDefault(schemaMap, "", "version"); v != "" && version == "" {
				version = v
			}
			if apiVer := k8sparser.GetStringOrDefault(schemaMap, "", "apiVersion"); apiVer != "" && (group == "" || version == "") {
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
		}
	}

	if version == "" {
		version = "v1alpha1"
	}
	return group, kind, version
}

// buildPropertiesFromIntent reconstructs a nested FormProperty tree from flat
// dot-separated field paths in the intent map.
func buildPropertiesFromIntent(props map[string]models.FormProperty, fields map[string]FieldIntent) {
	for path, fi := range fields {
		parts := strings.Split(path, ".")
		insertProperty(props, parts, fi, "spec")
	}
}

// insertProperty inserts a FieldIntent into the nested property tree at the given path parts.
func insertProperty(props map[string]models.FormProperty, parts []string, fi FieldIntent, parentPath string) {
	if len(parts) == 0 {
		return
	}

	name := parts[0]
	currentPath := parentPath + "." + name

	if len(parts) == 1 {
		// Leaf field — create the FormProperty
		prop := models.FormProperty{
			Type:        fi.Type,
			Title:       name,
			Description: fi.Description,
			Path:        currentPath,
		}
		if fi.Default != "" {
			prop.Default = parseDefault(fi.Type, fi.Default)
		}
		// Set Items for array fields so the frontend knows the element type
		if fi.Type == "array" && fi.ElemType != "" {
			prop.Items = &models.FormProperty{
				Type: fi.ElemType,
			}
		}
		props[name] = prop
		return
	}

	// Intermediate object — ensure it exists
	existing, ok := props[name]
	if !ok {
		existing = models.FormProperty{
			Type:       "object",
			Title:      name,
			Path:       currentPath,
			Properties: make(map[string]models.FormProperty),
		}
	}
	if existing.Properties == nil {
		existing.Properties = make(map[string]models.FormProperty)
	}
	insertProperty(existing.Properties, parts[1:], fi, currentPath)
	props[name] = existing
}

// parseDefault converts a string default value to an appropriate Go type
// based on the field type.
func parseDefault(fieldType, value string) interface{} {
	switch fieldType {
	case "integer":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			return v
		}
	case "number":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
	case "boolean":
		if v, err := strconv.ParseBool(value); err == nil {
			return v
		}
	}
	return value
}

// elemTypeToOpenAPI extracts the element type for array (Slice) types.
// Returns empty string for non-array types.
func elemTypeToOpenAPI(typ types.Type) string {
	if s, ok := typ.(types.Slice); ok {
		return typeToOpenAPI(s.Elem)
	}
	return ""
}

// typeToOpenAPI converts a simpleschema Type to an OpenAPI type string.
func typeToOpenAPI(typ types.Type) string {
	switch t := typ.(type) {
	case types.Atomic:
		s := string(t)
		// simpleschema uses "float" but OpenAPI uses "number"
		if s == "float" {
			return "number"
		}
		return s
	case types.Slice:
		return "array"
	case types.Map:
		return "object"
	case types.Object:
		return "object"
	default:
		// Custom types and others default to string
		return "string"
	}
}
