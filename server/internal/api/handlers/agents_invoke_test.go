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
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/websocket"
)

// fakeInvoker is a canned A2AInvoker that can also observe the store at call
// time (proving record-created-before-call ordering).
type fakeInvoker struct {
	mu     sync.Mutex
	calls  int
	result *kagent.A2AResult
	err    error
	// contextIDs records the contextID passed to each Invoke call (session
	// continuity — proves kagentContextId threading, AC #6).
	contextIDs []string
	// onInvoke runs inside Invoke (before returning) — used to assert the run
	// record already exists when kagent is hit.
	onInvoke func()
}

func (f *fakeInvoker) Invoke(_ context.Context, _, _, _, _, contextID string) (*kagent.A2AResult, error) {
	f.mu.Lock()
	f.calls++
	f.contextIDs = append(f.contextIDs, contextID)
	hook := f.onInvoke
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	return f.result, f.err
}

func (f *fakeInvoker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// firstContextID returns the contextID of the first Invoke call (the primary
// A2A invoke — the revision invoke, if any, always passes "").
func (f *fakeInvoker) firstContextID(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.contextIDs, "invoker was never called")
	return f.contextIDs[0]
}

// recordingBroadcaster captures broadcast lifecycle transitions.
type recordingBroadcaster struct {
	mu     sync.Mutex
	events []websocket.AgentRunUpdateData
}

func (b *recordingBroadcaster) BroadcastAgentRunUpdate(action websocket.Action, data websocket.AgentRunUpdateData) {
	b.mu.Lock()
	defer b.mu.Unlock()
	data.Action = action
	b.events = append(b.events, data)
}

func (b *recordingBroadcaster) snapshot() []websocket.AgentRunUpdateData {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]websocket.AgentRunUpdateData, len(b.events))
	copy(out, b.events)
	return out
}

// invokeHarness bundles the handler with its observable dependencies.
type invokeHarness struct {
	handler     *AgentsInvokeHandler
	store       *runs.RedisStore
	resultStore *runs.RedisResultStore
	invoker     *fakeInvoker
	broadcaster *recordingBroadcaster
	finished    chan string
}

func newInvokeHarness(t *testing.T, namespaces []string, invoker *fakeInvoker, objects ...runtime.Object) *invokeHarness {
	t.Helper()
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := runs.NewRedisStore(redisClient)
	resultStore := runs.NewRedisResultStore(redisClient)

	client := newFakeAgentsClient(t, objects...)

	broadcaster := &recordingBroadcaster{}
	handler := NewAgentsInvokeHandler(client, &stubNamespaces{namespaces: namespaces}, store, invoker, broadcaster, resultStore, nil)

	finished := make(chan string, 4)
	handler.onRunFinished = func(runID string) { finished <- runID }

	return &invokeHarness{handler: handler, store: store, resultStore: resultStore, invoker: invoker, broadcaster: broadcaster, finished: finished}
}

// getResult fetches the persisted terminal Result for a run id.
func (h *invokeHarness) getResult(t *testing.T, runID string) *runs.Result {
	t.Helper()
	got, err := h.resultStore.Get(context.Background(), runID)
	require.NoError(t, err)
	return got
}

func invokeRequestFor(t *testing.T, namespace, name, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/"+namespace+"/"+name+"/invoke", strings.NewReader(body))
	req.SetPathValue("namespace", namespace)
	req.SetPathValue("name", name)
	userCtx := &middleware.UserContext{
		UserID: "user-123",
		Email:  "dev@example.com",
	}
	return req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))
}

// waitRunFinished blocks until the background goroutine has fully completed.
func (h *invokeHarness) waitRunFinished(t *testing.T) string {
	t.Helper()
	select {
	case id := <-h.finished:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("background run completion did not finish in time")
		return ""
	}
}

func (h *invokeHarness) listRuns(t *testing.T) []runs.Run {
	t.Helper()
	got, err := h.store.List(context.Background(), runs.Filter{})
	require.NoError(t, err)
	return got
}

