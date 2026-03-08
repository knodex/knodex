package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/services"
)

// licenseAccessChecker is the subset of rbac.PolicyEnforcer needed by LicenseHandler.
type licenseAccessChecker interface {
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

// LicenseHandler provides HTTP handlers for the license API endpoints.
type LicenseHandler struct {
	licenseService services.LicenseService
	accessChecker  licenseAccessChecker
	logger         *slog.Logger
}

// NewLicenseHandler creates a new LicenseHandler.
// accessChecker should be a rbac.PolicyEnforcer (or nil if not available).
func NewLicenseHandler(licenseService services.LicenseService, accessChecker licenseAccessChecker, logger *slog.Logger) *LicenseHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &LicenseHandler{
		licenseService: licenseService,
		accessChecker:  accessChecker,
		logger:         logger.With("handler", "license"),
	}
}

// GetStatus handles GET /api/v1/license
// Returns the current license status. Any authenticated user can view this.
// Design decision: license status (edition, feature flags) is intentionally
// readable by all users so the UI can adapt feature visibility. Sensitive
// fields (license key, token) are never exposed. Write access (UpdateLicense)
// requires settings:update permission (serveradmin only).
func (h *LicenseHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if h.licenseService == nil {
		response.WriteJSON(w, http.StatusOK, &services.LicenseStatus{
			Licensed:   false,
			Enterprise: false,
			Status:     "oss",
			Message:    "Open source edition",
		})
		return
	}

	status := h.licenseService.GetStatus()
	response.WriteJSON(w, http.StatusOK, status)
}

// updateLicenseRequest is the request body for POST /api/v1/license.
type updateLicenseRequest struct {
	Token string `json:"token"`
}

// UpdateLicense handles POST /api/v1/license
// Validates and applies a new license JWT at runtime.
// Requires settings:update permission (admin-only).
func (h *LicenseHandler) UpdateLicense(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")

	// Check user authentication
	userCtx, ok := middleware.GetUserContext(r)
	if !ok {
		response.Unauthorized(w, "User context not found")
		return
	}

	// Check settings:update permission using Casbin
	if h.accessChecker == nil {
		h.logger.Warn("policy enforcer unavailable, denying license update",
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return
	}

	allowed, err := h.accessChecker.CanAccessWithGroups(
		r.Context(),
		userCtx.UserID,
		userCtx.Groups,
		"settings/*",
		"update",
	)
	if err != nil {
		h.logger.Error("failed to check license update permission",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.InternalError(w, "Failed to check authorization")
		return
	}
	if !allowed {
		h.logger.Warn("license update denied",
			"requestId", requestID,
			"userId", userCtx.UserID,
		)
		response.Forbidden(w, "permission denied")
		return
	}

	// Parse request body
	var req updateLicenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid request body", nil)
		return
	}
	if req.Token == "" {
		response.BadRequest(w, "License token is required", nil)
		return
	}

	// Validate and apply the new license
	if err := h.licenseService.UpdateLicense(req.Token); err != nil {
		h.logger.Warn("license update failed",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		response.BadRequest(w, "Invalid license token", nil)
		return
	}

	h.logger.Info("license updated successfully",
		"requestId", requestID,
		"userId", userCtx.UserID,
	)

	// Return updated status
	status := h.licenseService.GetStatus()
	response.WriteJSON(w, http.StatusOK, status)
}
