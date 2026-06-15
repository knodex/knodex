// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * GroupTypeahead — OIDC group entry with a typeahead of observed groups and a
 * raw free-text fallback (FR-T3, Story 10.4).
 *
 * Suggestions come from GET /api/v1/groups/observed (most-recently-seen first).
 * They are a convenience, never a constraint: an operator can always commit a
 * typed value that matches no suggestion (e.g. a group nobody has logged in
 * with yet). With an empty store the field still accepts raw entry — no
 * suggestions, no error. Selected groups render as removable chips.
 */
import { useMemo, useState, useCallback } from "react";
import { Plus, X } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { validateGroupId } from "@/lib/oidc-groups";
import type { ObservedGroup } from "@/types/team";

interface GroupTypeaheadProps {
  /** Currently selected group strings. */
  selected: string[];
  /** Called with the new selection when a group is added or removed. */
  onChange: (groups: string[]) => void;
  /** Observed groups to suggest, most-recently-seen first. */
  suggestions: ObservedGroup[];
  /** Whether the user may edit. */
  canEdit: boolean;
  /** Disables inputs while a parent mutation is in flight. */
  isLoading?: boolean;
}

export function GroupTypeahead({
  selected,
  onChange,
  suggestions,
  canEdit,
  isLoading = false,
}: GroupTypeaheadProps) {
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  // Filter suggestions by typed text (case-insensitive substring), preserving
  // the API's most-recent-first order, and hiding already-selected groups.
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return suggestions
      .filter((s) => !selected.includes(s.name))
      .filter((s) => q === "" || s.name.toLowerCase().includes(q));
  }, [suggestions, selected, query]);

  const commit = useCallback(
    (value: string) => {
      const trimmed = value.trim();
      const validationError = validateGroupId(trimmed);
      if (validationError) {
        setError(validationError);
        return;
      }
      if (selected.includes(trimmed)) {
        setError("This group is already added");
        return;
      }
      onChange([...selected, trimmed]);
      setQuery("");
      setError(null);
      setOpen(false);
    },
    [selected, onChange]
  );

  const handleRemove = useCallback(
    (group: string) => {
      onChange(selected.filter((g) => g !== group));
    },
    [selected, onChange]
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") {
        e.preventDefault();
        // Free-text fallback: commit exactly what was typed, suggestion or not.
        commit(query);
      } else if (e.key === "Escape") {
        setOpen(false);
      }
    },
    [commit, query]
  );

  return (
    <div className="space-y-3" data-testid="group-typeahead">
      <Label className="text-muted-foreground">OIDC Groups</Label>

      {/* Selected groups as chips */}
      {selected.length > 0 ? (
        <div className="flex flex-wrap gap-2" data-testid="group-chips">
          {selected.map((group) => (
            <Badge
              key={group}
              variant="secondary"
              className="flex items-center gap-1 py-1 px-2 font-mono text-xs"
              data-testid={`group-chip-${group}`}
            >
              <span className="max-w-[200px] truncate">{group}</span>
              {canEdit && (
                <button
                  type="button"
                  onClick={() => handleRemove(group)}
                  disabled={isLoading}
                  className="ml-1 hover:text-destructive focus:outline-none rounded"
                  aria-label={`Remove group ${group}`}
                  data-testid={`remove-group-${group}`}
                >
                  <X className="h-3 w-3" />
                </button>
              )}
            </Badge>
          ))}
        </div>
      ) : (
        <p className="text-sm text-muted-foreground italic">No groups added</p>
      )}

      {/* Typeahead input + suggestions */}
      {canEdit && (
        <div className="space-y-2">
          <div className="relative">
            <div className="flex gap-2">
              <Input
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setError(null);
                  setOpen(true);
                }}
                onFocus={() => setOpen(true)}
                onKeyDown={handleKeyDown}
                placeholder="Type or pick an OIDC group"
                className="flex-1 font-mono text-sm h-8"
                disabled={isLoading}
                aria-label="OIDC group"
                data-testid="group-input"
              />
              <Button
                type="button"
                size="sm"
                onClick={() => commit(query)}
                disabled={isLoading || !query.trim()}
                className="h-8"
                data-testid="group-add-button"
              >
                <Plus className="h-4 w-4 mr-1" />
                Add
              </Button>
            </div>

            {open && filtered.length > 0 && (
              <div
                className="absolute z-10 mt-1 w-full rounded-md border bg-popover p-1 shadow-md max-h-48 overflow-y-auto"
                data-testid="group-suggestions"
              >
                {filtered.map((s) => (
                  <button
                    key={s.name}
                    type="button"
                    onClick={() => commit(s.name)}
                    className="flex w-full items-center justify-between gap-2 rounded-sm px-2 py-1.5 text-sm outline-none hover:bg-accent hover:text-accent-foreground"
                    data-testid={`group-suggestion-${s.name}`}
                  >
                    <span className="font-mono truncate">{s.name}</span>
                  </button>
                ))}
              </div>
            )}
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
          <p className="text-xs text-muted-foreground">
            Pick from groups seen at login, or type any group from your identity
            provider and press Enter.
          </p>
        </div>
      )}
    </div>
  );
}
