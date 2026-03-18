// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { InstanceDependsOn } from "./InstanceDependsOn";
import type { Instance } from "@/types/rgd";

// Mock the hooks
vi.mock("@/hooks/useRGDs", () => ({
  useRGDResourceGraph: vi.fn().mockReturnValue({
    data: null,
    isLoading: false,
  }),
}));

vi.mock("@/hooks/useInstances", () => ({
  useInstanceList: vi.fn().mockReturnValue({
    data: { items: [], totalCount: 0, page: 1, pageSize: 100 },
    isLoading: false,
  }),
}));

vi.mock("@/components/instances/HealthBadge", () => ({
  HealthBadge: ({ health }: { health: string }) => (
    <span data-testid="health-badge">{health}</span>
  ),
}));

function createTestInstance(
  overrides: Partial<Instance> = {}
): Instance {
  return {
    name: "test-instance",
    namespace: "test-ns",
    rgdName: "test-rgd",
    rgdNamespace: "default",
    apiVersion: "kro.run/v1alpha1",
    kind: "TestResource",
    health: "Healthy",
    conditions: [],
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

describe("InstanceDependsOn", () => {
  it("renders nothing when instance has no externalRef in spec", () => {
    const instance = createTestInstance({ spec: { replicas: 3 } });
    const { container } = renderWithProviders(
      <InstanceDependsOn instance={instance} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when spec is undefined", () => {
    const instance = createTestInstance({ spec: undefined });
    const { container } = renderWithProviders(
      <InstanceDependsOn instance={instance} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders dependency section when externalRef entries exist", () => {
    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceDependsOn instance={instance} />);
    expect(screen.getByText("Depends On (1)")).toBeInTheDocument();
  });

  it("renders multiple dependency entries", () => {
    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
          vault: { name: "my-vault", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceDependsOn instance={instance} />);
    expect(screen.getByText("Depends On (2)")).toBeInTheDocument();
  });

  it("shows resolving state while instances are loading", async () => {
    const { useInstanceList } = await import("@/hooks/useInstances");
    vi.mocked(useInstanceList).mockReturnValueOnce({
      data: undefined,
      isLoading: true,
    } as ReturnType<typeof useInstanceList>);

    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceDependsOn instance={instance} />);
    expect(screen.getByText("Resolving my-cluster...")).toBeInTheDocument();
  });

  it("shows 'Not deployed' for unresolved dependencies", () => {
    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceDependsOn instance={instance} />);
    expect(screen.getByText("Not deployed")).toBeInTheDocument();
  });

  it("renders resolved dependency as a clickable card with health badge", async () => {
    const { useInstanceList } = await import("@/hooks/useInstances");
    vi.mocked(useInstanceList).mockReturnValue({
      data: {
        items: [
          {
            name: "my-cluster",
            namespace: "default",
            rgdName: "aks-cluster",
            rgdNamespace: "default",
            apiVersion: "kro.run/v1alpha1",
            kind: "AKSCluster",
            health: "Healthy" as const,
            conditions: [],
            createdAt: "2026-01-01T00:00:00Z",
            updatedAt: "2026-01-01T00:00:00Z",
          },
        ],
        totalCount: 1,
        page: 1,
        pageSize: 100,
      },
      isLoading: false,
    } as ReturnType<typeof useInstanceList>);

    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceDependsOn instance={instance} />);

    // Should render as a link to the dependency instance
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/instances/default/AKSCluster/my-cluster");

    // Should show instance name and health badge
    expect(screen.getByText("my-cluster")).toBeInTheDocument();
    expect(screen.getByTestId("health-badge")).toHaveTextContent("Healthy");

    // Should show kind and namespace badges
    expect(screen.getByText("AKSCluster")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
  });
});
