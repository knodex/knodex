import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  listSSOProviders,
  getSSOProvider,
  createSSOProvider,
  updateSSOProvider,
  deleteSSOProvider,
} from "@/api/sso";
import type {
  CreateSSOProviderRequest,
  UpdateSSOProviderRequest,
} from "@/types/sso";

/**
 * Hook for fetching SSO providers list
 */
export function useSSOProviders() {
  return useQuery({
    queryKey: ["sso-providers"],
    queryFn: () => listSSOProviders(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook for fetching a single SSO provider by name
 */
export function useSSOProvider(name: string) {
  return useQuery({
    queryKey: ["sso-provider", name],
    queryFn: () => getSSOProvider(name),
    enabled: !!name,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook for creating a new SSO provider
 */
export function useCreateSSOProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateSSOProviderRequest) =>
      createSSOProvider(request),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["sso-providers"] });
    },
  });
}

/**
 * Hook for updating an SSO provider
 */
export function useUpdateSSOProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      name,
      request,
    }: {
      name: string;
      request: UpdateSSOProviderRequest;
    }) => updateSSOProvider(name, request),
    onSuccess: (data) => {
      queryClient.setQueryData(["sso-provider", data.name], data);
      queryClient.invalidateQueries({ queryKey: ["sso-providers"] });
    },
  });
}

/**
 * Hook for deleting an SSO provider
 */
export function useDeleteSSOProvider() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => deleteSSOProvider(name),
    onSuccess: (_, name) => {
      queryClient.removeQueries({ queryKey: ["sso-provider", name] });
      queryClient.invalidateQueries({ queryKey: ["sso-providers"] });
    },
  });
}
