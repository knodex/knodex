import apiClient from "./client";

/**
 * K8s resource item returned from the API
 */
export interface K8sResourceItem {
  name: string;
  namespace: string;
  labels?: Record<string, string>;
  createdAt: string;
}

/**
 * Response from the K8s resources endpoint
 */
export interface K8sResourceListResponse {
  items: K8sResourceItem[];
  count: number;
}

/**
 * List K8s resources by type
 * Used for ExternalRef selectors in deployment forms
 *
 * @param apiVersion - Resource API version (e.g., "v1", "apps/v1")
 * @param kind - Resource kind (e.g., "ConfigMap", "Secret")
 * @param namespace - Optional namespace to filter by
 * @returns List of K8s resources
 */
export async function listK8sResources(
  apiVersion: string,
  kind: string,
  namespace?: string
): Promise<K8sResourceItem[]> {
  const params: Record<string, string> = { apiVersion, kind };
  if (namespace) {
    params.namespace = namespace;
  }

  const response = await apiClient.get<K8sResourceListResponse>(
    "/v1/resources",
    { params }
  );
  return response.data.items;
}
