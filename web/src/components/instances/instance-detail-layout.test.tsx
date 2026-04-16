// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { InstanceDetailLayout, Panel } from "./instance-detail-layout";
import { InstanceDetailSkeleton } from "./instance-detail-skeleton";

describe("Panel", () => {
  it("renders title and children", () => {
    render(<Panel title="Status">Content here</Panel>);
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Content here")).toBeInTheDocument();
  });
});

describe("InstanceDetailLayout", () => {
  it("renders three panels", () => {
    render(
      <InstanceDetailLayout
        statusPanel={<div>Status</div>}
        resourceTreePanel={<div>Tree</div>}
        configPanel={<div>Config</div>}
      />
    );
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("Tree")).toBeInTheDocument();
    expect(screen.getByText("Config")).toBeInTheDocument();
  });

  it("has data-testid", () => {
    render(
      <InstanceDetailLayout
        statusPanel={<div />}
        resourceTreePanel={<div />}
        configPanel={<div />}
      />
    );
    expect(screen.getByTestId("instance-detail-layout")).toBeInTheDocument();
  });
});

describe("InstanceDetailSkeleton", () => {
  it("renders skeleton panels", () => {
    render(<InstanceDetailSkeleton />);
    expect(screen.getByTestId("instance-detail-skeleton")).toBeInTheDocument();
  });

  it("uses animate-token-shimmer", () => {
    const { container } = render(<InstanceDetailSkeleton />);
    const shimmerElements = container.querySelectorAll(".animate-token-shimmer");
    expect(shimmerElements.length).toBeGreaterThan(0);
  });
});
