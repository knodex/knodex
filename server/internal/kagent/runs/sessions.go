// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package runs

import (
	"sort"
	"time"
)

// Session is a chat conversation assembled from Knodex-owned run records
// (Story 50.6). It is a server-side VIEW over Store.List results — NOT a new
// durable store and NOT read back from kagent's session store. Runs are
// grouped by the Knodex-side ConversationID; a run with an empty
// ConversationID (legacy, pre-50.6) forms its own singleton session keyed by
// its run id, so nothing is ever dropped.
type Session struct {
	// ID is the grouping key: the ConversationID, or the lone run's ID for a
	// singleton legacy session.
	ID             string `json:"id"`
	AgentType      string `json:"agentType"`
	AgentNamespace string `json:"agentNamespace"`
	// FirstPrompt is the oldest run's InputSummary — enough to identify the
	// conversation in a list.
	FirstPrompt    string    `json:"firstPrompt"`
	StartedAt      time.Time `json:"startedAt"`      // oldest run's invocation time
	LastActivityAt time.Time `json:"lastActivityAt"` // newest run's invocation time
	RunCount       int       `json:"runCount"`
	Status         string    `json:"status"` // newest run's status
	// Runs are the conversation's runs ordered oldest→newest for replay.
	Runs []Run `json:"runs"`
}

// GroupSessions folds a run slice into sessions keyed by ConversationID
// (fallback key = run ID when ConversationID == ""). Input order does not
// matter — the result is deterministic: sessions are ordered most-recent-first
// by LastActivityAt (tiebreak ID), and each session's Runs are ordered
// oldest→newest (tiebreak ID). A conversation is with one agent, so the
// session's agent fields come from its runs (all identical in practice).
func GroupSessions(in []Run) []Session {
	groups := make(map[string][]Run)
	for _, run := range in {
		key := run.ConversationID
		if key == "" {
			key = run.ID
		}
		groups[key] = append(groups[key], run)
	}

	sessions := make([]Session, 0, len(groups))
	for key, groupRuns := range groups {
		// Oldest→newest for replay (deterministic tiebreak on ID for
		// same-millisecond timestamps).
		sort.Slice(groupRuns, func(i, j int) bool {
			if groupRuns[i].Timestamp.Equal(groupRuns[j].Timestamp) {
				return groupRuns[i].ID < groupRuns[j].ID
			}
			return groupRuns[i].Timestamp.Before(groupRuns[j].Timestamp)
		})
		oldest := groupRuns[0]
		newest := groupRuns[len(groupRuns)-1]
		sessions = append(sessions, Session{
			ID:             key,
			AgentType:      newest.AgentType,
			AgentNamespace: newest.AgentNamespace,
			FirstPrompt:    oldest.InputSummary,
			StartedAt:      oldest.Timestamp,
			LastActivityAt: newest.Timestamp,
			RunCount:       len(groupRuns),
			Status:         newest.Status,
			Runs:           groupRuns,
		})
	}

	// Most-recent-first by last activity (tiebreak ID for stable paging).
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].LastActivityAt.Equal(sessions[j].LastActivityAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].LastActivityAt.After(sessions[j].LastActivityAt)
	})
	return sessions
}
