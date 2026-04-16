// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package children provides child resource discovery for KRO instances
// using the kro.run/* labels that KRO sets on every resource it creates.
package children

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	krograph "github.com/kubernetes-sigs/kro/pkg/graph"

	"github.com/knodex/knodex/server/internal/k8s/parser"
	kroadapter "github.com/knodex/knodex/server/internal/kro/graph"
	"github.com/knodex/knodex/server/internal/kro/metadata"
	kroparser "github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/models"
)

// RGDProvider abstracts RGD lookup for testability.
type RGDProvider interface {
	GetRGD(namespace, name string) (*models.CatalogRGD, bool)
}

// InstanceProvider abstracts instance lookup for testability.
type InstanceProvider interface {
	GetInstance(namespace, kind, name string) (*models.Instance, bool)
}

// RemoteClientProvider abstracts access to remote cluster dynamic clients.
type RemoteClientProvider interface {
	GetDynamicClient(clusterRef string) (dynamic.Interface, error)
	IsClusterReachable(clusterRef string) bool
}

// GraphProvider abstracts KRO graph lookup for the children service.
type GraphProvider interface {
	GetGraph(namespace, name string) *krograph.Graph
}

// Service discovers child resources of KRO instances via label selectors.
type Service struct {
	dynamicClient        dynamic.Interface
	rgdProvider          RGDProvider
	instanceProvider     InstanceProvider
	resourceParser       *kroparser.ResourceParser
	graphProvider        GraphProvider
	remoteClientProvider RemoteClientProvider
	logger               *slog.Logger
}

// SetRemoteClientProvider sets the remote client provider for multi-cluster queries.
func (s *Service) SetRemoteClientProvider(provider RemoteClientProvider) {
	s.remoteClientProvider = provider
}

// NewService creates a new child resource discovery service.
// graphProvider is optional — when non-nil, it provides pre-built KRO graphs
// for richer resource metadata. When nil, falls back to the parser.
func NewService(
	dynamicClient dynamic.Interface,
	rgdProvider RGDProvider,
	instanceProvider InstanceProvider,
	resourceParser *kroparser.ResourceParser,
	graphProvider GraphProvider,
	logger *slog.Logger,
) *Service {
	return &Service{
		dynamicClient:    dynamicClient,
		rgdProvider:      rgdProvider,
		instanceProvider: instanceProvider,
		resourceParser:   resourceParser,
		graphProvider:    graphProvider,
		logger:           logger,
	}
}

