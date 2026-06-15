// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, keepPreviousData } from "@tanstack/react-query";
import {
  listAgentSessions,
  getAgentSession,
  type AgentSessionsListParams,
} from "@/api/agent-sessions";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Paginated chat-session list (Story 50.6). Keyed under ["agents","sessions"]
 * so it coexists with the ["agents","runs"] caches; FREQUENT staleness since a
 * session list is a cheap non-LLM view that converges as new turns land.
 */
export function useAgentSessions(params?: AgentSessionsListParams) {
  return useQuery({
    queryKey: ["agents", "sessions", params],
    queryFn: () => listAgentSessions(params),
    placeholderData: keepPreviousData,
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * One session's runs for read-only replay (Story 50.6). Enabled only when an
 * id is present; the per-run full results are fetched separately (non-polling)
 * by the replay page.
 */
export function useAgentSession(id: string | null | undefined) {
  return useQuery({
    queryKey: ["agents", "sessions", id],
    queryFn: () => getAgentSession(id as string),
    enabled: id != null && id !== "",
    staleTime: STALE_TIME.FREQUENT,
  });
}
