// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

// projectListGVR maps the Project CRD GVR to its list kind so that
// fake dynamic clients can handle LIST operations without panicking.
var projectListGVR = map[schema.GroupVersionResource]string{
	{Group: ProjectGroup, Version: ProjectVersion, Resource: ProjectResource}: "ProjectList",
}

// mockPolicyHandler implements ProjectPolicyHandler for testing
type mockPolicyHandler struct {
	mu               sync.Mutex
	loadedProjects   []string
	removedProjects  []string
	cacheInvalidated int
	watcherRestarts  int
	loadError        error
	removeError      error
}

func (m *mockPolicyHandler) LoadProjectPolicies(ctx context.Context, projectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadError != nil {
		return m.loadError
	}
	m.loadedProjects = append(m.loadedProjects, projectName)
	return nil
}

func (m *mockPolicyHandler) RemoveProjectPolicies(ctx context.Context, projectName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.removeError != nil {
		return m.removeError
	}
	m.removedProjects = append(m.removedProjects, projectName)
	return nil
}

func (m *mockPolicyHandler) InvalidateCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheInvalidated++
}

func (m *mockPolicyHandler) IncrementWatcherRestarts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watcherRestarts++
}

func (m *mockPolicyHandler) getLoadedProjects() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.loadedProjects))
	copy(result, m.loadedProjects)
	return result
}

func (m *mockPolicyHandler) getRemovedProjects() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.removedProjects))
	copy(result, m.removedProjects)
	return result
}

func (m *mockPolicyHandler) getCacheInvalidatedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cacheInvalidated
}

func (m *mockPolicyHandler) getWatcherRestartsCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watcherRestarts
}

func TestNewProjectWatcher(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	watcher := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	if watcher == nil {
		t.Fatal("expected watcher to be created")
	}

	if watcher.IsRunning() {
		t.Error("expected watcher to not be running initially")
	}
}

func TestProjectWatcher_DefaultConfig(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	config := ProjectWatcherConfig{
		ResyncPeriod: 0, // Should default to 30 minutes
		Logger:       nil,
	}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", config)
	pw := w.(*projectWatcher)

	if pw.config.ResyncPeriod != 30*time.Minute {
		t.Errorf("expected default resync period 30m, got %v", pw.config.ResyncPeriod)
	}

	if pw.logger == nil {
		t.Error("expected default logger to be set")
	}
}

func TestProjectWatcher_CustomConfig(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}
	logger := slog.Default()

	config := ProjectWatcherConfig{
		ResyncPeriod: 5 * time.Minute,
		Logger:       logger,
	}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", config)
	pw := w.(*projectWatcher)

	if pw.config.ResyncPeriod != 5*time.Minute {
		t.Errorf("expected resync period 5m, got %v", pw.config.ResyncPeriod)
	}
}

func TestProjectWatcher_IsRunning(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	// Initially not running
	if w.IsRunning() {
		t.Error("expected watcher to not be running initially")
	}
}

func TestProjectWatcher_StopWhenNotRunning(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	// Should not panic when stopping a non-running watcher
	w.Stop()

	if w.IsRunning() {
		t.Error("expected watcher to remain stopped")
	}
}

func TestProjectWatcher_LastSyncTime(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	// Initially zero
	if !w.LastSyncTime().IsZero() {
		t.Error("expected last sync time to be zero initially")
	}
}

func TestProjectWatcher_ExtractProject(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Test with unstructured object
	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": "test-project",
			},
		},
	}

	extracted, err := pw.extractProject(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if extracted.GetName() != "test-project" {
		t.Errorf("expected project name 'test-project', got %s", extracted.GetName())
	}
}

func TestProjectWatcher_ExtractProjectInvalidType(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Test with invalid type
	_, err := pw.extractProject("invalid")
	if err == nil {
		t.Error("expected error for invalid type")
	}

	watcherErr, ok := err.(*ProjectWatcherError)
	if !ok {
		t.Errorf("expected ProjectWatcherError, got %T", err)
	}

	if watcherErr.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestProjectWatcherError(t *testing.T) {
	t.Parallel()

	err := &ProjectWatcherError{Message: "test error"}

	if err.Error() != "test error" {
		t.Errorf("expected error message 'test error', got %s", err.Error())
	}
}

// Test that watcher correctly identifies Project GVR
func TestProjectWatcher_GVR(t *testing.T) {
	t.Parallel()

	// Verify Project CRD constants
	expectedGVR := schema.GroupVersionResource{
		Group:    "knodex.io",
		Version:  "v1alpha1",
		Resource: "projects",
	}

	if ProjectGroup != expectedGVR.Group {
		t.Errorf("expected group %s, got %s", expectedGVR.Group, ProjectGroup)
	}

	if ProjectVersion != expectedGVR.Version {
		t.Errorf("expected version %s, got %s", expectedGVR.Version, ProjectVersion)
	}

	if ProjectResource != expectedGVR.Resource {
		t.Errorf("expected resource %s, got %s", expectedGVR.Resource, ProjectResource)
	}
}

// Test concurrent access to watcher state
func TestProjectWatcher_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent reads of state
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = w.IsRunning()
				_ = w.LastSyncTime()
			}
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

