// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ViolationsByEnforcement } from "./ViolationsByEnforcement";
import type { ComplianceSummary } from "@/types/compliance";

describe("ViolationsByEnforcement", () => {
  const mockSummary: ComplianceSummary = {
    totalTemplates: 10,
    totalConstraints: 25,
    totalViolations: 100,
    byEnforcement: {
      deny: 50,
      warn: 30,
      dryrun: 20,
    },
  };

  it("renders the card with title and description", () => {
    render(<ViolationsByEnforcement summary={mockSummary} />);

    expect(screen.getByText("Violations by Enforcement")).toBeInTheDocument();
    expect(
      screen.getByText("Distribution of violations by enforcement action")
    ).toBeInTheDocument();
  });

  it("displays all enforcement types with counts", () => {
    render(<ViolationsByEnforcement summary={mockSummary} />);

    expect(screen.getByText("Deny")).toBeInTheDocument();
    expect(screen.getByText("Warn")).toBeInTheDocument();
    expect(screen.getByText("Dry Run")).toBeInTheDocument();

    expect(screen.getByText("50")).toBeInTheDocument();
    expect(screen.getByText("30")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
  });

  it("displays correct percentages", () => {
    render(<ViolationsByEnforcement summary={mockSummary} />);

    expect(screen.getByText("(50%)")).toBeInTheDocument();
    expect(screen.getByText("(30%)")).toBeInTheDocument();
    expect(screen.getByText("(20%)")).toBeInTheDocument();
  });

  it("displays total violations", () => {
    render(<ViolationsByEnforcement summary={mockSummary} />);

    expect(screen.getByText("Total Violations")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
  });

  it("shows loading skeletons when isLoading is true", () => {
    render(<ViolationsByEnforcement isLoading={true} />);

    // Title should still be visible
    expect(screen.getByText("Violations by Enforcement")).toBeInTheDocument();
    // Skeletons should be rendered (don't show actual data)
    expect(screen.queryByText("Deny")).not.toBeInTheDocument();
  });

  it("shows empty state when no violations", () => {
    const emptySummary: ComplianceSummary = {
      totalTemplates: 5,
      totalConstraints: 10,
      totalViolations: 0,
      byEnforcement: {},
    };

    render(<ViolationsByEnforcement summary={emptySummary} />);

    expect(screen.getByText("All Clear!")).toBeInTheDocument();
    expect(screen.getByText("No policy violations detected")).toBeInTheDocument();
  });

  it("handles missing enforcement counts gracefully", () => {
    const partialSummary: ComplianceSummary = {
      totalTemplates: 5,
      totalConstraints: 10,
      totalViolations: 10,
      byEnforcement: {
        deny: 10,
        // warn and dryrun are missing
      },
    };

    render(<ViolationsByEnforcement summary={partialSummary} />);

    // Should still render all enforcement types
    expect(screen.getByText("Deny")).toBeInTheDocument();
    expect(screen.getByText("Warn")).toBeInTheDocument();
    expect(screen.getByText("Dry Run")).toBeInTheDocument();

    // Missing counts should default to 0
    // Total is also 10, so there are two "10" values
    expect(screen.getAllByText("10")).toHaveLength(2); // deny + total
    // 0 values for warn and dryrun with (0%)
    expect(screen.getAllByText("0")).toHaveLength(2);
    expect(screen.getAllByText("(0%)")).toHaveLength(2);
  });

  it("formats large numbers with locale", () => {
    const largeSummary: ComplianceSummary = {
      totalTemplates: 10,
      totalConstraints: 25,
      totalViolations: 10000,
      byEnforcement: {
        deny: 5000,
        warn: 3000,
        dryrun: 2000,
      },
    };

    render(<ViolationsByEnforcement summary={largeSummary} />);

    expect(screen.getByText("5,000")).toBeInTheDocument();
    expect(screen.getByText("3,000")).toBeInTheDocument();
    expect(screen.getByText("2,000")).toBeInTheDocument();
    expect(screen.getByText("10,000")).toBeInTheDocument();
  });

  it("handles undefined summary", () => {
    render(<ViolationsByEnforcement />);

    // Should show empty state since total is 0
    expect(screen.getByText("All Clear!")).toBeInTheDocument();
  });
});
