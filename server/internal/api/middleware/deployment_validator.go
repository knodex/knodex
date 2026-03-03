package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/provops-org/knodex/server/internal/rbac"
)

// DeploymentValidatorConfig holds configuration for deployment validation middleware
type DeploymentValidatorConfig struct {
	// ProjectService provides project CRUD operations
	ProjectService rbac.ProjectServiceInterface
	// PolicyEnforcer provides Casbin policy enforcement
	PolicyEnforcer rbac.PolicyEnforcer
	// Logger for validation logging
	Logger *slog.Logger
}

// DeploymentRequest represents fields needed for deployment validation
// These are extracted from the instance creation request body
type DeploymentRequest struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	ProjectID    string `json:"projectId,omitempty"`
	RGDName      string `json:"rgdName"`
	RGDNamespace string `json:"rgdNamespace,omitempty"`
	// Deprecated: ClusterServer is ignored. All deployments target the local cluster.
	ClusterServer  string `json:"clusterServer,omitempty"`
	RepositoryID   string `json:"repositoryId,omitempty"`
	DeploymentMode string `json:"deploymentMode,omitempty"`
}

// ValidationError represents a deployment policy validation failure
type ValidationError struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details string   `json:"details"`
	Allowed []string `json:"allowed,omitempty"`
}

// Validation error codes
const (
	ErrCodeProjectNotFound       = "PROJECT_NOT_FOUND"
	ErrCodePermissionDenied      = "PERMISSION_DENIED"
	ErrCodeDestinationNotAllowed = "DESTINATION_NOT_ALLOWED"
)

// deploymentRequestContextKey is the context key for validated deployment request
type deploymentRequestContextKey struct{}

// validatedProjectContextKey is the context key for validated project
type validatedProjectContextKey struct{}

// GetValidatedDeploymentRequest retrieves the validated deployment request from context
func GetValidatedDeploymentRequest(r *http.Request) (*DeploymentRequest, bool) {
	req, ok := r.Context().Value(deploymentRequestContextKey{}).(*DeploymentRequest)
	return req, ok
}

// GetValidatedProject retrieves the validated project from context
func GetValidatedProject(r *http.Request) (*rbac.Project, bool) {
	project, ok := r.Context().Value(validatedProjectContextKey{}).(*rbac.Project)
	return project, ok
}

