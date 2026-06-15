// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useCategories, useCategory, useCategoriesEnabled } from "./useCategories";
import * as categoriesApi from "@/api/categories";
import type { ReactNode } from "react";

vi.mock("@/api/categories");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

describe("useCategories hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("useCategories", () => {
    it("returns the category list via select", async () => {
      vi.mocked(categoriesApi.listCategories).mockResolvedValue({
        categories: [
          { slug: "web", name: "Web", count: 2 },
          { slug: "data", name: "Data", count: 1 },
        ],
      } as never);

      const { result } = renderHook(() => useCategories(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toHaveLength(2);
      expect(categoriesApi.listCategories).toHaveBeenCalledTimes(1);
    });

    it("returns an empty array when categories is missing (graceful degradation)", async () => {
      vi.mocked(categoriesApi.listCategories).mockResolvedValue({} as never);

      const { result } = renderHook(() => useCategories(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toEqual([]);
    });
  });

  describe("useCategory", () => {
    it("does not fetch when slug is undefined", async () => {
      const { result } = renderHook(() => useCategory(undefined), {
        wrapper: createWrapper(),
      });

      await new Promise((r) => setTimeout(r, 50));
      expect(result.current.fetchStatus).toBe("idle");
      expect(categoriesApi.getCategory).not.toHaveBeenCalled();
    });

    it("fetches a category when a slug is provided", async () => {
      vi.mocked(categoriesApi.getCategory).mockResolvedValue({
        slug: "web",
        name: "Web",
        count: 2,
      } as never);

      const { result } = renderHook(() => useCategory("web"), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(categoriesApi.getCategory).toHaveBeenCalledWith("web");
    });
  });

  describe("useCategoriesEnabled", () => {
    it("is enabled when at least one category is visible", async () => {
      vi.mocked(categoriesApi.listCategories).mockResolvedValue({
        categories: [{ slug: "web", name: "Web", count: 1 }],
      } as never);

      const { result } = renderHook(() => useCategoriesEnabled(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.enabled).toBe(true);
      expect(result.current.categories).toHaveLength(1);
    });

    it("is disabled when no categories are visible", async () => {
      vi.mocked(categoriesApi.listCategories).mockResolvedValue({
        categories: [],
      } as never);

      const { result } = renderHook(() => useCategoriesEnabled(), {
        wrapper: createWrapper(),
      });

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.enabled).toBe(false);
    });
  });
});
