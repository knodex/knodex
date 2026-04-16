// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CatalogFilters, type FilterState } from "./CatalogFilters";

const defaultFilters: FilterState = {
  search: "",
  tags: [],
  category: "",
  projectScoped: false,
  producesKind: "",
};

describe("CatalogFilters", () => {
  it("renders search input", () => {
    render(
      <CatalogFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableTags={[]}
      />
    );
    expect(
      screen.getByPlaceholderText("Search by name, description, or tags...")
    ).toBeInTheDocument();
  });

  describe("active filters indicator", () => {
    it("shows clear button when filters are active", () => {
      render(
        <CatalogFilters
          filters={{ ...defaultFilters, search: "test" }}
          onFiltersChange={vi.fn()}
          availableTags={[]}
        />
      );
      expect(screen.getByLabelText("Clear all filters")).toBeInTheDocument();
    });

    it("clears all filters when clear is clicked", () => {
      const onFiltersChange = vi.fn();
      render(
        <CatalogFilters
          filters={{
            search: "test",
            tags: [],
            category: "",
            projectScoped: false,
            producesKind: "",
          }}
          onFiltersChange={onFiltersChange}
          availableTags={[]}
        />
      );

      fireEvent.click(screen.getByLabelText("Clear all filters"));

      expect(onFiltersChange).toHaveBeenCalledWith({
        search: "",
        tags: [],
        category: "",
        projectScoped: false,
        producesKind: "",
      });
    });

    it("shows active indicator when producesKind is set", () => {
      render(
        <CatalogFilters
          filters={{ ...defaultFilters, producesKind: "Cluster" }}
          onFiltersChange={vi.fn()}
          availableTags={[]}
        />
      );
      expect(screen.getByLabelText("Clear all filters")).toBeInTheDocument();
    });

    it("clears producesKind when clear is clicked", () => {
      const onFiltersChange = vi.fn();
      render(
        <CatalogFilters
          filters={{ ...defaultFilters, producesKind: "Cluster" }}
          onFiltersChange={onFiltersChange}
          availableTags={[]}
        />
      );

      fireEvent.click(screen.getByLabelText("Clear all filters"));

      expect(onFiltersChange).toHaveBeenCalledWith({
        search: "",
        tags: [],
        category: "",
        projectScoped: false,
        producesKind: "",
      });
    });

    it("does not show active filters indicator when no filters", () => {
      render(
        <CatalogFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableTags={[]}
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
