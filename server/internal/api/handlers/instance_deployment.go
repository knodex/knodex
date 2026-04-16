// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/kro"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/history"
	"github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/repository"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// instanceCreateTimeout is the max time for creating an instance in Kubernetes.
	// 60s allows for webhook validation, resource quotas, and API server latency.
	instanceCreateTimeout = 60 * time.Second
)

// CreateInstanceRequest represents the request body for creating an instance
type CreateInstanceRequest struct {
	// Name is the name of the instance to create
	Name string `json:"name"`
	// Namespace is the target namespace for the instance
	Namespace string `json:"namespace"`
	// ProjectID is the project to deploy into (optional, for future project-based deployment)
	ProjectID string `json:"projectId,omitempty"`
	// RGDName is the name of the RGD to create an instance of
	RGDName string `json:"rgdName"`
	// RGDNamespace is the namespace of the RGD
	RGDNamespace string `json:"rgdNamespace,omitempty"`
	// Spec is the instance spec values
	Spec map[string]interface{} `json:"spec"`
	// DeploymentMode specifies how to deploy: direct, gitops, or hybrid
	// If not specified, defaults to "direct"
	DeploymentMode string `json:"deploymentMode,omitempty"`
	// RepositoryID is the Git repository config ID (required for gitops/hybrid)
	RepositoryID string `json:"repositoryId,omitempty"`
	// GitBranch overrides the repository's default branch for this deployment
	GitBranch string `json:"gitBranch,omitempty"`
	// GitPath overrides the auto-generated semantic path for this deployment
	GitPath string `json:"gitPath,omitempty"`
	// ClusterRef is the target CAPI cluster for multi-cluster deployments
	ClusterRef string `json:"clusterRef,omitempty"`
}

// CreateInstanceResponse represents the response after creating an instance
type CreateInstanceResponse struct {
	Name           string              `json:"name"`
	Namespace      string              `json:"namespace"`
	RGDName        string              `json:"rgdName"`
	APIGroup       string              `json:"apiGroup"`
	Kind           string              `json:"kind"`
	Version        string              `json:"version"`
	Status         string              `json:"status"`
	CreatedAt      string              `json:"createdAt"`
	DeploymentMode string              `json:"deploymentMode,omitempty"`
	GitInfo        *deployment.GitInfo `json:"gitInfo,omitempty"`
}

// InstanceDeploymentHandler handles instance deployment and creation
type InstanceDeploymentHandler struct {
	rgdWatcher           *watcher.RGDWatcher
	instanceTracker      *watcher.InstanceTracker
	dynamicClient        dynamic.Interface
	kubeClient           kubernetes.Interface
	deploymentController *deployment.Controller
	repoService          *repository.Service
	historyService       *history.Service
	recorder             audit.Recorder
	logger               *slog.Logger
}

// InstanceDeploymentHandlerConfig holds configuration for creating an InstanceDeploymentHandler.
type InstanceDeploymentHandlerConfig struct {
	RGDWatcher      *watcher.RGDWatcher
	InstanceTracker *watcher.InstanceTracker
	DynamicClient   dynamic.Interface
	KubeClient      kubernetes.Interface
	RepoService     *repository.Service
	HistoryService  *history.Service
	AuditRecorder   audit.Recorder
	Logger          *slog.Logger
}

// NewInstanceDeploymentHandler creates a new instance deployment handler.
func NewInstanceDeploymentHandler(config InstanceDeploymentHandlerConfig) *InstanceDeploymentHandler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	var deployCtrl *deployment.Controller
	if config.DynamicClient != nil && config.KubeClient != nil {
		deployCtrl = deployment.NewController(config.DynamicClient, config.KubeClient, logger)
	}

	return &InstanceDeploymentHandler{
		rgdWatcher:           config.RGDWatcher,
		instanceTracker:      config.InstanceTracker,
		dynamicClient:        config.DynamicClient,
		kubeClient:           config.KubeClient,
		deploymentController: deployCtrl,
		repoService:          config.RepoService,
		historyService:       config.HistoryService,
		recorder:             config.AuditRecorder,
		logger:               logger.With("component", "instance-deployment-handler"),
	}
}

