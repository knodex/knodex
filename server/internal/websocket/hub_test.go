// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map not initialized")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}

	if hub.register == nil {
		t.Error("register channel not initialized")
	}

	if hub.unregister == nil {
		t.Error("unregister channel not initialized")
	}

	if hub.lastUpdate == nil {
		t.Error("lastUpdate map not initialized")
	}

	if hub.ctx == nil {
		t.Error("ctx not initialized")
	}

	if hub.logger == nil {
		t.Error("logger not initialized")
	}

	if hub.countRequest == nil {
		t.Error("countRequest channel not initialized")
	}
}

func TestHub_GetMetrics(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	metrics := hub.GetMetrics()

	if metrics["activeConnections"] != 0 {
		t.Errorf("expected activeConnections=0, got %v", metrics["activeConnections"])
	}

	if metrics["totalConnections"] != int64(0) {
		t.Errorf("expected totalConnections=0, got %v", metrics["totalConnections"])
	}

	if metrics["totalMessages"] != int64(0) {
		t.Errorf("expected totalMessages=0, got %v", metrics["totalMessages"])
	}

	if metrics["maxConnections"] != MaxConnections {
		t.Errorf("expected maxConnections=%d, got %v", MaxConnections, metrics["maxConnections"])
	}
}

func TestHub_ActiveConnections(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	if hub.ActiveConnections() != 0 {
		t.Errorf("expected 0 active connections, got %d", hub.ActiveConnections())
	}
}

func TestHub_ShouldBroadcast_Debouncing(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	key := "test:key"

	// First broadcast should succeed
	if !hub.shouldBroadcast(key) {
		t.Error("first broadcast should succeed")
	}

	// Immediate second broadcast should be debounced
	if hub.shouldBroadcast(key) {
		t.Error("immediate second broadcast should be debounced")
	}

	// Wait for debounce interval to pass
	time.Sleep(DebounceInterval + 10*time.Millisecond)

	// After debounce interval, broadcast should succeed
	if !hub.shouldBroadcast(key) {
		t.Error("broadcast after debounce interval should succeed")
	}
}

func TestHub_Register(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	regChan := hub.Register()
	if regChan == nil {
		t.Error("Register() returned nil channel")
	}

	unregChan := hub.Unregister()
	if unregChan == nil {
		t.Error("Unregister() returned nil channel")
	}
}

func TestHub_ContextCancel(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Start hub in goroutine
	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Cancelling context should cause Run() to return
	cancel()

	// Wait for hub to stop with timeout
	select {
	case <-done:
		// Success - hub stopped
	case <-time.After(1 * time.Second):
		t.Fatal("hub did not stop within timeout")
	}
}

func TestHub_ContextCancel_ClosesClients(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())

	// Start hub in goroutine
	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Simulate adding clients directly (bypassing register channel for simplicity)
	hub.mu.Lock()
	client1 := &Client{send: make(chan *Message, 1)}
	client2 := &Client{send: make(chan *Message, 1)}
	hub.clients[client1] = true
	hub.clients[client2] = true
	hub.mu.Unlock()

	// Verify clients exist
	if hub.ActiveConnections() != 0 {
		// Note: ActiveConnections tracks via connectionsMu, not direct client count
		// Just verify clients map has entries
		hub.mu.RLock()
		clientCount := len(hub.clients)
		hub.mu.RUnlock()
		if clientCount != 2 {
			t.Fatalf("expected 2 clients, got %d", clientCount)
		}
	}

	// Cancel context to stop hub
	cancel()

	// Wait for hub to stop
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("hub did not stop within timeout")
	}

	// Verify clients were closed
	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()

	if clientCount != 0 {
		t.Errorf("expected 0 clients after stop, got %d", clientCount)
	}
}

