// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kro/diff"
	"github.com/knodex/knodex/server/internal/models"
)

// revisionMockAuthorizer implements rbac.Authorizer for testing.
type revisionMockAuthorizer struct {
	allowed bool
}

func (m *revisionMockAuthorizer) CanAccess(_ context.Context, _, _, _ string) (bool, error) {
	return m.allowed, nil
}

func (m *revisionMockAuthorizer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return m.allowed, nil
}

func (m *revisionMockAuthorizer) EnforceProjectAccess(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *revisionMockAuthorizer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}

func (m *revisionMockAuthorizer) HasRole(_ context.Context, _, _ string) (bool, error) {
	return m.allowed, nil
}

func newTestRevisionHandler(provider *mockGraphRevisionProvider, allowed bool) *RevisionHandler {
	return NewRevisionHandler(RevisionHandlerConfig{
		Provider:       provider,
		PolicyEnforcer: &revisionMockAuthorizer{allowed: allowed},
	})
}

func newTestRevisionHandlerWithDiff(provider *mockGraphRevisionProvider, allowed bool, diffSvc *diff.DiffService) *RevisionHandler {
	return NewRevisionHandler(RevisionHandlerConfig{
		Provider:       provider,
		PolicyEnforcer: &revisionMockAuthorizer{allowed: allowed},
		DiffService:    diffSvc,
	})
}

func revisionTestUserContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserContextKey, &middleware.UserContext{
		UserID:      "test@example.com",
		Email:       "test@example.com",
		CasbinRoles: []string{"role:serveradmin"},
	})
	return r.WithContext(ctx)
}

func TestRevisionHandler_ListRevisions_NilProvider(t *testing.T) {
	h := NewRevisionHandler(RevisionHandlerConfig{Provider: nil})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions", nil)
	r.SetPathValue("name", "my-rgd")

	h.ListRevisions(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestRevisionHandler_ListRevisions_Success(t *testing.T) {
	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{
			"my-rgd": {
				{RevisionNumber: 2, RGDName: "my-rgd"},
				{RevisionNumber: 1, RGDName: "my-rgd"},
			},
		},
	}

	h := newTestRevisionHandler(provider, true)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions", nil)
	r.SetPathValue("name", "my-rgd")
	r = revisionTestUserContext(r)

	h.ListRevisions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var list models.GraphRevisionList
	json.NewDecoder(w.Body).Decode(&list)
	if list.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", list.TotalCount)
	}
}

func TestRevisionHandler_ListRevisions_Forbidden(t *testing.T) {
	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{},
	}

	h := newTestRevisionHandler(provider, false)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions", nil)
	r.SetPathValue("name", "my-rgd")
	r = revisionTestUserContext(r)

	h.ListRevisions(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRevisionHandler_GetRevision_Success(t *testing.T) {
	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{
			"my-rgd": {
				{RevisionNumber: 3, RGDName: "my-rgd", ContentHash: "abc"},
			},
		},
	}

	h := newTestRevisionHandler(provider, true)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/3", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("revision", "3")
	r = revisionTestUserContext(r)

	h.GetRevision(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var rev models.GraphRevision
	json.NewDecoder(w.Body).Decode(&rev)
	if rev.RevisionNumber != 3 {
		t.Errorf("RevisionNumber = %d, want 3", rev.RevisionNumber)
	}
}

func TestRevisionHandler_GetRevision_NotFound(t *testing.T) {
	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{},
	}

	h := newTestRevisionHandler(provider, true)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/99", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("revision", "99")
	r = revisionTestUserContext(r)

	h.GetRevision(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestRevisionHandler_GetRevision_InvalidRevisionNumber(t *testing.T) {
	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{},
	}

	h := newTestRevisionHandler(provider, true)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/abc", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("revision", "abc")

	h.GetRevision(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRevisionHandler_GetRevision_MissingName(t *testing.T) {
	provider := &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{},
	}

	h := newTestRevisionHandler(provider, true)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds//revisions/1", nil)
	r.SetPathValue("name", "")
	r.SetPathValue("revision", "1")

	h.GetRevision(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func newDiffProvider() *mockGraphRevisionProvider {
	return &mockGraphRevisionProvider{
		revisions: map[string][]models.GraphRevision{
			"my-rgd": {
				{RevisionNumber: 1, RGDName: "my-rgd", Snapshot: map[string]interface{}{"apiVersion": "v1"}},
				{RevisionNumber: 2, RGDName: "my-rgd", Snapshot: map[string]interface{}{"apiVersion": "v1", "kind": "RGD"}},
			},
		},
	}
}

func newTestDiffService(t *testing.T) *diff.DiffService {
	t.Helper()
	svc, err := diff.NewDiffService()
	if err != nil {
		t.Fatalf("failed to create diff service: %v", err)
	}
	return svc
}

func TestRevisionHandler_DiffRevisions_NilDiffService(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), true, nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/1/diff/2", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "2")

	h.DiffRevisions(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestRevisionHandler_DiffRevisions_NilProvider(t *testing.T) {
	h := NewRevisionHandler(RevisionHandlerConfig{
		Provider:    nil,
		DiffService: newTestDiffService(t),
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/1/diff/2", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "2")

	h.DiffRevisions(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestRevisionHandler_DiffRevisions_Success(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), true, newTestDiffService(t))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/1/diff/2", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "2")
	r = revisionTestUserContext(r)

	h.DiffRevisions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var d models.RevisionDiff
	if err := json.NewDecoder(w.Body).Decode(&d); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if d.RGDName != "my-rgd" {
		t.Errorf("RGDName = %q, want %q", d.RGDName, "my-rgd")
	}
	if d.Rev1 != 1 || d.Rev2 != 2 {
		t.Errorf("Rev1=%d Rev2=%d, want 1 and 2", d.Rev1, d.Rev2)
	}
	if len(d.Added) == 0 {
		t.Error("expected added fields (rev2 has 'kind' not in rev1)")
	}
}

func TestRevisionHandler_DiffRevisions_InvalidRev1(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), true, newTestDiffService(t))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/abc/diff/2", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "abc")
	r.SetPathValue("rev2", "2")
	r = revisionTestUserContext(r)

	h.DiffRevisions(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRevisionHandler_DiffRevisions_InvalidRev2(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), true, newTestDiffService(t))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/1/diff/xyz", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "xyz")
	r = revisionTestUserContext(r)

	h.DiffRevisions(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRevisionHandler_DiffRevisions_Forbidden(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), false, newTestDiffService(t))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/1/diff/2", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "2")
	r = revisionTestUserContext(r)

	h.DiffRevisions(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRevisionHandler_DiffRevisions_InvalidDNSName(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), true, newTestDiffService(t))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/INVALID_NAME!/revisions/1/diff/2", nil)
	r.SetPathValue("name", "INVALID_NAME!")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "2")
	r = revisionTestUserContext(r)

	h.DiffRevisions(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRevisionHandler_DiffRevisions_RevisionNotFound(t *testing.T) {
	h := newTestRevisionHandlerWithDiff(newDiffProvider(), true, newTestDiffService(t))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/rgds/my-rgd/revisions/1/diff/99", nil)
	r.SetPathValue("name", "my-rgd")
	r.SetPathValue("rev1", "1")
	r.SetPathValue("rev2", "99")
	r = revisionTestUserContext(r)

	h.DiffRevisions(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
