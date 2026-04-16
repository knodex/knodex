// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package watcher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	faketesting "k8s.io/client-go/testing"

	"github.com/knodex/knodex/server/internal/models"
)

func TestInstanceTracker_CalculateHealth(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	tests := []struct {
		name       string
		conditions []models.InstanceCondition
		status     map[string]interface{}
		expected   models.InstanceHealth
	}{
		{
			name:       "no conditions, no status",
			conditions: nil,
			status:     nil,
			expected:   models.HealthUnknown,
		},
		{
			name:       "phase running",
			conditions: nil,
			status:     map[string]interface{}{"phase": "Running"},
			expected:   models.HealthHealthy,
		},
		{
			name:       "phase pending",
			conditions: nil,
			status:     map[string]interface{}{"phase": "Pending"},
			expected:   models.HealthProgressing,
		},
		{
			name:       "phase failed",
			conditions: nil,
			status:     map[string]interface{}{"phase": "Failed"},
			expected:   models.HealthUnhealthy,
		},
		{
			name: "ready condition true",
			conditions: []models.InstanceCondition{
				{Type: "Ready", Status: "True"},
			},
			status:   nil,
			expected: models.HealthHealthy,
		},
		{
			name: "ready condition false",
			conditions: []models.InstanceCondition{
				{Type: "Ready", Status: "False"},
			},
			status:   nil,
			expected: models.HealthUnhealthy,
		},
		{
			name: "ready condition false with progressing reason",
			conditions: []models.InstanceCondition{
				{Type: "Ready", Status: "False", Reason: "Progressing"},
			},
			status:   nil,
			expected: models.HealthProgressing,
		},
		{
			name: "ready condition unknown",
			conditions: []models.InstanceCondition{
				{Type: "Ready", Status: "Unknown"},
			},
			status:   nil,
			expected: models.HealthProgressing,
		},
		{
			name: "multiple conditions all true",
			conditions: []models.InstanceCondition{
				{Type: "Available", Status: "True"},
				{Type: "Progressing", Status: "True"},
			},
			status:   nil,
			expected: models.HealthHealthy,
		},
		{
			name: "some conditions false",
			conditions: []models.InstanceCondition{
				{Type: "Available", Status: "True"},
				{Type: "Progressing", Status: "False"},
			},
			status:   nil,
			expected: models.HealthDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := tracker.calculateHealth(tt.conditions, tt.status)
			if health != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, health)
			}
		})
	}
}

func TestInstanceTracker_ExtractConditions(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	status := map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":               "Ready",
				"status":             "True",
				"reason":             "AllReady",
				"message":            "All components are ready",
				"lastTransitionTime": "2025-01-15T10:00:00Z",
			},
			map[string]interface{}{
				"type":   "Available",
				"status": "True",
			},
		},
	}

	conditions := tracker.extractConditions(status)
	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}

	if conditions[0].Type != "Ready" {
		t.Errorf("expected first condition type 'Ready', got '%s'", conditions[0].Type)
	}
	if conditions[0].Status != "True" {
		t.Errorf("expected first condition status 'True', got '%s'", conditions[0].Status)
	}
	if conditions[0].Reason != "AllReady" {
		t.Errorf("expected first condition reason 'AllReady', got '%s'", conditions[0].Reason)
	}
}

func TestInstanceTracker_ExtractConditions_Empty(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	conditions := tracker.extractConditions(nil)
	if conditions != nil {
		t.Error("expected nil conditions for nil status")
	}

	conditions = tracker.extractConditions(map[string]interface{}{})
	if conditions != nil {
		t.Error("expected nil conditions for empty status")
	}
}

func TestInstanceTracker_ListInstances(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	cache.Set(&models.Instance{
		Name:         "inst1",
		Namespace:    "default",
		RGDName:      "rgd-a",
		RGDNamespace: "default",
		Health:       models.HealthHealthy,
	})
	cache.Set(&models.Instance{
		Name:         "inst2",
		Namespace:    "default",
		RGDName:      "rgd-b",
		RGDNamespace: "default",
		Health:       models.HealthUnhealthy,
	})

	result := tracker.ListInstances(models.InstanceListOptions{
		Page:     1,
		PageSize: 10,
	})

	if result.TotalCount != 2 {
		t.Errorf("expected 2 instances, got %d", result.TotalCount)
	}
}

func TestInstanceTracker_GetInstance(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	cache.Set(&models.Instance{
		Name:      "test-instance",
		Namespace: "default",
		Kind:      "WebApp",
		RGDName:   "test-rgd",
	})

	instance, ok := tracker.GetInstance("default", "WebApp", "test-instance")
	if !ok {
		t.Fatal("expected to find instance")
	}
	if instance.Name != "test-instance" {
		t.Errorf("expected name 'test-instance', got '%s'", instance.Name)
	}

	_, ok = tracker.GetInstance("default", "WebApp", "non-existent")
	if ok {
		t.Error("expected not to find non-existent instance")
	}
}

func TestInstanceTracker_GetInstancesByRGD(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "default", RGDName: "rgd-b", RGDNamespace: "default"})

	instances := tracker.GetInstancesByRGD("default", "rgd-a")
	if len(instances) != 2 {
		t.Errorf("expected 2 instances for rgd-a, got %d", len(instances))
	}
}

