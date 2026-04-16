// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { Announcer, type Announcement } from "./Announcer";

function makeAnnouncement(
  overrides: Partial<Announcement> = {}
): Announcement {
  return {
    id: "test-1",
    message: "Instance web-app is now Healthy",
    priority: "polite",
    timestamp: Date.now(),
    ...overrides,
  };
}

describe("Announcer", () => {
  it("renders polite and assertive aria-live regions", () => {
    render(<Announcer announcements={[]} />);

    const polite = screen.getByRole("status");
    expect(polite).toHaveAttribute("aria-live", "polite");
    expect(polite).toHaveAttribute("aria-atomic", "true");

    const assertive = screen.getByRole("alert");
    expect(assertive).toHaveAttribute("aria-live", "assertive");
    expect(assertive).toHaveAttribute("aria-atomic", "true");
  });

  it("announces polite messages in the polite region", () => {
    const announcement = makeAnnouncement({ priority: "polite" });
    render(<Announcer announcements={[announcement]} />);

    const polite = screen.getByRole("status");
    expect(polite).toHaveTextContent("Instance web-app is now Healthy");
  });

  it("announces assertive messages in the assertive region", () => {
    const announcement = makeAnnouncement({
      id: "err-1",
      message: "Connection lost",
      priority: "assertive",
    });
    render(<Announcer announcements={[announcement]} />);

    const assertive = screen.getByRole("alert");
    expect(assertive).toHaveTextContent("Connection lost");
  });

  it("clears announcement after 3 seconds", async () => {
    vi.useFakeTimers();
    const onRead = vi.fn();
    const announcement = makeAnnouncement();

    act(() => {
      render(
        <Announcer announcements={[announcement]} onAnnouncementRead={onRead} />
      );
    });

    expect(screen.getByRole("status")).toHaveTextContent(
      "Instance web-app is now Healthy"
    );

    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(screen.getByRole("status")).toHaveTextContent("");
    expect(onRead).toHaveBeenCalledWith("test-1");

    vi.useRealTimers();
  });

  it("both aria-live regions are visually hidden (sr-only)", () => {
    render(<Announcer announcements={[]} />);

    const polite = screen.getByRole("status");
    const assertive = screen.getByRole("alert");

    expect(polite.className).toContain("sr-only");
    expect(assertive.className).toContain("sr-only");
  });
});