// DeploymentValidator returns middleware that validates deployment requests against Project policies
// It validates:
// 1. Project exists (if projectId is provided)
// 2. User has deploy permission in the project
// 3. Target namespace is allowed by project destinations
func DeploymentValidator(config DeploymentValidatorConfig) func(http.Handler) http.Handler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate POST requests (instance creation)
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			// Read and restore request body for downstream handler
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				writeValidationError(w, http.StatusBadRequest, &ValidationError{
					Code:    "INVALID_REQUEST",
					Message: "Failed to read request body",
					Details: err.Error(),
				})
				return
			}
			// Restore the body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Parse deployment request
			var req DeploymentRequest
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				writeValidationError(w, http.StatusBadRequest, &ValidationError{
					Code:    "INVALID_REQUEST",
					Message: "Invalid request body",
					Details: err.Error(),
				})
				return
			}

			// If no projectId provided, skip project policy validation
			// The handler will use permission checks without project policy constraints
			if req.ProjectID == "" {
				logger.Debug("no projectId in request, skipping project policy validation",
					slog.String("name", req.Name),
					slog.String("namespace", req.Namespace),
				)
				next.ServeHTTP(w, r)
				return
			}

			// Get user context (should be set by Auth middleware)
			userCtx, ok := GetUserContext(r)
			if !ok {
				writeValidationError(w, http.StatusUnauthorized, &ValidationError{
					Code:    "UNAUTHORIZED",
					Message: "Authentication required",
					Details: "User context not found in request",
				})
				return
			}

			// Users with "*, *" permission can bypass project policy validation
			hasAdminPermission := false
			if config.PolicyEnforcer != nil {
				if canAccess, err := config.PolicyEnforcer.CanAccess(r.Context(), userCtx.UserID, "*", "*"); err == nil {
					hasAdminPermission = canAccess
				}
			}

			// Users with admin permissions bypass project policy validation
			if hasAdminPermission {
				logger.Debug("admin permission bypassing project policy validation",
					slog.String("user_id", userCtx.UserID),
					slog.String("project_id", req.ProjectID),
				)
				// Still fetch project to validate it exists
				if config.ProjectService != nil {
					project, err := config.ProjectService.GetProject(r.Context(), req.ProjectID)
					if err != nil {
						logger.Warn("project not found for global admin request",
							slog.String("project_id", req.ProjectID),
							slog.String("user_id", userCtx.UserID),
							slog.Any("error", err),
						)
						writeValidationError(w, http.StatusNotFound, &ValidationError{
							Code:    ErrCodeProjectNotFound,
							Message: fmt.Sprintf("Project '%s' not found", req.ProjectID),
							Details: "The specified project does not exist or has been deleted.",
						})
						return
					}
					// Store validated project in context for handler
					ctx := context.WithValue(r.Context(), validatedProjectContextKey{}, project)
					r = r.WithContext(ctx)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Validate deployment request against project policies

			validationErr := validateDeployment(r.Context(), config, userCtx.UserID, userCtx.Groups, &req, logger)
			if validationErr != nil {
				logger.Warn("deployment validation failed",
					slog.String("user_id", userCtx.UserID),
					slog.String("project_id", req.ProjectID),
					slog.String("code", validationErr.Code),
					slog.String("message", validationErr.Message),
				)
				writeValidationError(w, http.StatusForbidden, validationErr)
				return
			}

			// Store validated request and project in context for handler
			ctx := r.Context()
			ctx = context.WithValue(ctx, deploymentRequestContextKey{}, &req)

			// Fetch and store project if not already done
			if config.ProjectService != nil {
				project, _ := config.ProjectService.GetProject(r.Context(), req.ProjectID)
				if project != nil {
					ctx = context.WithValue(ctx, validatedProjectContextKey{}, project)
				}
			}

			logger.Info("deployment validation passed",
				slog.String("user_id", userCtx.UserID),
				slog.String("project_id", req.ProjectID),
				slog.String("namespace", req.Namespace),
				slog.String("rgd", req.RGDName),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// validateDeployment performs full deployment validation against project policies
func validateDeployment(
	ctx context.Context,
	config DeploymentValidatorConfig,
	userID string,
	groups []string,
	req *DeploymentRequest,
	logger *slog.Logger,
) *ValidationError {
	// 1. Validate Project exists
	if config.ProjectService == nil {
		logger.Error("project service not configured")
		return &ValidationError{
			Code:    "INTERNAL_ERROR",
			Message: "Project service not available",
			Details: "The project service is not configured.",
		}
	}

	project, err := config.ProjectService.GetProject(ctx, req.ProjectID)
	if err != nil {
		logger.Error("project not found",
			slog.String("project_id", req.ProjectID),
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
		return &ValidationError{
			Code:    ErrCodeProjectNotFound,
			Message: fmt.Sprintf("Project '%s' not found", req.ProjectID),
			Details: "The specified project does not exist or has been deleted.",
		}
	}

	// 2. Validate user has deploy permission for instances
	// NOTE: Uses CanAccessWithGroups to check both direct user permissions AND OIDC group permissions
	// This enables project-scoped roles (e.g., proj:proj-azuread-staging:admin) to grant deploy access
	//
	// Permission check uses project-scoped object format: "instances/{projectId}/*"
	// - Built-in roles (platform-admin, developer) have policy "instances/*" which matches via wildcard
	// - Project roles have policy "instances/{projectId}/*" which matches exactly
	// This single check handles both role types through Casbin's wildcard matching.
	if config.PolicyEnforcer != nil {
		object := fmt.Sprintf("instances/%s/*", req.ProjectID)
		action := "create"

		canDeploy, err := config.PolicyEnforcer.CanAccessWithGroups(ctx, userID, groups, object, action)
		if err != nil {
			logger.Error("permission check failed",
				slog.String("user_id", userID),
				slog.String("project_id", req.ProjectID),
				slog.String("object", object),
				slog.Any("error", err),
			)
			return &ValidationError{
				Code:    "INTERNAL_ERROR",
				Message: "Failed to check permissions",
				Details: err.Error(),
			}
		}

		if !canDeploy {
			logger.Warn("deploy permission denied",
				slog.String("user_id", userID),
				slog.String("project_id", req.ProjectID),
				slog.String("object", object),
				slog.String("action", action),
			)
			return &ValidationError{
				Code:    ErrCodePermissionDenied,
				Message: "You don't have permission to deploy instances",
				Details: fmt.Sprintf("Required permission: 'create' on instances for project '%s'", req.ProjectID),
			}
		}
	}

	// 3. Validate destination namespace against project's allowed destinations
	if len(project.Spec.Destinations) > 0 {
		dest := rbac.Destination{
			Namespace: req.Namespace,
		}

		if !rbac.ValidateDestinationAgainstAllowed(dest, project.Spec.Destinations) {
			logger.Warn("destination not allowed",
				slog.String("user_id", userID),
				slog.String("project_id", req.ProjectID),
				slog.String("namespace", req.Namespace),
				slog.Any("allowed_destinations", project.Spec.Destinations),
			)

			// Format allowed destinations for error message
			allowedDestStr := make([]string, len(project.Spec.Destinations))
			for i, d := range project.Spec.Destinations {
				allowedDestStr[i] = fmt.Sprintf("namespace:%s", d.Namespace)
			}

			return &ValidationError{
				Code:    ErrCodeDestinationNotAllowed,
				Message: fmt.Sprintf("Destination namespace '%s' is not allowed by project '%s'", req.Namespace, req.ProjectID),
				Details: "This destination does not match any allowed destination patterns.",
				Allowed: allowedDestStr,
			}
		}
	}

	// All validations passed
	return nil
}

// writeValidationError writes a JSON validation error response
func writeValidationError(w http.ResponseWriter, statusCode int, validationErr *ValidationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error": validationErr,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to write validation error response", "error", err)
	}
}
