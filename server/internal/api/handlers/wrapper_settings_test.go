// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/services/wrapper"
)

const wrapperTestNamespace = "test-ns"

type mockWrapperAccessChecker struct {
	allowed bool
	err     error
}

func (m *mockWrapperAccessChecker) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return m.allowed, m.err
}

func newWrapperTestHandler(allowed bool) (*WrapperSettingsHandler, *wrapper.Store, *mockAuditRecorder) {
	cs := fake.NewSimpleClientset()
	store := wrapper.NewStore(cs, wrapperTestNamespace)
	checker := &mockWrapperAccessChecker{allowed: allowed}
	recorder := &mockAuditRecorder{}
	return NewWrapperSettingsHandler(store, recorder, checker), store, recorder
}

func wrapperRequest(method, path string, body any, kindPathParam string) *http.Request {
	var req *http.Request
	if body != nil {
		data, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if kindPathParam != "" {
		req.SetPathValue("kind", kindPathParam)
	}
	userCtx := &middleware.UserContext{
		UserID:      "admin-1",
		Email:       "admin@example.com",
		DisplayName: "Admin",
		CasbinRoles: []string{"role:serveradmin"},
	}
	ctx := context.WithValue(req.Context(), middleware.UserContextKey, userCtx)
	req = req.WithContext(ctx)
	req.Header.Set("X-Request-ID", "test-req-id")
	return req
}

func TestWrapperSettingsHandler_List_Empty(t *testing.T) {
	t.Parallel()
	h, _, _ := newWrapperTestHandler(true)

	req := wrapperRequest("GET", "/api/v1/settings/wrappers", nil, "")
	rec := httptest.NewRecorder()
	h.ListWrappers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp []WrapperResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(resp) != 0 {
		t.Errorf("expected empty list, got %+v", resp)
	}
}

func TestWrapperSettingsHandler_Put_Success(t *testing.T) {
	t.Parallel()
	h, store, recorder := newWrapperTestHandler(true)

	body := WrapperRequest{RGDName: "wrapped-project-v1"}
	req := wrapperRequest("PUT", "/api/v1/settings/wrappers/Project", body, "Project")
	rec := httptest.NewRecorder()
	h.PutWrapper(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	entry, err := store.Get(context.Background(), "Project")
	if err != nil {
		t.Fatalf("entry should be stored: %v", err)
	}
	if entry.RGDName != "wrapped-project-v1" {
		t.Errorf("expected rgdName=wrapped-project-v1, got %q", entry.RGDName)
	}
	if len(recorder.events) != 1 {
		t.Errorf("expected 1 audit event, got %d", len(recorder.events))
	} else {
		ev := recorder.events[0]
		if ev.Resource != "settings" || ev.Name != "wrapper:Project" || ev.Result != "success" {
			t.Errorf("audit event shape unexpected: %+v", ev)
		}
	}
}

func TestWrapperSettingsHandler_Put_RejectsUnsupportedKind(t *testing.T) {
	t.Parallel()
	h, _, _ := newWrapperTestHandler(true)
	req := wrapperRequest("PUT", "/api/v1/settings/wrappers/Secret", WrapperRequest{RGDName: "wrapped-secret"}, "Secret")
	rec := httptest.NewRecorder()
	h.PutWrapper(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWrapperSettingsHandler_Put_InvalidRGDName(t *testing.T) {
	t.Parallel()
	h, _, _ := newWrapperTestHandler(true)
	req := wrapperRequest("PUT", "/api/v1/settings/wrappers/Project", WrapperRequest{RGDName: "BadName"}, "Project")
	rec := httptest.NewRecorder()
	h.PutWrapper(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWrapperSettingsHandler_Get_NotFound(t *testing.T) {
	t.Parallel()
	h, _, _ := newWrapperTestHandler(true)
	req := wrapperRequest("GET", "/api/v1/settings/wrappers/Project", nil, "Project")
	rec := httptest.NewRecorder()
	h.GetWrapper(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWrapperSettingsHandler_Delete_Idempotent(t *testing.T) {
	t.Parallel()
	h, store, _ := newWrapperTestHandler(true)
	if err := store.Put(context.Background(), wrapper.Entry{Kind: wrapper.KindProject, RGDName: "wrapped-project-v1"}); err != nil {
		t.Fatal(err)
	}

	req := wrapperRequest("DELETE", "/api/v1/settings/wrappers/Project", nil, "Project")
	rec := httptest.NewRecorder()
	h.DeleteWrapper(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second delete should 404.
	req2 := wrapperRequest("DELETE", "/api/v1/settings/wrappers/Project", nil, "Project")
	rec2 := httptest.NewRecorder()
	h.DeleteWrapper(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("expected 404 on second delete, got %d", rec2.Code)
	}
}

func TestWrapperSettingsHandler_Forbidden_OnReadDenied(t *testing.T) {
	t.Parallel()
	h, _, recorder := newWrapperTestHandler(false)
	req := wrapperRequest("GET", "/api/v1/settings/wrappers", nil, "")
	rec := httptest.NewRecorder()
	h.ListWrappers(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	// AC10: read denials must also emit a denied audit event.
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 audit event on denied read, got %d", len(recorder.events))
	}
	ev := recorder.events[0]
	if ev.Result != "denied" || ev.Resource != "settings" {
		t.Errorf("expected denied/settings audit event, got result=%q resource=%q", ev.Result, ev.Resource)
	}
}

func TestWrapperSettingsHandler_Forbidden_OnWriteDenied(t *testing.T) {
	t.Parallel()
	h, _, recorder := newWrapperTestHandler(false)
	req := wrapperRequest("PUT", "/api/v1/settings/wrappers/Project", WrapperRequest{RGDName: "wrapped-project-v1"}, "Project")
	rec := httptest.NewRecorder()
	h.PutWrapper(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(recorder.events) != 1 || recorder.events[0].Result != "denied" {
		t.Errorf("expected one denied audit event, got %+v", recorder.events)
	}
}

func TestWrapperSettingsHandler_InternalError_OnAuthzCheckError(t *testing.T) {
	t.Parallel()
	cs := fake.NewSimpleClientset()
	store := wrapper.NewStore(cs, wrapperTestNamespace)
	checker := &mockWrapperAccessChecker{allowed: false, err: errors.New("upstream blew up")}
	recorder := &mockAuditRecorder{}
	h := NewWrapperSettingsHandler(store, recorder, checker)

	req := wrapperRequest("PUT", "/api/v1/settings/wrappers/Project", WrapperRequest{RGDName: "wrapped-project-v1"}, "Project")
	rec := httptest.NewRecorder()
	h.PutWrapper(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
