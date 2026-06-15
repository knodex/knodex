// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useInstanceKubernetesEvents,
  useInstanceEvents,
  useInstanceHistory,
} from "./useHistory";
import * as historyApi from "@/api/history";
import type { ReactNode } from "react";

vi.mock("@/api/history");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("useHistory event hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useInstanceKubernetesEvents", () => {
    it("does not fetch when kind or name is empty", async () => {
      const { result } = renderHook(
        () => useInstanceKubernetesEvents("g", "ns", "", "name"),
        { wrapper: createWrapper() },
      );
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(historyApi.getInstanceKubernetesEvents).not.toHaveBeenCalled();
    });

    it("fetches kubernetes events when identity is complete", async () => {
      vi.mocked(historyApi.getInstanceKubernetesEvents).mockResolvedValue(
        [] as never,
      );
      const { result } = renderHook(
        () => useInstanceKubernetesEvents("g", "ns", "WebApp", "name"),
        { wrapper: createWrapper() },
      );
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(historyApi.getInstanceKubernetesEvents).toHaveBeenCalledWith(
        "g",
        "ns",
        "WebApp",
        "name",
      );
    });
  });

  describe("useInstanceEvents", () => {
    it("does not fetch when name is empty", async () => {
      const { result } = renderHook(
        () => useInstanceEvents("g", "ns", "WebApp", ""),
        { wrapper: createWrapper() },
      );
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(historyApi.getInstanceEvents).not.toHaveBeenCalled();
    });

    it("fetches K8s events when identity is complete", async () => {
      vi.mocked(historyApi.getInstanceEvents).mockResolvedValue([] as never);
      const { result } = renderHook(
        () => useInstanceEvents("g", "ns", "WebApp", "name"),
        { wrapper: createWrapper() },
      );
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(historyApi.getInstanceEvents).toHaveBeenCalledWith(
        "g",
        "ns",
        "WebApp",
        "name",
      );
    });
  });

  describe("useInstanceHistory disabled branch", () => {
    it("does not fetch when kind is empty", async () => {
      const { result } = renderHook(
        () => useInstanceHistory("g", "ns", "", "name"),
        { wrapper: createWrapper() },
      );
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(historyApi.getInstanceHistory).not.toHaveBeenCalled();
    });
  });
});
