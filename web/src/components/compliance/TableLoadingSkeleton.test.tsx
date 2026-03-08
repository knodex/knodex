// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TableLoadingSkeleton } from "./TableLoadingSkeleton";

describe("TableLoadingSkeleton", () => {
  const columns = [
    { header: "Name", width: "25%" },
    { header: "Kind", width: "15%" },
    { header: "Status", width: "20%" },
    { header: "Created", width: "40%" },
  ];

  it("renders correct number of columns (AC-SHARED-03)", () => {
    render(<TableLoadingSkeleton columns={columns} />);

    // Should render header cells for each column
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Kind")).toBeInTheDocument();
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Created")).toBeInTheDocument();
  });

  it("renders default number of skeleton rows", () => {
    const { container } = render(<TableLoadingSkeleton columns={columns} />);

    // Should render multiple skeleton rows (default is typically 5-10)
    // Count the Skeleton elements in table body
    const skeletons = container.querySelectorAll("[data-slot='skeleton'], .animate-pulse");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("renders specified number of rows", () => {
    const { container } = render(<TableLoadingSkeleton columns={columns} rows={3} />);

    // Should render exactly 3 rows
    const tableBody = container.querySelector("tbody");
    const rows = tableBody?.querySelectorAll("tr");
    expect(rows?.length).toBe(3);
  });

  it("renders double-line skeletons when showDoubleLines is true", () => {
    const { container } = render(
      <TableLoadingSkeleton columns={columns} showDoubleLines />
    );

    // When showDoubleLines is true, each cell should have 2 skeleton lines
    // We can check for multiple skeleton elements per cell
    const skeletons = container.querySelectorAll("[data-slot='skeleton'], .animate-pulse");
    expect(skeletons.length).toBeGreaterThan(columns.length);
  });

  it("hides columns marked as hideOnMobile", () => {
    const columnsWithHidden = [
      { header: "Name", width: "30%" },
      { header: "Details", width: "70%", hideOnMobile: true },
    ];

    const { container } = render(<TableLoadingSkeleton columns={columnsWithHidden} />);

    // The hidden column should have the hidden class
    const headerCells = container.querySelectorAll("th");
    // One header should have a class that hides on mobile
    const hiddenHeaders = Array.from(headerCells).filter(
      (th) => th.className.includes("hidden") && th.className.includes("md:table-cell")
    );
    expect(hiddenHeaders.length).toBe(1);
  });

  it("applies column widths", () => {
    const { container } = render(<TableLoadingSkeleton columns={columns} />);

    const headerCells = container.querySelectorAll("th");
    // Check that style widths are applied
    expect(headerCells[0]).toHaveStyle({ width: "25%" });
    expect(headerCells[1]).toHaveStyle({ width: "15%" });
  });

  it("handles empty columns array", () => {
    const { container } = render(<TableLoadingSkeleton columns={[]} />);

    // Should render without crashing, might show empty table
    const table = container.querySelector("table");
    expect(table).toBeInTheDocument();
  });

  it("handles single column", () => {
    const singleColumn = [{ header: "Name", width: "100%" }];
    render(<TableLoadingSkeleton columns={singleColumn} />);

    expect(screen.getByText("Name")).toBeInTheDocument();
  });

  it("handles many columns", () => {
    const manyColumns = Array.from({ length: 10 }, (_, i) => ({
      header: `Column ${i + 1}`,
      width: "10%",
    }));

    render(<TableLoadingSkeleton columns={manyColumns} />);

    expect(screen.getByText("Column 1")).toBeInTheDocument();
    expect(screen.getByText("Column 10")).toBeInTheDocument();
  });

  it("renders with zero rows", () => {
    const { container } = render(<TableLoadingSkeleton columns={columns} rows={0} />);

    // Should render table structure but no body rows
    expect(container.querySelector("table")).toBeInTheDocument();
    const tableBody = container.querySelector("tbody");
    const rows = tableBody?.querySelectorAll("tr");
    expect(rows?.length).toBe(0);
  });
});
