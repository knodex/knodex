package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/services"
)

// ViewsHandler provides HTTP handlers for view API endpoints.
type ViewsHandler struct {
	service        services.ViewsService
	licenseService services.LicenseService
	logger         *slog.Logger
}

// NewViewsHandler creates a new view HTTP handler.
func NewViewsHandler(service services.ViewsService, logger *slog.Logger) *ViewsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ViewsHandler{
		service: service,
		logger:  logger.With("component", "views-handler"),
	}
}

// SetLicenseService sets the license service for enterprise license checking.
func (h *ViewsHandler) SetLicenseService(ls services.LicenseService) {
	h.licenseService = ls
}

// checkViewsLicense validates the license for the views feature (read-only endpoints).
// Returns false and writes an error if the license check fails.
// Note: If write endpoints are added to views, add a checkViewsLicenseForWrite
// method following the ComplianceHandler.checkEnabledForWrite pattern for read-only mode.
func (h *ViewsHandler) checkViewsLicense(w http.ResponseWriter) bool {
	featureDetail := map[string]string{"feature": services.FeatureViews}

	// Fully licensed (valid or in grace period)
	if h.licenseService.IsFeatureEnabled(services.FeatureViews) {
		if h.licenseService.IsGracePeriod() {
			w.Header().Set("X-License-Warning", "expired")
		}
		return true
	}

	// Read-only mode: expired past grace but feature was in the license
	// Views are read-only by nature, so allow access
	if h.licenseService.IsReadOnly() && h.licenseService.HasFeature(services.FeatureViews) {
		return true
	}

	// Not licensed or feature not in license
	if !h.licenseService.IsLicensed() {
		response.WriteError(w, http.StatusPaymentRequired, "LICENSE_REQUIRED",
			"This feature requires a valid enterprise license", featureDetail)
	} else {
		response.WriteError(w, http.StatusPaymentRequired, "LICENSE_REQUIRED",
			"Views feature is not included in your license", featureDetail)
	}
	return false
}

// ListViews handles GET /api/v1/ee/views
// Returns the list of configured views with RGD counts.
func (h *ViewsHandler) ListViews(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		h.logger.Error("views service not configured")
		response.ServiceUnavailable(w, "views service not configured")
		return
	}

	// Check license validity
	if h.licenseService != nil && !h.checkViewsLicense(w) {
		return
	}

	// Get views with counts
	result := h.service.ListViews(r.Context())

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("failed to encode response", "error", err)
		response.InternalError(w, "failed to encode response")
		return
	}
}

// GetView handles GET /api/v1/ee/views/{slug}
// Returns a specific view by slug.
func (h *ViewsHandler) GetView(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		h.logger.Error("views service not configured")
		response.ServiceUnavailable(w, "views service not configured")
		return
	}

	// Check license validity
	if h.licenseService != nil && !h.checkViewsLicense(w) {
		return
	}

	// Extract slug from URL path
	slug := r.PathValue("slug")
	if slug == "" {
		response.BadRequest(w, "view slug required", nil)
		return
	}

	// Get view by slug
	view := h.service.GetView(slug)
	if view == nil {
		response.NotFound(w, "view", slug)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(view); err != nil {
		h.logger.Error("failed to encode response", "error", err)
		response.InternalError(w, "failed to encode response")
		return
	}
}
