// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * FilterChip — Linear-style chip primitive for filter bars.
 *
 * Two surfaces:
 *   - <FilterChip> button: use for the `add` slot or standalone (when there's
 *     no underlying Select).
 *   - filterChipClasses(state): className helper to skin a <SelectTrigger>
 *     so it adopts the chip look while keeping shadcn Select behavior.
 */

import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const filterChipVariants = cva(
  cn(
    // `w-auto` defeats the `w-full` baked into shadcn's <SelectTrigger> base classes
    // so chips hug their content when these classes are layered on a trigger.
    // `rounded-md` matches the radius used by other filter affordances across
    // the app (catalog, projects). Don't switch to rounded-full — it looks
    // out of place next to the rest of the UI.
    "inline-flex items-center gap-1.5 h-8 w-auto px-2.5 rounded-md text-xs font-medium",
    "border transition-colors duration-150",
    "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--brand-primary)]/40",
    "disabled:pointer-events-none disabled:opacity-50"
  ),
  {
    variants: {
      state: {
        idle: cn(
          "bg-transparent border-[var(--border-default)] text-muted-foreground",
          "hover:border-[var(--border-hover)] hover:text-foreground"
        ),
        active: cn(
          "bg-[var(--brand-primary)]/10 border-[var(--brand-primary)]/30 text-foreground",
          "hover:bg-[var(--brand-primary)]/15"
        ),
        add: cn(
          "bg-transparent border-dashed border-[var(--border-default)] text-muted-foreground",
          "hover:border-[var(--border-hover)] hover:text-foreground"
        ),
      },
    },
    defaultVariants: {
      state: "idle",
    },
  }
);

export interface FilterChipProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof filterChipVariants> {
  /** Show a small teal dot before the label (typically when `state="active"`) */
  showDot?: boolean;
}

export const FilterChip = React.forwardRef<HTMLButtonElement, FilterChipProps>(
  ({ className, state = "idle", showDot, children, type = "button", ...props }, ref) => {
    return (
      <button
        ref={ref}
        type={type}
        className={cn(filterChipVariants({ state }), className)}
        {...props}
      >
        {showDot && <FilterChipDot />}
        {children}
      </button>
    );
  }
);
FilterChip.displayName = "FilterChip";

/**
 * Small teal dot indicating an active filter. Render inline before the label.
 */
export function FilterChipDot({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-block h-1.5 w-1.5 rounded-full bg-[var(--brand-primary)] shrink-0",
        className
      )}
    />
  );
}

/**
 * className helper for adopting the chip look on a different host element
 * (e.g. a shadcn <SelectTrigger>). Keeps the underlying primitive's behavior.
 */
// eslint-disable-next-line react-refresh/only-export-components -- helper is part of the primitive API
export function filterChipClasses(state: "idle" | "active" = "idle") {
  return filterChipVariants({ state });
}

// eslint-disable-next-line react-refresh/only-export-components -- variants are part of component API
export { filterChipVariants };
