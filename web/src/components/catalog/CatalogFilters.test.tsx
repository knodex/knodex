import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { CatalogFilters, type FilterState } from "./CatalogFilters";

const defaultFilters: FilterState = {
  search: "",
  tags: [],
  project: "",
};

describe("CatalogFilters", () => {
  it("renders search input", () => {
    render(
      <CatalogFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableTags={[]}
        availableProjects={[]}
      />
    );
    expect(
      screen.getByPlaceholderText("Search by name, description, or tags...")
    ).toBeInTheDocument();
  });

  describe("project filter", () => {
    it("does not render project dropdown when no projects", () => {
      render(
        <CatalogFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={[]}
        />
      );
      expect(
        screen.queryByLabelText("Filter by project")
      ).not.toBeInTheDocument();
    });

    it("renders project dropdown when projects exist", () => {
      render(
        <CatalogFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={["project-a", "project-b"]}
        />
      );
      expect(screen.getByLabelText("Filter by project")).toBeInTheDocument();
    });

    it("displays selected project in trigger", () => {
      render(
        <CatalogFilters
          filters={{ ...defaultFilters, project: "project-a" }}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={["project-a", "project-b"]}
        />
      );
      // The project trigger button should contain the selected project name
      const trigger = screen.getByLabelText("Filter by project");
      expect(within(trigger).getByText("project-a")).toBeInTheDocument();
    });
  });

  describe("active filters indicator", () => {
    it("shows project in active filters", () => {
      render(
        <CatalogFilters
          filters={{ ...defaultFilters, project: "my-project" }}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={["my-project"]}
        />
      );
      // Should show "Filters:" label
      expect(screen.getByText("Filters:")).toBeInTheDocument();
      // Find the active filter chip for project (in the filter chips area)
      const filtersSection = screen.getByText("Filters:").parentElement;
      expect(filtersSection).not.toBeNull();
      expect(within(filtersSection!).getByText("my-project")).toBeInTheDocument();
    });

    it("shows clear button when filters are active", () => {
      render(
        <CatalogFilters
          filters={{ ...defaultFilters, project: "my-project" }}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={["my-project"]}
        />
      );
      expect(screen.getByLabelText("Clear all filters")).toBeInTheDocument();
    });

    it("clears all filters including project when clear is clicked", () => {
      const onFiltersChange = vi.fn();
      render(
        <CatalogFilters
          filters={{
            search: "test",
            tags: [],
            project: "my-project",
          }}
          onFiltersChange={onFiltersChange}
          availableTags={[]}
          availableProjects={["my-project"]}
        />
      );

      fireEvent.click(screen.getByLabelText("Clear all filters"));

      expect(onFiltersChange).toHaveBeenCalledWith({
        search: "",
        tags: [],
        project: "",
      });
    });

    it("does not show active filters indicator when no filters", () => {
      render(
        <CatalogFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={[]}
        />
      );
      expect(screen.queryByText("Filters:")).not.toBeInTheDocument();
    });
  });

  describe("search debouncing", () => {
    it("updates search value immediately in input", () => {
      render(
        <CatalogFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableTags={[]}
          availableProjects={[]}
        />
      );

      const input = screen.getByPlaceholderText(
        "Search by name, description, or tags..."
      );
      fireEvent.change(input, { target: { value: "test-search" } });

      expect(input).toHaveValue("test-search");
    });
  });
});
