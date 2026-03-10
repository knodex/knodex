// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useState, useCallback, useEffect, useMemo } from "react";
import { Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { MultiSelect } from "@/components/ui/multi-select";
import { cn } from "@/lib/utils";

export interface FilterState {
  search: string;
  tags: string[];
  project: string;
}

interface CatalogFiltersProps {
  filters: FilterState;
  onFiltersChange: (filters: FilterState) => void;
  availableTags: string[];
  availableProjects: string[];
}

// Special value for "All" options since Select requires non-empty values
const ALL_PROJECTS_VALUE = "__all_projects__";

export function CatalogFilters({
  filters,
  onFiltersChange,
  availableTags,
  availableProjects,
}: CatalogFiltersProps) {
  const [searchValue, setSearchValue] = useState(filters.search);

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchValue !== filters.search) {
        onFiltersChange({ ...filters, search: searchValue });
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [searchValue, filters, onFiltersChange]);

  // Sync external search changes (intentional state sync from props)
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- Intentional sync from external filter state
    setSearchValue(filters.search);
  }, [filters.search]);

  const handleSearchChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setSearchValue(e.target.value);
    },
    []
  );

  const handleProjectChange = useCallback(
    (value: string) => {
      const project = value === ALL_PROJECTS_VALUE ? "" : value;
      onFiltersChange({ ...filters, project });
    },
    [filters, onFiltersChange]
  );

  const handleClearFilters = useCallback(() => {
    setSearchValue("");
    onFiltersChange({ search: "", tags: [], project: "" });
  }, [onFiltersChange]);

  const hasActiveFilters = useMemo(
    () => filters.search || filters.tags.length > 0 || filters.project,
    [filters]
  );

  const normalizedAvailableTags = useMemo(
    () => [...new Set(availableTags.map((t) => t.toLowerCase()))],
    [availableTags]
  );

  // Convert filter values to select values
  const projectSelectValue = filters.project || ALL_PROJECTS_VALUE;

  return (
    <div className="space-y-3">
      {/* Modern Filter Bar - Vercel-inspired */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-2">
        {/* Search Input */}
        <div className="relative flex-[2] min-w-[280px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <Input
            type="text"
            placeholder="Search by name, description, or tags..."
            value={searchValue}
            onChange={handleSearchChange}
            className={cn(
              "pl-9 pr-10 h-9 text-sm",
              "bg-background border border-border",
              "hover:border-primary/30 transition-colors duration-200",
              "focus-visible:ring-2 focus-visible:ring-ring/40",
              "placeholder:text-muted-foreground shadow-sm"
            )}
            aria-label="Search resource definitions"
          />
          {searchValue && (
            <button
              type="button"
              onClick={() => {
                setSearchValue("");
                onFiltersChange({ ...filters, search: "" });
              }}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground/60 hover:text-foreground transition-colors"
              aria-label="Clear search"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {/* Compact Filters - All in one row */}
        <div className="flex flex-wrap gap-2 sm:flex-nowrap">
          {/* Project Selector - Only show if there are projects */}
          {availableProjects.length > 0 && (
            <Select
              value={projectSelectValue}
              onValueChange={handleProjectChange}
            >
              <SelectTrigger
                className={cn(
                  "h-9 text-sm min-w-[140px]",
                  "bg-background border border-border shadow-sm",
                  "hover:border-primary/30 hover:bg-muted/30 transition-all duration-200",
                  "focus:ring-2 focus:ring-ring/40",
                  filters.project ? "text-foreground font-medium" : "text-muted-foreground"
                )}
                aria-label="Filter by project"
              >
                <SelectValue placeholder="All projects" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_PROJECTS_VALUE}>All projects</SelectItem>
                {availableProjects.map((project) => (
                  <SelectItem key={project} value={project}>
                    {project}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          {/* Tag Filters */}
          {normalizedAvailableTags.length > 0 && (
            <MultiSelect
              options={normalizedAvailableTags.map((tag) => ({
                label: tag,
                value: tag,
              }))}
              selected={filters.tags}
              onChange={(tags) => onFiltersChange({ ...filters, tags })}
              placeholder="Select tags..."
              className="min-w-[180px]"
            />
          )}
        </div>
      </div>

      {/* Active Filters Indicator */}
      {hasActiveFilters && (
        <div className="flex items-center justify-between text-xs">
          <div className="flex items-center gap-2 text-muted-foreground/70">
            <span>Filters:</span>
            <div className="flex flex-wrap gap-1.5">
              {filters.project && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80">
                  {filters.project}
                </span>
              )}
              {filters.search && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80">
                  {filters.search}
                </span>
              )}
              {filters.tags.length > 0 && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80">
                  {filters.tags.length} tag{filters.tags.length > 1 ? 's' : ''}
                </span>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={handleClearFilters}
            className="flex items-center gap-1 px-2 py-1 rounded-md text-muted-foreground/70 hover:text-foreground hover:bg-muted/30 transition-all duration-200 font-medium"
            aria-label="Clear all filters"
          >
            <X className="h-3 w-3" />
            Clear
          </button>
        </div>
      )}
    </div>
  );
}
