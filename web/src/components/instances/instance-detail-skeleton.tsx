// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Placeholder rendered while InstanceDetailView is loading.
 *
 * Shape mirrors {@link ./InstanceDetailView.tsx} so the layout doesn't shift
 * when data lands:
 *   1. Rounded "Instance details" card — title bar (heading + action buttons)
 *      then a side-by-side identity strip, then a one-line source row.
 *   2. TabBar row — six pill placeholders matching {@link ./hooks/useInstanceTabs}.
 *   3. min-h-[300px] tabpanel block — reserves vertical space for the active
 *      tab's content so the scroll position doesn't jump.
 */
export function InstanceDetailSkeleton() {
  return (
    <div className="space-y-0" data-testid="instance-detail-skeleton">
      {/* ── Instance details card ── */}
      <div
        className="rounded-lg border"
        style={{
          borderColor: "var(--border-default)",
          background: "var(--surface-primary)",
        }}
      >
        {/* Title bar + action buttons (mirrors InstanceDetailView lines 97-113) */}
        <div
          className="flex items-center justify-between px-6 py-4"
          style={{ borderBottom: "1px solid var(--border-subtle)" }}
        >
          <Skeleton className="h-4 w-32" />
          <div className="flex items-center gap-2">
            <Skeleton className="h-8 w-8 rounded-md" />
            <Skeleton className="h-8 w-8 rounded-md" />
            <Skeleton className="h-8 w-20 rounded-md" />
          </div>
        </div>

        {/* Identity + key metadata row (mirrors InstanceHeaderCard) */}
        <div
          className="grid grid-cols-1 lg:grid-cols-2 gap-6 px-6 py-5"
          style={{ borderBottom: "1px solid var(--border-subtle)" }}
        >
          <div className="space-y-3">
            <Skeleton className="h-5 w-48" />
            <Skeleton className="h-3.5 w-36" />
            <Skeleton className="h-3.5 w-40" />
          </div>
          <div className="space-y-3">
            <Skeleton className="h-3.5 w-3/4" />
            <Skeleton className="h-3.5 w-2/3" />
            <Skeleton className="h-3.5 w-1/2" />
          </div>
        </div>

        {/* Source row — git info / direct mode indicator */}
        <div className="flex items-center gap-3 px-6 py-3">
          <Skeleton className="h-3.5 w-4 rounded-sm" />
          <Skeleton className="h-3.5 w-64" />
        </div>
      </div>

      {/* ── Tab bar ── */}
      <div
        className="mt-6 flex items-center gap-2 border-b"
        style={{ borderColor: "var(--border-subtle)" }}
      >
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="mb-[-1px] h-9 w-24 rounded-t-md" />
        ))}
      </div>

      {/* ── Tab content placeholder — matches min-h-[300px] of real tabpanel ── */}
      <div className="mt-6 space-y-3 min-h-[300px]">
        <Skeleton className="h-4 w-1/3" />
        <Skeleton className="h-3.5 w-3/4" />
        <Skeleton className="h-3.5 w-2/3" />
        <Skeleton className="h-3.5 w-4/5" />
        <Skeleton className="h-3.5 w-1/2" />
      </div>
    </div>
  );
}
