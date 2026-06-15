// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useMemo } from "react";
import { Zap, Loader2, AlertCircle, RefreshCw } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@/components/ui/table";
import { ListTableHeader } from "@/components/ui/list-table";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useInstanceEvents } from "@/hooks/useHistory";
import type { KubernetesEvent } from "@/types/history";
import { cn } from "@/lib/utils";

type FilterType = "all" | "Normal" | "Warning";

interface InstanceEventsProps {
  group: string;
  namespace: string;
  kind: string;
  name: string;
}

function getRelativeTime(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();

  const seconds = Math.floor(diffMs / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}d ago`;
  if (hours > 0) return `${hours}h ago`;
  if (minutes > 0) return `${minutes}m ago`;
  if (seconds > 5) return `${seconds}s ago`;
  return "just now";
}

function formatTimestamp(timestamp: string): string {
  return new Date(timestamp).toLocaleString();
}

function EventRow({ event }: { event: KubernetesEvent }) {
  const isWarning = event.type === "Warning";

  return (
    <TableRow className={cn(isWarning && "bg-[var(--status-warning)]/5")}>
      <TableCell className="whitespace-nowrap text-[var(--text-muted)]">
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="cursor-default">{getRelativeTime(event.lastSeen)}</span>
          </TooltipTrigger>
          <TooltipContent>{formatTimestamp(event.lastSeen)}</TooltipContent>
        </Tooltip>
      </TableCell>
      <TableCell className="whitespace-nowrap">
        <span className="inline-flex items-center gap-1.5 text-xs font-medium">
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full",
              isWarning ? "bg-[var(--status-warning)]" : "bg-[var(--status-healthy)]"
            )}
          />
          <span className={cn(isWarning ? "text-status-warning" : "text-[var(--text-muted)]")}>
            {event.type}
          </span>
        </span>
      </TableCell>
      <TableCell className="whitespace-nowrap font-medium text-[var(--text-primary)]">{event.reason}</TableCell>
      <TableCell className="whitespace-nowrap font-mono text-xs text-[var(--text-muted)]">
        {event.object}
      </TableCell>
      <TableCell className="max-w-[400px] truncate text-[var(--text-secondary)]">
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="cursor-default">{event.message}</span>
          </TooltipTrigger>
          <TooltipContent className="max-w-[500px]">{event.message}</TooltipContent>
        </Tooltip>
      </TableCell>
    </TableRow>
  );
}

export function InstanceEvents({ group, namespace, kind, name }: InstanceEventsProps) {
  const [filterType, setFilterType] = useState<FilterType>("all");

  const { data, isLoading, error, refetch } = useInstanceEvents(
    group,
    namespace,
    kind,
    name
  );

  const events = useMemo(() => data?.events ?? [], [data?.events]);

  const filteredEvents = useMemo(() => {
    if (filterType === "all") return events;
    return events.filter((e) => e.type === filterType);
  }, [events, filterType]);

  const warningCount = useMemo(
    () => events.filter((e) => e.type === "Warning").length,
    [events]
  );
  const normalCount = useMemo(
    () => events.filter((e) => e.type === "Normal").length,
    [events]
  );

  if (isLoading) {
    return (
      <div className="rounded-lg border border-[var(--border-default)] bg-[var(--surface-primary)] overflow-hidden">
        <div className="flex items-center gap-2.5 px-4 py-3 border-b border-[var(--border-subtle)]">
          <Zap className="h-4 w-4 text-[var(--text-muted)]" />
          <h3 className="text-sm font-medium text-[var(--text-primary)]">Events</h3>
        </div>
        <div className="p-8 flex items-center justify-center">
          <Loader2 className="h-5 w-5 animate-spin text-[var(--text-muted)]" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-[var(--border-default)] bg-[var(--surface-primary)] overflow-hidden">
        <div className="flex items-center gap-2.5 px-4 py-3 border-b border-[var(--border-subtle)]">
          <Zap className="h-4 w-4 text-[var(--text-muted)]" />
          <h3 className="text-sm font-medium text-[var(--text-primary)]">Events</h3>
        </div>
        <div className="p-6 flex items-center gap-3 text-[var(--text-secondary)]">
          <AlertCircle className="h-5 w-5 text-[var(--status-error)]" />
          <div>
            <p className="text-sm">Failed to load events</p>
            <p className="text-xs mt-1 text-[var(--text-muted)]">
              {error instanceof Error ? error.message : "An unexpected error occurred"}
            </p>
          </div>
          <Button variant="ghost" size="sm" onClick={() => refetch()} className="ml-auto">
            <RefreshCw className="h-4 w-4 mr-1.5" />
            Retry
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-[var(--border-default)] bg-[var(--surface-primary)] overflow-hidden">
      {/* Header with filter */}
      <div className="px-4 py-3 border-b border-[var(--border-subtle)] flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <Zap className="h-4 w-4 text-[var(--text-muted)]" />
          <h3 className="text-sm font-medium text-[var(--text-primary)]">Events</h3>
          <span className="text-xs text-[var(--text-muted)]">{filteredEvents.length}</span>
        </div>

        {/* segmented filter control */}
        <div className="flex items-center gap-0.5 rounded-md border border-[var(--border-subtle)] bg-[var(--surface-bg)] p-0.5">
          {(["all", "Warning", "Normal"] as const).map((type) => {
            const label =
              type === "all"
                ? `All (${events.length})`
                : type === "Warning"
                  ? `Warning (${warningCount})`
                  : `Normal (${normalCount})`;
            const active = filterType === type;
            return (
              <button
                key={type}
                type="button"
                onClick={() => setFilterType(type)}
                className={cn(
                  "px-2.5 py-1 rounded text-xs font-medium transition-colors",
                  active
                    ? type === "Warning"
                      ? "bg-[var(--status-warning)]/10 text-status-warning"
                      : type === "Normal"
                        ? "bg-primary/10 text-primary"
                        : "bg-[var(--surface-elevated)] text-[var(--text-primary)]"
                    : "text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                )}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Table */}
      {filteredEvents.length === 0 ? (
        <div className="text-center py-8 text-[var(--text-muted)]">
          <Zap className="h-8 w-8 mx-auto mb-2 opacity-50" />
          <p className="text-sm">No events</p>
        </div>
      ) : (
        <Table>
          <ListTableHeader>
            <TableRow>
              <TableHead className="w-[100px]">Last Seen</TableHead>
              <TableHead className="w-[70px]">Type</TableHead>
              <TableHead className="w-[150px]">Reason</TableHead>
              <TableHead className="w-[200px]">Object</TableHead>
              <TableHead>Message</TableHead>
            </TableRow>
          </ListTableHeader>
          <TableBody>
            {filteredEvents.map((event, index) => (
              <EventRow key={`${event.lastSeen}-${event.object}-${event.reason}-${index}`} event={event} />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
