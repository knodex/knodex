// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package main provides the OSS (non-enterprise) database initialization.
//
// Story 15.2 / R5-5: Postgres is now mandatory on EVERY edition. OSS gains a
// canonical persistent user roster (identity.users + identity.federated_identities)
// materialized on every OIDC login, so the OSS binary now opens a pgx pool and
// runs the SHARED migration source (identity schema only — no audit/compliance/
// license tables). The process fails fast if DATABASE_URL is unset or the pool
// cannot be established (AC22).
package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/knodex/knodex/server/internal/config"
	db "github.com/knodex/knodex/server/internal/database"
)

// ossDBInitTimeout caps the budget for connection + migration during OSS startup.
const ossDBInitTimeout = 60 * time.Second

// InitDatabaseManager initializes the edition-neutral PostgreSQL manager for OSS:
// validates DATABASE_URL, runs the shared schema migrations under an advisory
// lock, and opens the identity connection pool. Returns an error (→ non-zero
// exit) when DATABASE_URL is unset or Postgres is unreachable (AC22 fail-fast).
func InitDatabaseManager(ctx context.Context, cfg *config.Config) (io.Closer, error) {
	return initDatabaseManager(ctx, cfg, db.NewManager)
}

// managerCtor matches db.NewManager's signature so tests can inject a fake.
type managerCtor func(ctx context.Context, cfg db.Config) (*db.Manager, error)

func initDatabaseManager(ctx context.Context, cfg *config.Config, newManager managerCtor) (io.Closer, error) {
	if cfg.Database.URL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required (R5-5: Postgres is now the durable store for the canonical user roster on every edition); set DATABASE_URL, e.g. postgres://user:pass@host:5432/dbname?sslmode=require")
	}

	initCtx, cancel := context.WithTimeout(ctx, ossDBInitTimeout)
	defer cancel()

	dbCfg := db.Config{
		URL:              cfg.Database.URL,
		MigrationSources: []db.MigrationSource{db.SharedMigrationSource()},
	}
	m, err := newManager(initCtx, dbCfg)
	if err != nil {
		return nil, fmt.Errorf("database manager initialization failed: %w", err)
	}

	db.RegisterManager(m)
	return m, nil
}