func TestAgentsInvokeHandler_SuccessLifecycle(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "ctx-789", Text: "scale to 3 replicas"}}
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
		agentUnstructured("helper", "alpha-apps", "helps"))

	// Record-created-before-call ordering: assert the record already exists —
	// with status running — at the moment the fake A2A endpoint is hit.
	var recordsAtCallTime []runs.Run
	invoker.onInvoke = func() {
		recordsAtCallTime = h.listRuns(t)
	}

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"what should I do?","contextRef":"rgd:webapp"}`))

	// 202 with the run record body.
	require.Equal(t, http.StatusAccepted, w.Code)
	var accepted runs.Run
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	assert.NotEmpty(t, accepted.ID)
	assert.Equal(t, "dev@example.com", accepted.Actor, "actor must be the authenticated user's email")
	assert.Equal(t, "helper", accepted.AgentType)
	assert.Equal(t, "alpha-apps", accepted.AgentNamespace)
	assert.Equal(t, "rgd:webapp", accepted.ContextRef)
	assert.Equal(t, "what should I do?", accepted.InputSummary)
	assert.Equal(t, runs.StatusRunning, accepted.Status)
	assert.Equal(t, runs.TriggerOnDemand, accepted.TriggerType)

	h.waitRunFinished(t)

	// The record existed (status running) BEFORE the A2A call.
	require.Len(t, recordsAtCallTime, 1)
	assert.Equal(t, runs.StatusRunning, recordsAtCallTime[0].Status)

	// Terminal state: completed, contextId captured as kagentSessionId.
	got := h.listRuns(t)
	require.Len(t, got, 1)
	assert.Equal(t, runs.StatusCompleted, got[0].Status)
	assert.Equal(t, "ctx-789", got[0].KagentSessionID)
	assert.Equal(t, "scale to 3 replicas", got[0].RecommendationSummary)
	require.NotNil(t, got[0].CompletedAt)

	// Broadcast lifecycle: add (running) then update (completed), actorId =
	// UserID for hub-side filtering, actor = email for display.
	events := h.broadcaster.snapshot()
	require.Len(t, events, 2)
	assert.Equal(t, websocket.ActionAdd, events[0].Action)
	assert.Equal(t, runs.StatusRunning, events[0].Status)
	assert.Equal(t, websocket.ActionUpdate, events[1].Action)
	assert.Equal(t, runs.StatusCompleted, events[1].Status)
	for _, e := range events {
		assert.Equal(t, "user-123", e.ActorID)
		assert.Equal(t, "dev@example.com", e.Actor)
		assert.Equal(t, accepted.ID, e.RunID)
	}
}

func TestAgentsInvokeHandler_FailurePath(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{err: errors.New("agent exploded")}
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
		agentUnstructured("helper", "alpha-apps", "helps"))

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))
	require.Equal(t, http.StatusAccepted, w.Code)

	h.waitRunFinished(t)

	got := h.listRuns(t)
	require.Len(t, got, 1)
	assert.Equal(t, runs.StatusFailed, got[0].Status)
	assert.Contains(t, got[0].RecommendationSummary, "agent exploded")
	require.NotNil(t, got[0].CompletedAt)

	events := h.broadcaster.snapshot()
	require.Len(t, events, 2)
	assert.Equal(t, runs.StatusFailed, events[1].Status)
}

func TestAgentsInvokeHandler_UnauthorizedNamespace_404_NoSideEffects(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "x"}}
	// The agent EXISTS in beta-apps, but the caller can only see alpha-apps.
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
		agentUnstructured("helper", "beta-apps", "helps"))

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "beta-apps", "helper", `{"message":"hi"}`))

	assert.Equal(t, http.StatusNotFound, w.Code, "denied namespace must be a 404 (existence non-leak)")
	assert.Empty(t, h.listRuns(t), "no run record may be written for a denied invoke")
	assert.Equal(t, 0, invoker.callCount(), "A2A must never be called for a denied invoke")
	assert.Empty(t, h.broadcaster.snapshot())
}

func TestAgentsInvokeHandler_MissingAgentCR_404(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{}
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker) // no agents seeded

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "ghost", `{"message":"hi"}`))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, h.listRuns(t))
	assert.Equal(t, 0, invoker.callCount())
}

func TestAgentsInvokeHandler_RolelessUser_EmptyNamespaces_404(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{}
	h := newInvokeHarness(t, []string{}, invoker,
		agentUnstructured("helper", "alpha-apps", "helps"))

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))

	assert.Equal(t, http.StatusNotFound, w.Code, "roleless user (empty namespace set) must fail closed with 404")
	assert.Equal(t, 0, invoker.callCount())
}

func TestAgentsInvokeHandler_NilAuthz_FailsClosed404(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	store := runs.NewRedisStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	client := newFakeAgentsClient(t, agentUnstructured("helper", "alpha-apps", ""))
	handler := NewAgentsInvokeHandler(client, nil, store, &fakeInvoker{}, nil, nil, nil)

	w := httptest.NewRecorder()
	handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentsInvokeHandler_MissingUserContext_401(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{}
	h := newInvokeHarness(t, []string{"*"}, invoker)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/alpha-apps/helper/invoke", strings.NewReader(`{"message":"hi"}`))
	req.SetPathValue("namespace", "alpha-apps")
	req.SetPathValue("name", "helper")
	w := httptest.NewRecorder()
	h.handler.Invoke(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAgentsInvokeHandler_MessageValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
	}{
		{"invalid JSON", `{not json`},
		{"missing message", `{}`},
		{"empty message", `{"message":""}`},
		{"control chars only", "{\"message\":\"\\u0007\\u0000\"}"},
		{"message too long", `{"message":"` + strings.Repeat("a", 8193) + `"}`},
		{"contextRef too long", `{"message":"hi","contextRef":"` + strings.Repeat("c", 513) + `"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			invoker := &fakeInvoker{}
			h := newInvokeHarness(t, []string{"*"}, invoker,
				agentUnstructured("helper", "alpha-apps", ""))

			w := httptest.NewRecorder()
			h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", tc.body))

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Equal(t, 0, invoker.callCount())
			assert.Empty(t, h.listRuns(t))
		})
	}
}

