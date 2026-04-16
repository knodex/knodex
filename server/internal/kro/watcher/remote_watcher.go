// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/multicluster"
	"github.com/knodex/knodex/server/internal/rbac"
)

// RemoteWatchStatus indicates the connectivity state of a remote watch target.
type RemoteWatchStatus string

const (
	RemoteWatchStatusConnected   RemoteWatchStatus = "Connected"
	RemoteWatchStatusUnreachable RemoteWatchStatus = "ClusterUnreachable"
)

// PlatformServiceNamespaces are the shared service namespaces watched by Platform Projects.
var PlatformServiceNamespaces = []string{"cert-manager", "ingress-nginx", "flux-system"}

// GVRs to watch on remote clusters.
var remoteWatchGVRs = []schema.GroupVersionResource{
	{Group: "cert-manager.io", Version: "v1", Resource: "certificates"},
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
}

// Backoff constants for reconnection.
const (
	reconnectInitialDelay = 5 * time.Second
	reconnectMaxDelay     = 5 * time.Minute
	reconnectFactor       = 2.0
	reconcileInterval     = 30 * time.Second
	informerResyncPeriod  = 10 * time.Minute
)

// ProjectLister provides access to project list for remote watcher sync.
type ProjectLister interface {
	ListProjects(ctx context.Context) (*rbac.ProjectList, error)
}

// remoteWatchTarget tracks a single remote cluster watch.
type remoteWatchTarget struct {
	clusterRef    string
	dynamicClient dynamic.Interface
	factory       dynamicinformer.DynamicSharedInformerFactory
	namespaces    []string
	status        RemoteWatchStatus
	stopCh        chan struct{}
	lastError     string
	// backoff tracking
	failureCount int
	lastAttempt  time.Time
}

// RemoteChangeCallback is called when remote resources change.
type RemoteChangeCallback func()

// ClusterRecoveryCallback is called when a cluster transitions from unreachable to connected.
type ClusterRecoveryCallback func(clusterRef string)

// RemoteWatcher watches resources on child clusters using CAPI kubeconfig secrets.
// A RemoteWatcher is single-use: once Stop is called, it cannot be restarted.
type RemoteWatcher struct {
	k8sClient     kubernetes.Interface
	projectLister ProjectLister
	cache         *RemoteResourceCache
	targets       map[string]*remoteWatchTarget
	mu            sync.RWMutex
	logger        *slog.Logger
	running       atomic.Bool
	synced        atomic.Bool
	stopped       atomic.Bool
	stopCh        chan struct{}
	done          chan struct{}
	stopOnce      sync.Once

	onChangeCallbacks   []RemoteChangeCallback
	onRecoveryCallbacks []ClusterRecoveryCallback
}

// NewRemoteWatcher creates a new remote watcher.
// k8sClient is the management cluster client used to read CAPI kubeconfig secrets.
// projectLister provides access to project list for automatic sync (may be nil for testing).
func NewRemoteWatcher(k8sClient kubernetes.Interface, projectLister ProjectLister) *RemoteWatcher {
	return &RemoteWatcher{
		k8sClient:     k8sClient,
		projectLister: projectLister,
		cache:         NewRemoteResourceCache(),
		targets:       make(map[string]*remoteWatchTarget),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		logger:        slog.Default().With("component", "remote-watcher"),
	}
}

// SetOnChangeCallback registers a callback invoked when remote resources change.
func (w *RemoteWatcher) SetOnChangeCallback(cb RemoteChangeCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChangeCallbacks = append(w.onChangeCallbacks, cb)
}

// SetOnRecoveryCallback registers a callback invoked when a cluster recovers from unreachable state.
func (w *RemoteWatcher) SetOnRecoveryCallback(cb ClusterRecoveryCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onRecoveryCallbacks = append(w.onRecoveryCallbacks, cb)
}

// notifyRecovery calls all registered recovery callbacks.
func (w *RemoteWatcher) notifyRecovery(clusterRef string) {
	w.mu.RLock()
	callbacks := make([]ClusterRecoveryCallback, len(w.onRecoveryCallbacks))
	copy(callbacks, w.onRecoveryCallbacks)
	w.mu.RUnlock()

	for _, cb := range callbacks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("recovery callback panicked", "cluster", clusterRef, "panic", r)
				}
			}()
			cb(clusterRef)
		}()
	}
}

