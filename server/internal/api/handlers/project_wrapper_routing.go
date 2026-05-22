// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/services/wrapper"
)

// toWrapperInstanceSpec converts a project request payload into the `spec` map
// of the wrapper-RGD instance. The wrapper RGD's spec.schema MUST accept the
// Project spec payload as a superset (documented authoring contract). v1 sends
// the same shape that toProjectSpec produces, but as map[string]any so the
// dynamic client can write it.
func toWrapperInstanceSpec(req *CreateProjectRequest) map[string]any {
	spec := map[string]any{
		"description": req.Description,
	}

	if len(req.Destinations) > 0 {
		dests := make([]any, 0, len(req.Destinations))
		for _, d := range req.Destinations {
			dests = append(dests, map[string]any{
				"namespace": d.Namespace,
				"name":      d.Name,
			})
		}
		spec["destinations"] = dests
	} else {
		spec["destinations"] = []any{}
	}

	if len(req.Roles) > 0 {
		roles := make([]any, 0, len(req.Roles))
		for _, r := range req.Roles {
			role := map[string]any{
				"name":        r.Name,
				"description": r.Description,
				"policies":    toAnySlice(r.Policies),
				"groups":      toAnySlice(r.Groups),
			}
			if len(r.Destinations) > 0 {
				role["destinations"] = toAnySlice(r.Destinations)
			}
			roles = append(roles, role)
		}
		spec["roles"] = roles
	}

	return spec
}

// updateRequestToInstanceSpec converts an update payload (which lacks the Name
// field) into the wrapper instance spec. Mirrors toWrapperInstanceSpec.
func updateRequestToInstanceSpec(req *UpdateProjectRequest) map[string]any {
	// Re-use the create-side helper by translating into a CreateProjectRequest
	// view. This avoids drift between the two converters.
	createView := &CreateProjectRequest{
		Description:  req.Description,
		Destinations: req.Destinations,
		Roles:        req.Roles,
	}
	return toWrapperInstanceSpec(createView)
}

func toAnySlice(in []string) []any {
	if in == nil {
		return []any{}
	}
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

// createProjectViaWrapper performs the wrapper-RGD-routed create. Returns
// handled=true if the wrapper path took ownership of the response (success or
// failure); handled=false means the caller should fall through to direct
// Project creation (currently never returned — the wrapper path either succeeds
// or fails the request with a structured 422/5xx error).
func (h *ProjectHandler) createProjectViaWrapper(
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context,
	req *CreateProjectRequest,
	userCtx *middleware.UserContext,
	rgdName, requestID string,
) (handled bool) {
	if _, _, err := h.wrapperHelpers.ResolveInstanceGVK(rgdName); err != nil {
		h.writeWrapperMisconfigured(w, r, ctx, userCtx, "create", req.Name, rgdName, err, requestID)
		return true
	}

	instanceSpec := toWrapperInstanceSpec(req)
	if _, err := h.wrapperHelpers.CreateViaWrapper(ctx, rgdName, req.Name, instanceSpec); err != nil {
		slog.Error("failed to create wrapper instance",
			"requestId", requestID,
			"userId", userCtx.UserID,
			"project", req.Name,
			"wrapperRGD", rgdName,
			"error", err,
		)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "create",
			Resource:  "projects",
			Name:      req.Name,
			Project:   req.Name,
			RequestID: requestID,
			Result:    "error",
			Details: map[string]any{
				"wrapperUsed": true,
				"wrapperRGD":  rgdName,
				"error":       err.Error(),
			},
		})
		// Translate kro's "instance already exists" into 409 so operators can
		// distinguish duplicate-name from genuine infrastructure failures.
		if k8sErrors.IsAlreadyExists(err) {
			response.WriteError(w, http.StatusConflict, "CONFLICT",
				"Project already exists: "+req.Name, map[string]string{
					"resource":   "Project",
					"identifier": req.Name,
				})
			return true
		}
		response.InternalError(w, "Failed to create project via wrapper")
		return true
	}

	slog.Info("project created via wrapper",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", req.Name,
		"wrapperRGD", rgdName,
	)

	createDetails := map[string]any{
		"description": req.Description,
		"wrapperUsed": true,
		"wrapperRGD":  rgdName,
	}
	if len(req.Destinations) > 0 {
		destNames := make([]string, len(req.Destinations))
		for i, d := range req.Destinations {
			destNames[i] = d.Namespace
		}
		createDetails["destinations"] = destNames
	}
	if len(req.Roles) > 0 {
		roleNames := make([]string, len(req.Roles))
		for i, r := range req.Roles {
			roleNames[i] = r.Name
		}
		createDetails["roles"] = roleNames
	}
	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "projects",
		Name:      req.Name,
		Project:   req.Name,
		RequestID: requestID,
		Result:    "success",
		Details:   createDetails,
	})

	// Project will be materialized by kro shortly. Return the request shape so
	// the UI can reflect what the user submitted while polling for readiness.
	resp := map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"wrapperRGD":  rgdName,
		"status":      "creating",
	}
	response.WriteJSON(w, http.StatusCreated, resp)
	return true
}

