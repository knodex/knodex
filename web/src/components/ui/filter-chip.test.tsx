// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { FilterChip, FilterChipDot, filterChipClasses } from "./filter-chip";

describe("FilterChip", () => {
  it("renders children", () => {
    render(<FilterChip>Health</FilterChip>);
    expect(screen.getByRole("button", { name: "Health" })).toBeInTheDocument();
  });

  it("defaults to idle state when state prop is omitted", () => {
    render(<FilterChip>Health</FilterChip>);
    const btn = screen.getByRole("button");
    expect(btn.className).toMatch(/border-/);
  });

  it("shows a dot when showDot is true", () => {
    const { container } = render(<FilterChip showDot>Health: 2 selected</FilterChip>);
    expect(container.querySelector("[aria-hidden='true']")).toBeInTheDocument();
  });

  it("omits the dot when showDot prop is not provided", () => {
    const { container } = render(<FilterChip>Health</FilterChip>);
    expect(container.querySelector("[aria-hidden='true']")).toBeNull();
  });

  it("omits the dot when showDot is explicitly false", () => {
    const { container } = render(<FilterChip showDot={false}>Health</FilterChip>);
    expect(container.querySelector("[aria-hidden='true']")).toBeNull();
  });

  it("calls onClick when clicked", () => {
    const onClick = vi.fn();
    render(<FilterChip onClick={onClick}>Click me</FilterChip>);
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("defaults to type='button' (not 'submit')", () => {
    render(<FilterChip>Health</FilterChip>);
    expect(screen.getByRole("button")).toHaveAttribute("type", "button");
  });

  it("forwards ref to the underlying button", () => {
    const ref = { current: null as HTMLButtonElement | null };
    render(<FilterChip ref={ref}>Health</FilterChip>);
    expect(ref.current).toBeInstanceOf(HTMLButtonElement);
  });

  it("supports `add` state for the trailing + slot", () => {
    render(<FilterChip state="add" aria-label="Add filter">+</FilterChip>);
    const btn = screen.getByLabelText("Add filter");
    expect(btn.className).toMatch(/border-dashed/);
  });
});

describe("FilterChipDot", () => {
  it("renders a non-focusable indicator", () => {
    const { container } = render(<FilterChipDot />);
    const dot = container.querySelector("span");
    expect(dot).not.toBeNull();
    expect(dot).toHaveAttribute("aria-hidden", "true");
  });
});

describe("filterChipClasses", () => {
  it("returns idle classes by default", () => {
    const classes = filterChipClasses();
    // Chips use rounded-md to match other filter affordances across the app.
    expect(classes).toMatch(/rounded-md/);
  });

  it("returns active classes when state='active'", () => {
    const idle = filterChipClasses("idle");
    const active = filterChipClasses("active");
    expect(idle).not.toEqual(active);
  });
});
