// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"github.com/knodex/knodex/server/internal/api/middleware"
	"github.com/knodex/knodex/server/internal/api/response"
	"github.com/knodex/knodex/server/internal/kagent"
	"github.com/knodex/knodex/server/internal/kagent/rgdspec"
	"github.com/knodex/knodex/server/internal/kagent/runs"
	"github.com/knodex/knodex/server/internal/rbac"
	"github.com/knodex/knodex/server/internal/util/sanitize"
	"github.com/knodex/knodex/server/internal/websocket"
)

const (
	// maxInvokeMessageChars bounds the user message length (after control-char
	// sanitization) so a single invoke cannot smuggle megabytes into Redis.
	maxInvokeMessageChars = 8192
	// maxContextRefChars bounds the optional context reference.
	maxContextRefChars = 512
	// maxConversationIDChars bounds the optional Story 50.6 conversation
	// grouping key (generous for a UUID).
	maxConversationIDChars = 128
	// maxKagentContextIDChars bounds the optional session-continuation id (a
	// kagent contextId — generous for a UUID).
	maxKagentContextIDChars = 128
	// maxInputSummaryChars is how much of the message lands on the run record.
	maxInputSummaryChars = 256
	// maxRecommendationChars is how much of the agent's answer (or the error
	// summary on failure) lands on the run record.
	maxRecommendationChars = 1024

	// invokeCallTimeout bounds the background A2A call. Matches the A2A
	// client's own 120s http timeout (LLM latency) with headroom.
	invokeCallTimeout = 125 * time.Second
	// storeUpdateTimeout is the short fresh context for the completing
	// store.Update — independent of the A2A call's context.
	storeUpdateTimeout = 5 * time.Second
	// specValidationTimeout bounds one ValidateSpec call (Story 50.3). A fresh
	// context independent of the (possibly exhausted) A2A call context — sized
	// for the Gatekeeper webhook round-trip.
	specValidationTimeout = 30 * time.Second
)

// A2AInvoker is the narrow invocation seam this handler needs from the
// kagent A2A client (one-method interface — 49.1 review lesson).
// contextID, when non-empty, resumes an existing kagent session so the
// agent retains memory of prior turns (A2A configuration.contextId).
type A2AInvoker interface {
	Invoke(ctx context.Context, namespace, name, message, actor, contextID string) (*kagent.A2AResult, error)
}

// RunBroadcaster is the narrow slice of websocket.Hub this handler needs to
// push run lifecycle transitions. Nil-safe: a nil broadcaster skips pushes.
type RunBroadcaster interface {
	BroadcastAgentRunUpdate(action websocket.Action, data websocket.AgentRunUpdateData)
}

// invokeRequest is the POST body for the invoke endpoint.
type invokeRequest struct {
	Message    string `json:"message"`
	ContextRef string `json:"contextRef"`
	// ConversationID groups a chat's turns into one session (Story 50.6);
	// optional and additive — empty for callers that don't supply it.
	ConversationID string `json:"conversationId"`
	// KagentContextID, when non-empty, resumes an existing kagent session so
	// the agent retains memory of prior turns. The caller obtains this value
	// from a prior run result's kagentSessionId field.
	KagentContextID string `json:"kagentContextId"`
}

// AgentsInvokeHandler serves POST /api/v1/agents/{namespace}/{name}/invoke
// (Story 49.4). Knodex OWNS the run record: it is written (status running)
// BEFORE the kagent A2A call and updated when the response returns. The
// browser only ever talks to this server — the kagent base URL, cluster
// credentials, and LLM keys never reach the frontend (NFR-A4/A7).
//
// Authorization is the Casbin-derived accessible-namespace set applied inside
// the handler (single enforcement layer, NFR-A3). Denied namespace and
// nonexistent agent both yield 404 — existence non-leak, mirroring the
// unauthorized instance-delete posture.
type AgentsInvokeHandler struct {
	dynamicClient dynamic.Interface
	authz         AccessibleNamespacesProvider
	store         runs.Store
	resultStore   runs.ResultStore
	invoker       A2AInvoker
	broadcaster   RunBroadcaster
	// specValidator is the Story 50.3 EE policy-validation seam, re-homed onto
	// this surviving invoke path by Story 53.5. Nil-safe: OSS builds (and EE
	// builds whose validator failed to construct) pass nil and the result
	// simply carries no policyValidation.
	specValidator AgentSpecValidator

	// now is the clock, overridable in tests.
	now func() time.Time
	// onRunFinished is a test synchronization hook invoked after the
	// background goroutine has persisted the terminal store.Update and
	// broadcast it. Nil in production.
	onRunFinished func(runID string)
}

