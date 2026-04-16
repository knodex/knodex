// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"

	"github.com/knodex/knodex/server/internal/deployment"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kro"
	"github.com/knodex/knodex/server/internal/kro/metadata"

	"github.com/knodex/knodex/server/internal/models"
)

// InstanceChangeCallback is called when instances change in the cache (legacy)
type InstanceChangeCallback func()

// InstanceAction represents the type of change to an instance
type InstanceAction string

const (
	// InstanceActionAdd indicates a new instance was added
	InstanceActionAdd InstanceAction = "add"
	// InstanceActionUpdate indicates an instance was updated
	InstanceActionUpdate InstanceAction = "update"
	// InstanceActionDelete indicates an instance was deleted
	InstanceActionDelete InstanceAction = "delete"
)

// InstanceUpdateCallback is called when instances are added, updated, or deleted
// with details about what changed
type InstanceUpdateCallback func(action InstanceAction, namespace, kind, name string, instance *models.Instance)

// rgdRef holds the namespace and name of an RGD associated with an informer
type rgdRef struct {
	namespace string
	name      string
}

// informerEntry tracks a registered event handler on a shared informer.
// Each RGD gets its own handler registration, even when multiple RGDs share the same GVR informer.
type informerEntry struct {
	reg cache.ResourceEventHandlerRegistration
	gvr schema.GroupVersionResource
	rgd rgdRef
}

// InstanceTracker watches for instances (CRs) created by RGDs
type InstanceTracker struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	factory         dynamicinformer.DynamicSharedInformerFactory
	cache           *InstanceCache
	rgdWatcher      *RGDWatcher

	// Map of informer key (namespace/name@gvr) to handler registration.
	// Multiple RGDs sharing the same GVR share one informer but have separate handlers.
	informers   map[string]informerEntry
	informersMu sync.RWMutex

	stopCh            chan struct{}
	synced            atomic.Bool
	running           atomic.Bool
	logger            *slog.Logger
	onChangeCallbacks []InstanceChangeCallback
	onUpdateCallbacks []InstanceUpdateCallback

	// wg tracks background goroutines (e.g. cache sync) so Stop() can wait for them.
	wg sync.WaitGroup

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// NewInstanceTracker creates a new instance tracker using a shared dynamic client
// and informer factory. The factory deduplicates informers: multiple RGDs producing
// the same GVR share a single watch stream, reducing API server load.
func NewInstanceTracker(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface, factory dynamicinformer.DynamicSharedInformerFactory, rgdWatcher *RGDWatcher) *InstanceTracker {
	return &InstanceTracker{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		factory:         factory,
		cache:           NewInstanceCache(),
		rgdWatcher:      rgdWatcher,
		informers:       make(map[string]informerEntry),
		stopCh:          make(chan struct{}),
		logger:          slog.Default().With("component", "instance-tracker"),
	}
}

// NewInstanceTrackerWithCache creates a tracker with an existing cache (for testing)
func NewInstanceTrackerWithCache(cache *InstanceCache) *InstanceTracker {
	t := &InstanceTracker{
		cache:     cache,
		informers: make(map[string]informerEntry),
		stopCh:    make(chan struct{}),
		logger:    slog.Default().With("component", "instance-tracker"),
	}
	t.running.Store(true)
	t.synced.Store(true)
	return t
}

// NewInstanceTrackerForTest creates a tracker with cache and dynamic client.
// Enables handler tests that need both GetInstance (cache) and DeleteInstance (dynamic client).
func NewInstanceTrackerForTest(cache *InstanceCache, dynamicClient dynamic.Interface) *InstanceTracker {
	t := NewInstanceTrackerWithCache(cache)
	t.dynamicClient = dynamicClient
	return t
}

// SetDiscoveryClient sets the discovery client on the tracker.
// Primarily used for testing to inject a fake discovery client.
//
// NOT safe for concurrent use: must be called before Start() or any goroutine
// that reads t.discoveryClient. Production code sets this once at construction
// via NewInstanceTracker; tests should call it before any handler invocations.
func (t *InstanceTracker) SetDiscoveryClient(client discovery.DiscoveryInterface) {
	t.discoveryClient = client
}

