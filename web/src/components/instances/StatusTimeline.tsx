// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import {
  CheckCircle2,
  Clock,
  AlertTriangle,
  GitBranch,
  Loader2,
  CloudUpload,
  RefreshCw,
  Server,
  XCircle,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { getInstanceStatusTimeline } from "@/api/rgd";
import type { StatusTransition } from "@/types/rgd";

interface StatusTimelineProps {
  instanceId?: string;
  className?: string;
}

// Status configuration for display
const STATUS_CONFIG: Record<
  string,
  { label: string; icon: React.ReactNode; className: string }
> = {
  PushedToGit: {
    label: "Pushed to Git",
    icon: <GitBranch className="h-4 w-4" />,
    className: "text-status-info border-status-info",
  },
  WaitingForSync: {
    label: "Waiting for Sync",
    icon: <Clock className="h-4 w-4" />,
    className: "text-status-warning border-status-warning",
  },
  Syncing: {
    label: "Syncing",
    icon: <RefreshCw className="h-4 w-4 animate-spin" />,
    className: "text-primary border-primary",
  },
  Creating: {
    label: "Creating Resources",
    icon: <Server className="h-4 w-4" />,
    className: "text-status-pending border-status-pending",
  },
  Ready: {
    label: "Ready",
    icon: <CheckCircle2 className="h-4 w-4" />,
    className: "text-status-success border-status-success",
  },
  GitOpsFailed: {
    label: "GitOps Failed",
    icon: <XCircle className="h-4 w-4" />,
    className: "text-destructive border-destructive",
  },
};

function getStatusConfig(status: string) {
  return (
    STATUS_CONFIG[status] || {
      label: status,
      icon: <CloudUpload className="h-4 w-4" />,
      className: "text-muted-foreground border-muted-foreground",
    }
  );
}

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatDuration(start: string, end: string): string {
  const startDate = new Date(start);
  const endDate = new Date(end);
  const durationMs = endDate.getTime() - startDate.getTime();

  if (durationMs < 1000) {
    return `${durationMs}ms`;
  } else if (durationMs < 60000) {
    return `${(durationMs / 1000).toFixed(1)}s`;
  } else if (durationMs < 3600000) {
    return `${Math.floor(durationMs / 60000)}m ${Math.floor(
      (durationMs % 60000) / 1000
    )}s`;
  } else {
    return `${Math.floor(durationMs / 3600000)}h ${Math.floor(
      (durationMs % 3600000) / 60000
    )}m`;
  }
}

interface TimelineItemProps {
  transition: StatusTransition;
  isLast: boolean;
  nextTimestamp?: string;
}

