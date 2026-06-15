// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { listSecrets, createSecret, checkSecretExists, updateSecret, deleteSecret } from "@/api/secrets";
import type { CreateSecretRequest, UpdateSecretRequest } from "@/types/secret";
import { STALE_TIME } from "@/lib/query-client";
import { is404 } from "@/lib/errors";

/**
 * Hook for fetching secrets list across the user's accessible namespaces.
 * The optional `namespace` filter narrows to a single namespace. The list
 * is server-filtered to the caller's accessible namespaces — no project
 * lens is required.
 */
export function useSecretList(
  options?: { namespace?: string; limit?: number; continue?: string },
) {
  const namespace = options?.namespace;
  return useQuery({
    queryKey: ["secrets", namespace ?? "*", options?.limit, options?.continue],
    queryFn: () => listSecrets(options),
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for creating a new secret. Namespace is passed per-call (not at
 * hook construction) so the component can drive it from form state.
 */
export function useCreateSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ namespace, ...req }: { namespace: string } & CreateSecretRequest) => {
      return createSecret(namespace, req);
    },
    onSuccess: (_, { namespace }) => {
      queryClient.invalidateQueries({ queryKey: ["secrets"] });
      queryClient.invalidateQueries({ queryKey: ["secret", namespace] });
    },
  });
}

/**
 * Hook for updating a secret. The namespace is the access boundary.
 */
export function useUpdateSecret() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      name,
      namespace,
      ...req
    }: { name: string; namespace: string } & UpdateSecretRequest) => {
      return updateSecret(name, namespace, req);
    },
    onSuccess: (_, { name, namespace }) => {
      queryClient.invalidateQueries({ queryKey: ["secrets"] });
      queryClient.invalidateQueries({ queryKey: ["secret", namespace, name] });
    },
  });
}

/**
 * Hook for checking if a secret exists (lightweight existence check).
 * Returns { exists, isLoading, isError } based on the HEAD endpoint.
 * 404 is treated as "not found" (exists=false), not as an error.
 */
export function useSecretExists(name: string, namespace: string) {
  const query = useQuery({
    queryKey: ["secret", namespace, name, "exists"],
    queryFn: async () => {
      try {
        await checkSecretExists(name, namespace);
        return true;
      } catch (err) {
        // 404 means "not found" — treat as exists=false, not an error.
        // Use the shared predicate so interceptor-normalized ApiError 404s
        // (top-level .status) are caught alongside raw axios errors.
        if (is404(err)) {
          return false;
        }
        throw err;
      }
    },
    enabled: !!name && !!namespace,
    staleTime: STALE_TIME.STATIC, // secrets rarely change between page loads
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
      namespace,
    }: {
      name: string;
      namespace: string;
    }) => {
      return deleteSecret(name, namespace);
    },
    onSuccess: (_, { name, namespace }) => {
      queryClient.invalidateQueries({ queryKey: ["secrets"] });
      queryClient.invalidateQueries({ queryKey: ["secret", namespace, name] });
    },
  });
}