// CreateInstance handles instance creation.
// K8s-aligned routes: POST /api/v1/namespaces/{ns}/instances/{kind} (namespaced)
//
//	POST /api/v1/instances/{kind} (cluster-scoped)
//
// Enforces project-scoped namespace deployment.
// Supports deployment modes: direct, gitops, hybrid.
func (h *InstanceDeploymentHandler) CreateInstance(w http.ResponseWriter, r *http.Request) {
	// Check if watcher is available
	if h.rgdWatcher == nil {
		response.ServiceUnavailable(w, "RGD watcher not available")
		return
	}

	// Check if dynamic client is available
	if h.dynamicClient == nil {
		response.ServiceUnavailable(w, "Kubernetes client not available")
		return
	}

	// Get user context from middleware
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User context not found", nil)
		return
	}

	// Parse request body (with 1MB size limit via DecodeJSON)
	req, err := helpers.DecodeJSON[CreateInstanceRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// K8s-aligned routes: namespace and kind come from path parameters.
	// Path namespace takes precedence over body namespace.
	pathNamespace := r.PathValue("namespace")
	pathKind := r.PathValue("kind")

	// STORY-348: DNS-1123 validation for K8s-bound path params
	if pathNamespace != "" && !sanitize.IsValidDNS1123Label(pathNamespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}
	if pathKind != "" && !sanitize.IsValidK8sKind(pathKind) {
		response.BadRequest(w, "kind must be a valid Kubernetes Kind name (CamelCase, starting with uppercase letter)", nil)
		return
	}

	if pathNamespace != "" {
		req.Namespace = pathNamespace
	}

	// STORY-348: Validate body namespace when path namespace is absent (cluster-scoped route).
	// Path namespace is already validated above; body namespace needs the same check.
	if pathNamespace == "" && req.Namespace != "" && !sanitize.IsValidDNS1123Label(req.Namespace) {
		response.BadRequest(w, "namespace must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)", nil)
		return
	}

	h.logger.Info("CreateInstance request received",
		"name", req.Name,
		"namespace", req.Namespace,
		"rgdName", req.RGDName)

	// Validate fields that are always required (namespace validated after RGD lookup for scope awareness)
	validationErrors := make(map[string]string)
	if req.Name == "" {
		validationErrors["name"] = "name is required"
	}
	if req.RGDName == "" {
		validationErrors["rgdName"] = "rgdName is required"
	}
	if len(validationErrors) > 0 {
		response.BadRequest(w, "validation failed", validationErrors)
		return
	}

	// Per-request permission checks removed - authorization is handled by:
	// 1. CasbinAuthz middleware for route-level authorization
	// 2. DeploymentValidator middleware for project/namespace policy validation (POST /api/v1/instances)
	// The permissionService is now only used for namespace filtering, not authorization.

	// Validate name format (DNS-1123 subdomain)
	if !sanitize.IsValidDNS1123Subdomain(req.Name) {
		response.BadRequest(w, "invalid instance name", map[string]string{
			"name": "must be lowercase alphanumeric with hyphens, starting and ending with alphanumeric",
		})
		return
	}

	// Validate ClusterRef format if provided (DNS-1123 label — K8s resource name)
	if req.ClusterRef != "" && !sanitize.IsValidDNS1123Label(req.ClusterRef) {
		response.BadRequest(w, "invalid clusterRef", map[string]string{
			"clusterRef": "must be a valid DNS-1123 label (lowercase alphanumeric with hyphens, max 63 chars)",
		})
		return
	}

	// Security: validate spec against injection/DoS attack patterns
	// before applying to Kubernetes (INJ-VULN-02)
	if req.Spec != nil {
		if err := deployment.ValidateSpecMap(req.Spec, 0, deployment.MaxSpecDepth); err != nil {
			response.BadRequest(w, "invalid spec: "+err.Error(), nil)
			return
		}
	}

	// Look up the RGD (needed before namespace validation to check scope)
	var rgd *models.CatalogRGD
	var found bool

	if req.RGDNamespace != "" {
		rgd, found = h.rgdWatcher.GetRGD(req.RGDNamespace, req.RGDName)
	} else {
		rgd, found = h.rgdWatcher.GetRGDByName(req.RGDName)
	}

	if !found {
		response.NotFound(w, "RGD", req.RGDName)
		return
	}

	// Validate path kind matches RGD kind early (before further processing)
	if pathKind != "" && pathKind != rgd.Kind {
		response.BadRequest(w, "kind in URL path does not match RGD kind", map[string]string{
			"pathKind": pathKind,
			"rgdKind":  rgd.Kind,
		})
		return
	}

	// Validate namespace: required for namespace-scoped RGDs, optional/ignored for cluster-scoped
	if !rgd.IsClusterScoped && req.Namespace == "" {
		response.BadRequest(w, "validation failed", map[string]string{
			"namespace": "namespace is required for namespace-scoped RGDs",
		})
		return
	}

	// Use namespace from request (empty for cluster-scoped)
	namespace := req.Namespace
	if rgd.IsClusterScoped {
		namespace = ""
	}

	// Extract API group and kind from the RGD
	apiGroup := kro.RGDGroup
	kind := rgd.Kind
	version := "v1alpha1"

	if rgd.APIVersion != "" {
		parts := strings.Split(rgd.APIVersion, "/")
		if len(parts) == 2 {
			apiGroup = parts[0]
			version = parts[1]
		}
	}

	// Determine deployment mode (default to direct if not specified)
	deployMode := deployment.ModeDirect
	if req.DeploymentMode != "" {
		deployMode = deployment.DeploymentMode(req.DeploymentMode)
		if !deployMode.IsValid() {
			response.BadRequest(w, "invalid deployment mode", map[string]string{
				"deploymentMode": "must be one of: direct, gitops, hybrid",
			})
			return
		}
	}

	// FAIL-FAST: Validate deployment mode is allowed for this RGD (before other validation)
	if !models.IsDeploymentModeAllowed(rgd.AllowedDeploymentModes, string(deployMode)) {
		allowedModes := rgd.AllowedDeploymentModes
		if len(allowedModes) == 0 {
			allowedModes = []string{"direct", "gitops", "hybrid"}
		}
		// Use WriteJSON directly to support array type in details (WriteError only supports map[string]string)
		response.WriteJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
			"code":    "DEPLOYMENT_MODE_NOT_ALLOWED",
			"message": "Deployment mode '" + string(deployMode) + "' is not allowed for RGD '" + req.RGDName + "'. Allowed modes: " + strings.Join(allowedModes, ", "),
			"details": map[string]interface{}{
				"allowedModes":  allowedModes, // Return as array for easier frontend consumption
				"requestedMode": string(deployMode),
			},
		})
		return
	}

	// Validate repository ID for gitops/hybrid modes
	if (deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid) && req.RepositoryID == "" {
		response.BadRequest(w, "repository ID required for gitops/hybrid mode", map[string]string{
			"repositoryId": "required when deploymentMode is gitops or hybrid",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), instanceCreateTimeout)
	defer cancel()

	// Build deployment request
	deployReq := &deployment.DeployRequest{
		InstanceID:      uuid.New().String(),
		Name:            req.Name,
		Namespace:       namespace,
		RGDName:         req.RGDName,
		RGDNamespace:    rgd.Namespace,
		APIVersion:      apiGroup + "/" + version,
		Kind:            kind,
		Spec:            req.Spec,
		IsClusterScoped: rgd.IsClusterScoped,
		DeploymentMode:  deployMode,
		ProjectID:       req.ProjectID,
		ClusterRef:      req.ClusterRef,
		CreatedBy:       userCtx.Email,
		CreatedAt:       time.Now(),
	}

	// Look up repository configuration for GitOps/Hybrid modes
	if (deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid) && req.RepositoryID != "" {
		// Validate repository service is available
		if h.repoService == nil {
			h.logger.Error("repository service not configured for GitOps deployment",
				"instance", req.Name,
				"repository_id", req.RepositoryID,
			)
			response.InternalError(w, "repository service not configured")
			return
		}

		// Look up the repository config by ID
		repoConfig, err := h.repoService.GetRepositoryConfig(ctx, req.RepositoryID)
		if err != nil {
			h.logger.Error("failed to get repository config",
				"repository_id", req.RepositoryID,
				"error", err,
			)
			response.BadRequest(w, "repository not found", map[string]string{
				"repositoryId": "repository configuration not found: " + req.RepositoryID,
			})
			return
		}

		// Convert to deployment.RepositoryConfig (shared with instance_crud.go)
		deployReq.Repository = buildDeployRepoConfig(repoConfig)

		// Set GitBranch override (from request or default to repo's default branch)
		if req.GitBranch != "" {
			deployReq.GitBranch = req.GitBranch
		} else {
			deployReq.GitBranch = repoConfig.Spec.DefaultBranch
		}

		// Set GitPath override (from request or will be auto-generated by controller)
		if req.GitPath != "" {
			deployReq.GitPath = req.GitPath
		}

		h.logger.Info("repository config loaded for GitOps deployment",
			"instance", req.Name,
			"repository_id", req.RepositoryID,
			"owner", deployReq.Repository.Owner,
			"repo", deployReq.Repository.Repo,
			"branch", deployReq.GitBranch,
			"path", deployReq.GitPath,
		)
	}

	// Check if deployment controller is available for GitOps modes
	if (deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid) &&
		h.deploymentController == nil {
		// Fall back to direct mode if deployment controller not configured
		h.logger.Warn("deployment controller not configured, falling back to direct mode",
			"requested_mode", deployMode,
			"instance", req.Name,
		)
		deployMode = deployment.ModeDirect
		deployReq.DeploymentMode = deployMode
	}

	var deployResult *deployment.DeployResult
	var deployErr error

	if h.deploymentController != nil && (deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid) {
		// Use deployment controller for GitOps/Hybrid modes
		deployResult, deployErr = h.deploymentController.Deploy(ctx, deployReq)
	} else {
		// Fallback to direct deployment (legacy behavior)
		deployResult, deployErr = h.directDeploy(ctx, deployReq)
	}

	if deployErr != nil {
		h.handleDeployError(w, deployErr)
		return
	}

	// Build response
	resp := CreateInstanceResponse{
		Name:           deployResult.Name,
		Namespace:      deployResult.Namespace,
		RGDName:        req.RGDName,
		APIGroup:       apiGroup,
		Kind:           kind,
		Version:        version,
		Status:         string(deployResult.Status),
		CreatedAt:      deployResult.DeployedAt.Format(time.RFC3339),
		DeploymentMode: string(deployResult.Mode),
	}

	// Add GitInfo if available
	if deployResult.GitPushed {
		resp.GitInfo = &deployment.GitInfo{
			CommitSHA:  deployResult.GitCommitSHA,
			Path:       deployResult.ManifestPath,
			PushStatus: deployment.GitPushSuccess,
		}
	} else if deployMode == deployment.ModeDirect {
		resp.GitInfo = &deployment.GitInfo{
			PushStatus: deployment.GitPushNotApplicable,
		}
	}

	// Record deployment creation in history service (sets rgdName for timeline merge)
	if h.historyService != nil {
		if err := h.historyService.RecordCreation(r.Context(), namespace, kind, req.Name, req.RGDName, userCtx.Email, models.DeploymentMode(deployMode)); err != nil {
			h.logger.Warn("failed to record deployment creation in history", "error", err, "instance", req.Name)
		}

		// For GitOps-only deployments, record a WaitingForSync event so the
		// Deployment Timeline shows that the instance is pending its first sync.
		if deployMode == deployment.ModeGitOps {
			waitEvent := models.DeploymentEvent{
				EventType: models.EventTypeWaitingForSync,
				Status:    string(deployment.StatusWaitingForSync),
				User:      userCtx.Email,
				Message:   "Instance is awaiting its first GitOps synchronization to provision resources.",
			}
			if err := h.historyService.RecordEvent(r.Context(), namespace, kind, req.Name, waitEvent); err != nil {
				h.logger.Warn("failed to record WaitingForSync event in history", "error", err, "instance", req.Name)
			}
		}
	}

	// Build audit details — NEVER include instance Spec (may contain secrets)
	deployDetails := map[string]any{
		"rgdName":        req.RGDName,
		"deploymentMode": string(deployMode),
		"rgdNamespace":   rgd.Namespace,
		"kind":           kind,
	}
	if deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid {
		deployDetails["gitBranch"] = deployReq.GitBranch
		deployDetails["gitPath"] = deployReq.GitPath
		deployDetails["repositoryId"] = req.RepositoryID
	}

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "deploy",
		Resource:  "instances",
		Name:      req.Name,
		Project:   req.ProjectID,
		Namespace: namespace,
		RequestID: r.Header.Get("X-Request-ID"),
		Result:    "success",
		Details:   deployDetails,
	})

	response.WriteJSON(w, http.StatusCreated, resp)
}

