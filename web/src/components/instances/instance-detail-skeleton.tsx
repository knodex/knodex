// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

export function InstanceDetailSkeleton() {
  return (
    <div className="space-y-4" data-testid="instance-detail-skeleton">
      {/* Top row: two panels */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <SkeletonPanel lines={6} />
        <SkeletonPanel lines={8} />
      </div>
      {/* Bottom row: config panel */}
      <SkeletonPanel lines={5} />
    </div>
  );
}

function SkeletonPanel({ lines }: { lines: number }) {
  return (
    <div
      className="border p-4 space-y-3"
      style={{
        backgroundColor: "var(--surface-primary)",
        borderColor: "rgba(255,255,255,0.08)",
        borderRadius: "var(--radius-token-lg)",
      }}
    >
      <Skeleton className="h-4 w-24" />
      <div className="space-y-2">
        {Array.from({ length: lines }).map((_, i) => (
          <Skeleton key={i} className="h-3.5" style={{ width: `${70 + (i * 7) % 30}%` }} />
        ))}
      </div>
    </div>
  );
}
