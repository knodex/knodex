// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

// Package postgres implements the edition-neutral identity store (Story 15.2).
// It is the single implementation of services.IdentityService on EVERY edition
// (OSS, Enterprise, Cloud) — there is NO build tag. The store owns its own
// short-lived transactions via database.CheckoutWithOrg (RLS org scoping) and
// invokes the configured hooks synchronously AFTER each commit (R5-7). A login
// materializes a canonical identity.users row (JIT on first login; last_seen_at
// refresh + resurrect-on-login thereafter) and a federated_identities row keyed
// by the (org_id, issuer, sub) OIDC triple.
package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/knodex/knodex/server/internal/database"
	"github.com/knodex/knodex/server/internal/services"
)

// Config configures the base store. SourceKind defaults to "oidc_jit". Hooks
// are optional (nil callbacks are no-ops). OrgID is the static single-org scope
// (R5-9) used by the
// methods that do not carry an org in their signature (BilledSeatCount, GetByID,
// List, Remove); ObserveLogin/GetByFederation use the org from their arguments.
type Config struct {
	SourceKind string
	Hooks      services.IdentityHooks
	Logger     *slog.Logger
	OrgID      string
}

// Store is the Postgres-backed identity store.
type Store struct {
	pool       *pgxpool.Pool
	sourceKind string
	hooks      services.IdentityHooks
	logger     *slog.Logger
	orgID      string
}

// Compile-time assertion that Store implements the port.
var _ services.IdentityService = (*Store)(nil)

// New constructs a Store. The pool must be non-nil in production; a nil pool is
// tolerated only so unit tests that never touch the DB can construct the type.
func New(pool *pgxpool.Pool, cfg Config) *Store {
	sk := cfg.SourceKind
	if sk == "" {
		sk = services.SourceKindOIDCJIT
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{
		pool:       pool,
		sourceKind: sk,
		hooks:      cfg.Hooks,
		logger:     logger.With("subsys", "identity"),
		orgID:      cfg.OrgID,
	}
}

const userColumns = `id, org_id, email, display_name, state, first_seen_at, last_seen_at`

// scanUser scans a row in userColumns order into a UserRecord.
func scanUser(row pgx.Row) (*services.UserRecord, error) {
	var u services.UserRecord
	var id string
	if err := row.Scan(&id, &u.OrgID, &u.Email, &u.DisplayName, &u.State, &u.FirstSeenAt, &u.LastSeenAt); err != nil {
		return nil, err
	}
	u.ID = services.UserID(id)
	return &u, nil
}

// ObserveLogin materializes or refreshes the canonical user for a successful
// OIDC login (AC9). It runs in one CheckoutWithOrg transaction and fires hooks
// post-commit.
func (s *Store) ObserveLogin(ctx context.Context, p services.ObserveLoginParams) (services.ObserveLoginResult, error) {
	if p.OrgID == "" {
		return services.ObserveLoginResult{}, fmt.Errorf("identity: ObserveLogin requires a non-empty OrgID")
	}
	if p.Issuer == "" || p.Sub == "" {
		return services.ObserveLoginResult{}, fmt.Errorf("identity: ObserveLogin requires non-empty Issuer and Sub")
	}

	normEmail := strings.ToLower(strings.TrimSpace(p.Email))
	providerKind := p.ProviderKind
	if providerKind == "" {
		providerKind = "oidc"
	}

	var (
		result   services.ObserveLoginResult
		rec      *services.UserRecord
		oldEmail string
	)

	err := database.CheckoutWithOrg(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		// 1. Lookup the federation row by (org_id, issuer, sub).
		var internalUserID string
		lookupErr := tx.QueryRow(ctx,
			`SELECT internal_user_id FROM identity.federated_identities
			 WHERE org_id = $1 AND issuer = $2 AND sub = $3`,
			p.OrgID, p.Issuer, p.Sub).Scan(&internalUserID)

		switch {
		case lookupErr == nil:
			// Subsequent login — refresh the existing user.
			u, oe, r, uerr := s.refreshExisting(ctx, tx, internalUserID, normEmail, p.DisplayName, p.EmailVerified)
			if uerr != nil {
				return uerr
			}
			rec, oldEmail, result = u, oe, r
			result.Created = false
			return nil

		case errors.Is(lookupErr, pgx.ErrNoRows):
			// First login by this (issuer, sub). Try SCIM reconciliation first
			// (R5-6): a pre-provisioned row with this external_id and sub IS NULL.
			var scimUserID string
			scimErr := tx.QueryRow(ctx,
				`SELECT internal_user_id FROM identity.federated_identities
				 WHERE org_id = $1 AND external_id = $2 AND sub IS NULL
				 LIMIT 1`,
				p.OrgID, p.Sub).Scan(&scimUserID)

			switch {
			case scimErr == nil:
				// Backfill the SCIM-provisioned federation row. Target the SINGLE
				// row the lookup resolved (internal_user_id), NOT a bare
				// external_id predicate — the partial unique index only spans
				// (org_id, source_connector_id, external_id), so two connectors in
				// one org may share an external_id with sub IS NULL, and an
				// unscoped UPDATE could mutate more than one connector's row. Also
				// align `issuer` to the login issuer so the row's natural key
				// (org_id, issuer, sub) matches subsequent logins and the partial
				// unique fed_identities_issuer_sub_uq keys on the right issuer.
				if _, err := tx.Exec(ctx,
					`UPDATE identity.federated_identities
					 SET sub = $1, issuer = $2, updated_at = now()
					 WHERE internal_user_id = $3 AND sub IS NULL`,
					p.Sub, p.Issuer, scimUserID); err != nil {
					return fmt.Errorf("backfill scim sub: %w", err)
				}
				u, oe, r, uerr := s.refreshExisting(ctx, tx, scimUserID, normEmail, p.DisplayName, p.EmailVerified)
				if uerr != nil {
					return uerr
				}
				// SCIM user was already provisioned — not a new creation.
				rec, oldEmail, result = u, oe, r
				result.Created = false
				return nil

			case errors.Is(scimErr, pgx.ErrNoRows):
				// Brand-new JIT user.
				u, ierr := s.insertNew(ctx, tx, p.OrgID, p.Issuer, p.Sub, normEmail, p.DisplayName, providerKind)
				if ierr != nil {
					return ierr
				}
				rec = u
				result = services.ObserveLoginResult{ID: u.ID, Created: true}
				return nil

			default:
				return fmt.Errorf("scim reconcile lookup: %w", scimErr)
			}

		default:
			return fmt.Errorf("federation lookup: %w", lookupErr)
		}
	})
	if err != nil {
		return services.ObserveLoginResult{}, err
	}

	s.fireObserveHooks(ctx, rec, oldEmail, result)
	return result, nil
}

