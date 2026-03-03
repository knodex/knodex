package history

import (
	"context"
	"testing"
	"time"

	"github.com/provops-org/knodex/server/internal/models"
)

func TestNewService(t *testing.T) {
	t.Parallel()

	// Test creating service without Redis (in-memory mode)
	svc := NewService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.inMemoryCache == nil {
		t.Error("expected inMemoryCache to be initialized")
	}
}

func TestRecordEvent(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Record an event
	event := models.DeploymentEvent{
		EventType:      models.EventTypeCreated,
		Status:         "Pending",
		User:           "test-user",
		DeploymentMode: models.DeploymentModeDirect,
		Message:        "Test event",
	}

	err := svc.RecordEvent(ctx, "test-ns", "TestKind", "test-instance", event)
	if err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	// Retrieve history
	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if history == nil {
		t.Fatal("expected non-nil history")
	}
	if len(history.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(history.Events))
	}
	if history.Events[0].EventType != models.EventTypeCreated {
		t.Errorf("expected EventTypeCreated, got %s", history.Events[0].EventType)
	}
	if history.Events[0].ID == "" {
		t.Error("expected event ID to be generated")
	}
	if history.Events[0].Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestRecordCreation(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user@example.com", models.DeploymentModeDirect)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if history.RGDName != "test-rgd" {
		t.Errorf("expected RGDName 'test-rgd', got '%s'", history.RGDName)
	}
	if history.DeploymentMode != models.DeploymentModeDirect {
		t.Errorf("expected DeploymentModeDirect, got '%s'", history.DeploymentMode)
	}
	if len(history.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(history.Events))
	}
	if history.Events[0].EventType != models.EventTypeCreated {
		t.Errorf("expected EventTypeCreated, got %s", history.Events[0].EventType)
	}
	if history.Events[0].User != "test-user@example.com" {
		t.Errorf("expected user 'test-user@example.com', got '%s'", history.Events[0].User)
	}
}

func TestRecordGitPush(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// First create the instance
	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user", models.DeploymentModeGitOps)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	// Record git push
	err = svc.RecordGitPush(ctx, "test-ns", "TestKind", "test-instance", "abc123def", "https://github.com/org/repo", "main", "test-user")
	if err != nil {
		t.Fatalf("RecordGitPush failed: %v", err)
	}

	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(history.Events))
	}

	pushEvent := history.Events[1]
	if pushEvent.EventType != models.EventTypePushedToGit {
		t.Errorf("expected EventTypePushedToGit, got %s", pushEvent.EventType)
	}
	if pushEvent.GitCommitSHA != "abc123def" {
		t.Errorf("expected GitCommitSHA 'abc123def', got '%s'", pushEvent.GitCommitSHA)
	}
	if pushEvent.GitRepository != "https://github.com/org/repo" {
		t.Errorf("expected GitRepository 'https://github.com/org/repo', got '%s'", pushEvent.GitRepository)
	}
	if pushEvent.GitBranch != "main" {
		t.Errorf("expected GitBranch 'main', got '%s'", pushEvent.GitBranch)
	}
	if history.LastGitCommit != "abc123def" {
		t.Errorf("expected LastGitCommit 'abc123def', got '%s'", history.LastGitCommit)
	}
}

func TestRecordStatusChange(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Create instance first
	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user", models.DeploymentModeDirect)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	// Record status change
	err = svc.RecordStatusChange(ctx, "test-ns", "TestKind", "test-instance", "Pending", "Ready")
	if err != nil {
		t.Fatalf("RecordStatusChange failed: %v", err)
	}

	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(history.Events))
	}

	statusEvent := history.Events[1]
	if statusEvent.EventType != models.EventTypeReady {
		t.Errorf("expected EventTypeReady, got %s", statusEvent.EventType)
	}
	if statusEvent.Status != "Ready" {
		t.Errorf("expected status 'Ready', got '%s'", statusEvent.Status)
	}
	if history.CurrentStatus != "Ready" {
		t.Errorf("expected CurrentStatus 'Ready', got '%s'", history.CurrentStatus)
	}
}

