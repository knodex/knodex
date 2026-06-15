// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/rbac"
)

// agentsGVR is the kagent Agent CRD group/version/resource. The CRD is a
// known quantity (agents.kagent.dev, served at kagent.dev/v1alpha2) — no
// discovery round-trip needed.
var agentsGVR = schema.GroupVersionResource{
	Group:    "kagent.dev",
	Version:  "v1alpha2",
	Resource: "agents",
}

// AccessibleNamespacesProvider is the narrow slice of
// services.AuthorizationService this handler needs: the Casbin-derived set of
// namespaces the caller may see. Kept as a one-method interface so tests can
// stub it trivially (Story 49.1 review lesson).
type AccessibleNamespacesProvider interface {
	GetAccessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error)
}

// InstalledAgent is the DTO for a single deployed kagent Agent CR.
// Names/namespaces/descriptions/timestamps only — nothing credential-shaped
// (NFR-A4). The optional Model carries the resolved {provider, name} display
// pair (Story 50.4); it is omitted when the model cannot be resolved — the
// agent stays fully listable either way.
type InstalledAgent struct {
	Name        string             `json:"name"`
	Namespace   string             `json:"namespace"`
	Description string             `json:"description"`
	CreatedAt   string             `json:"createdAt"`
	Model       *kagent.AgentModel `json:"model,omitempty"`
	// ModelConfig is the agent's bound spec.declarative.modelConfig name (empty
	// for non-declarative agents). Surfaced so the model editor pre-selects the
	// EXACT bound config by name — two configs sharing a {provider, model} pair
	// can't be told apart by the resolved Model alone.
	ModelConfig string `json:"modelConfig,omitempty"`
}

// AgentsResponse is the envelope for GET /api/v1/agents: one Casbin-scoped
// list of the caller's accessible Agent CRs. Always non-nil (default []).
// There is no hub/global bucket — every agent is governed by the single
// namespace-visibility filter (Story 53.1).
type AgentsResponse struct {
	Agents []InstalledAgent `json:"agents"`
}

// AgentsInstalledHandler serves GET /api/v1/agents. Authorization IS the
// Casbin-derived accessible-namespace set applied inside the handler — the
// exact single enforcement layer GET /api/v1/instances uses (NFR-A3). No
// can-i resource, no second destination-check layer.
type AgentsInstalledHandler struct {
	dynamicClient dynamic.Interface
	authz         AccessibleNamespacesProvider
	modelResolver kagent.ModelResolver
}

// NewAgentsInstalledHandler creates an AgentsInstalledHandler. Any dep may be
// nil: a nil authz fails closed (zero accessible namespaces → empty list), a
// nil dynamic client returns the empty envelope, and a nil modelResolver
// simply omits the model field (fail-soft).
func NewAgentsInstalledHandler(dynamicClient dynamic.Interface, authz AccessibleNamespacesProvider, modelResolver kagent.ModelResolver) *AgentsInstalledHandler {
	return &AgentsInstalledHandler{dynamicClient: dynamicClient, authz: authz, modelResolver: modelResolver}
}

// accessibleNamespaces resolves the caller's namespace patterns, failing
// closed (empty slice) when no authorization service is configured — same
// posture as InstanceCRUDHandler.getAccessibleNamespaces.
func (h *AgentsInstalledHandler) accessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if h.authz == nil {
		return []string{}, nil
	}
	return h.authz.GetAccessibleNamespaces(ctx, userCtx)
}

// filteredItem holds an admitted Agent CR with its pre-formatted timestamp, so
// the concurrent model-resolution step doesn't re-derive it.
type filteredItem struct {
	item      *unstructured.Unstructured
	createdAt string
}

// emptyAgentsResponse is the empty (but non-nil) envelope.
func emptyAgentsResponse() AgentsResponse {
	return AgentsResponse{Agents: []InstalledAgent{}}
}

