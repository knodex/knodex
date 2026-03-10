// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package metadata

import "fmt"

// ValidateInstanceLabels checks that a set of labels contains the required
// KRO instance labels. Returns an error describing any missing labels.
func ValidateInstanceLabels(labels map[string]string) error {
	required := []string{
		ResourceGraphDefinitionNameLabel,
		InstanceLabel,
		InstanceIDLabel,
	}
	var missing []string
	for _, key := range required {
		if labels[key] == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required KRO instance labels: %v", missing)
	}
	return nil
}
