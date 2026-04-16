// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package diff_test

import (
	"testing"

	"github.com/knodex/knodex/server/internal/kro/diff"
	"github.com/knodex/knodex/server/internal/models"
)

// --- ComputeDiff tests ---

func TestComputeDiff_IdenticalSpecs(t *testing.T) {
	spec := map[string]interface{}{
		"apiVersion": "kro.run/v1alpha1",
		"kind":       "ResourceGraphDefinition",
		"spec": map[string]interface{}{
			"schema": map[string]interface{}{
				"apiVersion": "v1",
			},
		},
	}

	d, err := diff.ComputeDiff(spec, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Identical {
		t.Error("expected Identical=true for same spec")
	}
	if len(d.Added) != 0 || len(d.Removed) != 0 || len(d.Modified) != 0 {
		t.Errorf("expected empty diff, got added=%d removed=%d modified=%d",
			len(d.Added), len(d.Removed), len(d.Modified))
	}
}

func TestComputeDiff_EmptySpecs(t *testing.T) {
	d, err := diff.ComputeDiff(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Identical {
		t.Error("expected Identical=true for nil specs")
	}
}

func TestComputeDiff_AddedField(t *testing.T) {
	old := map[string]interface{}{"apiVersion": "v1"}
	new := map[string]interface{}{"apiVersion": "v1", "kind": "RGD"}

	d, err := diff.ComputeDiff(old, new)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Identical {
		t.Error("expected Identical=false")
	}
	if len(d.Added) != 1 {
		t.Fatalf("expected 1 added field, got %d", len(d.Added))
	}
	if d.Added[0].Path != "kind" {
		t.Errorf("expected added path='kind', got %q", d.Added[0].Path)
	}
	if d.Added[0].NewValue != "RGD" {
		t.Errorf("expected added value='RGD', got %v", d.Added[0].NewValue)
	}
}

func TestComputeDiff_RemovedField(t *testing.T) {
	old := map[string]interface{}{"apiVersion": "v1", "kind": "RGD"}
	new := map[string]interface{}{"apiVersion": "v1"}

	d, err := diff.ComputeDiff(old, new)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Removed) != 1 {
		t.Fatalf("expected 1 removed field, got %d", len(d.Removed))
	}
	if d.Removed[0].Path != "kind" {
		t.Errorf("expected removed path='kind', got %q", d.Removed[0].Path)
	}
	if d.Removed[0].OldValue != "RGD" {
		t.Errorf("expected old value='RGD', got %v", d.Removed[0].OldValue)
	}
}

func TestComputeDiff_ModifiedField(t *testing.T) {
	old := map[string]interface{}{"apiVersion": "v1alpha1"}
	new := map[string]interface{}{"apiVersion": "v1beta1"}

	d, err := diff.ComputeDiff(old, new)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Modified) != 1 {
		t.Fatalf("expected 1 modified field, got %d", len(d.Modified))
	}
	if d.Modified[0].Path != "apiVersion" {
		t.Errorf("expected modified path='apiVersion', got %q", d.Modified[0].Path)
	}
	if d.Modified[0].OldValue != "v1alpha1" {
		t.Errorf("expected old='v1alpha1', got %v", d.Modified[0].OldValue)
	}
	if d.Modified[0].NewValue != "v1beta1" {
		t.Errorf("expected new='v1beta1', got %v", d.Modified[0].NewValue)
	}
}

func TestComputeDiff_NestedField(t *testing.T) {
	old := map[string]interface{}{
		"spec": map[string]interface{}{
			"schema": map[string]interface{}{
				"version": "v1",
			},
		},
	}
	new := map[string]interface{}{
		"spec": map[string]interface{}{
			"schema": map[string]interface{}{
				"version": "v2",
			},
		},
	}

	d, err := diff.ComputeDiff(old, new)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Modified) != 1 {
		t.Fatalf("expected 1 modified, got %d: %v", len(d.Modified), d.Modified)
	}
	if d.Modified[0].Path != "spec.schema.version" {
		t.Errorf("expected path='spec.schema.version', got %q", d.Modified[0].Path)
	}
}

