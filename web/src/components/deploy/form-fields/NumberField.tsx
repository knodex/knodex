// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import type { NumberFieldProps } from "./types";
import { inputBaseClasses, getInputBorderClass } from "./utils";

/**
 * Numeric input field supporting min/max constraints
 */
export function NumberField({
  name,
  label,
  description,
  required,
  error,
  min,
  max,
  register,
}: NumberFieldProps) {
  return (
    <div className="space-y-1.5" data-testid={`field-${name}`}>
      <label
        htmlFor={name}
        className="text-sm font-medium text-foreground flex items-center gap-1"
      >
        {label}
        {required && <span className="text-destructive">*</span>}
      </label>
      {description && (
        <p className="text-xs text-muted-foreground">{description}</p>
      )}
      <input
        id={name}
        type="number"
        min={min}
        max={max}
        {...register(name, {
          setValueAs: (v: string) => {
            // Return undefined for empty strings to avoid NaN in form data
            if (v === '' || v === undefined || v === null) return undefined;
            const num = Number(v);
            return isNaN(num) ? undefined : num;
          }
        })}
        data-testid={`input-${name}`}
        className={cn(inputBaseClasses, getInputBorderClass(!!error))}
      />
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
