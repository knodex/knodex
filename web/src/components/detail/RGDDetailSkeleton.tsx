// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Placeholder rendered while RGDDetailView is loading.
 *
 * Shape mirrors {@link ./RGDDetailView.tsx} (see lines 138-212): rounded
 * header card with icon + title + meta strip + action buttons, a tag row,
 * then a TabBar and a min-h-[300px] tabpanel. Keeping the geometry identical
 * avoids the layout reflow users see when swapping a centered spinner for
 * the multi-tab detail view.
 */
export function RGDDetailSkeleton() {
  return (
    <div className="space-y-6" data-testid="rgd-detail-skeleton">
      {/* ── Header card ── */}
      <div className="rounded-lg border border-border bg-card p-6">
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4">
          <div className="flex items-start gap-4">
            {/* Icon tile (mirrors line 144) */}
            <Skeleton className="h-12 w-12 rounded-lg shrink-0" />
            <div className="space-y-2">
              {/* Title + version pill (mirrors lines 148-158) */}
              <div className="flex items-center gap-3">
                <Skeleton className="h-7 w-56" />
                <Skeleton className="h-5 w-16 rounded-full" />
              </div>
              {/* Meta strip — group/kind/owner badges (mirrors line 159) */}
              <div className="flex flex-wrap items-center gap-2 pt-1">
                <Skeleton className="h-5 w-24 rounded-full" />
                <Skeleton className="h-5 w-20 rounded-full" />
                <Skeleton className="h-5 w-28 rounded-full" />
              </div>
            </div>
          </div>
          {/* Action buttons (mirrors line 170) */}
          <div className="flex items-center gap-2 shrink-0">
            <Skeleton className="h-9 w-24 rounded-md" />
            <Skeleton className="h-9 w-28 rounded-md" />
          </div>
        </div>
        {/* Tag row (mirrors line 192) */}
        <div className="flex flex-wrap gap-2 mt-4">
          <Skeleton className="h-5 w-16 rounded-full" />
          <Skeleton className="h-5 w-20 rounded-full" />
          <Skeleton className="h-5 w-14 rounded-full" />
        </div>
      </div>

      {/* ── Tab bar — six pill placeholders for BASE_TABS + conditional tabs ── */}
      <div className="flex items-center gap-2 border-b border-[var(--border-subtle)]">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="mb-[-1px] h-9 w-24 rounded-t-md" />
        ))}
      </div>

      {/* ── Tab content block — reserves real tabpanel min-height ── */}
      <div className="min-h-[300px] space-y-3">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-3.5 w-3/4" />
        <Skeleton className="h-3.5 w-2/3" />
        <Skeleton className="h-3.5 w-4/5" />
        <Skeleton className="h-3.5 w-1/2" />
      </div>
    </div>
  );
}
