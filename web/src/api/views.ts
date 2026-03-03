import apiClient from "./client";
import type { View, ViewList } from "@/types/view";

/**
 * List all configured views with RGD counts.
 * Enterprise feature - returns 404 in OSS builds.
 * @returns List of views with counts
 */
export async function listViews(): Promise<ViewList> {
  const response = await apiClient.get<ViewList>("/v1/ee/views");
  return response.data;
}

/**
 * Get a specific view by slug.
 * Enterprise feature - returns 404 in OSS builds.
 * @param slug - View slug
 * @returns View details
 */
export async function getView(slug: string): Promise<View> {
  const response = await apiClient.get<View>(
    `/v1/ee/views/${encodeURIComponent(slug)}`
  );
  return response.data;
}

/**
 * Check if the current API response indicates views feature is unavailable.
 * This happens in OSS builds (404) or when no views are configured (503).
 * @param error - Error object from API call
 * @returns True if views feature is unavailable
 */
export function isViewsUnavailable(error: unknown): boolean {
  if (error && typeof error === "object" && "status" in error) {
    const status = (error as { status: number }).status;
    return status === 404 || status === 503;
  }
  if (error && typeof error === "object" && "response" in error) {
    const axiosError = error as { response?: { status?: number } };
    const status = axiosError.response?.status;
    return status === 404 || status === 503;
  }
  return false;
}
