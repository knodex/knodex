package watcher

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/knodex/knodex/server/internal/kro"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/testutil"
)

// createTestRGD delegates to testutil.NewUnstructuredRGD with default "Active" status.
func createTestRGD(name, namespace string, annotations map[string]string, labels map[string]string) *unstructured.Unstructured {
	return createTestRGDWithStatus(name, namespace, annotations, labels, "Active")
}

// createTestRGDWithStatus delegates to testutil.NewUnstructuredRGD with the given status.
func createTestRGDWithStatus(name, namespace string, annotations map[string]string, labels map[string]string, statusState string) *unstructured.Unstructured {
	var opts []testutil.RGDOption
	if annotations != nil {
		opts = append(opts, testutil.WithAnnotations(annotations))
	}
	if labels != nil {
		opts = append(opts, testutil.WithLabels(labels))
	}
	opts = append(opts, testutil.WithStatus(statusState))
	return testutil.NewUnstructuredRGD(name, namespace, opts...)
}

func TestNewRGDWatcherWithClient(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)

	watcher := NewRGDWatcherWithClient(fakeClient)

	if watcher == nil {
		t.Fatal("expected watcher to be created")
	}

	if watcher.cache == nil {
		t.Error("expected cache to be initialized")
	}

	if watcher.IsSynced() {
		t.Error("expected watcher not to be synced initially")
	}

	if watcher.IsRunning() {
		t.Error("expected watcher not to be running initially")
	}
}

func TestRGDWatcher_ShouldIncludeInCatalog(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			name: "catalog annotation true",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			expected: true,
		},
		{
			name: "catalog annotation yes",
			annotations: map[string]string{
				kro.CatalogAnnotation: "yes",
			},
			expected: true,
		},
		{
			name: "catalog annotation 1",
			annotations: map[string]string{
				kro.CatalogAnnotation: "1",
			},
			expected: true,
		},
		{
			name: "catalog annotation TRUE (case insensitive)",
			annotations: map[string]string{
				kro.CatalogAnnotation: "TRUE",
			},
			expected: true,
		},
		{
			name: "catalog annotation false",
			annotations: map[string]string{
				kro.CatalogAnnotation: "false",
			},
			expected: false,
		},
		{
			name: "catalog annotation no",
			annotations: map[string]string{
				kro.CatalogAnnotation: "no",
			},
			expected: false,
		},
		{
			name: "other annotations only",
			annotations: map[string]string{
				"other-annotation": "true",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgd := createTestRGD("test-rgd", "default", tt.annotations, nil)
			result := watcher.shouldIncludeInCatalog(rgd)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRGDWatcher_UnstructuredToRGD(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "A test RGD for PostgreSQL",
		kro.TagsAnnotation:        "database, postgres, aws",
		kro.CategoryAnnotation:    "database",
		kro.IconAnnotation:        "postgres-icon",
		kro.VersionAnnotation:     "v2.0",
	}

	labels := map[string]string{
		"app":     "postgres",
		"env":     "prod",
		"version": "14",
	}

	u := createTestRGD("postgres-cluster", "databases", annotations, labels)

	rgd := watcher.unstructuredToRGD(u)

	if rgd.Name != "postgres-cluster" {
		t.Errorf("expected name 'postgres-cluster', got %q", rgd.Name)
	}
	if rgd.Namespace != "databases" {
		t.Errorf("expected namespace 'databases', got %q", rgd.Namespace)
	}
	if rgd.Description != "A test RGD for PostgreSQL" {
		t.Errorf("expected description, got %q", rgd.Description)
	}
	if rgd.Version != "v2.0" {
		t.Errorf("expected version 'v2.0', got %q", rgd.Version)
	}
	if rgd.Category != "database" {
		t.Errorf("expected category 'database', got %q", rgd.Category)
	}
	if rgd.Icon != "postgres-icon" {
		t.Errorf("expected icon 'postgres-icon', got %q", rgd.Icon)
	}

	// Check tags
	expectedTags := []string{"database", "postgres", "aws"}
	if len(rgd.Tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(rgd.Tags))
	}
	for i, tag := range expectedTags {
		if rgd.Tags[i] != tag {
			t.Errorf("expected tag %q at index %d, got %q", tag, i, rgd.Tags[i])
		}
	}

	// Check organization (no organization label, should be empty = shared)
	if rgd.Organization != "" {
		t.Errorf("expected empty organization for RGD without org label, got %q", rgd.Organization)
	}

	// Check title (no title annotation, should fall back to name)
	if rgd.Title != "postgres-cluster" {
		t.Errorf("expected title to fall back to name 'postgres-cluster', got %q", rgd.Title)
	}

	// Check labels
	if rgd.Labels["app"] != "postgres" {
		t.Errorf("expected label app=postgres, got %q", rgd.Labels["app"])
	}

	// Check API version and kind from spec
	if rgd.APIVersion != "example.com/v1" {
		t.Errorf("expected apiVersion 'example.com/v1', got %q", rgd.APIVersion)
	}
	if rgd.Kind != "TestResource" {
		t.Errorf("expected kind 'TestResource', got %q", rgd.Kind)
	}
}

