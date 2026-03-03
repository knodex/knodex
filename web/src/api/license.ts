import apiClient from "./client";
import type { LicenseStatus } from "@/types/license";

/**
 * Get the current license status
 * @returns License status including validity, features, and expiry info
 */
export async function getLicenseStatus(): Promise<LicenseStatus> {
  const response = await apiClient.get<LicenseStatus>("/v1/license");
  return response.data;
}

/**
 * Update the license with a new JWT token (admin-only)
 * @param token - The new license JWT string
 * @returns Updated license status
 */
export async function updateLicense(token: string): Promise<LicenseStatus> {
  const response = await apiClient.post<LicenseStatus>("/v1/license", { token });
  return response.data;
}
