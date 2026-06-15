// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent"
)

// newInstalledHandler builds an AgentsInstalledHandler with no model resolver
// (the model field is omitted) — the default for the pre-Story-50.4 tests.
func newInstalledHandler(client dynamic.Interface, authz AccessibleNamespacesProvider) *AgentsInstalledHandler {
	return NewAgentsInstalledHandler(client, authz, nil)
}

// stubModelResolver is a canned kagent.ModelResolver: it returns the model for
// any (namespace, modelConfigName) present in its map, miss otherwise. It also
// records lookups so tests can assert a non-declarative agent triggers none.
// The mutex makes it safe for the concurrent goroutine fan-out in ListAgents.
type stubModelResolver struct {
	mu      sync.Mutex
	models  map[string]kagent.AgentModel // keyed "namespace/modelConfigName"
	lookups []string
}

func (s *stubModelResolver) ResolveForAgent(_ context.Context, namespace, modelConfigName string) (*kagent.AgentModel, bool) {
	key := namespace + "/" + modelConfigName
	s.mu.Lock()
	s.lookups = append(s.lookups, key)
	m, ok := s.models[key]
	s.mu.Unlock()
	if !ok {
		return nil, false
	}
	out := m
	return &out, true
}

// agentWithModelConfig builds an Agent CR whose spec.declarative.modelConfig
// names a ModelConfig (empty modelConfig ⇒ no declarative block).
func agentWithModelConfig(name, namespace, description, modelConfig string) *unstructured.Unstructured {
	u := agentUnstructured(name, namespace, description)
	if modelConfig != "" {
		spec, _ := u.Object["spec"].(map[string]interface{})
		spec["type"] = "Declarative"
		spec["declarative"] = map[string]interface{}{"modelConfig": modelConfig}
	}
	return u
}

// stubNamespaces is a canned AccessibleNamespacesProvider.
type stubNamespaces struct {
	namespaces []string
	err        error
}

func (s *stubNamespaces) GetAccessibleNamespaces(context.Context, *middleware.UserContext) ([]string, error) {
	return s.namespaces, s.err
}

// agentsListKinds maps the kagent Agent GVR to its list kind so the fake
// dynamic client can serve LIST without panicking.
var agentsListKinds = map[schema.GroupVersionResource]string{
	{Group: "kagent.dev", Version: "v1alpha2", Resource: "agents"}: "AgentList",
}

func agentUnstructured(name, namespace, description string) *unstructured.Unstructured {
	spec := map[string]interface{}{}
	if description != "" {
		spec["description"] = description
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kagent.dev/v1alpha2",
			"kind":       "Agent",
			"metadata": map[string]interface{}{
				"name":              name,
				"namespace":         namespace,
				"creationTimestamp": "2026-06-01T10:00:00Z",
			},
			"spec": spec,
		},
	}
}

// newFakeAgentsClient seeds Agents across two namespaces (alpha-apps, beta-apps).
func newFakeAgentsClient(t *testing.T, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, agentsListKinds, objects...)
}

func authedInstalledRequest(t *testing.T) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	userCtx := &middleware.UserContext{
		UserID: "test-user",
		Email:  "test@example.com",
	}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func decodeAgentsResponse(t *testing.T, w *httptest.ResponseRecorder) AgentsResponse {
	t.Helper()
	var resp AgentsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// seedAgents are the default fixtures: two namespaces, three agents.
func seedAgents() []runtime.Object {
	return []runtime.Object{
		agentUnstructured("zeta-helper", "alpha-apps", "Helps with zeta things"),
		agentUnstructured("alpha-helper", "alpha-apps", "Helps with alpha things"),
		agentUnstructured("beta-helper", "beta-apps", "Helps with beta things"),
	}
}

func TestAgentsInstalledHandler_NamespaceFilterMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		namespaces []string
		wantAgents []string // "namespace/name" expected, in response order
	}{
		{
			name:       "single namespace filters out others",
			namespaces: []string{"alpha-apps"},
			wantAgents: []string{"alpha-apps/alpha-helper", "alpha-apps/zeta-helper"},
		},
		{
			name:       "global admin wildcard sees all",
			namespaces: []string{"*"},
			wantAgents: []string{"alpha-apps/alpha-helper", "alpha-apps/zeta-helper", "beta-apps/beta-helper"},
		},
		{
			name:       "wildcard pattern matches prefix",
			namespaces: []string{"alpha-*"},
			wantAgents: []string{"alpha-apps/alpha-helper", "alpha-apps/zeta-helper"},
		},
		{
			name:       "no overlap yields empty list",
			namespaces: []string{"gamma-apps"},
			wantAgents: []string{},
		},
		{
			// The most common real-world shape: a user who is a member of
			// several projects resolves to multiple explicit namespaces.
			// Also proves namespace-then-name sorting without a wildcard.
			name:       "multiple explicit namespaces see the union",
			namespaces: []string{"alpha-apps", "beta-apps"},
			wantAgents: []string{"alpha-apps/alpha-helper", "alpha-apps/zeta-helper", "beta-apps/beta-helper"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newFakeAgentsClient(t, seedAgents()...)
			handler := newInstalledHandler(client, &stubNamespaces{namespaces: tc.namespaces})

			w := httptest.NewRecorder()
			handler.ListAgents(w, authedInstalledRequest(t))

			assert.Equal(t, http.StatusOK, w.Code)
			resp := decodeAgentsResponse(t, w)

			got := make([]string, 0, len(resp.Agents))
			for _, a := range resp.Agents {
				got = append(got, a.Namespace+"/"+a.Name)
			}
			// Equality on the ordered slice also asserts deterministic
			// namespace-then-name sorting.
			assert.Equal(t, tc.wantAgents, got)
		})
	}
}