func TestRGDWatcher_UnstructuredToRGD_TitleAnnotation(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
		kro.TitleAnnotation:   "Prometheus Monitoring Stack",
	}

	u := createTestRGD("prometheus-stack", "default", annotations, nil)
	rgd := watcher.unstructuredToRGD(u)

	if rgd.Title != "Prometheus Monitoring Stack" {
		t.Errorf("expected title 'Prometheus Monitoring Stack', got %q", rgd.Title)
	}
	if rgd.Name != "prometheus-stack" {
		t.Errorf("expected name 'prometheus-stack', got %q", rgd.Name)
	}
}

func TestRGDWatcher_UnstructuredToRGD_TitleEmptyAnnotationFallback(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
		kro.TitleAnnotation:   "", // Empty title annotation - should fall back to name
	}

	u := createTestRGD("my-app", "default", annotations, nil)
	rgd := watcher.unstructuredToRGD(u)

	if rgd.Title != "my-app" {
		t.Errorf("expected empty title annotation to fall back to name 'my-app', got %q", rgd.Title)
	}
}

func TestRGDWatcher_UnstructuredToRGD_TitleFallbackToName(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
		// No title annotation - should fall back to name
	}

	u := createTestRGD("redis-cache", "default", annotations, nil)
	rgd := watcher.unstructuredToRGD(u)

	if rgd.Title != "redis-cache" {
		t.Errorf("expected title to fall back to name 'redis-cache', got %q", rgd.Title)
	}
}

func TestRGDWatcher_UnstructuredToRGD_DefaultVersion(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
		// No version annotation
	}

	u := createTestRGD("test-rgd", "default", annotations, nil)
	rgd := watcher.unstructuredToRGD(u)

	if rgd.Version != "v1" {
		t.Errorf("expected default version 'v1', got %q", rgd.Version)
	}
}

func TestRGDWatcher_HandleAdd(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add an RGD with catalog annotation
	annotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "Test RGD",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)

	watcher.handleAdd(rgd)

	// Verify it was added to cache
	cached, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to be added to cache")
	}
	if cached.Description != "Test RGD" {
		t.Errorf("expected description, got %q", cached.Description)
	}
}

func TestRGDWatcher_HandleAdd_NoCatalogAnnotation(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add an RGD without catalog annotation
	rgd := createTestRGD("test-rgd", "default", nil, nil)

	watcher.handleAdd(rgd)

	// Verify it was NOT added to cache
	_, found := watcher.cache.Get("default", "test-rgd")
	if found {
		t.Error("expected RGD without catalog annotation not to be cached")
	}
}

func TestRGDWatcher_HandleUpdate(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add initial RGD
	annotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "Original",
	}
	oldRGD := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(oldRGD)

	// Update RGD
	newAnnotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "Updated",
	}
	newRGD := createTestRGD("test-rgd", "default", newAnnotations, nil)
	watcher.handleUpdate(oldRGD, newRGD)

	// Verify it was updated in cache
	cached, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to remain in cache after update")
	}
	if cached.Description != "Updated" {
		t.Errorf("expected updated description, got %q", cached.Description)
	}
}

func TestRGDWatcher_HandleUpdate_RemoveFromCatalog(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add initial RGD with catalog annotation
	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	oldRGD := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(oldRGD)

	// Update RGD to remove catalog annotation
	newAnnotations := map[string]string{
		kro.CatalogAnnotation: "false",
	}
	newRGD := createTestRGD("test-rgd", "default", newAnnotations, nil)
	watcher.handleUpdate(oldRGD, newRGD)

	// Verify it was removed from cache
	_, found := watcher.cache.Get("default", "test-rgd")
	if found {
		t.Error("expected RGD to be removed from cache when catalog annotation removed")
	}
}

