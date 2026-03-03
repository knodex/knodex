package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/dynamic"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/audit"
	"github.com/provops-org/knodex/server/internal/deployment"
	"github.com/provops-org/knodex/server/internal/models"
	"github.com/provops-org/knodex/server/internal/rbac"
	"github.com/provops-org/knodex/server/internal/services"
	"github.com/provops-org/knodex/server/internal/watcher"
)

const (
	// instanceDeleteTimeout is the max time for deleting an instance from Kubernetes.
	// 30s allows for finalizers and cascading deletes; may need increase for complex resources.
	instanceDeleteTimeout = 30 * time.Second
)

// InstanceCRUDHandler handles basic CRUD operations for instances
type InstanceCRUDHandler struct {
	instanceTracker *watcher.InstanceTracker
	dynamicClient   dynamic.Interface
	authService     *services.AuthorizationService
	recorder        audit.Recorder
	logger          *slog.Logger
}

// InstanceCRUDHandlerConfig holds configuration for creating an InstanceCRUDHandler
type InstanceCRUDHandlerConfig struct {
	InstanceTracker *watcher.InstanceTracker
	DynamicClient   dynamic.Interface
	AuthService     *services.AuthorizationService
	AuditRecorder   audit.Recorder
	Logger          *slog.Logger
}

// NewInstanceCRUDHandler creates a new instance CRUD handler
func NewInstanceCRUDHandler(config InstanceCRUDHandlerConfig) *InstanceCRUDHandler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &InstanceCRUDHandler{
		instanceTracker: config.InstanceTracker,
		dynamicClient:   config.DynamicClient,
		authService:     config.AuthService,
		recorder:        config.AuditRecorder,
		logger:          logger.With("component", "instance-crud-handler"),
	}
}

// getAccessibleNamespaces retrieves the user's accessible namespaces using AuthorizationService.
// Returns:
// - nil: User can see all namespaces (global admin or auth not configured)
// - empty slice: User has no namespace access (secure default for unauthenticated)
// - non-empty slice: User can only see instances in these namespaces
func (h *InstanceCRUDHandler) getAccessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if userCtx == nil {
		return []string{}, nil
	}
	if h.authService == nil {
		return nil, nil
	}
	return h.authService.GetAccessibleNamespaces(ctx, userCtx)
}

// InstanceCountResponse represents the response for instance count endpoint
type InstanceCountResponse struct {
	Count int `json:"count"`
}

// ListInstances handles GET /api/v1/instances
// Filters results based on user's project namespaces
func (h *InstanceCRUDHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	// Check if instance tracker is available
	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	// Get user context from middleware (optional - if not present, no project filtering is applied)
	userCtx, _ := middleware.GetUserContext(r)

	// Get user's accessible namespaces for filtering (nil if unauthenticated/global admin)
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	opts := models.DefaultInstanceListOptions()

	if ns := query.Get("namespace"); ns != "" {
		// If user specifies a namespace, verify they have access to it
		// Uses wildcard pattern matching (e.g., "staging*" matches "staging-team-a")
		if userNamespaces != nil {
			hasAccess := rbac.MatchNamespaceInList(ns, userNamespaces)
			if !hasAccess {
				// User requested a namespace they don't have access to
				response.WriteJSON(w, http.StatusOK, models.InstanceList{
					Items:      []models.Instance{},
					TotalCount: 0,
					Page:       1,
					PageSize:   opts.PageSize,
				})
				return
			}
		}
		opts.Namespace = ns
	}

	if rgdName := query.Get("rgdName"); rgdName != "" {
		opts.RGDName = rgdName
	}
	if rgdNs := query.Get("rgdNamespace"); rgdNs != "" {
		opts.RGDNamespace = rgdNs
	}
	if health := query.Get("health"); health != "" {
		opts.Health = models.InstanceHealth(health)
	}
	if deployMode := query.Get("deploymentMode"); deployMode != "" {
		opts.DeploymentMode = deployment.DeploymentMode(deployMode)
	}
	if projectID := query.Get("projectId"); projectID != "" {
		opts.ProjectID = projectID
	}
	if search := query.Get("search"); search != "" {
		opts.Search = search
	}
	if page := query.Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			opts.Page = p
		}
	}
	if pageSize := query.Get("pageSize"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil && ps > 0 && ps <= 100 {
			opts.PageSize = ps
		}
	}
	if sortBy := query.Get("sortBy"); sortBy != "" {
		if !isAllowedSortBy(sortBy) {
			response.BadRequest(w, "invalid sortBy value; allowed values: name, createdAt, updatedAt, health", nil)
			return
		}
		opts.SortBy = sortBy
	}
	if sortOrder := query.Get("sortOrder"); sortOrder != "" {
		if !isAllowedSortOrder(sortOrder) {
			response.BadRequest(w, "invalid sortOrder value; allowed values: asc, desc", nil)
			return
		}
		opts.SortOrder = sortOrder
	}

	// Get all instances
	result := h.instanceTracker.ListInstances(opts)

	slog.Debug("ListInstances result from tracker",
		"count", result.TotalCount,
		"items", len(result.Items),
		"filteringEnabled", userNamespaces != nil,
		"allowedNamespaceCount", len(userNamespaces))

	// Filter instances by user's project namespaces (defense in depth)
	// Global admins (userNamespaces == nil) see all instances
	// Uses wildcard pattern matching (e.g., "staging*" matches "staging-team-a")
	if userNamespaces != nil {
		filtered := make([]models.Instance, 0)
		for _, instance := range result.Items {
			// Check if instance namespace matches any allowed pattern
			if rbac.MatchNamespaceInList(instance.Namespace, userNamespaces) {
				filtered = append(filtered, instance)
			}
		}
		result.Items = filtered
		result.TotalCount = len(filtered)
	}

	response.WriteJSON(w, http.StatusOK, result)
}

