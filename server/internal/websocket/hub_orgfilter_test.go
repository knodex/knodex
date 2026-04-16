// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// mockAddr implements net.Addr for testing
type mockAddr struct{}

func (m *mockAddr) Network() string { return "tcp" }
func (m *mockAddr) String() string  { return "127.0.0.1:12345" }

// mockConn implements a minimal interface for websocket connections
type mockConn struct{}

func (m *mockConn) RemoteAddr() net.Addr {
	return &mockAddr{}
}

// mockPolicyEnforcer implements ClientPolicyEnforcer for testing
// ArgoCD-aligned: uses Casbin-style permission checks instead of boolean flags
type mockPolicyEnforcer struct {
	// allowAll simulates global admin access (*, * permission)
	allowAll bool
}

func (m *mockPolicyEnforcer) CanAccessWithGroups(ctx context.Context, user string, groups []string, object, action string) (bool, error) {
	// If allowAll is true, grant access for wildcard checks (simulates role:serveradmin)
	if m.allowAll && object == "*" && action == "*" {
		return true, nil
	}
	return false, nil
}

// newMockClient creates a test WebSocket client.
// The addAdminPolicy parameter configures the mock Casbin policy enforcer
// to grant wildcard access (simulating role:serveradmin), rather than setting a boolean flag.
func newMockClient(hub *Hub, userID string, projects []string, addAdminPolicy bool) *Client {
	client := &Client{
		hub:           hub,
		conn:          nil, // Set to nil for testing (won't be used in filtering tests)
		send:          make(chan *Message, 256),
		subscriptions: make(map[string]bool),
		userID:        userID,
		projects:      projects,
		groups:        []string{},
	}

	// Set up policy enforcer via Casbin mock (ArgoCD-aligned: policies, not boolean flags)
	if addAdminPolicy {
		client.policyEnforcer = &mockPolicyEnforcer{allowAll: true}
	} else {
		client.policyEnforcer = &mockPolicyEnforcer{allowAll: false}
	}

	// Subscribe to all events for testing
	client.subscriptions["all"] = true
	return client
}

