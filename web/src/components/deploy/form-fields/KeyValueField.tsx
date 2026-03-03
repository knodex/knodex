import { useState } from "react";
import { ChevronDown, ChevronRight, Plus, Trash2 } from "lucide-react";
import type { KeyValueFieldProps } from "./types";

/**
 * Editor for arbitrary key-value pairs.
 * Used for object types without a defined schema.
 */
export function KeyValueField({
  label,
  description,
  value,
  onChange,
  error,
}: KeyValueFieldProps) {
  const [isOpen, setIsOpen] = useState(true);
  const entries = Object.entries(value);

  const addEntry = () => {
    const newKey = `key${entries.length + 1}`;
    onChange({ ...value, [newKey]: "" });
  };

  const removeEntry = (key: string) => {
    const newValue = { ...value };
    delete newValue[key];
    onChange(newValue);
  };

  const updateEntry = (oldKey: string, newKey: string, newValue: string) => {
    const result: Record<string, string> = {};
    for (const [k, v] of Object.entries(value)) {
      if (k === oldKey) {
        result[newKey] = newValue;
      } else {
        result[k] = v;
      }
    }
    onChange(result);
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
          <span className="text-muted-foreground font-normal">
            ({entries.length})
          </span>
        </button>
        <button
          type="button"
          onClick={addEntry}
          className="flex items-center gap-1 text-xs text-primary hover:text-primary/80 transition-colors"
        >
          <Plus className="h-3 w-3" />
          Add
        </button>
      </div>
      {description && (
        <p className="text-xs text-muted-foreground ml-6">{description}</p>
      )}
      {isOpen && entries.length > 0 && (
        <div className="ml-4 pl-4 border-l-2 border-border space-y-2">
          {entries.map(([key, val]) => (
            <div key={key} className="flex items-center gap-2">
              <input
                type="text"
                value={key}
                onChange={(e) => updateEntry(key, e.target.value, val)}
                placeholder="Key"
                className="flex-1 px-2 py-1 text-sm rounded border border-border bg-background"
              />
              <span className="text-muted-foreground">=</span>
              <input
                type="text"
                value={val}
                onChange={(e) => updateEntry(key, key, e.target.value)}
                placeholder="Value"
                className="flex-1 px-2 py-1 text-sm rounded border border-border bg-background"
              />
              <button
                type="button"
                onClick={() => removeEntry(key)}
                className="p-1 text-muted-foreground hover:text-destructive transition-colors"
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
