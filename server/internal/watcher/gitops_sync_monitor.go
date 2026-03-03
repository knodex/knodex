package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/provops-org/knodex/server/internal/deployment"
	"github.com/provops-org/knodex/server/internal/models"
)

// GitOps-related annotation keys
const (
	// InstanceIDAnnotation stores the unique instance ID for correlation
	InstanceIDAnnotation = "knodex.io/instance-id"
	// GitCommitSHAAnnotation stores the Git commit SHA
	GitCommitSHAAnnotation = "knodex.io/git-commit-sha"
	// PushedAtAnnotation stores when the manifest was pushed to Git
	PushedAtAnnotation = "knodex.io/pushed-at"
)

// Default configuration values
const (
	// DefaultStuckThreshold is the default time after which an instance is considered stuck
	DefaultStuckThreshold = 10 * time.Minute
	// DefaultCheckInterval is how often to check for stuck instances
	DefaultCheckInterval = 1 * time.Minute
)

// PendingGitOpsInstance represents an instance that has been pushed to Git
// but not yet synced to the cluster by ArgoCD/Flux
type PendingGitOpsInstance struct {
	// InstanceID is the unique identifier for correlation
	InstanceID string `json:"instanceId"`
	// Name is the expected instance name
	Name string `json:"name"`
	// Namespace is the expected instance namespace
	Namespace string `json:"namespace"`
	// RGDName is the ResourceGraphDefinition name
	RGDName string `json:"rgdName"`
	// RGDNamespace is the ResourceGraphDefinition namespace
	RGDNamespace string `json:"rgdNamespace"`
	// DeploymentMode is the deployment mode (gitops or hybrid)
	DeploymentMode deployment.DeploymentMode `json:"deploymentMode"`
	// Status is the current instance status
	Status deployment.InstanceStatus `json:"status"`
	// GitInfo contains Git push details
	GitInfo *deployment.GitInfo `json:"gitInfo,omitempty"`
	// PushedAt is when the manifest was pushed to Git
	PushedAt time.Time `json:"pushedAt"`
	// ProjectID is the project that owns this instance
	ProjectID string `json:"projectId,omitempty"`
	// StatusHistory tracks status transitions
	StatusHistory []StatusTransition `json:"statusHistory,omitempty"`
}

// StatusTransition records a status change event
type StatusTransition struct {
	// FromStatus is the previous status (empty for initial status)
	FromStatus deployment.InstanceStatus `json:"fromStatus,omitempty"`
	// ToStatus is the new status
	ToStatus deployment.InstanceStatus `json:"toStatus"`
	// Timestamp is when the transition occurred
	Timestamp time.Time `json:"timestamp"`
	// Reason provides context for the transition
	Reason string `json:"reason,omitempty"`
}

// StuckInstanceCallback is called when an instance is detected as stuck
type StuckInstanceCallback func(instance *PendingGitOpsInstance, duration time.Duration)

// GitOpsSyncMonitorConfig configures the sync monitor behavior
type GitOpsSyncMonitorConfig struct {
	// StuckThreshold is how long an instance can be in PushedToGit before being considered stuck
	StuckThreshold time.Duration
	// CheckInterval is how often to check for stuck instances
	CheckInterval time.Duration
	// OnStuckInstance is called when an instance is detected as stuck
	OnStuckInstance StuckInstanceCallback
}

// DefaultGitOpsSyncMonitorConfig returns default configuration
func DefaultGitOpsSyncMonitorConfig() GitOpsSyncMonitorConfig {
	return GitOpsSyncMonitorConfig{
		StuckThreshold: DefaultStuckThreshold,
		CheckInterval:  DefaultCheckInterval,
	}
}

// GitOpsSyncMonitor tracks GitOps instances from push to cluster sync
type GitOpsSyncMonitor struct {
	// pendingInstances maps instanceID to pending instance
	pendingInstances map[string]*PendingGitOpsInstance
	mu               sync.RWMutex

	// instanceTracker provides cluster instance updates
	instanceTracker *InstanceTracker

	// config holds monitor configuration
	config GitOpsSyncMonitorConfig

	// logger for structured logging
	logger *slog.Logger

	// stopCh signals the monitor to stop
	stopCh chan struct{}

	// running indicates if the monitor is active
	running bool

	// callbacks for status changes
	onStatusChange func(instance *PendingGitOpsInstance)
}

