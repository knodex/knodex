import { useQuery, useMutation, keepPreviousData, useQueryClient } from "@tanstack/react-query";
import {
  listInstances,
  getInstance,
  deleteInstance,
  getInstanceCount,
} from "@/api/rgd";
import type { InstanceListParams } from "@/types/rgd";

/**
 * Hook for fetching paginated instance list
 */
export function useInstanceList(params?: InstanceListParams) {
  return useQuery({
    queryKey: ["instances", params],
    queryFn: () => listInstances(params),
    placeholderData: keepPreviousData,
    staleTime: 30 * 1000, // 30 seconds - instances change frequently
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
    enabled: !!namespace && !!kind && !!name,
    staleTime: 15 * 1000, // 15 seconds
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

