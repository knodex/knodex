// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listRepositories,
  getRepository,
  createRepository,
  updateRepository,
  deleteRepository,
} from "@/api/repository";
import type { CreateRepositoryRequest, UpdateRepositoryRequest } from "@/types/repository";

/**
 * Hook for fetching repositories list, optionally filtered by project
 */
export function useRepositories(projectId?: string) {
  return useQuery({
    queryKey: ["repositories", projectId ?? "all"],
    queryFn: () => listRepositories(projectId),
    staleTime: 5 * 60 * 1000, // 5 minutes - repositories don't change often
  });
}

/**
 * Hook for fetching a single repository by ID
 */
export function useRepository(id: string) {
  return useQuery({
    queryKey: ["repository", id],
    queryFn: () => getRepository(id),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook for creating a new repository
 */
export function useCreateRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateRepositoryRequest) => createRepository(request),
    onSuccess: () => {
      // Invalidate repositories list
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
    },
  });
}

/**
 * Hook for updating a repository
 */
export function useUpdateRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      request,
    }: {
      id: string;
      request: UpdateRepositoryRequest;
    }) => updateRepository(id, request),
    onSuccess: (data) => {
      // Update cache
      queryClient.setQueryData(["repository", data.id], data);
      // Invalidate list
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
    },
  });
}

/**
 * Hook for deleting a repository
 */
export function useDeleteRepository() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => deleteRepository(id),
    onSuccess: (_, id) => {
      // Remove from cache
      queryClient.removeQueries({ queryKey: ["repository", id] });
      // Invalidate list
      queryClient.invalidateQueries({ queryKey: ["repositories"] });
    },
  });
}
