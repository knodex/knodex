// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/knodex/knodex/server/internal/config"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kro"
	"github.com/knodex/knodex/server/internal/models"
)

// ChangeCallback is called when RGDs change in the cache
type ChangeCallback func()

// RGDAction represents the type of RGD change
type RGDAction string

const (
	// RGDActionAdd indicates a new RGD was added
	RGDActionAdd RGDAction = "add"
	// RGDActionUpdate indicates an RGD was updated
	RGDActionUpdate RGDAction = "update"
	// RGDActionDelete indicates an RGD was deleted
	RGDActionDelete RGDAction = "delete"
)

// RGDUpdateCallback is called when an individual RGD changes
type RGDUpdateCallback func(action RGDAction, name string, rgd *models.CatalogRGD)

// RGDWatcher watches for ResourceGraphDefinition resources in the cluster
type RGDWatcher struct {
	dynamicClient     dynamic.Interface
	cache             *RGDCache
	informer          cache.SharedIndexInformer
	stopCh            chan struct{}
	done              chan struct{} // signals informer goroutine has exited
	synced            atomic.Bool
	running           atomic.Bool
	logger            *slog.Logger
	onChangeCallbacks []ChangeCallback
	onUpdateCallbacks []RGDUpdateCallback

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// rgdGVR returns the GroupVersionResource for KRO ResourceGraphDefinitions.
// Delegates to kro.RGDGVR() for the canonical GVR definition.
var rgdGVR = kro.RGDGVR()

// NewRGDWatcher creates a new RGD watcher
func NewRGDWatcher(cfg *config.Kubernetes) (*RGDWatcher, error) {
	var restConfig *rest.Config
	var err error

	if cfg.InCluster {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		restConfig, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &RGDWatcher{
		dynamicClient: dynamicClient,
		cache:         NewRGDCache(),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		logger:        slog.Default().With("component", "rgd-watcher"),
	}, nil
}

// NewRGDWatcherWithClient creates a watcher with an existing dynamic client (for testing)
func NewRGDWatcherWithClient(client dynamic.Interface) *RGDWatcher {
	return &RGDWatcher{
		dynamicClient: client,
		cache:         NewRGDCache(),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		logger:        slog.Default().With("component", "rgd-watcher"),
	}
}

// NewRGDWatcherWithCache creates a watcher with an existing cache (for testing)
func NewRGDWatcherWithCache(cache *RGDCache) *RGDWatcher {
	w := &RGDWatcher{
		cache:  cache,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
		logger: slog.Default().With("component", "rgd-watcher"),
	}
	// Mark as running and synced for tests
	w.running.Store(true)
	w.synced.Store(true)
	return w
}

// DynamicClient returns the underlying dynamic Kubernetes client
func (w *RGDWatcher) DynamicClient() dynamic.Interface {
	return w.dynamicClient
}

// Start begins watching for RGD resources
func (w *RGDWatcher) Start(ctx context.Context) error {
	if w.running.Load() {
		w.logger.Warn("watcher already running")
		return nil
	}

	w.logger.Info("starting RGD watcher")

	// Reinitialize channels for fresh start - allows restart after stop
	w.stopCh = make(chan struct{})
	w.done = make(chan struct{})
	// Reset stopOnce when starting fresh
	w.stopOnce = sync.Once{}

	// Create dynamic informer factory
	// Resync every 30 seconds to ensure we don't miss events
	factory := dynamicinformer.NewDynamicSharedInformerFactory(w.dynamicClient, 30*time.Second)

	// Get informer for RGDs (all namespaces)
	w.informer = factory.ForResource(rgdGVR).Informer()

	// Add event handlers
	_, err := w.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.handleAdd,
		UpdateFunc: w.handleUpdate,
		DeleteFunc: w.handleDelete,
	})
	if err != nil {
		return err
	}

	// Start informer in background
	go func() {
		defer close(w.done) // Signal completion when goroutine exits
		w.running.Store(true)
		w.informer.Run(w.stopCh)
		w.running.Store(false)
		w.logger.Info("RGD watcher stopped")
	}()

	// Wait for initial sync
	go func() {
		if !cache.WaitForCacheSync(ctx.Done(), w.informer.HasSynced) {
			w.logger.Error("failed to sync RGD cache")
			return
		}
		w.synced.Store(true)
		w.logger.Info("RGD cache synced", "count", w.cache.Count())

		// Notify callback after initial sync so graph can be built
		w.notifyChange()
	}()

	return nil
}