// ListChildResources discovers all child resources for the given instance
// by querying the K8s API with KRO label selectors.
func (s *Service) ListChildResources(ctx context.Context, namespace, kind, name string) (*models.ChildResourceResponse, error) {
	if kind == "" || name == "" {
		return nil, fmt.Errorf("kind and name are required")
	}

	// Validate instance exists
	instance, found := s.instanceProvider.GetInstance(namespace, kind, name)
	if !found {
		return nil, fmt.Errorf("instance %s/%s/%s not found", namespace, kind, name)
	}

	// Get the parent RGD to discover resource types
	rgd, rgdFound := s.rgdProvider.GetRGD(instance.RGDNamespace, instance.RGDName)
	if !rgdFound || rgd == nil {
		return nil, fmt.Errorf("RGD %s/%s not found for instance", instance.RGDNamespace, instance.RGDName)
	}

	// Get resource definitions (which kinds to search for) from cached graph or parser
	var resourceGraph *kroparser.ResourceGraph
	if s.graphProvider != nil {
		if g := s.graphProvider.GetGraph(rgd.Namespace, rgd.Name); g != nil {
			adapter := kroadapter.NewUIGraphAdapter(g)
			resourceGraph = adapter.GetResourceGraph(rgd.Name, rgd.Namespace, rgd.RawSpec)
		}
	}
	if resourceGraph == nil {
		var err error
		resourceGraph, err = s.resourceParser.ParseRGDResources(rgd.Name, rgd.Namespace, rgd.RawSpec)
		if err != nil {
			return nil, fmt.Errorf("parsing RGD resources: %w", err)
		}
	}

	// Build label selector for this instance's children
	selector := buildChildLabelSelector(name, namespace, kind)

	// Determine which dynamic client to use based on target cluster annotation
	targetCluster := instance.TargetCluster
	client := s.dynamicClient
	var clusterUnreachable bool
	var unreachableClusters []string

	if targetCluster != "" && s.remoteClientProvider != nil {
		if !s.remoteClientProvider.IsClusterReachable(targetCluster) {
			unreachableClusters = []string{targetCluster}
			s.logger.Warn("target cluster unreachable, returning empty children",
				"cluster", targetCluster, "instance", name)

			return &models.ChildResourceResponse{
				InstanceName:        instance.Name,
				InstanceNamespace:   instance.Namespace,
				InstanceKind:        instance.Kind,
				TotalCount:          0,
				Groups:              nil,
				ClusterUnreachable:  true,
				UnreachableClusters: unreachableClusters,
			}, nil
		}

		remoteClient, err := s.remoteClientProvider.GetDynamicClient(targetCluster)
		if err != nil {
			s.logger.Warn("failed to get remote dynamic client, falling back to unreachable",
				"cluster", targetCluster, "error", err)

			return &models.ChildResourceResponse{
				InstanceName:        instance.Name,
				InstanceNamespace:   instance.Namespace,
				InstanceKind:        instance.Kind,
				TotalCount:          0,
				Groups:              nil,
				ClusterUnreachable:  true,
				UnreachableClusters: []string{targetCluster},
			}, nil
		}
		client = remoteClient
	}

	// Query each resource type defined in the RGD
	var allChildren []models.ChildResource
	for _, resDef := range resourceGraph.Resources {
		if resDef.ExternalRef != nil {
			continue // Skip external references — they aren't owned children
		}

		gvr, err := resolveGVR(resDef.APIVersion, resDef.Kind)
		if err != nil {
			s.logger.Warn("skipping resource type: cannot resolve GVR",
				"kind", resDef.Kind,
				"apiVersion", resDef.APIVersion,
				"error", err,
			)
			continue
		}

		children, err := s.queryChildResourcesWithClient(ctx, client, gvr, instance.Namespace, selector)
		if err != nil {
			// If the remote cluster fails mid-query, mark as unreachable and return partial results
			if targetCluster != "" {
				s.logger.Warn("remote cluster query failed mid-request",
					"cluster", targetCluster, "gvr", gvr.String(), "error", err)
				clusterUnreachable = true
				unreachableClusters = []string{targetCluster}
				continue
			}
			s.logger.Warn("failed to list child resources",
				"gvr", gvr.String(),
				"namespace", instance.Namespace,
				"error", err,
			)
			continue
		}

		// Tag children with cluster name
		if targetCluster != "" {
			for i := range children {
				children[i].Cluster = targetCluster
			}
		}
		allChildren = append(allChildren, children...)
	}

	// Group by node-id
	groups := groupByNodeID(allChildren)

	return &models.ChildResourceResponse{
		InstanceName:        instance.Name,
		InstanceNamespace:   instance.Namespace,
		InstanceKind:        instance.Kind,
		TotalCount:          len(allChildren),
		Groups:              groups,
		ClusterUnreachable:  clusterUnreachable,
		UnreachableClusters: unreachableClusters,
	}, nil
}

// buildChildLabelSelector constructs the label selector string for finding
// child resources of a specific instance.
func buildChildLabelSelector(instanceName, instanceNamespace, instanceKind string) string {
	// Use the kro.run/ prefix labels (current KRO standard).
	// When KRO migrates to internal.kro.run/, the labels on resources will change
	// and we'll need to update the selector prefix. LabelWithFallback handles
	// reading; selectors must match what's on the resources.
	parts := []string{
		fmt.Sprintf("%s=%s", metadata.InstanceLabel, instanceName),
		fmt.Sprintf("%s=%s", metadata.InstanceKindLabel, instanceKind),
	}
	if instanceNamespace != "" {
		parts = append(parts, fmt.Sprintf("%s=%s", metadata.InstanceNamespaceLabel, instanceNamespace))
	}
	return strings.Join(parts, ",")
}

// queryChildResourcesWithClient lists resources matching the label selector for a given GVR
// using the provided dynamic client (management or remote).
func (s *Service) queryChildResourcesWithClient(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace, selector string) ([]models.ChildResource, error) {
	var list *unstructured.UnstructuredList
	var err error

	if namespace != "" {
		list, err = client.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
	} else {
		list, err = client.Resource(gvr).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", gvr.Resource, err)
	}

	children := make([]models.ChildResource, 0, len(list.Items))
	for i := range list.Items {
		child := unstructuredToChildResource(&list.Items[i])
		children = append(children, child)
	}
	return children, nil
}

