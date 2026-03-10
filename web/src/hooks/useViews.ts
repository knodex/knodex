// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import { listViews, getView, isViewsUnavailable } from "@/api/views";
import { isEnterprise } from "./useCompliance";

/**
 * Hook for fetching the list of configured views.
 * Only enabled when enterprise features are active.
 * Returns empty list if no views configured or feature unavailable.
 */
export function useViews() {
  return useQuery({
    queryKey: ["views"],
    queryFn: listViews,
    enabled: isEnterprise(),
    staleTime: 60 * 1000, // 1 minute - view config changes infrequently
    retry: (failureCount, error) => {
      // Don't retry on 404 (OSS) or 503 (no config)
      if (isViewsUnavailable(error)) {
        return false;
      }
      return failureCount < 2;
    },
    // Return empty list on error for graceful degradation
    select: (data) => data.views || [],
  });
}

/**
 * Hook for fetching a specific view by slug.
 * Only enabled when enterprise features are active and slug is provided.
 */
export function useView(slug: string | undefined) {
  return useQuery({
    queryKey: ["views", slug],
    queryFn: () => getView(slug!),
    enabled: isEnterprise() && !!slug,
    staleTime: 60 * 1000,
    retry: (failureCount, error) => {
      if (isViewsUnavailable(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for checking if custom views are available.
 * Returns true if enterprise build AND at least one view is configured.
 */
export function useViewsEnabled() {
  const { data: views, isLoading, error } = useViews();

  return {
    enabled: !isLoading && !error && (views?.length ?? 0) > 0,
    isLoading,
    views,
  };
}
