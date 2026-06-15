//go:build e2e

// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Layer-1 live wired-path E2E for the persistent-identity feature (Epic 15,
// Story 15.5). These tests drive a REAL mock-OIDC authorization-code login
// through the deployed server — NOT a self-signed bearer JWT — because the
// OIDC callback (server/internal/auth/provisioning.go EvaluateOIDCUser →
// ObserveLogin) is the SOLE trigger of identity persistence. A self-signed JWT
// (GenerateTestJWT/GenerateOIDCJWT) is validated as a bearer token but never
// runs ObserveLogin, so it writes ZERO identity rows; it is used here only for
// the operator API calls and the AC7 negative-authz check.
//
// The suite proves, end-to-end against real binaries: a real login materializes
// the canonical identity.users + identity.federated_identities row (AC1); a
// second login refreshes last_seen_at without duplicating rows (AC2); DELETE
// reclaims the seat and re-login resurrects it with EE audit events (AC3/AC4);
// the entitlement-based billed-seat count is uniform and age-invariant (AC5);
// last_seen_at drives the display-only isInactive flag and never billing (AC6);
// and the Users API is operator-gated (AC7).
//
// Cross-org RLS WITH CHECK isolation, verified/unverified email divergence,
// email normalization, the audit-emit-failure metric, and the source_kind
// round-trip are Layer-2 concerns — they cannot be expressed through a
// single-org deployed HTTP API and are proven by the existing //go:build
// integration suites (store_test.go, ee/identity/audit_hooks_test.go) run
// against the deployed Postgres. See the assertion-→-layer mapping table in the
// story Dev Notes.
//
// Live prerequisites (the whole file self-skips when they are absent, so
// `go test -tags=e2e ./...` stays green without a cluster):
//   - E2E_API_URL              server base URL (default http://localhost:8080)
//   - E2E_OIDC_BASE_URL        host-reachable mock-OIDC base (default http://localhost:8081)
//   - KNODEX_TEST_DATABASE_URL pgx DSN to the deployed Postgres (port-forward)
//
// Recommended runner: scripts/e2e-identity.sh (Make target `e2e-identity`),
// which deploys, port-forwards server+mock-oidc+postgres, and exports the env.
package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/knodex/knodex/server/internal/database"
	"github.com/knodex/knodex/server/internal/services"
)

// =============================================================================
// Live configuration + gating
// =============================================================================

// identityE2EConfig captures the per-edition knobs that make the same lifecycle
// assertions portable across OSS / Enterprise deploys.
type identityE2EConfig struct {
	apiURL      string // deployed server base URL (port-forwarded)
	oidcBaseURL string // host-reachable mock-OIDC base (port-forwarded)
	provider    string // registered SSO provider name (Task 4 overlay)
	issuer      string // expected federated_identities.issuer (in-cluster issuerURL)
	org         string // cfg.Organization the deploy runs under (RLS scope)

	sourceKind         string // expected federated_identities.source_kind for this edition
	auditEnabled       bool   // EE has audit.events; OSS does not
	inactiveThreshDays int    // deploy's IDENTITY_INACTIVE_THRESHOLD_DAYS
}

