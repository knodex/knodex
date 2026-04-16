// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/history"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newHistoryRequest creates an HTTP request with path values set via Go 1.22+ routing.
func newHistoryRequest(method, path string, pathValues map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

func TestHistoryHandler_GetHistory_NotFound_FallsThrough(t *testing.T) {
	// In-memory service with no data — GetHistory returns "not found",
	// GetDeletedHistory also returns "not found" → expect 404.
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeNotFound, resp.Code)
}

func TestHistoryHandler_GetHistory_Found(t *testing.T) {
	svc := history.NewService(nil)

	// Seed history via RecordCreation
	err := svc.RecordCreation(t.Context(), "default", "MyKind", "my-instance", "test-rgd", "admin", models.DeploymentModeDirect)
	require.NoError(t, err)

	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHistoryHandler_GetHistory_DeletedHistory(t *testing.T) {
	svc := history.NewService(nil)

	// Create and then delete to move to deleted history
	err := svc.RecordCreation(t.Context(), "default", "MyKind", "my-instance", "test-rgd", "admin", models.DeploymentModeDirect)
	require.NoError(t, err)
	err = svc.RecordDeletion(t.Context(), "default", "MyKind", "my-instance", "admin")
	require.NoError(t, err)

	handler := NewHistoryHandler(svc, nil, nil)

	// Active history is gone, but deleted history should be found
	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHistoryHandler_ExportHistory_NotFound(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history/export",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.ExportHistory(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHistoryHandler_ExportHistory_Found(t *testing.T) {
	svc := history.NewService(nil)

	err := svc.RecordCreation(t.Context(), "default", "MyKind", "my-instance", "test-rgd", "admin", models.DeploymentModeDirect)
	require.NoError(t, err)

	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history/export?format=json",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.ExportHistory(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHistoryHandler_GetHistory_RedisUnavailable_Returns503(t *testing.T) {
	// Redis client pointing to a non-existent address triggers a connection error,
	// which is NOT a "not found" error → handler should return 503.
	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1", // invalid port
		DialTimeout: 1,             // fail fast (1ns)
	})
	svc := history.NewService(badClient)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeServiceUnavailable, resp.Code)
}

func TestHistoryHandler_ExportHistory_RedisUnavailable_Returns503(t *testing.T) {
	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1",
		DialTimeout: 1,
	})
	svc := history.NewService(badClient)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history/export",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.ExportHistory(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestHistoryHandler_GetTimeline_NotFound(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/timeline",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetTimeline(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeNotFound, resp.Code)
}

func TestHistoryHandler_GetTimeline_RedisUnavailable_Returns503(t *testing.T) {
	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1",
		DialTimeout: 1,
	})
	svc := history.NewService(badClient)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/timeline",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetTimeline(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeServiceUnavailable, resp.Code)
}

