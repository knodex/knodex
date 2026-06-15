// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knodex/knodex/server/internal/util/sanitize"
)

const (
	// resultKeyPrefix is the per-run full-response payload key:
	// agentruns:result:{runID}.
	resultKeyPrefix = "agentruns:result:"

	// resultTTL bounds a result's lifetime. Aligned with runTTL (7d) for
	// faithful session replay (Story 50.6): a session lists for as long as its
	// run records live, so its full responses must live equally long or replay
	// would degrade to the recommendationSummary fallback within the retention
	// window. Memory stays bounded by the maxEntries=1000 run cap; the AC #7
	// summary fallback still covers the evicted tail beyond that cap.
	resultTTL = 7 * 24 * time.Hour
)

// Result is the full terminal payload of an agent invocation (Story 50.1).
// The run record's recommendationSummary is capped at 1024 chars — far too
// small for an RGD spec — so the complete agent text lives here, keyed by run
// id. Deliberately NOT part of the Store interface: EE's audit decorator
// wraps Store (49.5) and must neither break nor pull full LLM output into
// audit scope.
type Result struct {
	RunID string `json:"runId"`
	// AgentNamespace mirrors the run record's namespace ("" for built-ins) so
	// the result endpoint can apply the same visibility filter without a
	// second store read.
	AgentNamespace string `json:"agentNamespace"`
	// Status is the terminal run status: StatusCompleted or StatusFailed.
	Status string `json:"status"`
	// Response is the full agent text (completed runs).
	Response string `json:"response"`
	// Error is the sanitized failure description (failed runs).
	Error       string    `json:"error"`
	CompletedAt time.Time `json:"completedAt"`
	// PolicyValidation is the Gatekeeper policy-validation outcome (Story
	// 50.3). Nil on OSS builds and unlicensed EE builds — the field is a
	// plain data shape here; behavior lives behind the EE validator seam.
	PolicyValidation *PolicyValidation `json:"policyValidation,omitempty"`
	// InputRequired is true when the agent entered the A2A "input-required"
	// state: it asked clarifying questions (in Response) and is waiting for
	// the user to provide answers before generating a final spec. The next
	// turn's message should contain the user's answers.
	InputRequired bool `json:"inputRequired,omitempty"`
	// DataParts holds structured (kind="data") parts from the A2A response.
	// Non-nil when InputRequired is true and the agent provided structured
	// question forms (type="questions"). Each element is a raw JSON value
	// whose schema is defined by the agent — today the web parses the
	// "questions" type to render option chips.
	DataParts []json.RawMessage `json:"dataParts,omitempty"`
	// KagentSessionID is the A2A contextId returned by kagent. The client
	// MUST pass this as kagentContextId on the next invoke request so kagent
	// resumes the same session and the agent retains memory of prior turns.
	KagentSessionID string `json:"kagentSessionId,omitempty"`
	// InputTokens, OutputTokens, TotalTokens are the LLM token counts
	// reported by kagent (result.metadata["kagent_usage_metadata"]).
	// Zero when the controller did not report usage.
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
}

// ResultStore persists full invocation results, keyed by run id. Get returns
// (nil, nil) when no result exists yet — the handler's 404 (in-flight) path.
type ResultStore interface {
	Put(ctx context.Context, result *Result) error
	Get(ctx context.Context, runID string) (*Result, error)
}

// RedisResultStore implements ResultStore on Redis with per-result JSON
// payload keys and a fixed TTL. No index: results are only ever fetched by
// run id.
type RedisResultStore struct {
	client *redis.Client
}

// NewRedisResultStore creates a Redis-backed result store.
func NewRedisResultStore(client *redis.Client) *RedisResultStore {
	return &RedisResultStore{client: client}
}

// resultKey returns the Redis key for a run's result payload.
func resultKey(runID string) string {
	return resultKeyPrefix + runID
}

// removeControlCharsKeepNewlines strips non-printable control characters but
// preserves newlines and tabs — the Response field is multi-line YAML and
// RemoveControlChars would flatten it (breaking the web's fenced-block
// extraction). Terminal-escape smuggling (ESC, NUL, BEL, ...) is still
// removed.
func removeControlCharsKeepNewlines(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	for _, r := range s {
		if r >= 32 && r != 127 || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// sanitizeResult strips control characters from every stored string field —
// the same defense as sanitizeRun: agent output is attacker-influenced and
// must not smuggle terminal escapes into stored records. Response and Error
// keep newlines/tabs (multi-line agent output); the id-like fields do not.
func sanitizeResult(result *Result) {
	result.RunID = sanitize.RemoveControlChars(result.RunID)
	result.AgentNamespace = sanitize.RemoveControlChars(result.AgentNamespace)
	result.Status = sanitize.RemoveControlChars(result.Status)
	result.Response = removeControlCharsKeepNewlines(result.Response)
	result.Error = removeControlCharsKeepNewlines(result.Error)
	sanitizePolicyValidation(result.PolicyValidation)
}

// sanitizePolicyValidation strips control characters from the Story 50.3
// policy-validation fields. Constraint names and messages come from cluster
// objects / the Gatekeeper webhook (untrusted) — strict sanitization; only
// RevisedResponse is multi-line agent YAML and keeps newlines/tabs.
func sanitizePolicyValidation(pv *PolicyValidation) {
	if pv == nil {
		return
	}
	pv.Status = sanitize.RemoveControlChars(pv.Status)
	pv.Reason = sanitize.RemoveControlChars(pv.Reason)
	pv.RevisedResponse = removeControlCharsKeepNewlines(pv.RevisedResponse)
	pv.RevisedStatus = sanitize.RemoveControlChars(pv.RevisedStatus)
	sanitizePolicyViolations(pv.Violations)
	sanitizePolicyViolations(pv.RevisedViolations)
}

// sanitizePolicyViolations strictly sanitizes every violation string in
// place — all fields are cluster-sourced and none is legitimately multi-line.
func sanitizePolicyViolations(violations []PolicyViolation) {
	for i := range violations {
		v := &violations[i]
		v.Constraint = sanitize.RemoveControlChars(v.Constraint)
		v.ConstraintKind = sanitize.RemoveControlChars(v.ConstraintKind)
		v.EnforcementAction = sanitize.RemoveControlChars(v.EnforcementAction)
		v.Message = sanitize.RemoveControlChars(v.Message)
		v.ResourceID = sanitize.RemoveControlChars(v.ResourceID)
	}
}

// Put persists a result under agentruns:result:{runID} with resultTTL.
func (s *RedisResultStore) Put(ctx context.Context, result *Result) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("agent run result store: redis client unavailable")
	}
	if result == nil || result.RunID == "" {
		return fmt.Errorf("agent run result store: result with non-empty RunID required")
	}

	sanitizeResult(result)
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal agent run result: %w", err)
	}
	if err := s.client.Set(ctx, resultKey(result.RunID), payload, resultTTL).Err(); err != nil {
		return fmt.Errorf("put agent run result: %w", err)
	}
	return nil
}

// Get fetches the result for runID. A missing key returns (nil, nil) — the
// caller maps that to 404 (in-flight or unknown, indistinguishable by
// design).
func (s *RedisResultStore) Get(ctx context.Context, runID string) (*Result, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("agent run result store: redis client unavailable")
	}
	raw, err := s.client.Get(ctx, resultKey(runID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent run result: %w", err)
	}
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse agent run result: %w", err)
	}
	return &result, nil
}