// Stop stops the watcher without waiting for completion.
// Safe to call multiple times - uses sync.Once to prevent double-close panic.
func (w *RGDWatcher) Stop() {
	if !w.running.Load() {
		return
	}
	w.logger.Info("stopping RGD watcher")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

// StopAndWait stops the watcher and waits for the informer goroutine to exit.
// Returns true if stopped cleanly, false if timeout was reached.
// This should be used during graceful shutdown to prevent goroutine leaks.
func (w *RGDWatcher) StopAndWait(timeout time.Duration) bool {
	if !w.running.Load() {
		return true
	}

	w.logger.Info("stopping RGD watcher and waiting for completion")
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})

	// Wait for done signal or timeout
	select {
	case <-w.done:
		w.logger.Info("RGD watcher stopped cleanly")
		return true
	case <-time.After(timeout):
		w.logger.Warn("RGD watcher stop timed out", "timeout", timeout)
		return false
	}
}

// IsSynced returns true if the initial cache sync is complete
func (w *RGDWatcher) IsSynced() bool {
	return w.synced.Load()
}

// IsRunning returns true if the watcher is running
func (w *RGDWatcher) IsRunning() bool {
	return w.running.Load()
}

// Cache returns the RGD cache
func (w *RGDWatcher) Cache() *RGDCache {
	return w.cache
}

// SetOnChangeCallback adds a callback to be invoked when RGDs change.
// Multiple callbacks can be registered and all will be called.
func (w *RGDWatcher) SetOnChangeCallback(cb ChangeCallback) {
	w.onChangeCallbacks = append(w.onChangeCallbacks, cb)
}

// notifyChange invokes all registered change callbacks
func (w *RGDWatcher) notifyChange() {
	w.logger.Debug("notifyChange called", "callbackCount", len(w.onChangeCallbacks))
	for i, cb := range w.onChangeCallbacks {
		func(idx int, callback ChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("panic in onChangeCallback", "index", idx, "error", r)
				}
			}()
			callback()
		}(i, cb)
	}
}

// SetOnUpdateCallback adds a callback to be invoked with RGD details when RGDs change.
// Multiple callbacks can be registered and all will be called.
func (w *RGDWatcher) SetOnUpdateCallback(cb RGDUpdateCallback) {
	w.onUpdateCallbacks = append(w.onUpdateCallbacks, cb)
}

// notifyUpdate invokes all registered update callbacks with individual RGD details
func (w *RGDWatcher) notifyUpdate(action RGDAction, name string, rgd *models.CatalogRGD) {
	w.logger.Debug("notifyUpdate called", "action", action, "name", name, "callbackCount", len(w.onUpdateCallbacks))
	for i, cb := range w.onUpdateCallbacks {
		func(idx int, callback RGDUpdateCallback) {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("panic in onUpdateCallback", "index", idx, "error", r)
				}
			}()
			callback(action, name, rgd)
		}(i, cb)
	}
}

// All returns all cached RGDs
func (w *RGDWatcher) All() []*models.CatalogRGD {
	return w.cache.All()
}

// handleAdd processes a new RGD
func (w *RGDWatcher) handleAdd(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		w.logger.Error("unexpected object type in add handler", "type", obj)
		return
	}

	// Check if RGD should be in catalog
	if !w.shouldIncludeInCatalog(u) {
		w.logger.Debug("skipping RGD (not eligible for catalog)",
			"name", u.GetName(),
			"namespace", u.GetNamespace())
		return
	}

	rgd := w.unstructuredToRGD(u)
	w.cache.Set(rgd)
	w.logger.Info("added RGD to catalog",
		"name", rgd.Name,
		"namespace", rgd.Namespace,
		"tags", rgd.Tags)
	w.notifyChange()
	w.notifyUpdate(RGDActionAdd, rgd.Name, rgd)
}

