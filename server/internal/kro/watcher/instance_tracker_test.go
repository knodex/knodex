package watcher

import (
	"testing"
	"time"

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

	// Simulate that there's an informer for rgd-a that will be removed
	fakeKey := "cluster/rgd-a@kro.run/v1alpha1/webapps"
	fakeStopCh := make(chan struct{})
	tracker.informersMu.Lock()
	tracker.informers[fakeKey] = fakeStopCh
	tracker.informerRGDs[fakeKey] = rgdRef{namespace: "", name: "rgd-a"}
	tracker.informersMu.Unlock()

	// Track delete notifications
	var deletedInstances []string
	tracker.SetOnUpdateCallback(func(action InstanceAction, namespace, kind, name string, instance *models.Instance) {
		if action == InstanceActionDelete {
			deletedInstances = append(deletedInstances, namespace+"/"+kind+"/"+name)
		}
	})

	// Simulate handleRGDChange where rgd-a is removed (currentKeys won't contain fakeKey)
	// We manually replicate the removal logic since handleRGDChange requires rgdWatcher
	tracker.informersMu.Lock()
	for key, stopCh := range tracker.informers {
		// Simulate: rgd-a is no longer in currentKeys
		close(stopCh)
		if ref, ok := tracker.informerRGDs[key]; ok {
			removed := cache.DeleteByRGD(ref.namespace, ref.name)
			for _, inst := range removed {
				tracker.notifyUpdate(InstanceActionDelete, inst.Namespace, inst.Kind, inst.Name, nil)
			}
			delete(tracker.informerRGDs, key)
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

func TestInstanceTracker_InformerRGDs_Tracked(t *testing.T) {
	tracker := NewInstanceTrackerWithCache(NewInstanceCache())

	// Verify informerRGDs is initialized
	if tracker.informerRGDs == nil {
		t.Fatal("expected informerRGDs to be initialized")
	}
	if len(tracker.informerRGDs) != 0 {
		t.Errorf("expected empty informerRGDs, got %d entries", len(tracker.informerRGDs))
	}
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
