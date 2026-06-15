// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SubTitle } from "./SubTitle";

describe("SubTitle", () => {
  it("renders the title as a heading", () => {
    render(<SubTitle title="General" />);
    expect(screen.getByRole("heading", { name: "General" })).toBeInTheDocument();
  });

  it("renders the description when given", () => {
    render(<SubTitle title="General" description="Platform configuration" />);
    expect(screen.getByText("Platform configuration")).toBeInTheDocument();
  });

  it("omits the description when not given", () => {
    render(<SubTitle title="General" />);
    expect(screen.queryByText("Platform configuration")).not.toBeInTheDocument();
  });

  it("renders the action node in the action slot", () => {
    render(<SubTitle title="SSO Providers" action={<button>Add Provider</button>} />);
    expect(screen.getByRole("button", { name: "Add Provider" })).toBeInTheDocument();
  });

  it("omits the action slot when no action is given", () => {
    const { container } = render(<SubTitle title="General" />);
    expect(container.querySelector("button")).toBeNull();
  });
});
