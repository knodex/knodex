// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { RunStatusBadge } from "./RunStatusBadge";

describe("RunStatusBadge", () => {
  it("maps running → progressing (pulsing dot is the live spinner, UX-DR6)", () => {
    render(<RunStatusBadge status="running" />);

    const badge = screen.getByTestId("run-status-badge");
    expect(badge).toHaveAttribute("data-status", "running");
    expect(screen.getByRole("status")).toHaveAttribute("aria-label", "Status: progressing");
    // The progressing dot carries the pulse animation class.
    expect(badge.querySelector('[data-testid="status-dot"]')?.className).toContain(
      "animate-status-pulse"
    );
  });

  it("maps completed → healthy", () => {
    render(<RunStatusBadge status="completed" />);

    expect(screen.getByTestId("run-status-badge")).toHaveAttribute("data-status", "completed");
    expect(screen.getByRole("status")).toHaveAttribute("aria-label", "Status: healthy");
  });

  it("maps failed → error", () => {
    render(<RunStatusBadge status="failed" />);

    expect(screen.getByTestId("run-status-badge")).toHaveAttribute("data-status", "failed");
    expect(screen.getByRole("status")).toHaveAttribute("aria-label", "Status: error");
    expect(screen.getByText("Failed")).toBeInTheDocument();
  });
});
