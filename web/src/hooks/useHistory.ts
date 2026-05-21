// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation } from "@tanstack/react-query";
import {
  getInstanceHistory,
  getInstanceTimeline,
  getInstanceKubernetesEvents,
  getInstanceEvents,
  exportInstanceHistory,
  downloadHistoryExport,
} from "@/api/history";
import type { HistoryExportFormat } from "@/types/history";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching deployment history for an instance
 */
export function useInstanceHistory(group: string, namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance-history", group, namespace, kind, name],
    queryFn: () => getInstanceHistory(group, namespace, kind, name),
    enabled: !!kind && !!name,
    staleTime: STALE_TIME.STANDARD, // history doesn't change frequently
  });
}

/**
 * Hook for fetching timeline for an instance
 */
export function useInstanceTimeline(group: string, namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance-timeline", group, namespace, kind, name],
    queryFn: () => getInstanceTimeline(group, namespace, kind, name),
    enabled: !!kind && !!name,
    staleTime: STALE_TIME.STANDARD,
  });
}

/**
 * Hook for fetching Kubernetes Events for an instance (STORY-406)
 * Uses ?source=kubernetes filter on the timeline endpoint
 */
export function useInstanceKubernetesEvents(group: string, namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance-timeline", group, namespace, kind, name, "kubernetes"],
    queryFn: () => getInstanceKubernetesEvents(group, namespace, kind, name),
    enabled: !!kind && !!name,
    staleTime: STALE_TIME.FREQUENT, // events change more frequently than deployment history
  });
}

/**
 * Hook for fetching K8s Events for an instance and its child resources.
 * Queries the K8s API on-demand (not stored in Redis). Auto-refreshes every 15s.
 */
export function useInstanceEvents(group: string, namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance-events", group, namespace, kind, name],
    queryFn: () => getInstanceEvents(group, namespace, kind, name),
    enabled: !!kind && !!name,
    staleTime: STALE_TIME.REALTIME,
    refetchInterval: 15 * 1000, // Auto-refresh every 15s
  });
}

/**
 * Hook for exporting deployment history
 */
export function useExportHistory() {
  return useMutation({
    mutationFn: async ({
      group,
      namespace,
      kind,
      name,
      format,
    }: {
      group: string;
      namespace: string;
      kind: string;
      name: string;
      format: HistoryExportFormat;
    }) => {
      const blob = await exportInstanceHistory(group, namespace, kind, name, format);
      downloadHistoryExport(blob, name, format);
      return { success: true };
    },
  });
}
