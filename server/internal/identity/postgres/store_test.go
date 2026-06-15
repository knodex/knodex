//go:build integration

// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Integration tests for the edition-neutral identity store (Story 15.2 AC14/AC15).
// They require Postgres reachable via KNODEX_TEST_DATABASE_URL and the SHARED
// migration source runs against a BLANK database (the migration tracks state in
// schema_migrations, so point each run at a freshly-created database to avoid
// stale-version skips across test binaries).
//
// The connecting role may be the postgres superuser — TestRLSRejectsRawConnection
// SET LOCAL ROLEs into the non-BYPASSRLS knodex_app role itself, so it validates
// the NFR-U13 RLS guarantee regardless of the login role (superusers/BYPASSRLS
// roles otherwise bypass even FORCE ROW LEVEL SECURITY).
//
//	docker run --rm -d --name knodex-pg -e POSTGRES_PASSWORD=knodex -p 5432:5432 postgres:16
//	export KNODEX_TEST_DATABASE_URL=postgres://postgres:knodex@localhost:5432/postgres?sslmode=disable
//	cd server && go test -tags=integration -count=1 ./internal/identity/postgres/...
package postgres

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/knodex/knodex/server/internal/database"
	"github.com/knodex/knodex/server/internal/services"
)

// testManager runs the shared migrations and returns a manager whose IdentityPool
// backs the store under test.
//
// NOTE on RLS fidelity: org isolation is enforced by Postgres RLS, which is
// bypassed by superuser/BYPASSRLS connection roles (and KNODEX_TEST_DATABASE_URL
// usually points at the postgres superuser). Single-org functional tests do not
// depend on RLS (the store scopes via CheckoutWithOrg + explicit predicates), so
// they pass under any role. The RLS-specific assertions (TestRLSRejectsRawConnection,
// the cross-org-hiding check in TestCrossOrgIsolation) validate the policy directly
// by SET LOCAL ROLE-ing into the non-BYPASSRLS knodex_app role on a raw connection,
// so they exercise RLS faithfully regardless of the login role.
func testManager(t *testing.T) *db.Manager {
	t.Helper()
	url := os.Getenv("KNODEX_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KNODEX_TEST_DATABASE_URL not set; skipping Postgres integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mgr, err := db.NewManager(ctx, db.Config{
		URL:              url,
		MigrationSources: []db.MigrationSource{db.SharedMigrationSource()},
	})
	require.NoError(t, err, "NewManager (shared migrations)")
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr
}

// assertRLSHidesRow opens a raw connection, SET LOCAL ROLEs into the non-BYPASSRLS
// knodex_app role, scopes app.org_id to scopeOrg, and asserts the given user id is
// invisible — i.e. the RLS policy hides cross-org rows. This validates the same
// guarantee the store's GetByID relies on, but works even when the pool connects
// as a superuser (which would otherwise bypass RLS).
func assertRLSHidesRow(t *testing.T, pool *pgxpool.Pool, scopeOrg string, hiddenID services.UserID) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE knodex_app"); err != nil {
		t.Skipf("cannot assume non-BYPASSRLS knodex_app role (run migrations as a role that can create/grant it): %v", err)
	}
	_, err = tx.Exec(ctx, "SELECT set_config('app.org_id', $1, true)", scopeOrg)
	require.NoError(t, err)
	var n int64
	require.NoError(t, tx.QueryRow(ctx,
		"SELECT COUNT(*) FROM identity.users WHERE id = $1", string(hiddenID)).Scan(&n))
	assert.Equal(t, int64(0), n, "RLS must hide a cross-org row from a knodex_app connection scoped to %s", scopeOrg)
}

// truncateIdentity clears both identity tables for the given org via CheckoutWithOrg.
func truncateIdentity(t *testing.T, pool *pgxpool.Pool, org string) {
	t.Helper()
	require.NoError(t, db.CheckoutWithOrg(context.Background(), pool, org, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), "DELETE FROM identity.federated_identities"); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), "DELETE FROM identity.users")
		return err
	}))
}