func TestHub_SendCountsToClients_PerClientCounts(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	client1 := newMockClient(hub, "user1", []string{"proj-a"}, false)
	client2 := newMockClient(hub, "user2", []string{"proj-b"}, false)

	hub.mu.Lock()
	hub.clients[client1] = true
	hub.clients[client2] = true
	hub.mu.Unlock()

	countFn := func(_ context.Context, _ string, projects []string, _ []string) (int, int) {
		if len(projects) > 0 && projects[0] == "proj-a" {
			return 5, 3
		}
		if len(projects) > 0 && projects[0] == "proj-b" {
			return 2, 7
		}
		return 0, 0
	}

	hub.SendCountsToClients(countFn)

	// Read messages from each client
	select {
	case msg := <-client1.send:
		var data CountsUpdateData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal client1 data: %v", err)
		}
		if data.RGDCount != 5 || data.InstanceCount != 3 {
			t.Errorf("client1: expected rgd=5 instance=3, got rgd=%d instance=%d", data.RGDCount, data.InstanceCount)
		}
	case <-time.After(time.Second):
		t.Fatal("client1 did not receive counts message")
	}

	select {
	case msg := <-client2.send:
		var data CountsUpdateData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal client2 data: %v", err)
		}
		if data.RGDCount != 2 || data.InstanceCount != 7 {
			t.Errorf("client2: expected rgd=2 instance=7, got rgd=%d instance=%d", data.RGDCount, data.InstanceCount)
		}
	case <-time.After(time.Second):
		t.Fatal("client2 did not receive counts message")
	}
}

func TestHub_SendCountsToClients_SubscriptionCheck(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	// Create client with NO subscriptions (empty list)
	client := newMockClientWithSubscriptions(hub, "user1", []string{"proj-a"}, false, []string{})

	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	countFn := func(_ context.Context, _ string, _ []string, _ []string) (int, int) {
		return 10, 20
	}

	hub.SendCountsToClients(countFn)

	// Client should not receive a message
	select {
	case <-client.send:
		t.Error("unsubscribed client should not receive counts message")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message
	}
}

func TestHub_SendCountsToClients_Debouncing(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)

	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	callCount := 0
	countFn := func(_ context.Context, _ string, _ []string, _ []string) (int, int) {
		callCount++
		return callCount, callCount
	}

	// Rapid calls - should be debounced
	hub.SendCountsToClients(countFn)
	hub.SendCountsToClients(countFn) // Should be debounced
	hub.SendCountsToClients(countFn) // Should be debounced

	// Only 1 message should have been sent (first call succeeds, rest debounced)
	select {
	case <-client.send:
		// First message received
	case <-time.After(time.Second):
		t.Fatal("did not receive first message")
	}

	// No second message should arrive
	select {
	case <-client.send:
		t.Error("debounced calls should not produce additional messages")
	case <-time.After(50 * time.Millisecond):
		// Expected - debounced
	}

	if callCount != 1 {
		t.Errorf("countFn should have been called once due to debouncing, called %d times", callCount)
	}
}

func TestHub_SendCountsToClients_GlobalAdmin(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	// Global admin (allowAll=true)
	client := newMockClient(hub, "admin", []string{"proj-a"}, true)

	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	var receivedProjects []string
	countFn := func(_ context.Context, _ string, projects []string, _ []string) (int, int) {
		receivedProjects = projects
		return 100, 200
	}

	hub.SendCountsToClients(countFn)

	// Global admin should receive nil projects (unfiltered)
	if receivedProjects != nil {
		t.Errorf("expected nil projects for global admin, got %v", receivedProjects)
	}

	select {
	case msg := <-client.send:
		var data CountsUpdateData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal data: %v", err)
		}
		if data.RGDCount != 100 || data.InstanceCount != 200 {
			t.Errorf("expected rgd=100 instance=200, got rgd=%d instance=%d", data.RGDCount, data.InstanceCount)
		}
	case <-time.After(time.Second):
		t.Fatal("global admin did not receive counts message")
	}
}

func TestHub_HandleRegister_InitialCountPush(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	countFnCalled := false
	hub.SetCountFunc(func(_ context.Context, userID string, projects []string, _ []string) (int, int) {
		countFnCalled = true
		if userID != "user1" {
			t.Errorf("expected userID 'user1', got '%s'", userID)
		}
		return 10, 20
	})

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.register <- client

	// Count push is processed within the event loop
	select {
	case msg := <-client.send:
		if msg.Type != MessageTypeCountsUpdate {
			t.Errorf("expected message type %s, got %s", MessageTypeCountsUpdate, msg.Type)
		}
		var data CountsUpdateData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal counts data: %v", err)
		}
		if data.RGDCount != 10 || data.InstanceCount != 20 {
			t.Errorf("expected rgd=10 instance=20, got rgd=%d instance=%d", data.RGDCount, data.InstanceCount)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not receive initial counts message on registration")
	}

	if !countFnCalled {
		t.Error("expected countFn to be called on client registration")
	}

	cancel()
	<-done
}

