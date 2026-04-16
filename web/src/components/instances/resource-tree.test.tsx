// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResourceTree } from "./resource-tree";
import type { Instance, ResourceGraph } from "@/types/rgd";

// React Flow requires ResizeObserver and IntersectionObserver
beforeAll(() => {
  globalThis.ResizeObserver = class {
    observe = vi.fn();
    unobserve = vi.fn();
    disconnect = vi.fn();
  } as unknown as typeof ResizeObserver;
  Element.prototype.scrollIntoView = vi.fn();
});

const mockInstance: Instance = {
  name: "my-instance",
  namespace: "default",
  rgdName: "test-rgd",
  rgdNamespace: "default",
  apiVersion: "v1",
  kind: "MyDB",
  health: "Healthy",
  conditions: [],
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

const mockGraph: ResourceGraph = {
  rgdName: "test-rgd",
  rgdNamespace: "default",
  resources: [
    { id: "deployment", apiVersion: "apps/v1", kind: "Deployment", isTemplate: true, isConditional: false, dependsOn: [] },
    { id: "service", apiVersion: "v1", kind: "Service", isTemplate: true, isConditional: false, dependsOn: ["deployment"] },
  ],
  edges: [{ from: "deployment", to: "service", type: "dependency" }],
};

describe("ResourceTree", () => {
  it("renders empty state when no graph", () => {
    render(<ResourceTree instance={mockInstance} />);
    expect(screen.getByText("No resource graph available")).toBeInTheDocument();
  });

  it("renders data-testid when graph provided", () => {
    render(<ResourceTree instance={mockInstance} resourceGraph={mockGraph} />);
    expect(screen.getByTestId("resource-tree")).toBeInTheDocument();
  });

  it("renders node names from graph", () => {
    render(<ResourceTree instance={mockInstance} resourceGraph={mockGraph} />);
    expect(screen.getByText("deployment")).toBeInTheDocument();
    expect(screen.getByText("service")).toBeInTheDocument();
  });
});
