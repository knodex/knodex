// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listRoleTemplates,
  createRoleTemplate,
  updateRoleTemplate,
  deleteRoleTemplate,
} from "@/api/role-templates";
import type { RoleTemplate } from "@/api/role-templates";
import { STALE_TIME } from "@/lib/query-client";

/** Shared React Query key for the role-template catalog. */
export const ROLE_TEMPLATES_QUERY_KEY = ["role-templates"] as const;

/**
 * Hook for fetching the role-template catalog.
 *
 * Used both by the Settings → Role Templates page (operator-gated; a 403
 * surfaces as an error there) and by the project-role preset buttons
 * (AddRoleForm / roles-step), where preset access piggybacks on the same
 * operator-readable catalog. Templates change rarely, so STATIC stale time.
 */
export function useRoleTemplates() {
  return useQuery({
    queryKey: ROLE_TEMPLATES_QUERY_KEY,
    queryFn: () => listRoleTemplates(),
    staleTime: STALE_TIME.STATIC,
  });
}

/** Hook for creating a role template. */
export function useCreateRoleTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (template: RoleTemplate) => createRoleTemplate(template),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ROLE_TEMPLATES_QUERY_KEY });
    },
  });
}

/** Hook for updating a role template (name is immutable). */
export function useUpdateRoleTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      name,
      template,
    }: {
      name: string;
      template: RoleTemplate;
    }) => updateRoleTemplate(name, template),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ROLE_TEMPLATES_QUERY_KEY });
    },
  });
}

/** Hook for deleting a role template. */
export function useDeleteRoleTemplate() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => deleteRoleTemplate(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ROLE_TEMPLATES_QUERY_KEY });
    },
  });
}
