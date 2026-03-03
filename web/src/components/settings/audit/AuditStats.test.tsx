import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AuditStats } from "./AuditStats";
import type { AuditStats as AuditStatsType } from "@/types/audit";

const mockStats: AuditStatsType = {
  totalEvents: 12345,
  eventsToday: 42,
  topUsers: [
    { userId: "admin@test.local", count: 20 },
    { userId: "dev@test.local", count: 15 },
    { userId: "viewer@test.local", count: 5 },
  ],
  deniedAttempts: 3,
  byActionToday: { login: 10, create: 20, get: 12 },
  byResultToday: { success: 39, denied: 3 },
};

describe("AuditStats", () => {
  it("renders all four stat cards", () => {
    render(<AuditStats stats={mockStats} />);

    expect(screen.getByText("Total Events")).toBeInTheDocument();
    expect(screen.getByText("Events Today")).toBeInTheDocument();
    expect(screen.getByText("Top Users Today")).toBeInTheDocument();
    expect(screen.getByText("Denied Attempts")).toBeInTheDocument();
  });

  it("displays stat values from data", () => {
    render(<AuditStats stats={mockStats} />);

    expect(screen.getByText("12,345")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("displays top users with counts", () => {
    render(<AuditStats stats={mockStats} />);

    expect(screen.getByText("admin@test.local")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getByText("dev@test.local")).toBeInTheDocument();
    expect(screen.getByText("15")).toBeInTheDocument();
    expect(screen.getByText("viewer@test.local")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("shows max 3 top users", () => {
    const statsWithManyUsers: AuditStatsType = {
      ...mockStats,
      topUsers: [
        { userId: "user1@test.local", count: 50 },
        { userId: "user2@test.local", count: 40 },
        { userId: "user3@test.local", count: 30 },
        { userId: "user4@test.local", count: 20 },
      ],
    };
    render(<AuditStats stats={statsWithManyUsers} />);

    expect(screen.getByText("user1@test.local")).toBeInTheDocument();
    expect(screen.getByText("user2@test.local")).toBeInTheDocument();
    expect(screen.getByText("user3@test.local")).toBeInTheDocument();
    expect(screen.queryByText("user4@test.local")).not.toBeInTheDocument();
  });

  it("shows empty activity message when no top users", () => {
    const statsNoUsers: AuditStatsType = {
      ...mockStats,
      topUsers: [],
    };
    render(<AuditStats stats={statsNoUsers} />);

    expect(screen.getByText("No activity today")).toBeInTheDocument();
  });

  it("renders loading skeletons when isLoading is true", () => {
    const { container } = render(<AuditStats isLoading />);

    // Should have skeleton elements (Skeleton renders divs with specific classes)
    const skeletons = container.querySelectorAll('[class*="animate-pulse"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("shows zero values when stats is undefined", () => {
    render(<AuditStats />);

    expect(screen.getByText("Total Events")).toBeInTheDocument();
    // Values should be 0
    const zeros = screen.getAllByText("0");
    expect(zeros.length).toBeGreaterThanOrEqual(3);
  });

  it("applies danger variant to denied attempts when > 0", () => {
    const { container } = render(<AuditStats stats={mockStats} />);

    // The denied attempts card should have red text
    const deniedValue = screen.getByText("3");
    expect(deniedValue.className).toContain("text-red-");
  });

  it("does not apply danger variant when denied attempts is 0", () => {
    const statsNoDenied: AuditStatsType = {
      ...mockStats,
      deniedAttempts: 0,
    };
    render(<AuditStats stats={statsNoDenied} />);

    // Find the Denied Attempts card's value
    const cards = screen.getAllByText("0");
    // All zero values should use default variant (text-foreground)
    cards.forEach((card) => {
      expect(card.className).not.toContain("text-red-");
    });
  });

  it("renders subtitles", () => {
    render(<AuditStats stats={mockStats} />);

    expect(screen.getByText("Total tracked events")).toBeInTheDocument();
    expect(screen.getByText("Since midnight UTC")).toBeInTheDocument();
    expect(screen.getByText("Access denied today")).toBeInTheDocument();
  });

  it("suppresses error when still loading", () => {
    render(<AuditStats error={new Error("API failure")} isLoading />);

    // Error should NOT be shown while loading
    expect(
      screen.queryByText("Failed to load audit statistics")
    ).not.toBeInTheDocument();
  });

  it("shows error message when error is provided", () => {
    render(<AuditStats error={new Error("API failure")} />);

    expect(
      screen.getByText("Failed to load audit statistics")
    ).toBeInTheDocument();
    // Should NOT render stat cards
    expect(screen.queryByText("Total Events")).not.toBeInTheDocument();
  });
});
