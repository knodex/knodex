// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { InstanceStatusBanner } from "./InstanceStatusBanner";
import type { InstanceHealth } from "@/types/rgd";

describe("InstanceStatusBanner", () => {
  it("renders nothing for Healthy state", () => {
    const { container } = render(<InstanceStatusBanner health="Healthy" />);
    expect(container.innerHTML).toBe("");
  });

  it("renders Progressing state with correct message", () => {
    render(<InstanceStatusBanner health="Progressing" />);
    expect(screen.getByText("This instance is being provisioned or updated.")).toBeInTheDocument();
  });

  it("renders Degraded state with correct message", () => {
    render(<InstanceStatusBanner health="Degraded" />);
    expect(screen.getByText("This instance is experiencing degraded performance.")).toBeInTheDocument();
  });

  it("renders Unhealthy state with correct message", () => {
    render(<InstanceStatusBanner health="Unhealthy" />);
    expect(screen.getByText("This instance is unhealthy and may require attention.")).toBeInTheDocument();
  });

  it("renders Unknown state with correct message", () => {
    render(<InstanceStatusBanner health="Unknown" />);
    expect(screen.getByText("The status of this instance is unknown.")).toBeInTheDocument();
  });

  it("overrides message when state is DELETING", () => {
    render(<InstanceStatusBanner health="Healthy" state="DELETING" />);
    expect(screen.getByText("This instance is being deleted.")).toBeInTheDocument();
    expect(screen.queryByText(/healthy/i)).not.toBeInTheDocument();
  });

  it("falls back to Unknown config for unrecognized health values", () => {
    render(<InstanceStatusBanner health={"Bogus" as InstanceHealth} />);
    expect(screen.getByText("The status of this instance is unknown.")).toBeInTheDocument();
  });

  it("has role=status for accessibility", () => {
    render(<InstanceStatusBanner health="Degraded" />);
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("applies animate-spin to Progressing icon but not when DELETING", () => {
    const { container, rerender } = render(<InstanceStatusBanner health="Progressing" />);
    const icon = container.querySelector("svg");
    expect(icon?.classList.contains("animate-spin")).toBe(true);

    rerender(<InstanceStatusBanner health="Progressing" state="DELETING" />);
    const iconAfter = container.querySelector("svg");
    expect(iconAfter?.classList.contains("animate-spin")).toBe(false);
  });
});
