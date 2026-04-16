// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package models

import "time"

// ChildResource represents a single Kubernetes resource created by KRO
// as part of an instance's resource graph.
type ChildResource struct {
	// Name is the child resource name
	Name string `json:"name"`
	// Namespace is the child resource namespace (empty for cluster-scoped)
	Namespace string `json:"namespace"`
	// Kind is the Kubernetes resource kind (e.g., "Deployment", "Service")
	Kind string `json:"kind"`
	// APIVersion is the Kubernetes API version (e.g., "apps/v1")
	APIVersion string `json:"apiVersion"`
	// NodeID identifies which resource node in the RGD graph produced this resource
	NodeID string `json:"nodeId"`
	// Health is the calculated health status
	Health InstanceHealth `json:"health"`
	// Phase is the resource phase from status (e.g., "Running", "Pending")
	Phase string `json:"phase,omitempty"`
	// Status is a human-readable status message from status.message (e.g., condition details)
	Status string `json:"status,omitempty"`
	// CreatedAt is when the resource was created
	CreatedAt time.Time `json:"createdAt"`
	// Labels from the resource metadata
	Labels map[string]string `json:"labels,omitempty"`
	// Cluster is the cluster name where this child resource lives (empty = management cluster)
	Cluster string `json:"cluster,omitempty"`
	// ClusterStatus indicates cluster connectivity ("unreachable" when cluster is down)
	ClusterStatus string `json:"clusterStatus,omitempty"`
}

// ChildResourceGroup groups child resources by their node-id within
// the RGD resource graph, providing per-node health summaries.
type ChildResourceGroup struct {
	// NodeID is the kro.run/node-id value grouping these resources
	NodeID string `json:"nodeId"`
	// Kind is the Kubernetes resource kind for this group
	Kind string `json:"kind"`
	// APIVersion is the Kubernetes API version for this group
	APIVersion string `json:"apiVersion"`
	// Count is the total number of resources in this group
	Count int `json:"count"`
	// ReadyCount is the number of healthy resources
	ReadyCount int `json:"readyCount"`
	// Health is the aggregated health for the group
	Health InstanceHealth `json:"health"`
	// Resources are the individual child resources in this group
	Resources []ChildResource `json:"resources"`
}

// ChildResourceResponse is the API response for listing child resources
// of an instance, grouped by node-id.
type ChildResourceResponse struct {
	// InstanceName is the parent instance name
	InstanceName string `json:"instanceName"`
	// InstanceNamespace is the parent instance namespace
	InstanceNamespace string `json:"instanceNamespace"`
	// InstanceKind is the parent instance kind
	InstanceKind string `json:"instanceKind"`
	// TotalCount is the total number of child resources across all groups
	TotalCount int `json:"totalCount"`
	// Groups are the child resources grouped by node-id
	Groups []ChildResourceGroup `json:"groups"`
	// ClusterUnreachable is true when one or more target clusters are unreachable
	ClusterUnreachable bool `json:"clusterUnreachable,omitempty"`
	// UnreachableClusters lists the names of clusters that are unreachable
	UnreachableClusters []string `json:"unreachableClusters,omitempty"`
}

// AggregateGroupHealth computes the overall health for a group from its resources.
// Resources with HealthNone (no health concept) are skipped. If every resource
// is HealthNone, the group itself returns HealthNone.
func AggregateGroupHealth(resources []ChildResource) InstanceHealth {
	if len(resources) == 0 {
		return HealthUnknown
	}

	hasUnhealthy := false
	hasDegraded := false
	hasProgressing := false
	hasUnknown := false
	assessableCount := 0

	for _, r := range resources {
		if r.Health == HealthNone {
			continue // resource type has no health concept — skip
		}
		assessableCount++
		switch r.Health {
		case HealthUnhealthy:
			hasUnhealthy = true
		case HealthDegraded:
			hasDegraded = true
		case HealthProgressing:
			hasProgressing = true
		case HealthUnknown:
			hasUnknown = true
		}
	}

	if assessableCount == 0 {
		return HealthNone // all resources in this group have no health concept
	}

	switch {
	case hasUnhealthy:
		return HealthUnhealthy
	case hasDegraded:
		return HealthDegraded
	case hasProgressing:
		return HealthProgressing
	case hasUnknown:
		return HealthUnknown
	default:
		return HealthHealthy
	}
}
