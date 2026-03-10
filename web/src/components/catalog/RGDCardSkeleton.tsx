// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

export function RGDCardSkeleton() {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-3">
          <div className="h-9 w-9 rounded-md bg-muted animate-shimmer" />
          <div className="space-y-2">
            <div className="h-4 w-28 rounded bg-muted animate-shimmer" />
            <div className="h-3 w-20 rounded bg-muted animate-shimmer" />
          </div>
        </div>
        <div className="h-5 w-12 rounded bg-muted animate-shimmer" />
      </div>

      <div className="space-y-2 mb-3">
        <div className="h-4 w-full rounded bg-muted animate-shimmer" />
        <div className="h-4 w-3/4 rounded bg-muted animate-shimmer" />
      </div>

      <div className="flex gap-1.5 mb-3">
        <div className="h-[22px] w-14 rounded-md bg-muted animate-shimmer" />
        <div className="h-[22px] w-10 rounded-md bg-muted animate-shimmer" />
      </div>

      <div className="flex justify-between pt-3">
        <div className="h-3 w-20 rounded bg-muted animate-shimmer" />
        <div className="h-3 w-16 rounded bg-muted animate-shimmer" />
      </div>
    </div>
  );
}
