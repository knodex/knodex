// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useRepositories,
  useRepository,
  useCreateRepository,
  useUpdateRepository,
  useDeleteRepository,
} from "./useRepositories";
import * as repoApi from "@/api/repository";
import type { ReactNode } from "react";

vi.mock("@/api/repository");

function createClientAndWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const removeSpy = vi.spyOn(queryClient, "removeQueries");
  const setDataSpy = vi.spyOn(queryClient, "setQueryData");
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }
  return { queryClient, invalidateSpy, removeSpy, setDataSpy, Wrapper };
}

describe("useRepositories hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useRepositories", () => {
    it("lists all repositories with no project filter", async () => {
      vi.mocked(repoApi.listRepositories).mockResolvedValue({
        repositories: [],
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useRepositories(), { wrapper: Wrapper });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(repoApi.listRepositories).toHaveBeenCalledWith(undefined);
    });

    it("forwards the projectId filter", async () => {
      vi.mocked(repoApi.listRepositories).mockResolvedValue({
        repositories: [],
      } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useRepositories("alpha"), {
        wrapper: Wrapper,
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(repoApi.listRepositories).toHaveBeenCalledWith("alpha");
    });
  });

  describe("useRepository", () => {
    it("does not fetch when id is empty", async () => {
      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useRepository(""), { wrapper: Wrapper });

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(repoApi.getRepository).not.toHaveBeenCalled();
    });

    it("fetches a single repository by id", async () => {
      vi.mocked(repoApi.getRepository).mockResolvedValue({ id: "r1" } as never);

      const { Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useRepository("r1"), {
        wrapper: Wrapper,
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(repoApi.getRepository).toHaveBeenCalledWith("r1");
    });
  });

  describe("useCreateRepository", () => {
    it("invalidates the repositories list on success", async () => {
      vi.mocked(repoApi.createRepository).mockResolvedValue({ id: "r1" } as never);

      const { invalidateSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useCreateRepository(), {
        wrapper: Wrapper,
      });

      result.current.mutate({ url: "https://example.com" } as never);
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(repoApi.createRepository).toHaveBeenCalledWith({
        url: "https://example.com",
      });
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["repositories"]);
    });
  });

  describe("useUpdateRepository", () => {
    it("updates the cache and invalidates the list", async () => {
      vi.mocked(repoApi.updateRepository).mockResolvedValue({ id: "r1" } as never);

      const { invalidateSpy, setDataSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useUpdateRepository(), {
        wrapper: Wrapper,
      });

      result.current.mutate({ id: "r1", request: { url: "u" } as never });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(repoApi.updateRepository).toHaveBeenCalledWith("r1", { url: "u" });
      expect(setDataSpy).toHaveBeenCalledWith(["repository", "r1"], { id: "r1" });
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["repositories"]);
    });
  });

  describe("useDeleteRepository", () => {
    it("removes the cached repository and invalidates the list", async () => {
      vi.mocked(repoApi.deleteRepository).mockResolvedValue(undefined);

      const { invalidateSpy, removeSpy, Wrapper } = createClientAndWrapper();
      const { result } = renderHook(() => useDeleteRepository(), {
        wrapper: Wrapper,
      });

      result.current.mutate("r1");
      await waitFor(() => expect(result.current.isSuccess).toBe(true));

      expect(repoApi.deleteRepository).toHaveBeenCalledWith("r1");
      expect(removeSpy).toHaveBeenCalledWith({ queryKey: ["repository", "r1"] });
      const keys = invalidateSpy.mock.calls.map(([f]) => f?.queryKey);
      expect(keys).toContainEqual(["repositories"]);
    });
  });
});
