// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS (non-enterprise) migrate-only entrypoint.
//
// Story 15.2 / R5-5: Postgres is now mandatory on EVERY edition, so OSS also runs
// the shared identity-schema migration. This entrypoint lets a Helm
// pre-install/pre-upgrade Job prepare the OSS database before the server
// Deployment rolls out (mirroring the EE path in ee_migrate.go, but with only the
// shared migration source — no audit/compliance/license tables).
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/knodex/knodex/server/internal/config"
	db "github.com/knodex/knodex/server/internal/database"
)

// RunMigrationsOnly applies all pending OSS migrations (the shared identity
// schema) under the same advisory lock as normal startup, then exits.
func RunMigrationsOnly(ctx context.Context, cfg *config.Config) error {
	return runMigrationsWithCtor(ctx, cfg, db.NewManager)
}

// runMigrationsWithCtor is the testable implementation. It reuses
// initDatabaseManager from oss_database.go (which supplies the shared migration
// source and fails fast on a missing DATABASE_URL), then closes the manager
// immediately — --migrate-only does not boot the HTTP server.
func runMigrationsWithCtor(
	ctx context.Context,
	cfg *config.Config,
	ctor managerCtor,
) error {
	closer, err := initDatabaseManager(ctx, cfg, ctor)
	if err != nil {
		return fmt.Errorf("migrate-only: %w", err)
	}
	if closer != nil {
		if cerr := closer.Close(); cerr != nil {
			// Migrations have already succeeded; close failure is not fatal.
			slog.Warn("migrate-only: failed to close database manager", "error", cerr)
		}
	}
	return nil
}
