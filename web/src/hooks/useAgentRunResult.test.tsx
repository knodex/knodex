// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  useAgentRunResult,
  resultRefetchInterval,
  RESULT_REFETCH_INTERVAL_MS,
  RESULT_TIMEOUT_MS,
} from "./useAgentRunResult";
import * as agentRunsApi from "@/api/agent-runs";
import type { AgentRunResult } from "@/api/agent-runs";

vi.mock("@/api/agent-runs");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

const completedResult: AgentRunResult = {
  runId: "run-1",
  agentNamespace: "",
  status: "completed",
  response: "Here is your spec:\n```yaml\nkind: ResourceGraphDefinition\n```",
  error: "",
  completedAt: "2026-06-06T10:00:30Z",
};

/**
 * The conditional poll is the CONTRACT — it must keep firing while the
 * server answers 404 (mapped to null) and stop the moment a result lands.
 */
describe("resultRefetchInterval", () => {
  it("polls every 5s before the first fetch resolves (no data yet)", () => {
    expect(resultRefetchInterval({ state: { data: undefined } })).toBe(
      RESULT_REFETCH_INTERVAL_MS
    );
  });

  it("polls every 5s while the result is null (404 — still in-flight)", () => {
    expect(resultRefetchInterval({ state: { data: null } })).toBe(
      RESULT_REFETCH_INTERVAL_MS
    );
  });

  it("stops polling once the result has arrived", () => {
    expect(resultRefetchInterval({ state: { data: completedResult } })).toBe(false);
  });
});

describe("useAgentRunResult", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not fetch while runId is null", async () => {
    const { result } = renderHook(() => useAgentRunResult(null), {
      wrapper: createWrapper(),
    });

    expect(result.current.result).toBeUndefined();
    expect(result.current.timedOut).toBe(false);
    expect(agentRunsApi.getAgentRunResult).not.toHaveBeenCalled();
  });

  it("fetches and returns the result once available", async () => {
    vi.mocked(agentRunsApi.getAgentRunResult).mockResolvedValue(completedResult);

    const { result } = renderHook(() => useAgentRunResult("run-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.result).toEqual(completedResult);
    });
    expect(agentRunsApi.getAgentRunResult).toHaveBeenCalledWith("run-1");
    expect(result.current.timedOut).toBe(false);
  });

  it("returns null result while in-flight (server 404) without flagging an error", async () => {
    vi.mocked(agentRunsApi.getAgentRunResult).mockResolvedValue(null);

    const { result } = renderHook(() => useAgentRunResult("run-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.result).toBeNull();
    });
    expect(result.current.isError).toBe(false);
    expect(result.current.timedOut).toBe(false);
  });

  it("surfaces non-404 failures as isError", async () => {
    vi.mocked(agentRunsApi.getAgentRunResult).mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => useAgentRunResult("run-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it("covers the Story 50.3 worst chain: two 125s A2A calls + two 30s validations", () => {
    // Pinned: slightly over 2×125s (primary + one policy revision) plus
    // 2×30s Gatekeeper validations ≈ 310s of server budget.
    expect(RESULT_TIMEOUT_MS).toBe(330_000);
    expect(RESULT_TIMEOUT_MS).toBeGreaterThan(2 * 125_000 + 2 * 30_000);
  });

  it("flips timedOut after the hard timeout without a result", async () => {
    vi.useFakeTimers();
    try {
      vi.mocked(agentRunsApi.getAgentRunResult).mockResolvedValue(null);

      const { result } = renderHook(() => useAgentRunResult("run-1"), {
        wrapper: createWrapper(),
      });

      // Just before the timeout: still patiently polling.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(RESULT_TIMEOUT_MS - 1_000);
      });
      expect(result.current.timedOut).toBe(false);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(2_000);
      });
      expect(result.current.timedOut).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it("never reports timedOut once a result has arrived", async () => {
    vi.useFakeTimers();
    try {
      vi.mocked(agentRunsApi.getAgentRunResult).mockResolvedValue(completedResult);

      const { result } = renderHook(() => useAgentRunResult("run-1"), {
        wrapper: createWrapper(),
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(200_000);
      });
      expect(result.current.result).toEqual(completedResult);
      expect(result.current.timedOut).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });
});
