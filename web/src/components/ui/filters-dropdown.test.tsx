// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FiltersDropdown } from "./filters-dropdown";

function setup(activeCount = 0) {
  return render(
    <FiltersDropdown activeCount={activeCount}>
      <div data-testid="chip-panel">
        <button>Status</button>
        <button>Owner</button>
      </div>
    </FiltersDropdown>
  );
}

describe("FiltersDropdown", () => {
  it("renders a trigger button labeled 'Filters' by default", () => {
    setup();
    expect(screen.getByRole("button", { name: "Filters" })).toBeInTheDocument();
  });

  it("does NOT render the active-count badge when activeCount is 0", () => {
    setup(0);
    expect(screen.queryByTestId("filters-active-count")).toBeNull();
  });

  it("renders the active-count badge only when activeCount > 0", () => {
    setup(3);
    const badge = screen.getByTestId("filters-active-count");
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveTextContent("3");
  });

  it("opens the popover on click and renders its children", async () => {
    const user = userEvent.setup();
    setup();
    await user.click(screen.getByRole("button", { name: "Filters" }));
    await waitFor(() => {
      expect(screen.getByTestId("chip-panel")).toBeInTheDocument();
    });
  });

  it("closes the popover on Escape", async () => {
    const user = userEvent.setup();
    setup();
    await user.click(screen.getByRole("button", { name: "Filters" }));
    await waitFor(() => {
      expect(screen.getByTestId("chip-panel")).toBeInTheDocument();
    });
    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByTestId("chip-panel")).toBeNull();
    });
  });

  it("closes the popover on outside click", async () => {
    const user = userEvent.setup();
    render(
      <div>
        <button data-testid="outside">outside</button>
        <FiltersDropdown>
          <div data-testid="chip-panel">
            <button>Status</button>
          </div>
        </FiltersDropdown>
      </div>
    );
    await user.click(screen.getByRole("button", { name: "Filters" }));
    await waitFor(() => {
      expect(screen.getByTestId("chip-panel")).toBeInTheDocument();
    });
    await user.click(screen.getByTestId("outside"));
    await waitFor(() => {
      expect(screen.queryByTestId("chip-panel")).toBeNull();
    });
  });

  it("trigger chevron has the rotate-on-open class wired", () => {
    setup();
    const button = screen.getByRole("button", { name: "Filters" });
    const chevron = button.querySelector("svg");
    expect(chevron).not.toBeNull();
    expect(chevron?.getAttribute("class") || "").toMatch(
      /group-data-\[state=open\]:rotate-180/
    );
  });

  it("uses a custom label when provided", () => {
    render(
      <FiltersDropdown label="More filters">
        <div />
      </FiltersDropdown>
    );
    expect(
      screen.getByRole("button", { name: "More filters" })
    ).toBeInTheDocument();
  });
});
