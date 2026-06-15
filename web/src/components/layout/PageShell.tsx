// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import * as React from "react";
import { cn } from "@/lib/utils";

interface PageShellProps {
  /** Left-aligned slot (search/filters/view toggle/etc.). Rendered verbatim. */
  toolbar?: React.ReactNode;
  /** Right-aligned action slot (most commonly a Deploy / Create … button). */
  primaryAction?: React.ReactNode;
  /** Passthrough for spacing only; prefer page-level `space-y-*`. */
  className?: string;
}

export const PageShell = React.forwardRef<HTMLDivElement, PageShellProps>(
  ({ toolbar, primaryAction, className }, ref) => (
    <div
      ref={ref}
      className={cn(
        "flex items-center gap-3",
        primaryAction ? "justify-between" : undefined,
        className
      )}
    >
      {toolbar ? <div className="min-w-0 flex-1">{toolbar}</div> : null}
      {primaryAction}
    </div>
  )
);
PageShell.displayName = "PageShell";
