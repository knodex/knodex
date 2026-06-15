// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import { formatDistanceToNow } from "@/lib/date";
import type { ChildResourceGroup } from "@/types/rgd";
import type { KubernetesEvent } from "@/types/history";

const MAX_RAIL_RESOURCES = 7;
const MAX_RAIL_EVENTS = 3;

const DOT_CLASS: Record<string, string> = {
  Healthy: "bg-[var(--status-healthy)]",
  Degraded: "bg-[var(--status-warning)]",
  Unhealthy: "bg-[var(--status-error)] shadow-[0_0_5px_rgba(244,63,94,0.6)]",
  Progressing: "bg-[var(--status-info)]",
  Unknown: "bg-[var(--status-inactive)]",
};

interface InstanceResourcesSummaryCardProps {
  groups: ChildResourceGroup[];
  totalCount: number;
  onViewAll: () => void;
}

/** Rail mini-list of child resources — failing groups surfaced first. */
export function InstanceResourcesSummaryCard({
  groups,
  totalCount,
  onViewAll,
}: InstanceResourcesSummaryCardProps) {
  if (groups.length === 0) return null;

  const failingCount = groups.filter(
    (g) => g.health === "Unhealthy" || g.health === "Degraded"
  ).length;
  const sorted = [...groups].sort((a, b) => {
    const rank = (g: ChildResourceGroup) =>
      g.health === "Unhealthy" ? 0 : g.health === "Degraded" ? 1 : g.health === "Progressing" ? 2 : 3;
    return rank(a) - rank(b);
  });
  const visible = sorted.slice(0, MAX_RAIL_RESOURCES);

  return (
    <div
      className="rounded-lg border overflow-hidden border-[var(--border-default)] bg-[var(--surface-primary)]"
      data-testid="resources-summary-card"
    >
      <div className="flex items-center px-4 py-3 border-b border-[var(--border-subtle)]">
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Resources</h3>
        {failingCount > 0 && (
          <span className="ml-auto text-xs text-[var(--status-error)]">
            {failingCount} failing
          </span>
        )}
      </div>
      <div className="py-1">
        {visible.map((group) => {
          const unhealthy = group.health === "Unhealthy" || group.health === "Degraded";
          return (
            <div key={group.nodeId} className="flex items-center gap-2.5 px-4 py-2 text-xs">
              <span
                className={cn(
                  "inline-block h-[7px] w-[7px] shrink-0 rounded-full",
                  DOT_CLASS[group.health] ?? DOT_CLASS.Unknown
                )}
              />
              <span className="font-mono text-[var(--text-primary)] truncate">{group.nodeId}</span>
              <span className="text-[11px] text-[var(--text-muted)] truncate">{group.kind}</span>
              <span
                className={cn(
                  "ml-auto shrink-0 font-mono text-[11px]",
                  unhealthy ? "text-[var(--status-error)]" : "text-[var(--text-secondary)]"
                )}
              >
                {group.readyCount}/{group.count}
              </span>
            </div>
          );
        })}
      </div>
      <button
        type="button"
        onClick={onViewAll}
        className="block w-full border-t border-[var(--border-subtle)] py-2.5 text-center text-xs text-primary hover:bg-primary/5 transition-colors"
      >
        All {totalCount} resource{totalCount === 1 ? "" : "s"} →
      </button>
    </div>
  );
}

interface InstanceRecentActivityCardProps {
  events: KubernetesEvent[];
  onViewAll: () => void;
}

/** Rail timeline of the most recent K8s events. */
export function InstanceRecentActivityCard({
  events,
  onViewAll,
}: InstanceRecentActivityCardProps) {
  if (events.length === 0) return null;

  const recent = [...events]
    .sort((a, b) => new Date(b.lastSeen).getTime() - new Date(a.lastSeen).getTime())
    .slice(0, MAX_RAIL_EVENTS);

  return (
    <div
      className="rounded-lg border overflow-hidden border-[var(--border-default)] bg-[var(--surface-primary)]"
      data-testid="recent-activity-card"
    >
      <div className="px-4 py-3 border-b border-[var(--border-subtle)]">
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Recent activity</h3>
      </div>
      <div className="py-1.5">
        {recent.map((event, idx) => (
          <div key={`${event.lastSeen}-${event.reason}-${idx}`} className="flex gap-3 px-4 py-2">
            <div className="flex flex-col items-center">
              <span
                className={cn(
                  "mt-1 h-2 w-2 shrink-0 rounded-full border-2",
                  event.type === "Warning"
                    ? "border-[var(--status-warning)]"
                    : "border-[var(--status-healthy)]"
                )}
              />
              {idx < recent.length - 1 && (
                <span className="mt-1 w-px flex-1 bg-[var(--border-default)]" />
              )}
            </div>
            <div className="min-w-0 pb-0.5">
              <div className="text-xs font-medium text-[var(--text-primary)]">{event.reason}</div>
              <div className="mt-0.5 text-[11px] text-[var(--text-muted)]">
                {formatDistanceToNow(event.lastSeen)} · <span className="font-mono">{event.object}</span>
              </div>
              {event.message && (
                <div className="mt-0.5 text-[11px] leading-snug text-[var(--text-secondary)] line-clamp-2">
                  {event.message}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={onViewAll}
        className="block w-full border-t border-[var(--border-subtle)] py-2.5 text-center text-xs text-primary hover:bg-primary/5 transition-colors"
      >
        All {events.length} event{events.length === 1 ? "" : "s"} →
      </button>
    </div>
  );
}
