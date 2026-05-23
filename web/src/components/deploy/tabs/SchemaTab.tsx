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

  // Feature-toggle pattern (mirrors ObjectField): when the tab's object has an
  // `enabled` boolean child, render the enabled checkbox always but hide peer
  // fields when enabled=false. Only applies to schema-kind tabs since reserved
  // keys prevent a top-level `enabled` from reaching the General tab.
  const hasEnabledToggle =
    tab.kind !== "general" && tab.properties?.enabled?.type === "boolean";
  const enabledFieldName = hasEnabledToggle ? `${tab.id}.enabled` : "";
  const enabledValue = useWatch({
    control,
    name: enabledFieldName || "__schemaTabEnabledNoop__",
  });
  const isFeatureEnabled = hasEnabledToggle ? Boolean(enabledValue) : true;

  return (
    <div className="space-y-4" data-testid={`schema-tab-${tab.id}`}>
      {ordered.flatMap(([key, prop]) => {
        const isPeer = hasEnabledToggle && key !== "enabled";
        if (isPeer && !isFeatureEnabled) return [];
        return renderFlattenableField({
          parentName,
          key,
          prop,
          required: requiredSet.has(key),
          deploymentNamespace,
        });
      })}
    </div>
  );
}