func newStore(pool *pgxpool.Pool, org string, hooks services.IdentityHooks) *Store {
	return New(pool, Config{OrgID: org, Hooks: hooks})
}

func TestObserveLogin_FirstThenSubsequent(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-first"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	p := services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-1", Email: "Alice@Example.com", DisplayName: "Alice", EmailVerified: true}

	r1, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	assert.True(t, r1.Created, "first login creates")
	assert.NotEmpty(t, r1.ID)

	u, err := s.GetByID(ctx, r1.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", u.Email, "email lowercased on insert")
	first := u.FirstSeenAt

	time.Sleep(5 * time.Millisecond)
	r2, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	assert.False(t, r2.Created, "second login does not create")
	assert.Equal(t, r1.ID, r2.ID, "stable id")

	u2, err := s.GetByID(ctx, r1.ID)
	require.NoError(t, err)
	assert.Equal(t, first, u2.FirstSeenAt, "first_seen_at never changes")
	assert.True(t, u2.LastSeenAt.After(first) || u2.LastSeenAt.Equal(first), "last_seen_at advanced")
}

func TestObserveLogin_VerifiedEmailChange(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-emailchange"
	truncateIdentity(t, pool, org)
	ctx := context.Background()

	var changed bool
	var oldSeen string
	s := newStore(pool, org, services.IdentityHooks{
		OnEmailChanged: func(_ context.Context, _ *services.UserRecord, old string) error {
			changed, oldSeen = true, old
			return nil
		},
	})

	p := services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-1", Email: "a@x.com", DisplayName: "A", EmailVerified: true}
	r, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)

	p.Email = "new@x.com"
	r2, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	assert.True(t, r2.EmailChanged, "verified email change flagged")
	assert.True(t, changed, "OnEmailChanged hook fired")
	assert.Equal(t, "a@x.com", oldSeen, "hook saw old email")

	u, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "new@x.com", u.Email)
}

func TestObserveLogin_UnverifiedEmailDivergencePreserved(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-unverified"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	p := services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-1", Email: "a@x.com", DisplayName: "A", EmailVerified: true}
	r, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)

	// Same sub, different email, but UNVERIFIED — must not overwrite.
	p.Email = "evil@x.com"
	p.EmailVerified = false
	r2, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	assert.False(t, r2.EmailChanged)

	u, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "a@x.com", u.Email, "unverified divergence preserves stored email")
}

func TestObserveLogin_DisplayNameChange(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-display"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	p := services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-1", Email: "a@x.com", DisplayName: "Old", EmailVerified: true}
	r, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)

	p.DisplayName = "New"
	_, err = s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	u, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", u.DisplayName)

	// Empty display name does not blank the stored value (COALESCE/NULLIF).
	p.DisplayName = ""
	_, err = s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	u, err = s.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", u.DisplayName)
}

func TestRemoveAndResurrect(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-resurrect"
	truncateIdentity(t, pool, org)
	ctx := context.Background()

	var removedFired, resurrectFired bool
	s := newStore(pool, org, services.IdentityHooks{
		OnRemoved:     func(_ context.Context, _ *services.UserRecord) error { removedFired = true; return nil },
		OnResurrected: func(_ context.Context, _ *services.UserRecord) error { resurrectFired = true; return nil },
	})

	p := services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-1", Email: "a@x.com", DisplayName: "A", EmailVerified: true}
	r, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)

	require.NoError(t, s.Remove(ctx, r.ID))
	assert.True(t, removedFired)
	u, err := s.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, services.UserStateRemoved, u.State)

	// Idempotent remove.
	removedFired = false
	require.NoError(t, s.Remove(ctx, r.ID))
	assert.False(t, removedFired, "idempotent remove fires no hook")

	// Remove of unknown id → ErrUserNotFound.
	assert.ErrorIs(t, s.Remove(ctx, services.UserID("does-not-exist")), services.ErrUserNotFound)

	// Login resurrects.
	r2, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	assert.True(t, r2.Resurrected)
	assert.True(t, resurrectFired)
	u, err = s.GetByID(ctx, r.ID)
	require.NoError(t, err)
	assert.Equal(t, services.UserStateActive, u.State)
}