// Start begins the reconcile loop that syncs from projects and retries unreachable clusters.
// A stopped watcher cannot be restarted; create a new one instead.
func (w *RemoteWatcher) Start(ctx context.Context) error {
	if w.stopped.Load() {
		return fmt.Errorf("remote watcher has been stopped and cannot be restarted")
	}
	if w.running.Load() {
		w.logger.Warn("remote watcher already running")
		return nil
	}

	w.logger.Info("starting remote watcher")

	w.running.Store(true)
	w.synced.Store(true) // starts synced — targets are added dynamically

	// Initial sync from projects at startup
	if w.projectLister != nil {
		w.syncProjectTargets(ctx)
	}

	go func() {
		defer close(w.done)
		w.reconcileLoop(ctx)
	}()

	return nil
}

// Stop stops the remote watcher. Once stopped, it cannot be restarted.
func (w *RemoteWatcher) Stop() {
	if !w.running.Load() {
		return
	}
	w.logger.Info("stopping remote watcher")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.running.Store(false)
	w.stopped.Store(true)

	// Stop all targets
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, target := range w.targets {
		if target.stopCh != nil {
			select {
			case <-target.stopCh:
			default:
				close(target.stopCh)
			}
		}
	}
}

// StopAndWait stops the watcher and waits up to timeout for the reconcile goroutine to exit.
// Once stopped, the watcher cannot be restarted.
func (w *RemoteWatcher) StopAndWait(timeout time.Duration) bool {
	if !w.running.Load() {
		return true
	}

	w.logger.Info("stopping remote watcher and waiting for completion")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	w.running.Store(false)
	w.stopped.Store(true)

	// Stop all targets
	w.mu.Lock()
	for _, target := range w.targets {
		if target.stopCh != nil {
			select {
			case <-target.stopCh:
			default:
				close(target.stopCh)
			}
		}
	}
	w.mu.Unlock()

	select {
	case <-w.done:
		w.logger.Info("remote watcher stopped cleanly")
		return true
	case <-time.After(timeout):
		w.logger.Warn("remote watcher stop timed out", "timeout", timeout)
		return false
	}
}

// IsSynced returns true if the remote watcher has completed initial sync.
func (w *RemoteWatcher) IsSynced() bool {
	return w.synced.Load()
}

// IsRunning returns true if the remote watcher is running.
func (w *RemoteWatcher) IsRunning() bool {
	return w.running.Load()
}

// Cache returns the remote resource cache.
func (w *RemoteWatcher) Cache() *RemoteResourceCache {
	return w.cache
}

// Targets returns a snapshot of current watch targets (for diagnostics).
func (w *RemoteWatcher) Targets() map[string]RemoteWatchStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make(map[string]RemoteWatchStatus, len(w.targets))
	for ref, t := range w.targets {
		result[ref] = t.status
	}
	return result
}

// GetDynamicClient returns the dynamic client for a remote cluster.
// Returns an error if the cluster is not tracked or unreachable.
func (w *RemoteWatcher) GetDynamicClient(clusterRef string) (dynamic.Interface, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	target, ok := w.targets[clusterRef]
	if !ok {
		return nil, fmt.Errorf("cluster %s is not a tracked watch target", clusterRef)
	}
	if target.status == RemoteWatchStatusUnreachable {
		return nil, fmt.Errorf("cluster %s is unreachable", clusterRef)
	}
	if target.dynamicClient == nil {
		return nil, fmt.Errorf("cluster %s has no dynamic client", clusterRef)
	}
	return target.dynamicClient, nil
}

// IsClusterReachable returns true if the cluster is tracked and connected.
func (w *RemoteWatcher) IsClusterReachable(clusterRef string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	target, ok := w.targets[clusterRef]
	return ok && target.status == RemoteWatchStatusConnected
}

// AddWatchTarget adds a remote cluster to the watch list.
// namespaces specifies which namespaces to watch on the remote cluster.
// Idempotent: no-op if the target already exists and is Connected.
func (w *RemoteWatcher) AddWatchTarget(ctx context.Context, clusterRef string, namespaces []string) error {
	// Check idempotency and determine namespaces under read lock
	w.mu.RLock()
	if existing, ok := w.targets[clusterRef]; ok && existing.status == RemoteWatchStatusConnected {
		w.mu.RUnlock()
		return nil
	}
	w.mu.RUnlock()

	// Determine namespaces to watch
	watchNamespaces := namespaces

	// Build dynamic client for remote cluster (network I/O — outside lock)
	dynClient, err := multicluster.BuildRemoteDynamicClient(ctx, w.k8sClient, clusterRef)
	if err != nil {
		w.logger.Warn("failed to build remote client, marking cluster unreachable",
			"cluster", clusterRef, "error", err)

		w.mu.Lock()
		w.targets[clusterRef] = &remoteWatchTarget{
			clusterRef: clusterRef,
			namespaces: watchNamespaces,
			status:     RemoteWatchStatusUnreachable,
			lastError:  err.Error(),
			stopCh:     make(chan struct{}),
		}
		w.mu.Unlock()
		w.cache.SetClusterStatus(clusterRef, RemoteWatchStatusUnreachable)
		return err
	}

	w.mu.Lock()
	// Re-check under write lock to close TOCTOU window (concurrent AddWatchTarget for same cluster)
	if existing, ok := w.targets[clusterRef]; ok && existing.status == RemoteWatchStatusConnected {
		w.mu.Unlock()
		return nil
	}
	target, err := w.buildTarget(clusterRef, dynClient, watchNamespaces)
	w.mu.Unlock()
	if err != nil {
		return err
	}

	// Start factory outside the lock — factory.Start launches informer goroutines
	// that may invoke callbacks; holding the lock during Start risks deadlock.
	target.factory.Start(target.stopCh)
	return nil
}