func TestInstanceTracker_CountInstancesByRGD(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	cache.Set(&models.Instance{Name: "inst1", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "default", RGDName: "rgd-a", RGDNamespace: "default"})

	count := tracker.CountInstancesByRGD("default", "rgd-a")
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestInstanceTracker_GVRFromRGD(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	tests := []struct {
		name        string
		rgd         *models.CatalogRGD
		expectedGrp string
		expectedVer string
		expectedRes string
	}{
		{
			name: "custom group",
			rgd: &models.CatalogRGD{
				APIVersion: "example.com/v1",
				Kind:       "WebApp",
			},
			expectedGrp: "example.com",
			expectedVer: "v1",
			expectedRes: "webapps",
		},
		{
			name: "core group",
			rgd: &models.CatalogRGD{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			expectedGrp: "",
			expectedVer: "v1",
			expectedRes: "configmaps",
		},
		{
			name: "with version",
			rgd: &models.CatalogRGD{
				APIVersion: "apps/v1beta1",
				Kind:       "Deployment",
			},
			expectedGrp: "apps",
			expectedVer: "v1beta1",
			expectedRes: "deployments",
		},
		{
			// STORY-294 AC #5: PluralName is deliberately ignored by gvrFromRGD.
			// Discovery-based resolution (ResolveGVR) is the correct path for irregular
			// plurals. gvrFromRGD uses naive kind+"s" as a fast-path fallback only.
			name: "PluralName field is intentionally ignored - naive pluralization still used",
			rgd: &models.CatalogRGD{
				APIVersion: "example.com/v1",
				Kind:       "Proxy",
				PluralName: "proxies", // set, but gvrFromRGD does not use it
			},
			expectedGrp: "example.com",
			expectedVer: "v1",
			expectedRes: "proxys", // naive kind+"s", NOT PluralName
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvr := tracker.gvrFromRGD(tt.rgd)
			if gvr.Group != tt.expectedGrp {
				t.Errorf("expected group '%s', got '%s'", tt.expectedGrp, gvr.Group)
			}
			if gvr.Version != tt.expectedVer {
				t.Errorf("expected version '%s', got '%s'", tt.expectedVer, gvr.Version)
			}
			if gvr.Resource != tt.expectedRes {
				t.Errorf("expected resource '%s', got '%s'", tt.expectedRes, gvr.Resource)
			}
		})
	}
}

// --- STORY-298: Dual-prefix label migration compatibility ---

func TestBelongsToRGD_LegacyPrefix(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	u := newUnstructuredWithLabels(map[string]string{
		"kro.run/resource-graph-definition-name": "my-rgd",
	})

	if !tracker.belongsToRGD(u, "my-rgd", "") {
		t.Error("expected belongsToRGD=true for legacy kro.run/ prefix")
	}
}

func TestBelongsToRGD_InternalPrefix(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	u := newUnstructuredWithLabels(map[string]string{
		"internal.kro.run/resource-graph-definition-name": "my-rgd",
	})

	if !tracker.belongsToRGD(u, "my-rgd", "") {
		t.Error("expected belongsToRGD=true for internal.kro.run/ prefix")
	}
}

func TestBelongsToRGD_BothPrefixes_NewWins(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	u := newUnstructuredWithLabels(map[string]string{
		"internal.kro.run/resource-graph-definition-name": "new-rgd",
		"kro.run/resource-graph-definition-name":          "old-rgd",
	})

	// New prefix takes priority — should match "new-rgd", not "old-rgd"
	if !tracker.belongsToRGD(u, "new-rgd", "") {
		t.Error("expected belongsToRGD=true: new prefix should take priority")
	}
	if tracker.belongsToRGD(u, "old-rgd", "") {
		t.Error("expected belongsToRGD=false for old-rgd when new prefix is present")
	}
}

func TestBelongsToRGD_NeitherPrefix(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	u := newUnstructuredWithLabels(map[string]string{
		"unrelated-label": "value",
	})

	if tracker.belongsToRGD(u, "my-rgd", "") {
		t.Error("expected belongsToRGD=false when no RGD label is present")
	}
}

func TestBelongsToRGD_MixedCluster(t *testing.T) {
	// Simulates a mixed-version cluster: some instances with old prefix, some with new
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	oldInstance := newUnstructuredWithLabels(map[string]string{
		"kro.run/resource-graph-definition-name": "shared-rgd",
	})
	newInstance := newUnstructuredWithLabels(map[string]string{
		"internal.kro.run/resource-graph-definition-name": "shared-rgd",
	})

	if !tracker.belongsToRGD(oldInstance, "shared-rgd", "") {
		t.Error("old-prefix instance should belong to shared-rgd")
	}
	if !tracker.belongsToRGD(newInstance, "shared-rgd", "") {
		t.Error("new-prefix instance should belong to shared-rgd")
	}
}

// newUnstructuredWithLabels creates an Unstructured object with the given labels.
func newUnstructuredWithLabels(labels map[string]string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "TestResource",
			"metadata": map[string]interface{}{
				"name":      "test-instance",
				"namespace": "default",
				"labels":    toInterfaceMap(labels),
			},
		},
	}
}

// toInterfaceMap converts map[string]string to map[string]interface{} for unstructured objects.
func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func TestInstanceTracker_IsSyncedAndRunning(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	if !tracker.IsSynced() {
		t.Error("expected synced to be true for test tracker")
	}

	if !tracker.IsRunning() {
		t.Error("expected running to be true for test tracker")
	}
}

func TestInstanceTracker_Cache(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	if tracker.Cache() != cache {
		t.Error("expected Cache() to return the same cache")
	}
}

func TestInstanceTracker_SetOnChangeCallback(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	callbackCalled := false
	tracker.SetOnChangeCallback(func() {
		callbackCalled = true
	})

	tracker.notifyChange()

	if !callbackCalled {
		t.Error("expected callback to be called")
	}
}

func TestInstanceTracker_SetOnUpdateCallback(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	var receivedAction InstanceAction
	var receivedNamespace, receivedKind, receivedName string
	var receivedInstance *models.Instance

	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		receivedAction = action
		receivedNamespace = namespace
		receivedKind = kind
		receivedName = name
		receivedInstance = instance
	})

	testInstance := &models.Instance{
		Name:      "test-instance",
		Namespace: "test-ns",
		Kind:      "TestKind",
		Health:    models.HealthHealthy,
	}

	tracker.notifyUpdate(InstanceActionAdd, "test-ns", "TestKind", "test-instance", testInstance)

	if receivedAction != InstanceActionAdd {
		t.Errorf("expected action %s, got %s", InstanceActionAdd, receivedAction)
	}
	if receivedNamespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got '%s'", receivedNamespace)
	}
	if receivedKind != "TestKind" {
		t.Errorf("expected kind 'TestKind', got '%s'", receivedKind)
	}
	if receivedName != "test-instance" {
		t.Errorf("expected name 'test-instance', got '%s'", receivedName)
	}
	if receivedInstance != testInstance {
		t.Error("expected to receive the same instance")
	}
}

func TestInstanceTracker_NotifyUpdate_CallsLegacyCallback(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	legacyCallbackCalled := false
	tracker.SetOnChangeCallback(func() {
		legacyCallbackCalled = true
	})

	updateCallbackCalled := false
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		updateCallbackCalled = true
	})

	tracker.notifyUpdate(InstanceActionUpdate, "ns", "TestKind", "name", nil)

	if !updateCallbackCalled {
		t.Error("expected update callback to be called")
	}
	if !legacyCallbackCalled {
		t.Error("expected legacy callback to be called by notifyUpdate")
	}
}

func TestInstanceTracker_MultipleUpdateCallbacks(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	callback1Called := false
	callback2Called := false

	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		callback1Called = true
	})
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		callback2Called = true
	})

	tracker.notifyUpdate(InstanceActionAdd, "ns", "TestKind", "name", nil)

	if !callback1Called {
		t.Error("expected first update callback to be called")
	}
	if !callback2Called {
		t.Error("expected second update callback to be called")
	}
}

