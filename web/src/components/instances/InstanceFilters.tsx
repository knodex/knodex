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
import type { InstanceHealth } from "@/types/rgd";
import type { Project } from "@/types/project";
import { cn } from "@/lib/utils";

export interface InstanceFilterState {
  search: string;
  rgd: string;
  health: InstanceHealth | "";
  project: string;
}

interface InstanceFiltersProps {
  filters: InstanceFilterState;
  onFiltersChange: (filters: InstanceFilterState) => void;
  availableRgds: string[];
  projects: Project[];
  projectsLoading?: boolean;
}

// Special value for "All" options since Select requires non-empty values
const ALL_VALUE = "__all__";
const ALL_PROJECTS_VALUE = "__all_projects__";

const HEALTH_OPTIONS: { value: InstanceHealth | ""; label: string }[] = [
  { value: "", label: "All health" },
  { value: "Healthy", label: "Healthy" },
  { value: "Degraded", label: "Degraded" },
  { value: "Unhealthy", label: "Unhealthy" },
  { value: "Progressing", label: "Progressing" },
  { value: "Unknown", label: "Unknown" },
];

export function InstanceFilters({
  filters,
  onFiltersChange,
  availableRgds,
  projects,
  projectsLoading,
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

  const handleProjectChange = useCallback(
    (value: string) => {
      const project = value === ALL_PROJECTS_VALUE ? "" : value;
      onFiltersChange({ ...filters, project });
    },
    [filters, onFiltersChange]
  );

  const handleClearFilters = useCallback(() => {
    setSearchValue("");
    onFiltersChange({ search: "", rgd: "", health: "", project: "" });
  }, [onFiltersChange]);

  const hasActiveFilters = useMemo(
    () => filters.search || filters.rgd || filters.health || filters.project,
    [filters]
  );

  return (
    <div className="space-y-3">
      {/* Modern Filter Bar - Vercel-inspired */}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-2">
        {/* Search Input */}
        <div className="relative flex-[2] min-w-[280px]">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
          <Input
            type="text"
            placeholder="Search by name..."
            value={searchValue}
            onChange={handleSearchChange}
            className={cn(
              "pl-9 pr-10 h-9 text-sm",
              "bg-background border border-border",
              "hover:border-primary/30 transition-colors duration-200",
              "focus-visible:ring-2 focus-visible:ring-ring/40",
              "placeholder:text-muted-foreground shadow-sm"
            )}
            aria-label="Search instances"
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

        {/* Compact Filters */}
        <div className="flex flex-wrap gap-2 sm:flex-nowrap">
          {/* Project Selector - Only show if user has projects */}
          {projects.length > 0 && (
            <Select
              value={filters.project || ALL_PROJECTS_VALUE}
              onValueChange={handleProjectChange}
              disabled={projectsLoading}
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
                <SelectValue placeholder={projectsLoading ? "Loading..." : "All projects"} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_PROJECTS_VALUE}>All projects</SelectItem>
                {projects.map((project) => (
                  <SelectItem key={project.name} value={project.name}>
                    {project.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}

          {/* RGD Selector */}
          <Select
            value={filters.rgd || ALL_VALUE}
            onValueChange={handleRgdChange}
          >
            <SelectTrigger
              className={cn(
                "h-9 text-sm min-w-[140px]",
                "bg-background border border-border shadow-sm",
                "hover:border-primary/30 hover:bg-muted/30 transition-all duration-200",
                "focus:ring-2 focus:ring-ring/40",
                filters.rgd ? "text-foreground font-medium" : "text-muted-foreground"
              )}
              aria-label="Filter by RGD"
            >
              <SelectValue placeholder="All RGDs" />
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

          {/* Health Selector */}
          <Select
            value={filters.health || ALL_VALUE}
            onValueChange={handleHealthChange}
          >
            <SelectTrigger
              className={cn(
                "h-9 text-sm min-w-[120px]",
                "bg-background border border-border shadow-sm",
                "hover:border-primary/30 hover:bg-muted/30 transition-all duration-200",
                "focus:ring-2 focus:ring-ring/40",
                filters.health ? "text-foreground font-medium" : "text-muted-foreground"
              )}
              aria-label="Filter by health"
            >
              <SelectValue placeholder="All health" />
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
              {filters.rgd && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80">
                  {filters.rgd}
                </span>
              )}
              {filters.health && (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted/40 text-foreground/80">
                  {filters.health}
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
