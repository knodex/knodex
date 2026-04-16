// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package deployment

import (
	"fmt"
	"strings"
)

// MaxSpecDepth is the maximum nesting depth allowed for instance spec maps.
// Shared across all validation call-sites to prevent drift.
const MaxSpecDepth = 10

// ValidationError represents a schema validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// yamlSpecialChars contains YAML injection sequences prohibited in spec map keys.
var yamlSpecialChars = []string{": ", "- ", "| ", "> ", "{{", "}}", "${"}

// ValidateSpecMap recursively validates the spec map to prevent YAML injection and DoS attacks.
// Checks for:
// - Excessive nesting depth (prevents stack overflow)
// - Malicious map keys (prevents YAML injection)
// - Excessively long string values (prevents memory exhaustion)
func ValidateSpecMap(spec map[string]interface{}, currentDepth, maxDepth int) error {
	// Check depth limit to prevent deeply nested structures
	if currentDepth > maxDepth {
		return &ValidationError{
			Field:   "spec",
			Message: fmt.Sprintf("spec nesting depth exceeds maximum of %d levels", maxDepth),
		}
	}

	for key, value := range spec {
		// Validate map keys to prevent YAML injection and null byte attacks
		if strings.ContainsAny(key, "\n\r\x00") {
			return &ValidationError{
				Field:   "spec",
				Message: fmt.Sprintf("spec contains key with prohibited control character: %q", key),
			}
		}

		// Check for YAML injection sequences in keys
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
			if err := ValidateSpecMap(nestedMap, currentDepth+1, maxDepth); err != nil {
				return fmt.Errorf("validating nested map key %q: %w", key, err)
			}
		}

		// Recursively validate slices (including nested slices)
		if slice, ok := value.([]interface{}); ok {
			if err := validateSliceItems(slice, currentDepth+1, maxDepth); err != nil {
				return fmt.Errorf("validating slice items for key %q: %w", key, err)
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

// validateSliceItems recursively validates items within slices, including nested slices,
// maps, and string values. This prevents attackers from hiding malicious content inside
// deeply nested arrays that would otherwise bypass ValidateSpecMap.
func validateSliceItems(slice []interface{}, currentDepth, maxDepth int) error {
	if currentDepth > maxDepth {
		return &ValidationError{
			Field:   "spec",
			Message: fmt.Sprintf("spec nesting depth exceeds maximum of %d levels", maxDepth),
		}
	}

	for i, item := range slice {
		switch v := item.(type) {
		case map[string]interface{}:
			if err := ValidateSpecMap(v, currentDepth, maxDepth); err != nil {
				return fmt.Errorf("validating map at slice index %d: %w", i, err)
			}
		case []interface{}:
			if err := validateSliceItems(v, currentDepth+1, maxDepth); err != nil {
				return fmt.Errorf("validating nested slice at index %d: %w", i, err)
			}
		case string:
			if len(v) > 65536 { // 64KB max per string
				return &ValidationError{
					Field:   "spec",
					Message: fmt.Sprintf("spec contains excessively long string at index %d", i),
				}
			}
		}
	}
	return nil
}
