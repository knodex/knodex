// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import React, { useState, useCallback, useEffect, useMemo } from "react";
import { Search, X } from "@/lib/icons";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
} from "@/components/ui/select";
import type { InstanceHealth } from "@/types/rgd";
import { cn } from "@/lib/utils";
import {
  filterSearchClasses,
  filterSearchIconClasses,
  filterClearButtonClasses,
} from "@/components/ui/filter-bar";
import { FilterChip, FilterChipDot, filterChipClasses } from "@/components/ui/filter-chip";
import { FiltersDropdown } from "@/components/ui/filters-dropdown";

export interface InstanceFilterState {
  search: string;
  rgd: string;
  health: InstanceHealth | "";
  scope: string; // "" = all, "namespaced" = namespace-scoped only, "cluster" = cluster-scoped only
}

interface InstanceFiltersProps {
  filters: InstanceFilterState;
  onFiltersChange: (filters: InstanceFilterState) => void;
  availableRgds: string[];
}

// Special value for "All" options since Select requires non-empty values
const ALL_VALUE = "__all__";
const ALL_SCOPE_VALUE = "__all_scope__";

const HEALTH_OPTIONS: { value: InstanceHealth | ""; label: string }[] = [
  { value: "", label: "All health" },
  { value: "Healthy", label: "Healthy" },
  { value: "Degraded", label: "Degraded" },
  { value: "Unhealthy", label: "Unhealthy" },
  { value: "Progressing", label: "Progressing" },
  { value: "Unknown", label: "Unknown" },
];

const SCOPE_LABEL: Record<string, string> = {
  namespaced: "Namespaced",
  cluster: "Cluster-Scoped",
};

