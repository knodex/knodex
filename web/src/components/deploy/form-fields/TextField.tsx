// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import type { TextFieldProps } from "./types";
import { inputBaseClasses, getInputBorderClass } from "./utils";

/**
 * Text input field supporting various formats (text, email, password, textarea)
 */
export function TextField({
  name,
  label,
  description,
  required,
  error,
  format,
  register,
}: TextFieldProps) {
  const inputType =
    format === "password" ? "password" : format === "email" ? "email" : "text";
  const isTextarea =
    format === "textarea" ||
    (!format && description?.toLowerCase().includes("multiline"));

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
      {isTextarea ? (
        <textarea
          id={name}
          {...register(name)}
          data-testid={`input-${name}`}
          className={cn(inputBaseClasses, getInputBorderClass(!!error))}
          rows={4}
        />
      ) : (
        <input
          id={name}
          type={inputType}
          {...register(name)}
          data-testid={`input-${name}`}
          className={cn(inputBaseClasses, getInputBorderClass(!!error))}
        />
      )}
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
