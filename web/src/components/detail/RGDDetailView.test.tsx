// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RGDDetailView } from "./RGDDetailView";
import type { CatalogRGD } from "@/types/rgd";

// Mock hooks
vi.mock("@/hooks/useRGDs", () => ({
  useRGD: vi.fn().mockReturnValue({ data: null }),
  useRGDList: vi.fn().mockReturnValue({
    data: { items: [], totalCount: 0, page: 1, pageSize: 1000 },
    isLoading: false,
  }),
  useRGDResourceGraph: vi.fn().mockReturnValue({
    data: null,
    isLoading: false,
    error: null,
  }),
  useRGDRevisions: vi.fn().mockReturnValue({
    data: { items: [], totalCount: 0 },
    isLoading: false,
  }),
  useRGDInstances: vi.fn().mockReturnValue({
    data: { items: [], totalCount: 0 },
    isLoading: false,
  }),
}));

// Mock useKindToRGDMap for catalog-aware depends-on count
vi.mock("@/hooks/useKindToRGDMap", () => ({
  useKindToRGDMap: vi.fn().mockReturnValue({
    kindToRGD: new Map([["AKSCluster", { name: "aks-rgd" }]]),
    isLoading: false,
  }),
}));

// Mock DependsOnTab to isolate tests
vi.mock("./DependsOnTab", () => ({
  DependsOnTab: () => <div data-testid="depends-on-tab">DependsOnTab</div>,
}));

// Mock AddOnsTab to isolate tests
vi.mock("./AddOnsTab", () => ({
  AddOnsTab: () => <div data-testid="add-ons-tab">AddOnsTab</div>,
}));

// Mock StatusCard to isolate from its dependencies
vi.mock("@/components/instances/StatusCard", () => ({
  StatusCard: ({ instance }: { instance: { name: string } }) => (
    <div data-testid="status-card">{instance.name}</div>
  ),
}));

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD",
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

