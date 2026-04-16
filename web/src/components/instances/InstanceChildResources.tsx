// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useInstanceChildren } from "@/hooks/useInstances";
import type { ChildResourceGroup, ChildResource } from "@/types/rgd";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronRight, Box, Loader2, AlertTriangle } from "@/lib/icons";
import { useState } from "react";
import { formatDistanceToNow } from "@/lib/date";

const HEALTH_DOT_CLASS: Record<string, string> = {
  Healthy: "bg-[var(--status-healthy)]",
  Degraded: "bg-[var(--status-warning)]",
  Unhealthy: "bg-[var(--status-error)]",
  Progressing: "bg-[var(--status-info)]",
  Unknown: "bg-[var(--status-inactive)]",
};

function HealthDot({ health }: { health: string }) {
  if (health === "None") return null;
  return (
    <span
      className={cn("inline-block h-2 w-2 rounded-full", HEALTH_DOT_CLASS[health] ?? HEALTH_DOT_CLASS.Unknown)}
      title={health}
    />
  );
}

function ClusterBadge({ cluster, status }: { cluster?: string; status?: string }) {
  if (!cluster) return null;
  const isUnreachable = status === "unreachable";
  return (
    <span className={cn(
      "text-xs rounded-full px-2 py-0.5",
      isUnreachable
        ? "bg-[var(--status-warning)]/10 text-[var(--status-warning)]"
        : "bg-[var(--status-info)]/10 text-[var(--status-info)]"
    )}>
      {isUnreachable ? `${cluster} (unreachable)` : cluster}
    </span>
  );
}

function ResourceRow({ resource }: { resource: ChildResource }) {
  return (
    <div className="flex items-center gap-3 py-1.5 px-3 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]">
      <HealthDot health={resource.health} />
      <span className="font-mono text-xs text-[var(--text-primary)]">{resource.name}</span>
      <ClusterBadge cluster={resource.cluster} status={resource.clusterStatus} />
      {resource.phase && (
        <span className="text-xs text-[var(--text-tertiary)]">{resource.phase}</span>
      )}
      <span className="ml-auto text-xs text-[var(--text-tertiary)]">
        {resource.createdAt ? formatDistanceToNow(resource.createdAt) : ""}
      </span>
    </div>
  );
}

function ResourceGroupCard({ group }: { group: ChildResourceGroup }) {
  const [expanded, setExpanded] = useState(false);
  const ChevronIcon = expanded ? ChevronDown : ChevronRight;

  return (
    <div className="border border-[var(--border-default)] rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-[var(--bg-hover)] transition-colors"
      >
        <ChevronIcon className="h-4 w-4 text-[var(--text-tertiary)]" />
        <Box className="h-4 w-4 text-[var(--text-secondary)]" />
        <span className="font-medium text-sm text-[var(--text-primary)]">{group.nodeId}</span>
        <span className="text-xs text-[var(--text-tertiary)]">{group.kind}</span>
        <div className="ml-auto flex items-center gap-2">
          <HealthDot health={group.health} />
          {group.health !== "None" && (
            <span className="text-xs text-[var(--text-secondary)]">
              {group.readyCount}/{group.count} ready
            </span>
          )}
        </div>
      </button>
      {expanded && (
        <div className="border-t border-[var(--border-default)] bg-[var(--bg-secondary)]">
          {group.resources.map((resource) => (
            <ResourceRow key={`${resource.namespace}/${resource.name}`} resource={resource} />
          ))}
        </div>
      )}
    </div>
  );
}

interface InstanceChildResourcesProps {
  namespace: string;
  kind: string;
  name: string;
}

export function InstanceChildResources({ namespace, kind, name }: InstanceChildResourcesProps) {
  const { data, isLoading, error } = useInstanceChildren(namespace, kind, name);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8 text-[var(--text-tertiary)]">
        <Loader2 className="h-5 w-5 animate-spin mr-2" />
        <span className="text-sm">Discovering child resources...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-sm text-[var(--status-error)] py-4 px-2">
        Failed to load child resources: {error.message}
      </div>
    );
  }

  if (!data || (data.totalCount === 0 && !data.clusterUnreachable)) {
    return (
      <div className="text-sm text-[var(--text-tertiary)] py-8 text-center">
        No child resources found for this instance.
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {data.clusterUnreachable && data.unreachableClusters && data.unreachableClusters.length > 0 && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-[var(--status-warning)]/10 text-[var(--status-warning)] text-sm">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          <span>
            Cluster {data.unreachableClusters.join(", ")}: temporarily unreachable — showing last known data
          </span>
        </div>
      )}
      {data.groups.map((group) => (
        <ResourceGroupCard key={group.nodeId} group={group} />
      ))}
    </div>
  );
}
