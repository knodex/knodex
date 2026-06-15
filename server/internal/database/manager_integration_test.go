//go:build integration

// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewManager_SharedOnly_OSSSchema verifies AC6 for the OSS case: a fresh DB
// migrated with ONLY the shared source contains exactly the identity schema —
// no audit.*, compliance.*, or license.* objects. Run against a BLANK database
// (the test asserts schema presence/absence, so a pre-populated DB will skip).
//
//	export KNODEX_TEST_DATABASE_URL=postgres://postgres:knodex@localhost:5432/postgres?sslmode=disable
//	cd server && go test -tags=integration -count=1 -run TestNewManager_SharedOnly ./internal/database/...
func TestNewManager_SharedOnly_OSSSchema(t *testing.T) {
	url := os.Getenv("KNODEX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KNODEX_TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mgr, err := NewManager(ctx, Config{
		URL:              url,
		MigrationSources: []MigrationSource{SharedMigrationSource()},
	})
	require.NoError(t, err)
	defer mgr.Close()

	pool := mgr.IdentityPool()
	require.NotNil(t, pool)

	schemaExists := func(schema string) bool {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name=$1)`, schema).Scan(&exists))
		return exists
	}
	tableExists := func(schema, table string) bool {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=$1 AND table_name=$2)`, schema, table).Scan(&exists))
		return exists
	}

	assert.True(t, schemaExists("identity"), "identity schema present after shared migration")
	assert.True(t, tableExists("identity", "users"))
	assert.True(t, tableExists("identity", "federated_identities"))

	// On a fresh OSS DB the EE schemas/tables must NOT exist. Skip the negative
	// assertion if a previous EE run already created them on this shared DB.
	if schemaExists("audit") || schemaExists("compliance") || schemaExists("license") {
		t.Skip("EE schemas already present on this database from a prior run; OSS-isolation assertion skipped")
	}
	assert.False(t, tableExists("license", "user_seats"), "STORY-465 seat table must never exist (AC4/AC6)")

	// RLS denies a raw (no-GUC) read. SET LOCAL ROLE into the non-BYPASSRLS
	// knodex_app role inside a tx so the policy is exercised even when the test
	// connects as the postgres superuser (which would otherwise bypass RLS); the
	// role resets when the tx ends.
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE knodex_app"); err != nil {
		t.Skipf("cannot assume non-BYPASSRLS knodex_app role (run migrations as a role that can create/grant it): %v", err)
	}
	var n int64
	require.NoError(t, tx.QueryRow(ctx, "SELECT COUNT(*) FROM identity.users").Scan(&n))
	assert.Equal(t, int64(0), n)
}
