// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * useDeleteInstance cache-invalidation contract (Story 49.3, AC 4).
 *
 * Deleting a kagent-agent instance garbage-collects the Agent CR (KRO
 * ownerReferences), so the Agents hub's installed query must refetch. The
 * invalidation lives in onSettled — it must run even when the DELETE fails
 * (e.g. 404 for an already-gone instance) so stale entries never linger.
 * The Playwright flow cannot prove this — page.goto() resets the QueryClient
 * — so the contract is pinned here.
 */

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useDeleteInstance } from "./useInstances";
import * as rgdApi from "@/api/rgd";
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
  const removeSpy = vi.spyOn(queryClient, "removeQueries");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { queryClient, invalidateSpy, removeSpy, Wrapper };
}

const target = {
  group: "kro.run",
  namespace: "alpha-apps",
  kind: "KagentAgent",
  name: "my-agent",
};

describe("useDeleteInstance", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("invalidates the installed-agents query after a successful delete (Story 49.3 hub freshness)", async () => {
    vi.mocked(rgdApi.deleteInstance).mockResolvedValue(undefined);
    const { invalidateSpy, removeSpy, Wrapper } = createClientAndWrapper();

    const { result } = renderHook(() => useDeleteInstance(), {
      wrapper: Wrapper,
    });

    result.current.mutate(target);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(rgdApi.deleteInstance).toHaveBeenCalledWith(
      "kro.run",
      "alpha-apps",
      "KagentAgent",
      "my-agent",
    );

    // The deleted instance's own cache entry is dropped …
    expect(removeSpy).toHaveBeenCalledWith({
      queryKey: ["instance", "kro.run", "alpha-apps", "KagentAgent", "my-agent"],
    });

    // … and the hub + list queries refetch.
    const invalidatedKeys = invalidateSpy.mock.calls.map(
      ([filters]) => filters?.queryKey,
    );
    expect(invalidatedKeys).toContainEqual(["agents", "installed"]);
    expect(invalidatedKeys).toContainEqual(["instances"]);
    expect(invalidatedKeys).toContainEqual(["rgds"]);
  });

  it("still invalidates when the delete fails (onSettled contract — stale entries must not linger)", async () => {
    vi.mocked(rgdApi.deleteInstance).mockRejectedValue(
      new Error("404 instance not found"),
    );
    const { invalidateSpy, removeSpy, Wrapper } = createClientAndWrapper();

    const { result } = renderHook(() => useDeleteInstance(), {
      wrapper: Wrapper,
    });

    result.current.mutate(target);
    await waitFor(() => expect(result.current.isError).toBe(true));

    // Even a failed DELETE (e.g. already garbage-collected) refreshes the
    // caches so the UI converges on the cluster's actual state.
    expect(removeSpy).toHaveBeenCalledWith({
      queryKey: ["instance", "kro.run", "alpha-apps", "KagentAgent", "my-agent"],
    });
    const invalidatedKeys = invalidateSpy.mock.calls.map(
      ([filters]) => filters?.queryKey,
    );
    expect(invalidatedKeys).toContainEqual(["agents", "installed"]);
    expect(invalidatedKeys).toContainEqual(["instances"]);
  });
});
