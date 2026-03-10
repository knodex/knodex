// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { ObjectFieldProps } from "./types";
import { FormField } from "./FormField";

/**
 * Collapsible object field that renders nested properties
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
  const [isOpen, setIsOpen] = useState(depth < 2);

  if (!property.properties) return null;

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
          {Object.entries(property.properties).map(([key, prop]) => (
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
      )}
    </div>
  );
}
