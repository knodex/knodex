// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo } from "react";
import { Check, ChevronsUpDown, X, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useApiResources, groupKindsByApiGroup } from "@/hooks/useApiResources";
import { getApiGroupDisplayName } from "@/api/apiResources";

interface KindSelectorProps {
  value: string[];
  onChange: (value: string[]) => void;
  apiGroups: string[];
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  "data-testid"?: string;
}

/**
 * Multi-select combobox for Kubernetes resource Kinds
 * - Filters available kinds based on selected API groups
 * - Groups kinds by API group in the dropdown
 * - Shows selected kinds as removable badges
 * - Supports type-ahead filtering
 */
export function KindSelector({
  value,
  onChange,
  apiGroups,
  placeholder = "Select kinds...",
  disabled = false,
  className,
  "data-testid": testId,
}: KindSelectorProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");

  const { getKindsForApiGroups, isLoading, isError, error } = useApiResources();

  // Get available kinds based on selected API groups
  const availableResources = useMemo(
    () => getKindsForApiGroups(apiGroups),
    [getKindsForApiGroups, apiGroups]
  );

  // Group kinds by API group for display
  const groupedKinds = useMemo(
    () => groupKindsByApiGroup(availableResources),
    [availableResources]
  );

  // Filter kinds based on search (searches across all groups)
  const filteredGroupedKinds = useMemo(() => {
    if (!search) return groupedKinds;

    const searchLower = search.toLowerCase();
    const filtered = new Map<string, string[]>();

    for (const [group, kinds] of groupedKinds) {
      const matchingKinds = kinds.filter((kind) =>
        kind.toLowerCase().includes(searchLower)
      );
      if (matchingKinds.length > 0) {
        filtered.set(group, matchingKinds);
      }
    }

    return filtered;
  }, [groupedKinds, search]);

  // Toggle a kind selection
  const toggleKind = useCallback(
    (kind: string) => {
      if (value.includes(kind)) {
        onChange(value.filter((k) => k !== kind));
      } else {
        onChange([...value, kind]);
      }
    },
    [value, onChange]
  );

  // Remove a kind from selection
  const removeKind = useCallback(
    (kind: string) => {
      onChange(value.filter((k) => k !== kind));
    },
    [value, onChange]
  );

  // Handle keyboard events for accessibility
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Backspace" && search === "" && value.length > 0) {
        // Remove last selected kind on backspace when search is empty
        onChange(value.slice(0, -1));
      }
    },
    [search, value, onChange]
  );

  // Get the API group for a kind to display context in badges
  const getKindApiGroup = useCallback(
    (kind: string): string | undefined => {
      for (const resource of availableResources) {
        if (resource.kind === kind) {
          return getApiGroupDisplayName(resource.apiGroup);
        }
      }
      return undefined;
    },
    [availableResources]
  );

  // Check if there are any kinds to select
  const hasKinds = filteredGroupedKinds.size > 0;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-label="Select resource kinds"
          disabled={disabled || isLoading}
          className={cn(
            "w-full justify-between h-auto min-h-9 px-3 py-2",
            className
          )}
          data-testid={testId}
        >
          <div className="flex flex-wrap gap-1 flex-1 text-left">
            {isLoading ? (
              <span className="text-muted-foreground flex items-center gap-2">
                <Loader2 className="h-3 w-3 animate-spin" />
                Loading kinds...
              </span>
            ) : isError ? (
              <span className="text-destructive text-sm">
                Failed to load kinds
              </span>
            ) : value.length === 0 ? (
              <span className="text-muted-foreground">{placeholder}</span>
            ) : (
              value.map((kind) => {
                const apiGroup = getKindApiGroup(kind);
                return (
                  <Badge
                    key={kind}
                    variant="secondary"
                    className="mr-1 mb-0.5"
                    data-testid={`selected-kind-${kind}`}
                  >
                    {kind}
                    {apiGroup && apiGroups.length !== 1 && (
                      <span className="ml-1 text-xs opacity-70">
                        ({apiGroup})
                      </span>
                    )}
                    <button
                      type="button"
                      className="ml-1 rounded-full outline-none ring-offset-background focus:ring-2 focus:ring-ring focus:ring-offset-2"
                      onClick={(e) => {
                        e.stopPropagation();
                        removeKind(kind);
                      }}
                      aria-label={`Remove ${kind}`}
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </Badge>
                );
              })
            )}
          </div>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="Search kinds..."
            value={search}
            onValueChange={setSearch}
            onKeyDown={handleKeyDown}
            data-testid="kind-search"
          />
          {!hasKinds ? (
            <CommandEmpty>
              {isError ? (
                <div className="text-destructive text-sm p-2">
                  {error instanceof Error
                    ? error.message
                    : "Failed to load kinds"}
                </div>
              ) : apiGroups.length === 0 ? (
                "Select API groups first to see available kinds"
              ) : (
                "No kinds found for selected API groups"
              )}
            </CommandEmpty>
          ) : (
            <ScrollArea className="h-[200px] pr-3">
              <div className="p-1">
                {Array.from(filteredGroupedKinds.entries()).map(([group, kinds]) => (
                  <div key={group}>
                    <div className="px-2 py-1.5 text-xs font-medium text-muted-foreground">
                      {group}
                    </div>
                    {kinds.map((kind) => (
                      <CommandItem
                        key={`${group}-${kind}`}
                        value={`${group}-${kind}`}
                        onSelect={() => toggleKind(kind)}
                        data-testid={`kind-option-${kind}`}
                      >
                        <Check
                          className={cn(
                            "mr-2 h-4 w-4",
                            value.includes(kind) ? "opacity-100" : "opacity-0"
                          )}
                        />
                        {kind}
                      </CommandItem>
                    ))}
                  </div>
                ))}
              </div>
            </ScrollArea>
          )}
        </Command>
      </PopoverContent>
    </Popover>
  );
}

export default KindSelector;
