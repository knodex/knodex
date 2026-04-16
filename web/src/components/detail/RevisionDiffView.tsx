// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { X, GitCompare, Plus, Minus, RefreshCw } from "@/lib/icons";
import { useMemo } from "react";
import yaml from "js-yaml";
import { useRGDRevision, useRGDRevisionDiff } from "@/hooks/useRGDs";

interface RevisionDiffViewProps {
  rgdName: string;
  rev1: number;
  rev2: number;
  onClose: () => void;
}

// --- Simple line-level diff ---

type LineKind = "same" | "added" | "removed";

interface DiffLine {
  left: string | null;
  leftKind: LineKind;
  right: string | null;
  rightKind: LineKind;
}

/** Compute longest common subsequence length table for two arrays of strings. */
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

/** Build side-by-side diff lines from two YAML string arrays. */
function buildSideBySide(oldLines: string[], newLines: string[]): DiffLine[] {
  const dp = lcsTable(oldLines, newLines);
  const result: DiffLine[] = [];

  let i = oldLines.length;
  let j = newLines.length;

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && oldLines[i - 1] === newLines[j - 1]) {
      result.push({ left: oldLines[i - 1], leftKind: "same", right: newLines[j - 1], rightKind: "same" });
      i--;
      j--;
    } else if (j > 0 && (i === 0 || dp[i][j - 1] >= dp[i - 1][j])) {
      result.push({ left: null, leftKind: "added", right: newLines[j - 1], rightKind: "added" });
      j--;
    } else {
      result.push({ left: oldLines[i - 1], leftKind: "removed", right: null, rightKind: "removed" });
      i--;
    }
  }

  return result.reverse();
}

// --- CSS variable colour helpers ---

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

// --- Sub-components ---

interface YAMLPanelProps {
  title: string;
  lines: Array<{ content: string | null; kind: LineKind }>;
}

function YAMLPanel({ title, lines }: YAMLPanelProps) {
  return (
    <div className="flex flex-col min-w-0 flex-1 border border-border rounded-md overflow-hidden">
      <div className="px-3 py-2 bg-muted/50 border-b border-border text-xs font-medium text-muted-foreground flex-shrink-0">
        {title}
      </div>
      <div className="overflow-auto flex-1">
        <pre className="text-xs font-mono leading-5 p-2 min-h-full">
          {lines.map((l, idx) => (
            <div
              key={idx}
              className={`flex gap-2 px-1 rounded-sm ${lineClassName(l.kind)}`}
            >
              <span className="select-none w-3 flex-shrink-0 opacity-60">
                {linePrefix(l.kind)}
              </span>
              <span className="whitespace-pre">{l.content ?? ""}</span>
            </div>
          ))}
        </pre>
      </div>
    </div>
  );
}

// --- Main component ---

export function RevisionDiffView({
  rgdName,
  rev1,
  rev2,
  onClose,
}: RevisionDiffViewProps) {

  const { data: diff, isLoading: diffLoading, error: diffError } = useRGDRevisionDiff(rgdName, rev1, rev2);
  const { data: leftRevision, isLoading: leftLoading } = useRGDRevision(rgdName, rev1);
  const { data: rightRevision, isLoading: rightLoading } = useRGDRevision(rgdName, rev2);

  const isLoading = diffLoading || leftLoading || rightLoading;
  const error = diffError;

  const sideBySide = useMemo<DiffLine[]>(() => {
    const oldSnap = leftRevision?.snapshot ?? {};
    const newSnap = rightRevision?.snapshot ?? {};

    const oldYaml = yaml.dump(oldSnap, { lineWidth: 120, noRefs: true });
    const newYaml = yaml.dump(newSnap, { lineWidth: 120, noRefs: true });

    const oldLines = oldYaml.split("\n");
    const newLines = newYaml.split("\n");

    // Trim trailing empty line that yaml.dump appends.
    if (oldLines[oldLines.length - 1] === "") oldLines.pop();
    if (newLines[newLines.length - 1] === "") newLines.pop();

    return buildSideBySide(oldLines, newLines);
  }, [leftRevision?.snapshot, rightRevision?.snapshot]);

  const leftLines = sideBySide.map((l) => ({ content: l.left, kind: l.leftKind }));
  const rightLines = sideBySide.map((l) => ({ content: l.right, kind: l.rightKind }));

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-border bg-card p-4">
      {/* Header */}
      <div className="flex items-center justify-between gap-3 flex-shrink-0">
        <div className="flex items-center gap-2 text-sm font-medium text-foreground">
          <GitCompare className="h-4 w-4 text-muted-foreground" />
          <span>
            Rev #{rev1} vs Rev #{rev2}
          </span>
          {isLoading && <RefreshCw className="h-3 w-3 animate-spin text-muted-foreground" />}
        </div>

        {/* Diff summary badges */}
        {diff && !diff.identical && (
          <div className="flex items-center gap-2 text-xs">
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
          </div>
        )}
        {diff?.identical && (
          <span className="text-xs text-muted-foreground">No differences</span>
        )}

        <button
          type="button"
          onClick={onClose}
          className="ml-auto flex-shrink-0 rounded p-1 hover:bg-muted text-muted-foreground hover:text-foreground transition-colors"
          aria-label="Close diff view"
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Error state */}
      {error && (
        <div className="text-xs text-destructive px-2 py-1 bg-destructive/10 rounded">
          Failed to load diff: {error instanceof Error ? error.message : "Unknown error"}
        </div>
      )}

      {/* Content: YAML diff */}
      {isLoading && !leftRevision?.snapshot && !rightRevision?.snapshot ? (
        <div className="h-64 rounded-md border border-border bg-muted/30 animate-token-shimmer" />
      ) : (
        <div className="flex gap-2 overflow-hidden" style={{ minHeight: "16rem", maxHeight: "32rem" }}>
          <YAMLPanel
            title={`Rev #${rev1} (old)`}
            lines={leftLines}
          />
          <YAMLPanel
            title={`Rev #${rev2} (new)`}
            lines={rightLines}
          />
        </div>
      )}
    </div>
  );
}
