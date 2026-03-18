// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { DependsOnTab } from "./DependsOnTab";
import type { CatalogRGD } from "@/types/rgd";

// Mock the shared hook
vi.mock("@/hooks/useKindToRGDMap", () => ({
  useKindToRGDMap: vi.fn().mockReturnValue({
    kindToRGD: new Map(),
    isLoading: false,
  }),
}));

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "Test RGD",
    version: "v1",
    tags: [],
    category: "compute",
    labels: {},
    instances: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
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

describe("DependsOnTab", () => {
  it("renders empty state when no dependencies", () => {
    const rgd = createTestRGD({ dependsOnKinds: [] });
    renderWithProviders(<DependsOnTab rgd={rgd} />);
    expect(screen.getByText("No dependencies")).toBeInTheDocument();
  });

  it("renders empty state when dependsOnKinds is undefined", () => {
    const rgd = createTestRGD();
    renderWithProviders(<DependsOnTab rgd={rgd} />);
    expect(screen.getByText("No dependencies")).toBeInTheDocument();
  });

  it("renders dependency cards when dependsOnKinds has entries", () => {
    const rgd = createTestRGD({
      dependsOnKinds: ["AKSCluster", "KeyVault"],
    });
    renderWithProviders(<DependsOnTab rgd={rgd} />);
    expect(screen.getByText("Dependencies")).toBeInTheDocument();
    expect(screen.getByText("AKSCluster")).toBeInTheDocument();
    expect(screen.getByText("KeyVault")).toBeInTheDocument();
  });

  it("shows 'Not found in catalog' for unresolved kinds", () => {
    const rgd = createTestRGD({
      dependsOnKinds: ["AKSCluster"],
    });
    renderWithProviders(<DependsOnTab rgd={rgd} />);
    expect(screen.getByText("Not found in catalog")).toBeInTheDocument();
  });

  it("renders resolved card as a link when parent RGD is found", async () => {
    const { useKindToRGDMap } = await import("@/hooks/useKindToRGDMap");
    const map = new Map();
    map.set("AKSCluster", createTestRGD({
      name: "aks-cluster-rgd",
      kind: "AKSCluster",
      description: "Managed AKS cluster",
      tags: ["azure", "kubernetes", "infra"],
    }));
    vi.mocked(useKindToRGDMap).mockReturnValue({
      kindToRGD: map,
      isLoading: false,
    });

    const rgd = createTestRGD({ dependsOnKinds: ["AKSCluster"] });
    renderWithProviders(<DependsOnTab rgd={rgd} />);

    // Should render links to the parent RGD (title link + view details link)
    const links = screen.getAllByRole("link");
    expect(links.some((l) => l.getAttribute("href") === "/catalog/aks-cluster-rgd")).toBe(true);

    // Should show title, description, and first 3 tags
    expect(screen.getByText("aks-cluster-rgd")).toBeInTheDocument();
    expect(screen.getByText("Managed AKS cluster")).toBeInTheDocument();
    expect(screen.getByText("azure")).toBeInTheDocument();
    expect(screen.getByText("kubernetes")).toBeInTheDocument();
    expect(screen.getByText("infra")).toBeInTheDocument();
  });
});
