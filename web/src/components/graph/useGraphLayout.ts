// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import type { Edge } from "@xyflow/react";
import { MarkerType } from "@xyflow/react";
import type { ResourceNode, ResourceEdge } from "@/types/rgd";

// Layout constants shared across graph views
export const NODE_WIDTH = 200;
export const NODE_HEIGHT_REGULAR = 100;
export const NODE_HEIGHT_COLLECTION = 120;
export const HORIZONTAL_GAP = 100;
export const VERTICAL_GAP = 40;

export interface GraphLayoutInput {
  resources: ResourceNode[];
  edges: ResourceEdge[];
  topologicalOrder?: string[];
}

export interface LayoutNodePosition {
  id: string;
  x: number;
  y: number;
  isCollection: boolean;
}

export interface GraphLayoutResult {
  positions: LayoutNodePosition[];
  flowEdges: Edge[];
}

/**
 * Builds levels from a server-provided topological order.
 * Nodes are assigned to levels based on their dependencies:
 * a node goes in the earliest level where all its dependencies are in prior levels.
 */
function buildLevelsFromOrder(
  order: string[],
  resources: ResourceNode[],
): string[][] {
  const resourceMap = new Map(resources.map((r) => [r.id, r]));
  const nodeLevel = new Map<string, number>();
  const levels: string[][] = [];

  for (const nodeId of order) {
    const resource = resourceMap.get(nodeId);
    if (!resource) continue;

    let level = 0;
    for (const dep of resource.dependsOn) {
      const depLevel = nodeLevel.get(dep);
      if (depLevel !== undefined) {
        level = Math.max(level, depLevel + 1);
      }
    }

    nodeLevel.set(nodeId, level);
    while (levels.length <= level) {
      levels.push([]);
    }
    levels[level].push(nodeId);
  }

  // Add any resources not in the order (shouldn't happen, but be safe)
  for (const res of resources) {
    if (!nodeLevel.has(res.id)) {
      if (levels.length === 0) levels.push([]);
      levels[levels.length - 1].push(res.id);
    }
  }

  return levels;
}

/**
 * Computes topological levels from resource dependencies (client-side fallback).
 * Used when the server does not provide a topologicalOrder.
 */
function computeTopologicalLevels(resources: ResourceNode[]): string[][] {
  const assigned = new Set<string>();
  const queue: string[] = [];

  resources.forEach((res) => {
    if (res.dependsOn.length === 0) {
      queue.push(res.id);
    }
  });

  const levels: string[][] = [];

  while (queue.length > 0 || assigned.size < resources.length) {
    const currentLevel: string[] = [];

    while (queue.length > 0) {
      const nodeId = queue.shift()!;
      if (!assigned.has(nodeId)) {
        currentLevel.push(nodeId);
        assigned.add(nodeId);
      }
    }

    if (currentLevel.length > 0) {
      levels.push(currentLevel);
    }

    resources.forEach((res) => {
      if (!assigned.has(res.id)) {
        const allDepsAssigned = res.dependsOn.every((dep) => assigned.has(dep));
        if (allDepsAssigned) {
          queue.push(res.id);
        }
      }
    });

    // Break infinite loops (circular dependencies)
    if (queue.length === 0 && assigned.size < resources.length) {
      const unassigned = resources.find((res) => !assigned.has(res.id));
      if (unassigned) {
        queue.push(unassigned.id);
      }
    }
  }

  return levels;
}

/**
 * Computes topological sort layout positions for a resource graph.
 * When topologicalOrder is provided (from server), uses it directly for level assignment.
 * Otherwise falls back to client-side topological sort.
 */
export function computeGraphLayout(input: GraphLayoutInput): GraphLayoutResult {
  const { resources, edges, topologicalOrder } = input;

  const positions: LayoutNodePosition[] = [];
  const flowEdges: Edge[] = [];

  if (resources.length === 0) {
    return { positions, flowEdges };
  }

  // Build levels from topological order (server-provided or computed locally).
  // Each level contains nodes whose dependencies are all in earlier levels.
  const levels = topologicalOrder?.length
    ? buildLevelsFromOrder(topologicalOrder, resources)
    : computeTopologicalLevels(resources);

  // Position nodes by level
  levels.forEach((levelNodes, levelIndex) => {
    const maxHeight = levelNodes.some((id) => {
      const r = resources.find((res) => res.id === id);
      return r?.isCollection;
    })
      ? NODE_HEIGHT_COLLECTION
      : NODE_HEIGHT_REGULAR;

    const startY = -((levelNodes.length - 1) * (maxHeight + VERTICAL_GAP)) / 2;

    levelNodes.forEach((nodeId, nodeIndex) => {
      const resource = resources.find((r) => r.id === nodeId);
      if (!resource) return;

      positions.push({
        id: resource.id,
        x: levelIndex * (NODE_WIDTH + HORIZONTAL_GAP),
        y: startY + nodeIndex * (maxHeight + VERTICAL_GAP),
        isCollection: resource.isCollection === true,
      });
    });
  });

  // Create flow edges
  edges.forEach((edge, index) => {
    flowEdges.push({
      id: `edge-${index}`,
      source: edge.to,
      target: edge.from,
      type: "smoothstep",
      animated: true,
      style: {
        stroke: "hsl(var(--graph-edge))",
        strokeWidth: 2,
      },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: "hsl(var(--graph-edge))",
      },
      label: edge.type !== "reference" ? edge.type : undefined,
      labelStyle: {
        fontSize: 10,
        fill: "hsl(var(--muted-foreground))",
      },
    });
  });

  return { positions, flowEdges };
}

/**
 * React hook wrapping computeGraphLayout with memoization.
 */
export function useGraphLayout(input: GraphLayoutInput): GraphLayoutResult {
  return useMemo(() => computeGraphLayout(input), [input]);
}
