// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import {
  getAuditEvents,
  getAuditEvent,
  getAuditStats,
  getAuditConfig,
  updateAuditConfig,
} from "@/api/audit";
import { isEnterprise } from "@/hooks/useCompliance";
import type { AuditEventFilter, AuditConfig } from "@/types/audit";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching paginated audit events with optional filtering.
 * Only enabled when enterprise features are active.
 */
export function useAuditEvents(params?: AuditEventFilter) {
  return useQuery({
    queryKey: ["audit", "events", params],
    queryFn: () => getAuditEvents(params),
    enabled: isEnterprise(),
    placeholderData: keepPreviousData,
    staleTime: STALE_TIME.FREQUENT,
    retry: (failureCount, error) => {
      if (is403(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching a single audit event by ID.
 */
export function useAuditEvent(id: string | null) {
  return useQuery({
    queryKey: ["audit", "event", id],
    queryFn: () => getAuditEvent(id!),
    enabled: isEnterprise() && !!id,
    staleTime: STALE_TIME.STANDARD,
    retry: (failureCount, error) => {
      if (is403(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching aggregate audit statistics.
 * Auto-refreshes every 30 seconds when the component is mounted.
 */
export function useAuditStats() {
  return useQuery({
    queryKey: ["audit", "stats"],
    queryFn: getAuditStats,
    enabled: isEnterprise(),
    staleTime: STALE_TIME.FREQUENT,
    refetchInterval: 30_000, // polling interval — independent of staleTime
    retry: (failureCount, error) => {
      if (is403(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching audit configuration.
 */
export function useAuditConfig() {
  return useQuery({
    queryKey: ["audit", "config"],
    queryFn: getAuditConfig,
    enabled: isEnterprise(),
    staleTime: STALE_TIME.STANDARD,
    retry: (failureCount, error) => {
      if (is403(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * Hook for updating audit configuration.
 * Invalidates config query on success.
 */
export function useUpdateAuditConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (config: AuditConfig) => updateAuditConfig(config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["audit", "config"] });
      queryClient.invalidateQueries({ queryKey: ["audit", "stats"] });
    },
  });
}

/** Check if error is a 403 Forbidden */
function is403(error: unknown): boolean {
  if (error && typeof error === "object" && "response" in error) {
    return (error as { response?: { status?: number } }).response?.status === 403;
  }
  return false;
}
