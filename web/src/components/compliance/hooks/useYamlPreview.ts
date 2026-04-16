// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ConstraintTemplate } from "@/types/compliance";
import type { ConstraintFormValues } from "./useConstraintFormValidation";
import { cleanParameters, buildMatchRules } from "./constraintUtils";

/**
 * Hook for generating YAML preview of a constraint being created.
 *
 * Note: formValues from watch() produces a new reference every render,
 * so memoization is intentionally omitted — the YAML preview recalculates
 * on every render, which is acceptable for a preview panel.
 */
export function useYamlPreview(
  template: ConstraintTemplate,
  formValues: ConstraintFormValues,
  canUseFormMode: boolean,
  useFormMode: boolean
) {
  const effectiveParameters = canUseFormMode && useFormMode && formValues.params
    ? cleanParameters(formValues.params)
    : formValues.parametersRaw;
  return generateYamlPreview(template, formValues, effectiveParameters);
}

function generateYamlPreview(
  template: ConstraintTemplate,
  values: ConstraintFormValues,
  effectiveParameters: Record<string, unknown> | string | undefined
): string {
  const lines: string[] = [];

  lines.push(`apiVersion: constraints.gatekeeper.sh/v1beta1`);
  lines.push(`kind: ${template.kind}`);
  lines.push(`metadata:`);
  lines.push(`  name: ${values.name || "<name>"}`);
  lines.push(`spec:`);
  lines.push(`  enforcementAction: ${values.enforcementAction}`);

  const match = buildMatchRules(values);
  if (match) {
    lines.push(`  match:`);
    if (match.kinds && match.kinds.length > 0) {
      lines.push(`    kinds:`);
      for (const kind of match.kinds) {
        lines.push(`      - apiGroups:`);
        if (kind.apiGroups.length === 0) {
          lines.push(`          - ""`);
        } else {
          for (const ag of kind.apiGroups) {
            lines.push(`          - "${ag}"`);
          }
        }
        lines.push(`        kinds:`);
        for (const k of kind.kinds) {
          lines.push(`          - ${k}`);
        }
      }
    }
    if (match.namespaces && match.namespaces.length > 0) {
      lines.push(`    namespaces:`);
      for (const ns of match.namespaces) {
        lines.push(`      - ${ns}`);
      }
    }
  }

  let params: Record<string, unknown> | undefined;
  if (typeof effectiveParameters === "object" && effectiveParameters !== null) {
    params = effectiveParameters;
  } else if (typeof effectiveParameters === "string" && effectiveParameters.trim() !== "{}") {
    try {
      params = JSON.parse(effectiveParameters);
    } catch {
      lines.push(`  # Invalid JSON in parameters`);
      return lines.join("\n");
    }
  }

  if (params && Object.keys(params).length > 0) {
    lines.push(`  parameters:`);
    const yamlParams = jsonToYaml(params, 4);
    lines.push(yamlParams);
  }

  return lines.join("\n");
}

export function jsonToYaml(obj: unknown, indent: number): string {
  const spaces = " ".repeat(indent);

  if (obj === null || obj === undefined) {
    return `${spaces}~`;
  }

  if (typeof obj === "string") {
    if (obj.includes("\n")) {
      return `|\n${obj.split("\n").map((l) => spaces + "  " + l).join("\n")}`;
    }
    const escaped = obj.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
    return `"${escaped}"`;
  }

  if (typeof obj === "number" || typeof obj === "boolean") {
    return String(obj);
  }

  if (Array.isArray(obj)) {
    if (obj.length === 0) return "[]";
    return obj
      .map((item) => {
        const value = jsonToYaml(item, indent + 2);
        if (typeof item === "object" && item !== null) {
          return `${spaces}- ${value.trim().replace(/^\s+/, "")}`;
        }
        return `${spaces}- ${value}`;
      })
      .join("\n");
  }

  if (typeof obj === "object") {
    const entries = Object.entries(obj as Record<string, unknown>);
    if (entries.length === 0) return "{}";
    return entries
      .map(([key, value]) => {
        const yamlValue = jsonToYaml(value, indent + 2);
        if (typeof value === "object" && value !== null && !Array.isArray(value)) {
          return `${spaces}${key}:\n${yamlValue}`;
        }
        if (Array.isArray(value)) {
          return `${spaces}${key}:\n${yamlValue}`;
        }
        return `${spaces}${key}: ${yamlValue}`;
      })
      .join("\n");
  }

  return String(obj);
}
