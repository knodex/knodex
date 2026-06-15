// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/k8s/parser"
)

const (
	// DefaultTeamWatcherResyncPeriod is how often to re-list all Teams from the
	// K8s API. Mirrors the Project watcher's resync cadence.
	DefaultTeamWatcherResyncPeriod = 30 * time.Minute
)

// TeamWatcher watches for Team CRD changes and keeps a TeamStore in sync.
//
// Namespace-scoped; symmetric with the Project watcher — one Knodex install
// observes only its own namespace.
type TeamWatcher interface {
	// Start begins watching for Team changes. Blocks until the context is
	// canceled or Stop is called.
	Start(ctx context.Context) error

	// Stop gracefully stops the watcher.
	Stop()

	// IsRunning returns true if the watcher is actively watching.
	IsRunning() bool

	// LastSyncTime returns the last time the store was synced.
	LastSyncTime() time.Time
}

// TeamWatcherConfig holds configuration for the team watcher.
type TeamWatcherConfig struct {
	// ResyncPeriod is how often to resync even without changes. Default: 30m.
	ResyncPeriod time.Duration

	// Logger for structured logging.
	Logger *slog.Logger

	// OnChange, if set, is invoked AFTER the store is updated for a team
	// upsert or delete. It lets a consumer (the app wires this to a Casbin
	// policy re-sync) regenerate policies for projects that bind the changed
	// team. The callback MUST NOT block — the app runs the re-sync on a
	// separate goroutine — or it risks wedging the informer goroutine.
	OnChange func(teamName string)
}

// teamWatcher implements TeamWatcher using a namespace-scoped dynamic informer.
type teamWatcher struct {
	dynamicClient dynamic.Interface
	store         *TeamStore
	config        TeamWatcherConfig
	namespace     string // Namespace to watch for Team CRDs
	logger        *slog.Logger

	mu           sync.RWMutex
	running      bool
	lastSyncTime time.Time
	stopCh       chan struct{}
	informer     cache.SharedIndexInformer

	// stopOnce ensures the stop channel is only closed once, preventing a
	// panic from concurrent Stop() calls.
	stopOnce sync.Once
}

// NewTeamWatcher creates a new namespace-scoped Team CRD watcher. Symmetric
// with NewProjectWatcher — one Knodex install observes only Teams in its own
// namespace.
func NewTeamWatcher(dynamicClient dynamic.Interface, store *TeamStore, namespace string, config TeamWatcherConfig) TeamWatcher {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if config.ResyncPeriod == 0 {
		config.ResyncPeriod = DefaultTeamWatcherResyncPeriod
	}

	return &teamWatcher{
		dynamicClient: dynamicClient,
		store:         store,
		config:        config,
		namespace:     namespace,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins watching for Team changes.
func (w *teamWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil // Already running
	}
	w.running = true
	w.stopCh = make(chan struct{})
	// Reset stopOnce when starting fresh - allows restart after stop.
	w.stopOnce = sync.Once{}
	w.mu.Unlock()

	w.logger.Info("starting team watcher",
		"resyncPeriod", w.config.ResyncPeriod.String(),
		"namespace", w.namespace)

	// Namespace-scoped informer factory; symmetric with the Project watcher —
	// one Knodex install observes only its own namespace.
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		w.dynamicClient,
		w.config.ResyncPeriod,
		w.namespace,
		nil,
	)

	gvr := schema.GroupVersionResource{
		Group:    TeamGroup,
		Version:  TeamVersion,
		Resource: TeamResource,
	}

	informer := factory.ForResource(gvr).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.onAdd,
		UpdateFunc: w.onUpdate,
		DeleteFunc: w.onDelete,
	})
	if err != nil {
		w.logger.Error("failed to add team event handler", "error", err)
		w.setNotRunning()
		return fmt.Errorf("add event handler: %w", err)
	}

	w.mu.Lock()
	w.informer = informer
	w.mu.Unlock()

	factory.Start(w.stopCh)

	// Close stopCh on any return below so the informer factory's goroutines tear
	// down, regardless of whether we exit via ctx cancellation, an explicit
	// Stop(), or a cache-sync failure. The app wires this watcher to runCtx only
	// (no Stop() call, unlike the Project watcher's PolicyCacheManager), so this
	// prevents an informer goroutine leak.
	defer w.stopOnce.Do(func() {
		close(w.stopCh)
	})

	w.logger.Info("waiting for team informer cache sync")
	if !cache.WaitForCacheSync(w.stopCh, informer.HasSynced) {
		w.logger.Error("failed to sync team informer cache")
		w.setNotRunning()
		return nil
	}

	w.logger.Info("team informer cache synced, watching for team changes")
	w.updateLastSyncTime()

	select {
	case <-ctx.Done():
		w.logger.Info("team watcher stopping due to context cancellation")
	case <-w.stopCh:
		w.logger.Info("team watcher stopping due to stop signal")
	}

	w.setNotRunning()
	return nil
}

// Stop gracefully stops the watcher. Safe to call multiple times.
func (w *teamWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.logger.Info("stopping team watcher")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.running = false
}

