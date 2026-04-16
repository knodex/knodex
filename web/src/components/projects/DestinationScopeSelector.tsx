// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * DestinationScopeSelector — multi-select for scoping a role to specific destinations.
 * When no destinations are selected, the role gets project-wide access (backward compatible).
 * When destinations are selected, the role's policies only apply to those namespaces.
 */
import { useCallback } from "react";
import { MapPin } from "@/lib/icons";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import type { Destination } from "@/types/project";

interface DestinationScopeSelectorProps {
  /** All destinations defined on the project */
  projectDestinations: Destination[];
  /** Currently selected destinations for this role (empty = project-wide) */
  selectedDestinations: string[];
  /** Callback when selection changes */
  onChange: (destinations: string[]) => void;
  /** Whether the selector is editable */
  canEdit: boolean;
  /** Whether an operation is in progress */
  isLoading?: boolean;
}

export function DestinationScopeSelector({
  projectDestinations,
  selectedDestinations,
  onChange,
  canEdit,
  isLoading = false,
}: DestinationScopeSelectorProps) {
  const isProjectWide = selectedDestinations.length === 0;

  const toggleDestination = useCallback(
    (namespace: string) => {
      if (!canEdit || isLoading) return;
      const isSelected = selectedDestinations.includes(namespace);
      if (isSelected) {
        onChange(selectedDestinations.filter((d) => d !== namespace));
      } else {
        onChange([...selectedDestinations, namespace]);
      }
    },
    [canEdit, isLoading, selectedDestinations, onChange]
  );

  if (projectDestinations.length === 0) return null;

  return (
    <div>
      <Label className="text-muted-foreground mb-2 flex items-center gap-1.5">
        <MapPin className="h-3.5 w-3.5" />
        Destination Scope
      </Label>
      <p className="text-xs text-muted-foreground mb-2">
        {isProjectWide
          ? "This role has access to all project destinations."
          : `This role is restricted to ${selectedDestinations.length} destination${selectedDestinations.length !== 1 ? "s" : ""}.`}
      </p>
      <div className="flex flex-wrap gap-1.5">
        {projectDestinations.map((dest) => {
          const ns = dest.namespace || "";
          const isSelected = selectedDestinations.includes(ns);
          const showAsActive = isProjectWide || isSelected;

          return (
            <Badge
              key={ns}
              variant={showAsActive ? "default" : "outline"}
              className={[
                "cursor-default text-xs transition-colors",
                canEdit && !isLoading ? "cursor-pointer" : "",
                !showAsActive ? "opacity-40" : "",
                isProjectWide && canEdit ? "border-dashed" : "",
              ]
                .filter(Boolean)
                .join(" ")}
              onClick={() => toggleDestination(ns)}
            >
              {ns}
            </Badge>
          );
        })}
      </div>
      {isProjectWide && canEdit && (
        <p className="text-xs text-muted-foreground mt-1.5 italic">
          Click a destination to restrict this role to specific namespaces.
        </p>
      )}
      {!isProjectWide && canEdit && (
        <button
          type="button"
          className="text-xs text-primary hover:underline mt-1.5"
          onClick={() => onChange([])}
          disabled={isLoading}
        >
          Clear scope (grant project-wide access)
        </button>
      )}
    </div>
  );
}

/**
 * Compact read-only display of destination scope for badges/summaries.
 */
export function DestinationScopeBadge({
  destinations,
}: {
  destinations?: string[];
}) {
  if (!destinations || destinations.length === 0) {
    return (
      <Badge variant="outline" className="text-xs">
        All destinations
      </Badge>
    );
  }
  return (
    <Badge variant="outline" className="text-xs">
      <MapPin className="h-3 w-3 mr-1" />
      {destinations.length} dest{destinations.length !== 1 ? "s" : ""}
    </Badge>
  );
}