func TestRGDWatcher_HandleDelete(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add RGD
	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(rgd)

	// Verify it's in cache
	_, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to be in cache before delete")
	}

	// Delete RGD
	watcher.handleDelete(rgd)

	// Verify it was removed from cache
	_, found = watcher.cache.Get("default", "test-rgd")
	if found {
		t.Error("expected RGD to be removed from cache after delete")
	}
}

func TestRGDWatcher_ListRGDs(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add multiple RGDs
	for i := 0; i < 5; i++ {
		annotations := map[string]string{
			kro.CatalogAnnotation: "true",
			kro.CategoryAnnotation: func() string {
				if i%2 == 0 {
					return "database"
				}
				return "cache"
			}(),
		}
		rgd := createTestRGD("rgd-"+string(rune('a'+i)), "default", annotations, nil)
		watcher.handleAdd(rgd)
	}

	// List all
	opts := models.DefaultListOptions()
	result := watcher.ListRGDs(opts)

	if result.TotalCount != 5 {
		t.Errorf("expected 5 RGDs, got %d", result.TotalCount)
	}

	// List with filter
	opts.Category = "database"
	result = watcher.ListRGDs(opts)

	if result.TotalCount != 3 {
		t.Errorf("expected 3 database RGDs, got %d", result.TotalCount)
	}
}

func TestRGDWatcher_GetRGD(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "Test RGD",
	}
	rgd := createTestRGD("test-rgd", "test-ns", annotations, nil)
	watcher.handleAdd(rgd)

	// Get by namespace and name
	cached, found := watcher.GetRGD("test-ns", "test-rgd")
	if !found {
		t.Fatal("expected to find RGD")
	}
	if cached.Name != "test-rgd" {
		t.Errorf("expected name 'test-rgd', got %q", cached.Name)
	}

	// Get non-existent
	_, found = watcher.GetRGD("test-ns", "nonexistent")
	if found {
		t.Error("expected not to find nonexistent RGD")
	}
}

func TestRGDWatcher_GetRGDByName(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd1 := createTestRGD("unique-rgd", "ns1", annotations, nil)
	rgd2 := createTestRGD("other-rgd", "ns2", annotations, nil)
	watcher.handleAdd(rgd1)
	watcher.handleAdd(rgd2)

	// Find by name across namespaces
	cached, found := watcher.GetRGDByName("unique-rgd")
	if !found {
		t.Fatal("expected to find RGD by name")
	}
	if cached.Namespace != "ns1" {
		t.Errorf("expected namespace 'ns1', got %q", cached.Namespace)
	}

	// Non-existent name
	_, found = watcher.GetRGDByName("nonexistent")
	if found {
		t.Error("expected not to find nonexistent RGD by name")
	}
}

// TestRGDWatcher_StartStop tests basic start/stop lifecycle
// Note: Full informer integration tests require a real or properly configured fake cluster
// This test verifies the basic state transitions without running the full informer
func TestRGDWatcher_StartStop(t *testing.T) {
	t.Skip("Skipping: requires properly configured fake client with custom list kinds")
}

// TestRGDWatcher_StartIdempotent is skipped for similar reasons
func TestRGDWatcher_StartIdempotent(t *testing.T) {
	t.Skip("Skipping: requires properly configured fake client with custom list kinds")
}

func TestRGDWatcher_StopIdempotent(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Stop without starting - should not panic
	watcher.Stop()

	// Stop again - should not panic
	watcher.Stop()
}

func TestRGDWatcher_StopAndWaitIdempotent(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// StopAndWait without starting - should return true immediately
	result := watcher.StopAndWait(time.Second)
	if !result {
		t.Error("expected StopAndWait to return true when not running")
	}

	// StopAndWait again - should not panic and return true
	result = watcher.StopAndWait(time.Second)
	if !result {
		t.Error("expected StopAndWait to return true on second call")
	}
}

// TestRGDWatcher_RestartAfterStop verifies that Start() can be called after StopAndWait()
// Note: Full lifecycle test is skipped because it requires properly configured fake client.
// This test verifies the state machine allows restart without panic.
func TestRGDWatcher_RestartAfterStop(t *testing.T) {
	t.Skip("Skipping: requires properly configured fake client with custom list kinds")

	// When properly configured, this test should:
	// 1. Start the watcher
	// 2. Verify it's running
	// 3. Call StopAndWait and verify clean shutdown
	// 4. Call Start again and verify it's running again
	// 5. Call StopAndWait again and verify clean shutdown
}

