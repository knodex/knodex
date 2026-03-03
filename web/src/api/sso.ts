import apiClient from "./client";
import type {
  SSOProvider,
  CreateSSOProviderRequest,
  UpdateSSOProviderRequest,
} from "@/types/sso";

/**
 * List all SSO providers
 */
export async function listSSOProviders(): Promise<SSOProvider[]> {
  const response = await apiClient.get<SSOProvider[]>(
    "/v1/settings/sso/providers"
  );
  return response.data;
}

/**
 * Get a single SSO provider by name
 */
export async function getSSOProvider(name: string): Promise<SSOProvider> {
  const response = await apiClient.get<SSOProvider>(
    `/v1/settings/sso/providers/${encodeURIComponent(name)}`
  );
  return response.data;
}

/**
 * Create a new SSO provider
 */
export async function createSSOProvider(
  request: CreateSSOProviderRequest
): Promise<SSOProvider> {
  const response = await apiClient.post<SSOProvider>(
    "/v1/settings/sso/providers",
    request
  );
  return response.data;
}

/**
 * Update an existing SSO provider
 */
export async function updateSSOProvider(
  name: string,
  request: UpdateSSOProviderRequest
): Promise<SSOProvider> {
  const response = await apiClient.put<SSOProvider>(
    `/v1/settings/sso/providers/${encodeURIComponent(name)}`,
    request
  );
  return response.data;
}

/**
 * Delete an SSO provider
 */
export async function deleteSSOProvider(name: string): Promise<void> {
  await apiClient.delete(
    `/v1/settings/sso/providers/${encodeURIComponent(name)}`
  );
}
