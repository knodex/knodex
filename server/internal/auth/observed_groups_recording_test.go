// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeObservedGroupsRecorder captures Record calls. recording happens in a
// goroutine, so callers wait on the recorded channel.
type fakeObservedGroupsRecorder struct {
	mu       sync.Mutex
	recorded [][]string
	err      error
	called   chan []string
}

func newFakeRecorder(err error) *fakeObservedGroupsRecorder {
	return &fakeObservedGroupsRecorder{err: err, called: make(chan []string, 8)}
}

func (f *fakeObservedGroupsRecorder) Record(_ context.Context, groups []string) error {
	f.mu.Lock()
	f.recorded = append(f.recorded, groups)
	f.mu.Unlock()
	f.called <- groups
	return f.err
}

func (f *fakeObservedGroupsRecorder) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.recorded)
}

func newRecordingTestService() *Service {
	return &Service{
		config: &Config{
			JWTSecret: "test-secret",
			JWTExpiry: time.Hour,
		},
	}
}

func TestGenerateTokenWithGroups_RecordsObservedGroups(t *testing.T) {
	t.Parallel()
	svc := newRecordingTestService()
	rec := newFakeRecorder(nil)
	svc.SetObservedGroupsStore(rec)

	groups := []string{"alpha-devs", "beta-ops"}
	token, _, err := svc.GenerateTokenWithGroups("user-1", "u@test.local", "User", groups)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}
	if token == "" {
		t.Fatal("expected a token")
	}

	select {
	case got := <-rec.called:
		if len(got) != 2 || got[0] != "alpha-devs" || got[1] != "beta-ops" {
			t.Errorf("recorded groups = %v, want %v", got, groups)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Record was not called within timeout")
	}
}

func TestGenerateTokenWithGroups_RecorderErrorDoesNotFailLogin(t *testing.T) {
	t.Parallel()
	svc := newRecordingTestService()
	rec := newFakeRecorder(errors.New("redis down"))
	svc.SetObservedGroupsStore(rec)

	token, _, err := svc.GenerateTokenWithGroups("user-1", "u@test.local", "User", []string{"alpha-devs"})
	if err != nil {
		t.Fatalf("token generation must not fail when recorder errors: %v", err)
	}
	if token == "" {
		t.Fatal("expected a token despite recorder error")
	}

	// The recorder is still invoked (and its error swallowed/logged).
	select {
	case <-rec.called:
	case <-time.After(2 * time.Second):
		t.Fatal("Record was not called within timeout")
	}
}

func TestGenerateTokenWithGroups_NoRecordingWhenGroupsEmpty(t *testing.T) {
	t.Parallel()
	svc := newRecordingTestService()
	rec := newFakeRecorder(nil)
	svc.SetObservedGroupsStore(rec)

	if _, _, err := svc.GenerateTokenWithGroups("user-1", "u@test.local", "User", nil); err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}

	// Give any (erroneously-spawned) goroutine a chance to fire.
	time.Sleep(100 * time.Millisecond)
	if n := rec.callCount(); n != 0 {
		t.Errorf("Record called %d times for empty groups, want 0", n)
	}
}

func TestGenerateTokenWithGroups_NoRecordingWhenStoreNil(t *testing.T) {
	t.Parallel()
	svc := newRecordingTestService()
	// observedGroups intentionally left nil.

	token, _, err := svc.GenerateTokenWithGroups("user-1", "u@test.local", "User", []string{"alpha-devs"})
	if err != nil {
		t.Fatalf("GenerateTokenWithGroups() error = %v", err)
	}
	if token == "" {
		t.Fatal("expected a token with nil recorder")
	}
}
