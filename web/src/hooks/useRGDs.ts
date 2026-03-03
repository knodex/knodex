import { useQuery, useMutation, keepPreviousData, useQueryClient } from "@tanstack/react-query";
import { listRGDs, getRGD, getRGDSchema, listRGDInstances, createInstance, getRGDResourceGraph, getRGDCount } from "@/api/rgd";
import { listK8sResources } from "@/api/k8s";
import type { RGDListParams, CreateInstanceRequest } from "@/types/rgd";

/**
 * Hook for fetching paginated RGD list
 */
export function useRGDList(params?: RGDListParams) {
  return useQuery({
    queryKey: ["rgds", params],
    queryFn: () => listRGDs(params),
    placeholderData: keepPreviousData,
    staleTime: 30 * 1000, // 30 seconds - prevent immediate refetches on filter changes
  });
}

/**
 * Hook for fetching RGD count (for sidebar badge)
 * Uses lightweight count endpoint to avoid fetching full list
 */
export function useRGDCount() {
  return useQuery({
    queryKey: ["rgds", "count"],
    queryFn: getRGDCount,
    staleTime: Infinity, // Counts pushed via WebSocket - no polling needed
    refetchOnWindowFocus: false,
    refetchOnMount: false,
  });
}

/**
 * Hook for fetching a single RGD by name
 */
export function useRGD(name: string, namespace?: string) {
  return useQuery({
    queryKey: ["rgd", name, namespace],
    queryFn: () => getRGD(name, namespace),
    enabled: !!name,
  });
}

/**
 * Hook for fetching the internal resource graph of an RGD
 * Shows K8s resources (templates and externalRefs) within a single RGD
 */
export function useRGDResourceGraph(name: string, namespace?: string) {
  return useQuery({
    queryKey: ["rgd-resource-graph", name, namespace],
    queryFn: () => getRGDResourceGraph(name, namespace),
    enabled: !!name,
    staleTime: 30 * 1000, // Cache for 30 seconds
  });
}

/**
 * Hook for fetching the CRD schema for an RGD
 */
export function useRGDSchema(name: string, namespace?: string) {
  return useQuery({
    queryKey: ["rgd-schema", name, namespace],
    queryFn: () => getRGDSchema(name, namespace),
    enabled: !!name,
    staleTime: 60 * 1000, // Cache for 1 minute; WebSocket invalidation handles most updates
  });
}

/**
 * Hook for fetching instances of a specific RGD
 */
export function useRGDInstances(rgdName: string, namespace?: string) {
  return useQuery({
    queryKey: ["rgd-instances", rgdName, namespace],
    queryFn: () => listRGDInstances(rgdName, namespace),
    enabled: !!rgdName,
    staleTime: 30 * 1000, // Refresh instances every 30 seconds
  });
}

/**
 * Hook for creating an instance of an RGD
 */
export function useCreateInstance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateInstanceRequest) => createInstance(request),
    onSuccess: (data) => {
      // Invalidate instances list to refresh
      queryClient.invalidateQueries({ queryKey: ["rgd-instances", data.rgdName] });
      // Invalidate global instances list (used by InstancesPage)
      queryClient.invalidateQueries({ queryKey: ["instances"] });
      // Also invalidate the RGD list to update instance count
      queryClient.invalidateQueries({ queryKey: ["rgds"] });
    },
  });
}

/**
 * Hook for fetching K8s resources for ExternalRef selectors
 * Used to populate dropdowns with existing resources of a specific type
 */
export function useK8sResources(
  apiVersion: string,
  kind: string,
  namespace?: string,
  enabled: boolean = true
) {
  return useQuery({
    queryKey: ["k8s-resources", apiVersion, kind, namespace],
    queryFn: () => listK8sResources(apiVersion, kind, namespace),
    enabled: enabled && !!apiVersion && !!kind,
    staleTime: 30 * 1000, // Cache for 30 seconds
    retry: (failureCount, error) => {
      // Don't retry on 403 (forbidden) errors
      if (error && typeof error === "object" && "response" in error) {
        const axiosError = error as { response?: { status?: number } };
        if (axiosError.response?.status === 403) {
          return false;
        }
      }
      return failureCount < 2;
    },
  });
}
