// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import {
  Clock,
  CheckCircle2,
  AlertCircle,
  GitCommit,
  Download,
  ChevronDown,
  ChevronUp,
  User,
  RefreshCw,
  Loader2,
  XCircle,
  PlusCircle,
  FileCode,
  Trash2,
  Edit,
  Activity,
  AlertTriangle,
  ExternalLink,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useInstanceTimeline, useExportHistory } from "@/hooks/useHistory";
import type { TimelineEntry, DeploymentEventType, HistoryExportFormat } from "@/types/history";
import { cn } from "@/lib/utils";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

interface DeploymentTimelineProps {
  namespace: string;
  kind: string;
  name: string;
}

const EVENT_ICONS: Record<DeploymentEventType, React.ComponentType<{ className?: string }>> = {
  Created: PlusCircle,
  ManifestGenerated: FileCode,
  PushedToGit: GitCommit,
  WaitingForSync: Clock,
  Synced: RefreshCw,
  Creating: Loader2,
  Ready: CheckCircle2,
  Degraded: AlertTriangle,
  Failed: XCircle,
  Deleted: Trash2,
  Updated: Edit,
  StatusChanged: Activity,
};

const EVENT_COLORS: Record<DeploymentEventType, string> = {
  Created: "text-status-info bg-status-info/10 border-status-info/20",
  ManifestGenerated: "text-primary bg-primary/10 border-primary/20",
  PushedToGit: "text-status-warning bg-status-warning/10 border-status-warning/20",
  WaitingForSync: "text-status-warning bg-status-warning/10 border-status-warning/20",
  Synced: "text-status-pending bg-status-pending/10 border-status-pending/20",
  Creating: "text-status-info bg-status-info/10 border-status-info/20",
  Ready: "text-status-success bg-status-success/10 border-status-success/20",
  Degraded: "text-status-warning bg-status-warning/10 border-status-warning/20",
  Failed: "text-destructive bg-destructive/10 border-destructive/20",
  Deleted: "text-muted-foreground bg-secondary border-border",
  Updated: "text-status-info bg-status-info/10 border-status-info/20",
  StatusChanged: "text-primary bg-primary/10 border-primary/20",
};

function getEventLabel(eventType: DeploymentEventType): string {
  switch (eventType) {
    case "Created": return "Created";
    case "ManifestGenerated": return "Manifest Generated";
    case "PushedToGit": return "Pushed to Git";
    case "WaitingForSync": return "Waiting for Sync";
    case "Synced": return "Synced";
    case "Creating": return "Creating";
    case "Ready": return "Ready";
    case "Degraded": return "Degraded";
    case "Failed": return "Failed";
    case "Deleted": return "Deleted";
    case "Updated": return "Updated";
    case "StatusChanged": return "Status Changed";
    default: return eventType;
  }
}

function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  return date.toLocaleString();
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
  return "just now";
}

