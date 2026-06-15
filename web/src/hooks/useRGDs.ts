// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, keepPreviousData, useQueryClient } from "@tanstack/react-query";
import { listRGDs, getRGD, getRGDSchema, listRGDInstances, createInstance, getRGDResourceGraph, getRGDDefinitionGraph, getRGDRevision, getRGDRevisions, getRGDRevisionDiff, getRGDFilters } from "@/api/rgd";
import { listK8sResources } from "@/api/k8s";
import type { RGDListParams, CreateInstanceRequest } from "@/types/rgd";
import { STALE_TIME } from "@/lib/query-client";
import { is403 } from "@/lib/errors";

/**
 * Hook for fetching paginated RGD list
 */
export function useRGDList(params?: RGDListParams) {
  return useQuery({
    queryKey: ["rgds", params],
    queryFn: () => listRGDs(params),
    enabled: params !== undefined,
    placeholderData: keepPreviousData,
    staleTime: STALE_TIME.FREQUENT, // prevent immediate refetches on filter changes
  });
}

/**
 * Hook for fetching authorized RGD filter options (categories, tags, projects).
 * The server returns only categories the user is authorized to see via Casbin policies.
 */
export function useRGDFilters() {
  return useQuery({
    queryKey: ["rgds", "filters"],
    queryFn: getRGDFilters,
    staleTime: STALE_TIME.FREQUENT,
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
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for fetching the definition graph of an RGD
 * Uses the new /graph endpoint with collection metadata (isCollection, forEach, readyWhen)
 */
export function useRGDDefinitionGraph(name: string, namespace?: string) {
  return useQuery({
    queryKey: ["rgd-definition-graph", name, namespace],
    queryFn: () => getRGDDefinitionGraph(name, namespace),
    enabled: !!name,
    staleTime: STALE_TIME.FREQUENT,
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
    staleTime: STALE_TIME.STANDARD, // WebSocket invalidation handles most updates
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
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for creating an instance of an RGD
 */
export function useCreateInstance() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ group, kind, ...request }: CreateInstanceRequest & { group: string; kind: string }) =>
      createInstance(group, kind, request),
    onSuccess: (data) => {
      // Invalidate instances list to refresh
      queryClient.invalidateQueries({ queryKey: ["rgd-instances", data.rgdName] });
      // Invalidate global instances list (used by InstancesPage)
      queryClient.invalidateQueries({ queryKey: ["instances"] });
      // Also invalidate the RGD list to update instance count
      queryClient.invalidateQueries({ queryKey: ["rgds"] });
      // Keep the Agents hub fresh: a deployed kagent-agent RGD instance must
      // appear in Installed Agents without waiting out the 30s staleTime.
      // Unconditional — detecting agent-ness client-side isn't worth it; a
      // non-agent invalidation is one cheap LIST (Story 49.3).
      queryClient.invalidateQueries({ queryKey: ["agents", "installed"] });
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
    staleTime: STALE_TIME.FREQUENT,
    retry: (failureCount, error) => {
      // Don't retry on 403 (forbidden) errors
      if (is403(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching GraphRevision history for an RGD
 */
export function useRGDRevisions(rgdName: string) {
  return useQuery({
    queryKey: ["rgd", rgdName, "revisions"],
    queryFn: () => getRGDRevisions(rgdName),
    enabled: !!rgdName,
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for fetching a single GraphRevision by number (includes snapshot).
 * staleTime: Infinity because revisions are immutable.
 */
export function useRGDRevision(rgdName: string, revision: number | null) {
  return useQuery({
    queryKey: ["rgd", rgdName, "revision", revision],
    queryFn: () => getRGDRevision(rgdName, revision!),
    enabled: !!rgdName && revision !== null,
    staleTime: Infinity,
  });
}

/**
 * Hook for fetching the structured diff between two RGD revisions.
 * staleTime: Infinity because revisions are immutable — diffs never change.
 */
export function useRGDRevisionDiff(rgdName: string, rev1: number | null, rev2: number | null) {
  return useQuery({
    queryKey: ["rgd", rgdName, "revisions", "diff", rev1, rev2],
    queryFn: () => getRGDRevisionDiff(rgdName, rev1!, rev2!),
    enabled: !!rgdName && rev1 !== null && rev2 !== null,
    staleTime: Infinity,
  });
}
