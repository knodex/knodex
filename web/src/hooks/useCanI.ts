// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import { canI } from "@/api/auth";
import { useIsAuthenticated } from "./useAuth";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for real-time permission checking via the backend Casbin enforcer.
 * Follows ArgoCD pattern: GET /api/v1/account/can-i/{resource}/{action}/{subresource}
 *
 * This replaces client-side JWT permission checking with server-side Casbin evaluation,
 * ensuring permissions are always current even after policy changes.
 *
 * @param resource - Resource type (e.g., 'projects', 'instances', 'repositories')
 * @param action - Action to check (e.g., 'create', 'update', 'delete', 'get')
 * @param subresource - Optional subresource/scope (e.g., project name). Use '-' or omit for no subresource.
 * @returns { allowed: boolean, isLoading: boolean } - Permission result and loading state
 *
 * @example
 * // Check if user can create projects
 * const { allowed: canCreateProject, isLoading } = useCanI('projects', 'create');
 *
 * @example
 * // Check if user can delete instances in a specific project
 * const { allowed: canDelete } = useCanI('instances', 'delete', projectName);
 */
export function useCanI(
  resource: string,
  action: string,
  subresource: string = "-"
): { allowed: boolean | undefined; isLoading: boolean; isError: boolean } {
  const isAuthenticated = useIsAuthenticated();

  const { data, isLoading, isError } = useQuery({
    queryKey: ["can-i", resource, action, subresource],
    queryFn: () => canI(resource, action, subresource),
    enabled: isAuthenticated && !!resource && !!action,
    staleTime: STALE_TIME.STANDARD, // reduces refetch during dev (Tilt resilience)
    gcTime: 5 * 60 * 1000, // 5 minutes garbage collection
    retry: 2, // Retry transient failures (e.g., Tilt server restart)
    retryDelay: 1000, // 1 second between retries
  });

  return {
    allowed: data,
    isLoading,
    isError,
  };
}

/**
 * Hook to check if user can perform an action on ANY resource of a type.
 * Useful for showing/hiding UI elements like "Create" buttons.
 *
 * @param resource - Resource type (e.g., 'projects', 'instances')
 * @param action - Action to check (e.g., 'create', 'delete')
 * @returns { allowed: boolean, isLoading: boolean }
 *
 * @example
 * const { allowed: canCreateAnyProject } = useCanI('projects', 'create');
 * // Shows "Create Project" button if user has projects:create permission
 */
// Note: useCanI with subresource="-" already handles this case

/**
 * Internal hook for checking multiple permissions with configurable aggregation.
 */
function useCanIMultiple(
  permissions: Array<[string, string, string?]>,
  mode: "all" | "any"
): { allowed: boolean | undefined; isLoading: boolean; isError: boolean } {
  const isAuthenticated = useIsAuthenticated();

  const queryKey = [`can-i-${mode}`, ...permissions.map((p) => p.join(":"))];

  const { data, isLoading, isError } = useQuery({
    queryKey,
    queryFn: async () => {
      const results = await Promise.all(
        permissions.map(([resource, action, subresource = "-"]) =>
          canI(resource, action, subresource)
        )
      );
      return mode === "all"
        ? results.every((r) => r === true)
        : results.some((r) => r === true);
    },
    enabled: isAuthenticated && permissions.length > 0,
    staleTime: STALE_TIME.STANDARD,
    gcTime: 5 * 60 * 1000,
    retry: 2,
    retryDelay: 1000,
  });

  return {
    allowed: data,
    isLoading,
    isError,
  };
}

/**
 * Hook to check multiple permissions at once.
 * Returns true only if ALL permissions are granted.
 *
 * @param permissions - Array of [resource, action, subresource?] tuples
 * @returns { allowed: boolean, isLoading: boolean }
 *
 * @example
 * const { allowed } = useCanIAll([
 *   ['projects', 'update', projectName],
 *   ['projects', 'delete', projectName],
 * ]);
 */
export function useCanIAll(
  permissions: Array<[string, string, string?]>
): { allowed: boolean | undefined; isLoading: boolean; isError: boolean } {
  return useCanIMultiple(permissions, "all");
}

/**
 * Hook to check if user can perform an action on at least one of several resources.
 * Returns true if ANY permission is granted.
 *
 * @param permissions - Array of [resource, action, subresource?] tuples
 * @returns { allowed: boolean, isLoading: boolean }
 *
 * @example
 * // Check if user can deploy to at least one project
 * const { allowed: canDeployAnywhere } = useCanIAny([
 *   ['instances', 'create', 'project-a'],
 *   ['instances', 'create', 'project-b'],
 * ]);
 */
export function useCanIAny(
  permissions: Array<[string, string, string?]>
): { allowed: boolean | undefined; isLoading: boolean; isError: boolean } {
  return useCanIMultiple(permissions, "any");
}
