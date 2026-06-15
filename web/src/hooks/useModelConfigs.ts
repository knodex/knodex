// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getModelConfigs, patchAgentModel } from "@/api/agents";
import type { AgentsResponse } from "@/api/agents";
import { STALE_TIME } from "@/lib/query-client";

/**
 * Lists the ModelConfigs an agent can be repointed at. STATIC staleTime —
 * ModelConfigs are operator-managed cluster config that changes rarely. Only
 * enabled when both namespace and name are known (the edit modal is open), so
 * it never fires for collapsed cards.
 */
export function useModelConfigs(namespace: string | undefined, name: string | undefined) {
  return useQuery({
    queryKey: ["agents", "modelconfigs", namespace, name],
    queryFn: () => getModelConfigs(namespace as string, name as string),
    enabled: Boolean(namespace) && Boolean(name),
    staleTime: STALE_TIME.STATIC,
  });
}

/**
 * Repoints an agent at a ModelConfig (patches only spec.declarative.modelConfig).
 * On success it consumes the server's echoed {provider, name} to patch the one
 * edited agent in the cached agent list (instant badge update, no full
 * refetch / N re-resolves), then invalidates ["agents","list"] as a
 * reconciling backstop.
 */
export function usePatchAgentModel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      namespace,
      name,
      modelConfig,
    }: {
      namespace: string;
      name: string;
      modelConfig: string;
    }) => patchAgentModel(namespace, name, modelConfig),
    onSuccess: (model, { namespace, name, modelConfig }) => {
      queryClient.setQueryData<AgentsResponse>(["agents", "list"], (prev) => {
        if (!prev) return prev;
        const patch = (a: (typeof prev.agents)[number]) =>
          a.namespace === namespace && a.name === name
            ? { ...a, model: model.provider || model.name ? model : undefined, modelConfig }
            : a;
        return { agents: prev.agents.map(patch) };
      });
      queryClient.invalidateQueries({ queryKey: ["agents", "list"] });
    },
  });
}