// ListAgents handles GET /api/v1/agents.
// Flow: resolve the Casbin-derived namespace set → one cluster-wide LIST of
// Agent CRs → admit each agent whose namespace matches the caller's set
// (client-side wildcard match, since patterns like "dev-*" cannot be a K8s
// list selector) → concurrent best-effort model resolution per admitted agent.
// Zero accessible namespaces means an empty list (fail-closed, Story 53.1).
func (h *AgentsInstalledHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	userNamespaces, err := h.accessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		response.InternalError(w, "Failed to get user namespaces")
		return
	}

	// Fail-closed: no accessible namespaces ⇒ nothing to list (Story 53.1
	// removed the FR13 global-hub carve-out). Skip the cluster LIST entirely.
	if len(userNamespaces) == 0 {
		response.WriteJSON(w, http.StatusOK, emptyAgentsResponse())
		return
	}

	// A nil client means no cluster to query — there are simply no agents to
	// list, so return the empty envelope with 200 (consistent with the
	// CRD-absent path below). Never 500: a degraded server with no K8s client
	// must not turn every page load into an error boundary.
	if h.dynamicClient == nil {
		response.WriteJSON(w, http.StatusOK, emptyAgentsResponse())
		return
	}

	list, err := h.dynamicClient.Resource(agentsGVR).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		// CRD vanished between the 49.1 presence check and this list —
		// graceful empty list, not a 5xx.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			response.WriteJSON(w, http.StatusOK, emptyAgentsResponse())
			return
		}
		response.InternalError(w, "Failed to list agents")
		return
	}

	// Partition first — admission is cheap, model resolution is not.
	filtered := make([]filteredItem, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		if !rbac.MatchNamespaceInList(parser.GetNamespace(item), userNamespaces) {
			continue
		}
		createdAt := ""
		if created := parser.GetCreationTimestamp(item); !created.IsZero() {
			createdAt = created.UTC().Format(time.RFC3339)
		}
		filtered = append(filtered, filteredItem{item: item, createdAt: createdAt})
	}

	// Resolve models concurrently: each goroutine writes to a unique index so
	// there is no data race on the slice itself. The TTL cache in ModelResolver
	// is mutex-guarded, so concurrent cache reads/writes are safe.
	models := make([]*kagent.AgentModel, len(filtered))
	var wg sync.WaitGroup
	ctx := r.Context()
	for i := range filtered {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			models[i] = h.resolveModel(ctx, filtered[i].item)
		}(i)
	}
	wg.Wait()

	resp := emptyAgentsResponse()
	for i, fi := range filtered {
		resp.Agents = append(resp.Agents, InstalledAgent{
			Name:      parser.GetName(fi.item),
			Namespace: parser.GetNamespace(fi.item),
			// kagent Agent spec carries a description; tolerate absence.
			Description: parser.GetSpecFieldStringOrDefault(fi.item, "", "description"),
			CreatedAt:   fi.createdAt,
			Model:       models[i],
			ModelConfig: kagent.ModelConfigNameFromAgent(fi.item),
		})
	}

	sort.Slice(resp.Agents, func(i, j int) bool {
		if resp.Agents[i].Namespace != resp.Agents[j].Namespace {
			return resp.Agents[i].Namespace < resp.Agents[j].Namespace
		}
		return resp.Agents[i].Name < resp.Agents[j].Name
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// resolveModel resolves the {provider, name} for one Agent CR, best-effort.
// Namespace is derived from the agent CR itself — the caller does not supply
// it separately. Returns nil when there is no resolver, the agent has no
// spec.declarative.modelConfig, or the ModelConfig cannot be read — the model
// badge simply does not render. The lookup runs with the server's dynamic
// client; its result is only ever attached to an agent the caller is already
// authorized to see (see resolver doc comment).
func (h *AgentsInstalledHandler) resolveModel(ctx context.Context, agent *unstructured.Unstructured) *kagent.AgentModel {
	if h.modelResolver == nil {
		return nil
	}
	modelConfigName := kagent.ModelConfigNameFromAgent(agent)
	if modelConfigName == "" {
		return nil
	}
	model, ok := h.modelResolver.ResolveForAgent(ctx, parser.GetNamespace(agent), modelConfigName)
	if !ok {
		return nil
	}
	return model
}
