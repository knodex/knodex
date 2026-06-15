// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { InstancesPage } from "./InstancesPage";
import type { Instance } from "@/types/rgd";
import * as useCanIModule from "@/hooks/useCanI";

// Mock hooks
const mockUseInstanceList = vi.fn();
vi.mock("@/hooks/useInstances", () => ({
  useInstanceList: (...args: unknown[]) => mockUseInstanceList(...args),
}));

vi.mock("@/hooks/useCanI", () => ({
  useCanI: vi.fn(),
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

vi.mock("./InstancesListSkeleton", () => ({
  InstancesListSkeleton: () => <div data-testid="instances-list-skeleton" />,
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
  // Default: user can deploy (gate is exercised explicitly in the Deploy CTA suite).
  vi.mocked(useCanIModule.useCanI).mockReturnValue({
    allowed: true,
    isLoading: false,
    isError: false,
  });
});

describe("InstancesPage", () => {
  describe("loading state", () => {
    it("renders the list skeleton while loading in default (table) view", () => {
      mockUseInstanceList.mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
        isFetching: true,
        refetch: vi.fn(),
      });

      renderPage();

      expect(screen.getByTestId("instances-list-skeleton")).toBeInTheDocument();
    });

    it("renders card skeletons when grid view is selected", () => {
      localStorage.setItem("knodex.instances.view", "grid");
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
    });
  });

  describe("Deploy CTA (AC #1 — Casbin-gated)", () => {
    function withEmptyList() {
      mockUseInstanceList.mockReturnValue({
        data: { items: [], totalCount: 0, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });
    }

    it("renders Deploy CTA linking to /catalog when user has instances:create permission", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: true,
        isLoading: false,
        isError: false,
      });
      withEmptyList();

      renderPage();

      const button = screen.getByTestId("deploy-new-button");
      expect(button).toBeInTheDocument();
      expect(button).toHaveAttribute("href", "/catalog");
      expect(button).toHaveAttribute("aria-label", "Deploy a resource");
      expect(button).toHaveTextContent("Deploy");
    });

    it("has brand-primary background CSS class", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: true,
        isLoading: false,
        isError: false,
      });
      withEmptyList();

      renderPage();

      const button = screen.getByTestId("deploy-new-button");
      expect(button).toHaveClass("bg-[var(--brand-primary)]");
    });

    it("hides the Deploy CTA when the user lacks instances:create permission", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: false,
        isLoading: false,
        isError: false,
      });
      withEmptyList();

      renderPage();

      expect(screen.queryByTestId("deploy-new-button")).not.toBeInTheDocument();
    });

    it("hides the Deploy CTA while the permission check is loading", () => {
      vi.mocked(useCanIModule.useCanI).mockReturnValue({
        allowed: undefined,
        isLoading: true,
        isError: false,
      });
      withEmptyList();

      renderPage();

      expect(screen.queryByTestId("deploy-new-button")).not.toBeInTheDocument();
    });
  });

  describe("default view (AC #1)", () => {
    it("renders the table (list) view by default on first visit", () => {
      const instances = [createTestInstance({ name: "inst-1" })];

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
  });

  describe("empty state", () => {
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

  describe("view mode toggle + persistence (AC #2)", () => {
    function withInstance() {
      mockUseInstanceList.mockReturnValue({
        data: { items: [createTestInstance()], totalCount: 1, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });
    }

    it("respects 'grid' from the new localStorage key", () => {
      localStorage.setItem("knodex.instances.view", "grid");
      withInstance();
      renderPage();
      expect(screen.getByTestId("status-card")).toBeInTheDocument();
      expect(screen.queryByTestId("instances-list-view")).not.toBeInTheDocument();
    });

    it("migrates the legacy localStorage key on read", () => {
      localStorage.setItem("instances-view-mode", "grid");
      withInstance();
      renderPage();
      // Legacy preference honored
      expect(screen.getByTestId("status-card")).toBeInTheDocument();
      // Legacy key cleared, new key written
      expect(localStorage.getItem("instances-view-mode")).toBeNull();
      expect(localStorage.getItem("knodex.instances.view")).toBe("grid");
    });

    it("switches to grid view when grid toggle is clicked and persists the choice", () => {
      withInstance();
      renderPage();

      // Default is table
      expect(screen.getByTestId("instances-list-view")).toBeInTheDocument();

      fireEvent.click(screen.getByLabelText("Grid view"));

      expect(screen.getByTestId("status-card")).toBeInTheDocument();
      expect(localStorage.getItem("knodex.instances.view")).toBe("grid");
    });

    it("switches back to table view when table toggle is clicked", () => {
      localStorage.setItem("knodex.instances.view", "grid");
      withInstance();
      renderPage();

      expect(screen.getByTestId("status-card")).toBeInTheDocument();

      fireEvent.click(screen.getByLabelText("Table view"));

      expect(screen.getByTestId("instances-list-view")).toBeInTheDocument();
      expect(localStorage.getItem("knodex.instances.view")).toBe("list");
    });
  });

  describe("list footer (AC #6 — 48.1 ListFooter, degraded/errored merged bucket)", () => {
    it("renders the breakdown in table view, merging Degraded + Unhealthy", () => {
      const items = [
        createTestInstance({ name: "a", health: "Healthy" }),
        createTestInstance({ name: "b", health: "Healthy" }),
        createTestInstance({ name: "c", health: "Degraded" }),
        createTestInstance({ name: "d", health: "Unhealthy" }),
        createTestInstance({ name: "e", health: "Progressing" }),
      ];
      mockUseInstanceList.mockReturnValue({
        data: { items, totalCount: 5, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const footer = screen.getByTestId("instances-list-footer");
      expect(footer).toHaveTextContent("5 total");
      expect(footer).toHaveTextContent("2 healthy");
      expect(footer).toHaveTextContent("1 progressing");
      // Degraded(1) + Unhealthy(1) merge into a single "degraded / errored" bucket.
      expect(footer).toHaveTextContent("2 degraded / errored");
    });

    it("does not surface the Unknown bucket; total still reflects every visible row", () => {
      const items = [
        // @ts-expect-error -- simulating a future API value not in the InstanceHealth enum.
        createTestInstance({ name: "future", health: "Suspended" }),
        createTestInstance({ name: "ok", health: "Healthy" }),
      ];
      mockUseInstanceList.mockReturnValue({
        data: { items, totalCount: 2, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      const footer = screen.getByTestId("instances-list-footer");
      expect(footer).toHaveTextContent("2 total");
      expect(footer).toHaveTextContent("1 healthy");
      // Unknown is intentionally not shown (transient ingestion gap, not actionable).
      expect(footer).not.toHaveTextContent("unknown");
    });

    it("renders the footer in grid view too (parity with the list view)", () => {
      localStorage.setItem("knodex.instances.view", "grid");
      mockUseInstanceList.mockReturnValue({
        data: { items: [createTestInstance()], totalCount: 1, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      expect(screen.getByTestId("instances-list-footer")).toBeInTheDocument();
    });

    it("does not render the footer in the empty state", () => {
      mockUseInstanceList.mockReturnValue({
        data: { items: [], totalCount: 0, page: 1, pageSize: 20 },
        isLoading: false,
        isError: false,
        error: null,
        isFetching: false,
        refetch: vi.fn(),
      });

      renderPage();

      expect(screen.queryByTestId("instances-list-footer")).not.toBeInTheDocument();
    });
  });
});
