// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listProjects,
  getProject,
  createProject,
  updateProject,
  deleteProject,
} from "@/api/projects";
import type { CreateProjectRequest, UpdateProjectRequest } from "@/types/project";

/**
 * Hook for fetching projects list
 */
export function useProjects() {
  return useQuery({
    queryKey: ["projects"],
    queryFn: () => listProjects(),
    staleTime: 5 * 60 * 1000, // 5 minutes - projects don't change often
  });
}

/**
 * Hook for fetching a single project by name
 */
export function useProject(name: string) {
  return useQuery({
    queryKey: ["project", name],
    queryFn: () => getProject(name),
    enabled: !!name,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook for creating a new project
 */
export function useCreateProject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateProjectRequest) => createProject(request),
    onSuccess: () => {
      // Invalidate projects list
      queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
  });
}

/**
 * Hook for updating a project
 */
export function useUpdateProject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      name,
      request,
    }: {
      name: string;
      request: UpdateProjectRequest;
    }) => updateProject(name, request),
    onSuccess: (data) => {
      // Update cache
      queryClient.setQueryData(["project", data.name], data);
      // Invalidate list
      queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
  });
}

/**
 * Hook for deleting a project
 */
export function useDeleteProject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => deleteProject(name),
    onSuccess: (_, name) => {
      // Remove from cache
      queryClient.removeQueries({ queryKey: ["project", name] });
      // Invalidate list
      queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
  });
}
