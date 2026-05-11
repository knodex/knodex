// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"strings"
	"testing"

	"github.com/knodex/knodex/server/internal/config"
)

// TestRunMigrationsOnly_OSSReturnsError verifies the OSS stub returns the
// documented error string. This is the contract the Helm chart README points
// operators at when they accidentally deploy the OSS image with a
// --migrate-only Job — they should see a clear "rebuild with -tags=enterprise"
// message in the Job logs, not a silent no-op.
func TestRunMigrationsOnly_OSSReturnsError(t *testing.T) {
	err := RunMigrationsOnly(context.Background(), &config.Config{})
	if err == nil {
		t.Fatal("expected error in OSS build, got nil")
	}
	if !strings.Contains(err.Error(), "--migrate-only requires an enterprise build") {
		t.Errorf("error must mention enterprise-build requirement, got: %v", err)
	}
	if !strings.Contains(err.Error(), "-tags=enterprise") {
		t.Errorf("error must include the rebuild hint, got: %v", err)
	}
}