// Start begins watching for instances of all known RGD types
func (t *InstanceTracker) Start(ctx context.Context) error {
	if t.running.Load() {
		t.logger.Warn("instance tracker already running")
		return nil
	}

	t.logger.Info("starting instance tracker")
	t.running.Store(true)

	// Start informers for all currently known RGD types
	t.startInformersForKnownRGDs()

	// Register callback with RGD watcher to handle new RGDs
	if t.rgdWatcher != nil {
		t.rgdWatcher.SetOnChangeCallback(func() {
			t.handleRGDChange()
		})

		// Register update callback to propagate RGD status changes to cached instances
		t.rgdWatcher.SetOnUpdateCallback(func(action RGDAction, name string, rgd *models.CatalogRGD) {
			if action == RGDActionUpdate && rgd != nil {
				t.updateInstancesRGDStatus(rgd)
			}
		})
	}

	// Wait for initial informer caches to sync in background.
	// Uses factory.WaitForCacheSync instead of a fixed sleep — accurate on fast and slow clusters.
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		if t.factory == nil {
			t.synced.Store(true)
			return
		}
		// Apply 30s timeout (NFR18: <30s startup). If sync takes longer,
		// the tracker becomes operational serving stale data.
		syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
		defer syncCancel()

		// Also cancel when stopCh closes (AC #1: cancellable via t.stopCh)
		go func() {
			select {
			case <-t.stopCh:
				syncCancel()
			case <-syncCtx.Done():
			}
		}()

		syncResults := t.factory.WaitForCacheSync(syncCtx.Done())
		allSynced := true
		for gvr, synced := range syncResults {
			if !synced {
				t.logger.Error("informer cache failed to sync", "gvr", gvr.String())
				allSynced = false
			}
		}
		if !allSynced {
			t.logger.Error("instance tracker cache sync incomplete, serving stale data")
			return
		}
		t.synced.Store(true)
		t.logger.Info("instance tracker synced", "count", t.cache.Count())
	}()

	return nil
}

// Stop removes all event handlers and marks the tracker as stopped.
// Closing stopCh also stops informers started via factory.Start(stopCh).
// Safe to call multiple times — uses sync.Once to prevent double-close panic.
func (t *InstanceTracker) Stop() {
	if !t.running.Load() {
		return
	}

	t.logger.Info("stopping instance tracker")

	// Remove all event handlers from shared informers
	t.informersMu.Lock()
	for key, entry := range t.informers {
		if t.factory != nil {
			informer := t.factory.ForResource(entry.gvr).Informer()
			if err := informer.RemoveEventHandler(entry.reg); err != nil {
				t.logger.Error("failed to remove event handler during stop", "error", err, "key", key)
			}
		}
		t.logger.Debug("removed handler", "key", key)
	}
	t.informers = make(map[string]informerEntry)
	t.informersMu.Unlock()

	t.stopOnce.Do(func() {
		close(t.stopCh)
	})
	t.wg.Wait()
	t.running.Store(false)
}

// IsSynced returns true if the initial sync is complete
func (t *InstanceTracker) IsSynced() bool {
	return t.synced.Load()
}

// IsRunning returns true if the tracker is running
// Note: Primarily used for testing and health checks
func (t *InstanceTracker) IsRunning() bool {
	return t.running.Load()
}

// Cache returns the instance cache
// Note: Primarily used for testing purposes
func (t *InstanceTracker) Cache() *InstanceCache {
	return t.cache
}

// SetOnChangeCallback adds a callback to be invoked when instances change.
// Multiple callbacks can be registered and all will be called.
func (t *InstanceTracker) SetOnChangeCallback(cb InstanceChangeCallback) {
	t.onChangeCallbacks = append(t.onChangeCallbacks, cb)
}

// SetOnUpdateCallback adds a callback to be invoked with instance details when instances change.
// Multiple callbacks can be registered and all will be called.
func (t *InstanceTracker) SetOnUpdateCallback(cb InstanceUpdateCallback) {
	t.onUpdateCallbacks = append(t.onUpdateCallbacks, cb)
}

