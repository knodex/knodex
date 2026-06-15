// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"errors"
	"net/http"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/roletemplates"
)

// RoleTemplateListResponse wraps the catalog for GET /api/v1/settings/role-templates.
type RoleTemplateListResponse struct {
	Templates []roletemplates.RoleTemplate `json:"templates"`
}

// RoleTemplatesHandler serves the operator-gated, ConfigMap-backed catalog of
// reusable PROJECT-role templates (Story 18.1).
//
// Role templates are operator configuration CRUD, NOT an authorization surface:
// every handler is gated with the SAME settings/* get|update Casbin check used
// by the Teams (10.4) and observed-groups (10.3) endpoints — no new can-i
// resource, no second enforcement layer (NFR-T1). A template is only ever
// copied into Project.spec.roles[] by the web UI; it never participates in
// Enforce(), and editing/deleting one does not touch projects already created.
type RoleTemplatesHandler struct {
	store    *roletemplates.Store
	enforcer rbac.Authorizer
}

// NewRoleTemplatesHandler creates the role-templates CRUD handler.
func NewRoleTemplatesHandler(store *roletemplates.Store, enforcer rbac.Authorizer) *RoleTemplatesHandler {
	return &RoleTemplatesHandler{store: store, enforcer: enforcer}
}

// requireOperator gates a handler with the shared settings/* Casbin check.
// action is "get" for reads, "update" for mutations. Returns the user context
// when access is granted, or nil after writing the 401/403/500 response.
func (h *RoleTemplatesHandler) requireOperator(w http.ResponseWriter, r *http.Request, action string) *middleware.UserContext {
	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return nil
	}
	if !helpers.RequireAccess(w, r.Context(), h.enforcer, userCtx, "settings/*", action, r.Header.Get("X-Request-ID")) {
		return nil
	}
	return userCtx
}

// ListRoleTemplates handles GET /api/v1/settings/role-templates.
func (h *RoleTemplatesHandler) ListRoleTemplates(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "get") == nil {
		return
	}
	templates, err := h.store.List(r.Context())
	if err != nil {
		response.InternalError(w, "failed to list role templates")
		return
	}
	response.WriteJSON(w, http.StatusOK, RoleTemplateListResponse{Templates: templates})
}

// GetRoleTemplate handles GET /api/v1/settings/role-templates/{name}.
func (h *RoleTemplatesHandler) GetRoleTemplate(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "get") == nil {
		return
	}
	name := r.PathValue("name")
	tmpl, err := h.store.Get(r.Context(), name)
	if err != nil {
		if errors.Is(err, roletemplates.ErrNotFound) {
			response.NotFound(w, "role template", name)
			return
		}
		response.InternalError(w, "failed to get role template")
		return
	}
	response.WriteJSON(w, http.StatusOK, tmpl)
}

// CreateRoleTemplate handles POST /api/v1/settings/role-templates.
func (h *RoleTemplatesHandler) CreateRoleTemplate(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "update") == nil {
		return
	}
	req, err := helpers.DecodeJSON[roletemplates.RoleTemplate](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}
	created, err := h.store.Create(r.Context(), *req)
	if err != nil {
		h.writeStoreError(w, err, req.Name)
		return
	}
	response.WriteJSON(w, http.StatusCreated, created)
}

// UpdateRoleTemplate handles PUT /api/v1/settings/role-templates/{name}.
// The path name is authoritative; the body name is ignored.
func (h *RoleTemplatesHandler) UpdateRoleTemplate(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "update") == nil {
		return
	}
	name := r.PathValue("name")
	req, err := helpers.DecodeJSON[roletemplates.RoleTemplate](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}
	updated, err := h.store.Update(r.Context(), name, *req)
	if err != nil {
		h.writeStoreError(w, err, name)
		return
	}
	response.WriteJSON(w, http.StatusOK, updated)
}

// DeleteRoleTemplate handles DELETE /api/v1/settings/role-templates/{name}.
func (h *RoleTemplatesHandler) DeleteRoleTemplate(w http.ResponseWriter, r *http.Request) {
	if h.requireOperator(w, r, "update") == nil {
		return
	}
	name := r.PathValue("name")
	if err := h.store.Delete(r.Context(), name); err != nil {
		h.writeStoreError(w, err, name)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeStoreError maps a store error to the standardized response: a
// *ValidationError → 400 with a field map; ErrNotFound → 404; ErrAlreadyExists
// → 409; everything else → 500.
func (h *RoleTemplatesHandler) writeStoreError(w http.ResponseWriter, err error, name string) {
	var verr *roletemplates.ValidationError
	switch {
	case errors.As(err, &verr):
		response.BadRequest(w, "Validation failed", map[string]string{verr.Field: verr.Message})
	case errors.Is(err, roletemplates.ErrNotFound):
		response.NotFound(w, "role template", name)
	case errors.Is(err, roletemplates.ErrAlreadyExists):
		response.Conflict(w, "role template", name)
	case errors.Is(err, roletemplates.ErrConflict):
		response.Conflict(w, "role template", "concurrent modification — please retry")
	default:
		response.InternalError(w, "failed to persist role template")
	}
}
