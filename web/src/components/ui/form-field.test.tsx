// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import * as React from "react";
import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FormField } from "./form-field";

describe("FormField", () => {
  it("renders a label that programmatically associates with the child input", () => {
    render(
      <FormField label="Name" htmlFor="name">
        <input id="name" />
      </FormField>
    );
    const input = screen.getByLabelText("Name");
    expect(input.tagName).toBe("INPUT");
    expect(input.id).toBe("name");
  });

  it("wires aria-describedby to the hint id when hint is set", () => {
    render(
      <FormField label="Name" htmlFor="name" hint="Required for billing">
        <input id="name" />
      </FormField>
    );
    const input = screen.getByLabelText("Name");
    expect(input.getAttribute("aria-describedby")).toBe("name-hint");
    expect(screen.getByText("Required for billing")).toHaveAttribute(
      "id",
      "name-hint"
    );
  });

  it("wires aria-describedby to the error id when error is set", () => {
    render(
      <FormField label="Name" htmlFor="name" error="Must not be empty">
        <input id="name" />
      </FormField>
    );
    const input = screen.getByLabelText("Name");
    expect(input.getAttribute("aria-describedby")).toBe("name-error");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toHaveAttribute("id", "name-error");
  });

  it("error takes precedence over hint when both are set", () => {
    render(
      <FormField
        label="Name"
        htmlFor="name"
        hint="A hint"
        error="An error"
      >
        <input id="name" />
      </FormField>
    );
    const input = screen.getByLabelText("Name");
    expect(input.getAttribute("aria-describedby")).toBe("name-error");
    expect(screen.queryByText("A hint")).toBeNull();
    expect(screen.getByText("An error")).toBeInTheDocument();
  });

  it("renders the required asterisk only when required", () => {
    const { rerender, container } = render(
      <FormField label="Email" htmlFor="email">
        <input id="email" />
      </FormField>
    );
    expect(container.querySelector('[aria-hidden="true"]')).toBeNull();
    rerender(
      <FormField label="Email" htmlFor="email" required>
        <input id="email" />
      </FormField>
    );
    const asterisk = container.querySelector('[aria-hidden="true"]');
    expect(asterisk).not.toBeNull();
    expect(asterisk?.textContent).toBe("*");
  });

  it("preserves existing aria-describedby on the child input", () => {
    render(
      <FormField label="Name" htmlFor="name" hint="hint text">
        <input id="name" aria-describedby="custom-desc" />
      </FormField>
    );
    const input = screen.getByLabelText("Name");
    const desc = input.getAttribute("aria-describedby")!;
    expect(desc).toContain("custom-desc");
    expect(desc).toContain("name-hint");
  });
});

describe("FormField — disabled-submit consumer pattern (contract for 48.5)", () => {
  function MiniForm() {
    const [first, setFirst] = React.useState("");
    const [last, setLast] = React.useState("");
    const canSubmit = first.length > 0 && last.length > 0;
    return (
      <form>
        <FormField label="First" htmlFor="first" required>
          <input
            id="first"
            value={first}
            onChange={(e) => setFirst(e.target.value)}
          />
        </FormField>
        <FormField label="Last" htmlFor="last" required>
          <input
            id="last"
            value={last}
            onChange={(e) => setLast(e.target.value)}
          />
        </FormField>
        <button type="submit" disabled={!canSubmit}>
          Submit
        </button>
      </form>
    );
  }

  it("submit stays disabled until both required fields have ≥1 char", async () => {
    const user = userEvent.setup();
    render(<MiniForm />);
    const submit = screen.getByRole("button", { name: "Submit" });
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/First/), "A");
    expect(submit).toBeDisabled();
    await user.type(screen.getByLabelText(/Last/), "B");
    expect(submit).toBeEnabled();
  });
});
