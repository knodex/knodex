package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/provops-org/knodex/server/internal/api/helpers"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/audit"
	"github.com/provops-org/knodex/server/internal/rbac"
	"github.com/provops-org/knodex/server/internal/repository"
)

// RepositoryHandler handles repository configuration endpoints
type RepositoryHandler struct {
	repoService       *repository.Service
	permissionService *rbac.PermissionService
	enforcer          rbac.PolicyEnforcer
	recorder          audit.Recorder
}

// NewRepositoryHandler creates a new RepositoryHandler
// Added enforcer parameter for project-scoped authorization
func NewRepositoryHandler(repoService *repository.Service, permissionService *rbac.PermissionService, enforcer rbac.PolicyEnforcer, recorder audit.Recorder) *RepositoryHandler {
	return &RepositoryHandler{
		repoService:       repoService,
		permissionService: permissionService,
		enforcer:          enforcer,
		recorder:          recorder,
	}
}

// CreateRepositoryConfigRequest represents the request body for creating a repository config
// using the ArgoCD-style credential model (repoURL + authType + inline credentials)
type CreateRepositoryConfigRequest struct {
	Name          string `json:"name"`
	ProjectID     string `json:"projectId"`
	RepoURL       string `json:"repoURL"`
	AuthType      string `json:"authType"`
	DefaultBranch string `json:"defaultBranch"`
	Enabled       bool   `json:"enabled"`

	// Auth-specific credentials (only one should be provided based on authType)
	SSHAuth       *repository.SSHAuthConfig       `json:"sshAuth,omitempty"`
	HTTPSAuth     *repository.HTTPSAuthConfig     `json:"httpsAuth,omitempty"`
	GitHubAppAuth *repository.GitHubAppAuthConfig `json:"githubAppAuth,omitempty"`

	// Legacy fields — detected and rejected with 400
	Owner     string `json:"owner,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
}

// UpdateRepositoryConfigRequest represents the request body for updating a repository config
type UpdateRepositoryConfigRequest struct {
	Name          string `json:"name,omitempty"`
	DefaultBranch string `json:"defaultBranch,omitempty"`
	Enabled       *bool  `json:"enabled,omitempty"`

	// Credential update fields (ArgoCD-style)
	RepoURL       string                          `json:"repoURL,omitempty"`
	AuthType      string                          `json:"authType,omitempty"`
	SSHAuth       *repository.SSHAuthConfig       `json:"sshAuth,omitempty"`
	HTTPSAuth     *repository.HTTPSAuthConfig     `json:"httpsAuth,omitempty"`
	GitHubAppAuth *repository.GitHubAppAuthConfig `json:"githubAppAuth,omitempty"`
}

// CreateRepositoryConfig handles POST /api/v1/repositories
// Creates a repository config using the ArgoCD-style format with inline credentials
// Note: Access control is handled by authorization middleware
func (h *RepositoryHandler) CreateRepositoryConfig(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Parse request body
	req, err := helpers.DecodeJSON[CreateRepositoryConfigRequest](r, w, 0)
	if err != nil {
		slog.Error("failed to parse create repository request", "error", err)
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Reject legacy format (owner/repo/secretName)
	if req.Owner != "" || req.SecretKey != "" {
		response.BadRequest(w, "Legacy format (owner/repo/secretName) is no longer supported. Use repoURL/authType format.", nil)
		return
	}

	// Validate required fields
	if req.Name == "" {
		response.BadRequest(w, "name is required", nil)
		return
	}
	if req.ProjectID == "" {
		response.BadRequest(w, "projectId is required", nil)
		return
	}
	if req.RepoURL == "" {
		response.BadRequest(w, "repoURL is required", nil)
		return
	}
	if req.AuthType == "" {
		response.BadRequest(w, "authType is required", nil)
		return
	}
	if req.DefaultBranch == "" {
		response.BadRequest(w, "defaultBranch is required", nil)
		return
	}

	// Validate auth type and corresponding auth config
	if !repository.ValidateAuthType(req.AuthType) {
		response.BadRequest(w, fmt.Sprintf("authType must be one of: %v", repository.ValidAuthTypes()), nil)
		return
	}

	// Validate that the appropriate auth config is provided
	switch req.AuthType {
	case repository.AuthTypeSSH:
		if req.SSHAuth == nil {
			response.BadRequest(w, "sshAuth is required for SSH authentication", nil)
			return
		}
	case repository.AuthTypeHTTPS:
		if req.HTTPSAuth == nil {
			response.BadRequest(w, "httpsAuth is required for HTTPS authentication", nil)
			return
		}
		// SECURITY: Reject insecureSkipTLSVerify to prevent MITM attacks
		if req.HTTPSAuth.InsecureSkipTLSVerify {
			response.BadRequest(w, "insecureSkipTLSVerify is not allowed; use a trusted CA certificate instead", nil)
			return
		}
	case repository.AuthTypeGitHubApp:
		if req.GitHubAppAuth == nil {
			response.BadRequest(w, "githubAppAuth is required for GitHub App authentication", nil)
			return
		}
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	createdBy := userCtx.UserID

	// Check project-scoped repository create permission
	repoObject := fmt.Sprintf("repositories/%s/*", req.ProjectID)

	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, repoObject, "create", requestID) {
		return
	}

	// Create repository using the service method
	serviceReq := repository.CreateRepositoryRequest{
		Name:          req.Name,
		ProjectID:     req.ProjectID,
		RepoURL:       req.RepoURL,
		AuthType:      req.AuthType,
		DefaultBranch: req.DefaultBranch,
		Enabled:       req.Enabled,
		SSHAuth:       req.SSHAuth,
		HTTPSAuth:     req.HTTPSAuth,
		GitHubAppAuth: req.GitHubAppAuth,
	}

	repoConfig, err := h.repoService.CreateRepositoryConfigWithCredentials(r.Context(), serviceReq, createdBy)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			slog.Warn("repository config creation rejected", "project_id", req.ProjectID, "repo_url", req.RepoURL, "auth_type", req.AuthType, "error", err)
			response.BadRequest(w, "repository configuration already exists", nil)
			return
		}
		if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "required") ||
			strings.Contains(errMsg, "PEM format") || strings.Contains(errMsg, "must be") {
			slog.Warn("repository config creation rejected", "project_id", req.ProjectID, "repo_url", req.RepoURL, "auth_type", req.AuthType, "error", err)
			response.BadRequest(w, "invalid repository configuration or credentials", nil)
			return
		}

		if strings.Contains(errMsg, "not authorized") || strings.Contains(errMsg, "authorization") {
			slog.Warn("repository config creation forbidden", "project_id", req.ProjectID, "created_by", createdBy, "error", err)
			response.Forbidden(w, "insufficient permissions to manage repositories")
			return
		}

		slog.Error("failed to create repository config", "project_id", req.ProjectID, "repo_url", req.RepoURL, "auth_type", req.AuthType, "error", err)
		response.InternalError(w, "failed to create repository configuration")
		return
	}

	slog.Info("repository config created successfully",
		"repo_config_id", repoConfig.Name,
		"project_id", req.ProjectID,
		"repo_url", req.RepoURL,
		"auth_type", req.AuthType,
		"created_by", createdBy,
	)

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "repositories",
		Name:      repoConfig.Name,
		Project:   req.ProjectID,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"authType": req.AuthType},
	})

	response.WriteJSON(w, http.StatusCreated, repoConfig.ToRepositoryConfigInfo())
}

// ListRepositoryConfigs handles GET /api/v1/repositories
// Query parameters:
//   - projectId: optional filter by project ID
//
// Access control filters results based on user permissions
func (h *RepositoryHandler) ListRepositoryConfigs(w http.ResponseWriter, r *http.Request) {
	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Get optional projectId filter from query parameters
	projectID := r.URL.Query().Get("projectId")

	// If projectId is specified, check permission for that project
	if h.enforcer != nil && projectID != "" {
		repoObject := fmt.Sprintf("repositories/%s/*", projectID)
		allowed, err := h.enforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, repoObject, "get")
		if err != nil {
			slog.Error("failed to check repository authorization",
				"userId", userCtx.UserID,
				"projectId", projectID,
				"error", err,
			)
			response.InternalError(w, "Failed to check authorization")
			return
		}
		if !allowed {
			slog.Warn("unauthorized repository list attempt for project",
				"userId", userCtx.UserID,
				"projectId", projectID,
			)
			response.Forbidden(w, "Insufficient permissions to list repositories for this project")
			return
		}
	}

	// List repository configs (optionally filtered by project)
	repoConfigList, err := h.repoService.ListRepositoryConfigs(r.Context(), projectID)
	if err != nil {
		slog.Error("failed to list repository configs",
			"project_id", projectID,
			"error", err,
		)
		response.InternalError(w, "failed to list repository configurations")
		return
	}

	// Filter results based on user permissions if no projectId filter
	// Users without global access only see repos for projects they can access
	var filteredItems []repository.RepositoryConfig
	if h.enforcer != nil && projectID == "" {
		// Check global access first
		hasGlobalAccess, _ := h.enforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, "repositories/*", "get")
		if hasGlobalAccess {
			filteredItems = repoConfigList.Items
		} else {
			// Filter to repos in projects user can access
			for _, repoConfig := range repoConfigList.Items {
				repoProjectID := repoConfig.Spec.ProjectID
				if repoProjectID == "" {
					continue // Skip repos without project association
				}
				repoObject := fmt.Sprintf("repositories/%s/*", repoProjectID)
				allowed, _ := h.enforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, repoObject, "get")
				if allowed {
					filteredItems = append(filteredItems, repoConfig)
				}
			}
		}
	} else {
		filteredItems = repoConfigList.Items
	}

	// Convert to RepositoryConfigInfo list
	infoList := make([]*repository.RepositoryConfigInfo, len(filteredItems))
	for i, repoConfig := range filteredItems {
		rc := repoConfig // Create a copy to avoid pointer issues
		infoList[i] = rc.ToRepositoryConfigInfo()
	}

	// Return in the format expected by frontend: { items: [...], totalCount: N }
	responseData := map[string]interface{}{
		"items":      infoList,
		"totalCount": len(infoList),
	}

	response.WriteJSON(w, http.StatusOK, responseData)
}

// GetRepositoryConfig handles GET /api/v1/repositories/{repoId}
// Access control checks project-scoped permission
func (h *RepositoryHandler) GetRepositoryConfig(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoId")
	requestID := r.Header.Get("X-Request-ID")

	if repoID == "" {
		response.BadRequest(w, "repository ID is required", nil)
		return
	}

	// Get repository config
	repoConfig, err := h.repoService.GetRepositoryConfig(r.Context(), repoID)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "repository configuration", repoID)
			return
		}
		slog.Error("failed to get repository config",
			"repo_id", repoID,
			"error", err,
		)
		response.InternalError(w, "failed to get repository configuration")
		return
	}

	// Get user context and check project-scoped permission
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	var repoObject string
	if repoConfig.Spec.ProjectID != "" {
		repoObject = fmt.Sprintf("repositories/%s/%s", repoConfig.Spec.ProjectID, repoID)
	} else {
		repoObject = fmt.Sprintf("repositories/*/%s", repoID)
	}

	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, repoObject, "get", requestID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, repoConfig.ToRepositoryConfigInfo())
}

// UpdateRepositoryConfig handles PATCH /api/v1/repositories/{repoId}
// Access control checks project-scoped permission
func (h *RepositoryHandler) UpdateRepositoryConfig(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoId")
	requestID := r.Header.Get("X-Request-ID")

	if repoID == "" {
		response.BadRequest(w, "repository ID is required", nil)
		return
	}

	// Parse request body
	req, err := helpers.DecodeJSON[UpdateRepositoryConfigRequest](r, w, 0)
	if err != nil {
		slog.Error("failed to parse update repository config request", "error", err)
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	updatedBy := userCtx.UserID

	// Get current repository config
	repoConfig, err := h.repoService.GetRepositoryConfig(r.Context(), repoID)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "repository configuration", repoID)
			return
		}
		slog.Error("failed to get repository config",
			"repo_id", repoID,
			"error", err,
		)
		response.InternalError(w, "failed to get repository configuration")
		return
	}

	// Check project-scoped permission for update
	var repoObject string
	if repoConfig.Spec.ProjectID != "" {
		repoObject = fmt.Sprintf("repositories/%s/%s", repoConfig.Spec.ProjectID, repoID)
	} else {
		repoObject = fmt.Sprintf("repositories/*/%s", repoID)
	}

	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, repoObject, "update", requestID) {
		return
	}

	// Update metadata fields (only update non-empty fields from request)
	if req.Name != "" {
		repoConfig.Spec.Name = req.Name
	}
	if req.DefaultBranch != "" {
		repoConfig.Spec.DefaultBranch = req.DefaultBranch
	}
	if req.Enabled != nil {
		repoConfig.Spec.Enabled = *req.Enabled
	}

	// Handle credential updates if authType or auth config fields are provided
	if req.AuthType != "" || req.SSHAuth != nil || req.HTTPSAuth != nil || req.GitHubAppAuth != nil {
		// Determine the auth type to use
		authType := req.AuthType
		if authType == "" {
			authType = repoConfig.Spec.AuthType
		}

		if !repository.ValidateAuthType(authType) {
			response.BadRequest(w, fmt.Sprintf("authType must be one of: %v", repository.ValidAuthTypes()), nil)
			return
		}

		// SECURITY: Reject insecureSkipTLSVerify
		if req.HTTPSAuth != nil && req.HTTPSAuth.InsecureSkipTLSVerify {
			response.BadRequest(w, "insecureSkipTLSVerify is not allowed; use a trusted CA certificate instead", nil)
			return
		}

		// Update credentials via credential manager
		if h.repoService.CredentialManager() != nil {
			updateReq := repository.CreateRepositorySecretRequest{
				Name:          repoConfig.Spec.Name,
				ProjectID:     repoConfig.Spec.ProjectID,
				RepoURL:       repoConfig.Spec.RepoURL,
				AuthType:      authType,
				DefaultBranch: repoConfig.Spec.DefaultBranch,
				Enabled:       repoConfig.Spec.Enabled,
				SSHAuth:       req.SSHAuth,
				HTTPSAuth:     req.HTTPSAuth,
				GitHubAppAuth: req.GitHubAppAuth,
				CreatedBy:     updatedBy,
			}

			// Use RepoURL from request if provided
			if req.RepoURL != "" {
				updateReq.RepoURL = req.RepoURL
				repoConfig.Spec.RepoURL = req.RepoURL
			}

			err = h.repoService.UpdateRepositorySecret(r.Context(), repoConfig.Spec.SecretRef.Name, updateReq)
			if err != nil {
				errMsg := err.Error()
				if strings.Contains(errMsg, "invalid") || strings.Contains(errMsg, "required") ||
					strings.Contains(errMsg, "PEM format") || strings.Contains(errMsg, "must be") {
					response.BadRequest(w, "invalid credential update: "+errMsg, nil)
					return
				}
				slog.Error("failed to update repository credentials",
					"repo_id", repoID,
					"error", err,
				)
				response.InternalError(w, "failed to update repository credentials")
				return
			}
		}

		// Update auth type in spec
		repoConfig.Spec.AuthType = authType
	}

	// Save metadata changes to the Secret
	updatedRepoConfig, err := h.repoService.UpdateRepositoryMetadata(
		r.Context(), repoID,
		repoConfig.Spec.Name, repoConfig.Spec.DefaultBranch, repoConfig.Spec.Enabled,
		updatedBy,
	)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
			slog.Warn("repository config update rejected", "repo_id", repoID, "error", err)
			response.NotFound(w, "repository configuration", repoID)
			return
		}

		slog.Error("failed to update repository config",
			"repo_id", repoID,
			"error", err,
		)
		response.InternalError(w, "failed to update repository configuration")
		return
	}

	slog.Info("repository config updated successfully",
		"repo_config_id", repoID,
		"updated_by", updatedBy,
	)

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "repositories",
		Name:      repoID,
		Project:   repoConfig.Spec.ProjectID,
		RequestID: requestID,
		Result:    "success",
	})

	response.WriteJSON(w, http.StatusOK, updatedRepoConfig.ToRepositoryConfigInfo())
}

// DeleteRepositoryConfig handles DELETE /api/v1/repositories/{repoId}
// Access control checks project-scoped permission
func (h *RepositoryHandler) DeleteRepositoryConfig(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoId")
	requestID := r.Header.Get("X-Request-ID")

	if repoID == "" {
		response.BadRequest(w, "repository ID is required", nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	deletedBy := userCtx.UserID

	// Get repository config first to check project-scoped permission
	repoConfig, err := h.repoService.GetRepositoryConfig(r.Context(), repoID)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "repository configuration", repoID)
			return
		}
		slog.Error("failed to get repository config for authorization",
			"repo_id", repoID,
			"error", err,
		)
		response.InternalError(w, "failed to get repository configuration")
		return
	}

	var repoObject string
	if repoConfig.Spec.ProjectID != "" {
		repoObject = fmt.Sprintf("repositories/%s/%s", repoConfig.Spec.ProjectID, repoID)
	} else {
		repoObject = fmt.Sprintf("repositories/*/%s", repoID)
	}

	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, repoObject, "delete", requestID) {
		return
	}

	// Delete repository config
	err = h.repoService.DeleteRepositoryConfig(r.Context(), repoID, deletedBy)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "repository configuration", repoID)
			return
		}
		if strings.Contains(err.Error(), "not authorized") || strings.Contains(err.Error(), "authorization") {
			slog.Warn("repository config deletion forbidden",
				"repo_id", repoID,
				"deleted_by", deletedBy,
				"error", err,
			)
			response.Forbidden(w, "insufficient permissions to manage repositories")
			return
		}

		slog.Error("failed to delete repository config",
			"repo_id", repoID,
			"error", err,
		)
		response.InternalError(w, "failed to delete repository configuration")
		return
	}

	slog.Info("repository config deleted successfully",
		"repo_config_id", repoID,
		"deleted_by", deletedBy,
	)

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "repositories",
		Name:      repoID,
		Project:   repoConfig.Spec.ProjectID,
		RequestID: requestID,
		Result:    "success",
	})

	// Return 204 No Content on successful deletion
	w.WriteHeader(http.StatusNoContent)
}

// TestConnection handles POST /api/v1/repositories/test-connection
// Tests repository credentials using ArgoCD-style authentication methods
// Note: Access control is handled by authorization middleware
func (h *RepositoryHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	// Parse request body
	req, err := helpers.DecodeJSON[repository.TestConnectionWithCredentialsRequest](r, w, 0)
	if err != nil {
		slog.Error("failed to parse test connection request", "error", err)
		response.BadRequest(w, err.Error(), nil)
		return
	}

	// Test the connection
	result, err := h.repoService.TestConnectionWithCredentials(r.Context(), *req)
	if err != nil {
		slog.Error("failed to test repository connection",
			"user_id", userCtx.UserID,
			"repo_url", req.RepoURL,
			"auth_type", req.AuthType,
			"error", err,
		)
		response.InternalError(w, "failed to test repository connection")
		return
	}

	// Log the test result
	if result.Valid {
		slog.Info("repository connection test passed",
			"user_id", userCtx.UserID,
			"repo_url", req.RepoURL,
			"auth_type", req.AuthType,
		)
	} else {
		slog.Info("repository connection test failed",
			"user_id", userCtx.UserID,
			"repo_url", req.RepoURL,
			"auth_type", req.AuthType,
			"message", result.Message,
		)
	}

	// Return the result
	response.WriteJSON(w, http.StatusOK, result)
}

// TestRepositoryConnection handles POST /api/v1/repositories/{repoId}/test
// Tests connectivity to a GitHub repository using a saved repository configuration
// Note: Access control is handled by authorization middleware
func (h *RepositoryHandler) TestRepositoryConnection(w http.ResponseWriter, r *http.Request) {
	repoID := r.PathValue("repoId")
	requestID := r.Header.Get("X-Request-ID")

	if repoID == "" {
		response.BadRequest(w, "repository ID is required", nil)
		return
	}

	// Get user context
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	userID := userCtx.UserID

	// Get repository config
	repoConfig, err := h.repoService.GetRepositoryConfig(r.Context(), repoID)
	if err != nil {
		if helpers.IsNotFoundError(err) {
			response.NotFound(w, "repository configuration", repoID)
			return
		}
		slog.Error("failed to get repository config",
			"repo_id", repoID,
			"error", err,
		)
		response.InternalError(w, "failed to get repository configuration")
		return
	}

	// Check project-scoped repository get permission
	// Testing a repo connection requires read access to the repository
	var repoObject string
	if repoConfig.Spec.ProjectID != "" {
		repoObject = fmt.Sprintf("repositories/%s/%s", repoConfig.Spec.ProjectID, repoID)
	} else {
		repoObject = fmt.Sprintf("repositories/*/%s", repoID)
	}

	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, repoObject, "get", requestID) {
		return
	}

	// Test the connection
	err = h.repoService.TestRepositoryConnection(r.Context(), repoID, userID)
	if err != nil {
		// Extract user-friendly error message
		errMsg := err.Error()

		// Check for common error types that should return specific HTTP codes
		if strings.Contains(errMsg, "not authorized") || strings.Contains(errMsg, "authorization") {
			slog.Warn("repository connection test forbidden",
				"repo_id", repoID,
				"user_id", userID,
				"error", err,
			)
			response.Forbidden(w, "insufficient permissions to test repository connection")
			return
		}

		// All connection test failures are user errors (bad credentials, no access, etc.)
		slog.Info("repository connection test failed",
			"repo_id", repoID,
			"repo_url", repoConfig.Spec.RepoURL,
			"error", errMsg,
		)

		// Return generic error message — internal details logged server-side
		response.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Connection test failed",
			"error":   sanitizeConnectionError(errMsg),
		})
		return
	}

	// Success
	slog.Info("repository connection test successful",
		"repo_id", repoID,
		"repo_url", repoConfig.Spec.RepoURL,
		"user_id", userID,
	)

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
		"details": map[string]string{
			"repository": repoConfig.Spec.RepoURL,
			"branch":     repoConfig.Spec.DefaultBranch,
		},
	})
}

// sanitizeConnectionError returns a user-safe error message for connection test failures.
// Strips internal details (IPs, service names, stack traces) while preserving actionable info.
func sanitizeConnectionError(errMsg string) string {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(lower, "authentication") || strings.Contains(lower, "credentials") || strings.Contains(lower, "401"):
		return "Authentication failed — check your credentials"
	case strings.Contains(lower, "not found") || strings.Contains(lower, "404"):
		return "Repository not found — check the repository URL"
	case strings.Contains(lower, "forbidden") || strings.Contains(lower, "403"):
		return "Access denied — check your repository permissions"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline"):
		return "Connection timed out — check network connectivity"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host"):
		return "Unable to reach repository host — check the URL and network"
	default:
		return "Connection test failed — check repository URL and credentials"
	}
}
