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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/provops-org/knodex/server/internal/api/helpers"
	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/audit"
	"github.com/provops-org/knodex/server/internal/deployment"
	"github.com/provops-org/knodex/server/internal/deployment/vcs"
	"github.com/provops-org/knodex/server/internal/models"
	"github.com/provops-org/knodex/server/internal/repository"
	"github.com/provops-org/knodex/server/internal/watcher"
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
	dynamicClient        dynamic.Interface
	kubeClient           kubernetes.Interface
	deploymentController *deployment.Controller
	repoService          *repository.Service
	recorder             audit.Recorder
	logger               *slog.Logger
}

// InstanceDeploymentHandlerConfig holds configuration for creating an InstanceDeploymentHandler.
type InstanceDeploymentHandlerConfig struct {
	RGDWatcher    *watcher.RGDWatcher
	DynamicClient dynamic.Interface
	KubeClient    kubernetes.Interface
	RepoService   *repository.Service
	AuditRecorder audit.Recorder
	Logger        *slog.Logger
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
		dynamicClient:        config.DynamicClient,
		kubeClient:           config.KubeClient,
		deploymentController: deployCtrl,
		repoService:          config.RepoService,
		recorder:             config.AuditRecorder,
		logger:               logger.With("component", "instance-deployment-handler"),
	}
}

// CreateInstance handles POST /api/v1/instances
// Enforces project-scoped namespace deployment
// Supports deployment modes: direct, gitops, hybrid
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

	slog.Info("CreateInstance request received",
		"name", req.Name,
		"namespace", req.Namespace,
		"rgdName", req.RGDName)

	// Validate required fields
	validationErrors := make(map[string]string)
	if req.Name == "" {
		validationErrors["name"] = "name is required"
	}
	if req.Namespace == "" {
		validationErrors["namespace"] = "namespace is required"
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

	// Use namespace from request
	namespace := req.Namespace

	// Validate name format (DNS-1123 subdomain)
	if !isValidDNS1123Name(req.Name) {
		response.BadRequest(w, "invalid instance name", map[string]string{
			"name": "must be lowercase alphanumeric with hyphens, starting and ending with alphanumeric",
		})
		return
	}

	// Look up the RGD
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

	// Extract API group and kind from the RGD
	apiGroup := "kro.run"
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
		InstanceID:     uuid.New().String(),
		Name:           req.Name,
		Namespace:      namespace,
		RGDName:        req.RGDName,
		RGDNamespace:   rgd.Namespace,
		APIVersion:     apiGroup + "/" + version,
		Kind:           kind,
		Spec:           req.Spec,
		DeploymentMode: deployMode,
		ProjectID:      req.ProjectID,
		CreatedBy:      userCtx.Email,
		CreatedAt:      time.Now(),
	}

	// Look up repository configuration for GitOps/Hybrid modes
	if (deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid) && req.RepositoryID != "" {
		// Validate repository service is available
		if h.repoService == nil {
			slog.Error("repository service not configured for GitOps deployment",
				"instance", req.Name,
				"repository_id", req.RepositoryID,
			)
			response.InternalError(w, "repository service not configured")
			return
		}

		// Look up the repository config by ID
		repoConfig, err := h.repoService.GetRepositoryConfig(ctx, req.RepositoryID)
		if err != nil {
			slog.Error("failed to get repository config",
				"repository_id", req.RepositoryID,
				"error", err,
			)
			response.BadRequest(w, "repository not found", map[string]string{
				"repositoryId": "repository configuration not found: " + req.RepositoryID,
			})
			return
		}

		// Verify repository is enabled
		if !repoConfig.Spec.Enabled {
			response.BadRequest(w, "repository is disabled", map[string]string{
				"repositoryId": "repository is disabled and cannot be used for deployments",
			})
			return
		}

		// Detect provider from repository URL
		parsedURL, parseErr := vcs.ParseRepoURL(repoConfig.Spec.RepoURL)
		provider := ""
		if parseErr == nil && parsedURL.Provider != "" {
			provider = string(parsedURL.Provider)
		}

		// Determine the correct secret key based on auth type
		// HTTPS with bearer token uses "bearerToken", basic auth uses "password"
		// SSH uses "sshPrivateKey", GitHub App has its own keys
		secretKey := "bearerToken" // Default to bearer token for HTTPS
		switch repoConfig.Spec.AuthType {
		case "https":
			// For HTTPS, prefer bearerToken (GitHub PAT), fall back to password
			secretKey = "bearerToken"
		case "ssh":
			secretKey = "sshPrivateKey"
		case "github-app":
			// GitHub App uses dynamic token generation (handled elsewhere)
			secretKey = "githubAppPrivateKey"
		}

		// Convert to deployment.RepositoryConfig
		// Owner/Repo are parsed from RepoURL (no longer stored in spec)
		owner := ""
		repo := ""
		if parsedURL != nil {
			owner = parsedURL.Owner
			repo = parsedURL.Repo
		}
		deployReq.Repository = &deployment.RepositoryConfig{
			ID:            repoConfig.Name,
			Name:          repoConfig.Spec.Name,
			ProjectID:     repoConfig.Spec.ProjectID,
			Provider:      provider,
			Owner:         owner,
			Repo:          repo,
			Branch:        repoConfig.Spec.DefaultBranch,
			DefaultBranch: repoConfig.Spec.DefaultBranch,
			BasePath:      "manifests", // Default base path for GitOps
			Enabled:       repoConfig.Spec.Enabled,
			SecretRef: deployment.SecretReference{
				Name:      repoConfig.Spec.SecretRef.Name,
				Namespace: repoConfig.Spec.SecretRef.Namespace,
				Key:       secretKey,
			},
			SecretName:      repoConfig.Spec.SecretRef.Name,
			SecretNamespace: repoConfig.Spec.SecretRef.Namespace,
			SecretKey:       secretKey,
		}

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

		slog.Info("repository config loaded for GitOps deployment",
			"instance", req.Name,
			"repository_id", req.RepositoryID,
			"owner", owner,
			"repo", repo,
			"branch", deployReq.GitBranch,
			"path", deployReq.GitPath,
		)
	}

	// Check if deployment controller is available for GitOps modes
	if (deployMode == deployment.ModeGitOps || deployMode == deployment.ModeHybrid) &&
		h.deploymentController == nil {
		// Fall back to direct mode if deployment controller not configured
		slog.Warn("deployment controller not configured, falling back to direct mode",
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
		deployResult, deployErr = h.directDeploy(ctx, deployReq, apiGroup, version, kind, namespace)
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
		Details:   map[string]any{"rgdName": req.RGDName, "deploymentMode": string(deployMode)},
	})

	response.WriteJSON(w, http.StatusCreated, resp)
}

