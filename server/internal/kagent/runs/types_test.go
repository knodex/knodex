// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_WireFormat_CamelCaseKeys pins the JSON contract of the run record
// (AC #3): the exact camelCase key set every consumer (web table, E2E mocks,
// the 49.5 EE store) relies on, including triggerType's "on_demand" default.
// The handler tests marshal AND unmarshal through this same struct, so a tag
// typo would round-trip undetected there — only a raw-key assertion catches
// it.
func TestRun_WireFormat_CamelCaseKeys(t *testing.T) {
	t.Parallel()

	completed := time.Date(2026, 6, 6, 10, 0, 30, 0, time.UTC)
	run := Run{
		ID:                    "run-1",
		Actor:                 "dev@example.com",
		AgentType:             "helper",
		AgentNamespace:        "alpha-apps",
		ContextRef:            "rgd:webapp",
		KagentSessionID:       "ctx-123",
		ConversationID:        "conv-1",
		InputSummary:          "what should I do?",
		RecommendationSummary: "scale to 3 replicas",
		ActionTaken:           "",
		Timestamp:             time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC),
		CompletedAt:           &completed,
		Status:                StatusCompleted,
		TriggerType:           TriggerOnDemand,
	}

	payload, err := json.Marshal(run)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &raw))

	wantKeys := []string{
		"id", "actor", "agentType", "agentNamespace", "contextRef",
		"kagentSessionId", "conversationId", "inputSummary", "recommendationSummary",
		"actionTaken", "timestamp", "completedAt", "status", "triggerType",
	}
	gotKeys := make([]string, 0, len(raw))
	for k := range raw {
		gotKeys = append(gotKeys, k)
	}
	assert.ElementsMatch(t, wantKeys, gotKeys, "run record wire keys must be exactly the locked camelCase set")

	// The FR15 default rides the wire verbatim.
	assert.JSONEq(t, `"on_demand"`, string(raw["triggerType"]))
	assert.JSONEq(t, `"ctx-123"`, string(raw["kagentSessionId"]), "contextId must surface under kagentSessionId")
}

// TestRun_WireFormat_CompletedAtOmittedWhileRunning proves the only optional
// key: completedAt is absent while a run is in-flight (the web type marks it
// `completedAt?`), and every other key is still present even when empty.
func TestRun_WireFormat_CompletedAtOmittedWhileRunning(t *testing.T) {
	t.Parallel()

	run := Run{
		ID:          "run-1",
		Actor:       "dev@example.com",
		AgentType:   "helper",
		Timestamp:   time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC),
		Status:      StatusRunning,
		TriggerType: TriggerOnDemand,
	}

	payload, err := json.Marshal(run)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &raw))

	assert.NotContains(t, raw, "completedAt", "completedAt must be omitted while running")
	// Empty strings still serialize — the table renders them, it never guards
	// against missing keys.
	for _, key := range []string{"agentNamespace", "contextRef", "kagentSessionId", "conversationId", "recommendationSummary", "actionTaken"} {
		assert.Contains(t, raw, key, "empty %s must still be present on the wire", key)
	}
}
