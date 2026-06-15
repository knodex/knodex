// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DataGrid } from "./data-grid";

describe("DataGrid", () => {
  it("renders one cell per item", () => {
    const { container } = render(
      <DataGrid
        items={[
          { label: "Name", value: "alice" },
          { label: "Email", value: "alice@x.io" },
          { label: "Role", value: "admin" },
        ]}
      />
    );
    expect(container.querySelectorAll("dt").length).toBe(3);
    expect(container.querySelectorAll("dd").length).toBe(3);
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
  });

  it("defaults to 2 columns", () => {
    const { container } = render(
      <DataGrid items={[{ label: "x", value: "y" }]} />
    );
    const grid = container.querySelector("dl") as HTMLDListElement;
    expect(grid.style.gridTemplateColumns).toBe("repeat(2, minmax(0, 1fr))");
  });

  it("honors the columns prop", () => {
    const { container } = render(
      <DataGrid columns={4} items={[{ label: "x", value: "y" }]} />
    );
    const grid = container.querySelector("dl") as HTMLDListElement;
    expect(grid.style.gridTemplateColumns).toBe("repeat(4, minmax(0, 1fr))");
  });

  it("applies font-mono on value when mono is true", () => {
    render(
      <DataGrid
        items={[
          { label: "Plain", value: "p-val" },
          { label: "Mono", value: "m-val", mono: true },
        ]}
      />
    );
    expect(screen.getByText("p-val")).not.toHaveClass("font-mono");
    expect(screen.getByText("m-val")).toHaveClass("font-mono");
  });

  it("renders empty when items is empty", () => {
    const { container } = render(<DataGrid items={[]} />);
    expect(container.querySelectorAll("dt").length).toBe(0);
  });
});
