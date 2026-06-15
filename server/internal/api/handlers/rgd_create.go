// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/knodex/knodex/server/internal/api/helpers"
	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/kro"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// rgdCreateTimeout bounds the cluster create call (mirrors the
// instanceCreateTimeout posture; RGD creation has no webhook fan-out so a
// tighter bound suffices).
const rgdCreateTimeout = 30 * time.Second

// maxActionTakenChars caps the run record's actionTaken value.
const maxActionTakenChars = 256

// RGDCreateHandler serves POST /api/v1/rgds (Story 50.2 AC #5/#6): create a
// ResourceGraphDefinition in the cluster from a user-reviewed YAML payload —
// the RGD Builder's "Use this spec" deploy hand-off.
//
// SECURITY: the payload is user-editable text and the handler creates objects
// as the server's service account. The kind lock (kro.run/v1alpha1
// ResourceGraphDefinition ONLY) is the boundary that keeps this endpoint from
// becoming a generic manifest-apply — everything else is rejected with 400
// before the cluster is touched. Authorization is the existing CasbinAuthz
// inference (rgds/*, create — serveradmin-only by default); the handler adds
// NO second authorization layer.
type RGDCreateHandler struct {
	dynamicClient dynamic.Interface
	// runStore stamps the originating builder run's reserved actionTaken
	// field (best-effort). Nil-safe.
	runStore runs.Store
	// recorder is the audit recorder; nil in OSS builds (audit.RecordEvent is
	// nil-safe).
	recorder audit.Recorder
	logger   *slog.Logger
}

// NewRGDCreateHandler creates an RGDCreateHandler. runStore and recorder may
// be nil (actionTaken stamping / audit become no-ops).
func NewRGDCreateHandler(dynamicClient dynamic.Interface, runStore runs.Store, recorder audit.Recorder, logger *slog.Logger) *RGDCreateHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &RGDCreateHandler{
		dynamicClient: dynamicClient,
		runStore:      runStore,
		recorder:      recorder,
		logger:        logger,
	}
}

// rgdCreateRequest is the POST /api/v1/rgds body. Name overrides the YAML's
// metadata.name when set; the YAML is the user-edited spec text.
type rgdCreateRequest struct {
	Name string `json:"name"`
	YAML string `json:"yaml"`
	// RunID links the create back to the originating RGD Builder run for the
	// best-effort actionTaken stamp (AC #6).
	RunID string `json:"runId"`
}

// rgdCreateResponse is the 201 payload.
type rgdCreateResponse struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

