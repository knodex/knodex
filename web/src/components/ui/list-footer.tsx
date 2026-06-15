// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * ListFooter — uniform end-of-list summary row.
 *
 * Renders `{total} {totalLabel} · {v1} {l1} · {v2} {l2} · …` with `·` separators
 * marked `aria-hidden`. Numeric tokens are emphasized via `text-foreground`,
 * labels use `text-muted-foreground`.
 */

import * as React from "react";

import { cn } from "@/lib/utils";

export interface ListFooterProps extends React.HTMLAttributes<HTMLDivElement> {
  total: number;
  totalLabel?: string;
  breakdown?: Array<[label: string, value: number | string]>;
}

export const ListFooter = React.forwardRef<HTMLDivElement, ListFooterProps>(
  (
    { total, totalLabel = "total", breakdown, className, ...props },
    ref
  ) => {
    return (
      <div
        ref={ref}
        className={cn(
          "flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground",
          className
        )}
        {...props}
      >
        <span>
          <span className="text-foreground font-medium">{total}</span>{" "}
          {totalLabel}
        </span>
        {breakdown?.map(([label, value], i) => (
          <React.Fragment key={`${label}-${i}`}>
            <span aria-hidden="true" className="opacity-60">
              ·
            </span>
            <span>
              <span className="text-foreground font-medium">{value}</span>{" "}
              {label}
            </span>
          </React.Fragment>
        ))}
      </div>
    );
  }
);
ListFooter.displayName = "ListFooter";