func TestAgentsInvokeHandler_NilStore_503(t *testing.T) {
	t.Parallel()

	client := newFakeAgentsClient(t, agentUnstructured("helper", "alpha-apps", ""))
	handler := NewAgentsInvokeHandler(client, &stubNamespaces{namespaces: []string{"*"}}, nil, &fakeInvoker{}, nil, nil, nil)

	w := httptest.NewRecorder()
	handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var errResp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "SERVICE_UNAVAILABLE", errResp["code"])
}

func TestAgentsInvokeHandler_StoreCreateFails_503_NoA2ACall(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := runs.NewRedisStore(redisClient)
	client := newFakeAgentsClient(t, agentUnstructured("helper", "alpha-apps", ""))
	invoker := &fakeInvoker{}
	handler := NewAgentsInvokeHandler(client, &stubNamespaces{namespaces: []string{"*"}}, store, invoker, nil, nil, nil)

	// Kill Redis so Create fails.
	mr.Close()

	w := httptest.NewRecorder()
	handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, 0, invoker.callCount(), "A2A must not be called when the record cannot be written")
}

func TestAgentsInvokeHandler_ActorFallsBackToUserID(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "c"}}
	h := newInvokeHarness(t, []string{"*"}, invoker,
		agentUnstructured("helper", "alpha-apps", ""))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/alpha-apps/helper/invoke", strings.NewReader(`{"message":"hi"}`))
	req.SetPathValue("namespace", "alpha-apps")
	req.SetPathValue("name", "helper")
	userCtx := &middleware.UserContext{UserID: "sub-only-user"} // no email
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, userCtx))

	w := httptest.NewRecorder()
	h.handler.Invoke(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	var accepted runs.Run
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	assert.Equal(t, "sub-only-user", accepted.Actor)

	h.waitRunFinished(t)
}

func TestAgentsInvokeHandler_WildcardAdmin_Allowed(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "c"}}
	h := newInvokeHarness(t, []string{"*"}, invoker,
		agentUnstructured("helper", "beta-apps", ""))

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "beta-apps", "helper", `{"message":"hi"}`))
	assert.Equal(t, http.StatusAccepted, w.Code)
	h.waitRunFinished(t)
}