func TestRGDWatcher_Cache(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	cache := watcher.Cache()

	if cache == nil {
		t.Error("expected Cache() to return non-nil cache")
	}

	if cache != watcher.cache {
		t.Error("expected Cache() to return the same cache instance")
	}
}

func TestRGDWatcher_HandleDeleteTombstone(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add RGD first
	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(rgd)

	// Simulate a tombstone delete (DeletedFinalStateUnknown)
	// This happens when the watch was disconnected and we missed the delete event
	// The informer gives us a DeletedFinalStateUnknown wrapper
	// For this test, we just call handleDelete directly with the RGD
	// In real scenarios, the cache.DeletedFinalStateUnknown wraps the object

	watcher.handleDelete(rgd)

	// Verify it was removed
	_, found := watcher.cache.Get("default", "test-rgd")
	if found {
		t.Error("expected RGD to be removed after delete")
	}
}

func TestRGDWatcher_TagsParsing(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	tests := []struct {
		name         string
		tagsValue    string
		expectedTags []string
	}{
		{
			name:         "simple tags",
			tagsValue:    "aws,database,postgres",
			expectedTags: []string{"aws", "database", "postgres"},
		},
		{
			name:         "tags with spaces",
			tagsValue:    "aws, database , postgres",
			expectedTags: []string{"aws", "database", "postgres"},
		},
		{
			name:         "empty tags",
			tagsValue:    "",
			expectedTags: nil,
		},
		{
			name:         "single tag",
			tagsValue:    "database",
			expectedTags: []string{"database"},
		},
		{
			name:         "tags with empty entries",
			tagsValue:    "aws,,postgres",
			expectedTags: []string{"aws", "postgres"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{
				kro.CatalogAnnotation: "true",
			}
			if tt.tagsValue != "" {
				annotations[kro.TagsAnnotation] = tt.tagsValue
			}

			rgd := createTestRGD("test-rgd", "default", annotations, nil)
			result := watcher.unstructuredToRGD(rgd)

			if len(result.Tags) != len(tt.expectedTags) {
				t.Errorf("expected %d tags, got %d: %v", len(tt.expectedTags), len(result.Tags), result.Tags)
				return
			}

			for i, expected := range tt.expectedTags {
				if result.Tags[i] != expected {
					t.Errorf("expected tag %q at index %d, got %q", expected, i, result.Tags[i])
				}
			}
		})
	}
}

func TestRGDWatcher_UpdatedAtTimestamp_InitialAdd(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)

	// Convert to our model
	result := watcher.unstructuredToRGD(rgd)

	// On initial creation, UpdatedAt should equal CreatedAt
	if !result.UpdatedAt.Equal(result.CreatedAt) {
		t.Errorf("expected UpdatedAt to equal CreatedAt on initial creation, got UpdatedAt=%v CreatedAt=%v",
			result.UpdatedAt, result.CreatedAt)
	}
}

func TestRGDWatcher_UpdatedAtTimestamp_ReSyncNoChange(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add initial RGD
	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(rgd)

	// Get the initial timestamp
	initial, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to be in cache")
	}
	initialUpdatedAt := initial.UpdatedAt

	// Wait a bit to ensure time difference would be detectable
	time.Sleep(10 * time.Millisecond)

	// Simulate a re-sync event: same resourceVersion, no actual changes
	// In Kubernetes, informers re-sync every 30 seconds even without changes
	watcher.handleUpdate(rgd, rgd)

	// Get the RGD after re-sync
	updated, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to remain in cache after re-sync")
	}

	// UpdatedAt should NOT have changed for a re-sync event
	if !updated.UpdatedAt.Equal(initialUpdatedAt) {
		t.Errorf("expected UpdatedAt to remain unchanged on re-sync, got initial=%v updated=%v",
			initialUpdatedAt, updated.UpdatedAt)
	}
}

