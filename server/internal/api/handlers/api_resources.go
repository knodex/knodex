package handlers

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/discovery"

	"github.com/provops-org/knodex/server/internal/api/response"
)

// APIResource represents a Kubernetes API resource
type APIResource struct {
	APIGroup string `json:"apiGroup"` // Empty string for core API group, displayed as "core" in UI
	Kind     string `json:"kind"`
}

// APIResourcesResponse represents the response for API resources listing
type APIResourcesResponse struct {
	Resources []APIResource `json:"resources"`
	Count     int           `json:"count"`
}

// APIResourcesHandler handles API resource discovery requests
type APIResourcesHandler struct {
	discoveryClient discovery.DiscoveryInterface
	logger          *slog.Logger

	// Cache for API resources
	cacheMu   sync.RWMutex
	cache     []APIResource
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewAPIResourcesHandler creates a new APIResourcesHandler
func NewAPIResourcesHandler(discoveryClient discovery.DiscoveryInterface) *APIResourcesHandler {
	return &APIResourcesHandler{
		discoveryClient: discoveryClient,
		logger:          slog.Default().With("handler", "api-resources"),
		cacheTTL:        5 * time.Minute,
	}
}

// ListAPIResources returns available API groups and kinds from the cluster
// GET /api/v1/kubernetes/api-resources
// Query params:
//   - search: Filter by kind name (case-insensitive prefix/substring match)
//   - apiGroup: Filter by specific API group (use "core" for core API group)
func (h *APIResourcesHandler) ListAPIResources(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	searchQuery := strings.ToLower(r.URL.Query().Get("search"))
	apiGroupFilter := r.URL.Query().Get("apiGroup")

	// Track if API group filter was explicitly provided
	// (needed because "core" maps to "" which is also the zero value)
	hasApiGroupFilter := apiGroupFilter != ""

	// Handle "core" as alias for empty string (core API group)
	if apiGroupFilter == "core" {
		apiGroupFilter = ""
		hasApiGroupFilter = true
	}

	// Get resources (from cache or fresh fetch)
	resources, err := h.getResources()
	if err != nil {
		h.logger.Error("failed to get API resources", "error", err)
		response.ServiceUnavailable(w, "unable to connect to Kubernetes cluster")
		return
	}

	// Filter resources
	filtered := h.filterResources(resources, searchQuery, apiGroupFilter, hasApiGroupFilter)

	resp := APIResourcesResponse{
		Resources: filtered,
		Count:     len(filtered),
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// getResources returns API resources from cache or fetches fresh data
func (h *APIResourcesHandler) getResources() ([]APIResource, error) {
	h.cacheMu.RLock()
	if time.Since(h.cacheTime) < h.cacheTTL && len(h.cache) > 0 {
		resources := make([]APIResource, len(h.cache))
		copy(resources, h.cache)
		h.cacheMu.RUnlock()
		return resources, nil
	}
	h.cacheMu.RUnlock()

	// Fetch fresh data
	resources, err := h.fetchAPIResources()
	if err != nil {
		return nil, err
	}

	// Update cache
	h.cacheMu.Lock()
	h.cache = resources
	h.cacheTime = time.Now()
	h.cacheMu.Unlock()

	return resources, nil
}

// fetchAPIResources fetches API resources from the Kubernetes API server
func (h *APIResourcesHandler) fetchAPIResources() ([]APIResource, error) {
	if h.discoveryClient == nil {
		return nil, &ServiceUnavailableError{Message: "discovery client not available"}
	}

	// Get server preferred resources
	// This returns resources grouped by their API group version
	_, resourceLists, err := h.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		// Log the error but continue with partial results if available
		h.logger.Warn("partial failure getting server resources", "error", err)
		if resourceLists == nil {
			return nil, err
		}
	}

	// Build unique resources map to deduplicate
	resourceMap := make(map[string]APIResource)

	for _, resourceList := range resourceLists {
		if resourceList == nil {
			continue
		}

		// Extract API group from GroupVersion (e.g., "apps/v1" -> "apps", "v1" -> "")
		apiGroup := ""
		if gv := resourceList.GroupVersion; gv != "" {
			parts := strings.Split(gv, "/")
			if len(parts) == 2 {
				apiGroup = parts[0]
			}
			// If only one part (e.g., "v1"), it's the core API group (empty string)
		}

		for _, resource := range resourceList.APIResources {
			// Skip subresources (they contain "/")
			if strings.Contains(resource.Name, "/") {
				continue
			}

			// Use Kind as the unique key per API group
			key := apiGroup + "/" + resource.Kind
			if _, exists := resourceMap[key]; !exists {
				resourceMap[key] = APIResource{
					APIGroup: apiGroup,
					Kind:     resource.Kind,
				}
			}
		}
	}

	// Convert map to slice
	resources := make([]APIResource, 0, len(resourceMap))
	for _, r := range resourceMap {
		resources = append(resources, r)
	}

	// Sort by API group, then by kind
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].APIGroup != resources[j].APIGroup {
			// Empty string (core) should come first
			if resources[i].APIGroup == "" {
				return true
			}
			if resources[j].APIGroup == "" {
				return false
			}
			return resources[i].APIGroup < resources[j].APIGroup
		}
		return resources[i].Kind < resources[j].Kind
	})

	h.logger.Info("fetched API resources", "count", len(resources))
	return resources, nil
}

// filterResources filters resources by search query and API group
func (h *APIResourcesHandler) filterResources(resources []APIResource, searchQuery, apiGroupFilter string, hasApiGroupFilter bool) []APIResource {
	if searchQuery == "" && !hasApiGroupFilter {
		return resources
	}

	filtered := make([]APIResource, 0)
	for _, r := range resources {
		// Filter by API group if specified
		// Note: hasApiGroupFilter is needed because apiGroupFilter="" is valid (core API group)
		if hasApiGroupFilter && r.APIGroup != apiGroupFilter {
			continue
		}

		// Filter by search query (case-insensitive match on kind)
		if searchQuery != "" {
			kindLower := strings.ToLower(r.Kind)
			if !strings.Contains(kindLower, searchQuery) {
				continue
			}
		}

		filtered = append(filtered, r)
	}

	return filtered
}

// ServiceUnavailableError indicates the service is temporarily unavailable
type ServiceUnavailableError struct {
	Message string
}

func (e *ServiceUnavailableError) Error() string {
	return e.Message
}
