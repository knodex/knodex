// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { InstancesListView } from "./InstancesListView";
import type { Instance } from "@/types/rgd";

function createTestInstance(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "my-instance",
    namespace: "default",
    rgdName: "my-rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "TestResource",
    health: "Healthy",
    conditions: [],
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    uid: "test-uid",
    ...overrides,
  };
}

const items: Instance[] = [
  createTestInstance({ name: "bravo-instance", kind: "Database", health: "Degraded", updatedAt: "2026-01-02T00:00:00Z" }),
  createTestInstance({ name: "alpha-instance", kind: "Cache", health: "Healthy", updatedAt: "2026-01-03T00:00:00Z" }),
  createTestInstance({ name: "charlie-instance", kind: "App", health: "Unhealthy", updatedAt: "2026-01-01T00:00:00Z" }),
];

describe("InstancesListView", () => {
  it("renders all items in a table", () => {
    render(<InstancesListView items={items} />);
    expect(screen.getByText("alpha-instance")).toBeInTheDocument();
    expect(screen.getByText("bravo-instance")).toBeInTheDocument();
    expect(screen.getByText("charlie-instance")).toBeInTheDocument();
  });

  it("sorts by name ascending by default", () => {
    render(<InstancesListView items={items} />);
    const rows = screen.getAllByRole("button", { name: /View details for/ });
    expect(rows[0]).toHaveAttribute("aria-label", "View details for alpha-instance");
    expect(rows[1]).toHaveAttribute("aria-label", "View details for bravo-instance");
    expect(rows[2]).toHaveAttribute("aria-label", "View details for charlie-instance");
  });

  it("sorts by health using severity order via the Status column", () => {
    render(<InstancesListView items={items} />);
    const statusHeader = screen.getByRole("button", { name: /Status/ });
    fireEvent.click(statusHeader);

    const rows = screen.getAllByRole("button", { name: /View details for/ });
    // ascending health order: Healthy(0) < Degraded(2) < Unhealthy(4)
    expect(rows[0]).toHaveAttribute("aria-label", "View details for alpha-instance");
    expect(rows[1]).toHaveAttribute("aria-label", "View details for bravo-instance");
    expect(rows[2]).toHaveAttribute("aria-label", "View details for charlie-instance");
  });

  it("toggles sort direction on same column click", () => {
    render(<InstancesListView items={items} />);
    const nameHeader = screen.getByRole("button", { name: /^Name$/ });
    fireEvent.click(nameHeader); // descending

    const rows = screen.getAllByRole("button", { name: /View details for/ });
    expect(rows[0]).toHaveAttribute("aria-label", "View details for charlie-instance");
    expect(rows[2]).toHaveAttribute("aria-label", "View details for alpha-instance");
  });

  it("calls onInstanceClick when a row is clicked", () => {
    const onClick = vi.fn();
    render(<InstancesListView items={items} onInstanceClick={onClick} />);
    const row = screen.getAllByRole("button", { name: /View details for/ })[0];
    fireEvent.click(row);
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(onClick).toHaveBeenCalledWith(expect.objectContaining({ name: "alpha-instance" }));
  });

  it("triggers click on Enter key", () => {
    const onClick = vi.fn();
    render(<InstancesListView items={items} onInstanceClick={onClick} />);
    const row = screen.getAllByRole("button", { name: /View details for/ })[0];
    fireEvent.keyDown(row, { key: "Enter" });
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("shows cluster-scoped indicator for cluster-scoped instances", () => {
    const clusterItems = [createTestInstance({ isClusterScoped: true, namespace: "" })];
    render(<InstancesListView items={clusterItems} />);
    expect(screen.getByText("Cluster-Scoped")).toBeInTheDocument();
  });

  it("renders the Status column header", () => {
    render(<InstancesListView items={items} />);
    expect(screen.getByRole("columnheader", { name: /Status/ })).toBeInTheDocument();
  });

  it("renders actor microcopy under the Updated timestamp", () => {
    const gitopsItem = [createTestInstance({ name: "gitops-instance", deploymentMode: "gitops" })];
    render(<InstancesListView items={gitopsItem} />);
    expect(screen.getByText("via GitOps")).toBeInTheDocument();
  });

  it("shows 'manual edit' actor label when instance has drifted", () => {
    const driftedItem = [createTestInstance({ name: "drifted", deploymentMode: "gitops", gitopsDrift: true })];
    render(<InstancesListView items={driftedItem} />);
    expect(screen.getByText("manual edit")).toBeInTheDocument();
  });

  it("sets aria-sort on active column header", () => {
    render(<InstancesListView items={items} />);
    const nameColumnHeader = screen.getByRole("columnheader", { name: /^Name$/ });
    expect(nameColumnHeader).toHaveAttribute("aria-sort", "ascending");
  });

  describe("health stripe (story 48.3 — first-cell box-shadow replaces HealthBadge)", () => {
    it("renders a colored inset stripe on the first cell using the health token", () => {
      render(<InstancesListView items={[createTestInstance({ name: "solo", health: "Healthy" })]} />);
      const cell = screen.getByTestId("instance-row-health-stripe");
      // JSDOM doesn't resolve CSS vars; assert the raw inline-style substring.
      expect(cell.getAttribute("style")).toContain("var(--status-healthy)");
      expect(cell.getAttribute("style")).toContain("inset 3px 0 0");
    });

    it("falls back to the inactive token for Unknown health", () => {
      render(<InstancesListView items={[createTestInstance({ name: "solo", health: "Unknown" })]} />);
      const cell = screen.getByTestId("instance-row-health-stripe");
      expect(cell.getAttribute("style")).toContain("var(--status-inactive)");
    });

    it("exposes the health state via aria-label and renders no badge text in the first cell", () => {
      render(<InstancesListView items={[createTestInstance({ name: "solo", health: "Degraded" })]} />);
      const cell = screen.getByTestId("instance-row-health-stripe");
      expect(cell).toHaveAttribute("aria-label", "Health: Degraded");
      expect(cell).toHaveAttribute("data-health", "Degraded");
      // Stripe is a box-shadow only — the cell carries no visible content.
      expect(cell).toHaveTextContent("");
    });

    it("keeps the Status column sortable after the badge is replaced by the stripe", () => {
      render(<InstancesListView items={items} />);
      const statusHeader = screen.getByRole("button", { name: /Status/ });
      fireEvent.click(statusHeader);
      const rows = screen.getAllByRole("button", { name: /View details for/ });
      // ascending health order: Healthy(0) < Degraded(2) < Unhealthy(4)
      expect(rows[0]).toHaveAttribute("aria-label", "View details for alpha-instance");
      expect(rows[1]).toHaveAttribute("aria-label", "View details for bravo-instance");
      expect(rows[2]).toHaveAttribute("aria-label", "View details for charlie-instance");
    });
  });
});