// writeWrapperMisconfigured emits a 422 WRAPPER_MISCONFIGURED response and the
// matching error-result audit event. Used by all three CRUD paths.
func (h *ProjectHandler) writeWrapperMisconfigured(
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context,
	userCtx *middleware.UserContext,
	action, projectName, rgdName string,
	resolveErr error,
	requestID string,
) {
	reason := "rgd_not_found"
	message := "Wrapper RGD \"" + rgdName + "\" registered for Kind Project not found in cluster"
	if errors.Is(resolveErr, wrapper.ErrWrapperRGDNotReady) {
		reason = "rgd_not_ready"
		message = "Wrapper RGD \"" + rgdName + "\" registered for Kind Project is not ready (missing apiVersion or kind in schema)"
	}
	slog.Warn("wrapper RGD misconfigured",
		"requestId", requestID,
		"userId", userCtx.UserID,
		"project", projectName,
		"wrapperRGD", rgdName,
		"reason", reason,
	)
	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    action,
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "error",
		Details: map[string]any{
			"wrapperUsed":   true,
			"wrapperRGD":    rgdName,
			"reason":        reason,
			"wrapperBroken": true,
		},
	})
	response.WriteError(w, http.StatusUnprocessableEntity, "WRAPPER_MISCONFIGURED", message, map[string]string{
		"kind":          wrapper.KindProject,
		"registeredRGD": rgdName,
		"reason":        reason,
	})
}

// updateProjectViaWrapper PATCHes the wrapper instance's spec on behalf of the
// user. Returns handled=true if the wrapper path took ownership of the request
// (success or 422). Returns handled=false when the registry entry is missing
// (self-heal fallback to direct Project update; caller continues).
func (h *ProjectHandler) updateProjectViaWrapper(
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context,
	projectName string,
	req *UpdateProjectRequest,
	userCtx *middleware.UserContext,
	annotations map[string]string,
	requestID string,
) (handled bool) {
	rgdName, ok := h.wrapperHelpers.LookupWrapper(wrapper.KindProject)
	if !ok {
		slog.Warn("project marked as wrapped but no registry entry; falling back to direct update",
			"requestId", requestID,
			"project", projectName,
			"marker", wrapper.OwningRGDInstance(annotations),
		)
		return false
	}

	if _, _, err := h.wrapperHelpers.ResolveInstanceGVK(rgdName); err != nil {
		h.writeWrapperMisconfigured(w, r, ctx, userCtx, "update", projectName, rgdName, err, requestID)
		return true
	}

	// IsWrapped guarantees the annotation is non-empty before this function is
	// called (see call site guard in project_handler.go).
	instanceName := wrapper.OwningRGDInstance(annotations)
	spec := updateRequestToInstanceSpec(req)
	if _, err := h.wrapperHelpers.UpdateViaWrapper(ctx, rgdName, instanceName, spec); err != nil {
		slog.Error("failed to update wrapper instance",
			"requestId", requestID,
			"project", projectName,
			"instance", instanceName,
			"wrapperRGD", rgdName,
			"error", err,
		)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "update",
			Resource:  "projects",
			Name:      projectName,
			Project:   projectName,
			RequestID: requestID,
			Result:    "error",
			Details: map[string]any{
				"wrapperUsed": true,
				"wrapperRGD":  rgdName,
				"error":       err.Error(),
			},
		})
		response.InternalError(w, "Failed to update project via wrapper")
		return true
	}

	slog.Info("project updated via wrapper",
		"requestId", requestID,
		"project", projectName,
		"wrapperRGD", rgdName,
	)
	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "update",
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"wrapperUsed": true,
			"wrapperRGD":  rgdName,
			"description": req.Description,
		},
	})
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"name":        projectName,
		"description": req.Description,
		"wrapperRGD":  rgdName,
		"status":      "updating",
	})
	return true
}

