// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { InstanceFilters, type InstanceFilterState } from "./InstanceFilters";
import type { Project } from "@/types/project";

// Mock project factory
function createProject(overrides: Partial<Project> = {}): Project {
  return {
    name: "test-project",
    resourceVersion: "1",
    createdAt: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

const defaultFilters: InstanceFilterState = {
  search: "",
  rgd: "",
  health: "",
  project: "",
};

describe("InstanceFilters", () => {
  it("renders search input", () => {
    render(
      <InstanceFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableRgds={[]}
        projects={[]}
      />
    );
    expect(screen.getByPlaceholderText("Search by name...")).toBeInTheDocument();
  });

  it("renders RGD selector", () => {
    render(
      <InstanceFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableRgds={["my-database", "web-app"]}
        projects={[]}
      />
    );
    expect(screen.getByLabelText("Filter by RGD")).toBeInTheDocument();
  });

  it("renders health selector", () => {
    render(
      <InstanceFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableRgds={[]}
        projects={[]}
      />
    );
    expect(screen.getByLabelText("Filter by health")).toBeInTheDocument();
  });

  describe("project filter", () => {
    it("does not render project dropdown when no projects", () => {
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={[]}
        />
      );
      expect(
        screen.queryByLabelText("Filter by project")
      ).not.toBeInTheDocument();
    });

    it("renders project dropdown when projects exist", () => {
      const projects = [
        createProject({ name: "project-a" }),
        createProject({ name: "project-b" }),
      ];
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={projects}
        />
      );
      expect(screen.getByLabelText("Filter by project")).toBeInTheDocument();
    });

    it("shows loading state when projectsLoading is true", () => {
      const projects = [createProject({ name: "project-a" })];
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={projects}
          projectsLoading={true}
        />
      );
      const projectTrigger = screen.getByLabelText("Filter by project");
      expect(projectTrigger).toBeDisabled();
    });

    it("displays selected project in trigger", () => {
      const projects = [
        createProject({ name: "project-a" }),
        createProject({ name: "project-b" }),
      ];
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, project: "project-a" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={projects}
        />
      );
      // The project trigger button should contain the selected project name
      const trigger = screen.getByLabelText("Filter by project");
      expect(within(trigger).getByText("project-a")).toBeInTheDocument();
    });
  });

  describe("active filters indicator", () => {
    it("shows project in active filters", () => {
      const projects = [createProject({ name: "my-project" })];
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, project: "my-project" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={projects}
        />
      );
      expect(screen.getByText("Filters:")).toBeInTheDocument();
      // Find the active filter chip for project (in the filter chips area)
      const filtersSection = screen.getByText("Filters:").parentElement;
      expect(filtersSection).not.toBeNull();
      expect(within(filtersSection!).getByText("my-project")).toBeInTheDocument();
    });

    it("shows health in active filters", () => {
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, health: "Healthy" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={[]}
        />
      );
      expect(screen.getByText("Filters:")).toBeInTheDocument();
    });

    it("shows clear button when filters are active", () => {
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, health: "Healthy" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={[]}
        />
      );
      expect(screen.getByLabelText("Clear all filters")).toBeInTheDocument();
    });

    it("clears all filters including project when clear is clicked", () => {
      const onFiltersChange = vi.fn();
      const projects = [createProject({ name: "my-project" })];
      render(
        <InstanceFilters
          filters={{
            search: "test",
            rgd: "",
            health: "",
            project: "my-project",
          }}
          onFiltersChange={onFiltersChange}
          availableRgds={[]}
          projects={projects}
        />
      );

      fireEvent.click(screen.getByLabelText("Clear all filters"));

      expect(onFiltersChange).toHaveBeenCalledWith({
        search: "",
        rgd: "",
        health: "",
        project: "",
      });
    });

    it("does not show active filters indicator when no filters", () => {
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={[]}
        />
      );
      expect(screen.queryByText("Filters:")).not.toBeInTheDocument();
    });
  });

  describe("search debouncing", () => {
    it("updates search value immediately in input", () => {
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
          projects={[]}
        />
      );

      const input = screen.getByPlaceholderText("Search by name...");
      fireEvent.change(input, { target: { value: "test-search" } });

      expect(input).toHaveValue("test-search");
    });
  });
});
