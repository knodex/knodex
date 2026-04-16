// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ProjectResourcesTab } from "./ProjectResourcesTab";
import type { Project, ResourceAggregationResponse } from "@/types/project";

vi.mock("@/hooks/useProjects", () => ({
  useProjectResources: vi.fn(),
}));

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  );
};

const mockProject: Project = {
  name: "team-alpha",
  type: "app",
  description: "Test project",
  clusters: [{ clusterRef: "prod-eu-west" }, { clusterRef: "prod-us-east" }],
  namespace: "team-alpha-ns",
  resourceVersion: "1",
  createdAt: "2026-01-01T00:00:00Z",
};

const mockResourcesResponse: ResourceAggregationResponse = {
  items: [
    {
      name: "cert-eu",
      kind: "Certificate",
      cluster: "prod-eu-west",
      namespace: "team-alpha-ns",
      status: "Ready",
      age: "2d",
    },
    {
      name: "cert-us",
      kind: "Certificate",
      cluster: "prod-us-east",
      namespace: "team-alpha-ns",
      status: "NotReady",
      age: "1h",
    },
  ],
  totalCount: 2,
};

describe("ProjectResourcesTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders table with correct columns when resources loaded", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: mockResourcesResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    // Check column headers
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Kind")).toBeInTheDocument();
    expect(screen.getByText("Cluster")).toBeInTheDocument();
    expect(screen.getByText("Namespace")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Age")).toBeInTheDocument();

    // Check data rows
    expect(screen.getByText("cert-eu")).toBeInTheDocument();
    expect(screen.getByText("cert-us")).toBeInTheDocument();
    expect(screen.getByText("prod-eu-west")).toBeInTheDocument();
    expect(screen.getByText("prod-us-east")).toBeInTheDocument();
  });

  it("shows skeleton loader during loading (no spinner)", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as any);

    const { container } = renderWithProviders(
      <ProjectResourcesTab project={mockProject} />
    );

    // Skeleton loader should be present (TableLoadingSkeleton renders with data-testid or skeleton classes)
    // No spinner (Loader2 animate-spin) should be in the DOM
    expect(container.querySelector(".animate-spin")).toBeNull();
    // The table should not be rendered
    expect(screen.queryByText("cert-eu")).not.toBeInTheDocument();
  });

  it("shows unreachable cluster banner for each unreachable cluster", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: {
        items: [mockResourcesResponse.items[0]],
        totalCount: 1,
        clusterStatus: {
          "prod-eu-west": { phase: "ready" as const },
          "prod-us-east": {
            phase: "unreachable" as const,
            message: "connection refused",
          },
        },
      },
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    expect(
      screen.getByText((_, el) => {
        return (
          el?.tagName === "STRONG" && el?.textContent === "prod-us-east"
        ) as boolean;
      })
    ).toBeInTheDocument();
    expect(screen.getByText(/connection refused/)).toBeInTheDocument();
  });

  it("clicking a row opens side panel with resource details", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: mockResourcesResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    // Click the first resource row
    const row = screen.getByText("cert-eu").closest("tr")!;
    await userEvent.click(row);

    // Side panel should show resource details
    expect(screen.getByText("cert-eu (Certificate)")).toBeInTheDocument();
    // YAML-like content should be in a <pre> block
    expect(screen.getByText(/name: cert-eu/)).toBeInTheDocument();
  });

  it("closing the sheet clears selectedResource", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: mockResourcesResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    // Open panel
    const row = screen.getByText("cert-eu").closest("tr")!;
    await userEvent.click(row);
    expect(screen.getByText("cert-eu (Certificate)")).toBeInTheDocument();

    // Close the sheet via the close button (Radix Sheet renders a close button)
    const closeButton = screen.getByRole("button", { name: /close/i });
    await userEvent.click(closeButton);

    // Panel title should no longer be visible
    expect(screen.queryByText("cert-eu (Certificate)")).not.toBeInTheDocument();
  });

  it("kind selector switches between Certificate and Ingress", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: mockResourcesResponse,
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    // Initially Certificate is active
    expect(useProjectResources).toHaveBeenCalledWith(
      "team-alpha",
      "Certificate",
      true
    );

    // Click Ingress button
    const ingressButton = screen.getByRole("button", { name: "Ingress" });
    await userEvent.click(ingressButton);

    // Hook should be re-called with Ingress
    expect(useProjectResources).toHaveBeenCalledWith("team-alpha", "Ingress", true);
  });

  it("empty state renders correct message when items is empty", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: { items: [], totalCount: 0 },
      isLoading: false,
      error: null,
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    expect(
      screen.getByText("No Certificate resources found across your clusters")
    ).toBeInTheDocument();
  });

  it("error state shows error message", async () => {
    const { useProjectResources } = await import("@/hooks/useProjects");
    vi.mocked(useProjectResources).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Network error"),
    } as any);

    renderWithProviders(<ProjectResourcesTab project={mockProject} />);

    expect(screen.getByText("Failed to load resources")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();
  });
});
