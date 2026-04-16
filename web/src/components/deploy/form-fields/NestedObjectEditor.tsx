// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { cn } from "@/lib/utils";
import type { NestedObjectEditorProps } from "./types";
import { formatLabel } from "./utils";
import { orderProperties } from "@/lib/order-properties";

/**
 * Inline editor for object items within arrays.
 * Handles simple field types (string, number, boolean) within nested objects.
 */
export function NestedObjectEditor({
  property,
  value,
  onChange,
  depth,
}: NestedObjectEditorProps) {
  if (!property.properties) return null;

  const updateField = (key: string, fieldValue: unknown) => {
    onChange({ ...value, [key]: fieldValue });
  };

  return (
    <div
      className={cn(
        "space-y-3 p-3 rounded-md border border-border bg-secondary/30",
        depth > 2 && "bg-secondary/10"
      )}
    >
      {orderProperties(Object.entries(property.properties), property.propertyOrder).map(([key, prop]) => {
        const fieldValue = value[key];
        const isSimple =
          prop.type === "string" ||
          prop.type === "number" ||
          prop.type === "integer" ||
          prop.type === "boolean";

        if (!isSimple) return null;

        return (
          <div key={key} className="space-y-1">
            <label className="text-xs font-medium text-foreground">
              {prop.title || formatLabel(key)}
              {property.required?.includes(key) && (
                <span className="text-destructive">*</span>
              )}
            </label>
            {prop.type === "boolean" ? (
              <input
                type="checkbox"
                checked={Boolean(fieldValue)}
                onChange={(e) => updateField(key, e.target.checked)}
                className="h-4 w-4 rounded border-border bg-background text-primary"
              />
            ) : prop.enum ? (
              <select
                value={String(fieldValue || "")}
                onChange={(e) => updateField(key, e.target.value)}
                className="w-full px-2 py-1 text-sm rounded border border-border bg-background"
              >
                <option value="">Select...</option>
                {prop.enum.map((opt) => (
                  <option key={String(opt)} value={String(opt)}>
                    {String(opt)}
                  </option>
                ))}
              </select>
            ) : (
              <input
                type={prop.type === "string" ? "text" : "number"}
                value={(fieldValue as string | number) ?? ""}
                onChange={(e) =>
                  updateField(
                    key,
                    prop.type === "string" ? e.target.value : Number(e.target.value)
                  )
                }
                className="w-full px-2 py-1 text-sm rounded border border-border bg-background"
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