func TestHub_HandleRegister_NoCountFunc(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// No SetCountFunc called - countFn is nil
	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.register <- client
	time.Sleep(20 * time.Millisecond)

	// Client should be registered but receive no counts message
	select {
	case <-client.send:
		t.Error("client should not receive any message when countFn is nil")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message
	}

	// Verify client was registered
	hub.mu.RLock()
	_, registered := hub.clients[client]
	hub.mu.RUnlock()
	if !registered {
		t.Error("client should be registered in hub")
	}

	cancel()
	<-done
}

func TestHub_HandleRegister_GlobalAdmin_InitialCountPush(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var receivedProjects []string
	hub.SetCountFunc(func(_ context.Context, _ string, projects []string, _ []string) (int, int) {
		receivedProjects = projects
		return 50, 100
	})

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	// Global admin client
	client := newMockClient(hub, "admin", []string{"proj-a"}, true)
	hub.register <- client

	// Global admin should receive nil projects (unfiltered counts)
	select {
	case msg := <-client.send:
		var data CountsUpdateData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal data: %v", err)
		}
		if data.RGDCount != 50 || data.InstanceCount != 100 {
			t.Errorf("expected rgd=50 instance=100, got rgd=%d instance=%d", data.RGDCount, data.InstanceCount)
		}
	case <-time.After(time.Second):
		t.Fatal("global admin did not receive initial counts on registration")
	}

	if receivedProjects != nil {
		t.Errorf("expected nil projects for global admin, got %v", receivedProjects)
	}

	cancel()
	<-done
}

func TestHub_CleanLastUpdate(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	// Seed 1000 stale entries (timestamps 2s ago — well past 10*DebounceInterval=1s)
	staleTime := time.Now().Add(-2 * time.Second)
	hub.lastUpdateLock.Lock()
	for i := 0; i < 1000; i++ {
		hub.lastUpdate[fmt.Sprintf("key:%d", i)] = staleTime
	}
	hub.lastUpdateLock.Unlock()

	// Verify entries exist
	hub.lastUpdateLock.Lock()
	beforeCount := len(hub.lastUpdate)
	hub.lastUpdateLock.Unlock()
	if beforeCount != 1000 {
		t.Fatalf("expected 1000 entries before cleanup, got %d", beforeCount)
	}

	// Run cleanup
	hub.cleanLastUpdate()

	// Verify all stale entries removed
	hub.lastUpdateLock.Lock()
	afterCount := len(hub.lastUpdate)
	hub.lastUpdateLock.Unlock()
	if afterCount != 0 {
		t.Errorf("expected 0 entries after cleanup, got %d", afterCount)
	}
}

func TestHub_CleanLastUpdate_PreservesRecent(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	staleTime := time.Now().Add(-2 * time.Second)
	freshTime := time.Now()

	hub.lastUpdateLock.Lock()
	// Add stale entries
	for i := 0; i < 5; i++ {
		hub.lastUpdate[fmt.Sprintf("stale:%d", i)] = staleTime
	}
	// Add fresh entries
	for i := 0; i < 3; i++ {
		hub.lastUpdate[fmt.Sprintf("fresh:%d", i)] = freshTime
	}
	hub.lastUpdateLock.Unlock()

	hub.cleanLastUpdate()

	hub.lastUpdateLock.Lock()
	remaining := len(hub.lastUpdate)
	hub.lastUpdateLock.Unlock()

	if remaining != 3 {
		t.Errorf("expected 3 fresh entries after cleanup, got %d", remaining)
	}
}

func TestHub_MaxConnections_EnvVar(t *testing.T) {
	t.Setenv("WEBSOCKET_MAX_CONNECTIONS", "200")

	hub := NewHub(nil)
	if hub.maxConnections != 200 {
		t.Errorf("expected maxConnections=200, got %d", hub.maxConnections)
	}
}

