// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"log/slog"
	"net/http"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/services/wrapper"
)

// wrapperAccessChecker is the subset of rbac.PolicyEnforcer needed by WrapperSettingsHandler.
type wrapperAccessChecker interface {
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

// WrapperSettingsHandler exposes CRUD endpoints for the wrapper registry.
// Casbin-gated on the `settings/*` resource (read=get, write=update).
type WrapperSettingsHandler struct {
	store         *wrapper.Store
	recorder      audit.Recorder
	accessChecker wrapperAccessChecker
}

// NewWrapperSettingsHandler creates a new WrapperSettingsHandler.
func NewWrapperSettingsHandler(store *wrapper.Store, recorder audit.Recorder, accessChecker wrapperAccessChecker) *WrapperSettingsHandler {
	return &WrapperSettingsHandler{
		store:         store,
		recorder:      recorder,
		accessChecker: accessChecker,
	}
}

// WrapperResponse is the JSON representation of a wrapper registry entry.
type WrapperResponse struct {
	Kind    string `json:"kind"`
	RGDName string `json:"rgdName"`
}

// WrapperRequest is the JSON body for upserts.
type WrapperRequest struct {
	RGDName string `json:"rgdName"`
}

// requireSettingsUpdate returns true if the user has settings:update permission.
// Mirrors sso_settings.requireSettingsUpdate verbatim aside from the audit "Name" prefix.
func (h *WrapperSettingsHandler) requireSettingsUpdate(w http.ResponseWriter, r *http.Request, userCtx *middleware.UserContext, auditAction, auditName string) bool {
	requestID := r.Header.Get("X-Request-ID")

	if h.accessChecker == nil {
		slog.Warn("wrapper settings: policy enforcer unavailable, denying write operation",
			"userId", userCtx.UserID,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"reason": "policy enforcer unavailable", "settingsType": "wrapper"},
		})
		response.Forbidden(w, "permission denied")
		return false
	}

	allowed, err := h.accessChecker.CanAccessWithGroups(
		r.Context(), userCtx.UserID, userCtx.Groups, "settings/*", "update",
	)
	if err != nil {
		slog.Error("failed to check wrapper settings permission",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "error",
			Details:   map[string]any{"reason": "policy check failed", "settingsType": "wrapper"},
		})
		response.InternalError(w, "Failed to check authorization")
		return false
	}
	if !allowed {
		slog.Warn("wrapper settings write denied",
			"requestId", requestID,
			"userId", userCtx.UserID,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"reason": "insufficient permissions", "settingsType": "wrapper"},
		})
		response.Forbidden(w, "permission denied")
		return false
	}
	return true
}

// requireSettingsRead returns true if the user has settings:get permission.
// Mirrors requireSettingsUpdate: emits a denied/error audit event before
// returning false so that all wrapper-settings access attempts (read or write)
// are traceable (AC10).
func (h *WrapperSettingsHandler) requireSettingsRead(w http.ResponseWriter, r *http.Request, userCtx *middleware.UserContext, auditAction, auditName string) bool {
	requestID := r.Header.Get("X-Request-ID")

	if h.accessChecker == nil {
		slog.Warn("wrapper settings: policy enforcer unavailable, denying read operation",
			"userId", userCtx.UserID,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"reason": "policy enforcer unavailable", "settingsType": "wrapper"},
		})
		response.Forbidden(w, "permission denied")
		return false
	}
	allowed, err := h.accessChecker.CanAccessWithGroups(
		r.Context(), userCtx.UserID, userCtx.Groups, "settings/*", "get",
	)
	if err != nil {
		slog.Error("failed to check wrapper settings read permission",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"error", err,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "error",
			Details:   map[string]any{"reason": "policy check failed", "settingsType": "wrapper"},
		})
		response.InternalError(w, "Failed to check authorization")
		return false
	}
	if !allowed {
		slog.Warn("wrapper settings read denied",
			"requestId", requestID,
			"userId", userCtx.UserID,
		)
		audit.RecordEvent(h.recorder, r.Context(), audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    auditAction,
			Resource:  "settings",
			Name:      auditName,
			RequestID: requestID,
			Result:    "denied",
			Details:   map[string]any{"reason": "insufficient permissions", "settingsType": "wrapper"},
		})
		response.Forbidden(w, "permission denied")
		return false
	}
	return true
}

