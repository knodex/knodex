// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useInstanceList,
  useInstance,
  useUpdateInstanceSpec,
  useInstanceChildren,
} from "./useInstances";
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
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { queryClient, invalidateSpy, Wrapper };
}

describe("useInstances query/mutation hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useInstanceList", () => {
    it("fetches instances forwarding params", async () => {
      vi.mocked(rgdApi.listInstances).mockResolvedValue({
        items: [],
        pageCount: 0,
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useInstanceList({ namespace: "alpha-apps" } as never),
        { wrapper: Wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.listInstances).toHaveBeenCalledWith({
        namespace: "alpha-apps",
      });
    });
  });

  describe("useInstance", () => {
    it("does not fetch when group is empty (cluster-scoped guard)", async () => {
      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useInstance("", "ns", "WebApp", "my-app"),
        { wrapper: Wrapper },
      );

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(rgdApi.getInstance).not.toHaveBeenCalled();
    });

    it("fetches when group, kind and name are present (empty namespace ok)", async () => {
      vi.mocked(rgdApi.getInstance).mockResolvedValue({ name: "my-app" } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useInstance("kro.run", "", "WebApp", "my-app"),
        { wrapper: Wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getInstance).toHaveBeenCalledWith(
        "kro.run",
        "",
        "WebApp",
        "my-app",
      );
    });
  });

  describe("useUpdateInstanceSpec", () => {
    it("invalidates instance and list caches on success", async () => {
      vi.mocked(rgdApi.updateInstanceSpec).mockResolvedValue(undefined as never);

      const { invalidateSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useUpdateInstanceSpec(), {
        wrapper: Wrapper,
      });

      result.current.mutate({
        group: "kro.run",
        namespace: "alpha-apps",
        kind: "WebApp",
        name: "my-app",
        request: { spec: {} } as never,
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(rgdApi.updateInstanceSpec).toHaveBeenCalledWith(
        "kro.run",
        "alpha-apps",
        "WebApp",
        "my-app",
        { spec: {} },
      );
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual([
        "instance",
        "kro.run",
        "alpha-apps",
        "WebApp",
        "my-app",
      ]);
      expect(keys).toContainEqual(["instances"]);
    });
  });

  describe("useInstanceChildren", () => {
    it("does not fetch when name is empty", async () => {
      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useInstanceChildren("kro.run", "ns", "WebApp", ""),
        { wrapper: Wrapper },
      );

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(rgdApi.getInstanceChildren).not.toHaveBeenCalled();
    });

    it("fetches child resources when identity is complete", async () => {
      vi.mocked(rgdApi.getInstanceChildren).mockResolvedValue({} as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(
        () => useInstanceChildren("kro.run", "ns", "WebApp", "my-app"),
        { wrapper: Wrapper },
      );

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getInstanceChildren).toHaveBeenCalledWith(
        "kro.run",
        "ns",
        "WebApp",
        "my-app",
      );
    });
  });
});
