// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package kagent

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// modelConfigListKinds maps the ModelConfig GVR to its list kind so the fake
// dynamic client can serve Get/List without panicking.
var modelConfigListKinds = map[schema.GroupVersionResource]string{
	ModelConfigGVR: "ModelConfigList",
}

// modelConfigUnstructured builds a ModelConfig CR. A nil/empty field is
// omitted from the spec so we can exercise partial-spec resolution. The
// apiKeySecret/baseUrl fields are seeded to PROVE the resolver never copies
// them into AgentModel.
func modelConfigUnstructured(name, namespace, provider, model string) *unstructured.Unstructured {
	spec := map[string]interface{}{
		// Secret-shaped fields that MUST NOT leak into the DTO.
		"apiKeySecret":    "kagent-openai",
		"apiKeySecretKey": "OPENAI_API_KEY",
		"openAI": map[string]interface{}{
			"baseUrl": "https://api.openai.com/v1",
		},
	}
	if provider != "" {
		spec["provider"] = provider
	}
	if model != "" {
		spec["model"] = model
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kagent.dev/v1alpha2",
			"kind":       "ModelConfig",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
}

func newFakeModelConfigClient(objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, modelConfigListKinds, objects...)
}

func TestModelResolver_ResolveForAgent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		seed            *unstructured.Unstructured
		namespace       string
		modelConfigName string
		wantOK          bool
		wantProvider    string
		wantName        string
	}{
		{
			name:            "resolve hit returns provider and model",
			seed:            modelConfigUnstructured("default-model-config", "kagent", "OpenAI", "gpt-4.1-mini"),
			namespace:       "kagent",
			modelConfigName: "default-model-config",
			wantOK:          true,
			wantProvider:    "OpenAI",
			wantName:        "gpt-4.1-mini",
		},
		{
			name:            "missing ModelConfig is a miss",
			seed:            modelConfigUnstructured("default-model-config", "kagent", "OpenAI", "gpt-4.1-mini"),
			namespace:       "kagent",
			modelConfigName: "does-not-exist",
			wantOK:          false,
		},
		{
			name:            "empty modelConfig name is a miss with no lookup",
			seed:            modelConfigUnstructured("default-model-config", "kagent", "OpenAI", "gpt-4.1-mini"),
			namespace:       "kagent",
			modelConfigName: "",
			wantOK:          false,
		},
		{
			name:            "only provider present still resolves",
			seed:            modelConfigUnstructured("provider-only", "kagent", "Anthropic", ""),
			namespace:       "kagent",
			modelConfigName: "provider-only",
			wantOK:          true,
			wantProvider:    "Anthropic",
			wantName:        "",
		},
		{
			name:            "only model present still resolves",
			seed:            modelConfigUnstructured("model-only", "kagent", "", "claude-sonnet-4"),
			namespace:       "kagent",
			modelConfigName: "model-only",
			wantOK:          true,
			wantProvider:    "",
			wantName:        "claude-sonnet-4",
		},
		{
			name:            "neither provider nor model is a miss",
			seed:            modelConfigUnstructured("empty", "kagent", "", ""),
			namespace:       "kagent",
			modelConfigName: "empty",
			wantOK:          false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newFakeModelConfigClient(tc.seed)
			resolver := NewModelResolver(client)

			model, ok := resolver.ResolveForAgent(context.Background(), tc.namespace, tc.modelConfigName)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				require.NotNil(t, model)
				assert.Equal(t, tc.wantProvider, model.Provider)
				assert.Equal(t, tc.wantName, model.Name)
			} else {
				assert.Nil(t, model)
			}
		})
	}
}

// TestModelResolver_NeverCopiesSecretFields pins NFR-A4/A7: AgentModel only
// ever carries provider + name. There is no field on AgentModel that could
// hold the seeded apiKeySecret/baseUrl, but this test documents the contract
// and would fail loudly if the struct were ever widened with a leaking field.
func TestModelResolver_NeverCopiesSecretFields(t *testing.T) {
	t.Parallel()
	client := newFakeModelConfigClient(
		modelConfigUnstructured("default-model-config", "kagent", "OpenAI", "gpt-4.1-mini"),
	)
	resolver := NewModelResolver(client)

	model, ok := resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	require.True(t, ok)
	require.NotNil(t, model)
	assert.Equal(t, AgentModel{Provider: "OpenAI", Name: "gpt-4.1-mini"}, *model)
}

