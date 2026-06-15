// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Re-exports the tab-building logic from the shared lib so deploy-directory
 * consumers can import from a co-located module.  The canonical implementation
 * lives in `@/lib/build-tabs` so non-deploy consumers (e.g. tests, hooks) are
 * unaffected.
 */
export {
  buildTabsFromSchema,
  RESERVED_BASICS_KEYS,
  RESERVED_TAB_IDS,
  TOP_LEVEL_GENERAL_OBJECT_KEYS,
} from "@/lib/build-tabs";

export type { DeployTab, TabKind } from "@/lib/build-tabs";