// directDeploy performs legacy direct deployment without the controller.
// Namespace comes from req.Namespace (not a separate parameter).
func (h *InstanceDeploymentHandler) directDeploy(ctx context.Context, req *deployment.DeployRequest) (*deployment.DeployResult, error) {
	// Build labels, annotations, and metadata via shared builder
	// Note: KRO automatically sets "kro.run/resource-graph-definition-name" label on instances
	// Note: app.kubernetes.io/managed-by will be set by KRO or GitOps tool (ArgoCD/Flux)
	mb := deployment.NewInstanceMetadataBuilder(req)

	obj := mb.BuildUnstructured(req.APIVersion, req.Kind, req.Spec)

	// Resolve GVR using discovery (falls back to naive pluralization).
	// ResolveGVR always returns nil error (fallback is built-in); the check guards
	// against future contract changes.
	gvr, err := h.instanceTracker.ResolveGVR(req.APIVersion, req.Kind)
	if err != nil {
		h.logger.Error("Failed to resolve GVR for direct deploy",
			"apiVersion", req.APIVersion, "kind", req.Kind, "error", err)
		return nil, err
	}

	// Use scope-aware dynamic client call
	var created *unstructured.Unstructured
	if req.IsClusterScoped {
		created, err = h.dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
	} else {
		created, err = h.dynamicClient.Resource(gvr).Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	}
	if err != nil {
		h.logger.Error("Failed to create resource in Kubernetes",
			"gvr", gvr.String(),
			"namespace", req.Namespace,
			"name", req.Name,
			"isClusterScoped", req.IsClusterScoped,
			"error", err.Error())
		return nil, err
	}

	return &deployment.DeployResult{
		Name:            created.GetName(),
		Namespace:       created.GetNamespace(),
		Mode:            deployment.ModeDirect,
		Status:          deployment.StatusReady,
		ClusterDeployed: true,
		DeployedAt:      created.GetCreationTimestamp().Time,
	}, nil
}

