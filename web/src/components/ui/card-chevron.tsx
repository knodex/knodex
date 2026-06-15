// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * CardChevron — absolutely-positioned hover affordance for clickable cards.
 *
 * Render this as a direct child of a card root that has the `group` and
 * `relative` Tailwind classes. The chevron is invisible by default and fades
 * in on group hover (150ms).
 *
 * Shape (a) of the hover-chevron primitive — chose this over the utility-class
 * shape (b) for type safety + colocation with the other UI primitives.
 */

import * as React from "react";

import { ChevronRight } from "@/lib/icons";
import { cn } from "@/lib/utils";

export type CardChevronProps = React.HTMLAttributes<HTMLSpanElement>;

export const CardChevron = React.forwardRef<HTMLSpanElement, CardChevronProps>(
  ({ className, ...props }, ref) => {
    return (
      <span
        ref={ref}
        aria-hidden="true"
        className={cn(
          "absolute top-3 right-3 text-muted-foreground",
          "opacity-0 transition-opacity duration-150",
          "group-hover:opacity-100 group-focus-within:opacity-100",
          className
        )}
        {...props}
      >
        <ChevronRight className="h-4 w-4" />
      </span>
    );
  }
);
CardChevron.displayName = "CardChevron";
