// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package kagent

import (
	"context"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/knodex/knodex/server/internal/k8s/parser"
)

// ModelConfigGVR is the kagent ModelConfig CRD group/version/resource
// (modelconfigs.kagent.dev, served at kagent.dev/v1alpha2). A ModelConfig
// carries the actual provider + model an Agent runs on. Exported so the
// model-edit handler lists/verifies ModelConfigs against the SAME GVR the
// resolver reads, with no second definition to drift.
var ModelConfigGVR = schema.GroupVersionResource{
	Group:    "kagent.dev",
	Version:  "v1alpha2",
	Resource: "modelconfigs",
}

// AgentModel is the frontend-safe model identity for an agent: the
// provider + model display pair, and NOTHING else. Provider and model name
// are intentionally surfaced as a transparency feature (UX-DR3); the
// ModelConfig's apiKeySecret reference and any provider endpoint/base-URL
// are NEVER copied here (NFR-A4 and the secret half of NFR-A7).
type AgentModel struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
}

// ModelResolver resolves an agent's model identity from the ModelConfig CR it
// references. One method so handlers and tests can stub it trivially (the
// 49.1/49.2 narrow-interface lesson). Every failure mode is fail-soft:
// a (nil, false) miss never surfaces an error, so the agents surface stays
// fully listable and invocable when resolution fails (AC #4).
type ModelResolver interface {
	// ResolveForAgent reads the named ModelConfig in namespace and returns its
	// {provider, model}. Returns (nil, false) on any miss: empty inputs,
	// missing ModelConfig, absent CRD, RBAC denial, or a ModelConfig with
	// neither provider nor model set.
	ResolveForAgent(ctx context.Context, namespace, modelConfigName string) (*AgentModel, bool)
}

// ModelConfigNameFromAgent extracts spec.declarative.modelConfig from an Agent
// CR (nil-safe; "" when the agent is non-declarative or omits a modelConfig).
// The installed/hub agents handler already holds the Agent CR, so it resolves
// the model in one step via ResolveForAgent without re-fetching the CR.
func ModelConfigNameFromAgent(agent *unstructured.Unstructured) string {
	return parser.GetSpecFieldStringOrDefault(agent, "", "declarative", "modelConfig")
}

// ModelFromConfigCR extracts the {provider, model} identity from a ModelConfig
// CR. Returns ok=false when BOTH are empty (a degenerate config that carries no
// resolvable model) — the single rule every read path shares so the resolver,
// the modelconfigs list, and the post-edit echo can never disagree on what a
// "no model" config means. apiKeySecret / provider endpoints are deliberately
// NOT read (NFR-A4/A7).
func ModelFromConfigCR(obj *unstructured.Unstructured) (AgentModel, bool) {
	provider := parser.GetSpecFieldStringOrDefault(obj, "", "provider")
	name := parser.GetSpecFieldStringOrDefault(obj, "", "model")
	if provider == "" && name == "" {
		return AgentModel{}, false
	}
	return AgentModel{Provider: provider, Name: name}, true
}

// modelConfigCacheTTL bounds how long a resolved model or Agent CR lookup is
// served from memory. ModelConfigs and Agent CR modelConfig references change
// rarely; a few minutes keeps repeated hub/page loads at memory speed without
// going stale in any way a user would notice.
const modelConfigCacheTTL = 5 * time.Minute

type cachedModel struct {
	model   AgentModel // value, not pointer — no dereference indirection on read
	expires time.Time
}

// dynamicModelResolver resolves models via the SERVER's dynamic client.
// ModelConfigs live in the kagent namespace (typically outside user projects),
// so a per-user client could not read them — but this is safe because the
// resolved value is only ever ATTACHED to an agent the caller is already
// authorized to see (the installed-agents namespace filter / FR13 built-ins);
// we never expose an arbitrary ModelConfig, only the one a visible agent binds.
type dynamicModelResolver struct {
	client dynamic.Interface
	ttl    time.Duration

	mu         sync.Mutex
	modelCache map[string]cachedModel // "ns/modelConfigName" → AgentModel
}

// NewModelResolver builds a ModelResolver backed by the dynamic client. A nil
// client yields a resolver that always misses (fail-soft) rather than panics.
func NewModelResolver(client dynamic.Interface) ModelResolver {
	return &dynamicModelResolver{
		client:     client,
		ttl:        modelConfigCacheTTL,
		modelCache: make(map[string]cachedModel),
	}
}

// ResolveForAgent implements ModelResolver. The modelCache is positive-only: a
// miss is never cached, so a ModelConfig that appears later (CRD installed,
// config created) resolves on the next call instead of being pinned absent for
// the TTL — mirrors the presence checker's "never cache a degraded/negative
// result authoritatively" rule.
func (r *dynamicModelResolver) ResolveForAgent(ctx context.Context, namespace, modelConfigName string) (*AgentModel, bool) {
	if r == nil || r.client == nil || namespace == "" || modelConfigName == "" {
		return nil, false
	}

	key := namespace + "/" + modelConfigName

	r.mu.Lock()
	if c, ok := r.modelCache[key]; ok && time.Now().Before(c.expires) {
		model := c.model // copy value — no pointer dereference needed
		r.mu.Unlock()
		return &model, true
	}
	r.mu.Unlock()

	obj, err := r.client.Resource(ModelConfigGVR).Namespace(namespace).Get(ctx, modelConfigName, metav1.GetOptions{})
	if err != nil {
		// Fail-soft for every error: ModelConfig not found (renamed/deleted),
		// CRD absent (NoMatch), RBAC denial, transport error. No error bubbles
		// into the agents response — the model field is simply omitted.
		return nil, false
	}

	// provider has an OpenAI default in the CRD, so a real ModelConfig almost
	// always carries it; we read what is actually present and never synthesize.
	model, ok := ModelFromConfigCR(obj)
	if !ok {
		return nil, false
	}

	r.mu.Lock()
	r.modelCache[key] = cachedModel{model: model, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()

	out := model // copy before returning pointer
	return &out, true
}
