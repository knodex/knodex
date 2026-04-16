// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useState, useCallback, useEffect, useMemo } from "react";
import { Link } from "react-router-dom";
import { RefreshCw, LayoutGrid, List, Plus } from "@/lib/icons";
import { useInstanceList } from "@/hooks/useInstances";
import { useProjects } from "@/hooks/useProjects";
import { useCurrentProject } from "@/hooks/useAuth";
import type { Instance, InstanceListParams, InstanceHealth } from "@/types/rgd";
import { StatusCard } from "./StatusCard";
import { InstancesListView } from "./InstancesListView";
import { EmptyState } from "./EmptyState";
import { Pagination } from "@/components/catalog/Pagination";
import { InstanceFilters, type InstanceFilterState } from "./InstanceFilters";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { StatusCardSkeleton } from "./StatusCardSkeleton";
import { getInstanceFiltersFromURL, setInstanceFiltersToURL } from "@/lib/url-utils";
import { getSafeErrorMessage } from "@/lib/errors";
import { filterByProjectNamespaces } from "@/lib/project-utils";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/layout/PageHeader";
import { useIsMobile } from "@/hooks/useIsMobile";
import { MobileInstanceCard } from "./MobileInstanceCard";

const PAGE_SIZE = 20;
const INSTANCES_VIEW_KEY = "instances-view-mode";
type ViewMode = "grid" | "list";

interface InstancesPageProps {
  onInstanceClick?: (instance: Instance) => void;
}

