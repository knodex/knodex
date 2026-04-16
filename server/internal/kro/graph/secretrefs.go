// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package graph

import (
	"fmt"
	"strings"

	krograph "github.com/kubernetes-sigs/kro/pkg/graph"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema"

	"github.com/knodex/knodex/server/internal/kro/parser"
)

// ExtractSecretRefs extracts SecretRef entries from external nodes in a KRO graph.
// This preserves the same classification logic (provided/dynamic/fixed) as the
// parser-based version, adapted to read from graph.Node instead of ResourceDefinition.
//
// rawSpec is the RGD's raw spec (for schema.spec.externalRef description lookup).
func ExtractSecretRefs(g *krograph.Graph, rawSpec map[string]interface{}) []parser.SecretRef {
	if g == nil {
		return nil
	}

	schemaExternalRefs := extractSchemaExternalRefMap(rawSpec)

	var refs []parser.SecretRef
	for _, node := range g.Nodes {
		if node.Meta.Type != krograph.NodeTypeExternal {
			continue
		}
		if node.Template == nil || node.Template.GetKind() != "Secret" {
			continue
		}

		id := nodeID(node)
		internalID := node.Meta.ID

		ref := parser.SecretRef{
			ID:            id,
			ExternalRefID: internalID,
			Description:   extractExternalRefDescription(schemaExternalRefs, internalID),
		}

		// Extract name/namespace from externalRef metadata
		nameExpr := getTemplateMetadataField(node.Template.Object, "name")
		nsExpr := getTemplateMetadataField(node.Template.Object, "namespace")

		// Classify: provided (passthrough), dynamic (CEL), or fixed (literal)
		isPassthrough := internalID != "" &&
			nameExpr == fmt.Sprintf("${schema.spec.externalRef.%s.name}", internalID) &&
			(nsExpr == "" || nsExpr == fmt.Sprintf("${schema.spec.externalRef.%s.namespace}", internalID))
		isDynamic := !isPassthrough && (strings.Contains(nameExpr, "${") || strings.Contains(nsExpr, "${"))

		switch {
		case isPassthrough:
			ref.Type = "provided"
		case isDynamic:
			ref.Type = "dynamic"
			ref.NameExpr = nameExpr
			ref.NamespaceExpr = nsExpr
		default:
			ref.Type = "fixed"
			ref.Name = nameExpr
			ref.Namespace = nsExpr
		}

		refs = append(refs, ref)
	}
	return refs
}

// getTemplateMetadataField extracts a string field from an externalRef template's metadata.
func getTemplateMetadataField(obj map[string]interface{}, field string) string {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	val, ok := metadata[field].(string)
	if !ok {
		return ""
	}
	return val
}

// extractSchemaExternalRefMap extracts the schema.spec.externalRef map from an RGD spec.
func extractSchemaExternalRefMap(rawSpec map[string]interface{}) map[string]interface{} {
	if rawSpec == nil {
		return nil
	}
	schema, ok := rawSpec["schema"].(map[string]interface{})
	if !ok {
		return nil
	}
	spec, ok := schema["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	extRef, ok := spec["externalRef"].(map[string]interface{})
	if !ok {
		return nil
	}
	return extRef
}

// extractExternalRefDescription extracts the description from a schema externalRef field.
func extractExternalRefDescription(schemaExternalRefs map[string]interface{}, fieldName string) string {
	if schemaExternalRefs == nil || fieldName == "" {
		return ""
	}
	fieldMap, ok := schemaExternalRefs[fieldName].(map[string]interface{})
	if !ok {
		return ""
	}
	nameValue, ok := fieldMap["name"].(string)
	if !ok {
		return ""
	}
	_, markers, err := simpleschema.ParseField(nameValue)
	if err != nil {
		return ""
	}
	for _, m := range markers {
		if m.MarkerType == simpleschema.MarkerTypeDescription {
			return m.Value
		}
	}
	return ""
}
