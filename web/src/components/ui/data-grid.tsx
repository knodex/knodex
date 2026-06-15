// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * DataGrid — 2-column label/value stack primitive (NOT a table).
 *
 * Renders a CSS grid block of label/value cells. The label is the small
 * 11px muted line, the value is 13px primary text (mono when `mono: true`).
 *
 * Naming-collision note: NOT the data-table primitive `list-table.tsx`.
 * Distinct file, distinct purpose.
 */

import * as React from "react";

import { cn } from "@/lib/utils";

export interface DataGridItem {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
}

export interface DataGridProps extends React.HTMLAttributes<HTMLDListElement> {
  items: DataGridItem[];
  columns?: 1 | 2 | 3 | 4;
}

export function DataGrid({
  items,
  columns = 2,
  className,
  style,
  ...props
}: DataGridProps) {
  return (
    <dl
      className={cn("text-sm", className)}
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
        rowGap: "14px",
        columnGap: "32px",
        ...style,
      }}
      {...props}
    >
      {items.map((item, i) => (
        <div key={`${item.label}-${i}`} className="flex flex-col gap-1">
          <dt className="text-[11px] text-muted-foreground">{item.label}</dt>
          <dd
            className={cn(
              "text-[13px] text-foreground",
              item.mono && "font-mono"
            )}
          >
            {item.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
