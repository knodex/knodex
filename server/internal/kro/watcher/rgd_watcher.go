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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	krov1alpha1 "github.com/kubernetes-sigs/kro/api/v1alpha1"
	krograph "github.com/kubernetes-sigs/kro/pkg/graph"

	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kro"
	kroadapter "github.com/knodex/knodex/server/internal/kro/graph"
	kroparser "github.com/knodex/knodex/server/internal/kro/parser"
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
	factory           dynamicinformer.DynamicSharedInformerFactory
	cache             *RGDCache
	informer          cache.SharedIndexInformer
	stopCh            chan struct{}
	done              chan struct{} // signals informer goroutine has exited
	synced            atomic.Bool
	running           atomic.Bool
	logger            *slog.Logger
	onChangeCallbacks []ChangeCallback
	onUpdateCallbacks []RGDUpdateCallback

	// builder is the KRO graph builder for producing *graph.Graph from RGDs.
	// Nil when K8s cluster is unavailable (e.g., tests without a cluster).
	builder *krograph.Builder

	// graphCache stores the most recent *graph.Graph per RGD (keyed by "namespace/name").
	// Protected by graphMu. Nil entry means the builder failed for that RGD.
	graphCache map[string]*krograph.Graph
	graphMu    sync.RWMutex

	// packageFilter restricts catalog ingestion to RGDs matching these package names.
	// When nil/empty, all catalog-annotated RGDs are ingested (backward compatible).
	// Protected by packageFilterMu for safe concurrent access.
	packageFilter   map[string]bool
	packageFilterMu sync.RWMutex

	// stopOnce ensures the stop channel is only closed once
	// preventing panic from concurrent Stop() calls
	stopOnce sync.Once
}

// rgdGVR returns the GroupVersionResource for KRO ResourceGraphDefinitions.
// Delegates to kro.RGDGVR() for the canonical GVR definition.
var rgdGVR = kro.RGDGVR()

// fallbackParser is reused across extractFallbackMetadata calls when the
// KRO graph builder is unavailable. ResourceParser is stateless and safe for reuse.
var fallbackParser = kroparser.NewResourceParser()

// NewRGDWatcher creates a new RGD watcher using a shared dynamic client and informer factory.
// The dynamic client and factory should be created at the application level so that
// all watchers share the same rate limits (QPS=50/Burst=100) and informer cache.
//
// If restConfig is non-nil, a KRO graph.Builder is created to produce rich *graph.Graph
// objects on each watch event. When nil, the watcher falls back to the lightweight parser.
func NewRGDWatcher(dynamicClient dynamic.Interface, factory dynamicinformer.DynamicSharedInformerFactory, restConfig *rest.Config) *RGDWatcher {
	w := &RGDWatcher{
		dynamicClient: dynamicClient,
		factory:       factory,
		cache:         NewRGDCache(),
		graphCache:    make(map[string]*krograph.Graph),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		logger:        slog.Default().With("component", "rgd-watcher"),
	}

	// Create KRO graph builder if cluster config is available
	if restConfig != nil {
		httpClient, err := rest.HTTPClientFor(restConfig)
		if err != nil {
			w.logger.Warn("failed to create HTTP client for graph builder, falling back to lightweight parser", "error", err)
		} else {
			builder, err := krograph.NewBuilder(restConfig, httpClient)
			if err != nil {
				w.logger.Warn("failed to create KRO graph builder, falling back to lightweight parser", "error", err)
			} else {
				w.builder = builder
				w.logger.Info("KRO graph builder initialized")
			}
		}
	}

	return w
}

// NewRGDWatcherWithClient creates a watcher with an existing dynamic client (for testing).
// Creates its own factory from the client for backward compatibility.
func NewRGDWatcherWithClient(client dynamic.Interface) *RGDWatcher {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(client, 10*time.Minute)
	return NewRGDWatcher(client, factory, nil)
}

// NewRGDWatcherWithCache creates a watcher with an existing cache (for testing)
func NewRGDWatcherWithCache(cache *RGDCache) *RGDWatcher {
	w := &RGDWatcher{
		cache:      cache,
		graphCache: make(map[string]*krograph.Graph),
		stopCh:     make(chan struct{}),
		done:       make(chan struct{}),
		logger:     slog.Default().With("component", "rgd-watcher"),
	}
	// Mark as running and synced for tests
	w.running.Store(true)
	w.synced.Store(true)
	return w
}

