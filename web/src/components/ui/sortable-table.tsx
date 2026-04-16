// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo } from "react";
import { ArrowUpDown, ArrowUp, ArrowDown } from "@/lib/icons";
import { TableHead } from "@/components/ui/table";

type SortDir = "asc" | "desc";

// Defined at module scope to prevent React from treating these as new component types
// on every render (which would cause DOM remount and keyboard focus loss on sort click).
export function SortIcon<T extends string>({
  field,
  sortField,
  sortDir,
}: {
  field: T;
  sortField: T;
  sortDir: SortDir;
}) {
  if (sortField !== field) return <ArrowUpDown className="ml-1 h-3 w-3 opacity-40 inline" />;
  return sortDir === "asc"
    ? <ArrowUp className="ml-1 h-3 w-3 inline" />
    : <ArrowDown className="ml-1 h-3 w-3 inline" />;
}

interface SortableHeadProps<T extends string> {
  field: T;
  sortField: T;
  sortDir: SortDir;
  onSort: (field: T) => void;
  children: React.ReactNode;
  className?: string;
}

function SortableHeadInner<T extends string>({
  field,
  sortField,
  sortDir,
  onSort,
  children,
  className,
}: SortableHeadProps<T>) {
  // aria-sort must be on the th (columnheader role), not on the inner button
  const ariaSortValue: React.AriaAttributes["aria-sort"] =
    sortField === field ? (sortDir === "asc" ? "ascending" : "descending") : "none";
  return (
    <TableHead className={className} aria-sort={ariaSortValue}>
      <button
        onClick={() => onSort(field)}
        className="flex items-center gap-0 text-left font-medium text-muted-foreground hover:text-foreground transition-colors"
      >
        {children}
        <SortIcon field={field} sortField={sortField} sortDir={sortDir} />
      </button>
    </TableHead>
  );
}

export const SortableHead = memo(SortableHeadInner) as typeof SortableHeadInner;
