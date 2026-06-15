// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent"
)

// stubEnforcer is a canned secretsCreateChecker: allow/deny (and optional error)
// for the explicit secrets-create gate in CreateModel.
type stubEnforcer struct {
	allow bool
	err   error
}

func (s *stubEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, _, _ string) (bool, error) {
	return s.allow, s.err
}

// modelsListKinds maps the GVRs the AgentModelsHandler touches to list kinds so
// the fake dynamic client can serve LIST/CREATE without panicking.
var modelsListKinds = map[schema.GroupVersionResource]string{
	{Group: "kagent.dev", Version: "v1alpha2", Resource: "modelconfigs"}:                  "ModelConfigList",
	{Group: "agents.knodex.io", Version: "v1alpha1", Resource: "knodexagentmodelconfigs"}: "KnodexAgentModelConfigList",
}

func newFakeModelsClient(t *testing.T, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, modelsListKinds, objects...)
}

// modelConfigUnstructured builds a kagent ModelConfig CR. apiKeySecretRef is
// intentionally populated so tests can assert it NEVER reaches the response.
func modelConfigUnstructured(name, namespace, provider, model string) *unstructured.Unstructured {
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
				"apiKeySecretRef": map[string]interface{}{
					"name":      name + "-secret",
					"namespace": namespace,
				},
			},
		},
	}
}

func authedModelsRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	userCtx := &middleware.UserContext{UserID: "test-user", Email: "test@example.com", Groups: []string{"alpha-devs"}}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func decodeModelsResponse(t *testing.T, w *httptest.ResponseRecorder) ModelsResponse {
	t.Helper()
	var resp ModelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func seedModelConfigs() []runtime.Object {
	return []runtime.Object{
		modelConfigUnstructured("zeta-model", "alpha-apps", "openai", "gpt-4o"),
		modelConfigUnstructured("alpha-model", "alpha-apps", "anthropic", "claude"),
		modelConfigUnstructured("beta-model", "beta-apps", "gemini", "gemini-pro"),
	}
}

// --- ListModels ---

func TestListModels_NamespaceFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		namespaces []string
		want       []string // "namespace/name", in response order
	}{
		{"single ns", []string{"alpha-apps"}, []string{"alpha-apps/alpha-model", "alpha-apps/zeta-model"}},
		{"wildcard all", []string{"*"}, []string{"alpha-apps/alpha-model", "alpha-apps/zeta-model", "beta-apps/beta-model"}},
		{"no overlap empty", []string{"gamma-apps"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newFakeModelsClient(t, seedModelConfigs()...)
			h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: tc.namespaces}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

			w := httptest.NewRecorder()
			h.ListModels(w, authedModelsRequest(t, http.MethodGet, "/api/v1/agents/models", ""))

			assert.Equal(t, http.StatusOK, w.Code)
			resp := decodeModelsResponse(t, w)
			got := make([]string, 0, len(resp.Models))
			for _, m := range resp.Models {
				got = append(got, m.Namespace+"/"+m.Name)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestListModels_FailClosedEmpty(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t, seedModelConfigs()...)
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.ListModels(w, authedModelsRequest(t, http.MethodGet, "/api/v1/agents/models", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"models": []}`, w.Body.String())
}

func TestListModels_CRDAbsent_EmptyList200(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	client.PrependReactor("list", "modelconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "kagent.dev", Kind: "ModelConfig"}}
	})
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"*"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.ListModels(w, authedModelsRequest(t, http.MethodGet, "/api/v1/agents/models", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"models": []}`, w.Body.String())
}

// NFR-A4: ListModels must never surface a secret reference or value.
func TestListModels_NoSecretInOutput(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t, seedModelConfigs()...)
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"*"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.ListModels(w, authedModelsRequest(t, http.MethodGet, "/api/v1/agents/models", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "apiKeySecretRef")
	assert.NotContains(t, body, "secret")
	// Sanity: the model identity DID make it through.
	assert.Contains(t, body, "gpt-4o")
}

func TestListModels_NilClient_EmptyList200(t *testing.T) {
	t.Parallel()
	h := NewAgentModelsHandler(nil, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.ListModels(w, authedModelsRequest(t, http.MethodGet, "/api/v1/agents/models", ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"models": []}`, w.Body.String())
}

func TestListModels_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t, seedModelConfigs()...)
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"*"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/models", nil)
	w := httptest.NewRecorder()
	h.ListModels(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- CreateModel ---

const validModelBody = `{"name":"my-model","provider":"openai","model":"gpt-4o","namespace":"alpha-apps","apiKey":"sk-secret-value"}`

func getSecret(t *testing.T, k8s kubernetes.Interface, ns, name string) (bool, error) {
	t.Helper()
	_, err := k8s.CoreV1().Secrets(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func getInstance(t *testing.T, client dynamic.Interface, ns, name string) (*unstructured.Unstructured, error) {
	t.Helper()
	return client.Resource(agentModelConfigGVR).Namespace(ns).Get(context.Background(), name, metav1.GetOptions{})
}

func TestCreateModel_HappyPath(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	k8s := k8sfake.NewSimpleClientset()
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8s, &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", validModelBody))

	require.Equal(t, http.StatusCreated, w.Code)

	// Response carries the summary, never the apiKey.
	var summary ModelSummary
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &summary))
	assert.Equal(t, "my-model", summary.Name)
	assert.Equal(t, "alpha-apps", summary.Namespace)
	assert.Equal(t, "OpenAI", summary.Provider)
	assert.Equal(t, "gpt-4o", summary.Model)
	assert.NotContains(t, w.Body.String(), "sk-secret-value")
	assert.NotContains(t, w.Body.String(), "apiKey")

	// Secret created with key apiKey + the provided value.
	secret, err := k8s.CoreV1().Secrets("alpha-apps").Get(context.Background(), "my-model", metav1.GetOptions{})
	require.NoError(t, err)
	// fake clientset surfaces StringData verbatim (no encode step).
	assert.Equal(t, "sk-secret-value", secret.StringData["apiKey"])
	assert.Equal(t, "knodex", secret.Labels["knodex.io/managed-by"])

	// KnodexAgentModelConfig instance created referencing the Secret.
	inst, err := getInstance(t, client, "alpha-apps", "my-model")
	require.NoError(t, err)
	spec, _, _ := unstructured.NestedMap(inst.Object, "spec")
	assert.Equal(t, "OpenAI", spec["provider"])
	assert.Equal(t, "gpt-4o", spec["model"])
	secName, _, _ := unstructured.NestedString(inst.Object, "spec", "externalRef", "providerSecret", "name")
	assert.Equal(t, "my-model", secName)
}

func TestCreateModel_DeniedNamespace_404(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	k8s := k8sfake.NewSimpleClientset()
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"beta-apps"}}, k8s, &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", validModelBody))

	assert.Equal(t, http.StatusNotFound, w.Code)
	// Nothing minted in a namespace the caller can't see.
	exists, err := getSecret(t, k8s, "alpha-apps", "my-model")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCreateModel_MissingSecretsCreate_403(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	k8s := k8sfake.NewSimpleClientset()
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8s, &stubEnforcer{allow: false})

	w := httptest.NewRecorder()
	h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", validModelBody))

	assert.Equal(t, http.StatusForbidden, w.Code)
	exists, err := getSecret(t, k8s, "alpha-apps", "my-model")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCreateModel_NilEnforcer_403(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	k8s := k8sfake.NewSimpleClientset()
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8s, nil)

	w := httptest.NewRecorder()
	h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", validModelBody))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Orphan cleanup: an instance-create failure AFTER the Secret was created must
// best-effort delete the Secret — no orphaned credential.
func TestCreateModel_InstanceFailure_OrphanSecretDeleted(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	client.PrependReactor("create", "knodexagentmodelconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("kro webhook rejected")
	})
	k8s := k8sfake.NewSimpleClientset()
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8s, &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", validModelBody))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// The Secret minted before the failure must be gone.
	exists, err := getSecret(t, k8s, "alpha-apps", "my-model")
	require.NoError(t, err)
	assert.False(t, exists, "orphaned Secret must be cleaned up on instance-create failure")
}

func TestCreateModel_SecretConflict_409(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	k8s := k8sfake.NewSimpleClientset()
	k8s.PrependReactor("create", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "secrets"}, "my-model")
	})
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8s, &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", validModelBody))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestCreateModel_Validation_400(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"bad name":          `{"name":"Bad_Name","provider":"openai","model":"gpt-4o","namespace":"alpha-apps","apiKey":"k"}`,
		"bad namespace":     `{"name":"my-model","provider":"openai","model":"gpt-4o","namespace":"Bad_NS","apiKey":"k"}`,
		"empty provider":    `{"name":"my-model","provider":"","model":"gpt-4o","namespace":"alpha-apps","apiKey":"k"}`,
		"empty apiKey":      `{"name":"my-model","provider":"openai","model":"gpt-4o","namespace":"alpha-apps","apiKey":""}`,
		"whitespace apiKey": `{"name":"my-model","provider":"openai","model":"gpt-4o","namespace":"alpha-apps","apiKey":"   "}`,
		"unknown provider":  `{"name":"my-model","provider":"notaprovider","model":"gpt-4o","namespace":"alpha-apps","apiKey":"k"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := newFakeModelsClient(t)
			h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

			w := httptest.NewRecorder()
			h.CreateModel(w, authedModelsRequest(t, http.MethodPost, "/api/v1/agents/models", body))

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestCreateModel_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t)
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/models", strings.NewReader(validModelBody))
	w := httptest.NewRecorder()
	h.CreateModel(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Guard that the reused kagent extractor still drives the summary (no hand
// re-parse drift).
func TestListModels_UsesSharedExtractor(t *testing.T) {
	t.Parallel()
	client := newFakeModelsClient(t, modelConfigUnstructured("m", "alpha-apps", "azure", "gpt-4o-mini"))
	h := NewAgentModelsHandler(client, &stubNamespaces{namespaces: []string{"alpha-apps"}}, k8sfake.NewSimpleClientset(), &stubEnforcer{allow: true})

	w := httptest.NewRecorder()
	h.ListModels(w, authedModelsRequest(t, http.MethodGet, "/api/v1/agents/models", ""))

	resp := decodeModelsResponse(t, w)
	require.Len(t, resp.Models, 1)
	want, _ := kagent.ModelFromConfigCR(modelConfigUnstructured("m", "alpha-apps", "azure", "gpt-4o-mini"))
	assert.Equal(t, want.Provider, resp.Models[0].Provider)
	assert.Equal(t, want.Name, resp.Models[0].Model)
}
