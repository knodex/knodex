// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useRef, useEffect, useMemo, Fragment } from "react";
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
  GitBranch,
} from "@/lib/icons";
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
  RevisionChanged: GitBranch,
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
  RevisionChanged: "text-primary bg-primary/10 border-primary/20",
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
    case "RevisionChanged": return "Revision Changed";
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

/**
 * Compute proportional gap weights for ALL connectors including the "Now" gap.
 * Returns { eventGaps: number[], nowGap: number } where all weights share the
 * same scale so time proportions are visually accurate across the full rail.
 */
function computeAllGapWeights(timeline: TimelineEntry[]): { eventGaps: number[]; nowGap: number } {
  if (timeline.length === 0) return { eventGaps: [], nowGap: 0 };

  const timestamps = timeline.map((e) => new Date(e.timestamp).getTime());
  const now = Date.now();

  // Collect ALL gaps: between events + to "now"
  const allGaps: number[] = [];
  for (let i = 1; i < timestamps.length; i++) {
    allGaps.push(Math.max(timestamps[i] - timestamps[i - 1], 0));
  }
  const nowGapMs = Math.max(now - timestamps[timestamps.length - 1], 0);
  allGaps.push(nowGapMs);

  // Normalize all gaps together, with a floor of 0.05 so even near-zero gaps are visible
  const maxGap = Math.max(...allGaps, 1);
  const normalized = allGaps.map((g) => Math.max(g / maxGap, 0.05));

  return {
    eventGaps: normalized.slice(0, -1),
    nowGap: normalized[normalized.length - 1],
  };
}

// Minimum connector width in px
const MIN_GAP_PX = 20;

function TimelineNode({
  entry,
  index,
  isSelected,
  onSelect,
  nodeRef,
}: {
  entry: TimelineEntry;
  index: number;
  isSelected: boolean;
  onSelect: (index: number) => void;
  nodeRef?: React.Ref<HTMLButtonElement>;
}) {
  const Icon = EVENT_ICONS[entry.eventType] || Activity;
  const colorClass = EVENT_COLORS[entry.eventType] || EVENT_COLORS.StatusChanged;

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          ref={nodeRef}
          type="button"
          onClick={() => onSelect(index)}
          className="flex flex-col items-center shrink-0 gap-1.5 group focus:outline-none"
          style={{ width: 72 }}
          aria-label={`${getEventLabel(entry.eventType)}${entry.isCurrent ? " (current)" : ""}`}
        >
          {/* Node circle */}
          <div
            className={cn(
              "relative flex h-9 w-9 items-center justify-center rounded-full border-2 transition-all",
              colorClass,
              isSelected && "ring-2 ring-offset-2 ring-offset-background ring-primary scale-110",
              entry.isCurrent && !isSelected && "ring-2 ring-primary/50 ring-offset-1 ring-offset-background",
              !isSelected && "group-hover:scale-105",
            )}
          >
            <Icon
              className={cn(
                "h-4 w-4",
                entry.eventType === "Creating" && "animate-spin",
              )}
            />
          </div>

          {/* Label */}
          <span className={cn(
            "text-[10px] leading-tight text-center max-w-full truncate",
            isSelected ? "text-foreground font-medium" : "text-muted-foreground",
          )}>
            {getEventLabel(entry.eventType)}
          </span>

          {/* Relative time */}
          <span className="text-[9px] text-muted-foreground/70 leading-none">
            {getRelativeTime(entry.timestamp)}
          </span>
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs">
        <p className="font-medium">{getEventLabel(entry.eventType)}</p>
        <p className="text-xs text-muted-foreground">{formatTimestamp(entry.timestamp)}</p>
        {entry.user && <p className="text-xs">by {entry.user}</p>}
        {entry.message && <p className="text-xs mt-1">{entry.message}</p>}
      </TooltipContent>
    </Tooltip>
  );
}

function TimelineConnector({ weight, dashed }: { weight: number; dashed?: boolean }) {
  return (
    <div
      className="self-start flex items-center"
      style={{ flexGrow: weight, flexShrink: 0, flexBasis: MIN_GAP_PX, marginTop: 16 /* center on the 36px node circle */ }}
    >
      <div className={cn("h-px w-full", dashed ? "border-t border-dashed border-muted-foreground/30" : "bg-border")} />
    </div>
  );
}

function NowMarker() {
  return (
    <div className="flex flex-col items-center shrink-0 gap-1.5" style={{ width: 40 }}>
      {/* Pulsing "now" dot */}
      <div className="relative flex h-9 w-9 items-center justify-center">
        <div className="absolute h-3 w-3 rounded-full bg-primary/20 animate-ping" />
        <div className="h-2.5 w-2.5 rounded-full bg-primary" />
      </div>
      <span className="text-[10px] font-medium text-primary leading-tight">Now</span>
    </div>
  );
}

