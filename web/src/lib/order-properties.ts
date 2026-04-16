// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import type { FormProperty } from "@/types/rgd";

/**
 * Orders entries by a propertyOrder array. Listed fields come first
 * in annotation order; unlisted fields follow alphabetically.
 * When propertyOrder is absent/empty, pure alphabetical.
 */
export function orderEntries<T>(
  entries: [string, T][],
  propertyOrder?: string[]
): [string, T][] {
  if (!propertyOrder?.length) {
    return [...entries].sort(([a], [b]) => a.localeCompare(b));
  }

  // Deduplicate: first occurrence wins
  const orderMap = new Map<string, number>();
  for (let i = 0; i < propertyOrder.length; i++) {
    if (!orderMap.has(propertyOrder[i])) {
      orderMap.set(propertyOrder[i], i);
    }
  }

  return [...entries].sort(([a], [b]) => {
    const aIdx = orderMap.get(a) ?? Number.MAX_SAFE_INTEGER;
    const bIdx = orderMap.get(b) ?? Number.MAX_SAFE_INTEGER;
    if (aIdx === bIdx) return a.localeCompare(b);
    return aIdx - bIdx;
  });
}

/**
 * Convenience alias for FormProperty entries (most common usage).
 */
export function orderProperties(
  entries: [string, FormProperty][],
  propertyOrder?: string[]
): [string, FormProperty][] {
  return orderEntries(entries, propertyOrder);
}