func TestShouldSendToClient_GlobalAdmin(t *testing.T) {
	hub := NewHub(nil)

	// Global admin should receive all events regardless of project
	globalAdmin := newMockClient(hub, "admin-user", []string{"project-a"}, true)

	tests := []struct {
		name    string
		message *Message
		want    bool
	}{
		{
			name: "instance update from different project",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-b", Name: "instance-1", ProjectID: "project-b"}),
			},
			want: true,
		},
		{
			name: "rgd update from different project",
			message: &Message{
				Type:      MessageTypeRGDUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-1", ProjectID: "project-c"}),
			},
			want: true,
		},
		{
			name: "instance update without project ID",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "default", Name: "instance-2", ProjectID: ""}),
			},
			want: true, // Global admin receives even without project ID
		},
		{
			name: "error message",
			message: &Message{
				Type:      MessageTypeError,
				Timestamp: time.Now(),
				Data:      mustMarshal(ErrorData{Code: "TEST", Message: "test error"}),
			},
			want: true,
		},
		{
			name: "pong message",
			message: &Message{
				Type:      MessageTypePong,
				Timestamp: time.Now(),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hub.shouldSendToClient(globalAdmin, tt.message, hub.extractProjectID(tt.message))
			if got != tt.want {
				t.Errorf("shouldSendToClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldSendToClient_RegularUser_SingleProject(t *testing.T) {
	hub := NewHub(nil)

	// User belongs to project-a only
	user := newMockClient(hub, "user-1", []string{"project-a"}, false)

	tests := []struct {
		name    string
		message *Message
		want    bool
	}{
		{
			name: "instance update from user's project",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-a", Name: "instance-1", ProjectID: "project-a"}),
			},
			want: true,
		},
		{
			name: "instance update from different project",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-b", Name: "instance-2", ProjectID: "project-b"}),
			},
			want: false, // Should NOT receive updates from other projects
		},
		{
			name: "rgd update from user's project",
			message: &Message{
				Type:      MessageTypeRGDUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-1", ProjectID: "project-a"}),
			},
			want: true,
		},
		{
			name: "rgd update from different project",
			message: &Message{
				Type:      MessageTypeRGDUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-2", ProjectID: "project-b"}),
			},
			want: false, // Should NOT receive updates from other projects
		},
		{
			name: "instance update without project ID",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "default", Name: "instance-3", ProjectID: ""}),
			},
			want: false, // Safety default: don't send if no project ID
		},
		{
			name: "error message",
			message: &Message{
				Type:      MessageTypeError,
				Timestamp: time.Now(),
				Data:      mustMarshal(ErrorData{Code: "TEST", Message: "test error"}),
			},
			want: true, // Always send error messages
		},
		{
			name: "pong message",
			message: &Message{
				Type:      MessageTypePong,
				Timestamp: time.Now(),
			},
			want: true, // Always send pong messages
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hub.shouldSendToClient(user, tt.message, hub.extractProjectID(tt.message))
			if got != tt.want {
				t.Errorf("shouldSendToClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldSendToClient_RegularUser_MultipleProjects(t *testing.T) {
	hub := NewHub(nil)

	// User belongs to project-a and project-b
	user := newMockClient(hub, "user-2", []string{"project-a", "project-b"}, false)

	tests := []struct {
		name    string
		message *Message
		want    bool
	}{
		{
			name: "instance update from first project",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-a", Name: "instance-1", ProjectID: "project-a"}),
			},
			want: true,
		},
		{
			name: "instance update from second project",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-b", Name: "instance-2", ProjectID: "project-b"}),
			},
			want: true,
		},
		{
			name: "instance update from non-member project",
			message: &Message{
				Type:      MessageTypeInstanceUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-c", Name: "instance-3", ProjectID: "project-c"}),
			},
			want: false, // Should NOT receive updates from projects user doesn't belong to
		},
		{
			name: "rgd update from first project",
			message: &Message{
				Type:      MessageTypeRGDUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-1", ProjectID: "project-a"}),
			},
			want: true,
		},
		{
			name: "rgd update from second project",
			message: &Message{
				Type:      MessageTypeRGDUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-2", ProjectID: "project-b"}),
			},
			want: true,
		},
		{
			name: "rgd update from non-member project",
			message: &Message{
				Type:      MessageTypeRGDUpdate,
				Timestamp: time.Now(),
				Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-3", ProjectID: "project-c"}),
			},
			want: false, // Should NOT receive updates from projects user doesn't belong to
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hub.shouldSendToClient(user, tt.message, hub.extractProjectID(tt.message))
			if got != tt.want {
				t.Errorf("shouldSendToClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldSendToClient_NoSubscription(t *testing.T) {
	hub := NewHub(nil)

	// User with no subscriptions (ArgoCD-aligned: Casbin policy enforcer for access checks)
	client := &Client{
		hub:            hub,
		conn:           nil,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool), // Empty - no subscriptions
		userID:         "user-3",
		projects:       []string{"project-a"},
		groups:         []string{},
		policyEnforcer: &mockPolicyEnforcer{allowAll: false},
	}

	message := &Message{
		Type:      MessageTypeInstanceUpdate,
		Timestamp: time.Now(),
		Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-a", Name: "instance-1", ProjectID: "project-a"}),
	}

	got := hub.shouldSendToClient(client, message, hub.extractProjectID(message))
	if got != false {
		t.Errorf("shouldSendToClient() = %v, want false (no subscription)", got)
	}

	// But error and pong messages should still go through
	errorMsg := &Message{
		Type:      MessageTypeError,
		Timestamp: time.Now(),
		Data:      mustMarshal(ErrorData{Code: "TEST", Message: "test"}),
	}

	got = hub.shouldSendToClient(client, errorMsg, hub.extractProjectID(errorMsg))
	if got != true {
		t.Errorf("shouldSendToClient() = %v, want true (error message always sent)", got)
	}
}

func TestShouldSendToClient_InvalidMessageType(t *testing.T) {
	hub := NewHub(nil)
	user := newMockClient(hub, "user-4", []string{"project-a"}, false)

	// Unknown message type
	message := &Message{
		Type:      MessageType("unknown_type"),
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"test": "data"}`),
	}

	got := hub.shouldSendToClient(user, message, hub.extractProjectID(message))
	if got != false {
		t.Errorf("shouldSendToClient() = %v, want false (unknown message type)", got)
	}
}

func TestShouldSendToClient_MalformedData(t *testing.T) {
	hub := NewHub(nil)
	user := newMockClient(hub, "user-5", []string{"project-a"}, false)

	// Message with malformed JSON data
	message := &Message{
		Type:      MessageTypeInstanceUpdate,
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{invalid json`),
	}

	// Should return false when data can't be unmarshaled
	got := hub.shouldSendToClient(user, message, hub.extractProjectID(message))
	if got != false {
		t.Errorf("shouldSendToClient() = %v, want false (malformed data)", got)
	}
}

func TestShouldSendToClient_IsolationBetweenUsers(t *testing.T) {
	hub := NewHub(nil)

	// Two users in different projects
	userA := newMockClient(hub, "user-a", []string{"project-a"}, false)
	userB := newMockClient(hub, "user-b", []string{"project-b"}, false)

	// Message for project-a
	messageProjectA := &Message{
		Type:      MessageTypeInstanceUpdate,
		Timestamp: time.Now(),
		Data:      mustMarshal(InstanceUpdateData{Action: ActionAdd, Namespace: "project-a", Name: "instance-1", ProjectID: "project-a"}),
	}

	// User A should receive
	if got := hub.shouldSendToClient(userA, messageProjectA, hub.extractProjectID(messageProjectA)); !got {
		t.Error("User A should receive message from their project")
	}

	// User B should NOT receive
	if got := hub.shouldSendToClient(userB, messageProjectA, hub.extractProjectID(messageProjectA)); got {
		t.Error("User B should NOT receive message from different project (isolation violated)")
	}

	// Message for project-b
	messageProjectB := &Message{
		Type:      MessageTypeRGDUpdate,
		Timestamp: time.Now(),
		Data:      mustMarshal(RGDUpdateData{Action: ActionUpdate, Name: "rgd-1", ProjectID: "project-b"}),
	}

	// User B should receive
	if got := hub.shouldSendToClient(userB, messageProjectB, hub.extractProjectID(messageProjectB)); !got {
		t.Error("User B should receive message from their project")
	}

	// User A should NOT receive
	if got := hub.shouldSendToClient(userA, messageProjectB, hub.extractProjectID(messageProjectB)); got {
		t.Error("User A should NOT receive message from different project (isolation violated)")
	}
}

func TestClientUserContext_ThreadSafety(t *testing.T) {
	hub := NewHub(nil)
	client := newMockClient(hub, "user-6", []string{"project-a"}, false)

	// Concurrently set and get user context
	// ArgoCD-aligned: SetUserContext now takes groups, casbinRoles, and policyEnforcer
	done := make(chan bool)
	mockEnforcer := &mockPolicyEnforcer{allowAll: false}
	for i := 0; i < 10; i++ {
		go func(idx int) {
			client.SetUserContext("user-concurrent", []string{"project-concurrent"}, []string{}, mockEnforcer)
			userID, projects, _ := client.GetUserContext()
			if userID == "" || len(projects) == 0 {
				t.Errorf("Concurrent access %d: got empty user context", idx)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// newMockClientWithSubscriptions creates a test WebSocket client with specific subscriptions.
// The addAdminPolicy parameter configures the mock Casbin policy enforcer
// to grant wildcard access (simulating role:serveradmin), rather than setting a boolean flag.
func newMockClientWithSubscriptions(hub *Hub, userID string, projects []string, addAdminPolicy bool, subscriptions []string) *Client {
	client := &Client{
		hub:           hub,
		conn:          nil,
		send:          make(chan *Message, 256),
		subscriptions: make(map[string]bool),
		userID:        userID,
		projects:      projects,
		groups:        []string{},
	}

	// Set up policy enforcer via Casbin mock (ArgoCD-aligned: policies, not boolean flags)
	if addAdminPolicy {
		client.policyEnforcer = &mockPolicyEnforcer{allowAll: true}
	} else {
		client.policyEnforcer = &mockPolicyEnforcer{allowAll: false}
	}

	for _, sub := range subscriptions {
		client.subscriptions[sub] = true
	}
	return client
}

func TestShouldSendToClient_ViolationUpdate_GlobalAdmin(t *testing.T) {
	hub := NewHub(nil)

	// Global admin subscribed to violations should receive all violation updates
	globalAdmin := newMockClientWithSubscriptions(hub, "admin-user", []string{"project-a"}, true, []string{"violations"})

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionAdd,
			ConstraintKind:    "K8sRequiredLabels",
			ConstraintName:    "require-app-label",
			Resource:          ViolationResourceData{Kind: "Pod", Namespace: "default", Name: "test-pod"},
			Message:           "Missing required label: app",
			EnforcementAction: "deny",
		}),
	}

	got := hub.shouldSendToClient(globalAdmin, violationMsg, hub.extractProjectID(violationMsg))
	if !got {
		t.Error("Global admin with violations subscription should receive violation updates")
	}
}

func TestShouldSendToClient_ViolationUpdate_GlobalAdmin_AllSubscription(t *testing.T) {
	hub := NewHub(nil)

	// Global admin subscribed to "all" should also receive violation updates
	globalAdmin := newMockClientWithSubscriptions(hub, "admin-user", []string{"project-a"}, true, []string{"all"})

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionDelete, // Resolved violation
			ConstraintKind:    "K8sContainerRatios",
			ConstraintName:    "container-limits",
			Resource:          ViolationResourceData{Kind: "Deployment", Namespace: "production", Name: "web-app", APIGroup: "apps"},
			Message:           "Container resources exceed allowed ratios",
			EnforcementAction: "warn",
		}),
	}

	got := hub.shouldSendToClient(globalAdmin, violationMsg, hub.extractProjectID(violationMsg))
	if !got {
		t.Error("Global admin with 'all' subscription should receive violation updates")
	}
}

