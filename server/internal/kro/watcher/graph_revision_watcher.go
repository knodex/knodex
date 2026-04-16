// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kro"
	"github.com/knodex/knodex/server/internal/models"
)

// RevisionChangeCallback is called when any GraphRevision changes in the cache.
type RevisionChangeCallback func()

// RevisionUpdateCallback is called when an individual GraphRevision changes.
type RevisionUpdateCallback func(action string, rgdName string, revision int)

// RevisionAddCallback is called when a new GraphRevision is added to the cache.
// Signature omits the action string since it is always "add".
type RevisionAddCallback func(rgdName string, revision int)

// graphRevisionGVR returns the GroupVersionResource for KRO GraphRevisions.
var graphRevisionGVR = kro.GraphRevisionGVR()

// GraphRevisionWatcher watches for GraphRevision resources in the cluster.
type GraphRevisionWatcher struct {
	factory  dynamicinformer.DynamicSharedInformerFactory
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
	done     chan struct{}
	synced   atomic.Bool
	running  atomic.Bool
	logger   *slog.Logger

	// Cache: map of RGD name -> sorted revisions (descending by revision number)
	mu    sync.RWMutex
	cache map[string][]models.GraphRevision

	onChangeCallbacks []RevisionChangeCallback
	onUpdateCallbacks []RevisionUpdateCallback
	onAddCallbacks    []RevisionAddCallback

	stopOnce sync.Once
}

// NewGraphRevisionWatcher creates a new GraphRevision watcher using a shared informer factory.
func NewGraphRevisionWatcher(factory dynamicinformer.DynamicSharedInformerFactory) *GraphRevisionWatcher {
	return &GraphRevisionWatcher{
		factory: factory,
		cache:   make(map[string][]models.GraphRevision),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
		logger:  slog.Default().With("component", "graph-revision-watcher"),
	}
}

// SetOnChangeCallback adds a callback to be invoked when GraphRevisions change.
func (w *GraphRevisionWatcher) SetOnChangeCallback(cb RevisionChangeCallback) {
	w.onChangeCallbacks = append(w.onChangeCallbacks, cb)
}

// SetOnUpdateCallback adds a callback to be invoked with revision details when GraphRevisions change.
func (w *GraphRevisionWatcher) SetOnUpdateCallback(cb RevisionUpdateCallback) {
	w.onUpdateCallbacks = append(w.onUpdateCallbacks, cb)
}

// SetOnAddCallback adds a callback to be invoked when a new GraphRevision is added.
func (w *GraphRevisionWatcher) SetOnAddCallback(cb RevisionAddCallback) {
	w.onAddCallbacks = append(w.onAddCallbacks, cb)
}

// Start begins watching for GraphRevision resources.
func (w *GraphRevisionWatcher) Start(ctx context.Context) error {
	if w.running.Load() {
		w.logger.Warn("watcher already running")
		return nil
	}

	w.logger.Info("starting GraphRevision watcher")

	// Reinitialize channels for fresh start
	w.stopCh = make(chan struct{})
	w.done = make(chan struct{})
	w.stopOnce = sync.Once{}

	w.informer = w.factory.ForResource(graphRevisionGVR).Informer()

	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		UpdateFunc: w.handleUpdate,
		DeleteFunc: w.handleDelete,
	})
	if err != nil {
		return err
	}

	go func() {
		defer close(w.done)
		w.running.Store(true)
		w.informer.Run(w.stopCh)
		w.running.Store(false)
		w.logger.Info("GraphRevision watcher stopped")
	}()

	go func() {
		if !cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced) {
			w.logger.Error("failed to sync GraphRevision cache")
			return
		}
		w.synced.Store(true)
		w.mu.RLock()
		count := 0
		for _, revs := range w.cache {
			count += len(revs)
		}
		w.mu.RUnlock()
		w.logger.Info("GraphRevision cache synced", "count", count)
		w.notifyChange()
	}()

	return nil
}

