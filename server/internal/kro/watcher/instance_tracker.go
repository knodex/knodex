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

	// instanceTrackerResyncPeriod is how often to re-list instance resources from K8s API.
	// 30s balances real-time instance visibility with API server load.
	instanceTrackerResyncPeriod = 30 * time.Second
)

// InstanceUpdateCallback is called when instances are added, updated, or deleted
// with details about what changed
type InstanceUpdateCallback func(action InstanceAction, namespace, kind, name string, instance *models.Instance)

// rgdRef holds the namespace and name of an RGD associated with an informer
type rgdRef struct {
	namespace string
	name      string
}

// InstanceTracker watches for instances (CRs) created by RGDs
type InstanceTracker struct {
	dynamicClient   dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	cache           *InstanceCache
	rgdWatcher      *RGDWatcher

	// Map of informer key (namespace/name@gvr) to informer stop channel
	informers   map[string]chan struct{}
	informersMu sync.RWMutex

	// Map of informer key to the RGD it tracks (for cache cleanup on stop)
	informerRGDs map[string]rgdRef

	stopCh            chan struct{}
	synced            atomic.Bool
	running           atomic.Bool
	logger            *slog.Logger
	onChangeCallbacks []InstanceChangeCallback
	onUpdateCallbacks []InstanceUpdateCallback

	// resyncPeriod for informers
	resyncPeriod time.Duration
}

// NewInstanceTracker creates a new instance tracker
func NewInstanceTracker(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface, rgdWatcher *RGDWatcher) *InstanceTracker {
	return &InstanceTracker{
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		cache:           NewInstanceCache(),
		rgdWatcher:      rgdWatcher,
		informers:       make(map[string]chan struct{}),
		informerRGDs:    make(map[string]rgdRef),
		stopCh:          make(chan struct{}),
		logger:          slog.Default().With("component", "instance-tracker"),
		resyncPeriod:    instanceTrackerResyncPeriod,
	}
}

// NewInstanceTrackerWithCache creates a tracker with an existing cache (for testing)
func NewInstanceTrackerWithCache(cache *InstanceCache) *InstanceTracker {
	t := &InstanceTracker{
		cache:        cache,
		informers:    make(map[string]chan struct{}),
		informerRGDs: make(map[string]rgdRef),
		stopCh:       make(chan struct{}),
		logger:       slog.Default().With("component", "instance-tracker"),
		resyncPeriod: instanceTrackerResyncPeriod,
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
	}

	// Mark as synced after initial informers are started
	go func() {
		// Give informers time to sync
		time.Sleep(2 * time.Second)
		t.synced.Store(true)
		t.logger.Info("instance tracker synced", "count", t.cache.Count())
	}()

	return nil
}

