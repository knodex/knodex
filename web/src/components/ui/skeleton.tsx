// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Skeleton component for loading states
 * Provides pulse animation with reduced-motion support
 */
import { cn } from "@/lib/utils";

type SkeletonProps = React.HTMLAttributes<HTMLDivElement>;

export function Skeleton({ className, ...props }: SkeletonProps) {
  return (
    <div
      className={cn(
        "animate-pulse rounded-md bg-muted",
        "motion-reduce:animate-none motion-reduce:bg-muted/50",
        className
      )}
      {...props}
    />
  );
}

// Preset skeleton components for common use cases
export function SkeletonText({ className, ...props }: SkeletonProps) {
  return <Skeleton className={cn("h-4 w-full", className)} {...props} />;
}

export function SkeletonHeading({ className, ...props }: SkeletonProps) {
  return <Skeleton className={cn("h-6 w-3/4", className)} {...props} />;
}

export function SkeletonCircle({ className, ...props }: SkeletonProps) {
  return <Skeleton className={cn("h-12 w-12 rounded-full", className)} {...props} />;
}

export function SkeletonButton({ className, ...props }: SkeletonProps) {
  return <Skeleton className={cn("h-10 w-24 rounded-md", className)} {...props} />;
}

export function SkeletonCard({ className, ...props }: SkeletonProps) {
  return (
    <div className={cn("rounded-lg border border-border p-4 space-y-3", className)} {...props}>
      <div className="flex items-center gap-3">
        <SkeletonCircle className="h-10 w-10" />
        <div className="flex-1 space-y-2">
          <SkeletonHeading className="w-1/2" />
          <SkeletonText className="w-3/4" />
        </div>
      </div>
      <SkeletonText />
      <SkeletonText className="w-5/6" />
      <div className="pt-2">
        <SkeletonButton />
      </div>
    </div>
  );
}
