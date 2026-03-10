// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import apiClient from "./client";

/**
 * Represents a Kubernetes API resource
 */
export interface APIResource {
  apiGroup: string; // Empty string for core API group
  kind: string;
}

/**
 * Response from the API resources endpoint
 */
export interface APIResourcesResponse {
  resources: APIResource[];
  count: number;
}

/**
 * Parameters for listing API resources
 */
export interface APIResourcesParams {
  search?: string;
  apiGroup?: string;
}

/**
 * Get the display name for an API group
 * Empty string (core API group) is displayed as "core"
 */
export function getApiGroupDisplayName(apiGroup: string): string {
  return apiGroup === "" ? "core" : apiGroup;
}

/**
 * Get the API group value from a display name
 * "core" is converted back to empty string for API requests
 */
export function getApiGroupValue(displayName: string): string {
  return displayName === "core" ? "" : displayName;
}

/**
 * List available Kubernetes API resources from the cluster
 * @param params - Optional search and filter parameters
 * @returns List of API resources with their groups and kinds
 */
export async function listApiResources(
  params?: APIResourcesParams
): Promise<APIResourcesResponse> {
  const queryParams = new URLSearchParams();

  if (params?.search) queryParams.append("search", params.search);
  if (params?.apiGroup !== undefined) {
    // Convert "core" to empty string for the API
    const apiGroupValue = params.apiGroup === "core" ? "" : params.apiGroup;
    queryParams.append("apiGroup", apiGroupValue);
  }

  const queryString = queryParams.toString();
  const url = queryString
    ? `/v1/kubernetes/api-resources?${queryString}`
    : "/v1/kubernetes/api-resources";

  const response = await apiClient.get<APIResourcesResponse>(url);
  return response.data;
}