// notifyChange invokes all registered change callbacks
func (t *InstanceTracker) notifyChange() {
	for i, cb := range t.onChangeCallbacks {
		func(idx int, callback InstanceChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					t.logger.Error("panic in onChangeCallback", "index", idx, "error", r)
				}
			}()
			callback()
		}(i, cb)
	}
}

// notifyUpdate invokes all registered update callbacks with instance details
func (t *InstanceTracker) notifyUpdate(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
	for i, cb := range t.onUpdateCallbacks {
		func(idx int, callback InstanceUpdateCallback) {
			defer func() {
				if r := recover(); r != nil {
					t.logger.Error("panic in onUpdateCallback", "index", idx, "error", r)
				}
			}()
			callback(action, namespace, kind, name, instance)
		}(i, cb)
	}
	// Also call the change callbacks
	t.notifyChange()
}

// startInformersForKnownRGDs starts informers for all RGDs in the cache
func (t *InstanceTracker) startInformersForKnownRGDs() {
	if t.rgdWatcher == nil {
		return
	}

	rgds := t.rgdWatcher.All()
	for _, rgd := range rgds {
		if rgd.APIVersion != "" && rgd.Kind != "" {
			t.ensureInformerForRGD(rgd)
		}
	}
}

// handleRGDChange is called when RGDs are added, updated, or deleted
func (t *InstanceTracker) handleRGDChange() {
	if t.rgdWatcher == nil {
		return
	}

	// Get all current RGDs
	rgds := t.rgdWatcher.All()
	currentKeys := make(map[string]bool)

	// Start informers for any new RGDs
	for _, rgd := range rgds {
		if rgd.APIVersion != "" && rgd.Kind != "" {
			gvr := t.gvrFromRGD(rgd)
			key := t.informerKey(rgd, gvr)
			currentKeys[key] = true
			t.ensureInformerForRGD(rgd)
		}
	}

	// Remove handlers for RGDs that no longer exist and purge their cached instances.
	// The shared informer itself keeps running — other RGDs may share the same GVR.
	t.informersMu.Lock()
	for key, entry := range t.informers {
		if !currentKeys[key] {
			// Remove event handler (don't stop the informer — other RGDs may share it)
			if t.factory != nil {
				informer := t.factory.ForResource(entry.gvr).Informer()
				if err := informer.RemoveEventHandler(entry.reg); err != nil {
					t.logger.Error("failed to remove event handler", "error", err, "key", key)
				}
			}

			// Purge cached instances for this RGD
			removed := t.cache.DeleteByRGD(entry.rgd.namespace, entry.rgd.name)
			for _, inst := range removed {
				t.notifyUpdate(InstanceActionDelete, inst.Namespace, inst.Kind, inst.Name, nil)
			}

			delete(t.informers, key)
			t.logger.Info("removed handler for deleted RGD", "key", key)
		}
	}
	t.informersMu.Unlock()
}

// informerKey generates a unique key for an informer based on RGD and GVR
func (t *InstanceTracker) informerKey(rgd *models.CatalogRGD, gvr schema.GroupVersionResource) string {
	// Use namespace/name@gvr format to ensure each RGD gets its own informer
	// even if multiple RGDs share the same GVR
	if rgd.Namespace == "" {
		return fmt.Sprintf("cluster/%s@%s", rgd.Name, gvr.String())
	}
	return fmt.Sprintf("%s/%s@%s", rgd.Namespace, rgd.Name, gvr.String())
}

