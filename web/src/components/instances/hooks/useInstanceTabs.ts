// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useMemo } from "react";
import {
  Activity,
  Clock,
  Code,
  Puzzle,
  Link2,
  Boxes,
  Zap,
} from "@/lib/icons";
import { useRGDList } from "@/hooks/useRGDs";
import { useDynamicTabs } from "@/hooks/useDynamicTabs";
import type { Tab, ConditionalTab } from "@/hooks/useDynamicTabs";
import type { Instance } from "@/types/rgd";
import { createElement } from "react";

export type InstanceTabId = "status" | "addons" | "deployment-history" | "external-refs" | "spec" | "children" | "events";

export interface InstanceTabCounts {
  events: number;
  externalRefs: number;
  resourcesReady: number;
  resourcesTotal: number;
  history: number;
}

export function useInstanceTabs(
  instance: Instance,
  counts: InstanceTabCounts,
  hasSpec: boolean,
) {
  // Fetch add-ons count for tab visibility (React Query deduplicates with InstanceAddOns)
  const { data: addOnsData } = useRGDList(
    instance.kind ? { extendsKind: instance.kind, pageSize: 100 } : undefined
  );
  const addOnsCount = addOnsData?.totalCount ?? 0;

  const baseTabs = useMemo<Tab<InstanceTabId>[]>(() => [
    { id: "status", label: "Overview", icon: createElement(Activity, { className: "h-4 w-4" }) },
    {
      id: "children",
      label: "Resources",
      icon: createElement(Boxes, { className: "h-4 w-4" }),
      count: counts.resourcesTotal > 0 ? `${counts.resourcesReady}/${counts.resourcesTotal}` : undefined,
      countVariant: counts.resourcesTotal > 0 && counts.resourcesReady < counts.resourcesTotal ? "warn" : "default",
    },
    {
      id: "events",
      label: "Events",
      icon: createElement(Zap, { className: "h-4 w-4" }),
      count: counts.events > 0 ? String(counts.events) : undefined,
    },
    {
      id: "deployment-history",
      label: "History",
      icon: createElement(Clock, { className: "h-4 w-4" }),
      count: counts.history > 0 ? String(counts.history) : undefined,
    },
  ], [counts.resourcesReady, counts.resourcesTotal, counts.events, counts.history]);

  // Build conditional tabs
  const conditionalTabs = useMemo<ConditionalTab<InstanceTabId>[]>(() => [
    {
      condition: counts.externalRefs > 0,
      tab: {
        id: "external-refs",
        label: "References",
        icon: createElement(Link2, { className: "h-4 w-4" }),
        count: String(counts.externalRefs),
      },
      position: 3, // after Events
    },
    {
      condition: addOnsCount > 0,
      tab: {
        id: "addons",
        label: "Add-ons",
        icon: createElement(Puzzle, { className: "h-4 w-4" }),
        count: String(addOnsCount),
      },
      position: 4,
    },
    {
      condition: hasSpec,
      tab: { id: "spec", label: "Spec", icon: createElement(Code, { className: "h-4 w-4" }) },
    },
  ], [addOnsCount, counts.externalRefs, hasSpec]);

  const { tabs, activeTab, setActiveTab } = useDynamicTabs(baseTabs, conditionalTabs, "status" as InstanceTabId);

  return {
    tabs,
    activeTab,
    setActiveTab,
  };
}