// Stop stops the watcher without waiting for completion.
func (w *GraphRevisionWatcher) Stop() {
	if !w.running.Load() {
		return
	}
	w.logger.Info("stopping GraphRevision watcher")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

// StopAndWait stops the watcher and waits for the informer goroutine to exit.
func (w *GraphRevisionWatcher) StopAndWait(timeout time.Duration) bool {
	if !w.running.Load() {
		return true
	}

	w.logger.Info("stopping GraphRevision watcher and waiting for completion")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})

	select {
	case <-w.done:
		w.logger.Info("GraphRevision watcher stopped cleanly")
		return true
	case <-time.After(timeout):
		w.logger.Warn("GraphRevision watcher stop timed out", "timeout", timeout)
		return false
	}
}

// IsSynced returns true if the initial cache sync is complete.
func (w *GraphRevisionWatcher) IsSynced() bool {
	return w.synced.Load()
}

// IsRunning returns true if the watcher is running.
func (w *GraphRevisionWatcher) IsRunning() bool {
	return w.running.Load()
}

// ListRevisions returns all cached revisions for the given RGD name, sorted by revision number descending.
func (w *GraphRevisionWatcher) ListRevisions(rgdName string) models.GraphRevisionList {
	w.mu.RLock()
	defer w.mu.RUnlock()

	revisions, ok := w.cache[rgdName]
	if !ok {
		return models.GraphRevisionList{
			Items:      []models.GraphRevision{},
			TotalCount: 0,
		}
	}

	// Return copies without Snapshot (omit large payloads from list responses)
	items := make([]models.GraphRevision, len(revisions))
	for i, r := range revisions {
		r.Snapshot = nil
		items[i] = r
	}

	return models.GraphRevisionList{
		Items:      items,
		TotalCount: len(items),
	}
}

// GetRevision returns a specific revision by RGD name and revision number.
func (w *GraphRevisionWatcher) GetRevision(rgdName string, revision int) (*models.GraphRevision, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	revisions, ok := w.cache[rgdName]
	if !ok {
		return nil, false
	}

	for i := range revisions {
		if revisions[i].RevisionNumber == revision {
			rev := revisions[i]
			return &rev, true
		}
	}
	return nil, false
}

// GetLatestRevision returns the highest-numbered revision for an RGD.
func (w *GraphRevisionWatcher) GetLatestRevision(rgdName string) (*models.GraphRevision, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	revisions, ok := w.cache[rgdName]
	if !ok || len(revisions) == 0 {
		return nil, false
	}

	// Cache is sorted descending, so first element is the latest
	rev := revisions[0]
	return &rev, true
}

// handleAdd processes a new GraphRevision.
func (w *GraphRevisionWatcher) handleAdd(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		w.logger.Error("unexpected object type in add handler", "type", obj)
		return
	}

	rev := w.parseRevision(u)
	w.addToCache(rev)
	w.logger.Debug("added GraphRevision",
		"rgdName", rev.RGDName,
		"revision", rev.RevisionNumber)
	w.notifyChange()
	w.notifyUpdate("add", rev.RGDName, rev.RevisionNumber)
	w.notifyAdd(rev.RGDName, rev.RevisionNumber)
}

// handleUpdate processes an updated GraphRevision.
func (w *GraphRevisionWatcher) handleUpdate(oldObj, newObj interface{}) {
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		w.logger.Error("unexpected object type in update handler", "type", newObj)
		return
	}

	rev := w.parseRevision(u)
	w.addToCache(rev)
	w.logger.Debug("updated GraphRevision",
		"rgdName", rev.RGDName,
		"revision", rev.RevisionNumber)
	w.notifyChange()
	w.notifyUpdate("update", rev.RGDName, rev.RevisionNumber)
}

// handleDelete processes a deleted GraphRevision.
func (w *GraphRevisionWatcher) handleDelete(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			w.logger.Error("unexpected object type in delete handler", "type", obj)
			return
		}
		u, ok = tombstone.Obj.(*unstructured.Unstructured)
		if !ok {
			w.logger.Error("unexpected tombstone object type", "type", tombstone.Obj)
			return
		}
	}

	rev := w.parseRevision(u)
	w.removeFromCache(rev.RGDName, rev.RevisionNumber)
	w.logger.Debug("deleted GraphRevision",
		"rgdName", rev.RGDName,
		"revision", rev.RevisionNumber)
	w.notifyChange()
	w.notifyUpdate("delete", rev.RGDName, rev.RevisionNumber)
}

