package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/provops-org/knodex/server/internal/api/middleware"
	"github.com/provops-org/knodex/server/internal/api/response"
	"github.com/provops-org/knodex/server/internal/services"
	"github.com/provops-org/knodex/server/internal/watcher"
)

// GitOpsSyncMonitor provides access to GitOps sync tracking data
type GitOpsSyncMonitor interface {
	// GetStatusTimeline returns the status history for an instance
	GetStatusTimeline(instanceID string) []watcher.StatusTransition
	// GetPendingInstance returns a pending GitOps instance by ID and existence bool
	GetPendingInstance(instanceID string) (*watcher.PendingGitOpsInstance, bool)
	// GetStuckInstances returns instances stuck longer than threshold
	GetStuckInstances() []*watcher.PendingGitOpsInstance
	// ListPending returns all pending GitOps instances
	ListPending() []*watcher.PendingGitOpsInstance
}

// InstanceGitOpsHandler handles GitOps monitoring and status tracking
type InstanceGitOpsHandler struct {
	syncMonitor GitOpsSyncMonitor
	authService *services.AuthorizationService
	logger      *slog.Logger
}

// InstanceGitOpsHandlerConfig holds configuration for creating an InstanceGitOpsHandler
type InstanceGitOpsHandlerConfig struct {
	SyncMonitor GitOpsSyncMonitor
	AuthService *services.AuthorizationService
	Logger      *slog.Logger
}

// NewInstanceGitOpsHandler creates a new GitOps monitoring handler
func NewInstanceGitOpsHandler(config InstanceGitOpsHandlerConfig) *InstanceGitOpsHandler {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &InstanceGitOpsHandler{
		syncMonitor: config.SyncMonitor,
		authService: config.AuthService,
		logger:      logger.With("component", "instance-gitops-handler"),
	}
}

// getAccessibleNamespaces retrieves the user's accessible namespaces using AuthorizationService.
// Returns:
// - nil: User can see all namespaces (global admin or auth not configured)
// - empty slice: User has no namespace access (secure default for unauthenticated)
// - non-empty slice: User can only see instances in these namespaces
func (h *InstanceGitOpsHandler) getAccessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if userCtx == nil {
		return []string{}, nil
	}
	if h.authService == nil {
		return nil, nil
	}
	return h.authService.GetAccessibleNamespaces(ctx, userCtx)
}

// StatusTimelineResponse represents the timeline of status changes for an instance (AC-8)
type StatusTimelineResponse struct {
	InstanceID     string                     `json:"instanceId"`
	Name           string                     `json:"name,omitempty"`
	Namespace      string                     `json:"namespace,omitempty"`
	CurrentStatus  string                     `json:"currentStatus,omitempty"`
	DeploymentMode string                     `json:"deploymentMode,omitempty"`
	PushedAt       *time.Time                 `json:"pushedAt,omitempty"`
	IsStuck        bool                       `json:"isStuck"`
	Timeline       []StatusTransitionResponse `json:"timeline"`
}

// StatusTransitionResponse represents a single status transition
type StatusTransitionResponse struct {
	FromStatus string    `json:"fromStatus"`
	ToStatus   string    `json:"toStatus"`
	Timestamp  time.Time `json:"timestamp"`
	Reason     string    `json:"reason,omitempty"`
}

// PendingInstanceResponse represents a pending GitOps instance for API response
type PendingInstanceResponse struct {
	InstanceID     string    `json:"instanceId"`
	Name           string    `json:"name"`
	Namespace      string    `json:"namespace"`
	RGDName        string    `json:"rgdName"`
	RGDNamespace   string    `json:"rgdNamespace,omitempty"`
	DeploymentMode string    `json:"deploymentMode"`
	Status         string    `json:"status"`
	PushedAt       time.Time `json:"pushedAt"`
	CommitSHA      string    `json:"commitSha,omitempty"`
	IsStuck        bool      `json:"isStuck"`
}