// handleUpdate processes an updated RGD
func (w *RGDWatcher) handleUpdate(oldObj, newObj interface{}) {
	u, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		w.logger.Error("unexpected object type in update handler", "type", newObj)
		return
	}

	oldU, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		w.logger.Error("unexpected old object type in update handler", "type", oldObj)
		return
	}

	// Check if RGD should be in catalog
	if !w.shouldIncludeInCatalog(u) {
		// If it was previously in catalog, remove it
		w.cache.Delete(u.GetNamespace(), u.GetName())
		w.logger.Debug("removed RGD from catalog (no longer eligible)",
			"name", u.GetName(),
			"namespace", u.GetNamespace())
		w.notifyChange()
		w.notifyUpdate(RGDActionDelete, u.GetName(), nil)
		return
	}

	rgd := w.unstructuredToRGD(u)

	// Only update the timestamp if the resource actually changed
	// Compare resourceVersion to detect real changes vs re-sync events
	if oldU.GetResourceVersion() != u.GetResourceVersion() {
		rgd.UpdatedAt = time.Now()
	} else {
		// Keep the existing updatedAt timestamp for re-sync events
		if existingRGD, found := w.cache.Get(rgd.Namespace, rgd.Name); found {
			rgd.UpdatedAt = existingRGD.UpdatedAt
		}
	}

	w.cache.Set(rgd)
	w.logger.Debug("updated RGD in catalog",
		"name", rgd.Name,
		"namespace", rgd.Namespace)
	w.notifyChange()
	w.notifyUpdate(RGDActionUpdate, rgd.Name, rgd)
}

// handleDelete processes a deleted RGD
func (w *RGDWatcher) handleDelete(obj interface{}) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// Handle DeletedFinalStateUnknown
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

	w.cache.Delete(u.GetNamespace(), u.GetName())
	w.logger.Info("removed RGD from catalog",
		"name", u.GetName(),
		"namespace", u.GetNamespace())
	w.notifyChange()
	w.notifyUpdate(RGDActionDelete, u.GetName(), nil)
}

// shouldIncludeInCatalog checks if the RGD has the catalog annotation and is Active.
// Simplified visibility model:
// - knodex.io/catalog: "true" is the GATEWAY to the catalog
// - RGDs without this annotation are NOT part of the catalog system (invisible to everyone)
// - catalog: true alone = visible to ALL authenticated users (public)
// - catalog: true + project label = visible to project members only
// - status.state must be "Active" — Inactive or unprocessed RGDs are excluded
func (w *RGDWatcher) shouldIncludeInCatalog(u *unstructured.Unstructured) bool {
	// Check for catalog annotation using parser helper
	value, ok := parser.GetAnnotation(u, kro.CatalogAnnotation)
	if !ok {
		return false
	}

	// Accept "true", "yes", "1"
	value = strings.ToLower(value)
	if value != "true" && value != "yes" && value != "1" {
		return false
	}

	// Check KRO status - only include Active RGDs
	// Empty status means KRO hasn't processed this RGD yet (exclude)
	state := parser.GetStatusFieldStringOrDefault(u, "", "state")
	if state != "Active" {
		w.logger.Debug("skipping inactive RGD",
			"name", parser.GetName(u),
			"state", state)
		return false
	}

	return true
}

