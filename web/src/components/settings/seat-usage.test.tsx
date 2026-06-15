// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { SeatUsageWidget } from "./seat-usage";
import type { SeatUsage } from "@/types/license";

function makeSeats(overrides: Partial<SeatUsage> = {}): SeatUsage {
  return {
    used: 3,
    allowed: 10,
    windowDays: 30,
    percent: 0.3,
    threshold: "ok",
    lastUpdated: "2026-06-01T10:00:00Z",
    advisoryOnly: false,
    ...overrides,
  };
}

describe("SeatUsageWidget", () => {
  it("renders {used} / {allowed} (AC #1)", () => {
    render(<SeatUsageWidget seats={makeSeats()} maxUsers={10} />);
    expect(screen.getByTestId("users-seat-usage")).toHaveTextContent("3 / 10");
  });

  it("shows 'calculating…' on the cold-start sentinel, never a misleading 0 (AC #1)", () => {
    render(
      <SeatUsageWidget seats={makeSeats({ lastUpdated: "" })} maxUsers={10} />,
    );
    const widget = screen.getByTestId("users-seat-usage");
    expect(widget).toHaveTextContent("calculating…");
    expect(widget).not.toHaveTextContent("0 / 10");
    expect(widget).toHaveAttribute("data-threshold", "calculating");
  });

  it("shows advisory-only copy when advisoryOnly (AC #1)", () => {
    render(
      <SeatUsageWidget seats={makeSeats({ advisoryOnly: true })} maxUsers={10} />,
    );
    expect(screen.getByTestId("users-seat-usage")).toHaveTextContent(
      /informational/i,
    );
  });

  it("renders nothing (null) when seats is undefined — OSS silent (AC #1)", () => {
    const { container } = render(
      <SeatUsageWidget seats={undefined} maxUsers={10} />,
    );
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByTestId("users-seat-usage")).not.toBeInTheDocument();
  });

  it("renders unlimited when allowed === 0 (AC #1)", () => {
    render(<SeatUsageWidget seats={makeSeats({ allowed: 0 })} maxUsers={0} />);
    expect(screen.getByTestId("users-seat-usage")).toHaveTextContent(
      "3 (Unlimited)",
    );
  });
});
