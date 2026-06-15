// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/models"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/sanitize"
)

// agentModelConfigGVR is the KRO-generated instance CRD from
// deploy/charts/knodex/files/agents/model-config.yaml (schema.apiVersion
// agents.knodex.io/v1alpha1, kind KnodexAgentModelConfig). Hardcoded — it is a
// known asset shipped with the chart, so no discovery round-trip. CreateModel
// creates this instance DIRECTLY (not via InstanceDeploymentHandler) so the
// Models page works regardless of catalog ingestion state; KRO reconciles the
// instance into a kagent ModelConfig just as `kubectl apply` would.
var agentModelConfigGVR = schema.GroupVersionResource{
	Group:    "agents.knodex.io",
	Version:  "v1alpha1",
	Resource: "knodexagentmodelconfigs",
}

// secretsCreateChecker is the narrow slice of rbac.PolicyEnforcer CreateModel
// needs: an explicit Casbin enforce for the credential-minting (secrets create)
// gate. One method so tests stub it trivially.
type secretsCreateChecker interface {
	CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error)
}

// ModelSummary is the frontend-safe view of a kagent ModelConfig CR: its
// identity + the resolved {provider, model} display pair. The apiKeySecretRef
// and any secret value are NEVER copied here (NFR-A4) — the Models tab only
// needs to show what a model is, never how it authenticates.
type ModelSummary struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
}

// ModelsResponse is the envelope for GET /api/v1/agents/models. Always non-nil
// (default []).
type ModelsResponse struct {
	Models []ModelSummary `json:"models"`
}

// createModelRequest is the POST body. apiKey is write-only — it is consumed to
// mint the Secret and NEVER echoed back.
type createModelRequest struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Namespace string `json:"namespace"`
	APIKey    string `json:"apiKey"`
}

// AgentModelsHandler serves the Models tab pair: list the caller's accessible
// ModelConfigs, and create one by orchestrating a Secret + a
// KnodexAgentModelConfig instance behind a single POST. Authorization is the SAME single Casbin
// accessible-namespace layer as the agents list (NFR-A3) PLUS, for the write,
// an explicit secrets-create enforce — see authorizeCreate.
type AgentModelsHandler struct {
	dynamicClient dynamic.Interface
	authz         AccessibleNamespacesProvider
	k8sClient     kubernetes.Interface
	enforcer      secretsCreateChecker
}

// NewAgentModelsHandler creates an AgentModelsHandler. Every dep may be nil and
// fails soft/closed exactly like AgentsInstalledHandler: nil authz ⇒ zero
// accessible namespaces (empty list / create denied), nil dynamic or k8s client
// ⇒ the empty envelope on read and a create error, nil enforcer ⇒ create denied.
func NewAgentModelsHandler(dynamicClient dynamic.Interface, authz AccessibleNamespacesProvider, k8sClient kubernetes.Interface, enforcer secretsCreateChecker) *AgentModelsHandler {
	return &AgentModelsHandler{
		dynamicClient: dynamicClient,
		authz:         authz,
		k8sClient:     k8sClient,
		enforcer:      enforcer,
	}
}

func (h *AgentModelsHandler) accessibleNamespaces(ctx context.Context, userCtx *middleware.UserContext) ([]string, error) {
	if h.authz == nil {
		return []string{}, nil
	}
	return h.authz.GetAccessibleNamespaces(ctx, userCtx)
}

func emptyModelsResponse() ModelsResponse {
	return ModelsResponse{Models: []ModelSummary{}}
}

// ListModels handles GET /api/v1/agents/models. Mirrors ListAgents: resolve the
// Casbin-derived namespace set → one cluster-wide LIST of ModelConfig CRs →
// admit each whose namespace matches the caller's set. Zero accessible
// namespaces or a nil client ⇒ empty list (fail-closed). CRD-absent ⇒ 200 empty
// (never 5xx). NEVER reads apiKeySecretRef (NFR-A4).
func (h *AgentModelsHandler) ListModels(w http.ResponseWriter, r *http.Request) {
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
	if len(userNamespaces) == 0 || h.dynamicClient == nil {
		response.WriteJSON(w, http.StatusOK, emptyModelsResponse())
		return
	}

	list, err := h.dynamicClient.Resource(kagent.ModelConfigGVR).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		// CRD absent (kagent half-installed) ⇒ empty pool, not a 5xx — same
		// graceful-empty posture as ListAgents.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			response.WriteJSON(w, http.StatusOK, emptyModelsResponse())
			return
		}
		response.InternalError(w, "Failed to list models")
		return
	}

	resp := emptyModelsResponse()
	for i := range list.Items {
		item := &list.Items[i]
		ns := parser.GetNamespace(item)
		if !rbac.MatchNamespaceInList(ns, userNamespaces) {
			continue
		}
		// ok=false (both empty) still lists the config — the name identifies it
		// even when there's no {provider, model} label to show.
		model, _ := kagent.ModelFromConfigCR(item)
		resp.Models = append(resp.Models, ModelSummary{
			Name:      parser.GetName(item),
			Namespace: ns,
			Provider:  model.Provider,
			Model:     model.Name,
		})
	}

	sort.Slice(resp.Models, func(i, j int) bool {
		if resp.Models[i].Namespace != resp.Models[j].Namespace {
			return resp.Models[i].Namespace < resp.Models[j].Namespace
		}
		return resp.Models[i].Name < resp.Models[j].Name
	})

	response.WriteJSON(w, http.StatusOK, resp)
}

