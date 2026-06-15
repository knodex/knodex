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
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/k8s/parser"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/kro"
)

const validRGDYAML = `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-stack
  annotations:
    knodex.io/description: "Generated web app stack"
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppStack
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          annotations:
            knodex.io/generated-from: "a web app"
`

// rgdCreateHarness bundles the handler with observable dependencies.
type rgdCreateHarness struct {
	handler  *RGDCreateHandler
	dynamic  *dynamicfake.FakeDynamicClient
	runStore *runs.RedisStore
	recorder *mockAuditRecorder
}

func newRGDCreateHarness(t *testing.T) *rgdCreateHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := runs.NewRedisStore(client)
	recorder := &mockAuditRecorder{}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	return &rgdCreateHarness{
		handler:  NewRGDCreateHandler(dyn, store, recorder, nil),
		dynamic:  dyn,
		runStore: store,
		recorder: recorder,
	}
}

func rgdCreateRequestFor(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rgds", strings.NewReader(body))
	userCtx := &middleware.UserContext{
		UserID: "user-123",
		Email:  "dev@example.com",
	}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

func rgdCreateBody(t *testing.T, req rgdCreateRequest) string {
	t.Helper()
	raw, err := json.Marshal(req)
	require.NoError(t, err)
	return string(raw)
}

func (h *rgdCreateHarness) getRGD(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	// RGDs are cluster-scoped — no .Namespace() call.
	got, err := h.dynamic.Resource(kro.RGDGVR()).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return got.Object
}

func TestRGDCreateHandler_MissingUserContext_401(t *testing.T) {
	t.Parallel()
	h := newRGDCreateHarness(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/rgds", strings.NewReader(`{"yaml":"x"}`))
	w := httptest.NewRecorder()
	h.handler.Create(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRGDCreateHandler_BadRequests_400(t *testing.T) {
	t.Parallel()

	deploymentYAML := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
	wrongVersionYAML := "apiVersion: kro.run/v2\nkind: ResourceGraphDefinition\nmetadata:\n  name: x\n"
	wrongKindYAML := "apiVersion: kro.run/v1alpha1\nkind: Instance\nmetadata:\n  name: x\n"

	cases := []struct {
		name string
		body string
	}{
		{"invalid JSON", `{not json`},
		{"empty yaml", `{"yaml":""}`},
		{"missing yaml", `{}`},
		{"invalid YAML payload", `{"yaml":"kind: [unclosed"}`},
		{"wrong kind", `{"yaml":` + jsonString(wrongKindYAML) + `}`},
		{"wrong apiVersion", `{"yaml":` + jsonString(wrongVersionYAML) + `}`},
		// THE SECURITY BOUNDARY: an arbitrary manifest must never reach the cluster.
		{"Deployment manifest rejected", `{"yaml":` + jsonString(deploymentYAML) + `}`},
		{"bad name", `{"name":"Bad_Name!","yaml":` + jsonString(validRGDYAML) + `}`},
		{"missing name everywhere", `{"yaml":"apiVersion: kro.run/v1alpha1\nkind: ResourceGraphDefinition\n"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newRGDCreateHarness(t)
			w := httptest.NewRecorder()
			h.handler.Create(w, rgdCreateRequestFor(t, tc.body))

			assert.Equal(t, http.StatusBadRequest, w.Code)
			// Nothing may have reached the cluster.
			list := h.dynamic.Actions()
			for _, action := range list {
				assert.NotEqual(t, "create", action.GetVerb(), "no create may reach the cluster on 400")
			}
		})
	}
}

func TestRGDCreateHandler_HappyPath_201(t *testing.T) {
	t.Parallel()
	h := newRGDCreateHarness(t)

	w := httptest.NewRecorder()
	h.handler.Create(w, rgdCreateRequestFor(t, rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML})))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp rgdCreateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "webapp-stack", resp.Name)
	assert.Equal(t, kro.RGDKind, resp.Kind)
	assert.Equal(t, "kro.run/v1alpha1", resp.APIVersion)

	// Object exists in the cluster (cluster-scoped) with the catalog gateway
	// ensured AND the caller's own annotations preserved.
	obj := h.getRGD(t, "webapp-stack")
	annotations := parser.GetStringOrDefault(obj, "", "metadata", "annotations", kro.CatalogAnnotation)
	assert.Equal(t, "true", annotations, "catalog gateway annotation must be ensured")
	desc := parser.GetStringOrDefault(obj, "", "metadata", "annotations", "knodex.io/description")
	assert.Equal(t, "Generated web app stack", desc, "caller annotations must be preserved")
	resources, err := parser.GetSlice(obj, "spec", "resources")
	require.NoError(t, err)
	assert.Len(t, resources, 1, "spec.resources must survive the round-trip")

	// Audit event recorded with the no-Spec rule respected.
	event := h.recorder.lastEvent()
	assert.Equal(t, "create", event.Action)
	assert.Equal(t, "rgds", event.Resource)
	assert.Equal(t, "webapp-stack", event.Name)
	assert.Equal(t, "user-123", event.UserID)
	assert.Equal(t, "dev@example.com", event.UserEmail)
	assert.Equal(t, "success", event.Result)
	assert.Equal(t, "rgd-builder", event.Details["generatedBy"])
	_, hasRunID := event.Details["runId"]
	assert.False(t, hasRunID, "runId must only appear in Details when the request carried one")
	for _, v := range event.Details {
		if s, ok := v.(string); ok {
			assert.NotContains(t, s, "kind: ResourceGraphDefinition", "the YAML body must never land in audit details")
		}
	}
}

func TestRGDCreateHandler_NameOverride(t *testing.T) {
	t.Parallel()
	h := newRGDCreateHarness(t)

	body := rgdCreateBody(t, rgdCreateRequest{Name: "renamed-stack", YAML: validRGDYAML})
	w := httptest.NewRecorder()
	h.handler.Create(w, rgdCreateRequestFor(t, body))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp rgdCreateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "renamed-stack", resp.Name)

	obj := h.getRGD(t, "renamed-stack")
	assert.Equal(t, "renamed-stack", parser.GetStringOrDefault(obj, "", "metadata", "name"))
}

func TestRGDCreateHandler_StatusDropped(t *testing.T) {
	t.Parallel()
	h := newRGDCreateHarness(t)

	withStatus := validRGDYAML + "status:\n  state: ACTIVE\n"
	w := httptest.NewRecorder()
	h.handler.Create(w, rgdCreateRequestFor(t, rgdCreateBody(t, rgdCreateRequest{YAML: withStatus})))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	obj := h.getRGD(t, "webapp-stack")
	_, hasStatus := obj["status"]
	assert.False(t, hasStatus, "caller-supplied status must be dropped")
}

func TestRGDCreateHandler_Duplicate_409(t *testing.T) {
	t.Parallel()
	h := newRGDCreateHarness(t)

	body := rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML})
	w1 := httptest.NewRecorder()
	h.handler.Create(w1, rgdCreateRequestFor(t, body))
	require.Equal(t, http.StatusCreated, w1.Code)

	w2 := httptest.NewRecorder()
	h.handler.Create(w2, rgdCreateRequestFor(t, body))
	assert.Equal(t, http.StatusConflict, w2.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &errResp))
	assert.Equal(t, "CONFLICT", errResp["code"])
}

// seedRun writes a run record for the actionTaken tests.
func (h *rgdCreateHarness) seedRun(t *testing.T, id, actor, actionTaken string) {
	t.Helper()
	run := &runs.Run{
		ID:          id,
		Actor:       actor,
		AgentType:   "rgd-builder",
		Timestamp:   time.Now().UTC(),
		Status:      runs.StatusCompleted,
		TriggerType: runs.TriggerOnDemand,
		ActionTaken: actionTaken,
	}
	require.NoError(t, h.runStore.Create(context.Background(), run))
}

func (h *rgdCreateHarness) findRun(t *testing.T, id string) *runs.Run {
	t.Helper()
	all, err := h.runStore.List(context.Background(), runs.Filter{})
	require.NoError(t, err)
	for i := range all {
		if all[i].ID == id {
			return &all[i]
		}
	}
	return nil
}

func TestRGDCreateHandler_ActionTaken(t *testing.T) {
	t.Parallel()

	t.Run("set on actor match", func(t *testing.T) {
		t.Parallel()
		h := newRGDCreateHarness(t)
		h.seedRun(t, "run-1", "dev@example.com", "")

		body := rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML, RunID: "run-1"})
		w := httptest.NewRecorder()
		h.handler.Create(w, rgdCreateRequestFor(t, body))
		require.Equal(t, http.StatusCreated, w.Code)

		run := h.findRun(t, "run-1")
		require.NotNil(t, run)
		assert.Equal(t, "rgd-created: webapp-stack", run.ActionTaken)
	})

	t.Run("set on UserID actor match", func(t *testing.T) {
		t.Parallel()
		h := newRGDCreateHarness(t)
		h.seedRun(t, "run-uid", "user-123", "")

		body := rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML, RunID: "run-uid"})
		w := httptest.NewRecorder()
		h.handler.Create(w, rgdCreateRequestFor(t, body))
		require.Equal(t, http.StatusCreated, w.Code)

		run := h.findRun(t, "run-uid")
		require.NotNil(t, run)
		assert.Equal(t, "rgd-created: webapp-stack", run.ActionTaken)
	})

	t.Run("NOT set on actor mismatch", func(t *testing.T) {
		t.Parallel()
		h := newRGDCreateHarness(t)
		h.seedRun(t, "run-2", "someone-else@example.com", "")

		body := rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML, RunID: "run-2"})
		w := httptest.NewRecorder()
		h.handler.Create(w, rgdCreateRequestFor(t, body))
		require.Equal(t, http.StatusCreated, w.Code, "actor mismatch must never fail the create")

		run := h.findRun(t, "run-2")
		require.NotNil(t, run)
		assert.Empty(t, run.ActionTaken, "another actor's run must not be stamped")
	})

	t.Run("NOT set for unknown run", func(t *testing.T) {
		t.Parallel()
		h := newRGDCreateHarness(t)

		body := rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML, RunID: "ghost"})
		w := httptest.NewRecorder()
		h.handler.Create(w, rgdCreateRequestFor(t, body))
		assert.Equal(t, http.StatusCreated, w.Code, "unknown run must never fail the create")
	})

	t.Run("NOT overwritten when already populated", func(t *testing.T) {
		t.Parallel()
		h := newRGDCreateHarness(t)
		h.seedRun(t, "run-3", "dev@example.com", "rgd-created: default/earlier")

		body := rgdCreateBody(t, rgdCreateRequest{YAML: validRGDYAML, RunID: "run-3"})
		w := httptest.NewRecorder()
		h.handler.Create(w, rgdCreateRequestFor(t, body))
		require.Equal(t, http.StatusCreated, w.Code)

		run := h.findRun(t, "run-3")
		require.NotNil(t, run)
		assert.Equal(t, "rgd-created: default/earlier", run.ActionTaken, "fill-only-if-empty")
	})
}

func TestRGDCreateHandler_NilRunStoreAndRecorder_Safe(t *testing.T) {
	t.Parallel()
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	handler := NewRGDCreateHandler(dyn, nil, nil, nil)

	body := `{"yaml":` + jsonString(validRGDYAML) + `,"runId":"run-x"}`
	w := httptest.NewRecorder()
	handler.Create(w, rgdCreateRequestFor(t, body))

	assert.Equal(t, http.StatusCreated, w.Code, "nil runStore/recorder must be safe (OSS posture)")
}

func TestRGDCreateHandler_NilDynamicClient_503(t *testing.T) {
	t.Parallel()
	handler := NewRGDCreateHandler(nil, nil, nil, nil)

	w := httptest.NewRecorder()
	handler.Create(w, rgdCreateRequestFor(t, `{"yaml":`+jsonString(validRGDYAML)+`}`))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// jsonString JSON-encodes s for embedding in a request body literal.
func jsonString(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
