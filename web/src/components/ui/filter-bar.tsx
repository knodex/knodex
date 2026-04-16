// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Shared filter bar primitives for consistent styling across pages.
 * Used by CatalogFilters, InstanceFilters, and any future filter bars.
 */

import { cn } from "@/lib/utils";

/** Shared class names for search input within filter bars */
export const filterSearchClasses = cn(
  "pl-9 pr-10 h-9 text-[var(--text-size-sm)]",
  "bg-transparent border border-[var(--border-default)]",
  "hover:border-[var(--border-hover)] transition-colors duration-150",
  "focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/30 focus-visible:border-[var(--brand-primary)]/50",
  "placeholder:text-muted-foreground"
);

/** Shared class names for the search icon */
export const filterSearchIconClasses =
  "absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none";

/** Shared class names for clear button inside search */
export const filterClearButtonClasses =
  "absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors";

/** Shared class names for select triggers in filter bars */
export function filterSelectClasses(isActive: boolean) {
  return cn(
    "h-9 text-sm",
    "bg-transparent border border-[var(--border-default)]",
    "hover:border-[var(--border-hover)] transition-all duration-150",
    "focus:ring-1 focus:ring-[var(--brand-primary)]/30",
    isActive ? "text-foreground font-medium" : "text-muted-foreground"
  );
}
