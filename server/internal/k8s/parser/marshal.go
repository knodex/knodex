// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ToYAML marshals an object to YAML bytes.
// Returns an error if the object cannot be marshaled.
func ToYAML(obj interface{}) ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot marshal nil to YAML")
	}
	return yaml.Marshal(obj)
}

// ToYAMLString marshals an object to a YAML string.
// Returns an error if the object cannot be marshaled.
func ToYAMLString(obj interface{}) (string, error) {
	bytes, err := ToYAML(obj)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
