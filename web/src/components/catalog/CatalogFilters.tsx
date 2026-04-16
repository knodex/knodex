// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useState, useCallback, useEffect, useMemo } from "react";
import { Search, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import { MultiSelect } from "@/components/ui/multi-select";
import { cn } from "@/lib/utils";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";

export interface FilterState {
  search: string;
  tags: string[];
  category: string;
  projectScoped: boolean;
  producesKind: string;
}

interface CatalogFiltersProps {
  filters: FilterState;
  onFiltersChange: (filters: FilterState) => void;
  availableTags: string[];
}

export function CatalogFilters({
  filters,
  onFiltersChange,
  availableTags,
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

  const handleProjectScopedToggle = useCallback(() => {
    onFiltersChange({ ...filters, projectScoped: !filters.projectScoped });
  }, [filters, onFiltersChange]);

  const handleClearFilters = useCallback(() => {
    setSearchValue("");
    onFiltersChange({ search: "", tags: [], category: "", projectScoped: false, producesKind: "" });
  }, [onFiltersChange]);

  const hasActiveFilters = useMemo(
    () => filters.search || filters.tags.length > 0 || filters.category || filters.projectScoped || filters.producesKind,
    [filters]
  );

  const normalizedAvailableTags = useMemo(
    () => [...new Set(availableTags.map((t) => t.toLowerCase()))],
    [availableTags]
  );

  return (
    <div className="space-y-3">
      {/* Modern Filter Bar - Vercel-inspired */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-2">
        {/* Search Input */}
        <div className="relative flex-[2] min-w-[280px]">
          <Search className={filterSearchIconClasses} />
          <Input
            type="text"
            placeholder="Search by name, description, or tags..."
            value={searchValue}
            onChange={handleSearchChange}
            className={filterSearchClasses}
            aria-label="Search resource definitions"
          />
          {searchValue && (
            <button
              type="button"
              onClick={() => {
                setSearchValue("");
                onFiltersChange({ ...filters, search: "" });
              }}
              className={filterClearButtonClasses}
              aria-label="Clear search"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {/* Compact Filters - All in one row */}
        <div className="flex flex-wrap gap-2 sm:flex-nowrap">
          {/* Project Scoped Toggle */}
          <button
            type="button"
            onClick={handleProjectScopedToggle}
            className={cn(
              "h-9 px-3 text-sm rounded-[var(--radius-token-md)] border transition-all duration-150 whitespace-nowrap",
              filters.projectScoped
                ? "bg-[var(--brand-primary)]/10 border-[var(--brand-primary)]/30 text-[var(--brand-primary)] font-medium"
                : "bg-transparent border-[var(--border-default)] text-muted-foreground hover:border-[var(--border-hover)] hover:text-foreground"
            )}
            aria-pressed={filters.projectScoped}
            aria-label="Show only project-scoped RGDs"
          >
            Project scoped
          </button>

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
              {filters.category && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80">
                  {filters.category}
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