// unstructuredToChildResource converts an unstructured K8s object to a ChildResource model.
func unstructuredToChildResource(u *unstructured.Unstructured) models.ChildResource {
	labels := parser.GetLabels(u)
	nodeID := metadata.LabelWithFallback(labels, metadata.InternalNodeIDLabel, metadata.NodeIDLabel)

	status := parser.GetStatusOrEmpty(u)
	health := deriveChildHealth(u, status)
	phase := parser.GetStringOrDefault(status, "", "phase")
	// status.message provides human-readable context beyond the health enum
	// (e.g. condition messages, error details). Empty when the resource is healthy.
	statusMsg := parser.GetStringOrDefault(status, "", "message")

	createdAt := u.GetCreationTimestamp().Time
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	return models.ChildResource{
		Name:       parser.GetName(u),
		Namespace:  parser.GetNamespace(u),
		Kind:       u.GetKind(),
		APIVersion: u.GetAPIVersion(),
		NodeID:     nodeID,
		Health:     health,
		Phase:      phase,
		Status:     statusMsg,
		CreatedAt:  createdAt,
		Labels:     labels,
	}
}

// statuslessKinds contains resource kinds that have no meaningful status subresource.
// These are healthy by definition once they exist in the cluster.
var statuslessKinds = map[string]bool{
	"ConfigMap":          true,
	"Secret":             true,
	"ServiceAccount":     true,
	"Role":               true,
	"ClusterRole":        true,
	"RoleBinding":        true,
	"ClusterRoleBinding": true,
	"NetworkPolicy":      true,
	"Endpoints":          true,
}

// deriveChildHealth determines health from status conditions, replica counts, or phase.
func deriveChildHealth(u *unstructured.Unstructured, status map[string]interface{}) models.InstanceHealth {
	kind := u.GetKind()

	// Resources with no health concept: don't fabricate a status.
	if statuslessKinds[kind] {
		return models.HealthNone
	}

	// Service: no conditions or phase — health not assessable.
	if kind == "Service" {
		return models.HealthNone
	}

	// StatefulSet: replica-based health via readyReplicas.
	if kind == "StatefulSet" {
		return deriveReplicaHealth(
			parser.GetInt64OrDefault(status, 0, "replicas"),
			parser.GetInt64OrDefault(status, 0, "readyReplicas"),
		)
	}

	// DaemonSet: uses desiredNumberScheduled / numberReady.
	if kind == "DaemonSet" {
		return deriveReplicaHealth(
			parser.GetInt64OrDefault(status, 0, "desiredNumberScheduled"),
			parser.GetInt64OrDefault(status, 0, "numberReady"),
		)
	}

	// Job: Complete / Failed conditions (not Ready/Available).
	if kind == "Job" {
		if parser.IsConditionTrue(u, "Complete") {
			return models.HealthHealthy
		}
		if parser.IsConditionTrue(u, "Failed") {
			return models.HealthUnhealthy
		}
		return models.HealthProgressing
	}

	// Check Ready condition first (Pods, custom resources, etc.)
	readyCond := parser.GetCondition(u, "Ready")
	if readyCond != nil {
		condStatus := parser.GetStringOrDefault(readyCond, "", "status")
		switch condStatus {
		case "True":
			return models.HealthHealthy
		case "False":
			reason := strings.ToLower(parser.GetStringOrDefault(readyCond, "", "reason"))
			if strings.Contains(reason, "progress") || strings.Contains(reason, "pending") {
				return models.HealthProgressing
			}
			return models.HealthUnhealthy
		case "Unknown":
			return models.HealthProgressing
		}
	}

	// Check Available condition (Deployments).
	availCond := parser.GetCondition(u, "Available")
	if availCond != nil {
		condStatus := parser.GetStringOrDefault(availCond, "", "status")
		if condStatus == "True" {
			return models.HealthHealthy
		}
		if condStatus == "False" {
			return models.HealthUnhealthy
		}
	}

	// Fallback to phase (Pods, PVCs, PVs, Namespaces, etc.)
	phase := parser.GetStringOrDefault(status, "", "phase")
	if phase != "" {
		switch strings.ToLower(phase) {
		case "running", "ready", "active", "healthy", "bound", "succeeded":
			return models.HealthHealthy
		case "pending", "creating", "initializing", "progressing":
			return models.HealthProgressing
		case "failed", "error", "unhealthy", "crashloopbackoff":
			return models.HealthUnhealthy
		case "degraded", "warning":
			return models.HealthDegraded
		}
	}

	// No status at all: can't assess health — don't fabricate a signal.
	if len(status) == 0 {
		return models.HealthNone
	}

	return models.HealthUnknown
}

