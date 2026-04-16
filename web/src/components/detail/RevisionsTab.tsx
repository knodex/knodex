// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { GitBranch, Hash, Clock, AlertCircle, GitCompare } from "@/lib/icons";
import { useState, useMemo } from "react";
import { useRGDRevisions } from "@/hooks/useRGDs";
import { Badge } from "@/components/ui/badge";
import { formatDistanceToNow, formatDateTime } from "@/lib/date";
import type { GraphRevisionCondition } from "@/types/rgd";
import { RevisionDiffView } from "./RevisionDiffView";

interface RevisionsTabProps {
  rgdName: string;
  /** The current active revision number from the parent RGD */
  currentRevision?: number;
}

export function RevisionsTab({ rgdName, currentRevision }: RevisionsTabProps) {
  const { data, isLoading, error } = useRGDRevisions(rgdName);

  // manualSelectedRevs: null means "use default (latest two)", otherwise user-chosen.
  const [manualSelectedRevs, setManualSelectedRevs] = useState<[number, number] | null>(null);
  const [showDiff, setShowDiff] = useState(false);

  const revisions = useMemo(() => data?.items ?? [], [data?.items]);

  // Derive the active selection: user choice OR default to latest two revisions.
  const selectedRevs = useMemo<[number, number] | null>(() => {
    if (manualSelectedRevs !== null) return manualSelectedRevs;
    if (revisions.length < 2) return null;
    const sorted = [...revisions].sort((a, b) => b.revisionNumber - a.revisionNumber);
    return [sorted[1].revisionNumber, sorted[0].revisionNumber];
  }, [manualSelectedRevs, revisions]);

  const toggleRevision = (num: number) => {
    if (!selectedRevs) {
      // Nothing selected yet — shouldn't happen when revisions >= 2 but handle gracefully.
      return;
    }
    const [r1, r2] = selectedRevs;

    if (num === r1 || num === r2) {
      // Deselect — find the other selected revision and pick the next available
      const other = num === r1 ? r2 : r1;
      const sorted = [...revisions].sort((a, b) => b.revisionNumber - a.revisionNumber);
      const next = sorted.find((r) => r.revisionNumber !== other && r.revisionNumber !== num);
      if (next) {
        const pair = [Math.min(other, next.revisionNumber), Math.max(other, next.revisionNumber)] as [number, number];
        setManualSelectedRevs(pair);
        setShowDiff(false);
      }
      return;
    }

    // Replace the revision that is farther from the new selection (swap one out).
    const diffR1 = Math.abs(num - r1);
    const diffR2 = Math.abs(num - r2);
    const newPair: [number, number] = diffR1 > diffR2
      ? [Math.min(num, r2), Math.max(num, r2)]
      : [Math.min(num, r1), Math.max(num, r1)];

    setManualSelectedRevs(newPair);
    setShowDiff(false);
  };

  const isSelected = (num: number) => selectedRevs !== null && (num === selectedRevs[0] || num === selectedRevs[1]);

  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div
            key={i}
            className="h-16 rounded-lg border border-border bg-card animate-token-shimmer"
          />
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <AlertCircle className="h-10 w-10 text-destructive mb-3" />
        <p className="text-sm font-medium text-foreground">Failed to load revisions</p>
        <p className="text-xs text-muted-foreground mt-1">
          {error instanceof Error ? error.message : "Unknown error"}
        </p>
      </div>
    );
  }

  if (revisions.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <GitBranch className="h-10 w-10 text-muted-foreground mb-3" />
        <p className="text-sm font-medium text-foreground">No revisions found</p>
        <p className="text-xs text-muted-foreground mt-1">
          GraphRevision history will appear here when available.
        </p>
      </div>
    );
  }

  const canCompare = revisions.length >= 2 && selectedRevs !== null;

  return (
    <div className="space-y-4">
      {/* Header with compare button */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <GitBranch className="h-4 w-4" />
          <span>
            {revisions.length} revision{revisions.length !== 1 ? "s" : ""}
          </span>
        </div>
        {canCompare && (
          <button
            type="button"
            data-testid="compare-button"
            onClick={() => setShowDiff((v) => !v)}
            className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted transition-colors"
          >
            <GitCompare className="h-3.5 w-3.5" />
            {showDiff ? "Hide Diff" : "Compare"}
            {selectedRevs && (
              <span className="text-muted-foreground">
                #{selectedRevs[0]} ↔ #{selectedRevs[1]}
              </span>
            )}
          </button>
        )}
      </div>

      {/* Revision list */}
      <div className="space-y-2">
        {revisions.map((rev) => {
          const isCurrent = currentRevision !== undefined && rev.revisionNumber === currentRevision;
          const selected = isSelected(rev.revisionNumber);

          return (
            <div
              key={rev.revisionNumber}
              data-testid={`revision-${rev.revisionNumber}`}
              className={`flex items-center justify-between gap-4 rounded-lg border px-4 py-3 transition-colors ${
                selected
                  ? "border-primary bg-primary/5"
                  : "border-border bg-card hover:border-muted-foreground/40"
              }`}
            >
              {/* Checkbox + revision number */}
              <div className="flex items-center gap-3 min-w-0">
                {revisions.length >= 2 && (
                  <input
                    type="checkbox"
                    data-testid={`revision-select-${rev.revisionNumber}`}
                    checked={selected}
                    onChange={() => toggleRevision(rev.revisionNumber)}
                    aria-label={`Select revision ${rev.revisionNumber} for comparison`}
                    className="h-4 w-4 rounded border-border accent-primary cursor-pointer"
                  />
                )}
                <span className="text-sm font-semibold text-foreground tabular-nums">
                  #{rev.revisionNumber}
                </span>
                {isCurrent && (
                  <Badge data-testid="current-badge" variant="default" className="text-xs">
                    Current
                  </Badge>
                )}
              </div>

              {/* Center: conditions */}
              <div className="flex items-center gap-1.5 flex-shrink-0">
                {rev.conditions?.map((cond) => (
                  <ConditionBadge key={cond.type} condition={cond} />
                ))}
              </div>

              {/* Right: timestamp + hash */}
              <div className="flex items-center gap-3 text-xs text-muted-foreground flex-shrink-0">
                {rev.contentHash && (
                  <span className="inline-flex items-center gap-1 font-mono" title={rev.contentHash}>
                    <Hash className="h-3 w-3" />
                    {rev.contentHash.slice(0, 7)}
                  </span>
                )}
                <span
                  className="inline-flex items-center gap-1"
                  title={formatDateTime(rev.createdAt)}
                >
                  <Clock className="h-3 w-3" />
                  {formatDistanceToNow(rev.createdAt)}
                </span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Inline diff view */}
      {showDiff && selectedRevs && (
        <RevisionDiffView
          rgdName={rgdName}
          rev1={selectedRevs[0]}
          rev2={selectedRevs[1]}
          onClose={() => setShowDiff(false)}
        />
      )}
    </div>
  );
}

function ConditionBadge({ condition }: { condition: GraphRevisionCondition }) {
  const isTrue = condition.status === "True";
  return (
    <Badge
      variant={isTrue ? "default" : "destructive"}
      className="text-xs"
      title={condition.message || condition.reason || ""}
    >
      {condition.type}
    </Badge>
  );
}
