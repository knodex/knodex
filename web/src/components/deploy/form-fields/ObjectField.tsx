// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { useFormContext } from "react-hook-form";
import { ChevronDown, ChevronRight } from "@/lib/icons";
import type { ObjectFieldProps } from "./types";
import { FormField } from "./FormField";
import { AdvancedConfigToggle, useAdvancedConfigToggle } from "../AdvancedConfigToggle";
import { orderProperties } from "@/lib/order-properties";

/**
 * Collapsible object field that renders nested properties.
 *
 * Feature-toggle pattern: when the object has an "enabled" boolean child,
 * peer fields and the inline advanced section are only shown when enabled=true.
 *
 * When `inlineAdvancedSection` is set, the "advanced" sub-key is rendered
 * inside an AdvancedConfigToggle instead of as a plain collapsible ObjectField.
 */
export function ObjectField({
  name,
  label,
  description,
  property,
  required,
  depth,
  deploymentNamespace,
  inlineAdvancedSection,
}: ObjectFieldProps) {
  const [isOpen, setIsOpen] = useState(false);
  const { isExpanded: isInlineAdvancedExpanded, toggle: toggleInlineAdvanced } = useAdvancedConfigToggle();
  const { watch } = useFormContext();

  if (!property.properties) return null;

  // Detect feature-toggle pattern: object has an "enabled" boolean child
  const hasEnabledToggle = property.properties.enabled?.type === "boolean";
  const isFeatureEnabled = hasEnabledToggle ? watch(`${name}.enabled`) : true;

  // When inlineAdvancedSection is set, split children into regular and advanced
  const entries = orderProperties(Object.entries(property.properties), property.propertyOrder);
  const hasInlineAdvanced = !!inlineAdvancedSection;
  const regularChildren = hasInlineAdvanced
    ? entries.filter(([key]) => key !== "advanced")
    : entries;
  const advancedChild = hasInlineAdvanced
    ? entries.find(([key]) => key === "advanced")
    : undefined;

  // Split regular children: "enabled" toggle always shows, peers only when enabled
  const enabledChild = hasEnabledToggle
    ? regularChildren.filter(([key]) => key === "enabled")
    : [];
  const peerChildren = hasEnabledToggle
    ? regularChildren.filter(([key]) => key !== "enabled")
    : regularChildren;

  return (
    <div className="space-y-2" data-testid={`field-${name}`}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 text-sm font-medium text-foreground hover:text-primary transition-colors"
      >
        {isOpen ? (
          <ChevronDown className="h-4 w-4" />
        ) : (
          <ChevronRight className="h-4 w-4" />
        )}
        {label}
        {required && <span className="text-destructive">*</span>}
      </button>
      {description && (
        <p className="text-xs text-muted-foreground ml-6">{description}</p>
      )}
      {isOpen && (
        <div className="ml-4 pl-4 border-l-2 border-border space-y-4">
          {/* "enabled" toggle — always visible */}
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

          {/* Peer fields — only when enabled (or no toggle pattern) */}
          {isFeatureEnabled && peerChildren.map(([key, prop]) => (
            <FormField
              key={key}
              name={`${name}.${key}`}
              property={prop}
              required={property.required?.includes(key)}
              depth={depth + 1}
              deploymentNamespace={deploymentNamespace}
            />
          ))}

          {/* Per-feature inline advanced toggle — only when enabled */}
          {isFeatureEnabled && advancedChild && advancedChild[1].properties && (
            <AdvancedConfigToggle
              advancedSection={inlineAdvancedSection ?? null}
              isExpanded={isInlineAdvancedExpanded}
              onToggle={toggleInlineAdvanced}
            >
              {orderProperties(Object.entries(advancedChild[1].properties), advancedChild[1].propertyOrder).map(([childKey, childProp]) => (
                <FormField
                  key={childKey}
                  name={`${name}.advanced.${childKey}`}
                  property={childProp}
                  required={advancedChild[1].required?.includes(childKey)}
                  depth={depth + 2}
                  deploymentNamespace={deploymentNamespace}
                />
              ))}
            </AdvancedConfigToggle>
          )}
        </div>
      )}
    </div>
  );
}
