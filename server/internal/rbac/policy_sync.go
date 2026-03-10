// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	// DefaultPolicySyncInterval is how often to run background policy sync.
	// 10min catches any missed watch events; primary sync is event-driven.
	DefaultPolicySyncInterval = 10 * time.Minute
)

// PolicySyncService handles background policy synchronization
// It periodically syncs policies from Kubernetes even when the watcher
// is running, to ensure eventual consistency
type PolicySyncService interface {
	// Start begins background policy synchronization
	Start(ctx context.Context) error

	// Stop gracefully stops the sync service
	Stop()

	// IsRunning returns true if the service is running
	IsRunning() bool

	// TriggerSync triggers an immediate sync
	TriggerSync()

	// LastSyncTime returns the last successful sync time
	LastSyncTime() time.Time

	// LastSyncError returns the last sync error (nil if successful)
	LastSyncError() error
}

// PolicySyncConfig holds configuration for the sync service
type PolicySyncConfig struct {
	// SyncInterval is how often to sync policies
	// Default: 10 minutes
	SyncInterval time.Duration

	// Logger for structured logging
	Logger *slog.Logger
}

// PolicySyncer combines PolicyLoader.SyncPolicies with MetricsProvider.IncrementBackgroundSyncs
// for background policy synchronization. This is a subset of the focused interfaces.
type PolicySyncer interface {
	// SyncPolicies synchronizes all Project policies (from PolicyLoader)
	SyncPolicies(ctx context.Context) error

	// IncrementBackgroundSyncs increments the background sync counter (from MetricsProvider)
	IncrementBackgroundSyncs()
}

// policySyncService implements PolicySyncService
type policySyncService struct {
	syncer PolicySyncer
	config PolicySyncConfig
	logger *slog.Logger

	// State tracking
	mu            sync.RWMutex
	running       bool
	lastSyncTime  time.Time
	lastSyncError error
	stopCh        chan struct{}
	triggerCh     chan struct{}

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// NewPolicySyncService creates a new background policy sync service
func NewPolicySyncService(syncer PolicySyncer, config PolicySyncConfig) PolicySyncService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Default sync interval
	if config.SyncInterval == 0 {
		config.SyncInterval = DefaultPolicySyncInterval
	}

	return &policySyncService{
		syncer:    syncer,
		config:    config,
		logger:    logger,
		stopCh:    make(chan struct{}),
		triggerCh: make(chan struct{}, 1), // Buffered to avoid blocking
	}
}

// Start begins background policy synchronization
func (s *policySyncService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil // Already running
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.triggerCh = make(chan struct{}, 1)
	// Reset stopOnce when starting fresh - allows restart after stop
	s.stopOnce = sync.Once{}
	s.mu.Unlock()

	s.logger.Info("starting background policy sync",
		"interval", s.config.SyncInterval.String())

	// Do initial sync
	s.doSync(ctx)

	// Start background loop
	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("background sync stopping due to context cancellation")
			s.setNotRunning()
			return nil

		case <-s.stopCh:
			s.logger.Info("background sync stopping due to stop signal")
			s.setNotRunning()
			return nil

		case <-ticker.C:
			s.doSync(ctx)

		case <-s.triggerCh:
			s.logger.Info("manual sync triggered")
			s.doSync(ctx)
		}
	}
}

// Stop gracefully stops the sync service
// Safe to call multiple times - uses sync.Once to prevent double-close panic
func (s *policySyncService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("stopping background policy sync")
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.running = false
}

// IsRunning returns true if the service is running
func (s *policySyncService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TriggerSync triggers an immediate sync
func (s *policySyncService) TriggerSync() {
	s.mu.RLock()
	ch := s.triggerCh
	s.mu.RUnlock()

	select {
	case ch <- struct{}{}:
		// Trigger sent
	default:
		// Channel full, sync already pending
	}
}

// LastSyncTime returns the last successful sync time
func (s *policySyncService) LastSyncTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSyncTime
}

// LastSyncError returns the last sync error
func (s *policySyncService) LastSyncError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastSyncError
}

// setNotRunning marks the service as not running (thread-safe)
func (s *policySyncService) setNotRunning() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
}

// doSync performs a single policy sync operation
func (s *policySyncService) doSync(ctx context.Context) {
	s.logger.Debug("starting policy sync")
	startTime := time.Now()

	// Perform the sync
	err := s.syncer.SyncPolicies(ctx)

	duration := time.Since(startTime)

	s.mu.Lock()
	if err != nil {
		s.lastSyncError = err
		s.logger.Error("policy sync failed",
			"error", err,
			"duration", duration.String())
	} else {
		s.lastSyncTime = time.Now()
		s.lastSyncError = nil
		s.logger.Info("policy sync completed",
			"duration", duration.String())
		// Increment metric
		s.syncer.IncrementBackgroundSyncs()
	}
	s.mu.Unlock()
}

