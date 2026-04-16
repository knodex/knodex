// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { memo } from "react";
import { useFormContext, useController } from "react-hook-form";
import { cn } from "@/lib/utils";
import type { NumberFieldProps } from "./types";
import { inputBaseClasses, getInputBorderClass } from "./utils";

/**
 * Numeric input field. When both min and max are provided, renders a slider
 * synced with a number input for intuitive range-bounded entry.
 */
export const NumberField = memo(function NumberField({
  name,
  label,
  description,
  required,
  error,
  min,
  max,
  isInteger,
}: NumberFieldProps) {
  const { control } = useFormContext();
  const { field } = useController({
    name,
    control,
    rules: {
      ...(min !== undefined && { min }),
      ...(max !== undefined && { max }),
    },
  });

  const hasSlider = min !== undefined && max !== undefined;
  const numValue = field.value === undefined || field.value === "" ? "" : Number(field.value);
  const sliderValue = numValue === "" ? min ?? 0 : (numValue as number);

  const handleSliderChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = isInteger ? parseInt(e.target.value, 10) : parseFloat(e.target.value);
    field.onChange(isNaN(v) ? undefined : v);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = e.target.value;
    if (raw === "" || raw === "-") {
      field.onChange(undefined);
      return;
    }
    const v = isInteger ? parseInt(raw, 10) : parseFloat(raw);
    field.onChange(isNaN(v) ? undefined : v);
  };

  const step = isInteger ? 1 : "any";
  const pct = hasSlider && numValue !== ""
    ? Math.round(((sliderValue - min!) / (max! - min!)) * 100)
    : undefined;

  // Spread field to satisfy react-hooks/refs rule (avoids inline field.ref / field.onBlur access)
  const { onChange: _onChange, value: _value, ...fieldSpread } = field;

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

      {hasSlider ? (
        <div className="space-y-2">
          {/* Slider track */}
          <div className="relative flex items-center">
            <input
              {...fieldSpread}
              type="range"
              min={min}
              max={max}
              step={step}
              value={sliderValue}
              onChange={handleSliderChange}
              aria-label={label}
              aria-valuemin={min}
              aria-valuemax={max}
              aria-valuenow={sliderValue}
              className={cn(
                "w-full h-2 appearance-none rounded-full cursor-pointer",
                "bg-muted",
                "[&::-webkit-slider-thumb]:appearance-none",
                "[&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4",
                "[&::-webkit-slider-thumb]:rounded-full",
                "[&::-webkit-slider-thumb]:bg-primary",
                "[&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-background",
                "[&::-webkit-slider-thumb]:shadow-sm",
                "[&::-webkit-slider-thumb]:cursor-pointer",
                "[&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4",
                "[&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-0",
                "[&::-moz-range-thumb]:bg-primary [&::-moz-range-thumb]:cursor-pointer",
              )}
              style={{
                background: pct !== undefined
                  ? `linear-gradient(to right, hsl(var(--primary)) ${pct}%, hsl(var(--muted)) ${pct}%)`
                  : undefined,
              }}
            />
          </div>

          {/* Min / current value / max labels */}
          <div className="flex items-center justify-between text-xs text-muted-foreground select-none">
            <span>{min}</span>
            <span className={cn(
              "px-2 py-0.5 rounded-full text-xs font-medium tabular-nums",
              "bg-primary/10 text-primary"
            )}>
              {numValue === "" ? "—" : numValue}
            </span>
            <span>{max}</span>
          </div>

          {/* Number input for precise entry */}
          <input
            {...fieldSpread}
            id={name}
            type="number"
            min={min}
            max={max}
            step={step}
            value={numValue}
            onChange={handleInputChange}
            data-testid={`input-${name}`}
            aria-invalid={!!error}
            aria-describedby={error ? `error-${name}` : undefined}
            className={cn(inputBaseClasses, getInputBorderClass(!!error))}
          />
        </div>
      ) : (
        <input
          {...fieldSpread}
          id={name}
          type="number"
          min={min}
          max={max}
          step={step}
          value={numValue}
          onChange={handleInputChange}
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
