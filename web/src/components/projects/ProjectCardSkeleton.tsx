// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Skeleton loader for ProjectCard component
 */
export function ProjectCardSkeleton() {
  return (
    <div className="flex items-center justify-between p-4 rounded-lg border border-border">
      <div className="flex items-center gap-4 flex-1 min-w-0">
        {/* Icon Skeleton */}
        <div className="flex h-10 w-10 rounded-full bg-muted animate-shimmer flex-shrink-0" />

        {/* Project Info Skeleton */}
        <div className="flex-1 min-w-0 space-y-2">
          <div className="h-4 w-32 rounded bg-muted animate-shimmer" />
          <div className="h-3 w-48 rounded bg-muted animate-shimmer" />
          <div className="h-3 w-40 rounded bg-muted animate-shimmer" />
        </div>

        {/* Badges Skeleton */}
        <div className="flex items-center gap-2 flex-shrink-0">
          <div className="h-6 w-20 rounded bg-muted animate-shimmer" />
          <div className="h-6 w-20 rounded bg-muted animate-shimmer" />
          <div className="h-6 w-24 rounded bg-muted animate-shimmer" />
        </div>
      </div>

      {/* Actions Skeleton */}
      <div className="flex items-center gap-2 ml-4">
        <div className="h-8 w-8 rounded bg-muted animate-shimmer" />
        <div className="h-8 w-8 rounded bg-muted animate-shimmer" />
      </div>
    </div>
  );
}
