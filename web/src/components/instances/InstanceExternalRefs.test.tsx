// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { InstanceExternalRefs } from "./InstanceExternalRefs";
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

describe("InstanceExternalRefs", () => {
  it("renders nothing when instance has no externalRef in spec", () => {
    const instance = createTestInstance({ spec: { replicas: 3 } });
    const { container } = renderWithProviders(
      <InstanceExternalRefs instance={instance} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when spec is undefined", () => {
    const instance = createTestInstance({ spec: undefined });
    const { container } = renderWithProviders(
      <InstanceExternalRefs instance={instance} />
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders external ref cards when externalRef entries exist", () => {
    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceExternalRefs instance={instance} />);
    expect(screen.getByText("my-cluster")).toBeInTheDocument();
  });

  it("renders multiple external ref entries", () => {
    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
          vault: { name: "my-vault", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceExternalRefs instance={instance} />);
    expect(screen.getByText("my-cluster")).toBeInTheDocument();
    expect(screen.getByText("my-vault")).toBeInTheDocument();
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
    renderWithProviders(<InstanceExternalRefs instance={instance} />);
    expect(screen.getByText("Resolving my-cluster...")).toBeInTheDocument();
  });

  it("shows 'External resource' for unresolved external refs", () => {
    const instance = createTestInstance({
      spec: {
        externalRef: {
          cluster: { name: "my-cluster", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceExternalRefs instance={instance} />);
    expect(screen.getByText("External resource")).toBeInTheDocument();
  });

  it("renders resolved external ref as a clickable card with health badge", async () => {
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
    renderWithProviders(<InstanceExternalRefs instance={instance} />);

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

  it("shows Secret badge when externalRef kind is Secret", async () => {
    const { useRGDResourceGraph } = await import("@/hooks/useRGDs");
    vi.mocked(useRGDResourceGraph).mockReturnValue({
      data: {
        resources: [
          {
            id: "dbSecret",
            apiVersion: "v1",
            kind: "Secret",
            isTemplate: false,
            isConditional: false,
            dependsOn: [],
            externalRef: {
              apiVersion: "v1",
              kind: "Secret",
              nameExpr: "${schema.spec.externalRef.dbSecret.name}",
              usesSchemaSpec: true,
              schemaField: "spec.externalRef.dbSecret",
            },
          },
        ],
        edges: [],
      },
      isLoading: false,
    } as ReturnType<typeof useRGDResourceGraph>);

    const instance = createTestInstance({
      spec: {
        externalRef: {
          dbSecret: { name: "my-db-credentials", namespace: "default" },
        },
      },
    });
    renderWithProviders(<InstanceExternalRefs instance={instance} />);

    // Should show Secret badge (appears as both badge and kindLabel)
    const secretTexts = screen.getAllByText("Secret");
    expect(secretTexts.length).toBeGreaterThanOrEqual(1);
  });
});