// NewAgentsInvokeHandler creates an AgentsInvokeHandler. authz and
// broadcaster may be nil (fail-closed / skip respectively); a nil store
// yields 503, a nil invoker fails runs immediately. A nil resultStore means
// the full terminal Result is never written, so GET .../result 404s forever
// (the chat polling contract breaks); a nil specValidator skips policy
// validation (the OSS path — the result carries no policyValidation).
func NewAgentsInvokeHandler(dynamicClient dynamic.Interface, authz AccessibleNamespacesProvider, store runs.Store, invoker A2AInvoker, broadcaster RunBroadcaster, resultStore runs.ResultStore, specValidator AgentSpecValidator) *AgentsInvokeHandler {
	return &AgentsInvokeHandler{
		dynamicClient: dynamicClient,
		authz:         authz,
		store:         store,
		resultStore:   resultStore,
		invoker:       invoker,
		broadcaster:   broadcaster,
		specValidator: specValidator,
		now:           time.Now,
	}
}

// truncateRunes cuts s to at most n runes without splitting a UTF-8 sequence.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// broadcast pushes a run lifecycle transition when a broadcaster is wired.
// actorID is the UserContext.UserID — the hub filters delivery on it.
func (h *AgentsInvokeHandler) broadcast(action websocket.Action, run *runs.Run, actorID string) {
	if h.broadcaster == nil {
		return
	}
	h.broadcaster.BroadcastAgentRunUpdate(action, websocket.AgentRunUpdateData{
		RunID:          run.ID,
		AgentType:      run.AgentType,
		AgentNamespace: run.AgentNamespace,
		Status:         run.Status,
		ActorID:        actorID,
		Actor:          run.Actor,
		Timestamp:      run.Timestamp,
	})
}

// Invoke handles POST /api/v1/agents/{namespace}/{name}/invoke.
// Lifecycle: validate → authz (404 non-leak) → store.Create(running) →
// broadcast add → 202 with the run record → background A2A call →
// store.Update(completed|failed) → broadcast update.
func (h *AgentsInvokeHandler) Invoke(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserContext(r)
	if !ok || userCtx == nil {
		response.Unauthorized(w, "authentication required")
		return
	}

	namespace := r.PathValue("namespace")
	name := r.PathValue("name")

	// Shared body decode/validation — writes its own 400s. kagentContextID,
	// when non-empty, resumes a prior kagent session (multi-turn chat); it was
	// previously decoded then discarded, breaking session continuity (53.5).
	message, contextRef, conversationID, kagentContextID, ok := decodeInvokeBody(w, r)
	if !ok {
		return
	}

	// Authz: Casbin-derived accessible namespaces, fail-closed. Denial is a
	// 404 (existence non-leak) — never a 403 that confirms the agent exists.
	userNamespaces := []string{}
	if h.authz != nil {
		var err error
		userNamespaces, err = h.authz.GetAccessibleNamespaces(r.Context(), userCtx)
		if err != nil {
			response.InternalError(w, "Failed to get user namespaces")
			return
		}
	}
	if !rbac.MatchNamespaceInList(namespace, userNamespaces) {
		response.NotFound(w, "agent", namespace+"/"+name)
		return
	}

	if h.dynamicClient == nil {
		response.InternalError(w, "kubernetes client unavailable")
		return
	}

	// Existence check: an absent Agent CR yields the SAME 404 as a denied
	// namespace.
	if _, err := h.dynamicClient.Resource(agentsGVR).Namespace(namespace).Get(r.Context(), name, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			response.NotFound(w, "agent", namespace+"/"+name)
			return
		}
		response.InternalError(w, "Failed to get agent")
		return
	}

	// The run store is the system of record — without it no invocation is
	// auditable, so refuse rather than call kagent untracked. Checked AFTER
	// authz so a denied caller always sees the non-leaking 404, never a 503.
	if h.store == nil {
		response.WriteError(w, http.StatusServiceUnavailable, response.ErrCodeServiceUnavailable,
			"agent run store unavailable", nil)
		return
	}

	actor := userCtx.Email
	if actor == "" {
		actor = userCtx.UserID
	}
	actorID := userCtx.UserID

	run := &runs.Run{
		ID:             uuid.NewString(),
		Actor:          actor,
		AgentType:      name,
		AgentNamespace: namespace,
		ContextRef:     contextRef,
		ConversationID: conversationID,
		InputSummary:   truncateRunes(message, maxInputSummaryChars),
		Timestamp:      h.now().UTC(),
		Status:         runs.StatusRunning,
		TriggerType:    runs.TriggerOnDemand,
	}

	// The record MUST exist before kagent is called (AC #4) — if it cannot be
	// written, do NOT invoke the agent.
	if err := h.store.Create(r.Context(), run); err != nil {
		response.WriteError(w, http.StatusServiceUnavailable, response.ErrCodeServiceUnavailable,
			"failed to record agent run", nil)
		return
	}
	h.broadcast(websocket.ActionAdd, run, actorID)

	// Complete the call in the background. The goroutine MUST NOT inherit
	// r.Context() — it is canceled the moment the 202 is written, and a
	// canceled context would mark every run failed (the 49.1 cache-poisoning
	// lesson transposed).
	runCopy := *run
	go h.completeRun(runCopy, message, kagentContextID, actorID)

	response.WriteJSON(w, http.StatusAccepted, run)
}

