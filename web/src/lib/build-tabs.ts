// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { orderEntries } from "@/lib/order-properties";
import { formatLabel } from "@/components/deploy/form-fields";
import type { FormSchema, FormProperty } from "@/types/rgd";

export type TabKind = "general" | "schema" | "review";

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
 * Field names owned by the General tab's Knodex plumbing section.
 * Excluded from the schema-driven section of the General tab and stripped
 * from the deploy payload's `spec` field.
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
export const RESERVED_TAB_IDS = ["general", "review"] as const;

/**
 * Top-level object property keys that should NOT become their own tab.
 * Their `properties` are folded into the General tab as a nested object
 * field (rendered inline via NestedObjectEditor).
 *
 * Nested externalRef (e.g. `schema.spec.db.externalRef`) is unaffected —
 * it stays under the owning object's tab via the normal recursive render.
 */
export const TOP_LEVEL_GENERAL_OBJECT_KEYS = ["externalRef"] as const;

const RESERVED_BASICS_SET = new Set<string>(RESERVED_BASICS_KEYS);
const RESERVED_TAB_ID_SET = new Set<string>(RESERVED_TAB_IDS);
const TOP_LEVEL_GENERAL_OBJECT_SET = new Set<string>(
  TOP_LEVEL_GENERAL_OBJECT_KEYS
);

/**
 * Derive the ordered tab list from an RGD form schema.
 *
 * Rules:
 * 1. Always emit a `general` tab as the first tab. It hosts the Knodex-owned
 *    plumbing fields (instance name, project, namespace, deployment mode)
 *    plus any top-level scalar properties from the RGD schema.
 * 2. Walk top-level properties ordered by `schema.propertyOrder`.
 * 3. Skip any key in `RESERVED_BASICS_KEYS` (owned by the General tab plumbing).
 * 4. Scalar properties are swept into the General tab's properties map.
 * 5. Top-level keys in `TOP_LEVEL_GENERAL_OBJECT_KEYS` (e.g. `externalRef`)
 *    are also folded into the General tab as nested object fields.
 * 6. All other object properties each become their own tab.
 *    Tab id is prefixed with `"rgd-"` when it would collide with a reserved tab id.
 * 7. Always append a Review + Deploy tab last.
 */
export function buildTabsFromSchema(
  schema: FormSchema | null | undefined
): DeployTab[] {
  const generalProperties: Record<string, FormProperty> = {};
  const objectTabs: DeployTab[] = [];

  if (schema?.properties) {
    const entries = orderEntries(
      Object.entries(schema.properties),
      schema.propertyOrder
    );

    for (const [key, prop] of entries) {
      if (RESERVED_BASICS_SET.has(key)) continue;

      const isObject = prop.type === "object" && !!prop.properties;
      if (isObject && !TOP_LEVEL_GENERAL_OBJECT_SET.has(key)) {
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
        // scalar OR top-level externalRef (folded into General)
        generalProperties[key] = prop;
      }
    }
  }

  const generalKeys = Object.keys(generalProperties);
  const generalPropertyOrder = schema?.propertyOrder?.filter(
    (k) => generalProperties[k] !== undefined
  );
  const generalRequired =
    schema?.required?.filter((k) => generalProperties[k] !== undefined) ?? [];

  const tabs: DeployTab[] = [
    {
      id: "general",
      kind: "general",
      label: "General",
      properties: generalKeys.length > 0 ? generalProperties : undefined,
      propertyOrder: generalPropertyOrder,
      required: generalRequired,
    },
  ];

  tabs.push(...objectTabs);
  tabs.push({ id: "review", kind: "review", label: "Review + Deploy" });
  return tabs;
}
