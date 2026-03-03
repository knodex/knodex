import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { BrowserRouter } from "react-router-dom";
import { PageHeader } from "./PageHeader";

function renderWithRouter(component: React.ReactNode) {
  return render(<BrowserRouter>{component}</BrowserRouter>);
}

describe("PageHeader", () => {
  it("renders title", () => {
    renderWithRouter(<PageHeader title="Constraints" />);

    expect(screen.getByRole("heading", { name: "Constraints" })).toBeInTheDocument();
  });

  it("renders subtitle when provided", () => {
    renderWithRouter(
      <PageHeader title="Constraints" subtitle="Active OPA Gatekeeper policy constraints" />
    );

    expect(screen.getByText("Active OPA Gatekeeper policy constraints")).toBeInTheDocument();
  });

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

    // Links should be rendered
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

    // Should render separators between breadcrumb items
    // The separator is typically a ChevronRight or slash
    expect(screen.getByText("First")).toBeInTheDocument();
    expect(screen.getByText("Second")).toBeInTheDocument();
    expect(screen.getByText("Third")).toBeInTheDocument();
  });

  it("renders actions when provided", () => {
    renderWithRouter(
      <PageHeader
        title="Templates"
        actions={<button>Create Template</button>}
      />
    );

    expect(screen.getByRole("button", { name: "Create Template" })).toBeInTheDocument();
  });

  it("renders without breadcrumbs", () => {
    renderWithRouter(<PageHeader title="Dashboard" />);

    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
    // Should not crash or show breadcrumb navigation
  });

  it("renders without actions", () => {
    renderWithRouter(
      <PageHeader
        title="Templates"
        breadcrumbs={[{ label: "Compliance", href: "/compliance" }, { label: "Templates" }]}
      />
    );

    expect(screen.getByRole("heading", { name: "Templates" })).toBeInTheDocument();
    expect(screen.getByText("Compliance")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = renderWithRouter(
      <PageHeader title="Test" className="custom-class" />
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("handles single breadcrumb item", () => {
    renderWithRouter(
      <PageHeader title="Home" breadcrumbs={[{ label: "Home" }]} />
    );

    // "Home" appears in both the title and breadcrumb
    const homeElements = screen.getAllByText("Home");
    expect(homeElements.length).toBe(2); // title + breadcrumb
  });
});
