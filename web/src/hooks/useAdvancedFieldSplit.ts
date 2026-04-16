// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import type { FormProperty, AdvancedSection } from "@/types/rgd";
import { useAdvancedConfigToggle } from "@/components/deploy/AdvancedConfigToggle";
import { orderProperties } from "@/lib/order-properties";

/**
 * Standalone check for whether a field name is under the global advanced section.
 * Used by DeployPage's controlling-field classification without calling the full hook.
 */
export function isAdvancedField(fieldName: string): boolean {
  return fieldName === "advanced" || fieldName.startsWith("advanced.");
}

/**
 * Shared hook that splits schema properties into regular and advanced (global "advanced" key only).
 * Per-feature advanced sections (e.g., bastion.advanced) stay in regularProperties —
 * their inline advanced toggles are handled by ObjectField, not this global split.
 */
export function useAdvancedFieldSplit(
  properties: Record<string, FormProperty>,
  advancedSections?: AdvancedSection[],
  propertyOrder?: string[]
) {
  const globalSection =
    advancedSections?.find((s) => s.path === "advanced") ?? null;

  const { regularProperties, advancedProperties } = useMemo(() => {
    const regular: Array<[string, FormProperty]> = [];
    const advanced: Array<[string, FormProperty]> = [];

    for (const [name, prop] of orderProperties(Object.entries(properties), propertyOrder)) {
      if (isAdvancedField(name)) {
        // Flatten the "advanced" object container's children
        if (
          name === "advanced" &&
          prop.type === "object" &&
          prop.properties
        ) {
          for (const [childName, childProp] of orderProperties(
            Object.entries(prop.properties),
            prop.propertyOrder
          )) {
            advanced.push([`advanced.${childName}`, childProp]);
          }
        } else {
          advanced.push([name, prop]);
        }
      } else {
        regular.push([name, prop]);
      }
    }

    return { regularProperties: regular, advancedProperties: advanced };
  }, [properties, propertyOrder]);

  const { isExpanded: isAdvancedExpanded, toggle: toggleAdvanced } =
    useAdvancedConfigToggle();

  return {
    regularProperties,
    advancedProperties,
    globalSection,
    isAdvancedExpanded,
    toggleAdvanced,
  };
}