func TestRecordDeletion(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Create instance first
	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user", models.DeploymentModeDirect)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	// Delete instance
	err = svc.RecordDeletion(ctx, "test-ns", "TestKind", "test-instance", "admin-user")
	if err != nil {
		t.Fatalf("RecordDeletion failed: %v", err)
	}

	// Original history should be gone
	_, err = svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err == nil {
		t.Error("expected error getting history after deletion")
	}

	// Deleted history should be available
	deletedHistory, err := svc.GetDeletedHistory(ctx, "test-ns/TestKind/test-instance")
	if err != nil {
		t.Fatalf("GetDeletedHistory failed: %v", err)
	}

	if deletedHistory == nil {
		t.Fatal("expected non-nil deleted history")
	}
	if deletedHistory.CurrentStatus != "Deleted" {
		t.Errorf("expected CurrentStatus 'Deleted', got '%s'", deletedHistory.CurrentStatus)
	}
	if len(deletedHistory.Events) != 2 {
		t.Errorf("expected 2 events (Created + Deleted), got %d", len(deletedHistory.Events))
	}
	if deletedHistory.Events[1].EventType != models.EventTypeDeleted {
		t.Errorf("expected last event to be EventTypeDeleted, got %s", deletedHistory.Events[1].EventType)
	}
}

func TestRecordDeletion_CleansUpKeyWhenHistoryMissing(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Manually insert a key into the in-memory cache to simulate an orphaned key
	key := historyKey("orphan-ns", "OrphanKind", "orphan-instance")
	svc.mu.Lock()
	svc.inMemoryCache[key] = &models.DeploymentHistory{
		InstanceID:   "orphan-ns/OrphanKind/orphan-instance",
		InstanceName: "orphan-instance",
		Namespace:    "orphan-ns",
	}
	svc.mu.Unlock()

	// Verify the key exists
	svc.mu.RLock()
	_, exists := svc.inMemoryCache[key]
	svc.mu.RUnlock()
	if !exists {
		t.Fatal("expected key to exist before test")
	}

	// Delete the in-memory cache entry for the history key so GetHistory will fail,
	// but leave a different key format to simulate the orphan scenario
	svc.mu.Lock()
	delete(svc.inMemoryCache, key)
	svc.mu.Unlock()

	// Now put back just the raw key (not through RecordCreation) to simulate orphaned state
	svc.mu.Lock()
	svc.inMemoryCache[key] = &models.DeploymentHistory{
		InstanceID: "orphan-ns/OrphanKind/orphan-instance",
	}
	svc.mu.Unlock()

	// RecordDeletion should succeed even though it creates proper history
	// The important thing is: after RecordDeletion the active key is removed
	err := svc.RecordDeletion(ctx, "orphan-ns", "OrphanKind", "orphan-instance", "admin")
	if err != nil {
		t.Fatalf("RecordDeletion failed: %v", err)
	}

	// Active key should be cleaned up
	_, err = svc.GetHistory(ctx, "orphan-ns", "OrphanKind", "orphan-instance")
	if err == nil {
		t.Error("expected active history key to be cleaned up after deletion")
	}
}

func TestRecordDeletion_NoHistory_CleansActiveKey(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Call RecordDeletion for an instance that never had history recorded.
	// This should not error and should still attempt to clean up any active key.
	err := svc.RecordDeletion(ctx, "no-history-ns", "SomeKind", "no-history-instance", "admin")
	if err != nil {
		t.Fatalf("RecordDeletion should not fail for missing history: %v", err)
	}

	// Verify the active key does not exist (it shouldn't have been created)
	_, err = svc.GetHistory(ctx, "no-history-ns", "SomeKind", "no-history-instance")
	if err == nil {
		t.Error("expected no active history to exist")
	}
}

func TestGetTimeline(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Create instance with multiple events
	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user", models.DeploymentModeGitOps)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	err = svc.RecordGitPush(ctx, "test-ns", "TestKind", "test-instance", "abc123", "https://github.com/org/repo", "main", "test-user")
	if err != nil {
		t.Fatalf("RecordGitPush failed: %v", err)
	}

	err = svc.RecordStatusChange(ctx, "test-ns", "TestKind", "test-instance", "PushedToGit", "Ready")
	if err != nil {
		t.Fatalf("RecordStatusChange failed: %v", err)
	}

	timeline, err := svc.GetTimeline(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetTimeline failed: %v", err)
	}

	if len(timeline) != 3 {
		t.Errorf("expected 3 timeline entries, got %d", len(timeline))
	}

	// Check first entry (Created)
	if timeline[0].EventType != models.EventTypeCreated {
		t.Errorf("expected first event to be Created, got %s", timeline[0].EventType)
	}
	if timeline[0].IsCompleted != true {
		t.Error("expected IsCompleted to be true")
	}
	if timeline[0].IsCurrent != false {
		t.Error("expected IsCurrent to be false for first event")
	}

	// Check last entry (Ready) is marked as current
	if timeline[2].IsCurrent != true {
		t.Error("expected last event to be marked as current")
	}
}

