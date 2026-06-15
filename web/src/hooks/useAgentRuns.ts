// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { listAgentRuns, type AgentRunsListParams, type AgentRunsResponse } from "@/api/agent-runs";
import { STALE_TIME } from "@/lib/query-client";

/** Poll interval while any run on the current page is in-flight. */
export const RUNNING_REFETCH_INTERVAL_MS = 5_000;

/**
 * Poll while any run on the current page is in-flight, stop otherwise.
 * Exported for unit testing — this conditional is the ONLY live-update path
 * for non-actors (the WS push is actor-or-admin delivery).
 */
export function runsRefetchInterval(query: {
  state: { data?: AgentRunsResponse };
}): number | false {
  return query.state.data?.items.some((run) => run.status === "running")
    ? RUNNING_REFETCH_INTERVAL_MS
    : false;
}

/**
 * Hook for the paginated agent run history (Story 49.4).
 *
 * Live updates arrive two ways: the WebSocket agent_run_update push
 * invalidates the ["agents", "runs"] prefix (actor + global admins only —
 * see useWebSocket), and a cheap conditional 5s poll keeps NON-actors'
 * tables converging while any visible run is still running (the list is a
 * <500ms non-LLM op per NFR-A1).
 */
export function useAgentRuns(params?: AgentRunsListParams) {
  return useQuery({
    queryKey: ["agents", "runs", params],
    queryFn: () => listAgentRuns(params),
    placeholderData: keepPreviousData,
    staleTime: STALE_TIME.REALTIME,
    refetchInterval: runsRefetchInterval,
  });
}