// ensureInformerForRGD ensures an event handler is registered on a shared informer
// for the given RGD type. Multiple RGDs sharing the same GVR share one informer
// but get separate event handlers that filter by RGD ownership.
func (t *InstanceTracker) ensureInformerForRGD(rgd *models.CatalogRGD) {
	if t.factory == nil {
		return
	}

	gvr := t.gvrFromRGD(rgd)
	key := t.informerKey(rgd, gvr)

	// Register handler under lock, but defer factory.Start outside the critical section
	// to avoid holding informersMu while the factory acquires its own internal lock.
	var needsStart bool
	var informer cache.SharedIndexInformer

	t.informersMu.Lock()

	// Check if handler already registered for this specific RGD
	if _, exists := t.informers[key]; exists {
		t.informersMu.Unlock()
		return
	}

	// Get or create informer from shared factory — same GVR returns same informer
	informer = t.factory.ForResource(gvr).Informer()

	// Add event handler for this specific RGD
	reg, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			t.handleInstanceAdd(obj, rgd.Name, rgd.Namespace)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			t.handleInstanceUpdate(oldObj, newObj, rgd.Name, rgd.Namespace)
		},
		DeleteFunc: func(obj interface{}) {
			t.handleInstanceDelete(obj)
		},
	})
	if err != nil {
		t.informersMu.Unlock()
		t.logger.Error("failed to add event handler", "error", err, "gvr", gvr.String())
		return
	}

	t.informers[key] = informerEntry{
		reg: reg,
		gvr: gvr,
		rgd: rgdRef{namespace: rgd.Namespace, name: rgd.Name},
	}
	needsStart = true

	t.informersMu.Unlock()

	// Start factory outside the lock — idempotent, already-running informers are not restarted
	if needsStart {
		t.factory.Start(t.stopCh)
	}

	t.logger.Info("registered handler for RGD",
		"key", key,
		"rgd", rgd.Name,
		"namespace", rgd.Namespace,
		"gvr", gvr.String())

	// Wait for informer cache sync in background
	go func() {
		if !cache.WaitForCacheSync(t.stopCh, informer.HasSynced) {
			t.logger.Error("informer cache failed to sync", "gvr", gvr.String())
		} else {
			t.logger.Info("informer cache synced", "gvr", gvr.String())
		}
	}()
}

// parseAPIVersion splits an apiVersion string into group and version.
// For "group/version" format (e.g., "example.com/v1"), returns (group, version).
// For version-only format (e.g., "v1alpha1"), defaults group to kro.RGDGroup
// for alpha/beta versions, or "" for stable core K8s versions (e.g., "v1").
func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Version-only apiVersion (e.g., "v1" or "v1alpha1")
	version = apiVersion
	// Kro-created RGDs often only specify version (e.g., "v1alpha1")
	// and Kro adds the KRO domain as the group when creating the CRD.
	// Core Kubernetes resources use stable versions (e.g., "v1", "v2") with empty group.
	// So we default to the KRO group only for alpha/beta versions.
	if strings.Contains(version, "alpha") || strings.Contains(version, "beta") {
		group = kro.RGDGroup
	}
	return group, version
}

// gvrFromRGD extracts the GVR from an RGD using discovery client
func (t *InstanceTracker) gvrFromRGD(rgd *models.CatalogRGD) schema.GroupVersionResource {
	group, version := parseAPIVersion(rgd.APIVersion)

	// Try to use discovery client to resolve GVR properly
	if t.discoveryClient != nil {
		gvk := schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    rgd.Kind,
		}

		if gvr, err := t.resolveGVRFromGVK(gvk); err == nil {
			t.logger.Debug("resolved GVR using discovery",
				"kind", rgd.Kind,
				"gvr", gvr.String())
			return gvr
		} else {
			t.logger.Warn("failed to resolve GVR using discovery, falling back to simple pluralization",
				"kind", rgd.Kind,
				"error", err)
		}
	}

	// Fallback: Convert Kind to resource (lowercase plural)
	// This is a simple pluralization - may not work for all cases
	resource := strings.ToLower(rgd.Kind) + "s"

	t.logger.Warn("using simple pluralization for resource name",
		"kind", rgd.Kind,
		"resource", resource)

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
}

// resolveGVRFromGVK uses the discovery client to resolve a GVK to GVR
func (t *InstanceTracker) resolveGVRFromGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// Get API resources from discovery client
	groupResources, err := restmapper.GetAPIGroupResources(t.discoveryClient)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to get API group resources: %w", err)
	}

	// Create REST mapper
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Map GVK to GVR
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, fmt.Errorf("failed to map GVK to GVR: %w", err)
	}

	return mapping.Resource, nil
}

