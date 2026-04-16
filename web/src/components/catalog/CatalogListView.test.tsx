// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CatalogListView } from "./CatalogListView";
import type { CatalogRGD } from "@/types/rgd";

function createTestRGD(overrides: Partial<CatalogRGD> = {}): CatalogRGD {
  return {
    name: "test-rgd",
    namespace: "default",
    description: "A test RGD",
    tags: ["database"],
    category: "database",
    labels: {},
    instances: 3,
    status: "Active",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

const items: CatalogRGD[] = [
  createTestRGD({ name: "bravo-rgd", title: "Bravo", instances: 5, category: "networking", updatedAt: "2026-01-02T00:00:00Z" }),
  createTestRGD({ name: "alpha-rgd", title: "Alpha", instances: 1, category: "database", updatedAt: "2026-01-03T00:00:00Z" }),
  createTestRGD({ name: "charlie-rgd", title: "Charlie", instances: 10, category: "compute", updatedAt: "2026-01-01T00:00:00Z" }),
];

describe("CatalogListView", () => {
  it("renders all items in a table", () => {
    render(<CatalogListView items={items} />);
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Bravo")).toBeInTheDocument();
    expect(screen.getByText("Charlie")).toBeInTheDocument();
  });

  it("sorts by name ascending by default", () => {
    render(<CatalogListView items={items} />);
    const rows = screen.getAllByRole("button", { name: /View details for/ });
    expect(rows[0]).toHaveAttribute("aria-label", "View details for Alpha");
    expect(rows[1]).toHaveAttribute("aria-label", "View details for Bravo");
    expect(rows[2]).toHaveAttribute("aria-label", "View details for Charlie");
  });

  it("toggles sort direction when clicking same column header", () => {
    render(<CatalogListView items={items} />);
    const nameHeader = screen.getByRole("button", { name: /Name/ });

    // Click to sort descending
    fireEvent.click(nameHeader);
    const rows = screen.getAllByRole("button", { name: /View details for/ });
    expect(rows[0]).toHaveAttribute("aria-label", "View details for Charlie");
    expect(rows[2]).toHaveAttribute("aria-label", "View details for Alpha");
  });

  it("sorts by instances when clicking Instances column", () => {
    render(<CatalogListView items={items} />);
    const instancesHeader = screen.getByRole("button", { name: /Instances/ });
    fireEvent.click(instancesHeader);

    const rows = screen.getAllByRole("button", { name: /View details for/ });
    // ascending: 1, 5, 10
    expect(rows[0]).toHaveAttribute("aria-label", "View details for Alpha");
    expect(rows[1]).toHaveAttribute("aria-label", "View details for Bravo");
    expect(rows[2]).toHaveAttribute("aria-label", "View details for Charlie");
  });

  it("calls onRGDClick when a row is clicked", () => {
    const onClick = vi.fn();
    render(<CatalogListView items={items} onRGDClick={onClick} />);
    const row = screen.getAllByRole("button", { name: /View details for/ })[0];
    fireEvent.click(row);
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onClick).toHaveBeenCalledWith(expect.objectContaining({ name: "alpha-rgd" }));
  });

  it("triggers click on Enter key", () => {
    const onClick = vi.fn();
    render(<CatalogListView items={items} onRGDClick={onClick} />);
    const row = screen.getAllByRole("button", { name: /View details for/ })[0];
    fireEvent.keyDown(row, { key: "Enter" });
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("triggers click on Space key", () => {
    const onClick = vi.fn();
    render(<CatalogListView items={items} onRGDClick={onClick} />);
    const row = screen.getAllByRole("button", { name: /View details for/ })[0];
    fireEvent.keyDown(row, { key: " " });
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("sets aria-sort on the active column header", () => {
    render(<CatalogListView items={items} />);
    // Name is default sort column
    const nameColumnHeader = screen.getByRole("columnheader", { name: /Name/ });
    expect(nameColumnHeader).toHaveAttribute("aria-sort", "ascending");

    // Other columns should be "none"
    const categoryColumnHeader = screen.getByRole("columnheader", { name: /Category/ });
    expect(categoryColumnHeader).toHaveAttribute("aria-sort", "none");
  });
});
