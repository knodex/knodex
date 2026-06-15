// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * FormField — generic label + hint OR error + required asterisk wrapper.
 *
 * Programmatically associates the label and child input (caller-owned `id`),
 * and injects `aria-describedby` on the child input pointing to the hint OR
 * error node id when those props are set.
 *
 * Naming-collision note: distinct from `web/src/components/deploy/FormField.tsx`
 * (a deploy-form-specific component). Consumers disambiguate via import path;
 * both coexist.
 */

import * as React from "react";

import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";

export interface FormFieldProps {
  label: string;
  /**
   * The DOM id of the controlled input/textarea/select rendered as the child.
   * Required (no auto-id) — caller owns the id and we wire `<label htmlFor>`
   * and `aria-describedby` against it.
   */
  htmlFor: string;
  hint?: string;
  error?: string;
  required?: boolean;
  children: React.ReactNode;
  className?: string;
}

export function FormField({
  label,
  htmlFor,
  hint,
  error,
  required,
  children,
  className,
}: FormFieldProps) {
  const hintId = hint && !error ? `${htmlFor}-hint` : undefined;
  const errorId = error ? `${htmlFor}-error` : undefined;
  const describedBy = errorId ?? hintId;

  // Inject aria-describedby on the (single) child input. Keep it simple — one
  // child case is enough for the contract; callers pass `<input id={htmlFor} />`.
  const child = React.Children.only(children) as React.ReactElement<{
    "aria-describedby"?: string;
    "aria-invalid"?: boolean | "true" | "false";
  }>;
  const enhancedChild = describedBy
    ? React.cloneElement(child, {
        "aria-describedby": [child.props["aria-describedby"], describedBy]
          .filter(Boolean)
          .join(" "),
        ...(error ? { "aria-invalid": true as const } : {}),
      })
    : child;

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      <Label htmlFor={htmlFor}>
        {label}
        {required ? (
          <span aria-hidden="true" className="ml-0.5 text-destructive">
            *
          </span>
        ) : null}
      </Label>
      {enhancedChild}
      {error ? (
        <p
          id={errorId}
          role="alert"
          className="text-xs text-destructive"
        >
          {error}
        </p>
      ) : hint ? (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