// parseRevision converts an unstructured GraphRevision to our model.
func (w *GraphRevisionWatcher) parseRevision(obj *unstructured.Unstructured) models.GraphRevision {
	labels := parser.GetLabels(obj)
	annotations := parser.GetAnnotations(obj)

	// Extract spec fields using parser accessors
	spec, _ := parser.GetMap(obj.Object, "spec")
	revisionNum := parser.GetInt64OrDefault(spec, 0, "revision")

	// Extract snapshot (the frozen RGD spec)
	snapshot, _ := parser.GetMap(spec, "snapshot")

	// Extract RGD name from snapshot.name
	rgdName := parser.GetStringOrDefault(snapshot, "", "name")
	if rgdName == "" {
		// Fallback: try label
		rgdName = labels["internal.kro.run/rgd-name"]
	}

	// Extract content hash
	contentHash := parser.GetStringOrDefault(spec, "", "contentHash")

	// Extract conditions from status
	var conditions []models.GraphRevisionCondition
	condSlice := parser.GetConditions(obj)
	for _, c := range condSlice {
		if cm, ok := c.(map[string]interface{}); ok {
			cond := models.GraphRevisionCondition{
				Type:    parser.GetStringOrDefault(cm, "", "type"),
				Status:  parser.GetStringOrDefault(cm, "", "status"),
				Reason:  parser.GetStringOrDefault(cm, "", "reason"),
				Message: parser.GetStringOrDefault(cm, "", "message"),
			}
			conditions = append(conditions, cond)
		}
	}

	createdAt := parser.GetCreationTimestamp(obj)

	return models.GraphRevision{
		RevisionNumber:  int(revisionNum),
		RGDName:         rgdName,
		Namespace:       parser.GetNamespace(obj),
		Conditions:      conditions,
		ContentHash:     contentHash,
		CreatedAt:       createdAt,
		Labels:          labels,
		Annotations:     annotations,
		ResourceVersion: parser.GetResourceVersion(obj),
		Snapshot:        snapshot,
	}
}

// addToCache adds or updates a revision in the cache, maintaining descending sort order.
func (w *GraphRevisionWatcher) addToCache(rev models.GraphRevision) {
	w.mu.Lock()
	defer w.mu.Unlock()

	revisions := w.cache[rev.RGDName]

	// Remove existing entry with same revision number (update case)
	filtered := make([]models.GraphRevision, 0, len(revisions))
	for _, r := range revisions {
		if r.RevisionNumber != rev.RevisionNumber {
			filtered = append(filtered, r)
		}
	}
	filtered = append(filtered, rev)

	// Sort descending by revision number
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].RevisionNumber > filtered[j].RevisionNumber
	})

	w.cache[rev.RGDName] = filtered
}

// removeFromCache removes a revision from the cache.
func (w *GraphRevisionWatcher) removeFromCache(rgdName string, revisionNumber int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	revisions, ok := w.cache[rgdName]
	if !ok {
		return
	}

	filtered := make([]models.GraphRevision, 0, len(revisions))
	for _, r := range revisions {
		if r.RevisionNumber != revisionNumber {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		delete(w.cache, rgdName)
	} else {
		w.cache[rgdName] = filtered
	}
}

// notifyChange invokes all registered change callbacks.
func (w *GraphRevisionWatcher) notifyChange() {
	for i, cb := range w.onChangeCallbacks {
		func(idx int, callback RevisionChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("panic in onChangeCallback", "index", idx, "error", r)
				}
			}()
			callback()
		}(i, cb)
	}
}

// notifyUpdate invokes all registered update callbacks.
func (w *GraphRevisionWatcher) notifyUpdate(action string, rgdName string, revision int) {
	for i, cb := range w.onUpdateCallbacks {
		func(idx int, callback RevisionUpdateCallback) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("panic in onUpdateCallback", "index", idx, "error", r)
				}
			}()
			callback(action, rgdName, revision)
		}(i, cb)
	}
}

// notifyAdd invokes all registered add callbacks.
func (w *GraphRevisionWatcher) notifyAdd(rgdName string, revision int) {
	for i, cb := range w.onAddCallbacks {
		func(idx int, callback RevisionAddCallback) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("panic in onAddCallback", "index", idx, "error", r)
				}
			}()
			callback(rgdName, revision)
		}(i, cb)
	}
}
