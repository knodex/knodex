import { useState, useCallback, useEffect, useMemo } from "react";
import { RefreshCw } from "lucide-react";
import { useInstanceList } from "@/hooks/useInstances";
import { useProjects } from "@/hooks/useProjects";
import type { Instance, InstanceListParams, InstanceHealth } from "@/types/rgd";
import { InstanceCard } from "./InstanceCard";
import { InstanceCardSkeleton } from "./InstanceCardSkeleton";
import { EmptyState } from "./EmptyState";
import { Pagination } from "@/components/catalog/Pagination";
import { InstanceFilters, type InstanceFilterState } from "./InstanceFilters";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { getInstanceFiltersFromURL, setInstanceFiltersToURL } from "@/lib/url-utils";
import { getSafeErrorMessage } from "@/lib/errors";
import { filterByProjectNamespaces } from "@/lib/project-utils";

const PAGE_SIZE = 20; // Increased from 20 to handle more instances (Global Admin can see many instances across orgs)

interface InstancesPageProps {
  onInstanceClick?: (instance: Instance) => void;
}

export function InstancesPage({ onInstanceClick }: InstancesPageProps) {
  const [filters, setFilters] = useState<InstanceFilterState>(() => {
    const urlFilters = getInstanceFiltersFromURL();
    return {
      ...urlFilters,
      health: urlFilters.health as InstanceHealth,
    };
  });
  const [page, setPage] = useState(1);

  // Fetch user's accessible projects (RBAC-aware)
  const { data: projectsData, isLoading: projectsLoading } = useProjects();

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

  // Filter instances by selected project's allowed namespaces (client-side filtering)
  const filteredItems = useMemo(() => {
    if (dataItems.length === 0) return [];

    // If no project selected, return all items
    if (!filters.project) return dataItems;

    // Find the selected project
    const selectedProject = projects.find((p) => p.name === filters.project);
    if (!selectedProject) return dataItems;

    // Filter by project's allowed namespaces
    return filterByProjectNamespaces(dataItems, selectedProject);
  }, [dataItems, filters.project, projects]);

  // Extract available RGDs from filtered instances
  const availableRgds = useMemo(() => {
    const rgds = new Set<string>();

    filteredItems.forEach((instance) => {
      if (instance.rgdName) {
        rgds.add(instance.rgdName);
      }
    });

    return Array.from(rgds).sort();
  }, [filteredItems]);

  const hasActiveFilters =
    !!filters.search ||
    !!filters.rgd ||
    !!filters.health ||
    !!filters.project;

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
      <InstanceFilters
        filters={filters}
        onFiltersChange={handleFiltersChange}
        availableRgds={availableRgds}
        projects={projects}
        projectsLoading={projectsLoading}
      />

      {/* Grid */}
      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <InstanceCardSkeleton key={i} />
          ))}
        </div>
      ) : filteredItems.length === 0 ? (
        <EmptyState hasFilters={hasActiveFilters} />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 animate-fade-in-up">
            {filteredItems.map((instance) => (
              <InstanceCard
                key={`${instance.namespace}/${instance.kind}/${instance.name}`}
                instance={instance}
                onClick={handleInstanceClick}
              />
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
