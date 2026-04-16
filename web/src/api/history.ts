// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";
import { instancePath } from "./rgd";
import type { DeploymentHistory, TimelineResponse, HistoryExportFormat, InstanceEventsResponse } from "@/types/history";

/**
 * Get deployment history for an instance
 */
export async function getInstanceHistory(
  namespace: string,
  kind: string,
  name: string
): Promise<DeploymentHistory> {
  const response = await apiClient.get<DeploymentHistory>(
    `${instancePath(namespace, kind, name)}/history`
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
    `${instancePath(namespace, kind, name)}/timeline`
  );
  return response.data;
}

/**
 * Get Kubernetes Events for an instance (filtered timeline with ?source=kubernetes)
 */
export async function getInstanceKubernetesEvents(
  namespace: string,
  kind: string,
  name: string
): Promise<TimelineResponse> {
  const response = await apiClient.get<TimelineResponse>(
    `${instancePath(namespace, kind, name)}/timeline?source=kubernetes`
  );
  return response.data;
}

/**
 * Get Kubernetes Events for an instance and its child resources (on-demand from K8s API)
 */
export async function getInstanceEvents(
  namespace: string,
  kind: string,
  name: string
): Promise<InstanceEventsResponse> {
  const response = await apiClient.get<InstanceEventsResponse>(
    `${instancePath(namespace, kind, name)}/events`
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
    `${instancePath(namespace, kind, name)}/history/export`,
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