// completeRun performs the A2A call and persists the terminal state in the
// LOCKED order: (1) resultStore.Put full Result → (2) store.Update terminal
// run → (3) WS broadcast. The result MUST be readable before any terminal
// signal fires, or the re-pointed chat (53.2) races a 404 after seeing
// "completed". kagentContextID resumes a prior kagent session (multi-turn).
func (h *AgentsInvokeHandler) completeRun(run runs.Run, message, kagentContextID, actorID string) {
	if h.onRunFinished != nil {
		defer h.onRunFinished(run.ID)
	}

	callCtx, cancel := context.WithTimeout(context.Background(), invokeCallTimeout)
	defer cancel()

	var a2aResult *kagent.A2AResult
	var err error = errNoInvoker
	if h.invoker != nil {
		a2aResult, err = h.invoker.Invoke(callCtx, run.AgentNamespace, run.AgentType, message, run.Actor, kagentContextID)
	}

	completedAt := h.now().UTC()
	run.CompletedAt = &completedAt
	result := &runs.Result{
		RunID:          run.ID,
		AgentNamespace: run.AgentNamespace,
		CompletedAt:    completedAt,
	}
	if err != nil {
		run.Status = runs.StatusFailed
		run.RecommendationSummary = truncateRunes(sanitize.RemoveControlChars(err.Error()), maxRecommendationChars)
		result.Status = runs.StatusFailed
		result.Error = err.Error()
	} else {
		run.Status = runs.StatusCompleted
		run.KagentSessionID = a2aResult.ContextID
		logA2ANonTextParts(a2aResult, run.AgentType, "runID", run.ID)
		// Deterministic traceability (Story 50.2): verify/backfill
		// knodex.io/generated-from on every generated resource template from
		// the FULL user message (run.InputSummary is 256-char truncated and
		// must NOT be used). BackfillResponse is fail-soft — any problem
		// returns the text unchanged, never failing the run. The summary is
		// computed from the backfilled text so it stays consistent with the
		// fetchable result.
		backfilled := rgdspec.BackfillResponse(a2aResult.Text, message)
		run.RecommendationSummary = truncateRunes(sanitize.RemoveControlChars(backfilled), maxRecommendationChars)
		run.InputTokens = a2aResult.InputTokens
		run.OutputTokens = a2aResult.OutputTokens
		run.TotalTokens = a2aResult.TotalTokens
		result.Status = runs.StatusCompleted
		result.Response = backfilled
		result.InputRequired = a2aResult.InputRequired
		result.DataParts = a2aResult.DataParts
		result.KagentSessionID = a2aResult.ContextID
		result.InputTokens = a2aResult.InputTokens
		result.OutputTokens = a2aResult.OutputTokens
		result.TotalTokens = a2aResult.TotalTokens
		// Story 50.3: validate the backfilled spec against cluster policy
		// (EE-injected; nil on OSS) BEFORE the result is persisted. Fail-soft
		// throughout — a policy problem never flips a completed run to failed,
		// and the LOCKED Put → Update → WS order below is untouched.
		if h.specValidator != nil {
			result.PolicyValidation = h.validatePolicy(run.AgentNamespace, run.AgentType, message, backfilled, run.Actor)
		}
	}

	// Short fresh contexts for the persisting writes — independent of the
	// (possibly exhausted) A2A call context.
	if h.resultStore != nil {
		putCtx, cancelPut := context.WithTimeout(context.Background(), storeUpdateTimeout)
		// A failed Put must NOT abort the run-record update: the run history
		// must not dangle at "running". The page's hard timeout covers the
		// missing-result corner.
		_ = h.resultStore.Put(putCtx, result)
		cancelPut()
	}

	updateCtx, cancelUpdate := context.WithTimeout(context.Background(), storeUpdateTimeout)
	defer cancelUpdate()
	if updateErr := h.store.Update(updateCtx, &run); updateErr != nil {
		// The record stays "running" in the store; surfacing is best-effort.
		// The web's conditional refetch will show it stale rather than wrong.
		return
	}
	h.broadcast(websocket.ActionUpdate, &run, actorID)
}

