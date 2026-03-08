package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"github.com/knodex/knodex/server/internal/api/response"
)

const (
	// k8sListTimeout is the max time for listing Kubernetes resources.
	// 10s is adequate for single-namespace list operations.
	k8sListTimeout = 10 * time.Second

	// discoveryRefreshInterval controls how often API group discovery is refreshed.
	// 5 minutes balances freshness with API server load.
	discoveryRefreshInterval = 5 * time.Minute
)

// K8sResourceHandler handles K8s resource listing requests.
// Security relies on Kubernetes RBAC (service account permissions) rather than
// a hardcoded allowlist, so externalRef selectors work with any resource type
// including custom CRDs.
type K8sResourceHandler struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface

	// Cached discovery state for Kind → GVR resolution
	mu             sync.RWMutex
	groupResources []*restmapper.APIGroupResources
	lastRefresh    time.Time
}

// NewK8sResourceHandler creates a new K8s resource handler
func NewK8sResourceHandler(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface) *K8sResourceHandler {
	return &K8sResourceHandler{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
	}
}

// K8sResourceItem represents a K8s resource in the API response
type K8sResourceItem struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt string            `json:"createdAt"`
}

// K8sResourceListResponse represents the list response
type K8sResourceListResponse struct {
	Items []K8sResourceItem `json:"items"`
	Count int               `json:"count"`
}

// resolveGVR uses the discovery client to resolve apiVersion + Kind into a GroupVersionResource.
// This supports any resource type (including custom CRDs) without a hardcoded allowlist.
// Results are cached and refreshed periodically.
func (h *K8sResourceHandler) resolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	if h.discoveryClient == nil {
		// Fallback: naive pluralization when discovery is unavailable
		return naiveGVR(apiVersion, kind), nil
	}

	mapper, err := h.getMapper()
	if err != nil {
		slog.Warn("discovery failed, falling back to naive pluralization",
			"apiVersion", apiVersion, "kind", kind, "error", err)
		return naiveGVR(apiVersion, kind), nil
	}

	group, version := parseAPIVersion(apiVersion)
	gvk := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("unknown resource type %s/%s: %w", apiVersion, kind, err)
	}

	return mapping.Resource, nil
}

// getMapper returns a cached REST mapper, refreshing discovery if stale.
func (h *K8sResourceHandler) getMapper() (meta.RESTMapper, error) {
	h.mu.RLock()
	if h.groupResources != nil && time.Since(h.lastRefresh) < discoveryRefreshInterval {
		gr := h.groupResources
		h.mu.RUnlock()
		return restmapper.NewDiscoveryRESTMapper(gr), nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check after acquiring write lock
	if h.groupResources != nil && time.Since(h.lastRefresh) < discoveryRefreshInterval {
		return restmapper.NewDiscoveryRESTMapper(h.groupResources), nil
	}

	groupResources, err := restmapper.GetAPIGroupResources(h.discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %w", err)
	}

	h.groupResources = groupResources
	h.lastRefresh = time.Now()
	return restmapper.NewDiscoveryRESTMapper(groupResources), nil
}

// naiveGVR builds a GVR using simple lowercase+s pluralization.
// Used as a fallback when the discovery client is unavailable.
func naiveGVR(apiVersion, kind string) schema.GroupVersionResource {
	group, version := parseAPIVersion(apiVersion)
	resource := strings.ToLower(kind) + "s"

	// Handle well-known irregular plurals
	switch strings.ToLower(kind) {
	case "ingress":
		resource = "ingresses"
	case "networkpolicy":
		resource = "networkpolicies"
	}

	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

// parseAPIVersion parses an apiVersion string into group and version
func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		// Core API group (v1)
		return "", parts[0]
	}
	return parts[0], parts[1]
}

// ListResources handles GET /api/v1/resources
// @Summary List K8s resources
// @Description Lists K8s resources of a specific type. Used for ExternalRef selectors in deployment forms.
// Security is enforced by Kubernetes RBAC (service account permissions), not a hardcoded allowlist.
// @Tags resources
// @Accept json
// @Produce json
// @Param apiVersion query string true "Resource API version (e.g., v1, apps/v1, alz.example.com/v1)"
// @Param kind query string true "Resource kind (e.g., ConfigMap, Secret, ALZHub)"
// @Param namespace query string false "Namespace to list from (optional, defaults to all namespaces)"
// @Success 200 {object} K8sResourceListResponse
// @Failure 400 {object} api.ErrorResponse
// @Failure 403 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 503 {object} api.ErrorResponse
// @Router /api/v1/resources [get]
func (h *K8sResourceHandler) ListResources(w http.ResponseWriter, r *http.Request) {
	if h.dynamicClient == nil {
		response.ServiceUnavailable(w, "Kubernetes client not available")
		return
	}

	// Get query parameters
	apiVersion := r.URL.Query().Get("apiVersion")
	kind := r.URL.Query().Get("kind")
	namespace := r.URL.Query().Get("namespace")

	// Validate required parameters
	if apiVersion == "" {
		response.BadRequest(w, "apiVersion is required", map[string]string{
			"apiVersion": "query parameter is required",
		})
		return
	}

	if kind == "" {
		response.BadRequest(w, "kind is required", map[string]string{
			"kind": "query parameter is required",
		})
		return
	}

	// Resolve Kind to GVR using K8s discovery API (supports custom CRDs)
	gvr, err := h.resolveGVR(apiVersion, kind)
	if err != nil {
		response.NotFound(w, "resource type", apiVersion+"/"+kind)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), k8sListTimeout)
	defer cancel()

	listOptions := metav1.ListOptions{
		Limit: 100, // Limit results for performance
	}

	// List resources (namespaced or cluster-scoped)
	var resourceClient dynamic.ResourceInterface
	if namespace != "" {
		resourceClient = h.dynamicClient.Resource(gvr).Namespace(namespace)
	} else {
		resourceClient = h.dynamicClient.Resource(gvr)
	}

	unstructuredList, err := resourceClient.List(ctx, listOptions)
	if err != nil {
		h.handleK8sError(w, err, apiVersion, kind)
		return
	}

	// Convert to response items
	items := make([]K8sResourceItem, 0, len(unstructuredList.Items))
	for _, item := range unstructuredList.Items {
		items = append(items, K8sResourceItem{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Labels:    item.GetLabels(),
			CreatedAt: item.GetCreationTimestamp().Format(time.RFC3339),
		})
	}

	response.WriteJSON(w, http.StatusOK, K8sResourceListResponse{
		Items: items,
		Count: len(items),
	})
}

// handleK8sError maps Kubernetes API errors to appropriate HTTP responses.
func (h *K8sResourceHandler) handleK8sError(w http.ResponseWriter, err error, apiVersion, kind string) {
	if k8serrors.IsForbidden(err) {
		response.Forbidden(w, "service account does not have permission to list "+apiVersion+"/"+kind)
		return
	}
	if k8serrors.IsNotFound(err) {
		response.NotFound(w, "resource type", apiVersion+"/"+kind)
		return
	}
	slog.Error("failed to list K8s resources", "apiVersion", apiVersion, "kind", kind, "error", err)
	response.InternalError(w, "failed to list resources")
}
