// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { AxiosError } from "axios";
import { listSecrets, createSecret, checkSecretExists, getSecret, updateSecret, deleteSecret } from "@/api/secrets";
import type { CreateSecretRequest, UpdateSecretRequest } from "@/types/secret";
/**
 * Hook for fetching secrets list for a project with optional pagination.
 */
export function useSecretList(
  project: string,
  options?: { limit?: number; continue?: string },
) {
  return useQuery({
    queryKey: ["secrets", project, options?.limit, options?.continue],
    queryFn: () => listSecrets(project, options),
    enabled: !!project,
    staleTime: 30 * 1000,
  });
}

/**
 * Hook for creating a new secret.
 * Project is passed per-call (not at hook construction) to avoid stale closures
 * when the selected project changes while the hook is mounted.
 */
export function useCreateSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ project, ...req }: { project: string } & CreateSecretRequest) => {
      return createSecret(project, req);
    },
    onSuccess: (_, { project }) => {
      queryClient.invalidateQueries({ queryKey: ["secrets", project] });
    },
  });
}

/**
 * Hook for fetching a single secret with full data (values included)
 */
export function useSecret(name: string, project: string, namespace: string) {
  return useQuery({
    queryKey: ["secrets", project, namespace, name],
    queryFn: () => getSecret(name, project, namespace),
    enabled: !!name && !!project && !!namespace,
    staleTime: 30 * 1000,
  });
}

/**
 * Hook for updating a secret.
 * Project is passed per-call to avoid stale closures.
 */
export function useUpdateSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      name,
      project,
      ...req
    }: { name: string; project: string } & UpdateSecretRequest) => {
      return updateSecret(name, project, req);
    },
    onSuccess: (_, { project, name, namespace }) => {
      queryClient.invalidateQueries({ queryKey: ["secrets", project] });
      queryClient.invalidateQueries({ queryKey: ["secrets", project, namespace, name] });
    },
  });
}

/**
 * Hook for checking if a secret exists (lightweight existence check).
 * Returns { exists, isLoading, isError } based on GET single secret API.
 * 404 is treated as "not found" (exists=false), not as an error.
 */
export function useSecretExists(name: string, project: string, namespace: string) {
  const query = useQuery({
    queryKey: ["secrets", project, namespace, name, "exists"],
    queryFn: async () => {
      try {
        await checkSecretExists(name, project, namespace);
        return true;
      } catch (err) {
        // 404 means "not found" — treat as exists=false, not an error
        if (err instanceof AxiosError && err.response?.status === 404) {
          return false;
        }
        throw err;
      }
    },
    enabled: !!name && !!project && !!namespace,
    staleTime: 5 * 60 * 1000, // 5 minutes — secrets rarely change between page loads
    refetchOnWindowFocus: false,
  });

  return {
    exists: query.data ?? undefined,
    isLoading: query.isLoading,
    isError: query.isError,
  };
}

/**
 * Hook for deleting a secret.
 */
export function useDeleteSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      name,
      project,
      namespace,
    }: {
      name: string;
      project: string;
      namespace: string;
    }) => {
      return deleteSecret(name, project, namespace);
    },
    onSuccess: (_, { project }) => {
      queryClient.invalidateQueries({ queryKey: ["secrets", project] });
    },
  });
}