function TimelineItem({ entry, isLast }: { entry: TimelineEntry; isLast: boolean }) {
  const Icon = EVENT_ICONS[entry.eventType] || Activity;
  const colorClass = EVENT_COLORS[entry.eventType] || EVENT_COLORS.StatusChanged;

  return (
    <div className="relative flex gap-4 pb-6 last:pb-0">
      {/* Connector line */}
      {!isLast && (
        <div className="absolute left-[17px] top-10 bottom-0 w-px bg-border" />
      )}

      {/* Icon */}
      <div
        className={cn(
          "relative z-10 flex h-9 w-9 shrink-0 items-center justify-center rounded-full border",
          colorClass,
          entry.isCurrent && "ring-2 ring-primary ring-offset-2 ring-offset-background"
        )}
      >
        <Icon className={cn(
          "h-4 w-4",
          entry.eventType === "Creating" && "animate-spin"
        )} />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 pt-0.5">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-medium text-sm text-foreground">
            {getEventLabel(entry.eventType)}
          </span>
          {entry.isCurrent && (
            <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-primary/10 text-primary">
              Current
            </span>
          )}
          {entry.isCompleted && !entry.isCurrent && (
            <CheckCircle2 className="h-3.5 w-3.5 text-muted-foreground" />
          )}
        </div>

        <div className="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
          <Clock className="h-3 w-3" />
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="cursor-default">
                {getRelativeTime(entry.timestamp)}
              </span>
            </TooltipTrigger>
            <TooltipContent>
              <p>{formatTimestamp(entry.timestamp)}</p>
            </TooltipContent>
          </Tooltip>
          {entry.user && (
            <>
              <span className="text-border">|</span>
              <User className="h-3 w-3" />
              <span>{entry.user}</span>
            </>
          )}
        </div>

        {entry.message && (
          <p className="mt-1.5 text-sm text-muted-foreground">
            {entry.message}
          </p>
        )}

        {entry.gitCommitUrl && (
          <a
            href={entry.gitCommitUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 mt-2 text-xs text-primary hover:underline"
          >
            <GitCommit className="h-3 w-3" />
            View commit
            <ExternalLink className="h-3 w-3" />
          </a>
        )}
      </div>
    </div>
  );
}

export function DeploymentTimeline({ namespace, kind, name }: DeploymentTimelineProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [exportFormat, setExportFormat] = useState<HistoryExportFormat>("json");

  const { data: timelineData, isLoading, error, refetch } = useInstanceTimeline(namespace, kind, name);
  const exportHistory = useExportHistory();

  const handleExport = async () => {
    try {
      await exportHistory.mutateAsync({ namespace, kind, name, format: exportFormat });
    } catch {
      // Error handled by mutation
    }
  };

  if (isLoading) {
    return (
      <div className="rounded-lg border border-border bg-card">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 className="text-sm font-medium text-foreground">Deployment History</h3>
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
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 className="text-sm font-medium text-foreground">Deployment History</h3>
        </div>
        <div className="p-6 flex items-center gap-3 text-muted-foreground">
          <AlertCircle className="h-5 w-5" />
          <div>
            <p className="text-sm">Failed to load deployment history</p>
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

  const timeline = timelineData?.timeline || [];

  return (
    <div className="rounded-lg border border-border bg-card overflow-hidden">
      {/* Header */}
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full px-4 py-3 border-b border-border flex items-center justify-between hover:bg-secondary/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-medium text-foreground">Deployment History</h3>
          <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-secondary text-muted-foreground">
            {timeline.length} events
          </span>
        </div>
        {isExpanded ? (
          <ChevronUp className="h-4 w-4 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-4 w-4 text-muted-foreground" />
        )}
      </button>

      {isExpanded && (
        <>
          {/* Timeline */}
          <div className="p-4">
            {timeline.length === 0 ? (
              <div className="text-center py-6 text-muted-foreground">
                <Clock className="h-8 w-8 mx-auto mb-2 opacity-50" />
                <p className="text-sm">No deployment history available</p>
              </div>
            ) : (
              <div className="space-y-0">
                {timeline.map((entry, index) => (
                  <TimelineItem
                    key={`${entry.timestamp}-${entry.eventType}`}
                    entry={entry}
                    isLast={index === timeline.length - 1}
                  />
                ))}
              </div>
            )}
          </div>

          {/* Export section */}
          {timeline.length > 0 && (
            <div className="px-4 py-3 border-t border-border bg-secondary/30 flex items-center justify-between gap-4">
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">Export as:</span>
                <select
                  value={exportFormat}
                  onChange={(e) => setExportFormat(e.target.value as HistoryExportFormat)}
                  className="text-xs border border-border rounded px-2 py-1 bg-background text-foreground"
                >
                  <option value="json">JSON</option>
                  <option value="csv">CSV</option>
                </select>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={handleExport}
                disabled={exportHistory.isPending}
                className="gap-1.5"
              >
                {exportHistory.isPending ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Download className="h-3.5 w-3.5" />
                )}
                Download
              </Button>
            </div>
          )}

          {/* Export error */}
          {exportHistory.isError && (
            <div className="px-4 py-2 border-t border-destructive/20 bg-destructive/5 flex items-center gap-2">
              <AlertCircle className="h-4 w-4 text-destructive" />
              <span className="text-xs text-destructive">
                Failed to export history
              </span>
            </div>
          )}
        </>
      )}
    </div>
  );
}