// GetStatusTimeline handles GET /api/v1/instances/timeline/{instanceId}
// Returns the status history for a GitOps-deployed instance (AC-8)
func (h *InstanceGitOpsHandler) GetStatusTimeline(w http.ResponseWriter, r *http.Request) {
	if h.syncMonitor == nil {
		response.ServiceUnavailable(w, "GitOps sync monitor not available")
		return
	}

	instanceID := r.PathValue("instanceId")
	if instanceID == "" {
		response.BadRequest(w, "instanceId is required", nil)
		return
	}

	// Get timeline from sync monitor
	timeline := h.syncMonitor.GetStatusTimeline(instanceID)

	// Get pending instance info if available
	pending, _ := h.syncMonitor.GetPendingInstance(instanceID)

	resp := StatusTimelineResponse{
		InstanceID: instanceID,
		Timeline:   make([]StatusTransitionResponse, 0, len(timeline)),
	}

	// Populate from pending instance if available
	if pending != nil {
		resp.Name = pending.Name
		resp.Namespace = pending.Namespace
		resp.CurrentStatus = string(pending.Status)
		resp.DeploymentMode = string(pending.DeploymentMode)
		resp.PushedAt = &pending.PushedAt

		// Check if stuck (AC-6)
		stuckInstances := h.syncMonitor.GetStuckInstances()
		for _, stuck := range stuckInstances {
			if stuck.InstanceID == instanceID {
				resp.IsStuck = true
				break
			}
		}
	}

	// Convert timeline to response format
	for _, t := range timeline {
		resp.Timeline = append(resp.Timeline, StatusTransitionResponse{
			FromStatus: string(t.FromStatus),
			ToStatus:   string(t.ToStatus),
			Timestamp:  t.Timestamp,
			Reason:     t.Reason,
		})
	}

	response.WriteJSON(w, http.StatusOK, resp)
}

// GetPendingInstances handles GET /api/v1/instances/pending
// Returns all pending GitOps instances awaiting cluster sync
func (h *InstanceGitOpsHandler) GetPendingInstances(w http.ResponseWriter, r *http.Request) {
	if h.syncMonitor == nil {
		response.ServiceUnavailable(w, "GitOps sync monitor not available")
		return
	}

	// Get user context for namespace filtering
	userCtx, _ := middleware.GetUserContext(r)
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Get all pending instances
	pendingList := h.syncMonitor.ListPending()

	// Get stuck instances for comparison
	stuckInstances := h.syncMonitor.GetStuckInstances()
	stuckIDs := make(map[string]bool)
	for _, s := range stuckInstances {
		stuckIDs[s.InstanceID] = true
	}

	// Filter and convert to response
	var result []PendingInstanceResponse
	for _, p := range pendingList {
		// Apply namespace filtering if user has limited access
		if userNamespaces != nil {
			hasAccess := false
			for _, ns := range userNamespaces {
				if p.Namespace == ns {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				continue
			}
		}

		commitSHA := ""
		if p.GitInfo != nil {
			commitSHA = p.GitInfo.CommitSHA
		}

		result = append(result, PendingInstanceResponse{
			InstanceID:     p.InstanceID,
			Name:           p.Name,
			Namespace:      p.Namespace,
			RGDName:        p.RGDName,
			RGDNamespace:   p.RGDNamespace,
			DeploymentMode: string(p.DeploymentMode),
			Status:         string(p.Status),
			PushedAt:       p.PushedAt,
			CommitSHA:      commitSHA,
			IsStuck:        stuckIDs[p.InstanceID],
		})
	}

	if result == nil {
		result = []PendingInstanceResponse{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"items": result,
		"total": len(result),
	})
}

// GetStuckInstances handles GET /api/v1/instances/stuck
// Returns instances stuck in PushedToGit state longer than configured threshold (AC-6, AC-7)
func (h *InstanceGitOpsHandler) GetStuckInstances(w http.ResponseWriter, r *http.Request) {
	if h.syncMonitor == nil {
		response.ServiceUnavailable(w, "GitOps sync monitor not available")
		return
	}

	// Get user context for namespace filtering
	userCtx, _ := middleware.GetUserContext(r)
	userNamespaces, err := h.getAccessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("failed to get accessible namespaces", "error", err)
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Get stuck instances
	stuckList := h.syncMonitor.GetStuckInstances()

	// Filter and convert to response
	var result []PendingInstanceResponse
	for _, s := range stuckList {
		// Apply namespace filtering if user has limited access
		if userNamespaces != nil {
			hasAccess := false
			for _, ns := range userNamespaces {
				if s.Namespace == ns {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				continue
			}
		}

		commitSHA := ""
		if s.GitInfo != nil {
			commitSHA = s.GitInfo.CommitSHA
		}

		result = append(result, PendingInstanceResponse{
			InstanceID:     s.InstanceID,
			Name:           s.Name,
			Namespace:      s.Namespace,
			RGDName:        s.RGDName,
			RGDNamespace:   s.RGDNamespace,
			DeploymentMode: string(s.DeploymentMode),
			Status:         string(s.Status),
			PushedAt:       s.PushedAt,
			CommitSHA:      commitSHA,
			IsStuck:        true,
		})
	}

	if result == nil {
		result = []PendingInstanceResponse{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"items": result,
		"total": len(result),
	})
}
