// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/history"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/services"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

func sanitizeFilename(name string) string {
	return sanitize.Filename(name)
}

// HistoryHandler handles deployment history HTTP requests
type HistoryHandler struct {
	historyService   *history.Service
	revisionProvider services.GraphRevisionProvider // nil = feature disabled
	logger           *slog.Logger
}

// NewHistoryHandler creates a new history handler
func NewHistoryHandler(hs *history.Service, rp services.GraphRevisionProvider, logger *slog.Logger) *HistoryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &HistoryHandler{
		historyService:   hs,
		revisionProvider: rp,
		logger:           logger.With("component", "history-handler"),
	}
}

// GetHistory handles instance deployment history retrieval.
// K8s-aligned routes: /api/v1/namespaces/{ns}/instances/{kind}/{name}/history (namespaced)
//
//	/api/v1/instances/{kind}/{name}/history (cluster-scoped)
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if h.historyService == nil {
		response.ServiceUnavailable(w, "History service not available")
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

	// Get history from service
	instanceID := namespace + "/" + kind + "/" + name
	historyData, err := h.historyService.GetHistory(r.Context(), namespace, kind, name)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			h.logger.Error("failed to retrieve history", "instanceId", instanceID, "error", err)
			response.ServiceUnavailable(w, "history service unavailable")
			return
		}
		// Not found — try deleted history
		deletedHistory, delErr := h.historyService.GetDeletedHistory(r.Context(), instanceID)
		if delErr != nil {
			response.NotFound(w, "History", instanceID)
			return
		}
		response.WriteJSON(w, http.StatusOK, deletedHistory)
		return
	}

	response.WriteJSON(w, http.StatusOK, historyData)
}

// GetTimeline handles instance timeline retrieval.
// K8s-aligned routes: /api/v1/namespaces/{ns}/instances/{kind}/{name}/timeline (namespaced)
//
//	/api/v1/instances/{kind}/{name}/timeline (cluster-scoped)
//
// Query params: ?source=kubernetes (only K8s Events), ?source=deployment (only deployment events)
func (h *HistoryHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	if h.historyService == nil {
		response.ServiceUnavailable(w, "History service not available")
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

	// Parse optional source filter (STORY-405: K8s Events support)
	source := r.URL.Query().Get("source")

	// Get full history (not just timeline) so we have RGDName for revision lookup
	instanceID := namespace + "/" + kind + "/" + name
	historyData, err := h.historyService.GetHistory(r.Context(), namespace, kind, name)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			h.logger.Error("failed to retrieve timeline", "instanceId", instanceID, "error", err)
			response.ServiceUnavailable(w, "history service unavailable")
			return
		}
		// Not found — try deleted history (deleted instances still have a timeline)
		deletedHistory, delErr := h.historyService.GetDeletedHistory(r.Context(), instanceID)
		if delErr != nil {
			response.NotFound(w, "Timeline", instanceID)
			return
		}
		timeline := deletedHistory.GetTimeline()
		// Apply source filter to deleted history too
		if source != "" {
			timeline = filterTimelineBySource(timeline, source)
		}
		response.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"namespace": namespace,
			"kind":      kind,
			"name":      name,
			"timeline":  timeline,
		})
		return
	}

	// Use filtered timeline when source is specified, otherwise use standard timeline
	var timeline []models.TimelineEntry
	if source != "" {
		timeline, err = h.historyService.GetFilteredTimeline(r.Context(), namespace, kind, name, source)
		if err != nil {
			h.logger.Error("failed to retrieve filtered timeline", "instanceId", instanceID, "source", source, "error", err)
			response.ServiceUnavailable(w, "history service unavailable")
			return
		}
		// GetFilteredTimeline returns nil for unknown source values (backward-safe)
		if timeline == nil {
			response.BadRequest(w, "invalid source filter: must be 'kubernetes' or 'deployment'", nil)
			return
		}
		// Skip revision merge for K8s events (they don't have revision markers)
		// Merge revision markers only for deployment events
	} else {
		timeline = historyData.GetTimeline()
		// Merge revision markers if provider is available (STORY-402: query-time merge)
		if h.revisionProvider != nil && historyData.RGDName != "" {
			revisions := h.revisionProvider.ListRevisions(historyData.RGDName)
			if len(revisions.Items) > 0 {
				timeline = mergeRevisionMarkers(timeline, revisions.Items, historyData.CreatedAt)
			}
		}
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"namespace": namespace,
		"kind":      kind,
		"name":      name,
		"timeline":  timeline,
	})
}