export function InstanceFilters({
  filters,
  onFiltersChange,
  availableRgds,
}: InstanceFiltersProps) {
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

  const handleRgdChange = useCallback(
    (value: string) => {
      const rgd = value === ALL_VALUE ? "" : value;
      onFiltersChange({ ...filters, rgd });
    },
    [filters, onFiltersChange]
  );

  const handleHealthChange = useCallback(
    (value: string) => {
      const health = value === ALL_VALUE ? "" : (value as InstanceHealth);
      onFiltersChange({ ...filters, health });
    },
    [filters, onFiltersChange]
  );

  const handleScopeChange = useCallback(
    (value: string) => {
      const scope = value === ALL_SCOPE_VALUE ? "" : value;
      onFiltersChange({ ...filters, scope });
    },
    [filters, onFiltersChange]
  );

  const handleClearFilters = useCallback(() => {
    setSearchValue("");
    onFiltersChange({ search: "", rgd: "", health: "", scope: "" });
  }, [onFiltersChange]);

  const hasActiveFilters = useMemo(
    () => filters.search || filters.rgd || filters.health || filters.scope,
    [filters]
  );

  // Non-search active filters drive the FiltersDropdown count badge.
  const activeFilterCount =
    (filters.rgd ? 1 : 0) + (filters.health ? 1 : 0) + (filters.scope ? 1 : 0);

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-2">
        {/* Search Input */}
        <div className="relative flex-[2] min-w-[280px]">
          <Search className={filterSearchIconClasses} />
          <Input
            type="text"
            placeholder="Filter instances..."
            value={searchValue}
            onChange={handleSearchChange}
            className={filterSearchClasses}
            aria-label="Search instances"
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

        {/* Collapsed filter chips — single Filters button with popover */}
        <FiltersDropdown activeCount={activeFilterCount}>
          <div className="flex flex-col items-start gap-2">
            <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground/70">
              Filter by
            </span>

            {/* RGD chip */}
            <Select
              value={filters.rgd || ALL_VALUE}
              onValueChange={handleRgdChange}
            >
              <SelectTrigger
                className={cn(
                  filterChipClasses(filters.rgd ? "active" : "idle"),
                  "max-w-[220px]"
                )}
                aria-label="Filter by RGD"
              >
                <span className="inline-flex items-center gap-1.5 truncate">
                  {filters.rgd && <FilterChipDot />}
                  <span className="truncate">
                    {filters.rgd ? `RGD: ${filters.rgd}` : "RGD"}
                  </span>
                </span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_VALUE}>All RGDs</SelectItem>
                {availableRgds.filter(rgd => rgd).map((rgd) => (
                  <SelectItem key={rgd} value={rgd} className="text-xs">
                    {rgd}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {/* Health chip */}
            <Select
              value={filters.health || ALL_VALUE}
              onValueChange={handleHealthChange}
            >
              <SelectTrigger
                className={cn(
                  filterChipClasses(filters.health ? "active" : "idle"),
                  "max-w-[220px]"
                )}
                aria-label="Filter by health"
              >
                <span className="inline-flex items-center gap-1.5 truncate">
                  {filters.health && <FilterChipDot />}
                  <span className="truncate">
                    {filters.health ? `Health: ${filters.health}` : "Health"}
                  </span>
                </span>
              </SelectTrigger>
              <SelectContent>
                {HEALTH_OPTIONS.map((opt) => (
                  <SelectItem
                    key={opt.value || ALL_VALUE}
                    value={opt.value || ALL_VALUE}
                    className="text-xs"
                  >
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {/* Scope chip */}
            <Select
              value={filters.scope || ALL_SCOPE_VALUE}
              onValueChange={handleScopeChange}
            >
              <SelectTrigger
                className={cn(
                  filterChipClasses(filters.scope ? "active" : "idle"),
                  "max-w-[220px]"
                )}
                aria-label="Filter by scope"
              >
                <span className="inline-flex items-center gap-1.5 truncate">
                  {filters.scope && <FilterChipDot />}
                  <span className="truncate">
                    {filters.scope
                      ? `Scope: ${SCOPE_LABEL[filters.scope]}`
                      : "Scope"}
                  </span>
                </span>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_SCOPE_VALUE}>All scopes</SelectItem>
                <SelectItem value="namespaced" className="text-xs">Namespaced</SelectItem>
                <SelectItem value="cluster" className="text-xs">Cluster-Scoped</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </FiltersDropdown>
      </div>

      {/* Active Filters Indicator */}
      {hasActiveFilters && (
        <div className="flex items-center justify-between text-xs">
          <div className="flex items-center gap-2 text-muted-foreground/70">
            <span>Filters:</span>
            <div className="flex flex-wrap gap-1.5">
              {filters.search && (
                <FilterChip
                  state="active"
                  showDot
                  onClick={() => {
                    setSearchValue("");
                    onFiltersChange({ ...filters, search: "" });
                  }}
                  aria-label={`Remove ${filters.search} filter`}
                  data-testid="remove-search-filter"
                  className="h-6 px-2 text-[11px]"
                >
                  <span>{filters.search}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </FilterChip>
              )}
              {filters.rgd && (
                <FilterChip
                  state="active"
                  showDot
                  onClick={() => onFiltersChange({ ...filters, rgd: "" })}
                  aria-label={`Remove ${filters.rgd} filter`}
                  data-testid="remove-rgd-filter"
                  className="h-6 px-2 text-[11px]"
                >
                  <span>{filters.rgd}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </FilterChip>
              )}
              {filters.health && (
                <FilterChip
                  state="active"
                  showDot
                  onClick={() => onFiltersChange({ ...filters, health: "" })}
                  aria-label={`Remove ${filters.health} filter`}
                  data-testid="remove-health-filter"
                  className="h-6 px-2 text-[11px]"
                >
                  <span>{filters.health}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </FilterChip>
              )}
              {filters.scope && (
                <FilterChip
                  state="active"
                  showDot
                  onClick={() => onFiltersChange({ ...filters, scope: "" })}
                  aria-label="Remove scope filter"
                  data-testid="remove-scope-filter"
                  className="h-6 px-2 text-[11px]"
                >
                  <span>{SCOPE_LABEL[filters.scope] ?? filters.scope}</span>
                  <X className="h-3 w-3 text-muted-foreground/70" />
                </FilterChip>
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
