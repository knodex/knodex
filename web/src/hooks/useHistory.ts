import { useQuery, useMutation } from "@tanstack/react-query";
import {
  getInstanceHistory,
  getInstanceTimeline,
  exportInstanceHistory,
  downloadHistoryExport,
} from "@/api/history";
import type { HistoryExportFormat } from "@/types/history";

/**
 * Hook for fetching deployment history for an instance
 */
export function useInstanceHistory(namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance-history", namespace, kind, name],
    queryFn: () => getInstanceHistory(namespace, kind, name),
    enabled: !!namespace && !!kind && !!name,
    staleTime: 60 * 1000, // 1 minute - history doesn't change frequently
  });
}

/**
 * Hook for fetching timeline for an instance
 */
export function useInstanceTimeline(namespace: string, kind: string, name: string) {
  return useQuery({
    queryKey: ["instance-timeline", namespace, kind, name],
    queryFn: () => getInstanceTimeline(namespace, kind, name),
    enabled: !!namespace && !!kind && !!name,
    staleTime: 60 * 1000, // 1 minute
  });
}

/**
 * Hook for exporting deployment history
 */
export function useExportHistory() {
  return useMutation({
    mutationFn: async ({
      namespace,
      kind,
      name,
      format,
    }: {
      namespace: string;
      kind: string;
      name: string;
      format: HistoryExportFormat;
    }) => {
      const blob = await exportInstanceHistory(namespace, kind, name, format);
      downloadHistoryExport(blob, name, format);
      return { success: true };
    },
  });
}