// PolicyCacheManagerEnforcer defines the subset of enforcer methods
// needed by PolicyCacheManager for status reporting.
type PolicyCacheManagerEnforcer interface {
	CacheController
	MetricsProvider
}

// PolicyCacheManager orchestrates cache, watcher, and sync services
// This provides a single entry point for managing policy caching infrastructure
type PolicyCacheManager struct {
	enforcer PolicyCacheManagerEnforcer
	watcher  ProjectWatcher
	syncer   PolicySyncService
	logger   *slog.Logger

	mu      sync.RWMutex
	running bool
	synced  bool
	stopCh  chan struct{}

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// PolicyCacheManagerConfig holds configuration for the cache manager
type PolicyCacheManagerConfig struct {
	// Enabled determines if caching is enabled
	Enabled bool

	// WatchEnabled determines if the Project CRD watcher is enabled
	WatchEnabled bool

	// SyncInterval is the background sync interval
	SyncInterval time.Duration

	// ResyncPeriod is the informer resync period
	ResyncPeriod time.Duration

	// Logger for structured logging
	Logger *slog.Logger
}

// NewPolicyCacheManager creates a new policy cache manager
func NewPolicyCacheManager(
	enforcer PolicyCacheManagerEnforcer,
	watcher ProjectWatcher,
	syncer PolicySyncService,
	logger *slog.Logger,
) *PolicyCacheManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &PolicyCacheManager{
		enforcer: enforcer,
		watcher:  watcher,
		syncer:   syncer,
		logger:   logger,
	}
}

// Start starts all caching components
func (m *PolicyCacheManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	// Reset stopOnce when starting fresh - allows restart after stop
	m.stopOnce = sync.Once{}
	m.mu.Unlock()

	m.logger.Info("starting policy cache manager")

	// Start watcher in background
	if m.watcher != nil {
		go func() {
			if err := m.watcher.Start(ctx); err != nil {
				m.logger.Error("watcher error", "error", err)
			}
		}()
	}

	// Start syncer in background
	if m.syncer != nil {
		go func() {
			if err := m.syncer.Start(ctx); err != nil {
				m.logger.Error("syncer error", "error", err)
			}
		}()
	}

	return nil
}

// Stop stops all caching components
// Safe to call multiple times - uses sync.Once to prevent double-close panic
func (m *PolicyCacheManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.logger.Info("stopping policy cache manager")

	if m.watcher != nil {
		m.watcher.Stop()
	}

	if m.syncer != nil {
		m.syncer.Stop()
	}

	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.running = false
}

// Status returns the current status of caching components
type PolicyCacheStatus struct {
	CacheEnabled    bool       `json:"cache_enabled"`
	CacheStats      CacheStats `json:"cache_stats"`
	WatcherRunning  bool       `json:"watcher_running"`
	WatcherLastSync time.Time  `json:"watcher_last_sync"`
	SyncerRunning   bool       `json:"syncer_running"`
	SyncerLastSync  time.Time  `json:"syncer_last_sync"`
	SyncerLastError string     `json:"syncer_last_error,omitempty"`
	PolicyReloads   int64      `json:"policy_reloads"`
	BackgroundSyncs int64      `json:"background_syncs"`
	WatcherRestarts int64      `json:"watcher_restarts"`
}

// IsPolicySynced returns true once the initial policy sync has completed.
// Used by the health checker and authz middleware to gate traffic until RBAC is ready.
func (m *PolicyCacheManager) IsPolicySynced() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.synced
}

// MarkSynced marks the initial policy sync as complete.
// Called by app.go after the synchronous SyncPolicies + RestorePersistedRoles sequence.
func (m *PolicyCacheManager) MarkSynced() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synced = true
}

// Status returns the current status of all caching components
func (m *PolicyCacheManager) Status() PolicyCacheStatus {
	status := PolicyCacheStatus{
		CacheEnabled: true, // Assume enabled if manager exists
	}

	// Get cache stats from enforcer
	if m.enforcer != nil {
		status.CacheStats = m.enforcer.CacheStats()
		metrics := m.enforcer.Metrics()
		status.PolicyReloads = metrics.PolicyReloads
		status.BackgroundSyncs = metrics.BackgroundSyncs
		status.WatcherRestarts = metrics.WatcherRestarts
	}

	// Get watcher status
	if m.watcher != nil {
		status.WatcherRunning = m.watcher.IsRunning()
		status.WatcherLastSync = m.watcher.LastSyncTime()
	}

	// Get syncer status
	if m.syncer != nil {
		status.SyncerRunning = m.syncer.IsRunning()
		status.SyncerLastSync = m.syncer.LastSyncTime()
		if err := m.syncer.LastSyncError(); err != nil {
			status.SyncerLastError = err.Error()
		}
	}

	return status
}
