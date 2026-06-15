// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // register pgx5 driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

// NormalizeDSN converts "postgres://" or "postgresql://" URL prefixes to the
// "pgx5://" scheme required by golang-migrate's pgx5 driver. Other schemes pass through.
func NormalizeDSN(dsn string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(dsn, prefix) {
			return "pgx5://" + strings.TrimPrefix(dsn, prefix)
		}
	}
	return dsn
}

// runMigrations acquires a session-level advisory lock on AdvisoryLockKey, merges
// the provided migration sources (by numeric filename prefix), applies all pending
// migrations, and logs the resulting schema version. The lock is released
// automatically when lockConn closes (session-level lock).
func runMigrations(ctx context.Context, dsn string, sources []MigrationSource, logger *slog.Logger) error {
	lockConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect for migration lock: %w", err)
	}
	defer lockConn.Close(ctx)

	if _, err := lockConn.Exec(ctx, "SELECT pg_advisory_lock($1)", AdvisoryLockKey); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}
	logger.Info("migration advisory lock acquired", "lock_key", AdvisoryLockKey)

	merged, err := mergeSources(sources)
	if err != nil {
		return fmt.Errorf("merge migration sources: %w", err)
	}

	src, err := iofs.New(merged, ".")
	if err != nil {
		return fmt.Errorf("create migrations source: %w", err)
	}
	// NOTE: do not close src here — m.Close() owns the source driver.

	m, err := migrate.NewWithSourceInstance("iofs", src, NormalizeDSN(dsn))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			logger.Warn("migrate close errors", "src", srcErr, "db", dbErr)
		}
	}()

	// golang-migrate's m.Up() does NOT honor ctx; it watches m.GracefulStop instead.
	// Bridge ctx cancellation to GracefulStop so SIGTERM during a long migration
	// stops the migrator after the current statement (FR-PG6 graceful shutdown).
	upDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			select {
			case m.GracefulStop <- true:
			case <-upDone:
			}
		case <-upDone:
		}
	}()

	upErr := m.Up()
	close(upDone)
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", upErr)
	}
	if v, dirty, verErr := m.Version(); verErr == nil {
		logger.Info("migration complete", "version", v, "dirty", dirty)
	}
	return nil
}