// deleteProjectViaWrapper deletes the owning wrapper-RGD instance. Returns
// handled=true if the wrapper path took ownership of the request. Returns
// handled=false when the registry entry is missing (self-heal → direct delete).
func (h *ProjectHandler) deleteProjectViaWrapper(
	w http.ResponseWriter,
	r *http.Request,
	ctx context.Context,
	projectName string,
	userCtx *middleware.UserContext,
	snapshot snapshotForDelete,
	requestID string,
	deleteDescription string,
	deleteDestsCount, deleteRolesCount int,
) (handled bool) {
	annotations := snapshot.GetAnnotations()
	rgdName, ok := h.wrapperHelpers.LookupWrapper(wrapper.KindProject)
	if !ok {
		slog.Warn("project marked as wrapped but no registry entry; falling back to direct delete",
			"requestId", requestID,
			"project", projectName,
			"marker", wrapper.OwningRGDInstance(annotations),
		)
		return false
	}

	if _, _, err := h.wrapperHelpers.ResolveInstanceGVK(rgdName); err != nil {
		h.writeWrapperMisconfigured(w, r, ctx, userCtx, "delete", projectName, rgdName, err, requestID)
		return true
	}

	// IsWrapped guarantees the annotation is non-empty before this function is
	// called (see call site guard in project_handler.go).
	instanceName := wrapper.OwningRGDInstance(annotations)

	if err := h.wrapperHelpers.DeleteViaWrapper(ctx, rgdName, instanceName); err != nil {
		slog.Error("failed to delete wrapper instance",
			"requestId", requestID,
			"project", projectName,
			"instance", instanceName,
			"wrapperRGD", rgdName,
			"error", err,
		)
		audit.RecordEvent(h.recorder, ctx, audit.Event{
			UserID:    userCtx.UserID,
			UserEmail: userCtx.Email,
			SourceIP:  audit.SourceIP(r),
			Action:    "delete",
			Resource:  "projects",
			Name:      projectName,
			Project:   projectName,
			RequestID: requestID,
			Result:    "error",
			Details: map[string]any{
				"wrapperUsed": true,
				"wrapperRGD":  rgdName,
				"error":       err.Error(),
			},
		})
		response.InternalError(w, "Failed to delete project via wrapper")
		return true
	}

	slog.Info("project deleted via wrapper",
		"requestId", requestID,
		"project", projectName,
		"wrapperRGD", rgdName,
	)
	audit.RecordEvent(h.recorder, ctx, audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "delete",
		Resource:  "projects",
		Name:      projectName,
		Project:   projectName,
		RequestID: requestID,
		Result:    "success",
		Details: map[string]any{
			"wrapperUsed":       true,
			"wrapperRGD":        rgdName,
			"description":       deleteDescription,
			"destinationsCount": deleteDestsCount,
			"rolesCount":        deleteRolesCount,
		},
	})
	response.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "deleting",
		"project": projectName,
	})
	return true
}

// snapshotForDelete is the minimal interface we need from the project snapshot
// passed into deleteProjectViaWrapper. Implemented by *rbac.Project (via
// promoted metav1.ObjectMeta).
type snapshotForDelete interface {
	GetAnnotations() map[string]string
}
