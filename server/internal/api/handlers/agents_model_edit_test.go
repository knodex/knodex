// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent"
)

// agentEditListKinds registers BOTH the Agent and ModelConfig GVRs so the fake
// dynamic client can serve Get/List/Patch on each.
var agentEditListKinds = map[schema.GroupVersionResource]string{
	{Group: "kagent.dev", Version: "v1alpha2", Resource: "agents"}:       "AgentList",
	{Group: "kagent.dev", Version: "v1alpha2", Resource: "modelconfigs"}: "ModelConfigList",
}

func newFakeAgentEditClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), agentEditListKinds, objects...)
}

// modelConfigCR builds a kagent ModelConfig with the given provider/model.
func modelConfigCR(name, namespace, provider, model string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kagent.dev/v1alpha2",
			"kind":       "ModelConfig",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"provider": provider,
				"model":    model,
			},
		},
	}
}

// declarativeAgent builds an Agent whose declarative block carries a
// modelConfig AND a systemMessage — the systemMessage is the canary that must
// survive a model patch untouched.
func declarativeAgent(name, namespace, modelConfig string) *unstructured.Unstructured {
	u := agentUnstructured(name, namespace, "an agent")
	spec, _ := u.Object["spec"].(map[string]interface{})
	spec["type"] = "Declarative"
	spec["declarative"] = map[string]interface{}{
		"modelConfig":   modelConfig,
		"systemMessage": "you are a curated agent — do not change me",
	}
	return u
}

// modelEditRequest builds an authed request with the {namespace}/{name} path
// values set explicitly — httptest.NewRequest does NOT populate PathValue
// without routing through a ServeMux, so the handler-direct tests set them here.
func modelEditRequest(t *testing.T, method, namespace, name, suffix, body string, roles []string) *http.Request {
	t.Helper()
	target := "/api/v1/agents/" + namespace + "/" + name + "/" + suffix
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	r.SetPathValue("namespace", namespace)
	r.SetPathValue("name", name)
	userCtx := &middleware.UserContext{UserID: "u", Email: "u@example.com", CasbinRoles: roles}
	return r.WithContext(context.WithValue(r.Context(), middleware.UserContextKey, userCtx))
}