function TimelineItem({ transition, isLast, nextTimestamp }: TimelineItemProps) {
  const config = getStatusConfig(transition.toStatus);
  const duration = nextTimestamp
    ? formatDuration(transition.timestamp, nextTimestamp)
    : null;

  return (
    <div className="relative flex gap-4">
      {/* Timeline line */}
      {!isLast && (
        <div className="absolute left-[11px] top-6 h-full w-0.5 bg-border" />
      )}

      {/* Icon */}
      <div
        className={cn(
          "relative z-10 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 bg-background",
          config.className
        )}
      >
        {config.icon}
      </div>

      {/* Content */}
      <div className="flex-1 pb-4">
        <div className="flex items-center justify-between gap-2">
          <span className={cn("font-medium text-sm", config.className)}>
            {config.label}
          </span>
          {duration && (
            <span className="text-xs text-muted-foreground">
              Duration: {duration}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2 mt-1">
          <Clock className="h-3 w-3 text-muted-foreground" />
          <span className="text-xs text-muted-foreground">
            {formatTimestamp(transition.timestamp)}
          </span>
        </div>
        {transition.message && (
          <p className="text-xs text-muted-foreground mt-1">
            {transition.message}
          </p>
        )}
      </div>
    </div>
  );
}

export function StatusTimeline({ instanceId, className }: StatusTimelineProps) {
  const {
    data: timeline,
    isLoading,
    error,
  } = useQuery({
    queryKey: ["instanceTimeline", instanceId],
    queryFn: () => getInstanceStatusTimeline(instanceId!),
    enabled: !!instanceId,
    refetchInterval: 5000, // Refresh every 5 seconds for active deployments
    staleTime: 2000,
  });

  if (!instanceId) {
    return null;
  }

  if (isLoading) {
    return (
      <div
        className={cn(
          "rounded-lg border border-border bg-card overflow-hidden",
          className
        )}
      >
        <div className="px-4 py-3 border-b border-border">
          <h3 className="text-sm font-medium text-foreground">
            Deployment Timeline
          </h3>
        </div>
        <div className="px-4 py-8 flex items-center justify-center">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          <span className="ml-2 text-sm text-muted-foreground">
            Loading timeline...
          </span>
        </div>
      </div>
    );
  }

  if (error || !timeline) {
    return (
      <div
        className={cn(
          "rounded-lg border border-border bg-card overflow-hidden",
          className
        )}
      >
        <div className="px-4 py-3 border-b border-border">
          <h3 className="text-sm font-medium text-foreground">
            Deployment Timeline
          </h3>
        </div>
        <div className="px-4 py-4 text-sm text-muted-foreground">
          No timeline data available for this instance.
        </div>
      </div>
    );
  }

  const hasTimeline = timeline.timeline && timeline.timeline.length > 0;
  const currentStatus = timeline.currentStatus || "Unknown";
  const currentConfig = getStatusConfig(currentStatus);

  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-card overflow-hidden",
        className
      )}
    >
      <div className="px-4 py-3 border-b border-border">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-foreground">
            Deployment Timeline
          </h3>
          <div className="flex items-center gap-2">
            {timeline.isStuck && (
              <div className="flex items-center gap-1.5 text-status-warning">
                <AlertTriangle className="h-4 w-4" />
                <span className="text-xs font-medium">Stuck</span>
              </div>
            )}
            <div
              className={cn(
                "flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium",
                currentConfig.className,
                "bg-opacity-10"
              )}
              style={{
                backgroundColor: `color-mix(in srgb, currentColor 10%, transparent)`,
              }}
            >
              {currentConfig.icon}
              <span>{currentConfig.label}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Stuck instance warning */}
      {timeline.isStuck && (
        <div className="px-4 py-3 bg-status-warning/10 border-b border-status-warning/20">
          <div className="flex items-start gap-2">
            <AlertTriangle className="h-4 w-4 text-status-warning shrink-0 mt-0.5" />
            <div>
              <span className="text-sm font-medium text-status-warning">
                Instance appears stuck
              </span>
              <p className="text-xs text-status-warning/80 mt-0.5">
                This instance has been waiting for sync longer than expected.
                Check your GitOps controller (ArgoCD/Flux) for issues.
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="px-4 py-4">
        {hasTimeline ? (
          <div className="space-y-0">
            {timeline.timeline.map((transition, index) => (
              <TimelineItem
                key={`${transition.toStatus}-${transition.timestamp}`}
                transition={transition}
                isLast={index === timeline.timeline.length - 1}
                nextTimestamp={timeline.timeline[index + 1]?.timestamp}
              />
            ))}
          </div>
        ) : (
          <div className="flex flex-col items-center gap-3 py-4 text-center">
            <div
              className={cn(
                "flex h-10 w-10 items-center justify-center rounded-full border-2",
                currentConfig.className
              )}
            >
              {currentConfig.icon}
            </div>
            <div>
              <p className="text-sm font-medium text-foreground">
                {currentConfig.label}
              </p>
              {timeline.pushedAt && (
                <p className="text-xs text-muted-foreground mt-1">
                  Pushed at {formatTimestamp(timeline.pushedAt)}
                </p>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Deployment info footer */}
      {timeline.deploymentMode && (
        <div className="px-4 py-2 border-t border-border bg-secondary/30">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>Deployment Mode: {timeline.deploymentMode}</span>
            {timeline.instanceId && (
              <span className="font-mono">ID: {timeline.instanceId.slice(0, 8)}...</span>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