// ResolveGVR resolves a GroupVersionResource from an apiVersion string and kind
// using the discovery client. Falls back to naive pluralization if discovery fails.
// The returned error is nil even on discovery failure — callers always get a best-effort GVR.
func (t *InstanceTracker) ResolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	group, version := parseAPIVersion(apiVersion)

	if t.discoveryClient != nil {
		gvk := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
		gvr, err := t.resolveGVRFromGVK(gvk)
		if err == nil {
			return gvr, nil
		}
		t.logger.Warn("GVR resolution via discovery failed, falling back to naive pluralization",
			"kind", kind, "error", err)
	} else {
		t.logger.Warn("discovery client unavailable, falling back to naive pluralization",
			"kind", kind)
	}

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(kind) + "s",
	}, nil
}

// handleInstanceAdd processes a new instance
func (t *InstanceTracker) handleInstanceAdd(obj interface{}, rgdName, rgdNamespace string) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		t.logger.Error("unexpected object type in add handler", "type", obj)
		return
	}

	// Check if this instance belongs to the expected RGD
	if !t.belongsToRGD(u, rgdName, rgdNamespace) {
		return
	}

	instance := t.unstructuredToInstance(u, rgdName, rgdNamespace)
	t.cache.Set(instance)
	t.logger.Info("added instance",
		"name", instance.Name,
		"namespace", instance.Namespace,
		"rgd", rgdName)
	t.notifyUpdate(InstanceActionAdd, instance.Namespace, instance.Kind, instance.Name, instance)
}

// handleInstanceUpdate processes an updated instance
func (t *InstanceTracker) handleInstanceUpdate(oldObj, newObj interface{}, rgdName, rgdNamespace string) {
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		t.logger.Error("unexpected object type in update handler", "type", newObj)
		return
	}

	// Check if this instance belongs to the expected RGD
	if !t.belongsToRGD(u, rgdName, rgdNamespace) {
		// Instance doesn't belong to this RGD. However, since multiple RGDs can produce
		// the same CRD type, we must be careful not to delete instances owned by other RGDs.
		// Only delete if the cached instance was previously owned by THIS RGD (disassociation case).
		cachedInstance, exists := t.cache.Get(parser.GetNamespace(u), parser.GetKind(u), parser.GetName(u))
		if exists && cachedInstance.RGDName == rgdName && cachedInstance.RGDNamespace == rgdNamespace {
			// The cached instance belonged to this RGD but is now disassociated - delete it
			t.cache.Delete(parser.GetNamespace(u), parser.GetKind(u), parser.GetName(u))
			t.notifyUpdate(InstanceActionDelete, parser.GetNamespace(u), parser.GetKind(u), parser.GetName(u), nil)
			t.logger.Debug("deleted disassociated instance",
				"name", parser.GetName(u),
				"namespace", parser.GetNamespace(u),
				"rgd", rgdName)
		}
		// Otherwise, ignore - the instance belongs to a different RGD
		return
	}

	instance := t.unstructuredToInstance(u, rgdName, rgdNamespace)
	t.cache.Set(instance)
	t.logger.Debug("updated instance",
		"name", instance.Name,
		"namespace", instance.Namespace,
		"health", instance.Health)
	t.notifyUpdate(InstanceActionUpdate, instance.Namespace, instance.Kind, instance.Name, instance)
}

// handleInstanceDelete processes a deleted instance
func (t *InstanceTracker) handleInstanceDelete(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Handle DeletedFinalStateUnknown
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			t.logger.Error("unexpected object type in delete handler", "type", obj)
			return
		}
		u, ok = tombstone.Obj.(*unstructured.Unstructured)
		if !ok {
			t.logger.Error("unexpected tombstone object type", "type", tombstone.Obj)
			return
		}
	}

	namespace := parser.GetNamespace(u)
	kind := parser.GetKind(u)
	name := parser.GetName(u)
	t.cache.Delete(namespace, kind, name)
	t.logger.Debug("deleted instance",
		"name", name,
		"namespace", namespace,
		"kind", kind)
	t.notifyUpdate(InstanceActionDelete, namespace, kind, name, nil)
}

