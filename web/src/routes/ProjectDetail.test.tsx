// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { ProjectDetail } from "./ProjectDetail";
import type { Project } from "@/types/project";

// Mock all hooks and child components to isolate ProjectDetail logic
vi.mock("@/hooks/useProjects", () => ({
  useProject: vi.fn(),
  useUpdateProject: vi.fn(() => ({
    mutateAsync: vi.fn(),
    isPending: false,
  })),
}));

vi.mock("@/hooks/useCanI", () => ({
  useCanI: vi.fn(() => ({
    allowed: true,
    isLoading: false,
    isError: false,
  })),
}));

// Mock tab components to simplify rendering
vi.mock("@/components/projects/tabs/ProjectOverviewTab", () => ({
  ProjectOverviewTab: () => <div data-testid="overview-tab">Overview</div>,
}));
vi.mock("@/components/projects/tabs/ProjectRolesTab", () => ({
  ProjectRolesTab: () => <div data-testid="roles-tab">Roles</div>,
}));
vi.mock("@/components/projects/tabs/ProjectDestinationsTab", () => ({
  ProjectDestinationsTab: () => (
    <div data-testid="destinations-tab">Destinations</div>
  ),
}));
vi.mock("@/components/projects/tabs/ProjectResourcesTab", () => ({
  ProjectResourcesTab: () => (
    <div data-testid="resources-tab">Resources</div>
  ),
}));

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

function renderProjectDetail(projectName: string) {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/projects/${projectName}`]}>
        <Routes>
          <Route path="/projects/:name" element={<ProjectDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const monoClusterProject: Project = {
  name: "mono-app",
  type: "app",
  description: "Monocluster project",
  resourceVersion: "1",
  createdAt: "2026-01-01T00:00:00Z",
  // No clusters field — monocluster mode
};

const multiClusterProject: Project = {
  name: "multi-app",
  type: "app",
  description: "Multi-cluster project",
  clusters: [{ clusterRef: "prod-eu-west" }, { clusterRef: "prod-us-east" }],
  namespace: "multi-app-ns",
  resourceVersion: "2",
  createdAt: "2026-01-01T00:00:00Z",
};

describe("ProjectDetail — Resources tab visibility", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render Resources tab for monocluster project", async () => {
    const { useProject } = await import("@/hooks/useProjects");
    vi.mocked(useProject).mockReturnValue({
      data: monoClusterProject,
      isLoading: false,
      error: null,
    } as any);

    renderProjectDetail("mono-app");

    // Tab triggers: Overview, Roles, Destinations should be present
    expect(screen.getByRole("tab", { name: /Overview/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Roles/i })).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: /Destinations/i })
    ).toBeInTheDocument();

    // Resources tab should NOT be present
    expect(
      screen.queryByRole("tab", { name: /Resources/i })
    ).not.toBeInTheDocument();
  });

  it("renders Resources tab for multi-cluster project", async () => {
    const { useProject } = await import("@/hooks/useProjects");
    vi.mocked(useProject).mockReturnValue({
      data: multiClusterProject,
      isLoading: false,
      error: null,
    } as any);

    renderProjectDetail("multi-app");

    // All 4 tabs should be present
    expect(screen.getByRole("tab", { name: /Overview/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /Roles/i })).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: /Destinations/i })
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: /Resources/i })
    ).toBeInTheDocument();
  });

  it("renders 3 tabs for monocluster projects", async () => {
    const { useProject } = await import("@/hooks/useProjects");
    vi.mocked(useProject).mockReturnValue({
      data: monoClusterProject,
      isLoading: false,
      error: null,
    } as any);

    renderProjectDetail("mono-app");

    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(3);
  });

  it("renders 4 tabs for multi-cluster projects", async () => {
    const { useProject } = await import("@/hooks/useProjects");
    vi.mocked(useProject).mockReturnValue({
      data: multiClusterProject,
      isLoading: false,
      error: null,
    } as any);

    renderProjectDetail("multi-app");

    const tabs = screen.getAllByRole("tab");
    expect(tabs).toHaveLength(4);
  });
});