// insertNew JIT-creates a user + federation row, returning the new record.
func (s *Store) insertNew(ctx context.Context, tx pgx.Tx, org, issuer, sub, email, displayName, providerKind string) (*services.UserRecord, error) {
	id := ulid.Make().String()
	row := tx.QueryRow(ctx,
		`INSERT INTO identity.users (id, org_id, email, display_name, state)
		 VALUES ($1, $2, $3, $4, 'active')
		 RETURNING `+userColumns,
		id, org, email, displayName)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO identity.federated_identities
		   (org_id, issuer, sub, internal_user_id, provider_kind, source_kind)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		org, issuer, sub, id, providerKind, s.sourceKind); err != nil {
		return nil, fmt.Errorf("insert federated identity: %w", err)
	}
	return u, nil
}

// refreshExisting applies the subsequent-login update rules (AC9) to an existing
// user and returns the post-update record, the prior email, and the result flags
// (Created left false by the caller).
func (s *Store) refreshExisting(ctx context.Context, tx pgx.Tx, internalUserID, normEmail, displayName string, emailVerified bool) (*services.UserRecord, string, services.ObserveLoginResult, error) {
	var result services.ObserveLoginResult

	// Load current state for the decision (and lock the row for the update).
	var storedEmail, storedState string
	if err := tx.QueryRow(ctx,
		`SELECT email, state FROM identity.users WHERE id = $1 FOR UPDATE`,
		internalUserID).Scan(&storedEmail, &storedState); err != nil {
		return nil, "", result, fmt.Errorf("load user for refresh: %w", err)
	}

	if storedState == services.UserStateRemoved {
		result.Resurrected = true
	}

	// Email rules (R5 / AC9).
	newEmail := storedEmail
	if normEmail != "" && storedEmail != normEmail {
		if emailVerified {
			newEmail = normEmail
			result.EmailChanged = true
			s.logger.Info("identity email changed", "event", "email_changed", "user_id", internalUserID)
		} else {
			// Unverified divergence: keep the stored email, surface a warning.
			s.logger.Warn("identity unverified email divergence",
				"event", "unverified_email_divergence", "user_id", internalUserID)
		}
	}

	row := tx.QueryRow(ctx,
		`UPDATE identity.users
		 SET last_seen_at = now(),
		     state        = 'active',
		     email        = $2,
		     display_name = COALESCE(NULLIF($3, ''), display_name)
		 WHERE id = $1
		 RETURNING `+userColumns,
		internalUserID, newEmail, displayName)
	u, err := scanUser(row)
	if err != nil {
		return nil, "", result, fmt.Errorf("update user: %w", err)
	}
	result.ID = u.ID
	return u, storedEmail, result, nil
}

