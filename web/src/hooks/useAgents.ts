// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { listAgents, listModels, createModel, listAgentTemplates } from "@/api/agents";
import type { CreateModelRequest } from "@/api/agents";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Hook for fetching the caller's Casbin-scoped agents (Story 53.2). FREQUENT
 * staleTime — agent deployments change when teammates deploy. Only mounted
 * inside the ready-state workspace, so it never fires against a kagent-less
 * cluster.
 */
export function useAgents() {
  return useQuery({
    queryKey: ["agents", "list"],
    queryFn: listAgents,
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for fetching the caller's Casbin-scoped models (Story 53.4). FREQUENT
 * staleTime — model configs change when teammates create them. The list is
 * server-filtered to accessible namespaces.
 */
export function useModels() {
  return useQuery({
    queryKey: ["agents", "models"],
    queryFn: listModels,
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for fetching agent-template RGDs (discovered by schema.kind). FREQUENT
 * staleTime — templates change when operators apply/remove agent RGDs.
 */
export function useAgentTemplates() {
  return useQuery({
    queryKey: ["agents", "templates"],
    queryFn: listAgentTemplates,
    staleTime: STALE_TIME.FREQUENT,
  });
}

/**
 * Hook for creating a model. On success invalidates the models list so the new
 * model surfaces (modulo KRO reconcile latency for the downstream ModelConfig).
 */
export function useCreateModel() {
  const queryClient = useQueryClient();
  return useMutation({
    // Wrap so createModel receives ONLY the request (React Query passes a
    // second context arg to a bare mutationFn that would otherwise leak through).
    mutationFn: (req: CreateModelRequest) => createModel(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["agents", "models"] });
    },
  });
}