// unstructuredToRGD converts an unstructured RGD to our model
func (w *RGDWatcher) unstructuredToRGD(u *unstructured.Unstructured) *models.CatalogRGD {
	// Use parser helpers for safe metadata extraction
	annotations := parser.GetAnnotations(u)
	labels := parser.GetLabels(u)

	// Extract tags from annotation (comma-separated)
	var tags []string
	if tagsStr, ok := parser.GetAnnotation(u, kro.TagsAnnotation); ok && tagsStr != "" {
		for _, tag := range strings.Split(tagsStr, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}

	// Extract version from annotation or fall back to "v1"
	version := parser.GetAnnotationOrDefault(u, kro.VersionAnnotation, "v1")

	// Extract API version and kind from spec.schema (RGD structure)
	// Using parser's type-safe field accessors
	apiVersion := parser.GetSpecFieldStringOrDefault(u, "", "schema", "apiVersion")
	kind := parser.GetSpecFieldStringOrDefault(u, "", "schema", "kind")

	// Fallback to spec level (legacy or alternative format)
	if apiVersion == "" {
		apiVersion = parser.GetSpecFieldStringOrDefault(u, "", "apiVersion")
	}
	if kind == "" {
		kind = parser.GetSpecFieldStringOrDefault(u, "", "kind")
	}

	// If apiVersion doesn't contain a group (no "/"), prepend the RGD's group
	// Resources created by an RGD are in the same group as the RGD itself
	if apiVersion != "" && !strings.Contains(apiVersion, "/") {
		rgdAPIVersion := parser.GetAPIVersion(u)
		if parts := strings.Split(rgdAPIVersion, "/"); len(parts) == 2 {
			apiVersion = parts[0] + "/" + apiVersion
		}
	}

	// Parse creation timestamp using parser helper
	createdAt := parser.GetCreationTimestamp(u)

	// Use creation timestamp as the base for updatedAt
	// This will be overridden by handleUpdate when actual changes occur
	updatedAt := createdAt

	// Store raw spec for dependency parsing
	rawSpec := parser.GetSpecOrEmpty(u)

	// Extract KRO status state (defaults to "Inactive" if not yet processed)
	status := parser.GetStatusFieldStringOrDefault(u, "Inactive", "state")

	// Parse allowed deployment modes from annotation
	var allowedModes []string
	if modesStr, ok := parser.GetAnnotation(u, kro.DeploymentModesAnnotation); ok {
		result := models.ParseDeploymentModesWithInvalid(modesStr)
		allowedModes = result.ValidModes
		// Log warning for invalid modes
		if len(result.InvalidModes) > 0 {
			w.logger.Warn("RGD has invalid deployment mode values in annotation",
				"name", parser.GetName(u),
				"invalidModes", strings.Join(result.InvalidModes, ","),
				"validModes", strings.Join(result.ValidModes, ","))
		}
	}

	// Extract organization label (empty = shared RGD, not scoped to any org)
	organization := strings.TrimSpace(parser.GetLabelOrDefault(u, kro.RGDOrganizationLabel, ""))

	// Extract display title from annotation, falling back to the K8s resource name
	title := parser.GetAnnotationOrDefault(u, kro.TitleAnnotation, parser.GetName(u))
	if title == "" {
		title = parser.GetName(u)
	}

	return &models.CatalogRGD{
		Name:                   parser.GetName(u),
		Title:                  title,
		Namespace:              parser.GetNamespace(u),
		Description:            parser.GetAnnotationOrDefault(u, kro.DescriptionAnnotation, ""),
		Version:                version,
		Tags:                   tags,
		Category:               parser.GetAnnotationOrDefault(u, kro.CategoryAnnotation, ""),
		Icon:                   parser.GetAnnotationOrDefault(u, kro.IconAnnotation, ""),
		Organization:           organization,
		Labels:                 labels,
		Annotations:            annotations,
		InstanceCount:          0, // Will be populated by instance tracker
		APIVersion:             apiVersion,
		Kind:                   kind,
		Status:                 status,
		AllowedDeploymentModes: allowedModes,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
		ResourceVersion:        parser.GetResourceVersion(u),
		RawSpec:                rawSpec,
	}
}

// ListRGDs returns all RGDs matching the given options
func (w *RGDWatcher) ListRGDs(opts models.ListOptions) models.CatalogRGDList {
	return w.cache.List(opts)
}

// GetRGD returns a single RGD by namespace and name
func (w *RGDWatcher) GetRGD(namespace, name string) (*models.CatalogRGD, bool) {
	return w.cache.Get(namespace, name)
}

// GetRGDByName searches for an RGD by name across all namespaces
// Returns the first match if found
func (w *RGDWatcher) GetRGDByName(name string) (*models.CatalogRGD, bool) {
	all := w.cache.All()
	for _, rgd := range all {
		if rgd.Name == name {
			return rgd, true
		}
	}
	return nil, false
}

// GetRGDByKind searches for an RGD by its Kind across all namespaces.
// Returns the first match if found. If multiple RGDs share the same Kind,
// a warning is logged since the result may be non-deterministic.
// This is used by the schema enricher for cross-RGD externalRef Kind resolution.
func (w *RGDWatcher) GetRGDByKind(kind string) (*models.CatalogRGD, bool) {
	all := w.cache.All()
	var match *models.CatalogRGD
	var matchCount int
	for _, rgd := range all {
		if rgd.Kind == kind {
			matchCount++
			if match == nil {
				match = rgd
			}
		}
	}
	if matchCount > 1 {
		w.logger.Warn("multiple RGDs found with same Kind, using first match",
			"kind", kind,
			"matchCount", matchCount,
			"selectedRGD", match.Name,
			"selectedNamespace", match.Namespace)
	}
	if match != nil {
		return match, true
	}
	return nil, false
}

// RefreshRGD forces a re-fetch of a specific RGD from the cluster
func (w *RGDWatcher) RefreshRGD(ctx context.Context, namespace, name string) error {
	u, err := w.dynamicClient.Resource(rgdGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if w.shouldIncludeInCatalog(u) {
		rgd := w.unstructuredToRGD(u)
		w.cache.Set(rgd)
	} else {
		w.cache.Delete(namespace, name)
	}

	return nil
}
