// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { Link } from "react-router-dom";
import { GitCompare, Plus, Minus, RefreshCw, ExternalLink, Info } from "@/lib/icons";
import yaml from "js-yaml";
import {
  Sheet,
  SheetContent,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import { useRGDRevisionDiff, useRGDRevision } from "@/hooks/useRGDs";

interface RevisionDiffDrawerProps {
  rgdName: string;
  currentRevision: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type LineKind = "same" | "added" | "removed";

/** Compute LCS length table for two string arrays. */
function lcsTable(a: string[], b: string[]): number[][] {
  const m = a.length;
  const n = b.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1]);
    }
  }
  return dp;
}

/** Build unified diff lines from two string arrays. */
function buildUnifiedDiff(oldLines: string[], newLines: string[]): Array<{ content: string; kind: LineKind }> {
  const dp = lcsTable(oldLines, newLines);
  const result: Array<{ content: string; kind: LineKind }> = [];

  let i = oldLines.length;
  let j = newLines.length;

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && oldLines[i - 1] === newLines[j - 1]) {
      result.push({ content: oldLines[i - 1], kind: "same" });
      i--;
      j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      result.push({ content: newLines[j - 1], kind: "added" });
      j--;
    } else {
      result.push({ content: oldLines[i - 1], kind: "removed" });
      i--;
    }
  }

  return result.reverse();
}

function lineClassName(kind: LineKind): string {
  switch (kind) {
    case "added":
      return "bg-[hsl(var(--diff-added,142_71%_45%)/0.15)] text-[hsl(var(--diff-added,142_71%_45%))]";
    case "removed":
      return "bg-[hsl(var(--diff-removed,0_84%_60%)/0.15)] text-[hsl(var(--diff-removed,0_84%_60%))]";
    default:
      return "";
  }
}

function linePrefix(kind: LineKind): string {
  switch (kind) {
    case "added":
      return "+";
    case "removed":
      return "-";
    default:
      return " ";
  }
}