// PreflightInstance validates an instance deployment using a Kubernetes server-side
// dry-run without creating any resources. It catches admission webhook violations
// (including Gatekeeper policies) on the KRO instance resource itself.
//
// Note: this validates the KRO instance resource. Violations on child resources
// (e.g. Deployments created by KRO) are not caught here — those surface as
// condition errors after deployment.
//
// K8s-aligned routes: POST /api/v1/namespaces/{ns}/instances/{kind}/preflight
//
//	POST /api/v1/instances/{kind}/preflight
func (h *InstanceDeploymentHandler) PreflightInstance(w http.ResponseWriter, r *http.Request) {
	if h.rgdWatcher == nil {
		response.InternalError(w, "RGD watcher not available")
		return
	}

	namespace := r.PathValue("namespace")
	pathKind := r.PathValue("kind")

	req, err := helpers.DecodeJSON[CreateInstanceRequest](r, w, 0)
	if err != nil {
		return // DecodeJSON already wrote the error response
	}

	if req.Name == "" {
		req.Name = "preflight-check"
	}
	if namespace != "" {
		req.Namespace = namespace
	}

	if req.Spec != nil {
		if err := deployment.ValidateSpecMap(req.Spec, 0, deployment.MaxSpecDepth); err != nil {
			response.BadRequest(w, "invalid spec: "+err.Error(), nil)
			return
		}
	}

	var rgd *models.CatalogRGD
	var found bool
	if req.RGDNamespace != "" {
		rgd, found = h.rgdWatcher.GetRGD(req.RGDNamespace, req.RGDName)
	} else {
		rgd, found = h.rgdWatcher.GetRGDByName(req.RGDName)
	}
	if !found {
		response.NotFound(w, "RGD", req.RGDName)
		return
	}

	if pathKind != "" && pathKind != rgd.Kind {
		response.BadRequest(w, "kind in URL path does not match RGD kind", nil)
		return
	}

	// Build API version from RGD (same logic as CreateInstance)
	apiGroup := kro.RGDGroup
	version := "v1alpha1"
	if rgd.APIVersion != "" {
		parts := strings.Split(rgd.APIVersion, "/")
		if len(parts) == 2 {
			apiGroup = parts[0]
			version = parts[1]
		}
	}
	apiVersion := apiGroup + "/" + version

	mb := deployment.NewInstanceMetadataBuilder(&deployment.DeployRequest{
		Name:            req.Name,
		Namespace:       req.Namespace,
		RGDName:         req.RGDName,
		APIVersion:      apiVersion,
		Kind:            rgd.Kind,
		IsClusterScoped: rgd.IsClusterScoped,
		Spec:            req.Spec,
	})
	obj := mb.BuildUnstructured(apiVersion, rgd.Kind, req.Spec)

	gvr, err := h.instanceTracker.ResolveGVR(apiVersion, rgd.Kind)
	if err != nil {
		response.InternalError(w, "failed to resolve resource type")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), instanceCreateTimeout)
	defer cancel()

	dryRunOpts := metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}
	if rgd.IsClusterScoped {
		_, err = h.dynamicClient.Resource(gvr).Create(ctx, obj, dryRunOpts)
	} else {
		_, err = h.dynamicClient.Resource(gvr).Namespace(req.Namespace).Create(ctx, obj, dryRunOpts)
	}

	type preflightResponse struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message,omitempty"`
	}

	if err != nil {
		errMsg := err.Error()
		friendlyMsg := parseAdmissionWebhookError(errMsg)
		response.WriteJSON(w, http.StatusOK, preflightResponse{Valid: false, Message: friendlyMsg})
		return
	}

	response.WriteJSON(w, http.StatusOK, preflightResponse{Valid: true})
}

