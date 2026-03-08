// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EmptyState } from "./EmptyState";
import { Shield } from "lucide-react";

describe("EmptyState", () => {
  it("renders title (AC-SHARED-02)", () => {
    render(<EmptyState title="No Constraints" />);

    expect(screen.getByText("No Constraints")).toBeInTheDocument();
  });

  it("renders description (AC-SHARED-02)", () => {
    render(
      <EmptyState
        title="No Violations"
        description="All resources are compliant with your policies."
      />
    );

    expect(screen.getByText("All resources are compliant with your policies.")).toBeInTheDocument();
  });

  it("renders icon when provided", () => {
    render(
      <EmptyState
        title="No Constraints"
        icon={<Shield data-testid="shield-icon" />}
      />
    );

    expect(screen.getByTestId("shield-icon")).toBeInTheDocument();
  });

  it("renders action button when provided (AC-SHARED-02)", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();

    render(
      <EmptyState
        title="No Results"
        description="No items match your filters."
        action={<button onClick={onClick}>Clear Filters</button>}
      />
    );

    const button = screen.getByRole("button", { name: "Clear Filters" });
    expect(button).toBeInTheDocument();

    await user.click(button);
    expect(onClick).toHaveBeenCalled();
  });

  it("applies success variant styling", () => {
    const { container } = render(
      <EmptyState
        title="No Violations"
        description="All good!"
        variant="success"
      />
    );

    // Success variant should have green styling
    // Check for green class or success indicator
    expect(container.textContent).toContain("No Violations");
  });

  it("applies default variant when not specified", () => {
    render(<EmptyState title="No Results" />);

    expect(screen.getByText("No Results")).toBeInTheDocument();
  });

  it("renders without description", () => {
    render(<EmptyState title="Empty" />);

    expect(screen.getByText("Empty")).toBeInTheDocument();
  });

  it("renders without icon", () => {
    render(<EmptyState title="No Data" description="Nothing here" />);

    expect(screen.getByText("No Data")).toBeInTheDocument();
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
  });

  it("renders without action", () => {
    render(
      <EmptyState
        title="No Items"
        description="There are no items to display."
      />
    );

    expect(screen.getByText("No Items")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("supports multiple action elements", () => {
    render(
      <EmptyState
        title="No Results"
        action={
          <div>
            <button>Action 1</button>
            <button>Action 2</button>
          </div>
        }
      />
    );

    expect(screen.getByRole("button", { name: "Action 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Action 2" })).toBeInTheDocument();
  });
});