func TestRGDWatcher_UpdatedAtTimestamp_ActualChange(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add initial RGD with resourceVersion "1"
	annotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "Original description",
	}
	oldRGD := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(oldRGD)

	// Get the initial timestamp
	initial, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to be in cache")
	}
	initialUpdatedAt := initial.UpdatedAt

	// Wait to ensure time difference is detectable
	time.Sleep(10 * time.Millisecond)

	// Create an updated RGD with different resourceVersion
	newAnnotations := map[string]string{
		kro.CatalogAnnotation:     "true",
		kro.DescriptionAnnotation: "Updated description",
	}
	newRGD := createTestRGD("test-rgd", "default", newAnnotations, nil)

	// Change the resourceVersion to simulate an actual update
	metadata := newRGD.Object["metadata"].(map[string]interface{})
	metadata["resourceVersion"] = "2"

	// Handle the update
	watcher.handleUpdate(oldRGD, newRGD)

	// Get the RGD after update
	updated, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to remain in cache after update")
	}

	// UpdatedAt SHOULD have changed for an actual resource change
	if updated.UpdatedAt.Equal(initialUpdatedAt) || updated.UpdatedAt.Before(initialUpdatedAt) {
		t.Errorf("expected UpdatedAt to be updated after actual change, got initial=%v updated=%v",
			initialUpdatedAt, updated.UpdatedAt)
	}

	// Verify the description was updated
	if updated.Description != "Updated description" {
		t.Errorf("expected description to be updated, got %q", updated.Description)
	}
}

func TestRGDWatcher_UpdatedAtTimestamp_MultipleReSyncs(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add initial RGD
	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(rgd)

	// Get the initial timestamp
	initial, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to be in cache")
	}
	initialUpdatedAt := initial.UpdatedAt

	// Simulate multiple re-sync events
	for i := 0; i < 5; i++ {
		time.Sleep(5 * time.Millisecond)
		watcher.handleUpdate(rgd, rgd)
	}

	// Get the RGD after multiple re-syncs
	updated, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to remain in cache after multiple re-syncs")
	}

	// UpdatedAt should still be the same as initial
	if !updated.UpdatedAt.Equal(initialUpdatedAt) {
		t.Errorf("expected UpdatedAt to remain unchanged after multiple re-syncs, got initial=%v updated=%v",
			initialUpdatedAt, updated.UpdatedAt)
	}
}

func TestRGDWatcher_UpdatedAtTimestamp_ChangeAfterReSyncs(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add initial RGD with resourceVersion "1"
	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(rgd)

	initial, _ := watcher.cache.Get("default", "test-rgd")
	initialUpdatedAt := initial.UpdatedAt

	// Simulate multiple re-sync events (no timestamp change expected)
	for i := 0; i < 3; i++ {
		time.Sleep(5 * time.Millisecond)
		watcher.handleUpdate(rgd, rgd)
	}

	// Verify timestamp hasn't changed after re-syncs
	afterResyncs, _ := watcher.cache.Get("default", "test-rgd")
	if !afterResyncs.UpdatedAt.Equal(initialUpdatedAt) {
		t.Errorf("timestamp changed during re-syncs when it shouldn't")
	}

	time.Sleep(10 * time.Millisecond)

	// Now make an actual change (different resourceVersion)
	updatedRGD := createTestRGD("test-rgd", "default", annotations, nil)
	metadata := updatedRGD.Object["metadata"].(map[string]interface{})
	metadata["resourceVersion"] = "2"

	watcher.handleUpdate(rgd, updatedRGD)

	// Verify timestamp DID change after actual update
	afterUpdate, _ := watcher.cache.Get("default", "test-rgd")
	if afterUpdate.UpdatedAt.Equal(initialUpdatedAt) || afterUpdate.UpdatedAt.Before(initialUpdatedAt) {
		t.Errorf("expected UpdatedAt to change after actual update, got initial=%v updated=%v",
			initialUpdatedAt, afterUpdate.UpdatedAt)
	}

	// Do more re-syncs with the new version
	newUpdatedAt := afterUpdate.UpdatedAt
	for i := 0; i < 3; i++ {
		time.Sleep(5 * time.Millisecond)
		watcher.handleUpdate(updatedRGD, updatedRGD)
	}

	// Timestamp should stay at the last actual update time
	final, _ := watcher.cache.Get("default", "test-rgd")
	if !final.UpdatedAt.Equal(newUpdatedAt) {
		t.Errorf("expected UpdatedAt to remain stable after re-syncs following update, got updated=%v final=%v",
			newUpdatedAt, final.UpdatedAt)
	}
}

