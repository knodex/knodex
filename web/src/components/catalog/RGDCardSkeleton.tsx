// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

export function RGDCardSkeleton() {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-3 mb-3">
        <div className="flex items-center gap-3">
          <Skeleton className="h-9 w-9" />
          <div className="space-y-2">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-3 w-20" />
          </div>
        </div>
        <Skeleton className="h-5 w-12" />
      </div>

      <div className="space-y-2 mb-3">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </div>

      <div className="flex gap-1.5 mb-3">
        <Skeleton className="h-[22px] w-14" />
        <Skeleton className="h-[22px] w-10" />
      </div>

      <div className="flex justify-between pt-3">
        <Skeleton className="h-3 w-20" />
        <Skeleton className="h-3 w-16" />
      </div>
    </div>
  );
}
