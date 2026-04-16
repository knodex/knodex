// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo } from "react";
import type { CheckboxFieldProps } from "./types";

/**
 * Boolean checkbox input field
 */
export const CheckboxField = memo(function CheckboxField({
  name,
  label,
  description,
  required,
  error,
  register,
}: CheckboxFieldProps) {
  return (
    <div className="space-y-1.5" data-testid={`field-${name}`}>
      <div className="flex items-center gap-2">
        <input
          id={name}
          type="checkbox"
          {...register(name)}
          data-testid={`input-${name}`}
          aria-invalid={!!error}
          aria-describedby={error ? `error-${name}` : undefined}
          className="h-4 w-4 rounded border-border bg-background text-primary focus:ring-primary/50"
        />
        <label
          htmlFor={name}
          className="text-sm font-medium text-foreground flex items-center gap-1"
        >
          {label}
          {required && <span className="text-destructive">*</span>}
        </label>
      </div>
      {description && (
        <p className="text-xs text-muted-foreground ml-6">{description}</p>
      )}
      {error && <p id={`error-${name}`} className="text-xs text-destructive ml-6" role="alert">{error}</p>}
    </div>
  );
});
