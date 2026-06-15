// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestCheckoutWithOrg_EmptyOrgID verifies that an empty orgID is rejected before
// any pool or transaction interaction (defense-in-depth guard).
// Passing nil for pool is safe because the guard fires before Acquire is called.
func TestCheckoutWithOrg_EmptyOrgID(t *testing.T) {
	err := CheckoutWithOrg(context.Background(), nil, "", func(tx pgx.Tx) error {
		t.Fatal("fn must not be called when orgID is empty")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for empty orgID, got nil")
	}
}
