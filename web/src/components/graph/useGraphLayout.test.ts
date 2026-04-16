// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { computeGraphLayout } from "./useGraphLayout";
import type { ResourceNode, ResourceEdge } from "@/types/rgd";

function makeResource(id: string, dependsOn: string[] = [], isCollection = false): ResourceNode {
  return {
    id,
    apiVersion: "apps/v1",
    kind: "Deployment",
    isTemplate: true,
    isConditional: false,
    dependsOn,
    isCollection,
  };
}

describe("computeGraphLayout", () => {
  it("returns empty positions and edges for empty graph", () => {
    const result = computeGraphLayout({ resources: [], edges: [] });
    expect(result.positions).toHaveLength(0);
    expect(result.flowEdges).toHaveLength(0);
  });

  it("produces valid positions for a single node", () => {
    const result = computeGraphLayout({
      resources: [makeResource("a")],
      edges: [],
    });
    expect(result.positions).toHaveLength(1);
    expect(result.positions[0].id).toBe("a");
    expect(typeof result.positions[0].x).toBe("number");
    expect(typeof result.positions[0].y).toBe("number");
  });

  it("assigns different x positions to nodes at different levels", () => {
    // b depends on a, so a is level 0, b is level 1
    const result = computeGraphLayout({
      resources: [makeResource("a"), makeResource("b", ["a"])],
      edges: [{ from: "b", to: "a", type: "reference" }],
    });
    const posA = result.positions.find((p) => p.id === "a")!;
    const posB = result.positions.find((p) => p.id === "b")!;
    expect(posB.x).toBeGreaterThan(posA.x);
  });

  it("handles circular dependencies without infinite loop", () => {
    // a → b → a (circular)
    const result = computeGraphLayout({
      resources: [makeResource("a", ["b"]), makeResource("b", ["a"])],
      edges: [
        { from: "a", to: "b", type: "reference" },
        { from: "b", to: "a", type: "reference" },
      ],
    });
    // Should not hang; all nodes should be assigned
    expect(result.positions).toHaveLength(2);
  });

  it("creates flow edges with smoothstep type", () => {
    const edges: ResourceEdge[] = [{ from: "b", to: "a", type: "reference" }];
    const result = computeGraphLayout({
      resources: [makeResource("a"), makeResource("b", ["a"])],
      edges,
    });
    expect(result.flowEdges).toHaveLength(1);
    expect(result.flowEdges[0].type).toBe("smoothstep");
  });

  it("assigns all resources a position", () => {
    const resources = [
      makeResource("a"),
      makeResource("b", ["a"]),
      makeResource("c", ["a"]),
      makeResource("d", ["b", "c"]),
    ];
    const result = computeGraphLayout({ resources, edges: [] });
    expect(result.positions).toHaveLength(4);
    const ids = result.positions.map((p) => p.id);
    expect(ids).toContain("a");
    expect(ids).toContain("b");
    expect(ids).toContain("c");
    expect(ids).toContain("d");
  });

  it("marks collection nodes correctly", () => {
    const result = computeGraphLayout({
      resources: [makeResource("col", [], true)],
      edges: [],
    });
    expect(result.positions[0].isCollection).toBe(true);
  });

  it("uses server-provided topologicalOrder when available", () => {
    const resources = [
      makeResource("a"),
      makeResource("b", ["a"]),
      makeResource("c", ["a"]),
      makeResource("d", ["b", "c"]),
    ];
    // Server provides order: a, b, c, d
    const result = computeGraphLayout({
      resources,
      edges: [],
      topologicalOrder: ["a", "b", "c", "d"],
    });
    expect(result.positions).toHaveLength(4);
    // a should be at level 0, b and c at level 1, d at level 2
    const posA = result.positions.find((p) => p.id === "a")!;
    const posB = result.positions.find((p) => p.id === "b")!;
    const posD = result.positions.find((p) => p.id === "d")!;
    expect(posA.x).toBeLessThan(posB.x);
    expect(posB.x).toBeLessThan(posD.x);
  });
});
