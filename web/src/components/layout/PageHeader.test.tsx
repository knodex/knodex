// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { PageHeader } from "./PageHeader";

function renderWithRouter(component: React.ReactNode) {
  return render(<BrowserRouter>{component}</BrowserRouter>);
}

describe("PageHeader", () => {
  it("renders title", () => {
    renderWithRouter(<PageHeader title="Catalog" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Catalog");
  });

  it("renders description when provided", () => {
    renderWithRouter(<PageHeader title="Settings" description="Manage platform configuration" />);
    expect(screen.getByText("Manage platform configuration")).toBeInTheDocument();
  });

  it("renders subtitle as alias for description", () => {
    renderWithRouter(
      <PageHeader title="Constraints" subtitle="Active OPA Gatekeeper policy constraints" />
    );
    expect(screen.getByText("Active OPA Gatekeeper policy constraints")).toBeInTheDocument();
  });

  it("does not render description when not provided", () => {
    const { container } = renderWithRouter(<PageHeader title="Catalog" />);
    expect(container.querySelector("p")).toBeNull();
  });

  it("accepts count prop without rendering it (title moved to TopBar)", () => {
    renderWithRouter(<PageHeader title="Catalog" count={42} />);
    // Count badge was removed — visible title is now in TopBar, h1 is sr-only
    expect(screen.queryByText("42")).not.toBeInTheDocument();
  });

  it("does not render count badge when not provided", () => {
    const { container } = renderWithRouter(<PageHeader title="Catalog" />);
    expect(container.querySelector(".tabular-nums")).toBeNull();
  });

  it("renders children as actions", () => {
    renderWithRouter(
      <PageHeader title="Instances">
        <button>Filter</button>
      </PageHeader>
    );
    expect(screen.getByRole("button", { name: "Filter" })).toBeInTheDocument();
  });

  it("renders named actions prop", () => {
    renderWithRouter(
      <PageHeader title="Templates" actions={<button>Create Template</button>} />
    );
    expect(screen.getByRole("button", { name: "Create Template" })).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = renderWithRouter(<PageHeader title="Test" className="mb-6" />);
    expect(container.firstChild).toHaveClass("mb-6");
  });

  it("h1 has tabIndex=-1 for programmatic focus", () => {
    renderWithRouter(<PageHeader title="Instances" />);
    const h1 = screen.getByRole("heading", { level: 1 });
    expect(h1).toHaveAttribute("tabindex", "-1");
  });

  describe("breadcrumbs", () => {
    it("renders breadcrumbs with links (AC-NAVL-01)", () => {
      renderWithRouter(
        <PageHeader
          title="Constraint Detail"
          breadcrumbs={[
            { label: "Compliance", href: "/compliance" },
            { label: "Constraints", href: "/compliance/constraints" },
            { label: "require-team-label" },
          ]}
        />
      );

      const complianceLink = screen.getByRole("link", { name: "Compliance" });
      expect(complianceLink).toHaveAttribute("href", "/compliance");

      const constraintsLink = screen.getByRole("link", { name: "Constraints" });
      expect(constraintsLink).toHaveAttribute("href", "/compliance/constraints");

      // Last item should not be a link
      expect(screen.getByText("require-team-label")).toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "require-team-label" })).not.toBeInTheDocument();
    });

    it("renders breadcrumb separators", () => {
      renderWithRouter(
        <PageHeader
          title="Test"
          breadcrumbs={[
            { label: "First", href: "/first" },
            { label: "Second", href: "/second" },
            { label: "Third" },
          ]}
        />
      );

      expect(screen.getByText("First")).toBeInTheDocument();
      expect(screen.getByText("Second")).toBeInTheDocument();
      expect(screen.getByText("Third")).toBeInTheDocument();
    });

    it("renders without breadcrumbs", () => {
      renderWithRouter(<PageHeader title="Dashboard" />);
      expect(screen.getByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
    });

    it("handles single breadcrumb item", () => {
      renderWithRouter(
        <PageHeader title="Home" breadcrumbs={[{ label: "Home" }]} />
      );

      const homeElements = screen.getAllByText("Home");
      expect(homeElements.length).toBe(2); // title + breadcrumb
    });
  });
});