func TestHub_MaxConnections_Default(t *testing.T) {
	t.Setenv("WEBSOCKET_MAX_CONNECTIONS", "")

	hub := NewHub(nil)
	if hub.maxConnections != MaxConnections {
		t.Errorf("expected maxConnections=%d (default), got %d", MaxConnections, hub.maxConnections)
	}
}

func TestHub_MaxConnections_InvalidNonNumeric(t *testing.T) {
	t.Setenv("WEBSOCKET_MAX_CONNECTIONS", "abc")

	hub := NewHub(nil)
	if hub.maxConnections != MaxConnections {
		t.Errorf("expected maxConnections=%d (default) for invalid input, got %d", MaxConnections, hub.maxConnections)
	}
}

func TestHub_MaxConnections_ZeroFallsBackToDefault(t *testing.T) {
	t.Setenv("WEBSOCKET_MAX_CONNECTIONS", "0")

	hub := NewHub(nil)
	if hub.maxConnections != MaxConnections {
		t.Errorf("expected maxConnections=%d (default) for zero, got %d", MaxConnections, hub.maxConnections)
	}
}

func TestHub_MaxConnections_NegativeFallsBackToDefault(t *testing.T) {
	t.Setenv("WEBSOCKET_MAX_CONNECTIONS", "-1")

	hub := NewHub(nil)
	if hub.maxConnections != MaxConnections {
		t.Errorf("expected maxConnections=%d (default) for negative, got %d", MaxConnections, hub.maxConnections)
	}
}