// buildTarget initializes informers for a remote cluster target but does NOT start them.
// Must be called with w.mu held. The caller must call factory.Start(stopCh) outside the lock.
func (w *RemoteWatcher) buildTarget(clusterRef string, dynClient dynamic.Interface, namespaces []string) (*remoteWatchTarget, error) {
	// Close the previous target's stopCh to prevent goroutine leaks
	if old, ok := w.targets[clusterRef]; ok && old.stopCh != nil {
		select {
		case <-old.stopCh:
		default:
			close(old.stopCh)
		}
	}

	stopCh := make(chan struct{})
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, informerResyncPeriod)

	target := &remoteWatchTarget{
		clusterRef:    clusterRef,
		dynamicClient: dynClient,
		factory:       factory,
		namespaces:    namespaces,
		status:        RemoteWatchStatusConnected,
		stopCh:        stopCh,
	}

	// Register informer event handlers for each GVR
	for _, gvr := range remoteWatchGVRs {
		informer := factory.ForResource(gvr).Informer()
		_, err := informer.AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				u, ok := obj.(*unstructured.Unstructured)
				if !ok {
					return false
				}
				ns := parser.GetNamespace(u)
				for _, allowed := range namespaces {
					if ns == allowed {
						return true
					}
				}
				return false
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    w.makeAddHandler(clusterRef, gvr),
				UpdateFunc: w.makeUpdateHandler(clusterRef, gvr),
				DeleteFunc: w.makeDeleteHandler(clusterRef, gvr),
			},
		})
		if err != nil {
			w.logger.Error("failed to register informer handler", "cluster", clusterRef, "gvr", gvr, "error", err)
			close(stopCh)
			return nil, err
		}
	}

	w.targets[clusterRef] = target
	w.cache.SetClusterStatus(clusterRef, RemoteWatchStatusConnected)
	w.logger.Info("remote watch target built", "cluster", clusterRef, "namespaces", namespaces)

	return target, nil
}

// RemoveWatchTarget removes a remote cluster from the watch list.
// Idempotent: no-op if target doesn't exist.
func (w *RemoteWatcher) RemoveWatchTarget(clusterRef string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	target, ok := w.targets[clusterRef]
	if !ok {
		return
	}

	select {
	case <-target.stopCh:
	default:
		close(target.stopCh)
	}

	w.cache.DeleteCluster(clusterRef)
	delete(w.targets, clusterRef)
	w.logger.Info("remote watch target removed", "cluster", clusterRef)
}

// SyncFromProjects reconciles watch targets based on current project state.
// Currently a no-op: cluster bindings have been removed from the Project CRD.
// Remote watch targets will be driven by Casbin-gated cluster visibility in a future iteration.
func (w *RemoteWatcher) SyncFromProjects(_ context.Context, _ []*rbac.Project) {
	// No-op: cluster bindings removed from Project CRD.
	// Remote watcher targets will be managed via Casbin cluster policies in a future iteration.
}

// syncProjectTargets fetches projects from the API and syncs watch targets.
func (w *RemoteWatcher) syncProjectTargets(ctx context.Context) {
	if w.projectLister == nil {
		return
	}

	projectList, err := w.projectLister.ListProjects(ctx)
	if err != nil {
		w.logger.Warn("failed to list projects for remote watcher sync", "error", err)
		return
	}

	projects := make([]*rbac.Project, len(projectList.Items))
	for i := range projectList.Items {
		projects[i] = &projectList.Items[i]
	}
	w.SyncFromProjects(ctx, projects)
}

// reconcileLoop periodically syncs from projects and retries unreachable clusters.
func (w *RemoteWatcher) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.syncProjectTargets(ctx)
			w.reconcileTargets(ctx)
		}
	}
}