// SetPackageFilter configures the package filter for catalog ingestion.
// Only RGDs with a knodex.io/package label matching one of the given names are ingested.
// An empty/nil slice disables filtering (all catalog-annotated RGDs are ingested).
// Safe to call concurrently; protected by packageFilterMu.
func (w *RGDWatcher) SetPackageFilter(packages []string) {
	w.packageFilterMu.Lock()
	defer w.packageFilterMu.Unlock()
	if len(packages) == 0 {
		w.packageFilter = nil
		return
	}
	w.packageFilter = make(map[string]bool, len(packages))
	for _, pkg := range packages {
		w.packageFilter[pkg] = true
	}
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

	// Use the shared informer factory (injected at construction).
	// Get informer for RGDs (all namespaces)
	w.informer = w.factory.ForResource(rgdGVR).Informer()

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

// graphCacheKey returns the cache key for a graph entry (namespace/name).
func graphCacheKey(namespace, name string) string {
	return namespace + "/" + name
}

// defaultRGDConfig is the configuration passed to the KRO builder.
// These limits are for runtime validation; we use generous defaults for visualization.
var defaultRGDConfig = krograph.RGDConfig{
	MaxCollectionSize:          1000,
	MaxCollectionDimensionSize: 10,
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

	// Build graph via KRO builder (if available) and cache it
	w.buildAndCacheGraph(u, rgd)

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
		key := graphCacheKey(u.GetNamespace(), u.GetName())
		w.graphMu.Lock()
		delete(w.graphCache, key)
		w.graphMu.Unlock()
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

		// Rebuild graph only on real changes (not re-sync events)
		w.buildAndCacheGraph(u, rgd)
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

	// Clean up graph cache entry
	key := graphCacheKey(u.GetNamespace(), u.GetName())
	w.graphMu.Lock()
	delete(w.graphCache, key)
	w.graphMu.Unlock()

	w.logger.Info("removed RGD from catalog",
		"name", u.GetName(),
		"namespace", u.GetNamespace())
	w.notifyChange()
	w.notifyUpdate(RGDActionDelete, u.GetName(), nil)
}

// shouldIncludeInCatalog checks if the RGD has the catalog annotation.
// Simplified visibility model:
//   - knodex.io/catalog: "true" is the GATEWAY to the catalog
//   - RGDs without this annotation are NOT part of the catalog system (invisible to everyone)
//   - catalog: true alone = visible to ALL authenticated users (public)
//   - catalog: true + project label = visible to project members only
//   - Inactive RGDs are included with an "Inactive" status so the UI can show them
//     with a badge and disabled deploy button, preserving instance visibility
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

	// Package filter: when active, require matching knodex.io/package label
	w.packageFilterMu.RLock()
	pkgFilter := w.packageFilter
	w.packageFilterMu.RUnlock()
	if len(pkgFilter) > 0 {
		pkg := strings.ToLower(strings.TrimSpace(parser.GetLabelOrDefault(u, kro.RGDPackageLabel, "")))
		if !pkgFilter[pkg] {
			w.logger.Debug("skipping RGD (package filter mismatch)",
				"name", u.GetName(),
				"namespace", u.GetNamespace(),
				"package", pkg)
			return false
		}
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

	// Extract API version and kind from spec.schema (RGD structure)
	// Using parser's type-safe field accessors
	apiVersion := parser.GetSpecFieldStringOrDefault(u, "", "schema", "apiVersion")
	kind := parser.GetSpecFieldStringOrDefault(u, "", "schema", "kind")

	// Extract declared plural name from spec.schema.crd.spec.names.plural (if present)
	pluralName := parser.GetSpecFieldStringOrDefault(u, "", "schema", "crd", "spec", "names", "plural")

	// Extract scope from spec.schema.crd.spec.names.scope (defaults to Namespaced)
	scope := parser.GetSpecFieldStringOrDefault(u, "Namespaced", "schema", "crd", "spec", "names", "scope")
	isClusterScoped := scope == "Cluster"

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

	// Extract lastIssuedRevision from KRO status (integer field)
	lastIssuedRevision := 0
	if lastRevRaw, err := parser.GetStatusField(u, "lastIssuedRevision"); err == nil {
		switch v := lastRevRaw.(type) {
		case int64:
			lastIssuedRevision = int(v)
		case float64:
			lastIssuedRevision = int(v)
		case int:
			lastIssuedRevision = v
		}
	}

	// Parse extends-kind annotation (comma-separated parent Kinds)
	var extendsKinds []string
	if extendsStr, ok := parser.GetAnnotation(u, kro.ExtendsKindAnnotation); ok && extendsStr != "" {
		for _, k := range strings.Split(extendsStr, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				extendsKinds = append(extendsKinds, k)
			}
		}
	}

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

	// Extract package label (empty = no package declared)
	packageName := strings.TrimSpace(strings.ToLower(parser.GetLabelOrDefault(u, kro.RGDPackageLabel, "")))

	// Extract display title from annotation, falling back to the K8s resource name
	title := parser.GetAnnotationOrDefault(u, kro.TitleAnnotation, parser.GetName(u))
	if title == "" {
		title = parser.GetName(u)
	}

	// Note: DependsOnKinds and SecretRefs are populated by buildAndCacheGraph()
	// after this method returns, using either the KRO builder or fallback parser.

	return &models.CatalogRGD{
		Name:                   parser.GetName(u),
		Title:                  title,
		Namespace:              parser.GetNamespace(u),
		Description:            parser.GetAnnotationOrDefault(u, kro.DescriptionAnnotation, ""),
		Tags:                   tags,
		Category:               parser.GetAnnotationOrDefault(u, kro.CategoryAnnotation, ""),
		Icon:                   parser.GetAnnotationOrDefault(u, kro.IconAnnotation, ""),
		DocsURL:                parser.GetAnnotationOrDefault(u, kro.DocsURLAnnotation, ""),
		Organization:           organization,
		Package:                packageName,
		Labels:                 labels,
		Annotations:            annotations,
		ExtendsKinds:           extendsKinds,
		InstanceCount:          0, // Will be populated by instance tracker
		APIVersion:             apiVersion,
		Kind:                   kind,
		PluralName:             pluralName,
		IsClusterScoped:        isClusterScoped,
		Status:                 status,
		LastIssuedRevision:     lastIssuedRevision,
		AllowedDeploymentModes: allowedModes,
		CreatedAt:              createdAt,
		UpdatedAt:              updatedAt,
		ResourceVersion:        parser.GetResourceVersion(u),
		RawSpec:                rawSpec,
	}
}