func TestInstanceTracker_MultipleChangeCallbacks(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	callback1Called := false
	callback2Called := false

	tracker.SetOnChangeCallback(func() {
		callback1Called = true
	})
	tracker.SetOnChangeCallback(func() {
		callback2Called = true
	})

	tracker.notifyChange()

	if !callback1Called {
		t.Error("expected first change callback to be called")
	}
	if !callback2Called {
		t.Error("expected second change callback to be called")
	}
}

func TestInstanceTracker_CallbackPanicRecovery(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	callback2Called := false

	// First callback panics
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		panic("test panic")
	})
	// Second callback should still fire
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		callback2Called = true
	})

	// Should not panic
	tracker.notifyUpdate(InstanceActionAdd, "ns", "TestKind", "name", nil)

	if !callback2Called {
		t.Error("expected second callback to fire despite first callback panicking")
	}
}

func TestInstanceTracker_ChangeCallbackPanicRecovery(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	callback2Called := false

	// First callback panics
	tracker.SetOnChangeCallback(func() {
		panic("test panic")
	})
	// Second callback should still fire
	tracker.SetOnChangeCallback(func() {
		callback2Called = true
	})

	// Should not panic
	tracker.notifyChange()

	if !callback2Called {
		t.Error("expected second change callback to fire despite first callback panicking")
	}
}

func TestInstanceTracker_HandleRGDChange_PurgesCache(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	// Simulate instances from two RGDs
	cache.Set(&models.Instance{Name: "inst1", Namespace: "prod", Kind: "WebApp", RGDName: "rgd-a", RGDNamespace: ""})
	cache.Set(&models.Instance{Name: "inst2", Namespace: "prod", Kind: "WebApp", RGDName: "rgd-a", RGDNamespace: ""})
	cache.Set(&models.Instance{Name: "inst3", Namespace: "prod", Kind: "Database", RGDName: "rgd-b", RGDNamespace: ""})

	// Simulate that there's an informer handler registered for rgd-a that will be removed
	fakeKey := "cluster/rgd-a@kro.run/v1alpha1/webapps"
	tracker.informersMu.Lock()
	tracker.informers[fakeKey] = informerEntry{
		rgd: rgdRef{namespace: "", name: "rgd-a"},
	}
	tracker.informersMu.Unlock()

	// Track delete notifications
	var deletedInstances []string
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		if action == InstanceActionDelete {
			deletedInstances = append(deletedInstances, namespace+"/"+kind+"/"+name)
		}
	})

	// Simulate handleRGDChange where rgd-a is removed (currentKeys won't contain fakeKey).
	// We manually replicate the removal logic since handleRGDChange requires rgdWatcher and factory.
	tracker.informersMu.Lock()
	for key, entry := range tracker.informers {
		// Simulate: rgd-a is no longer in currentKeys
		removed := cache.DeleteByRGD(entry.rgd.namespace, entry.rgd.name)
		for _, inst := range removed {
			tracker.notifyUpdate(InstanceActionDelete, inst.Namespace, inst.Kind, inst.Name, nil)
		}
		delete(tracker.informers, key)
	}
	tracker.informersMu.Unlock()

	// Verify rgd-a instances were purged
	if cache.Count() != 1 {
		t.Errorf("expected 1 remaining instance (rgd-b), got %d", cache.Count())
	}

	// Verify rgd-b instance remains
	_, ok := cache.Get("prod", "Database", "inst3")
	if !ok {
		t.Error("expected rgd-b instance to remain")
	}

	// Verify delete notifications were fired for rgd-a instances
	if len(deletedInstances) != 2 {
		t.Errorf("expected 2 delete notifications, got %d", len(deletedInstances))
	}
}

func TestInstanceTracker_Informers_Tracked(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	// Verify informers map is initialized
	if tracker.informers == nil {
		t.Fatal("expected informers to be initialized")
	}
	if len(tracker.informers) != 0 {
		t.Errorf("expected empty informers, got %d entries", len(tracker.informers))
	}
}

func TestInstanceTracker_SharedFactory_DeduplicatesInformers(t *testing.T) {
	// Two RGDs that produce the same GVR should share one informer
	// but get separate event handler registrations.
	gvr := schema.GroupVersionResource{Group: "kro.run", Version: "v1alpha1", Resource: "webapps"}
	scheme := runtime.NewScheme()
	fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			gvr: "WebAppList",
		},
	)
	factory := dynamicinformer.NewDynamicSharedInformerFactory(fakeDynClient, 0)

	rgdCache := NewRGDCache()
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	tracker := NewInstanceTracker(fakeDynClient, nil, factory, rgdWatcher)
	t.Cleanup(tracker.Stop)

	// Two different RGDs that produce the same GVR (kro.run/v1alpha1/webapps)
	rgdA := &models.CatalogRGD{
		Name:       "rgd-a",
		Namespace:  "",
		APIVersion: "kro.run/v1alpha1",
		Kind:       "WebApp",
	}
	rgdB := &models.CatalogRGD{
		Name:       "rgd-b",
		Namespace:  "",
		APIVersion: "kro.run/v1alpha1",
		Kind:       "WebApp",
	}

	// Register both RGDs
	tracker.ensureInformerForRGD(rgdA)
	tracker.ensureInformerForRGD(rgdB)

	// Should have two separate handler registrations (one per RGD)
	tracker.informersMu.RLock()
	if len(tracker.informers) != 2 {
		t.Fatalf("expected 2 informer entries (one per RGD), got %d", len(tracker.informers))
	}

	// Both entries should reference the same GVR
	var gvrs []schema.GroupVersionResource
	for _, entry := range tracker.informers {
		gvrs = append(gvrs, entry.gvr)
	}
	tracker.informersMu.RUnlock()

	if gvrs[0] != gvrs[1] {
		t.Errorf("expected both entries to have same GVR, got %s and %s", gvrs[0], gvrs[1])
	}

	// The factory should return the same informer instance for the same GVR
	informerA := factory.ForResource(gvrs[0]).Informer()
	informerB := factory.ForResource(gvrs[1]).Informer()
	if informerA != informerB {
		t.Error("expected factory to return the same informer for the same GVR (deduplication)")
	}
}

func TestInstanceTracker_StopOnce_PreventsPanic(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	// running is already true from NewInstanceTrackerWithCache

	// Calling Stop() twice must not panic (double-close on stopCh)
	tracker.Stop()
	tracker.Stop() // second call should be a no-op
}