// Stop stops all informers
func (t *InstanceTracker) Stop() {
	if !t.running.Load() {
		return
	}

	t.logger.Info("stopping instance tracker")

	// Stop all informers
	t.informersMu.Lock()
	for key, stopCh := range t.informers {
		close(stopCh)
		t.logger.Debug("stopped informer", "key", key)
	}
	t.informers = make(map[string]chan struct{})
	t.informerRGDs = make(map[string]rgdRef)
	t.informersMu.Unlock()

	close(t.stopCh)
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

	// Stop informers for RGDs that no longer exist and purge their cached instances
	t.informersMu.Lock()
	for key, stopCh := range t.informers {
		if !currentKeys[key] {
			close(stopCh)

			// Purge cached instances for this RGD
			if ref, ok := t.informerRGDs[key]; ok {
				removed := t.cache.DeleteByRGD(ref.namespace, ref.name)
				for _, inst := range removed {
					t.notifyUpdate(InstanceActionDelete, inst.Namespace, inst.Kind, inst.Name, nil)
				}
				delete(t.informerRGDs, key)
			}

			delete(t.informers, key)
			t.logger.Info("stopped informer for removed RGD", "key", key)
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

// ensureInformerForRGD ensures an informer exists for the given RGD type
func (t *InstanceTracker) ensureInformerForRGD(rgd *models.CatalogRGD) {
	gvr := t.gvrFromRGD(rgd)
	key := t.informerKey(rgd, gvr)

	t.informersMu.Lock()
	defer t.informersMu.Unlock()

	// Check if informer already exists for this specific RGD
	if _, exists := t.informers[key]; exists {
		return
	}

	// Create new informer for this RGD
	stopCh := make(chan struct{})
	t.informers[key] = stopCh
	t.informerRGDs[key] = rgdRef{namespace: rgd.Namespace, name: rgd.Name}

	t.logger.Info("starting informer for RGD",
		"key", key,
		"rgd", rgd.Name,
		"namespace", rgd.Namespace,
		"gvr", gvr.String())

	go t.runInformer(gvr, rgd.Name, rgd.Namespace, stopCh)
}

// gvrFromRGD extracts the GVR from an RGD using discovery client
func (t *InstanceTracker) gvrFromRGD(rgd *models.CatalogRGD) schema.GroupVersionResource {
	// Parse apiVersion (e.g., "example.com/v1" or "kro.run/v1alpha1")
	parts := strings.Split(rgd.APIVersion, "/")
	var group, version string
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		// Version-only apiVersion (e.g., "v1" or "v1alpha1")
		version = rgd.APIVersion

		// Kro-created RGDs often only specify version (e.g., "v1alpha1")
		// and Kro adds the KRO domain as the group when creating the CRD.
		// Core Kubernetes resources use stable versions (e.g., "v1", "v2") with empty group.
		// So we default to the KRO group only for alpha/beta versions.
		if strings.Contains(version, "alpha") || strings.Contains(version, "beta") {
			group = kro.RGDGroup
		} else {
			// Core Kubernetes resource with stable version (e.g., "v1")
			group = ""
		}
	}

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

// runInformer runs an informer for a specific GVR
func (t *InstanceTracker) runInformer(gvr schema.GroupVersionResource, rgdName, rgdNamespace string, stopCh chan struct{}) {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(t.dynamicClient, t.resyncPeriod)
	informer := factory.ForResource(gvr).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			t.handleInstanceAdd(obj, rgdName, rgdNamespace)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			t.handleInstanceUpdate(oldObj, newObj, rgdName, rgdNamespace)
		},
		DeleteFunc: func(obj interface{}) {
			t.handleInstanceDelete(obj)
		},
	})
	if err != nil {
		t.logger.Error("failed to add event handler", "error", err, "gvr", gvr.String())
		return
	}

	// Start the factory (starts all informers in the factory)
	factory.Start(stopCh)

	// Wait for the informer cache to sync before processing events
	synced := cache.WaitForCacheSync(stopCh, informer.HasSynced)
	if !synced {
		t.logger.Error("informer cache failed to sync", "gvr", gvr.String())
		return
	}

	t.logger.Info("informer cache synced", "gvr", gvr.String())

	// Wait for stop signal
	<-stopCh
	t.logger.Debug("informer stopped", "gvr", gvr.String())
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
	// Currently a single key; the variadic signature supports adding legacy keys if KRO
	// renames labels in a future release.
	labelRGDName := metadata.LabelWithFallback(labels, metadata.ResourceGraphDefinitionNameLabel)

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

	return &models.Instance{
		Name:            parser.GetName(u),
		Namespace:       parser.GetNamespace(u),
		RGDName:         rgdName,
		RGDNamespace:    rgdNamespace,
		APIVersion:      parser.GetAPIVersion(u),
		Kind:            parser.GetKind(u),
		Health:          health,
		Phase:           phase,
		Message:         message,
		Conditions:      conditions,
		Spec:            spec,
		Status:          statusMap,
		Labels:          labels,
		Annotations:     annotations,
		CreatedAt:       createdAt,
		UpdatedAt:       updatedAt,
		ResourceVersion: parser.GetResourceVersion(u),
		UID:             parser.GetUID(u),
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

// ListInstances returns instances matching the given options
func (t *InstanceTracker) ListInstances(opts models.InstanceListOptions) models.InstanceList {
	return t.cache.List(opts)
}

// GetInstance returns a single instance by namespace, kind, and name
func (t *InstanceTracker) GetInstance(namespace, kind, name string) (*models.Instance, bool) {
	return t.cache.Get(namespace, kind, name)
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
// namespaces: list of namespace patterns the user has access to (nil = all, empty = none)
// matchFunc: function to match a namespace against patterns (e.g., rbac.MatchNamespaceInList)
func (t *InstanceTracker) CountInstancesByNamespaces(namespaces []string, matchFunc func(namespace string, patterns []string) bool) int {
	return t.cache.CountByNamespaces(namespaces, matchFunc)
}

// CountInstancesByRGDAndNamespaces returns the count of instances for a specific RGD
// filtered by the user's accessible namespaces.
// namespaces: list of namespace patterns the user has access to (nil = all, empty = none)
// matchFunc: function to match a namespace against patterns (e.g., rbac.MatchNamespaceInList)
func (t *InstanceTracker) CountInstancesByRGDAndNamespaces(rgdNamespace, rgdName string, namespaces []string, matchFunc func(namespace string, patterns []string) bool) int {
	return t.cache.CountByRGDAndNamespaces(rgdNamespace, rgdName, namespaces, matchFunc)
}

// DeleteInstance deletes an instance from the cluster
func (t *InstanceTracker) DeleteInstance(ctx context.Context, namespace, name, apiVersion, kind string) error {
	// Parse apiVersion to get GVR
	parts := strings.Split(apiVersion, "/")
	var group, version string
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		// Version-only apiVersion
		version = apiVersion
		// Default to KRO group for alpha/beta versions, empty for stable versions
		if strings.Contains(version, "alpha") || strings.Contains(version, "beta") {
			group = kro.RGDGroup
		} else {
			group = ""
		}
	}

	resource := strings.ToLower(kind) + "s"
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	err := t.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
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
