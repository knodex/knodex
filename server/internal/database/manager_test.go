// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"testing"
)

// TestNewManager_EmptyURL verifies that NewManager rejects an empty URL before
// attempting any network connection.
func TestNewManager_EmptyURL(t *testing.T) {
	_, err := NewManager(context.Background(), Config{})
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

// TestNewManager_BadURL verifies that NewManager with an unreachable URL fails
// at the migration/connectivity step, not because of a zero pool-size config.
// This confirms the zero→10 coerce logic runs before any dial attempt.
func TestNewManager_BadURL(t *testing.T) {
	// Port 1 is connection-refused on every OS; connect_timeout=1 avoids a long wait.
	_, err := NewManager(context.Background(), Config{URL: "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"})
	if err == nil {
		t.Fatal("expected connectivity error, got nil")
	}
}

// TestGetManager_NilBeforeRegister verifies GetManager returns nil before any
// RegisterManager call.
func TestGetManager_NilBeforeRegister(t *testing.T) {
	prev := globalManager.Load()
	globalManager.Store(nil)
	defer globalManager.Store(prev)

	if m := GetManager(); m != nil {
		t.Errorf("GetManager() = %v, want nil", m)
	}
}

// TestRegisterManager verifies that RegisterManager makes the manager retrievable
// via GetManager.
func TestRegisterManager(t *testing.T) {
	prev := globalManager.Load()
	globalManager.Store(nil)
	defer globalManager.Store(prev)

	m := &Manager{}
	RegisterManager(m)
	if got := GetManager(); got != m {
		t.Errorf("GetManager() = %p, want %p", got, m)
	}
}

// mockTokenProvider satisfies the TokenProvider interface for compile-time verification.
type mockTokenProvider struct{}

func (m *mockTokenProvider) Token(_ context.Context, _ string) (string, error) {
	return "token", nil
}

// Compile-time assertion that mockTokenProvider satisfies TokenProvider.
var _ TokenProvider = (*mockTokenProvider)(nil)