// validatePolicy runs the Story 50.3 policy-validation flow on the backfilled
// response: validate once and, when the spec FAILS, make exactly ONE agent
// revision attempt (a self-contained prompt — the revision A2A call has no
// session continuity), backfill the revision with the ORIGINAL message and
// re-validate it exactly once. Every step is fail-soft: a revision invoke
// error returns the violations without revised fields; this function never
// fails the run. namespace/name target the same agent the run invoked.
func (h *AgentsInvokeHandler) validatePolicy(namespace, name, message, backfilled, actor string) *runs.PolicyValidation {
	ctx, cancel := context.WithTimeout(context.Background(), specValidationTimeout)
	defer cancel()
	pv := h.specValidator.ValidateSpec(ctx, backfilled)
	if pv == nil || pv.Status != runs.PolicyStatusFailed {
		// Nothing to validate / not licensed (nil), passed, or unavailable —
		// no revision attempt in any of these states.
		return pv
	}

	// ONE revision attempt against the same agent with a fresh full-size A2A
	// budget (the original callCtx may be exhausted).
	revCtx, cancelRev := context.WithTimeout(context.Background(), invokeCallTimeout)
	defer cancelRev()
	var revResult *kagent.A2AResult
	var err error = errNoInvoker
	if h.invoker != nil {
		revResult, err = h.invoker.Invoke(revCtx, namespace, name,
			buildRevisionMessage(message, backfilled, pv.Violations), actor, "")
	}
	if err != nil || revResult == nil {
		// Fail-soft: violations without a revised spec. The nil guard keeps a
		// misbehaving invoker from panicking the background goroutine.
		return pv
	}
	logA2ANonTextParts(revResult, name, "phase", "revision")

	// Same traceability guarantee as the primary response: backfill with the
	// ORIGINAL requirement, then re-validate ONCE. No further loop.
	rev := rgdspec.BackfillResponse(revResult.Text, message)
	pv.RevisedResponse = rev
	ctx2, cancel2 := context.WithTimeout(context.Background(), specValidationTimeout)
	defer cancel2()
	if pv2 := h.specValidator.ValidateSpec(ctx2, rev); pv2 != nil {
		pv.RevisedStatus = pv2.Status
		pv.RevisedViolations = pv2.Violations
	}
	return pv
}

