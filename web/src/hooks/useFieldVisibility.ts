// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import type { ConditionalSection, ConditionRule } from "@/types/rgd";

/**
 * Result of the field visibility computation.
 */
export interface FieldVisibilityResult {
  /** Set of property paths that should be hidden */
  hiddenFields: Set<string>;
  /** Set of controlling field paths that should always be visible */
  controllingFields: Set<string>;
  /** Convenience function: returns true if the field should be rendered */
  isFieldVisible: (fieldName: string) => boolean;
}

/**
 * Hook to determine which form fields should be visible based on conditional sections.
 *
 * Improvements over useConditionalFields:
 * - Uses structured CEL rules (clientEvaluable + rules) for accurate evaluation
 * - AND-based hiding: a field is hidden only when ALL controlling sections have unmet conditions
 * - Falls back to expectedValue evaluation when rules are not available
 *
 * @param conditionalSections - Array of conditional sections from the schema
 * @param formValues - Current form values
 * @returns FieldVisibilityResult with hiddenFields, controllingFields, and isFieldVisible
 */
export function useFieldVisibility(
  conditionalSections: ConditionalSection[] | undefined,
  formValues: Record<string, unknown>
): FieldVisibilityResult {
  const controllingFields = useMemo(
    () => computeControllingFields(conditionalSections),
    [conditionalSections]
  );

  const hiddenFields = useMemo(() => {
    if (!conditionalSections || conditionalSections.length === 0) {
      return new Set<string>();
    }

    // Track per-field: how many sections reference it and how many are met
    const fieldSectionCount = new Map<string, number>();
    const fieldMetCount = new Map<string, number>();

    for (const section of conditionalSections) {
      const conditionMet = evaluateSectionCondition(section, formValues);

      for (const prop of section.affectedProperties) {
        fieldSectionCount.set(prop, (fieldSectionCount.get(prop) ?? 0) + 1);
        if (conditionMet) {
          fieldMetCount.set(prop, (fieldMetCount.get(prop) ?? 0) + 1);
        }
      }
    }

    // A field is hidden only when ALL referencing sections have condition=unmet
    const hidden = new Set<string>();
    for (const [field, totalSections] of fieldSectionCount) {
      const metSections = fieldMetCount.get(field) ?? 0;
      if (metSections === 0 && totalSections > 0) {
        hidden.add(field);
      }
    }

    return hidden;
  }, [conditionalSections, formValues]);

  const isFieldVisible = useMemo(() => {
    return (fieldName: string): boolean => {
      if (controllingFields.has(fieldName)) {
        return true;
      }
      return !hiddenFields.has(fieldName);
    };
  }, [hiddenFields, controllingFields]);

  return { hiddenFields, controllingFields, isFieldVisible };
}

/**
 * Evaluate whether a section's condition is met.
 * Uses structured rules when clientEvaluable is true, otherwise falls back to expectedValue.
 */
function evaluateSectionCondition(
  section: ConditionalSection,
  formValues: Record<string, unknown>
): boolean {
  // Prefer structured rules when available
  if (section.clientEvaluable && section.rules && section.rules.length > 0) {
    // AND semantics: ALL rules must be satisfied
    return section.rules.every((rule) => evaluateRule(rule, formValues));
  }

  // Fallback to expectedValue evaluation (legacy path)
  const controllingValue = getNestedValue(formValues, section.controllingField);
  return evaluateExpectedValue(controllingValue, section.expectedValue);
}

/**
 * Evaluate a single structured condition rule against form values.
 */
function evaluateRule(
  rule: ConditionRule,
  formValues: Record<string, unknown>
): boolean {
  const actualValue = getNestedValue(formValues, rule.field);

  switch (rule.op) {
    case "==":
      return looseEqual(actualValue, rule.value);
    case "!=":
      return !looseEqual(actualValue, rule.value);
    case ">":
      return toNumber(actualValue) > toNumber(rule.value);
    case "<":
      return toNumber(actualValue) < toNumber(rule.value);
    case ">=":
      return toNumber(actualValue) >= toNumber(rule.value);
    case "<=":
      return toNumber(actualValue) <= toNumber(rule.value);
    default:
      // Unknown operator — treat as not evaluable (show the field)
      return true;
  }
}

/**
 * Loose equality that handles boolean/string/number coercion.
 */
function looseEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true;

  // Handle boolean comparisons with form string values ("true"/"false")
  if (typeof b === "boolean") {
    if (typeof a === "string") {
      return (b === true && a === "true") || (b === false && a === "false");
    }
    return Boolean(a) === b;
  }

  // Handle numeric comparisons
  if (typeof b === "number" && typeof a === "number") {
    return a === b;
  }

  // Handle string comparisons
  if (typeof b === "string" && typeof a === "string") {
    return a === b;
  }

  return String(a) === String(b);
}

/**
 * Convert a value to a number for comparison operators.
 */
function toNumber(val: unknown): number {
  if (typeof val === "number") return val;
  if (typeof val === "string") return Number(val) || 0;
  if (typeof val === "boolean") return val ? 1 : 0;
  return 0;
}

/**
 * Get a nested value from an object using a dot-separated path.
 * Strips "spec." prefix since form values start from the spec level.
 */
function getNestedValue(
  obj: Record<string, unknown>,
  path: string
): unknown {
  const normalizedPath = path.replace(/^spec\./, "");
  const parts = normalizedPath.split(".");
  let current: unknown = obj;

  for (const part of parts) {
    if (current === null || current === undefined) return undefined;
    if (typeof current !== "object") return undefined;
    current = (current as Record<string, unknown>)[part];
  }

  return current;
}

/**
 * Evaluate condition using legacy expectedValue comparison.
 * Used as fallback when clientEvaluable is false.
 */
function evaluateExpectedValue(
  controllingValue: unknown,
  expectedValue: unknown
): boolean {
  if (expectedValue === undefined || expectedValue === null) {
    return Boolean(controllingValue);
  }

  if (typeof expectedValue === "boolean") {
    return expectedValue ? Boolean(controllingValue) : !controllingValue;
  }

  if (typeof expectedValue === "string") {
    const strValue = String(controllingValue).toLowerCase();
    const expectedStr = expectedValue.toLowerCase();
    if (expectedStr === "true") return strValue === "true" || controllingValue === true;
    if (expectedStr === "false") return strValue === "false" || controllingValue === false;
    return strValue === expectedStr;
  }

  if (typeof expectedValue === "number") {
    return Number(controllingValue) === expectedValue;
  }

  return controllingValue === expectedValue;
}

/**
 * Compute the set of controlling fields that should always be visible.
 * Includes parent paths for nested controlling fields.
 */
function computeControllingFields(
  conditionalSections: ConditionalSection[] | undefined
): Set<string> {
  const fields = new Set<string>();
  if (!conditionalSections) return fields;

  for (const section of conditionalSections) {
    const normalizedPath = section.controllingField.replace(/^spec\./, "");
    fields.add(normalizedPath);

    // Also add parent paths for nested controlling fields
    const parts = normalizedPath.split(".");
    for (let i = 1; i < parts.length; i++) {
      fields.add(parts.slice(0, i).join("."));
    }
  }

  return fields;
}