// filterTimelineBySource filters a timeline slice by source (helper for deleted history).
func filterTimelineBySource(timeline []models.TimelineEntry, source string) []models.TimelineEntry {
	if source == "" {
		return timeline
	}
	var filtered []models.TimelineEntry
	for _, entry := range timeline {
		switch source {
		case "kubernetes":
			if entry.EventType == models.EventTypeKubernetesEvent {
				filtered = append(filtered, entry)
			}
		case "deployment":
			if entry.EventType != models.EventTypeKubernetesEvent {
				filtered = append(filtered, entry)
			}
		}
	}
	return filtered
}

// mergeRevisionMarkers converts GraphRevisions to TimelineEntry markers and merges
// them with deployment timeline entries, sorted by timestamp ascending (oldest first).
// Revisions that predate instanceCreatedAt are excluded — they belong to the RGD's history,
// not this instance's. Pass a zero time to skip filtering.
func mergeRevisionMarkers(timeline []models.TimelineEntry, revisions []models.GraphRevision, instanceCreatedAt time.Time) []models.TimelineEntry {
	// Drop revisions that predate this instance
	if !instanceCreatedAt.IsZero() {
		filtered := make([]models.GraphRevision, 0, len(revisions))
		for _, r := range revisions {
			if !r.CreatedAt.Before(instanceCreatedAt) {
				filtered = append(filtered, r)
			}
		}
		revisions = filtered
	}

	// Convert revisions to timeline entries
	for i, rev := range revisions {
		var message string
		var prevRevision int
		if rev.RevisionNumber == 1 {
			message = "RGD Revision 1 (initial)"
		} else {
			// Revisions are sorted descending by revision number from the provider.
			// Compute previous revision from the next item in the list, or from RevisionNumber-1.
			prevRevision = rev.RevisionNumber - 1
			// If there's a next item in the list (lower revision), use its actual number
			if i+1 < len(revisions) {
				prevRevision = revisions[i+1].RevisionNumber
			}
			message = fmt.Sprintf("RGD Revision %d → %d", prevRevision, rev.RevisionNumber)
		}

		entry := models.TimelineEntry{
			Timestamp:        rev.CreatedAt,
			EventType:        models.EventTypeRevisionChanged,
			Status:           "",
			User:             "system",
			Message:          message,
			IsCompleted:      true,
			IsCurrent:        false,
			RevisionNumber:   rev.RevisionNumber,
			PreviousRevision: prevRevision,
		}
		timeline = append(timeline, entry)
	}

	// Sort by timestamp ascending (oldest first) to match deployment timeline order
	sort.SliceStable(timeline, func(i, j int) bool {
		return timeline[i].Timestamp.Before(timeline[j].Timestamp)
	})

	// Recalculate IsCurrent: only the last entry should be current
	for i := range timeline {
		timeline[i].IsCurrent = i == len(timeline)-1
	}

	return timeline
}

// ExportHistory handles instance deployment history export.
// K8s-aligned routes: /api/v1/namespaces/{ns}/instances/{kind}/{name}/history/export (namespaced)
//
//	/api/v1/instances/{kind}/{name}/history/export (cluster-scoped)
func (h *HistoryHandler) ExportHistory(w http.ResponseWriter, r *http.Request) {
	if h.historyService == nil {
		response.ServiceUnavailable(w, "History service not available")
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
	instanceID := namespace + "/" + kind + "/" + name
	historyData, err := h.historyService.GetHistory(r.Context(), namespace, kind, name)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			h.logger.Error("failed to retrieve history for export", "instanceId", instanceID, "error", err)
			response.ServiceUnavailable(w, "history service unavailable")
			return
		}
		// Not found — try deleted history
		historyData, err = h.historyService.GetDeletedHistory(r.Context(), instanceID)
		if err != nil {
			response.NotFound(w, "History", instanceID)
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
