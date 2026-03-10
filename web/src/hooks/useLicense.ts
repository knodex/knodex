// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getLicenseStatus, updateLicense } from "@/api/license";
import { isEnterprise } from "./useCompliance";

/**
 * Hook for fetching the current license status.
 * Only enabled when enterprise features are active.
 */
export function useLicenseStatus() {
  return useQuery({
    queryKey: ["license", "status"],
    queryFn: getLicenseStatus,
    enabled: isEnterprise(),
    staleTime: 5 * 60 * 1000, // 5 minutes - license doesn't change often
    retry: (failureCount, error) => {
      // Don't retry on auth errors
      if (error && "status" in (error as Record<string, unknown>) && (error as Record<string, unknown>).status === 401) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for checking if the system is licensed (convenience wrapper).
 * Returns false if not enterprise, loading, or error.
 */
export function useIsLicensed(): boolean {
  const { data } = useLicenseStatus();
  if (!isEnterprise()) return false;
  return data?.licensed ?? false;
}

/**
 * Hook for checking if a specific feature is enabled in the license.
 * Returns true when:
 * - License is valid or in grace period AND feature is in the license
 * - License is expired (read-only mode) AND feature was in the license
 *   (backend allows GET requests in read-only mode per AC-7)
 * Returns false if not enterprise or feature not in the license.
 */
export function useIsFeatureEnabled(feature: string): boolean {
  const { data } = useLicenseStatus();
  if (!isEnterprise()) return false;
  // Feature is enabled when licensed (valid or grace period)
  if (data?.licensed && data?.license?.features?.includes(feature)) return true;
  // Feature is also accessible in read-only mode (expired past grace period)
  // Backend still serves GET requests for features that were in the license
  if (data?.status === "expired" && data?.license?.features?.includes(feature)) return true;
  return false;
}

/**
 * Hook for checking if the license is in grace period.
 */
export function useIsGracePeriod(): boolean {
  const { data } = useLicenseStatus();
  return data?.status === "grace_period";
}

/**
 * Hook for updating the license token (admin-only).
 * Invalidates the license status cache on success.
 */
export function useUpdateLicense() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (token: string) => updateLicense(token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["license"] });
      // Also invalidate compliance and views which depend on license
      queryClient.invalidateQueries({ queryKey: ["compliance"] });
      queryClient.invalidateQueries({ queryKey: ["views"] });
    },
  });
}