// CreateModel handles POST /api/v1/agents/models. Orchestrates, in order:
// validate → authorize → create the API-key Secret directly → create the
// KnodexAgentModelConfig instance directly (KRO reconciles it into a kagent
// ModelConfig) → on instance-create failure, delete the orphaned Secret. The
// apiKey is write-only and never echoed back (NFR-A4).
func (h *AgentModelsHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	var req createModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid JSON body", nil)
		return
	}
	name := sanitize.RemoveControlChars(req.Name)
	provider := sanitize.RemoveControlChars(req.Provider)
	model := sanitize.RemoveControlChars(req.Model)
	namespace := sanitize.RemoveControlChars(req.Namespace)
	// apiKey is NOT control-char-stripped beyond rejecting empties — a provider
	// key is an opaque credential; we never mangle it, only refuse a blank one.
	apiKey := req.APIKey

	if fields := validateCreateModelRequest(name, provider, model, namespace, apiKey); len(fields) > 0 {
		response.BadRequest(w, "Validation failed", fields)
		return
	}
	// Canonicalize BEFORE persisting: kagent's ModelConfig.spec.provider is a
	// strict enum (capitalized) — a wrong-cased value creates an instance KRO
	// can never reconcile, failing silently long after this 201.
	canonical, known := canonicalKagentProvider(provider)
	if !known {
		response.BadRequest(w, "Validation failed", map[string]string{
			"provider": "unknown provider — expected one of " + strings.Join(kagentProviderEnum, ", "),
		})
		return
	}
	provider = canonical

	if !h.authorizeCreate(w, r, userCtx, namespace, name) {
		return
	}

	if h.k8sClient == nil || h.dynamicClient == nil {
		response.InternalError(w, "kubernetes client unavailable")
		return
	}

	// Create the API-key Secret directly (NOT via the model-provider-config
	// RGD). Secret key is "apiKey" — the in-repo convention the model-config.yaml
	// ModelConfig template reads (its sibling model-provider-config.yaml writes
	// stringData.apiKey). ManagedByLabel mirrors SecretsHandler.CreateSecret.
	// The Secret name = the model name; the ModelConfig is a different kind, so
	// there is no K8s name collision in the namespace.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{models.ManagedByLabel: models.ManagedByValue},
		},
		StringData: map[string]string{"apiKey": apiKey},
		Type:       corev1.SecretTypeOpaque,
	}
	if _, err := h.k8sClient.CoreV1().Secrets(namespace).Create(r.Context(), secret, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			response.Conflict(w, "secret", name)
			return
		}
		response.InternalError(w, "Failed to create model secret")
		return
	}

	// Create the KnodexAgentModelConfig instance directly. KRO's in-cluster
	// controller reconciles it → reads the externalRef Secret → produces the
	// kagent.dev/v1alpha2 ModelConfig the Models list and Create Agent picker see.
	instance := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": agentModelConfigGVR.Group + "/" + agentModelConfigGVR.Version,
			"kind":       "KnodexAgentModelConfig",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"provider": provider,
				"model":    model,
				"externalRef": map[string]interface{}{
					"providerSecret": map[string]interface{}{
						"name":      name,
						"namespace": namespace,
					},
				},
			},
		},
	}
	if _, err := h.dynamicClient.Resource(agentModelConfigGVR).Namespace(namespace).Create(r.Context(), instance, metav1.CreateOptions{}); err != nil {
		// No orphaned credentials: best-effort delete the Secret we just minted
		// before surfacing the error. Uses a background context so the cleanup
		// still runs even if the request context was the cause of the failure.
		_ = h.k8sClient.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
		if apierrors.IsAlreadyExists(err) {
			response.Conflict(w, "model", name)
			return
		}
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			// The KnodexAgentModelConfig CRD is absent — the model RGD half of
			// the agents stack is not installed.
			response.ServiceUnavailable(w, "model RGD not installed")
			return
		}
		response.InternalError(w, "Failed to create model")
		return
	}

	response.WriteJSON(w, http.StatusCreated, ModelSummary{
		Name:      name,
		Namespace: namespace,
		Provider:  provider,
		Model:     model,
	})
}