func TestMaxEventsPerInstance(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Create instance
	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user", models.DeploymentModeDirect)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	// Record many events (more than maxEventsPerInstance)
	for i := 0; i < 1100; i++ {
		event := models.DeploymentEvent{
			EventType: models.EventTypeStatusChanged,
			Status:    "Status" + string(rune(i%10)),
			Message:   "Event " + string(rune(i)),
		}
		err := svc.RecordEvent(ctx, "test-ns", "TestKind", "test-instance", event)
		if err != nil {
			t.Fatalf("RecordEvent failed at iteration %d: %v", i, err)
		}
	}

	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	// Should be limited to maxEventsPerInstance
	if len(history.Events) > maxEventsPerInstance {
		t.Errorf("expected at most %d events, got %d", maxEventsPerInstance, len(history.Events))
	}
}

func TestCreateHistoryFromInstance(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	instance := &models.Instance{
		UID:       "test-uid-123",
		Name:      "test-instance",
		Namespace: "test-ns",
		Kind:      "TestKind",
		RGDName:   "test-rgd",
		Health:    models.HealthHealthy,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now(),
	}

	err := svc.CreateHistoryFromInstance(ctx, instance, "migration-user")
	if err != nil {
		t.Fatalf("CreateHistoryFromInstance failed: %v", err)
	}

	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	// Should have Created event + Ready event (since Health is Healthy)
	if len(history.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(history.Events))
	}

	if history.InstanceID != "test-uid-123" {
		t.Errorf("expected InstanceID 'test-uid-123', got '%s'", history.InstanceID)
	}
	if history.CurrentStatus != "Ready" {
		t.Errorf("expected CurrentStatus 'Ready', got '%s'", history.CurrentStatus)
	}
}

func TestListAllHistories(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Create multiple instances
	err := svc.RecordCreation(ctx, "ns1", "Kind1", "instance1", "rgd1", "user1", models.DeploymentModeDirect)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	err = svc.RecordCreation(ctx, "ns2", "Kind2", "instance2", "rgd2", "user2", models.DeploymentModeGitOps)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	histories, err := svc.ListAllHistories(ctx)
	if err != nil {
		t.Fatalf("ListAllHistories failed: %v", err)
	}

	if len(histories) != 2 {
		t.Errorf("expected 2 histories, got %d", len(histories))
	}
}

func TestEventsSortedByTimestamp(t *testing.T) {
	t.Parallel()

	svc := NewService(nil)
	ctx := context.Background()

	// Create history first
	err := svc.RecordCreation(ctx, "test-ns", "TestKind", "test-instance", "test-rgd", "test-user", models.DeploymentModeDirect)
	if err != nil {
		t.Fatalf("RecordCreation failed: %v", err)
	}

	// Add events with explicit timestamps to test sorting
	now := time.Now()
	events := []models.DeploymentEvent{
		{
			Timestamp: now.Add(-30 * time.Minute),
			EventType: models.EventTypeCreating,
			Status:    "Creating",
			Message:   "Second event",
		},
		{
			Timestamp: now.Add(-10 * time.Minute),
			EventType: models.EventTypeReady,
			Status:    "Ready",
			Message:   "Third event",
		},
	}

	for _, e := range events {
		err := svc.RecordEvent(ctx, "test-ns", "TestKind", "test-instance", e)
		if err != nil {
			t.Fatalf("RecordEvent failed: %v", err)
		}
	}

	history, err := svc.GetHistory(ctx, "test-ns", "TestKind", "test-instance")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	// Events should be sorted by timestamp
	for i := 1; i < len(history.Events); i++ {
		if history.Events[i].Timestamp.Before(history.Events[i-1].Timestamp) {
			t.Errorf("events not sorted by timestamp: event %d (%s) is before event %d (%s)",
				i, history.Events[i].Timestamp,
				i-1, history.Events[i-1].Timestamp)
		}
	}
}
