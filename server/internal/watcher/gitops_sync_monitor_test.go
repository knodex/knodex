// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/deployment"
	krowatcher "github.com/knodex/knodex/server/internal/kro/watcher"
	"github.com/knodex/knodex/server/internal/models"
)

func TestNewGitOpsSyncMonitor(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	if monitor == nil {
		t.Fatal("expected non-nil monitor")
	}
	if monitor.pendingInstances == nil {
		t.Error("expected pendingInstances map to be initialized")
	}
	if monitor.running {
		t.Error("expected monitor to not be running initially")
	}
}

func TestTrackPushedInstance(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "test-instance-id-123"
	name := "my-app"
	namespace := "default"
	rgdName := "web-service"
	rgdNamespace := "kro-system"
	mode := deployment.ModeGitOps
	projID := "proj-123"

	gitInfo := &deployment.GitInfo{
		Branch:     "main",
		CommitSHA:  "abc123",
		PushStatus: deployment.GitPushSuccess,
		PushedAt:   time.Now().Format(time.RFC3339),
	}

	monitor.TrackPushedInstance(instanceID, name, namespace, rgdName, rgdNamespace, mode, gitInfo, projID)

	// Verify instance was tracked
	pending, exists := monitor.GetPendingInstance(instanceID)
	if !exists {
		t.Fatal("expected pending instance to exist")
	}

	if pending.InstanceID != instanceID {
		t.Errorf("expected instanceID %s, got %s", instanceID, pending.InstanceID)
	}
	if pending.Name != name {
		t.Errorf("expected name %s, got %s", name, pending.Name)
	}
	if pending.Namespace != namespace {
		t.Errorf("expected namespace %s, got %s", namespace, pending.Namespace)
	}
	if pending.RGDName != rgdName {
		t.Errorf("expected rgdName %s, got %s", rgdName, pending.RGDName)
	}
	if pending.Status != deployment.StatusPushedToGit {
		t.Errorf("expected status %s, got %s", deployment.StatusPushedToGit, pending.Status)
	}
	if pending.DeploymentMode != mode {
		t.Errorf("expected deploymentMode %s, got %s", mode, pending.DeploymentMode)
	}
	if pending.ProjectID != projID {
		t.Errorf("expected projectID %s, got %s", projID, pending.ProjectID)
	}

	// Verify status history
	if len(pending.StatusHistory) != 1 {
		t.Fatalf("expected 1 status history entry, got %d", len(pending.StatusHistory))
	}
	if pending.StatusHistory[0].ToStatus != deployment.StatusPushedToGit {
		t.Errorf("expected initial status %s, got %s", deployment.StatusPushedToGit, pending.StatusHistory[0].ToStatus)
	}
}

