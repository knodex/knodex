// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { ReactNode } from "react";
import { orderEntries } from "@/lib/order-properties";
import { TOP_LEVEL_GENERAL_OBJECT_KEYS } from "@/lib/build-tabs";
import { FormField } from "@/components/deploy/FormField";
import type { FormProperty } from "@/types/rgd";

const FLATTEN_KEYS = new Set<string>(TOP_LEVEL_GENERAL_OBJECT_KEYS);

interface RenderArgs {
  /** Dotted RHF path prefix (empty string for the General tab; tab.id for SchemaTabs). */
  parentName: string;
  key: string;
  prop: FormProperty;
  required: boolean;
  deploymentNamespace: string;
}

/**
 * Render a tab-owned field, flattening the special `externalRef` object so its
 * children render at the parent depth (no "External Ref" header, no extra
 * indentation). Applies both at the General tab (`parentName=""`) and at
 * object tabs (`parentName=<tab.id>`).
 *
 * For non-flatten keys the function falls back to a single `<FormField>` —
 * preserving the regular object/scalar render path.
 */
export function renderFlattenableField({
  parentName,
  key,
  prop,
  required,
  deploymentNamespace,
}: RenderArgs): ReactNode[] {
  const fieldName = parentName ? `${parentName}.${key}` : key;
  const isFoldedObject =
    FLATTEN_KEYS.has(key) && prop.type === "object" && !!prop.properties;

  if (!isFoldedObject) {
    return [
      <FormField
        key={`field-${fieldName}`}
        name={fieldName}
        property={prop}
        required={required}
        deploymentNamespace={deploymentNamespace}
      />,
    ];
  }

  const childEntries = orderEntries(
    Object.entries(prop.properties ?? {}),
    prop.propertyOrder
  );
  const childRequired = new Set(prop.required ?? []);
  return childEntries.map(([childKey, childProp]) => {
    const name = `${fieldName}.${childKey}`;
    return (
      <FormField
        key={`field-${name}`}
        name={name}
        property={childProp}
        required={childRequired.has(childKey)}
        deploymentNamespace={deploymentNamespace}
      />
    );
  });
}
