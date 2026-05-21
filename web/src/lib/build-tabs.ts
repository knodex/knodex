// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { orderEntries } from "@/lib/order-properties";
import { formatLabel } from "@/components/deploy/form-fields";
import type { FormSchema, FormProperty } from "@/types/rgd";

export type TabKind = "basics" | "general" | "schema" | "review";

export interface DeployTab {
  /** Tab id used in URL hash and as the React key. */
  id: string;
  label: string;
  kind: TabKind;
  /** Schema-driven properties owned by the tab (general + schema tabs only). */
  properties?: Record<string, FormProperty>;
  /** Display order for the tab's properties. */
  propertyOrder?: string[];
  /** Required field keys scoped to this tab's properties. */
  required?: string[];
}

/**
 * Field names owned by the Basics tab. Excluded from the auto-generated
 * General tab and stripped from the deploy payload's `spec` field.
 */
export const RESERVED_BASICS_KEYS = [
  "instanceName",
  "namespace",
  "project",
  "deploymentMode",
  "repositoryId",
  "gitBranch",
  "gitPath",
] as const;

/**
 * Tab ids owned by the page shell. RGDs declaring an object property whose
 * key collides with one of these gets a "rgd-" prefix on the tab id.
 */
export const RESERVED_TAB_IDS = ["basics", "general", "review"] as const;

const RESERVED_BASICS_SET = new Set<string>(RESERVED_BASICS_KEYS);
const RESERVED_TAB_ID_SET = new Set<string>(RESERVED_TAB_IDS);

/**
 * Derive the ordered tab list from an RGD form schema.
 *
 * Rules:
 * 1. Always prepend a Basics tab.
 * 2. Walk top-level properties ordered by `schema.propertyOrder`.
 * 3. Skip any key in `RESERVED_BASICS_KEYS`.
 * 4. Scalar properties are swept into a single auto-generated General tab.
 * 5. Object properties (including "advanced") each become their own tab.
 *    Tab id is prefixed with `"rgd-"` when it would collide with a reserved tab id.
 * 6. Always append a Review + Deploy tab last.
 */
export function buildTabsFromSchema(
  schema: FormSchema | null | undefined
): DeployTab[] {
  const tabs: DeployTab[] = [
    { id: "basics", kind: "basics", label: "Basics" },
  ];

  if (!schema?.properties) {
    tabs.push({ id: "review", kind: "review", label: "Review + Deploy" });
    return tabs;
  }

  const entries = orderEntries(
    Object.entries(schema.properties),
    schema.propertyOrder
  );

  const scalarMap: Record<string, FormProperty> = {};
  const objectTabs: DeployTab[] = [];

  for (const [key, prop] of entries) {
    if (RESERVED_BASICS_SET.has(key)) continue;

    const isObject = prop.type === "object" && !!prop.properties;
    if (isObject) {
      const tabId = RESERVED_TAB_ID_SET.has(key) ? `rgd-${key}` : key;
      objectTabs.push({
        id: tabId,
        kind: "schema",
        label: prop.title ?? formatLabel(key),
        properties: prop.properties,
        propertyOrder: prop.propertyOrder,
        required: prop.required,
      });
    } else {
      scalarMap[key] = prop;
    }
  }

  const scalarKeys = Object.keys(scalarMap);
  if (scalarKeys.length > 0) {
    const filteredPropertyOrder = schema.propertyOrder?.filter((k) =>
      scalarMap[k] !== undefined
    );
    const scalarRequired =
      schema.required?.filter((k) => scalarMap[k] !== undefined) ?? [];
    tabs.push({
      id: "general",
      kind: "general",
      label: "General",
      properties: scalarMap,
      propertyOrder: filteredPropertyOrder,
      required: scalarRequired,
    });
  }

  tabs.push(...objectTabs);
  tabs.push({ id: "review", kind: "review", label: "Review + Deploy" });
  return tabs;
}