// ListWrappers handles GET /api/v1/settings/wrappers.
func (h *WrapperSettingsHandler) ListWrappers(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}
	if !h.requireSettingsRead(w, r, userCtx, "list", "wrapper:*") {
		return
	}

	entries, err := h.store.List(ctx)
	if err != nil {
		slog.Error("failed to list wrapper entries",
			"requestId", requestID, "userId", userCtx.UserID, "error", err,
		)
		response.InternalError(w, "Failed to list wrappers")
		return
	}

	resp := make([]WrapperResponse, len(entries))
	for i, e := range entries {
		resp[i] = WrapperResponse{Kind: e.Kind, RGDName: e.RGDName}
	}
	response.WriteJSON(w, http.StatusOK, resp)
}

// GetWrapper handles GET /api/v1/settings/wrappers/{kind}.
func (h *WrapperSettingsHandler) GetWrapper(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()
	kind := r.PathValue("kind")

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}
	if !h.requireSettingsRead(w, r, userCtx, "get", "wrapper:"+kind) {
		return
	}

	entry, err := h.store.Get(ctx, kind)
	if err != nil {
		if wrapper.IsNotFound(err) {
			response.NotFound(w, "wrapper", kind)
			return
		}
		slog.Error("failed to get wrapper entry",
			"requestId", requestID, "userId", userCtx.UserID, "kind", kind, "error", err,
		)
		response.InternalError(w, "Failed to get wrapper")
		return
	}
	response.WriteJSON(w, http.StatusOK, WrapperResponse{Kind: entry.Kind, RGDName: entry.RGDName})
}

// PutWrapper handles PUT /api/v1/settings/wrappers/{kind} (upsert by Kind).
func (h *WrapperSettingsHandler) PutWrapper(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()
	kind := r.PathValue("kind")

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	auditName := "wrapper:" + kind
	if !h.requireSettingsUpdate(w, r, userCtx, "update", auditName) {
		return
	}

	if err := wrapper.ValidateKind(kind); err != nil {
		response.BadRequest(w, err.Error(), map[string]string{"kind": kind})
		return
	}

	req, err := helpers.DecodeJSON[WrapperRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}
	if err := wrapper.ValidateRGDName(req.RGDName); err != nil {
		response.BadRequest(w, err.Error(), map[string]string{"rgdName": req.RGDName})
		return
	}

	if err := h.store.Put(ctx, wrapper.Entry{Kind: kind, RGDName: req.RGDName}); err != nil {
		if k8serrors.IsConflict(err) {
			response.WriteError(w, http.StatusConflict, "CONFLICT",
				"Wrapper registry was modified concurrently; please retry", nil)
			return
		}
		slog.Error("failed to upsert wrapper entry",
			"requestId", requestID, "userId", userCtx.UserID, "kind", kind, "error", err,
		)
		response.InternalError(w, "Failed to save wrapper")
		return
	}

	slog.Info("wrapper entry upserted",
		"requestId", requestID, "userId", userCtx.UserID, "kind", kind, "rgdName", req.RGDName,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "settings",
		Name:      auditName,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"settingsType": "wrapper", "kind": kind, "rgdName": req.RGDName},
	})

	response.WriteJSON(w, http.StatusOK, WrapperResponse{Kind: kind, RGDName: req.RGDName})
}

// DeleteWrapper handles DELETE /api/v1/settings/wrappers/{kind}.
func (h *WrapperSettingsHandler) DeleteWrapper(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	ctx := r.Context()
	kind := r.PathValue("kind")

	userCtx := helpers.RequireUserContext(w, r)
	if userCtx == nil {
		return
	}

	auditName := "wrapper:" + kind
	if !h.requireSettingsUpdate(w, r, userCtx, "delete", auditName) {
		return
	}

	if err := h.store.Delete(ctx, kind); err != nil {
		if wrapper.IsNotFound(err) {
			response.NotFound(w, "wrapper", kind)
			return
		}
		slog.Error("failed to delete wrapper entry",
			"requestId", requestID, "userId", userCtx.UserID, "kind", kind, "error", err,
		)
		response.InternalError(w, "Failed to delete wrapper")
		return
	}

	slog.Info("wrapper entry deleted",
		"requestId", requestID, "userId", userCtx.UserID, "kind", kind,
	)

	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "settings",
		Name:      auditName,
		RequestID: requestID,
		Result:    "success",
		Details:   map[string]any{"settingsType": "wrapper", "kind": kind},
	})

	w.WriteHeader(http.StatusNoContent)
}
