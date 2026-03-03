package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	// DefaultProjectWatcherResyncPeriod is how often to re-list all Projects from K8s API.
	// 30min balances freshness vs API load; relies on watch events for real-time updates.
	DefaultProjectWatcherResyncPeriod = 30 * time.Minute
)

// ProjectWatcher watches for Project CRD changes and triggers policy reloads
type ProjectWatcher interface {
	// Start begins watching for Project changes
	// This blocks until the context is canceled
	Start(ctx context.Context) error

	// Stop gracefully stops the watcher
	Stop()

	// IsRunning returns true if the watcher is actively watching
	IsRunning() bool

	// LastSyncTime returns the last time policies were synced
	LastSyncTime() time.Time
}

// ProjectWatcherConfig holds configuration for the project watcher
type ProjectWatcherConfig struct {
	// ResyncPeriod is how often to resync even without changes
	// Default: 30 minutes
	ResyncPeriod time.Duration

	// Logger for structured logging
	Logger *slog.Logger
}

// ProjectPolicyHandler is called when Project changes are detected
type ProjectPolicyHandler interface {
	// LoadProjectPolicies loads policies for a project
	LoadProjectPolicies(ctx context.Context, projectName string) error

	// RemoveProjectPolicies removes policies for a project
	RemoveProjectPolicies(ctx context.Context, projectName string) error

	// InvalidateCache clears the authorization cache
	InvalidateCache()

	// IncrementWatcherRestarts increments the watcher restart counter
	IncrementWatcherRestarts()
}

// projectWatcher implements ProjectWatcher using Kubernetes dynamic informers
type projectWatcher struct {
	dynamicClient dynamic.Interface
	handler       ProjectPolicyHandler
	config        ProjectWatcherConfig
	logger        *slog.Logger

	// State tracking
	mu           sync.RWMutex
	running      bool
	lastSyncTime time.Time
	stopCh       chan struct{}
	informer     cache.SharedIndexInformer

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// NewProjectWatcher creates a new Project CRD watcher
func NewProjectWatcher(dynamicClient dynamic.Interface, handler ProjectPolicyHandler, config ProjectWatcherConfig) ProjectWatcher {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Default resync period
	if config.ResyncPeriod == 0 {
		config.ResyncPeriod = DefaultProjectWatcherResyncPeriod
	}

	return &projectWatcher{
		dynamicClient: dynamicClient,
		handler:       handler,
		config:        config,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins watching for Project changes
func (w *projectWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil // Already running
	}
	w.running = true
	w.stopCh = make(chan struct{})
	// Reset stopOnce when starting fresh - allows restart after stop
	w.stopOnce = sync.Once{}
	w.mu.Unlock()

	w.logger.Info("starting project watcher",
		"resyncPeriod", w.config.ResyncPeriod.String())

	// Create dynamic informer factory
	factory := dynamicinformer.NewDynamicSharedInformerFactory(
		w.dynamicClient,
		w.config.ResyncPeriod,
	)

	// Get GVR for Project CRD
	gvr := schema.GroupVersionResource{
		Group:    ProjectGroup,
		Version:  ProjectVersion,
		Resource: ProjectResource,
	}

	// Create informer for Project resources
	informer := factory.ForResource(gvr).Informer()

	// Register event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.onAdd,
		UpdateFunc: w.onUpdate,
		DeleteFunc: w.onDelete,
	})

	w.mu.Lock()
	w.informer = informer
	w.mu.Unlock()

	// Start the informer
	factory.Start(w.stopCh)

	// Wait for cache sync
	w.logger.Info("waiting for informer cache sync")
	if !cache.WaitForCacheSync(w.stopCh, informer.HasSynced) {
		w.logger.Error("failed to sync informer cache")
		w.setNotRunning()
		return nil
	}

	w.logger.Info("informer cache synced, watching for project changes")
	w.updateLastSyncTime()

	// Wait for stop signal or context cancellation
	select {
	case <-ctx.Done():
		w.logger.Info("project watcher stopping due to context cancellation")
	case <-w.stopCh:
		w.logger.Info("project watcher stopping due to stop signal")
	}

	w.setNotRunning()
	return nil
}

// Stop gracefully stops the watcher
// Safe to call multiple times - uses sync.Once to prevent double-close panic
func (w *projectWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.logger.Info("stopping project watcher")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.running = false
}

// IsRunning returns true if the watcher is actively watching
func (w *projectWatcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// LastSyncTime returns the last time policies were synced
func (w *projectWatcher) LastSyncTime() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastSyncTime
}

// setNotRunning marks the watcher as not running (thread-safe)
func (w *projectWatcher) setNotRunning() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = false
}

// updateLastSyncTime updates the last sync time (thread-safe)
func (w *projectWatcher) updateLastSyncTime() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastSyncTime = time.Now()
}

