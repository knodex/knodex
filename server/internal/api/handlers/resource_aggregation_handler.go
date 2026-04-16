// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/rbac"
)

// supportedKinds maps user-facing kind names to the cache resource type (plural, lowercase).
var supportedKinds = map[string]string{
	"Certificate": "certificates",
	"Ingress":     "ingresses",
}

// RemoteResourceCacheProvider abstracts access to the remote resource cache for testability.
type RemoteResourceCacheProvider interface {
	Cache() *watcher.RemoteResourceCache
}

// ResourceAggregationHandler handles resource aggregation across clusters.
type ResourceAggregationHandler struct {
	projectService rbac.ProjectServiceInterface
	enforcer       rbac.PolicyEnforcer
	cacheProvider  RemoteResourceCacheProvider
	logger         *slog.Logger
}

// NewResourceAggregationHandler creates a new resource aggregation handler.
func NewResourceAggregationHandler(
	projectService rbac.ProjectServiceInterface,
	enforcer rbac.PolicyEnforcer,
	cacheProvider RemoteResourceCacheProvider,
) *ResourceAggregationHandler {
	return &ResourceAggregationHandler{
		projectService: projectService,
		enforcer:       enforcer,
		cacheProvider:  cacheProvider,
		logger:         slog.Default().With("handler", "resource-aggregation"),
	}
}

// ListProjectResources handles GET /api/v1/projects/{name}/resources
func (h *ResourceAggregationHandler) ListProjectResources(w http.ResponseWriter, r *http.Request) {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return // 401 already written
	}

	projectName := r.PathValue("name")

	// AC4: Authorize using instances/get on this project
	hasAccess, err := h.enforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, "instances/"+projectName+"/*", "get")
	if err != nil {
		h.logger.Error("authorization check failed", "user", userCtx.UserID, "project", projectName, "error", err)
		response.InternalError(w, "authorization check failed")
		return
	}
	if !hasAccess {
		response.Forbidden(w, "permission denied")
		return
	}

	// AC5: Validate kind query param
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		response.BadRequest(w, "missing required query parameter: kind", nil)
		return
	}
	cacheKind, ok := supportedKinds[kind]
	if !ok {
		supported := make([]string, 0, len(supportedKinds))
		for k := range supportedKinds {
			supported = append(supported, k)
		}
		sort.Strings(supported)
		response.BadRequest(w, fmt.Sprintf("unsupported kind: %s. Supported kinds: %s", kind, strings.Join(supported, ", ")), nil)
		return
	}

	// Get the project
	project, err := h.projectService.GetProject(r.Context(), projectName)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "project", projectName)
			return
		}
		h.logger.Error("failed to get project", "project", projectName, "error", err)
		response.InternalError(w, "failed to get project")
		return
	}

	// Determine namespace scope from project destinations
	var allowedNamespaces []string
	for _, dest := range project.Spec.Destinations {
		if dest.Namespace != "" {
			allowedNamespaces = append(allowedNamespaces, dest.Namespace)
		}
	}

	allowedNSSet := make(map[string]struct{}, len(allowedNamespaces))
	for _, ns := range allowedNamespaces {
		allowedNSSet[ns] = struct{}{}
	}

	// Remote resource aggregation is currently disabled pending Casbin-driven cluster targeting.
	// When re-enabled, cluster refs will come from Casbin policies rather than project CRD bindings.
	cache := h.cacheProvider.Cache()
	var clusterRefs []string // empty — no cluster bindings on Project CRD

	var items []AggregatedResource
	var unreachableStatus map[string]ClusterStatus

	for _, clusterRef := range clusterRefs {
		// Check cluster reachability from the remote watcher cache
		watchStatus := cache.GetClusterStatus(clusterRef)
		if watchStatus == watcher.RemoteWatchStatusUnreachable {
			if unreachableStatus == nil {
				unreachableStatus = make(map[string]ClusterStatus)
			}
			unreachableStatus[clusterRef] = ClusterStatus{
				Phase:   "unreachable",
				Message: "cluster is unreachable",
			}
			continue
		}

		// Also check project-level ClusterState for unreachable
		if isClusterUnreachable(project, clusterRef) && watchStatus == "" {
			if unreachableStatus == nil {
				unreachableStatus = make(map[string]ClusterStatus)
			}
			unreachableStatus[clusterRef] = ClusterStatus{
				Phase:   "unreachable",
				Message: "cluster is unreachable",
			}
			continue
		}

		// Query the cache for this cluster+kind
		resources := cache.List(clusterRef, cacheKind)
		for _, u := range resources {
			ns := u.GetNamespace()
			if _, allowed := allowedNSSet[ns]; !allowed {
				continue
			}
			items = append(items, toAggregatedResource(u, kind, clusterRef))
		}
	}

	if items == nil {
		items = []AggregatedResource{}
	}

	response.WriteJSON(w, http.StatusOK, ResourceAggregationResponse{
		Items:         items,
		TotalCount:    len(items),
		ClusterStatus: unreachableStatus,
	})
}

// isClusterUnreachable checks if the remote watcher reports the cluster as unreachable.
func isClusterUnreachable(_ *rbac.Project, _ string) bool {
	// Cluster reachability is now checked via the remote watcher cache directly,
	// not via project ClusterStates. This function is kept for interface compatibility.
	return false
}

// toAggregatedResource converts an unstructured resource to the API response model.
func toAggregatedResource(u *unstructured.Unstructured, kind, clusterRef string) AggregatedResource {
	return AggregatedResource{
		Name:      u.GetName(),
		Kind:      kind,
		Cluster:   clusterRef,
		Namespace: u.GetNamespace(),
		Status:    extractStatus(u, kind),
		Age:       formatAge(u.GetCreationTimestamp().Time),
	}
}

// extractStatus extracts a human-readable status from the unstructured resource.
func extractStatus(u *unstructured.Unstructured, kind string) string {
	switch kind {
	case "Certificate":
		// cert-manager Certificate: search for condition with type=="Ready"
		status, statusErr := parser.GetStatus(u)
		conditions, _ := parser.GetSlice(status, "conditions")
		if statusErr == nil && len(conditions) > 0 {
			for _, c := range conditions {
				cond, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				condType, _ := cond["type"].(string)
				if condType != "Ready" {
					continue
				}
				status, _ := cond["status"].(string)
				if status == "True" {
					return "Ready"
				}
				reason, _ := cond["reason"].(string)
				if reason != "" {
					return reason
				}
				return "NotReady"
			}
		}
		return "Unknown"
	case "Ingress":
		// networking.k8s.io Ingress: check if .status.loadBalancer.ingress is populated
		ingresses, err := parser.GetSlice(u.Object, "status", "loadBalancer", "ingress")
		if err == nil && len(ingresses) > 0 {
			return "Active"
		}
		return "Pending"
	default:
		return "Unknown"
	}
}

// formatAge returns a human-readable age string.
func formatAge(created time.Time) string {
	if created.IsZero() {
		return "Unknown"
	}
	d := time.Since(created)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
