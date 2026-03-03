package audit

import (
	"context"
	"net/http"
	"testing"
)

// mockRecorder captures recorded events for testing.
type mockRecorder struct {
	events []Event
}

func (m *mockRecorder) Record(_ context.Context, event Event) {
	m.events = append(m.events, event)
}

func TestRecordEvent_NilRecorder(t *testing.T) {
	t.Parallel()

	// Must not panic when recorder is nil (OSS builds)
	RecordEvent(nil, context.Background(), Event{
		Action:   "create",
		Resource: "projects",
		Name:     "test-project",
		Result:   "success",
	})
}

func TestRecordEvent_WithRecorder(t *testing.T) {
	t.Parallel()

	mock := &mockRecorder{}
	ctx := context.Background()

	RecordEvent(mock, ctx, Event{
		UserID:    "user-1",
		UserEmail: "admin@test.local",
		SourceIP:  "10.0.0.1",
		Action:    "delete",
		Resource:  "instances",
		Name:      "my-instance",
		Project:   "alpha",
		Namespace: "alpha-ns",
		RequestID: "req-123",
		Result:    "success",
		Details:   map[string]any{"rgdName": "webapp"},
	})

	if len(mock.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(mock.events))
	}

	e := mock.events[0]
	if e.Action != "delete" {
		t.Errorf("expected action 'delete', got %q", e.Action)
	}
	if e.Resource != "instances" {
		t.Errorf("expected resource 'instances', got %q", e.Resource)
	}
	if e.Name != "my-instance" {
		t.Errorf("expected name 'my-instance', got %q", e.Name)
	}
	if e.UserEmail != "admin@test.local" {
		t.Errorf("expected email 'admin@test.local', got %q", e.UserEmail)
	}
	if e.Details["rgdName"] != "webapp" {
		t.Errorf("expected rgdName 'webapp' in details, got %v", e.Details["rgdName"])
	}
}

func TestSourceIP_XForwardedFor(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	if got := SourceIP(r); got != "1.2.3.4" {
		t.Errorf("expected '1.2.3.4', got %q", got)
	}
}

func TestSourceIP_XForwardedForSingle(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "5.6.7.8")
	if got := SourceIP(r); got != "5.6.7.8" {
		t.Errorf("expected '5.6.7.8', got %q", got)
	}
}

func TestSourceIP_XRealIP(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "9.8.7.6")
	if got := SourceIP(r); got != "9.8.7.6" {
		t.Errorf("expected '9.8.7.6', got %q", got)
	}
}

func TestSourceIP_RemoteAddr(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1:54321"
	if got := SourceIP(r); got != "192.168.1.1" {
		t.Errorf("expected '192.168.1.1', got %q", got)
	}
}

func TestSourceIP_RemoteAddrNoPort(t *testing.T) {
	t.Parallel()

	r, _ := http.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.1"
	if got := SourceIP(r); got != "192.168.1.1" {
		t.Errorf("expected '192.168.1.1', got %q", got)
	}
}
