// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

export function InstanceCardSkeleton() {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      {/* Header */}
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-3">
          <Skeleton className="h-9 w-9" />
          <div>
            <Skeleton className="h-4 w-32 mb-1" />
            <Skeleton className="h-3 w-20" />
          </div>
        </div>
        <Skeleton className="h-5 w-16 rounded-full" />
      </div>

      {/* RGD Info */}
      <Skeleton className="h-4 w-40 mb-3" />

      {/* Tags */}
      <div className="flex gap-1.5 mb-3">
        <Skeleton className="h-[22px] w-14" />
        <Skeleton className="h-[22px] w-18" />
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between pt-3">
        <Skeleton className="h-3 w-16" />
        <Skeleton className="h-3 w-20" />
      </div>
    </div>
  );
}
