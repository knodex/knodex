// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { Box, Search, FolderOpen } from "@/lib/icons";
import { MemoryRouter } from "react-router-dom";
import { EmptyState } from "./empty-state";
import { EmptyState as InstancesEmptyState } from "@/components/instances/EmptyState";
import { EmptyState as CatalogEmptyState } from "@/components/catalog/EmptyState";
import { ProjectList } from "@/components/projects/ProjectList";

describe("EmptyState", () => {
  it("renders icon, title, and description", () => {
    render(
      <EmptyState
        icon={Box}
        title="No items found"
        description="There are no items to display."
      />
    );
    expect(screen.getByText("No items found")).toBeInTheDocument();
    expect(
      screen.getByText("There are no items to display.")
    ).toBeInTheDocument();
  });

  it("renders action when provided", () => {
    render(
      <EmptyState
        icon={Box}
        title="Empty"
        description="Nothing here."
        action={<button>Add Item</button>}
      />
    );
    expect(screen.getByRole("button", { name: "Add Item" })).toBeInTheDocument();
  });

  it("does not render action slot when omitted", () => {
    const { container } = render(
      <EmptyState icon={Box} title="Empty" description="Nothing here." />
    );
    // The action wrapper div (mt-4) should not be present
    const actionDiv = container.querySelector(".mt-4");
    expect(actionDiv).not.toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = render(
      <EmptyState
        icon={Box}
        title="Custom"
        description="With class."
        className="my-custom-class"
      />
    );
    const root = container.firstElementChild;
    expect(root).toHaveClass("my-custom-class");
    expect(root).toHaveClass("flex", "flex-col", "items-center");
  });

  it("uses design tokens for text colors", () => {
    render(
      <EmptyState icon={Box} title="Token Test" description="Token desc." />
    );
    const title = screen.getByText("Token Test");
    expect(title).toHaveStyle({ color: "var(--text-primary)" });

    const description = screen.getByText("Token desc.");
    expect(description).toHaveStyle({ color: "var(--text-secondary)" });
  });

  it("renders icon inside 48px container with muted color", () => {
    const { container } = render(
      <EmptyState icon={Box} title="Icon Test" description="Desc." />
    );
    const iconContainer = container.querySelector(".h-12.w-12");
    expect(iconContainer).toBeInTheDocument();
    expect(iconContainer).toHaveClass("rounded-full", "bg-secondary");

    const svg = iconContainer?.querySelector("svg");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("h-6", "w-6");
    expect(svg).toHaveStyle({ color: "var(--text-muted)" });
  });

  it("renders instances no-data variant correctly", () => {
    render(
      <MemoryRouter>
        <EmptyState
          icon={Box}
          title="No instances running yet"
          description="Deploy an RGD from the catalog to create your first instance."
          action={<a href="/catalog">Browse Catalog</a>}
        />
      </MemoryRouter>
    );
    expect(screen.getByText("No instances running yet")).toBeInTheDocument();
    expect(screen.getByText("Browse Catalog")).toBeInTheDocument();
  });

  it("renders catalog no-data variant correctly", () => {
    render(
      <EmptyState
        icon={Box}
        title="No RGDs found"
        description="ResourceGraphDefinitions will appear here once they are created."
      />
    );
    expect(screen.getByText("No RGDs found")).toBeInTheDocument();
    expect(
      screen.getByText(
        "ResourceGraphDefinitions will appear here once they are created."
      )
    ).toBeInTheDocument();
  });

  it("renders projects no-data variant correctly", () => {
    render(
      <EmptyState
        icon={FolderOpen}
        title="No projects yet"
        description="Projects define RBAC boundaries for your deployments."
        action={<button>Create Project</button>}
      />
    );
    expect(screen.getByText("No projects yet")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Create Project" })
    ).toBeInTheDocument();
  });

  it("renders search/filter variant correctly", () => {
    render(
      <EmptyState
        icon={Search}
        title="No matching instances"
        description="Try adjusting your filters or search query."
      />
    );
    expect(screen.getByText("No matching instances")).toBeInTheDocument();
    expect(
      screen.getByText("Try adjusting your filters or search query.")
    ).toBeInTheDocument();
  });
});

// Wrapper component tests — verify actual feature wrappers delegate correctly
describe("Instances EmptyState wrapper", () => {
  it("renders no-data variant with CTA link to catalog", () => {
    render(
      <MemoryRouter>
        <InstancesEmptyState hasFilters={false} />
      </MemoryRouter>
    );
    expect(screen.getByText("No instances running yet")).toBeInTheDocument();
    expect(screen.getByText("Browse Catalog")).toBeInTheDocument();
  });

  it("renders filter variant with clear filters action", () => {
    const onClear = vi.fn();
    render(
      <InstancesEmptyState hasFilters={true} onClearFilters={onClear} />
    );
    expect(screen.getByText("No matching instances")).toBeInTheDocument();
    expect(screen.getByText("Clear filters")).toBeInTheDocument();
  });
});

describe("Catalog EmptyState wrapper", () => {
  it("renders no-data variant with project access link", () => {
    render(
      <MemoryRouter>
        <CatalogEmptyState hasFilters={false} />
      </MemoryRouter>
    );
    expect(screen.getByText("No RGDs found")).toBeInTheDocument();
    const link = screen.getByText("Check your project access");
    expect(link).toBeInTheDocument();
    expect(link.closest("a")).toHaveAttribute("href", "/projects");
  });

  it("renders filter variant with clear filters action", () => {
    const onClear = vi.fn();
    render(
      <CatalogEmptyState hasFilters={true} onClearFilters={onClear} />
    );
    expect(screen.getByText("No results found")).toBeInTheDocument();
    expect(screen.getByText("Clear filters")).toBeInTheDocument();
  });
});

describe("ProjectList empty states", () => {
  it("renders empty state when no projects exist", () => {
    render(<ProjectList projects={[]} />);
    // ProjectList renders its own inline empty state
    expect(screen.getByText("No projects yet")).toBeInTheDocument();
  });

  it("renders Create button when canManage and onCreate provided", () => {
    const onCreate = vi.fn();
    render(<ProjectList projects={[]} canManage={true} onCreate={onCreate} />);
    expect(screen.getByText("Create Project")).toBeInTheDocument();
  });

  it("does not render Create button when canManage is false", () => {
    render(<ProjectList projects={[]} canManage={false} />);
    expect(screen.queryByText("Create")).not.toBeInTheDocument();
  });
});
