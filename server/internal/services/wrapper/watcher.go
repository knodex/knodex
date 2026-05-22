// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package wrapper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/util/hash"
)

// DefaultResyncPeriod is the resync period for the wrapper ConfigMap informer.
const DefaultResyncPeriod = 30 * time.Second

// EntriesChangedFunc is invoked when the wrapper entries set changes content
// (i.e. after hash-dedup suppresses cache-warmup and resync events).
type EntriesChangedFunc func(entries []Entry)

// Watcher watches the wrapper ConfigMap and maintains an in-memory cache of
// the latest valid entries. Mirrors the shape of internal/sso/SSOWatcher to
// minimize per-feature divergence.
type Watcher struct {
	k8sClient kubernetes.Interface
	namespace string
	logger    *slog.Logger

	mu               sync.RWMutex
	lastValidEntries []Entry
	lastEntriesHash  string
	callbacks        []EntriesChangedFunc

	stopCh   chan struct{}
	stopOnce sync.Once
	running  bool
}

// NewWatcher constructs a wrapper Watcher.
func NewWatcher(k8sClient kubernetes.Interface, namespace string, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		k8sClient:        k8sClient,
		namespace:        namespace,
		logger:           logger,
		lastValidEntries: []Entry{},
	}
}

// OnEntriesChanged registers a callback invoked on every content change.
func (w *Watcher) OnEntriesChanged(fn EntriesChangedFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, fn)
}

// Entries returns a thread-safe copy of the last valid entries.
func (w *Watcher) Entries() []Entry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]Entry, len(w.lastValidEntries))
	copy(out, w.lastValidEntries)
	return out
}

// Lookup is a convenience hot-path accessor: returns the registered RGD name
// for the given Kind, or ("", false) when no entry exists.
func (w *Watcher) Lookup(kind string) (rgdName string, ok bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, e := range w.lastValidEntries {
		if e.Kind == kind {
			return e.RGDName, true
		}
	}
	return "", false
}

// Start begins watching the wrapper ConfigMap. Blocks until ctx is done or
// Stop() is called. Idempotent — a second Start returns nil immediately.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	// Note: deliberately do NOT reset stopOnce here. Re-using a sync.Once after
	// Stop() has consumed it would let setNotRunning close an already-closed
	// channel — and resetting it via `sync.Once{}` reintroduces the race the
	// guard exists to prevent. The running-flag guard above is sufficient for
	// idempotent Start; a single-shot lifecycle is the contract.
	w.mu.Unlock()

	w.logger.Info("starting wrapper watcher",
		"namespace", w.namespace,
		"configmap", ConfigMapName,
	)

	// Initial sync: load before the informer starts to ensure the cache is
	// populated before any event handlers fire.
	entries := w.loadEntries(ctx)
	w.mu.Lock()
	w.lastValidEntries = entries
	w.lastEntriesHash = entriesHash(entries)
	w.mu.Unlock()

	factory := informers.NewSharedInformerFactoryWithOptions(
		w.k8sClient,
		DefaultResyncPeriod,
		informers.WithNamespace(w.namespace),
	)

	cmInformer := factory.Core().V1().ConfigMaps().Informer()
	cmInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			cm, ok := obj.(*corev1.ConfigMap)
			return ok && cm.Name == ConfigMapName
		},
		Handler: cache.ResourceEventHandlerFuncs{
			// ctx is captured from Start's parameter; it remains valid for the
			// duration of the watcher's run (Start blocks until ctx is done).
			AddFunc: func(_ interface{}) { w.onConfigChange(ctx, "add") },
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldCM := oldObj.(*corev1.ConfigMap)
				newCM := newObj.(*corev1.ConfigMap)
				if oldCM.ResourceVersion == newCM.ResourceVersion {
					return
				}
				w.onConfigChange(ctx, "update")
			},
			DeleteFunc: func(_ interface{}) { w.onConfigMapDelete() },
		},
	})

	factory.Start(w.stopCh)

	if !cache.WaitForCacheSync(w.stopCh, cmInformer.HasSynced) {
		w.logger.Error("failed to sync wrapper informer cache")
		w.setNotRunning()
		return fmt.Errorf("failed to sync wrapper informer cache")
	}
	w.logger.Info("wrapper informer cache synced, watching for changes")

	select {
	case <-ctx.Done():
		w.logger.Info("wrapper watcher stopping due to context cancellation")
	case <-w.stopCh:
		w.logger.Info("wrapper watcher stopping due to stop signal")
	}

	w.setNotRunning()
	return nil
}

