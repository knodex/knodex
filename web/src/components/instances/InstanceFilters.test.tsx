// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { InstanceFilters, type InstanceFilterState } from "./InstanceFilters";

const defaultFilters: InstanceFilterState = {
  search: "",
  rgd: "",
  health: "",
  scope: "",
};

describe("InstanceFilters", () => {
  it("renders search input", () => {
    render(
      <InstanceFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableRgds={[]}
      />
    );
    expect(screen.getByPlaceholderText("Filter instances...")).toBeInTheDocument();
  });

  it("renders RGD selector", () => {
    render(
      <InstanceFilters
        filters={defaultFilters}
        onFiltersChange={vi.fn()}
        availableRgds={["my-database", "web-app"]}
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
      />
    );
    expect(screen.getByLabelText("Filter by health")).toBeInTheDocument();
  });

  describe("active filters indicator", () => {
    it("shows health in active filters", () => {
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, health: "Healthy" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
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
        />
      );
      expect(screen.getByLabelText("Clear all filters")).toBeInTheDocument();
    });

    it("clears all filters when clear is clicked", () => {
      const onFiltersChange = vi.fn();
      render(
        <InstanceFilters
          filters={{
            search: "test",
            rgd: "",
            health: "",
            scope: "",
          }}
          onFiltersChange={onFiltersChange}
          availableRgds={[]}
        />
      );

      fireEvent.click(screen.getByLabelText("Clear all filters"));

      expect(onFiltersChange).toHaveBeenCalledWith({
        search: "",
        rgd: "",
        health: "",
        scope: "",
      });
    });

    it("does not show active filters indicator when no filters", () => {
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
        />
      );
      expect(screen.queryByText("Filters:")).not.toBeInTheDocument();
    });

    describe("individual dismiss buttons", () => {
      it("clears search filter when dismiss is clicked", () => {
        const onFiltersChange = vi.fn();
        render(
          <InstanceFilters
            filters={{ ...defaultFilters, search: "my-app", rgd: "my-rgd" }}
            onFiltersChange={onFiltersChange}
            availableRgds={["my-rgd"]}
          />
        );

        fireEvent.click(screen.getByTestId("remove-search-filter"));

        expect(onFiltersChange).toHaveBeenCalledWith({
          ...defaultFilters,
          search: "",
          rgd: "my-rgd",
        });
      });

      it("clears rgd filter when dismiss is clicked", () => {
        const onFiltersChange = vi.fn();
        render(
          <InstanceFilters
            filters={{ ...defaultFilters, rgd: "my-database" }}
            onFiltersChange={onFiltersChange}
            availableRgds={["my-database"]}
          />
        );

        fireEvent.click(screen.getByTestId("remove-rgd-filter"));

        expect(onFiltersChange).toHaveBeenCalledWith({
          ...defaultFilters,
          rgd: "",
        });
      });

      it("clears health filter when dismiss is clicked", () => {
        const onFiltersChange = vi.fn();
        render(
          <InstanceFilters
            filters={{ ...defaultFilters, health: "Unhealthy" }}
            onFiltersChange={onFiltersChange}
            availableRgds={[]}
          />
        );

        fireEvent.click(screen.getByTestId("remove-health-filter"));

        expect(onFiltersChange).toHaveBeenCalledWith({
          ...defaultFilters,
          health: "",
        });
      });

      it("clears scope filter when dismiss is clicked", () => {
        const onFiltersChange = vi.fn();
        render(
          <InstanceFilters
            filters={{ ...defaultFilters, scope: "cluster" }}
            onFiltersChange={onFiltersChange}
            availableRgds={[]}
          />
        );

        fireEvent.click(screen.getByTestId("remove-scope-filter"));

        expect(onFiltersChange).toHaveBeenCalledWith({
          ...defaultFilters,
          scope: "",
        });
      });
    });
  });

  describe("scope filter", () => {
    it("renders scope selector", () => {
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
        />
      );
      expect(screen.getByLabelText("Filter by scope")).toBeInTheDocument();
    });

    it("shows Cluster-Scoped in active filters when scope is cluster", () => {
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, scope: "cluster" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
        />
      );
      expect(screen.getByText("Filters:")).toBeInTheDocument();
      const filtersSection = screen.getByText("Filters:").parentElement;
      expect(filtersSection).not.toBeNull();
      expect(within(filtersSection!).getByText("Cluster-Scoped")).toBeInTheDocument();
    });

    it("shows Namespaced in active filters when scope is namespaced", () => {
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, scope: "namespaced" }}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
        />
      );
      expect(screen.getByText("Filters:")).toBeInTheDocument();
      const filtersSection = screen.getByText("Filters:").parentElement;
      expect(filtersSection).not.toBeNull();
      expect(within(filtersSection!).getByText("Namespaced")).toBeInTheDocument();
    });

    it("clears scope when clear all filters is clicked", () => {
      const onFiltersChange = vi.fn();
      render(
        <InstanceFilters
          filters={{ ...defaultFilters, scope: "cluster" }}
          onFiltersChange={onFiltersChange}
          availableRgds={[]}
        />
      );

      fireEvent.click(screen.getByLabelText("Clear all filters"));

      expect(onFiltersChange).toHaveBeenCalledWith({
        search: "",
        rgd: "",
        health: "",
        scope: "",
      });
    });
  });

  describe("search debouncing", () => {
    it("updates search value immediately in input", () => {
      render(
        <InstanceFilters
          filters={defaultFilters}
          onFiltersChange={vi.fn()}
          availableRgds={[]}
        />
      );

      const input = screen.getByPlaceholderText("Filter instances...");
      fireEvent.change(input, { target: { value: "test-search" } });

      expect(input).toHaveValue("test-search");
    });
  });
});