func TestHistoryHandler_NilService(t *testing.T) {
	handler := NewHistoryHandler(nil, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/history",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

// =============================================================================
// STORY-348: DNS-1123 Validation Tests
// =============================================================================

func TestHistoryHandler_GetHistory_InvalidKind_Returns400(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/INVALID_KIND/my-instance/history",
		map[string]string{"namespace": "default", "kind": "INVALID_KIND", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeBadRequest, resp.Code)
	assert.Contains(t, resp.Message, "kind must be a valid Kubernetes Kind name")
}

func TestHistoryHandler_GetHistory_InvalidName_Returns400(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/INVALID_NAME/history",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "INVALID_NAME"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeBadRequest, resp.Code)
	assert.Contains(t, resp.Message, "name must be a valid DNS-1123 subdomain")
}

func TestHistoryHandler_GetHistory_InvalidNamespace_Returns400(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/INVALID_NS/instances/MyKind/my-instance/history",
		map[string]string{"namespace": "INVALID_NS", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetHistory(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var resp response.ErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, response.ErrCodeBadRequest, resp.Code)
	assert.Contains(t, resp.Message, "namespace must be a valid DNS-1123 label")
}

func TestHistoryHandler_GetTimeline_InvalidKind_Returns400(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/bad_kind/my-instance/timeline",
		map[string]string{"namespace": "default", "kind": "bad_kind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetTimeline(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =============================================================================
// STORY-402: Revision Timeline Merge Tests
// =============================================================================

func TestHistoryHandler_GetTimeline_NilProvider_NoRevisionMarkers(t *testing.T) {
	svc := history.NewService(nil)
	err := svc.RecordCreation(t.Context(), "default", "MyKind", "my-instance", "test-rgd", "admin", models.DeploymentModeDirect)
	require.NoError(t, err)

	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/timeline",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetTimeline(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	timeline := result["timeline"].([]interface{})

	// No RevisionChanged events should appear
	for _, entry := range timeline {
		e := entry.(map[string]interface{})
		assert.NotEqual(t, "RevisionChanged", e["eventType"], "RevisionChanged should not appear with nil provider")
	}
}

func TestHistoryHandler_GetTimeline_WithProvider_MergedTimeline(t *testing.T) {
	svc := history.NewService(nil)
	err := svc.RecordCreation(t.Context(), "default", "MyKind", "my-instance", "test-rgd", "admin", models.DeploymentModeDirect)
	require.NoError(t, err)

	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{
			"test-rgd": {
				{RevisionNumber: 2, RGDName: "test-rgd", CreatedAt: time.Now().Add(20 * time.Minute)},
				{RevisionNumber: 1, RGDName: "test-rgd", CreatedAt: time.Now().Add(10 * time.Minute)},
			},
		},
	}

	handler := NewHistoryHandler(svc, provider, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/my-instance/timeline",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "my-instance"})
	rr := httptest.NewRecorder()
	handler.GetTimeline(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&result))
	timeline := result["timeline"].([]interface{})

	// Should have deployment events + 2 revision markers
	revisionCount := 0
	for _, entry := range timeline {
		e := entry.(map[string]interface{})
		if e["eventType"] == "RevisionChanged" {
			revisionCount++
		}
	}
	assert.Equal(t, 2, revisionCount, "expected 2 revision markers in merged timeline")
}

func TestMergeRevisionMarkers_MessageFormatting(t *testing.T) {
	baseTimeline := []models.TimelineEntry{
		{
			Timestamp: time.Now().Add(-2 * time.Hour),
			EventType: models.EventTypeCreated,
			Status:    "Pending",
			User:      "admin",
			IsCurrent: true,
		},
	}

	revisions := []models.GraphRevision{
		{RevisionNumber: 3, RGDName: "test-rgd", CreatedAt: time.Now().Add(-10 * time.Minute)},
		{RevisionNumber: 2, RGDName: "test-rgd", CreatedAt: time.Now().Add(-30 * time.Minute)},
		{RevisionNumber: 1, RGDName: "test-rgd", CreatedAt: time.Now().Add(-60 * time.Minute)},
	}

	// instanceCreatedAt before all revisions — all 3 should appear
	merged := mergeRevisionMarkers(baseTimeline, revisions, time.Now().Add(-3*time.Hour))

	// Should have 4 entries total (1 deployment + 3 revisions)
	require.Len(t, merged, 4)

	// Check revision messages
	var revEntries []models.TimelineEntry
	for _, e := range merged {
		if e.EventType == models.EventTypeRevisionChanged {
			revEntries = append(revEntries, e)
		}
	}
	require.Len(t, revEntries, 3)

	// Sorted ascending by timestamp, so rev 1 first, then 2, then 3
	assert.Equal(t, "RGD Revision 1 (initial)", revEntries[0].Message)
	assert.Equal(t, 1, revEntries[0].RevisionNumber)
	assert.Equal(t, 0, revEntries[0].PreviousRevision)

	assert.Equal(t, "RGD Revision 1 → 2", revEntries[1].Message)
	assert.Equal(t, 2, revEntries[1].RevisionNumber)
	assert.Equal(t, 1, revEntries[1].PreviousRevision)

	assert.Equal(t, "RGD Revision 2 → 3", revEntries[2].Message)
	assert.Equal(t, 3, revEntries[2].RevisionNumber)
	assert.Equal(t, 2, revEntries[2].PreviousRevision)
}

func TestMergeRevisionMarkers_MultipleRapidRevisions(t *testing.T) {
	baseTimeline := []models.TimelineEntry{
		{
			Timestamp: time.Now().Add(-2 * time.Hour),
			EventType: models.EventTypeCreated,
			Status:    "Pending",
		},
	}

	// 5 revisions created in rapid succession (1 second apart)
	now := time.Now()
	revisions := []models.GraphRevision{
		{RevisionNumber: 5, RGDName: "test-rgd", CreatedAt: now.Add(-1 * time.Second)},
		{RevisionNumber: 4, RGDName: "test-rgd", CreatedAt: now.Add(-2 * time.Second)},
		{RevisionNumber: 3, RGDName: "test-rgd", CreatedAt: now.Add(-3 * time.Second)},
		{RevisionNumber: 2, RGDName: "test-rgd", CreatedAt: now.Add(-4 * time.Second)},
		{RevisionNumber: 1, RGDName: "test-rgd", CreatedAt: now.Add(-5 * time.Second)},
	}

	merged := mergeRevisionMarkers(baseTimeline, revisions, time.Now().Add(-3*time.Hour))

	// Each revision appears as a separate marker
	revisionCount := 0
	for _, e := range merged {
		if e.EventType == models.EventTypeRevisionChanged {
			revisionCount++
		}
	}
	assert.Equal(t, 5, revisionCount, "each rapid revision should be a separate marker")
}

func TestMergeRevisionMarkers_CorrectFields(t *testing.T) {
	baseTimeline := []models.TimelineEntry{}

	revisions := []models.GraphRevision{
		{RevisionNumber: 2, RGDName: "test-rgd", CreatedAt: time.Now().Add(-10 * time.Minute)},
		{RevisionNumber: 1, RGDName: "test-rgd", CreatedAt: time.Now().Add(-20 * time.Minute)},
	}

	merged := mergeRevisionMarkers(baseTimeline, revisions, time.Time{})
	require.Len(t, merged, 2)

	// Check fields on revision entries (sorted ascending, so rev 1 first)
	rev1 := merged[0]
	assert.Equal(t, models.EventTypeRevisionChanged, rev1.EventType)
	assert.Equal(t, "system", rev1.User)
	assert.Equal(t, "", rev1.Status)
	assert.True(t, rev1.IsCompleted)
	assert.Equal(t, 1, rev1.RevisionNumber)
	assert.Equal(t, 0, rev1.PreviousRevision)

	rev2 := merged[1]
	assert.Equal(t, 2, rev2.RevisionNumber)
	assert.Equal(t, 1, rev2.PreviousRevision)
	// Last entry should be IsCurrent
	assert.True(t, rev2.IsCurrent)
	assert.False(t, rev1.IsCurrent)
}

func TestMergeRevisionMarkers_FiltersPreInstanceRevisions(t *testing.T) {
	instanceCreatedAt := time.Now().Add(-1 * time.Hour)

	baseTimeline := []models.TimelineEntry{
		{Timestamp: instanceCreatedAt, EventType: models.EventTypeCreated, Status: "Pending"},
	}

	revisions := []models.GraphRevision{
		{RevisionNumber: 3, RGDName: "test-rgd", CreatedAt: time.Now().Add(-10 * time.Minute)},  // after instance → included
		{RevisionNumber: 2, RGDName: "test-rgd", CreatedAt: time.Now().Add(-90 * time.Minute)},  // before instance → filtered
		{RevisionNumber: 1, RGDName: "test-rgd", CreatedAt: time.Now().Add(-120 * time.Minute)}, // before instance → filtered
	}

	merged := mergeRevisionMarkers(baseTimeline, revisions, instanceCreatedAt)

	revisionCount := 0
	for _, e := range merged {
		if e.EventType == models.EventTypeRevisionChanged {
			revisionCount++
			assert.Equal(t, 3, e.RevisionNumber, "only revision 3 should appear")
		}
	}
	assert.Equal(t, 1, revisionCount, "only post-instance revisions should appear")
}

func TestMergeRevisionMarkers_ZeroTimeSkipsFilter(t *testing.T) {
	baseTimeline := []models.TimelineEntry{}
	revisions := []models.GraphRevision{
		{RevisionNumber: 2, RGDName: "test-rgd", CreatedAt: time.Now().Add(-10 * time.Minute)},
		{RevisionNumber: 1, RGDName: "test-rgd", CreatedAt: time.Now().Add(-20 * time.Minute)},
	}

	// Zero instanceCreatedAt → no filtering, both revisions appear
	merged := mergeRevisionMarkers(baseTimeline, revisions, time.Time{})
	assert.Len(t, merged, 2)
}

func TestHistoryHandler_ExportHistory_InvalidName_Returns400(t *testing.T) {
	svc := history.NewService(nil)
	handler := NewHistoryHandler(svc, nil, nil)

	req := newHistoryRequest("GET", "/api/v1/namespaces/default/instances/MyKind/name_invalid/history/export",
		map[string]string{"namespace": "default", "kind": "MyKind", "name": "name_invalid"})
	rr := httptest.NewRecorder()
	handler.ExportHistory(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
