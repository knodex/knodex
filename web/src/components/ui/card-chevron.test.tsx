// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { CardChevron } from "./card-chevron";

describe("CardChevron", () => {
  it("renders an aria-hidden chevron", () => {
    const { container } = render(<CardChevron />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toBeInTheDocument();
    expect(el.getAttribute("aria-hidden")).toBe("true");
    expect(el.querySelector("svg")).not.toBeNull();
  });

  it("is hidden by default (opacity-0) and reveals on group hover (group-hover:opacity-100)", () => {
    const { container } = render(<CardChevron />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveClass("opacity-0");
    expect(el).toHaveClass("group-hover:opacity-100");
  });

  it("has the absolute-positioning + transition classes", () => {
    const { container } = render(<CardChevron />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveClass("absolute", "top-3", "right-3");
    expect(el).toHaveClass("transition-opacity", "duration-150");
  });

  it("merges a caller-provided className", () => {
    const { container } = render(<CardChevron className="custom-x" />);
    const el = container.firstElementChild as HTMLElement;
    expect(el).toHaveClass("custom-x");
    expect(el).toHaveClass("absolute");
  });

  it("renders inside a .group parent and stays opacity-0 until hover (visual contract)", () => {
    // We can't simulate Tailwind's :group-hover styling in jsdom (no real
    // CSS engine), but we pin the class contract so a refactor that drops
    // group-hover:opacity-100 fails immediately.
    const { container } = render(
      <div className="group relative">
        <CardChevron />
      </div>
    );
    const el = container.querySelector("[aria-hidden='true']") as HTMLElement;
    expect(el).toHaveClass("opacity-0", "group-hover:opacity-100");
  });
});
