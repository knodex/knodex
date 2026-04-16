// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Skeleton loader for ProjectCard component
 */
export function ProjectCardSkeleton() {
  return (
    <div className="flex items-center justify-between p-4 rounded-lg border border-border">
      <div className="flex items-center gap-4 flex-1 min-w-0">
        {/* Icon Skeleton */}
        <Skeleton className="h-10 w-10 rounded-full flex-shrink-0" />

        {/* Project Info Skeleton */}
        <div className="flex-1 min-w-0 space-y-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-48" />
          <Skeleton className="h-3 w-40" />
        </div>

        {/* Badges Skeleton */}
        <div className="flex items-center gap-2 flex-shrink-0">
          <Skeleton className="h-6 w-20" />
          <Skeleton className="h-6 w-20" />
          <Skeleton className="h-6 w-24" />
        </div>
      </div>

      {/* Actions Skeleton */}
      <div className="flex items-center gap-2 ml-4">
        <Skeleton className="h-8 w-8" />
        <Skeleton className="h-8 w-8" />
      </div>
    </div>
  );
}
