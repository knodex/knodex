// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"testing"

	"github.com/knodex/knodex/server/internal/services"
)

// TestIdentitySourceKind_Default asserts the composition-root constant selects
// "oidc_jit". It needs no database, so it runs inside `make verify-build-matrix`.
func TestIdentitySourceKind_Default(t *testing.T) {
	if IdentitySourceKind != services.SourceKindOIDCJIT {
		t.Fatalf("build must select IdentitySourceKind=%q, got %q",
			services.SourceKindOIDCJIT, IdentitySourceKind)
	}
}
