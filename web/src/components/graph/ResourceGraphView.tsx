// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  MarkerType,
  Position,
  Handle,
} from "@xyflow/react";
import type { Node, Edge, NodeProps } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
  ExternalLink,
  AlertCircle,
  FileCode2,
} from "lucide-react";
import type { ResourceGraph, ResourceNode } from "@/types/rgd";
import { cn } from "@/lib/utils";

interface ResourceGraphViewProps {
  resourceGraph: ResourceGraph;
}

// Kind-based colors using CSS variables for theming
// These use HSL values that work with opacity modifiers
const KIND_COLORS: Record<string, { hsl: string; hex: string }> = {
  Deployment: { hsl: "var(--graph-template)", hex: "#3b82f6" },      // Blue
  StatefulSet: { hsl: "var(--accent)", hex: "#8b5cf6" },              // Violet/Indigo
  DaemonSet: { hsl: "var(--accent)", hex: "#8b5cf6" },                // Violet
  Service: { hsl: "var(--success)", hex: "#10b981" },                 // Emerald
  Ingress: { hsl: "var(--warning)", hex: "#f59e0b" },                 // Amber
  ConfigMap: { hsl: "var(--primary)", hex: "#14b8a6" },               // Teal
  Secret: { hsl: "var(--destructive)", hex: "#ef4444" },              // Red
  PersistentVolumeClaim: { hsl: "var(--accent)", hex: "#ec4899" },    // Pink
  Job: { hsl: "var(--graph-external)", hex: "#f97316" },              // Orange
  CronJob: { hsl: "var(--graph-external)", hex: "#f97316" },          // Orange
  ServiceAccount: { hsl: "var(--accent)", hex: "#6366f1" },           // Indigo
  default: { hsl: "var(--muted-foreground)", hex: "#6b7280" },        // Gray
};

function getKindColor(kind: string): { hsl: string; hex: string } {
  return KIND_COLORS[kind] || KIND_COLORS.default;
}

type ResourceNodeData = {
  label: string;
  apiVersion: string;
  kind: string;
  isTemplate: boolean;
  isConditional: boolean;
  conditionExpr?: string;
  hasExternalRef: boolean;
  [key: string]: unknown;
};

type ResourceNodeType = Node<ResourceNodeData, "resource">;

function ResourceNodeComponent({ data }: NodeProps<ResourceNodeType>) {
  const kindColor = getKindColor(data.kind);

  return (
    <>
      <Handle
        type="target"
        position={Position.Left}
        className="!bg-muted-foreground !border-border !w-2 !h-2"
      />
      <div
        className={cn(
          "px-4 py-3 rounded-lg border-2 min-w-[180px] transition-all bg-card",
          "hover:border-muted-foreground",
          data.isTemplate ? "border-l-[4px] border-l-graph-template" : "border-l-[4px] border-l-graph-external"
        )}
        style={{
          borderColor: data.isTemplate
            ? "hsl(var(--graph-template))"
            : "hsl(var(--graph-external))",
          borderLeftWidth: "4px",
        }}
      >
        <div className="flex items-center gap-2 mb-1">
          {data.isTemplate ? (
            <FileCode2 className="h-4 w-4 text-graph-template" />
          ) : (
            <ExternalLink className="h-4 w-4 text-graph-external" />
          )}
          <span className="text-sm font-medium text-foreground truncate">
            {data.label}
          </span>
          {data.isConditional && (
            <span
              className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-graph-conditional/20 text-graph-conditional"
              title={data.conditionExpr || "Conditional resource"}
            >
              ?
            </span>
          )}
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          <span className="font-mono truncate">{data.kind}</span>
        </div>
        <div className="mt-2 flex items-center gap-2">
          <span
            className="px-1.5 py-0.5 rounded text-xs"
            style={{
              backgroundColor: `hsl(${kindColor.hsl} / 0.2)`,
              color: `hsl(${kindColor.hsl})`,
            }}
          >
            {data.apiVersion}
          </span>
          <span
            className={cn(
              "px-1.5 py-0.5 rounded text-xs",
              data.isTemplate
                ? "bg-graph-template/20 text-graph-template"
                : "bg-graph-external/20 text-graph-external"
            )}
          >
            {data.isTemplate ? "Template" : "ExternalRef"}
          </span>
        </div>
      </div>
      <Handle
        type="source"
        position={Position.Right}
        className="!bg-muted-foreground !border-border !w-2 !h-2"
      />
    </>
  );
}

const nodeTypes = {
  resource: ResourceNodeComponent,
};