// authorizeCreate applies the two-gate write authorization. On any non-ok
// return it has ALREADY written the response. The accessible-namespace gate
// alone is action-agnostic (a read-only user still has their namespaces in it),
// so a write that mints a credential needs the explicit secrets-create enforce.
//
// Instances-create is intentionally NOT enforced: the instances Casbin object
// is instances/{project}/{namespace} — it requires a project segment the
// namespace-keyed agents URL surface does not carry (mirroring how secrets
// themselves are enforced namespace-only, with no project segment). Fabricating
// a project would be wrong; the namespace gate establishes the user may act in
// the namespace, and the secrets-create gate covers the credential-minting risk.
func (h *AgentModelsHandler) authorizeCreate(w http.ResponseWriter, r *http.Request, userCtx *middleware.UserContext, namespace, name string) bool {
	userNamespaces, err := h.accessibleNamespaces(r.Context(), userCtx)
	if err != nil {
		response.InternalError(w, "Failed to get user namespaces")
		return false
	}
	if !rbac.MatchNamespaceInList(namespace, userNamespaces) {
		// 404 non-leak: an unauthorized caller cannot tell a namespace they
		// cannot see from one that does not exist (same posture as secrets).
		response.NotFound(w, "namespace", namespace)
		return false
	}

	if h.enforcer == nil {
		response.Forbidden(w, "permission denied")
		return false
	}
	// secrets/{namespace}/{name} — the namespace-keyed secrets Casbin object
	// (no project segment; see CLAUDE.md). Exact and cheap.
	object := "secrets/" + namespace + "/" + name
	allowed, err := h.enforcer.CanAccessWithGroups(r.Context(), userCtx.UserID, userCtx.Groups, object, "create")
	if err != nil {
		response.InternalError(w, "Failed to check authorization")
		return false
	}
	if !allowed {
		response.Forbidden(w, "permission denied")
		return false
	}
	return true
}

// kagentProviderEnum is the kagent 0.9.6 ModelConfig.spec.provider enum
// (CRD modelconfigs.kagent.dev v1alpha2) — kagent rejects anything else.
var kagentProviderEnum = []string{
	"OpenAI", "AzureOpenAI", "Anthropic", "Gemini",
	"GeminiVertexAI", "AnthropicVertexAI", "Ollama", "Bedrock", "SAPAICore",
}

// canonicalKagentProvider maps a case-insensitive provider (plus the legacy
// "azure" alias the UI used to send) to its canonical kagent enum value.
func canonicalKagentProvider(provider string) (string, bool) {
	lower := strings.ToLower(provider)
	if lower == "azure" {
		return "AzureOpenAI", true
	}
	for _, p := range kagentProviderEnum {
		if strings.ToLower(p) == lower {
			return p, true
		}
	}
	return "", false
}

// validateCreateModelRequest returns a field→message map of validation errors
// (empty when valid). namespace = DNS-1123 label, name = DNS-1123 subdomain,
// provider/model/apiKey non-empty (after control-char stripping for the first
// two). apiKey is checked blank-after-trim so a whitespace-only key can't mint a
// broken Secret — but it is validated, NOT mutated: the caller still stores the
// original key verbatim (we never mangle a credential).
func validateCreateModelRequest(name, provider, model, namespace, apiKey string) map[string]string {
	errs := map[string]string{}
	if !sanitize.IsValidDNS1123Label(namespace) {
		errs["namespace"] = "namespace must be a valid DNS-1123 label"
	}
	if !sanitize.IsValidDNS1123Subdomain(name) {
		errs["name"] = "name must be a valid DNS-1123 subdomain"
	}
	if provider == "" {
		errs["provider"] = "provider is required"
	}
	if model == "" {
		errs["model"] = "model is required"
	}
	if strings.TrimSpace(apiKey) == "" {
		errs["apiKey"] = "apiKey is required"
	}
	return errs
}
