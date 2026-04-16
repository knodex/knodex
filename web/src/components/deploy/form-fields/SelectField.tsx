// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo } from "react";
import { cn } from "@/lib/utils";
import type { SelectFieldProps } from "./types";
import { inputBaseClasses, getInputBorderClass } from "./utils";

/**
 * Dropdown select field for enum values
 */
export const SelectField = memo(function SelectField({
  name,
  label,
  description,
  options,
  required,
  error,
  defaultValue,
  register,
}: SelectFieldProps) {

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
      <select
        id={name}
        defaultValue={defaultValue}
        {...register(name)}
        data-testid={`input-${name}`}
        aria-invalid={!!error}
        aria-describedby={error ? `error-${name}` : undefined}
        className={cn(inputBaseClasses, getInputBorderClass(!!error))}
      >
        <option value="">Select {label.toLowerCase()}...</option>
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
      {error && <p id={`error-${name}`} className="text-xs text-destructive" role="alert">{error}</p>}
    </div>
  );
});
