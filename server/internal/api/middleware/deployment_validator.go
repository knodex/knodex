// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/sanitize"
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

// validationError represents a deployment policy validation failure
type validationError struct {
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
				response.BadRequest(w, "Failed to read request body", nil)
				return
			}
			// Restore the body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Parse deployment request
			var req DeploymentRequest
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				response.BadRequest(w, "Invalid request body", nil)
				return
			}

			// K8s-aligned routes (STORY-327): namespace comes from path parameter.
			// Path namespace takes precedence over body namespace.
			// Note: The handler also extracts path namespace independently because
			// this middleware operates on a local copy of req — the original body
			// bytes (forwarded via r.Body) are unchanged.
			if pathNS := r.PathValue("namespace"); pathNS != "" {
				req.Namespace = pathNS
			}

			// Security: projectId is required — without it, all project policy
			// validation (Casbin permission check, destination namespace whitelist)
			// would be skipped. CasbinAuthz middleware is also bypassed for instance
			// creation, so this is the sole authorization checkpoint.
			req.ProjectID = strings.TrimSpace(req.ProjectID)
			if req.ProjectID == "" {
				response.BadRequest(w, "projectId is required for instance deployment", nil)
				return
			}
			if !sanitize.IsValidDNS1123Label(req.ProjectID) {
				response.BadRequest(w, "projectId must be a valid DNS-1123 label", nil)
				return
			}

			// Get user context (should be set by Auth middleware)
			userCtx, ok := GetUserContext(r)
			if !ok {
				response.Unauthorized(w, "Authentication required")
				return
			}

			// Users with "*, *" permission can bypass project policy validation
			// Uses CanAccessWithGroups to also detect admin via OIDC group → role mapping
			hasAdminPermission := false
			if config.PolicyEnforcer != nil {
				if canAccess, err := config.PolicyEnforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, "*", "*"); err == nil {
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
						response.NotFound(w, "project", req.ProjectID)
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
			valErr := validateDeployment(r.Context(), config, userCtx.UserID, userCtx.Groups, &req, logger)
			if valErr != nil {
				logger.Warn("deployment validation failed",
					slog.String("user_id", userCtx.UserID),
					slog.String("project_id", req.ProjectID),
					slog.String("code", valErr.Code),
					slog.String("message", valErr.Message),
				)
				details := map[string]string{"details": valErr.Details}
				if len(valErr.Allowed) > 0 {
					details["allowed"] = strings.Join(valErr.Allowed, ", ")
				}
				response.WriteError(w, http.StatusForbidden, response.ErrorCode(valErr.Code), valErr.Message, details)
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
) *validationError {
	// 1. Validate Project exists
	if config.ProjectService == nil {
		logger.Error("project service not configured")
		return &validationError{
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
		return &validationError{
			Code:    ErrCodeProjectNotFound,
			Message: fmt.Sprintf("Project '%s' not found", req.ProjectID),
			Details: "The specified project does not exist or has been deleted.",
		}
	}

	// 2. Validate user has deploy permission for instances
	// NOTE: Uses CanAccessWithGroups to check both direct user permissions AND OIDC group permissions
	// This enables project-scoped roles (e.g., proj:proj-azuread-staging:admin) to grant deploy access
	//
	// Permission check uses namespace-scoped object format.
	// For namespace-scoped instances: "instances/{projectId}/{namespace}/*"
	// For cluster-scoped instances: "instances/{projectId}/*" (no namespace dimension)
	// This enables namespace-scoped Casbin policies (roles[].destinations) to work correctly.
	if config.PolicyEnforcer != nil {
		var object string
		if req.Namespace != "" {
			object = fmt.Sprintf("instances/%s/%s/*", req.ProjectID, req.Namespace)
		} else {
			object = fmt.Sprintf("instances/%s/*", req.ProjectID)
		}
		action := "create"

		canDeploy, err := config.PolicyEnforcer.CanAccessWithGroups(ctx, userID, groups, object, action)
		if err != nil {
			logger.Error("permission check failed",
				slog.String("user_id", userID),
				slog.String("project_id", req.ProjectID),
				slog.String("object", object),
				slog.Any("error", err),
			)
			return &validationError{
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
			return &validationError{
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

			return &validationError{
				Code:    ErrCodeDestinationNotAllowed,
				Message: fmt.Sprintf("Destination namespace '%s' is not allowed by project '%s'", req.Namespace, req.ProjectID),
				Details: "This destination does not match any allowed destination patterns.",
				Allowed: allowedDestStr,
			}
		}
		// Note: role-scoped destination enforcement is handled by Casbin namespace-scoped
		// policies (roles[].destinations). No second authorization layer needed here.
	}

	// All validations passed
	return nil
}
