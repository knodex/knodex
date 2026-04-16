// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/deployment/vcs"
	"github.com/knodex/knodex/server/internal/drift"
	"github.com/knodex/knodex/server/internal/kro/children"
	kroadapter "github.com/knodex/knodex/server/internal/kro/graph"
	"github.com/knodex/knodex/server/internal/kro/parser"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/collection"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// instanceDeleteTimeout is the max time for deleting an instance from Kubernetes.
	// 30s allows for finalizers and cascading deletes; may need increase for complex resources.
	instanceDeleteTimeout = 30 * time.Second

	// instanceUpdateTimeout is the max time for updating an instance spec in Kubernetes.
	// 30s allows for webhook validation and API server latency.
	instanceUpdateTimeout = 30 * time.Second
)

// UpdateInstanceRequest represents the request body for updating an instance spec
type UpdateInstanceRequest struct {
	// Spec is the updated spec values
	Spec map[string]interface{} `json:"spec"`
	// ResourceVersion for optimistic concurrency control (optional)
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// RepositoryID is the Git repository config ID (required for gitops/hybrid mode updates)
	RepositoryID string `json:"repositoryId,omitempty"`
	// GitBranch overrides the repository's default branch for this update
	GitBranch string `json:"gitBranch,omitempty"`
	// GitPath overrides the auto-generated semantic path for this update
	GitPath string `json:"gitPath,omitempty"`
}

// UpdateInstanceResponse represents the response after updating an instance
type UpdateInstanceResponse struct {
	Name           string              `json:"name"`
	Namespace      string              `json:"namespace"`
	Kind           string              `json:"kind"`
	Status         string              `json:"status"`
	DeploymentMode string              `json:"deploymentMode,omitempty"`
	GitInfo        *deployment.GitInfo `json:"gitInfo,omitempty"`
}

// InstanceCRUDHandler handles basic CRUD operations for instances
type InstanceCRUDHandler struct {
	instanceTracker      *watcher.InstanceTracker
	rgdWatcher           *watcher.RGDWatcher
	resourceParser       *parser.ResourceParser
	dynamicClient        dynamic.Interface
	k8sClient            kubernetes.Interface
	authService          *services.AuthorizationService
	deploymentController *deployment.Controller
	repoService          *repository.Service
	driftService         *drift.Service
	childService         *children.Service
	recorder             audit.Recorder
	logger               *slog.Logger
}

// InstanceCRUDHandlerConfig holds configuration for creating an InstanceCRUDHandler
type InstanceCRUDHandlerConfig struct {
	InstanceTracker      *watcher.InstanceTracker
	RGDWatcher           *watcher.RGDWatcher
	DynamicClient        dynamic.Interface
	K8sClient            kubernetes.Interface
	AuthService          *services.AuthorizationService
	DeploymentController *deployment.Controller
	RepoService          *repository.Service
	DriftService         *drift.Service
	ChildService         *children.Service
	AuditRecorder        audit.Recorder
	Logger               *slog.Logger
}

// NewInstanceCRUDHandler creates a new instance CRUD handler
func NewInstanceCRUDHandler(config InstanceCRUDHandlerConfig) *InstanceCRUDHandler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &InstanceCRUDHandler{
		instanceTracker:      config.InstanceTracker,
		rgdWatcher:           config.RGDWatcher,
		resourceParser:       parser.NewResourceParser(),
		dynamicClient:        config.DynamicClient,
		k8sClient:            config.K8sClient,
		authService:          config.AuthService,
		deploymentController: config.DeploymentController,
		repoService:          config.RepoService,
		driftService:         config.DriftService,
		childService:         config.ChildService,
		recorder:             config.AuditRecorder,
		logger:               logger.With("component", "instance-crud-handler"),
	}
}

// validateInstancePathParams validates constraints on instance path parameters.
// Returns true if validation failed (400 response already written); caller should return.
// name and kind are required (must pass empty-check before calling).
// namespace is optional — empty means cluster-scoped, skipped.
func validateInstancePathParams(w http.ResponseWriter, namespace, kind, name string) bool {
	if !sanitize.IsValidDNS1123Subdomain(name) {
		response.BadRequest(w, "name must be a valid DNS-1123 subdomain (lowercase alphanumeric, hyphens, and dots)", nil)
		return true
	}
	if !sanitize.IsValidK8sKind(kind) {
		response.BadRequest(w, "kind must be a valid Kubernetes Kind name (CamelCase, starting with uppercase letter)", nil)
		return true
	}
	if namespace != "" && !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return true
	}
	return false
}

// getAccessibleNamespaces retrieves the user's accessible namespaces using AuthorizationService.
// Returns:
// - nil: User can see all namespaces (global admin — only from Casbin evaluation)
// - empty slice: User has no namespace access (secure default)
// - non-empty slice: User can only see instances in these namespaces
func (h *InstanceCRUDHandler) getAccessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if userCtx == nil {
		return []string{}, nil
	}
	if h.authService == nil {
		// Fail closed: no auth service means no namespace access
		return []string{}, nil
	}
	return h.authService.GetAccessibleNamespaces(ctx, userCtx)
}

// getAccessibleProjects retrieves the user's accessible projects using AuthorizationService.
// Returns:
// - nil: User can see all projects (global admin — only from Casbin evaluation)
// - empty slice: User has no project access (secure default)
// - non-empty slice: User can only see instances in these projects
func (h *InstanceCRUDHandler) getAccessibleProjects(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if userCtx == nil {
		return []string{}, nil
	}
	if h.authService == nil {
		// Fail closed: no auth service means no project access
		return []string{}, nil
	}
	return h.authService.GetAccessibleProjects(ctx, userCtx)
}