func TestModelResolver_CRDAbsent_IsMiss(t *testing.T) {
	t.Parallel()
	client := newFakeModelConfigClient()
	client.PrependReactor("get", "modelconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, &meta.NoKindMatchError{
			GroupKind: schema.GroupKind{Group: "kagent.dev", Kind: "ModelConfig"},
		}
	})
	resolver := NewModelResolver(client)

	model, ok := resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	assert.False(t, ok)
	assert.Nil(t, model)
}

func TestModelResolver_NotFound_IsMiss(t *testing.T) {
	t.Parallel()
	client := newFakeModelConfigClient()
	client.PrependReactor("get", "modelconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(
			schema.GroupResource{Group: "kagent.dev", Resource: "modelconfigs"}, "default-model-config")
	})
	resolver := NewModelResolver(client)

	model, ok := resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	assert.False(t, ok)
	assert.Nil(t, model)
}

func TestModelResolver_TransportError_IsMiss(t *testing.T) {
	t.Parallel()
	client := newFakeModelConfigClient()
	client.PrependReactor("get", "modelconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("apiserver on fire")
	})
	resolver := NewModelResolver(client)

	model, ok := resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	assert.False(t, ok)
	assert.Nil(t, model)
}

func TestModelResolver_NilClient_IsMiss(t *testing.T) {
	t.Parallel()
	resolver := NewModelResolver(nil)
	model, ok := resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	assert.False(t, ok)
	assert.Nil(t, model)
}

// TestModelResolver_PositiveCacheServesRepeat proves the positive cache: a
// second resolve is served from memory (no second Get action), while a miss is
// NOT cached (a later-created ModelConfig resolves on the next call).
func TestModelResolver_PositiveCacheServesRepeat(t *testing.T) {
	t.Parallel()
	client := newFakeModelConfigClient(
		modelConfigUnstructured("default-model-config", "kagent", "OpenAI", "gpt-4.1-mini"),
	)
	resolver := NewModelResolver(client)

	_, ok := resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	require.True(t, ok)
	gets := countGetActions(client, "modelconfigs")
	require.Equal(t, 1, gets)

	_, ok = resolver.ResolveForAgent(context.Background(), "kagent", "default-model-config")
	require.True(t, ok)
	// Served from cache — no additional Get.
	assert.Equal(t, 1, countGetActions(client, "modelconfigs"))
}

func TestModelResolver_MissNotCached(t *testing.T) {
	t.Parallel()
	client := newFakeModelConfigClient()
	resolver := NewModelResolver(client)

	_, ok := resolver.ResolveForAgent(context.Background(), "kagent", "later")
	require.False(t, ok)

	// The ModelConfig appears after the first (missed) lookup.
	gvr := ModelConfigGVR
	_, err := client.Resource(gvr).Namespace("kagent").Create(context.Background(),
		modelConfigUnstructured("later", "kagent", "OpenAI", "gpt-4o"), metav1.CreateOptions{})
	require.NoError(t, err)

	model, ok := resolver.ResolveForAgent(context.Background(), "kagent", "later")
	require.True(t, ok, "a miss must not be cached — the now-present ModelConfig resolves")
	assert.Equal(t, "gpt-4o", model.Name)
}

func TestModelConfigNameFromAgent(t *testing.T) {
	t.Parallel()

	declarative := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"type": "Declarative",
				"declarative": map[string]interface{}{
					"modelConfig": "default-model-config",
				},
			},
		},
	}
	assert.Equal(t, "default-model-config", ModelConfigNameFromAgent(declarative))

	nonDeclarative := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{"type": "BYO"},
		},
	}
	assert.Equal(t, "", ModelConfigNameFromAgent(nonDeclarative))

	assert.Equal(t, "", ModelConfigNameFromAgent(nil))
}

// countGetActions counts get actions against the named resource on the fake
// client's action log.
func countGetActions(client *dynamicfake.FakeDynamicClient, resource string) int {
	n := 0
	for _, a := range client.Actions() {
		if a.GetVerb() == "get" && a.GetResource().Resource == resource {
			n++
		}
	}
	return n
}