// NewGitOpsSyncMonitor creates a new GitOps sync monitor
func NewGitOpsSyncMonitor(instanceTracker *InstanceTracker, config GitOpsSyncMonitorConfig) *GitOpsSyncMonitor {
	return &GitOpsSyncMonitor{
		pendingInstances: make(map[string]*PendingGitOpsInstance),
		instanceTracker:  instanceTracker,
		config:           config,
		logger:           slog.Default().With("component", "gitops-sync-monitor"),
		stopCh:           make(chan struct{}),
	}
}

// Start begins monitoring for GitOps sync events
func (m *GitOpsSyncMonitor) Start(ctx context.Context) error {
	if m.running {
		m.logger.Warn("gitops sync monitor already running")
		return nil
	}

	m.logger.Info("starting gitops sync monitor",
		"stuckThreshold", m.config.StuckThreshold,
		"checkInterval", m.config.CheckInterval)

	m.running = true

	// Register for instance updates from the tracker
	if m.instanceTracker != nil {
		m.instanceTracker.SetOnUpdateCallback(m.handleInstanceUpdate)
	}

	// Start background checker for stuck instances
	go m.runStuckInstanceChecker(ctx)

	return nil
}

// Stop stops the sync monitor
func (m *GitOpsSyncMonitor) Stop() {
	if !m.running {
		return
	}

	m.logger.Info("stopping gitops sync monitor")
	close(m.stopCh)
	m.running = false
}

// SetOnStatusChangeCallback sets a callback for status changes
func (m *GitOpsSyncMonitor) SetOnStatusChangeCallback(cb func(instance *PendingGitOpsInstance)) {
	m.onStatusChange = cb
}

// TrackPushedInstance registers an instance that has been pushed to Git
func (m *GitOpsSyncMonitor) TrackPushedInstance(instanceID, name, namespace, rgdName, rgdNamespace string, mode deployment.DeploymentMode, gitInfo *deployment.GitInfo, projectID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	pushedAt := now
	if gitInfo != nil && gitInfo.PushedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, gitInfo.PushedAt); err == nil {
			pushedAt = parsed
		}
	}

	pending := &PendingGitOpsInstance{
		InstanceID:     instanceID,
		Name:           name,
		Namespace:      namespace,
		RGDName:        rgdName,
		RGDNamespace:   rgdNamespace,
		DeploymentMode: mode,
		Status:         deployment.StatusPushedToGit,
		GitInfo:        gitInfo,
		PushedAt:       pushedAt,
		ProjectID:      projectID,
		StatusHistory: []StatusTransition{
			{
				ToStatus:  deployment.StatusPushedToGit,
				Timestamp: now,
				Reason:    "Manifest pushed to Git repository",
			},
		},
	}

	m.pendingInstances[instanceID] = pending

	m.logger.Info("tracking pushed instance",
		"instanceId", instanceID,
		"name", name,
		"namespace", namespace,
		"deploymentMode", mode)

	m.notifyStatusChange(pending)
}

// UpdateStatus updates the status of a pending instance
func (m *GitOpsSyncMonitor) UpdateStatus(instanceID string, newStatus deployment.InstanceStatus, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, exists := m.pendingInstances[instanceID]
	if !exists {
		return false
	}

	if pending.Status == newStatus {
		return false
	}

	oldStatus := pending.Status
	pending.Status = newStatus
	pending.StatusHistory = append(pending.StatusHistory, StatusTransition{
		FromStatus: oldStatus,
		ToStatus:   newStatus,
		Timestamp:  time.Now(),
		Reason:     reason,
	})

	m.logger.Info("instance status updated",
		"instanceId", instanceID,
		"fromStatus", oldStatus,
		"toStatus", newStatus,
		"reason", reason)

	m.notifyStatusChange(pending)
	return true
}

// GetPendingInstance returns a pending instance by ID
func (m *GitOpsSyncMonitor) GetPendingInstance(instanceID string) (*PendingGitOpsInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.pendingInstances[instanceID]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent race conditions
	result := *instance
	if instance.GitInfo != nil {
		gitInfoCopy := *instance.GitInfo
		result.GitInfo = &gitInfoCopy
	}
	result.StatusHistory = make([]StatusTransition, len(instance.StatusHistory))
	copy(result.StatusHistory, instance.StatusHistory)

	return &result, true
}

