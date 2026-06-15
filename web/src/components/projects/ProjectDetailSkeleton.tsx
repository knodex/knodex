// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Placeholder rendered while the project detail page is loading.
 *
 * Shape mirrors {@link ../../routes/ProjectDetail.tsx} (lines 152-204):
 * back-link, then a flat (no card) header strip with avatar tile + name +
 * description + meta line, then a TabBar (overview / instances / roles /
 * destinations / resources), then a min-h-[300px] tabpanel placeholder.
 */
export function ProjectDetailSkeleton() {
  return (
    <div className="space-y-0" data-testid="project-detail-skeleton">
      {/* Back link */}
      <Skeleton className="mb-6 h-4 w-32" />

      {/* Header — Vercel-style flat, no card wrapper */}
      <div className="mb-6 flex items-start gap-3">
        {/* 56×56 avatar tile (mirrors lines 166-176) */}
        <Skeleton className="h-14 w-14 shrink-0 rounded-2xl" />
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-7 w-64" />
          <Skeleton className="h-3.5 w-3/4" />
          <Skeleton className="h-3 w-56" />
        </div>
      </div>

      {/* Tab bar — BASE_TABS (overview/instances/roles/destinations) + optional resources */}
      <div className="flex items-center gap-2 border-b border-[var(--border-subtle)]">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="mb-[-1px] h-9 w-24 rounded-t-md" />
        ))}
      </div>

      {/* Tabpanel placeholder — reserves the same min-h-[300px] as real content */}
      <div className="pt-6 min-h-[300px] space-y-3">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-3.5 w-3/4" />
        <Skeleton className="h-3.5 w-2/3" />
        <Skeleton className="h-3.5 w-4/5" />
        <Skeleton className="h-3.5 w-1/2" />
      </div>
    </div>
  );
}
