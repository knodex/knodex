// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
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
}));

vi.mock("@/hooks/useKindToRGDMap", () => ({
  useKindToRGDMap: vi.fn().mockReturnValue({ kindToRGD: new Map(), isLoading: false }),
}));

// Mock DependsOnTab to isolate Overview tab tests
vi.mock("./DependsOnTab", () => ({
  DependsOnTab: () => <div data-testid="depends-on-tab">DependsOnTab</div>,
}));

// Mock AddOnsTab to isolate tests
vi.mock("./AddOnsTab", () => ({
  AddOnsTab: () => <div data-testid="add-ons-tab">AddOnsTab</div>,
}));

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD",
    version: "v1",
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
  it("shows Inactive badge when status is not Active", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD({ status: "Inactive" })} onBack={vi.fn()} />
    );
    // Badge text + Overview Status value both show "Inactive"
    const inactiveElements = screen.getAllByText("Inactive");
    expect(inactiveElements.length).toBeGreaterThanOrEqual(1);
  });

  it("does not show Inactive badge for active RGDs", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD({ status: "Active" })} onBack={vi.fn()} />
    );
    expect(screen.queryByText("Inactive")).not.toBeInTheDocument();
  });

  it("shows Status field in Overview tab", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD({ status: "Inactive" })} onBack={vi.fn()} />
    );
    // The Overview tab shows Status as a label in the Details section
    expect(screen.getByText("Status")).toBeInTheDocument();
    // The value should show "Inactive" (in the details section, separate from the badge)
    const statusValues = screen.getAllByText("Inactive");
    // At least 2: one badge + one in Overview details
    expect(statusValues.length).toBeGreaterThanOrEqual(2);
  });

  it("renders Deploy button when onDeploy is provided", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD({ status: "Active" })} onBack={vi.fn()} onDeploy={vi.fn()} />
    );
    expect(screen.getByText("Deploy")).toBeInTheDocument();
  });

  it("does not render Deploy button when onDeploy is undefined", () => {
    renderWithProviders(
      <RGDDetailView rgd={createTestRGD({ status: "Inactive" })} onBack={vi.fn()} />
    );
    expect(screen.queryByText("Deploy")).not.toBeInTheDocument();
  });

  describe("Overview Tab - Depends On row", () => {
    it("shows Depends On row when dependsOnKinds has entries", () => {
      const rgd = createTestRGD({
        dependsOnKinds: ["AKSCluster", "KeyVault"],
      });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.getByText("Depends On")).toBeInTheDocument();
      expect(screen.getByText("AKSCluster")).toBeInTheDocument();
      expect(screen.getByText("KeyVault")).toBeInTheDocument();
    });

    it("does not show Depends On row when dependsOnKinds is empty", () => {
      const rgd = createTestRGD({ dependsOnKinds: [] });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.queryByText("Depends On")).not.toBeInTheDocument();
    });

    it("does not show Depends On row when dependsOnKinds is undefined", () => {
      const rgd = createTestRGD();
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.queryByText("Depends On")).not.toBeInTheDocument();
    });

    it("renders DependsOnKindLink as a link when parent RGD is found", async () => {
      const { useKindToRGDMap } = await import("@/hooks/useKindToRGDMap");
      const map = new Map();
      map.set("AKSCluster", createTestRGD({ name: "aks-cluster-rgd", kind: "AKSCluster" }));
      vi.mocked(useKindToRGDMap).mockReturnValue({ kindToRGD: map, isLoading: false });

      const rgd = createTestRGD({ dependsOnKinds: ["AKSCluster"] });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      const link = screen.getByRole("link", { name: "AKSCluster" });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute("href", "/catalog/aks-cluster-rgd");
    });

    it("renders DependsOnKindLink as plain text when parent RGD not found", async () => {
      const { useKindToRGDMap } = await import("@/hooks/useKindToRGDMap");
      vi.mocked(useKindToRGDMap).mockReturnValue({ kindToRGD: new Map(), isLoading: false });

      const rgd = createTestRGD({ dependsOnKinds: ["UnknownKind"] });
      renderWithProviders(
        <RGDDetailView rgd={rgd} onBack={vi.fn()} />
      );

      expect(screen.getByText("UnknownKind")).toBeInTheDocument();
      // Should be plain text, not a link
      expect(screen.queryByRole("link", { name: "UnknownKind" })).not.toBeInTheDocument();
    });
  });

  describe("Overview Tab - Extends row (ExtendsKindLink pure display)", () => {
    it("renders ExtendsKindLink as a link when parent RGD is resolved via useKindToRGDMap", async () => {
      const { useKindToRGDMap } = await import("@/hooks/useKindToRGDMap");
      const map = new Map();
      map.set("SimpleApp", createTestRGD({ name: "simple-app-rgd", kind: "SimpleApp" }));
      vi.mocked(useKindToRGDMap).mockReturnValue({ kindToRGD: map, isLoading: false });

      const rgd = createTestRGD({ extendsKinds: ["SimpleApp"] });
      renderWithProviders(<RGDDetailView rgd={rgd} onBack={vi.fn()} />);

      const link = screen.getByRole("link", { name: "SimpleApp" });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute("href", "/catalog/simple-app-rgd");
    });

    it("renders ExtendsKindLink as plain text when parent RGD is not found in catalog", async () => {
      const { useKindToRGDMap } = await import("@/hooks/useKindToRGDMap");
      vi.mocked(useKindToRGDMap).mockReturnValue({ kindToRGD: new Map(), isLoading: false });

      const rgd = createTestRGD({ extendsKinds: ["UnknownParentKind"] });
      renderWithProviders(<RGDDetailView rgd={rgd} onBack={vi.fn()} />);

      expect(screen.getByText("UnknownParentKind")).toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "UnknownParentKind" })).not.toBeInTheDocument();
    });

    it("does not render Extends row when extendsKinds is empty", () => {
      const rgd = createTestRGD({ extendsKinds: [] });
      renderWithProviders(<RGDDetailView rgd={rgd} onBack={vi.fn()} />);
      expect(screen.queryByText("Extends")).not.toBeInTheDocument();
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
});
