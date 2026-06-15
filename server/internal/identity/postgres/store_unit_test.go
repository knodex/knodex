// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Unit tests that need no database — they run in the default (non-integration)
// suite so CI without Postgres still exercises the port contract, the reserved
// verbs, and the cursor codec.
package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/knodex/knodex/server/internal/services"
)

// TestStoreImplementsPort is a compile-time + runtime assertion that *Store
// satisfies the IdentityService port (AC8/AC13).
func TestStoreImplementsPort(t *testing.T) {
	var _ services.IdentityService = New(nil, Config{})
}

// TestReservedVerbsReturnNotImplemented covers AC13: Provision/Deactivate are
// present on the base store and return ErrNotImplemented until a caller lands.
func TestReservedVerbsReturnNotImplemented(t *testing.T) {
	s := New(nil, Config{OrgID: "x"})
	_, err := s.Provision(context.Background(), services.ProvisionParams{})
	assert.ErrorIs(t, err, services.ErrNotImplemented)
	assert.ErrorIs(t, s.Deactivate(context.Background(), services.UserID("x")), services.ErrNotImplemented)
}

// TestDefaultSourceKind verifies the store defaults SourceKind to oidc_jit when
// the composition root leaves it blank (AC8).
func TestDefaultSourceKind(t *testing.T) {
	s := New(nil, Config{})
	assert.Equal(t, services.SourceKindOIDCJIT, s.sourceKind)
	s2 := New(nil, Config{SourceKind: services.SourceKindKeycloakProjection})
	assert.Equal(t, services.SourceKindKeycloakProjection, s2.sourceKind)
}

// TestCursorRoundTrip exercises the keyset cursor codec and malformed-token path
// (AC12) without a database.
func TestCursorRoundTrip(t *testing.T) {
	in := listCursor{LastSeenAt: time.Now().UTC().Truncate(time.Second), ID: "01ABCDEF000000000000000000"}
	tok := encodeCursor(in)
	out, err := decodeCursor(tok)
	assert.NoError(t, err)
	assert.Equal(t, in.ID, out.ID)
	assert.True(t, in.LastSeenAt.Equal(out.LastSeenAt))

	_, err = decodeCursor("!!!not base64!!!")
	assert.ErrorIs(t, err, services.ErrInvalidPageToken)

	_, err = decodeCursor(encodeCursor(listCursor{ID: ""}))
	assert.ErrorIs(t, err, services.ErrInvalidPageToken)
}
