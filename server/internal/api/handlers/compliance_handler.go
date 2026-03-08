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

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/collection"
	utilrand "github.com/knodex/knodex/server/internal/util/rand"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// ComplianceHandler handles compliance-related HTTP requests.
// This handler provides REST API endpoints for OPA Gatekeeper compliance data
// including ConstraintTemplates, Constraints, and Violations.
type ComplianceHandler struct {
	service        services.ComplianceService
	historyService services.ViolationHistoryService
	licenseService services.LicenseService
	policyEnforcer rbac.PolicyEnforcer
	projectService rbac.ProjectServiceInterface
	recorder       audit.Recorder
	redisClient    *redis.Client
	logger         *slog.Logger
}

// NewComplianceHandler creates a new ComplianceHandler.
// service: The ComplianceService interface (returns enterprise-required errors in OSS, actual data in EE)
// policyEnforcer: Casbin policy enforcer for permission checks
// logger: Structured logger for request logging
func NewComplianceHandler(service services.ComplianceService, policyEnforcer rbac.PolicyEnforcer, logger *slog.Logger) *ComplianceHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ComplianceHandler{
		service:        service,
		policyEnforcer: policyEnforcer,
		logger:         logger.With("handler", "compliance"),
	}
}

// SetLicenseService sets the license service for enterprise license checking.
func (h *ComplianceHandler) SetLicenseService(ls services.LicenseService) {
	h.licenseService = ls
}

// SetViolationHistoryService sets the violation history service.
func (h *ComplianceHandler) SetViolationHistoryService(svc services.ViolationHistoryService) {
	h.historyService = svc
}

// SetAuditRecorder sets the audit recorder for compliance event tracking.
func (h *ComplianceHandler) SetAuditRecorder(r audit.Recorder) {
	h.recorder = r
}

// SetProjectService sets the project service for project-scoped violation filtering.
func (h *ComplianceHandler) SetProjectService(ps rbac.ProjectServiceInterface) {
	h.projectService = ps
}

// SetRedisClient sets the Redis client for enforcement confirmation tokens.
func (h *ComplianceHandler) SetRedisClient(client *redis.Client) {
	h.redisClient = client
}