// GetCount handles GET /api/v1/instances/count
// Returns the total count of instances accessible to the user
// @Summary Get instance count
// @Description Returns the total count of instances accessible to the user
// @Tags instances
// @Accept json
// @Produce json
// @Success 200 {object} InstanceCountResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 503 {object} api.ErrorResponse
// @Router /api/v1/instances/count [get]
func (h *InstanceCRUDHandler) GetCount(w http.ResponseWriter, r *http.Request) {
	// Check if instance tracker is available
	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	// Get user context from middleware
	userCtx, _ := middleware.GetUserContext(r)

	// Get user's accessible namespaces for filtering
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Use efficient namespace-aware count directly from the tracker
	// This properly counts all instances matching user's namespace access
	// (nil namespaces = global admin sees all, empty = no access)
	count := h.instanceTracker.CountInstancesByNamespaces(userNamespaces, rbac.MatchNamespaceInList)

	resp := InstanceCountResponse{
		Count: count,
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetInstance handles GET /api/v1/instances/{namespace}/{kind}/{name}
// Verifies user has access to the instance's namespace
func (h *InstanceCRUDHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	// Check if instance tracker is available
	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	// Get user context from middleware (optional - if not present, no project filtering is applied)
	userCtx, _ := middleware.GetUserContext(r)

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if namespace == "" || kind == "" || name == "" {
		response.BadRequest(w, "namespace, kind, and name are required", nil)
		return
	}

	// Get user's accessible namespaces for filtering (nil if unauthenticated/global admin)
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Check if user has access to this namespace (defense in depth)
	// Uses wildcard pattern matching (e.g., "staging*" matches "staging-team-a")
	if userNamespaces != nil {
		hasAccess := rbac.MatchNamespaceInList(namespace, userNamespaces)
		if !hasAccess {
			response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
			return
		}
	}

	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	response.WriteJSON(w, http.StatusOK, instance)
}

// DeleteInstance handles DELETE /api/v1/instances/{namespace}/{kind}/{name}
// Authorization handled by CasbinAuthz middleware
func (h *InstanceCRUDHandler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	// Check if instance tracker is available
	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	if h.dynamicClient == nil {
		response.ServiceUnavailable(w, "Kubernetes client not available")
		return
	}

	// User context validation - user must be authenticated
	// Authorization is handled by CasbinAuthz middleware
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User context not found", nil)
		return
	}

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if namespace == "" || kind == "" || name == "" {
		response.BadRequest(w, "namespace, kind, and name are required", nil)
		return
	}

	// Per-request permission checks removed - authorization is handled by CasbinAuthz middleware.
	// The middleware enforces route-level authorization via PolicyEnforcer.CanAccessWithGroups().

	// Get instance to find its API version
	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), instanceDeleteTimeout)
	defer cancel()

	err := h.instanceTracker.DeleteInstance(ctx, namespace, name, instance.APIVersion, instance.Kind)
	if err != nil {
		h.handleDeleteError(w, namespace, kind, name, err)
		return
	}

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "instances",
		Name:      name,
		Project:   instance.ProjectName,
		Namespace: namespace,
		RequestID: r.Header.Get("X-Request-ID"),
		Result:    "success",
		Details:   map[string]any{"rgdName": instance.RGDName},
	})

	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status":    "deleted",
		"name":      name,
		"namespace": namespace,
	})
}

// handleDeleteError maps instance deletion errors to HTTP responses.
// Raw error details are logged server-side only; clients receive generic messages.
func (h *InstanceCRUDHandler) handleDeleteError(w http.ResponseWriter, namespace, kind, name string, err error) {
	errMsg := err.Error()
	slog.Error("failed to delete instance", "namespace", namespace, "kind", kind, "name", name, "error", errMsg)

	if strings.Contains(errMsg, "not found") {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}
	if strings.Contains(errMsg, "forbidden") {
		response.WriteError(w, http.StatusForbidden, "FORBIDDEN",
			"Permission denied: The service account does not have permission to delete this resource.",
			nil)
		return
	}
	response.InternalError(w, "Failed to delete instance")
}

// allowedSortByValues defines the allowed sortBy fields for instance listing.
var allowedSortByValues = map[string]bool{
	"name":      true,
	"createdAt": true,
	"updatedAt": true,
	"health":    true,
}

// allowedSortOrderValues defines the allowed sortOrder values.
var allowedSortOrderValues = map[string]bool{
	"asc":  true,
	"desc": true,
}

func isAllowedSortBy(v string) bool {
	return allowedSortByValues[v]
}

func isAllowedSortOrder(v string) bool {
	return allowedSortOrderValues[v]
}
