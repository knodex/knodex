// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import { Skeleton, SkeletonCard } from "@/components/ui/skeleton";

interface PageSkeletonProps {
  variant?: "grid" | "list" | "detail";
  cardCount?: number;
  className?: string;
}

export function PageSkeleton({ variant = "grid", cardCount = 8, className }: PageSkeletonProps) {
  return (
    <div data-testid="page-skeleton" className={cn("animate-token-fade-in", className)}>
      {/* Header skeleton */}
      <div className="mb-6 space-y-2">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-4 w-32" />
      </div>

      {/* Content skeleton based on variant */}
      {variant === "grid" && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: cardCount }).map((_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      )}
      {variant === "list" && (
        <div className="space-y-3">
          {Array.from({ length: cardCount }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      )}
      {variant === "detail" && (
        <div className="space-y-4">
          <Skeleton className="h-48 w-full" />
          <div className="grid gap-4 sm:grid-cols-2">
            <Skeleton className="h-32 w-full" />
            <Skeleton className="h-32 w-full" />
          </div>
        </div>
      )}
    </div>
  );
}