func TestComputeDiff_NilVsEmptySpec(t *testing.T) {
	d, err := diff.ComputeDiff(nil, map[string]interface{}{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Added) != 1 || d.Added[0].Path != "key" {
		t.Errorf("expected 1 added field 'key', got %v", d.Added)
	}
}

// --- DiffService / LRU cache tests ---

// mockProvider implements GraphRevisionProvider for testing.
type mockProvider struct {
	revisions map[string]map[int]*models.GraphRevision
	calls     int
}

func (m *mockProvider) GetRevision(rgdName string, revision int) (*models.GraphRevision, bool) {
	m.calls++
	if rgds, ok := m.revisions[rgdName]; ok {
		if rev, ok := rgds[revision]; ok {
			return rev, true
		}
	}
	return nil, false
}

func (m *mockProvider) ListRevisions(rgdName string) models.GraphRevisionList {
	return models.GraphRevisionList{}
}

func (m *mockProvider) GetLatestRevision(rgdName string) (*models.GraphRevision, bool) {
	return nil, false
}

func newMockProvider() *mockProvider {
	return &mockProvider{
		revisions: map[string]map[int]*models.GraphRevision{
			"my-rgd": {
				1: {RevisionNumber: 1, RGDName: "my-rgd", Snapshot: map[string]interface{}{"apiVersion": "v1"}},
				2: {RevisionNumber: 2, RGDName: "my-rgd", Snapshot: map[string]interface{}{"apiVersion": "v1", "kind": "RGD"}},
			},
		},
	}
}

func TestDiffService_GetDiff_CacheMiss(t *testing.T) {
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := newMockProvider()

	d, err := svc.GetDiff(provider, "my-rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Added) != 1 || d.Added[0].Path != "kind" {
		t.Errorf("expected 1 added field 'kind', got %v", d.Added)
	}
	if provider.calls != 2 {
		t.Errorf("expected 2 GetRevision calls on cache miss, got %d", provider.calls)
	}
}

func TestDiffService_GetDiff_CacheHit(t *testing.T) {
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := newMockProvider()

	// First call — cache miss.
	_, err = svc.GetDiff(provider, "my-rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	callsAfterFirst := provider.calls

	// Second call — should be a cache hit (no additional GetRevision calls).
	_, err = svc.GetDiff(provider, "my-rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.calls != callsAfterFirst {
		t.Errorf("expected cache hit (no extra calls), but got %d additional calls",
			provider.calls-callsAfterFirst)
	}
}

func TestDiffService_GetDiff_CanonicalOrdering(t *testing.T) {
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := newMockProvider()

	d1, err := svc.GetDiff(provider, "my-rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Call with reversed args — should use cache (same canonical key).
	callsAfter := provider.calls
	d2, err := svc.GetDiff(provider, "my-rgd", 2, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.calls != callsAfter {
		t.Error("reversed args should hit cache")
	}
	if d1.Rev1 != d2.Rev1 || d1.Rev2 != d2.Rev2 {
		t.Errorf("expected same result regardless of arg order")
	}
}

func TestDiffService_GetDiff_RevisionNotFound(t *testing.T) {
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := newMockProvider()

	_, err = svc.GetDiff(provider, "my-rgd", 1, 99)
	if err == nil {
		t.Error("expected error for missing revision")
	}
}

func TestDiffService_PreComputeConsecutiveDiff(t *testing.T) {
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := newMockProvider()

	// Pre-compute revision 2 (should compute diff between 1 and 2).
	svc.PreComputeConsecutiveDiff(provider, "my-rgd", 2)

	callsAfterPrecompute := provider.calls
	if callsAfterPrecompute == 0 {
		t.Error("expected GetRevision calls during pre-compute")
	}

	// GetDiff should now be a cache hit.
	_, err = svc.GetDiff(provider, "my-rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.calls != callsAfterPrecompute {
		t.Error("expected cache hit after pre-compute, but extra calls were made")
	}
}

func TestDiffService_PreComputeConsecutiveDiff_SkipsRevision1(t *testing.T) {
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	provider := newMockProvider()

	// Revision 1 has no predecessor — should be a no-op.
	svc.PreComputeConsecutiveDiff(provider, "my-rgd", 1)
	if provider.calls != 0 {
		t.Errorf("expected no GetRevision calls for revision 1, got %d", provider.calls)
	}
}

func TestDiffService_CacheEviction(t *testing.T) {
	// Use a very small cache (size=1) to test eviction.
	svc, err := diff.NewDiffServiceWithSize(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add two revisions so we can evict first entry.
	provider := &mockProvider{
		revisions: map[string]map[int]*models.GraphRevision{
			"rgd": {
				1: {RevisionNumber: 1, RGDName: "rgd", Snapshot: map[string]interface{}{"v": "1"}},
				2: {RevisionNumber: 2, RGDName: "rgd", Snapshot: map[string]interface{}{"v": "2"}},
				3: {RevisionNumber: 3, RGDName: "rgd", Snapshot: map[string]interface{}{"v": "3"}},
			},
		},
	}

	_, err = svc.GetDiff(provider, "rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls1 := provider.calls

	// This evicts (1,2) from cache.
	_, err = svc.GetDiff(provider, "rgd", 2, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls2 := provider.calls

	// (1,2) should be a cache miss again (evicted).
	_, err = svc.GetDiff(provider, "rgd", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All three calls should have caused provider.calls to increase.
	if provider.calls <= calls2 {
		t.Errorf("expected evicted entry to be re-fetched; calls1=%d calls2=%d total=%d",
			calls1, calls2, provider.calls)
	}
}
