// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Canonical user-roster DTOs — mirror the API serialization of
 * `server/internal/api/handlers/users.go` exactly (camelCase JSON tags).
 *
 * This is a CONSUMER of the frozen 15.8 Users API. Do not add fields the API
 * does not serialize; `externalId`/`sourceConnectorId` are intentionally NOT
 * served (NFR-U2) and therefore absent here.
 */

/** A single federated (IdP) identity linked to a user. */
export interface FederatedIdentity {
  issuer: string;
  sub: string;
  providerKind: string;
  sourceKind: string;
  createdAt: string;
  updatedAt: string;
}

/** Active/removed lifecycle state of a roster user (`services.UserState*`). */
export type UserState = "active" | "removed";

/**
 * Effective two-axis application role (Epic 17). Server-derived from Casbin and
 * read-only here — `auditor` is deferred but tolerated so a future server value
 * never breaks the type. NOT a project role.
 */
export type ApplicationRole = "member" | "serveradmin" | (string & {});

/** A canonical `identity.users` row with its federated identities expanded. */
export interface User {
  id: string;
  email: string;
  displayName: string;
  state: UserState;
  /** Display-only inactivity flag (FR-U14) — rendered in 16.3, not 16.1. */
  isInactive: boolean;
  firstSeenAt: string;
  lastSeenAt: string;
  federatedIdentities: FederatedIdentity[];
  /**
   * Server-derived, read-only application role (Story 17.3): "serveradmin" iff
   * the user effectively holds role:serveradmin in Casbin, else "member". This
   * is an APPLICATION role (orthogonal to project roles) and has no assignment
   * affordance in the UI — it changes at the IdP / group-mapping (Path A).
   */
  applicationRole: ApplicationRole;
}

/** A keyset-paginated page from `GET /api/v1/users`. */
export interface UsersListResponse {
  users: User[];
  /** Opaque cursor for the next page; omitted when there are no more pages. */
  nextPageToken?: string;
}

/**
 * Reclaim-semantics body returned by `DELETE /api/v1/users/{id}` (200, not 204
 * — the note can't ride on an empty body). Mirrors `handlers.DeleteUserResponse`
 * (server/internal/api/handlers/users.go:71). `note` carries the verbatim
 * reclaim + IdP-revocation copy ("Seat reclaimed. Permanent exclusion requires
 * IdP-side revocation.").
 */
export interface DeleteUserResponse {
  id: string;
  state: string;
  note: string;
}
