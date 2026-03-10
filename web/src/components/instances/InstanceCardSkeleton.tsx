// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

export function InstanceCardSkeleton() {
  return (
    <div className="rounded-lg border border-border bg-card p-4 animate-shimmer">
      {/* Header */}
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-3">
          <div className="h-9 w-9 rounded-md bg-secondary" />
          <div>
            <div className="h-4 w-32 bg-secondary rounded mb-1" />
            <div className="h-3 w-20 bg-secondary rounded" />
          </div>
        </div>
        <div className="h-5 w-16 bg-secondary rounded-full" />
      </div>

      {/* RGD Info */}
      <div className="h-4 w-40 bg-secondary rounded mb-3" />

      {/* Tags */}
      <div className="flex gap-1.5 mb-3">
        <div className="h-[22px] w-14 bg-secondary rounded-md" />
        <div className="h-[22px] w-18 bg-secondary rounded-md" />
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between pt-3">
        <div className="h-3 w-16 bg-secondary rounded" />
        <div className="h-3 w-20 bg-secondary rounded" />
      </div>
    </div>
  );
}
