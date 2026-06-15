// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"encoding/json"
	"net/http"
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// ModelConfigSummary is the frontend-safe view of a kagent ModelConfig CR:
// its name plus the resolved {provider, model} display pair. The
// apiKeySecretRef and any provider endpoint are NEVER copied here (NFR-A4) —
// the model-edit dropdown only ever needs a name to select and a label to
// show.
type ModelConfigSummary struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ModelConfigsResponse is the envelope for GET
// /api/v1/agents/{namespace}/{name}/modelconfigs. Always non-nil (default []).
type ModelConfigsResponse struct {
	ModelConfigs []ModelConfigSummary `json:"modelConfigs"`
}

// editModelRequest is the PATCH body — only the ModelConfig reference is
// editable. systemMessage, tools, and type are curated agent behavior and are
// never accepted here.
type editModelRequest struct {
	ModelConfig string `json:"modelConfig"`
}

// AgentModelEditHandler serves the model-edit pair: list the ModelConfigs in an
// agent's namespace, and patch ONLY spec.declarative.modelConfig on the Agent
// CR. systemMessage/tools/type are never touched — a merge patch on the single
// modelConfig field leaves all sibling declarative fields intact.
//
// Authorization is the SAME single Casbin accessible-namespace check as the
// agents list / BYOA invoke, resolved from the live Agent CR (one GET): a
// denied namespace and a missing agent both yield 404 — existence non-leak
// (Story 53.1 removed the hub serveradmin branch).
type AgentModelEditHandler struct {
	dynamicClient dynamic.Interface
	authz         AccessibleNamespacesProvider
}

// NewAgentModelEditHandler creates an AgentModelEditHandler. A nil authz fails
// closed (zero accessible namespaces → installed agents 404); a nil dynamic
// client makes every request a 500 (no cluster to read or patch).
func NewAgentModelEditHandler(dynamicClient dynamic.Interface, authz AccessibleNamespacesProvider) *AgentModelEditHandler {
	return &AgentModelEditHandler{dynamicClient: dynamicClient, authz: authz}
}

// validateAgentPath rejects a namespace/name that cannot name a real Agent CR
// before it ever reaches the cluster API — a clean 400 instead of an opaque
// client error, the same fail-fast posture EditModel applies to modelConfig.
// Kubernetes namespaces are DNS-1123 labels; Agent CR names are DNS-1123
// subdomains. Returns false and writes the 400 on failure.
func validateAgentPath(w http.ResponseWriter, namespace, name string) bool {
	if !sanitize.IsValidDNS1123Label(namespace) {
		response.BadRequest(w, "invalid namespace", map[string]string{"field": "namespace"})
		return false
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		response.BadRequest(w, "invalid agent name", map[string]string{"field": "name"})
		return false
	}
	return true
}

// authorizeAgent fetches the Agent CR at namespace/name and applies the
// bucket-aware gate. On any non-ok return it has ALREADY written the response
// (404/403/500); callers just return. The fetched CR is handed back so the
// caller can inspect it (EditModel uses it for the declarative guard) without a
// second GET.
func (h *AgentModelEditHandler) authorizeAgent(w http.ResponseWriter, r *http.Request, userCtx *middleware.UserContext, namespace, name string) (*unstructured.Unstructured, bool) {
	if h.dynamicClient == nil {
		response.InternalError(w, "kubernetes client unavailable")
		return nil, false
	}

	agent, err := h.dynamicClient.Resource(agentsGVR).Namespace(namespace).Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		// Missing CR or absent CRD: 404 with the same shape a denied installed
		// namespace produces below — an unauthorized caller cannot tell an
		// existing private agent from a missing one.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			response.NotFound(w, "agent", namespace+"/"+name)
			return nil, false
		}
		response.InternalError(w, "Failed to get agent")
		return nil, false
	}

	// Single Casbin enforcement layer, fail-closed, 404 non-leak — identical
	// to AgentsInvokeHandler.
	userNamespaces := []string{}
	if h.authz != nil {
		ns, err := h.authz.GetAccessibleNamespaces(r.Context(), userCtx)
		if err != nil {
			response.InternalError(w, "Failed to get user namespaces")
			return nil, false
		}
		userNamespaces = ns
	}
	if !rbac.MatchNamespaceInList(namespace, userNamespaces) {
		response.NotFound(w, "agent", namespace+"/"+name)
		return nil, false
	}
	return agent, true
}