func TestInstanceAction_Values(t *testing.T) {
	if InstanceActionAdd != "add" {
		t.Errorf("expected InstanceActionAdd to be 'add', got '%s'", InstanceActionAdd)
	}
	if InstanceActionUpdate != "update" {
		t.Errorf("expected InstanceActionUpdate to be 'update', got '%s'", InstanceActionUpdate)
	}
	if InstanceActionDelete != "delete" {
		t.Errorf("expected InstanceActionDelete to be 'delete', got '%s'", InstanceActionDelete)
	}
}

func TestInstanceHealth_Values(t *testing.T) {
	// Verify health values are correct strings
	if models.HealthHealthy != "Healthy" {
		t.Errorf("expected HealthHealthy to be 'Healthy', got '%s'", models.HealthHealthy)
	}
	if models.HealthDegraded != "Degraded" {
		t.Errorf("expected HealthDegraded to be 'Degraded', got '%s'", models.HealthDegraded)
	}
	if models.HealthUnhealthy != "Unhealthy" {
		t.Errorf("expected HealthUnhealthy to be 'Unhealthy', got '%s'", models.HealthUnhealthy)
	}
	if models.HealthProgressing != "Progressing" {
		t.Errorf("expected HealthProgressing to be 'Progressing', got '%s'", models.HealthProgressing)
	}
	if models.HealthUnknown != "Unknown" {
		t.Errorf("expected HealthUnknown to be 'Unknown', got '%s'", models.HealthUnknown)
	}
}

// --- STORY-271: Instance lifecycle with inactive RGDs ---

func TestInstanceTracker_InactiveRGD_InstancesPreserved(t *testing.T) {
	// When an RGD goes inactive, instances should remain in cache
	// because inactive RGDs now stay in the catalog cache (STORY-271).
	// Key: the informer key for the inactive RGD must still appear in currentKeys
	// so handleRGDChange does NOT close the informer and purge instances.
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:       "my-rgd",
		Namespace:  "default",
		Status:     "Active",
		Kind:       "TestResource",
		APIVersion: "example.com/v1",
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	instanceCache := NewInstanceCache()
	instanceCache.Set(&models.Instance{
		Name:         "instance-1",
		Namespace:    "ns1",
		Kind:         "TestResource",
		RGDName:      "my-rgd",
		RGDNamespace: "default",
		RGDStatus:    "Active",
	})
	instanceCache.Set(&models.Instance{
		Name:         "instance-2",
		Namespace:    "ns2",
		Kind:         "TestResource",
		RGDName:      "my-rgd",
		RGDNamespace: "default",
		RGDStatus:    "Active",
	})

	tracker := NewInstanceTrackerWithCache(instanceCache)
	tracker.rgdWatcher = rgdWatcher

	// Pre-create an informer entry for the RGD (simulates a running informer)
	rgd := rgdCache.All()[0]
	gvr := tracker.gvrFromRGD(rgd)
	key := tracker.informerKey(rgd, gvr)
	tracker.informers[key] = informerEntry{
		gvr: gvr,
		rgd: rgdRef{namespace: "default", name: "my-rgd"},
	}

	// Simulate RGD going inactive — it stays in the catalog cache
	rgdCache.Set(&models.CatalogRGD{
		Name:       "my-rgd",
		Namespace:  "default",
		Status:     "Inactive",
		Kind:       "TestResource",
		APIVersion: "example.com/v1",
	})

	// Trigger handleRGDChange — instances should NOT be purged because
	// the inactive RGD remains in cache and its key is still in currentKeys
	tracker.handleRGDChange()

	// Verify instances are still in cache
	if _, found := instanceCache.Get("ns1", "TestResource", "instance-1"); !found {
		t.Error("expected instance-1 to remain in cache after RGD goes inactive")
	}
	if _, found := instanceCache.Get("ns2", "TestResource", "instance-2"); !found {
		t.Error("expected instance-2 to remain in cache after RGD goes inactive")
	}

	// Verify the informer is still running (not closed)
	tracker.informersMu.RLock()
	_, informerExists := tracker.informers[key]
	tracker.informersMu.RUnlock()
	if !informerExists {
		t.Error("expected informer to still be running for inactive RGD")
	}
}

func TestInstanceTracker_DeletedRGD_InstancesPurged(t *testing.T) {
	// When an RGD is truly deleted, instances should be purged
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:       "my-rgd",
		Namespace:  "default",
		Status:     "Active",
		Kind:       "TestResource",
		APIVersion: "example.com/v1",
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	instanceCache := NewInstanceCache()
	instanceCache.Set(&models.Instance{
		Name:         "instance-1",
		Namespace:    "ns1",
		Kind:         "TestResource",
		RGDName:      "my-rgd",
		RGDNamespace: "default",
	})

	tracker := NewInstanceTrackerWithCache(instanceCache)
	tracker.rgdWatcher = rgdWatcher

	// Build current informer keys for the existing RGD
	rgd := rgdCache.All()[0]
	gvr := tracker.gvrFromRGD(rgd)
	key := tracker.informerKey(rgd, gvr)
	tracker.informers[key] = informerEntry{
		gvr: gvr,
		rgd: rgdRef{namespace: "default", name: "my-rgd"},
	}

	// Now truly delete the RGD from cache
	rgdCache.Delete("default", "my-rgd")

	// Trigger handleRGDChange — instances SHOULD be purged
	tracker.handleRGDChange()

	// Verify instance was purged
	if _, found := instanceCache.Get("ns1", "TestResource", "instance-1"); found {
		t.Error("expected instance-1 to be purged after RGD is deleted")
	}
}

func TestInstanceTracker_RGDStatusChangePropagation(t *testing.T) {
	// When RGD status changes, instances should get updated RGDStatus
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:      "my-rgd",
		Namespace: "default",
		Status:    "Active",
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	instanceCache := NewInstanceCache()
	instanceCache.Set(&models.Instance{
		Name:         "instance-1",
		Namespace:    "ns1",
		Kind:         "TestResource",
		RGDName:      "my-rgd",
		RGDNamespace: "default",
		RGDStatus:    "Active",
	})

	tracker := NewInstanceTrackerWithCache(instanceCache)
	tracker.rgdWatcher = rgdWatcher

	// Track WebSocket notifications
	var notifications []string
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		if action == InstanceActionUpdate && instance != nil {
			notifications = append(notifications, namespace+"/"+kind+"/"+name+"="+instance.RGDStatus)
		}
	})

	// Simulate RGD status change to Inactive
	updatedRGD := &models.CatalogRGD{
		Name:      "my-rgd",
		Namespace: "default",
		Status:    "Inactive",
	}
	tracker.updateInstancesRGDStatus(updatedRGD)

	// Verify instance's RGDStatus was updated
	inst, found := instanceCache.Get("ns1", "TestResource", "instance-1")
	if !found {
		t.Fatal("expected instance to remain in cache")
	}
	if inst.RGDStatus != "Inactive" {
		t.Errorf("expected RGDStatus 'Inactive', got %q", inst.RGDStatus)
	}

	// Verify WebSocket notification was fired
	if len(notifications) != 1 {
		t.Errorf("expected 1 WebSocket notification, got %d", len(notifications))
	}
	if len(notifications) > 0 && notifications[0] != "ns1/TestResource/instance-1=Inactive" {
		t.Errorf("unexpected notification: %s", notifications[0])
	}
}

