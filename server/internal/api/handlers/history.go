package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/history"
	"github.com/provops-org/knodex/server/internal/models"
)

// sanitizeFilename removes potentially dangerous characters from filenames
// to prevent HTTP header injection attacks via Content-Disposition header
var filenameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeFilename(name string) string {
	// Replace invalid characters with underscore
	sanitized := filenameRegex.ReplaceAllString(name, "_")
	// Prevent path traversal
	sanitized = strings.ReplaceAll(sanitized, "..", "_")
	// Limit length to prevent buffer issues
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}
	// Ensure not empty
	if sanitized == "" {
		sanitized = "export"
	}
	return sanitized
}

// HistoryHandler handles deployment history HTTP requests
type HistoryHandler struct {
	historyService *history.Service
}

// NewHistoryHandler creates a new history handler
func NewHistoryHandler(hs *history.Service) *HistoryHandler {
	return &HistoryHandler{
		historyService: hs,
	}
}

// GetHistory handles GET /api/v1/instances/{namespace}/{kind}/{name}/history
// Returns the deployment history for an instance
// Note: Access control is handled by authorization middleware
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if h.historyService == nil {
		response.ServiceUnavailable(w, "History service not available")
		return
	}

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if namespace == "" || kind == "" || name == "" {
		response.BadRequest(w, "namespace, kind, and name are required", nil)
		return
	}

	// Get history from service
	historyData, err := h.historyService.GetHistory(r.Context(), namespace, kind, name)
	if err != nil {
		// If not found, try to get deleted history
		deletedHistory, delErr := h.historyService.GetDeletedHistory(r.Context(), namespace+"/"+kind+"/"+name)
		if delErr != nil {
			response.NotFound(w, "History", namespace+"/"+kind+"/"+name)
			return
		}
		response.WriteJSON(w, http.StatusOK, deletedHistory)
		return
	}

	response.WriteJSON(w, http.StatusOK, historyData)
}

// GetTimeline handles GET /api/v1/instances/{namespace}/{kind}/{name}/timeline
// Returns a simplified timeline for UI display
// Note: Access control is handled by authorization middleware
func (h *HistoryHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	if h.historyService == nil {
		response.ServiceUnavailable(w, "History service not available")
		return
	}

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if namespace == "" || kind == "" || name == "" {
		response.BadRequest(w, "namespace, kind, and name are required", nil)
		return
	}

	// Get timeline from service
	timeline, err := h.historyService.GetTimeline(r.Context(), namespace, kind, name)
	if err != nil {
		response.NotFound(w, "Timeline", namespace+"/"+kind+"/"+name)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"namespace": namespace,
		"kind":      kind,
		"name":      name,
		"timeline":  timeline,
	})
}

// ExportHistory handles GET /api/v1/instances/{namespace}/{kind}/{name}/history/export
// Exports deployment history to CSV or JSON format
// Note: Access control is handled by authorization middleware
func (h *HistoryHandler) ExportHistory(w http.ResponseWriter, r *http.Request) {
	if h.historyService == nil {
		response.ServiceUnavailable(w, "History service not available")
		return
	}

	namespace := r.PathValue("namespace")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	if namespace == "" || kind == "" || name == "" {
		response.BadRequest(w, "namespace, kind, and name are required", nil)
		return
	}

	// Get export format from query parameter (default: json)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// Validate format
	if format != "json" && format != "csv" {
		response.BadRequest(w, "invalid format: must be 'json' or 'csv'", nil)
		return
	}

	// Get history from service
	historyData, err := h.historyService.GetHistory(r.Context(), namespace, kind, name)
	if err != nil {
		// Try deleted history
		historyData, err = h.historyService.GetDeletedHistory(r.Context(), namespace+"/"+kind+"/"+name)
		if err != nil {
			response.NotFound(w, "History", namespace+"/"+kind+"/"+name)
			return
		}
	}

	// Set headers based on format
	// Sanitize filename to prevent HTTP header injection
	filename := sanitizeFilename(name) + "-deployment-history"

	switch models.HistoryExportFormat(format) {
	case models.ExportFormatCSV:
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+".csv\"")
		if err := historyData.ExportToCSV(w); err != nil {
			response.InternalError(w, "Failed to export history to CSV: "+err.Error())
			return
		}
	case models.ExportFormatJSON:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+".json\"")
		if err := historyData.ExportToJSON(w); err != nil {
			response.InternalError(w, "Failed to export history to JSON: "+err.Error())
			return
		}
	}
}
