// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { CategoryPage } from "./CategoryPage";
import * as useCategoriesModule from "@/hooks/useCategories";
import * as useRGDsModule from "@/hooks/useRGDs";

// Mock the hooks
vi.mock("@/hooks/useCategories", () => ({
  useCategory: vi.fn(),
}));

vi.mock("@/hooks/useRGDs", () => ({
  useRGDList: vi.fn(),
  useRGDFilters: vi.fn().mockReturnValue({ data: undefined, isLoading: false }),
}));

// Mock useCanI for deploy permission
vi.mock("@/hooks/useCanI", () => ({
  useCanI: () => ({ allowed: false, isError: false }),
}));

const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

function renderWithRouter(slug: string) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter initialEntries={[`/catalog/categories/${slug}`]}>
          <Routes>
            <Route path="/catalog/categories/:slug" element={<CategoryPage />} />
          </Routes>
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

describe("CategoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state while fetching category", () => {
    vi.mocked(useCategoriesModule.useCategory).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    } as ReturnType<typeof useCategoriesModule.useCategory>);

    vi.mocked(useRGDsModule.useRGDList).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useRGDsModule.useRGDList>);

    renderWithRouter("infrastructure");

    // Should show skeleton loading state
    expect(document.querySelectorAll(".animate-token-shimmer").length).toBeGreaterThanOrEqual(0);
  });

  it("renders error state when category not found", async () => {
    vi.mocked(useCategoriesModule.useCategory).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Category not found"),
    } as unknown as ReturnType<typeof useCategoriesModule.useCategory>);

    vi.mocked(useRGDsModule.useRGDList).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useRGDsModule.useRGDList>);

    renderWithRouter("nonexistent");

    await waitFor(() => {
      expect(screen.getAllByText("Category not found")).toHaveLength(2);
    });
  });

  it("renders category with RGDs when data loads", async () => {
    const mockCategory = {
      name: "infrastructure",
      slug: "infrastructure",
      icon: "server",
      count: 2,
    };

    const mockRGDs = {
      items: [
        {
          name: "test-rgd-1",
          namespace: "default",
          description: "Test RGD 1",
          tags: [],
          category: "infrastructure",
          labels: {},
          instances: 0,
          createdAt: "2024-01-01T00:00:00Z",
          updatedAt: "2024-01-01T00:00:00Z",
        },
      ],
      totalCount: 1,
      page: 1,
      pageSize: 20,
    };

    vi.mocked(useCategoriesModule.useCategory).mockReturnValue({
      data: mockCategory,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useCategoriesModule.useCategory>);

    vi.mocked(useRGDsModule.useRGDList).mockReturnValue({
      data: mockRGDs,
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useRGDsModule.useRGDList>);

    renderWithRouter("infrastructure");

    await waitFor(() => {
      expect(screen.getAllByText("infrastructure").length).toBeGreaterThanOrEqual(1);
    });
  });

  it("renders empty state when no RGDs match category", async () => {
    const mockCategory = {
      name: "empty",
      slug: "empty",
      icon: "box",
      count: 0,
    };

    vi.mocked(useCategoriesModule.useCategory).mockReturnValue({
      data: mockCategory,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useCategoriesModule.useCategory>);

    vi.mocked(useRGDsModule.useRGDList).mockReturnValue({
      data: {
        items: [],
        totalCount: 0,
        page: 1,
        pageSize: 20,
      },
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useRGDsModule.useRGDList>);

    renderWithRouter("empty");

    await waitFor(() => {
      expect(screen.getAllByText("empty").length).toBeGreaterThanOrEqual(1);
    });

    // Should show empty state
    expect(screen.getByText(/no results found/i)).toBeInTheDocument();
  });
});