func TestInstanceCondition_Fields(t *testing.T) {
	condition := models.InstanceCondition{
		Type:               "Ready",
		Status:             "True",
		Reason:             "AllReady",
		Message:            "All components are ready",
		LastTransitionTime: time.Now(),
	}

	if condition.Type != "Ready" {
		t.Errorf("expected Type 'Ready', got '%s'", condition.Type)
	}
}

// --- STORY-291: DeleteInstance GVR resolution via discovery ---

func TestInstanceTracker_DeleteInstance_DiscoveryResolvesIrregularPlural(t *testing.T) {
	// Set up fake discovery with Proxy -> proxies mapping
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "proxies", Kind: "Proxy", Verbs: metav1.Verbs{"delete", "get", "list"}},
			},
		},
	}

	// Set up fake dynamic client that accepts any delete
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeDynamic.PrependReactor("delete", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	// Create tracker with discovery and dynamic clients
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.dynamicClient = fakeDynamic
	tracker.discoveryClient = fakeDiscovery

	// Call DeleteInstance with irregular-plural kind "Proxy"
	err := tracker.DeleteInstance(context.Background(), "default", "my-proxy", "example.com/v1", "Proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the dynamic client received "proxies" (not "proxys")
	actions := fakeDynamic.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	deleteAction, ok := actions[0].(faketesting.DeleteAction)
	if !ok {
		t.Fatalf("expected DeleteAction, got %T", actions[0])
	}

	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxies"}
	if deleteAction.GetResource() != expectedGVR {
		t.Errorf("expected GVR %v, got %v", expectedGVR, deleteAction.GetResource())
	}
}

func TestInstanceTracker_DeleteInstance_DiscoveryFails_FallsBackToNaive(t *testing.T) {
	// Set up fake discovery with NO resources — mapper will fail to find Proxy
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}
	// No Resources set — discovery resolution will fail

	// Set up fake dynamic client that accepts any delete
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeDynamic.PrependReactor("delete", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	// Create tracker with discovery (that will fail) and dynamic clients
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.dynamicClient = fakeDynamic
	tracker.discoveryClient = fakeDiscovery

	// Call DeleteInstance — discovery will fail, should fall back to naive "proxys"
	err := tracker.DeleteInstance(context.Background(), "default", "my-proxy", "example.com/v1", "Proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the dynamic client received naive "proxys" (fallback)
	actions := fakeDynamic.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	deleteAction, ok := actions[0].(faketesting.DeleteAction)
	if !ok {
		t.Fatalf("expected DeleteAction, got %T", actions[0])
	}

	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxys"}
	if deleteAction.GetResource() != expectedGVR {
		t.Errorf("expected fallback GVR %v, got %v", expectedGVR, deleteAction.GetResource())
	}
}

func TestInstanceTracker_DeleteInstance_StandardPluralPassesNamespaceAndName(t *testing.T) {
	// Set up fake discovery with App -> apps mapping (standard plural)
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "apps", Kind: "App", Verbs: metav1.Verbs{"delete", "get", "list"}},
			},
		},
	}

	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeDynamic.PrependReactor("delete", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.dynamicClient = fakeDynamic
	tracker.discoveryClient = fakeDiscovery

	// Pre-populate cache so we can verify cache cleanup
	cache.Set(&models.Instance{
		Name:      "my-app",
		Namespace: "production",
		Kind:      "App",
		RGDName:   "test-rgd",
	})

	// Track delete notifications
	var deletedKey string
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		if action == InstanceActionDelete {
			deletedKey = namespace + "/" + kind + "/" + name
		}
	})

	err := tracker.DeleteInstance(context.Background(), "production", "my-app", "example.com/v1", "App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := fakeDynamic.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	deleteAction, ok := actions[0].(faketesting.DeleteAction)
	if !ok {
		t.Fatalf("expected DeleteAction, got %T", actions[0])
	}

	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "apps"}
	if deleteAction.GetResource() != expectedGVR {
		t.Errorf("expected GVR %v, got %v", expectedGVR, deleteAction.GetResource())
	}
	if deleteAction.GetNamespace() != "production" {
		t.Errorf("expected namespace %q, got %q", "production", deleteAction.GetNamespace())
	}
	if deleteAction.GetName() != "my-app" {
		t.Errorf("expected name %q, got %q", "my-app", deleteAction.GetName())
	}

	// Verify cache cleanup (M1 fix: cache eviction was previously untested)
	if _, found := cache.Get("production", "App", "my-app"); found {
		t.Error("expected instance to be removed from cache after DeleteInstance")
	}

	// Verify delete notification was fired
	if deletedKey != "production/App/my-app" {
		t.Errorf("expected delete notification for 'production/App/my-app', got %q", deletedKey)
	}
}

