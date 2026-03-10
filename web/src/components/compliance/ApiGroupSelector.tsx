// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback } from "react";
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
import { useApiResources } from "@/hooks/useApiResources";

interface ApiGroupSelectorProps {
  value: string[];
  onChange: (value: string[]) => void;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
  "data-testid"?: string;
}

/**
 * Multi-select combobox for Kubernetes API Groups
 * - Displays "core" for the core API group (empty string)
 * - Shows selected groups as removable badges
 * - Supports keyboard navigation
 * - Auto-fetches available API groups from the cluster
 */
export function ApiGroupSelector({
  value,
  onChange,
  placeholder = "Select API groups...",
  disabled = false,
  className,
  "data-testid": testId,
}: ApiGroupSelectorProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");

  const { apiGroups, isLoading, isError, error } = useApiResources();

  // Filter API groups based on search
  const filteredGroups = apiGroups.filter((group) =>
    group.toLowerCase().includes(search.toLowerCase())
  );

  // Toggle a group selection
  const toggleGroup = useCallback(
    (group: string) => {
      if (value.includes(group)) {
        onChange(value.filter((g) => g !== group));
      } else {
        onChange([...value, group]);
      }
    },
    [value, onChange]
  );

  // Remove a group from selection
  const removeGroup = useCallback(
    (group: string) => {
      onChange(value.filter((g) => g !== group));
    },
    [value, onChange]
  );

  // Handle keyboard events for accessibility
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Backspace" && search === "" && value.length > 0) {
        // Remove last selected group on backspace when search is empty
        onChange(value.slice(0, -1));
      }
    },
    [search, value, onChange]
  );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          aria-label="Select API groups"
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
                Loading API groups...
              </span>
            ) : isError ? (
              <span className="text-destructive text-sm">
                Failed to load API groups
              </span>
            ) : value.length === 0 ? (
              <span className="text-muted-foreground">{placeholder}</span>
            ) : (
              value.map((group) => (
                <Badge
                  key={group}
                  variant="secondary"
                  className="mr-1 mb-0.5"
                  data-testid={`selected-api-group-${group}`}
                >
                  {group}
                  <button
                    type="button"
                    className="ml-1 rounded-full outline-none ring-offset-background focus:ring-2 focus:ring-ring focus:ring-offset-2"
                    onClick={(e) => {
                      e.stopPropagation();
                      removeGroup(group);
                    }}
                    aria-label={`Remove ${group}`}
                  >
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              ))
            )}
          </div>
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder="Search API groups..."
            value={search}
            onValueChange={setSearch}
            onKeyDown={handleKeyDown}
            data-testid="api-group-search"
          />
          {filteredGroups.length === 0 ? (
            <CommandEmpty>
              {isError ? (
                <div className="text-destructive text-sm p-2">
                  {error instanceof Error ? error.message : "Failed to load API groups"}
                </div>
              ) : (
                "No API groups found"
              )}
            </CommandEmpty>
          ) : (
            <ScrollArea className="h-[200px] pr-3">
              <div className="p-1">
                {filteredGroups.map((group) => (
                  <CommandItem
                    key={group}
                    value={group}
                    onSelect={() => toggleGroup(group)}
                    data-testid={`api-group-option-${group}`}
                  >
                    <Check
                      className={cn(
                        "mr-2 h-4 w-4",
                        value.includes(group) ? "opacity-100" : "opacity-0"
                      )}
                    />
                    <span className={group === "core" ? "font-medium" : ""}>
                      {group}
                    </span>
                    {group === "core" && (
                      <span className="ml-2 text-xs text-muted-foreground">
                        (core API)
                      </span>
                    )}
                  </CommandItem>
                ))}
              </div>
            </ScrollArea>
          )}
        </Command>
      </PopoverContent>
    </Popover>
  );
}

export default ApiGroupSelector;