// isProjectAccessible checks if a project name is in the user's accessible projects list.
// Returns true if accessibleProjects is nil (global admin) or if the project is in the list.
func isProjectAccessible(projectName string, accessibleProjects []string) bool {
	if accessibleProjects == nil {
		return true // Global admin or auth not configured
	}
	for _, p := range accessibleProjects {
		if p == projectName {
			return true
		}
	}
	return false
}

// authorizeInstanceAccess checks whether the user is authorized to access the given instance.
// For cluster-scoped instances: checks project access via Casbin-verified project list.
// For namespace-scoped instances: checks namespace access via project destinations.
// Returns (authorized, error). Error is non-nil only for internal failures (e.g., project lookup).
// Global admins have ["*"] which passes all MatchNamespaceInList checks.
func (h *InstanceCRUDHandler) authorizeInstanceAccess(ctx context.Context, userCtx *middleware.UserContext, instance *models.Instance, userNamespaces []string) (bool, error) {
	if instance.IsClusterScoped {
		userProjects, err := h.getAccessibleProjects(ctx, userCtx)
		if err != nil {
			return false, err
		}
		return isProjectAccessible(instance.ProjectName, userProjects), nil
	}

	return rbac.MatchNamespaceInList(instance.Namespace, userNamespaces), nil
}

// buildInstanceAccessFilter returns a predicate that checks instance access for both
// cluster-scoped (project check) and namespace-scoped (namespace check) instances.
// Used by ListInstances and GetCount to apply consistent authorization filtering.
// Global admins have ["*"] which passes all MatchNamespaceInList checks uniformly.
func buildInstanceAccessFilter(userNamespaces []string, userProjects []string) func(models.Instance) bool {
	return func(inst models.Instance) bool {
		if inst.IsClusterScoped {
			return isProjectAccessible(inst.ProjectName, userProjects)
		}
		return rbac.MatchNamespaceInList(inst.Namespace, userNamespaces)
	}
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

	// Get user context from middleware (unauthenticated requests fail closed — empty results)
	userCtx, _ := middleware.GetUserContext(r)

	// Get user's accessible namespaces for filtering (nil = global admin, empty = no access)
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
		// Verify user has access to the requested namespace
		// Uses wildcard pattern matching (e.g., "staging*" matches "staging-team-a")
		// Global admins have ["*"] which matches everything
		if !rbac.MatchNamespaceInList(ns, userNamespaces) {
			// User requested a namespace they don't have access to
			response.WriteJSON(w, http.StatusOK, models.InstanceList{
				Items:      []models.Instance{},
				TotalCount: 0,
				Page:       1,
				PageSize:   opts.PageSize,
			})
			return
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

	h.logger.Debug("ListInstances result from tracker",
		"count", result.TotalCount,
		"items", len(result.Items),
		"allowedNamespaceCount", len(userNamespaces))

	// Filter instances by user's project namespaces (defense in depth).
	// Global admins have ["*"] which passes all MatchNamespaceInList checks.
	// Cluster-scoped instances are filtered by project authorization (STORY-301).
	userProjects, projErr := h.getAccessibleProjects(r.Context(), userCtx)
	if projErr != nil {
		h.logger.Error("failed to get accessible projects", "error", projErr)
		response.InternalError(w, "Failed to get user projects")
		return
	}

	filter := buildInstanceAccessFilter(userNamespaces, userProjects)
	result.Items = collection.Filter(result.Items, filter)
	result.TotalCount = len(result.Items)

	// Enrich instances with drift info (batch: single Redis MGET)
	h.batchEnrichInstanceDrift(r.Context(), result.Items)

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

	// CountInstancesByNamespaces with ["*"] matches all namespace-scoped instances.
	// Non-admin: CountInstancesByNamespaces incorrectly includes all cluster-scoped
	// instances regardless of project access (pre-STORY-301 behavior in cache).
	// Use the shared access filter to match ListInstances behavior (STORY-301, STORY-309).
	userProjects, projErr := h.getAccessibleProjects(r.Context(), userCtx)
	if projErr != nil {
		h.logger.Error("failed to get accessible projects for count", "error", projErr)
		response.InternalError(w, "Failed to get user projects")
		return
	}

	filter := buildInstanceAccessFilter(userNamespaces, userProjects)
	count := h.instanceTracker.CountFilteredInstances(func(inst *models.Instance) bool {
		return filter(*inst)
	})

	response.WriteJSON(w, http.StatusOK, InstanceCountResponse{Count: count})
}

// GetInstance handles GET /api/v1/instances/{namespace}/{kind}/{name}
// Verifies user has access to the instance's namespace
func (h *InstanceCRUDHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	// Check if instance tracker is available
	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	// User context validation - user must be authenticated (STORY-309: consistent with Delete/Update)
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User context not found", nil)
		return
	}

	namespace := r.PathValue("namespace") // empty for cluster-scoped instances
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if kind == "" || name == "" {
		response.BadRequest(w, "kind and name are required", nil)
		return
	}

	// STORY-348: DNS-1123 validation for K8s-bound path params
	if validateInstancePathParams(w, namespace, kind, name) {
		return
	}

	// Get user's accessible namespaces for filtering (nil if global admin)
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	// Unified authorization: handles both cluster-scoped (project) and namespace-scoped (namespace) checks
	authorized, authErr := h.authorizeInstanceAccess(r.Context(), userCtx, instance, userNamespaces)
	if authErr != nil {
		h.logger.Error("failed to check instance access", "error", authErr)
		response.InternalError(w, "Failed to check instance access")
		return
	}
	if !authorized {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	// Enrich with drift info for gitops instances
	h.enrichInstanceDrift(r.Context(), instance)

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

	namespace := r.PathValue("namespace") // empty for cluster-scoped instances
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if kind == "" || name == "" {
		response.BadRequest(w, "kind and name are required", nil)
		return
	}

	// STORY-348: DNS-1123 validation for K8s-bound path params
	if validateInstancePathParams(w, namespace, kind, name) {
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

	// Unified authorization: handles both cluster-scoped (project) and namespace-scoped (namespace) checks.
	// The middleware cannot resolve empty namespaces to projects for cluster-scoped instances,
	// so handler-level authorization is defense in depth (STORY-301, STORY-309).
	userNamespaces, nsErr := h.getAccessibleNamespaces(r.Context(), userCtx)
	if nsErr != nil {
		h.logger.Error("failed to get accessible namespaces", "error", nsErr)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}
	authorized, authErr := h.authorizeInstanceAccess(r.Context(), userCtx, instance, userNamespaces)
	if authErr != nil {
		h.logger.Error("failed to check instance access", "error", authErr)
		response.InternalError(w, "Failed to check instance access")
		return
	}
	if !authorized {
		// Return 404 (not 403) to avoid leaking resource existence to cross-project users.
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), instanceDeleteTimeout)
	defer cancel()

	// For GitOps/Hybrid deployments, clean up the manifest from Git before K8s deletion.
	// This prevents orphaned manifests in the Git repo and ensures ArgoCD doesn't recreate the resource.
	if instance.DeploymentMode == deployment.ModeGitOps || instance.DeploymentMode == deployment.ModeHybrid {
		if err := h.deleteFromGit(ctx, instance, userCtx.Email); err != nil {
			h.logger.Warn("failed to delete instance from Git",
				"namespace", namespace,
				"kind", kind,
				"name", name,
				"deploymentMode", instance.DeploymentMode,
				"error", err,
			)
			// For Hybrid mode, continue with K8s deletion even if Git cleanup fails.
			// For GitOps mode, also continue — the user explicitly wants the instance gone.
		}
	}

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
		Details: map[string]any{
			"rgdName":        instance.RGDName,
			"kind":           instance.Kind,
			"health":         string(instance.Health),
			"deploymentMode": string(instance.DeploymentMode),
		},
	})

	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status":    "deleted",
		"name":      name,
		"namespace": namespace,
	})
}

