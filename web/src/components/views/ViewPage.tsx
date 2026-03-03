import { useState, useCallback, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { RefreshCw, ArrowLeft } from "lucide-react";
import { useRGDList } from "@/hooks/useRGDs";
import { useView } from "@/hooks/useViews";
import type { CatalogRGD, RGDListParams } from "@/types/rgd";
import { RGDCard } from "@/components/catalog/RGDCard";
import { RGDCardSkeleton } from "@/components/catalog/RGDCardSkeleton";
import { EmptyState } from "@/components/catalog/EmptyState";
import { Pagination } from "@/components/catalog/Pagination";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { getSafeErrorMessage } from "@/lib/errors";
import { getLucideIcon } from "@/lib/icons";

const PAGE_SIZE = 20;

interface ViewPageProps {
  onRGDClick?: (rgd: CatalogRGD) => void;
}

export function ViewPage({ onRGDClick }: ViewPageProps) {
  const { slug } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const [page, setPage] = useState(1);

  // Fetch view config
  const {
    data: view,
    isLoading: viewLoading,
    isError: viewError,
    error: viewErrorObj,
  } = useView(slug);

  // Build query params with category filter from view config
  const params: RGDListParams = useMemo(
    () => ({
      page,
      pageSize: PAGE_SIZE,
      category: view?.category,
    }),
    [page, view?.category]
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
    navigate("/catalog");
  }, [navigate]);

  // Get view icon - must be before any early returns (rules of hooks)
  const viewIconElement = useMemo(() => {
    const Icon = getLucideIcon(view?.icon);
    return <Icon className="h-5 w-5" />;
  }, [view?.icon]);

  // Show loading state while view config loads
  if (viewLoading) {
    return (
      <section className="space-y-6">
        <div className="flex items-center gap-4">
          <div className="h-8 w-8 bg-muted rounded-lg animate-pulse" />
          <div className="space-y-2">
            <div className="h-6 w-48 bg-muted rounded animate-pulse" />
            <div className="h-4 w-32 bg-muted rounded animate-pulse" />
          </div>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <RGDCardSkeleton key={i} />
          ))}
        </div>
      </section>
    );
  }

  // Show error if view not found
  if (viewError || !view) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <Alert variant="destructive" showIcon className="max-w-md">
          <AlertTitle>View not found</AlertTitle>
          <AlertDescription>
            {viewErrorObj
              ? getSafeErrorMessage(viewErrorObj)
              : `The view "${slug}" could not be found.`}
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
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
            {viewIconElement}
          </div>
          <div>
            <h2 className="text-xl font-semibold text-foreground">
              {view.name}
            </h2>
            <p className="text-sm text-muted-foreground">
              {view.description || `${items.length} resources available`}
            </p>
          </div>
        </div>
        {isFetching && !rgdLoading && (
          <span className="flex items-center gap-2 text-xs text-muted-foreground">
            <RefreshCw className="h-3 w-3 animate-spin" />
            Syncing
          </span>
        )}
      </div>

      {/* Category badge */}
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground">
          Category filter:
        </span>
        <span className="inline-flex items-center rounded-md bg-muted px-2.5 py-0.5 text-sm font-medium">
          {view.category}
        </span>
      </div>

      {/* Grid */}
      {rgdLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <RGDCardSkeleton key={i} />
          ))}
        </div>
      ) : items.length === 0 ? (
        <EmptyState hasFilters={true} />
      ) : (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 animate-fade-in-up">
            {items.map((rgd) => (
              <RGDCard
                key={`${rgd.namespace}/${rgd.name}`}
                rgd={rgd}
                onClick={handleRGDClick}
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
