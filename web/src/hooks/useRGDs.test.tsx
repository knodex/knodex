// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * useCreateInstance cache-invalidation contract (Story 49.3, AC 2).
 *
 * The Agents hub's installed query (["agents", "installed"], 30s staleTime)
 * must be invalidated when an instance is created, so a deployed kagent-agent
 * appears in "Installed Agents" without waiting out the staleTime. The
 * Playwright flow cannot prove this — page.goto() reloads the SPA and resets
 * the QueryClient — so the invalidation contract is pinned here.
 */

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useCreateInstance } from "./useRGDs";
import * as rgdApi from "@/api/rgd";
import type { CreateInstanceResponse } from "@/types/rgd";
import type { ReactNode } from "react";

vi.mock("@/api/rgd");

function createClientAndWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { queryClient, invalidateSpy, Wrapper };
}

const createResponse: CreateInstanceResponse = {
  name: "my-agent",
  namespace: "alpha-apps",
  rgdName: "kagent-agent",
  apiGroup: "kro.run",
  kind: "KagentAgent",
  version: "v1alpha1",
  status: "created",
  createdAt: "2026-06-06T10:00:00Z",
};

describe("useCreateInstance", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls the GVK-aware create API with (group, kind, request)", async () => {
    vi.mocked(rgdApi.createInstance).mockResolvedValue(createResponse);
    const { Wrapper } = createClientAndWrapper();

    const { result } = renderHook(() => useCreateInstance(), {
      wrapper: Wrapper,
    });

    result.current.mutate({
      group: "kro.run",
      kind: "KagentAgent",
      name: "my-agent",
      namespace: "alpha-apps",
      rgdName: "kagent-agent",
      spec: { agentName: "my-agent" },
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(rgdApi.createInstance).toHaveBeenCalledWith(
      "kro.run",
      "KagentAgent",
      {
        name: "my-agent",
        namespace: "alpha-apps",
        rgdName: "kagent-agent",
        spec: { agentName: "my-agent" },
      },
    );
  });

  it("invalidates the installed-agents query on success (Story 49.3 hub freshness)", async () => {
    vi.mocked(rgdApi.createInstance).mockResolvedValue(createResponse);
    const { invalidateSpy, Wrapper } = createClientAndWrapper();

    const { result } = renderHook(() => useCreateInstance(), {
      wrapper: Wrapper,
    });

    result.current.mutate({
      group: "kro.run",
      kind: "KagentAgent",
      name: "my-agent",
      namespace: "alpha-apps",
      rgdName: "kagent-agent",
      spec: {},
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const invalidatedKeys = invalidateSpy.mock.calls.map(
      ([filters]) => filters?.queryKey,
    );
    // The hub's installed list must refetch — unconditional, even for
    // non-agent instances (one cheap LIST).
    expect(invalidatedKeys).toContainEqual(["agents", "installed"]);
    // Existing invalidations must stay intact.
    expect(invalidatedKeys).toContainEqual(["rgd-instances", "kagent-agent"]);
    expect(invalidatedKeys).toContainEqual(["instances"]);
    expect(invalidatedKeys).toContainEqual(["rgds"]);
  });

  it("does not invalidate any query when creation fails", async () => {
    vi.mocked(rgdApi.createInstance).mockRejectedValue(
      new Error("403 PERMISSION_DENIED"),
    );
    const { invalidateSpy, Wrapper } = createClientAndWrapper();

    const { result } = renderHook(() => useCreateInstance(), {
      wrapper: Wrapper,
    });

    result.current.mutate({
      group: "kro.run",
      kind: "KagentAgent",
      name: "my-agent",
      namespace: "alpha-apps",
      rgdName: "kagent-agent",
      spec: {},
    });

    await waitFor(() => expect(result.current.isError).toBe(true));

    // A denied deploy (AC 3) must not churn caches — nothing changed.
    expect(invalidateSpy).not.toHaveBeenCalled();
  });
});
