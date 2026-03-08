// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useNamespaces, useProjectNamespaces } from "./useNamespaces";
import * as namespacesApi from "@/api/namespaces";
import type { ReactNode } from "react";

// Mock the namespaces API
vi.mock("@/api/namespaces");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("Namespace hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useNamespaces", () => {
    it("should fetch all namespaces excluding system namespaces by default", async () => {
      vi.mocked(namespacesApi.listNamespaces).mockResolvedValue({
        namespaces: ["default", "dev-team1", "staging", "production"],
        count: 4,
      });

      const { result } = renderHook(() => useNamespaces(), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual({
        namespaces: ["default", "dev-team1", "staging", "production"],
        count: 4,
      });
      expect(namespacesApi.listNamespaces).toHaveBeenCalledWith(true);
    });

    it("should fetch all namespaces including system when excludeSystem is false", async () => {
      vi.mocked(namespacesApi.listNamespaces).mockResolvedValue({
        namespaces: ["default", "kube-system", "kube-public", "dev-team1"],
        count: 4,
      });

      const { result } = renderHook(() => useNamespaces(false), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(namespacesApi.listNamespaces).toHaveBeenCalledWith(false);
    });

    it("should handle errors", async () => {
      const error = new Error("Failed to fetch namespaces");
      vi.mocked(namespacesApi.listNamespaces).mockRejectedValue(error);

      const { result } = renderHook(() => useNamespaces(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });

    it("should have correct query key for cache management", async () => {
      vi.mocked(namespacesApi.listNamespaces).mockResolvedValue({
        namespaces: ["default"],
        count: 1,
      });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useNamespaces(true), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify the query is cached with the correct key
      const cachedData = queryClient.getQueryData(["namespaces", { excludeSystem: true }]);
      expect(cachedData).toEqual({ namespaces: ["default"], count: 1 });
    });
  });

  describe("useProjectNamespaces", () => {
    it("should fetch namespaces for a project", async () => {
      vi.mocked(namespacesApi.getProjectNamespaces).mockResolvedValue({
        namespaces: ["dev-team1", "dev-team2", "staging"],
        count: 3,
      });

      const { result } = renderHook(() => useProjectNamespaces("alpha"), {
        wrapper: createWrapper(),
      });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data).toEqual({
        namespaces: ["dev-team1", "dev-team2", "staging"],
        count: 3,
      });
      expect(namespacesApi.getProjectNamespaces).toHaveBeenCalledWith("alpha");
    });

    it("should not fetch when projectName is empty", async () => {
      const { result } = renderHook(() => useProjectNamespaces(""), {
        wrapper: createWrapper(),
      });

      // Wait a bit to ensure the query doesn't run
      await new Promise(resolve => setTimeout(resolve, 100));

      expect(result.current.isLoading).toBe(false);
      expect(result.current.fetchStatus).toBe("idle");
      expect(namespacesApi.getProjectNamespaces).not.toHaveBeenCalled();
    });

    it("should return empty namespaces for project with no matching namespaces", async () => {
      vi.mocked(namespacesApi.getProjectNamespaces).mockResolvedValue({
        namespaces: [],
        count: 0,
      });

      const { result } = renderHook(() => useProjectNamespaces("empty-project"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(result.current.data?.namespaces).toEqual([]);
      expect(result.current.data?.count).toBe(0);
    });

    it("should handle errors", async () => {
      const error = new Error("Project not found");
      vi.mocked(namespacesApi.getProjectNamespaces).mockRejectedValue(error);

      const { result } = renderHook(() => useProjectNamespaces("nonexistent"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toBe(error);
    });

    it("should have correct query key for cache management", async () => {
      vi.mocked(namespacesApi.getProjectNamespaces).mockResolvedValue({
        namespaces: ["production"],
        count: 1,
      });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result } = renderHook(() => useProjectNamespaces("prod-project"), { wrapper });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      // Verify the query is cached with the correct key
      const cachedData = queryClient.getQueryData(["projectNamespaces", "prod-project"]);
      expect(cachedData).toEqual({ namespaces: ["production"], count: 1 });
    });

    it("should refetch when project name changes", async () => {
      vi.mocked(namespacesApi.getProjectNamespaces)
        .mockResolvedValueOnce({ namespaces: ["dev-ns"], count: 1 })
        .mockResolvedValueOnce({ namespaces: ["prod-ns"], count: 1 });

      const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      );

      const { result, rerender } = renderHook(
        ({ projectName }) => useProjectNamespaces(projectName),
        {
          wrapper,
          initialProps: { projectName: "dev-project" },
        }
      );

      await waitFor(() => {
        expect(result.current.data?.namespaces).toEqual(["dev-ns"]);
      });

      // Change project name
      rerender({ projectName: "prod-project" });

      await waitFor(() => {
        expect(result.current.data?.namespaces).toEqual(["prod-ns"]);
      });

      expect(namespacesApi.getProjectNamespaces).toHaveBeenCalledTimes(2);
      expect(namespacesApi.getProjectNamespaces).toHaveBeenNthCalledWith(1, "dev-project");
      expect(namespacesApi.getProjectNamespaces).toHaveBeenNthCalledWith(2, "prod-project");
    });
  });
});
