// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

// ResourceAggregationResponse is the response for the resource aggregation endpoint.
type ResourceAggregationResponse struct {
	Items         []AggregatedResource     `json:"items"`
	TotalCount    int                      `json:"totalCount"`
	ClusterStatus map[string]ClusterStatus `json:"clusterStatus,omitempty"`
}

// AggregatedResource represents a single resource from a remote cluster.
type AggregatedResource struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Age       string `json:"age"`
}

// ClusterStatus represents the connectivity state of a remote cluster.
type ClusterStatus struct {
	Phase   string `json:"phase"`
	Message string `json:"message,omitempty"`
}