// ListModelConfigs handles GET /api/v1/agents/{namespace}/{name}/modelconfigs.
// The {name} segment scopes authorization to a specific agent (so the same
// hub/installed gate as EditModel applies); the ModelConfigs themselves are
// listed across the whole {namespace} — they are the pool the agent can be
// repointed at.
func (h *AgentModelEditHandler) ListModelConfigs(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	namespace := sanitize.RemoveControlChars(r.PathValue("namespace"))
	name := sanitize.RemoveControlChars(r.PathValue("name"))
	if !validateAgentPath(w, namespace, name) {
		return
	}

	if _, ok := h.authorizeAgent(w, r, userCtx, namespace, name); !ok {
		return
	}

	list, err := h.dynamicClient.Resource(kagent.ModelConfigGVR).Namespace(namespace).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		// CRD absent (kagent half-installed) is an empty pool, not a 5xx —
		// mirrors the installed-list graceful-empty posture.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			response.WriteJSON(w, http.StatusOK, ModelConfigsResponse{ModelConfigs: []ModelConfigSummary{}})
			return
		}
		response.InternalError(w, "Failed to list model configs")
		return
	}

	out := make([]ModelConfigSummary, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		// ok=false (both empty) still lists the config — the dropdown needs the
		// name to select even when there's no label to show.
		model, _ := kagent.ModelFromConfigCR(item)
		out = append(out, ModelConfigSummary{
			Name:     parser.GetName(item),
			Provider: model.Provider,
			Model:    model.Name,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	response.WriteJSON(w, http.StatusOK, ModelConfigsResponse{ModelConfigs: out})
}

// EditModel handles PATCH /api/v1/agents/{namespace}/{name}/model. It patches
// ONLY spec.declarative.modelConfig (a JSON merge patch on that one field
// leaves systemMessage/tools/type untouched) after verifying the target
// ModelConfig exists — so the UI gets a clean 404 instead of silently breaking
// the agent with a dangling reference.
func (h *AgentModelEditHandler) EditModel(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	namespace := sanitize.RemoveControlChars(r.PathValue("namespace"))
	name := sanitize.RemoveControlChars(r.PathValue("name"))
	if !validateAgentPath(w, namespace, name) {
		return
	}

	var req editModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid JSON body", nil)
		return
	}
	modelConfig := sanitize.RemoveControlChars(req.ModelConfig)
	if modelConfig == "" {
		response.BadRequest(w, "modelConfig is required", map[string]string{"field": "modelConfig"})
		return
	}
	// Same name rules as agent/ModelConfig CR names — a malformed value can
	// never reach the cluster as a patch.
	if !sanitize.IsValidDNS1123Subdomain(modelConfig) {
		response.BadRequest(w, "modelConfig must be a valid DNS-1123 subdomain", map[string]string{"field": "modelConfig"})
		return
	}

	agent, ok := h.authorizeAgent(w, r, userCtx, namespace, name)
	if !ok {
		return
	}

	// Guard: only declarative agents have a spec.declarative.modelConfig to
	// repoint. Patching the modelConfig onto a non-declarative agent would
	// FABRICATE a spec.declarative block (carrying only modelConfig, no
	// systemMessage) — a malformed CR. Require the block to already exist;
	// the merge patch below then only overwrites the single leaf.
	if _, hasDeclarative := parser.GetSpecOrEmpty(agent)["declarative"]; !hasDeclarative {
		response.BadRequest(w, "agent is not declarative; its model cannot be changed", nil)
		return
	}

	// Verify the target ModelConfig exists BEFORE patching: a patch that points
	// the agent at a missing ModelConfig would break it silently. The GET also
	// yields the {provider, model} we echo back so the UI updates its badge
	// without a re-resolve.
	mc, err := h.dynamicClient.Resource(kagent.ModelConfigGVR).Namespace(namespace).Get(r.Context(), modelConfig, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			response.NotFound(w, "modelconfig", namespace+"/"+modelConfig)
			return
		}
		response.InternalError(w, "Failed to get model config")
		return
	}

	// Merge patch on the single field — declarative siblings (systemMessage,
	// tools, type) are preserved. A strategic-merge patch is not available for
	// CRDs, but a JSON merge patch on a scalar leaf is exactly an overwrite of
	// that leaf. Built via json.Marshal (never string concatenation) so the
	// value is structurally escaped.
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"declarative": map[string]any{"modelConfig": modelConfig},
		},
	})
	if err != nil {
		response.InternalError(w, "Failed to build patch")
		return
	}
	if _, err := h.dynamicClient.Resource(agentsGVR).Namespace(namespace).Patch(r.Context(), name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			// Raced with a delete between authorize and patch.
			response.NotFound(w, "agent", namespace+"/"+name)
			return
		}
		response.InternalError(w, "Failed to update agent model")
		return
	}

	// Echo the new model identity (provider/model only — never the secret ref),
	// via the SAME extractor the resolver/list use so the response can't disagree
	// with the badge the next /agents/installed load renders.
	model, _ := kagent.ModelFromConfigCR(mc)
	response.WriteJSON(w, http.StatusOK, model)
}
