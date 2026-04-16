// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useMemo, useEffect } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { RefreshCw, ArrowLeft, LayoutGrid, List } from "@/lib/icons";
import { useRGDList, useRGDFilters } from "@/hooks/useRGDs";
import { useCategory } from "@/hooks/useCategories";
import type { CatalogRGD, RGDListParams } from "@/types/rgd";
import { CatalogFilters, type FilterState } from "@/components/catalog/CatalogFilters";
import { getCatalogFiltersFromURL, setCatalogFiltersToURL } from "@/lib/url-utils";
import { CatalogCard } from "@/components/catalog/catalog-card";
import { CatalogCardSkeleton } from "@/components/catalog/catalog-card-skeleton";
import { CatalogListView } from "@/components/catalog/CatalogListView";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/catalog/EmptyState";
import { Pagination } from "@/components/catalog/Pagination";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/layout/PageHeader";
import { cn } from "@/lib/utils";
import { getSafeErrorMessage } from "@/lib/errors";
import { getLucideIcon } from "@/lib/icons";

const PAGE_SIZE = 20;
const VIEW_MODE_KEY = "view-page-view-mode";
type ViewMode = "grid" | "list";

interface CategoryPageProps {
  onRGDClick?: (rgd: CatalogRGD) => void;
}

export function CategoryPage({ onRGDClick }: CategoryPageProps) {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState<FilterState>(getCatalogFiltersFromURL);
  const { data: filterOptions } = useRGDFilters();

  // View mode: grid or list, persisted to localStorage
  const [viewMode, setViewMode] = useState<ViewMode>(() => {
    const stored = localStorage.getItem(VIEW_MODE_KEY);
    return stored === "grid" ? "grid" : "list";
  });

  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    localStorage.setItem(VIEW_MODE_KEY, mode);
  }, []);

  // Fetch category config
  const {
    data: category,
    isLoading: categoryLoading,
    isError: categoryError,
    error: categoryErrorObj,
  } = useCategory(slug);

  // Sync filters to URL
  useEffect(() => {
    setCatalogFiltersToURL(filters);
  }, [filters]);

  const handleFiltersChange = useCallback((newFilters: FilterState) => {
    setFilters(newFilters);
    setPage(1);
  }, []);

  // Build query params with category filter + search/status
  const params: RGDListParams = useMemo(
    () => ({
      page,
      pageSize: PAGE_SIZE,
      category: category?.slug,
      search: filters.search || undefined,
      producesKind: filters.producesKind || undefined,
      status: "Active",
    }),
    [page, category?.slug, filters.search, filters.producesKind]
  );

  // Fetch RGDs filtered by category
  const {
    data: rgdData,
    isLoading: rgdLoading,
    isError: rgdError,
    error: rgdErrorObj,
    isFetching,
    refetch,
  } = useRGDList(params);

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage);
  }, []);

  const handleRGDClick = useCallback(
    (rgd: CatalogRGD) => {
      if (onRGDClick) {
        onRGDClick(rgd);
      } else {
        navigate(`/catalog/${encodeURIComponent(rgd.name)}`);
      }
    },
    [onRGDClick, navigate]
  );

  const handleRetry = useCallback(() => {
    refetch();
  }, [refetch]);

  const handleBackToCatalog = useCallback(() => {
    navigate("/catalog", { replace: true });
  }, [navigate]);

  // Get category icon — must be before any early returns (rules of hooks)
  const categoryIconElement = useMemo(() => {
    const Icon = getLucideIcon(category?.icon);
    return <Icon className="h-4 w-4" />;
  }, [category?.icon]);

  // Show loading state while category config loads
  if (categoryLoading) {
    return (
      <section className="space-y-6">
        <div className="flex items-center gap-3">
          <Skeleton className="h-8 w-8 rounded-[var(--radius-token-sm)]" />
          <div className="space-y-2">
            <Skeleton className="h-5 w-48" />
            <Skeleton className="h-3.5 w-32" />
          </div>
        </div>
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
      </section>
    );
  }

  // Show error if category not found
  if (categoryError || !category) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Alert variant="destructive" showIcon className="max-w-md">
          <AlertTitle>Category not found</AlertTitle>
          <AlertDescription>
            {categoryErrorObj
              ? getSafeErrorMessage(categoryErrorObj)
              : `The category "${slug}" could not be found.`}
          </AlertDescription>
        </Alert>
        <Button
          variant="outline"
          className="mt-4"
          onClick={handleBackToCatalog}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Catalog
        </Button>
      </div>
    );
  }

  // Show error if RGD list fails
  if (rgdError) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Alert
          variant="destructive"
          showIcon
          onRetry={handleRetry}
          className="max-w-md"
        >
          <AlertTitle>Failed to load resources</AlertTitle>
          <AlertDescription>{getSafeErrorMessage(rgdErrorObj)}</AlertDescription>
        </Alert>
      </div>
    );
  }

  const items = rgdData?.items ?? [];

  return (
    <section className="space-y-6">
      {/* Page header (sr-only title) */}
      <PageHeader title={category.name} />

      {/* Header + View Toggle */}
      <div className="flex items-start gap-3">
        <div className="flex items-center gap-2.5 flex-1 min-w-0">
          <div
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-token-sm)]"
            style={{ backgroundColor: "rgba(255,255,255,0.06)", color: "var(--text-secondary)" }}
          >
            {categoryIconElement}
          </div>
          <div className="min-w-0">
            <h2
              className="font-semibold leading-snug"
              style={{ fontSize: "16px", fontWeight: 600, color: "var(--text-primary)" }}
            >
              {category.name}
            </h2>
            <p
              className="leading-relaxed"
              style={{ fontSize: "13px", color: "var(--text-secondary)" }}
            >
              {`${rgdData?.totalCount ?? items.length} resources available`}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0 pt-[1px]">
          {isFetching && !rgdLoading && (
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
                  : "text-[var(--text-muted)] hover:text-[var(--text-primary)]"
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
                  : "text-[var(--text-muted)] hover:text-[var(--text-primary)]"
              )}
              aria-label="List view"
              aria-pressed={viewMode === "list"}
            >
              <List className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Search + Status Filters */}
      <CatalogFilters
        filters={filters}
        onFiltersChange={handleFiltersChange}
        availableTags={filterOptions?.tags ?? []}
      />

      {/* Category pill */}
      <div className="flex flex-wrap gap-2">
        <span
          className="px-3 py-1 rounded-full text-xs font-medium"
          style={{
            backgroundColor: "var(--brand-primary)",
            color: "var(--surface-bg)",
          }}
        >
          {category.slug}
        </span>
        {filters.producesKind && (
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
        )}
      </div>

      {/* Grid or List view */}
      {rgdLoading ? (
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
      ) : items.length === 0 ? (
        <EmptyState hasFilters={true} />
      ) : viewMode === "list" ? (
        <>
          <CatalogListView items={items} onRGDClick={handleRGDClick} compact />
          {rgdData && (
            <Pagination
              page={rgdData.page}
              pageSize={rgdData.pageSize}
              totalCount={rgdData.totalCount}
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
            {items.map((rgd) => (
              <CatalogCard
                key={`${rgd.namespace}/${rgd.name}`}
                rgd={rgd}
                onCardClick={handleRGDClick}
              />
            ))}
          </div>
          {rgdData && (
            <Pagination
              page={rgdData.page}
              pageSize={rgdData.pageSize}
              totalCount={rgdData.totalCount}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}
    </section>
  );
}
