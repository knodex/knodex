package rbac

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockPolicySyncer implements PolicySyncer for testing
type mockPolicySyncer struct {
	mu              sync.Mutex
	syncCount       int
	backgroundSyncs int64
	syncError       error
	syncDelay       time.Duration
}

func (m *mockPolicySyncer) SyncPolicies(ctx context.Context) error {
	if m.syncDelay > 0 {
		select {
		case <-time.After(m.syncDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncCount++
	return m.syncError
}

func (m *mockPolicySyncer) IncrementBackgroundSyncs() {
	atomic.AddInt64(&m.backgroundSyncs, 1)
}

func (m *mockPolicySyncer) getSyncCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncCount
}

func (m *mockPolicySyncer) getBackgroundSyncs() int64 {
	return atomic.LoadInt64(&m.backgroundSyncs)
}

func (m *mockPolicySyncer) setSyncError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncError = err
}

func TestNewPolicySyncService(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	if service == nil {
		t.Fatal("expected service to be created")
	}

	if service.IsRunning() {
		t.Error("expected service to not be running initially")
	}
}

func TestPolicySyncService_DefaultConfig(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 0, // Should default to 10 minutes
		Logger:       nil,
	}

	service := NewPolicySyncService(syncer, config)
	ps := service.(*policySyncService)

	if ps.config.SyncInterval != 10*time.Minute {
		t.Errorf("expected default sync interval 10m, got %v", ps.config.SyncInterval)
	}

	if ps.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestPolicySyncService_CustomConfig(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 5 * time.Minute,
	}

	service := NewPolicySyncService(syncer, config)
	ps := service.(*policySyncService)

	if ps.config.SyncInterval != 5*time.Minute {
		t.Errorf("expected sync interval 5m, got %v", ps.config.SyncInterval)
	}
}

func TestPolicySyncService_IsRunning(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	if service.IsRunning() {
		t.Error("expected service to not be running initially")
	}
}

func TestPolicySyncService_StopWhenNotRunning(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	// Should not panic when stopping a non-running service
	service.Stop()

	if service.IsRunning() {
		t.Error("expected service to remain stopped")
	}
}

func TestPolicySyncService_LastSyncTime(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	if !service.LastSyncTime().IsZero() {
		t.Error("expected last sync time to be zero initially")
	}
}

func TestPolicySyncService_LastSyncError(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	if service.LastSyncError() != nil {
		t.Error("expected last sync error to be nil initially")
	}
}

func TestPolicySyncService_StartAndStop(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 100 * time.Millisecond, // Short interval for testing
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start in background
	go func() {
		service.Start(ctx)
	}()

	// Wait for initial sync
	time.Sleep(50 * time.Millisecond)

	if !service.IsRunning() {
		t.Error("expected service to be running")
	}

	// Stop the service
	service.Stop()

	// Wait for service to stop
	time.Sleep(50 * time.Millisecond)

	if service.IsRunning() {
		t.Error("expected service to be stopped")
	}

	// Verify initial sync occurred
	if syncer.getSyncCount() < 1 {
		t.Error("expected at least one sync to have occurred")
	}
}

func TestPolicySyncService_InitialSync(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 1 * time.Hour, // Long interval - we only test initial sync
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())

	// Start in background
	go func() {
		service.Start(ctx)
	}()

	// Wait for initial sync
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop
	cancel()

	// Verify initial sync occurred
	if syncer.getSyncCount() != 1 {
		t.Errorf("expected 1 sync (initial), got %d", syncer.getSyncCount())
	}
}

func TestPolicySyncService_SyncError(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	syncer.setSyncError(errors.New("sync failed"))

	config := PolicySyncConfig{
		SyncInterval: 1 * time.Hour,
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())

	// Start in background
	go func() {
		service.Start(ctx)
	}()

	// Wait for initial sync
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop
	cancel()

	// Verify error is recorded
	if service.LastSyncError() == nil {
		t.Error("expected last sync error to be set")
	}

	// Background syncs should not be incremented on error
	if syncer.getBackgroundSyncs() != 0 {
		t.Errorf("expected 0 background syncs after error, got %d", syncer.getBackgroundSyncs())
	}
}

func TestPolicySyncService_TriggerSync(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 1 * time.Hour, // Long interval
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start in background
	go func() {
		service.Start(ctx)
	}()

	// Wait for initial sync
	time.Sleep(50 * time.Millisecond)

	initialSyncCount := syncer.getSyncCount()

	// Trigger manual sync
	service.TriggerSync()

	// Wait for triggered sync
	time.Sleep(50 * time.Millisecond)

	if syncer.getSyncCount() <= initialSyncCount {
		t.Error("expected triggered sync to have occurred")
	}
}

