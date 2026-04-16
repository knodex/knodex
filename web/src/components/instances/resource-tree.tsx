// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo, useState } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  Handle,
  Position,
} from "@xyflow/react";
import type { Node, Edge, NodeProps } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { cn } from "@/lib/utils";
import type { Instance, ResourceNode as ResourceNodeType, ResourceGraph } from "@/types/rgd";
import { ResourceNodeDetail } from "./resource-node-detail";

// --- Custom Node ---

interface ResourceNodeData {
  kind: string;
  name: string;
  health: "healthy" | "progressing" | "failed" | "unknown";
  [key: string]: unknown;
}

function ResourceTreeNode({ data }: NodeProps<Node<ResourceNodeData>>) {
  const healthColor =
    data.health === "healthy"
      ? "var(--status-healthy)"
      : data.health === "failed"
        ? "var(--status-error)"
        : data.health === "progressing"
          ? "var(--status-info)"
          : "var(--status-inactive)";

  return (
    <div
      className="px-3 py-2 rounded-lg border text-xs"
      style={{
        backgroundColor: "var(--surface-primary)",
        borderColor: "rgba(255,255,255,0.12)",
        minWidth: 140,
      }}
    >
      <Handle type="target" position={Position.Top} className="!bg-transparent !border-0 !w-0 !h-0" />
      <div className="flex items-center gap-2">
        <span
          className="h-2 w-2 rounded-full shrink-0"
          style={{ backgroundColor: healthColor }}
        />
        <div className="min-w-0">
          <div className="font-medium truncate" style={{ color: "var(--text-primary)" }}>
            {data.name}
          </div>
          <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>
            {data.kind}
          </div>
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-transparent !border-0 !w-0 !h-0" />
    </div>
  );
}

const nodeTypes = { resourceNode: ResourceTreeNode };

// --- Layout helper ---

function layoutNodes(resources: ResourceNodeType[]): { nodes: Node<ResourceNodeData>[]; edges: Edge[] } {
  if (!resources || resources.length === 0) {
    return { nodes: [], edges: [] };
  }

  const nodeWidth = 160;
  const nodeHeight = 50;
  const horizontalGap = 40;
  const verticalGap = 80;

  // Build dependency levels
  const levels = new Map<string, number>();
  const visited = new Set<string>();

  function computeLevel(id: string): number {
    if (levels.has(id)) return levels.get(id)!;
    if (visited.has(id)) return 0;
    visited.add(id);

    const node = resources.find((r) => r.id === id);
    if (!node || !node.dependsOn || node.dependsOn.length === 0) {
      levels.set(id, 0);
      return 0;
    }

    const maxParentLevel = Math.max(...node.dependsOn.map((dep) => computeLevel(dep)));
    const level = maxParentLevel + 1;
    levels.set(id, level);
    return level;
  }

  resources.forEach((r) => computeLevel(r.id));

  // Group by level
  const levelGroups = new Map<number, ResourceNodeType[]>();
  resources.forEach((r) => {
    const level = levels.get(r.id) ?? 0;
    if (!levelGroups.has(level)) levelGroups.set(level, []);
    levelGroups.get(level)!.push(r);
  });

  const nodes: Node<ResourceNodeData>[] = [];
  levelGroups.forEach((group, level) => {
    const totalWidth = group.length * nodeWidth + (group.length - 1) * horizontalGap;
    const startX = -totalWidth / 2;

    group.forEach((r, i) => {
      nodes.push({
        id: r.id,
        type: "resourceNode",
        position: {
          x: startX + i * (nodeWidth + horizontalGap),
          y: level * (nodeHeight + verticalGap),
        },
        data: {
          kind: r.kind,
          name: r.id,
          health: r.readyWhen ? "healthy" : "unknown",
        },
      });
    });
  });

  const edges: Edge[] = [];
  resources.forEach((r) => {
    r.dependsOn?.forEach((dep) => {
      edges.push({
        id: `${dep}-${r.id}`,
        source: dep,
        target: r.id,
        style: { stroke: "rgba(255,255,255,0.15)" },
      });
    });
  });

  return { nodes, edges };
}

// --- Component ---

interface ResourceTreeProps {
  instance: Instance;
  resourceGraph?: ResourceGraph;
  className?: string;
}

export function ResourceTree({ instance, resourceGraph, className }: ResourceTreeProps) {
  const [selectedNode, setSelectedNode] = useState<string | null>(null);

  const { nodes, edges } = useMemo(() => {
    if (!resourceGraph?.resources) return { nodes: [], edges: [] };
    return layoutNodes(resourceGraph.resources);
  }, [resourceGraph]);

  const showMinimap = nodes.length > 8;

  const handleNodeClick = useCallback((_: React.MouseEvent, node: Node) => {
    setSelectedNode(node.id);
  }, []);

  if (nodes.length === 0) {
    return (
      <div className={cn("flex items-center justify-center py-12", className)}>
        <p className="text-sm" style={{ color: "var(--text-muted)" }}>
          No resource graph available
        </p>
      </div>
    );
  }

  return (
    <div className={cn("h-[400px]", className)} data-testid="resource-tree">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={handleNodeClick}
        fitView
        proOptions={{ hideAttribution: true }}
      >
        <Background />
        <Controls showInteractive={false} />
        {showMinimap && <MiniMap />}
      </ReactFlow>

      {selectedNode && (
        <ResourceNodeDetail
          nodeId={selectedNode}
          instance={instance}
          onClose={() => setSelectedNode(null)}
        />
      )}
    </div>
  );
}