// deriveReplicaHealth computes health from desired vs ready replica counts.
// Covers StatefulSet (replicas/readyReplicas) and DaemonSet (desiredNumberScheduled/numberReady).
func deriveReplicaHealth(desired, ready int64) models.InstanceHealth {
	if desired == 0 {
		// Scaled to zero intentionally — healthy.
		return models.HealthHealthy
	}
	if ready >= desired {
		return models.HealthHealthy
	}
	if ready == 0 {
		return models.HealthProgressing
	}
	return models.HealthDegraded
}

// groupByNodeID groups child resources by their kro.run/node-id label value.
func groupByNodeID(children []models.ChildResource) []models.ChildResourceGroup {
	groupMap := make(map[string]*models.ChildResourceGroup)
	var order []string

	for _, child := range children {
		nodeID := child.NodeID
		if nodeID == "" {
			nodeID = "unknown"
		}

		group, exists := groupMap[nodeID]
		if !exists {
			group = &models.ChildResourceGroup{
				NodeID:     nodeID,
				Kind:       child.Kind,
				APIVersion: child.APIVersion,
			}
			groupMap[nodeID] = group
			order = append(order, nodeID)
		}

		group.Resources = append(group.Resources, child)
		group.Count++
		if child.Health == models.HealthHealthy {
			group.ReadyCount++
		}
	}

	// Compute aggregate health and build ordered result
	groups := make([]models.ChildResourceGroup, 0, len(order))
	for _, nodeID := range order {
		group := groupMap[nodeID]
		group.Health = models.AggregateGroupHealth(group.Resources)
		groups = append(groups, *group)
	}
	return groups
}

// resolveGVR converts an apiVersion + kind to a GroupVersionResource.
// Uses a well-known mapping for common Kubernetes kinds. Unknown kinds fall
// back to the lowercase+s plural form (e.g. "Widget" → "widgets"), which
// works for most CRDs that follow standard naming conventions. If the guessed
// plural is wrong the subsequent API List call will fail and be logged as a
// warning (the resource type is then skipped gracefully).
func resolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("parsing apiVersion %q: %w", apiVersion, err)
	}

	return schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: kindToResource(kind),
	}, nil
}

// wellKnownResources maps Kubernetes Kinds to their plural resource names.
// Covers >95% of real-world RGD resources. Package-level to avoid repeated allocation.
var wellKnownResources = map[string]string{
	// Core v1
	"Pod":                   "pods",
	"Service":               "services",
	"ConfigMap":             "configmaps",
	"Secret":                "secrets",
	"ServiceAccount":        "serviceaccounts",
	"Namespace":             "namespaces",
	"PersistentVolumeClaim": "persistentvolumeclaims",
	"PersistentVolume":      "persistentvolumes",
	"Endpoints":             "endpoints",
	// apps/v1
	"Deployment":  "deployments",
	"StatefulSet": "statefulsets",
	"DaemonSet":   "daemonsets",
	"ReplicaSet":  "replicasets",
	// batch/v1
	"Job":     "jobs",
	"CronJob": "cronjobs",
	// networking.k8s.io/v1
	"Ingress":       "ingresses",
	"NetworkPolicy": "networkpolicies",
	// rbac.authorization.k8s.io/v1
	"Role":               "roles",
	"RoleBinding":        "rolebindings",
	"ClusterRole":        "clusterroles",
	"ClusterRoleBinding": "clusterrolebindings",
	// policy/v1
	"PodDisruptionBudget": "poddisruptionbudgets",
	// autoscaling/v2
	"HorizontalPodAutoscaler": "horizontalpodautoscalers",
}

// kindToResource maps a Kubernetes Kind to its plural resource name.
// For uncommon types, the caller should use K8s discovery (future enhancement).
func kindToResource(kind string) string {
	if r, ok := wellKnownResources[kind]; ok {
		return r
	}

	// Fallback: lowercase + "s" (works for many CRDs following conventions)
	return strings.ToLower(kind) + "s"
}