func TestPolicySyncService_TriggerSyncBuffered(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	syncer.syncDelay = 100 * time.Millisecond // Slow sync

	config := PolicySyncConfig{
		SyncInterval: 1 * time.Hour,
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start in background
	go func() {
		service.Start(ctx)
	}()

	// Wait for service to start (but initial sync is still running due to delay)
	time.Sleep(50 * time.Millisecond)

	// Multiple triggers should not block (buffered channel)
	service.TriggerSync()
	service.TriggerSync()
	service.TriggerSync()

	// This should not block
}

func TestPolicySyncService_StartTwice(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 1 * time.Hour,
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start in background
	go func() {
		service.Start(ctx)
	}()

	// Wait for service to start
	time.Sleep(50 * time.Millisecond)

	// Second start should return immediately (already running)
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("unexpected error on second start: %v", err)
	}
}

func TestPolicySyncService_ContextCancellation(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 1 * time.Hour,
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		service.Start(ctx)
		close(done)
	}()

	// Wait for service to start
	time.Sleep(50 * time.Millisecond)

	if !service.IsRunning() {
		t.Error("expected service to be running")
	}

	// Cancel context
	cancel()

	// Wait for service to stop
	select {
	case <-done:
		// Service stopped
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for service to stop")
	}

	if service.IsRunning() {
		t.Error("expected service to be stopped after context cancellation")
	}
}

// Test PolicyCacheManager
func TestNewPolicyCacheManager(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}
}

func TestPolicyCacheManager_StartAndStop(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stop the manager
	manager.Stop()
}

func TestPolicyCacheManager_StartTwice(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// First start
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second start should return nil (already running)
	err = manager.Start(ctx)
	if err != nil {
		t.Errorf("unexpected error on second start: %v", err)
	}

	manager.Stop()
}

func TestPolicyCacheManager_StopWhenNotRunning(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	// Should not panic when stopping a non-running manager
	manager.Stop()
}

func TestPolicyCacheManager_Status(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	status := manager.Status()

	if !status.CacheEnabled {
		t.Error("expected cache enabled in status")
	}
}

// Test PolicyCacheStatus struct
func TestPolicyCacheStatus(t *testing.T) {
	t.Parallel()

	status := PolicyCacheStatus{
		CacheEnabled:    true,
		CacheStats:      CacheStats{Hits: 10, Misses: 5, HitRate: 66.67, Size: 100},
		WatcherRunning:  true,
		WatcherLastSync: time.Now(),
		SyncerRunning:   true,
		SyncerLastSync:  time.Now(),
		SyncerLastError: "test error",
		PolicyReloads:   5,
		BackgroundSyncs: 10,
		WatcherRestarts: 2,
	}

	if !status.CacheEnabled {
		t.Error("expected CacheEnabled to be true")
	}

	if status.CacheStats.Hits != 10 {
		t.Errorf("expected 10 hits, got %d", status.CacheStats.Hits)
	}

	if !status.WatcherRunning {
		t.Error("expected WatcherRunning to be true")
	}

	if status.SyncerLastError != "test error" {
		t.Errorf("expected 'test error', got %s", status.SyncerLastError)
	}
}

// Concurrent access tests
func TestPolicySyncService_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	syncer := &mockPolicySyncer{}
	config := PolicySyncConfig{
		SyncInterval: 100 * time.Millisecond,
	}
	service := NewPolicySyncService(syncer, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start service
	go func() {
		service.Start(ctx)
	}()

	// Wait for service to start
	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent reads and triggers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = service.IsRunning()
				_ = service.LastSyncTime()
				_ = service.LastSyncError()
				service.TriggerSync()
			}
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestPolicyCacheManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)
	defer manager.Stop()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent status reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = manager.Status()
			}
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

// Test IsPolicySynced / MarkSynced
func TestPolicyCacheManager_IsPolicySynced(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	// Should be false initially
	if manager.IsPolicySynced() {
		t.Error("expected IsPolicySynced to be false initially")
	}

	// Mark as synced
	manager.MarkSynced()

	// Should be true after MarkSynced
	if !manager.IsPolicySynced() {
		t.Error("expected IsPolicySynced to be true after MarkSynced")
	}
}

func TestPolicyCacheManager_IsPolicySynced_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	var wg sync.WaitGroup
	numGoroutines := 20

	// Half the goroutines read, half write
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		if i%2 == 0 {
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_ = manager.IsPolicySynced()
				}
			}()
		} else {
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					manager.MarkSynced()
				}
			}()
		}
	}

	wg.Wait()

	// After all goroutines finish, synced must be true
	if !manager.IsPolicySynced() {
		t.Error("expected IsPolicySynced to be true after concurrent MarkSynced calls")
	}
}

// Benchmark tests
func BenchmarkPolicySyncService_IsRunning(b *testing.B) {
	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.IsRunning()
	}
}

func BenchmarkPolicySyncService_LastSyncTime(b *testing.B) {
	syncer := &mockPolicySyncer{}
	service := NewPolicySyncService(syncer, PolicySyncConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.LastSyncTime()
	}
}

func BenchmarkPolicyCacheManager_Status(b *testing.B) {
	manager := NewPolicyCacheManager(nil, nil, nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.Status()
	}
}
