// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import { useRGDList } from "./useRGDs";
import type { CatalogRGD } from "@/types/rgd";

/** Max catalog page size for Kind-to-RGD resolution (server max is 100). */
const KIND_RESOLUTION_PAGE_SIZE = 100;

interface KindToRGDMapResult {
  kindToRGD: Map<string, CatalogRGD>;
  isLoading: boolean;
}

/**
 * Shared hook that returns a Map from Kind to CatalogRGD.
 * Used by DependsOnKindLink and DependsOnTab to resolve
 * dependency Kinds to their parent RGDs without each component
 * independently fetching the full catalog.
 *
 * React Query deduplicates the underlying API call, so multiple
 * components using this hook share a single request.
 *
 * When multiple RGDs define the same Kind, the first match wins
 * (consistent with the backend's GetRGDByKind behavior).
 */
export function useKindToRGDMap(): KindToRGDMapResult {
  const { data, isLoading } = useRGDList({ pageSize: KIND_RESOLUTION_PAGE_SIZE });

  const kindToRGD = useMemo(() => {
    const map = new Map<string, CatalogRGD>();
    if (data?.items) {
      for (const item of data.items) {
        // First-match wins — consistent with backend GetRGDByKind()
        if (item.kind && !map.has(item.kind)) {
          map.set(item.kind, item);
        }
      }
    }
    return map;
  }, [data]);

  return { kindToRGD, isLoading };
}