func getAgentDeclarative(t *testing.T, client *dynamicfake.FakeDynamicClient, namespace, name string) map[string]interface{} {
	t.Helper()
	got, err := client.Resource(agentsGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	decl, _, _ := unstructured.NestedMap(got.Object, "spec", "declarative")
	return decl
}

// --- ListModelConfigs ---------------------------------------------------------

func TestListModelConfigs_InstalledAgent_SortedPool(t *testing.T) {
	client := newFakeAgentEditClient(
		declarativeAgent("byoa", "alpha-apps", "cfg-a"),
		modelConfigCR("cfg-b", "alpha-apps", "azure", "gpt-4"),
		modelConfigCR("cfg-a", "alpha-apps", "openai", "gpt-4o"),
		modelConfigCR("other-ns-cfg", "beta-apps", "openai", "gpt-4o"),
	)
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	h.ListModelConfigs(w, modelEditRequest(t, http.MethodGet, "alpha-apps", "byoa", "modelconfigs", "", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var resp ModelConfigsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// Sorted by name; scoped to the agent's namespace (beta-apps excluded).
	require.Len(t, resp.ModelConfigs, 2)
	assert.Equal(t, "cfg-a", resp.ModelConfigs[0].Name)
	assert.Equal(t, "openai", resp.ModelConfigs[0].Provider)
	assert.Equal(t, "gpt-4o", resp.ModelConfigs[0].Model)
	assert.Equal(t, "cfg-b", resp.ModelConfigs[1].Name)
}

func TestListModelConfigs_DeniedNamespace_404NonLeak(t *testing.T) {
	client := newFakeAgentEditClient(
		declarativeAgent("byoa", "alpha-apps", "cfg-a"),
		modelConfigCR("cfg-a", "alpha-apps", "openai", "gpt-4o"),
	)
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"other"}})

	w := httptest.NewRecorder()
	h.ListModelConfigs(w, modelEditRequest(t, http.MethodGet, "alpha-apps", "byoa", "modelconfigs", "", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListModelConfigs_MissingAgent_404(t *testing.T) {
	client := newFakeAgentEditClient()
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	w := httptest.NewRecorder()
	h.ListModelConfigs(w, modelEditRequest(t, http.MethodGet, "alpha-apps", "ghost", "modelconfigs", "", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- EditModel ----------------------------------------------------------------

func TestEditModel_InstalledAgent_PatchesModelOnly(t *testing.T) {
	client := newFakeAgentEditClient(
		declarativeAgent("byoa", "alpha-apps", "old-cfg"),
		modelConfigCR("new-cfg", "alpha-apps", "azure", "gpt-4"),
	)
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	h.EditModel(w, modelEditRequest(t, http.MethodPatch, "alpha-apps", "byoa", "model",
		`{"modelConfig":"new-cfg"}`, nil))

	require.Equal(t, http.StatusOK, w.Code)
	var model kagent.AgentModel
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &model))
	assert.Equal(t, "azure", model.Provider)
	assert.Equal(t, "gpt-4", model.Name)

	// The patch repointed modelConfig AND preserved the curated systemMessage.
	decl := getAgentDeclarative(t, client, "alpha-apps", "byoa")
	assert.Equal(t, "new-cfg", decl["modelConfig"])
	assert.Equal(t, "you are a curated agent — do not change me", decl["systemMessage"])
}

func TestEditModel_NonDeclarativeAgent_400(t *testing.T) {
	// A plain agent with NO spec.declarative block (e.g. a non-declarative /
	// BYO agent). Patching would fabricate a malformed declarative block.
	client := newFakeAgentEditClient(
		agentUnstructured("byoa", "alpha-apps", "a non-declarative agent"),
		modelConfigCR("new-cfg", "alpha-apps", "azure", "gpt-4"),
	)
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	h.EditModel(w, modelEditRequest(t, http.MethodPatch, "alpha-apps", "byoa", "model",
		`{"modelConfig":"new-cfg"}`, nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	// The agent must NOT have had a declarative block fabricated onto it.
	got, err := client.Resource(agentsGVR).Namespace("alpha-apps").Get(
		context.Background(), "byoa", metav1.GetOptions{})
	require.NoError(t, err)
	_, hasDecl, _ := unstructured.NestedMap(got.Object, "spec", "declarative")
	assert.False(t, hasDecl, "spec.declarative must not be created on a non-declarative agent")
}

func TestModelEdit_InvalidPath_400(t *testing.T) {
	client := newFakeAgentEditClient(declarativeAgent("byoa", "alpha-apps", "cfg-a"))
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	cases := []struct {
		name, ns, agent string
	}{
		{"uppercase namespace", "Alpha_Apps", "byoa"},
		{"namespace with slash", "a/b", "byoa"},
		{"invalid name chars", "alpha-apps", "Bad_Name"},
	}
	for _, tc := range cases {
		t.Run("list/"+tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ListModelConfigs(w, modelEditRequest(t, http.MethodGet, tc.ns, tc.agent, "modelconfigs", "", nil))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
		t.Run("patch/"+tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.EditModel(w, modelEditRequest(t, http.MethodPatch, tc.ns, tc.agent, "model",
				`{"modelConfig":"new-cfg"}`, nil))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestEditModel_ModelConfigNotFound_404(t *testing.T) {
	client := newFakeAgentEditClient(
		declarativeAgent("byoa", "alpha-apps", "old-cfg"),
	)
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	w := httptest.NewRecorder()
	h.EditModel(w, modelEditRequest(t, http.MethodPatch, "alpha-apps", "byoa", "model",
		`{"modelConfig":"ghost-cfg"}`, nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
	// The agent must NOT have been patched to a dangling reference.
	decl := getAgentDeclarative(t, client, "alpha-apps", "byoa")
	assert.Equal(t, "old-cfg", decl["modelConfig"])
}

func TestEditModel_InvalidBody(t *testing.T) {
	client := newFakeAgentEditClient(declarativeAgent("byoa", "alpha-apps", "old-cfg"))
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}})

	cases := []struct {
		name string
		body string
	}{
		{"empty modelConfig", `{"modelConfig":""}`},
		{"missing field", `{}`},
		{"invalid dns label", `{"modelConfig":"Not A Valid Name"}`},
		{"malformed json", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.EditModel(w, modelEditRequest(t, http.MethodPatch, "alpha-apps", "byoa", "model", tc.body, nil))
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestEditModel_NilClient_500(t *testing.T) {
	h := NewAgentModelEditHandler(nil, &stubNamespaces{namespaces: []string{"alpha-apps"}})
	w := httptest.NewRecorder()
	h.EditModel(w, modelEditRequest(t, http.MethodPatch, "alpha-apps", "byoa", "model",
		`{"modelConfig":"new-cfg"}`, nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestModelEdit_Unauthenticated_401(t *testing.T) {
	client := newFakeAgentEditClient()
	h := NewAgentModelEditHandler(client, &stubNamespaces{namespaces: []string{"*"}})

	// No UserContext on the request.
	w := httptest.NewRecorder()
	h.ListModelConfigs(w, httptest.NewRequest(http.MethodGet, "/api/v1/agents/alpha-apps/byoa/modelconfigs", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = httptest.NewRecorder()
	h.EditModel(w, httptest.NewRequest(http.MethodPatch, "/api/v1/agents/alpha-apps/byoa/model", strings.NewReader(`{"modelConfig":"x"}`)))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
