// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import { getAgentsStatus } from "@/api/agents";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching kagent presence status for the Agents hub (Story 49.1).
 * FREQUENT staleTime — presence can change when an operator installs kagent.
 */
export function useAgentsStatus() {
  return useQuery({
    queryKey: ["agents", "status"],
    queryFn: getAgentsStatus,
    staleTime: STALE_TIME.FREQUENT,
  });
}