// onAdd handles Project creation events
func (w *projectWatcher) onAdd(obj interface{}) {
	project, err := w.extractProject(obj)
	if err != nil {
		w.logger.Error("failed to extract project from add event", "error", err)
		return
	}

	projectName := project.GetName()
	w.logger.Info("project added, loading policies",
		"project", projectName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := w.handler.LoadProjectPolicies(ctx, projectName); err != nil {
		w.logger.Error("failed to load policies for new project",
			"project", projectName,
			"error", err)
		return
	}

	w.updateLastSyncTime()
	w.logger.Info("policies loaded for new project", "project", projectName)
}

// onUpdate handles Project update events
func (w *projectWatcher) onUpdate(oldObj, newObj interface{}) {
	oldProject, err := w.extractProject(oldObj)
	if err != nil {
		w.logger.Error("failed to extract old project from update event", "error", err)
		return
	}

	newProject, err := w.extractProject(newObj)
	if err != nil {
		w.logger.Error("failed to extract new project from update event", "error", err)
		return
	}

	// Only reload if spec changed (not just status)
	if oldProject.GetResourceVersion() == newProject.GetResourceVersion() {
		return
	}

	projectName := newProject.GetName()
	w.logger.Info("project updated, reloading policies",
		"project", projectName,
		"oldVersion", oldProject.GetResourceVersion(),
		"newVersion", newProject.GetResourceVersion())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Reload policies for the updated project
	if err := w.handler.LoadProjectPolicies(ctx, projectName); err != nil {
		w.logger.Error("failed to reload policies for updated project",
			"project", projectName,
			"error", err)
		return
	}

	w.updateLastSyncTime()
	w.logger.Info("policies reloaded for updated project", "project", projectName)
}

// onDelete handles Project deletion events
func (w *projectWatcher) onDelete(obj interface{}) {
	// Handle deleted object or tombstone
	var project *unstructured.Unstructured
	var err error

	switch t := obj.(type) {
	case *unstructured.Unstructured:
		project = t
	case cache.DeletedFinalStateUnknown:
		// The object was deleted before we could observe its final state
		project, err = w.extractProject(t.Obj)
		if err != nil {
			w.logger.Error("failed to extract project from tombstone", "error", err)
			return
		}
	default:
		w.logger.Error("unexpected object type in delete event",
			"type", fmt.Sprintf("%T", obj))
		return
	}

	projectName := project.GetName()
	w.logger.Info("project deleted, removing policies", "project", projectName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := w.handler.RemoveProjectPolicies(ctx, projectName); err != nil {
		w.logger.Error("failed to remove policies for deleted project",
			"project", projectName,
			"error", err)
		return
	}

	w.updateLastSyncTime()
	w.logger.Info("policies removed for deleted project", "project", projectName)
}

// extractProject converts an informer object to *unstructured.Unstructured
func (w *projectWatcher) extractProject(obj interface{}) (*unstructured.Unstructured, error) {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
		return t, nil
	default:
		w.logger.Error("unexpected object type", "type", fmt.Sprintf("%T", obj))
		return nil, &ProjectWatcherError{Message: "unexpected object type in informer"}
	}
}

// ProjectWatcherError represents a watcher-specific error
type ProjectWatcherError struct {
	Message string
}

func (e *ProjectWatcherError) Error() string {
	return e.Message
}

// ProjectWatcherManager manages the watcher lifecycle with reconnection
type ProjectWatcherManager struct {
	watcher ProjectWatcher
	handler ProjectPolicyHandler
	logger  *slog.Logger

	// Reconnection settings
	initialBackoff time.Duration
	maxBackoff     time.Duration

	// State
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// NewProjectWatcherManager creates a manager that handles watcher lifecycle
func NewProjectWatcherManager(watcher ProjectWatcher, handler ProjectPolicyHandler, logger *slog.Logger) *ProjectWatcherManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &ProjectWatcherManager{
		watcher:        watcher,
		handler:        handler,
		logger:         logger,
		initialBackoff: 1 * time.Second,
		maxBackoff:     5 * time.Minute,
	}
}

// Start begins the managed watcher with automatic reconnection
func (m *ProjectWatcherManager) Start(ctx context.Context) error {
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

	go m.runWithReconnect(ctx)
	return nil
}

// Stop stops the managed watcher
// Safe to call multiple times - uses sync.Once to prevent double-close panic
func (m *ProjectWatcherManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
	m.watcher.Stop()
	m.running = false
}

// runWithReconnect runs the watcher with exponential backoff reconnection
func (m *ProjectWatcherManager) runWithReconnect(ctx context.Context) {
	backoff := m.initialBackoff
	restartCount := 0

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("watcher manager stopping due to context cancellation")
			return
		case <-m.stopCh:
			m.logger.Info("watcher manager stopping due to stop signal")
			return
		default:
		}

		// Run the watcher
		m.logger.Info("starting project watcher", "restartCount", restartCount)
		err := m.watcher.Start(ctx)

		if err != nil {
			m.logger.Error("watcher error", "error", err)
		}

		// Check if we should stop
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
		}

		// Watcher stopped unexpectedly, reconnect with backoff
		restartCount++
		if m.handler != nil {
			m.handler.IncrementWatcherRestarts()
		}

		m.logger.Warn("watcher stopped, reconnecting with backoff",
			"backoff", backoff.String(),
			"restartCount", restartCount)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		}

		// Exponential backoff with max
		backoff = backoff * 2
		if backoff > m.maxBackoff {
			backoff = m.maxBackoff
		}
	}
}