export function ResourceGraphView({ resourceGraph }: ResourceGraphViewProps) {
  const { nodes: flowNodes, edges: flowEdges } = useMemo(() => {
    const nodes: ResourceNodeType[] = [];
    const edges: Edge[] = [];

    // Layout constants
    const NODE_WIDTH = 200;
    const NODE_HEIGHT = 100;
    const HORIZONTAL_GAP = 100;
    const VERTICAL_GAP = 40;

    // Build dependency map to calculate levels
    const dependencyCount: Record<string, number> = {};
    const dependents: Record<string, string[]> = {};

    resourceGraph.resources.forEach((res) => {
      dependencyCount[res.id] = res.dependsOn.length;
      dependents[res.id] = [];
    });

    resourceGraph.edges.forEach((edge) => {
      if (dependents[edge.to]) {
        dependents[edge.to].push(edge.from);
      }
    });

    // Topological sort to arrange nodes by level
    const levels: string[][] = [];
    const assigned = new Set<string>();
    const queue: string[] = [];

    // Start with nodes that have no dependencies
    resourceGraph.resources.forEach((res) => {
      if (res.dependsOn.length === 0) {
        queue.push(res.id);
      }
    });

    while (queue.length > 0 || assigned.size < resourceGraph.resources.length) {
      const currentLevel: string[] = [];

      // Process all nodes in the queue
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

      // Add nodes whose dependencies are all assigned
      resourceGraph.resources.forEach((res) => {
        if (!assigned.has(res.id)) {
          const allDepsAssigned = res.dependsOn.every((dep) =>
            assigned.has(dep)
          );
          if (allDepsAssigned) {
            queue.push(res.id);
          }
        }
      });

      // Break infinite loops (circular dependencies)
      if (
        queue.length === 0 &&
        assigned.size < resourceGraph.resources.length
      ) {
        const unassigned = resourceGraph.resources.find(
          (res) => !assigned.has(res.id)
        );
        if (unassigned) {
          queue.push(unassigned.id);
        }
      }
    }

    // Position nodes by level
    levels.forEach((levelNodes, levelIndex) => {
      const startY =
        -((levelNodes.length - 1) * (NODE_HEIGHT + VERTICAL_GAP)) / 2;

      levelNodes.forEach((nodeId, nodeIndex) => {
        const resource = resourceGraph.resources.find(
          (r) => r.id === nodeId
        ) as ResourceNode;
        if (!resource) return;

        nodes.push({
          id: resource.id,
          type: "resource",
          position: {
            x: levelIndex * (NODE_WIDTH + HORIZONTAL_GAP),
            y: startY + nodeIndex * (NODE_HEIGHT + VERTICAL_GAP),
          },
          data: {
            label: resource.kind,
            apiVersion: resource.apiVersion,
            kind: resource.kind,
            isTemplate: resource.isTemplate,
            isConditional: resource.isConditional,
            conditionExpr: resource.conditionExpr,
            hasExternalRef: !!resource.externalRef,
          },
        });
      });
    });

    // Create edges - using CSS variables for theming
    resourceGraph.edges.forEach((edge, index) => {
      edges.push({
        id: `edge-${index}`,
        source: edge.to, // Edge goes from dependency to dependent
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

    return { nodes, edges };
  }, [resourceGraph]);

  const [nodes, , onNodesChange] = useNodesState(flowNodes);
  const [edges, , onEdgesChange] = useEdgesState(flowEdges);

  const onInit = useCallback(() => {
    // Graph initialized
  }, []);

  if (resourceGraph.resources.length === 0) {
    return (
      <div className="h-[400px] rounded-lg border border-border bg-card flex flex-col items-center justify-center gap-4">
        <AlertCircle className="h-8 w-8 text-muted-foreground" />
        <div className="text-center">
          <p className="text-sm text-foreground font-medium">
            No resources defined
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            This RGD doesn't have any resources in its spec
          </p>
        </div>
      </div>
    );
  }

  const templateCount = resourceGraph.resources.filter(
    (r) => r.isTemplate
  ).length;
  const externalRefCount = resourceGraph.resources.filter(
    (r) => !r.isTemplate
  ).length;
  const conditionalCount = resourceGraph.resources.filter(
    (r) => r.isConditional
  ).length;

  return (
    <div className="space-y-4">
      {/* Stats */}
      <div className="flex gap-4 text-xs text-muted-foreground">
        <span>
          <strong className="text-foreground">{resourceGraph.resources.length}</strong> resources
        </span>
        <span>
          <strong className="text-graph-template">{templateCount}</strong> templates
        </span>
        {externalRefCount > 0 && (
          <span>
            <strong className="text-graph-external">{externalRefCount}</strong> external refs
          </span>
        )}
        {conditionalCount > 0 && (
          <span>
            <strong className="text-graph-conditional">{conditionalCount}</strong> conditional
          </span>
        )}
      </div>

      <div className="h-[400px] rounded-lg border border-border bg-card overflow-hidden">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onInit={onInit}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.3 }}
          minZoom={0.5}
          maxZoom={1.5}
          defaultEdgeOptions={{
            type: "smoothstep",
          }}
          proOptions={{ hideAttribution: true }}
        >
          <Background color="hsl(var(--graph-background))" gap={20} size={1} />
          <Controls className="!bg-card !border-border !rounded-lg [&>button]:!bg-secondary [&>button]:!border-border [&>button]:!text-foreground [&>button:hover]:!bg-muted" />
          <MiniMap
            className="!bg-card !border-border !rounded-lg"
            nodeColor={(node) => {
              const data = node.data as ResourceNodeData;
              // MiniMap requires hex colors for canvas rendering
              return data.isTemplate
                ? KIND_COLORS.Deployment.hex
                : KIND_COLORS.Job.hex;
            }}
          />
        </ReactFlow>
      </div>

      {/* Legend */}
      <div className="flex flex-wrap gap-4 text-xs text-muted-foreground">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 rounded border-2 border-graph-template bg-graph-template/10" />
          <span>Template</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 rounded border-2 border-graph-external bg-graph-external/10" />
          <span>ExternalRef</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-graph-conditional/20 text-graph-conditional">
            ?
          </span>
          <span>Conditional (includeWhen)</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-8 h-0.5 bg-graph-edge" />
          <span>Dependency</span>
        </div>
      </div>
    </div>
  );
}