// TestAgentsInvokeHandler_PersistsConversationID proves the Story 50.6 grouping
// key flows from the BYOA invoke body onto the run record.
func TestAgentsInvokeHandler_PersistsConversationID(t *testing.T) {
	t.Parallel()
	invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "ctx", Text: "ok"}}
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
		agentUnstructured("helper", "alpha-apps", "helps"))

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi","conversationId":"conv-byoa"}`))

	require.Equal(t, http.StatusAccepted, w.Code)
	var accepted runs.Run
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	assert.Equal(t, "conv-byoa", accepted.ConversationID)

	h.waitRunFinished(t)
	got := h.listRuns(t)
	require.Len(t, got, 1)
	assert.Equal(t, "conv-byoa", got[0].ConversationID)
}

// stubInvokeValidator answers a canned validation for each ValidateSpec call,
// counting calls. statuses is consumed in order; the last entry repeats once
// exhausted.
type stubInvokeValidator struct {
	mu       sync.Mutex
	calls    int
	statuses []string
}

func (s *stubInvokeValidator) ValidateSpec(_ context.Context, _ string) *runs.PolicyValidation {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.calls
	if idx >= len(s.statuses) {
		idx = len(s.statuses) - 1
	}
	status := s.statuses[idx]
	s.calls++
	pv := &runs.PolicyValidation{Status: status}
	if status == runs.PolicyStatusFailed {
		pv.Violations = []runs.PolicyViolation{{Constraint: "no-privileged", Message: "privileged not allowed"}}
	}
	return pv
}

func (s *stubInvokeValidator) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// TestAgentsInvokeHandler_WritesFetchableResult proves Story 53.5 AC #1/#3/#4:
// a completed BYOA run persists a full, fetchable Result (backfilled response,
// session id, tokens) and — with a validator wired — the policy outcome.
func TestAgentsInvokeHandler_WritesFetchableResult(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{result: &kagent.A2AResult{
		ContextID: "ctx-789", Text: "scale to 3 replicas",
		InputRequired: true, InputTokens: 11, OutputTokens: 22, TotalTokens: 33,
	}}
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
		agentUnstructured("helper", "alpha-apps", "helps"))
	h.handler.specValidator = &stubInvokeValidator{statuses: []string{runs.PolicyStatusPassed}}

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"what should I do?"}`))
	require.Equal(t, http.StatusAccepted, w.Code)
	var accepted runs.Run
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))

	h.waitRunFinished(t)

	result := h.getResult(t, accepted.ID)
	require.NotNil(t, result, "completed BYOA run must write a fetchable Result")
	assert.Equal(t, runs.StatusCompleted, result.Status)
	assert.Equal(t, "scale to 3 replicas", result.Response)
	assert.Equal(t, "ctx-789", result.KagentSessionID)
	assert.True(t, result.InputRequired)
	assert.Equal(t, 11, result.InputTokens)
	assert.Equal(t, 22, result.OutputTokens)
	assert.Equal(t, 33, result.TotalTokens)
	require.NotNil(t, result.PolicyValidation, "wired validator's outcome must land on the result")
	assert.Equal(t, runs.PolicyStatusPassed, result.PolicyValidation.Status)
}

// TestAgentsInvokeHandler_OSSNoValidator proves AC #5: with no validator
// (OSS) the result carries no policyValidation, and a failed run still writes
// a fetchable Result with status=failed/error.
func TestAgentsInvokeHandler_OSSNoValidator(t *testing.T) {
	t.Parallel()

	t.Run("completed run, nil PolicyValidation", func(t *testing.T) {
		t.Parallel()
		invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "c", Text: "ok"}}
		h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
			agentUnstructured("helper", "alpha-apps", "helps")) // nil validator

		w := httptest.NewRecorder()
		h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))
		require.Equal(t, http.StatusAccepted, w.Code)
		var accepted runs.Run
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))

		h.waitRunFinished(t)
		result := h.getResult(t, accepted.ID)
		require.NotNil(t, result)
		assert.Equal(t, runs.StatusCompleted, result.Status)
		assert.Nil(t, result.PolicyValidation, "OSS build (no validator) must produce no policyValidation")
	})

	t.Run("failed run writes status=failed/error", func(t *testing.T) {
		t.Parallel()
		invoker := &fakeInvoker{err: errors.New("agent exploded")}
		h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
			agentUnstructured("helper", "alpha-apps", "helps"))

		w := httptest.NewRecorder()
		h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))
		require.Equal(t, http.StatusAccepted, w.Code)
		var accepted runs.Run
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))

		h.waitRunFinished(t)
		result := h.getResult(t, accepted.ID)
		require.NotNil(t, result, "a failed run must still write a fetchable Result")
		assert.Equal(t, runs.StatusFailed, result.Status)
		assert.Contains(t, result.Error, "agent exploded")
		assert.Nil(t, result.PolicyValidation)
	})
}

// TestAgentsInvokeHandler_RevisionFlow proves AC #4: a failed validation
// triggers exactly ONE revision invoke and a single re-validation, landing
// RevisedStatus; the agent is invoked exactly twice (primary + revision).
func TestAgentsInvokeHandler_RevisionFlow(t *testing.T) {
	t.Parallel()

	invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "c", Text: "```yaml\nkind: ResourceGraphDefinition\n```"}}
	validator := &stubInvokeValidator{statuses: []string{runs.PolicyStatusFailed, runs.PolicyStatusPassed}}
	h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
		agentUnstructured("helper", "alpha-apps", "helps"))
	h.handler.specValidator = validator

	w := httptest.NewRecorder()
	h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"deploy a web app"}`))
	require.Equal(t, http.StatusAccepted, w.Code)
	var accepted runs.Run
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))

	h.waitRunFinished(t)

	result := h.getResult(t, accepted.ID)
	require.NotNil(t, result)
	require.NotNil(t, result.PolicyValidation)
	assert.Equal(t, runs.PolicyStatusFailed, result.PolicyValidation.Status)
	assert.Equal(t, runs.PolicyStatusPassed, result.PolicyValidation.RevisedStatus,
		"the re-validated revision must record RevisedStatus")
	assert.Equal(t, 2, invoker.callCount(), "exactly one revision invoke beyond the primary")
	assert.Equal(t, 2, validator.callCount(), "validate once, then re-validate the revision once")
}