// parseAdmissionWebhookError extracts a user-friendly message from a Kubernetes
// admission webhook denial error. Returns the original error string if no
// known pattern matches.
func parseAdmissionWebhookError(errMsg string) string {
	// Gatekeeper: admission webhook "validation.gatekeeper.sh" denied the request: [constraint] reason
	if idx := strings.Index(errMsg, `admission webhook "validation.gatekeeper.sh" denied the request: `); idx != -1 {
		rest := errMsg[idx+len(`admission webhook "validation.gatekeeper.sh" denied the request: `):]
		// Extract [constraint-name]
		if rest != "" && rest[0] == '[' {
			end := strings.Index(rest, "]")
			if end > 1 {
				constraint := rest[1:end]
				reason := strings.TrimSpace(rest[end+1:])
				return `Blocked by Gatekeeper policy "` + constraint + `": ` + reason
			}
		}
		return "Blocked by Gatekeeper policy: " + rest
	}
	// Generic admission webhook denial
	if idx := strings.Index(errMsg, `admission webhook "`); idx != -1 {
		return "Blocked by admission webhook: " + errMsg[idx:]
	}
	return errMsg
}

// handleDeployError maps deployment errors to HTTP responses.
// Raw error details are logged server-side only; clients receive generic messages.
func (h *InstanceDeploymentHandler) handleDeployError(w http.ResponseWriter, err error) {
	errMsg := err.Error()

	// Log all deployment errors for debugging (server-side only)
	h.logger.Error("Deployment failed", "error", errMsg)

	if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no matches") {
		response.WriteError(w, http.StatusNotFound, response.ErrCodeNotFound,
			"Resource type not available. The requested resource definition may not be ready yet.",
			nil)
		return
	}
	if strings.Contains(errMsg, "already exists") {
		response.WriteError(w, http.StatusConflict, "CONFLICT",
			"Instance already exists: An instance with this name already exists in the specified namespace.",
			nil)
		return
	}
	if strings.Contains(errMsg, "forbidden") {
		response.WriteError(w, http.StatusForbidden, "FORBIDDEN",
			"Permission denied: The service account does not have permission to create this resource.",
			nil)
		return
	}
	if strings.Contains(errMsg, "admission webhook") {
		friendlyMsg := parseAdmissionWebhookError(errMsg)
		response.WriteError(w, http.StatusUnprocessableEntity, "ADMISSION_DENIED", friendlyMsg, nil)
		return
	}
	if strings.Contains(errMsg, "GitHub") || strings.Contains(errMsg, "git push") || strings.Contains(errMsg, "git clone") {
		response.WriteError(w, http.StatusBadGateway, "GITOPS_ERROR",
			"GitOps deployment failed",
			nil)
		return
	}

	response.InternalError(w, "Failed to create instance")
}