func TestObserveLogin_SCIMReconcile(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-scim"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	// Pre-insert a SCIM-pushed user: a users row + a federation row with
	// sub=NULL, external_id=X, source_kind='scim_push'.
	const externalID = "ext-abc"
	var preID string
	require.NoError(t, db.CheckoutWithOrg(ctx, pool, org, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO identity.users (id, org_id, email, display_name, state)
			 VALUES ('01SCIMUSER000000000000000A', $1, 'scim@x.com', 'Scim', 'active') RETURNING id`,
			org).Scan(&preID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO identity.federated_identities
			   (org_id, issuer, sub, external_id, source_connector_id, internal_user_id, provider_kind, source_kind)
			 VALUES ($1, 'https://idp', NULL, $2, 'conn-1', $3, 'scim', 'scim_push')`,
			org, externalID, preID)
		return err
	}))

	// First OIDC login whose sub == external_id reconciles (no new user).
	p := services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: externalID, Email: "scim@x.com", DisplayName: "Scim", EmailVerified: true}
	r, err := s.ObserveLogin(ctx, p)
	require.NoError(t, err)
	assert.False(t, r.Created, "SCIM-provisioned user is reconciled, not created")
	assert.Equal(t, services.UserID(preID), r.ID)

	// Exactly one user remains; the federation row's sub is backfilled.
	n, err := s.BilledSeatCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	var sub string
	require.NoError(t, db.CheckoutWithOrg(ctx, pool, org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT sub FROM identity.federated_identities WHERE external_id=$1`, externalID).Scan(&sub)
	}))
	assert.Equal(t, externalID, sub, "sub backfilled to the login sub")
}

func TestBilledSeatCount_ActiveOnly(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-seats"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	mk := func(sub string) services.UserID {
		r, err := s.ObserveLogin(ctx, services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: sub, Email: sub + "@x.com", EmailVerified: true})
		require.NoError(t, err)
		return r.ID
	}
	mk("u1")
	id2 := mk("u2")
	mk("u3")

	n, err := s.BilledSeatCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	require.NoError(t, s.Remove(ctx, id2))
	n, err = s.BilledSeatCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "removed users drop out of the billed count")

	// Age does not matter (entitlement, not window) — backdate last_seen_at far past.
	require.NoError(t, db.CheckoutWithOrg(ctx, pool, org, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE identity.users SET last_seen_at = now() - interval '400 days' WHERE state='active'`)
		return err
	}))
	n, err = s.BilledSeatCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "BilledSeatCount is unaffected by last_seen_at age")
}

func TestCrossOrgIsolation(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const orgA, orgB = "it-orgA", "it-orgB"
	truncateIdentity(t, pool, orgA)
	truncateIdentity(t, pool, orgB)
	ctx := context.Background()
	sA := newStore(pool, orgA, services.IdentityHooks{})
	sB := newStore(pool, orgB, services.IdentityHooks{})

	pA := services.ObserveLoginParams{OrgID: orgA, Issuer: "https://idp", Sub: "same-sub", Email: "a@x.com", EmailVerified: true}
	pB := pA
	pB.OrgID = orgB

	rA, err := sA.ObserveLogin(ctx, pA)
	require.NoError(t, err)
	rB, err := sB.ObserveLogin(ctx, pB)
	require.NoError(t, err)
	assert.NotEqual(t, rA.ID, rB.ID, "same (issuer,sub) in two orgs → two distinct ULIDs")

	// Each org sees exactly one.
	nA, _ := sA.BilledSeatCount(ctx)
	nB, _ := sB.BilledSeatCount(ctx)
	assert.Equal(t, int64(1), nA)
	assert.Equal(t, int64(1), nB)

	// The core RLS guarantee: org A's connection cannot see org B's row. Validated
	// directly against the policy (robust even when the pool connects as superuser).
	assertRLSHidesRow(t, pool, orgA, rB.ID)

	// Cross-org GetByID maps that hidden row to ErrUserNotFound (never 403). This
	// goes through the store pool, which bypasses RLS if it connects as a
	// superuser/BYPASSRLS role; only assert the strict mapping when RLS is enforced
	// on that connection (the guarantee itself is already covered above).
	_, err = sA.GetByID(ctx, rB.ID)
	if rlsEnforced(t, pool) {
		assert.ErrorIs(t, err, services.ErrUserNotFound, "RLS must hide cross-org rows")
	} else {
		t.Log("store pool bypasses RLS (superuser/BYPASSRLS); strict GetByID mapping covered by assertRLSHidesRow above")
	}
}

