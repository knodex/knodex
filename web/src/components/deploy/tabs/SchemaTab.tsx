// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useWatch, useFormContext } from "react-hook-form";
import type { DeployTab } from "@/lib/build-tabs";
import { orderEntries } from "@/lib/order-properties";
import { FormField } from "@/components/deploy/FormField";

interface SchemaTabProps {
  tab: DeployTab;
}

export function SchemaTab({ tab }: SchemaTabProps) {
  const { control } = useFormContext();
  const deploymentNamespace = (useWatch({ control, name: "namespace" }) as string | undefined) ?? "";
  const requiredSet = useMemo(
    () => new Set(tab.required ?? []),
    [tab.required]
  );

  const ordered = useMemo(
    () => orderEntries(Object.entries(tab.properties ?? {}), tab.propertyOrder),
    [tab.properties, tab.propertyOrder]
  );

  // For the General tab use bare keys; for object-typed tabs nest under tab.id.
  const fieldName = (key: string) =>
    tab.kind === "general" ? key : `${tab.id}.${key}`;

  return (
    <div className="space-y-4" data-testid={`schema-tab-${tab.id}`}>
      {ordered.map(([key, prop]) => (
        <FormField
          key={`${tab.id}-${key}`}
          name={fieldName(key)}
          property={prop}
          required={requiredSet.has(key)}
          deploymentNamespace={deploymentNamespace}
        />
      ))}
    </div>
  );
}
