// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package dex

import (
	"fmt"
	"os"
)

// writeConfigFile writes the Dex config YAML to a file with secure permissions.
func writeConfigFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing Dex config to %s: %w", path, err)
	}
	return nil
}

// readConfigFile reads a Dex config YAML file.
func readConfigFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading Dex config from %s: %w", path, err)
	}
	return data, nil
}

// toStringSlice converts an interface{} to []string (for YAML deserialized data).
func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	default:
		return nil
	}
}