func TestRGDWatcher_OrganizationLabelParsing(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	tests := []struct {
		name        string
		annotations map[string]string
		labels      map[string]string
		expectedOrg string
	}{
		{
			name: "organization label extracted and stored",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			labels: map[string]string{
				kro.RGDOrganizationLabel: "orgA",
			},
			expectedOrg: "orgA",
		},
		{
			name: "no organization label results in empty string",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			expectedOrg: "",
		},
		{
			name: "organization and project labels coexist independently",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			labels: map[string]string{
				kro.RGDOrganizationLabel: "orgA",
				kro.RGDProjectLabel:      "payments",
			},
			expectedOrg: "orgA",
		},
		{
			name: "whitespace-only organization label normalizes to empty string",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			labels: map[string]string{
				kro.RGDOrganizationLabel: "  ",
			},
			expectedOrg: "",
		},
		{
			name: "organization label with leading/trailing whitespace is trimmed",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			labels: map[string]string{
				kro.RGDOrganizationLabel: " orgA ",
			},
			expectedOrg: "orgA",
		},
		{
			name: "organization set as annotation (not label) does NOT populate field",
			annotations: map[string]string{
				kro.CatalogAnnotation:    "true",
				kro.RGDOrganizationLabel: "orgA",
			},
			labels:      nil,
			expectedOrg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgd := createTestRGD("test-rgd", "default", tt.annotations, tt.labels)
			result := watcher.unstructuredToRGD(rgd)

			if result.Organization != tt.expectedOrg {
				t.Errorf("expected Organization %q, got %q", tt.expectedOrg, result.Organization)
			}

			// For coexistence test: verify project label is independently preserved
			if tt.labels != nil && tt.labels[kro.RGDProjectLabel] != "" {
				if result.Labels[kro.RGDProjectLabel] != tt.labels[kro.RGDProjectLabel] {
					t.Errorf("expected project label %q, got %q",
						tt.labels[kro.RGDProjectLabel], result.Labels[kro.RGDProjectLabel])
				}
			}
		})
	}
}

func TestRGDWatcher_DeploymentModesParsing(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	tests := []struct {
		name          string
		modesValue    string
		expectedModes []string
	}{
		{
			name:          "no annotation - all modes allowed",
			modesValue:    "",
			expectedModes: nil,
		},
		{
			name:          "single mode - gitops",
			modesValue:    "gitops",
			expectedModes: []string{"gitops"},
		},
		{
			name:          "single mode - direct",
			modesValue:    "direct",
			expectedModes: []string{"direct"},
		},
		{
			name:          "two modes - direct,gitops",
			modesValue:    "direct,gitops",
			expectedModes: []string{"direct", "gitops"},
		},
		{
			name:          "all three modes",
			modesValue:    "direct,gitops,hybrid",
			expectedModes: []string{"direct", "gitops", "hybrid"},
		},
		{
			name:          "case insensitive - GitOps",
			modesValue:    "GitOps",
			expectedModes: []string{"gitops"},
		},
		{
			name:          "case insensitive - DIRECT,HYBRID",
			modesValue:    "DIRECT,HYBRID",
			expectedModes: []string{"direct", "hybrid"},
		},
		{
			name:          "with whitespace",
			modesValue:    " gitops , hybrid ",
			expectedModes: []string{"gitops", "hybrid"},
		},
		{
			name:          "invalid mode ignored",
			modesValue:    "gitops,invalid,direct",
			expectedModes: []string{"gitops", "direct"},
		},
		{
			name:          "all invalid modes - returns nil",
			modesValue:    "foo,bar,baz",
			expectedModes: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{
				kro.CatalogAnnotation: "true",
			}
			if tt.modesValue != "" {
				annotations[kro.DeploymentModesAnnotation] = tt.modesValue
			}

			rgd := createTestRGD("test-rgd", "default", annotations, nil)
			result := watcher.unstructuredToRGD(rgd)

			if len(result.AllowedDeploymentModes) != len(tt.expectedModes) {
				t.Errorf("expected %d modes, got %d: %v", len(tt.expectedModes), len(result.AllowedDeploymentModes), result.AllowedDeploymentModes)
				return
			}

			for i, expected := range tt.expectedModes {
				if result.AllowedDeploymentModes[i] != expected {
					t.Errorf("expected mode %q at index %d, got %q", expected, i, result.AllowedDeploymentModes[i])
				}
			}
		})
	}
}

