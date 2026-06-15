// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AxiosError, AxiosHeaders } from "axios";
import {
  useRGDList,
  useRGDFilters,
  useRGD,
  useRGDResourceGraph,
  useRGDDefinitionGraph,
  useRGDSchema,
  useRGDInstances,
  useK8sResources,
  useRGDRevisions,
  useRGDRevision,
  useRGDRevisionDiff,
} from "./useRGDs";
import * as rgdApi from "@/api/rgd";
import * as k8sApi from "@/api/k8s";
import type { ReactNode } from "react";

vi.mock("@/api/rgd");
vi.mock("@/api/k8s");

function createWrapper(retry: boolean | ((c: number, e: Error) => boolean) = false) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function make403(): AxiosError {
  return new AxiosError("Forbidden", "ERR_BAD_REQUEST", undefined, undefined, {
    status: 403,
    data: {},
    statusText: "Forbidden",
    headers: {},
    config: { headers: new AxiosHeaders() },
  });
}

describe("useRGDs query hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useRGDList", () => {
    it("is disabled when params is undefined", async () => {
      const { result } = renderHook(() => useRGDList(undefined), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(rgdApi.listRGDs).not.toHaveBeenCalled();
    });

    it("fetches when params is provided (even empty object)", async () => {
      vi.mocked(rgdApi.listRGDs).mockResolvedValue({ items: [] } as never);
      const { result } = renderHook(() => useRGDList({}), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.listRGDs).toHaveBeenCalledWith({});
    });
  });

  describe("useRGDFilters", () => {
    it("fetches authorized filter options", async () => {
      vi.mocked(rgdApi.getRGDFilters).mockResolvedValue({
        categories: [],
      } as never);
      const { result } = renderHook(() => useRGDFilters(), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDFilters).toHaveBeenCalledTimes(1);
    });
  });

  describe("useRGD", () => {
    it("does not fetch when name is empty", async () => {
      const { result } = renderHook(() => useRGD(""), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(rgdApi.getRGD).not.toHaveBeenCalled();
    });

    it("fetches with name and namespace", async () => {
      vi.mocked(rgdApi.getRGD).mockResolvedValue({ name: "web" } as never);
      const { result } = renderHook(() => useRGD("web", "ns"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGD).toHaveBeenCalledWith("web", "ns");
    });
  });

  describe("useRGDResourceGraph", () => {
    it("does not fetch when name is empty", async () => {
      const { result } = renderHook(() => useRGDResourceGraph(""), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
    });

    it("fetches the resource graph", async () => {
      vi.mocked(rgdApi.getRGDResourceGraph).mockResolvedValue({} as never);
      const { result } = renderHook(() => useRGDResourceGraph("web"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDResourceGraph).toHaveBeenCalledWith("web", undefined);
    });
  });

  describe("useRGDDefinitionGraph", () => {
    it("fetches the definition graph", async () => {
      vi.mocked(rgdApi.getRGDDefinitionGraph).mockResolvedValue({} as never);
      const { result } = renderHook(() => useRGDDefinitionGraph("web", "ns"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDDefinitionGraph).toHaveBeenCalledWith("web", "ns");
    });

    it("does not fetch with empty name", async () => {
      const { result } = renderHook(() => useRGDDefinitionGraph(""), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
    });
  });

  describe("useRGDSchema", () => {
    it("fetches the CRD schema", async () => {
      vi.mocked(rgdApi.getRGDSchema).mockResolvedValue({} as never);
      const { result } = renderHook(() => useRGDSchema("web"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDSchema).toHaveBeenCalledWith("web", undefined);
    });

    it("does not fetch with empty name", async () => {
      const { result } = renderHook(() => useRGDSchema(""), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
    });
  });

  describe("useRGDInstances", () => {
    it("fetches instances of an RGD", async () => {
      vi.mocked(rgdApi.listRGDInstances).mockResolvedValue({ items: [] } as never);
      const { result } = renderHook(() => useRGDInstances("web", "ns"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.listRGDInstances).toHaveBeenCalledWith("web", "ns");
    });

    it("does not fetch with empty rgdName", async () => {
      const { result } = renderHook(() => useRGDInstances(""), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
    });
  });

  describe("useK8sResources", () => {
    it("does not fetch when explicitly disabled", async () => {
      const { result } = renderHook(
        () => useK8sResources("v1", "ConfigMap", "ns", false),
        { wrapper: createWrapper() },
      );
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(k8sApi.listK8sResources).not.toHaveBeenCalled();
    });

    it("does not fetch when apiVersion or kind is empty", async () => {
      const { result } = renderHook(() => useK8sResources("", "ConfigMap"), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
    });

    it("fetches when enabled with apiVersion and kind", async () => {
      vi.mocked(k8sApi.listK8sResources).mockResolvedValue({ items: [] } as never);
      const { result } = renderHook(
        () => useK8sResources("v1", "ConfigMap", "ns"),
        { wrapper: createWrapper() },
      );
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(k8sApi.listK8sResources).toHaveBeenCalledWith("v1", "ConfigMap", "ns");
    });

    it("does not retry on 403 (forbidden) errors", async () => {
      vi.mocked(k8sApi.listK8sResources).mockRejectedValue(make403());
      // Use the hook's own retry predicate (pass a real retry fn through).
      const { result } = renderHook(
        () => useK8sResources("v1", "ConfigMap", "ns"),
        { wrapper: createWrapper(() => false) },
      );
      await waitFor(() => expect(result.current.isError).toBe(true), {
        timeout: 5000,
      });
      // Single attempt — no retries for a 403.
      expect(k8sApi.listK8sResources).toHaveBeenCalledTimes(1);
    });
  });

  describe("useRGDRevisions", () => {
    it("fetches revision history", async () => {
      vi.mocked(rgdApi.getRGDRevisions).mockResolvedValue({ items: [] } as never);
      const { result } = renderHook(() => useRGDRevisions("web"), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDRevisions).toHaveBeenCalledWith("web");
    });

    it("does not fetch with empty rgdName", async () => {
      const { result } = renderHook(() => useRGDRevisions(""), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
    });
  });

  describe("useRGDRevision", () => {
    it("does not fetch when revision is null", async () => {
      const { result } = renderHook(() => useRGDRevision("web", null), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(rgdApi.getRGDRevision).not.toHaveBeenCalled();
    });

    it("fetches a specific immutable revision", async () => {
      vi.mocked(rgdApi.getRGDRevision).mockResolvedValue({} as never);
      const { result } = renderHook(() => useRGDRevision("web", 3), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDRevision).toHaveBeenCalledWith("web", 3);
    });
  });

  describe("useRGDRevisionDiff", () => {
    it("does not fetch when either revision is null", async () => {
      const { result } = renderHook(() => useRGDRevisionDiff("web", 1, null), {
        wrapper: createWrapper(),
      });
      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(rgdApi.getRGDRevisionDiff).not.toHaveBeenCalled();
    });

    it("fetches the diff when both revisions are present", async () => {
      vi.mocked(rgdApi.getRGDRevisionDiff).mockResolvedValue({} as never);
      const { result } = renderHook(() => useRGDRevisionDiff("web", 1, 2), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(rgdApi.getRGDRevisionDiff).toHaveBeenCalledWith("web", 1, 2);
    });
  });
});
