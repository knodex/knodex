// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"testing"

	"github.com/knodex/knodex/server/internal/audit"
	"github.com/knodex/knodex/server/internal/kagent/runs"
)

// stubRunStore is a minimal runs.Store double for dispatch assertions.
type stubRunStore struct{}

func (stubRunStore) Create(context.Context, *runs.Run) error               { return nil }
func (stubRunStore) Update(context.Context, *runs.Run) error               { return nil }
func (stubRunStore) List(context.Context, runs.Filter) ([]runs.Run, error) { return nil, nil }

// stubRecorder is a minimal audit.Recorder double.
type stubRecorder struct{}

func (stubRecorder) Record(context.Context, audit.Event) {}

// TestWrapAgentRunStore_OSS pins the OSS no-Postgres guarantee: the wrap is
// the identity function regardless of the recorder argument — the run store
// passes through untouched and no EE code is linked.
func TestWrapAgentRunStore_OSS(t *testing.T) {
	inner := stubRunStore{}

	if got := WrapAgentRunStore(inner, nil); got != runs.Store(inner) {
		t.Error("OSS wrap with nil recorder should return the inner store identically")
	}
	if got := WrapAgentRunStore(inner, stubRecorder{}); got != runs.Store(inner) {
		t.Error("OSS wrap should return the inner store identically even with a recorder")
	}
	if got := WrapAgentRunStore(nil, nil); got != nil {
		t.Errorf("OSS wrap with nil inner should return nil, got %T", got)
	}
}