func TestInstanceTracker_DeleteInstance_NilDiscoveryFallsBackToNaive(t *testing.T) {
	// Verify DeleteInstance is safe when discoveryClient is nil (e.g., NewInstanceTrackerForTest)
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeDynamic.PrependReactor("delete", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerForTest(cache, fakeDynamic)
	// discoveryClient is nil — must not panic

	err := tracker.DeleteInstance(context.Background(), "default", "my-proxy", "example.com/v1", "Proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := fakeDynamic.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	deleteAction, ok := actions[0].(faketesting.DeleteAction)
	if !ok {
		t.Fatalf("expected DeleteAction, got %T", actions[0])
	}

	// With nil discoveryClient, should fall back to naive pluralization
	expectedGVR := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxys"}
	if deleteAction.GetResource() != expectedGVR {
		t.Errorf("expected naive fallback GVR %v, got %v", expectedGVR, deleteAction.GetResource())
	}
}

// TestResolveGVR_IrregularPlural is a regression guard for STORY-294.
// Regression guard: naive kind+"s" returns wrong plurals for English-irregular kinds.
// This test ensures discovery is used, catching any reversion to naive pluralization.
func TestResolveGVR_IrregularPlural(t *testing.T) {
	tests := []struct {
		kind        string
		plural      string
		naivePlural string // what kind+"s" would incorrectly return
	}{
		{kind: "Proxy", plural: "proxies", naivePlural: "proxys"},
		{kind: "Policy", plural: "policies", naivePlural: "policys"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			fakeDiscovery := &fake.FakeDiscovery{
				Fake: &faketesting.Fake{},
			}
			fakeDiscovery.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "example.com/v1",
					APIResources: []metav1.APIResource{
						{Name: tt.plural, Kind: tt.kind, Verbs: metav1.Verbs{"get", "list", "create", "delete"}},
					},
				},
			}

			cache := NewInstanceCache()
			tracker := NewInstanceTrackerWithCache(cache)
			tracker.discoveryClient = fakeDiscovery

			gvr, err := tracker.ResolveGVR("example.com/v1", tt.kind)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: tt.plural}
			if gvr != expected {
				t.Errorf("ResolveGVR(%q) = %v, want %v -- naive kind+'s' would have returned %q",
					tt.kind, gvr, expected, tt.naivePlural)
			}
		})
	}
}

func TestInstanceTracker_ResolveGVR_DiscoverySuccess(t *testing.T) {
	// Set up fake discovery with Proxy -> proxies mapping
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "proxies", Kind: "Proxy", Verbs: metav1.Verbs{"get", "list", "create"}},
			},
		},
	}

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.discoveryClient = fakeDiscovery

	gvr, err := tracker.ResolveGVR("example.com/v1", "Proxy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxies"}
	if gvr != expected {
		t.Errorf("expected GVR %v, got %v", expected, gvr)
	}
}

func TestInstanceTracker_ResolveGVR_DiscoveryFails_FallsBackToNaive(t *testing.T) {
	// Set up fake discovery with NO resources — resolution will fail
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.discoveryClient = fakeDiscovery

	gvr, err := tracker.ResolveGVR("example.com/v1", "Proxy")
	if err != nil {
		t.Fatalf("unexpected error (should never return error): %v", err)
	}

	// Falls back to naive pluralization: "proxys" (wrong but best-effort)
	expected := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxys"}
	if gvr != expected {
		t.Errorf("expected naive fallback GVR %v, got %v", expected, gvr)
	}
}

func TestInstanceTracker_ResolveGVR_NilDiscovery_FallsBackToNaive(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	// discoveryClient is nil

	gvr, err := tracker.ResolveGVR("example.com/v1", "Proxy")
	if err != nil {
		t.Fatalf("unexpected error (should never return error): %v", err)
	}

	expected := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "proxys"}
	if gvr != expected {
		t.Errorf("expected naive fallback GVR %v, got %v", expected, gvr)
	}
}

func TestInstanceTracker_ResolveGVR_VersionOnlyAPIVersion(t *testing.T) {
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)

	gvr, err := tracker.ResolveGVR("v1alpha1", "MyResource")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Version-only alpha → uses kro.run group
	if gvr.Group != "kro.run" {
		t.Errorf("expected group 'kro.run', got %q", gvr.Group)
	}
	if gvr.Version != "v1alpha1" {
		t.Errorf("expected version 'v1alpha1', got %q", gvr.Version)
	}
}

// --- STORY-300: Cluster-scoped instance tracking ---

func TestInstanceTracker_DeleteInstance_ClusterScoped_NoNamespace(t *testing.T) {
	// Cluster-scoped delete should NOT call .Namespace() — verifies scope-aware logic
	fakeDiscovery := &fake.FakeDiscovery{
		Fake: &faketesting.Fake{},
	}
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "clusterconfigs", Kind: "ClusterConfig", Verbs: metav1.Verbs{"delete", "get", "list"}},
			},
		},
	}

	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeDynamic.PrependReactor("delete", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	cache := NewInstanceCache()
	cache.Set(&models.Instance{
		Name:      "my-config",
		Namespace: "",
		Kind:      "ClusterConfig",
		RGDName:   "cluster-config-rgd",
	})

	tracker := NewInstanceTrackerWithCache(cache)
	tracker.dynamicClient = fakeDynamic
	tracker.discoveryClient = fakeDiscovery

	var deletedKey string
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		if action == InstanceActionDelete {
			deletedKey = namespace + "/" + kind + "/" + name
		}
	})

	// Delete with empty namespace (cluster-scoped)
	err := tracker.DeleteInstance(context.Background(), "", "my-config", "example.com/v1", "ClusterConfig")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the dynamic client received cluster-scoped delete (empty namespace)
	actions := fakeDynamic.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	deleteAction, ok := actions[0].(faketesting.DeleteAction)
	if !ok {
		t.Fatalf("expected DeleteAction, got %T", actions[0])
	}

	// Cluster-scoped delete should have empty namespace
	if deleteAction.GetNamespace() != "" {
		t.Errorf("expected empty namespace for cluster-scoped delete, got %q", deleteAction.GetNamespace())
	}
	if deleteAction.GetName() != "my-config" {
		t.Errorf("expected name 'my-config', got %q", deleteAction.GetName())
	}

	// Verify cache cleanup
	if _, found := cache.Get("", "ClusterConfig", "my-config"); found {
		t.Error("expected instance to be removed from cache")
	}

	// Verify delete notification
	if deletedKey != "/ClusterConfig/my-config" {
		t.Errorf("expected delete notification for '/ClusterConfig/my-config', got %q", deletedKey)
	}
}

func TestInstanceTracker_DeleteInstance_NamespaceScoped_StillWorks(t *testing.T) {
	// L2: Regression guard — namespace-scoped delete must pass namespace, correct GVR, and name
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeDynamic.PrependReactor("delete", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		return true, nil, nil
	})

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerForTest(cache, fakeDynamic)

	err := tracker.DeleteInstance(context.Background(), "prod", "my-app", "example.com/v1", "App")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actions := fakeDynamic.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	deleteAction, ok := actions[0].(faketesting.DeleteAction)
	if !ok {
		t.Fatalf("expected DeleteAction, got %T", actions[0])
	}

	if deleteAction.GetNamespace() != "prod" {
		t.Errorf("expected namespace 'prod', got %q", deleteAction.GetNamespace())
	}
	if deleteAction.GetName() != "my-app" {
		t.Errorf("expected name 'my-app', got %q", deleteAction.GetName())
	}
	if deleteAction.GetResource().Group != "example.com" {
		t.Errorf("expected group 'example.com', got %q", deleteAction.GetResource().Group)
	}
}

