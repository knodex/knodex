// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { EnforcementBadge } from "./EnforcementBadge";

describe("EnforcementBadge", () => {
  it("renders deny badge with correct text (AC-VIO-05)", () => {
    render(<EnforcementBadge action="deny" />);

    const badge = screen.getByText("deny");
    expect(badge).toBeInTheDocument();
  });

  it("renders warn badge with correct text (AC-VIO-05)", () => {
    render(<EnforcementBadge action="warn" />);

    const badge = screen.getByText("warn");
    expect(badge).toBeInTheDocument();
  });

  it("renders dryrun badge with correct text (AC-VIO-05)", () => {
    render(<EnforcementBadge action="dryrun" />);

    const badge = screen.getByText("dryrun");
    expect(badge).toBeInTheDocument();
  });

  it("applies red styling for deny action (AC-VIO-05)", () => {
    render(<EnforcementBadge action="deny" />);

    const badge = screen.getByText("deny");
    // Should have red color classes
    expect(badge.className).toMatch(/red/i);
  });

  it("applies amber styling for warn action (AC-VIO-05)", () => {
    render(<EnforcementBadge action="warn" />);

    const badge = screen.getByText("warn");
    // Should have amber color classes
    expect(badge.className).toMatch(/amber/i);
  });

  it("applies blue styling for dryrun action (AC-VIO-05)", () => {
    render(<EnforcementBadge action="dryrun" />);

    const badge = screen.getByText("dryrun");
    // Should have blue color classes
    expect(badge.className).toMatch(/blue/i);
  });

  it("handles case-insensitive action values", () => {
    render(<EnforcementBadge action="DENY" />);

    const badge = screen.getByText("DENY");
    expect(badge).toBeInTheDocument();
    // Should still apply deny styling
    expect(badge.className).toMatch(/red/i);
  });

  it("applies custom className", () => {
    render(<EnforcementBadge action="deny" className="custom-class" />);

    const badge = screen.getByText("deny");
    expect(badge).toHaveClass("custom-class");
  });

  it("handles unknown action gracefully", () => {
    render(<EnforcementBadge action="unknown" />);

    const badge = screen.getByText("unknown");
    expect(badge).toBeInTheDocument();
    // Should use default styling (blue/dryrun)
    expect(badge.className).toMatch(/blue/i);
  });
});