// directDeploy performs legacy direct deployment without the controller
func (h *InstanceDeploymentHandler) directDeploy(ctx context.Context, req *deployment.DeployRequest, apiGroup, version, kind, namespace string) (*deployment.DeployResult, error) {
	// Build the unstructured object
	// Note: KRO automatically sets "kro.run/resource-graph-definition-name" label on instances
	// Note: app.kubernetes.io/managed-by will be set by KRO or GitOps tool (ArgoCD/Flux)
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": req.APIVersion,
			"kind":       req.Kind,
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					models.ProjectLabel:        namespace, // Use namespace name (e.g., "acme") not project ID
					models.DeploymentModeLabel: string(deployment.ModeDirect),
				},
			},
			"spec": req.Spec,
		},
	}

	// Create the resource
	gvr := schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  version,
		Resource: strings.ToLower(kind) + "s",
	}

	created, err := h.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		slog.Error("Failed to create resource in Kubernetes",
			"gvr", gvr.String(),
			"namespace", namespace,
			"name", req.Name,
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

// handleDeployError maps deployment errors to HTTP responses.
// Raw error details are logged server-side only; clients receive generic messages.
func (h *InstanceDeploymentHandler) handleDeployError(w http.ResponseWriter, err error) {
	errMsg := err.Error()

	// Log all deployment errors for debugging (server-side only)
	slog.Error("Deployment failed", "error", errMsg)

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
	if strings.Contains(errMsg, "GitHub") || strings.Contains(errMsg, "git push") || strings.Contains(errMsg, "git clone") {
		response.WriteError(w, http.StatusBadGateway, "GITOPS_ERROR",
			"GitOps deployment failed",
			nil)
		return
	}

	response.InternalError(w, "Failed to create instance")
}

// isValidDNS1123Name validates a name matches DNS-1123 subdomain format
func isValidDNS1123Name(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for i, c := range name {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '-' && i > 0 && i < len(name)-1 {
			continue
		}
		return false
	}
	return true
}