// buildAndCacheGraph converts an unstructured RGD to a typed v1alpha1.ResourceGraphDefinition,
// calls the KRO graph builder, and caches the result. If the builder is unavailable or fails,
// the watcher falls back to the lightweight parser for DependsOnKinds and SecretRefs.
//
// On success, DependsOnKinds and SecretRefs on the CatalogRGD are populated from the graph.
// On failure, they are populated via the fallback parser.
func (w *RGDWatcher) buildAndCacheGraph(u *unstructured.Unstructured, rgd *models.CatalogRGD) {
	key := graphCacheKey(u.GetNamespace(), u.GetName())

	if w.builder == nil {
		// No builder available — use fallback parser
		rgd.DependsOnKinds, rgd.ProducesKinds, rgd.SecretRefs = extractFallbackMetadata(rgd.RawSpec, rgd.Name)
		return
	}

	// Convert unstructured → typed v1alpha1.ResourceGraphDefinition
	var typedRGD krov1alpha1.ResourceGraphDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &typedRGD); err != nil {
		w.logger.Warn("failed to convert RGD to typed object, falling back to parser",
			"name", rgd.Name, "error", err)
		rgd.DependsOnKinds, rgd.ProducesKinds, rgd.SecretRefs = extractFallbackMetadata(rgd.RawSpec, rgd.Name)
		return
	}

	// Build graph via KRO builder
	g, err := w.builder.NewResourceGraphDefinition(&typedRGD, defaultRGDConfig)
	if err != nil {
		w.logger.Warn("KRO graph builder failed, falling back to parser",
			"name", rgd.Name, "error", err)
		// Store nil to indicate build failure (allows callers to check)
		w.graphMu.Lock()
		w.graphCache[key] = nil
		w.graphMu.Unlock()
		rgd.DependsOnKinds, rgd.ProducesKinds, rgd.SecretRefs = extractFallbackMetadata(rgd.RawSpec, rgd.Name)
		return
	}

	// Cache the graph
	w.graphMu.Lock()
	w.graphCache[key] = g
	w.graphMu.Unlock()

	// Extract DependsOnKinds, ProducesKinds, and SecretRefs from the graph
	rgd.DependsOnKinds, rgd.ProducesKinds, rgd.SecretRefs = extractGraphMetadata(g, rgd.RawSpec)

	w.logger.Debug("graph built and cached",
		"name", rgd.Name,
		"nodes", len(g.Nodes),
		"topoOrder", len(g.TopologicalOrder))
}