// fireObserveHooks invokes the matching post-commit hooks (R5-7). Hook errors
// are logged at ERROR and metered; they never propagate.
func (s *Store) fireObserveHooks(ctx context.Context, u *services.UserRecord, oldEmail string, r services.ObserveLoginResult) {
	if u == nil {
		return
	}
	if r.Created && s.hooks.OnFirstSeen != nil {
		s.runHook(ctx, "first_seen", func() error { return s.hooks.OnFirstSeen(ctx, u) })
	}
	if r.Resurrected && s.hooks.OnResurrected != nil {
		s.runHook(ctx, "resurrected", func() error { return s.hooks.OnResurrected(ctx, u) })
	}
	if r.EmailChanged && s.hooks.OnEmailChanged != nil {
		s.runHook(ctx, "email_changed", func() error { return s.hooks.OnEmailChanged(ctx, u, oldEmail) })
	}
}

// runHook executes a single hook, swallowing and metering any error.
func (s *Store) runHook(_ context.Context, eventType string, fn func() error) {
	if err := fn(); err != nil {
		auditEmitFailures.WithLabelValues(eventType).Inc()
		s.logger.Error("identity audit hook failed", "event_type", eventType, "error", err)
	}
}

// Remove soft-deletes a user (state='removed') — FR-U13 / AC10. Idempotent:
// removing an already-removed user is a no-op success (no hook). Returns
// ErrUserNotFound when no row in the caller's org.
func (s *Store) Remove(ctx context.Context, id services.UserID) error {
	var rec *services.UserRecord
	fired := false

	err := database.CheckoutWithOrg(ctx, s.pool, s.orgID, func(tx pgx.Tx) error {
		var state string
		if err := tx.QueryRow(ctx,
			`SELECT state FROM identity.users WHERE id = $1 FOR UPDATE`,
			string(id)).Scan(&state); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return services.ErrUserNotFound
			}
			return fmt.Errorf("load user for remove: %w", err)
		}
		if state == services.UserStateRemoved {
			return nil // idempotent no-op
		}
		row := tx.QueryRow(ctx,
			`UPDATE identity.users SET state = 'removed' WHERE id = $1 RETURNING `+userColumns,
			string(id))
		u, err := scanUser(row)
		if err != nil {
			return fmt.Errorf("update user removed: %w", err)
		}
		rec = u
		fired = true
		return nil
	})
	if err != nil {
		return err
	}
	if fired && rec != nil && s.hooks.OnRemoved != nil {
		s.runHook(ctx, "removed", func() error { return s.hooks.OnRemoved(ctx, rec) })
	}
	return nil
}

// GetByID returns a user by ULID, or ErrUserNotFound (cross-org returns
// ErrUserNotFound — RLS hides the row).
func (s *Store) GetByID(ctx context.Context, id services.UserID) (*services.UserRecord, error) {
	var out *services.UserRecord
	err := database.CheckoutWithOrg(ctx, s.pool, s.orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `SELECT `+userColumns+` FROM identity.users WHERE id = $1`, string(id))
		u, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return services.ErrUserNotFound
			}
			return err
		}
		out = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetByFederation resolves the canonical user behind an OIDC (org,issuer,sub)
