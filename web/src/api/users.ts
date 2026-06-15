// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import type { DeleteUserResponse, UsersListResponse } from "@/types/user";

/**
 * List the canonical user roster (operator-gated via `settings/* get`).
 *
 * Keyset-paginated: pass the `nextPageToken` from a prior page as `pageToken`
 * to fetch the next page. `limit` is clamped server-side to 1..200 (default 50;
 * out-of-range → 400).
 */
export async function listUsers(params?: {
  limit?: number;
  pageToken?: string;
}): Promise<UsersListResponse> {
  const response = await apiClient.get<UsersListResponse>("/v1/users", {
    params: {
      limit: params?.limit,
      pageToken: params?.pageToken,
    },
  });
  return response.data;
}

/**
 * Reclaim a seat by removing a user (operator-gated via `settings/* update`).
 *
 * This is a soft-delete / reclaim, NOT a hard delete: the underlying `Remove` is
 * idempotent and a subsequent SSO login resurrects the row. The server returns a
 * 200 with a `DeleteUserResponse` carrying the reclaim note (a 204 could not),
 * so we capture and return the body. A 404 (already removed) is left to the
 * caller to treat as a no-op (see `useReclaimUser`).
 */
export async function deleteUser(id: string): Promise<DeleteUserResponse> {
  const response = await apiClient.delete<DeleteUserResponse>(
    `/v1/users/${encodeURIComponent(id)}`,
  );
  return response.data;
}
