// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sessionRun(id, convID, agentType, namespace, input, status string, ts time.Time) Run {
	return Run{
		ID:             id,
		ConversationID: convID,
		AgentType:      agentType,
		AgentNamespace: namespace,
		InputSummary:   input,
		Status:         status,
		Timestamp:      ts,
	}
}

func sessionIDs(sessions []Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.ID
	}
	return out
}

func sessionRunIDs(rs []Run) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func TestGroupSessions_MultiTurnGrouping(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	// Newest-first input (as Store.List returns): turn3, turn2, turn1.
	in := []Run{
		sessionRun("r3", "conv-a", "rgd-builder", "", "third", StatusCompleted, base.Add(2*time.Minute)),
		sessionRun("r2", "conv-a", "rgd-builder", "", "second", StatusCompleted, base.Add(time.Minute)),
		sessionRun("r1", "conv-a", "rgd-builder", "", "first", StatusCompleted, base),
	}

	sessions := GroupSessions(in)
	require.Len(t, sessions, 1)
	s := sessions[0]
	assert.Equal(t, "conv-a", s.ID)
	assert.Equal(t, 3, s.RunCount)
	assert.Equal(t, "first", s.FirstPrompt, "first prompt is the OLDEST run's input")
	assert.Equal(t, base, s.StartedAt)
	assert.Equal(t, base.Add(2*time.Minute), s.LastActivityAt)
	assert.Equal(t, []string{"r1", "r2", "r3"}, sessionRunIDs(s.Runs), "runs ordered oldest→newest for replay")
}

func TestGroupSessions_SingletonLegacyRuns(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	// Two legacy runs (empty conversationId) → two singleton sessions keyed by
	// run id, never dropped or merged.
	in := []Run{
		sessionRun("r2", "", "rgd-builder", "", "b", StatusCompleted, base.Add(time.Minute)),
		sessionRun("r1", "", "rgd-builder", "", "a", StatusCompleted, base),
	}

	sessions := GroupSessions(in)
	require.Len(t, sessions, 2)
	assert.Equal(t, []string{"r2", "r1"}, sessionIDs(sessions), "singleton ids, most-recent-first")
	for _, s := range sessions {
		assert.Equal(t, 1, s.RunCount)
	}
}

func TestGroupSessions_OrderingMostRecentFirst(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	// conv-a's last activity is older than conv-b's last activity.
	in := []Run{
		sessionRun("b1", "conv-b", "rgd-builder", "", "b1", StatusCompleted, base.Add(10*time.Minute)),
		sessionRun("a2", "conv-a", "rgd-builder", "", "a2", StatusCompleted, base.Add(5*time.Minute)),
		sessionRun("a1", "conv-a", "rgd-builder", "", "a1", StatusCompleted, base),
	}

	sessions := GroupSessions(in)
	require.Len(t, sessions, 2)
	assert.Equal(t, []string{"conv-b", "conv-a"}, sessionIDs(sessions), "ordered by LastActivityAt desc")
}

func TestGroupSessions_StatusFromNewestRun(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	in := []Run{
		sessionRun("r2", "conv-a", "rgd-builder", "", "second", StatusFailed, base.Add(time.Minute)),
		sessionRun("r1", "conv-a", "rgd-builder", "", "first", StatusCompleted, base),
	}

	sessions := GroupSessions(in)
	require.Len(t, sessions, 1)
	assert.Equal(t, StatusFailed, sessions[0].Status, "session status reflects the newest run")
}

func TestGroupSessions_MixedAgentsNotMerged(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0).UTC()
	// Distinct conversations (different agents, different conversationIds) stay
	// separate sessions — a BYOA run never folds into a built-in conversation.
	in := []Run{
		sessionRun("byoa1", "conv-byoa", "helper", "alpha-apps", "byoa", StatusCompleted, base.Add(time.Minute)),
		sessionRun("bi1", "conv-bi", "rgd-builder", "", "builtin", StatusCompleted, base),
	}

	sessions := GroupSessions(in)
	require.Len(t, sessions, 2)
	byKind := map[string]Session{}
	for _, s := range sessions {
		byKind[s.ID] = s
	}
	assert.Equal(t, "helper", byKind["conv-byoa"].AgentType)
	assert.Equal(t, "alpha-apps", byKind["conv-byoa"].AgentNamespace)
	assert.Equal(t, "rgd-builder", byKind["conv-bi"].AgentType)
	assert.Equal(t, "", byKind["conv-bi"].AgentNamespace)
}

func TestGroupSessions_Empty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, GroupSessions(nil))
	assert.Empty(t, GroupSessions([]Run{}))
}
