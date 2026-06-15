// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Avatar, avatarInitials, avatarToneIndex } from "./avatar";

describe("avatarToneIndex", () => {
  it("returns the same tone for the same seed across calls", () => {
    expect(avatarToneIndex("alice")).toBe(avatarToneIndex("alice"));
    expect(avatarToneIndex("bob@x.com")).toBe(avatarToneIndex("bob@x.com"));
  });

  it("returns a value between 1 and 7 inclusive", () => {
    for (const seed of ["a", "ab", "carol", "dan", "eve@x.io", "frank", "grace"]) {
      const tone = avatarToneIndex(seed);
      expect(tone).toBeGreaterThanOrEqual(1);
      expect(tone).toBeLessThanOrEqual(7);
    }
  });

  it("distributes different seeds across multiple tones (not a constant)", () => {
    const seeds = ["alice", "bob", "carol", "dan", "eve", "frank", "grace", "henry"];
    const tones = new Set(seeds.map(avatarToneIndex));
    expect(tones.size).toBeGreaterThanOrEqual(2);
  });
});

describe("avatarInitials", () => {
  it("returns the first letter of each of the first two tokens", () => {
    expect(avatarInitials("Alice Doe")).toBe("AD");
    expect(avatarInitials("Alice")).toBe("A");
  });

  it("splits emails on @ and dots", () => {
    expect(avatarInitials("alice@example.com")).toBe("AE");
    expect(avatarInitials("alice")).toBe("A");
  });

  it("falls back to '?' for empty seed", () => {
    expect(avatarInitials("")).toBe("?");
  });

  it("uppercases the initials", () => {
    expect(avatarInitials("alice doe")).toBe("AD");
  });
});

describe("Avatar", () => {
  it("renders the same tone for the same name across separate renders", () => {
    const { container: c1 } = render(<Avatar name="alice" />);
    const { container: c2 } = render(<Avatar name="alice" />);
    const t1 = c1.firstElementChild?.getAttribute("data-tone");
    const t2 = c2.firstElementChild?.getAttribute("data-tone");
    expect(t1).toBeTruthy();
    expect(t1).toBe(t2);
  });

  it("renders different tones for at least one pair of different names", () => {
    const seeds = ["alice", "bob", "carol", "dan"];
    const tones = seeds.map((seed) => {
      const { container } = render(<Avatar name={seed} />);
      return container.firstElementChild?.getAttribute("data-tone");
    });
    const unique = new Set(tones);
    expect(unique.size).toBeGreaterThanOrEqual(2);
  });

  it("uses `name` for initials and falls back to `email` then '?'", () => {
    const { rerender } = render(<Avatar name="Alice Doe" />);
    expect(screen.getByText("AD")).toBeInTheDocument();
    rerender(<Avatar email="alice@x.com" />);
    expect(screen.getByText("AX")).toBeInTheDocument();
    rerender(<Avatar />);
    expect(screen.getByText("?")).toBeInTheDocument();
  });

  it("aria-label falls back name → email → '?'", () => {
    const { rerender } = render(<Avatar name="Alice" />);
    expect(screen.getByRole("img")).toHaveAttribute("aria-label", "Alice");
    rerender(<Avatar email="bob@x.com" />);
    expect(screen.getByRole("img")).toHaveAttribute("aria-label", "bob@x.com");
    rerender(<Avatar />);
    expect(screen.getByRole("img")).toHaveAttribute("aria-label", "?");
  });

  it("applies size class for sm/md/lg", () => {
    const { container, rerender } = render(<Avatar name="x" size="sm" />);
    expect(container.firstElementChild).toHaveClass("h-6", "w-6");
    rerender(<Avatar name="x" size="md" />);
    expect(container.firstElementChild).toHaveClass("h-9", "w-9");
    rerender(<Avatar name="x" size="lg" />);
    expect(container.firstElementChild).toHaveClass("h-11", "w-11");
  });

  it("defaults to size 'md'", () => {
    const { container } = render(<Avatar name="x" />);
    expect(container.firstElementChild).toHaveClass("h-9", "w-9");
  });

  it("sets background + foreground CSS via tone variables (no literal hex)", () => {
    const { container } = render(<Avatar name="alice" />);
    const el = container.firstElementChild as HTMLElement;
    const tone = el.getAttribute("data-tone");
    expect(tone).toBeTruthy();
    expect(el.style.backgroundColor).toContain(`--avatar-tone-${tone}-bg-hsl`);
    expect(el.style.color).toContain(`--avatar-tone-${tone}-fg`);
  });
});
