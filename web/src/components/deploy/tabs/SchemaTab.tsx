// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useWatch, useFormContext } from "react-hook-form";
import type { DeployTab } from "@/lib/build-tabs";
import { orderEntries } from "@/lib/order-properties";
import { renderFlattenableField } from "@/components/deploy/tabs/render-flattenable-field";

interface SchemaTabProps {
  tab: DeployTab;
}

export function SchemaTab({ tab }: SchemaTabProps) {
  const { control } = useFormContext();
  const deploymentNamespace =
    (useWatch({ control, name: "namespace" }) as string | undefined) ?? "";
  const requiredSet = useMemo(
    () => new Set(tab.required ?? []),
    [tab.required]
  );

  const ordered = useMemo(
    () => orderEntries(Object.entries(tab.properties ?? {}), tab.propertyOrder),
    [tab.properties, tab.propertyOrder]
  );

  // General-kind tabs use bare keys (no prefix); object tabs nest under tab.id.
  const parentName = tab.kind === "general" ? "" : tab.id;

  return (
    <div className="space-y-4" data-testid={`schema-tab-${tab.id}`}>
      {ordered.flatMap(([key, prop]) =>
        renderFlattenableField({
          parentName,
          key,
          prop,
          required: requiredSet.has(key),
          deploymentNamespace,
        })
      )}
    </div>
  );
}
