// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Format a camelCase or snake_case field name into a human-readable label
 */
export function formatLabel(name: string): string {
  return name
    .replace(/([A-Z])/g, " $1")
    .replace(/[_-]/g, " ")
    .replace(/^\w/, (c) => c.toUpperCase())
    .trim();
}

/**
 * Common input class names for form fields
 */
export const inputBaseClasses = [
  "w-full px-3 py-2 text-sm rounded-md border bg-background",
  "focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary",
].join(" ");

/**
 * Get error-aware border class
 */
export function getInputBorderClass(hasError: boolean): string {
  return hasError ? "border-destructive" : "border-border";
}
