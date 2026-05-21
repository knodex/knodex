// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useFormContext } from "react-hook-form";
import type { ObjectFieldProps } from "./types";
import { FormField } from "./FormField";
import { orderProperties } from "@/lib/order-properties";

/**
 * Object field rendered as a bold section header with always-visible children.
 *
 * Feature-toggle pattern: when the object has an "enabled" boolean child,
 * the enabled checkbox always shows; peer fields are only shown when enabled=true.
 */
export function ObjectField({
  name,
  label,
  description,
  property,
  required,
  depth,
  deploymentNamespace,
}: ObjectFieldProps) {
  const { watch } = useFormContext();

  if (!property.properties) return null;

  const hasEnabledToggle = property.properties.enabled?.type === "boolean";
  const isFeatureEnabled = hasEnabledToggle ? Boolean(watch(`${name}.enabled`)) : true;

  const entries = orderProperties(
    Object.entries(property.properties),
    property.propertyOrder
  );

  const enabledChild = hasEnabledToggle
    ? entries.filter(([key]) => key === "enabled")
    : [];
  const peerChildren = hasEnabledToggle
    ? entries.filter(([key]) => key !== "enabled")
    : entries;

  return (
    <div className="space-y-3" data-testid={`field-${name}`}>
      <div>
        <p className="text-sm font-semibold text-foreground">
          {label}
          {required && <span className="text-destructive ml-1">*</span>}
        </p>
        {description && (
          <p className="text-xs text-muted-foreground mt-0.5">{description}</p>
        )}
      </div>

      <div className="pl-3 border-l-2 border-border/40 space-y-4">
        {enabledChild.map(([key, prop]) => (
          <FormField
            key={key}
            name={`${name}.${key}`}
            property={prop}
            required={property.required?.includes(key)}
            depth={depth + 1}
            deploymentNamespace={deploymentNamespace}
          />
        ))}

        {isFeatureEnabled &&
          peerChildren.map(([key, prop]) => (
            <FormField
              key={key}
              name={`${name}.${key}`}
              property={prop}
              required={property.required?.includes(key)}
              depth={depth + 1}
              deploymentNamespace={deploymentNamespace}
            />
          ))}
      </div>
    </div>
  );
}