// Stop signals the watcher to shut down. Safe to call multiple times.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.running = false
}

// onConfigChange reloads entries on any add/update event, deduping by hash
// to avoid spurious notifications from cache warmup + periodic resync.
func (w *Watcher) onConfigChange(ctx context.Context, action string) {
	entries := w.loadEntries(ctx)
	newHash := entriesHash(entries)

	w.mu.Lock()
	unchanged := newHash != "" && newHash == w.lastEntriesHash
	w.lastValidEntries = entries
	w.lastEntriesHash = newHash
	w.mu.Unlock()

	if unchanged {
		w.logger.Debug("wrapper entries unchanged, skipping reload", "action", action)
		return
	}
	w.logger.Info("wrapper entries changed", "action", action, "count", len(entries))
	w.notifyCallbacks(entries)
}

// onConfigMapDelete intentionally keeps last-valid entries on deletion.
// Mirrors SSO precedent so an accidentally-deleted ConfigMap doesn't break
// resource routing — operators can recreate it without restarting the server.
func (w *Watcher) onConfigMapDelete() {
	w.logger.Warn("wrapper ConfigMap deleted, keeping last valid entries",
		"configmap", ConfigMapName,
	)
}

// loadEntries fetches and parses the current ConfigMap.
// Returns last-valid on parse error; empty slice when ConfigMap is missing.
func (w *Watcher) loadEntries(ctx context.Context) []Entry {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cm, err := w.k8sClient.CoreV1().ConfigMaps(w.namespace).Get(ctx, ConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return []Entry{}
		}
		w.logger.Error("failed to read wrapper ConfigMap", "error", err)
		return w.copyLastValid()
	}

	data, ok := cm.Data[ConfigMapKey]
	if !ok || data == "" {
		return []Entry{}
	}

	var entries []Entry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		w.logger.Error("malformed wrapper entries JSON, keeping last valid",
			"error", err,
		)
		return w.copyLastValid()
	}

	// Filter out malformed entries and deduplicate by Kind (last write wins,
	// mirroring Helm values override semantics). The API's Put enforces
	// Kind-uniqueness for runtime changes, but the Helm values file is a raw
	// data sink that doesn't enforce it — deduplicating here prevents the
	// second entry from silently becoming dead data.
	seen := make(map[string]bool, len(entries))
	valid := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.Kind == "" || e.RGDName == "" {
			w.logger.Warn("skipping wrapper entry with missing fields",
				"kind", e.Kind, "rgdName", e.RGDName,
			)
			continue
		}
		if seen[e.Kind] {
			w.logger.Warn("duplicate wrapper entry for Kind, keeping last",
				"kind", e.Kind,
			)
			// Replace the earlier entry for this Kind with the current one.
			for i, existing := range valid {
				if existing.Kind == e.Kind {
					valid[i] = e
					break
				}
			}
			continue
		}
		seen[e.Kind] = true
		valid = append(valid, e)
	}
	return valid
}

func (w *Watcher) copyLastValid() []Entry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]Entry, len(w.lastValidEntries))
	copy(out, w.lastValidEntries)
	return out
}

func (w *Watcher) notifyCallbacks(entries []Entry) {
	w.mu.RLock()
	cbs := make([]EntriesChangedFunc, len(w.callbacks))
	copy(cbs, w.callbacks)
	w.mu.RUnlock()
	for _, cb := range cbs {
		cb(entries)
	}
}

func (w *Watcher) setNotRunning() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.running = false
}

// entriesHash returns a deterministic SHA-256 of the entry set.
// Empty on marshal failure (caller falls through to reload).
func entriesHash(entries []Entry) string {
	data, err := json.Marshal(entries)
	if err != nil {
		return ""
	}
	return hash.SHA256(data)
}