export function InstancesPage({ onInstanceClick }: InstancesPageProps) {
  const isMobile = useIsMobile();
  const currentProject = useCurrentProject();
  const [filters, setFilters] = useState<InstanceFilterState>(() => {
    const urlFilters = getInstanceFiltersFromURL();
    return {
      search: urlFilters.search,
      rgd: urlFilters.rgd,
      health: urlFilters.health as InstanceHealth,
      scope: urlFilters.scope,
    };
  });
  const [page, setPage] = useState(1);

  // View mode: grid or list, persisted to localStorage
  const [viewMode, setViewMode] = useState<ViewMode>(() => {
    const stored = localStorage.getItem(INSTANCES_VIEW_KEY);
    return stored === "list" ? "list" : "grid";
  });

  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    localStorage.setItem(INSTANCES_VIEW_KEY, mode);
  }, []);

  // Fetch user's accessible projects (RBAC-aware) — needed for namespace filtering
  const { data: projectsData } = useProjects();

  // Build query params from filters - use rgdName for backend filtering
  const params: InstanceListParams = useMemo(
    () => ({
      page,
      pageSize: PAGE_SIZE,
      search: filters.search || undefined,
      rgdName: filters.rgd || undefined,
      health: filters.health || undefined,
    }),
    [page, filters]
  );

  const { data, isLoading, isError, error, isFetching, refetch } =
    useInstanceList(params);

  // Sync filters to URL
  useEffect(() => {
    setInstanceFiltersToURL(filters);
  }, [filters]);

  // Reset to page 1 when filters change
  const handleFiltersChange = useCallback((newFilters: InstanceFilterState) => {
    setFilters(newFilters);
    setPage(1);
  }, []);

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage);
  }, []);

  const handleInstanceClick = useCallback(
    (instance: Instance) => {
      onInstanceClick?.(instance);
    },
    [onInstanceClick]
  );

  const handleRetry = useCallback(() => {
    refetch();
  }, [refetch]);

  // Get the list of projects (memoized for stable reference)
  const projects = useMemo(() => projectsData?.items ?? [], [projectsData?.items]);

  // Memoize data items for stable reference
  const dataItems = useMemo(() => data?.items ?? [], [data?.items]);

  // Filter instances by selected project's allowed namespaces and scope (client-side filtering)
  const filteredItems = useMemo(() => {
    if (dataItems.length === 0) return [];

    let items = dataItems;

    // Filter by project's allowed namespaces
    if (currentProject) {
      const selectedProject = projects.find((p) => p.name === currentProject);
      if (selectedProject) {
        items = filterByProjectNamespaces(items, selectedProject);
      }
    }

    // Filter by scope (cluster-scoped vs namespaced)
    if (filters.scope === "cluster") {
      items = items.filter((instance) => instance.isClusterScoped === true);
    } else if (filters.scope === "namespaced") {
      items = items.filter((instance) => !instance.isClusterScoped);
    }

    return items;
  }, [dataItems, currentProject, filters.scope, projects]);

  // Extract available RGDs from all data items (not filtered), so the RGD dropdown
  // doesn't change when scope/project filters are applied.
  const availableRgds = useMemo(() => {
    const rgds = new Set<string>();

    dataItems.forEach((instance) => {
      if (instance.rgdName) {
        rgds.add(instance.rgdName);
      }
    });

    return Array.from(rgds).sort();
  }, [dataItems]);

  const hasActiveFilters =
    !!filters.search ||
    !!filters.rgd ||
    !!filters.health ||
    !!filters.scope;

  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Alert
          variant="destructive"
          showIcon
          onRetry={handleRetry}
          className="max-w-md"
        >
          <AlertTitle>Failed to load instances</AlertTitle>
          <AlertDescription>{getSafeErrorMessage(error)}</AlertDescription>
        </Alert>
      </div>
    );
  }

  return (
    <section className="space-y-6">
      {/* Page header (sr-only title) */}
      <PageHeader title="Instances" />

      {/* Filters + View Toggle + Deploy (same row) */}
      <div className="flex items-start gap-3">
        <div className="flex-1 min-w-0">
          <InstanceFilters
            filters={filters}
            onFiltersChange={handleFiltersChange}
            availableRgds={availableRgds}
          />
        </div>
        {!isMobile && (
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
            <Link
              to="/catalog"
              className="inline-flex items-center h-8 gap-1.5 rounded-[var(--radius-token-md)] px-2.5 text-xs font-medium text-black transition-all duration-150 bg-[var(--brand-primary)] hover:bg-[var(--brand-hover)] active:scale-[0.97]"
              data-testid="deploy-new-button"
            >
              <Plus className="h-3 w-3" />
              Deploy
            </Link>
          </div>
        )}
      </div>

      {/* Grid or List view */}
      {isLoading ? (
        <div className="grid gap-3" style={{ gridTemplateColumns: isMobile ? "1fr" : "repeat(auto-fill, minmax(300px, 1fr))" }}>
          {Array.from({ length: isMobile ? 4 : 8 }).map((_, i) => (
            <StatusCardSkeleton key={i} />
          ))}
        </div>
      ) : filteredItems.length === 0 ? (
        <EmptyState
          hasFilters={hasActiveFilters}
          onClearFilters={() => handleFiltersChange({ search: "", rgd: "", health: "", scope: "" })}
        />
      ) : isMobile ? (
        <div className="flex flex-col gap-2">
          {filteredItems.map((instance) => (
            <MobileInstanceCard
              key={`${instance.namespace || "_cluster"}/${instance.kind}/${instance.name}`}
              instance={instance}
              onClick={handleInstanceClick}
            />
          ))}
          {data && (
            <Pagination
              page={data.page}
              pageSize={data.pageSize}
              totalCount={filteredItems.length}
              onPageChange={handlePageChange}
            />
          )}
        </div>
      ) : viewMode === "list" ? (
        <>
          <InstancesListView items={filteredItems} onInstanceClick={handleInstanceClick} />
          {data && (
            <Pagination
              page={data.page}
              pageSize={data.pageSize}
              totalCount={filteredItems.length}
              onPageChange={handlePageChange}
            />
          )}
        </>
      ) : (
        <>
          <div className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))" }}>
            {filteredItems.map((instance, index) => (
              <div
                key={`${instance.namespace || "_cluster"}/${instance.kind}/${instance.name}`}
                className="animate-card-enter"
                style={{ animationDelay: `${Math.min(index * 40, 400)}ms` }}
              >
                <StatusCard
                  instance={instance}
                  onClick={onInstanceClick ? handleInstanceClick : undefined}
                />
              </div>
            ))}
          </div>
          {data && (
            <Pagination
              page={data.page}
              pageSize={data.pageSize}
              totalCount={filteredItems.length}
              onPageChange={handlePageChange}
            />
          )}
        </>
      )}
    </section>
  );
}