// Benchmark tests
func BenchmarkProjectWatcher_IsRunning(b *testing.B) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.IsRunning()
	}
}

func BenchmarkProjectWatcher_LastSyncTime(b *testing.B) {
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.LastSyncTime()
	}
}

// TestProjectWatcher_OnAdd tests the onAdd event handler
func TestProjectWatcher_OnAdd(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Create a valid project object
	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": "test-project-add",
			},
		},
	}

	// Call onAdd
	pw.onAdd(project)

	// Verify handler was called
	loaded := handler.getLoadedProjects()
	if len(loaded) != 1 {
		t.Errorf("expected 1 loaded project, got %d", len(loaded))
	}
	if len(loaded) > 0 && loaded[0] != "test-project-add" {
		t.Errorf("expected loaded project 'test-project-add', got %s", loaded[0])
	}

	// Verify sync time was updated
	if pw.LastSyncTime().IsZero() {
		t.Error("expected sync time to be updated")
	}
}

// TestProjectWatcher_OnAddWithError tests onAdd when handler returns error
func TestProjectWatcher_OnAddWithError(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{
		loadError: fmt.Errorf("failed to load policies"),
	}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": "test-project-error",
			},
		},
	}

	// Should not panic even with error
	pw.onAdd(project)

	// Sync time should NOT be updated on error
	if !pw.LastSyncTime().IsZero() {
		t.Error("expected sync time to not be updated on error")
	}
}

// TestProjectWatcher_OnAddInvalidType tests onAdd with invalid object type
func TestProjectWatcher_OnAddInvalidType(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Should not panic with invalid type
	pw.onAdd("invalid")

	// Handler should not be called
	loaded := handler.getLoadedProjects()
	if len(loaded) != 0 {
		t.Errorf("expected 0 loaded projects, got %d", len(loaded))
	}
}

// TestProjectWatcher_OnUpdate tests the onUpdate event handler
func TestProjectWatcher_OnUpdate(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	oldProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project-update",
				"resourceVersion": "1",
			},
		},
	}

	newProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project-update",
				"resourceVersion": "2", // Changed version
			},
		},
	}

	// Call onUpdate
	pw.onUpdate(oldProject, newProject)

	// Verify handler was called
	loaded := handler.getLoadedProjects()
	if len(loaded) != 1 {
		t.Errorf("expected 1 loaded project, got %d", len(loaded))
	}
	if len(loaded) > 0 && loaded[0] != "test-project-update" {
		t.Errorf("expected loaded project 'test-project-update', got %s", loaded[0])
	}
}

// TestProjectWatcher_OnUpdateSameVersion tests onUpdate with same resource version (should skip)
func TestProjectWatcher_OnUpdateSameVersion(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	oldProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project-same",
				"resourceVersion": "1",
			},
		},
	}

	newProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project-same",
				"resourceVersion": "1", // Same version - should skip
			},
		},
	}

	// Call onUpdate
	pw.onUpdate(oldProject, newProject)

	// Handler should NOT be called since version is same
	loaded := handler.getLoadedProjects()
	if len(loaded) != 0 {
		t.Errorf("expected 0 loaded projects (same version), got %d", len(loaded))
	}
}

// TestProjectWatcher_OnUpdateWithError tests onUpdate when handler returns error
func TestProjectWatcher_OnUpdateWithError(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{
		loadError: fmt.Errorf("failed to load policies"),
	}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	oldProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project-err",
				"resourceVersion": "1",
			},
		},
	}

	newProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project-err",
				"resourceVersion": "2",
			},
		},
	}

	// Should not panic
	pw.onUpdate(oldProject, newProject)
}

// TestProjectWatcher_OnUpdateInvalidOldType tests onUpdate with invalid old object type
func TestProjectWatcher_OnUpdateInvalidOldType(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	newProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project",
				"resourceVersion": "2",
			},
		},
	}

	// Should not panic with invalid old type
	pw.onUpdate("invalid", newProject)

	// Handler should not be called
	loaded := handler.getLoadedProjects()
	if len(loaded) != 0 {
		t.Errorf("expected 0 loaded projects, got %d", len(loaded))
	}
}