// Create handles POST /api/v1/rgds.
func (h *RGDCreateHandler) Create(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	req, err := helpers.DecodeJSON[rgdCreateRequest](r, w, 0)
	if err != nil {
		response.BadRequest(w, err.Error(), nil)
		return
	}
	if req.YAML == "" {
		response.BadRequest(w, "yaml is required", map[string]string{"field": "yaml"})
		return
	}

	if h.dynamicClient == nil {
		response.ServiceUnavailable(w, "kubernetes client unavailable")
		return
	}

	// Parse the YAML into an unstructured object. sigs.k8s.io/yaml goes
	// through JSON so the resulting map holds JSON-compatible types for the
	// dynamic client.
	var raw map[string]interface{}
	if err := sigsyaml.Unmarshal([]byte(req.YAML), &raw); err != nil {
		response.BadRequest(w, "invalid YAML payload", map[string]string{"field": "yaml"})
		return
	}
	if raw == nil {
		response.BadRequest(w, "yaml payload is empty", map[string]string{"field": "yaml"})
		return
	}
	obj := &unstructured.Unstructured{Object: raw}

	// THE KIND LOCK: only kro.run/v1alpha1 ResourceGraphDefinition passes.
	wantAPIVersion := kro.RGDGroup + "/" + kro.RGDVersion
	if parser.GetAPIVersion(obj) != wantAPIVersion || parser.GetKind(obj) != kro.RGDKind {
		response.BadRequest(w,
			"payload must be a "+wantAPIVersion+" "+kro.RGDKind,
			map[string]string{"field": "yaml"})
		return
	}

	// Name resolution: request override → YAML metadata.
	// RGDs are cluster-scoped — no namespace is set or validated.
	name := req.Name
	if name == "" {
		name = parser.GetName(obj)
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		response.BadRequest(w, "invalid ResourceGraphDefinition name", map[string]string{"field": "name"})
		return
	}
	obj.SetName(name)
	obj.SetNamespace("")

	// status is server-owned — never accept it from the payload.
	unstructured.RemoveNestedField(obj.Object, "status")

	// Catalog gateway (deterministic server guarantee): without
	// knodex.io/catalog: "true" the created RGD never surfaces in the Catalog
	// and the builder → deploy journey dead-ends. Ensure-if-missing only —
	// caller-provided annotations are preserved.
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	if _, exists := annotations[kro.CatalogAnnotation]; !exists {
		annotations[kro.CatalogAnnotation] = "true"
		obj.SetAnnotations(annotations)
	}

	ctx, cancel := context.WithTimeout(r.Context(), rgdCreateTimeout)
	defer cancel()
	// RGDs are cluster-scoped: no .Namespace() call.
	created, err := h.dynamicClient.Resource(kro.RGDGVR()).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		h.writeCreateError(w, err, name)
		return
	}

	// NEVER the YAML body in Details (the no-Spec rule): annotation
	// values inside the spec contain user prompt text.
	details := map[string]any{"generatedBy": "rgd-builder"}
	if req.RunID != "" {
		details["runId"] = req.RunID
	}
	audit.RecordEvent(h.recorder, r.Context(), audit.Event{
		UserID:    userCtx.UserID,
		UserEmail: userCtx.Email,
		SourceIP:  audit.SourceIP(r),
		Action:    "create",
		Resource:  "rgds",
		Name:      name,
		Group:     kro.RGDGroup,
		RequestID: r.Header.Get("X-Request-ID"),
		Result:    "success",
		Details:   details,
	})

	h.recordActionTaken(r.Context(), req.RunID, userCtx, name)

	response.WriteJSON(w, http.StatusCreated, rgdCreateResponse{
		Name:       parser.GetName(created),
		Kind:       kro.RGDKind,
		APIVersion: wantAPIVersion,
	})
}

// writeCreateError maps cluster create failures to API responses without
// echoing internal detail (transport/URL redaction discipline).
func (h *RGDCreateHandler) writeCreateError(w http.ResponseWriter, err error, name string) {
	switch {
	case apierrors.IsAlreadyExists(err):
		response.Conflict(w, "ResourceGraphDefinition", name)
	case apierrors.IsInvalid(err) || apierrors.IsBadRequest(err):
		response.BadRequest(w,
			"the cluster rejected the ResourceGraphDefinition: "+sanitize.RemoveControlChars(err.Error()),
			nil)
	default:
		h.logger.Error("failed to create ResourceGraphDefinition", "name", name, "error", err)
		response.InternalError(w, "failed to create ResourceGraphDefinition")
	}
}

// recordActionTaken best-effort stamps the originating run's reserved
// actionTaken field ("rgd-created: {name}", AC #6). Only when the run exists,
// its actor matches the caller, and actionTaken is still empty.
// Every failure logs and continues — it must NEVER fail the create. No WS
// broadcast: the runs list picks the change up on its next fetch.
//
// runs.Store has no Get (the interface is frozen — the 49.5 EE audit
// decorator's compile-time assertion breaks on changes), so the run is
// located via List.
func (h *RGDCreateHandler) recordActionTaken(ctx context.Context, runID string, userCtx *middleware.UserContext, name string) {
	if h.runStore == nil || runID == "" {
		return
	}
	all, err := h.runStore.List(ctx, runs.Filter{})
	if err != nil {
		h.logger.Warn("actionTaken: failed to list agent runs", "runId", runID, "error", err)
		return
	}
	var run *runs.Run
	for i := range all {
		if all[i].ID == runID {
			run = &all[i]
			break
		}
	}
	if run == nil {
		return // unknown run — best-effort, nothing to stamp
	}
	if run.Actor != userCtx.Email && run.Actor != userCtx.UserID {
		return // actor mismatch — never stamp someone else's run
	}
	if run.ActionTaken != "" {
		return // fill-only-if-empty
	}
	run.ActionTaken = truncateRunes("rgd-created: "+name, maxActionTakenChars)
	if err := h.runStore.Update(ctx, run); err != nil {
		h.logger.Warn("actionTaken: failed to update agent run", "runId", runID, "error", err)
	}
}