// GetGraph returns the cached *graph.Graph for the given RGD.
// Returns nil if the graph was not built (builder unavailable or build failed).
func (w *RGDWatcher) GetGraph(namespace, name string) *krograph.Graph {
	key := graphCacheKey(namespace, name)
	w.graphMu.RLock()
	defer w.graphMu.RUnlock()
	return w.graphCache[key]
}

// extractGraphMetadata extracts DependsOnKinds, ProducesKinds, and SecretRefs
// from a KRO *graph.Graph using the adapter package.
func extractGraphMetadata(g *krograph.Graph, rawSpec map[string]interface{}) ([]string, []models.GVKRef, []kroparser.SecretRef) {
	if g == nil {
		return nil, nil, nil
	}

	var dependsKinds []string
	seenDepends := make(map[string]bool)

	var producesKinds []models.GVKRef
	seenProduces := make(map[string]bool)

	for _, node := range g.Nodes {
		if node.Template == nil {
			continue
		}
		if node.Meta.Type == krograph.NodeTypeExternal || node.Meta.Type == krograph.NodeTypeExternalCollection {
			// External refs are consumed, not produced
			kind := node.Template.GetKind()
			if kind != "" && !seenDepends[kind] {
				seenDepends[kind] = true
				dependsKinds = append(dependsKinds, kind)
			}
		} else {
			// Regular, Collection, and other non-external nodes are produced resources
			kind := node.Template.GetKind()
			if kind == "" {
				continue
			}
			group, version := parseAPIVersion(node.Template.GetAPIVersion())
			gvkKey := group + "/" + version + "/" + kind
			if !seenProduces[gvkKey] {
				seenProduces[gvkKey] = true
				producesKinds = append(producesKinds, models.GVKRef{
					Group:   group,
					Version: version,
					Kind:    kind,
				})
			}
		}
	}

	secretRefs := kroadapter.ExtractSecretRefs(g, rawSpec)
	return dependsKinds, producesKinds, secretRefs
}

// extractFallbackMetadata parses the RGD spec to find DependsOnKinds, ProducesKinds,
// and SecretRefs. This is the fallback path used when the KRO graph builder is unavailable.
func extractFallbackMetadata(rawSpec map[string]interface{}, rgdName string) ([]string, []models.GVKRef, []kroparser.SecretRef) {
	if rawSpec == nil {
		return nil, nil, nil
	}

	parsedGraph, err := fallbackParser.ParseRGDResources(rgdName, "", rawSpec)
	if err != nil || parsedGraph == nil {
		return nil, nil, nil
	}

	// Extract unique externalRef Kinds (DependsOnKinds)
	extRefs := parsedGraph.GetExternalRefs()
	var dependsKinds []string
	if len(extRefs) > 0 {
		seen := make(map[string]bool)
		for _, ref := range extRefs {
			if ref.ExternalRef != nil && ref.ExternalRef.Kind != "" && !seen[ref.ExternalRef.Kind] {
				seen[ref.ExternalRef.Kind] = true
				dependsKinds = append(dependsKinds, ref.ExternalRef.Kind)
			}
		}
	}

	// Extract ProducesKinds from non-externalRef resources
	var producesKinds []models.GVKRef
	seenProduces := make(map[string]bool)
	for _, res := range parsedGraph.Resources {
		if res.ExternalRef != nil {
			continue // External refs are consumed, not produced
		}
		kind := res.Kind
		if kind == "" {
			continue
		}
		group, version := parseAPIVersion(res.APIVersion)
		gvkKey := group + "/" + version + "/" + kind
		if !seenProduces[gvkKey] {
			seenProduces[gvkKey] = true
			producesKinds = append(producesKinds, models.GVKRef{
				Group:   group,
				Version: version,
				Kind:    kind,
			})
		}
	}

	return dependsKinds, producesKinds, parsedGraph.SecretRefs
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
		// Rebuild graph cache to keep it consistent with the refreshed RGD.
		// Without this, the graph cache would become stale after a forced refresh,
		// causing DependsOnKinds and SecretRefs on the CatalogRGD to be empty.
		w.buildAndCacheGraph(u, rgd)
		w.cache.Set(rgd)
	} else {
		w.cache.Delete(namespace, name)
	}

	return nil
}
