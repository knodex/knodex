// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package runs owns the Knodex-side record of agent invocations (Story 49.4).
// There is NO AgentRun CRD in kagent — Knodex writes a run record at
// invocation time (status "running") and updates it when the A2A response
// returns. kagent's session store is never read back; the authoritative
// actor identity comes from the Knodex auth context.
package runs

import "time"

// Run statuses. The A2A call is synchronous request/response, so the
// lifecycle is exactly running → completed | failed — no intermediate states.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// TriggerOnDemand is the only trigger type in 49.4. The field exists now so
// future event-driven triggers (FR14/FR15) are purely additive — no schema
// rework.
const TriggerOnDemand = "on_demand"

// Run is a single agent invocation record, owned by Knodex (FR12/FR15).
// JSON tags are camelCase to match every existing Knodex API DTO; the epic's
// snake_case schema names map 1:1.
type Run struct {
	ID    string `json:"id"`
	Actor string `json:"actor"` // authenticated Knodex user (email, falling back to sub/UserID)
	// AgentType is the agent identifier users filter by: the Agent CR name
	// for BYOA runs; built-ins (50.x) use their static id with
	// AgentNamespace == "".
	AgentType      string `json:"agentType"`
	AgentNamespace string `json:"agentNamespace"`
	ContextRef     string `json:"contextRef"`
	// KagentSessionID is the A2A response's contextId.
	KagentSessionID string `json:"kagentSessionId"`
	// ConversationID is the Knodex-side grouping key for chat sessions (Story
	// 50.6). It is set by the frontend per mounted chat and echoed on every
	// invoke, so all turns of one conversation share it. Distinct from
	// KagentSessionID: today each turn is an independent A2A call with NO
	// contextId continuity, so kagentSessionId is not shared across turns and
	// cannot group a conversation. Empty for legacy runs (each forms a
	// singleton session keyed by ID).
	ConversationID        string     `json:"conversationId"`
	InputSummary          string     `json:"inputSummary"`
	RecommendationSummary string     `json:"recommendationSummary"`
	ActionTaken           string     `json:"actionTaken"` // reserved; populated by future stories (e.g. 50.2)
	Timestamp             time.Time  `json:"timestamp"`   // invocation time
	CompletedAt           *time.Time `json:"completedAt,omitempty"`
	Status                string     `json:"status"`
	TriggerType           string     `json:"triggerType"`
	// InputTokens, OutputTokens, TotalTokens are the LLM token counts from
	// kagent (result.metadata["kagent_usage_metadata"]). Zero when absent.
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
}