function renderWithProviders(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe("RGDDetailView", () => {
  it("renders Deploy button when onDeploy is provided", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD()} onBack={vi.fn()} onDeploy={vi.fn()} />
    );
    expect(screen.getAllByText("Deploy").length).toBeGreaterThanOrEqual(1);
  });

  it("does not render Deploy button when onDeploy is undefined", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD()} onBack={vi.fn()} />
    );
    expect(screen.queryByText("Deploy")).not.toBeInTheDocument();
  });

  describe("Secrets tab visibility", () => {
    const originalEnterprise = (globalThis as Record<string, unknown>).__ENTERPRISE__;

    beforeEach(() => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
    });

    afterEach(() => {
      (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
    });

    it("shows Secrets tab when secretRefs has entries", () => {
      const rgd = createTestRGD({
        secretRefs: [
          { type: "fixed", id: "0-Secret", externalRefId: "dbSecret", name: "my-secret", namespace: "default" },
        ],
      });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.getByRole("tab", { name: /Secrets \(1\)/i })).toBeInTheDocument();
    });

    it("hides Secrets tab when secretRefs is empty", () => {
      const rgd = createTestRGD({ secretRefs: [] });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.queryByRole("tab", { name: /Secrets/i })).not.toBeInTheDocument();
    });

    it("hides Secrets tab when secretRefs is undefined", () => {
      const rgd = createTestRGD();
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.queryByRole("tab", { name: /Secrets/i })).not.toBeInTheDocument();
    });
  });

  describe("Depends On tab visibility", () => {
    it("shows Depends On tab when dependsOnKinds has entries", () => {
      const rgd = createTestRGD({ dependsOnKinds: ["AKSCluster"] });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.getByRole("tab", { name: /Depends On \(1\)/i })).toBeInTheDocument();
    });

    it("hides Depends On tab when no dependencies", () => {
      const rgd = createTestRGD({ dependsOnKinds: [] });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.queryByRole("tab", { name: /Depends On/i })).not.toBeInTheDocument();
    });
  });

  describe("Instances tab", () => {
    it("renders Instances tab by default", () => {
      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} />
      );
      expect(screen.getByRole("tab", { name: /Instances/i })).toHaveAttribute("aria-selected", "true");
    });

    it("shows loading state while instances load", async () => {
      const { useRGDInstances } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDInstances).mockReturnValue({
        data: undefined,
        isLoading: true,
      } as any);

      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} />
      );
      expect(screen.getByText("Loading instances...")).toBeInTheDocument();
    });

    it("shows empty state when no instances", async () => {
      const { useRGDInstances } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDInstances).mockReturnValue({
        data: { items: [], totalCount: 0 },
        isLoading: false,
      } as any);

      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} />
      );
      expect(screen.getByText("No instances deployed yet")).toBeInTheDocument();
    });

    it("shows Deploy button in empty state when onDeploy is provided", async () => {
      const { useRGDInstances } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDInstances).mockReturnValue({
        data: { items: [], totalCount: 0 },
        isLoading: false,
      } as any);

      const onDeploy = vi.fn();
      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} onDeploy={onDeploy} />
      );
      expect(screen.getByText("No instances deployed yet")).toBeInTheDocument();
      // Deploy button in both header and empty state
      const deployButtons = screen.getAllByText("Deploy");
      expect(deployButtons.length).toBeGreaterThanOrEqual(2);
    });

    it("renders instance cards when instances exist", async () => {
      const { useRGDInstances } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDInstances).mockReturnValue({
        data: {
          items: [
            { name: "inst-1", namespace: "default", kind: "TestResource", uid: "uid-1", health: "Healthy", conditions: [], createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(), rgdName: "test-rgd", rgdNamespace: "default", apiVersion: "v1" },
            { name: "inst-2", namespace: "default", kind: "TestResource", uid: "uid-2", health: "Healthy", conditions: [], createdAt: new Date().toISOString(), updatedAt: new Date().toISOString(), rgdName: "test-rgd", rgdNamespace: "default", apiVersion: "v1" },
          ],
          totalCount: 2,
        },
        isLoading: false,
      } as any);

      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} />
      );
      expect(screen.getAllByTestId("status-card")).toHaveLength(2);
      expect(screen.getByText("inst-1")).toBeInTheDocument();
      expect(screen.getByText("inst-2")).toBeInTheDocument();
    });
  });

  describe("initialTab prop (STORY-400)", () => {
    it("activates Revisions tab when initialTab='revisions' and revisions exist", async () => {
      const { useRGDRevisions } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDRevisions).mockReturnValue({
        data: { items: [{ revisionNumber: 1, rgdName: "test-rgd", namespace: "default", conditions: [], createdAt: new Date().toISOString() }], totalCount: 1 },
        isLoading: false,
      } as any);

      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} initialTab="revisions" />
      );

      await waitFor(() => {
        expect(screen.getByRole("tab", { name: /Revisions \(1\)/i })).toHaveAttribute("aria-selected", "true");
      });
    });

    it("stays on Instances tab when initialTab='revisions' but no revisions exist", async () => {
      // Explicitly reset to 0 to prevent pollution from sibling tests that set totalCount: 1
      const { useRGDRevisions } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDRevisions).mockReturnValue({
        data: { items: [], totalCount: 0 },
        isLoading: false,
      } as any);

      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} initialTab="revisions" />
      );

      // Revisions tab never appears — initialTab cannot activate it
      expect(screen.queryByRole("tab", { name: /Revisions/i })).not.toBeInTheDocument();
      expect(screen.getByRole("tab", { name: /Instances/i })).toHaveAttribute("aria-selected", "true");
    });

    it("stays on Instances tab when initialTab is not provided even with revisions", async () => {
      const { useRGDRevisions } = await import("@/hooks/useRGDs");
      vi.mocked(useRGDRevisions).mockReturnValue({
        data: { items: [{ revisionNumber: 1, rgdName: "test-rgd", namespace: "default", conditions: [], createdAt: new Date().toISOString() }], totalCount: 1 },
        isLoading: false,
      } as any);

      renderWithProviders(
        <RGDDetailView rgd={createTestRGD()} />
      );

      // Revisions tab appears but Instances remains selected (no initialTab)
      await waitFor(() => {
        expect(screen.getByRole("tab", { name: /Revisions \(1\)/i })).toBeInTheDocument();
      });
      expect(screen.getByRole("tab", { name: /Instances/i })).toHaveAttribute("aria-selected", "true");
    });
  });
});