// rlsEnforced reports whether RLS policies apply to the pool's current connection
// role (i.e. it is neither a superuser nor a BYPASSRLS role).
func rlsEnforced(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var bypass bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypass))
	return !bypass
}

func TestRLSRejectsRawConnection(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-rls"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})
	_, err := s.ObserveLogin(ctx, services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "u1", Email: "a@x.com", EmailVerified: true})
	require.NoError(t, err)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	// Validate RLS enforcement explicitly. RLS (even FORCE) is bypassed by
	// superusers and BYPASSRLS roles, and tests routinely connect as the postgres
	// superuser — so a bare COUNT would NOT exercise the policy. SET LOCAL ROLE to
	// the non-BYPASSRLS knodex_app role (provisioned by migration 0005) inside a tx
	// so the policy applies regardless of the login role, and the role resets when
	// the tx ends (no pooled-connection poisoning). With app.org_id unset, the
	// NULLIF(...) policy yields NULL and must hide every row.
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE knodex_app"); err != nil {
		t.Skipf("cannot assume non-BYPASSRLS knodex_app role (run migrations as a role that can create/grant it): %v", err)
	}
	var n int64
	require.NoError(t, tx.QueryRow(ctx, "SELECT COUNT(*) FROM identity.users").Scan(&n))
	assert.Equal(t, int64(0), n, "RLS hides all rows from knodex_app when app.org_id is unset")
}

func TestGetByFederation(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-fed"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	r, err := s.ObserveLogin(ctx, services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "fed-sub", Email: "a@x.com", EmailVerified: true})
	require.NoError(t, err)

	u, err := s.GetByFederation(ctx, org, "https://idp", "fed-sub")
	require.NoError(t, err)
	assert.Equal(t, r.ID, u.ID)

	_, err = s.GetByFederation(ctx, org, "https://idp", "nope")
	assert.ErrorIs(t, err, services.ErrUserNotFound)
}