// deleteFromGit removes the instance manifest from the Git repository.
// Called during deletion of GitOps/Hybrid instances to clean up orphaned manifests.
func (h *InstanceCRUDHandler) deleteFromGit(ctx context.Context, instance *models.Instance, deletedBy string) error {
	if h.deploymentController == nil {
		return fmt.Errorf("deployment controller not configured")
	}

	// Resolve the repository config from the instance's Git info
	repoID := ""
	if instance.GitInfo != nil {
		repoID = instance.GitInfo.RepositoryID
	}
	if repoID == "" {
		// Check labels as fallback
		repoID = instance.Labels[models.RepositoryIDLabel]
	}
	if repoID == "" {
		return fmt.Errorf("no repository ID found on instance")
	}

	if h.repoService == nil {
		return fmt.Errorf("repository service not configured")
	}

	repoConfig, err := h.repoService.GetRepositoryConfig(ctx, repoID)
	if err != nil {
		return fmt.Errorf("failed to get repository config %q: %w", repoID, err)
	}

	deployRepo := buildDeployRepoConfig(repoConfig)
	return h.deploymentController.Delete(
		ctx,
		instance.Namespace,
		instance.Name,
		instance.Kind,
		instance.RGDName,
		deletedBy,
		instance.IsClusterScoped,
		deployRepo,
		instance.DeploymentMode,
	)
}