func TestHub_BroadcastDriftUpdate(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start hub
	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.handleRegister(client)

	// Drain the initial counts message if any
	select {
	case <-client.send:
	case <-time.After(50 * time.Millisecond):
	}

	hub.BroadcastDriftUpdate("default", "WebApp", "my-app", false, "proj-a")

	// Wait for broadcast to be processed
	select {
	case msg := <-client.send:
		if msg.Type != MessageTypeDriftUpdate {
			t.Errorf("expected message type %s, got %s", MessageTypeDriftUpdate, msg.Type)
		}
		var data DriftUpdateData
		if err := json.Unmarshal(msg.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal drift update data: %v", err)
		}
		if data.Namespace != "default" || data.Kind != "WebApp" || data.Name != "my-app" {
			t.Errorf("unexpected data: %+v", data)
		}
		if data.Drifted != false {
			t.Error("expected drifted=false")
		}
		if data.ProjectID != "proj-a" {
			t.Errorf("expected projectId=proj-a, got %s", data.ProjectID)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not receive drift update message")
	}

	cancel()
	<-done
}

func TestHub_BroadcastDriftUpdate_Debouncing(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.handleRegister(client)

	// Drain initial counts
	select {
	case <-client.send:
	case <-time.After(50 * time.Millisecond):
	}

	// Rapid calls — should be debounced
	hub.BroadcastDriftUpdate("ns", "Kind", "app", false, "proj-a")
	hub.BroadcastDriftUpdate("ns", "Kind", "app", false, "proj-a") // debounced
	hub.BroadcastDriftUpdate("ns", "Kind", "app", false, "proj-a") // debounced

	select {
	case <-client.send:
		// First message received
	case <-time.After(time.Second):
		t.Fatal("did not receive first drift update message")
	}

	// No second message should arrive
	select {
	case <-client.send:
		t.Error("debounced drift updates should not produce additional messages")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}

	cancel()
	<-done
}

func TestHub_HandleRegister_RejectsWhenLimitReached(t *testing.T) {
	t.Setenv("WEBSOCKET_MAX_CONNECTIONS", "2")

	hub := NewHub(nil)

	// Register 2 clients (at limit)
	client1 := newMockClient(hub, "user1", []string{"proj-a"}, false)
	client2 := newMockClient(hub, "user2", []string{"proj-a"}, false)
	hub.handleRegister(client1)
	hub.handleRegister(client2)

	hub.mu.RLock()
	count := len(hub.clients)
	hub.mu.RUnlock()
	if count != 2 {
		t.Fatalf("expected 2 registered clients, got %d", count)
	}

	// 3rd client should be rejected
	client3 := newMockClient(hub, "user3", []string{"proj-a"}, false)
	hub.handleRegister(client3)

	hub.mu.RLock()
	count = len(hub.clients)
	hub.mu.RUnlock()
	if count != 2 {
		t.Errorf("expected 2 clients after rejection, got %d", count)
	}

	// Rejected client should receive an error message
	select {
	case msg := <-client3.send:
		if msg.Type != MessageTypeError {
			t.Errorf("expected error message, got type %s", msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("rejected client did not receive error message")
	}
}

func TestHub_Run_ContextPropagation(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	type ctxKey string
	hubCtx := context.WithValue(context.Background(), ctxKey("hub"), "test-value")
	hubCtx, cancel := context.WithCancel(hubCtx)
	defer cancel()

	done := make(chan struct{})
	go func() {
		hub.Run(hubCtx)
		close(done)
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Verify context propagates to countFn via SendCountsToClients
	var receivedCtx context.Context
	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.mu.Lock()
	hub.clients[client] = true
	hub.mu.Unlock()

	hub.SendCountsToClients(func(ctx context.Context, _ string, _ []string, _ []string) (int, int) {
		receivedCtx = ctx
		return 1, 1
	})

	// Drain message
	select {
	case <-client.send:
	case <-time.After(time.Second):
		t.Fatal("did not receive counts message")
	}

	if receivedCtx == nil {
		t.Fatal("countFn did not receive a context")
	}
	if receivedCtx.Value(ctxKey("hub")) != "test-value" {
		t.Error("countFn received a context without the hub's value — context not propagated")
	}

	cancel()
	<-done
}

func TestHub_Run_GracefulShutdown_WithinTimeout(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Add a client
	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.register <- client

	// Give registration time to process
	time.Sleep(10 * time.Millisecond)

	// Cancel context and verify hub exits within 5 seconds (AC requirement)
	cancel()

	select {
	case <-done:
		// Success — hub exited cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("hub did not exit within 5 seconds of context cancellation")
	}

	// Verify closeAllClients was called
	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()
	if clientCount != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", clientCount)
	}
}

func TestHub_InjectedLogger_CapturesOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	testLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	hub := NewHub(testLogger)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)

	// Register and unregister a client to generate log output
	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.register <- client
	time.Sleep(10 * time.Millisecond)
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-done

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output captured by test logger, got empty string")
	}

	// Verify key log messages were captured
	for _, expected := range []string{"WebSocket hub started", "client registered", "client unregistered", "hub stopped"} {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("expected log output to contain %q", expected)
		}
	}
}

// countingPolicyEnforcer tracks how many times CanAccessWithGroups is called.
// Uses atomic counter to be safe under concurrent access.
type countingPolicyEnforcer struct {
	allowAll  bool
	callCount atomic.Int32
}

func (m *countingPolicyEnforcer) CanAccessWithGroups(_ context.Context, _ string, _ []string, object, action string) (bool, error) {
	m.callCount.Add(1)
	if m.allowAll && object == "*" && action == "*" {
		return true, nil
	}
	return false, nil
}

func TestClient_CachedHasGlobalAccess_CachesResult(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	enforcer := &countingPolicyEnforcer{allowAll: true}

	client := &Client{
		hub:            hub,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool),
		userID:         "admin",
		projects:       []string{},
		groups:         []string{},
		policyEnforcer: enforcer,
		done:           make(chan struct{}),
	}

	ctx := context.Background()

	// First call should invoke Casbin
	result1 := client.CachedHasGlobalAccess(ctx)
	if !result1 {
		t.Error("expected CachedHasGlobalAccess to return true for admin")
	}
	if enforcer.callCount.Load() != 1 {
		t.Errorf("expected 1 Casbin call after first access, got %d", enforcer.callCount.Load())
	}

	// Second rapid call should use cache (no additional Casbin call)
	result2 := client.CachedHasGlobalAccess(ctx)
	if !result2 {
		t.Error("expected cached result to be true")
	}
	if enforcer.callCount.Load() != 1 {
		t.Errorf("expected still 1 Casbin call after cached access, got %d", enforcer.callCount.Load())
	}
}

