import apiClient from "./client";
import type { DeploymentHistory, TimelineResponse, HistoryExportFormat } from "@/types/history";

/**
 * Get deployment history for an instance
 */
export async function getInstanceHistory(
  namespace: string,
  kind: string,
  name: string
): Promise<DeploymentHistory> {
  const response = await apiClient.get<DeploymentHistory>(
    `/v1/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}/history`
  );
  return response.data;
}

/**
 * Get timeline for an instance (simplified view for UI)
 */
export async function getInstanceTimeline(
  namespace: string,
  kind: string,
  name: string
): Promise<TimelineResponse> {
  const response = await apiClient.get<TimelineResponse>(
    `/v1/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}/timeline`
  );
  return response.data;
}

/**
 * Export deployment history in specified format
 */
export async function exportInstanceHistory(
  namespace: string,
  kind: string,
  name: string,
  format: HistoryExportFormat = "json"
): Promise<Blob> {
  const response = await apiClient.get(
    `/v1/instances/${encodeURIComponent(namespace)}/${encodeURIComponent(kind)}/${encodeURIComponent(name)}/history/export`,
    {
      params: { format },
      responseType: "blob",
    }
  );
  return response.data;
}

/**
 * Download exported history file
 */
export function downloadHistoryExport(
  blob: Blob,
  instanceName: string,
  format: HistoryExportFormat
): void {
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${instanceName}-deployment-history.${format}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  window.URL.revokeObjectURL(url);
}
