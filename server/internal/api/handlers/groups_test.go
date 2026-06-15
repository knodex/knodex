// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/knodex/knodex/server/internal/groups"
)

// fakeObservedGroupsLister is a fake observedGroupsLister for handler tests.
type fakeObservedGroupsLister struct {
	items []groups.ObservedGroup
	err   error
}

func (f *fakeObservedGroupsLister) List(_ context.Context) ([]groups.ObservedGroup, error) {
	return f.items, f.err
}

func TestGroupsHandler_ListObserved_Returns200WithGroups(t *testing.T) {
	t.Parallel()
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: true}
	seen := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	store := &fakeObservedGroupsLister{items: []groups.ObservedGroup{
		{Name: "alpha-devs", LastSeen: seen},
		{Name: "beta-ops", LastSeen: seen.Add(-time.Hour)},
	}}
	handler := NewGroupsHandler(enforcer, store)

	req := setAdminContext(httptest.NewRequest("GET", "/api/v1/groups/observed", nil))
	w := httptest.NewRecorder()

	handler.ListObserved(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var body ObservedGroupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(body.Groups))
	}
	if body.Groups[0].Name != "alpha-devs" || body.Groups[1].Name != "beta-ops" {
		t.Errorf("unexpected groups/order: %+v", body.Groups)
	}
	if !body.Groups[0].LastSeen.Equal(seen) {
		t.Errorf("lastSeen not preserved: %v", body.Groups[0].LastSeen)
	}
}

func TestGroupsHandler_ListObserved_Unauthorized(t *testing.T) {
	t.Parallel()
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: true}
	handler := NewGroupsHandler(enforcer, &fakeObservedGroupsLister{})

	// No user context → 401.
	req := httptest.NewRequest("GET", "/api/v1/groups/observed", nil)
	w := httptest.NewRecorder()

	handler.ListObserved(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Result().StatusCode)
	}
}

func TestGroupsHandler_ListObserved_Forbidden_NonOperator(t *testing.T) {
	t.Parallel()
	// Enforcer denies settings/* get → non-operator user gets 403 (AC #4).
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: false}
	handler := NewGroupsHandler(enforcer, &fakeObservedGroupsLister{})

	req := setReadonlyContext(httptest.NewRequest("GET", "/api/v1/groups/observed", nil))
	w := httptest.NewRecorder()

	handler.ListObserved(w, req)

	if w.Result().StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Result().StatusCode)
	}
}

func TestGroupsHandler_ListObserved_NilStoreReturnsEmptyList(t *testing.T) {
	t.Parallel()
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: true}
	handler := NewGroupsHandler(enforcer, nil)

	req := setAdminContext(httptest.NewRequest("GET", "/api/v1/groups/observed", nil))
	w := httptest.NewRecorder()

	handler.ListObserved(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with nil store, got %d", resp.StatusCode)
	}
	var body ObservedGroupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Groups == nil {
		t.Error("expected non-nil empty groups slice for typeahead consumption")
	}
	if len(body.Groups) != 0 {
		t.Errorf("expected empty list, got %d", len(body.Groups))
	}
}

func TestGroupsHandler_ListObserved_StoreErrorReturns500(t *testing.T) {
	t.Parallel()
	enforcer := &mockPolicyEnforcerForMetrics{canAccessResult: true}
	store := &fakeObservedGroupsLister{err: errors.New("redis boom")}
	handler := NewGroupsHandler(enforcer, store)

	req := setAdminContext(httptest.NewRequest("GET", "/api/v1/groups/observed", nil))
	w := httptest.NewRecorder()

	handler.ListObserved(w, req)

	if w.Result().StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Result().StatusCode)
	}
}
