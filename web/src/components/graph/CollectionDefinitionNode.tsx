// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import type { Node, NodeProps } from "@xyflow/react";
import { Handle, Position } from "@xyflow/react";
import { Layers } from "@/lib/icons";
import type { Iterator } from "@/types/rgd";
import { cn } from "@/lib/utils";

type CollectionNodeData = {
  label: string;
  apiVersion: string;
  kind: string;
  isTemplate: boolean;
  isConditional: boolean;
  conditionExpr?: string;
  hasExternalRef: boolean;
  isCollection: boolean;
  forEach: Iterator[];
  readyWhen?: string[];
  resourceId: string;
  [key: string]: unknown;
};

export type CollectionNodeType = Node<CollectionNodeData, "collection">;

function SourceBadge({ source }: { source: Iterator["source"] }) {
  const label = source === "schema" ? "S" : source === "resource" ? "R" : "L";
  return (
    <span
      className={cn(
        "inline-flex items-center justify-center w-5 h-5 rounded text-[10px] font-medium",
        source === "schema" && "bg-graph-collection/20 text-graph-collection",
        source === "resource" && "border border-dashed border-graph-collection text-graph-collection",
        source === "literal" && "bg-muted text-muted-foreground"
      )}
      title={source}
    >
      {label}
    </span>
  );
}

export function CollectionDefinitionNode({ data }: NodeProps<CollectionNodeType>) {
  const [expanded, setExpanded] = useState(false);
  const isDynamic = data.forEach.length > 0 && data.forEach.every((it) => it.source === "resource");

  return (
    <>
      <Handle
        type="target"
        position={Position.Left}
        className="!bg-muted-foreground !border-border !w-2 !h-2"
      />
      <div
        role="button"
        tabIndex={0}
        className="relative cursor-pointer"
        onClick={() => setExpanded(!expanded)}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setExpanded(!expanded); } }}
      >
        {/* Main card with stacked-card shadow effect */}
        <div
          className={cn(
            "px-4 py-3 rounded-lg border-2 min-w-[200px] transition-all bg-card",
            "hover:border-muted-foreground",
            "border-l-[4px] border-l-graph-collection"
          )}
          style={{
            borderColor: "hsl(var(--graph-collection))",
            borderLeftWidth: "4px",
            boxShadow:
              "4px 4px 0 -1px hsl(var(--card)), " +
              "4px 4px 0 0px hsl(var(--border)), " +
              "8px 8px 0 -1px hsl(var(--card)), " +
              "8px 8px 0 0px hsl(var(--border))",
          }}
        >
          {/* Header */}
          <div className="flex items-center gap-2 mb-1">
            <Layers className="h-4 w-4 text-graph-collection" />
            <span className="text-sm font-medium text-foreground truncate">
              {data.kind} — {data.resourceId}
            </span>
          </div>

          {/* Badges row */}
          <div className="flex items-center gap-2 mt-2 flex-wrap">
            <span
              className="px-1.5 py-0.5 rounded text-xs bg-graph-collection/20 text-graph-collection font-medium"
            >
              forEach
            </span>
            {isDynamic && (
              <span
                className="px-1.5 py-0.5 rounded text-xs font-medium border border-dashed border-graph-collection text-graph-collection"
              >
                dynamic
              </span>
            )}
            {data.isConditional && (
              <span
                className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-graph-conditional/20 text-graph-conditional"
                title={data.conditionExpr || "Conditional resource"}
              >
                ?
              </span>
            )}
            <span className="px-1.5 py-0.5 rounded text-xs text-muted-foreground">
              {data.apiVersion}
            </span>
          </div>

          {/* Expandable preview */}
          {expanded && (
            <div className="mt-3 pt-3 border-t border-border space-y-2 text-xs">
              {/* forEach iterators */}
              {data.forEach.length > 0 && (
                <div>
                  <div className="text-muted-foreground font-medium mb-1">forEach:</div>
                  {data.forEach.map((it, i) => (
                    <div key={i} className="flex items-center gap-2 ml-2 mb-0.5">
                      <span className="font-bold text-foreground">{it.name}:</span>
                      <span className="text-muted-foreground font-mono truncate max-w-[180px]" title={it.expression}>
                        {it.expression}
                      </span>
                      <SourceBadge source={it.source} />
                    </div>
                  ))}
                </div>
              )}

              {/* readyWhen */}
              {data.readyWhen && data.readyWhen.length > 0 && (
                <div>
                  <div className="text-muted-foreground font-medium mb-1">readyWhen:</div>
                  {data.readyWhen.map((expr, i) => (
                    <div key={i} className="ml-2 text-muted-foreground font-mono truncate max-w-[220px]" title={expr}>
                      {expr}
                    </div>
                  ))}
                </div>
              )}

              {/* includeWhen (conditionExpr) */}
              {data.conditionExpr && (
                <div>
                  <div className="text-muted-foreground font-medium mb-1">includeWhen:</div>
                  <div className="ml-2 flex items-center gap-1">
                    <span className="px-1 py-0.5 rounded text-[10px] font-medium bg-graph-conditional/20 text-graph-conditional">?</span>
                    <span className="text-muted-foreground font-mono truncate max-w-[200px]" title={data.conditionExpr}>
                      {data.conditionExpr}
                    </span>
                  </div>
                </div>
              )}
            </div>
          )}
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