func TestRGDWatcher_DeploymentModes_InvalidModesLogWarning(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Test that invalid modes are detected (we verify via the result, not log capture)
	// The warning is logged in unstructuredToRGD when invalid modes are present
	annotations := map[string]string{
		kro.CatalogAnnotation:         "true",
		kro.DeploymentModesAnnotation: "gitops,invalid,direct,unknown",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)

	// Parse and verify the result includes valid modes only
	result := watcher.unstructuredToRGD(rgd)

	// Should have 2 valid modes (gitops, direct), invalid modes filtered out
	if len(result.AllowedDeploymentModes) != 2 {
		t.Errorf("expected 2 valid modes, got %d: %v", len(result.AllowedDeploymentModes), result.AllowedDeploymentModes)
	}

	// Verify ParseDeploymentModesWithInvalid detects invalid modes
	parseResult := models.ParseDeploymentModesWithInvalid("gitops,invalid,direct,unknown")
	if len(parseResult.InvalidModes) != 2 {
		t.Errorf("expected 2 invalid modes, got %d: %v", len(parseResult.InvalidModes), parseResult.InvalidModes)
	}
	if parseResult.InvalidModes[0] != "invalid" || parseResult.InvalidModes[1] != "unknown" {
		t.Errorf("expected invalid modes [invalid, unknown], got %v", parseResult.InvalidModes)
	}
}

func TestRGDWatcher_DeploymentModes_HandleAddUpdate(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	// Add RGD with deployment modes restriction
	annotations := map[string]string{
		kro.CatalogAnnotation:         "true",
		kro.DeploymentModesAnnotation: "gitops",
	}
	rgd := createTestRGD("test-rgd", "default", annotations, nil)
	watcher.handleAdd(rgd)

	// Verify modes are in cache
	cached, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to be in cache")
	}
	if len(cached.AllowedDeploymentModes) != 1 || cached.AllowedDeploymentModes[0] != "gitops" {
		t.Errorf("expected [gitops], got %v", cached.AllowedDeploymentModes)
	}

	// Update to allow more modes
	newAnnotations := map[string]string{
		kro.CatalogAnnotation:         "true",
		kro.DeploymentModesAnnotation: "direct,gitops,hybrid",
	}
	newRGD := createTestRGD("test-rgd", "default", newAnnotations, nil)
	// Change resourceVersion to trigger update
	metadata := newRGD.Object["metadata"].(map[string]interface{})
	metadata["resourceVersion"] = "2"

	watcher.handleUpdate(rgd, newRGD)

	// Verify updated modes
	updated, found := watcher.cache.Get("default", "test-rgd")
	if !found {
		t.Fatal("expected RGD to remain in cache after update")
	}
	if len(updated.AllowedDeploymentModes) != 3 {
		t.Errorf("expected 3 modes after update, got %d: %v", len(updated.AllowedDeploymentModes), updated.AllowedDeploymentModes)
	}
}

// --- Status filtering tests (STORY-223) ---

func TestRGDWatcher_ShouldIncludeInCatalog_StatusFiltering(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	tests := []struct {
		name        string
		annotations map[string]string
		status      string // empty string = no status field
		expected    bool
	}{
		{
			name: "catalog true + Active status → included",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			status:   "Active",
			expected: true,
		},
		{
			name: "catalog true + Inactive status → excluded",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			status:   "Inactive",
			expected: false,
		},
		{
			name: "catalog true + no status (not yet processed) → excluded",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			status:   "", // No status field
			expected: false,
		},
		{
			name:        "no catalog annotation + Active status → excluded",
			annotations: map[string]string{},
			status:      "Active",
			expected:    false,
		},
		{
			name: "catalog true + unknown status string → excluded",
			annotations: map[string]string{
				kro.CatalogAnnotation: "true",
			},
			status:   "Processing",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgd := createTestRGDWithStatus("test-rgd", "default", tt.annotations, nil, tt.status)
			result := watcher.shouldIncludeInCatalog(rgd)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRGDWatcher_HandleAdd_InactiveExcluded(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}

	// Add RGD with Inactive status - should NOT be added to cache
	rgd := createTestRGDWithStatus("inactive-rgd", "default", annotations, nil, "Inactive")
	watcher.handleAdd(rgd)

	_, found := watcher.cache.Get("default", "inactive-rgd")
	if found {
		t.Error("expected Inactive RGD not to be added to cache")
	}
}

func TestRGDWatcher_HandleAdd_NoStatusExcluded(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}

	// Add RGD with no status (not yet processed by KRO)
	rgd := createTestRGDWithStatus("new-rgd", "default", annotations, nil, "")
	watcher.handleAdd(rgd)

	_, found := watcher.cache.Get("default", "new-rgd")
	if found {
		t.Error("expected RGD without status not to be added to cache")
	}
}

func TestRGDWatcher_HandleUpdate_InactiveToActive(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}

	// Initially Inactive - not in cache
	oldRGD := createTestRGDWithStatus("transitioning-rgd", "default", annotations, nil, "Inactive")
	watcher.handleAdd(oldRGD)
	_, found := watcher.cache.Get("default", "transitioning-rgd")
	if found {
		t.Fatal("expected Inactive RGD not to be in cache initially")
	}

	// Now transitions to Active via update event
	newRGD := createTestRGDWithStatus("transitioning-rgd", "default", annotations, nil, "Active")
	metadata := newRGD.Object["metadata"].(map[string]interface{})
	metadata["resourceVersion"] = "2"

	watcher.handleUpdate(oldRGD, newRGD)

	// Should now be in cache
	cached, found := watcher.cache.Get("default", "transitioning-rgd")
	if !found {
		t.Fatal("expected RGD to be added to cache after Inactive→Active transition")
	}
	if cached.Name != "transitioning-rgd" {
		t.Errorf("expected name 'transitioning-rgd', got %q", cached.Name)
	}
}

