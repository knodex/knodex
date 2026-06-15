// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listTeams,
  createTeam,
  updateTeam,
  deleteTeam,
  listObservedGroups,
} from "@/api/teams";
import type { CreateTeamRequest, UpdateTeamRequest } from "@/types/team";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching the teams list (operator-gated; 403 surfaces as an error).
 */
export function useTeams() {
  return useQuery({
    queryKey: ["teams"],
    queryFn: () => listTeams(),
    staleTime: STALE_TIME.STATIC, // teams change rarely
  });
}

/**
 * Hook for the observed-groups typeahead source (Story 10.3). Tolerates an
 * empty list (fresh cluster) — callers fall back to free-text entry.
 */
export function useObservedGroups() {
  return useQuery({
    queryKey: ["observed-groups"],
    queryFn: () => listObservedGroups(),
    staleTime: STALE_TIME.STATIC,
  });
}

/**
 * Hook for creating a team.
 */
export function useCreateTeam() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (request: CreateTeamRequest) => createTeam(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

/**
 * Hook for updating a team.
 */
export function useUpdateTeam() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      request,
    }: {
      name: string;
      request: UpdateTeamRequest;
    }) => updateTeam(name, request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}

/**
 * Hook for deleting a team.
 */
export function useDeleteTeam() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteTeam(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["teams"] });
    },
  });
}
