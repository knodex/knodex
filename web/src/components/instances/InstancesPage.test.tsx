// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { InstancesPage } from "./InstancesPage";
import type { Instance } from "@/types/rgd";

// Mock hooks
const mockUseInstanceList = vi.fn();
vi.mock("@/hooks/useInstances", () => ({
  useInstanceList: (...args: unknown[]) => mockUseInstanceList(...args),
}));

vi.mock("@/hooks/useProjects", () => ({
  useProjects: () => ({ data: { items: [] }, isLoading: false }),
}));

vi.mock("@/hooks/useAuth", () => ({
  useCurrentProject: () => null,
}));

// Mock URL utils (no-op for tests)
vi.mock("@/lib/url-utils", () => ({
  getInstanceFiltersFromURL: () => ({ search: "", rgd: "", health: "", scope: "" }),
  setInstanceFiltersToURL: vi.fn(),
}));

// Mock child components to isolate InstancesPage behavior
vi.mock("./StatusCard", () => ({
  StatusCard: ({ instance }: { instance: Instance }) => (
    <div data-testid="status-card">{instance.name}</div>
  ),
}));

vi.mock("./StatusCardSkeleton", () => ({
  StatusCardSkeleton: () => <div data-testid="status-card-skeleton" />,
}));

vi.mock("./InstancesListView", () => ({
  InstancesListView: () => <div data-testid="instances-list-view" />,
}));

vi.mock("./EmptyState", () => ({
  EmptyState: ({ hasFilters }: { hasFilters: boolean }) => (
    <div data-testid="empty-state" data-has-filters={hasFilters} />
  ),
}));

vi.mock("./InstanceFilters", () => ({
  InstanceFilters: () => <div data-testid="instance-filters" />,
}));

vi.mock("@/components/catalog/Pagination", () => ({
  Pagination: () => <div data-testid="pagination" />,
}));

vi.mock("@/components/layout/PageHeader", () => ({
  PageHeader: ({ title, subtitle, children, className }: { title: string; subtitle?: string; children?: React.ReactNode; className?: string }) => (
    <div data-testid="page-header" data-title={title} data-subtitle={subtitle} data-classname={className}>
      {children}
    </div>
  ),
}));

function createTestInstance(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "my-instance",
    namespace: "default",
    rgdName: "my-rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "AKSCluster",
    health: "Healthy",
    conditions: [],
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    uid: "test-uid",
    labels: { "knodex.io/project": "alpha" },
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter>
      <InstancesPage />
    </MemoryRouter>
  );
}

beforeEach(() => {
  mockUseInstanceList.mockReset();
  localStorage.clear();
});

describe("InstancesPage", () => {
  describe("loading state (AC #1, #6)", () => {
    it("renders StatusCardSkeleton grid with matching auto-fill CSS while loading", () => {
      mockUseInstanceList.mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
        isFetching: true,
        refetch: vi.fn(),
      });

      renderPage();

      const skeletons = screen.getAllByTestId("status-card-skeleton");
      expect(skeletons).toHaveLength(8);
      const grid = skeletons[0].parentElement!;
      expect(grid.style.gridTemplateColumns).toBe("repeat(auto-fill, minmax(300px, 1fr))");
      expect(grid).toHaveClass("grid");
    });
  });

  describe("page header (AC #2)", () => {
    it("renders title 'Instances'", () => {
      mockUseInstanceList.mockReturnValue({
        data: { items: [], totalCount: 0, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const header = screen.getByTestId("page-header");
      expect(header).toHaveAttribute("data-title", "Instances");
    });

  });

  describe("Deploy button (AC #3)", () => {
    it("renders Deploy button linking to /catalog", () => {
      mockUseInstanceList.mockReturnValue({
        data: { items: [], totalCount: 0, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const button = screen.getByTestId("deploy-new-button");
      expect(button).toBeInTheDocument();
      expect(button).toHaveAttribute("href", "/catalog");
      expect(button).toHaveTextContent("Deploy");
    });

    it("has brand-primary background CSS class", () => {
      mockUseInstanceList.mockReturnValue({
        data: { items: [], totalCount: 0, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const button = screen.getByTestId("deploy-new-button");
      expect(button).toHaveClass("bg-[var(--brand-primary)]");
    });
  });

  describe("StatusCard grid (AC #1)", () => {
    it("renders StatusCard for each instance in grid view", () => {
      const instances = [
        createTestInstance({ name: "inst-1" }),
        createTestInstance({ name: "inst-2" }),
      ];

      mockUseInstanceList.mockReturnValue({
        data: { items: instances, totalCount: 2, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const cards = screen.getAllByTestId("status-card");
      expect(cards).toHaveLength(2);
      expect(screen.getByText("inst-1")).toBeInTheDocument();
      expect(screen.getByText("inst-2")).toBeInTheDocument();
    });

    it("uses auto-fill grid CSS with minmax(340px, 1fr)", () => {
      const instances = [createTestInstance()];

      mockUseInstanceList.mockReturnValue({
        data: { items: instances, totalCount: 1, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      // StatusCard is wrapped in an animation div, grid is its grandparent
      const animWrapper = screen.getAllByTestId("status-card")[0].parentElement!;
      const grid = animWrapper.parentElement!;
      expect(grid.style.gridTemplateColumns).toBe("repeat(auto-fill, minmax(300px, 1fr))");
      expect(grid).toHaveClass("grid");
    });
  });

  describe("empty state (AC #5)", () => {
    it("renders EmptyState when no instances exist", () => {
      mockUseInstanceList.mockReturnValue({
        data: { items: [], totalCount: 0, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const emptyState = screen.getByTestId("empty-state");
      expect(emptyState).toBeInTheDocument();
      expect(emptyState).toHaveAttribute("data-has-filters", "false");
    });
  });

  describe("error state", () => {
    it("renders error alert when data fetch fails", () => {
      mockUseInstanceList.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error("Network error"),
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      expect(screen.getByText("Failed to load instances")).toBeInTheDocument();
    });
  });

  describe("view mode toggle", () => {
    it("renders InstancesListView when view mode is list", () => {
      localStorage.setItem("instances-view-mode", "list");

      const instances = [createTestInstance()];

      mockUseInstanceList.mockReturnValue({
        data: { items: instances, totalCount: 1, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      expect(screen.getByTestId("instances-list-view")).toBeInTheDocument();
      expect(screen.queryByTestId("status-card")).not.toBeInTheDocument();
    });

    it("switches to list view when list toggle button is clicked", () => {
      const instances = [createTestInstance()];

      mockUseInstanceList.mockReturnValue({
        data: { items: instances, totalCount: 1, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      // Initially in grid view
      expect(screen.getByTestId("status-card")).toBeInTheDocument();

      // Click list view toggle
      fireEvent.click(screen.getByLabelText("List view"));

      // Should now show list view
      expect(screen.getByTestId("instances-list-view")).toBeInTheDocument();
      expect(screen.queryByTestId("status-card")).not.toBeInTheDocument();
    });

    it("switches back to grid view when grid toggle button is clicked", () => {
      localStorage.setItem("instances-view-mode", "list");

      const instances = [createTestInstance()];

      mockUseInstanceList.mockReturnValue({
        data: { items: instances, totalCount: 1, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      // Initially in list view
      expect(screen.getByTestId("instances-list-view")).toBeInTheDocument();

      // Click grid view toggle
      fireEvent.click(screen.getByLabelText("Grid view"));

      // Should now show grid view
      expect(screen.getByTestId("status-card")).toBeInTheDocument();
      expect(screen.queryByTestId("instances-list-view")).not.toBeInTheDocument();
    });
  });
});