// TestAgentsInstalledHandler_EmptyNamespaces_EmptyList: a caller with zero
// accessible namespaces sees NO agents (Story 53.1 removed the FR13 global-hub
// carve-out — fail-closed). The cluster LIST is short-circuited entirely.
func TestAgentsInstalledHandler_EmptyNamespaces_EmptyList(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t, seedAgents()...)
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"agents": []}`, w.Body.String())
}

func TestAgentsInstalledHandler_NilAuthz_FailsClosed(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t, seedAgents()...)
	handler := newInstalledHandler(client, nil)

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	// Nil authz ⇒ zero accessible namespaces ⇒ empty list (fail-closed).
	assert.JSONEq(t, `{"agents": []}`, w.Body.String())
}

func TestAgentsInstalledHandler_AuthzError_500(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t, seedAgents()...)
	handler := newInstalledHandler(client, &stubNamespaces{err: errors.New("casbin exploded")})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "INTERNAL_ERROR", errResp["code"])
}

func TestAgentsInstalledHandler_ListNotFound_EmptyList200(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t)
	client.PrependReactor("list", "agents", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(
			schema.GroupResource{Group: "kagent.dev", Resource: "agents"}, "")
	})
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	// CRD vanished between presence check and list — graceful, not a 5xx.
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"agents": []}`, w.Body.String())
}

func TestAgentsInstalledHandler_ListNoMatch_EmptyList200(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t)
	client.PrependReactor("list", "agents", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, &meta.NoKindMatchError{
			GroupKind: schema.GroupKind{Group: "kagent.dev", Kind: "Agent"},
		}
	})
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"agents": []}`, w.Body.String())
}

func TestAgentsInstalledHandler_ListError_500(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t)
	client.PrependReactor("list", "agents", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("apiserver on fire")
	})
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestAgentsInstalledHandler_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t, seedAgents()...)
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	// No user context — simulates a request that bypassed auth middleware.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	handler.ListAgents(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "UNAUTHORIZED", errResp["code"])
}

// A nil dynamic client means no cluster to query — empty list with 200, not a
// 500. A degraded server with no K8s client must not turn every page load into
// an error boundary.
func TestAgentsInstalledHandler_NilDynamicClient_EmptyList200(t *testing.T) {
	t.Parallel()
	handler := newInstalledHandler(nil, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"agents": []}`, w.Body.String())
}

func TestAgentsInstalledHandler_DTOMapping(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t,
		agentUnstructured("described", "alpha-apps", "An agent with a description"),
		agentUnstructured("undescribed", "alpha-apps", ""), // spec.description absent
	)
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAgentsResponse(t, w)
	require.Len(t, resp.Agents, 2)

	// Sorted by name within the namespace.
	described := resp.Agents[0]
	assert.Equal(t, "described", described.Name)
	assert.Equal(t, "alpha-apps", described.Namespace)
	assert.Equal(t, "An agent with a description", described.Description)
	assert.Equal(t, "2026-06-01T10:00:00Z", described.CreatedAt)

	// Missing spec.description tolerated — empty string, never an error.
	undescribed := resp.Agents[1]
	assert.Equal(t, "undescribed", undescribed.Name)
	assert.Equal(t, "", undescribed.Description)
	assert.Equal(t, "2026-06-01T10:00:00Z", undescribed.CreatedAt)
}