func TestUpdateStatus(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "test-instance-update"
	monitor.TrackPushedInstance(instanceID, "app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Update status to Creating
	updated := monitor.UpdateStatus(instanceID, deployment.StatusCreating, "Instance being created")
	if !updated {
		t.Error("expected status update to succeed")
	}

	pending, _ := monitor.GetPendingInstance(instanceID)
	if pending.Status != deployment.StatusCreating {
		t.Errorf("expected status %s, got %s", deployment.StatusCreating, pending.Status)
	}

	// Verify status history has 2 entries
	if len(pending.StatusHistory) != 2 {
		t.Fatalf("expected 2 status history entries, got %d", len(pending.StatusHistory))
	}
	if pending.StatusHistory[1].FromStatus != deployment.StatusPushedToGit {
		t.Errorf("expected fromStatus %s, got %s", deployment.StatusPushedToGit, pending.StatusHistory[1].FromStatus)
	}
	if pending.StatusHistory[1].ToStatus != deployment.StatusCreating {
		t.Errorf("expected toStatus %s, got %s", deployment.StatusCreating, pending.StatusHistory[1].ToStatus)
	}

	// Update with same status should return false
	updated = monitor.UpdateStatus(instanceID, deployment.StatusCreating, "Same status")
	if updated {
		t.Error("expected status update with same status to return false")
	}

	// Update non-existent instance
	updated = monitor.UpdateStatus("non-existent", deployment.StatusReady, "")
	if updated {
		t.Error("expected status update for non-existent instance to return false")
	}
}

func TestGetPendingByNamespace(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	// Track multiple instances
	monitor.TrackPushedInstance("id-1", "app-1", "ns-1", "rgd", "", deployment.ModeGitOps, nil, "")
	monitor.TrackPushedInstance("id-2", "app-2", "ns-1", "rgd", "", deployment.ModeGitOps, nil, "")
	monitor.TrackPushedInstance("id-3", "app-1", "ns-2", "rgd", "", deployment.ModeGitOps, nil, "")

	// Find by namespace and name
	pending, found := monitor.GetPendingByNamespace("ns-1", "app-1")
	if !found {
		t.Fatal("expected to find pending instance")
	}
	if pending.Name != "app-1" || pending.Namespace != "ns-1" {
		t.Errorf("expected app-1/ns-1, got %s/%s", pending.Name, pending.Namespace)
	}

	// Not found
	_, found = monitor.GetPendingByNamespace("ns-1", "non-existent")
	if found {
		t.Error("expected not to find non-existent instance")
	}
}

func TestGetAllPending(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	// Initially empty
	all := monitor.GetAllPending()
	if len(all) != 0 {
		t.Errorf("expected 0 pending instances, got %d", len(all))
	}

	// Add some instances
	monitor.TrackPushedInstance("id-1", "app-1", "ns-1", "rgd", "", deployment.ModeGitOps, nil, "")
	monitor.TrackPushedInstance("id-2", "app-2", "ns-1", "rgd", "", deployment.ModeHybrid, nil, "")

	all = monitor.GetAllPending()
	if len(all) != 2 {
		t.Errorf("expected 2 pending instances, got %d", len(all))
	}

	// Verify ListPending alias works
	list := monitor.ListPending()
	if len(list) != 2 {
		t.Errorf("expected 2 from ListPending, got %d", len(list))
	}
}

func TestRemovePendingInstance(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "test-remove"
	monitor.TrackPushedInstance(instanceID, "app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Verify exists
	if monitor.PendingInstanceCount() != 1 {
		t.Error("expected 1 pending instance")
	}

	// Remove
	removed := monitor.RemovePendingInstance(instanceID)
	if !removed {
		t.Error("expected remove to succeed")
	}

	if monitor.PendingInstanceCount() != 0 {
		t.Error("expected 0 pending instances after removal")
	}

	// Remove again should return false
	removed = monitor.RemovePendingInstance(instanceID)
	if removed {
		t.Error("expected remove of already-removed instance to return false")
	}
}

func TestGetStuckInstances(t *testing.T) {
	t.Parallel()

	// Use very short threshold for testing
	config := GitOpsSyncMonitorConfig{
		StuckThreshold: 100 * time.Millisecond,
		CheckInterval:  50 * time.Millisecond,
	}
	monitor := NewGitOpsSyncMonitor(nil, config)

	// Track instances with PushedAt in the past
	pastTime := time.Now().Add(-200 * time.Millisecond).Format(time.RFC3339)
	gitInfo := &deployment.GitInfo{
		PushedAt: pastTime,
	}

	monitor.TrackPushedInstance("stuck-1", "app-stuck", "default", "rgd", "", deployment.ModeGitOps, gitInfo, "")

	// Track recent instance
	monitor.TrackPushedInstance("recent-1", "app-recent", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Get stuck instances
	stuck := monitor.GetStuckInstances()

	if len(stuck) != 1 {
		t.Errorf("expected 1 stuck instance, got %d", len(stuck))
	}
	if len(stuck) > 0 && stuck[0].InstanceID != "stuck-1" {
		t.Errorf("expected stuck instance stuck-1, got %s", stuck[0].InstanceID)
	}
}

func TestStuckInstanceCallback(t *testing.T) {
	t.Parallel()

	var callbackCount atomic.Int32

	config := GitOpsSyncMonitorConfig{
		StuckThreshold: 50 * time.Millisecond,
		CheckInterval:  25 * time.Millisecond,
		OnStuckInstance: func(instance *PendingGitOpsInstance, duration time.Duration) {
			callbackCount.Add(1)
		},
	}
	monitor := NewGitOpsSyncMonitor(nil, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitor
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Track an instance that's already stuck
	pastTime := time.Now().Add(-100 * time.Millisecond).Format(time.RFC3339)
	gitInfo := &deployment.GitInfo{PushedAt: pastTime}
	monitor.TrackPushedInstance("stuck-callback", "app", "default", "rgd", "", deployment.ModeGitOps, gitInfo, "")

	// Wait for callback to be triggered
	time.Sleep(100 * time.Millisecond)

	if callbackCount.Load() == 0 {
		t.Error("expected stuck callback to be called at least once")
	}
}

func TestGetStatusTimeline(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "timeline-test"
	monitor.TrackPushedInstance(instanceID, "app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Get initial timeline
	timeline := monitor.GetStatusTimeline(instanceID)
	if len(timeline) != 1 {
		t.Fatalf("expected 1 timeline entry, got %d", len(timeline))
	}
	if timeline[0].ToStatus != deployment.StatusPushedToGit {
		t.Errorf("expected initial status %s, got %s", deployment.StatusPushedToGit, timeline[0].ToStatus)
	}

	// Update status and check timeline grows
	monitor.UpdateStatus(instanceID, deployment.StatusCreating, "Creating")
	monitor.UpdateStatus(instanceID, deployment.StatusReady, "Ready")

	timeline = monitor.GetStatusTimeline(instanceID)
	if len(timeline) != 3 {
		t.Fatalf("expected 3 timeline entries, got %d", len(timeline))
	}

	// Verify status transitions
	expectedStatuses := []deployment.InstanceStatus{
		deployment.StatusPushedToGit,
		deployment.StatusCreating,
		deployment.StatusReady,
	}
	for i, status := range expectedStatuses {
		if timeline[i].ToStatus != status {
			t.Errorf("timeline[%d]: expected %s, got %s", i, status, timeline[i].ToStatus)
		}
	}

	// Non-existent instance should return nil
	nilTimeline := monitor.GetStatusTimeline("non-existent")
	if nilTimeline != nil {
		t.Error("expected nil timeline for non-existent instance")
	}
}

func TestHandleInstanceUpdate_WithAnnotation(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "annotation-test"
	monitor.TrackPushedInstance(instanceID, "app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Simulate cluster instance update with annotation
	clusterInstance := &models.Instance{
		Name:      "app",
		Namespace: "default",
		Health:    models.HealthHealthy,
		Annotations: map[string]string{
			InstanceIDAnnotation: instanceID,
		},
	}

	monitor.handleInstanceUpdate(krowatcher.InstanceActionAdd, "default", "TestKind", "app", clusterInstance)

	// Instance should be removed from pending (now Ready)
	_, exists := monitor.GetPendingInstance(instanceID)
	if exists {
		t.Error("expected instance to be removed from pending after becoming ready")
	}
}

func TestHandleInstanceUpdate_WithProgressingHealth(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "progressing-test"
	monitor.TrackPushedInstance(instanceID, "app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Simulate cluster instance that's still progressing
	clusterInstance := &models.Instance{
		Name:      "app",
		Namespace: "default",
		Health:    models.HealthProgressing,
		Annotations: map[string]string{
			InstanceIDAnnotation: instanceID,
		},
	}

	monitor.handleInstanceUpdate(krowatcher.InstanceActionUpdate, "default", "TestKind", "app", clusterInstance)

	// Instance should still be pending but with updated status
	pending, exists := monitor.GetPendingInstance(instanceID)
	if !exists {
		t.Fatal("expected instance to still be pending")
	}
	if pending.Status != deployment.StatusCreating {
		t.Errorf("expected status %s, got %s", deployment.StatusCreating, pending.Status)
	}
}

func TestHandleInstanceUpdate_ByNamespace(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "namespace-match"
	monitor.TrackPushedInstance(instanceID, "my-app", "prod", "rgd", "", deployment.ModeGitOps, nil, "")

	// Simulate cluster instance without annotation (fallback to namespace/name)
	clusterInstance := &models.Instance{
		Name:      "my-app",
		Namespace: "prod",
		Health:    models.HealthHealthy,
	}

	monitor.handleInstanceUpdate(krowatcher.InstanceActionAdd, "prod", "TestKind", "my-app", clusterInstance)

	// Instance should be removed from pending
	_, exists := monitor.GetPendingInstance(instanceID)
	if exists {
		t.Error("expected instance to be removed from pending after namespace/name correlation")
	}
}

func TestHandleInstanceUpdate_Delete(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	instanceID := "delete-test"
	monitor.TrackPushedInstance(instanceID, "delete-app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	if monitor.PendingInstanceCount() != 1 {
		t.Fatal("expected 1 pending instance")
	}

	// Simulate delete event
	monitor.handleInstanceUpdate(krowatcher.InstanceActionDelete, "default", "TestKind", "delete-app", nil)

	if monitor.PendingInstanceCount() != 0 {
		t.Error("expected 0 pending instances after delete")
	}
}

func TestOnStatusChangeCallback(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	var callbackInstance *PendingGitOpsInstance
	monitor.SetOnStatusChangeCallback(func(instance *PendingGitOpsInstance) {
		callbackInstance = instance
	})

	instanceID := "callback-test"
	monitor.TrackPushedInstance(instanceID, "app", "default", "rgd", "", deployment.ModeGitOps, nil, "")

	// Callback should have been called
	if callbackInstance == nil {
		t.Fatal("expected callback to be called")
	}
	if callbackInstance.InstanceID != instanceID {
		t.Errorf("expected instanceID %s, got %s", instanceID, callbackInstance.InstanceID)
	}

	// Update status and verify callback
	callbackInstance = nil
	monitor.UpdateStatus(instanceID, deployment.StatusCreating, "Creating")

	if callbackInstance == nil {
		t.Fatal("expected callback to be called on status update")
	}
	if callbackInstance.Status != deployment.StatusCreating {
		t.Errorf("expected status %s in callback, got %s", deployment.StatusCreating, callbackInstance.Status)
	}
}

func TestStartStop(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	if monitor.IsRunning() {
		t.Error("expected monitor to not be running initially")
	}

	ctx := context.Background()
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !monitor.IsRunning() {
		t.Error("expected monitor to be running after start")
	}

	// Starting again should be idempotent
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("second start failed: %v", err)
	}

	monitor.Stop()

	if monitor.IsRunning() {
		t.Error("expected monitor to not be running after stop")
	}

	// Stopping again should be safe
	monitor.Stop()
}

func TestPendingInstanceCount(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	if monitor.PendingInstanceCount() != 0 {
		t.Error("expected 0 initially")
	}

	monitor.TrackPushedInstance("id-1", "app-1", "ns", "rgd", "", deployment.ModeGitOps, nil, "")
	if monitor.PendingInstanceCount() != 1 {
		t.Error("expected 1 after adding")
	}

	monitor.TrackPushedInstance("id-2", "app-2", "ns", "rgd", "", deployment.ModeGitOps, nil, "")
	if monitor.PendingInstanceCount() != 2 {
		t.Error("expected 2 after adding second")
	}

	monitor.RemovePendingInstance("id-1")
	if monitor.PendingInstanceCount() != 1 {
		t.Error("expected 1 after removal")
	}
}

func TestDefaultGitOpsSyncMonitorConfig(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()

	if config.StuckThreshold != DefaultStuckThreshold {
		t.Errorf("expected stuck threshold %v, got %v", DefaultStuckThreshold, config.StuckThreshold)
	}
	if config.CheckInterval != DefaultCheckInterval {
		t.Errorf("expected check interval %v, got %v", DefaultCheckInterval, config.CheckInterval)
	}
	if config.OnStuckInstance != nil {
		t.Error("expected nil OnStuckInstance callback by default")
	}
}

func TestGetPendingInstanceReturnsCopy(t *testing.T) {
	t.Parallel()

	config := DefaultGitOpsSyncMonitorConfig()
	monitor := NewGitOpsSyncMonitor(nil, config)

	gitInfo := &deployment.GitInfo{
		Branch:    "main",
		CommitSHA: "abc123",
	}
	monitor.TrackPushedInstance("copy-test", "app", "ns", "rgd", "", deployment.ModeGitOps, gitInfo, "")

	// Get instance and modify it
	pending1, _ := monitor.GetPendingInstance("copy-test")
	pending1.Name = "modified"
	pending1.GitInfo.Branch = "modified-branch"

	// Get again and verify original is unchanged
	pending2, _ := monitor.GetPendingInstance("copy-test")
	if pending2.Name != "app" {
		t.Error("expected name to be unchanged (got a copy)")
	}
	if pending2.GitInfo.Branch != "main" {
		t.Error("expected branch to be unchanged (got a copy)")
	}
}