func TestShouldSendToClient_ViolationUpdate_NonAdmin_Denied(t *testing.T) {
	hub := NewHub(nil)

	// Non-admin user subscribed to violations should NOT receive violation updates
	// Compliance data is admin-only
	regularUser := newMockClientWithSubscriptions(hub, "regular-user", []string{"project-a"}, false, []string{"violations"})

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionAdd,
			ConstraintKind:    "K8sRequiredLabels",
			ConstraintName:    "require-app-label",
			Resource:          ViolationResourceData{Kind: "Pod", Namespace: "default", Name: "test-pod"},
			Message:           "Missing required label: app",
			EnforcementAction: "deny",
		}),
	}

	got := hub.shouldSendToClient(regularUser, violationMsg, hub.extractProjectID(violationMsg))
	if got {
		t.Error("Non-admin user should NOT receive violation updates (compliance is admin-only)")
	}
}

func TestShouldSendToClient_ViolationUpdate_NonAdmin_AllSubscription_Denied(t *testing.T) {
	hub := NewHub(nil)

	// Non-admin user with "all" subscription should still NOT receive violation updates
	regularUser := newMockClientWithSubscriptions(hub, "regular-user", []string{"project-a", "project-b"}, false, []string{"all"})

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionAdd,
			ConstraintKind:    "K8sPodSecurityPolicy",
			ConstraintName:    "no-privileged-containers",
			Resource:          ViolationResourceData{Kind: "Pod", Namespace: "project-a", Name: "privileged-pod"},
			Message:           "Privileged containers are not allowed",
			EnforcementAction: "deny",
		}),
	}

	got := hub.shouldSendToClient(regularUser, violationMsg, hub.extractProjectID(violationMsg))
	if got {
		t.Error("Non-admin user with 'all' subscription should still NOT receive violation updates")
	}
}

