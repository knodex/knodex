// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useQuery } from "@tanstack/react-query";
import { getSettings, type Settings } from "@/api/settings";

export function useSettings() {
  return useQuery<Settings>({
    queryKey: ["settings"],
    queryFn: getSettings,
    // Organization identity is set at server startup and never changes at runtime.
    // Unlike WebSocket-driven hooks, there's no push invalidation — a full page
    // reload is required if the server restarts with a different KNODEX_ORGANIZATION.
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  });
}