// UpdateInstance handles PATCH /api/v1/instances/{namespace}/{kind}/{name}
// Updates the spec of an existing instance. Authorization handled by CasbinAuthz middleware.
func (h *InstanceCRUDHandler) UpdateInstance(w http.ResponseWriter, r *http.Request) {
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

	namespace := r.PathValue("namespace") // empty for cluster-scoped instances
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if kind == "" || name == "" {
		response.BadRequest(w, "kind and name are required", nil)
		return
	}

	// STORY-348: DNS-1123 validation for K8s-bound path params
	if validateInstancePathParams(w, namespace, kind, name) {
		return
	}

	// Parse request body
	req, err := helpers.DecodeJSON[UpdateInstanceRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	if req.Spec == nil || len(req.Spec) == 0 {
		response.BadRequest(w, "spec is required and must not be empty", nil)
		return
	}

	// Security: validate spec against injection/DoS attack patterns
	// before applying to Kubernetes (INJ-VULN-02)
	if err := deployment.ValidateSpecMap(req.Spec, 0, deployment.MaxSpecDepth); err != nil {
		response.BadRequest(w, "invalid spec: "+err.Error(), nil)
		return
	}

	// Get instance from tracker cache to find its API version
	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	// Unified authorization: handles both cluster-scoped (project) and namespace-scoped (namespace) checks.
	// The middleware cannot resolve empty namespaces to projects for cluster-scoped instances,
	// so handler-level authorization is defense in depth (STORY-301, STORY-309).
	userNamespaces, nsErr := h.getAccessibleNamespaces(r.Context(), userCtx)
	if nsErr != nil {
		h.logger.Error("failed to get accessible namespaces", "error", nsErr)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}
	authorized, authErr := h.authorizeInstanceAccess(r.Context(), userCtx, instance, userNamespaces)
	if authErr != nil {
		h.logger.Error("failed to check instance access", "error", authErr)
		response.InternalError(w, "Failed to check instance access")
		return
	}
	if !authorized {
		// Return 404 (not 403) to avoid leaking resource existence to cross-project users.
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), instanceUpdateTimeout)
	defer cancel()

	// Determine deployment mode from instance labels
	deployMode := deployment.ParseDeploymentMode(instance.Labels[models.DeploymentModeLabel])

	// Resolve GVR from instance's API version and kind using discovery
	gvr, err := h.instanceTracker.ResolveGVR(instance.APIVersion, instance.Kind)
	if err != nil {
		h.logger.Error("failed to resolve GVR", "apiVersion", instance.APIVersion, "kind", instance.Kind, "error", err)
		response.InternalError(w, "Failed to resolve resource type")
		return
	}

	var gitInfo *deployment.GitInfo

	switch deployMode {
	case deployment.ModeGitOps:
		// GitOps mode: DO NOT patch K8s directly — ArgoCD/Flux would revert it as drift.
		// Only push updated manifest to Git via deployment controller.
		var gitErr error
		gitInfo, gitErr = h.pushSpecUpdateToGit(ctx, req, instance, userCtx)
		if gitErr != nil {
			// Record audit event for failed gitops update (security: track who attempted what)
			audit.RecordEvent(h.recorder, r.Context(), audit.Event{
				UserID:    userCtx.UserID,
				UserEmail: userCtx.Email,
				SourceIP:  audit.SourceIP(r),
				Action:    "update",
				Resource:  "instances",
				Name:      name,
				Project:   instance.ProjectName,
				Namespace: namespace,
				RequestID: r.Header.Get("X-Request-ID"),
				Result:    "error",
				Details: map[string]any{
					"rgdName":        instance.RGDName,
					"kind":           instance.Kind,
					"deploymentMode": string(deployMode),
					"error":          gitErr.Error(),
				},
			})
			h.handleGitOpsUpdateError(w, namespace, kind, name, gitErr)
			return
		}

	case deployment.ModeHybrid:
		// Hybrid mode: both K8s patch + Git push
		// Step 1: Patch K8s directly for immediate effect
		if patchErr := h.patchKubernetesSpec(ctx, req, instance, namespace, name, gvr); patchErr != nil {
			audit.RecordEvent(h.recorder, r.Context(), audit.Event{
				UserID:    userCtx.UserID,
				UserEmail: userCtx.Email,
				SourceIP:  audit.SourceIP(r),
				Action:    "update",
				Resource:  "instances",
				Name:      name,
				Project:   instance.ProjectName,
				Namespace: namespace,
				RequestID: r.Header.Get("X-Request-ID"),
				Result:    "error",
				Details: map[string]any{
					"rgdName":        instance.RGDName,
					"kind":           instance.Kind,
					"deploymentMode": string(deployMode),
					"error":          patchErr.Error(),
				},
			})
			h.handleUpdateError(w, namespace, kind, name, patchErr)
			return
		}
		// Step 2: Push to Git for audit trail (failure does not fail the update)
		var gitErr error
		gitInfo, gitErr = h.pushSpecUpdateToGit(ctx, req, instance, userCtx)
		if gitErr != nil {
			h.logger.Warn("hybrid update: Git push failed (K8s patch succeeded)",
				"namespace", namespace, "kind", kind, "name", name, "error", gitErr)
			gitInfo = &deployment.GitInfo{
				PushStatus: deployment.GitPushFailed,
				PushError:  "Git push failed: the Kubernetes resource was updated but the Git repository was not",
			}
		}

	default:
		// Direct mode (default): patch K8s resource via dynamic client
		if patchErr := h.patchKubernetesSpec(ctx, req, instance, namespace, name, gvr); patchErr != nil {
			audit.RecordEvent(h.recorder, r.Context(), audit.Event{
				UserID:    userCtx.UserID,
				UserEmail: userCtx.Email,
				SourceIP:  audit.SourceIP(r),
				Action:    "update",
				Resource:  "instances",
				Name:      name,
				Project:   instance.ProjectName,
				Namespace: namespace,
				RequestID: r.Header.Get("X-Request-ID"),
				Result:    "error",
				Details: map[string]any{
					"rgdName":        instance.RGDName,
					"kind":           instance.Kind,
					"deploymentMode": string(deployMode),
					"error":          patchErr.Error(),
				},
			})
			h.handleUpdateError(w, namespace, kind, name, patchErr)
			return
		}
	}

	// Store drift state in Redis for gitops/hybrid modes after successful Git push
	if h.driftService != nil && gitInfo != nil && gitInfo.PushStatus == deployment.GitPushSuccess {
		if driftErr := h.driftService.StoreDrift(ctx, namespace, kind, name, req.Spec); driftErr != nil {
			h.logger.Warn("failed to store drift state (non-fatal)",
				"namespace", namespace, "kind", kind, "name", name, "error", driftErr)
		}
	}

	// Build audit details — record WHICH spec keys changed without exposing VALUES (may contain secrets)
	auditDetails := map[string]any{
		"rgdName":        instance.RGDName,
		"kind":           instance.Kind,
		"deploymentMode": string(deployMode),
	}

	// Compute spec change details: which keys were added, removed, or modified
	// Only record key names — never the values (defense-in-depth against secret leakage)
	if instance.Spec != nil {
		specChanges := computeSpecChanges(instance.Spec, req.Spec)
		if len(specChanges) > 0 {
			auditDetails["specChanges"] = specChanges
		}
	} else {
		// No previous spec — record all new keys
		changedKeys := make([]string, 0, len(req.Spec))
		for k := range req.Spec {
			changedKeys = append(changedKeys, k)
		}
		if len(changedKeys) > 0 {
			auditDetails["specChanges"] = map[string]any{
				"added": changedKeys,
			}
		}
	}

	if gitInfo != nil && gitInfo.CommitSHA != "" {
		auditDetails["gitCommitSHA"] = gitInfo.CommitSHA
		auditDetails["gitBranch"] = gitInfo.Branch
		auditDetails["gitPath"] = gitInfo.Path
	}
	if gitInfo != nil && gitInfo.PushStatus == deployment.GitPushFailed {
		auditDetails["gitPushFailed"] = true
		auditDetails["gitPushError"] = gitInfo.PushError
	}
	if req.RepositoryID != "" {
		auditDetails["repositoryId"] = req.RepositoryID
	}

	// Determine audit result — "partial" when K8s succeeded but Git push failed (hybrid mode)
	auditResult := "success"
	if gitInfo != nil && gitInfo.PushStatus == deployment.GitPushFailed {
		auditResult = "partial"
	}

	// Record audit event
	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "instances",
		Name:      name,
		Project:   instance.ProjectName,
		Namespace: namespace,
		RequestID: r.Header.Get("X-Request-ID"),
		Result:    auditResult,
		Details:   auditDetails,
	})

	resp := UpdateInstanceResponse{
		Name:           name,
		Namespace:      namespace,
		Kind:           instance.Kind,
		Status:         "updated",
		DeploymentMode: string(deployMode),
		GitInfo:        gitInfo,
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// patchKubernetesSpec applies a merge patch to the Kubernetes resource spec.
func (h *InstanceCRUDHandler) patchKubernetesSpec(ctx context.Context, req *UpdateInstanceRequest, instance *models.Instance, namespace, name string, gvr schema.GroupVersionResource) error {
	patchData := map[string]interface{}{
		"spec": req.Spec,
	}
	if req.ResourceVersion != "" {
		patchData["metadata"] = map[string]interface{}{
			"resourceVersion": req.ResourceVersion,
		}
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %w", err)
	}

	// Scope-aware patch: cluster-scoped instances have empty namespace
	if namespace == "" {
		_, err = h.dynamicClient.Resource(gvr).Patch(
			ctx, name, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = h.dynamicClient.Resource(gvr).Namespace(namespace).Patch(
			ctx, name, k8stypes.MergePatchType, patchBytes, metav1.PatchOptions{})
	}
	return err
}

// pushSpecUpdateToGit pushes the updated instance manifest to Git via the deployment controller.
// Used for gitops and hybrid deployment modes.
func (h *InstanceCRUDHandler) pushSpecUpdateToGit(ctx context.Context, req *UpdateInstanceRequest, instance *models.Instance, userCtx *middleware.UserContext) (*deployment.GitInfo, error) {
	if h.deploymentController == nil {
		return nil, fmt.Errorf("deployment controller not configured for GitOps updates")
	}
	if h.repoService == nil {
		return nil, fmt.Errorf("repository service not configured for GitOps updates")
	}
	if req.RepositoryID == "" {
		return nil, fmt.Errorf("repositoryId is required for gitops/hybrid mode updates")
	}

	// Look up repository configuration
	repoConfig, err := h.repoService.GetRepositoryConfig(ctx, req.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	// Build deployment repository config (shared with CreateInstance in instance_deployment.go)
	deployRepo := buildDeployRepoConfig(repoConfig)

	// Build deploy request for Git push
	deployReq := &deployment.DeployRequest{
		Name:           instance.Name,
		Namespace:      instance.Namespace,
		RGDName:        instance.RGDName,
		RGDNamespace:   instance.RGDNamespace,
		APIVersion:     instance.APIVersion,
		Kind:           instance.Kind,
		Spec:           req.Spec,
		DeploymentMode: deployment.ModeGitOps, // Always use gitops mode for the Git push
		ProjectID:      instance.ProjectID,
		Repository:     deployRepo,
		CreatedBy:      userCtx.Email,
	}

	// Set branch override
	if req.GitBranch != "" {
		deployReq.GitBranch = req.GitBranch
	} else {
		deployReq.GitBranch = repoConfig.Spec.DefaultBranch
	}
	if req.GitPath != "" {
		deployReq.GitPath = req.GitPath
	}

	// Use the deployment controller to push to Git (gitops mode = git push only)
	result, err := h.deploymentController.Deploy(ctx, deployReq)
	if err != nil {
		return nil, err
	}

	gitInfo := &deployment.GitInfo{
		RepositoryID: req.RepositoryID,
		PushStatus:   deployment.GitPushSuccess,
	}
	if result.GitPushed {
		gitInfo.CommitSHA = result.GitCommitSHA
		gitInfo.Path = result.ManifestPath
		gitInfo.Branch = deployReq.GetEffectiveBranch()
	}
	return gitInfo, nil
}

// buildDeployRepoConfig converts a repository.RepositoryConfig to a deployment.RepositoryConfig.
// Shared between InstanceCRUDHandler and InstanceDeploymentHandler.
func buildDeployRepoConfig(repoConfig *repository.RepositoryConfig) *deployment.RepositoryConfig {
	// Parse repository URL to get owner/repo and provider
	parsedURL, parseErr := vcs.ParseRepoURL(repoConfig.Spec.RepoURL)
	provider := ""
	if parseErr == nil && parsedURL.Provider != "" {
		provider = string(parsedURL.Provider)
	}

	// Determine the correct secret key based on auth type
	secretKey := "bearerToken"
	switch repoConfig.Spec.AuthType {
	case "https":
		secretKey = "bearerToken"
	case "ssh":
		secretKey = "sshPrivateKey"
	case "github-app":
		secretKey = "githubAppPrivateKey"
	}

	owner := ""
	repo := ""
	if parsedURL != nil {
		owner = parsedURL.Owner
		repo = parsedURL.Repo
	}

	return &deployment.RepositoryConfig{
		ID:            repoConfig.Name,
		Name:          repoConfig.Spec.Name,
		ProjectID:     repoConfig.Spec.ProjectID,
		Provider:      provider,
		Owner:         owner,
		Repo:          repo,
		Branch:        repoConfig.Spec.DefaultBranch,
		DefaultBranch: repoConfig.Spec.DefaultBranch,
		BasePath:      "manifests",
		SecretRef: deployment.SecretReference{
			Name:      repoConfig.Spec.SecretRef.Name,
			Namespace: repoConfig.Spec.SecretRef.Namespace,
			Key:       secretKey,
		},
		SecretName:      repoConfig.Spec.SecretRef.Name,
		SecretNamespace: repoConfig.Spec.SecretRef.Namespace,
		SecretKey:       secretKey,
	}
}

// handleGitOpsUpdateError maps GitOps update errors to HTTP responses.
func (h *InstanceCRUDHandler) handleGitOpsUpdateError(w http.ResponseWriter, namespace, kind, name string, err error) {
	errMsg := err.Error()
	h.logger.Error("failed to push spec update to Git", "namespace", namespace, "kind", kind, "name", name, "error", errMsg)

	if strings.Contains(errMsg, "repository not found") || strings.Contains(errMsg, "not found") {
		response.BadRequest(w, "Repository configuration not found. Ensure the repository ID is correct.", nil)
		return
	}
	if strings.Contains(errMsg, "repositoryId is required") {
		response.BadRequest(w, "Repository ID is required for gitops/hybrid mode updates", map[string]string{
			"repositoryId": "required when instance deployment mode is gitops or hybrid",
		})
		return
	}
	if strings.Contains(errMsg, "deployment controller not configured") || strings.Contains(errMsg, "repository service not configured") {
		response.ServiceUnavailable(w, "GitOps deployment is not configured on this server")
		return
	}
	if strings.Contains(errMsg, "GitHub") || strings.Contains(errMsg, "git push") || strings.Contains(errMsg, "git clone") {
		response.WriteError(w, http.StatusBadGateway, "GITOPS_ERROR",
			"Failed to push spec update to Git repository",
			nil)
		return
	}
	response.InternalError(w, "Failed to update instance via GitOps")
}

// handleUpdateError maps instance update errors to HTTP responses.
// Raw error details are logged server-side only; clients receive generic messages.
func (h *InstanceCRUDHandler) handleUpdateError(w http.ResponseWriter, namespace, kind, name string, err error) {
	errMsg := err.Error()
	h.logger.Error("failed to update instance", "namespace", namespace, "kind", kind, "name", name, "error", errMsg)

	if strings.Contains(errMsg, "not found") {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}
	if strings.Contains(errMsg, "forbidden") {
		response.WriteError(w, http.StatusForbidden, "FORBIDDEN",
			"Permission denied: The service account does not have permission to update this resource.",
			nil)
		return
	}
	if strings.Contains(errMsg, "the object has been modified") || strings.Contains(errMsg, "Conflict") {
		response.WriteError(w, http.StatusConflict, "CONFLICT",
			"Resource version conflict: The instance was modified by another user. Please refresh and try again.",
			nil)
		return
	}
	if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "validation") {
		response.BadRequest(w, "Invalid spec: the provided spec failed validation", nil)
		return
	}
	response.InternalError(w, "Failed to update instance")
}

// computeSpecChanges compares old and new spec maps and returns a summary of changes.
// Only key names are recorded — never the actual values (security: specs may contain secrets).
// Returns a map with "added", "removed", and "modified" string slices.
func computeSpecChanges(oldSpec, newSpec map[string]interface{}) map[string]any {
	changes := map[string]any{}

	var added, removed, modified []string

	// Keys in new but not in old → added
	for k := range newSpec {
		if _, exists := oldSpec[k]; !exists {
			added = append(added, k)
		}
	}

	// Keys in old but not in new → removed
	for k := range oldSpec {
		if _, exists := newSpec[k]; !exists {
			removed = append(removed, k)
		}
	}

	// Keys in both but with different values → modified
	for k, newVal := range newSpec {
		if oldVal, exists := oldSpec[k]; exists {
			if !specValuesEqual(oldVal, newVal) {
				modified = append(modified, k)
			}
		}
	}

	if len(added) > 0 {
		changes["added"] = added
	}
	if len(removed) > 0 {
		changes["removed"] = removed
	}
	if len(modified) > 0 {
		changes["modified"] = modified
	}

	return changes
}

// specValuesEqual compares two spec values using JSON marshaling for deep equality.
// This handles nested maps and slices correctly.
func specValuesEqual(a, b interface{}) bool {
	aJSON, aErr := json.Marshal(a)
	bJSON, bErr := json.Marshal(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// handleDeleteError maps instance deletion errors to HTTP responses.
// Raw error details are logged server-side only; clients receive generic messages.
func (h *InstanceCRUDHandler) handleDeleteError(w http.ResponseWriter, namespace, kind, name string, err error) {
	errMsg := err.Error()
	h.logger.Error("failed to delete instance", "namespace", namespace, "kind", kind, "name", name, "error", errMsg)

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

// enrichInstanceDrift checks Redis for drift state and populates the instance's
// GitOpsDrift, DesiredSpec, and DriftedAt fields. Only checks gitops/hybrid mode instances.
// Gracefully degrades if drift service is unavailable.
func (h *InstanceCRUDHandler) enrichInstanceDrift(ctx context.Context, instance *models.Instance) {
	if h.driftService == nil || instance == nil {
		return
	}

	deployMode := deployment.ParseDeploymentMode(instance.Labels[models.DeploymentModeLabel])
	if deployMode != deployment.ModeGitOps && deployMode != deployment.ModeHybrid {
		return
	}

	isDrifted, desiredSpec, driftedAt, err := h.driftService.CheckDrift(ctx, instance.Namespace, instance.Kind, instance.Name, instance.Spec)
	if err != nil {
		return // Graceful degradation
	}

	instance.GitOpsDrift = isDrifted
	instance.DesiredSpec = desiredSpec
	instance.DriftedAt = driftedAt
}

// batchEnrichInstanceDrift checks Redis for drift state for all gitops/hybrid instances
// in a single MGET round-trip and populates each instance's GitOpsDrift, DesiredSpec, and DriftedAt fields.
func (h *InstanceCRUDHandler) batchEnrichInstanceDrift(ctx context.Context, instances []models.Instance) {
	if h.driftService == nil || len(instances) == 0 {
		return
	}

	// Build parallel slices: original instance indices and BatchCheckDrift inputs.
	// Only gitops/hybrid instances participate in drift checks.
	var indices []int
	var inputs []drift.DriftCheckInput
	for i := range instances {
		inst := &instances[i]
		deployMode := deployment.ParseDeploymentMode(inst.Labels[models.DeploymentModeLabel])
		if deployMode != deployment.ModeGitOps && deployMode != deployment.ModeHybrid {
			continue
		}
		indices = append(indices, i)
		inputs = append(inputs, drift.DriftCheckInput{
			Namespace: inst.Namespace,
			Kind:      inst.Kind,
			Name:      inst.Name,
			LiveSpec:  inst.Spec,
		})
	}
	if len(inputs) == 0 {
		return
	}

	for i, res := range h.driftService.BatchCheckDrift(ctx, inputs) {
		instances[indices[i]].GitOpsDrift = res.IsDrifted
		instances[indices[i]].DesiredSpec = res.DesiredSpec
		instances[indices[i]].DriftedAt = res.DriftedAt
	}
}

// GetInstanceGraph handles GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/graph
// and GET /api/v1/instances/{kind}/{name}/graph (cluster-scoped variant).
// Returns the runtime graph for an instance: same topology as the definition graph, but with
// CollectionStatus populated for forEach nodes. STORY-333 will fill in live counts; for now
// each collection node returns an empty CollectionStatus with Health: Healthy.
func (h *InstanceCRUDHandler) GetInstanceGraph(w http.ResponseWriter, r *http.Request) {
	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	if h.rgdWatcher == nil {
		response.ServiceUnavailable(w, "RGD watcher not available")
		return
	}

	// User context validation - user must be authenticated (consistent with GetInstance)
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User context not found", nil)
		return
	}

	namespace := r.PathValue("namespace") // empty for cluster-scoped instances
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if kind == "" || name == "" {
		response.BadRequest(w, "kind and name are required", nil)
		return
	}

	// STORY-348: DNS-1123 validation for K8s-bound path params
	if validateInstancePathParams(w, namespace, kind, name) {
		return
	}

	// Get user's accessible namespaces for filtering (nil if global admin)
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	// Unified authorization: handles both cluster-scoped (project) and namespace-scoped (namespace) checks
	authorized, authErr := h.authorizeInstanceAccess(r.Context(), userCtx, instance, userNamespaces)
	if authErr != nil {
		h.logger.Error("failed to check instance access", "error", authErr)
		response.InternalError(w, "Failed to check instance access")
		return
	}
	if !authorized {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	rgd, rgdFound := h.rgdWatcher.GetRGD(instance.RGDNamespace, instance.RGDName)
	if !rgdFound || rgd == nil {
		response.NotFound(w, "RGD", instance.RGDName)
		return
	}

	// Get resource graph from cached KRO graph or fallback parser
	var resourceGraph *parser.ResourceGraph
	if g := h.rgdWatcher.GetGraph(rgd.Namespace, rgd.Name); g != nil {
		adapter := kroadapter.NewUIGraphAdapter(g)
		resourceGraph = adapter.GetResourceGraph(rgd.Name, rgd.Namespace, rgd.RawSpec)
	} else {
		var err error
		resourceGraph, err = h.resourceParser.ParseRGDResources(rgd.Name, rgd.Namespace, rgd.RawSpec)
		if err != nil {
			response.InternalError(w, "failed to parse RGD resources: "+err.Error())
			return
		}
	}

	resp := RuntimeGraphResponse{
		RGDName:      resourceGraph.RGDName,
		RGDNamespace: resourceGraph.RGDNamespace,
		Resources:    make([]RuntimeGraphNode, len(resourceGraph.Resources)),
		ParseErrors:  resourceGraph.ParseErrors,
	}

	for i, res := range resourceGraph.Resources {
		runtimeNode := RuntimeGraphNode{
			ResourceNodeResponse: buildDefinitionNodeResponse(res),
		}
		if res.IsCollection {
			// STORY-333 will populate live counts via label queries; for now return empty status.
			runtimeNode.CollectionStatus = &models.CollectionStatus{
				Health: models.HealthHealthy,
			}
		}
		resp.Resources[i] = runtimeNode
	}

	resp.Edges = buildEdgeResponses(resourceGraph.Edges)

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetInstanceChildren handles GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/children
// and GET /api/v1/instances/{kind}/{name}/children (cluster-scoped variant).
// Returns all child resources created by KRO for this instance, grouped by node-id.
func (h *InstanceCRUDHandler) GetInstanceChildren(w http.ResponseWriter, r *http.Request) {
	if h.childService == nil {
		response.ServiceUnavailable(w, "Child resource service not available")
		return
	}

	if h.instanceTracker == nil {
		response.ServiceUnavailable(w, "Instance tracker not available")
		return
	}

	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User context not found", nil)
		return
	}

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if kind == "" || name == "" {
		response.BadRequest(w, "kind and name are required", nil)
		return
	}

	if validateInstancePathParams(w, namespace, kind, name) {
		return
	}

	// Get user's accessible namespaces for RBAC filtering
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Verify instance exists and user has access
	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	authorized, authErr := h.authorizeInstanceAccess(r.Context(), userCtx, instance, userNamespaces)
	if authErr != nil {
		h.logger.Error("failed to check instance access", "error", authErr)
		response.InternalError(w, "Failed to check instance access")
		return
	}
	if !authorized {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	// Discover child resources via label queries
	resp, err := h.childService.ListChildResources(r.Context(), namespace, kind, name)
	if err != nil {
		h.logger.Error("failed to list child resources",
			"namespace", namespace,
			"kind", kind,
			"name", name,
			"error", err,
		)
		response.InternalError(w, "Failed to list child resources")
		return
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// KubernetesEventResponse represents the API response for K8s events.
type KubernetesEventResponse struct {
	Events []KubernetesEventEntry `json:"events"`
}

// KubernetesEventEntry matches kubectl get events columns.
type KubernetesEventEntry struct {
	LastSeen  time.Time `json:"lastSeen"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Object    string    `json:"object"` // "Kind/Name"
	Message   string    `json:"message"`
	Count     int32     `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	Source    string    `json:"source"`
}

// GetInstanceEvents handles GET /api/v1/namespaces/{namespace}/instances/{kind}/{name}/events
// and GET /api/v1/instances/{kind}/{name}/events (cluster-scoped).
// Returns K8s Events for the instance and all its child resources, queried on-demand
// from the Kubernetes API (like kubectl get events).
func (h *InstanceCRUDHandler) GetInstanceEvents(w http.ResponseWriter, r *http.Request) {
	if h.k8sClient == nil {
		response.ServiceUnavailable(w, "Kubernetes client not available")
		return
	}

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if kind == "" || name == "" {
		response.BadRequest(w, "kind and name are required", nil)
		return
	}
	if validateInstancePathParams(w, namespace, kind, name) {
		return
	}

	// Get the instance to find its UID
	instance, found := h.instanceTracker.GetInstance(namespace, kind, name)
	if !found {
		response.NotFound(w, "Instance", namespace+"/"+kind+"/"+name)
		return
	}

	// Collect child resource names for matching
	childNames := make(map[string]bool)
	if h.childService != nil {
		childResp, err := h.childService.ListChildResources(r.Context(), namespace, kind, name)
		if err == nil && childResp != nil {
			for _, group := range childResp.Groups {
				for _, child := range group.Resources {
					childNames[child.Name] = true
				}
			}
		}
	}

	// Query all K8s Events in the instance's namespace
	evtNamespace := namespace
	if evtNamespace == "" {
		evtNamespace = "" // cluster-scoped: search all namespaces
	}
	eventList, err := h.k8sClient.CoreV1().Events(evtNamespace).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		h.logger.Error("failed to list K8s events", "error", err)
		response.InternalError(w, "Failed to query Kubernetes events")
		return
	}

	// Filter events: match by involvedObject.uid (instance) or involvedObject.name (children)
	var entries []KubernetesEventEntry
	for i := range eventList.Items {
		event := &eventList.Items[i]

		isInstanceEvent := string(event.InvolvedObject.UID) == instance.UID
		isChildEvent := childNames[event.InvolvedObject.Name]

		if !isInstanceEvent && !isChildEvent {
			continue
		}

		lastSeen := event.LastTimestamp.Time
		if lastSeen.IsZero() {
			lastSeen = event.EventTime.Time
		}
		if lastSeen.IsZero() {
			lastSeen = event.CreationTimestamp.Time
		}

		firstSeen := event.FirstTimestamp.Time
		if firstSeen.IsZero() {
			firstSeen = lastSeen
		}

		entries = append(entries, KubernetesEventEntry{
			LastSeen:  lastSeen,
			Type:      event.Type,
			Reason:    event.Reason,
			Object:    event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
			Message:   event.Message,
			Count:     event.Count,
			FirstSeen: firstSeen,
			Source:    event.Source.Component,
		})
	}

	// Sort by lastSeen descending (newest first)
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	if entries == nil {
		entries = []KubernetesEventEntry{}
	}

	response.WriteJSON(w, http.StatusOK, KubernetesEventResponse{Events: entries})
}
