// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, keepPreviousData, useQueryClient } from "@tanstack/react-query";
import {
  listInstances,
  getInstance,
  deleteInstance,
  updateInstanceSpec,
  getInstanceCount,
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
 * Hook for fetching instance count (for sidebar badge)
 * Uses lightweight count endpoint to avoid fetching full list
 */
export function useInstanceCount() {
  return useQuery({
    queryKey: ["instances", "count"],
    queryFn: getInstanceCount,
    staleTime: Infinity, // Counts pushed via WebSocket - no polling needed
    refetchOnWindowFocus: false,
    refetchOnMount: false,
  });
}

/**
 * Hook for fetching a single instance by namespace, kind, and name
 */
export function useInstance(namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance", namespace, kind, name],
    queryFn: () => getInstance(namespace, kind, name),
    enabled: !!kind && !!name, // namespace can be empty for cluster-scoped instances
    staleTime: STALE_TIME.REALTIME,
  });
}

/**
 * Hook for deleting an instance
 */
export function useDeleteInstance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ namespace, kind, name }: { namespace: string; kind: string; name: string }) =>
      deleteInstance(namespace, kind, name),
    onSettled: (_, __, { namespace, kind, name }) => {
      // Always invalidate cache, even if DELETE returns 404 (instance not found)
      // This ensures stale instances are removed from the UI
      queryClient.removeQueries({ queryKey: ["instance", namespace, kind, name] });
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
      namespace,
      kind,
      name,
      request,
    }: {
      namespace: string;
      kind: string;
      name: string;
      request: UpdateInstanceRequest;
    }) => updateInstanceSpec(namespace, kind, name, request),
    onSuccess: (_, { namespace, kind, name }) => {
      // Invalidate the specific instance and list caches to pick up updated spec
      queryClient.invalidateQueries({ queryKey: ["instance", namespace, kind, name] });
      queryClient.invalidateQueries({ queryKey: ["instances"] });
    },
  });
}

/**
 * Hook for fetching child resources of an instance, grouped by node-id.
 */
export function useInstanceChildren(namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance", namespace, kind, name, "children"],
    queryFn: () => getInstanceChildren(namespace, kind, name),
    enabled: !!kind && !!name,
    staleTime: STALE_TIME.FREQUENT, // child resources change with instance
  });
}

