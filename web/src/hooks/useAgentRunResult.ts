// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getAgentRunResult, type AgentRunResult } from "@/api/agent-runs";

/** Poll interval while the result is not yet available (404 → null). */
export const RESULT_REFETCH_INTERVAL_MS = 5_000;

/**
 * Hard client-side timeout: slightly over the server's worst-case budget,
 * so a run the server gave up on (or a lost result) surfaces as an
 * actionable error instead of an eternal spinner (AC #4). Story 50.3 raised
 * this from 150s: the worst chain is now TWO sequential 125s A2A calls
 * (primary + one policy-revision attempt) plus two 30s Gatekeeper
 * validations ≈ 310s; the late-result upgrade path stays the backstop.
 */
export const RESULT_TIMEOUT_MS = 330_000;

/**
 * Poll until the result arrives, stop once it has. `null` data means the
 * server answered 404 — the run is still in-flight. Exported for unit
 * testing: this conditional is the polling CONTRACT (WS invalidation via the
 * ["agents","runs"] prefix is latency sugar on top).
 */
export function resultRefetchInterval(query: {
  state: { data?: AgentRunResult | null };
}): number | false {
  return query.state.data == null ? RESULT_REFETCH_INTERVAL_MS : false;
}

export interface UseAgentRunResultReturn {
  /** The terminal result, or null/undefined while in-flight. */
  result: AgentRunResult | null | undefined;
  /** True when polling outlived RESULT_TIMEOUT_MS without a result. */
  timedOut: boolean;
  /** Non-404 fetch failure (404 maps to null, not an error). */
  isError: boolean;
}

/**
 * Poll the full result of an agent run (Story 50.1). Keyed under the
 * ["agents","runs"] prefix so the existing agent_run_update WebSocket
 * invalidation refreshes it for free; the 5s conditional poll is the
 * contract that works without WS. After RESULT_TIMEOUT_MS without a result,
 * `timedOut` flips true so the page can render the actionable timeout error.
 */
export function useAgentRunResult(runId: string | null): UseAgentRunResultReturn {
  const { data, isError } = useQuery({
    queryKey: ["agents", "runs", runId, "result"],
    queryFn: () => getAgentRunResult(runId as string),
    enabled: runId != null,
    refetchInterval: resultRefetchInterval,
    // 404→null is a VALID response (in-flight) — keep polling, don't retry.
    retry: false,
  });

  // Elapsed-based timeout, keyed by runId: the timer (re-)arms per run and
  // a stale flag from a previous run never leaks — no synchronous state
  // reset inside the effect needed.
  const [timedOutRun, setTimedOutRun] = useState<string | null>(null);
  const hasResult = data != null;
  useEffect(() => {
    if (runId == null || hasResult) return;
    const timer = setTimeout(() => setTimedOutRun(runId), RESULT_TIMEOUT_MS);
    return () => clearTimeout(timer);
  }, [runId, hasResult]);

  const timedOut = runId != null && timedOutRun === runId && !hasResult;
  return { result: data, timedOut, isError };
}