// ListResponse is a generic paginated list response.
type ListResponse[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

// checkEnabled verifies that the compliance feature is enabled and licensed for read operations.
// Returns false and writes an appropriate error:
//   - 402 Payment Required: OSS build (nil service) or no valid license
//   - 503 Service Unavailable: EE build but Gatekeeper unavailable
func (h *ComplianceHandler) checkEnabled(w http.ResponseWriter, r *http.Request) bool {
	return h.checkLicenseAccess(w, r, false)
}

// checkEnabledForWrite verifies that the compliance feature is enabled and licensed for write operations.
// Write operations are blocked when the license is expired past the grace period (read-only mode).
func (h *ComplianceHandler) checkEnabledForWrite(w http.ResponseWriter, r *http.Request) bool {
	return h.checkLicenseAccess(w, r, true)
}

// checkLicenseAccess is the shared license validation logic for compliance endpoints.
// isWrite determines whether write-specific restrictions (read-only mode) apply.
func (h *ComplianceHandler) checkLicenseAccess(w http.ResponseWriter, _ *http.Request, isWrite bool) bool {
	featureDetail := map[string]string{"feature": services.FeatureCompliance}

	// OSS build: service is nil, this is a licensing issue
	if h.service == nil {
		response.WriteError(w, http.StatusPaymentRequired, "LICENSE_REQUIRED",
			"This feature requires a valid enterprise license", featureDetail)
		return false
	}

	// EE build: check license
	if h.licenseService != nil {
		if !h.checkComplianceLicense(w, isWrite, featureDetail) {
			return false
		}
	}

	// EE build: service exists but Gatekeeper may be unavailable
	if !h.service.IsEnabled() {
		status := h.service.GetStatus()
		message := "OPA Gatekeeper is not available. Please verify Gatekeeper is installed in your cluster."
		if status != nil && status.Message != "" {
			message = status.Message
		}
		response.WriteError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", message, nil)
		return false
	}

	return true
}

// checkComplianceLicense validates the license for the compliance feature.
// Returns false and writes an error if the license check fails.
func (h *ComplianceHandler) checkComplianceLicense(w http.ResponseWriter, isWrite bool, featureDetail map[string]string) bool {
	// Fully licensed (valid or in grace period)
	if h.licenseService.IsFeatureEnabled(services.FeatureCompliance) {
		if h.licenseService.IsGracePeriod() {
			w.Header().Set("X-License-Warning", "expired")
		}
		return true
	}

	// Read-only mode: expired past grace but feature was in the license
	if h.licenseService.IsReadOnly() && h.licenseService.HasFeature(services.FeatureCompliance) {
		if isWrite {
			detail := map[string]string{"feature": services.FeatureCompliance, "reason": "license_expired"}
			response.WriteError(w, http.StatusPaymentRequired, "LICENSE_REQUIRED",
				"License expired - write operations require a valid license", detail)
			return false
		}
		return true
	}

	// Not licensed or feature not in license
	if !h.licenseService.IsLicensed() {
		response.WriteError(w, http.StatusPaymentRequired, "LICENSE_REQUIRED",
			"This feature requires a valid enterprise license", featureDetail)
	} else {
		response.WriteError(w, http.StatusPaymentRequired, "LICENSE_REQUIRED",
			"Compliance feature is not included in your license", featureDetail)
	}
	return false
}

// checkPermission verifies that the user has compliance:get permission.
// Returns false and writes 403 Forbidden if not permitted.
func (h *ComplianceHandler) checkPermission(w http.ResponseWriter, r *http.Request) bool {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.Unauthorized(w, "User context not found")
		return false
	}

	if h.policyEnforcer == nil {
		// Fail closed: no enforcer means no access
		h.logger.Warn("policy enforcer unavailable, denying compliance access",
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	// Check compliance:get permission using Casbin
	// Object is "compliance/*" for all compliance resources
	allowed, err := h.policyEnforcer.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"compliance/*",
		"get",
	)
	if err != nil {
		h.logger.Error("failed to check compliance permission",
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to check authorization")
		return false
	}

	if !allowed {
		h.logger.Warn("compliance access denied",
			"userId", userCtx.UserID,
			"groups", userCtx.Groups,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	return true
}

// GetStatus handles GET /api/v1/compliance/status
// Returns the compliance feature availability status.
// This endpoint always returns 200 OK with status information,
// allowing the frontend to understand why compliance is unavailable.
func (h *ComplianceHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Check permission (but don't require feature to be enabled)
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)
	h.logger.Info("getting compliance status",
		"requestId", requestID,
		"userId", userCtx.UserID,
	)

	// Build status response
	status := services.ComplianceStatus{}

	if h.service == nil {
		// OSS build: no license
		status.Available = false
		status.Enterprise = false
		status.Message = "Compliance features require an enterprise license"
	} else {
		// EE build: check Gatekeeper availability
		svcStatus := h.service.GetStatus()
		if svcStatus != nil {
			status.Available = svcStatus.Available
			status.Enterprise = svcStatus.Enterprise
			status.Message = svcStatus.Message
			status.Gatekeeper = svcStatus.Gatekeeper
		} else {
			// Fallback if GetStatus returns nil
			status.Available = h.service.IsEnabled()
			status.Enterprise = true
			if status.Available {
				status.Message = "Compliance features are available"
				status.Gatekeeper = "installed"
			} else {
				status.Message = "OPA Gatekeeper is not available"
				status.Gatekeeper = "not_installed"
			}
		}
	}

	h.logger.Info("compliance status retrieved",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"available", status.Available,
		"enterprise", status.Enterprise,
		"gatekeeper", status.Gatekeeper,
	)

	response.WriteJSON(w, http.StatusOK, status)
}

// parsePagination extracts page and pageSize from query parameters.
// Returns defaults if not specified: page=1, pageSize=20
func parsePagination(r *http.Request) (page int, pageSize int) {
	page = 1
	pageSize = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	return page, pageSize
}

// paginateSlice applies pagination to a slice.
// Returns the paginated subset of items.
func paginateSlice[T any](items []T, page, pageSize int) []T {
	if len(items) == 0 {
		return items
	}

	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}

	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	return items[start:end]
}

// GetSummary handles GET /api/v1/compliance/summary
// Returns aggregate compliance statistics.
func (h *ComplianceHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature
	if !h.checkEnabled(w, r) {
		return
	}

	// 2. Check permission
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)
	h.logger.Info("getting compliance summary",
		"requestId", requestID,
		"userId", userCtx.UserID,
	)

	// 3. Get summary from service
	summary, err := h.service.GetSummary(r.Context())
	if err != nil {
		h.logger.Error("failed to get compliance summary",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to get compliance summary")
		return
	}

	h.logger.Info("compliance summary retrieved",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"totalTemplates", summary.TotalTemplates,
		"totalConstraints", summary.TotalConstraints,
		"totalViolations", summary.TotalViolations,
	)

	response.WriteJSON(w, http.StatusOK, summary)
}

// ListTemplates handles GET /api/v1/compliance/templates
// Returns all ConstraintTemplates with pagination.
func (h *ComplianceHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature
	if !h.checkEnabled(w, r) {
		return
	}

	// 2. Check permission
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Parse pagination
	page, pageSize := parsePagination(r)

	h.logger.Info("listing constraint templates",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"page", page,
		"pageSize", pageSize,
	)

	// 4. Get templates from service
	templates, err := h.service.ListConstraintTemplates(r.Context())
	if err != nil {
		h.logger.Error("failed to list constraint templates",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to list constraint templates")
		return
	}

	// 5. Apply pagination
	total := len(templates)
	paginated := paginateSlice(templates, page, pageSize)

	h.logger.Info("constraint templates listed",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"total", total,
		"returned", len(paginated),
	)

	// 6. Return response
	resp := ListResponse[services.ConstraintTemplate]{
		Items:    paginated,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// GetTemplate handles GET /api/v1/compliance/templates/{name}
// Returns a specific ConstraintTemplate by name.
func (h *ComplianceHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature
	if !h.checkEnabled(w, r) {
		return
	}

	// 2. Check permission
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Get template name from path
	name := r.PathValue("name")
	if name == "" {
		response.BadRequest(w, "Template name is required", nil)
		return
	}

	h.logger.Info("getting constraint template",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"templateName", name,
	)

	// 4. Get template from service
	template, err := h.service.GetConstraintTemplate(r.Context(), name)
	if err != nil {
		if isComplianceNotFoundErr(err) {
			h.logger.Info("constraint template not found",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"templateName", name,
			)
			response.NotFound(w, "ConstraintTemplate", name)
			return
		}
		h.logger.Error("failed to get constraint template",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"templateName", name,
			"error", err,
		)
		response.InternalError(w, "Failed to get constraint template")
		return
	}

	h.logger.Info("constraint template retrieved",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"templateName", name,
	)

	response.WriteJSON(w, http.StatusOK, template)
}

// ListConstraints handles GET /api/v1/compliance/constraints
// Returns all Constraints with pagination and optional filtering.
// Query parameters:
//   - kind: Filter by constraint kind (e.g., K8sRequiredLabels)
//   - enforcement: Filter by enforcement action (deny, warn, dryrun)
//   - page: Page number (default: 1)
//   - pageSize: Results per page (default: 20, max: 100)
func (h *ComplianceHandler) ListConstraints(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature
	if !h.checkEnabled(w, r) {
		return
	}

	// 2. Check permission
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Parse query parameters
	page, pageSize := parsePagination(r)
	kindFilter := r.URL.Query().Get("kind")
	enforcementFilter := r.URL.Query().Get("enforcement")

	h.logger.Info("listing constraints",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"page", page,
		"pageSize", pageSize,
		"kindFilter", kindFilter,
		"enforcementFilter", enforcementFilter,
	)

	// 4. Get constraints from service
	constraints, err := h.service.ListConstraints(r.Context())
	if err != nil {
		h.logger.Error("failed to list constraints",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to list constraints")
		return
	}

	// 5. Apply filters
	filtered := constraints
	if kindFilter != "" {
		filtered = filterConstraintsByKind(filtered, kindFilter)
	}
	if enforcementFilter != "" {
		filtered = filterConstraintsByEnforcement(filtered, enforcementFilter)
	}

	// 6. Apply pagination
	total := len(filtered)
	paginated := paginateSlice(filtered, page, pageSize)

	h.logger.Info("constraints listed",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"total", total,
		"returned", len(paginated),
	)

	// 7. Return response
	resp := ListResponse[services.Constraint]{
		Items:    paginated,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// GetConstraint handles GET /api/v1/compliance/constraints/{kind}/{name}
// Returns a specific Constraint by kind and name.
func (h *ComplianceHandler) GetConstraint(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature
	if !h.checkEnabled(w, r) {
		return
	}

	// 2. Check permission
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Get kind and name from path
	kind := r.PathValue("kind")
	name := r.PathValue("name")
	if kind == "" || name == "" {
		response.BadRequest(w, "Constraint kind and name are required", nil)
		return
	}

	h.logger.Info("getting constraint",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"constraintKind", kind,
		"constraintName", name,
	)

	// 4. Get constraint from service
	constraint, err := h.service.GetConstraint(r.Context(), kind, name)
	if err != nil {
		if isComplianceNotFoundErr(err) {
			h.logger.Info("constraint not found",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"constraintKind", kind,
				"constraintName", name,
			)
			response.NotFound(w, "Constraint", kind+"/"+name)
			return
		}
		h.logger.Error("failed to get constraint",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"constraintKind", kind,
			"constraintName", name,
			"error", err,
		)
		response.InternalError(w, "Failed to get constraint")
		return
	}

	h.logger.Info("constraint retrieved",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"constraintKind", kind,
		"constraintName", name,
	)

	response.WriteJSON(w, http.StatusOK, constraint)
}

// ListViolations handles GET /api/v1/compliance/violations
// Returns violations with pagination and optional filtering.
// Query parameters:
//   - constraint: Filter by constraint (format: {kind}/{name})
//   - resource: Filter by resource (format: {kind}/{namespace}/{name})
//   - page: Page number (default: 1)
//   - pageSize: Results per page (default: 20, max: 100)
func (h *ComplianceHandler) ListViolations(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature
	if !h.checkEnabled(w, r) {
		return
	}

	// 2. Check permission
	if !h.checkPermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Parse query parameters
	page, pageSize := parsePagination(r)
	constraintFilter := r.URL.Query().Get("constraint")
	resourceFilter := r.URL.Query().Get("resource")

	h.logger.Info("listing violations",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"page", page,
		"pageSize", pageSize,
		"constraintFilter", constraintFilter,
		"resourceFilter", resourceFilter,
	)

	var violations []services.Violation
	var err error

	// 4. Get violations based on filters
	if constraintFilter != "" {
		// Filter by constraint: {kind}/{name}
		parts := strings.SplitN(constraintFilter, "/", 2)
		if len(parts) != 2 {
			response.BadRequest(w, "Invalid constraint filter format. Expected: {kind}/{name}", nil)
			return
		}
		violations, err = h.service.GetViolationsByConstraint(r.Context(), parts[0], parts[1])
	} else if resourceFilter != "" {
		// Filter by resource: {kind}/{namespace}/{name}
		parts := strings.SplitN(resourceFilter, "/", 3)
		if len(parts) != 3 {
			response.BadRequest(w, "Invalid resource filter format. Expected: {kind}/{namespace}/{name}", nil)
			return
		}
		violations, err = h.service.GetViolationsByResource(r.Context(), parts[0], parts[1], parts[2])
	} else {
		// No filter, get all violations
		violations, err = h.service.ListViolations(r.Context())
	}

	if err != nil {
		h.logger.Error("failed to list violations",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to list violations")
		return
	}

	// 5. Project-scoped filtering (AC-4: users only see violations from their project namespaces)
	violations = h.filterViolationsByAccess(r, userCtx, violations, requestID)

	// 6. Apply pagination
	total := len(violations)
	paginated := paginateSlice(violations, page, pageSize)

	h.logger.Info("violations listed",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"total", total,
		"returned", len(paginated),
	)

	// 7. Return response
	resp := ListResponse[services.Violation]{
		Items:    paginated,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// filterViolationsByAccess filters violations to only those from namespaces the user can access.
// Global admins (who have `compliance/*` access) see all violations.
// Project-scoped users only see violations from namespaces in their accessible projects.
func (h *ComplianceHandler) filterViolationsByAccess(r *http.Request, userCtx *middleware.UserContext, violations []services.Violation, requestID string) []services.Violation {
	if h.policyEnforcer == nil || h.projectService == nil || userCtx == nil {
		return violations
	}

	// Check if user has global compliance access - if so, no filtering needed
	globalAccess, err := h.policyEnforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, "compliance/*", "get")
	if err != nil {
		h.logger.Error("failed to check global compliance access, defaulting to filtered",
			"requestId", requestID, "userId", userCtx.UserID, "error", err)
		// Fail closed: filter if we can't determine access
	}
	if globalAccess {
		return violations
	}

	// Get user's accessible projects
	accessibleProjects, err := h.policyEnforcer.GetAccessibleProjects(r.Context(), userCtx.UserID, userCtx.Groups)
	if err != nil {
		h.logger.Error("failed to get accessible projects for violation filtering",
			"requestId", requestID, "userId", userCtx.UserID, "error", err)
		return []services.Violation{} // Fail closed
	}

	// Collect namespaces from accessible projects
	nsSet := make(map[string]bool)
	for _, projName := range accessibleProjects {
		proj, err := h.projectService.GetProject(r.Context(), projName)
		if err != nil {
			continue
		}
		for _, dest := range proj.Spec.Destinations {
			if dest.Namespace != "" {
				nsSet[dest.Namespace] = true
			}
		}
	}

	// Filter violations to accessible namespaces
	filtered := collection.Filter(violations, func(v services.Violation) bool {
		return nsSet[v.Resource.Namespace]
	})

	h.logger.Debug("filtered violations by project access",
		"requestId", requestID, "userId", userCtx.UserID,
		"total", len(violations), "filtered", len(filtered),
		"accessibleNamespaces", len(nsSet))

	return filtered
}

// filterConstraintsByKind filters constraints by kind.
func filterConstraintsByKind(constraints []services.Constraint, kind string) []services.Constraint {
	return collection.Filter(constraints, func(c services.Constraint) bool {
		return c.Kind == kind
	})
}

// filterConstraintsByEnforcement filters constraints by enforcement action.
func filterConstraintsByEnforcement(constraints []services.Constraint, enforcement string) []services.Constraint {
	return collection.Filter(constraints, func(c services.Constraint) bool {
		return strings.EqualFold(c.EnforcementAction, enforcement)
	})
}

// isComplianceNotFoundErr checks if an error indicates a resource was not found.
func isComplianceNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "notfound")
}

// UpdateConstraintEnforcementRequest is the request body for updating enforcement action.
type UpdateConstraintEnforcementRequest struct {
	EnforcementAction string `json:"enforcementAction"`
	ConfirmationToken string `json:"confirmationToken,omitempty"`
}

// EnforcementEscalationResponse is returned when enforcement escalation requires confirmation.
type EnforcementEscalationResponse struct {
	Message           string `json:"message"`
	ConfirmationToken string `json:"confirmationToken"`
	ExpiresIn         int    `json:"expiresIn"`
}

const (
	enforcementConfirmationTTL    = 5 * time.Minute
	enforcementConfirmationPrefix = "enforcement-confirm:"
	confirmationTokenLength       = 32 // 32 bytes = 64 hex chars
)

// isEscalation returns true if changing enforcement from oldAction to newAction
// is an escalation (i.e., escalating to "deny" from a non-deny action).
// Uses case-insensitive comparison since Kubernetes may return mixed-case enforcement actions.
func isEscalation(oldAction, newAction string) bool {
	return strings.EqualFold(newAction, "deny") && !strings.EqualFold(oldAction, "deny")
}

// generateConfirmationToken creates a cryptographically random confirmation token.
func generateConfirmationToken() (string, error) {
	return utilrand.GenerateRandomHex(confirmationTokenLength), nil
}

// storeConfirmationToken stores a confirmation token in Redis with TTL.
// The value encodes kind/name/targetAction so the token is bound to a specific operation.
func (h *ComplianceHandler) storeConfirmationToken(ctx context.Context, token, kind, name, targetAction string) error {
	key := enforcementConfirmationPrefix + token
	value := kind + "/" + name + "/" + targetAction
	return h.redisClient.Set(ctx, key, value, enforcementConfirmationTTL).Err()
}

// validateAndConsumeConfirmationToken validates a confirmation token against its bound operation
// and consumes it only if the binding matches. Uses GET to check binding first, then DEL to consume.
// This prevents a token from being destroyed when submitted against the wrong constraint.
// Returns (valid, error). If the token is not found (expired or never existed), valid=false.
func (h *ComplianceHandler) validateAndConsumeConfirmationToken(ctx context.Context, token, kind, name, targetAction string) (bool, error) {
	key := enforcementConfirmationPrefix + token

	// Step 1: Read the token value without deleting
	value, err := h.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // expired or not found
	}
	if err != nil {
		return false, fmt.Errorf("failed to validate confirmation token: %w", err)
	}

	// Step 2: Verify the token is bound to the correct operation BEFORE consuming
	expected := kind + "/" + name + "/" + targetAction
	if value != expected {
		return false, nil // token was for a different operation — do NOT consume it
	}

	// Step 3: Consume the token (delete from Redis) — single-use
	if err := h.redisClient.Del(ctx, key).Err(); err != nil {
		return false, fmt.Errorf("failed to consume confirmation token: %w", err)
	}

	return true, nil
}

// CreateConstraintRequest is the request body for creating a new constraint.
type CreateConstraintRequest struct {
	Name              string            `json:"name"`
	TemplateName      string            `json:"templateName"`
	EnforcementAction string            `json:"enforcementAction,omitempty"`
	Match             *ConstraintMatch  `json:"match,omitempty"`
	Parameters        map[string]any    `json:"parameters,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// constraintProtectedNamespaces is the blocklist of Kubernetes system namespaces
// that cannot be targeted by compliance constraints.
var constraintProtectedNamespaces = map[string]bool{
	"kube-system":       true,
	"kube-public":       true,
	"kube-node-lease":   true,
	"gatekeeper-system": true,
}

// ConstraintMatch defines which resources a constraint applies to.
type ConstraintMatch struct {
	Kinds      []MatchKind `json:"kinds,omitempty"`
	Namespaces []string    `json:"namespaces,omitempty"`
	Scope      string      `json:"scope,omitempty"`
}

// MatchKind specifies a group of Kubernetes resource kinds to match.
type MatchKind struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string `json:"kinds"`
}

// validateConstraintMatch validates match fields to prevent overly-broad constraints
// targeting system namespaces or using dangerous wildcards.
// IMPORTANT: Must be called in any handler that accepts ConstraintMatch input.
func validateConstraintMatch(match *ConstraintMatch) error {
	if match == nil {
		return nil
	}

	for _, ns := range match.Namespaces {
		if ns == "*" {
			return fmt.Errorf("wildcard namespace '*' is not allowed in match.namespaces")
		}
		if constraintProtectedNamespaces[ns] {
			return fmt.Errorf("namespace %q is protected and cannot be targeted by constraints", ns)
		}
	}

	if match.Scope == "*" {
		return fmt.Errorf("wildcard scope '*' is not allowed in match.scope")
	}

	for _, k := range match.Kinds {
		if isWildcardOnly(k.APIGroups) && isWildcardOnly(k.Kinds) {
			return fmt.Errorf("overly-broad kind selector with wildcard apiGroups and kinds is not allowed")
		}
	}

	return nil
}

// isWildcardOnly returns true if the slice is non-empty and contains only "*".
func isWildcardOnly(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if v != "*" {
			return false
		}
	}
	return true
}

// checkCreatePermission verifies that the user has compliance:create permission.
// Returns false and writes 403 Forbidden if not permitted.
func (h *ComplianceHandler) checkCreatePermission(w http.ResponseWriter, r *http.Request) bool {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.Unauthorized(w, "User context not found")
		return false
	}

	if h.policyEnforcer == nil {
		// Fail closed: no enforcer means no access
		h.logger.Warn("policy enforcer unavailable, denying compliance create",
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	// Check compliance:create permission using Casbin
	allowed, err := h.policyEnforcer.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"compliance/*",
		"create",
	)
	if err != nil {
		h.logger.Error("failed to check compliance create permission",
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to check authorization")
		return false
	}

	if !allowed {
		h.logger.Warn("compliance create denied",
			"userId", userCtx.UserID,
			"groups", userCtx.Groups,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	return true
}

// checkUpdatePermission verifies that the user has compliance:update permission.
// Returns false and writes 403 Forbidden if not permitted.
func (h *ComplianceHandler) checkUpdatePermission(w http.ResponseWriter, r *http.Request) bool {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.Unauthorized(w, "User context not found")
		return false
	}

	if h.policyEnforcer == nil {
		// Fail closed: no enforcer means no access
		h.logger.Warn("policy enforcer unavailable, denying compliance update",
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	// Check compliance:update permission using Casbin
	allowed, err := h.policyEnforcer.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"compliance/*",
		"update",
	)
	if err != nil {
		h.logger.Error("failed to check compliance update permission",
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to check authorization")
		return false
	}

	if !allowed {
		h.logger.Warn("compliance update denied",
			"userId", userCtx.UserID,
			"groups", userCtx.Groups,
		)
		response.Forbidden(w, "permission denied")
		return false
	}

	return true
}

// UpdateConstraintEnforcement handles PATCH /api/v1/compliance/constraints/{kind}/{name}/enforcement
// Updates a constraint's enforcement action (deny, warn, dryrun).
func (h *ComplianceHandler) UpdateConstraintEnforcement(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature (write operation - blocked in read-only mode)
	if !h.checkEnabledForWrite(w, r) {
		return
	}

	// 2. Check update permission (requires compliance:update, not just get)
	if !h.checkUpdatePermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Get kind and name from path
	kind := r.PathValue("kind")
	name := r.PathValue("name")
	if kind == "" || name == "" {
		response.BadRequest(w, "Constraint kind and name are required", nil)
		return
	}

	// 4. Parse request body
	var req UpdateConstraintEnforcementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}

	// 5. Validate enforcement action
	validActions := map[string]bool{"deny": true, "warn": true, "dryrun": true}
	if !validActions[req.EnforcementAction] {
		response.BadRequest(w, "Invalid enforcement action. Must be one of: deny, warn, dryrun", nil)
		return
	}

	// 6. Get current constraint to verify it exists and log the change
	currentConstraint, err := h.service.GetConstraint(r.Context(), kind, name)
	if err != nil {
		if isComplianceNotFoundErr(err) {
			h.logger.Info("constraint not found for enforcement update",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"constraintKind", kind,
				"constraintName", name,
			)
			response.NotFound(w, "Constraint", kind+"/"+name)
			return
		}
		h.logger.Error("failed to get constraint for enforcement update",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"constraintKind", kind,
			"constraintName", name,
			"error", err,
		)
		response.InternalError(w, "Failed to get constraint")
		return
	}

	oldAction := currentConstraint.EnforcementAction

	// 7. Skip update if no change
	if oldAction == req.EnforcementAction {
		h.logger.Info("constraint enforcement action unchanged",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"constraintKind", kind,
			"constraintName", name,
			"action", req.EnforcementAction,
		)
		response.WriteJSON(w, http.StatusOK, currentConstraint)
		return
	}

	// 7.5 Escalation confirmation flow (INJ-VULN-04)
	// Escalation to "deny" requires a two-phase confirmation to prevent mass-escalation attacks
	if isEscalation(oldAction, req.EnforcementAction) {
		if h.redisClient == nil {
			// Fail closed: cannot confirm without Redis
			response.ServiceUnavailable(w, "Confirmation service unavailable. Please try again.")
			return
		}

		if req.ConfirmationToken == "" {
			// Phase 1: Generate and return a confirmation token
			token, err := generateConfirmationToken()
			if err != nil {
				h.logger.Error("failed to generate confirmation token",
					"requestId", requestID,
					"error", err,
				)
				response.InternalError(w, "Failed to generate confirmation token")
				return
			}

			if err := h.storeConfirmationToken(r.Context(), token, kind, name, req.EnforcementAction); err != nil {
				h.logger.Error("failed to store confirmation token",
					"requestId", requestID,
					"error", err,
				)
				response.ServiceUnavailable(w, "Confirmation service unavailable. Please try again.")
				return
			}

			response.WriteJSON(w, http.StatusConflict, EnforcementEscalationResponse{
				Message:           "Changing to deny mode will block resource creation. Confirm by including the token.",
				ConfirmationToken: token,
				ExpiresIn:         300,
			})
			return
		}

		// Phase 2: Validate and consume the confirmation token
		valid, err := h.validateAndConsumeConfirmationToken(r.Context(), req.ConfirmationToken, kind, name, req.EnforcementAction)
		if err != nil {
			h.logger.Error("failed to validate confirmation token",
				"requestId", requestID,
				"error", err,
			)
			response.ServiceUnavailable(w, "Confirmation service unavailable. Please try again.")
			return
		}

		if !valid {
			// Token expired, already used, or for a different operation → 410 Gone
			response.WriteError(w, http.StatusGone, "GONE", "Confirmation token has expired. Please restart the confirmation flow.", nil)
			return
		}
	}

	h.logger.Info("updating constraint enforcement action",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"constraintKind", kind,
		"constraintName", name,
		"oldAction", oldAction,
		"newAction", req.EnforcementAction,
	)

	// 8. Update the constraint
	updatedConstraint, err := h.service.UpdateConstraintEnforcement(r.Context(), kind, name, req.EnforcementAction)
	if err != nil {
		if isComplianceNotFoundErr(err) {
			response.NotFound(w, "Constraint", kind+"/"+name)
			return
		}
		if isKubernetesForbiddenErr(err) {
			h.logger.Error("kubernetes RBAC forbidden for constraint update",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"constraintKind", kind,
				"constraintName", name,
				"error", err,
			)
			response.Forbidden(w, "insufficient permissions to manage constraint")
			return
		}
		h.logger.Error("failed to update constraint enforcement action",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"constraintKind", kind,
			"constraintName", name,
			"oldAction", oldAction,
			"newAction", req.EnforcementAction,
			"error", err,
		)
		response.InternalError(w, "Failed to update constraint enforcement action")
		return
	}

	// 9. Log audit entry
	h.logger.Info("constraint enforcement action updated",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"constraintKind", kind,
		"constraintName", name,
		"oldAction", oldAction,
		"newAction", req.EnforcementAction,
		"violationCount", updatedConstraint.ViolationCount,
	)

	auditDetails := map[string]any{
		"constraintName":            name,
		"kind":                      kind,
		"templateName":              currentConstraint.TemplateName,
		"enforcementAction":         req.EnforcementAction,
		"previousEnforcementAction": oldAction,
	}
	if isEscalation(oldAction, req.EnforcementAction) {
		auditDetails["confirmed"] = true
	}

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "enforcement_change",
		Resource:  "compliance",
		Name:      name,
		RequestID: requestID,
		Result:    "success",
		Details:   auditDetails,
	})

	// 10. Return updated constraint
	response.WriteJSON(w, http.StatusOK, updatedConstraint)
}

// CreateConstraint handles POST /api/v1/compliance/constraints
// Creates a new constraint from a ConstraintTemplate.
func (h *ComplianceHandler) CreateConstraint(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// 1. Check enterprise feature (write operation - blocked in read-only mode)
	if !h.checkEnabledForWrite(w, r) {
		return
	}

	// 2. Check create permission (requires compliance:create)
	if !h.checkCreatePermission(w, r) {
		return
	}

	userCtx, _ := middleware.GetUserContext(r)

	// 3. Parse request body
	var req CreateConstraintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}

	// 4. Validate required fields
	if req.Name == "" {
		response.BadRequest(w, "Constraint name is required", nil)
		return
	}
	if req.TemplateName == "" {
		response.BadRequest(w, "Template name is required", nil)
		return
	}

	// 5. Validate enforcement action if provided
	if req.EnforcementAction != "" {
		validActions := map[string]bool{"deny": true, "warn": true, "dryrun": true}
		if !validActions[req.EnforcementAction] {
			response.BadRequest(w, "Invalid enforcement action. Must be one of: deny, warn, dryrun", nil)
			return
		}
	}

	// 5a. Validate match fields to prevent overly-broad constraints
	if err := validateConstraintMatch(req.Match); err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}

	h.logger.Info("creating constraint",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"constraintName", req.Name,
		"templateName", req.TemplateName,
		"enforcementAction", req.EnforcementAction,
	)

	// 6. Convert handler request to service request
	svcReq := services.CreateConstraintRequest{
		Name:              req.Name,
		TemplateName:      req.TemplateName,
		EnforcementAction: req.EnforcementAction,
		Parameters:        req.Parameters,
		Labels:            req.Labels,
	}

	// Convert match rules if provided
	if req.Match != nil {
		svcReq.Match = &services.ConstraintMatch{
			Namespaces: req.Match.Namespaces,
			Scope:      req.Match.Scope,
		}
		for _, k := range req.Match.Kinds {
			svcReq.Match.Kinds = append(svcReq.Match.Kinds, services.MatchKind{
				APIGroups: k.APIGroups,
				Kinds:     k.Kinds,
			})
		}
	}

	// 7. Create the constraint
	constraint, err := h.service.CreateConstraint(r.Context(), svcReq)
	if err != nil {
		if isComplianceNotFoundErr(err) {
			h.logger.Info("constraint template not found",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"templateName", req.TemplateName,
			)
			response.NotFound(w, "ConstraintTemplate", req.TemplateName)
			return
		}

		// Check for already exists error
		if isAlreadyExistsErr(err) {
			h.logger.Info("constraint already exists",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"constraintName", req.Name,
			)
			response.WriteError(w, http.StatusConflict, "ALREADY_EXISTS",
				"A constraint with this name already exists", nil)
			return
		}

		if isKubernetesForbiddenErr(err) {
			h.logger.Error("kubernetes RBAC forbidden for constraint create",
				"requestId", requestID,
				"userId", userCtx.UserID,
				"constraintName", req.Name,
				"error", err,
			)
			response.Forbidden(w, "insufficient permissions to manage constraint")
			return
		}

		h.logger.Error("failed to create constraint",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"constraintName", req.Name,
			"templateName", req.TemplateName,
			"error", err,
		)
		response.InternalError(w, "Failed to create constraint")
		return
	}

	// 8. Log audit entry for constraint creation
	h.logger.Info("constraint created",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"constraintName", constraint.Name,
		"constraintKind", constraint.Kind,
		"templateName", constraint.TemplateName,
		"enforcementAction", constraint.EnforcementAction,
	)

	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "constraint_create",
		Resource:  "compliance",
		Name:      constraint.Name,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"constraintName":    constraint.Name,
			"kind":              constraint.Kind,
			"templateName":      constraint.TemplateName,
			"enforcementAction": constraint.EnforcementAction,
		},
	})

	// 9. Return created constraint
	response.WriteJSON(w, http.StatusCreated, constraint)
}

// isKubernetesForbiddenErr checks if an error indicates a Kubernetes RBAC forbidden response.
// Returns true if the error message contains "is forbidden" (K8s RBAC denial pattern).
func isKubernetesForbiddenErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "is forbidden")
}

// isAlreadyExistsErr checks if an error indicates a resource already exists.
func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "already exists") || strings.Contains(errStr, "alreadyexists")
}

// checkHistoryEnabled verifies that the violation history service is available.
// Returns false and writes an appropriate error if not available.
func (h *ComplianceHandler) checkHistoryEnabled(w http.ResponseWriter) bool {
	if h.historyService == nil || !h.historyService.IsAvailable() {
		response.ServiceUnavailable(w, "violation history unavailable: Redis not connected")
		return false
	}
	return true
}

// ListViolationHistory returns paginated violation history records.
// GET /api/v1/compliance/violations/history?since=...&until=...&page=...&pageSize=...&enforcement=...&constraint=...&resource=...&status=...
func (h *ComplianceHandler) ListViolationHistory(w http.ResponseWriter, r *http.Request) {
	if !h.checkEnabled(w, r) {
		return
	}
	if !h.checkPermission(w, r) {
		return
	}
	if !h.checkHistoryEnabled(w) {
		return
	}

	q := r.URL.Query()

	since, until, ok := parseTimeRange(w, q)
	if !ok {
		return
	}

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	opts := services.ViolationHistoryListOptions{
		Page:        page,
		PageSize:    pageSize,
		Enforcement: q.Get("enforcement"),
		Constraint:  q.Get("constraint"),
		Resource:    q.Get("resource"),
		Status:      q.Get("status"),
	}

	records, total, err := h.historyService.ListByTimeRange(r.Context(), since, until, opts)
	if err != nil {
		h.logger.Error("failed to list violation history", "error", err)
		response.InternalError(w, "failed to list violation history")
		return
	}

	response.WriteJSON(w, http.StatusOK, ListResponse[services.ViolationHistoryRecord]{
		Items:    records,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// CountViolationHistory returns the count of violation records matching filters.
// GET /api/v1/compliance/violations/history/count?since=...&until=...&enforcement=...&constraint=...&resource=...
func (h *ComplianceHandler) CountViolationHistory(w http.ResponseWriter, r *http.Request) {
	if !h.checkEnabled(w, r) {
		return
	}
	if !h.checkPermission(w, r) {
		return
	}
	if !h.checkHistoryEnabled(w) {
		return
	}

	q := r.URL.Query()

	since, until, ok := parseTimeRange(w, q)
	if !ok {
		return
	}

	filters := services.ViolationHistoryExportFilters{
		Enforcement: q.Get("enforcement"),
		Constraint:  q.Get("constraint"),
		Resource:    q.Get("resource"),
	}

	count, err := h.historyService.CountByTimeRange(r.Context(), since, until, filters)
	if err != nil {
		h.logger.Error("failed to count violation history", "error", err)
		response.InternalError(w, "failed to count violation history")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"count":         count,
		"retentionDays": h.historyService.GetRetentionDays(),
	})
}

// ExportViolationHistory exports violation history as CSV.
// GET /api/v1/compliance/violations/history/export?since=...&enforcement=...&constraint=...&resource=...
func (h *ComplianceHandler) ExportViolationHistory(w http.ResponseWriter, r *http.Request) {
	if !h.checkEnabled(w, r) {
		return
	}
	if !h.checkPermission(w, r) {
		return
	}
	if !h.checkHistoryEnabled(w) {
		return
	}

	q := r.URL.Query()

	sinceStr := q.Get("since")
	if sinceStr == "" {
		response.BadRequest(w, "missing required parameter: since", nil)
		return
	}

	since, err := time.Parse(time.RFC3339, sinceStr)
	if err != nil {
		response.BadRequest(w, "invalid since parameter: must be RFC3339 format", nil)
		return
	}

	filters := services.ViolationHistoryExportFilters{
		Enforcement: q.Get("enforcement"),
		Constraint:  q.Get("constraint"),
		Resource:    q.Get("resource"),
	}

	// Build filter-aware filename
	filename := buildExportFilename(since, filters)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := h.historyService.ExportCSV(r.Context(), since, filters, w); err != nil {
		h.logger.Error("failed to export violation history CSV", "error", err)
		// Headers already sent, so we can't write an error response
		return
	}
}

// buildExportFilename creates a descriptive CSV filename including active filters.
func buildExportFilename(since time.Time, filters services.ViolationHistoryExportFilters) string {
	parts := []string{"violations"}

	if filters.Enforcement != "" {
		parts = append(parts, sanitize.Filename(filters.Enforcement))
	}
	if filters.Constraint != "" {
		parts = append(parts, sanitize.Filename(filters.Constraint))
	}
	if filters.Resource != "" {
		parts = append(parts, sanitize.Filename(filters.Resource))
	}

	parts = append(parts, since.Format("2006-01-02"))
	parts = append(parts, time.Now().UTC().Format("2006-01-02"))

	return strings.Join(parts, "_") + ".csv"
}

// parseTimeRange parses since and until query parameters.
// If since is missing, defaults to 7 days ago.
// If until is missing, defaults to now.
func parseTimeRange(w http.ResponseWriter, q map[string][]string) (time.Time, time.Time, bool) {
	now := time.Now().UTC()
	since := now.Add(-7 * 24 * time.Hour)
	until := now

	if sinceStr := getQueryParam(q, "since"); sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			response.BadRequest(w, "invalid since parameter: must be RFC3339 format", nil)
			return time.Time{}, time.Time{}, false
		}
	}

	if untilStr := getQueryParam(q, "until"); untilStr != "" {
		var err error
		until, err = time.Parse(time.RFC3339, untilStr)
		if err != nil {
			response.BadRequest(w, "invalid until parameter: must be RFC3339 format", nil)
			return time.Time{}, time.Time{}, false
		}
	}

	return since, until, true
}

// getQueryParam safely gets a query parameter value.
func getQueryParam(q map[string][]string, key string) string {
	vals := q[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