func TestClient_CachedHasGlobalAccess_RefreshAfterTTL(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	enforcer := &countingPolicyEnforcer{allowAll: true}

	client := &Client{
		hub:            hub,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool),
		userID:         "admin",
		projects:       []string{},
		groups:         []string{},
		policyEnforcer: enforcer,
		done:           make(chan struct{}),
	}

	ctx := context.Background()

	// Populate cache
	client.CachedHasGlobalAccess(ctx)
	if enforcer.callCount.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", enforcer.callCount.Load())
	}

	// Manually expire the cache by backdating the timestamp
	client.userMu.Lock()
	client.globalAdminCachedAt = time.Now().Add(-globalAdminCacheTTL - time.Second)
	client.userMu.Unlock()

	// Next call should re-evaluate via Casbin
	client.CachedHasGlobalAccess(ctx)
	if enforcer.callCount.Load() != 2 {
		t.Errorf("expected 2 Casbin calls after TTL expiry, got %d", enforcer.callCount.Load())
	}
}

func TestClient_CachedHasGlobalAccess_NilEnforcer(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)

	client := &Client{
		hub:            hub,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool),
		userID:         "user1",
		projects:       []string{},
		groups:         []string{},
		policyEnforcer: nil, // No enforcer — fail closed
		done:           make(chan struct{}),
	}

	ctx := context.Background()

	result := client.CachedHasGlobalAccess(ctx)
	if result {
		t.Error("expected CachedHasGlobalAccess to return false when policyEnforcer is nil")
	}

	// Second call should still return false (cached false)
	result2 := client.CachedHasGlobalAccess(ctx)
	if result2 {
		t.Error("expected cached result to remain false for nil enforcer")
	}
}

func TestClient_CachedHasGlobalAccess_AdminRevoked(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	enforcer := &countingPolicyEnforcer{allowAll: true}

	client := &Client{
		hub:            hub,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool),
		userID:         "admin",
		projects:       []string{},
		groups:         []string{},
		policyEnforcer: enforcer,
		done:           make(chan struct{}),
	}

	ctx := context.Background()

	// Populate cache as admin
	result := client.CachedHasGlobalAccess(ctx)
	if !result {
		t.Fatal("expected true for admin")
	}

	// Revoke admin access and expire cache
	enforcer.allowAll = false
	client.userMu.Lock()
	client.globalAdminCachedAt = time.Now().Add(-globalAdminCacheTTL - time.Second)
	client.userMu.Unlock()

	// After TTL expiry, should re-evaluate and return false
	result = client.CachedHasGlobalAccess(ctx)
	if result {
		t.Error("expected CachedHasGlobalAccess to return false after admin revocation and TTL expiry")
	}
	if enforcer.callCount.Load() != 2 {
		t.Errorf("expected 2 Casbin calls, got %d", enforcer.callCount.Load())
	}
}

func TestClient_SetUserContext_InvalidatesCache(t *testing.T) {
	t.Parallel()

	hub := NewHub(nil)
	enforcer := &countingPolicyEnforcer{allowAll: true}

	client := &Client{
		hub:            hub,
		send:           make(chan *Message, 256),
		subscriptions:  make(map[string]bool),
		userID:         "admin",
		projects:       []string{},
		groups:         []string{},
		policyEnforcer: enforcer,
		done:           make(chan struct{}),
	}

	ctx := context.Background()

	// Populate cache
	client.CachedHasGlobalAccess(ctx)
	if enforcer.callCount.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", enforcer.callCount.Load())
	}

	// SetUserContext should invalidate cache
	newEnforcer := &countingPolicyEnforcer{allowAll: false}
	client.SetUserContext("admin", []string{}, []string{}, newEnforcer)

	// Next call should re-evaluate (not use stale cache)
	result := client.CachedHasGlobalAccess(ctx)
	if result {
		t.Error("expected false after SetUserContext with non-admin enforcer")
	}
	if newEnforcer.callCount.Load() != 1 {
		t.Errorf("expected new enforcer to be called once, got %d", newEnforcer.callCount.Load())
	}
}
