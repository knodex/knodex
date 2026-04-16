// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

import "time"

// GraphRevisionCondition represents a condition on a GraphRevision.
type GraphRevisionCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// GraphRevision represents a KRO GraphRevision CRD.
type GraphRevision struct {
	RevisionNumber  int                      `json:"revisionNumber"`
	RGDName         string                   `json:"rgdName"`
	Namespace       string                   `json:"namespace"`
	Conditions      []GraphRevisionCondition `json:"conditions"`
	ContentHash     string                   `json:"contentHash,omitempty"`
	CreatedAt       time.Time                `json:"createdAt"`
	Labels          map[string]string        `json:"labels,omitempty"`
	Annotations     map[string]string        `json:"annotations,omitempty"`
	ResourceVersion string                   `json:"resourceVersion,omitempty"`
	// Snapshot contains the frozen RGD spec (only populated for single get, not list).
	Snapshot map[string]interface{} `json:"snapshot,omitempty"`
}

// GraphRevisionList is a paginated list of GraphRevisions.
type GraphRevisionList struct {
	Items      []GraphRevision `json:"items"`
	TotalCount int             `json:"totalCount"`
}