func TestShouldSendToClient_ViolationUpdate_NoSubscription(t *testing.T) {
	hub := NewHub(nil)

	// Global admin WITHOUT violations subscription should NOT receive violation updates
	globalAdminNoSub := newMockClientWithSubscriptions(hub, "admin-user", []string{}, true, []string{"instances", "rgds"})

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionAdd,
			ConstraintKind:    "K8sRequiredLabels",
			ConstraintName:    "require-app-label",
			Resource:          ViolationResourceData{Kind: "Pod", Namespace: "default", Name: "test-pod"},
			Message:           "Missing required label: app",
			EnforcementAction: "deny",
		}),
	}

	got := hub.shouldSendToClient(globalAdminNoSub, violationMsg, hub.extractProjectID(violationMsg))
	if got {
		t.Error("Global admin without violations subscription should NOT receive violation updates")
	}
}

func TestShouldSendToClient_ViolationUpdate_ResolvedAction(t *testing.T) {
	hub := NewHub(nil)

	// Global admin should receive resolved (delete action) violation updates
	globalAdmin := newMockClientWithSubscriptions(hub, "admin-user", nil, true, []string{"violations"})

	resolvedViolationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionDelete, // Resolved
			ConstraintKind:    "K8sMemoryLimits",
			ConstraintName:    "memory-limit-required",
			Resource:          ViolationResourceData{Kind: "Deployment", Namespace: "staging", Name: "api-server", APIGroup: "apps"},
			Message:           "Memory limit is required",
			EnforcementAction: "warn",
		}),
	}

	got := hub.shouldSendToClient(globalAdmin, resolvedViolationMsg, hub.extractProjectID(resolvedViolationMsg))
	if !got {
		t.Error("Global admin should receive resolved (delete action) violation updates")
	}
}