// TestProjectWatcher_OnUpdateInvalidNewType tests onUpdate with invalid new object type
func TestProjectWatcher_OnUpdateInvalidNewType(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	oldProject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name":            "test-project",
				"resourceVersion": "1",
			},
		},
	}

	// Should not panic with invalid new type
	pw.onUpdate(oldProject, "invalid")

	// Handler should not be called
	loaded := handler.getLoadedProjects()
	if len(loaded) != 0 {
		t.Errorf("expected 0 loaded projects, got %d", len(loaded))
	}
}

// TestProjectWatcher_OnDelete tests the onDelete event handler
func TestProjectWatcher_OnDelete(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": "test-project-delete",
			},
		},
	}

	// Call onDelete
	pw.onDelete(project)

	// Verify handler was called
	removed := handler.getRemovedProjects()
	if len(removed) != 1 {
		t.Errorf("expected 1 removed project, got %d", len(removed))
	}
	if len(removed) > 0 && removed[0] != "test-project-delete" {
		t.Errorf("expected removed project 'test-project-delete', got %s", removed[0])
	}
}

// TestProjectWatcher_OnDeleteTombstone tests onDelete with DeletedFinalStateUnknown tombstone
func TestProjectWatcher_OnDeleteTombstone(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": "test-project-tombstone",
			},
		},
	}

	// Create tombstone object
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "test-project-tombstone",
		Obj: project,
	}

	// Call onDelete with tombstone
	pw.onDelete(tombstone)

	// Verify handler was called
	removed := handler.getRemovedProjects()
	if len(removed) != 1 {
		t.Errorf("expected 1 removed project, got %d", len(removed))
	}
	if len(removed) > 0 && removed[0] != "test-project-tombstone" {
		t.Errorf("expected removed project 'test-project-tombstone', got %s", removed[0])
	}
}

// TestProjectWatcher_OnDeleteTombstoneInvalidInner tests onDelete with tombstone containing invalid object
func TestProjectWatcher_OnDeleteTombstoneInvalidInner(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Create tombstone with invalid inner object
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "test-project-invalid",
		Obj: "invalid",
	}

	// Should not panic
	pw.onDelete(tombstone)

	// Handler should not be called
	removed := handler.getRemovedProjects()
	if len(removed) != 0 {
		t.Errorf("expected 0 removed projects, got %d", len(removed))
	}
}

// TestProjectWatcher_OnDeleteInvalidType tests onDelete with invalid object type
func TestProjectWatcher_OnDeleteInvalidType(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Should not panic with invalid type
	pw.onDelete(12345)

	// Handler should not be called
	removed := handler.getRemovedProjects()
	if len(removed) != 0 {
		t.Errorf("expected 0 removed projects, got %d", len(removed))
	}
}

// TestProjectWatcher_OnDeleteWithError tests onDelete when handler returns error
func TestProjectWatcher_OnDeleteWithError(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{
		removeError: fmt.Errorf("failed to remove policies"),
	}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	project := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "knodex.io/v1alpha1",
			"kind":       "Project",
			"metadata": map[string]interface{}{
				"name": "test-project-error",
			},
		},
	}

	// Should not panic
	pw.onDelete(project)

	// Sync time should NOT be updated on error
	if !pw.LastSyncTime().IsZero() {
		t.Error("expected sync time to not be updated on error")
	}
}

// TestProjectWatcher_SetNotRunning tests the setNotRunning method
func TestProjectWatcher_SetNotRunning(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Manually set running to true
	pw.mu.Lock()
	pw.running = true
	pw.mu.Unlock()

	if !pw.IsRunning() {
		t.Error("expected watcher to be running")
	}

	// Call setNotRunning
	pw.setNotRunning()

	if pw.IsRunning() {
		t.Error("expected watcher to not be running after setNotRunning")
	}
}

// TestProjectWatcher_UpdateLastSyncTime tests the updateLastSyncTime method
func TestProjectWatcher_UpdateLastSyncTime(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	handler := &mockPolicyHandler{}

	w := NewProjectWatcher(dynamicClient, handler, "knodex-system", ProjectWatcherConfig{})
	pw := w.(*projectWatcher)

	// Initially zero
	if !pw.LastSyncTime().IsZero() {
		t.Error("expected initial sync time to be zero")
	}

	// Update sync time
	pw.updateLastSyncTime()

	// Should be non-zero now
	if pw.LastSyncTime().IsZero() {
		t.Error("expected sync time to be updated")
	}

	// Should be recent (within last second)
	if time.Since(pw.LastSyncTime()) > time.Second {
		t.Error("expected sync time to be recent")
	}
}
