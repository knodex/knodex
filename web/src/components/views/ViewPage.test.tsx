// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ViewPage } from "./ViewPage";
import * as useViewsModule from "@/hooks/useViews";
import * as useRGDsModule from "@/hooks/useRGDs";

// Mock the hooks
vi.mock("@/hooks/useViews", () => ({
  useView: vi.fn(),
}));

vi.mock("@/hooks/useRGDs", () => ({
  useRGDList: vi.fn(),
}));

// Mock isEnterprise check
vi.mock("@/hooks/useCompliance", () => ({
  isEnterprise: () => true,
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
        <MemoryRouter initialEntries={[`/views/${slug}`]}>
          <Routes>
            <Route path="/views/:slug" element={<ViewPage />} />
          </Routes>
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

describe("ViewPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state while fetching view", () => {
    vi.mocked(useViewsModule.useView).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
    } as ReturnType<typeof useViewsModule.useView>);

    vi.mocked(useRGDsModule.useRGDList).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useRGDsModule.useRGDList>);

    renderWithRouter("testing");

    // Should show skeleton loading state
    expect(document.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
      0
    );
  });

  it("renders error state when view not found", async () => {
    vi.mocked(useViewsModule.useView).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("View not found"),
    } as unknown as ReturnType<typeof useViewsModule.useView>);

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
      // The component shows "View not found" in both title and description
      expect(screen.getAllByText("View not found")).toHaveLength(2);
    });
  });

  it("renders view with RGDs when data loads", async () => {
    const mockView = {
      name: "Testing Resources",
      slug: "testing",
      icon: "flask",
      category: "testing",
      order: 1,
      description: "RGDs for testing",
      count: 2,
    };

    const mockRGDs = {
      items: [
        {
          name: "test-rgd-1",
          namespace: "default",
          description: "Test RGD 1",
          version: "v1",
          tags: [],
          category: "testing",
          labels: {},
          instances: 0,
          createdAt: "2024-01-01T00:00:00Z",
          updatedAt: "2024-01-01T00:00:00Z",
        },
        {
          name: "test-rgd-2",
          namespace: "default",
          description: "Test RGD 2",
          version: "v1",
          tags: [],
          category: "testing",
          labels: {},
          instances: 3,
          createdAt: "2024-01-01T00:00:00Z",
          updatedAt: "2024-01-01T00:00:00Z",
        },
      ],
      totalCount: 2,
      page: 1,
      pageSize: 20,
    };

    vi.mocked(useViewsModule.useView).mockReturnValue({
      data: mockView,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useViewsModule.useView>);

    vi.mocked(useRGDsModule.useRGDList).mockReturnValue({
      data: mockRGDs,
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useRGDsModule.useRGDList>);

    renderWithRouter("testing");

    await waitFor(() => {
      expect(screen.getByText("Testing Resources")).toBeInTheDocument();
    });

    // Check description is shown
    expect(screen.getByText("RGDs for testing")).toBeInTheDocument();

    // Check category filter badge - uses getAllByText since category appears multiple times
    expect(screen.getAllByText("testing").length).toBeGreaterThan(0);
  });

  it("renders empty state when no RGDs match category", async () => {
    const mockView = {
      name: "Empty View",
      slug: "empty",
      icon: "box",
      category: "empty",
      order: 1,
      count: 0,
    };

    vi.mocked(useViewsModule.useView).mockReturnValue({
      data: mockView,
      isLoading: false,
      isError: false,
      error: null,
    } as unknown as ReturnType<typeof useViewsModule.useView>);

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
      expect(screen.getByText("Empty View")).toBeInTheDocument();
    });

    // Should show empty state (EmptyState component shows "No results found")
    expect(screen.getByText(/no results found/i)).toBeInTheDocument();
  });
});
