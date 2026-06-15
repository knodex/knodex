// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CheckoutWithOrg acquires a connection from pool, begins a transaction, applies
// SET LOCAL app.org_id = orgID (scoped to the transaction, enforces RLS policies),
// calls fn, then commits or rolls back.
//
// If orgID is empty, returns an error without touching the pool (defense-in-depth).
// SET LOCAL is transaction-scoped and safe with connection pooling — it does not bleed
// across requests.
func CheckoutWithOrg(ctx context.Context, pool *pgxpool.Pool, orgID string, fn func(tx pgx.Tx) error) error {
	if orgID == "" {
		return fmt.Errorf("orgID must not be empty: RLS requires app.org_id to be set")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire db connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// set_config is the parameterizable equivalent of SET LOCAL — PostgreSQL's SET
	// command does not accept $N placeholders; set_config(name, value, is_local=true) does.
	if _, err := tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", orgID); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("set app.org_id: %w", err)
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
