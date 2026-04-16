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
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
    <TableRow className={cn(isWarning && "bg-status-warning/5")}>
      <TableCell className="whitespace-nowrap text-muted-foreground">
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="cursor-default">{getRelativeTime(event.lastSeen)}</span>
          </TooltipTrigger>
          <TooltipContent>{formatTimestamp(event.lastSeen)}</TooltipContent>
        </Tooltip>
      </TableCell>
      <TableCell className="whitespace-nowrap">
        <span
          className={cn(
            "text-xs font-medium",
            isWarning ? "text-status-warning" : "text-muted-foreground"
          )}
        >
          {event.type}
        </span>
      </TableCell>
      <TableCell className="whitespace-nowrap font-medium">{event.reason}</TableCell>
      <TableCell className="whitespace-nowrap font-mono text-xs text-muted-foreground">
        {event.object}
      </TableCell>
      <TableCell className="max-w-[400px] truncate">
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

export function InstanceEvents({ namespace, kind, name }: InstanceEventsProps) {
  const [filterType, setFilterType] = useState<FilterType>("all");

  const { data, isLoading, error, refetch } = useInstanceEvents(
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
      <div className="rounded-lg border border-border bg-card">
        <div className="px-4 py-3 border-b border-border">
          <h3 className="text-sm font-medium text-foreground">Events</h3>
        </div>
        <div className="p-8 flex items-center justify-center">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-border bg-card">
        <div className="px-4 py-3 border-b border-border">
          <h3 className="text-sm font-medium text-foreground">Events</h3>
        </div>
        <div className="p-6 flex items-center gap-3 text-muted-foreground">
          <AlertCircle className="h-5 w-5" />
          <div>
            <p className="text-sm">Failed to load events</p>
            <p className="text-xs mt-1">
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
    <div className="rounded-lg border border-border bg-card overflow-hidden">
      {/* Header with filter */}
      <div className="px-4 py-3 border-b border-border flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Zap className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-medium text-foreground">Events</h3>
          <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-secondary text-muted-foreground">
            {filteredEvents.length}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          {(["all", "Warning", "Normal"] as const).map((type) => {
            const label =
              type === "all"
                ? `All (${events.length})`
                : type === "Warning"
                  ? `Warning (${warningCount})`
                  : `Normal (${normalCount})`;
            return (
              <button
                key={type}
                type="button"
                onClick={() => setFilterType(type)}
                className={cn(
                  "px-2.5 py-1 rounded-md text-xs font-medium transition-colors",
                  filterType === type
                    ? type === "Warning"
                      ? "bg-status-warning/10 text-status-warning"
                      : type === "Normal"
                        ? "bg-primary/10 text-primary"
                        : "bg-secondary text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary"
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
        <div className="text-center py-8 text-muted-foreground">
          <Zap className="h-8 w-8 mx-auto mb-2 opacity-50" />
          <p className="text-sm">No events</p>
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-[100px]">Last Seen</TableHead>
              <TableHead className="w-[70px]">Type</TableHead>
              <TableHead className="w-[150px]">Reason</TableHead>
              <TableHead className="w-[200px]">Object</TableHead>
              <TableHead>Message</TableHead>
            </TableRow>
          </TableHeader>
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
