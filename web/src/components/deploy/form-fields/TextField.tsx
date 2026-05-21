// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo } from "react";
import { useFormContext, useController } from "react-hook-form";
import { cn } from "@/lib/utils";
import type { TextFieldProps } from "./types";
import { inputBaseClasses, getInputBorderClass } from "./utils";

/**
 * Text input field supporting various formats (text, email, password, textarea)
 */
export const TextField = memo(function TextField({
  name,
  label,
  description,
  required,
  error,
  format,
}: TextFieldProps) {
  const { control } = useFormContext();
  const { field: { value, onChange, onBlur, ref } } = useController({ name, control });

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
          value={(value as string) ?? ""}
          onChange={onChange}
          onBlur={onBlur}
          ref={ref as React.Ref<HTMLTextAreaElement>}
          data-testid={`input-${name}`}
          aria-invalid={!!error}
          aria-describedby={error ? `error-${name}` : undefined}
          className={cn(inputBaseClasses, getInputBorderClass(!!error))}
          rows={4}
        />
      ) : (
        <input
          id={name}
          type={inputType}
          value={(value as string) ?? ""}
          onChange={onChange}
          onBlur={onBlur}
          ref={ref}
          data-testid={`input-${name}`}
          aria-invalid={!!error}
          aria-describedby={error ? `error-${name}` : undefined}
          className={cn(inputBaseClasses, getInputBorderClass(!!error))}
        />
      )}
      {error && <p id={`error-${name}`} className="text-xs text-destructive" role="alert">{error}</p>}
    </div>
  );
});
