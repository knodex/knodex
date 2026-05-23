// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * List-table primitives — visual shell + styled header used by every primary
 * list view in the app (catalog, instances, projects, secrets, repositories,
 * compliance pages, audit, etc).
 *
 * Two surfaces:
 *   - <ListTableShell> / <ListTableHeader>: drop-in wrappers around a table.
 *   - tableShellClasses / tableHeaderClasses: className strings for sites
 *     that need to inline-customize the host element.
 *
 * Why a primitive instead of one monolithic <DataTable>: each list view has
 * its own column shape, hover affordances, and row-action quirks. A single
 * <DataTable> ends up either react-table-shaped or leaks. The styling
 * primitive route gives single-touch redesigns on the visual layer without
 * locking the column/row structure.
 */

import * as React from "react";

import { cn } from "@/lib/utils";
import { TableHeader } from "@/components/ui/table";

/** Outer container styling for a list-table. Rounded card with border. */
export const tableShellClasses =
  "rounded-lg border border-border overflow-hidden animate-fade-in-up";

/** Styled table-header background used app-wide. */
export const tableHeaderClasses = "bg-card/95 backdrop-blur-sm";

/** Sticky positioning override — only works when the table is not inside a
 *  scroll container (i.e. a raw <table>, not shadcn's <Table> wrapper). */
export const tableHeaderStickyClasses = "sticky top-[52px] z-10";

interface ListTableShellProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Suppress the entrance animation. */
  noAnimation?: boolean;
}

/** Rounded card shell hosting a list-style table. */
export const ListTableShell = React.forwardRef<HTMLDivElement, ListTableShellProps>(
  ({ className, noAnimation, ...props }, ref) => (
    <div
      ref={ref}
      className={cn(
        "rounded-lg border border-border overflow-hidden",
        !noAnimation && "animate-fade-in-up",
        className
      )}
      {...props}
    />
  )
);
ListTableShell.displayName = "ListTableShell";

interface ListTableHeaderProps extends React.HTMLAttributes<HTMLTableSectionElement> {
  /** Pin the header at top:52px while scrolling. Use only with raw <table>
   *  consumers (no <Table> overflow-auto wrapper). */
  sticky?: boolean;
}

/** Pre-styled <thead> with the consistent background used across list views. */
export const ListTableHeader = React.forwardRef<HTMLTableSectionElement, ListTableHeaderProps>(
  ({ className, sticky = false, ...props }, ref) => (
    <TableHeader
      ref={ref}
      className={cn(
        tableHeaderClasses,
        sticky && tableHeaderStickyClasses,
        className
      )}
      {...props}
    />
  )
);
ListTableHeader.displayName = "ListTableHeader";