// GetPendingByNamespace returns all pending instances in a namespace
func (m *GitOpsSyncMonitor) GetPendingByNamespace(namespace, name string) (*PendingGitOpsInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pending := range m.pendingInstances {
		if pending.Namespace == namespace && pending.Name == name {
			copy := *pending
			return &copy, true
		}
	}
	return nil, false
}

// GetAllPending returns all pending GitOps instances
func (m *GitOpsSyncMonitor) GetAllPending() []*PendingGitOpsInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PendingGitOpsInstance, 0, len(m.pendingInstances))
	for _, pending := range m.pendingInstances {
		copy := *pending
		result = append(result, &copy)
	}
	return result
}

// ListPending is an alias for GetAllPending - returns all pending GitOps instances
func (m *GitOpsSyncMonitor) ListPending() []*PendingGitOpsInstance {
	return m.GetAllPending()
}

// RemovePendingInstance removes an instance from tracking
func (m *GitOpsSyncMonitor) RemovePendingInstance(instanceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pendingInstances[instanceID]; exists {
		delete(m.pendingInstances, instanceID)
		m.logger.Debug("removed pending instance", "instanceId", instanceID)
		return true
	}
	return false
}

// handleInstanceUpdate is called when cluster instances are updated
func (m *GitOpsSyncMonitor) handleInstanceUpdate(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
	if action == InstanceActionDelete {
		// Check if this was a pending instance that got deleted
		// Collect IDs to delete first to avoid concurrent map modification during iteration
		m.mu.Lock()
		idsToDelete := make([]string, 0)
		for id, pending := range m.pendingInstances {
			if pending.Namespace == namespace && pending.Name == name {
				idsToDelete = append(idsToDelete, id)
			}
		}
		// Now delete the collected IDs
		for _, id := range idsToDelete {
			delete(m.pendingInstances, id)
			m.logger.Info("pending instance deleted from cluster",
				"instanceId", id,
				"name", name,
				"namespace", namespace)
		}
		m.mu.Unlock()
		return
	}

	if instance == nil {
		return
	}

	// Try to correlate using annotation-based instance ID
	annotations := instance.Annotations
	if annotations != nil {
		if instanceID, hasID := annotations[InstanceIDAnnotation]; hasID {
			m.correlateByInstanceID(instanceID, instance)
			return
		}
	}

	// Fallback: correlate by namespace/name match
	m.correlateByNamespaceName(namespace, name, instance)
}

// correlateByInstanceID correlates a cluster instance with a pending instance using the instance ID
func (m *GitOpsSyncMonitor) correlateByInstanceID(instanceID string, instance *models.Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, exists := m.pendingInstances[instanceID]
	if !exists {
		m.logger.Debug("no pending instance found for ID", "instanceId", instanceID)
		return
	}

	m.logger.Info("correlated cluster instance with pending GitOps instance",
		"instanceId", instanceID,
		"name", instance.Name,
		"namespace", instance.Namespace,
		"health", instance.Health)

	// Determine new status based on instance health
	var newStatus deployment.InstanceStatus
	var reason string

	switch instance.Health {
	case models.HealthHealthy:
		newStatus = deployment.StatusReady
		reason = "Instance is healthy in cluster"
	case models.HealthProgressing:
		newStatus = deployment.StatusCreating
		reason = "Instance is being created in cluster"
	case models.HealthDegraded:
		newStatus = deployment.StatusDegraded
		reason = "Instance is degraded in cluster"
	case models.HealthUnhealthy:
		newStatus = deployment.StatusFailed
		reason = "Instance is unhealthy in cluster"
	default:
		newStatus = deployment.StatusCreating
		reason = "Instance appeared in cluster, status unknown"
	}

	// Update status if changed
	if pending.Status != newStatus {
		oldStatus := pending.Status
		pending.Status = newStatus
		pending.StatusHistory = append(pending.StatusHistory, StatusTransition{
			FromStatus: oldStatus,
			ToStatus:   newStatus,
			Timestamp:  time.Now(),
			Reason:     reason,
		})

		m.logger.Info("pending instance status transitioned",
			"instanceId", instanceID,
			"fromStatus", oldStatus,
			"toStatus", newStatus)

		// Notify outside the lock
		go m.notifyStatusChange(pending)
	}

	// If instance is ready, remove from pending
	if newStatus == deployment.StatusReady {
		delete(m.pendingInstances, instanceID)
		m.logger.Info("instance synced successfully, removed from pending",
			"instanceId", instanceID,
			"name", instance.Name)
	}
}