func TestInstanceTracker_UnstructuredToInstance_ClusterScoped(t *testing.T) {
	// When parent RGD is cluster-scoped, instance should have IsClusterScoped=true and empty namespace
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-config-rgd",
		Namespace:       "",
		Status:          "Active",
		Kind:            "ClusterConfig",
		APIVersion:      "example.com/v1",
		IsClusterScoped: true,
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	tracker.rgdWatcher = rgdWatcher

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "ClusterConfig",
			"metadata": map[string]interface{}{
				"name":              "my-cluster-config",
				"resourceVersion":   "12345",
				"uid":               "test-uid",
				"creationTimestamp": "2026-03-24T10:00:00Z",
			},
			"spec":   map[string]interface{}{"key": "value"},
			"status": map[string]interface{}{},
		},
	}

	instance := tracker.unstructuredToInstance(u, "cluster-config-rgd", "")

	if !instance.IsClusterScoped {
		t.Error("expected IsClusterScoped=true for cluster-scoped RGD instance")
	}
	if instance.Namespace != "" {
		t.Errorf("expected empty namespace for cluster-scoped instance, got %q", instance.Namespace)
	}
	if instance.Name != "my-cluster-config" {
		t.Errorf("expected name 'my-cluster-config', got %q", instance.Name)
	}
}

func TestInstanceTracker_UnstructuredToInstance_NamespaceScoped(t *testing.T) {
	// When parent RGD is namespace-scoped, instance should have IsClusterScoped=false
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "app-rgd",
		Namespace:       "",
		Status:          "Active",
		Kind:            "App",
		APIVersion:      "example.com/v1",
		IsClusterScoped: false,
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	tracker.rgdWatcher = rgdWatcher

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "App",
			"metadata": map[string]interface{}{
				"name":              "my-app",
				"namespace":         "prod",
				"resourceVersion":   "67890",
				"uid":               "test-uid-2",
				"creationTimestamp": "2026-03-24T10:00:00Z",
			},
			"spec":   map[string]interface{}{"replicas": float64(3)},
			"status": map[string]interface{}{},
		},
	}

	instance := tracker.unstructuredToInstance(u, "app-rgd", "")

	if instance.IsClusterScoped {
		t.Error("expected IsClusterScoped=false for namespace-scoped RGD instance")
	}
	if instance.Namespace != "prod" {
		t.Errorf("expected namespace 'prod', got %q", instance.Namespace)
	}
}

func TestInstanceTracker_UnstructuredToInstance_NilRGDWatcher(t *testing.T) {
	// M2: When rgdWatcher is nil, IsClusterScoped must default to false (not incoherent empty-ns + false).
	// This confirms the safe default — a cluster-scoped instance with nil watcher is stored as
	// {Namespace: "", IsClusterScoped: false} which is incorrect but stable; the watcher being nil
	// is a transient startup condition and is documented here as a known edge case.
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	// rgdWatcher is nil by default in NewInstanceTrackerWithCache

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "ClusterConfig",
			"metadata": map[string]interface{}{
				"name":              "orphan-config",
				"resourceVersion":   "1",
				"uid":               "uid-orphan",
				"creationTimestamp": "2026-03-24T10:00:00Z",
			},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}

	instance := tracker.unstructuredToInstance(u, "missing-rgd", "")

	// With nil rgdWatcher, IsClusterScoped defaults to false (safe default — no panic)
	if instance == nil {
		t.Fatal("expected non-nil instance even with nil rgdWatcher")
	}
	if instance.IsClusterScoped {
		t.Error("expected IsClusterScoped=false when rgdWatcher is nil (safe default)")
	}
	if instance.Name != "orphan-config" {
		t.Errorf("expected name 'orphan-config', got %q", instance.Name)
	}
}

func TestInstanceTracker_UnstructuredToInstance_TargetCluster(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	t.Run("annotation present", func(t *testing.T) {
		u := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "App",
				"metadata": map[string]interface{}{
					"name":              "my-app",
					"namespace":         "team-alpha",
					"resourceVersion":   "100",
					"uid":               "uid-target",
					"creationTimestamp": "2026-03-31T10:00:00Z",
					"annotations": map[string]interface{}{
						"knodex.io/target-cluster": "prod-eu-west",
					},
				},
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
		}
		instance := tracker.unstructuredToInstance(u, "test-rgd", "default")
		if instance.TargetCluster != "prod-eu-west" {
			t.Errorf("TargetCluster = %q, want %q", instance.TargetCluster, "prod-eu-west")
		}
	})

	t.Run("annotation absent", func(t *testing.T) {
		u := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "example.com/v1",
				"kind":       "App",
				"metadata": map[string]interface{}{
					"name":              "my-app",
					"namespace":         "default",
					"resourceVersion":   "101",
					"uid":               "uid-no-target",
					"creationTimestamp": "2026-03-31T10:00:00Z",
				},
				"spec":   map[string]interface{}{},
				"status": map[string]interface{}{},
			},
		}
		instance := tracker.unstructuredToInstance(u, "test-rgd", "default")
		if instance.TargetCluster != "" {
			t.Errorf("TargetCluster = %q, want empty string", instance.TargetCluster)
		}
	})
}

func TestInstanceTracker_HandleInstanceAdd_ClusterScoped(t *testing.T) {
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-rgd",
		Namespace:       "",
		Status:          "Active",
		Kind:            "ClusterConfig",
		APIVersion:      "example.com/v1",
		IsClusterScoped: true,
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.rgdWatcher = rgdWatcher

	var notifiedNamespace string
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		if action == InstanceActionAdd {
			notifiedNamespace = namespace
		}
	})

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "ClusterConfig",
			"metadata": map[string]interface{}{
				"name":              "my-config",
				"resourceVersion":   "1",
				"uid":               "uid-1",
				"creationTimestamp": "2026-03-24T10:00:00Z",
				"labels": map[string]interface{}{
					"kro.run/resource-graph-definition-name": "cluster-rgd",
				},
			},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		},
	}

	tracker.handleInstanceAdd(u, "cluster-rgd", "")

	// Verify instance in cache with empty namespace
	inst, ok := cache.Get("", "ClusterConfig", "my-config")
	if !ok {
		t.Fatal("expected cluster-scoped instance in cache")
	}
	if inst.Namespace != "" {
		t.Errorf("expected empty namespace, got %q", inst.Namespace)
	}
	if !inst.IsClusterScoped {
		t.Error("expected IsClusterScoped=true")
	}

	// Verify notification had empty namespace
	if notifiedNamespace != "" {
		t.Errorf("expected empty namespace in notification, got %q", notifiedNamespace)
	}
}

