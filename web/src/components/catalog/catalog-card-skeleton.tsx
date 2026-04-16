// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

export function CatalogCardSkeleton() {
  return (
    <div
      className="flex flex-col border"
      style={{
        backgroundColor: "var(--surface-primary)",
        borderColor: "rgba(255,255,255,0.08)",
        borderRadius: "var(--radius-token-lg)",
      }}
    >
      {/* Header: icon + name + version */}
      <div className="flex items-start justify-between gap-3 px-5 pt-5 pb-2">
        <div className="flex items-center gap-3">
          <Skeleton className="h-10 w-10 shrink-0 rounded-[var(--radius-token-md)]" />
          <Skeleton className="h-5 w-40" />
        </div>
        <Skeleton className="h-6 w-10 shrink-0 rounded-[var(--radius-token-sm)]" />
      </div>

      {/* Description (3 lines) */}
      <div className="px-5 pt-1 pb-5 space-y-2">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-4/5" />
        <Skeleton className="h-4 w-3/5" />
      </div>
    </div>
  );
}
