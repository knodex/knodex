// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState } from "react";
import { ChevronDown, ChevronRight, Plus, Trash2 } from "lucide-react";
import type { ArrayFieldProps } from "./types";
import { NestedObjectEditor } from "./NestedObjectEditor";

/**
 * Array field with add/remove functionality.
 * Supports arrays of simple types (string, number) and objects.
 */
export function ArrayField({
  name,
  label,
  description,
  property,
  value,
  onChange,
  required,
  error,
  depth,
}: ArrayFieldProps) {
  const [isOpen, setIsOpen] = useState(true);
  const items = property.items;
  const isSimpleType =
    items?.type === "string" ||
    items?.type === "number" ||
    items?.type === "integer";

  const addItem = () => {
    const newItem = isSimpleType
      ? items?.type === "string"
        ? ""
        : 0
      : items?.properties
        ? Object.fromEntries(Object.keys(items.properties).map((k) => [k, ""]))
        : "";
    onChange([...value, newItem]);
  };

  const removeItem = (index: number) => {
    onChange(value.filter((_, i) => i !== index));
  };

  const updateItem = (index: number, newValue: unknown) => {
    const newArray = [...value];
    newArray[index] = newValue;
    onChange(newArray);
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
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
          <span className="text-muted-foreground font-normal">
            ({value.length})
          </span>
        </button>
        <button
          type="button"
          onClick={addItem}
          className="flex items-center gap-1 text-xs text-primary hover:text-primary/80 transition-colors"
        >
          <Plus className="h-3 w-3" />
          Add
        </button>
      </div>
      {description && (
        <p className="text-xs text-muted-foreground ml-6">{description}</p>
      )}
      {isOpen && value.length > 0 && (
        <div className="ml-4 pl-4 border-l-2 border-border space-y-3">
          {value.map((item, index) => (
            <div key={index} className="flex items-start gap-2">
              <div className="flex-1">
                {isSimpleType ? (
                  <input
                    type={items?.type === "string" ? "text" : "number"}
                    value={item as string | number}
                    onChange={(e) =>
                      updateItem(
                        index,
                        items?.type === "string"
                          ? e.target.value
                          : Number(e.target.value)
                      )
                    }
                    className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
                  />
                ) : items?.properties ? (
                  <NestedObjectEditor
                    name={`${name}.${index}`}
                    property={items}
                    value={item as Record<string, unknown>}
                    onChange={(val) => updateItem(index, val)}
                    depth={depth + 1}
                  />
                ) : (
                  <input
                    type="text"
                    value={String(item)}
                    onChange={(e) => updateItem(index, e.target.value)}
                    className="w-full px-3 py-2 text-sm rounded-md border border-border bg-background focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary"
                  />
                )}
              </div>
              <button
                type="button"
                onClick={() => removeItem(index)}
                className="p-2 text-muted-foreground hover:text-destructive transition-colors"
              >
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      )}
      {error && <p className="text-xs text-destructive ml-6">{error}</p>}
    </div>
  );
}
