// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

export function StatusCardSkeleton() {
  return (
    <div
      data-testid="status-card-skeleton"
      className="rounded-[var(--radius-token-lg)] border border-[var(--border-default)] bg-[var(--surface-primary)]"
    >
      {/* Header */}
      <div className="flex items-center justify-between gap-2 px-3 pt-2.5 pb-1.5">
        <div className="flex items-center gap-1.5">
          <Skeleton className="h-2 w-2 rounded-full shrink-0" />
          <Skeleton className="h-3 w-28" />
        </div>
        <Skeleton className="h-3.5 w-12 rounded-[var(--radius-token-sm)]" />
      </div>

      {/* Body */}
      <div className="px-3 pb-2 space-y-1">
        <div className="flex items-center justify-between">
          <Skeleton className="h-2 w-12" />
          <Skeleton className="h-2 w-20" />
        </div>
        <div className="flex items-center justify-between">
          <Skeleton className="h-2 w-14" />
          <Skeleton className="h-2 w-16" />
        </div>
      </div>

      {/* Footer */}
      <div className="px-3 py-1.5" style={{ borderTop: "1px solid var(--border-subtle)" }}>
        <Skeleton className="h-2.5 w-24" />
      </div>
    </div>
  );
}
