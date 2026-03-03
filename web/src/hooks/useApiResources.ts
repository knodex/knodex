import { useQuery } from "@tanstack/react-query";
import { useMemo, useCallback } from "react";
import {
  listApiResources,
  getApiGroupDisplayName,
  type APIResource,
} from "@/api/apiResources";

/**
 * Hook for fetching and managing Kubernetes API resources
 * Used for autocomplete in constraint match rules
 *
 * @returns Object with:
 *   - apiGroups: List of unique API groups (with "core" for empty string)
 *   - resources: Full list of API resources
 *   - getKindsForApiGroups: Function to filter kinds by selected API groups
 *   - isLoading: Loading state
 *   - isError: Error state
 *   - error: Error object
 */
export function useApiResources() {
  const query = useQuery({
    queryKey: ["kubernetes", "api-resources"],
    queryFn: () => listApiResources(),
    staleTime: 5 * 60 * 1000, // 5 minutes - matches backend cache
    gcTime: 10 * 60 * 1000, // 10 minutes
    retry: 2,
  });

  // Extract unique API groups, sorted with "core" first
  const apiGroups = useMemo(() => {
    if (!query.data?.resources) return [];

    const uniqueGroups = new Set<string>();
    for (const resource of query.data.resources) {
      uniqueGroups.add(getApiGroupDisplayName(resource.apiGroup));
    }

    const groups = Array.from(uniqueGroups);
    groups.sort((a, b) => {
      // "core" always comes first
      if (a === "core") return -1;
      if (b === "core") return 1;
      return a.localeCompare(b);
    });

    return groups;
  }, [query.data?.resources]);

  // Function to get kinds filtered by selected API groups
  const getKindsForApiGroups = useCallback(
    (selectedGroups: string[]): APIResource[] => {
      if (!query.data?.resources) return [];

      // If no groups selected, return all resources
      if (selectedGroups.length === 0) {
        return query.data.resources;
      }

      // Convert display names back to API values for comparison
      const groupValues = new Set(
        selectedGroups.map((g) => (g === "core" ? "" : g))
      );

      return query.data.resources.filter((r) => groupValues.has(r.apiGroup));
    },
    [query.data?.resources]
  );

  // Get unique kinds from filtered resources
  const getUniqueKinds = useCallback(
    (selectedGroups: string[]): string[] => {
      const resources = getKindsForApiGroups(selectedGroups);
      const uniqueKinds = new Set<string>();
      for (const resource of resources) {
        uniqueKinds.add(resource.kind);
      }
      return Array.from(uniqueKinds).sort();
    },
    [getKindsForApiGroups]
  );

  return {
    apiGroups,
    resources: query.data?.resources ?? [],
    getKindsForApiGroups,
    getUniqueKinds,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
  };
}

/**
 * Get kinds grouped by API group for display
 * @param resources - List of API resources
 * @returns Map of API group display name to array of kinds
 */
export function groupKindsByApiGroup(
  resources: APIResource[]
): Map<string, string[]> {
  const grouped = new Map<string, string[]>();

  for (const resource of resources) {
    const groupName = getApiGroupDisplayName(resource.apiGroup);
    const kinds = grouped.get(groupName) ?? [];
    if (!kinds.includes(resource.kind)) {
      kinds.push(resource.kind);
    }
    grouped.set(groupName, kinds);
  }

  // Sort kinds within each group
  for (const [group, kinds] of grouped) {
    grouped.set(group, kinds.sort());
  }

  return grouped;
}
