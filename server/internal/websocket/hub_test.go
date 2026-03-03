package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	t.Parallel()

	hub := NewHub()

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

	if hub.stop == nil {
		t.Error("stop channel not initialized")
	}
}

func TestHub_GetMetrics(t *testing.T) {
	t.Parallel()

	hub := NewHub()

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

	hub := NewHub()

	if hub.ActiveConnections() != 0 {
		t.Errorf("expected 0 active connections, got %d", hub.ActiveConnections())
	}
}

func TestHub_ShouldBroadcast_Debouncing(t *testing.T) {
	t.Parallel()

	hub := NewHub()
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

	hub := NewHub()

	regChan := hub.Register()
	if regChan == nil {
		t.Error("Register() returned nil channel")
	}

	unregChan := hub.Unregister()
	if unregChan == nil {
		t.Error("Unregister() returned nil channel")
	}
}

func TestHub_Stop(t *testing.T) {
	t.Parallel()

	hub := NewHub()

	// Start hub in goroutine
	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Stop should cause Run() to return
	hub.Stop()

	// Wait for hub to stop with timeout
	select {
	case <-done:
		// Success - hub stopped
	case <-time.After(1 * time.Second):
		t.Fatal("hub did not stop within timeout")
	}
}

func TestHub_Stop_ClosesClients(t *testing.T) {
	t.Parallel()

	hub := NewHub()

	// Start hub in goroutine
	done := make(chan struct{})
	go func() {
		hub.Run()
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

	// Stop hub
	hub.Stop()

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

	hub := NewHub()

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

	hub := NewHub()

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

	hub := NewHub()

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

	hub := NewHub()

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

	hub := NewHub()

	countFnCalled := false
	hub.SetCountFunc(func(_ context.Context, userID string, projects []string, _ []string) (int, int) {
		countFnCalled = true
		if userID != "user1" {
			t.Errorf("expected userID 'user1', got '%s'", userID)
		}
		return 10, 20
	})

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)

	// Call handleRegister directly (spawns goroutine for initial count push)
	hub.handleRegister(client)

	// Wait for goroutine to complete
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
}

func TestHub_HandleRegister_NoCountFunc(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	// No SetCountFunc called - countFn is nil

	client := newMockClient(hub, "user1", []string{"proj-a"}, false)
	hub.handleRegister(client)

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
}

func TestHub_HandleRegister_GlobalAdmin_InitialCountPush(t *testing.T) {
	t.Parallel()

	hub := NewHub()

	var receivedProjects []string
	hub.SetCountFunc(func(_ context.Context, _ string, projects []string, _ []string) (int, int) {
		receivedProjects = projects
		return 50, 100
	})

	// Global admin client
	client := newMockClient(hub, "admin", []string{"proj-a"}, true)
	hub.handleRegister(client)

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
}
