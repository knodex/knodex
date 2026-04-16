// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import { listCategories, getCategory } from "@/api/categories";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching the list of categories visible to the current user.
 * OSS feature — always enabled. Server filters by Casbin rgds/{category}/* policies.
 * Returns empty list if no categories are available.
 */
export function useCategories() {
  return useQuery({
    queryKey: ["categories"],
    queryFn: listCategories,
    staleTime: STALE_TIME.STANDARD,
    retry: 2,
    // Return empty list on error for graceful degradation
    select: (data) => data.categories || [],
  });
}

/**
 * Hook for fetching a specific category by slug.
 * Only enabled when slug is provided.
 */
export function useCategory(slug: string | undefined) {
  return useQuery({
    queryKey: ["categories", slug],
    queryFn: () => getCategory(slug!),
    enabled: !!slug,
    staleTime: STALE_TIME.STANDARD,
    retry: 2,
  });
}

/**
 * Hook for checking if any categories are available for the current user.
 * Returns true if at least one category is visible.
 */
export function useCategoriesEnabled() {
  const { data: categories, isLoading, error } = useCategories();

  return {
    enabled: !isLoading && !error && (categories?.length ?? 0) > 0,
    isLoading,
    categories,
  };
}