// IsRunning returns true if the watcher is actively watching.
func (w *teamWatcher) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// LastSyncTime returns the last time the store was synced.
func (w *teamWatcher) LastSyncTime() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastSyncTime
}

func (w *teamWatcher) setNotRunning() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.running = false
}

func (w *teamWatcher) updateLastSyncTime() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastSyncTime = time.Now()
}

// onAdd handles Team creation events.
func (w *teamWatcher) onAdd(obj interface{}) {
	u, err := w.extractTeam(obj)
	if err != nil {
		w.logger.Error("failed to extract team from add event", "error", err)
		return
	}
	w.upsertFromUnstructured(u, "added")
}

// onUpdate handles Team update events.
func (w *teamWatcher) onUpdate(oldObj, newObj interface{}) {
	oldTeam, err := w.extractTeam(oldObj)
	if err != nil {
		w.logger.Error("failed to extract old team from update event", "error", err)
		return
	}
	newTeam, err := w.extractTeam(newObj)
	if err != nil {
		w.logger.Error("failed to extract new team from update event", "error", err)
		return
	}

	// Only reload if spec changed (not just status) - matches Project watcher.
	if oldTeam.GetResourceVersion() == newTeam.GetResourceVersion() {
		return
	}
	w.upsertFromUnstructured(newTeam, "updated")
}

// onDelete handles Team deletion events.
func (w *teamWatcher) onDelete(obj interface{}) {
	var u *unstructured.Unstructured
	var err error

	switch t := obj.(type) {
	case *unstructured.Unstructured:
		u = t
	case cache.DeletedFinalStateUnknown:
		u, err = w.extractTeam(t.Obj)
		if err != nil {
			w.logger.Error("failed to extract team from tombstone", "error", err)
			return
		}
	default:
		w.logger.Error("unexpected object type in team delete event",
			"type", fmt.Sprintf("%T", obj))
		return
	}

	teamName := u.GetName()
	w.store.Remove(teamName)
	w.updateLastSyncTime()
	w.logger.Info("team deleted, removed from store", "team", teamName)

	// Fire OnChange AFTER the store is updated so a triggered re-sync reads
	// fresh data (the team is now absent → bound roles contribute no groups).
	if w.config.OnChange != nil {
		w.config.OnChange(teamName)
	}
}

// upsertFromUnstructured extracts the Team spec via the parser library,
// validates it, and upserts the store. Invalid teams are rejected (logged, not
// stored) without panicking.
func (w *teamWatcher) upsertFromUnstructured(u *unstructured.Unstructured, verb string) {
	teamName := u.GetName()

	if err := ValidateTeamName(teamName); err != nil {
		w.logger.Error("invalid team name, skipping", "team", teamName, "error", err)
		return
	}

	// Extract every field through the parser library (CLAUDE.md invariant,
	// AC #3): no raw type assertions on spec fields.
	description := parser.GetSpecFieldStringOrDefault(u, "", "description")
	spec := parser.GetSpecOrEmpty(u)
	rawGroups, err := parser.GetSlice(spec, "oidcGroups")
	if err != nil {
		w.logger.Error("team has no oidcGroups, skipping", "team", teamName, "error", err)
		return
	}

	groups := make([]string, 0, len(rawGroups))
	for _, raw := range rawGroups {
		g, ok := raw.(string)
		if !ok {
			// Defensive: skip & log non-string elements rather than panic.
			w.logger.Warn("non-string oidcGroup element, skipping element",
				"team", teamName, "type", fmt.Sprintf("%T", raw))
			continue
		}
		groups = append(groups, g)
	}

	teamSpec := TeamSpec{Description: description, OIDCGroups: groups}
	if err := ValidateTeamSpec(teamSpec); err != nil {
		w.logger.Error("invalid team spec, skipping", "team", teamName, "error", err)
		return
	}

	w.store.Upsert(&Team{
		ObjectMeta: metav1.ObjectMeta{Name: teamName},
		Spec:       teamSpec,
	})
	w.updateLastSyncTime()
	w.logger.Info("team "+verb+", upserted into store",
		"team", teamName, "groups", len(groups))

	// Fire OnChange AFTER the store is updated so a triggered re-sync reads the
	// new group set (added groups grant / removed groups revoke on next enforce).
	if w.config.OnChange != nil {
		w.config.OnChange(teamName)
	}
}

// extractTeam converts an informer object to *unstructured.Unstructured.
func (w *teamWatcher) extractTeam(obj interface{}) (*unstructured.Unstructured, error) {
	switch t := obj.(type) {
	case *unstructured.Unstructured:
		return t, nil
	default:
		w.logger.Error("unexpected object type", "type", fmt.Sprintf("%T", obj))
		return nil, &TeamWatcherError{Message: "unexpected object type in informer"}
	}
}

// TeamWatcherError represents a watcher-specific error.
type TeamWatcherError struct {
	Message string
}

func (e *TeamWatcherError) Error() string {
	return e.Message
}