// TestAgentsInvokeHandler_ThreadsKagentContextID proves AC #6: a
// kagentContextId in the request body reaches the A2A Invoke call (session
// continuity), and its absence yields an empty contextID. Guards against a
// regression to the previously hard-coded "" that silently broke multi-turn
// chat.
func TestAgentsInvokeHandler_ThreadsKagentContextID(t *testing.T) {
	t.Parallel()

	t.Run("supplied contextId reaches the A2A call", func(t *testing.T) {
		t.Parallel()
		invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "c", Text: "ok"}}
		h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
			agentUnstructured("helper", "alpha-apps", "helps"))

		w := httptest.NewRecorder()
		h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper",
			`{"message":"continue","kagentContextId":"sess-42"}`))
		require.Equal(t, http.StatusAccepted, w.Code)
		h.waitRunFinished(t)

		assert.Equal(t, "sess-42", invoker.firstContextID(t),
			"the request's kagentContextId must thread through to the A2A Invoke call")
	})

	t.Run("absent contextId yields empty", func(t *testing.T) {
		t.Parallel()
		invoker := &fakeInvoker{result: &kagent.A2AResult{ContextID: "c", Text: "ok"}}
		h := newInvokeHarness(t, []string{"alpha-apps"}, invoker,
			agentUnstructured("helper", "alpha-apps", "helps"))

		w := httptest.NewRecorder()
		h.handler.Invoke(w, invokeRequestFor(t, "alpha-apps", "helper", `{"message":"hi"}`))
		require.Equal(t, http.StatusAccepted, w.Code)
		h.waitRunFinished(t)

		assert.Empty(t, invoker.firstContextID(t),
			"no kagentContextId in the body must pass an empty contextID")
	})
}
