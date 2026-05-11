// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS (non-enterprise) migrate-only entrypoint stub.
// Postgres is enterprise-only; OSS builds reject --migrate-only at startup so
// operators get a clear, actionable error instead of a silent no-op.
package main

import (
	"context"
	"errors"

	"github.com/knodex/knodex/server/internal/config"
)

// RunMigrationsOnly is the OSS stub for the --migrate-only entrypoint. The
// real implementation lives in ee_migrate.go behind the enterprise build tag.
func RunMigrationsOnly(_ context.Context, _ *config.Config) error {
	return errors.New("--migrate-only requires an enterprise build (Postgres is enterprise-only); rebuild with -tags=enterprise")
}
