// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Check, AlertCircle, Loader2 } from "@/lib/icons";
import { cn } from "@/lib/utils";

interface TimelineEntry {
  id: string;
  timestamp: string;
  resourceKind: string;
  resourceName: string;
  status: "creating" | "created" | "failed";
  message?: string;
}

interface DeployTimelineProps {
  /** Reserved for future WebSocket subscription (see STORY-382 comment). */
  instanceId?: string;
  instanceName: string;
  namespace: string;
  kind: string;
  rgdName?: string;
  className?: string;
}

export function DeployTimeline({
  instanceName,
  namespace,
  kind,
  rgdName,
  className,
}: DeployTimelineProps) {
  const navigate = useNavigate();
  const [entries, setEntries] = useState<TimelineEntry[]>([]);
  const [progress, setProgress] = useState({ total: 0, completed: 0 });
  const [deployStatus, setDeployStatus] = useState<"in_progress" | "complete" | "failed">("in_progress");
  const scrollRef = useRef<HTMLDivElement>(null);

  const progressPercent = progress.total > 0 ? (progress.completed / progress.total) * 100 : 0;

  // Simulate progress for now — real WebSocket subscription requires message handler API
  // STORY-382 added the server-side events; a future story should expose ws.onMessage to consumers
  useEffect(() => {
    // Simulate initial creating event
    const timer1 = setTimeout(() => {
      setEntries([{
        id: "1",
        timestamp: new Date().toISOString(),
        resourceKind: kind,
        resourceName: instanceName,
        status: "creating",
        message: "Creating instance...",
      }]);
      setProgress({ total: 1, completed: 0 });
    }, 500);

    // Simulate created event
    const timer2 = setTimeout(() => {
      setEntries((prev) => prev.map((e) =>
        e.id === "1" ? { ...e, status: "created" as const, message: "Instance created successfully" } : e
      ));
      setProgress({ total: 1, completed: 1 });
      setDeployStatus("complete");
    }, 2000);

    return () => {
      clearTimeout(timer1);
      clearTimeout(timer2);
    };
  }, [instanceName, kind]);

  // Auto-scroll to latest entry
  useEffect(() => {
    scrollRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [entries]);

  const handleViewInstance = useCallback(() => {
    navigate(`/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(instanceName)}`);
  }, [navigate, namespace, kind, instanceName]);

  const handleRedeploy = useCallback(() => {
    if (rgdName) {
      navigate(`/deploy/${encodeURIComponent(rgdName)}`);
    }
  }, [navigate, rgdName]);

  return (
    <div className={cn("space-y-4", className)} data-testid="deploy-timeline">
      {/* Progress bar */}
      <div className="h-1 rounded-full overflow-hidden bg-[var(--surface-elevated)]">
        <div
          className={cn(
            "h-full transition-all duration-500 rounded-full",
            deployStatus === "failed" ? "bg-[var(--status-error)]" : "bg-[var(--brand-primary)]",
          )}
          style={{ width: `${progressPercent}%` }}
        />
      </div>

      {/* Timeline entries */}
      <div className="max-h-[300px] overflow-y-auto space-y-1 px-1">
        {entries.map((entry) => (
          <div
            key={entry.id}
            className={cn(
              "flex items-start gap-3 rounded-md px-3 py-2 text-sm bg-white/[0.02]",
              entry.status === "failed" && "border-l-2 border-l-[var(--status-error)]"
            )}
          >
            {entry.status === "creating" && (
              <Loader2 className="h-4 w-4 mt-0.5 shrink-0 animate-spin text-[var(--brand-primary)]" />
            )}
            {entry.status === "created" && (
              <Check className="h-4 w-4 mt-0.5 shrink-0 text-[var(--status-healthy)]" />
            )}
            {entry.status === "failed" && (
              <AlertCircle className="h-4 w-4 mt-0.5 shrink-0 text-[var(--status-error)]" />
            )}
            <div className="min-w-0">
              <span className="text-[var(--text-primary)]">
                {entry.resourceKind} <span className="font-mono text-xs">{entry.resourceName}</span>
              </span>
              {entry.message && (
                <p className="text-xs mt-0.5 text-[var(--text-muted)]">{entry.message}</p>
              )}
            </div>
          </div>
        ))}
        <div ref={scrollRef} />
      </div>

      {/* Success state */}
      {deployStatus === "complete" && (
        <div className="flex flex-col items-center gap-3 py-6 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-[var(--status-healthy)]/10">
            <Check className="h-6 w-6 text-[var(--status-healthy)]" />
          </div>
          <div>
            <h3 className="font-semibold text-[var(--text-primary)]">Deployment successful</h3>
            <p className="text-sm text-[var(--text-muted)]">{instanceName}</p>
          </div>
          <button
            type="button"
            onClick={handleViewInstance}
            className="px-4 py-2 rounded-md text-sm font-medium bg-[var(--brand-primary)] text-[var(--surface-bg)]"
          >
            View Instance
          </button>
        </div>
      )}

      {/* Failure state */}
      {deployStatus === "failed" && (
        <div className="flex flex-col items-center gap-3 py-4">
          <button
            type="button"
            onClick={handleRedeploy}
            className="px-4 py-2 rounded-md text-sm font-medium border text-[var(--text-secondary)] border-white/10"
          >
            Redeploy with changes
          </button>
        </div>
      )}
    </div>
  );
}
