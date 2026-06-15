// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PageShell } from "./PageShell";

describe("PageShell", () => {
  it("renders the toolbar slot verbatim", () => {
    render(
      <PageShell
        toolbar={<input data-testid="filters" placeholder="search" />}
      />
    );
    expect(screen.getByTestId("filters")).toBeInTheDocument();
  });

  it("renders the primaryAction slot verbatim", () => {
    render(<PageShell primaryAction={<button>Deploy</button>} />);
    expect(screen.getByRole("button", { name: "Deploy" })).toBeInTheDocument();
  });

  it("renders both slots together", () => {
    render(
      <PageShell
        toolbar={<input data-testid="filters" />}
        primaryAction={<button>Create</button>}
      />
    );
    expect(screen.getByTestId("filters")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
  });

  it("renders an empty container with no chrome when both slots are omitted", () => {
    const { container } = render(<PageShell />);
    const outer = container.firstElementChild;
    expect(outer).not.toBeNull();
    expect(outer?.children.length).toBe(0);
    expect(container.querySelector("h1")).toBeNull();
    expect(container.querySelector("p")).toBeNull();
    expect(container.querySelector("nav")).toBeNull();
  });

  it("renders no h1, p, or nav in any slot configuration", () => {
    const { container } = render(
      <PageShell toolbar={<span>tools</span>} primaryAction={<span>act</span>} />
    );
    expect(container.querySelector("h1")).toBeNull();
    expect(container.querySelector("p")).toBeNull();
    expect(container.querySelector("nav")).toBeNull();
  });

  it("passes className through to the outer div", () => {
    const { container } = render(<PageShell className="mb-6" />);
    expect(container.firstChild).toHaveClass("mb-6");
  });

  it("forwards refs to the outer div", () => {
    const ref = { current: null as HTMLDivElement | null };
    render(<PageShell ref={ref} />);
    expect(ref.current).toBeInstanceOf(HTMLDivElement);
  });

  it("uses justify-between only when primaryAction is present", () => {
    const { container: c1 } = render(
      <PageShell toolbar={<span>t</span>} primaryAction={<span>p</span>} />
    );
    expect(c1.firstChild).toHaveClass("justify-between");

    const { container: c2 } = render(<PageShell toolbar={<span>t</span>} />);
    expect(c2.firstChild).not.toHaveClass("justify-between");
  });

  it("does not render a left filler when toolbar is omitted", () => {
    const { container } = render(
      <PageShell primaryAction={<button>Deploy</button>} />
    );
    // Only one child: the primaryAction (no flex-1 wrapper).
    const outer = container.firstElementChild;
    expect(outer?.children.length).toBe(1);
  });
});