// TestAgentsInstalledHandler_ReflectsDeletion pins the cache-free read path
// (Story 49.3, AC 4): the handler does a live LIST per request, so deleting an
// Agent CR (e.g. via KRO garbage collection after instance delete) is
// reflected on the very next call. If anyone introduces a cache here, this
// test breaks — by design.
func TestAgentsInstalledHandler_ReflectsDeletion(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t,
		agentUnstructured("keeper", "alpha-apps", "Stays installed"),
		agentUnstructured("goner", "alpha-apps", "Will be deleted"),
	)
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	// First call: both agents visible.
	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))
	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAgentsResponse(t, w)
	require.Len(t, resp.Agents, 2)

	// Delete one Agent CR on the fake tracker — simulates KRO ownerReference
	// garbage collection after the owning instance is deleted.
	gvr := schema.GroupVersionResource{Group: "kagent.dev", Version: "v1alpha2", Resource: "agents"}
	require.NoError(t, client.Resource(gvr).Namespace("alpha-apps").
		Delete(context.Background(), "goner", metav1.DeleteOptions{}))

	// Second call: the deletion is reflected immediately — no cache.
	w = httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))
	assert.Equal(t, http.StatusOK, w.Code)
	resp = decodeAgentsResponse(t, w)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "keeper", resp.Agents[0].Name)
}

func TestAgentsInstalledHandler_MissingCreationTimestamp_EmptyCreatedAt(t *testing.T) {
	t.Parallel()
	// An Agent CR with no metadata.creationTimestamp (e.g. hand-applied
	// fixture) must map to createdAt "" — never a zero-time string or panic.
	timeless := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kagent.dev/v1alpha2",
			"kind":       "Agent",
			"metadata": map[string]interface{}{
				"name":      "timeless",
				"namespace": "alpha-apps",
			},
			"spec": map[string]interface{}{"description": "no creation timestamp"},
		},
	}
	client := newFakeAgentsClient(t, timeless)
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAgentsResponse(t, w)
	require.Len(t, resp.Agents, 1)
	assert.Equal(t, "timeless", resp.Agents[0].Name)
	assert.Equal(t, "", resp.Agents[0].CreatedAt)
}

// TestAgentsInstalledHandler_ModelResolution covers Story 50.4 AC #2/#4: a
// resolvable model is attached as {provider, name}; an unresolvable one or a
// non-declarative agent leaves the field omitted; list ordering is preserved.
func TestAgentsInstalledHandler_ModelResolution(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t,
		agentWithModelConfig("with-model", "alpha-apps", "has a model", "default-model-config"),
		agentWithModelConfig("unresolvable", "alpha-apps", "model config missing", "ghost-config"),
		agentWithModelConfig("no-declarative", "alpha-apps", "BYO type, no modelConfig", ""),
	)
	resolver := &stubModelResolver{
		models: map[string]kagent.AgentModel{
			"alpha-apps/default-model-config": {Provider: "OpenAI", Name: "gpt-4.1-mini"},
			// "ghost-config" intentionally absent ⇒ resolver miss.
		},
	}
	handler := NewAgentsInstalledHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, resolver)

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAgentsResponse(t, w)
	require.Len(t, resp.Agents, 3)

	// Sorted by name: no-declarative, unresolvable, with-model.
	byName := map[string]InstalledAgent{}
	for _, a := range resp.Agents {
		byName[a.Name] = a
	}

	require.NotNil(t, byName["with-model"].Model)
	assert.Equal(t, "OpenAI", byName["with-model"].Model.Provider)
	assert.Equal(t, "gpt-4.1-mini", byName["with-model"].Model.Name)

	assert.Nil(t, byName["unresolvable"].Model, "resolver miss ⇒ model omitted")
	assert.Nil(t, byName["no-declarative"].Model, "no modelConfig ⇒ no lookup, model omitted")

	// A non-declarative agent must not even trigger a resolver lookup.
	assert.NotContains(t, resolver.lookups, "alpha-apps/")
	assert.ElementsMatch(t,
		[]string{"alpha-apps/default-model-config", "alpha-apps/ghost-config"},
		resolver.lookups)
}

// TestAgentsInstalledHandler_NilResolver_ModelOmitted pins the fail-soft
// default: with no resolver wired, every agent lists without a model field.
func TestAgentsInstalledHandler_NilResolver_ModelOmitted(t *testing.T) {
	t.Parallel()
	client := newFakeAgentsClient(t,
		agentWithModelConfig("with-config", "alpha-apps", "has a modelConfig", "default-model-config"),
	)
	handler := newInstalledHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	handler.ListAgents(w, authedInstalledRequest(t))

	assert.Equal(t, http.StatusOK, w.Code)
	resp := decodeAgentsResponse(t, w)
	require.Len(t, resp.Agents, 1)
	assert.Nil(t, resp.Agents[0].Model)

	// The DTO must serialize without a "model" key (omitempty) when absent.
	assert.NotContains(t, w.Body.String(), `"model"`)
}
