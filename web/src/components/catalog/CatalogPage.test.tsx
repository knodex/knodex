// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { CatalogPage } from "./CatalogPage";
import * as useRGDsModule from "@/hooks/useRGDs";
import * as useCanIModule from "@/hooks/useCanI";
import * as useAuthModule from "@/hooks/useAuth";
import type { CatalogRGD, RGDListParams } from "@/types/rgd";

vi.mock("@/hooks/useRGDs", () => ({
  useRGDList: vi.fn(),
  useRGDFilters: vi.fn(),
}));

vi.mock("@/hooks/useCanI", () => ({
  useCanI: vi.fn(),
}));

vi.mock("@/hooks/useAuth", () => ({
  useCurrentProject: vi.fn(() => null),
}));

vi.mock("@/stores/preferencesStore", () => ({
  usePreferencesStore: () => ({
    recentRgds: [],
    hydrate: vi.fn(),
    addRecent: vi.fn(),
  }),
}));

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

function createRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "desc",
    tags: [],
    category: "database",
    labels: {},
    instances: 2,
    status: "Active",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

type UseRGDFiltersReturn = ReturnType<typeof useRGDsModule.useRGDFilters>;

function mockRGDList(items: CatalogRGD[], totalCount = items.length): void {
  vi.mocked(useRGDsModule.useRGDList).mockImplementation(
    ((params?: RGDListParams) =>
      ({
        data: params === undefined ? undefined : { items, page: 1, pageSize: 20, totalCount },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      })) as unknown as typeof useRGDsModule.useRGDList
  );
}

function mockRGDFilters(categories: string[], tags: string[] = []): void {
  vi.mocked(useRGDsModule.useRGDFilters).mockReturnValue({
    data: { categories, tags, projects: [] },
    isLoading: false,
    isError: false,
  } as unknown as UseRGDFiltersReturn);
}

function renderPage(initialRoute = "/catalog") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialRoute]}>
        <CatalogPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("CatalogPage — Story 48.2", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAuthModule.useCurrentProject).mockReturnValue(null);
    mockRGDFilters(["database", "networking"]);
  });

  describe("Deploy CTA (AC #1)", () => {
    it("renders Deploy button when user has instances:create permission", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: true,
        isLoading: false,
        isError: false,
      });
      mockRGDList([createRGD()]);

      renderPage();

      const btn = screen.getByTestId("catalog-deploy-button");
      expect(btn).toBeInTheDocument();
      expect(btn).toHaveAttribute("aria-label", "Deploy a resource");
      expect(btn).toHaveTextContent("Deploy");
    });

    it("hides Deploy button when user lacks instances:create permission", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
        isError: false,
      });
      mockRGDList([createRGD()]);

      renderPage();

      expect(screen.queryByTestId("catalog-deploy-button")).not.toBeInTheDocument();
    });

    it("hides Deploy button while permission check is loading", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: undefined,
        isLoading: true,
        isError: false,
      });
      mockRGDList([createRGD()]);

      renderPage();

      expect(screen.queryByTestId("catalog-deploy-button")).not.toBeInTheDocument();
    });

    it("navigates to /instances?action=deploy when Deploy is clicked", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: true,
        isLoading: false,
        isError: false,
      });
      mockRGDList([createRGD()]);

      renderPage();

      fireEvent.click(screen.getByTestId("catalog-deploy-button"));
      expect(mockNavigate).toHaveBeenCalledWith("/instances?action=deploy");
    });
  });

  describe("ListFooter (AC #3)", () => {
    beforeEach(() => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: true,
        isLoading: false,
        isError: false,
      });
    });

    it("renders ListFooter with total, categories, and live-instance counts", () => {
      mockRGDList(
        [
          createRGD({ name: "rgd-a", instances: 3 }),
          createRGD({ name: "rgd-b", instances: 7 }),
        ],
        2
      );

      renderPage();

      const footer = screen.getByTestId("catalog-list-footer");
      expect(footer).toBeInTheDocument();
      // total 2 resources
      expect(footer).toHaveTextContent("2 resources");
      // 2 categories (from mockRGDFilters in outer beforeEach)
      expect(footer).toHaveTextContent("2 categories");
      // live instances = 3 + 7
      expect(footer).toHaveTextContent("10 live instances");
    });

    it("does NOT render ListFooter when result set is empty", () => {
      mockRGDList([], 0);

      renderPage();

      expect(screen.queryByTestId("catalog-list-footer")).not.toBeInTheDocument();
    });
  });
});
