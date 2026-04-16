// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import { listNamespaces, getProjectNamespaces } from "@/api/namespaces";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching all cluster namespaces
 * @param excludeSystem - If true, excludes system namespaces like kube-system (default: true)
 */
export function useNamespaces(excludeSystem: boolean = true) {
  return useQuery({
    queryKey: ["namespaces", { excludeSystem }],
    queryFn: () => listNamespaces(excludeSystem),
    staleTime: STALE_TIME.FREQUENT, // namespaces can change
    gcTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook for fetching namespaces allowed for a specific project
 * Returns real Kubernetes namespaces that match the project's destination patterns
 */
export function useProjectNamespaces(projectName: string) {
  return useQuery({
    queryKey: ["projectNamespaces", projectName],
    queryFn: () => getProjectNamespaces(projectName),
    enabled: !!projectName, // Only fetch when projectName is provided
    staleTime: STALE_TIME.FREQUENT, // namespaces can change
    gcTime: 5 * 60 * 1000, // 5 minutes
  });
}
