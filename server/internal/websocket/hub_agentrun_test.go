// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// contextWithCancelForHub starts the hub event loop and returns its cancel.
func contextWithCancelForHub(t *testing.T, hub *Hub) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	return ctx, cancel
}

// newAgentRunClient builds a mock client subscribed to agent_runs only
// (not "all"), proving the dedicated subscription gate.
func newAgentRunClient(hub *Hub, userID string, admin bool) *Client {
	client := newMockClient(hub, userID, []string{}, admin)
	client.subscriptions = map[string]bool{"agent_runs": true}
	return client
}

func agentRunMessage(t *testing.T, actorID, status string) *Message {
	t.Helper()
	msg, err := NewAgentRunUpdateMessage(ActionAdd, AgentRunUpdateData{
		RunID:          "run-1",
		AgentType:      "helper",
		AgentNamespace: "alpha-apps",
		Status:         status,
		ActorID:        actorID,
		Actor:          "dev@example.com",
		Timestamp:      time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("NewAgentRunUpdateMessage: %v", err)
	}
	return msg
}

func TestShouldSendToClient_AgentRun_ActorReceives(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	actor := newAgentRunClient(hub, "user-actor", false)
	msg := agentRunMessage(t, "user-actor", "running")

	got := hub.shouldSendToClient(actor, msg, hub.extractProjectID(msg), hub.extractAgentRunActorID(msg))
	if !got {
		t.Error("the invoking actor must receive agent run updates")
	}
}

func TestShouldSendToClient_AgentRun_OtherNonAdminDoesNot(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	other := newAgentRunClient(hub, "user-other", false)
	msg := agentRunMessage(t, "user-actor", "running")

	got := hub.shouldSendToClient(other, msg, hub.extractProjectID(msg), hub.extractAgentRunActorID(msg))
	if got {
		t.Error("a non-actor non-admin must NOT receive agent run updates")
	}
}

func TestShouldSendToClient_AgentRun_GlobalAdminReceives(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	admin := newAgentRunClient(hub, "admin-user", true)
	msg := agentRunMessage(t, "user-actor", "running")

	got := hub.shouldSendToClient(admin, msg, hub.extractProjectID(msg), hub.extractAgentRunActorID(msg))
	if !got {
		t.Error("a global admin must receive agent run updates")
	}
}

func TestShouldSendToClient_AgentRun_NoSubscriptionNotDelivered(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	// Actor identity matches but the client never subscribed to
	// agent_runs/all — the subscription gate must drop the message.
	actor := newMockClient(hub, "user-actor", []string{}, false)
	actor.subscriptions = map[string]bool{"instances": true}
	msg := agentRunMessage(t, "user-actor", "running")

	got := hub.shouldSendToClient(actor, msg, hub.extractProjectID(msg), hub.extractAgentRunActorID(msg))
	if got {
		t.Error("without an agent_runs/all subscription the message must not be delivered")
	}
}

func TestShouldSendToClient_AgentRun_AllSubscriptionDelivers(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	actor := newMockClient(hub, "user-actor", []string{}, false) // subscribes "all"
	msg := agentRunMessage(t, "user-actor", "running")

	got := hub.shouldSendToClient(actor, msg, hub.extractProjectID(msg), hub.extractAgentRunActorID(msg))
	if !got {
		t.Error("an 'all' subscription must cover agent run updates")
	}
}

func TestShouldSendToClient_AgentRun_EmptyActorIDFailsClosed(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	client := newAgentRunClient(hub, "", false)
	msg := agentRunMessage(t, "", "running")

	got := hub.shouldSendToClient(client, msg, hub.extractProjectID(msg), hub.extractAgentRunActorID(msg))
	if got {
		t.Error("an empty actorId must never match (fail-closed), even for an empty client userID")
	}
}

// TestHub_BroadcastAgentRunUpdate_AddAndUpdateBothDelivered proves the
// debounce key includes action+status: a fast add→update for the same run
// (well inside DebounceInterval) must deliver BOTH lifecycle transitions.
func TestHub_BroadcastAgentRunUpdate_AddAndUpdateBothDelivered(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	hubCtx, cancel := contextWithCancelForHub(t, hub)
	defer cancel()
	_ = hubCtx

	actor := newAgentRunClient(hub, "user-actor", false)
	hub.handleRegister(actor)

	data := AgentRunUpdateData{
		RunID:     "run-fast",
		AgentType: "helper",
		ActorID:   "user-actor",
		Actor:     "dev@example.com",
		Timestamp: time.Now().UTC(),
	}

	data.Status = "running"
	hub.BroadcastAgentRunUpdate(ActionAdd, data)
	data.Status = "failed"
	hub.BroadcastAgentRunUpdate(ActionUpdate, data) // milliseconds later — must NOT be debounced

	for i, wantStatus := range []string{"running", "failed"} {
		select {
		case msg := <-actor.send:
			if msg.Type != MessageTypeAgentRunUpdate {
				t.Fatalf("message %d: expected type %s, got %s", i, MessageTypeAgentRunUpdate, msg.Type)
			}
			var got AgentRunUpdateData
			if err := json.Unmarshal(msg.Data, &got); err != nil {
				t.Fatalf("message %d: unmarshal: %v", i, err)
			}
			if got.Status != wantStatus {
				t.Errorf("message %d: expected status %q, got %q", i, wantStatus, got.Status)
			}
		case <-time.After(time.Second):
			t.Fatalf("did not receive lifecycle message %d (%s) — debounce key must include action+status", i, wantStatus)
		}
	}
}

// TestHub_BroadcastAgentRunUpdate_SameTransitionDebounced proves repeats of
// the SAME (run, action, status) tuple are still debounced.
func TestHub_BroadcastAgentRunUpdate_SameTransitionDebounced(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	_, cancel := contextWithCancelForHub(t, hub)
	defer cancel()

	actor := newAgentRunClient(hub, "user-actor", false)
	hub.handleRegister(actor)

	data := AgentRunUpdateData{
		RunID:     "run-dup",
		AgentType: "helper",
		Status:    "running",
		ActorID:   "user-actor",
		Timestamp: time.Now().UTC(),
	}
	hub.BroadcastAgentRunUpdate(ActionAdd, data)
	hub.BroadcastAgentRunUpdate(ActionAdd, data) // identical tuple — debounced

	select {
	case <-actor.send:
	case <-time.After(time.Second):
		t.Fatal("did not receive first agent run message")
	}
	select {
	case <-actor.send:
		t.Error("identical (run, action, status) repeats must be debounced")
	case <-time.After(50 * time.Millisecond):
		// Expected.
	}
}

func TestExtractAgentRunActorID_NonAgentRunMessage(t *testing.T) {
	t.Parallel()
	hub := NewHub(nil)
	msg, err := NewInstanceUpdateMessage(ActionAdd, "g", "ns", "Kind", "name", nil, "proj-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := hub.extractAgentRunActorID(msg); got != "" {
		t.Errorf("expected empty actorID for non-agent-run message, got %q", got)
	}
}
