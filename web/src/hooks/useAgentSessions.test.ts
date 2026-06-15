// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { useAgentSessions, useAgentSession } from "./useAgentSessions";

vi.mock("@/api/agent-sessions", () => ({
  listAgentSessions: vi.fn(),
  getAgentSession: vi.fn(),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("useAgentSessions", () => {
  beforeEach(() => vi.clearAllMocks());

  it("fetches the session list with the given params", async () => {
    const { listAgentSessions } = await import("@/api/agent-sessions");
    const payload = { items: [], total: 0, page: 1, pageSize: 20 };
    vi.mocked(listAgentSessions).mockResolvedValue(payload);

    const { result } = renderHook(() => useAgentSessions({ agentType: "rgd-builder" }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(listAgentSessions).toHaveBeenCalledWith({ agentType: "rgd-builder" });
    expect(result.current.data).toEqual(payload);
  });
});

describe("useAgentSession", () => {
  beforeEach(() => vi.clearAllMocks());

  it("fetches one session when an id is provided", async () => {
    const { getAgentSession } = await import("@/api/agent-sessions");
    const session = { id: "conv-1", agentType: "rgd-builder", agentNamespace: "", runs: [] };
    vi.mocked(getAgentSession).mockResolvedValue(session);

    const { result } = renderHook(() => useAgentSession("conv-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(getAgentSession).toHaveBeenCalledWith("conv-1");
    expect(result.current.data).toEqual(session);
  });

  it("is disabled (does not fetch) when id is null or empty", async () => {
    const { getAgentSession } = await import("@/api/agent-sessions");

    const { result: nullResult } = renderHook(() => useAgentSession(null), {
      wrapper: createWrapper(),
    });
    const { result: emptyResult } = renderHook(() => useAgentSession(""), {
      wrapper: createWrapper(),
    });

    expect(nullResult.current.fetchStatus).toBe("idle");
    expect(emptyResult.current.fetchStatus).toBe("idle");
    expect(getAgentSession).not.toHaveBeenCalled();
  });
});