func TestInstanceTracker_MixedScope_Coexistence(t *testing.T) {
	// AC#2: Both namespace-scoped and cluster-scoped instances coexist
	rgdCache := NewRGDCache()
	rgdCache.Set(&models.CatalogRGD{
		Name:            "cluster-rgd",
		Namespace:       "",
		IsClusterScoped: true,
		Kind:            "ClusterConfig",
		APIVersion:      "example.com/v1",
	})
	rgdCache.Set(&models.CatalogRGD{
		Name:            "ns-rgd",
		Namespace:       "",
		IsClusterScoped: false,
		Kind:            "App",
		APIVersion:      "example.com/v1",
	})
	rgdWatcher := NewRGDWatcherWithCache(rgdCache)

	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	tracker.rgdWatcher = rgdWatcher

	// Add cluster-scoped instance
	cache.Set(&models.Instance{
		Name:            "global-config",
		Namespace:       "",
		Kind:            "ClusterConfig",
		RGDName:         "cluster-rgd",
		RGDNamespace:    "",
		IsClusterScoped: true,
	})

	// Add namespace-scoped instance
	cache.Set(&models.Instance{
		Name:            "my-app",
		Namespace:       "prod",
		Kind:            "App",
		RGDName:         "ns-rgd",
		RGDNamespace:    "",
		IsClusterScoped: false,
	})

	// Both should be retrievable
	if cache.Count() != 2 {
		t.Errorf("expected 2 instances, got %d", cache.Count())
	}

	clusterInst, ok := tracker.GetInstance("", "ClusterConfig", "global-config")
	if !ok {
		t.Fatal("expected to find cluster-scoped instance")
	}
	if !clusterInst.IsClusterScoped {
		t.Error("expected cluster-scoped instance to have IsClusterScoped=true")
	}

	nsInst, ok := tracker.GetInstance("prod", "App", "my-app")
	if !ok {
		t.Fatal("expected to find namespace-scoped instance")
	}
	if nsInst.IsClusterScoped {
		t.Error("expected namespace-scoped instance to have IsClusterScoped=false")
	}
}

func TestInstanceTracker_ResolveGVR_StableVersionOnly_EmptyGroup(t *testing.T) {
	// Regression guard: "v1" (no group) should produce group="" not "kro.run".
	// Core K8s resources (Pod, Service, ConfigMap) use apiVersion "v1" with no group.
	cache := NewInstanceCache()
	tracker := NewInstanceTrackerWithCache(cache)
	// discoveryClient is nil → falls back to naive pluralization

	gvr, err := tracker.ResolveGVR("v1", "Pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gvr.Group != "" {
		t.Errorf("expected empty group for stable apiVersion 'v1', got %q", gvr.Group)
	}
	if gvr.Version != "v1" {
		t.Errorf("expected version 'v1', got %q", gvr.Version)
	}
	if gvr.Resource != "pods" {
		t.Errorf("expected naive resource 'pods', got %q", gvr.Resource)
	}
}

func TestInstanceTracker_SyncDetection_NotSyncedBeforeStart(t *testing.T) {
	// A tracker created with NewInstanceTracker (not the test constructor)
	// should not be synced before Start() is called.
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "WidgetList"},
		&unstructured.UnstructuredList{},
	)
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: "WidgetList"})
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 0)

	tracker := NewInstanceTracker(dynClient, nil, factory, nil)
	t.Cleanup(tracker.Stop)

	require.False(t, tracker.IsSynced(), "expected IsSynced() == false before Start()")
}

func TestInstanceTracker_SyncDetection_SyncedAfterStart(t *testing.T) {
	// After Start() with a fake client (instant sync), IsSynced should become true.
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "WidgetList"},
		&unstructured.UnstructuredList{},
	)
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: "WidgetList"})
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 0)

	rgdWatcher := NewRGDWatcher(dynClient, factory, nil)
	rgdWatcher.cache.Set(&models.CatalogRGD{
		Name:       "test-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1",
		Kind:       "Widget",
	})

	tracker := NewInstanceTracker(dynClient, nil, factory, rgdWatcher)
	t.Cleanup(tracker.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, tracker.Start(ctx))
	require.Eventually(t, tracker.IsSynced, 5*time.Second, 10*time.Millisecond,
		"IsSynced() did not become true after Start()")
}

func TestInstanceTracker_SyncDetection_TestConstructorPresetsSynced(t *testing.T) {
	// NewInstanceTrackerWithCache (test constructor) pre-sets synced=true for test convenience
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())
	require.True(t, tracker.IsSynced(), "expected NewInstanceTrackerWithCache to pre-set synced=true")
}

func TestInstanceTracker_SyncDetection_NilFactory(t *testing.T) {
	// When factory is nil (test path), Start() should still mark synced=true
	tracker := NewInstanceTracker(nil, nil, nil, nil)
	t.Cleanup(tracker.Stop)

	require.NoError(t, tracker.Start(context.Background()))
	require.Eventually(t, tracker.IsSynced, 2*time.Second, 10*time.Millisecond,
		"IsSynced() did not become true with nil factory")
}

func TestInstanceTracker_SyncDetection_TimeoutKeepsSyncedFalse(t *testing.T) {
	// When WaitForCacheSync times out (context expires), synced must stay false.
	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: gvr.Group, Version: gvr.Version, Kind: "WidgetList"},
		&unstructured.UnstructuredList{},
	)
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvr: "WidgetList"})

	// Block all List calls so informers can never sync
	dynClient.PrependReactor("list", "*", func(action faketesting.Action) (bool, runtime.Object, error) {
		<-time.After(10 * time.Minute) // effectively block forever
		return false, nil, nil
	})

	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynClient, 0)

	rgdWatcher := NewRGDWatcher(dynClient, factory, nil)
	rgdWatcher.cache.Set(&models.CatalogRGD{
		Name:       "test-rgd",
		Namespace:  "default",
		APIVersion: "example.com/v1",
		Kind:       "Widget",
	})

	tracker := NewInstanceTracker(dynClient, nil, factory, rgdWatcher)
	t.Cleanup(tracker.Stop)

	// Use a very short context so the 30s internal timeout is capped to 200ms
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	require.NoError(t, tracker.Start(ctx))

	// Wait for the context to expire plus some margin
	time.Sleep(500 * time.Millisecond)
	require.False(t, tracker.IsSynced(), "expected IsSynced() == false after sync timeout")
}
