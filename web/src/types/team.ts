// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Team types for the OSS Teams management UI (Story 10.4).
 *
 * A Team is a cluster-scoped, named set of OIDC groups. It produces no
 * authorization on its own — it only grants access when a Project role binds it
 * via `roles[].teams[]`, at which point the server resolves the team to its
 * groups through the single Casbin enforcement layer (Story 10.2).
 *
 * These shapes mirror the backend DTOs in
 * `server/internal/api/handlers/team_handler.go`.
 */

/** A Team as returned by the API. */
export interface Team {
  /** Cluster-scoped team name (DNS-1123 subdomain). */
  name: string;
  /** Human-readable description. */
  description?: string;
  /** OIDC groups this team represents. */
  oidcGroups: string[];
  /**
   * True iff the team was materialized by the control-plane reconciler
   * (annotation knodex.io/created-by == "control-plane"). Always false for
   * operator-authored teams; always false in OSS/EE.
   */
  managed?: boolean;
}

/** Values managed by the TeamDrawer create/edit form. */
export interface TeamFormValues {
  name: string;
  description: string;
  oidcGroups: string[];
}

/** Response from GET /v1/teams. */
export interface TeamListResponse {
  items: Team[];
  totalCount: number;
}

/** Body for POST /v1/teams. */
export interface CreateTeamRequest {
  name: string;
  description?: string;
  oidcGroups: string[];
}

/** Body for PUT /v1/teams/{name} (name is immutable). */
export interface UpdateTeamRequest {
  description?: string;
  oidcGroups: string[];
}

/** A single observed OIDC group from GET /v1/groups/observed (Story 10.3). */
export interface ObservedGroup {
  /** The observed OIDC group identifier. */
  name: string;
  /** ISO-8601 timestamp of when this group was most recently seen at login. */
  lastSeen: string;
}

/** Response from GET /v1/groups/observed. */
export interface ObservedGroupsResponse {
  groups: ObservedGroup[];
}
