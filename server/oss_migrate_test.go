// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/knodex/knodex/server/internal/config"
)

// TestRunMigrationsOnly_OSSRequiresDatabaseURL verifies the OSS --migrate-only
// entrypoint fails fast with a clear DATABASE_URL message when no DSN is
// configured. As of Story 15.2 / R5-5, Postgres is mandatory on EVERY edition,
// so OSS legitimately runs the shared identity migration — the error operators
// see in the migrate Job logs is the missing-DSN guard, not an enterprise-build
// requirement (that obsolete contract was removed when OSS gained the database).
func TestRunMigrationsOnly_OSSRequiresDatabaseURL(t *testing.T) {
	err := RunMigrationsOnly(context.Background(), &config.Config{})
	if err == nil {
		t.Fatal("expected error in OSS build without DATABASE_URL, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Errorf("error must mention the DATABASE_URL requirement, got: %v", err)
	}
	if !strings.Contains(err.Error(), "migrate-only") {
		t.Errorf("error must be scoped to the migrate-only path, got: %v", err)
	}
}
