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

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/roletemplates"
	"k8s.io/client-go/kubernetes/fake"
)

// mockRTAuthorizer implements rbac.Authorizer for role-templates handler tests.
type mockRTAuthorizer struct {
	allowed bool
	err     error
}

func (m *mockRTAuthorizer) CanAccess(_ context.Context, _, _, _ string) (bool, error) {
	return m.allowed, m.err
}
func (m *mockRTAuthorizer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return m.allowed, m.err
}
func (m *mockRTAuthorizer) EnforceProjectAccess(_ context.Context, _, _, _ string) error {
	if !m.allowed {
		return errors.New("denied")
	}
	return nil
}
func (m *mockRTAuthorizer) GetAccessibleProjects(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}
func (m *mockRTAuthorizer) HasRole(_ context.Context, _, _ string) (bool, error) {
	return m.allowed, nil
}

func newRTHandler(allowed bool) *RoleTemplatesHandler {
	store := roletemplates.NewStore(fake.NewSimpleClientset(), "knodex")
	return NewRoleTemplatesHandler(store, &mockRTAuthorizer{allowed: allowed})
}

func rtRequest(t *testing.T, method, path string, body any, withUser bool) *http.Request {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("X-Request-Id", "test")
	if withUser {
		ctx := context.WithValue(req.Context(), middleware.UserContextKey,
			&middleware.UserContext{UserID: "admin@test.local", Groups: []string{"knodex-admins"}})
		req = req.WithContext(ctx)
	}
	return req
}

func TestListRoleTemplates_NoUser_401(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	h.ListRoleTemplates(rec, rtRequest(t, http.MethodGet, "/api/v1/settings/role-templates", nil, false))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListRoleTemplates_NonOperator_403(t *testing.T) {
	h := newRTHandler(false)
	rec := httptest.NewRecorder()
	h.ListRoleTemplates(rec, rtRequest(t, http.MethodGet, "/api/v1/settings/role-templates", nil, true))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestListRoleTemplates_Operator_ReturnsDefaults(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	h.ListRoleTemplates(rec, rtRequest(t, http.MethodGet, "/api/v1/settings/role-templates", nil, true))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp RoleTemplateListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Templates) != len(roletemplates.DefaultRoleTemplates()) {
		t.Fatalf("expected default templates, got %d", len(resp.Templates))
	}
}

func TestCreateRoleTemplate_Created(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	body := roletemplates.RoleTemplate{
		Name:     "operator",
		Label:    "Operator",
		Policies: []string{"p, proj:{project}:{role}, instances, *, */{project}/*, allow"},
	}
	h.CreateRoleTemplate(rec, rtRequest(t, http.MethodPost, "/api/v1/settings/role-templates", body, true))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestCreateRoleTemplate_Duplicate_409(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	body := roletemplates.RoleTemplate{
		Name:     "admin", // already a default
		Label:    "Admin",
		Policies: []string{"p, proj:{project}:{role}, projects, *, {project}, allow"},
	}
	h.CreateRoleTemplate(rec, rtRequest(t, http.MethodPost, "/api/v1/settings/role-templates", body, true))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestCreateRoleTemplate_Invalid_400(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	body := roletemplates.RoleTemplate{Name: "Bad Name", Policies: []string{"p"}}
	h.CreateRoleTemplate(rec, rtRequest(t, http.MethodPost, "/api/v1/settings/role-templates", body, true))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetRoleTemplate_NotFound_404(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	req := rtRequest(t, http.MethodGet, "/api/v1/settings/role-templates/ghost", nil, true)
	req.SetPathValue("name", "ghost")
	h.GetRoleTemplate(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateRoleTemplate_NotFound_404(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	body := roletemplates.RoleTemplate{Name: "ghost", Policies: []string{"p, x, y, z, allow"}}
	req := rtRequest(t, http.MethodPut, "/api/v1/settings/role-templates/ghost", body, true)
	req.SetPathValue("name", "ghost")
	h.UpdateRoleTemplate(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateRoleTemplate_OK(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	body := roletemplates.RoleTemplate{Label: "Dev", Description: "updated", Policies: []string{"p, proj:{project}:{role}, rgds, get, *, allow"}}
	req := rtRequest(t, http.MethodPut, "/api/v1/settings/role-templates/developer", body, true)
	req.SetPathValue("name", "developer")
	h.UpdateRoleTemplate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestDeleteRoleTemplate_NoContent(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	req := rtRequest(t, http.MethodDelete, "/api/v1/settings/role-templates/readonly", nil, true)
	req.SetPathValue("name", "readonly")
	h.DeleteRoleTemplate(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteRoleTemplate_NotFound_404(t *testing.T) {
	h := newRTHandler(true)
	rec := httptest.NewRecorder()
	req := rtRequest(t, http.MethodDelete, "/api/v1/settings/role-templates/ghost", nil, true)
	req.SetPathValue("name", "ghost")
	h.DeleteRoleTemplate(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteRoleTemplate_NonOperator_403(t *testing.T) {
	h := newRTHandler(false)
	rec := httptest.NewRecorder()
	req := rtRequest(t, http.MethodDelete, "/api/v1/settings/role-templates/admin", nil, true)
	req.SetPathValue("name", "admin")
	h.DeleteRoleTemplate(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