function DetailCard({ entry }: { entry: TimelineEntry }) {
  const Icon = EVENT_ICONS[entry.eventType] || Activity;
  const colorClass = EVENT_COLORS[entry.eventType] || EVENT_COLORS.StatusChanged;

  return (
    <div className="mt-3 mx-4 rounded-lg border border-border bg-secondary/30 p-3 animate-in fade-in-0 slide-in-from-top-1 duration-150">
      <div className="flex items-center gap-3">
        <div
          className={cn(
            "flex h-7 w-7 shrink-0 items-center justify-center rounded-full border",
            colorClass,
          )}
        >
          <Icon className={cn("h-3.5 w-3.5", entry.eventType === "Creating" && "animate-spin")} />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm text-foreground">
              {getEventLabel(entry.eventType)}
            </span>
            {entry.isCompleted && (
              <CheckCircle2 className="h-3.5 w-3.5 text-muted-foreground" />
            )}
          </div>
          <div className="flex items-center gap-2 mt-0.5 text-xs text-muted-foreground">
            <Clock className="h-3 w-3" />
            <span>{formatTimestamp(entry.timestamp)}</span>
            {entry.user && (
              <>
                <span className="text-border">|</span>
                <User className="h-3 w-3" />
                <span>{entry.user}</span>
              </>
            )}
          </div>
        </div>
      </div>

      {entry.message && (
        <p className="mt-2 text-sm text-muted-foreground pl-10">
          {entry.message}
        </p>
      )}

      {entry.gitCommitUrl && (
        <a
          href={entry.gitCommitUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 mt-2 pl-10 text-xs text-primary hover:underline"
        >
          <GitCommit className="h-3 w-3" />
          View commit
          <ExternalLink className="h-3 w-3" />
        </a>
      )}
    </div>
  );
}

export function DeploymentTimeline({ namespace, kind, name }: DeploymentTimelineProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const [exportFormat, setExportFormat] = useState<HistoryExportFormat>("json");

  const { data: timelineData, isLoading, error, refetch } = useInstanceTimeline(namespace, kind, name);
  const exportHistory = useExportHistory();

  const timeline = useMemo(() => timelineData?.timeline ?? [], [timelineData?.timeline]);

  const { eventGaps: gapWeights, nowGap: nowWeight } = useMemo(
    () => computeAllGapWeights(timeline),
    [timeline],
  );

  // Derive the default selected index without a setState-in-effect
  const defaultIndex = useMemo(() => {
    if (timeline.length === 0) return null;
    const idx = timeline.findIndex((e) => e.isCurrent);
    return idx >= 0 ? idx : timeline.length - 1;
  }, [timeline]);
  const resolvedIndex = selectedIndex ?? defaultIndex;

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const selectedNodeRef = useRef<HTMLButtonElement>(null);

  // Auto-scroll to the selected node
  useEffect(() => {
    if (selectedNodeRef.current && scrollContainerRef.current) {
      const container = scrollContainerRef.current;
      const node = selectedNodeRef.current;
      const nodeLeft = node.offsetLeft;
      const nodeWidth = node.offsetWidth;
      const containerWidth = container.clientWidth;
      const scrollTarget = nodeLeft - containerWidth / 2 + nodeWidth / 2;
      container.scrollTo?.({ left: scrollTarget, behavior: "smooth" });
    }
  }, [resolvedIndex]);

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

  const selectedEntry = resolvedIndex !== null ? timeline[resolvedIndex] : null;

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
          {timeline.length === 0 ? (
            <div className="text-center py-6 text-muted-foreground">
              <Clock className="h-8 w-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">No deployment history available</p>
            </div>
          ) : (
            <>
              {/* Horizontal timeline rail */}
              <div
                ref={scrollContainerRef}
                className="overflow-x-auto py-4"
              >
                <div className="flex items-start px-6" style={{ minWidth: "min-content" }}>
                  {timeline.map((entry, index) => (
                    <Fragment key={`${entry.timestamp}-${entry.eventType}-${index}`}>
                      <TimelineNode
                        entry={entry}
                        index={index}
                        isSelected={resolvedIndex === index}
                        onSelect={setSelectedIndex}
                        nodeRef={resolvedIndex === index ? selectedNodeRef : undefined}
                      />
                      {index < timeline.length - 1 && (
                        <TimelineConnector weight={gapWeights[index]} />
                      )}
                    </Fragment>
                  ))}
                  {/* "Now" marker at the end of the rail */}
                  <TimelineConnector weight={nowWeight} dashed />
                  <NowMarker />
                </div>
              </div>

              {/* Detail card for selected event */}
              {selectedEntry && <DetailCard entry={selectedEntry} />}
            </>
          )}

          {/* Export section */}
          {timeline.length > 0 && (
            <div className="px-4 py-3 mt-2 border-t border-border bg-secondary/30 flex items-center justify-between gap-4">
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
