// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { ProjectSelector } from "./ProjectSelector";
import { useUserStore } from "@/stores/userStore";
import { act } from "@testing-library/react";

// Mock useProjects hook
const mockUseProjects = vi.fn();
vi.mock("@/hooks/useProjects", () => ({
  useProjects: () => mockUseProjects(),
}));

// Helper to set store state directly
function setStoreState(overrides: Partial<ReturnType<typeof useUserStore.getState>>) {
  act(() => {
    useUserStore.setState(overrides);
  });
}

function createProjectItems(names: string[]) {
  return names.map((name) => ({ name, resourceVersion: "1", createdAt: "2026-01-01" }));
}

describe("ProjectSelector", () => {
  beforeEach(() => {
    act(() => {
      useUserStore.getState().logout();
    });
    mockUseProjects.mockReturnValue({ data: { items: [] } });
  });

  it("does not render when not authenticated", () => {
    const { container } = render(<ProjectSelector />);
    expect(container.firstChild).toBeNull();
  });

  it("renders 'All Projects' when currentProject is null", () => {
    mockUseProjects.mockReturnValue({ data: { items: createProjectItems(["alpha", "beta"]) } });
    setStoreState({
      isAuthenticated: true,
      currentProject: null,
    });

    render(<ProjectSelector />);
    expect(screen.getByTestId("project-selector")).toHaveTextContent("All Projects");
  });

  it("renders project name when currentProject is set", () => {
    mockUseProjects.mockReturnValue({ data: { items: createProjectItems(["alpha", "beta"]) } });
    setStoreState({
      isAuthenticated: true,
      currentProject: "beta",
    });

    render(<ProjectSelector />);
    expect(screen.getByTestId("project-selector")).toHaveTextContent("beta");
  });

  it("has correct aria-label for accessibility", () => {
    setStoreState({
      isAuthenticated: true,
      currentProject: null,
    });

    render(<ProjectSelector />);
    expect(screen.getByLabelText("Select project")).toBeInTheDocument();
  });

  it("renders when API returns no projects", () => {
    mockUseProjects.mockReturnValue({ data: { items: [] } });
    setStoreState({
      isAuthenticated: true,
      currentProject: null,
    });

    render(<ProjectSelector />);
    expect(screen.getByTestId("project-selector")).toHaveTextContent("All Projects");
  });

  it("renders with single project selected", () => {
    mockUseProjects.mockReturnValue({ data: { items: createProjectItems(["only-project"]) } });
    setStoreState({
      isAuthenticated: true,
      currentProject: "only-project",
    });

    render(<ProjectSelector />);
    expect(screen.getByTestId("project-selector")).toHaveTextContent("only-project");
  });
});