// belongsToRGD checks if an instance belongs to a specific RGD
func (t *InstanceTracker) belongsToRGD(u *unstructured.Unstructured, rgdName, rgdNamespace string) bool {
	labels := parser.GetLabels(u)
	if len(labels) == 0 {
		return false
	}

	// Use LabelWithFallback for forward-compatible label resolution across KRO versions.
	// KRO's label migration (KREP) moves labels from "kro.run/" to "internal.kro.run/".
	// Try the new prefix first; fall back to legacy for pre-migration instances.
	labelRGDName := metadata.LabelWithFallback(labels, metadata.InternalResourceGraphDefinitionNameLabel, metadata.ResourceGraphDefinitionNameLabel)

	// Check if RGD name matches
	if labelRGDName != rgdName {
		return false
	}

	// RGDs are cluster-scoped, so we accept instances from any namespace
	// Note: rgdNamespace parameter is kept for API compatibility but not used for filtering
	return true
}

// unstructuredToInstance converts an unstructured CR to an Instance model
func (t *InstanceTracker) unstructuredToInstance(u *unstructured.Unstructured, rgdName, rgdNamespace string) *models.Instance {
	labels := parser.GetLabels(u)
	if labels == nil {
		labels = make(map[string]string)
	}

	annotations := parser.GetAnnotations(u)
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Extract spec and status using parser library
	spec := parser.GetSpecOrEmpty(u)
	statusMap := parser.GetStatusOrEmpty(u)

	// Extract conditions from status
	conditions := t.extractConditions(statusMap)

	// Calculate health
	health := t.calculateHealth(conditions, statusMap)

	// Extract phase and message from status
	phase := parser.GetStatusFieldStringOrDefault(u, "", "phase")
	message := parser.GetStatusFieldStringOrDefault(u, "", "message")

	// Parse timestamps
	createdAt := parser.GetCreationTimestamp(u)
	updatedAt := createdAt
	if len(conditions) > 0 {
		// Use the most recent condition transition time
		for _, c := range conditions {
			if c.LastTransitionTime.After(updatedAt) {
				updatedAt = c.LastTransitionTime
			}
		}
	}

	// Look up parent RGD fields from the catalog cache
	var rgdStatus, rgdIcon, rgdCategory string
	var isClusterScoped bool
	var rgdProjectName string
	if t.rgdWatcher != nil {
		if parentRGD, found := t.rgdWatcher.GetRGD(rgdNamespace, rgdName); found {
			rgdStatus = parentRGD.Status
			rgdIcon = parentRGD.Icon
			rgdCategory = parentRGD.Category
			isClusterScoped = parentRGD.IsClusterScoped
			if parentRGD.Labels != nil {
				rgdProjectName = parentRGD.Labels[kro.RGDProjectLabel]
			}
		}
	}

	// Resolve project identity from instance labels/annotations, falling back to
	// the parent RGD's project label. Instances deployed via Knodex have both
	// a "knodex.io/project" label and a "knodex.io/project-id" annotation set by
	// the deployment generator. For instances deployed outside Knodex (e.g., kubectl),
	// only the parent RGD's project label is available.
	projectName := labels[kro.RGDProjectLabel]
	if projectName == "" {
		projectName = rgdProjectName
	}
	projectID := annotations[models.AnnotationProjectID]
	if projectID == "" {
		projectID = projectName // In Knodex, project name = project ID
	}

	return &models.Instance{
		Name:                    parser.GetName(u),
		Namespace:               parser.GetNamespace(u),
		RGDName:                 rgdName,
		RGDNamespace:            rgdNamespace,
		APIVersion:              parser.GetAPIVersion(u),
		Kind:                    parser.GetKind(u),
		Health:                  health,
		Phase:                   phase,
		Message:                 message,
		Conditions:              conditions,
		Spec:                    spec,
		Status:                  statusMap,
		Labels:                  labels,
		Annotations:             annotations,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
		ResourceVersion:         parser.GetResourceVersion(u),
		UID:                     parser.GetUID(u),
		TargetCluster:           annotations[models.AnnotationTargetCluster],
		IsClusterScoped:         isClusterScoped,
		ProjectID:               projectID,
		ProjectName:             projectName,
		RGDStatus:               rgdStatus,
		RGDIcon:                 rgdIcon,
		RGDCategory:             rgdCategory,
		DeploymentMode:          deployment.ParseDeploymentMode(labels[models.DeploymentModeLabel]),
		ReconciliationSuspended: annotations["kro.run/reconcile"] == "suspended",
		GitInfo:                 buildGitInfoFromAnnotations(annotations),
	}
}

