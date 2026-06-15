// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from "@tanstack/react-query";
import { deleteUser, listUsers } from "@/api/users";
import type { DeleteUserResponse, UsersListResponse } from "@/types/user";
import { STALE_TIME } from "@/lib/query-client";
import { is403, is404 } from "@/lib/errors";

/**
 * useUsers — keyset-paginated roster fetch over `GET /api/v1/users`.
 *
 * The roster API exists on ALL editions (Postgres mandatory, R5-5), so this is
 * deliberately NOT gated on `isEnterprise()` — the page renders on OSS and EE
 * alike. 403s (non-operator) are NOT retried (mirrors the `is403` predicate in
 * useAudit.ts) so the page can fall through to its Access Denied state quickly.
 *
 * Pagination uses `useInfiniteQuery` keyed on the opaque `nextPageToken`
 * cursor; consumers flatten `data.pages[*].users` and call `fetchNextPage()`
 * for the "Load more" affordance (`hasNextPage` reflects a present cursor).
 */
export function useUsers(limit?: number) {
  return useInfiniteQuery<UsersListResponse>({
    queryKey: ["users", limit],
    queryFn: ({ pageParam }) =>
      listUsers({ limit, pageToken: pageParam as string | undefined }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextPageToken ?? undefined,
    staleTime: STALE_TIME.STANDARD,
    retry: (failureCount, error) => {
      if (is403(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * useReclaimUser — reclaim a seat by removing a user (`DELETE /api/v1/users/{id}`,
 * operator-gated via `settings/* update`). Mirrors `useDeleteTeam`.
 *
 * **404-as-success:** the `Remove` is idempotent, so a 404 means the user was
 * already removed concurrently — we swallow it in the `mutationFn` and resolve
 * with a synthetic `removed` response. `onSuccess` therefore still fires and
 * `onError` never sees a 404 (no error-toast storm — AC #4). Any other error
 * propagates to the caller's `catch`.
 *
 * On success we invalidate BOTH `["users"]` (the roster flips the row to
 * `removed` / drops the active count) AND `["license"]` (the seat-usage widget's
 * `used` drops by one).
 */
export function useReclaimUser() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string): Promise<DeleteUserResponse> => {
      try {
        return await deleteUser(id);
      } catch (error) {
        if (is404(error)) {
          // Already removed — treat as a no-op success so the invalidations
          // below still refresh the (now-consistent) roster + seat count.
          return { id, state: "removed", note: "" };
        }
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      queryClient.invalidateQueries({ queryKey: ["license"] });
    },
  });
}