// loadIdentityE2EConfig resolves the live config from env and skips the test
// when the cluster prerequisites are not present. Editions select source_kind
// and audit expectations via E2E_EDITION ∈ {oss,ee} (default oss).
func loadIdentityE2EConfig(t *testing.T) identityE2EConfig {
	t.Helper()

	apiURL := envOrDefault("E2E_API_URL", "http://localhost:8080")
	if os.Getenv("KNODEX_TEST_DATABASE_URL") == "" {
		t.Skip("KNODEX_TEST_DATABASE_URL not set; skipping live identity E2E (needs a deployed Postgres via port-forward)")
	}

	edition := envOrDefault("E2E_EDITION", "oss")
	sourceKind := services.SourceKindOIDCJIT
	auditEnabled := false
	switch edition {
	case "ee", "enterprise":
		auditEnabled = true
	}
	// Allow explicit overrides so the runner can pin exact expectations.
	sourceKind = envOrDefault("E2E_EXPECTED_SOURCE_KIND", sourceKind)
	if v := os.Getenv("E2E_AUDIT_ENABLED"); v != "" {
		auditEnabled = v == "true" || v == "1"
	}

	return identityE2EConfig{
		apiURL:             apiURL,
		oidcBaseURL:        envOrDefault("E2E_OIDC_BASE_URL", "http://localhost:8081"),
		provider:           envOrDefault("E2E_OIDC_PROVIDER", "mock-oidc"),
		issuer:             envOrDefault("E2E_OIDC_ISSUER", "http://mock-oidc:8081"),
		org:                envOrDefault("E2E_ORG_ID", "default"),
		sourceKind:         sourceKind,
		auditEnabled:       auditEnabled,
		inactiveThreshDays: envIntOrDefault("E2E_INACTIVE_THRESHOLD_DAYS", 30),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// =============================================================================
// Task 1 — real-OIDC-login helper (the net-new deliverable)
// =============================================================================

// realOIDCLogin performs an actual OIDC authorization-code login for loginHint
// against the deployed server, the ONLY path that runs ObserveLogin. It drives
// three hops with manual redirect handling:
//
//  1. GET {api}/api/v1/auth/oidc/login?provider=… — the server 302-redirects to
//     the IdP /authorize endpoint, having stashed state+nonce+PKCE-verifier in
//     Redis. No `redirect` query param is sent, so the callback later returns a
//     JSON session directly (it does not bounce to a frontend page).
//  2. GET {mock-oidc}/authorize?…&login_hint=<email> — we rewrite the in-cluster
//     issuer host (mock-oidc:8081) to the host-reachable port-forward and inject
//     login_hint (which the server never sets) so the mock selects OUR user. The
//     mock 302-redirects back to the server callback with code+state.
//  3. GET {api}/api/v1/auth/oidc/callback?code=…&state=… — the server validates
//     state/nonce/verifier from Redis, exchanges the code at the IdP (in-cluster),
//     persists the identity (ObserveLogin), and issues a session.
//
// It is deliberately NOT used to authorize the Users API calls — operator access
// is exercised with a self-signed admin JWT, which is robust and does not depend
// on the deploy's group→role mapping.
func realOIDCLogin(t *testing.T, c identityE2EConfig, loginHint string) {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		// Capture every redirect manually so we can rewrite the IdP host and
		// inject login_hint between hops.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Hop 1 — initiate the flow at the server (no `redirect` → JSON callback).
	startURL := c.apiURL + "/api/v1/auth/oidc/login?provider=" + url.QueryEscape(c.provider)
	resp, err := client.Get(startURL)
	require.NoError(t, err, "OIDC login initiation request")
	authorizeLoc := resp.Header.Get("Location")
	requireStatus(t, resp, http.StatusFound, "login initiation should 302 to the IdP authorize endpoint")
	require.NotEmpty(t, authorizeLoc, "login initiation must return a Location header")

	// Rewrite the in-cluster issuer host to the host-reachable mock-OIDC base and
	// inject login_hint so the mock selects loginHint instead of the default user.
	authURL, err := url.Parse(authorizeLoc)
	require.NoError(t, err, "parse authorize redirect")
	oidcBase, err := url.Parse(c.oidcBaseURL)
	require.NoError(t, err, "parse E2E_OIDC_BASE_URL")
	authURL.Scheme = oidcBase.Scheme
	authURL.Host = oidcBase.Host
	q := authURL.Query()
	q.Set("login_hint", loginHint)
	authURL.RawQuery = q.Encode()

	// Hop 2 — authorize at the mock IdP → 302 back to the server callback.
	resp2, err := client.Get(authURL.String())
	require.NoError(t, err, "mock-OIDC authorize request")
	callbackLoc := resp2.Header.Get("Location")
	requireStatus(t, resp2, http.StatusFound, "authorize should 302 back to the server callback")
	require.NotEmpty(t, callbackLoc, "authorize must return a Location header")

	// Hop 3 — follow the callback. With an empty stored redirect URL the server
	// returns 200 JSON and sets the session cookie (NFR-U1: the login succeeds).
	resp3, err := client.Get(callbackLoc)
	require.NoError(t, err, "OIDC callback request")
	requireStatus(t, resp3, http.StatusOK, "callback should issue a session (200 JSON)")
}

// requireStatus reads+closes the response body and asserts the status code,
// surfacing the body on mismatch for diagnostics.
func requireStatus(t *testing.T, resp *http.Response, want int, msg string) {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equalf(t, want, resp.StatusCode, "%s (status=%d body=%s)", msg, resp.StatusCode, string(body))
}

// =============================================================================
// Task 2 — deployed-Postgres assertion helper
// =============================================================================

// identityDB wraps a pgx pool to the deployed Postgres, scoping every read/write
// to the deploy's org via CheckoutWithOrg (set_config('app.org_id', …)) exactly
// like the production checkout path — otherwise RLS hides the rows and we get
// false "0 rows" failures (the live pool connects as the non-BYPASSRLS
// knodex_app role).
type identityDB struct {
	pool *pgxpool.Pool
	org  string
}

// openIdentityDB connects to KNODEX_TEST_DATABASE_URL. The caller already
// ensured it is set (loadIdentityE2EConfig skips otherwise); this is the
// belt-and-suspenders skip for direct callers.
func openIdentityDB(t *testing.T, c identityE2EConfig) *identityDB {
	t.Helper()
	dsn := os.Getenv("KNODEX_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("KNODEX_TEST_DATABASE_URL not set; skipping deployed-Postgres assertions")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err, "connect to deployed Postgres")
	require.NoError(t, pool.Ping(ctx), "ping deployed Postgres")
	t.Cleanup(pool.Close)
	return &identityDB{pool: pool, org: c.org}
}

// withOrg runs fn inside an org-scoped transaction (RLS-safe).
func (d *identityDB) withOrg(t *testing.T, fn func(tx pgx.Tx) error) {
	t.Helper()
	require.NoError(t, db.CheckoutWithOrg(context.Background(), d.pool, d.org, fn))
}

// countFederation returns the (users, federated_identities) row counts for a
// given (issuer, sub) — used to prove idempotent re-login creates no duplicates.
func (d *identityDB) countFederation(t *testing.T, issuer, sub string) (users, feds int64) {
	t.Helper()
	d.withOrg(t, func(tx pgx.Tx) error {
		if err := tx.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM identity.federated_identities WHERE issuer=$1 AND sub=$2`,
			issuer, sub).Scan(&feds); err != nil {
			return err
		}
		return tx.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM identity.users u
			 WHERE EXISTS (SELECT 1 FROM identity.federated_identities f
			               WHERE f.internal_user_id = u.id AND f.issuer=$1 AND f.sub=$2)`,
			issuer, sub).Scan(&users)
	})
	return users, feds
}

// userByFederation returns the canonical user row reached via a federated
// identity. ok=false when no such row exists.
func (d *identityDB) userByFederation(t *testing.T, issuer, sub string) (id, state, email string, lastSeen time.Time, ok bool) {
	t.Helper()
	d.withOrg(t, func(tx pgx.Tx) error {
		err := tx.QueryRow(context.Background(),
			`SELECT u.id, u.state, u.email, u.last_seen_at
			 FROM identity.users u
			 JOIN identity.federated_identities f ON f.internal_user_id = u.id
			 WHERE f.issuer=$1 AND f.sub=$2`,
			issuer, sub).Scan(&id, &state, &email, &lastSeen)
		if err == pgx.ErrNoRows {
			return nil
		}
		if err == nil {
			ok = true
		}
		return err
	})
	return id, state, email, lastSeen, ok
}

// billedSeatCount runs the EXACT entitlement SQL the seat reconciler bills on
// (store.go BilledSeatCount: state='active', no last_seen_at window).
func (d *identityDB) billedSeatCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	d.withOrg(t, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM identity.users
			 WHERE org_id = current_setting('app.org_id', true) AND state = 'active'`).Scan(&n)
	})
	return n
}

// backdateLastSeen ages a user's last_seen_at by `days` days (direct SQL) to
// exercise the display-only isInactive flag and the age-invariance of billing.
func (d *identityDB) backdateLastSeen(t *testing.T, id string, days int) {
	t.Helper()
	d.withOrg(t, func(tx pgx.Tx) error {
		ct, err := tx.Exec(context.Background(),
			fmt.Sprintf(`UPDATE identity.users SET last_seen_at = now() - interval '%d days' WHERE id = $1`, days),
			id)
		if err != nil {
			return err
		}
		if ct.RowsAffected() != 1 {
			return fmt.Errorf("backdate affected %d rows, want 1", ct.RowsAffected())
		}
		return nil
	})
}

// auditActionsForUser returns the identity.user.* audit event actions recorded
// for a user id, ordered chronologically. EE/cloud only — guard with
// cfg.auditEnabled (OSS has no audit.events table).
func (d *identityDB) auditActionsForUser(t *testing.T, userID string) []string {
	t.Helper()
	var actions []string
	d.withOrg(t, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(),
			`SELECT action FROM audit.events
			 WHERE resolved_user_id = $1 AND action LIKE 'identity.user.%'
			 ORDER BY timestamp ASC`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				return err
			}
			actions = append(actions, a)
		}
		return rows.Err()
	})
	return actions
}

// truncateIdentity clears both identity tables for the org so reruns are
// deterministic on a shared cluster.
func (d *identityDB) truncateIdentity(t *testing.T) {
	t.Helper()
	d.withOrg(t, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), `DELETE FROM identity.federated_identities`); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), `DELETE FROM identity.users`)
		return err
	})
}

// =============================================================================
// API helpers (operator surface under test — Story 15.8)
// =============================================================================

// apiUserResponse mirrors handlers.UserResponse (we do not import the handler
// package — assert on the JSON contract only).
type apiUserResponse struct {
	ID                  string    `json:"id"`
	Email               string    `json:"email"`
	DisplayName         string    `json:"displayName"`
	State               string    `json:"state"`
	IsInactive          bool      `json:"isInactive"`
	FirstSeenAt         time.Time `json:"firstSeenAt"`
	LastSeenAt          time.Time `json:"lastSeenAt"`
	FederatedIdentities []struct {
		Issuer       string `json:"issuer"`
		Sub          string `json:"sub"`
		ProviderKind string `json:"providerKind"`
		SourceKind   string `json:"sourceKind"`
	} `json:"federatedIdentities"`
}

type apiUsersListResponse struct {
	Users         []apiUserResponse `json:"users"`
	NextPageToken string            `json:"nextPageToken"`
}

// operatorJWT mints a self-signed admin token. Operator API calls do not need to
// run ObserveLogin (only the subject login does), so a bearer token with the
// server-admin role is the robust, deploy-independent way to exercise the
// settings/*-gated Users API.
func operatorJWT() string {
	return GenerateTestJWT(JWTClaims{
		Subject:     "user-global-admin",
		Email:       "admin@test.local",
		CasbinRoles: []string{"role:serveradmin"},
	})
}

// listAllUsers pages through GET /api/v1/users and returns every user.
func listAllUsers(t *testing.T, c identityE2EConfig) []apiUserResponse {
	t.Helper()
	client := &http.Client{Timeout: 20 * time.Second}
	token := operatorJWT()
	var all []apiUserResponse
	pageToken := ""
	for {
		path := "/api/v1/users?limit=200"
		if pageToken != "" {
			path += "&pageToken=" + url.QueryEscape(pageToken)
		}
		resp, err := MakeAuthenticatedRequest(client, c.apiURL, "GET", path, token, nil)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.Equalf(t, http.StatusOK, resp.StatusCode, "GET /api/v1/users (body=%s)", string(body))
		var page apiUsersListResponse
		require.NoError(t, json.Unmarshal(body, &page))
		all = append(all, page.Users...)
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return all
}

// findUserByEmail returns the listed user matching email (case-insensitively
// normalized to lowercase, mirroring the store) or ok=false.
func findUserByEmail(users []apiUserResponse, email string) (apiUserResponse, bool) {
	for _, u := range users {
		if u.Email == email {
			return u, true
		}
	}
	return apiUserResponse{}, false
}

// getUser fetches GET /api/v1/users/{id}; ok=false on 404.
func getUser(t *testing.T, c identityE2EConfig, id string) (apiUserResponse, bool) {
	t.Helper()
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := MakeAuthenticatedRequest(client, c.apiURL, "GET", "/api/v1/users/"+url.PathEscape(id), operatorJWT(), nil)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return apiUserResponse{}, false
	}
	require.Equalf(t, http.StatusOK, resp.StatusCode, "GET /api/v1/users/{id} (body=%s)", string(body))
	var u apiUserResponse
	require.NoError(t, json.Unmarshal(body, &u))
	return u, true
}

// deleteUser issues DELETE /api/v1/users/{id} and returns the reclaim body.
func deleteUser(t *testing.T, c identityE2EConfig, id string) (state, note string) {
	t.Helper()
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := MakeAuthenticatedRequest(client, c.apiURL, "DELETE", "/api/v1/users/"+url.PathEscape(id), operatorJWT(), nil)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equalf(t, http.StatusOK, resp.StatusCode, "DELETE /api/v1/users/{id} should be 200 (body=%s)", string(body))
	var dr struct {
		State string `json:"state"`
		Note  string `json:"note"`
	}
	require.NoError(t, json.Unmarshal(body, &dr))
	return dr.State, dr.Note
}

// subjectFor derives the OIDC subject the mock IdP issues for a default test
// user email. The mock's DefaultTestUsers map email → "<local>-user-id".
func subjectFor(email string) string {
	switch email {
	case "admin@test.local":
		return "admin-user-id"
	case "developer@test.local":
		return "developer-user-id"
	case "viewer@test.local":
		return "viewer-user-id"
	case "nogroups@test.local":
		return "nogroups-user-id"
	case "multi@test.local":
		return "multi-user-id"
	case "platform-admin@test.local":
		return "platform-admin-user-id"
	default:
		return ""
	}
}

// =============================================================================
// Task 3 — lifecycle test bodies (AC1–AC6)
// =============================================================================

// TestIdentityE2E_LoginMaterializesRoster (AC1) — a real OIDC login creates the
// canonical user + exactly one expanded federated identity, surfaced by the
// Users API and confirmed by direct Postgres rows.
func TestIdentityE2E_LoginMaterializesRoster(t *testing.T) {
	c := loadIdentityE2EConfig(t)
	dbh := openIdentityDB(t, c)
	dbh.truncateIdentity(t)
	t.Cleanup(func() { dbh.truncateIdentity(t) })

	const email = "developer@test.local"
	sub := subjectFor(email)

	realOIDCLogin(t, c, email)

	// API: the user is listed, active, with one expanded federated identity.
	users := listAllUsers(t, c)
	u, ok := findUserByEmail(users, email)
	require.Truef(t, ok, "GET /api/v1/users must surface %s after a real login", email)
	assert.Equal(t, services.UserStateActive, u.State)
	require.Len(t, u.FederatedIdentities, 1, "exactly one federated identity expected")
	fi := u.FederatedIdentities[0]
	assert.Equal(t, c.issuer, fi.Issuer, "federated identity issuer must match the mock-OIDC issuer")
	assert.Equal(t, sub, fi.Sub, "federated identity sub must match the IdP subject")
	assert.Equal(t, "oidc", fi.ProviderKind)
	assert.Equal(t, c.sourceKind, fi.SourceKind, "source_kind must match the edition")

	// DB: exactly one users row and one federated_identities row for (issuer, sub).
	usersCount, fedsCount := dbh.countFederation(t, c.issuer, sub)
	assert.Equal(t, int64(1), usersCount, "exactly one identity.users row")
	assert.Equal(t, int64(1), fedsCount, "exactly one identity.federated_identities row")
}

// TestIdentityE2E_ReLoginNoDuplicate (AC2) — a second login advances
// last_seen_at and creates no new rows (FR-U1).
func TestIdentityE2E_ReLoginNoDuplicate(t *testing.T) {
	c := loadIdentityE2EConfig(t)
	dbh := openIdentityDB(t, c)
	dbh.truncateIdentity(t)
	t.Cleanup(func() { dbh.truncateIdentity(t) })

	const email = "developer@test.local"
	sub := subjectFor(email)

	realOIDCLogin(t, c, email)
	_, _, _, firstSeen, ok := dbh.userByFederation(t, c.issuer, sub)
	require.True(t, ok, "user must exist after first login")

	// Ensure a measurable last_seen_at delta across the two logins.
	time.Sleep(1100 * time.Millisecond)
	realOIDCLogin(t, c, email)

	id, _, _, secondSeen, ok := dbh.userByFederation(t, c.issuer, sub)
	require.True(t, ok)
	assert.Truef(t, secondSeen.After(firstSeen), "last_seen_at must strictly increase (%s !> %s)", secondSeen, firstSeen)

	usersCount, fedsCount := dbh.countFederation(t, c.issuer, sub)
	assert.Equal(t, int64(1), usersCount, "no duplicate users row on re-login")
	assert.Equal(t, int64(1), fedsCount, "no duplicate federated_identities row on re-login")

	// GET /{id} still returns a single record.
	u, ok := getUser(t, c, id)
	require.True(t, ok)
	assert.Equal(t, services.UserStateActive, u.State)
}

// TestIdentityE2E_RemoveReclaimResurrect (AC3, AC4) — DELETE reclaims the seat
// (count−1, state=removed, still listed); re-login resurrects (count+1,
// state=active); EE/cloud record removed→resurrected audit events in order.
func TestIdentityE2E_RemoveReclaimResurrect(t *testing.T) {
	c := loadIdentityE2EConfig(t)
	dbh := openIdentityDB(t, c)
	dbh.truncateIdentity(t)
	t.Cleanup(func() { dbh.truncateIdentity(t) })

	const email = "developer@test.local"
	sub := subjectFor(email)

	realOIDCLogin(t, c, email)
	id, _, _, _, ok := dbh.userByFederation(t, c.issuer, sub)
	require.True(t, ok)
	before := dbh.billedSeatCount(t)

	// DELETE → 200 + reclaim note; entitlement count drops by 1; still listed as removed.
	state, note := deleteUser(t, c, id)
	assert.Equal(t, services.UserStateRemoved, state)
	assert.NotEmpty(t, note, "reclaim note must ride in the DELETE body")
	assert.Equal(t, before-1, dbh.billedSeatCount(t), "removal must drop the entitlement count by 1")

	u, ok := getUser(t, c, id)
	require.True(t, ok, "removed user is soft-deleted, still retrievable")
	assert.Equal(t, services.UserStateRemoved, u.State)
	listed := listAllUsers(t, c)
	_, stillListed := findUserByEmail(listed, email)
	assert.True(t, stillListed, "removed user must still appear in the list")

	// Re-login → resurrect: state flips back to active, count +1.
	realOIDCLogin(t, c, email)
	assert.Equal(t, before, dbh.billedSeatCount(t), "resurrect must restore the entitlement count")
	u, ok = getUser(t, c, id)
	require.True(t, ok)
	assert.Equal(t, services.UserStateActive, u.State)

	// EE/cloud: audit trail contains removed → resurrected in order.
	if c.auditEnabled {
		actions := dbh.auditActionsForUser(t, id)
		assert.Containsf(t, actions, "identity.user.removed", "audit must contain removal (got %v)", actions)
		assert.Containsf(t, actions, "identity.user.resurrected", "audit must contain resurrection (got %v)", actions)
		removedIdx := indexOf(actions, "identity.user.removed")
		resurrectedIdx := indexOf(actions, "identity.user.resurrected")
		assert.Truef(t, removedIdx >= 0 && resurrectedIdx > removedIdx,
			"removed must precede resurrected (actions=%v)", actions)
	}
}

// TestIdentityE2E_EntitlementCountUniform (AC5) — the billed-seat count is
// state='active' only and never windowed by last_seen_at (FR-U7, R5-2).
func TestIdentityE2E_EntitlementCountUniform(t *testing.T) {
	c := loadIdentityE2EConfig(t)
	dbh := openIdentityDB(t, c)
	dbh.truncateIdentity(t)
	t.Cleanup(func() { dbh.truncateIdentity(t) })

	emails := []string{"developer@test.local", "viewer@test.local", "multi@test.local", "platform-admin@test.local"}
	for _, e := range emails {
		realOIDCLogin(t, c, e)
	}
	assert.Equal(t, int64(4), dbh.billedSeatCount(t), "four fresh logins ⇒ count 4")

	// Remove two.
	removed := emails[:2]
	for _, e := range removed {
		id, _, _, _, ok := dbh.userByFederation(t, c.issuer, subjectFor(e))
		require.True(t, ok)
		deleteUser(t, c, id)
	}
	assert.Equal(t, int64(2), dbh.billedSeatCount(t), "removing two ⇒ count 2")

	// Backdate one removed user 400 days — the count is state-based, not
	// last_seen_at-windowed, so it stays 2.
	idRemoved, _, _, _, ok := dbh.userByFederation(t, c.issuer, subjectFor(removed[0]))
	require.True(t, ok)
	dbh.backdateLastSeen(t, idRemoved, 400)
	assert.Equal(t, int64(2), dbh.billedSeatCount(t), "billing is state-based, never last_seen_at-windowed")
}

// TestIdentityE2E_InactiveIsDisplayOnly (AC6) — last_seen_at past the inactivity
// threshold flips isInactive in the list payload while billing is unchanged
// (FR-U14).
func TestIdentityE2E_InactiveIsDisplayOnly(t *testing.T) {
	c := loadIdentityE2EConfig(t)
	dbh := openIdentityDB(t, c)
	dbh.truncateIdentity(t)
	t.Cleanup(func() { dbh.truncateIdentity(t) })

	const email = "developer@test.local"
	sub := subjectFor(email)

	realOIDCLogin(t, c, email)
	id, _, _, _, ok := dbh.userByFederation(t, c.issuer, sub)
	require.True(t, ok)
	beforeCount := dbh.billedSeatCount(t)

	// Age past the deploy's threshold (+5 days of margin).
	dbh.backdateLastSeen(t, id, c.inactiveThreshDays+5)

	users := listAllUsers(t, c)
	u, ok := findUserByEmail(users, email)
	require.True(t, ok)
	assert.True(t, u.IsInactive, "user aged past the threshold must report isInactive=true")
	assert.Equal(t, services.UserStateActive, u.State, "isInactive is orthogonal to state")
	assert.Equal(t, beforeCount, dbh.billedSeatCount(t), "isInactive must not change the entitlement count")
}

// =============================================================================
// AC7 — Users API is operator-gated (single Casbin layer)
// =============================================================================

// TestIdentityE2E_UsersAPIOperatorGated (AC7) — a caller lacking settings/* is
// rejected with 403 on both reads and the delete. A self-signed non-settings JWT
// is used for the negative check only (it never touches persistence).
func TestIdentityE2E_UsersAPIOperatorGated(t *testing.T) {
	c := loadIdentityE2EConfig(t)
	_ = openIdentityDB(t, c) // gate on cluster presence; no rows needed

	client := &http.Client{Timeout: 20 * time.Second}
	// A regular OIDC user with no server-admin role and no settings/* policy.
	nonAdmin := GenerateOIDCJWT("nobody@test.local", []string{"some-unmapped-group"})

	resp, err := MakeAuthenticatedRequest(client, c.apiURL, "GET", "/api/v1/users", nonAdmin, nil)
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusForbidden, "GET /api/v1/users must reject a non-operator")

	resp, err = MakeAuthenticatedRequest(client, c.apiURL, "DELETE", "/api/v1/users/some-id", nonAdmin, nil)
	require.NoError(t, err)
	requireStatus(t, resp, http.StatusForbidden, "DELETE /api/v1/users/{id} must reject a non-operator")
}

// indexOf returns the first index of s in xs, or -1.
func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return -1
}
