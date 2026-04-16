// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect, useMemo } from "react";
import { RefreshCw, LayoutGrid, List } from "@/lib/icons";
import { useRGDList, useRGDFilters } from "@/hooks/useRGDs";
import type { CatalogRGD, RGDListParams } from "@/types/rgd";
import { CatalogCard } from "./catalog-card";
import { CatalogCardSkeleton } from "./catalog-card-skeleton";
import { EmptyState } from "./EmptyState";
import { Pagination } from "./Pagination";
import { CatalogFilters, type FilterState } from "./CatalogFilters";
import { CatalogListView } from "./CatalogListView";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { getCatalogFiltersFromURL, setCatalogFiltersToURL } from "@/lib/url-utils";
import { getSafeErrorMessage } from "@/lib/errors";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/layout/PageHeader";
import { usePreferencesStore } from "@/stores/preferencesStore";
import { useCurrentProject } from "@/hooks/useAuth";

const PAGE_SIZE = 20;
const CATALOG_VIEW_KEY = "catalog-view-mode";
type ViewMode = "grid" | "list";

interface CatalogPageProps {
  onRGDClick?: (rgd: CatalogRGD) => void;
}

export function CatalogPage({ onRGDClick }: CatalogPageProps) {
  const currentProject = useCurrentProject();
  const [filters, setFilters] = useState<FilterState>(getCatalogFiltersFromURL);
  const [page, setPage] = useState(1);

  // Preferences: favorites and recents
  const { recentRgds, hydrate, addRecent } = usePreferencesStore();

  // Hydrate preferences on first mount
  useEffect(() => {
    hydrate();
  }, [hydrate]);

  // View mode: grid or list, persisted to localStorage
  const [viewMode, setViewMode] = useState<ViewMode>(() => {
    const stored = localStorage.getItem(CATALOG_VIEW_KEY);
    return stored === "grid" ? "grid" : "list";
  });

  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    localStorage.setItem(CATALOG_VIEW_KEY, mode);
  }, []);

  // Build query params from filters
  const params: RGDListParams = useMemo(
    () => ({
      page,
      pageSize: PAGE_SIZE,
      search: filters.search || undefined,
      tags: filters.tags.length > 0 ? filters.tags : undefined,
      category: filters.category || undefined,
      producesKind: filters.producesKind || undefined,
      status: "Active",
    }),
    [page, filters]
  );

  const { data, isLoading, isError, error, isFetching, refetch } =
    useRGDList(params);

  const { data: filterOptions } = useRGDFilters();

  // Sync filters to URL
  useEffect(() => {
    setCatalogFiltersToURL(filters);
  }, [filters]);

  // Reset to page 1 when filters change
  const handleFiltersChange = useCallback((newFilters: FilterState) => {
    setFilters(newFilters);
    setPage(1);
  }, []);

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage);
  }, []);

  const handleRGDClick = useCallback(
    (rgd: CatalogRGD) => {
      addRecent(rgd.name);
      onRGDClick?.(rgd);
    },
    [onRGDClick, addRecent]
  );

  const handleRetry = useCallback(() => {
    refetch();
  }, [refetch]);

  // Memoize data items for stable reference
  const dataItems = useMemo(() => data?.items ?? [], [data?.items]);

  // Filter RGDs by project scope (client-side). Status and category are now server-side.
  const filteredItems = useMemo(() => {
    if (dataItems.length === 0) return [];

    // Project-scoped toggle is the only remaining client-side filter
    if (!filters.projectScoped) return dataItems;

    return dataItems.filter((rgd) => {
      const rgdProjectLabel = rgd.labels?.["knodex.io/project"];
      if (!rgdProjectLabel) return false;
      if (currentProject && rgdProjectLabel !== currentProject) return false;
      return true;
    });
  }, [dataItems, currentProject, filters.projectScoped]);

  // Available tags come from the server's filters endpoint, which returns tags
  // from all authorized RGDs (not just the current page). Consistent with
  // availableCategories sourcing.
  const availableTags = useMemo(
    () => filterOptions?.tags ?? [],
    [filterOptions?.tags]
  );

  // Available categories come from the server's filters endpoint, which returns only
  // categories the user is authorized to see via Casbin category-scoped policies.
  const availableCategories = useMemo(
    () => filterOptions?.categories ?? [],
    [filterOptions?.categories]
  );

  const handleCategoryChange = useCallback((category: string) => {
    setFilters((prev) => ({
      ...prev,
      category: prev.category === category ? "" : category,
    }));
    setPage(1);
  }, []);

  const hasActiveFilters =
    !!filters.search || filters.tags.length > 0 || !!filters.category || filters.projectScoped || !!filters.producesKind;

  // Recently used RGD data (max 5, only when no active filters)
  const recentRgdData = useMemo(() => {
    if (hasActiveFilters || recentRgds.length === 0) return [];
    return recentRgds
      .slice(0, 5)
      .map((name) => dataItems.find((rgd) => rgd.name === name))
      .filter((rgd): rgd is CatalogRGD => rgd != null);
  }, [recentRgds, dataItems, hasActiveFilters]);

  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Alert
          variant="destructive"
          showIcon
          onRetry={handleRetry}
          className="max-w-md"
        >
          <AlertTitle>Failed to load catalog</AlertTitle>
          <AlertDescription>{getSafeErrorMessage(error)}</AlertDescription>
        </Alert>
      </div>
    );
  }

  return (
    <section className="space-y-6">
      {/* Page header (sr-only title) */}
      <PageHeader title="Catalog" />

      {/* Filters + View Toggle (same row) */}
      <div className="flex items-start gap-3">
        <div className="flex-1 min-w-0">
          <CatalogFilters
            filters={filters}
            onFiltersChange={handleFiltersChange}
            availableTags={availableTags}
          />
        </div>
        <div className="flex items-center gap-2 shrink-0 pt-[1px]">
          {isFetching && !isLoading && (
            <span className="flex items-center gap-2 text-xs text-muted-foreground">
              <RefreshCw className="h-3 w-3 animate-spin" />
            </span>
          )}
          <div className="flex items-center h-9 border border-[var(--border-default)] rounded-[var(--radius-token-md)] p-0.5" role="group" aria-label="View mode">
            <button
              onClick={() => handleViewModeChange("grid")}
              className={cn(
                "h-full px-2 rounded-[var(--radius-token-sm)] transition-colors",
                viewMode === "grid"
                  ? "bg-[var(--brand-primary)] text-black"
                  : "text-muted-foreground hover:text-foreground"
              )}
              aria-label="Grid view"
              aria-pressed={viewMode === "grid"}
            >
              <LayoutGrid className="h-3.5 w-3.5" />
            </button>
            <button
              onClick={() => handleViewModeChange("list")}
              className={cn(
                "h-full px-2 rounded-[var(--radius-token-sm)] transition-colors",
                viewMode === "list"
                  ? "bg-[var(--brand-primary)] text-black"
                  : "text-muted-foreground hover:text-foreground"
              )}
              aria-label="List view"
              aria-pressed={viewMode === "list"}
            >
              <List className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Recently Used section */}
      {recentRgdData.length > 0 && (
        <section>
          <h2 className="text-sm font-medium mb-3 text-muted-foreground">
            Recently Used
          </h2>
          <div className="flex items-stretch gap-4 overflow-x-auto pb-2">
            {recentRgdData.map((rgd) => (
              <div key={rgd.name} className="flex-shrink-0 w-[320px] flex">
                <CatalogCard
                  rgd={rgd}
                  onCardClick={handleRGDClick}
                />
              </div>
            ))}
          </div>
        </section>
      )}

      {/* ProducesKind filter indicator */}
      {filters.producesKind && (
        <div className="flex items-center gap-2">
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-primary/10 text-primary">
            Produces: {filters.producesKind}
            <button
              type="button"
              onClick={() => handleFiltersChange({ ...filters, producesKind: "" })}
              className="ml-0.5 hover:text-primary/70 transition-colors"
              aria-label={`Clear produces ${filters.producesKind} filter`}
            >
              &times;
            </button>
          </span>
        </div>
      )}

      {/* Category filter pills */}
      {availableCategories.length > 0 && (
        <div className="flex flex-wrap gap-2" role="group" aria-label="Filter by category">
          {availableCategories.map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => handleCategoryChange(cat)}
              className="px-3 py-1 rounded-full text-xs font-medium transition-colors duration-150"
              style={{
                backgroundColor: filters.category === cat
                  ? "var(--brand-primary)"
                  : "rgba(255,255,255,0.06)",
                color: filters.category === cat
                  ? "var(--surface-bg)"
                  : "hsl(var(--muted-foreground))",
              }}
              aria-pressed={filters.category === cat}
            >
              {cat}
            </button>
          ))}
        </div>
      )}

      {/* Grid or List view */}
      {isLoading ? (
        <div
          className="animate-token-fade-in"
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))",
            gap: "16px",
          }}
        >
          {Array.from({ length: 8 }).map((_, i) => (
            <CatalogCardSkeleton key={i} />
          ))}
        </div>
      ) : filteredItems.length === 0 ? (
        <EmptyState
          hasFilters={hasActiveFilters}
          onClearFilters={() => handleFiltersChange({ search: "", tags: [], category: "", projectScoped: false, producesKind: "" })}
        />
      ) : viewMode === "list" ? (
        <>
          <CatalogListView items={filteredItems} onRGDClick={handleRGDClick} />
          {data && (
            <Pagination
              page={data.page}
              pageSize={data.pageSize}
              totalCount={filters.projectScoped ? filteredItems.length : data?.totalCount ?? filteredItems.length}
              onPageChange={handlePageChange}
            />
          )}
        </>
      ) : (
        <>
          <div
            className="animate-token-fade-in-up"
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))",
              gap: "16px",
            }}
          >
            {filteredItems.map((rgd) => (
              <CatalogCard
                key={`${rgd.namespace}/${rgd.name}`}
                rgd={rgd}
                onCardClick={handleRGDClick}
              />
            ))}
          </div>
          {data && (
            <Pagination
              page={data.page}
              pageSize={data.pageSize}
              totalCount={filters.projectScoped ? filteredItems.length : data?.totalCount ?? filteredItems.length}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}

    </section>
  );
}