func TestShouldSendToClient_ViolationUpdate_MultipleAdmins(t *testing.T) {
	hub := NewHub(nil)

	// Multiple global admins with violations subscription
	admin1 := newMockClientWithSubscriptions(hub, "admin-1", []string{"project-a"}, true, []string{"violations"})
	admin2 := newMockClientWithSubscriptions(hub, "admin-2", []string{"project-b"}, true, []string{"all"})
	regularUser := newMockClientWithSubscriptions(hub, "user-1", []string{"project-a", "project-b"}, false, []string{"violations", "all"})

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionAdd,
			ConstraintKind:    "K8sBlockNodePort",
			ConstraintName:    "block-nodeport",
			Resource:          ViolationResourceData{Kind: "Service", Namespace: "project-a", Name: "exposed-service"},
			Message:           "NodePort services are not allowed",
			EnforcementAction: "deny",
		}),
	}

	// Admin 1 should receive
	if got := hub.shouldSendToClient(admin1, violationMsg, hub.extractProjectID(violationMsg)); !got {
		t.Error("Admin 1 with violations subscription should receive violation updates")
	}

	// Admin 2 should receive (via "all" subscription)
	if got := hub.shouldSendToClient(admin2, violationMsg, hub.extractProjectID(violationMsg)); !got {
		t.Error("Admin 2 with 'all' subscription should receive violation updates")
	}

	// Regular user should NOT receive (compliance is admin-only)
	if got := hub.shouldSendToClient(regularUser, violationMsg, hub.extractProjectID(violationMsg)); got {
		t.Error("Regular user should NOT receive violation updates even with both subscriptions")
	}
}

