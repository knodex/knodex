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

// failing-first: surface what needs attention before healthy groups (mirrors the overview rail)
const HEALTH_RANK: Record<string, number> = {
  Unhealthy: 0,
  Degraded: 1,
  Progressing: 2,
  Unknown: 3,
  Healthy: 4,
};

function isFailing(health: string): boolean {
  return health === "Unhealthy" || health === "Degraded";
}

function HealthDot({ health }: { health: string }) {
  if (health === "None") return null;
  return (
    <span
      className={cn("inline-block h-[7px] w-[7px] shrink-0 rounded-full", HEALTH_DOT_CLASS[health] ?? HEALTH_DOT_CLASS.Unknown)}
      title={health}
    />
  );
}

function ClusterBadge({ cluster, status }: { cluster?: string; status?: string }) {
  if (!cluster) return null;
  const isUnreachable = status === "unreachable";
  return (
    <span className={cn(
      "text-[11px] rounded-full px-2 py-0.5",
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
    <div className="flex items-center gap-3 py-1.5 pl-11 pr-4 text-sm hover:bg-[var(--border-subtle)]">
      <HealthDot health={resource.health} />
      <span className="font-mono text-xs text-[var(--text-primary)] truncate">{resource.name}</span>
      <ClusterBadge cluster={resource.cluster} status={resource.clusterStatus} />
      {resource.phase && (
        <span className="text-[11px] text-[var(--text-muted)]">{resource.phase}</span>
      )}
      <span className="ml-auto shrink-0 text-[11px] text-[var(--text-muted)]">
        {resource.createdAt ? formatDistanceToNow(resource.createdAt) : ""}
      </span>
    </div>
  );
}

function ResourceGroupRow({ group }: { group: ChildResourceGroup }) {
  // failing groups open by default — the failure is the reason you opened this tab
  const [expanded, setExpanded] = useState(isFailing(group.health));
  const ChevronIcon = expanded ? ChevronDown : ChevronRight;
  const failing = isFailing(group.health);

  return (
    <div>
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-[var(--border-subtle)] transition-colors"
      >
        <ChevronIcon className="h-4 w-4 shrink-0 text-[var(--text-muted)]" />
        <Box className="h-4 w-4 shrink-0 text-[var(--text-secondary)]" />
        <span className="font-medium text-sm text-[var(--text-primary)] truncate">{group.nodeId}</span>
        <span className="text-xs text-[var(--text-muted)] truncate">{group.kind}</span>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          <HealthDot health={group.health} />
          {group.health !== "None" && (
            <span className={cn("text-xs", failing ? "text-[var(--status-error)]" : "text-[var(--text-secondary)]")}>
              {group.readyCount}/{group.count} ready
            </span>
          )}
        </div>
      </button>
      {expanded && (
        <div className="bg-[var(--surface-bg)] py-1">
          {group.resources.map((resource) => (
            <ResourceRow key={`${resource.namespace}/${resource.name}`} resource={resource} />
          ))}
        </div>
      )}
    </div>
  );
}

/** Shared card chrome so loading / error / empty / data states all read as one panel. */
function ResourcesCard({
  meta,
  children,
}: {
  meta?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-[var(--border-default)] bg-[var(--surface-primary)] overflow-hidden">
      <div className="flex items-center gap-2.5 px-4 py-3 border-b border-[var(--border-subtle)]">
        <Box className="h-4 w-4 text-[var(--text-muted)]" />
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Resources</h3>
        {meta}
      </div>
      {children}
    </div>
  );
}

interface InstanceChildResourcesProps {
  group: string;
  namespace: string;
  kind: string;
  name: string;
}

export function InstanceChildResources({ group, namespace, kind, name }: InstanceChildResourcesProps) {
  const { data, isLoading, error } = useInstanceChildren(group, namespace, kind, name);

  if (isLoading) {
    return (
      <ResourcesCard>
        <div className="flex items-center justify-center py-8 text-[var(--text-muted)]">
          <Loader2 className="h-5 w-5 animate-spin mr-2" />
          <span className="text-sm">Discovering child resources...</span>
        </div>
      </ResourcesCard>
    );
  }

  if (error) {
    return (
      <ResourcesCard>
        <div className="text-sm text-[var(--status-error)] py-6 px-4">
          Failed to load child resources: {error.message}
        </div>
      </ResourcesCard>
    );
  }

  if (!data || (data.totalCount === 0 && !data.clusterUnreachable)) {
    return (
      <ResourcesCard>
        <div className="text-sm text-[var(--text-muted)] py-8 text-center">
          No child resources found for this instance.
        </div>
      </ResourcesCard>
    );
  }

  const groups = data.groups ?? [];
  const total = groups.reduce((sum, g) => sum + g.count, 0);
  const ready = groups.reduce((sum, g) => sum + g.readyCount, 0);
  const failingCount = groups.filter((g) => isFailing(g.health)).length;
  const sorted = [...groups].sort(
    (a, b) => (HEALTH_RANK[a.health] ?? 3) - (HEALTH_RANK[b.health] ?? 3)
  );

  return (
    <ResourcesCard
      meta={
        <>
          {total > 0 && (
            <span className="text-xs text-[var(--text-muted)]">{ready}/{total} ready</span>
          )}
          {failingCount > 0 && (
            <span className="text-xs text-[var(--status-error)]">· {failingCount} failing</span>
          )}
        </>
      }
    >
      {data.clusterUnreachable && data.unreachableClusters && data.unreachableClusters.length > 0 && (
        <div className="flex items-center gap-2 px-4 py-2.5 border-b border-[var(--border-subtle)] bg-[var(--status-warning)]/10 text-[var(--status-warning)] text-xs">
          <AlertTriangle className="h-4 w-4 shrink-0" />
          <span>
            Cluster {data.unreachableClusters.join(", ")}: temporarily unreachable — showing last known data
          </span>
        </div>
      )}
      <div className="divide-y divide-[var(--border-subtle)]">
        {sorted.map((g) => (
          <ResourceGroupRow key={g.nodeId} group={g} />
        ))}
      </div>
    </ResourcesCard>
  );
}
