// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

package services

import (
	"context"
	"errors"
	"time"
)

// UserID is the ULID primary key of an identity.users row (26-char Crockford
// base32). It is globally unique across orgs — cross-org collisions are
// statistically impossible.
type UserID string

// UserRecord is the canonical persistent user materialized on every successful
// OIDC login. It maps 1:1 to a row in identity.users (Story 15.2 / FR-U1).
type UserRecord struct {
	ID          UserID
	OrgID       string
	Email       string
	DisplayName string
	// State is "active" or "removed" (soft-delete; resurrect-on-login — R5-3/R5-4).
	State       string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// State constants for UserRecord.State (CHECK (state IN ('active','removed'))).
const (
	UserStateActive  = "active"
	UserStateRemoved = "removed"
)

// FederatedIdentity maps 1:1 to a row in identity.federated_identities — one row
// per (org_id, issuer, sub) OIDC triple, with SCIM-ready external_id /
// source_connector_id / nullable sub (R5-6).
type FederatedIdentity struct {
	OrgID             string
	Issuer            string
	Sub               string // empty for SCIM-pushed rows that have not yet logged in
	ExternalID        string // IdP-side opaque id (SCIM externalID); empty on OIDC
	SourceConnectorID string // SCIM connector id; empty on OIDC
	InternalUserID    UserID
	ProviderKind      string // default "oidc"
	SourceKind        string // "oidc_jit" | "keycloak_projection" | "scim_push"
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SourceKind values for FederatedIdentity.SourceKind (R5-6).
const (
	SourceKindOIDCJIT            = "oidc_jit"
	SourceKindKeycloakProjection = "keycloak_projection"
	SourceKindSCIMPush           = "scim_push"
)

// ObserveLoginParams carries the OIDC-token-derived inputs for a login
// materialization. SourceKind is NOT here — it is a build-tag constant on the
// base store (R5-6 / Story 15.4). No tx is threaded (R5-7).
type ObserveLoginParams struct {
	OrgID         string
	Issuer        string
	Sub           string
	Email         string
	DisplayName   string
	ProviderKind  string
	EmailVerified bool
}

// ObserveLoginResult reports what the store did so callers (and post-commit
// hooks) can react.
type ObserveLoginResult struct {
	ID           UserID
	Created      bool // JIT INSERT path
	Resurrected  bool // row existed with state='removed' and was flipped to 'active'
	EmailChanged bool // a verified-email update overwrote a prior value AND the write succeeded
}

// ProvisionParams is the reserved push verb for SCIM (Epic 13) and Cloud
// invite-time provisioning (R5-6/R5-9). Declared now so SCIM is a thin adapter
// into the SAME store; the base-store impl MAY return ErrNotImplemented until a
// caller lands.
type ProvisionParams struct {
	OrgID       string
	ConnectorID string
	ExternalID  string
	Email       string
	DisplayName string
	Active      bool
}

// ListOpts parameterises IdentityService.List. PageToken is an opaque
// base64url keyset cursor; an empty token starts at the first page.
type ListOpts struct {
	PageSize  int
	PageToken string
}

// IdentityHooks are optional callbacks invoked synchronously AFTER the base
// store's transaction commits. A nil callback is a no-op. Hook errors are
// logged at ERROR and increment a metric; they do NOT roll back the identity
// write and do NOT fail the login (R5-7, NFR-U1).
type IdentityHooks struct {
	OnFirstSeen    func(ctx context.Context, u *UserRecord) error
	OnEmailChanged func(ctx context.Context, u *UserRecord, oldEmail string) error
	OnRemoved      func(ctx context.Context, u *UserRecord) error
	OnResurrected  func(ctx context.Context, u *UserRecord) error
}

// Identity store error sentinels.
var (
	// ErrUserNotFound is returned by GetByID/GetByFederation/Remove when no row
	// matches in the caller's org. Cross-org lookups also return this (RLS hides
	// the row) — never a 403.
	ErrUserNotFound = errors.New("identity: user not found")
	// ErrInvalidPageToken is returned by List for a malformed cursor.
	ErrInvalidPageToken = errors.New("identity: invalid page token")
	// ErrNotImplemented is returned by the reserved Provision/Deactivate verbs
	// until a caller lands (R5-6).
	ErrNotImplemented = errors.New("identity: not implemented")
)

// IdentityService is the canonical user-persistence port, implemented by the
// base Postgres store on all editions (R5-2). There is NO tx parameter anywhere
// (R5-7) and BilledSeatCount takes NO window argument (R5-2).
type IdentityService interface {
	// ObserveLogin materializes (JIT) or refreshes the user record for a
	// successful OIDC login (FR-U1). Best-effort at runtime: a failure must
	// never fail the login.
	ObserveLogin(ctx context.Context, p ObserveLoginParams) (ObserveLoginResult, error)

	// Provision is the reserved SCIM push verb (R5-6). MAY return ErrNotImplemented.
	Provision(ctx context.Context, p ProvisionParams) (ObserveLoginResult, error)

	// Deactivate is the reserved SCIM deactivate verb (R5-6). MAY return ErrNotImplemented.
	Deactivate(ctx context.Context, id UserID) error

	// Remove soft-deletes a user (state='removed') — FR-U13. Idempotent.
	Remove(ctx context.Context, id UserID) error

	// GetByID returns a user by ULID, or ErrUserNotFound.
	GetByID(ctx context.Context, id UserID) (*UserRecord, error)

	// GetByFederation resolves the canonical user behind an OIDC (org,issuer,sub)
	// triple — the R5-8 resolver. Returns ErrUserNotFound when unmatched.
	GetByFederation(ctx context.Context, orgID, issuer, sub string) (*UserRecord, error)

	// List returns a keyset-paginated page of users ordered last_seen_at DESC, id DESC.
	List(ctx context.Context, opts ListOpts) (page []*UserRecord, nextPageToken string, err error)

	// FederatedIdentitiesFor batch-loads the federated identities for the given
	// internal user IDs, scoped to the caller's org (RLS). It is the expansion
	// the Users API (Story 15.8 / FR-U5/FR-U6) needs because List/GetByID return
	// only UserRecord with no federated rows. The result maps each UserID to its
	// (possibly empty) slice of identities; ids not present have no key. An empty
	// ids slice is a no-op that returns an empty map without querying. The
	// implementation MUST use a single batched query (no N+1).
	FederatedIdentitiesFor(ctx context.Context, ids []UserID) (map[UserID][]FederatedIdentity, error)

	// BilledSeatCount returns COUNT(*) WHERE state='active' for the org —
	// entitlement-based and uniform across editions (FR-U7, R5-2). No window.
	BilledSeatCount(ctx context.Context) (int64, error)
}