// correlateByNamespaceName correlates by matching namespace and name
func (m *GitOpsSyncMonitor) correlateByNamespaceName(namespace, name string, instance *models.Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, pending := range m.pendingInstances {
		if pending.Namespace == namespace && pending.Name == name {
			m.logger.Info("correlated cluster instance by namespace/name",
				"instanceId", id,
				"name", name,
				"namespace", namespace)

			// Determine new status
			var newStatus deployment.InstanceStatus
			var reason string

			switch instance.Health {
			case models.HealthHealthy:
				newStatus = deployment.StatusReady
				reason = "Instance synced and healthy"
			case models.HealthProgressing:
				newStatus = deployment.StatusCreating
				reason = "Instance syncing from Git"
			default:
				newStatus = deployment.StatusCreating
				reason = "Instance appeared in cluster"
			}

			if pending.Status != newStatus {
				oldStatus := pending.Status
				pending.Status = newStatus
				pending.StatusHistory = append(pending.StatusHistory, StatusTransition{
					FromStatus: oldStatus,
					ToStatus:   newStatus,
					Timestamp:  time.Now(),
					Reason:     reason,
				})

				go m.notifyStatusChange(pending)
			}

			if newStatus == deployment.StatusReady {
				delete(m.pendingInstances, id)
			}
			return
		}
	}
}

// runStuckInstanceChecker periodically checks for stuck instances
func (m *GitOpsSyncMonitor) runStuckInstanceChecker(ctx context.Context) {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkForStuckInstances()
		}
	}
}

// checkForStuckInstances identifies instances stuck in PushedToGit state
func (m *GitOpsSyncMonitor) checkForStuckInstances() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	threshold := m.config.StuckThreshold

	for id, pending := range m.pendingInstances {
		// Only check instances in PushedToGit or WaitingForSync status
		if pending.Status != deployment.StatusPushedToGit && pending.Status != deployment.StatusWaitingForSync {
			continue
		}

		duration := now.Sub(pending.PushedAt)
		if duration > threshold {
			m.logger.Warn("instance stuck in GitOps sync",
				"instanceId", id,
				"name", pending.Name,
				"namespace", pending.Namespace,
				"status", pending.Status,
				"stuckDuration", duration.String(),
				"threshold", threshold.String())

			// Call stuck callback if configured
			if m.config.OnStuckInstance != nil {
				go m.config.OnStuckInstance(pending, duration)
			}
		}
	}
}

// GetStuckInstances returns all instances that have been stuck longer than the threshold
func (m *GitOpsSyncMonitor) GetStuckInstances() []*PendingGitOpsInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	threshold := m.config.StuckThreshold
	var stuck []*PendingGitOpsInstance

	for _, pending := range m.pendingInstances {
		if pending.Status != deployment.StatusPushedToGit && pending.Status != deployment.StatusWaitingForSync {
			continue
		}

		if now.Sub(pending.PushedAt) > threshold {
			copy := *pending
			stuck = append(stuck, &copy)
		}
	}

	return stuck
}

// GetStatusTimeline returns the status timeline for an instance
func (m *GitOpsSyncMonitor) GetStatusTimeline(instanceID string) []StatusTransition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pending, exists := m.pendingInstances[instanceID]
	if !exists {
		return nil
	}

	result := make([]StatusTransition, len(pending.StatusHistory))
	copy(result, pending.StatusHistory)
	return result
}

// notifyStatusChange calls the status change callback with panic recovery
func (m *GitOpsSyncMonitor) notifyStatusChange(instance *PendingGitOpsInstance) {
	if m.onStatusChange == nil {
		return
	}

	// Use panic recovery to prevent goroutine leaks
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("panic in status change callback",
				"panic", r,
				"instanceId", instance.InstanceID)
		}
	}()

	m.onStatusChange(instance)
}

// PendingInstanceCount returns the number of pending instances
func (m *GitOpsSyncMonitor) PendingInstanceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.pendingInstances)
}

// IsRunning returns whether the monitor is running
func (m *GitOpsSyncMonitor) IsRunning() bool {
	return m.running
}