// reconcileTargets attempts to reconnect unreachable clusters with exponential backoff.
// Collects unreachable targets under lock, then performs network I/O outside the lock.
func (w *RemoteWatcher) reconcileTargets(ctx context.Context) {
	// Collect unreachable targets that are ready for retry (under write lock
	// because we mutate lastAttempt and failureCount on each candidate).
	type retryCandidate struct {
		clusterRef string
		namespaces []string
	}
	var candidates []retryCandidate

	w.mu.Lock()
	for clusterRef, target := range w.targets {
		if target.status != RemoteWatchStatusUnreachable {
			continue
		}

		// Check backoff delay
		delay := backoffDelay(target.failureCount)
		if time.Since(target.lastAttempt) < delay {
			continue
		}

		target.lastAttempt = time.Now()
		target.failureCount++
		candidates = append(candidates, retryCandidate{
			clusterRef: clusterRef,
			namespaces: target.namespaces,
		})
	}
	w.mu.Unlock()

	// Perform network I/O outside the lock
	for _, c := range candidates {
		w.logger.Info("attempting reconnection to unreachable cluster",
			"cluster", c.clusterRef)

		dynClient, err := multicluster.BuildRemoteDynamicClient(ctx, w.k8sClient, c.clusterRef)
		if err != nil {
			w.mu.Lock()
			if target, ok := w.targets[c.clusterRef]; ok {
				target.lastError = err.Error()
				w.logger.Warn("reconnection failed",
					"cluster", c.clusterRef, "attempt", target.failureCount, "error", err)
			}
			w.mu.Unlock()
			continue
		}

		// Reconnection succeeded — restart informers
		w.mu.Lock()
		// Re-validate: target may have been removed or reconnected while we were unlocked
		currentTarget, ok := w.targets[c.clusterRef]
		if !ok || currentTarget.status != RemoteWatchStatusUnreachable {
			w.mu.Unlock()
			continue
		}
		// Use current namespaces from the map, not the stale candidate copy
		target, err := w.buildTarget(c.clusterRef, dynClient, currentTarget.namespaces)
		if err != nil {
			if t, ok := w.targets[c.clusterRef]; ok {
				t.lastError = err.Error()
			}
			w.logger.Warn("failed to restart informers after reconnection",
				"cluster", c.clusterRef, "error", err)
			w.mu.Unlock()
			continue
		}
		w.mu.Unlock()

		// Start factory outside the lock
		target.factory.Start(target.stopCh)
		w.logger.Info("reconnected to cluster", "cluster", c.clusterRef)

		// Notify recovery so dependent services (e.g., WebSocket) can refresh
		w.notifyRecovery(c.clusterRef)
	}
}

// backoffDelay calculates exponential backoff delay.
func backoffDelay(failureCount int) time.Duration {
	if failureCount <= 0 {
		return reconnectInitialDelay
	}
	delay := float64(reconnectInitialDelay) * math.Pow(reconnectFactor, float64(failureCount-1))
	if delay > float64(reconnectMaxDelay) {
		delay = float64(reconnectMaxDelay)
	}
	return time.Duration(delay)
}

// notifyChange calls all registered change callbacks.
func (w *RemoteWatcher) notifyChange() {
	w.mu.RLock()
	callbacks := make([]RemoteChangeCallback, len(w.onChangeCallbacks))
	copy(callbacks, w.onChangeCallbacks)
	w.mu.RUnlock()

	for _, cb := range callbacks {
		cb()
	}
}

func (w *RemoteWatcher) makeAddHandler(clusterRef string, gvr schema.GroupVersionResource) func(obj interface{}) {
	return func(obj interface{}) {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return
		}
		w.cache.Add(clusterRef, parser.GetNamespace(u), gvr.Resource, parser.GetName(u), u)
		w.notifyChange()
	}
}

func (w *RemoteWatcher) makeUpdateHandler(clusterRef string, gvr schema.GroupVersionResource) func(oldObj, newObj interface{}) {
	return func(_, newObj interface{}) {
		u, ok := newObj.(*unstructured.Unstructured)
		if !ok {
			return
		}
		w.cache.Update(clusterRef, parser.GetNamespace(u), gvr.Resource, parser.GetName(u), u)
		w.notifyChange()
	}
}

func (w *RemoteWatcher) makeDeleteHandler(clusterRef string, gvr schema.GroupVersionResource) func(obj interface{}) {
	return func(obj interface{}) {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok {
			tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
			if !ok {
				return
			}
			u, ok = tombstone.Obj.(*unstructured.Unstructured)
			if !ok {
				return
			}
		}
		w.cache.Delete(clusterRef, parser.GetNamespace(u), gvr.Resource, parser.GetName(u))
		w.notifyChange()
	}
}
