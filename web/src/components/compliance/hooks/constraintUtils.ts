// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ConstraintFormValues } from "./useConstraintFormValidation";
import { getApiGroupValue } from "@/api/apiResources";

/**
 * Clean parameters object by removing undefined/empty/NaN values
 */
export function cleanParameters(params: Record<string, unknown>): Record<string, unknown> {
  const result: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(params)) {
    if (value === undefined) continue;
    if (value === "") continue;
    if (typeof value === "number" && isNaN(value)) continue;
    if (Array.isArray(value) && value.length === 0) continue;

    if (value !== null && typeof value === "object" && !Array.isArray(value)) {
      const cleaned = cleanParameters(value as Record<string, unknown>);
      if (Object.keys(cleaned).length > 0) {
        result[key] = cleaned;
      }
    } else {
      result[key] = value;
    }
  }

  return result;
}

/**
 * Build match rules from form data
 */
export function buildMatchRules(data: ConstraintFormValues) {
  const hasKinds =
    data.matchKinds &&
    data.matchKinds.some((mk) => mk.kinds.length > 0);
  const hasNamespaces =
    data.matchNamespaces && data.matchNamespaces.trim() !== "";

  if (!hasKinds && !hasNamespaces) {
    return null;
  }

  const match: {
    kinds?: Array<{ apiGroups: string[]; kinds: string[] }>;
    namespaces?: string[];
  } = {};

  if (hasKinds) {
    match.kinds = data.matchKinds
      ?.filter((mk) => mk.kinds.length > 0)
      .map((mk) => ({
        apiGroups: mk.apiGroups.map((g) => getApiGroupValue(g)),
        kinds: mk.kinds,
      }));
  }

  if (hasNamespaces) {
    match.namespaces = data.matchNamespaces
      ?.split(",")
      .map((ns) => ns.trim())
      .filter(Boolean);
  }

  return match;
}