// buildGitInfoFromAnnotations constructs a GitInfo from knodex.io/git-* annotations
// written at deploy time. Returns nil if no git annotations are present.
func buildGitInfoFromAnnotations(annotations map[string]string) *deployment.GitInfo {
	repo := annotations["knodex.io/git-repository"]
	branch := annotations["knodex.io/git-branch"]
	path := annotations["knodex.io/git-path"]
	if repo == "" && branch == "" && path == "" {
		return nil
	}
	return &deployment.GitInfo{
		RepositoryURL: repo,
		Branch:        branch,
		Path:          path,
		PushStatus:    deployment.GitPushSuccess,
	}
}

// extractConditions extracts conditions from the status map
func (t *InstanceTracker) extractConditions(status map[string]interface{}) []models.InstanceCondition {
	if status == nil {
		return nil
	}

	conditionsRaw, err := parser.GetSlice(status, "conditions")
	if err != nil {
		return nil
	}

	var conditions []models.InstanceCondition
	for _, c := range conditionsRaw {
		condMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condition := models.InstanceCondition{
			Type:    parser.GetStringOrDefault(condMap, "", "type"),
			Status:  parser.GetStringOrDefault(condMap, "", "status"),
			Reason:  parser.GetStringOrDefault(condMap, "", "reason"),
			Message: parser.GetStringOrDefault(condMap, "", "message"),
		}

		if v := parser.GetStringOrDefault(condMap, "", "lastTransitionTime"); v != "" {
			if parsed, parseErr := time.Parse(time.RFC3339, v); parseErr == nil {
				condition.LastTransitionTime = parsed
			}
		}

		conditions = append(conditions, condition)
	}

	return conditions
}

// calculateHealth determines the health status from conditions and status
func (t *InstanceTracker) calculateHealth(conditions []models.InstanceCondition, status map[string]interface{}) models.InstanceHealth {
	if len(conditions) == 0 {
		// No conditions yet - check phase if available
		if status != nil {
			if phase := parser.GetStringOrDefault(status, "", "phase"); phase != "" {
				switch strings.ToLower(phase) {
				case "running", "ready", "active", "healthy":
					return models.HealthHealthy
				case "pending", "creating", "initializing", "progressing":
					return models.HealthProgressing
				case "failed", "error", "unhealthy":
					return models.HealthUnhealthy
				case "degraded", "warning":
					return models.HealthDegraded
				}
			}
		}
		return models.HealthUnknown
	}

	// Check Ready condition first
	for _, c := range conditions {
		if c.Type == "Ready" {
			switch c.Status {
			case "True":
				return models.HealthHealthy
			case "False":
				// Check reason for more context
				reason := strings.ToLower(c.Reason)
				if strings.Contains(reason, "progress") || strings.Contains(reason, "pending") {
					return models.HealthProgressing
				}
				if strings.Contains(reason, "degrad") {
					return models.HealthDegraded
				}
				return models.HealthUnhealthy
			case "Unknown":
				return models.HealthProgressing
			}
		}
	}

	// Check for any error conditions
	for _, c := range conditions {
		if c.Status == "False" && strings.Contains(strings.ToLower(c.Type), "error") {
			return models.HealthUnhealthy
		}
	}

	// Check for degraded conditions
	for _, c := range conditions {
		if c.Status == "False" {
			return models.HealthDegraded
		}
	}

	// All conditions are True or Unknown
	allTrue := true
	for _, c := range conditions {
		if c.Status != "True" {
			allTrue = false
			break
		}
	}

	if allTrue {
		return models.HealthHealthy
	}

	return models.HealthProgressing
}

