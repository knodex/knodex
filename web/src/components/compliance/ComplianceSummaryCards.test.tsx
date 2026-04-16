// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { ComplianceSummaryCards } from "./ComplianceSummaryCards";
import type { ComplianceSummary } from "@/types/compliance";

function renderWithRouter(component: React.ReactNode) {
  return render(<BrowserRouter>{component}</BrowserRouter>);
}

describe("ComplianceSummaryCards", () => {
  const mockSummary: ComplianceSummary = {
    totalTemplates: 10,
    totalConstraints: 25,
    totalViolations: 7,
    byEnforcement: {
      deny: 3,
      warn: 2,
      dryrun: 2,
    },
  };

  it("renders all three summary cards", () => {
    renderWithRouter(<ComplianceSummaryCards summary={mockSummary} />);

    expect(screen.getByText("Policy Templates")).toBeInTheDocument();
    expect(screen.getByText("Active Constraints")).toBeInTheDocument();
    expect(screen.getByText("Total Violations")).toBeInTheDocument();
  });

  it("displays correct values from summary", () => {
    renderWithRouter(<ComplianceSummaryCards summary={mockSummary} />);

    expect(screen.getByText("10")).toBeInTheDocument(); // templates
    expect(screen.getByText("25")).toBeInTheDocument(); // constraints
    expect(screen.getByText("7")).toBeInTheDocument(); // violations
  });

  it("displays enforcement breakdown inside violations card", () => {
    renderWithRouter(<ComplianceSummaryCards summary={mockSummary} />);

    // Enforcement badges render as "Label Count" text within the violations card
    expect(screen.getByText("Deny 3")).toBeInTheDocument();
    expect(screen.getByText("Warn 2")).toBeInTheDocument();
    expect(screen.getByText("Dry Run 2")).toBeInTheDocument();
  });

  it("shows loading skeletons when isLoading is true", () => {
    renderWithRouter(<ComplianceSummaryCards isLoading={true} />);

    // Should show card titles even when loading
    expect(screen.getByText("Policy Templates")).toBeInTheDocument();
    expect(screen.getByText("Active Constraints")).toBeInTheDocument();
    expect(screen.getByText("Total Violations")).toBeInTheDocument();
  });

  it("displays zero values correctly", () => {
    const emptySummary: ComplianceSummary = {
      totalTemplates: 0,
      totalConstraints: 0,
      totalViolations: 0,
      byEnforcement: {
        deny: 0,
        warn: 0,
        dryrun: 0,
      },
    };

    renderWithRouter(<ComplianceSummaryCards summary={emptySummary} />);

    // All values should be 0
    const zeros = screen.getAllByText("0");
    expect(zeros.length).toBeGreaterThan(0);
  });

  it("links to correct routes", () => {
    renderWithRouter(<ComplianceSummaryCards summary={mockSummary} />);

    const templatesLink = screen.getByRole("link", { name: /policy templates/i });
    expect(templatesLink).toHaveAttribute("href", "/compliance/templates");

    const constraintsLink = screen.getByRole("link", { name: /active constraints/i });
    expect(constraintsLink).toHaveAttribute("href", "/compliance/constraints");

    const violationsLink = screen.getByRole("link", { name: /total violations/i });
    expect(violationsLink).toHaveAttribute("href", "/compliance/violations");
  });

  it("shows subtitles for first two cards", () => {
    renderWithRouter(<ComplianceSummaryCards summary={mockSummary} />);

    expect(screen.getByText("ConstraintTemplates")).toBeInTheDocument();
    expect(screen.getByText("Enforcing policies")).toBeInTheDocument();
  });

  it("formats large numbers with locale", () => {
    const largeSummary: ComplianceSummary = {
      totalTemplates: 1000,
      totalConstraints: 2500,
      totalViolations: 10000,
      byEnforcement: {
        deny: 5000,
        warn: 3000,
        dryrun: 2000,
      },
    };

    renderWithRouter(<ComplianceSummaryCards summary={largeSummary} />);

    // Should format with commas for US locale
    expect(screen.getByText("1,000")).toBeInTheDocument();
    expect(screen.getByText("2,500")).toBeInTheDocument();
    expect(screen.getByText("10,000")).toBeInTheDocument();
  });
});