func TestRGDWatcher_HandleUpdate_ActiveToInactive(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	annotations := map[string]string{
		kro.CatalogAnnotation: "true",
	}

	// Initially Active - in cache
	oldRGD := createTestRGDWithStatus("transitioning-rgd", "default", annotations, nil, "Active")
	watcher.handleAdd(oldRGD)
	_, found := watcher.cache.Get("default", "transitioning-rgd")
	if !found {
		t.Fatal("expected Active RGD to be in cache initially")
	}

	// Transitions to Inactive (e.g., schema validation fails after edit)
	newRGD := createTestRGDWithStatus("transitioning-rgd", "default", annotations, nil, "Inactive")
	metadata := newRGD.Object["metadata"].(map[string]interface{})
	metadata["resourceVersion"] = "2"

	watcher.handleUpdate(oldRGD, newRGD)

	// Should be removed from cache
	_, found = watcher.cache.Get("default", "transitioning-rgd")
	if found {
		t.Error("expected RGD to be removed from cache after Active→Inactive transition")
	}
}

func TestRGDWatcher_UnstructuredToRGD_StatusField(t *testing.T) {
	fakeClient := testutil.NewFakeDynamicClient(t)
	watcher := NewRGDWatcherWithClient(fakeClient)

	tests := []struct {
		name           string
		statusState    string
		expectedStatus string
	}{
		{
			name:           "Active status",
			statusState:    "Active",
			expectedStatus: "Active",
		},
		{
			name:           "Inactive status",
			statusState:    "Inactive",
			expectedStatus: "Inactive",
		},
		{
			name:           "no status defaults to Inactive",
			statusState:    "",
			expectedStatus: "Inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{
				kro.CatalogAnnotation: "true",
			}
			rgd := createTestRGDWithStatus("test-rgd", "default", annotations, nil, tt.statusState)
			result := watcher.unstructuredToRGD(rgd)

			if result.Status != tt.expectedStatus {
				t.Errorf("expected Status %q, got %q", tt.expectedStatus, result.Status)
			}
		})
	}
}

// TestRGDWatcher_GetRGDByKind tests the GetRGDByKind method used for cross-RGD resolution
func TestRGDWatcher_GetRGDByKind(t *testing.T) {
	cache := NewRGDCache()
	cache.Set(&models.CatalogRGD{
		Name:      "akv-eso-binding",
		Namespace: "default",
		Kind:      "AKVESOBinding",
	})
	cache.Set(&models.CatalogRGD{
		Name:      "azure-key-vault",
		Namespace: "default",
		Kind:      "AzureKeyVault",
	})

	watcher := NewRGDWatcherWithCache(cache)

	tests := []struct {
		name      string
		kind      string
		wantFound bool
		wantName  string
	}{
		{
			name:      "find AKVESOBinding",
			kind:      "AKVESOBinding",
			wantFound: true,
			wantName:  "akv-eso-binding",
		},
		{
			name:      "find AzureKeyVault",
			kind:      "AzureKeyVault",
			wantFound: true,
			wantName:  "azure-key-vault",
		},
		{
			name:      "not found",
			kind:      "NonExistentKind",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rgd, found := watcher.GetRGDByKind(tt.kind)
			if found != tt.wantFound {
				t.Errorf("GetRGDByKind(%q) found = %v, want %v", tt.kind, found, tt.wantFound)
			}
			if found && rgd.Name != tt.wantName {
				t.Errorf("GetRGDByKind(%q) name = %q, want %q", tt.kind, rgd.Name, tt.wantName)
			}
		})
	}
}
