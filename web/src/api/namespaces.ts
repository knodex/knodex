// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";

/**
 * Response type for namespace list endpoints
 */
export interface NamespaceListResponse {
  namespaces: string[];
  count: number;
}

/**
 * List all cluster namespaces (admin only)
 * @param excludeSystem - If true, excludes system namespaces like kube-system
 */
export async function listNamespaces(
  excludeSystem: boolean = true
): Promise<NamespaceListResponse> {
  const response = await apiClient.get<NamespaceListResponse>("/v1/namespaces", {
    params: { excludeSystem },
  });
  return response.data;
}

/**
 * List namespaces allowed for a specific project
 * Returns real Kubernetes namespaces that match the project's destination patterns
 */
export async function getProjectNamespaces(
  projectName: string
): Promise<NamespaceListResponse> {
  const response = await apiClient.get<NamespaceListResponse>(
    `/v1/projects/${encodeURIComponent(projectName)}/namespaces`
  );
  return response.data;
}
