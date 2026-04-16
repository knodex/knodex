// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Globe } from "@/lib/icons";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

interface NamespaceSelectorProps {
  value: string;
  onChange: (namespace: string) => void;
  namespaces: string[];
  className?: string;
}

// Special value for "All namespaces" option since Select requires non-empty values
const ALL_NAMESPACES_VALUE = "__all__";

export function NamespaceSelector({
  value,
  onChange,
  namespaces,
  className,
}: NamespaceSelectorProps) {
  const handleValueChange = (newValue: string) => {
    onChange(newValue === ALL_NAMESPACES_VALUE ? "" : newValue);
  };

  // Convert value to select value (empty string becomes the special value)
  const selectValue = value || ALL_NAMESPACES_VALUE;

  return (
    <Select value={selectValue} onValueChange={handleValueChange}>
      <SelectTrigger
        className={cn(
          "h-8 px-2.5 text-sm border-border bg-card",
          "w-auto min-w-[140px]",
          value ? "text-foreground" : "text-muted-foreground",
          className
        )}
        aria-label="Select namespace"
      >
        <Globe className="h-3.5 w-3.5 mr-1.5 shrink-0" />
        <SelectValue placeholder="All namespaces">
          <span className="max-w-[120px] truncate font-mono text-xs">
            {value || "All namespaces"}
          </span>
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_NAMESPACES_VALUE}>All namespaces</SelectItem>
        {namespaces.filter(ns => ns).length === 0 ? (
          <div className="px-3 py-2 text-sm text-muted-foreground italic">
            No namespaces found
          </div>
        ) : (
          namespaces.filter(ns => ns).map((ns) => (
            <SelectItem key={ns} value={ns} className="font-mono text-sm">
              {ns}
            </SelectItem>
          ))
        )}
      </SelectContent>
    </Select>
  );
}
