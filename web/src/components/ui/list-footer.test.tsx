// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ListFooter } from "./list-footer";

describe("ListFooter", () => {
  it("renders the total with default totalLabel='total'", () => {
    render(<ListFooter total={42} />);
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("total")).toBeInTheDocument();
  });

  it("uses a custom totalLabel when provided", () => {
    render(<ListFooter total={7} totalLabel="instances" />);
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("instances")).toBeInTheDocument();
  });

  it("renders one breakdown entry per tuple", () => {
    render(
      <ListFooter
        total={10}
        breakdown={[
          ["healthy", 6],
          ["warning", 3],
          ["error", 1],
        ]}
      />
    );
    expect(screen.getByText("healthy")).toBeInTheDocument();
    expect(screen.getByText("warning")).toBeInTheDocument();
    expect(screen.getByText("error")).toBeInTheDocument();
    expect(screen.getByText("6")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("marks separators as aria-hidden", () => {
    const { container } = render(
      <ListFooter
        total={5}
        breakdown={[
          ["a", 2],
          ["b", 3],
        ]}
      />
    );
    const seps = container.querySelectorAll('[aria-hidden="true"]');
    // Two breakdown entries => two separators
    expect(seps.length).toBe(2);
    seps.forEach((sep) => {
      expect(sep.textContent).toBe("·");
    });
  });

  it("renders nothing extra when breakdown is omitted", () => {
    const { container } = render(<ListFooter total={1} />);
    expect(container.querySelectorAll('[aria-hidden="true"]').length).toBe(0);
  });

  it("supports string breakdown values", () => {
    render(<ListFooter total={1} breakdown={[["queued", "n/a"]]} />);
    expect(screen.getByText("n/a")).toBeInTheDocument();
  });
});