// triple — the R5-8 resolver. Returns ErrUserNotFound when unmatched.
func (s *Store) GetByFederation(ctx context.Context, orgID, issuer, sub string) (*services.UserRecord, error) {
	if orgID == "" {
		return nil, fmt.Errorf("identity: GetByFederation requires a non-empty orgID")
	}
	var out *services.UserRecord
	err := database.CheckoutWithOrg(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT `+columnsWithPrefix("u")+`
			 FROM identity.users u
			 JOIN identity.federated_identities f ON f.internal_user_id = u.id
			 WHERE f.org_id = $1 AND f.issuer = $2 AND f.sub = $3`,
			orgID, issuer, sub)
		u, err := scanUser(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return services.ErrUserNotFound
			}
			return err
		}
		out = u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// columnsWithPrefix returns userColumns qualified with a table alias.
func columnsWithPrefix(alias string) string {
	cols := strings.Split(userColumns, ", ")
	for i, c := range cols {
		cols[i] = alias + "." + c
	}
	return strings.Join(cols, ", ")
}

// listCursor is the opaque keyset cursor encoded into nextPageToken.
type listCursor struct {
	LastSeenAt time.Time `json:"l"`
	ID         string    `json:"i"`
}

func encodeCursor(c listCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(tok string) (listCursor, error) {
	var c listCursor
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return c, services.ErrInvalidPageToken
	}
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == "" {
		return c, services.ErrInvalidPageToken
	}
	return c, nil
}

const defaultListPageSize = 50
const maxListPageSize = 200

// List returns a keyset-paginated page ordered last_seen_at DESC, id DESC. A
// malformed cursor returns ErrInvalidPageToken.
func (s *Store) List(ctx context.Context, opts services.ListOpts) ([]*services.UserRecord, string, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultListPageSize
	}
	if pageSize > maxListPageSize {
		pageSize = maxListPageSize
	}

	var cur listCursor
	hasCursor := false
	if opts.PageToken != "" {
		c, err := decodeCursor(opts.PageToken)
		if err != nil {
			return nil, "", err
		}
		cur, hasCursor = c, true
	}

	var page []*services.UserRecord
	err := database.CheckoutWithOrg(ctx, s.pool, s.orgID, func(tx pgx.Tx) error {
		var rows pgx.Rows
		var err error
		if hasCursor {
			rows, err = tx.Query(ctx,
				`SELECT `+userColumns+` FROM identity.users
				 WHERE (last_seen_at, id) < ($1, $2)
				 ORDER BY last_seen_at DESC, id DESC
				 LIMIT $3`,
				cur.LastSeenAt, cur.ID, pageSize+1)
		} else {
			rows, err = tx.Query(ctx,
				`SELECT `+userColumns+` FROM identity.users
				 ORDER BY last_seen_at DESC, id DESC
				 LIMIT $1`,
				pageSize+1)
		}
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			u, serr := scanUser(rows)
			if serr != nil {
				return serr
			}
			page = append(page, u)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}

	next := ""
	if len(page) > pageSize {
		last := page[pageSize-1]
		page = page[:pageSize]
		next = encodeCursor(listCursor{LastSeenAt: last.LastSeenAt, ID: string(last.ID)})
	}
	return page, next, nil
}

// FederatedIdentitiesFor batch-loads federated identities for the given internal
// user IDs in one RLS-scoped read transaction (FR-U5/FR-U6, AC9). It groups rows
// into map[UserID][]FederatedIdentity. An empty ids slice is a no-op returning an
// empty map without touching the DB — never an N+1 per-user query.
func (s *Store) FederatedIdentitiesFor(ctx context.Context, ids []services.UserID) (map[services.UserID][]services.FederatedIdentity, error) {
	out := make(map[services.UserID][]services.FederatedIdentity, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	strIDs := make([]string, len(ids))
	for i, id := range ids {
		strIDs[i] = string(id)
	}

	err := database.CheckoutWithOrg(ctx, s.pool, s.orgID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT org_id, issuer, COALESCE(sub, ''), COALESCE(external_id, ''),
			        COALESCE(source_connector_id, ''), internal_user_id,
			        provider_kind, source_kind, created_at, updated_at
			 FROM identity.federated_identities
			 WHERE internal_user_id = ANY($1)`,
			strIDs)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var fi services.FederatedIdentity
			var internalUserID string
			if serr := rows.Scan(
				&fi.OrgID, &fi.Issuer, &fi.Sub, &fi.ExternalID,
				&fi.SourceConnectorID, &internalUserID,
				&fi.ProviderKind, &fi.SourceKind, &fi.CreatedAt, &fi.UpdatedAt,
			); serr != nil {
				return serr
			}
			fi.InternalUserID = services.UserID(internalUserID)
			out[fi.InternalUserID] = append(out[fi.InternalUserID], fi)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BilledSeatCount returns COUNT(*) WHERE state='active' for the org —
// entitlement-based and uniform across editions (FR-U7, R5-2). No window.
func (s *Store) BilledSeatCount(ctx context.Context) (int64, error) {
	var count int64
	err := database.CheckoutWithOrg(ctx, s.pool, s.orgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM identity.users
			 WHERE org_id = current_setting('app.org_id', true) AND state = 'active'`).Scan(&count)
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Provision is the reserved SCIM push verb (R5-6). No caller in this story.
func (s *Store) Provision(_ context.Context, _ services.ProvisionParams) (services.ObserveLoginResult, error) {
	return services.ObserveLoginResult{}, services.ErrNotImplemented
}

// Deactivate is the reserved SCIM deactivate verb (R5-6). No caller in this story.
func (s *Store) Deactivate(_ context.Context, _ services.UserID) error {
	return services.ErrNotImplemented
}
