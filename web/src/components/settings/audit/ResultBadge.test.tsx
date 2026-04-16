// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResultBadge } from "./ResultBadge";

describe("ResultBadge", () => {
  it("renders success badge with green color class", () => {
    render(<ResultBadge result="success" />);
    const badge = screen.getByText("success");
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain("bg-green-100");
  });

  it("renders denied badge with red color class", () => {
    render(<ResultBadge result="denied" />);
    const badge = screen.getByText("denied");
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain("bg-red-100");
  });

  it("renders error badge with yellow color class", () => {
    render(<ResultBadge result="error" />);
    const badge = screen.getByText("error");
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain("bg-amber-100");
  });

  it("renders unknown result without color class", () => {
    render(<ResultBadge result="unknown" />);
    const badge = screen.getByText("unknown");
    expect(badge).toBeInTheDocument();
    expect(badge.className).not.toContain("bg-green-100");
    expect(badge.className).not.toContain("bg-red-100");
    expect(badge.className).not.toContain("bg-yellow-100");
  });
});
