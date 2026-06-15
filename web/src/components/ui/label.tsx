// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import * as React from "react";

import { cn } from "@/lib/utils";

export type LabelProps = React.LabelHTMLAttributes<HTMLLabelElement>;

const Label = React.forwardRef<HTMLLabelElement, LabelProps>(
  ({ className, ...props }, ref) => {
    return (
      // Generic primitive — callers wire `htmlFor` / controls through {...props},
      // which the static a11y rule cannot see.
      // eslint-disable-next-line jsx-a11y/label-has-associated-control
      <label
        className={cn(
          "text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
          className
        )}
        ref={ref}
        {...props}
      />
    );
  }
);
Label.displayName = "Label";

export { Label };
