// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { QueryClient } from "@tanstack/react-query";

/** Standardized staleTime presets — use these instead of raw millisecond literals.
 *  Mapping: REALTIME ≤10s, FREQUENT 30s, STANDARD 60s, STATIC 5min */
export const STALE_TIME = {
  REALTIME: 10_000,    // 10s — actively changing data (WebSocket-pushed, auto-refresh)
  FREQUENT: 30_000,    // 30s — frequently updated resources
  STANDARD: 60_000,    // 60s — moderate update frequency
  STATIC: 300_000,     // 5min — rarely changing config
} as const;

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: STALE_TIME.STATIC,
      gcTime: 1000 * 60 * 10, // Keep unused cache for 10 min (> staleTime to avoid immediate GC)
      retry: 1,
      refetchOnWindowFocus: false, // WebSocket handles real-time updates
    },
  },
});
