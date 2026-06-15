// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Skeleton } from "@/components/ui/skeleton";

/**
 * Placeholder rendered inside the DeployDrawerShell while the RGD, schema,
 * and projects fetch.
 *
 * Why a dedicated component: the previous {@link ../ui/page-skeleton.tsx}
 * fallback rendered a 4-column card grid + search bar — geometry that
 * doesn't fit the 640-760px right-side Sheet and has nothing to do with a
 * vertical deploy form. This shape mirrors what {@link ./DeployPage.tsx}
 * renders once data lands: a row of step tabs (placeholder for
 * `DeployTabs`), then a stack of form field rows (label + input), then
 * footer-aligned action buttons (`DeployActionFooter`).
 */
export function DeployFormSkeleton() {
  return (
    <div className="space-y-6" data-testid="deploy-form-skeleton">
      {/* Step tabs row — mirrors DeployTabs (general / schema / review-ish) */}
      <div className="flex items-center gap-2 border-b border-[var(--border-subtle)] pb-3">
        <Skeleton className="h-7 w-24 rounded-md" />
        <Skeleton className="h-7 w-28 rounded-md" />
        <Skeleton className="h-7 w-20 rounded-md" />
      </div>

      {/* Form fields — label + input rows, the actual GeneralTab shape */}
      <div className="space-y-5">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="space-y-2">
            <Skeleton className="h-3.5 w-24" />
            <Skeleton className="h-9 w-full rounded-md" />
          </div>
        ))}

        {/* One taller field — namespace / description block */}
        <div className="space-y-2">
          <Skeleton className="h-3.5 w-28" />
          <Skeleton className="h-20 w-full rounded-md" />
        </div>
      </div>

      {/* Footer-aligned action buttons — Cancel + Next / Deploy.
          DeployActionFooter renders outside this slot via the Shell's `footer`
          prop, but during loading there's no footer yet, so we mirror it here
          to reserve the visual weight. */}
      <div className="flex justify-end gap-2 pt-4">
        <Skeleton className="h-9 w-24 rounded-md" />
        <Skeleton className="h-9 w-28 rounded-md" />
      </div>
    </div>
  );
}
