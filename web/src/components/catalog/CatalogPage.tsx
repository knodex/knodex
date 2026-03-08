// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect, useMemo } from "react";
import { RefreshCw } from "lucide-react";
import { useRGDList } from "@/hooks/useRGDs";
import type { CatalogRGD, RGDListParams } from "@/types/rgd";
import { RGDCard } from "./RGDCard";
import { RGDCardSkeleton } from "./RGDCardSkeleton";
import { EmptyState } from "./EmptyState";
import { Pagination } from "./Pagination";
import { CatalogFilters, type FilterState } from "./CatalogFilters";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { getCatalogFiltersFromURL, setCatalogFiltersToURL } from "@/lib/url-utils";
import { getSafeErrorMessage } from "@/lib/errors";

const PAGE_SIZE = 20;

interface CatalogPageProps {
  onRGDClick?: (rgd: CatalogRGD) => void;
}

export function CatalogPage({ onRGDClick }: CatalogPageProps) {
  const [filters, setFilters] = useState<FilterState>(getCatalogFiltersFromURL);
  const [page, setPage] = useState(1);

  // Build query params from filters
  const params: RGDListParams = useMemo(
    () => ({
      page,
      pageSize: PAGE_SIZE,
      search: filters.search || undefined,
      tags: filters.tags.length > 0 ? filters.tags : undefined,
    }),
    [page, filters]
  );

  const { data, isLoading, isError, error, isFetching, refetch } =
    useRGDList(params);

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
      onRGDClick?.(rgd);
    },
    [onRGDClick]
  );

  const handleRetry = useCallback(() => {
    refetch();
  }, [refetch]);

  // Memoize data items for stable reference
  const dataItems = useMemo(() => data?.items ?? [], [data?.items]);

  // Extract available projects from RGD labels (no separate API call needed)
  const availableProjects = useMemo(() => {
    const projects = new Set<string>();
    dataItems.forEach((rgd) => {
      const project = rgd.labels?.["knodex.io/project"];
      if (project) projects.add(project);
    });
    return Array.from(projects).sort();
  }, [dataItems]);

  // Filter RGDs by selected project (client-side filtering)
  // RGDs are cluster-scoped, so we filter by the knodex.io/project label
  const filteredItems = useMemo(() => {
    if (dataItems.length === 0) return [];

    // If no project selected, return all items
    if (!filters.project) return dataItems;

    // Filter RGDs by project label
    // Only show RGDs that belong to the selected project (exact match on label)
    return dataItems.filter((rgd) => {
      const rgdProjectLabel = rgd.labels?.["knodex.io/project"];
      return rgdProjectLabel === filters.project;
    });
  }, [dataItems, filters.project]);

  // Extract available tags from filtered RGDs
  const availableTags = useMemo(() => {
    const tags = new Set<string>();

    filteredItems.forEach((rgd) => {
      rgd.tags?.forEach((tag) => tags.add(tag.toLowerCase()));
    });

    return Array.from(tags).sort();
  }, [filteredItems]);

  const hasActiveFilters =
    !!filters.search || filters.tags.length > 0 || !!filters.project;

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
      {/* Summary bar */}
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          {filteredItems.length} available
          {hasActiveFilters && data && ` (filtered from ${data.totalCount})`}
        </p>
        {isFetching && !isLoading && (
          <span className="flex items-center gap-2 text-xs text-muted-foreground">
            <RefreshCw className="h-3 w-3 animate-spin" />
            Syncing
          </span>
        )}
      </div>

      {/* Filters */}
      <CatalogFilters
        filters={filters}
        onFiltersChange={handleFiltersChange}
        availableTags={availableTags}
        availableProjects={availableProjects}
      />

      {/* Grid */}
      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <RGDCardSkeleton key={i} />
          ))}
        </div>
      ) : filteredItems.length === 0 ? (
        <EmptyState hasFilters={hasActiveFilters} />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 animate-fade-in-up">
            {filteredItems.map((rgd) => (
              <RGDCard
                key={`${rgd.namespace}/${rgd.name}`}
                rgd={rgd}
                onClick={handleRGDClick}
              />
            ))}
          </div>
          {data && (
            <Pagination
              page={data.page}
              pageSize={data.pageSize}
              totalCount={data.totalCount}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}
    </section>
  );
}
