// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Plus, Trash2, Eye, EyeOff } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

import { createPairId } from "./keyValueTypes";
import type { KeyValuePair } from "./keyValueTypes";

interface KeyValueEditorProps {
  pairs: KeyValuePair[];
  onChange: (pairs: KeyValuePair[]) => void;
  errors?: Record<string, string>;
}

export function KeyValueEditor({ pairs, onChange, errors }: KeyValueEditorProps) {
  const addRow = () => {
    onChange([...pairs, { id: createPairId(), key: "", value: "", visible: false }]);
  };

  const removeRow = (index: number) => {
    onChange(pairs.filter((_, i) => i !== index));
  };

  const updateRow = (index: number, field: "key" | "value", val: string) => {
    const updated = pairs.map((pair, i) =>
      i === index ? { ...pair, [field]: val } : pair
    );
    onChange(updated);
  };

  const toggleVisibility = (index: number) => {
    const updated = pairs.map((pair, i) =>
      i === index ? { ...pair, visible: !pair.visible } : pair
    );
    onChange(updated);
  };

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-[1fr_1fr_auto_auto] gap-2 text-sm font-medium text-muted-foreground">
        <span>Key</span>
        <span>Value</span>
        <span />
        <span />
      </div>
      {pairs.map((pair, index) => (
        <div key={pair.id} className="grid grid-cols-[1fr_1fr_auto_auto] gap-2 items-center">
          <Input
            placeholder="KEY_NAME"
            value={pair.key}
            onChange={(e) => updateRow(index, "key", e.target.value)}
            aria-label={`Key ${index + 1}`}
          />
          <Input
            type={pair.visible ? "text" : "password"}
            placeholder="value"
            value={pair.value}
            onChange={(e) => updateRow(index, "value", e.target.value)}
            aria-label={`Value ${index + 1}`}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => toggleVisibility(index)}
            aria-label={pair.visible ? "Hide value" : "Show value"}
          >
            {pair.visible ? (
              <EyeOff className="h-4 w-4" />
            ) : (
              <Eye className="h-4 w-4" />
            )}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => removeRow(index)}
            disabled={pairs.length <= 1}
            aria-label="Remove row"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ))}
      {errors?.keys && (
        <p className="text-sm text-destructive">{errors.keys}</p>
      )}
      <Button type="button" variant="outline" size="sm" onClick={addRow}>
        <Plus className="h-4 w-4 mr-1" />
        Add Key-Value Pair
      </Button>
    </div>
  );
}
