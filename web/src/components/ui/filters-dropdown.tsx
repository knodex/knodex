// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * FiltersDropdown — collapses N non-search filter chips into a single `Filters`
 * button with chevron + optional count badge; chips live inside a popover.
 *
 * Wraps the existing shadcn `Popover` (Radix-backed) so outside-click + Esc
 * handling come for free.
 */

import * as React from "react";

import { ChevronDown } from "@/lib/icons";
import { cn } from "@/lib/utils";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { filterChipClasses } from "@/components/ui/filter-chip";

export interface FiltersDropdownProps {
  /** Number of currently active filters; shown as a badge when > 0. */
  activeCount?: number;
  /** Trigger label; defaults to "Filters". */
  label?: string;
  /** Popover body contents (filter chips, selects, etc.). */
  children: React.ReactNode;
  /** Optional className applied to the trigger button. */
  className?: string;
  /** Optional className applied to the popover content. */
  contentClassName?: string;
}

export function FiltersDropdown({
  activeCount = 0,
  label = "Filters",
  children,
  className,
  contentClassName,
}: FiltersDropdownProps) {
  const hasActive = activeCount > 0;
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          className={cn(
            filterChipClasses(hasActive ? "active" : "idle"),
            "group",
            className
          )}
          aria-label={label}
        >
          <span>{label}</span>
          {hasActive ? (
            <span
              className={cn(
                "inline-flex h-4 min-w-[16px] items-center justify-center rounded-full",
                "bg-[var(--brand-primary)]/20 px-1 text-[10px] font-semibold text-foreground"
              )}
              data-testid="filters-active-count"
            >
              {activeCount}
            </span>
          ) : null}
          <ChevronDown
            className={cn(
              "h-3.5 w-3.5 text-muted-foreground transition-transform duration-150",
              "group-data-[state=open]:rotate-180"
            )}
            aria-hidden="true"
          />
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className={cn("w-auto min-w-[240px] p-3", contentClassName)}
      >
        {children}
      </PopoverContent>
    </Popover>
  );
}