// buildRevisionMessage composes the self-contained revision prompt: the
// original requirement, the failing spec block (extracted from the backfilled
// response) and the violations. The revision A2A call has NO contextId
// continuation — the agent remembers nothing, so everything rides in the
// message.
func buildRevisionMessage(message, backfilled string, violations []runs.PolicyViolation) string {
	block, _, _, ok := rgdspec.ExtractYAMLBlock(backfilled)
	if !ok {
		// Defensive: a failed validation implies a spec block existed, but
		// fall back to the whole response rather than send an empty block.
		block = backfilled
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You previously generated this ResourceGraphDefinition for the requirement: %q\n\n", message)
	b.WriteString("```yaml\n")
	b.WriteString(block)
	if !strings.HasSuffix(block, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n\nGatekeeper policy validation rejected it with these violations:\n")
	for _, v := range violations {
		action := v.EnforcementAction
		if action == "" {
			action = "deny"
		}
		fmt.Fprintf(&b, "- constraint %q (%s): %s\n", v.Constraint, action, v.Message)
	}
	b.WriteString("\nProduce a REVISED ResourceGraphDefinition that satisfies these policies while still meeting the original requirement. Follow all your standard rules; output exactly ONE fenced yaml block.")
	return b.String()
}

// errNoInvoker is the terminal error when no A2A invoker is configured.
var errNoInvoker = &noInvokerError{}

type noInvokerError struct{}

func (*noInvokerError) Error() string { return "agent invoker not configured" }

// decodeInvokeBody decodes and validates the invoke request body
// (49.4 limits: 1..8192 message / ≤512 contextRef chars after control-char
// sanitization; Story 50.6 adds an optional ≤128-char conversationId; the
// session-continuity fix adds an optional ≤128-char kagentContextId). It
// writes the 400 response itself and returns ok=false on any validation
// failure.
func decodeInvokeBody(w http.ResponseWriter, r *http.Request) (message, contextRef, conversationID, kagentContextID string, ok bool) {
	var req invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid JSON body", nil)
		return "", "", "", "", false
	}
	message = sanitize.RemoveControlChars(req.Message)
	if message == "" {
		response.BadRequest(w, "message is required", map[string]string{"field": "message"})
		return "", "", "", "", false
	}
	if len([]rune(message)) > maxInvokeMessageChars {
		response.BadRequest(w, "message exceeds maximum length", map[string]string{"field": "message"})
		return "", "", "", "", false
	}
	contextRef = sanitize.RemoveControlChars(req.ContextRef)
	if len([]rune(contextRef)) > maxContextRefChars {
		response.BadRequest(w, "contextRef exceeds maximum length", map[string]string{"field": "contextRef"})
		return "", "", "", "", false
	}
	conversationID = sanitize.RemoveControlChars(req.ConversationID)
	if len([]rune(conversationID)) > maxConversationIDChars {
		response.BadRequest(w, "conversationId exceeds maximum length", map[string]string{"field": "conversationId"})
		return "", "", "", "", false
	}
	kagentContextID = sanitize.RemoveControlChars(req.KagentContextID)
	if len([]rune(kagentContextID)) > maxKagentContextIDChars {
		response.BadRequest(w, "kagentContextId exceeds maximum length", map[string]string{"field": "kagentContextId"})
		return "", "", "", "", false
	}
	return message, contextRef, conversationID, kagentContextID, true
}

// logA2ANonTextParts surfaces A2A response parts the parser could not render
// as user-facing text: structured data parts (captured verbatim on the
// result) and any unhandled kinds (e.g. file parts). kagent does not emit
// these for today's agents, but if it ever does, this WARN is the trail that
// keeps them from silently vanishing the way a dropped text part once did
// (see kagent.extractParts). It never changes the run outcome.
func logA2ANonTextParts(result *kagent.A2AResult, agent string, contextAttrs ...any) {
	if result == nil || (len(result.DataParts) == 0 && result.UnhandledParts == 0) {
		return
	}
	attrs := append([]any{
		"agent", agent,
		"dataParts", len(result.DataParts),
		"unhandledParts", result.UnhandledParts,
	}, contextAttrs...)
	slog.Warn("kagent A2A response contained non-text parts not rendered to the user", attrs...)
}
