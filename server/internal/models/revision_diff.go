// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

// DiffField represents a single changed field in a revision diff.
type DiffField struct {
	// Path is the dot-delimited field path (e.g., "spec.resources[0].apiVersion").
	Path string `json:"path"`
	// OldValue is the value in the older revision (nil for added fields).
	OldValue interface{} `json:"oldValue,omitempty"`
	// NewValue is the value in the newer revision (nil for removed fields).
	NewValue interface{} `json:"newValue,omitempty"`
}

// RevisionDiff represents the structured diff between two RGD revisions.
type RevisionDiff struct {
	// RGDName is the name of the ResourceGraphDefinition.
	RGDName string `json:"rgdName"`
	// Rev1 is the older revision number.
	Rev1 int `json:"rev1"`
	// Rev2 is the newer revision number.
	Rev2 int `json:"rev2"`
	// Added contains fields present in rev2 but not in rev1.
	Added []DiffField `json:"added"`
	// Removed contains fields present in rev1 but not in rev2.
	Removed []DiffField `json:"removed"`
	// Modified contains fields that exist in both revisions but with different values.
	Modified []DiffField `json:"modified"`
	// Identical is true when there are no differences between the two revisions.
	Identical bool `json:"identical"`
}