// updateInstancesRGDStatus updates the RGDStatus, RGDIcon, and RGDCategory fields on all cached instances
// belonging to the given RGD when the RGD's metadata changes.
func (t *InstanceTracker) updateInstancesRGDStatus(rgd *models.CatalogRGD) {
	instances := t.cache.GetByRGD(rgd.Namespace, rgd.Name)
	for _, inst := range instances {
		if inst.RGDStatus != rgd.Status || inst.RGDIcon != rgd.Icon || inst.RGDCategory != rgd.Category {
			// Clone before mutation to avoid data races with concurrent cache readers
			updated := *inst
			updated.RGDStatus = rgd.Status
			updated.RGDIcon = rgd.Icon
			updated.RGDCategory = rgd.Category
			t.cache.Set(&updated)
			t.notifyUpdate(InstanceActionUpdate, updated.Namespace, updated.Kind, updated.Name, &updated)
		}
	}
}

// ListInstances returns instances matching the given options
func (t *InstanceTracker) ListInstances(opts models.InstanceListOptions) models.InstanceList {
	return t.cache.List(opts)
}

// GetInstance returns a single instance by namespace, kind, and name
func (t *InstanceTracker) GetInstance(namespace, kind, name string) (*models.Instance, bool) {
	return t.cache.Get(namespace, kind, name)
}

// GetInstanceByUID returns the instance with the given Kubernetes UID.
func (t *InstanceTracker) GetInstanceByUID(uid string) (*models.Instance, bool) {
	return t.cache.GetByUID(uid)
}

// GetInstancesByRGD returns all instances for a specific RGD
// Note: Primarily used for testing purposes
func (t *InstanceTracker) GetInstancesByRGD(rgdNamespace, rgdName string) []*models.Instance {
	return t.cache.GetByRGD(rgdNamespace, rgdName)
}

// CountInstancesByRGD returns the instance count for a specific RGD
func (t *InstanceTracker) CountInstancesByRGD(rgdNamespace, rgdName string) int {
	return t.cache.CountByRGD(rgdNamespace, rgdName)
}

// CountInstancesByNamespaces returns the count of instances accessible to the user.
// namespaces: list of namespace patterns (["*"] = all, empty = none)
// matchFunc: function to match a namespace against patterns (e.g., rbac.MatchNamespaceInList)
func (t *InstanceTracker) CountInstancesByNamespaces(namespaces []string, matchFunc func(namespace string, patterns []string) bool) int {
	return t.cache.CountByNamespaces(namespaces, matchFunc)
}

// CountInstancesByRGDAndNamespaces returns the count of instances for a specific RGD
// filtered by the user's accessible namespaces.
// namespaces: list of namespace patterns (["*"] = all, empty = none)
// matchFunc: function to match a namespace against patterns (e.g., rbac.MatchNamespaceInList)
func (t *InstanceTracker) CountInstancesByRGDAndNamespaces(rgdNamespace, rgdName string, namespaces []string, matchFunc func(namespace string, patterns []string) bool) int {
	return t.cache.CountByRGDAndNamespaces(rgdNamespace, rgdName, namespaces, matchFunc)
}

// CountFilteredInstances returns the count of instances matching the given predicate.
// Use this when both namespace and project-based filtering is required (e.g., for
// non-admin users who need cluster-scoped instance counts filtered by project access).
func (t *InstanceTracker) CountFilteredInstances(filter func(*models.Instance) bool) int {
	return t.cache.CountFiltered(filter)
}

// DeleteInstance deletes an instance from the cluster
func (t *InstanceTracker) DeleteInstance(ctx context.Context, namespace, name, apiVersion, kind string) error {
	// Resolve GVR via discovery with naive-pluralization fallback.
	// ResolveGVR handles apiVersion parsing, discovery, fallback, and warning logs internally.
	gvr, _ := t.ResolveGVR(apiVersion, kind)

	// Scope-aware delete: cluster-scoped instances have empty namespace
	var err error
	if namespace == "" {
		slog.Debug("deleting cluster-scoped instance", "kind", kind, "name", name, "gvr", gvr)
		err = t.dynamicClient.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		err = t.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
	if err != nil {
		return err
	}

	// Proactively remove from cache as a safety net.
	// The informer handler is idempotent, so a duplicate delete is harmless.
	// This ensures cleanup even if the informer isn't running (e.g., RGD was deleted).
	t.cache.Delete(namespace, kind, name)
	t.notifyUpdate(InstanceActionDelete, namespace, kind, name, nil)

	return nil
}
