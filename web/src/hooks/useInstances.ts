// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, keepPreviousData, useQueryClient } from "@tanstack/react-query";
import {
  listInstances,
  getInstance,
  deleteInstance,
  updateInstanceSpec,
  getInstanceChildren,
} from "@/api/rgd";
import type { InstanceListParams, UpdateInstanceRequest } from "@/types/rgd";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching paginated instance list
 */
export function useInstanceList(params?: InstanceListParams) {
  return useQuery({
    queryKey: ["instances", params],
    queryFn: () => listInstances(params),
    placeholderData: keepPreviousData,
    staleTime: STALE_TIME.FREQUENT, // instances change frequently
  });
}

/**
 * Hook for fetching a single instance by group, namespace, kind, and name.
 * Group is part of the identity key so two CRDs sharing a Kind across different
 * apiGroups produce distinct cache entries. Namespace may be empty for
 * cluster-scoped instances; group MUST be non-empty (the backend rejects
 * empty apiGroup at the route via IsValidAPIGroup).
 */
export function useInstance(group: string, namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance", group, namespace, kind, name],
    queryFn: () => getInstance(group, namespace, kind, name),
    enabled: !!group && !!kind && !!name,
    staleTime: STALE_TIME.REALTIME,
  });
}

/**
 * Hook for deleting an instance
 */
export function useDeleteInstance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ group, namespace, kind, name }: { group: string; namespace: string; kind: string; name: string }) =>
      deleteInstance(group, namespace, kind, name),
    onSettled: (_, __, { group, namespace, kind, name }) => {
      // Always invalidate cache, even if DELETE returns 404 (instance not found)
      // This ensures stale instances are removed from the UI
      queryClient.removeQueries({ queryKey: ["instance", group, namespace, kind, name] });
      queryClient.invalidateQueries({ queryKey: ["instances"] });
      queryClient.invalidateQueries({ queryKey: ["rgds"] });
    },
  });
}

/**
 * Hook for updating an instance spec
 */
export function useUpdateInstanceSpec() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      group,
      namespace,
      kind,
      name,
      request,
    }: {
      group: string;
      namespace: string;
      kind: string;
      name: string;
      request: UpdateInstanceRequest;
    }) => updateInstanceSpec(group, namespace, kind, name, request),
    onSuccess: (_, { group, namespace, kind, name }) => {
      // Invalidate the specific instance and list caches to pick up updated spec
      queryClient.invalidateQueries({ queryKey: ["instance", group, namespace, kind, name] });
      queryClient.invalidateQueries({ queryKey: ["instances"] });
    },
  });
}

/**
 * Hook for fetching child resources of an instance, grouped by node-id.
 */
export function useInstanceChildren(group: string, namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance", group, namespace, kind, name, "children"],
    queryFn: () => getInstanceChildren(group, namespace, kind, name),
    enabled: !!group && !!kind && !!name,
    staleTime: STALE_TIME.FREQUENT, // child resources change with instance
  });
}