func TestShouldSendToClient_ViolationUpdate_EmptySubscriptions(t *testing.T) {
	hub := NewHub(nil)

	// Global admin with empty subscriptions (ArgoCD-aligned: Casbin policy enforcer for access checks)
	adminNoSubs := &Client{
		hub:            hub,
		conn:           nil,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool), // Empty
		userID:         "admin-empty",
		projects:       nil,
		groups:         []string{},
		policyEnforcer: &mockPolicyEnforcer{allowAll: true},
	}

	violationMsg := &Message{
		Type:      MessageTypeViolationUpdate,
		Timestamp: time.Now(),
		Data: mustMarshal(ViolationUpdateData{
			Action:            ActionAdd,
			ConstraintKind:    "K8sRequiredLabels",
			ConstraintName:    "require-team-label",
			Resource:          ViolationResourceData{Kind: "Namespace", Namespace: "", Name: "new-namespace"},
			Message:           "Missing required label: team",
			EnforcementAction: "deny",
		}),
	}

	got := hub.shouldSendToClient(adminNoSubs, violationMsg, hub.extractProjectID(violationMsg))
	if got {
		t.Error("Admin with no subscriptions should NOT receive violation updates")
	}
}

func TestShouldSendToClient_DriftUpdate_ProjectScoped(t *testing.T) {
	hub := NewHub(nil)

	userA := newMockClient(hub, "user-a", []string{"project-a"}, false)
	userB := newMockClient(hub, "user-b", []string{"project-b"}, false)

	driftMsg := &Message{
		Type:      MessageTypeDriftUpdate,
		Timestamp: time.Now(),
		Data:      mustMarshal(DriftUpdateData{Namespace: "default", Kind: "WebApp", Name: "my-app", Drifted: false, ProjectID: "project-a"}),
	}

	// User A (in project-a) should receive
	if got := hub.shouldSendToClient(userA, driftMsg, hub.extractProjectID(driftMsg)); !got {
		t.Error("User A should receive drift update from their project")
	}

	// User B (in project-b) should NOT receive
	if got := hub.shouldSendToClient(userB, driftMsg, hub.extractProjectID(driftMsg)); got {
		t.Error("User B should NOT receive drift update from different project")
	}
}

func TestShouldSendToClient_DriftUpdate_GlobalAdmin(t *testing.T) {
	hub := NewHub(nil)

	admin := newMockClient(hub, "admin", []string{"project-a"}, true)

	driftMsg := &Message{
		Type:      MessageTypeDriftUpdate,
		Timestamp: time.Now(),
		Data:      mustMarshal(DriftUpdateData{Namespace: "default", Kind: "WebApp", Name: "my-app", Drifted: false, ProjectID: "project-b"}),
	}

	// Global admin should receive drift updates from any project
	if got := hub.shouldSendToClient(admin, driftMsg, hub.extractProjectID(driftMsg)); !got {
		t.Error("Global admin should receive drift update from any project")
	}
}

func TestShouldSendToClient_DriftUpdate_NoSubscription(t *testing.T) {
	hub := NewHub(nil)

	// Client with NO subscriptions
	client := newMockClientWithSubscriptions(hub, "user-1", []string{"project-a"}, false, []string{})

	driftMsg := &Message{
		Type:      MessageTypeDriftUpdate,
		Timestamp: time.Now(),
		Data:      mustMarshal(DriftUpdateData{Namespace: "default", Kind: "WebApp", Name: "my-app", Drifted: false, ProjectID: "project-a"}),
	}

	if got := hub.shouldSendToClient(client, driftMsg, hub.extractProjectID(driftMsg)); got {
		t.Error("Client with no subscriptions should NOT receive drift updates")
	}
}

// Helper function to marshal data for tests
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
