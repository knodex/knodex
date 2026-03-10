// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import type { ConditionalSection } from "@/types/rgd";

/**
 * @deprecated Use useFieldVisibility instead. This hook does not support structured CEL rules
 * or AND-based hiding semantics. It is kept for backwards compatibility only.
 *
 * Hook to determine which form fields should be visible based on conditional sections.
 *
 * @param conditionalSections - Array of conditional sections from the schema
 * @param formValues - Current form values
 * @returns Set of property paths that should be hidden
 */
export function useConditionalFields(
  conditionalSections: ConditionalSection[] | undefined,
  formValues: Record<string, unknown>
): Set<string> {
  return useMemo(() => {
    const hiddenFields = new Set<string>();

    if (!conditionalSections || conditionalSections.length === 0) {
      return hiddenFields;
    }

    for (const section of conditionalSections) {
      // Get the controlling field's value from form values
      // controllingField is like "spec.ingress.enabled" - we need to traverse formValues
      const controllingValue = getNestedValue(
        formValues,
        section.controllingField
      );

      // Determine if the condition is met
      const conditionMet = evaluateCondition(
        controllingValue,
        section.expectedValue
      );

      // If condition is NOT met, mark affected properties as hidden
      if (!conditionMet) {
        for (const prop of section.affectedProperties) {
          hiddenFields.add(prop);
        }
      }
    }

    return hiddenFields;
  }, [conditionalSections, formValues]);
}

/**
 * Get a nested value from an object using a dot-separated path.
 * Supports paths like "spec.ingress.enabled"
 */
function getNestedValue(
  obj: Record<string, unknown>,
  path: string
): unknown {
  // Remove "spec." prefix if present (form values start from spec level)
  const normalizedPath = path.replace(/^spec\./, "");

  const parts = normalizedPath.split(".");
  let current: unknown = obj;

  for (const part of parts) {
    if (current === null || current === undefined) {
      return undefined;
    }
    if (typeof current !== "object") {
      return undefined;
    }
    current = (current as Record<string, unknown>)[part];
  }

  return current;
}

/**
 * Evaluate if a condition is met based on the controlling value and expected value.
 *
 * Rules:
 * - If expectedValue is true (boolean): controllingValue must be truthy
 * - If expectedValue is false (boolean): controllingValue must be falsy
 * - If expectedValue is a specific value: controllingValue must equal it
 * - If expectedValue is undefined/null: treat as truthy check
 */
function evaluateCondition(
  controllingValue: unknown,
  expectedValue: unknown
): boolean {
  // Handle undefined/null expected value as truthy check
  if (expectedValue === undefined || expectedValue === null) {
    return Boolean(controllingValue);
  }

  // Handle boolean expected values
  if (typeof expectedValue === "boolean") {
    if (expectedValue === true) {
      return Boolean(controllingValue);
    } else {
      return !controllingValue;
    }
  }

  // Handle string comparison (case-insensitive for "true"/"false" strings)
  if (typeof expectedValue === "string") {
    const strValue = String(controllingValue).toLowerCase();
    const expectedStr = expectedValue.toLowerCase();

    // Handle boolean-like strings
    if (expectedStr === "true") {
      return strValue === "true" || controllingValue === true;
    }
    if (expectedStr === "false") {
      return strValue === "false" || controllingValue === false;
    }

    return strValue === expectedStr;
  }

  // Handle numeric comparison
  if (typeof expectedValue === "number") {
    return Number(controllingValue) === expectedValue;
  }

  // Default to strict equality
  return controllingValue === expectedValue;
}

/**
 * Get all controlling fields from conditional sections.
 * These fields should always be visible since they control other fields.
 */
export function getControllingFields(
  conditionalSections: ConditionalSection[] | undefined
): Set<string> {
  const controllingFields = new Set<string>();

  if (!conditionalSections) {
    return controllingFields;
  }

  for (const section of conditionalSections) {
    // Normalize path by removing "spec." prefix
    const normalizedPath = section.controllingField.replace(/^spec\./, "");
    controllingFields.add(normalizedPath);

    // Also add parent paths for nested controlling fields
    // e.g., for "ingress.enabled", also add "ingress"
    const parts = normalizedPath.split(".");
    for (let i = 1; i < parts.length; i++) {
      controllingFields.add(parts.slice(0, i).join("."));
    }
  }

  return controllingFields;
}