func TestListCursorRoundTrip(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-list"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	for i := 0; i < 5; i++ {
		sub := string(rune('a' + i))
		_, err := s.ObserveLogin(ctx, services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: sub, Email: sub + "@x.com", EmailVerified: true})
		require.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	seen := map[string]bool{}
	page1, next, err := s.List(ctx, services.ListOpts{PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.NotEmpty(t, next)
	for _, u := range page1 {
		seen[string(u.ID)] = true
	}

	page2, _, err := s.List(ctx, services.ListOpts{PageSize: 2, PageToken: next})
	require.NoError(t, err)
	for _, u := range page2 {
		assert.False(t, seen[string(u.ID)], "cursor does not repeat rows")
		seen[string(u.ID)] = true
	}
	assert.GreaterOrEqual(t, len(seen), 4)

	// Malformed cursor.
	_, _, err = s.List(ctx, services.ListOpts{PageSize: 2, PageToken: "!!!not-base64!!!"})
	assert.ErrorIs(t, err, services.ErrInvalidPageToken)
}

func TestFederatedIdentitiesFor(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	const org = "it-fedfor"
	truncateIdentity(t, pool, org)
	ctx := context.Background()
	s := newStore(pool, org, services.IdentityHooks{})

	// Two users in this org, each with one OIDC federation row.
	r1, err := s.ObserveLogin(ctx, services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-1", Email: "a@x.com", EmailVerified: true})
	require.NoError(t, err)
	r2, err := s.ObserveLogin(ctx, services.ObserveLoginParams{OrgID: org, Issuer: "https://idp", Sub: "sub-2", Email: "b@x.com", EmailVerified: true})
	require.NoError(t, err)

	// Empty ids → empty map, no query.
	empty, err := s.FederatedIdentitiesFor(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Batch fetch for both users in one call.
	got, err := s.FederatedIdentitiesFor(ctx, []services.UserID{r1.ID, r2.ID})
	require.NoError(t, err)
	require.Len(t, got, 2, "both users present in the batch result")

	require.Len(t, got[r1.ID], 1, "user 1 has exactly one federated identity")
	fi := got[r1.ID][0]
	assert.Equal(t, org, fi.OrgID)
	assert.Equal(t, "https://idp", fi.Issuer)
	assert.Equal(t, "sub-1", fi.Sub)
	assert.Equal(t, r1.ID, fi.InternalUserID)
	assert.Equal(t, "oidc", fi.ProviderKind)
	assert.Equal(t, services.SourceKindOIDCJIT, fi.SourceKind)
	assert.Empty(t, fi.ExternalID, "OIDC rows have no external_id")
	assert.False(t, fi.CreatedAt.IsZero())

	require.Len(t, got[r2.ID], 1, "user 2 has exactly one federated identity")

	// RLS scoping: a store bound to a different org sees none of these rows.
	const otherOrg = "it-fedfor-other"
	truncateIdentity(t, pool, otherOrg)
	sOther := newStore(pool, otherOrg, services.IdentityHooks{})
	if rlsEnforced(t, pool) {
		isolated, err := sOther.FederatedIdentitiesFor(ctx, []services.UserID{r1.ID, r2.ID})
		require.NoError(t, err)
		assert.Empty(t, isolated, "RLS hides cross-org federated identities")
	} else {
		t.Log("store pool bypasses RLS (superuser/BYPASSRLS); cross-org isolation covered by TestCrossOrgIsolation")
	}
}

// TestIdentitySchemaPIIFloor introspects information_schema and asserts the EXACT
// column set on both identity tables (AC15). external_id / source_connector_id
// are IdP-side opaque ids — same privacy class as sub. Forbidden PII columns
// (ip_address, user_agent, session_id, last_login_ip, phone, mfa_factor) must be
// absent.
func TestIdentitySchemaPIIFloor(t *testing.T) {
	mgr := testManager(t)
	pool := mgr.IdentityPool()
	ctx := context.Background()

	cols := func(table string) []string {
		var out []string
		require.NoError(t, db.CheckoutWithOrg(ctx, pool, "pii-floor", func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx,
				`SELECT column_name FROM information_schema.columns
				 WHERE table_schema='identity' AND table_name=$1 ORDER BY column_name`, table)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var c string
				if err := rows.Scan(&c); err != nil {
					return err
				}
				out = append(out, c)
			}
			return rows.Err()
		}))
		sort.Strings(out)
		return out
	}

	wantUsers := []string{"display_name", "email", "first_seen_at", "id", "last_seen_at", "org_id", "state"}
	sort.Strings(wantUsers)
	assert.Equal(t, wantUsers, cols("users"), "identity.users exact column set")

	wantFed := []string{"created_at", "external_id", "internal_user_id", "issuer", "org_id", "provider_kind", "source_connector_id", "source_kind", "sub", "updated_at"}
	sort.Strings(wantFed)
	assert.Equal(t, wantFed, cols("federated_identities"), "identity.federated_identities exact column set")

	forbidden := map[string]bool{"ip_address": true, "user_agent": true, "session_id": true, "last_login_ip": true, "phone": true, "mfa_factor": true}
	for _, table := range []string{"users", "federated_identities"} {
		for _, c := range cols(table) {
			assert.False(t, forbidden[c], "forbidden PII column %q present on identity.%s", c, table)
		}
	}
}