export function RevisionDiffDrawer({
  rgdName,
  currentRevision,
  open,
  onOpenChange,
}: RevisionDiffDrawerProps) {
  const isInitialRevision = currentRevision === 1;
  const prevRevision = isInitialRevision ? null : currentRevision - 1;

  const {
    data: diff,
    isLoading: diffLoading,
    error: diffError,
  } = useRGDRevisionDiff(
    rgdName,
    open && !isInitialRevision ? prevRevision : null,
    open && !isInitialRevision ? currentRevision : null,
  );

  const {
    data: currentRevData,
    isLoading: currentRevLoading,
  } = useRGDRevision(rgdName, open ? currentRevision : null);

  const {
    data: prevRevData,
    isLoading: prevRevLoading,
  } = useRGDRevision(rgdName, open ? prevRevision : null);

  const isLoading = isInitialRevision
    ? currentRevLoading
    : diffLoading || currentRevLoading || prevRevLoading;

  const error = diffError;

  const prevSnapshot = prevRevData?.snapshot;
  const currentSnapshot = currentRevData?.snapshot;

  const unifiedLines = useMemo(() => {
    if (isInitialRevision || !prevSnapshot || !currentSnapshot) return [];
    const oldYaml = yaml.dump(prevSnapshot, { lineWidth: 120, noRefs: true });
    const newYaml = yaml.dump(currentSnapshot, { lineWidth: 120, noRefs: true });
    const oldLines = oldYaml.split("\n");
    const newLines = newYaml.split("\n");
    if (oldLines[oldLines.length - 1] === "") oldLines.pop();
    if (newLines[newLines.length - 1] === "") newLines.pop();
    return buildUnifiedDiff(oldLines, newLines);
  }, [isInitialRevision, prevSnapshot, currentSnapshot]);

  const snapshotYaml = useMemo(() => {
    if (!isInitialRevision || !currentSnapshot) return "";
    return yaml.dump(currentSnapshot, { lineWidth: 120, noRefs: true });
  }, [isInitialRevision, currentSnapshot]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <GitCompare className="h-4 w-4" />
            Revision Changes
          </SheetTitle>
          <SheetDescription>
            {isInitialRevision
              ? `Rev ${currentRevision} (initial)`
              : `Rev ${prevRevision} → Rev ${currentRevision}`}
          </SheetDescription>
        </SheetHeader>

        <div className="mt-6 space-y-4">
          {/* Loading state */}
          {isLoading && (
            <div className="space-y-3" data-testid="diff-loading">
              <div className="h-4 w-48 rounded bg-muted animate-pulse" />
              <div className="h-64 rounded-md border bg-muted/30 animate-pulse" />
            </div>
          )}

          {/* Error state */}
          {error && !isLoading && (
            <div className="text-sm text-destructive px-3 py-2 bg-destructive/10 rounded-md" data-testid="diff-error">
              Failed to load diff: {error instanceof Error ? error.message : "Unknown error"}
            </div>
          )}

          {/* Initial revision — no diff available */}
          {!isLoading && !error && isInitialRevision && (
            <>
              <div className="flex items-start gap-2 rounded-md border px-3 py-2 text-sm text-muted-foreground bg-muted/30" data-testid="initial-revision-notice">
                <Info className="h-4 w-4 mt-0.5 flex-shrink-0" />
                <span>This is the initial revision — no previous revision to compare.</span>
              </div>
              {snapshotYaml && (
                <div className="rounded-md border overflow-hidden">
                  <div className="px-3 py-2 bg-muted/50 border-b text-xs font-medium text-muted-foreground">
                    Rev #{currentRevision} — Full Spec
                  </div>
                  <div className="overflow-auto max-h-[60vh]">
                    <pre className="text-xs font-mono leading-5 p-3" data-testid="snapshot-yaml">
                      {snapshotYaml}
                    </pre>
                  </div>
                </div>
              )}
            </>
          )}

          {/* Structured diff view */}
          {!isLoading && !error && !isInitialRevision && diff && (
            <>
              {/* Diff summary badges */}
              <div className="flex items-center gap-3 text-xs">
                {diff.identical && (
                  <span className="text-muted-foreground">No differences</span>
                )}
                {!diff.identical && (
                  <>
                    {diff.added.length > 0 && (
                      <span className="inline-flex items-center gap-1 text-[hsl(var(--diff-added,142_71%_45%))]">
                        <Plus className="h-3 w-3" />
                        {diff.added.length} added
                      </span>
                    )}
                    {diff.removed.length > 0 && (
                      <span className="inline-flex items-center gap-1 text-[hsl(var(--diff-removed,0_84%_60%))]">
                        <Minus className="h-3 w-3" />
                        {diff.removed.length} removed
                      </span>
                    )}
                    {diff.modified.length > 0 && (
                      <span className="inline-flex items-center gap-1 text-[hsl(var(--diff-modified,38_92%_50%))]">
                        <RefreshCw className="h-3 w-3" />
                        {diff.modified.length} modified
                      </span>
                    )}
                  </>
                )}
              </div>

              {/* Unified YAML diff */}
              {unifiedLines.length > 0 && (
                <div className="rounded-md border overflow-hidden">
                  <div className="px-3 py-2 bg-muted/50 border-b text-xs font-medium text-muted-foreground">
                    YAML Changes
                  </div>
                  <div className="overflow-auto max-h-[60vh]">
                    <pre className="text-xs font-mono leading-5 p-2">
                      {unifiedLines.map((l, idx) => (
                        <div
                          key={idx}
                          className={`flex gap-2 px-1 rounded-sm ${lineClassName(l.kind)}`}
                        >
                          <span className="select-none w-3 flex-shrink-0 opacity-60">
                            {linePrefix(l.kind)}
                          </span>
                          <span className="whitespace-pre">{l.content}</span>
                        </div>
                      ))}
                    </pre>
                  </div>
                </div>
              )}
            </>
          )}

          <Separator />
        </div>

        <SheetFooter className="mt-4">
          {/* Open in Revision Explorer link */}
          <Link
            to={`/catalog/${encodeURIComponent(rgdName)}?tab=revisions`}
            onClick={() => onOpenChange(false)}
            className="inline-flex items-center gap-1.5 text-sm text-primary hover:underline"
            data-testid="revision-explorer-link"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            Open in Revision Explorer
          </Link>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
